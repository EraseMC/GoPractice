package handler

import (
	"sync"

	"github.com/df-mc/dragonfly/server/player"
)

type Router struct {
	mu   sync.RWMutex
	quit []func(p *player.Player)
}

func NewRouter() *Router {
	return &Router{}
}

func (r *Router) OnQuit(f func(p *player.Player)) {
	r.mu.Lock()
	r.quit = append(r.quit, f)
	r.mu.Unlock()
}

func (r *Router) quitHooks() []func(p *player.Player) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.quit
}

func (r *Router) Handler() *Handler {
	return &Handler{r: r}
}

type Handler struct {
	player.NopHandler
	r *Router
}

func (h *Handler) HandleQuit(p *player.Player) {
	hooks := h.r.quitHooks()
	for i := len(hooks) - 1; i >= 0; i-- {
		hooks[i](p)
	}
}
