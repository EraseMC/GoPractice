package duel

import (
	"slices"
	"time"

	"github.com/EraseMC/GoPractice/internal/state"
	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/entity"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

type listener struct {
	player.NopHandler
	m *Manager
}

func (m *Manager) Handler() player.Handler {
	return &listener{m: m}
}

func (m *Manager) AllowsHunger(p *player.Player) bool {
	match, _ := m.current(p)
	return match != nil && match.mode.id == "classic_nodebuff" && match.phase == running
}

func (m *Manager) AllowsDrop(p *player.Player, s item.Stack) bool {
	match, _ := m.current(p)
	_, sword := s.Item().(item.Sword)
	return match != nil && match.mode.id == "classic_nodebuff" && !sword
}

func (l *listener) HandleItemUse(ctx *player.Context) {
	p := ctx.Player()
	held, _ := p.HeldItems()
	if _, leave := held.Value("duel-leave-queue"); !leave {
		return
	}
	if o, ok := l.m.Players.Get(p.UUID()); ok && l.m.InQueue(o) {
		ctx.Cancel()
		l.m.leaveQueue(o, p)
	}
}

func (l *listener) HandleItemUseOnBlock(ctx *player.Context, _ cube.Pos, _ cube.Face, _ mgl64.Vec3) {
	l.HandleItemUse(ctx)
}

func (l *listener) HandleMove(ctx *player.Context, pos mgl64.Vec3, _ cube.Rotation) {
	p := ctx.Player()
	match, i := l.m.current(p)
	if match == nil || match.mode.id != "sumo" || match.phase != running {
		return
	}
	if liq, ok := p.Tx().Liquid(cube.PosFromVec3(pos)); ok {
		if _, water := liq.(block.Water); water && l.m.finish(p.Tx(), match, i, p, true) {
			ctx.Cancel()
		}
	}
}

func (l *listener) HandleHurt(ctx *player.Context, damage *float64, _ bool, _ *time.Duration, source world.DamageSource) {
	p := ctx.Player()
	match, i := l.m.current(p)
	if match == nil {
		return
	}
	switch source.(type) {
	case entity.VoidDamageSource:
		l.m.finish(p.Tx(), match, i, p, true)
		ctx.Cancel()
		return
	case entity.FallDamageSource, entity.SuffocationDamageSource, player.StarvationDamageSource:
		ctx.Cancel()
		return
	}
	if match.phase != running {
		ctx.Cancel()
		return
	}
	if s, ok := source.(entity.AttackDamageSource); ok {
		if s.Attacker.H() != match.players[1-i].Handle {
			ctx.Cancel()
			return
		}
		if match.mode.id == "boxing" {
			match.hits[1-i]++
			if match.hits[1-i] >= 100 {
				l.m.finish(p.Tx(), match, i, p, false)
				ctx.Cancel()
				return
			}
			l.m.matchBoard(match, 0)
			l.m.matchBoard(match, 1)
		}
	}
	if match.mode.noDamage() {
		*damage = 0
	} else if p.Health()+p.Absorption()-*damage <= mgl64.Epsilon {
		l.m.finish(p.Tx(), match, i, p, false)
		ctx.Cancel()
	}
}

func (l *listener) HandleQuit(p *player.Player) {
	m := l.m
	o, ok := m.Players.Get(p.UUID())
	if !ok {
		return
	}
	m.mu.Lock()
	m.queue = slices.DeleteFunc(m.queue, func(e entry) bool { return e.o == o })
	m.invites = slices.DeleteFunc(m.invites, func(i invite) bool { return i.from == o || i.to == o })
	m.mu.Unlock()
	match, i := m.current(p)
	if match != nil {
		match.gone[i] = true
		match.supplies[i] = remaining(match.mode, p)
		m.finish(p.Tx(), match, i, p, false)
	}
}

func (m *Manager) Refresh(o *state.Online, p *player.Player) {
	if match, i := m.current(p); match != nil {
		m.matchBoard(match, i)
		return
	}
	m.mu.Lock()
	queue := slices.Clone(m.queue)
	m.mu.Unlock()
	for _, entry := range queue {
		if entry.o == o {
			m.queueItem(o, p)
			m.queueBoard(entry, len(queue), time.Now())
			return
		}
	}
}
