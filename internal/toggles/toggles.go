package toggles

import (
	"strconv"
	"sync"
	"time"

	"github.com/EraseMC/GoPractice/internal/lang"
	"github.com/EraseMC/GoPractice/internal/netmw"
	"github.com/EraseMC/GoPractice/internal/state"
	"github.com/EraseMC/GoPractice/internal/store"
	"github.com/EraseMC/GoPractice/internal/ui"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type Toggle int

const (
	Scoreboard Toggle = iota
	CPS
	AutoSprint
	HidePlayers
	ArenaRespawn
	Lightning
)

var defaults = store.Settings{Scoreboard: true, Chat: true, HidePlayers: true, Lightning: true}

func get(s store.Settings, t Toggle) bool {
	switch t {
	case Scoreboard:
		return s.Scoreboard
	case CPS:
		return s.CPS
	case AutoSprint:
		return s.AutoSprint
	case HidePlayers:
		return s.HidePlayers
	case ArenaRespawn:
		return s.ArenaRespawn
	case Lightning:
		return s.Lightning
	}
	return false
}

func set(s *store.Settings, t Toggle, v bool) {
	switch t {
	case Scoreboard:
		s.Scoreboard = v
	case CPS:
		s.CPS = v
	case AutoSprint:
		s.AutoSprint = v
	case HidePlayers:
		s.HidePlayers = v
	case ArenaRespawn:
		s.ArenaRespawn = v
	case Lightning:
		s.Lightning = v
	}
}

type Manager struct {
	bundle  *lang.Bundle
	store   *store.Store
	players *state.Players
	forms   *ui.Forms

	Back         func(p *player.Player, tx *world.Tx)
	InCombat     func(o *state.Online) bool
	OnScoreboard func(o *state.Online, p *player.Player, tx *world.Tx, enabled bool)
	OnClick      func(o *state.Online, cps int)

	mu     sync.Mutex
	clicks map[uuid.UUID][]time.Time
	local  map[uuid.UUID]*store.Settings
}

func New(bundle *lang.Bundle, s *store.Store, players *state.Players, forms *ui.Forms) *Manager {
	return &Manager{
		bundle:  bundle,
		store:   s,
		players: players,
		forms:   forms,
		clicks:  map[uuid.UUID][]time.Time{},
		local:   map[uuid.UUID]*store.Settings{},
	}
}

func (m *Manager) Settings(o *state.Online) store.Settings {
	if prof, ok := m.store.ByXUID(o.XUID); ok {
		return prof.Settings
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.local[o.UUID]; ok {
		return *s
	}
	return defaults
}

func (m *Manager) Get(o *state.Online, t Toggle) bool {
	return get(m.Settings(o), t)
}

func (m *Manager) Set(o *state.Online, t Toggle, v bool) {
	if m.store.Update(o.XUID, func(p *store.Profile) { set(&p.Settings, t, v) }) {
		return
	}
	m.mu.Lock()
	s, ok := m.local[o.UUID]
	if !ok {
		d := defaults
		s = &d
		m.local[o.UUID] = s
	}
	set(s, t, v)
	m.mu.Unlock()
}

func (m *Manager) Forget(id uuid.UUID) {
	m.mu.Lock()
	delete(m.clicks, id)
	delete(m.local, id)
	m.mu.Unlock()
}

func (m *Manager) CPS(id uuid.UUID) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.countLocked(id, time.Now())
}

func (m *Manager) countLocked(id uuid.UUID, now time.Time) int {
	n := 0
	for _, t := range m.clicks[id] {
		if now.Sub(t) <= time.Second {
			n++
		}
	}
	return n
}

func (m *Manager) AddClick(id uuid.UUID) int {
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	list := m.clicks[id]
	keep := list[:0]
	for _, t := range list {
		if now.Sub(t) <= time.Second {
			keep = append(keep, t)
		}
	}
	m.clicks[id] = append(keep, now)
	return m.countLocked(id, now)
}

func (m *Manager) click(o *state.Online) {
	cps := m.AddClick(o.UUID)
	if m.Get(o, CPS) {
		o.Do(func(_ *world.Tx, p *player.Player) {
			p.SendTip("§e" + strconv.Itoa(cps) + " CPS")
		})
	}
	if m.OnClick != nil {
		m.OnClick(o, cps)
	}
}

func (m *Manager) Inbound(c *netmw.Conn, pk packet.Packet) bool {
	o, ok := m.players.Get(c.UUID())
	if !ok {
		return true
	}
	switch pk := pk.(type) {
	case *packet.PlayerAuthInput:
		m.input(o, pk.InputData)
		if pk.InputData.Load(packet.InputFlagMissedSwing) {
			m.click(o)
		}
	case *packet.LevelSoundEvent:
		if pk.SoundType == packet.SoundEventAttackNoDamage {
			m.click(o)
		}
	case *packet.InventoryTransaction:
		if _, ok := pk.TransactionData.(*protocol.UseItemOnEntityTransactionData); ok {
			m.click(o)
		}
	}
	return true
}

func (m *Manager) input(o *state.Online, in protocol.InputFlags) {
	stop := in.Load(packet.InputFlagDown) || in.Load(packet.InputFlagSneaking)
	start := in.Load(packet.InputFlagUp) && !in.Load(packet.InputFlagSneaking)
	if stop {
		if !in.Load(packet.InputFlagSprinting) && !in.Load(packet.InputFlagStartSprinting) {
			return
		}
	} else if !start || in.Load(packet.InputFlagSprinting) || !m.Get(o, AutoSprint) {
		return
	}
	o.Do(func(_ *world.Tx, p *player.Player) {
		switch {
		case p.Sprinting() && stop:
			p.StopSprinting()
		case !p.Sprinting() && start && m.Get(o, AutoSprint):
			p.StartSprinting()
		}
	})
}

const (
	WithBack = iota
	NoBack
)

func (m *Manager) lines(p *player.Player, key string) []string {
	l := lang.EN
	if o, ok := m.players.Get(p.UUID()); ok {
		l = o.Lang()
	}
	return m.bundle.Lines(l, key)
}

func (m *Manager) text(o *state.Online, key string) string {
	return m.bundle.Text(o.Lang(), key)
}

func (m *Manager) SendForm(p *player.Player, mode int) {
	form := ui.NewSimple(func(p *player.Player, tx *world.Tx, data *int) {
		if data == nil {
			return
		}
		switch *data {
		case 0:
			m.SendGeneralForm(p, mode)
		case 1:
			m.SendFFAForm(p, mode)
		case 2:
			if mode == WithBack && m.Back != nil {
				m.Back(p, tx)
			}
		}
	})
	contents := m.lines(p, "togglesForm")
	form.SetTitle(contents[0])
	form.AddButton(contents[1], ui.ImageNone, "")
	form.AddButton(contents[2], ui.ImageNone, "")
	if mode == WithBack {
		form.AddButton(contents[3], ui.ImageNone, "")
	}
	m.forms.Send(p, form)
}

type entry struct {
	toggle      Toggle
	on, off     string
	beforeApply func(o *state.Online) bool
	after       func(o *state.Online, p *player.Player, tx *world.Tx, enabled bool)
}

func (m *Manager) customForm(p *player.Player, mode int, key string, entries []entry) {
	o, ok := m.players.Get(p.UUID())
	if !ok {
		return
	}
	form := ui.NewCustom(func(p *player.Player, tx *world.Tx, data []any) {
		if data == nil {
			return
		}
		changed := false
		for i, e := range entries {
			v, _ := data[i].(bool)
			cur := m.Get(o, e.toggle)
			if v == cur {
				continue
			}
			changed = true
			if e.beforeApply != nil && !e.beforeApply(o) {
				continue
			}
			if cur {
				m.Set(o, e.toggle, false)
				o.Message(m.text(o, e.off))
			} else {
				m.Set(o, e.toggle, true)
				o.Message(m.text(o, e.on))
			}
			if e.after != nil {
				e.after(o, p, tx, !cur)
			}
		}
		if mode == WithBack && !changed {
			m.SendForm(p, mode)
		}
	})
	contents := m.bundle.Lines(o.Lang(), key)
	form.SetTitle(contents[0])
	for i, e := range entries {
		v := m.Get(o, e.toggle)
		form.AddToggle(contents[i+1], &v)
	}
	m.forms.Send(p, form)
}

func (m *Manager) SendGeneralForm(p *player.Player, mode int) {
	m.customForm(p, mode, "generalTogglesForm", []entry{
		{toggle: Scoreboard, on: "scoreboardEnabled", off: "scoreboardDisabled", after: func(o *state.Online, p *player.Player, tx *world.Tx, enabled bool) {
			if m.OnScoreboard != nil {
				m.OnScoreboard(o, p, tx, enabled)
			}
		}},
		{toggle: CPS, on: "cpsEnabled", off: "cpsDisabled"},
		{toggle: AutoSprint, on: "autosprintEnabled", off: "autosprintDisabled"},
	})
}

func (m *Manager) SendFFAForm(p *player.Player, mode int) {
	m.customForm(p, mode, "ffaTogglesForm", []entry{
		{toggle: HidePlayers, on: "hidePlayersEnabled", off: "hidePlayersDisabled", beforeApply: func(o *state.Online) bool {
			return m.InCombat == nil || !m.InCombat(o)
		}},
		{toggle: ArenaRespawn, on: "arenaRespawnEnabled", off: "arenaRespawnDisabled"},
		{toggle: Lightning, on: "lightningEnabled", off: "lightningDisabled"},
	})
}
