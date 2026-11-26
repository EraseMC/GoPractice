package items

import (
	"image/color"
	"math"
	"sync"
	"time"

	"github.com/EraseMC/GoPractice/internal/combat"
	"github.com/EraseMC/GoPractice/internal/entities"
	"github.com/EraseMC/GoPractice/internal/gamemode"
	"github.com/EraseMC/GoPractice/internal/lang"
	"github.com/EraseMC/GoPractice/internal/netmw"
	"github.com/EraseMC/GoPractice/internal/state"
	"github.com/EraseMC/GoPractice/internal/ui"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/block/cube/trace"
	"github.com/df-mc/dragonfly/server/entity"
	"github.com/df-mc/dragonfly/server/entity/effect"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/particle"
	"github.com/df-mc/dragonfly/server/world/sound"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/google/uuid"
)

const (
	PearlCooldown = 300

	pearlForce   = 2.425
	pearlGravity = 0.05
	pearlDrag    = 0.009

	splashForce   = 0.5
	splashGravity = 0.05
	splashDrag    = 0.01

	MaxHit  = 1.3055
	MaxMiss = 1.2015

	eyeHeight = 1.62
	soupHeal  = 6
)

var DefaultSplashColour = color.RGBA{R: 0x38, G: 0x5d, B: 0xc6, A: 0xff}

type Manager struct {
	bundle  *lang.Bundle
	players *state.Players
	combat  *combat.Manager

	PotionColour func(o *state.Online) (color.RGBA, bool)

	mu    sync.Mutex
	pearl map[uuid.UUID]int
}

func New(bundle *lang.Bundle, players *state.Players, c *combat.Manager) *Manager {
	return &Manager{
		bundle:  bundle,
		players: players,
		combat:  c,
		pearl:   map[uuid.UUID]int{},
	}
}

func (m *Manager) Handler() player.Handler {
	return &head{m: m}
}

func (m *Manager) Tail() player.Handler {
	return &tail{m: m}
}

func (m *Manager) text(o *state.Online, key string, args ...any) string {
	return m.bundle.Textf(o.Lang(), key, args...)
}

func (m *Manager) PearlCooldown(id uuid.UUID) (int, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.pearl[id]
	return t, ok
}

func (m *Manager) SetPearlCooldown(id uuid.UUID, ticks int) {
	m.mu.Lock()
	m.pearl[id] = ticks
	m.mu.Unlock()
}

func (m *Manager) ResetPearlCooldown(p *player.Player) {
	m.mu.Lock()
	_, ok := m.pearl[p.UUID()]
	delete(m.pearl, p.UUID())
	m.mu.Unlock()
	if ok {
		p.SetExperienceLevel(0)
		p.SetExperienceProgress(0)
	}
}

func (m *Manager) Forget(id uuid.UUID) {
	m.mu.Lock()
	delete(m.pearl, id)
	m.mu.Unlock()
}

func (m *Manager) Tick() {
	type step struct {
		id   uuid.UUID
		left int
	}
	m.mu.Lock()
	steps := make([]step, 0, len(m.pearl))
	for id, left := range m.pearl {
		if _, ok := m.players.Get(id); !ok {
			continue
		}
		steps = append(steps, step{id: id, left: left})
		if left <= 0 {
			delete(m.pearl, id)
		} else {
			m.pearl[id] = left - 1
		}
	}
	m.mu.Unlock()

	for _, s := range steps {
		o, ok := m.players.Get(s.id)
		if !ok {
			continue
		}
		left := s.left
		o.Do(func(_ *world.Tx, p *player.Player) {
			if left <= 0 {
				ui.ActionBar(p, m.text(o, "enderPearlCooldownEnded"))
				p.SetExperienceLevel(0)
				p.SetExperienceProgress(0)
				return
			}
			p.SetExperienceLevel(seconds(left))
			p.SetExperienceProgress(float64(left) / PearlCooldown)
		})
	}
}

func seconds(ticks int) int {
	return int(float64(ticks)/20 + 0.9)
}

func (m *Manager) Soup(p *player.Player) {
	if p.Health() >= p.MaxHealth() {
		return
	}
	p.Heal(soupHeal, soupHealing{})
	_, off := p.HeldItems()
	p.SetHeldItems(item.Stack{}, off)
}

type soupHealing struct{}

func (soupHealing) HealingSource() {}

func (m *Manager) throw(tx *world.Tx, p *player.Player, t world.EntityType, force float64, conf entity.ProjectileBehaviourConfig) {
	rot := p.Rotation()
	opts := world.EntitySpawnOpts{
		Position: p.Position().Add(mgl64.Vec3{0, eyeHeight, 0}),
		Velocity: Direction(rot).Mul(force),
		Rotation: rot,
	}
	owner, _ := m.combat.Online(p)
	m.combat.Spawn(tx, opts.New(t, conf), func(o *state.Online) bool { return m.combat.CanSee(o, owner) })
	tx.PlaySound(p.Position(), sound.ItemThrow{})
	held, off := p.HeldItems()
	p.SetHeldItems(held.Grow(-1), off)
}

func Direction(r cube.Rotation) mgl64.Vec3 {
	yaw, pitch := mgl64.DegToRad(r[0]), mgl64.DegToRad(r[1])
	xz := math.Cos(pitch)
	return mgl64.Vec3{-xz * math.Sin(yaw), -math.Sin(pitch), xz * math.Cos(yaw)}.Normalize()
}

func (m *Manager) ThrowPearl(tx *world.Tx, p *player.Player) {
	m.throw(tx, p, entities.PearlType, pearlForce, entity.ProjectileBehaviourConfig{
		Owner:    p.H(),
		Gravity:  pearlGravity,
		Drag:     pearlDrag,
		Collides: m.collides,
		Hit:      m.pearlHit,
	})
	p.SetCooldown(item.EnderPearl{}, time.Second)
}

func (m *Manager) ThrowSplash(tx *world.Tx, p *player.Player, s item.SplashPotion) {
	m.throw(tx, p, entities.SplashType, splashForce, entity.ProjectileBehaviourConfig{
		Owner:    p.H(),
		Gravity:  splashGravity,
		Drag:     splashDrag,
		Damage:   -1,
		Potion:   s.Type,
		Collides: m.collides,
		Hit:      m.splashHit,
	})
}

func owner(e *entity.Ent, tx *world.Tx) (*player.Player, bool) {
	b, ok := e.Behaviour().(*entity.ProjectileBehaviour)
	if !ok || b.Owner() == nil {
		return nil, false
	}
	ent, ok := b.Owner().Entity(tx)
	if !ok {
		return nil, false
	}
	p, ok := ent.(*player.Player)
	return p, ok
}

func (m *Manager) collides(e *entity.Ent, tx *world.Tx, other world.Entity) bool {
	target, ok := other.(*player.Player)
	if !ok {
		return true
	}
	if target.Dead() {
		return false
	}
	own, ok := owner(e, tx)
	if !ok {
		return true
	}
	if target.H() == own.H() {
		return true
	}
	oo, ok := m.combat.Online(own)
	if !ok {
		return true
	}
	enemy := m.combat.EnemyName(oo)
	if target.Name() == enemy {
		return true
	}
	return enemy == "" || !m.combat.Fighting(oo)
}

func (m *Manager) pearlHit(e *entity.Ent, tx *world.Tx, res trace.Result) {
	own, ok := owner(e, tx)
	if !ok {
		return
	}
	oo, _ := m.combat.Online(own)
	from := own.Position()
	m.combat.Particle(tx, from, particle.EndermanTeleport{}, oo)
	m.combat.Sound(tx, from, sound.Teleport{}, oo)
	own.Teleport(res.Position())
	m.combat.Sound(tx, res.Position(), sound.Teleport{}, oo)
}

func (m *Manager) splashHit(e *entity.Ent, tx *world.Tx, res trace.Result) {
	b, ok := e.Behaviour().(*entity.ProjectileBehaviour)
	if !ok {
		return
	}
	effects := b.Potion().Effects()
	own, hasOwner := owner(e, tx)
	var oo *state.Online
	if hasOwner {
		oo, _ = m.combat.Online(own)
	}

	colour := DefaultSplashColour
	if len(effects) > 0 {
		colour = Mix(effects)
		if oo != nil && m.PotionColour != nil {
			if c, ok := m.PotionColour(oo); ok {
				colour = c
			}
		}
	}
	pos := e.Position()
	m.combat.Particle(tx, pos, particle.Splash{Colour: colour}, oo)
	m.combat.Sound(tx, pos, sound.GlassBreak{}, oo)
	if len(effects) == 0 {
		return
	}

	var hit *world.EntityHandle
	if r, ok := res.(trace.EntityResult); ok {
		if pl, ok := r.Entity().(*player.Player); ok {
			hit = pl.H()
		}
	}
	area := cube.Box(pos[0]-0.05, pos[1], pos[2]-0.05, pos[0]+0.05, pos[1]+0.1, pos[2]+0.05).GrowVec3(mgl64.Vec3{1.85, 2.65, 1.85})
	for ent := range tx.EntitiesWithin(area.Grow(2)) {
		pl, ok := ent.(*player.Player)
		if !ok || pl.Dead() {
			continue
		}
		if !player.Type.BBox(pl).Translate(pl.Position()).IntersectsWith(area) {
			continue
		}
		po, ok := m.combat.Online(pl)
		if !ok {
			continue
		}
		if enemy := m.combat.EnemyName(po); enemy != "" {
			if _, online := m.players.ByName(enemy); online {
				if !hasOwner || (pl.H() != own.H() && enemy != own.Name()) {
					continue
				}
			}
		}
		mul := MaxMiss
		if pl.H() == hit {
			mul = MaxHit
		}
		Apply(pl, effects, mul)
	}
}

func Apply(l interface{ AddEffect(effect.Effect) }, effects []effect.Effect, mul float64) {
	for _, eff := range effects {
		lasting, ok := eff.Type().(effect.LastingType)
		if !ok {
			l.AddEffect(effect.NewInstantWithPotency(eff.Type(), eff.Level(), mul))
			continue
		}
		ticks := math.Round(float64(eff.Duration()/entities.Tick) * 0.75 * mul)
		if ticks < 20 {
			continue
		}
		l.AddEffect(effect.New(lasting, eff.Level(), time.Duration(ticks)*entities.Tick))
	}
}

func Mix(effects []effect.Effect) color.RGBA {
	var r, g, b, a, n int
	for _, eff := range effects {
		c := eff.Type().RGBA()
		for i := 0; i < eff.Level(); i++ {
			r += int(c.R)
			g += int(c.G)
			b += int(c.B)
			a += int(c.A)
			n++
		}
	}
	if n == 0 {
		return DefaultSplashColour
	}
	return color.RGBA{R: uint8(r / n), G: uint8(g / n), B: uint8(b / n), A: uint8(a / n)}
}

type head struct {
	player.NopHandler
	m *Manager
}

func (h *head) HandleItemUse(ctx *player.Context) {
	p := ctx.Player()
	held, _ := p.HeldItems()
	if _, ok := held.Item().(item.EnderPearl); !ok {
		return
	}
	o, ok := h.m.combat.Online(p)
	if !ok {
		return
	}
	if left, ok := h.m.PearlCooldown(o.UUID); ok {
		ctx.Cancel()
		ui.ActionBar(p, h.m.text(o, "enderPearlCooldownOngoing", seconds(left)))
		return
	}
	if !gamemode.IsAdventure(p.GameMode()) {
		return
	}
	h.m.SetPearlCooldown(o.UUID, PearlCooldown)
	ui.ActionBar(p, h.m.text(o, "enderPearlCooldownStarted"))
}

func (h *head) HandleQuit(p *player.Player) {
	h.m.Forget(p.UUID())
}

type tail struct {
	player.NopHandler
	m *Manager
}

func (t *tail) HandleItemUse(ctx *player.Context) {
	p := ctx.Player()
	held, _ := p.HeldItems()
	switch it := held.Item().(type) {
	case item.EnderPearl:
		ctx.Cancel()
		t.m.ThrowPearl(p.Tx(), p)
	case item.SplashPotion:
		ctx.Cancel()
		t.m.ThrowSplash(p.Tx(), p, it)
	case item.MushroomStew:
		ctx.Cancel()
		t.m.Soup(p)
	}
}

func (t *tail) HandleItemUseOnBlock(ctx *player.Context, _ cube.Pos, _ cube.Face, _ mgl64.Vec3) {
	p := ctx.Player()
	held, _ := p.HeldItems()
	if _, ok := held.Item().(item.MushroomStew); !ok {
		return
	}
	o, ok := t.m.combat.Online(p)
	if ok && o.InputMode() == netmw.InputModeTouch {
		t.m.Soup(p)
	}
}

func (t *tail) HandleItemConsume(ctx *player.Context, s item.Stack) {
	pot, ok := s.Item().(item.Potion)
	if !ok {
		return
	}
	ctx.Cancel()
	p := ctx.Player()
	for _, eff := range pot.Type.Effects() {
		p.AddEffect(eff)
	}
	p.ReleaseItem()
	held, off := p.HeldItems()
	p.SetHeldItems(held.Grow(-1), off)
}
