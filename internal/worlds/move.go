package worlds

import (
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

func Move(tx *world.Tx, p *player.Player, w *world.World, pos mgl64.Vec3, yaw, pitch float64) {
	if w == nil || tx.World() == w {
		p.Teleport(pos)
		Rotate(p, yaw, pitch)
		return
	}
	h := tx.RemoveEntity(p)
	w.Do(func(tx *world.Tx) {
		if np, ok := tx.AddEntityAt(h, pos).(*player.Player); ok {
			np.Teleport(pos)
			Rotate(np, yaw, pitch)
		}
	})
}

func Rotate(p *player.Player, yaw, pitch float64) {
	r := p.Rotation()
	if r.Yaw() == yaw && r.Pitch() == pitch {
		return
	}
	p.Move(mgl64.Vec3{}, yaw-r.Yaw(), pitch-r.Pitch())
}
