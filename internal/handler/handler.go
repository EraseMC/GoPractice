package handler

import (
	"sync"

	"github.com/df-mc/dragonfly/server/player"
)

type Router struct {
	mu   sync.RWMutex
	quit []func(p *player.Player)
	chat []func(ctx *player.Context, message *string)
}

func NewRouter() *Router {
	return &Router{}
}

func (r *Router) OnQuit(f func(p *player.Player)) {
	r.mu.Lock()
	r.quit = append(r.quit, f)
	r.mu.Unlock()
}

func (r *Router) OnChat(f func(ctx *player.Context, message *string)) {
	r.mu.Lock()
	r.chat = append(r.chat, f)
	r.mu.Unlock()
}

func (r *Router) quitHooks() []func(p *player.Player) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.quit
}

func (r *Router) chatHooks() []func(ctx *player.Context, message *string) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.chat
}

func (r *Router) Handler() *Handler {
	return &Handler{r: r}
}

type Handler struct {
	player.NopHandler
	r *Router
}

func (h *Handler) HandleChat(ctx *player.Context, message *string) {
	for _, f := range h.r.chatHooks() {
		f(ctx, message)
		if ctx.Cancelled() {
			return
		}
	}
}

func (h *Handler) HandleQuit(p *player.Player) {
	hooks := h.r.quitHooks()
	for i := len(hooks) - 1; i >= 0; i-- {
		hooks[i](p)
	}
}
