package overall

import (
	"time"

	"github.com/EraseMC/GoPractice/internal/gamemode"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/entity"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

type Handler struct {
	player.NopHandler

	AllowHunger func(p *player.Player) bool
	AllowDrop   func(p *player.Player, s item.Stack) bool
}

func (h *Handler) HandleHurt(ctx *player.Context, _ *float64, _ bool, _ *time.Duration, src world.DamageSource) {
	switch s := src.(type) {
	case entity.SuffocationDamageSource, player.StarvationDamageSource:
		ctx.Cancel()
	case entity.AttackDamageSource:
		if a, ok := s.Attacker.(*player.Player); ok && !gamemode.IsAdventure(a.GameMode()) {
			ctx.Cancel()
		}
	}
}

func (h *Handler) HandleFoodLoss(ctx *player.Context, _ int, _ *int) {
	if h.AllowHunger == nil || !h.AllowHunger(ctx.Player()) {
		ctx.Cancel()
	}
}

func (h *Handler) HandleItemDrop(ctx *player.Context, s item.Stack) {
	if h.AllowDrop == nil || !h.AllowDrop(ctx.Player(), s) {
		ctx.Cancel()
	}
}

func (h *Handler) HandleBlockBreak(ctx *player.Context, _ cube.Pos, _ *[]item.Stack, _ *int) {
	ctx.Cancel()
}

func (h *Handler) HandleBlockPlace(ctx *player.Context, _ cube.Pos, _ world.Block) {
	ctx.Cancel()
}

func (h *Handler) HandleBlockPick(ctx *player.Context, _ cube.Pos, _ world.Block) {
	ctx.Cancel()
}

func (h *Handler) HandleItemUseOnBlock(ctx *player.Context, _ cube.Pos, _ cube.Face, _ mgl64.Vec3) {
	p := ctx.Player()
	held, _ := p.HeldItems()
	switch held.Item().(type) {
	case item.SplashPotion, item.MushroomStew:
		if gamemode.IsAdventure(p.GameMode()) {
			return
		}
	}
	ctx.Cancel()
}
