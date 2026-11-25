package hub

import (
	"time"

	"github.com/EraseMC/GoPractice/internal/state"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/entity"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/item/inventory"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/google/uuid"
)

func (m *Manager) Handler() player.Handler {
	return &listener{m: m}
}

type listener struct {
	player.NopHandler
	m *Manager
}

func (l *listener) online(p *player.Player) (*state.Online, bool) {
	return l.m.players.Get(p.UUID())
}

func (l *listener) HandleItemUse(ctx *player.Context) {
	p := ctx.Player()
	o, ok := l.online(p)
	if !ok || !l.m.InHub(o) {
		return
	}
	ctx.Cancel()
	id, ok := heldHubItem(p)
	if !ok {
		return
	}
	tx := p.Tx()
	switch id {
	case itemDuels:
		if l.m.QueueForm != nil {
			l.m.QueueForm(o, p, tx)
		}
	case itemFFA:
		if l.m.FFAForm != nil {
			l.m.FFAForm(o, p, tx)
		}
	case itemSettings:
		l.m.SendPreferencesForm(p)
	case itemStats:
		l.m.stats.SendStatisticsForm(p)
	}
}

func (l *listener) HandleMove(ctx *player.Context, newPos mgl64.Vec3, _ cube.Rotation) {
	p := ctx.Player()
	o, ok := l.online(p)
	if !ok || !l.m.InHub(o) || newPos.Sub(l.m.spawn).Len() <= MaxDistance {
		return
	}
	l.m.Reset(o, p, p.Tx())
	ctx.Cancel()
}

func (l *listener) HandleAttackEntity(ctx *player.Context, e world.Entity, _, _ *float64, _ *bool) {
	p := ctx.Player()
	o, ok := l.online(p)
	if !ok {
		return
	}
	if l.m.npc.is(e) {
		if l.m.InHub(o) {
			l.m.SendMenuForm(p)
		}
		ctx.Cancel()
		return
	}
	if !l.m.InHub(o) {
		return
	}
	ctx.Cancel()
	target, ok := e.(*player.Player)
	if !ok {
		return
	}
	to, ok := l.m.players.Get(target.UUID())
	if !ok || !l.m.InHub(to) {
		return
	}
	if id, ok := heldHubItem(p); !ok || id != itemDuels {
		return
	}
	if l.m.inQueue(o) || l.m.inQueue(to) || l.m.InviteForm == nil {
		return
	}
	l.m.InviteForm(o, p, to, p.Tx())
}

func (l *listener) HandleItemUseOnEntity(ctx *player.Context, e world.Entity) {
	p := ctx.Player()
	o, ok := l.online(p)
	if ok && l.m.InHub(o) && l.m.npc.is(e) {
		l.m.SendMenuForm(p)
	}
}

func (l *listener) HandleHurt(ctx *player.Context, _ *float64, _ bool, _ *time.Duration, src world.DamageSource) {
	p := ctx.Player()
	o, ok := l.online(p)
	if !ok || !l.m.InHub(o) {
		return
	}
	if _, void := src.(entity.VoidDamageSource); void {
		l.m.Reset(o, p, p.Tx())
	}
	ctx.Cancel()
}

func (l *listener) HandleDeath(p *player.Player, _ world.DamageSource, keepInv *bool) {
	if o, ok := l.online(p); ok && l.m.InHub(o) {
		*keepInv = true
	}
}

func (l *listener) HandleRespawn(p *player.Player, pos *mgl64.Vec3, w **world.World) {
	o, ok := l.online(p)
	if !ok || !l.m.InHub(o) {
		return
	}
	*pos, *w = l.m.spawn, l.m.world
	p.H().Do(func(tx *world.Tx, e world.Entity) {
		if p, ok := e.(*player.Player); ok && l.m.InHub(o) {
			l.m.Reset(o, p, tx)
		}
	})
}

func (l *listener) HandleItemDrop(ctx *player.Context, _ item.Stack) {
	if o, ok := l.online(ctx.Player()); ok && l.m.InHub(o) {
		ctx.Cancel()
	}
}

func (m *Manager) InventoryHandler(id uuid.UUID) inventory.Handler {
	return &inventoryGuard{m: m, id: id}
}

type inventoryGuard struct {
	inventory.NopHandler
	m  *Manager
	id uuid.UUID
}

func (g *inventoryGuard) locked() bool {
	o, ok := g.m.players.Get(g.id)
	return ok && g.m.InHub(o)
}

func (g *inventoryGuard) HandleTake(ctx *inventory.Context, _ int, _ item.Stack) {
	if g.locked() {
		ctx.Cancel()
	}
}

func (g *inventoryGuard) HandlePlace(ctx *inventory.Context, _ int, _ item.Stack) {
	if g.locked() {
		ctx.Cancel()
	}
}

func (g *inventoryGuard) HandleDrop(ctx *inventory.Context, _ int, _ item.Stack) {
	if g.locked() {
		ctx.Cancel()
	}
}
