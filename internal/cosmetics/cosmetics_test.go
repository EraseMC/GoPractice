package cosmetics

import (
	"io"
	"log/slog"
	"testing"

	"github.com/EraseMC/GoPractice/internal/chat"
	"github.com/EraseMC/GoPractice/internal/gamemode"
	"github.com/EraseMC/GoPractice/internal/handler"
	"github.com/EraseMC/GoPractice/internal/hub"
	"github.com/EraseMC/GoPractice/internal/lang"
	"github.com/EraseMC/GoPractice/internal/netmw"
	"github.com/EraseMC/GoPractice/internal/punish"
	"github.com/EraseMC/GoPractice/internal/rank"
	"github.com/EraseMC/GoPractice/internal/state"
	"github.com/EraseMC/GoPractice/internal/store"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/player/skin"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func TestCosmeticsLifecycle(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	profiles, players, bundle := store.NewMemory(), state.NewPlayers(), lang.MustLoad()
	chatManager := chat.New(log, bundle, profiles, players, punish.New(bundle, profiles), func() uint64 { return 0 })
	h := hub.New(hub.Deps{Log: log, Store: profiles, Players: players, Chat: chatManager})
	m, err := New(Deps{Lang: bundle, Store: profiles, Players: players, Hub: h})
	if err != nil {
		t.Fatal(err)
	}
	w := world.Config{Log: log}.New()
	t.Cleanup(func() { _ = w.Close() })
	if err := h.Setup(w); err != nil {
		t.Fatal(err)
	}
	r := handler.NewRouter()
	r.Add(m.Handler())

	if err := w.Do(func(tx *world.Tx) {
		id := uuid.New()
		cfg := player.Config{Name: "CosmeticsPlayer", UUID: id, Position: mgl64.Vec3{801, 55, 780}, GameMode: world.GameModeAdventure}
		p := tx.AddEntity(world.EntitySpawnOpts{ID: id}.New(player.Type, cfg)).(*player.Player)
		o := players.Add(p, "1", nil, lang.EN)
		o.SetPlace(state.PlaceHub, "")
		p.Handle(r.Handler())
		profiles.Login(store.Identity{XUID: o.XUID, Name: o.Name, Locale: "en"})
		profiles.Update(o.XUID, func(profile *store.Profile) {
			profile.Rank = rank.Gangster
			profile.Cosmetics = store.Cosmetics{Cape: "Oldtralia", Tag: "GOAT", Fly: true, PotionColor: "Dark_Red", Capybara: true}
		})
		m.Join(o, p)
		if p.Skin().FullID != "Oldtralia_Default" || len(p.Skin().Cape.Pix) != 8192 {
			t.Error("expected default skin with the selected cape")
		}
		persona := skin.New(64, 64)
		persona.Persona = true
		p.SetSkin(persona)
		if p.Skin().Persona || p.Skin().CapeID != "Oldtralia" {
			t.Error("changing to a persona skin lost the applied cape or bypassed the adapter")
		}
		m.RankChanged(o, p)
		if p.GameMode() != gamemode.AdventureFly {
			t.Error("flight was not enabled in the hub")
		}
		if c, ok := m.PotionColour(o); !ok || c.R != 139 || c.A != 255 {
			t.Error("stored potion colour was not loaded")
		}

		m.Enter(tx, p)
		pet := m.pets[id]
		if pet == nil {
			t.Error("capybara was not spawned")
			return
		}
		actor, ok := pet.handle.Entity(tx)
		if !ok || actor.(*player.Player).Skin().ModelConfig.Default != "geometry.capybara" {
			t.Error("capybara has no actor with its custom model")
			return
		}
		if actor.Position() != p.Position() {
			t.Error("capybara did not spawn at the owner's position")
		}
		p.Teleport(p.Position().Add(mgl64.Vec3{2, 0, 0}))
		pet.HandleTick(player.NewEventContext(tx, actor.(*player.Player)), 1)
		if actor.Position() != p.Position() {
			t.Error("capybara didn't follow its owner")
		}
		viewer := &netmw.Conn{}
		pk := &packet.AddPlayer{UUID: pet.handle.UUID(), EntityRuntimeID: 17, EntityMetadata: protocol.NewEntityMetadata()}
		m.Outbound(viewer, pk)
		if pk.EntityMetadata[protocol.EntityDataKeySeatOffset] != (mgl32.Vec3{0.84, 0, 0}) || !pk.EntityMetadata.Flag(protocol.EntityDataKeyFlags, protocol.EntityDataFlagRiding) {
			t.Error("capybara is missing its riding metadata")
		}
		m.Outbound(viewer, &packet.RemoveActor{EntityUniqueID: 17})
		if len(pet.viewers) != 0 || len(m.views) != 0 {
			t.Error("removed viewers were retained")
		}

		m.Leave(tx, p)
		o.SetPlace(state.PlaceDuel, "")
		m.Enter(tx, p)
		if m.pets[id] != nil || !pet.handle.Closed() {
			t.Error("capybara stayed with its owner in a duel")
		}
		o.SetPlace(state.PlaceHub, "")
		m.Enter(tx, p)
		if m.pets[id] == nil {
			t.Error("capybara was not restored when returning to the hub")
		}
		profiles.Update(o.XUID, func(profile *store.Profile) {
			profile.Rank = rank.Wayner
			m.NormalizeProfile(profile)
		})
		m.RankChanged(o, p)
		if c := m.profile(o).Cosmetics; c != (store.Cosmetics{Fly: true}) {
			t.Errorf("saved cosmetics after demotion = %+v", c)
		}
		if len(p.Skin().Cape.Pix) != 0 || m.pets[id] != nil || p.GameMode() != gamemode.AdventureFly {
			t.Error("demotion did not remove the cape and pet while retaining allowed flight")
		}
	}).Wait(t.Context()); err != nil {
		t.Fatal(err)
	}
}
