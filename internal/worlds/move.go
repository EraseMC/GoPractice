package worlds

import (
	"sync"

	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

var (
	hooksMu sync.RWMutex
	leave   []func(tx *world.Tx, p *player.Player)
	enter   []func(tx *world.Tx, p *player.Player)
)

func OnLeave(f func(tx *world.Tx, p *player.Player)) {
	hooksMu.Lock()
	leave = append(leave, f)
	hooksMu.Unlock()
}

func OnEnter(f func(tx *world.Tx, p *player.Player)) {
	hooksMu.Lock()
	enter = append(enter, f)
	hooksMu.Unlock()
}

func hooks(list *[]func(tx *world.Tx, p *player.Player)) []func(tx *world.Tx, p *player.Player) {
	hooksMu.RLock()
	defer hooksMu.RUnlock()
	return *list
}

func Leaving(tx *world.Tx, p *player.Player) {
	for _, f := range hooks(&leave) {
		f(tx, p)
	}
}

func Move(tx *world.Tx, p *player.Player, w *world.World, pos mgl64.Vec3, yaw, pitch float64) {
	MoveThen(tx, p, w, pos, yaw, pitch, nil)
}

func MoveThen(tx *world.Tx, p *player.Player, w *world.World, pos mgl64.Vec3, yaw, pitch float64, then func(tx *world.Tx, p *player.Player)) {
	if w == nil || tx.World() == w {
		Rotate(p, yaw, pitch)
		p.Teleport(pos)
		if then != nil {
			then(tx, p)
		}
		return
	}
	Leaving(tx, p)
	h := tx.RemoveEntity(p)
	w.Do(func(tx *world.Tx) {
		np, ok := tx.AddEntityAt(h, pos).(*player.Player)
		if !ok {
			return
		}
		Rotate(np, yaw, pitch)
		np.Teleport(pos)
		for _, f := range hooks(&enter) {
			f(tx, np)
		}
		if then != nil {
			then(tx, np)
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
