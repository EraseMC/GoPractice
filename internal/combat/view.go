package combat

import (
	"net"

	"github.com/EraseMC/GoPractice/internal/state"
	"github.com/EraseMC/GoPractice/internal/toggles"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/entity"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

type Viewer struct {
	O *state.Online
	P *player.Player
	V world.Viewer
}

func (m *Manager) Viewers(tx *world.Tx, pos mgl64.Vec3) []Viewer {
	byAddr := map[string]world.Viewer{}
	for _, v := range tx.Viewers(pos) {
		a, ok := v.(interface{ Addr() net.Addr })
		if !ok || a.Addr() == nil {
			continue
		}
		byAddr[a.Addr().String()] = v
	}
	var out []Viewer
	for e := range tx.Players() {
		p, ok := e.(*player.Player)
		if !ok {
			continue
		}
		o, ok := m.Online(p)
		if !ok || o.Conn == nil || o.Conn.RemoteAddr() == nil {
			continue
		}
		v, ok := byAddr[o.Conn.RemoteAddr().String()]
		if !ok {
			continue
		}
		out = append(out, Viewer{O: o, P: p, V: v})
	}
	return out
}

func (m *Manager) hidden(o *state.Online) bool {
	return m.toggles.Get(o, toggles.HidePlayers) && m.Fighting(o)
}

func (m *Manager) CanSee(viewer, owner *state.Online) bool {
	if owner != nil {
		if viewer.UUID == owner.UUID {
			return true
		}
		if m.EnemyName(owner) == viewer.Name {
			return true
		}
	}
	return !m.hidden(viewer)
}

func (m *Manager) CanSeeLightning(viewer *state.Online) bool {
	return m.toggles.Get(viewer, toggles.Lightning) && !m.hidden(viewer)
}

func (m *Manager) Particle(tx *world.Tx, pos mgl64.Vec3, p world.Particle, owner *state.Online) {
	for _, v := range m.Viewers(tx, pos) {
		if m.CanSee(v.O, owner) {
			v.V.ViewParticle(pos, p)
		}
	}
}

func (m *Manager) Sound(tx *world.Tx, pos mgl64.Vec3, s world.Sound, owner *state.Online) {
	for _, v := range m.Viewers(tx, pos) {
		if m.CanSee(v.O, owner) {
			v.V.ViewSound(pos, s)
		}
	}
}

func (m *Manager) Spawn(tx *world.Tx, h *world.EntityHandle, show func(o *state.Online) bool) world.Entity {
	g := ghost{h: h}
	for e := range tx.Players() {
		p, ok := e.(*player.Player)
		if !ok {
			continue
		}
		o, ok := m.Online(p)
		if ok && !show(o) {
			p.HideEntity(g)
		}
	}
	return tx.AddEntity(h)
}

type ghost struct {
	h *world.EntityHandle
}

func (g ghost) H() *world.EntityHandle  { return g.h }
func (g ghost) Position() mgl64.Vec3    { return mgl64.Vec3{} }
func (g ghost) Rotation() cube.Rotation { return cube.Rotation{} }
func (g ghost) Close() error            { return nil }

func (m *Manager) Swing(tx *world.Tx, p *player.Player) {
	for _, v := range m.Viewers(tx, p.Position()) {
		if v.P.H() != p.H() {
			v.V.ViewEntityAction(p, entity.SwingArmAction{})
		}
	}
}
