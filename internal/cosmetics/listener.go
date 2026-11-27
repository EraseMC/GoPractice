package cosmetics

import (
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/player/skin"
	"github.com/df-mc/dragonfly/server/world"
)

type listener struct {
	player.NopHandler
	m *Manager
}

func (m *Manager) Handler() player.Handler {
	return &listener{m: m}
}

func (l *listener) HandleSkinChange(ctx *player.Context, sk *skin.Skin) {
	if o, ok := l.m.Players.Get(ctx.Player().UUID()); ok && o.Handle == ctx.Player().H() {
		*sk = l.m.skin(o, *sk)
	}
}

func (l *listener) HandleQuit(p *player.Player) {
	l.m.Leave(p.Tx(), p)
	if o, ok := l.m.Players.Get(p.UUID()); ok && o.Handle == p.H() && o.Conn != nil {
		l.m.forgetViewer(o.Conn)
	}
}

func (l *listener) HandleAttackEntity(ctx *player.Context, e world.Entity, _, _ *float64, _ *bool) {
	l.m.mu.Lock()
	p := l.m.petsByID[e.H().UUID()]
	l.m.mu.Unlock()
	if p != nil {
		ctx.Cancel()
	}
}
