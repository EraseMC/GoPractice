package duel

import (
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/EraseMC/GoPractice/internal/chat"
	"github.com/EraseMC/GoPractice/internal/combat"
	"github.com/EraseMC/GoPractice/internal/handler"
	"github.com/EraseMC/GoPractice/internal/hub"
	"github.com/EraseMC/GoPractice/internal/items"
	"github.com/EraseMC/GoPractice/internal/lang"
	"github.com/EraseMC/GoPractice/internal/overall"
	"github.com/EraseMC/GoPractice/internal/punish"
	"github.com/EraseMC/GoPractice/internal/scheduler"
	"github.com/EraseMC/GoPractice/internal/state"
	"github.com/EraseMC/GoPractice/internal/stats"
	"github.com/EraseMC/GoPractice/internal/store"
	"github.com/EraseMC/GoPractice/internal/toggles"
	"github.com/EraseMC/GoPractice/internal/ui"
	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/entity"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/google/uuid"
)

func TestDuelLifecycle(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	bundle, players, profiles, forms := lang.MustLoad(), state.NewPlayers(), store.NewMemory(), ui.NewForms()
	boards := ui.NewScoreboards(players)
	sched := scheduler.New(log)
	toggle := toggles.New(bundle, profiles, players, forms)
	stats := stats.New(bundle, profiles, players, forms)
	chat := chat.New(log, bundle, profiles, players, punish.New(bundle, profiles), sched.Tick)
	combatManager := combat.New(players, toggle, sched)
	itemsManager := items.New(bundle, players, combatManager)
	lobby := world.Config{Log: log}.New()
	t.Cleanup(func() { _ = lobby.Close() })
	h := hub.New(hub.Deps{Log: log, Lang: bundle, Store: profiles, Players: players, Forms: forms, Boards: boards, Chat: chat, Toggles: toggle, Stats: stats})
	if err := h.Setup(lobby); err != nil {
		t.Fatal(err)
	}
	m := New(Deps{Log: log, Lang: bundle, Players: players, Forms: forms, Boards: boards, Chat: chat, Toggles: toggle, Stats: stats, Combat: combatManager, Items: itemsManager, Hub: h})
	t.Cleanup(m.Close)
	if err := m.Load(filepath.Join("..", "..", "duels")); err != nil {
		t.Fatal(err)
	}
	for _, id := range ModeIDs() {
		if len(m.pool[id]) == 0 {
			t.Fatalf("no map for %s", id)
		}
	}
	h.InQueue = m.InQueue
	router := handler.NewRouter()
	router.Add(combatManager.Handler())
	router.Add(m.Handler())
	router.Add(&overall.Handler{AllowHunger: m.AllowsHunger, AllowDrop: m.AllowsDrop})
	router.AddLast(combatManager.Tail())
	combatManager.Profile = func(o *state.Online, _ *player.Player) combat.Profile {
		profile, _ := Profile(o)
		return profile
	}
	var a, b *state.Online
	do := func(w *world.World, f func(*world.Tx)) {
		t.Helper()
		if err := w.Do(f).Wait(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	do(lobby, func(tx *world.Tx) {
		add := func(name string) (*state.Online, *player.Player) {
			id := uuid.New()
			p := tx.AddEntity(world.EntitySpawnOpts{ID: id}.New(player.Type, player.Config{Name: name, UUID: id, GameMode: world.GameModeAdventure})).(*player.Player)
			p.Handle(router.Handler())
			o := players.Add(p, "", nil, lang.EN)
			o.SetPlace(state.PlaceHub, "")
			return o, p
		}
		var ap, bp *player.Player
		a, ap = add("Alpha")
		b, bp = add("Beta")
		kind, _ := findMode("boxing")
		m.enqueue(a, ap, tx, kind)
		if !m.InQueue(a) {
			t.Error("first player did not enter queue")
		}
		m.Refresh(a, ap)
		s, _ := ap.Inventory().Item(8)
		if _, ok := s.Item().(item.RedstoneWire); !ok {
			t.Error("missing queue exit item")
		}
		m.enqueue(b, bp, tx, kind)
		if m.InQueue(a) || m.InQueue(b) {
			t.Error("matched players remain queued")
		}
	})
	active := func() *match {
		t.Helper()
		m.mu.Lock()
		defer m.mu.Unlock()
		if len(m.matches) != 1 {
			t.Fatalf("active matches = %d", len(m.matches))
		}
		for _, match := range m.matches {
			return match
		}
		return nil
	}
	boxing := active()
	do(boxing.world, func(tx *world.Tx) {
		first, ok := playerIn(tx, a)
		second, ok2 := playerIn(tx, b)
		if !ok || !ok2 || !boxing.joined[0] || !boxing.joined[1] {
			t.Error("players did not reach the duel world")
			return
		}
		hit := func() {
			combatManager.ResetImmunity(second.UUID())
			second.Hurt(5, entity.AttackDamageSource{Attacker: first})
		}
		hit()
		if boxing.hits != [2]int{} || second.Health() != 20 {
			t.Error("damage accepted during countdown")
		}
		for range 4 {
			m.tickMatch(tx, boxing)
		}
		for range 100 {
			hit()
		}
		if boxing.phase != ended || boxing.players[boxing.winner] != a || second.GameMode() != world.GameModeSpectator {
			t.Error("the 100th Boxing hit did not end the match")
		}
		boxing.finished = time.Now().Add(-4 * time.Second)
		m.tickMatch(tx, boxing)
	})
	do(lobby, func(tx *world.Tx) {
		ap, ok := playerIn(tx, a)
		bp, ok2 := playerIn(tx, b)
		if !ok || !ok2 || !a.In(state.PlaceHub) || !b.In(state.PlaceHub) || m.Count() != 0 {
			t.Error("match cleanup did not return both players to the hub")
			return
		}
		m.Command(a, ap, tx, []string{"invite", b.Name, "sumo"})
		m.Command(b, bp, tx, []string{"accept"})
	})
	sumo := active()
	do(sumo.world, func(tx *world.Tx) {
		ap, ok := playerIn(tx, a)
		bp, ok2 := playerIn(tx, b)
		if !ok || !ok2 || !ap.Immobile() || !bp.Immobile() {
			t.Error("Sumo players must be frozen during countdown")
			return
		}
		for range 4 {
			m.tickMatch(tx, sumo)
		}
		if ap.Immobile() || bp.Immobile() {
			t.Error("Sumo players remained frozen after the countdown")
		}
		pos := cube.PosFromVec3(ap.Position()).Add(cube.Pos{0, 3, 0})
		tx.SetLiquid(pos, block.Water{Depth: 8})
		ctx := player.NewEventContext(tx, ap)
		router.Handler().HandleMove(ctx, pos.Vec3Centre(), ap.Rotation())
		if !ctx.Cancelled() || sumo.phase != ended || m.Surrender(b, bp, tx) || sumo.players[sumo.winner] != b {
			t.Error("water contact did not record a single Sumo winner")
		}
		m.release(tx, sumo)
	})
	do(lobby, func(tx *world.Tx) {
		ap, ok := playerIn(tx, a)
		bp, ok2 := playerIn(tx, b)
		if !ok || !ok2 {
			t.Error("players did not return after surrender")
			return
		}
		m.Command(a, ap, tx, []string{"invite", b.Name, "classic_nodebuff"})
		m.Command(b, bp, tx, []string{"accept", a.Name})
	})
	classic := active()
	do(classic.world, func(tx *world.Tx) {
		ap, ok := playerIn(tx, a)
		if !ok {
			t.Error("player did not reach Classic NoDebuff")
			return
		}
		sword, _ := ap.Inventory().Item(0)
		food, _ := ap.Inventory().Item(8)
		if m.AllowsHunger(ap) || m.AllowsDrop(ap, sword) || !m.AllowsDrop(ap, food) || food.Count() != 64 || remaining(classic.mode, ap) != 30 {
			t.Error("unexpected Classic NoDebuff kit or waiting rules")
		}
		for range 4 {
			m.tickMatch(tx, classic)
		}
		if !m.AllowsHunger(ap) {
			t.Error("Classic NoDebuff must allow hunger during combat")
		}
		router.Handler().HandleQuit(ap)
		if classic.phase != ended || classic.players[classic.winner] != b {
			t.Error("disconnect did not count as a loss")
		}
	})
	do(classic.world, func(tx *world.Tx) { m.release(tx, classic) })
	do(lobby, func(tx *world.Tx) {
		bp, ok := playerIn(tx, b)
		if !ok {
			t.Error("winner did not return after disconnect")
			return
		}
		id := uuid.New()
		p := tx.AddEntity(world.EntitySpawnOpts{ID: id}.New(player.Type, player.Config{Name: "Gamma", UUID: id, GameMode: world.GameModeAdventure})).(*player.Player)
		p.Handle(router.Handler())
		a = players.Add(p, "", nil, lang.EN)
		a.SetPlace(state.PlaceHub, "")
		m.Command(a, p, tx, []string{"invite", b.Name, "fist"})
		m.Command(b, bp, tx, []string{"accept"})
	})
	fist := active()
	do(fist.world, func(tx *world.Tx) {
		ap, ok := playerIn(tx, a)
		if !ok || !m.Surrender(a, ap, tx) || fist.phase != ended || fist.players[fist.winner] != b {
			t.Error("surrender during countdown did not record a loss")
		}
	})

}
