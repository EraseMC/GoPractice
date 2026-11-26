package items_test

import (
	"context"
	"io"
	"log/slog"
	"math"
	"testing"
	"time"

	"github.com/EraseMC/GoPractice/internal/combat"
	"github.com/EraseMC/GoPractice/internal/entities"
	"github.com/EraseMC/GoPractice/internal/handler"
	"github.com/EraseMC/GoPractice/internal/items"
	"github.com/EraseMC/GoPractice/internal/lang"
	"github.com/EraseMC/GoPractice/internal/scheduler"
	"github.com/EraseMC/GoPractice/internal/state"
	"github.com/EraseMC/GoPractice/internal/store"
	"github.com/EraseMC/GoPractice/internal/toggles"
	"github.com/EraseMC/GoPractice/internal/ui"
	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/entity"
	"github.com/df-mc/dragonfly/server/entity/effect"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/item/enchantment"
	"github.com/df-mc/dragonfly/server/item/potion"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/google/uuid"
)

type env struct {
	w       *world.World
	players *state.Players
	combat  *combat.Manager
	items   *items.Manager
	router  *handler.Router
}

func setup(t *testing.T) *env {
	t.Helper()
	bundle := lang.MustLoad()
	players := state.NewPlayers()
	sched := scheduler.New(slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go sched.Run(ctx)

	tg := toggles.New(bundle, store.NewMemory(), players, ui.NewForms())
	c := combat.New(players, tg, sched)
	im := items.New(bundle, players, c)
	r := handler.NewRouter()
	r.Add(c.Handler())
	r.Add(im.Handler())
	r.AddLast(im.Tail())
	r.AddLast(c.Tail())

	w := world.Config{Synchronous: true}.New()
	t.Cleanup(func() { _ = w.Close() })
	return &env{w: w, players: players, combat: c, items: im, router: r}
}

func (e *env) do(t *testing.T, f func(tx *world.Tx)) {
	t.Helper()
	if err := e.w.Do(f).Wait(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func (e *env) spawn(tx *world.Tx, name string, pos mgl64.Vec3, rot cube.Rotation) *player.Player {
	id := uuid.New()
	conf := player.Config{Name: name, UUID: id, Position: pos, Rotation: rot, GameMode: world.GameModeAdventure}
	p := tx.AddEntity(world.EntitySpawnOpts{ID: id, Position: pos, Rotation: rot}.New(player.Type, conf)).(*player.Player)
	p.Handle(e.router.Handler())
	e.players.Add(p, "", nil, lang.EN)
	return p
}

func diamondKit(p *player.Player) {
	prot := item.NewEnchantment(enchantment.Protection, 1)
	p.Armour().Set(
		item.NewStack(item.Helmet{Tier: item.ArmourTierDiamond{}}, 1).WithEnchantments(prot),
		item.NewStack(item.Chestplate{Tier: item.ArmourTierDiamond{}}, 1).WithEnchantments(prot),
		item.NewStack(item.Leggings{Tier: item.ArmourTierDiamond{}}, 1).WithEnchantments(prot),
		item.NewStack(item.Boots{Tier: item.ArmourTierDiamond{}}, 1).WithEnchantments(prot),
	)
}

func TestMeleeDamageImmunityAndKnockBack(t *testing.T) {
	e := setup(t)
	var attacker, victim *player.Player
	e.do(t, func(tx *world.Tx) {
		attacker = e.spawn(tx, "attacker", mgl64.Vec3{0.5, 64, 0.5}, cube.Rotation{0, 0})
		victim = e.spawn(tx, "victim", mgl64.Vec3{0.5, 64, 2.5}, cube.Rotation{180, 0})
		attacker.SetHeldItems(item.NewStack(item.Sword{Tier: item.ToolTierDiamond}, 1), item.Stack{})
		diamondKit(victim)

		if !attacker.AttackEntity(victim) {
			t.Fatal("first attack rejected")
		}
		lost := 20 - victim.Health()
		if lost < 1.344-1e-9 || lost > 1.6+1e-9 {
			t.Fatalf("pocketmine damage expected, lost %v", lost)
		}
		vel := victim.Velocity()
		if math.Abs(vel[0]) > 1e-9 || math.Abs(vel[1]-0.4) > 1e-9 || math.Abs(vel[2]-0.4) > 1e-9 {
			t.Fatalf("knockback %v", vel)
		}
		before := victim.Health()
		attacker.AttackEntity(victim)
		if victim.Health() != before {
			t.Fatal("hit during immunity was not cancelled")
		}
	})

	time.Sleep(time.Duration(combat.Default.Immunity+2) * scheduler.TickDuration)
	e.do(t, func(tx *world.Tx) {
		a, _ := attacker.H().Entity(tx)
		v, _ := victim.H().Entity(tx)
		attacker, victim = a.(*player.Player), v.(*player.Player)
		before := victim.Health()
		attacker.AttackEntity(victim)
		if victim.Health() >= before {
			t.Fatal("hit after immunity was not applied")
		}
	})
}

func TestResistanceTakesNoDamageButKnocksBack(t *testing.T) {
	e := setup(t)
	e.do(t, func(tx *world.Tx) {
		attacker := e.spawn(tx, "attacker", mgl64.Vec3{0.5, 64, 0.5}, cube.Rotation{0, 0})
		victim := e.spawn(tx, "victim", mgl64.Vec3{2.5, 64, 0.5}, cube.Rotation{0, 0})
		victim.AddEffect(effect.New(effect.Resistance, 255, time.Minute))
		attacker.SetHeldItems(item.NewStack(item.Stick{}, 1), item.Stack{})
		attacker.AttackEntity(victim)
		if victim.Health() != 20 {
			t.Fatalf("health %v", victim.Health())
		}
		if v := victim.Velocity(); math.Abs(v[0]-0.4) > 1e-9 {
			t.Fatalf("knockback %v", v)
		}
	})
}

func TestSoupAndPotion(t *testing.T) {
	e := setup(t)
	e.do(t, func(tx *world.Tx) {
		p := e.spawn(tx, "eater", mgl64.Vec3{0.5, 64, 0.5}, cube.Rotation{})
		p.Hurt(10, entity.FallDamageSource{})
		hp := p.Health()
		p.SetHeldItems(item.NewStack(item.MushroomStew{}, 1), item.Stack{})
		p.UseItem()
		if p.Health() != hp+6 {
			t.Fatalf("soup healed to %v from %v", p.Health(), hp)
		}
		if held, _ := p.HeldItems(); !held.Empty() {
			t.Fatalf("soup bowl kept: %v", held)
		}

		p.SetHeldItems(item.NewStack(item.MushroomStew{}, 1), item.Stack{})
		p.Heal(20, effect.InstantHealingSource{})
		p.UseItem()
		if held, _ := p.HeldItems(); held.Empty() {
			t.Fatal("soup consumed at full health")
		}
	})
}

func TestPearlTeleportsAndCooldown(t *testing.T) {
	e := setup(t)
	e.do(t, func(tx *world.Tx) {
		p := e.spawn(tx, "pearler", mgl64.Vec3{0.5, 64, 0.5}, cube.Rotation{0, 0})
		p.SetHeldItems(item.NewStack(item.EnderPearl{}, 16), item.Stack{})
		p.UseItem()
		if held, _ := p.HeldItems(); held.Count() != 15 {
			t.Fatalf("pearl not thrown, count %d", held.Count())
		}
		if left, ok := e.items.PearlCooldown(p.UUID()); !ok || left != items.PearlCooldown {
			t.Fatalf("cooldown %d %v", left, ok)
		}
		var pearl world.TickerEntity
		for ent := range tx.Entities() {
			if ent.H().Type() == entities.PearlType {
				pearl = ent.(world.TickerEntity)
			}
		}
		if pearl == nil {
			t.Fatal("pearl entity not spawned")
		}
		if sc, ok := pearl.(interface{ Scale() float64 }); !ok || sc.Scale() != 0.29 {
			t.Fatal("pearl scale")
		}
		pearlEnt := pearl.(world.Entity)
		start := pearlEnt.Position()
		pearl.Tick(tx, 1)
		moved := pearlEnt.Position().Sub(start)
		if math.Abs(moved[2]-2.425*0.991) > 1e-6 {
			t.Fatalf("pearl first step %v", moved)
		}
	})
}

func tickUntilGone(tx *world.Tx, h *world.EntityHandle, max int) bool {
	for i := 1; i <= max; i++ {
		ent, ok := h.Entity(tx)
		if !ok {
			return true
		}
		ent.(world.TickerEntity).Tick(tx, int64(i))
	}
	_, ok := h.Entity(tx)
	return !ok
}

func find(tx *world.Tx, t world.EntityType) *world.EntityHandle {
	for ent := range tx.Entities() {
		if ent.H().Type() == t {
			return ent.H()
		}
	}
	return nil
}

func TestPearlHitTeleportsOwner(t *testing.T) {
	e := setup(t)
	e.do(t, func(tx *world.Tx) {
		for y := 62; y <= 68; y++ {
			tx.SetBlock(cube.Pos{0, y, 6}, block.Stone{}, nil)
		}
		p := e.spawn(tx, "pearler", mgl64.Vec3{0.5, 64, 0.5}, cube.Rotation{0, 0})
		p.SetHeldItems(item.NewStack(item.EnderPearl{}, 1), item.Stack{})
		p.UseItem()
		h := find(tx, entities.PearlType)
		if h == nil {
			t.Fatal("pearl not spawned")
		}
		tickUntilGone(tx, h, 10)
		ent, _ := p.H().Entity(tx)
		pos := ent.Position()
		if math.Abs(pos[2]-6) > 1e-6 || math.Abs(pos[0]-0.5) > 1e-6 {
			t.Fatalf("owner not teleported to wall: %v", pos)
		}
	})
}

func TestSplashHealsArea(t *testing.T) {
	e := setup(t)
	e.do(t, func(tx *world.Tx) {
		for x := -2; x <= 2; x++ {
			for z := -2; z <= 3; z++ {
				tx.SetBlock(cube.Pos{x, 63, z}, block.Stone{}, nil)
			}
		}
		thrower := e.spawn(tx, "thrower", mgl64.Vec3{0.5, 64, 0.5}, cube.Rotation{0, 90})
		near := e.spawn(tx, "near", mgl64.Vec3{0.5, 64, 2}, cube.Rotation{})
		far := e.spawn(tx, "far", mgl64.Vec3{0.5, 64, 8}, cube.Rotation{})
		for _, p := range []*player.Player{thrower, near, far} {
			p.Hurt(10, entity.FallDamageSource{})
		}
		thrower.SetHeldItems(item.NewStack(item.SplashPotion{Type: potion.StrongHealing()}, 1), item.Stack{})
		thrower.UseItem()
		if held, _ := thrower.HeldItems(); !held.Empty() {
			t.Fatal("potion not thrown")
		}
		h := find(tx, entities.SplashType)
		if h == nil {
			t.Fatal("splash not spawned")
		}
		if !tickUntilGone(tx, h, 20) {
			t.Fatal("splash never landed")
		}
		want := 10 + 8*items.MaxMiss
		if math.Abs(thrower.Health()-want) > 1e-6 || math.Abs(near.Health()-want) > 1e-6 {
			t.Fatalf("healed thrower %v near %v, want %v", thrower.Health(), near.Health(), want)
		}
		if far.Health() != 10 {
			t.Fatalf("far player healed: %v", far.Health())
		}
	})
}

func TestDeathAnimation(t *testing.T) {
	e := setup(t)
	e.do(t, func(tx *world.Tx) {
		killer := e.spawn(tx, "killer", mgl64.Vec3{0.5, 64, 0.5}, cube.Rotation{})
		victim := e.spawn(tx, "victim", mgl64.Vec3{0.5, 64, 2.5}, cube.Rotation{})
		e.combat.DeathAnimation(tx, victim, killer.Position())
		if find(tx, entities.LightningType) == nil {
			t.Fatal("lightning not spawned")
		}
		corpses := 0
		for ent := range tx.Players() {
			p := ent.(*player.Player)
			if p.H() != killer.H() && p.H() != victim.H() {
				corpses++
				if !p.Dead() || p.NameTag() != "" {
					t.Fatalf("corpse alive %v or named %q", !p.Dead(), p.NameTag())
				}
			}
		}
		if corpses != 1 {
			t.Fatalf("corpses %d", corpses)
		}
	})
}
