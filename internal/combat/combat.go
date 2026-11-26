package combat

import (
	"sync"
	"time"

	"github.com/EraseMC/GoPractice/internal/scheduler"
	"github.com/EraseMC/GoPractice/internal/state"
	"github.com/EraseMC/GoPractice/internal/toggles"
	"github.com/df-mc/dragonfly/server/entity"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/google/uuid"
)

type Manager struct {
	players *state.Players
	toggles *toggles.Manager
	sched   *scheduler.Scheduler

	Profile  func(o *state.Online, p *player.Player) Profile
	Enemy    func(o *state.Online) string
	InCombat func(o *state.Online) bool

	mu     sync.Mutex
	immune map[uuid.UUID]uint64
	crits  map[uuid.UUID]bool
}

func New(players *state.Players, t *toggles.Manager, sched *scheduler.Scheduler) *Manager {
	return &Manager{
		players: players,
		toggles: t,
		sched:   sched,
		immune:  map[uuid.UUID]uint64{},
		crits:   map[uuid.UUID]bool{},
	}
}

func (m *Manager) Online(p *player.Player) (*state.Online, bool) {
	return m.players.Get(p.UUID())
}

func (m *Manager) ProfileOf(p *player.Player) Profile {
	if m.Profile == nil {
		return Default
	}
	o, ok := m.Online(p)
	if !ok {
		return Default
	}
	return m.Profile(o, p)
}

func (m *Manager) EnemyName(o *state.Online) string {
	if m.Enemy == nil || o == nil {
		return ""
	}
	return m.Enemy(o)
}

func (m *Manager) Fighting(o *state.Online) bool {
	return m.InCombat != nil && o != nil && m.InCombat(o)
}

func (m *Manager) Immune(id uuid.UUID) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	until, ok := m.immune[id]
	return ok && m.sched.Tick() < until
}

func (m *Manager) ResetImmunity(id uuid.UUID) {
	m.mu.Lock()
	delete(m.immune, id)
	m.mu.Unlock()
}

func (m *Manager) setImmune(id uuid.UUID, ticks int) {
	m.mu.Lock()
	m.immune[id] = m.sched.Tick() + uint64(max(ticks, 0))
	m.mu.Unlock()
}

func (m *Manager) setCrit(id uuid.UUID, crit bool) {
	m.mu.Lock()
	m.crits[id] = crit
	m.mu.Unlock()
}

func (m *Manager) takeCrit(id uuid.UUID) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	crit := m.crits[id]
	delete(m.crits, id)
	return crit
}

func (m *Manager) Forget(id uuid.UUID) {
	m.mu.Lock()
	delete(m.immune, id)
	delete(m.crits, id)
	m.mu.Unlock()
}

func (m *Manager) Handler() player.Handler {
	return &head{m: m}
}

func (m *Manager) Tail() player.Handler {
	return &tail{m: m}
}

type head struct {
	player.NopHandler
	m *Manager
}

func (h *head) HandleAttackEntity(ctx *player.Context, e world.Entity, _, _ *float64, critical *bool) {
	victim, ok := e.(*player.Player)
	if !ok {
		return
	}
	if _, online := h.m.Online(victim); !online {
		return
	}
	attacker := ctx.Player()
	if h.m.Immune(victim.UUID()) {
		attacker.SwingArm()
		ctx.Cancel()
		return
	}
	crit := Critical(attacker) && !h.m.ProfileOf(victim).NoCrit
	*critical = crit
	h.m.setCrit(attacker.UUID(), crit)
}

func (h *head) HandleHurt(ctx *player.Context, damage *float64, _ bool, immunity *time.Duration, src world.DamageSource) {
	p := ctx.Player()
	*immunity = 0
	if h.m.Immune(p.UUID()) {
		ctx.Cancel()
		return
	}
	if s, ok := src.(entity.AttackDamageSource); ok {
		if a, ok := s.Attacker.(*player.Player); ok {
			*damage = Damage(AttackOf(a, h.m.takeCrit(a.UUID())), DefenceOf(p))
		}
	}
}

func (h *head) HandleKnockBack(ctx *player.Context, src mgl64.Vec3, _, _ float64) {
	ctx.Cancel()
	p := ctx.Player()
	pos := p.Position()
	x, y, z, ok := h.m.ProfileOf(p).Velocity(pos[0]-src[0], pos[2]-src[2], pos[1])
	if !ok {
		return
	}
	p.SetVelocity(mgl64.Vec3{x, y, z})
}

func (h *head) HandleQuit(p *player.Player) {
	h.m.Forget(p.UUID())
}

type tail struct {
	player.NopHandler
	m *Manager
}

func (t *tail) HandleHurt(ctx *player.Context, _ *float64, _ bool, _ *time.Duration, _ world.DamageSource) {
	p := ctx.Player()
	t.m.setImmune(p.UUID(), t.m.ProfileOf(p).Immunity)
}
