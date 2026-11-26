package ffa

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/EraseMC/GoPractice/internal/chat"
	"github.com/EraseMC/GoPractice/internal/combat"
	"github.com/EraseMC/GoPractice/internal/handler"
	"github.com/EraseMC/GoPractice/internal/items"
	"github.com/EraseMC/GoPractice/internal/lang"
	"github.com/EraseMC/GoPractice/internal/punish"
	"github.com/EraseMC/GoPractice/internal/scheduler"
	"github.com/EraseMC/GoPractice/internal/state"
	"github.com/EraseMC/GoPractice/internal/stats"
	"github.com/EraseMC/GoPractice/internal/store"
	"github.com/EraseMC/GoPractice/internal/toggles"
	"github.com/EraseMC/GoPractice/internal/ui"
	"github.com/df-mc/dragonfly/server/entity"
	"github.com/df-mc/dragonfly/server/entity/effect"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/item/enchantment"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/google/uuid"
)

type env struct {
	w       *world.World
	m       *Manager
	players *state.Players
	router  *handler.Router
	bundle  *lang.Bundle
}

func setup(t *testing.T) *env {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	bundle := lang.MustLoad()
	players := state.NewPlayers()
	st := store.NewMemory()
	sched := scheduler.New(log)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go sched.Run(ctx)

	forms := ui.NewForms()
	tg := toggles.New(bundle, st, players, forms)
	c := combat.New(players, tg, sched)
	im := items.New(bundle, players, c)
	m := New(Deps{
		Log:     log,
		Lang:    bundle,
		Players: players,
		Forms:   forms,
		Boards:  ui.NewScoreboards(players),
		Chat:    chat.New(log, bundle, st, players, punish.New(bundle, st), sched.Tick),
		Toggles: tg,
		Stats:   stats.New(bundle, st, players, forms),
		Combat:  c,
		Items:   im,
	})
	c.Profile, c.Enemy, c.InCombat = m.Profile, m.EnemyName, m.InCombat
	r := handler.NewRouter()
	r.Add(c.Handler())
	r.Add(im.Handler())
	r.Add(m.Handler())
	r.AddLast(im.Tail())
	r.AddLast(c.Tail())

	w := world.Config{}.New()
	t.Cleanup(func() { _ = w.Close() })
	for _, a := range m.arenas {
		a.world = w
	}
	return &env{w: w, m: m, players: players, router: r, bundle: bundle}
}

func (e *env) do(t *testing.T, f func(tx *world.Tx)) {
	t.Helper()
	if err := e.w.Do(f).Wait(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func (e *env) spawn(tx *world.Tx, name string) (*state.Online, *player.Player) {
	id := uuid.New()
	conf := player.Config{Name: name, UUID: id, GameMode: world.GameModeAdventure}
	p := tx.AddEntity(world.EntitySpawnOpts{ID: id}.New(player.Type, conf)).(*player.Player)
	p.Handle(e.router.Handler())
	return e.players.Add(p, "", nil, lang.EN), p
}

func (e *env) arena(k Kind) *Arena {
	for _, a := range e.m.arenas {
		if a.Kind == k {
			return a
		}
	}
	return nil
}

func TestKits(t *testing.T) {
	e := setup(t)
	e.do(t, func(tx *world.Tx) {
		_, p := e.spawn(tx, "Kit")
		for _, a := range e.m.arenas {
			e.m.SetPlayerSettings(p)
			a.GiveKit(p)
			inv := p.Inventory()
			switch a.Kind {
			case NoDebuff:
				if Potions(p) != 34 {
					t.Fatalf("nodebuff potions = %d, want 34", Potions(p))
				}
				if s, _ := inv.Item(1); s.Count() != 16 {
					t.Fatalf("nodebuff pearls = %d", s.Count())
				}
				h := p.Armour().Helmet()
				if !h.Unbreakable() {
					t.Fatal("nodebuff helmet must be unbreakable")
				}
				if e, ok := h.Enchantment(enchantment.Unbreaking); !ok || e.Level() != 10 {
					t.Fatal("nodebuff helmet must have unbreaking 10")
				}
				if _, ok := p.Effect(effect.Speed); !ok {
					t.Fatal("nodebuff must give speed")
				}
			case Soup:
				if Soups(p) != 16 {
					t.Fatalf("soups = %d, want 16", Soups(p))
				}
				if s, _ := inv.Item(0); s.Item() != (item.Sword{Tier: item.ToolTierStone}) {
					t.Fatalf("soup slot 0 = %v", s.Item())
				}
			case Sumo, Resistance:
				eff, ok := p.Effect(effect.Resistance)
				if !ok || eff.Level() != 256 {
					t.Fatalf("%s resistance = %v", a.Name, eff)
				}
			case GApple:
				if s, _ := inv.Item(1); s.Count() != 8 {
					t.Fatalf("gapples = %d", s.Count())
				}
			case Combo:
				if p.Armour().Helmet().Unbreakable() {
					t.Fatal("combo armour must be breakable")
				}
				if s, _ := inv.Item(1); s.Count() != 3 {
					t.Fatalf("enchanted apples = %d", s.Count())
				}
			}
		}
	})
}

func hit(attacker, victim *player.Player, dmg float64) {
	victim.Hurt(dmg, entity.AttackDamageSource{Attacker: attacker})
}

func TestCombatAndKill(t *testing.T) {
	e := setup(t)
	a := e.arena(Fist)
	e.do(t, func(tx *world.Tx) {
		ao, ap := e.spawn(tx, "Attacker")
		vo, vp := e.spawn(tx, "Victim")
		wo, wp := e.spawn(tx, "Witness")
		for _, x := range []struct {
			o *state.Online
			p *player.Player
		}{{ao, ap}, {vo, vp}, {wo, wp}} {
			e.m.JoinArena(x.o, x.p, tx, a)
		}
		if e.m.Online(a) != 3 {
			t.Fatalf("online = %d", e.m.Online(a))
		}

		hit(ap, vp, 1)
		if e.m.EnemyName(ao) != "Victim" || e.m.EnemyName(vo) != "Attacker" {
			t.Fatal("first hit must start combat")
		}
		if vp.Health() != 19 {
			t.Fatalf("first hit health = %v, want 19", vp.Health())
		}

		before := wp.Health()
		hit(ap, wp, 1)
		if wp.Health() != before || e.m.InCombat(wo) {
			t.Fatal("attacker in combat must not hit a third player")
		}

		e.m.Combat.ResetImmunity(vp.UUID())
		vp.SetMaxHealth(1)
		hit(ap, vp, 1)
		if e.m.InCombat(ao) || e.m.InCombat(vo) {
			t.Fatal("kill must clear combat")
		}
		if vp.GameMode() != world.GameModeSpectator {
			t.Fatal("dead player must be a spectator")
		}
		if !e.m.HasRespawnDelay(vo) {
			t.Fatal("dead player must have a respawn delay")
		}
		if vp.Health() != vp.MaxHealth() || vp.Dead() {
			t.Fatal("dead player must keep full health")
		}
	})
}

func TestKillArgs(t *testing.T) {
	e := setup(t)
	e.do(t, func(tx *world.Tx) {
		_, dp := e.spawn(tx, "D")
		_, kp := e.spawn(tx, "K")
		nd := e.arena(NoDebuff)
		nd.GiveKit(dp)
		nd.GiveKit(kp)
		kp.Hurt(3.4, entity.VoidDamageSource{})
		got := e.bundle.Textf(lang.EN, nd.KillKey, nd.killArgs("D", "K", dp, kp)...)
		want := "§eD §7[§c34 POTS§7] §fwas killed by §eK §7[§c34 POTS§7] §7[§417 HP§7]"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
		sumo := e.arena(Sumo)
		if got := e.bundle.Textf(lang.EN, sumo.KillKey, sumo.killArgs("D", "K", dp, kp)...); got != "§eD §fwas killed by §eK" {
			t.Fatalf("sumo got %q", got)
		}
	})
}
