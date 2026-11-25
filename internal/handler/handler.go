package handler

import (
	"sync"
	"time"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

type Router struct {
	mu    sync.RWMutex
	parts []player.Handler
}

func NewRouter() *Router {
	return &Router{}
}

func (r *Router) Add(h player.Handler) {
	r.mu.Lock()
	r.parts = append(r.parts, h)
	r.mu.Unlock()
}

func (r *Router) OnQuit(f func(p *player.Player)) {
	r.Add(quitHook{f: f})
}

func (r *Router) OnChat(f func(ctx *player.Context, message *string)) {
	r.Add(chatHook{f: f})
}

func (r *Router) list() []player.Handler {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.parts
}

func (r *Router) Handler() *Handler {
	return &Handler{r: r}
}

type quitHook struct {
	player.NopHandler
	f func(p *player.Player)
}

func (q quitHook) HandleQuit(p *player.Player) { q.f(p) }

type chatHook struct {
	player.NopHandler
	f func(ctx *player.Context, message *string)
}

func (c chatHook) HandleChat(ctx *player.Context, message *string) { c.f(ctx, message) }

type Handler struct {
	player.NopHandler
	r *Router
}

func (h *Handler) each(ctx *player.Context, f func(part player.Handler)) {
	for _, part := range h.r.list() {
		f(part)
		if ctx.Cancelled() {
			return
		}
	}
}

func (h *Handler) HandleChat(ctx *player.Context, message *string) {
	h.each(ctx, func(part player.Handler) { part.HandleChat(ctx, message) })
}

func (h *Handler) HandleQuit(p *player.Player) {
	parts := h.r.list()
	for i := len(parts) - 1; i >= 0; i-- {
		parts[i].HandleQuit(p)
	}
}

func (h *Handler) HandleMove(ctx *player.Context, newPos mgl64.Vec3, newRot cube.Rotation) {
	h.each(ctx, func(part player.Handler) { part.HandleMove(ctx, newPos, newRot) })
}

func (h *Handler) HandleTeleport(ctx *player.Context, pos mgl64.Vec3) {
	h.each(ctx, func(part player.Handler) { part.HandleTeleport(ctx, pos) })
}

func (h *Handler) HandleFoodLoss(ctx *player.Context, from int, to *int) {
	h.each(ctx, func(part player.Handler) { part.HandleFoodLoss(ctx, from, to) })
}

func (h *Handler) HandleHurt(ctx *player.Context, damage *float64, immune bool, attackImmunity *time.Duration, src world.DamageSource) {
	h.each(ctx, func(part player.Handler) { part.HandleHurt(ctx, damage, immune, attackImmunity, src) })
}

func (h *Handler) HandleDeath(p *player.Player, src world.DamageSource, keepInv *bool) {
	for _, part := range h.r.list() {
		part.HandleDeath(p, src, keepInv)
	}
}

func (h *Handler) HandleRespawn(p *player.Player, pos *mgl64.Vec3, w **world.World) {
	for _, part := range h.r.list() {
		part.HandleRespawn(p, pos, w)
	}
}

func (h *Handler) HandleStartBreak(ctx *player.Context, pos cube.Pos) {
	h.each(ctx, func(part player.Handler) { part.HandleStartBreak(ctx, pos) })
}

func (h *Handler) HandleBlockBreak(ctx *player.Context, pos cube.Pos, drops *[]item.Stack, xp *int) {
	h.each(ctx, func(part player.Handler) { part.HandleBlockBreak(ctx, pos, drops, xp) })
}

func (h *Handler) HandleBlockPlace(ctx *player.Context, pos cube.Pos, b world.Block) {
	h.each(ctx, func(part player.Handler) { part.HandleBlockPlace(ctx, pos, b) })
}

func (h *Handler) HandleBlockPick(ctx *player.Context, pos cube.Pos, b world.Block) {
	h.each(ctx, func(part player.Handler) { part.HandleBlockPick(ctx, pos, b) })
}

func (h *Handler) HandleItemUse(ctx *player.Context) {
	h.each(ctx, func(part player.Handler) { part.HandleItemUse(ctx) })
}

func (h *Handler) HandleItemUseOnBlock(ctx *player.Context, pos cube.Pos, face cube.Face, clickPos mgl64.Vec3) {
	h.each(ctx, func(part player.Handler) { part.HandleItemUseOnBlock(ctx, pos, face, clickPos) })
}

func (h *Handler) HandleItemUseOnEntity(ctx *player.Context, e world.Entity) {
	h.each(ctx, func(part player.Handler) { part.HandleItemUseOnEntity(ctx, e) })
}

func (h *Handler) HandleAttackEntity(ctx *player.Context, e world.Entity, force, height *float64, critical *bool) {
	h.each(ctx, func(part player.Handler) { part.HandleAttackEntity(ctx, e, force, height, critical) })
}

func (h *Handler) HandleItemDrop(ctx *player.Context, s item.Stack) {
	h.each(ctx, func(part player.Handler) { part.HandleItemDrop(ctx, s) })
}

func (h *Handler) HandleItemPickup(ctx *player.Context, i *item.Stack) {
	h.each(ctx, func(part player.Handler) { part.HandleItemPickup(ctx, i) })
}
