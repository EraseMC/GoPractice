package hub

import (
	"math/rand/v2"
	"time"

	"github.com/EraseMC/GoPractice/assets"
	"github.com/EraseMC/GoPractice/internal/skins"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const (
	NPCNameTag     = "§6>§g>§e> §aOldtralia Network §e<§g<§6<"
	npcScale       = 1.5
	emoteInterval  = 8 * time.Second
	emoteProtocol  = 589
	emoteBreakdown = "1dbaa006-0ec6-42c3-9440-a3bfa0c6fdbe"
	emoteGiddy     = "738497ce-539f-4e06-9a03-dc528506a468"
	emoteElytra    = "7393aa53-9145-4e66-b23b-ec86def6c6f2"
	emoteKick      = "af61c759-6252-431d-a7de-94d477cfb54c"
)

var (
	npcPos    = mgl64.Vec3{801.5, 50, 772.5}
	npcRot    = cube.Rotation{1, 0}
	npcEmotes = []string{emoteElytra, emoteBreakdown, emoteKick, emoteGiddy}
)

type npc struct {
	m      *Manager
	handle *world.EntityHandle
	loader *world.Loader
}

func newNPC(m *Manager) (*npc, error) {
	s, err := skins.FromPNG(assets.FS, "skins/npc/npc.png")
	if err != nil {
		return nil, err
	}
	conf := player.Config{
		Name:     NPCNameTag,
		UUID:     uuid.New(),
		Skin:     s,
		Position: npcPos,
		Rotation: npcRot,
		GameMode: world.GameModeAdventure,
	}
	h := world.EntitySpawnOpts{Position: npcPos, Rotation: npcRot, ID: conf.UUID}.New(player.Type, conf)
	return &npc{m: m, handle: h}, nil
}

func (n *npc) spawn(tx *world.Tx) {
	n.loader = world.NewLoader(1, tx.World(), world.NopViewer{})
	n.loader.Move(tx, npcPos)
	n.loader.Load(tx, 9)
	p, ok := tx.AddEntity(n.handle).(*player.Player)
	if !ok {
		return
	}
	p.SetScale(npcScale)
	p.SetNameTag(NPCNameTag)
	p.SetAlwaysShowNameTag(true)
	p.SetImmobile()
	p.Handle(&npcHandler{n: n})
}

func (n *npc) is(e world.Entity) bool {
	return e != nil && n != nil && e.H() == n.handle
}

func (n *npc) emote(tx *world.Tx, p *player.Player, id string) {
	for _, o := range n.m.players.All() {
		if o.Conn == nil || o.Conn.Protocol() < emoteProtocol {
			continue
		}
		e, ok := o.Handle.Entity(tx)
		if !ok {
			continue
		}
		viewer, ok := e.(*player.Player)
		if !ok {
			continue
		}
		rid, ok := viewer.EntityRuntimeID(p)
		if !ok {
			continue
		}
		viewer.SendPacket(&packet.Emote{
			EntityRuntimeID: rid,
			EmoteID:         id,
			Flags:           packet.EmoteFlagServerSide | packet.EmoteFlagMuteChat,
		})
	}
}

type npcHandler struct {
	player.NopHandler
	n         *npc
	lastEmote time.Time
}

func (h *npcHandler) HandleTick(ctx *player.Context, _ int64) {
	ctx.Cancel()
	if time.Since(h.lastEmote) < emoteInterval {
		return
	}
	h.lastEmote = time.Now()
	h.n.emote(ctx.Player().Tx(), ctx.Player(), npcEmotes[rand.IntN(len(npcEmotes))])
}

func (h *npcHandler) HandleHurt(ctx *player.Context, _ *float64, _ bool, _ *time.Duration, _ world.DamageSource) {
	ctx.Cancel()
}

func (h *npcHandler) HandleFoodLoss(ctx *player.Context, _ int, _ *int) {
	ctx.Cancel()
}
