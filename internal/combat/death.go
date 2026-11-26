package combat

import (
	"math"

	"github.com/EraseMC/GoPractice/internal/entities"
	"github.com/EraseMC/GoPractice/internal/state"
	"github.com/df-mc/dragonfly/server/entity"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const (
	corpseKnockBack = 0.32
	corpseGravity   = 0.08
	corpseDrag      = 0.02
	corpseTicks     = 25
	thunderSound    = "ambient.weather.thunder"
)

func (m *Manager) DeathAnimation(tx *world.Tx, victim *player.Player, damager mgl64.Vec3) {
	m.spawnCorpse(tx, victim, damager)
	m.spawnLightning(tx, victim.Position())
}

func (m *Manager) spawnCorpse(tx *world.Tx, victim *player.Player, damager mgl64.Vec3) {
	pos, rot := victim.Position(), victim.Rotation()
	conf := player.Config{
		Name:     victim.Name(),
		UUID:     uuid.New(),
		Skin:     victim.Skin(),
		Position: pos,
		Rotation: rot,
		GameMode: world.GameModeAdventure,
	}
	h := world.EntitySpawnOpts{Position: pos, Rotation: rot, ID: conf.UUID}.New(player.Type, conf)
	e := m.Spawn(tx, h, func(o *state.Online) bool { return !m.hidden(o) })
	corpse, ok := e.(*player.Player)
	if !ok {
		return
	}
	corpse.SetNameTag("")
	corpse.Handle(corpseHandler{})
	corpse.Hurt(math.MaxFloat32, entity.VoidDamageSource{})
	dx, dz := pos[0]-damager[0], pos[2]-damager[2]
	f := math.Sqrt(dx*dx + dz*dz)
	if f <= 0 {
		return
	}
	vel := mgl64.Vec3{dx / f * corpseKnockBack, corpseKnockBack, dz / f * corpseKnockBack}
	mc := &entity.MovementComputer{Gravity: corpseGravity, Drag: corpseDrag}
	m.sched.Times(1, corpseTicks, func() {
		h.Do(func(tx *world.Tx, e world.Entity) {
			mv := mc.TickMovement(e, pos, vel, rot, tx)
			mv.Send()
			pos, vel = mv.Position(), mv.Velocity()
		})
	})
}

func (m *Manager) spawnLightning(tx *world.Tx, pos mgl64.Vec3) {
	viewers := m.Viewers(tx, pos)
	m.Spawn(tx, entities.Lightning(world.EntitySpawnOpts{Position: pos}), m.CanSeeLightning)
	pk := &packet.PlaySound{
		SoundName: thunderSound,
		Position:  mgl32.Vec3{float32(pos[0]), float32(pos[1]), float32(pos[2])},
		Volume:    1,
		Pitch:     1,
	}
	for _, v := range viewers {
		if m.CanSeeLightning(v.O) {
			v.P.SendPacket(pk)
		}
	}
}

type corpseHandler struct {
	player.NopHandler
}

func (corpseHandler) HandleDeath(_ *player.Player, _ world.DamageSource, keepInv *bool) {
	*keepInv = true
}

func (corpseHandler) HandleFoodLoss(ctx *player.Context, _ int, _ *int) {
	ctx.Cancel()
}
