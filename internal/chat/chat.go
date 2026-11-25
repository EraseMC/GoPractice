package chat

import (
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/EraseMC/GoPractice/internal/device"
	"github.com/EraseMC/GoPractice/internal/lang"
	"github.com/EraseMC/GoPractice/internal/punish"
	"github.com/EraseMC/GoPractice/internal/rank"
	"github.com/EraseMC/GoPractice/internal/state"
	"github.com/EraseMC/GoPractice/internal/store"
	"github.com/EraseMC/GoPractice/internal/tags"
	"github.com/EraseMC/GoPractice/internal/text"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/google/uuid"
)

const (
	MessageLengthLimit = 125
	Cooldown           = 1
	MaxCharLength      = 512
	MaxByteLength      = MaxCharLength * 4
	MessagesPerTick    = 2
)

var emoji = strings.NewReplacer(
	":skull:", "",
	":fire:", "",
	":eyes:", "",
	":clown:", "",
	":100:", "",
	":laugh:", "",
	":heart:", "",
)

type counter struct {
	tick uint64
	left int
}

type Manager struct {
	log     *slog.Logger
	bundle  *lang.Bundle
	store   *store.Store
	players *state.Players
	punish  *punish.Manager
	tick    func() uint64

	mu       sync.Mutex
	enabled  bool
	reply    map[string]string
	cooldown map[string]int64
	last     map[string]string
	counters map[uuid.UUID]*counter
}

func New(log *slog.Logger, bundle *lang.Bundle, s *store.Store, players *state.Players, p *punish.Manager, tick func() uint64) *Manager {
	return &Manager{
		log:      log,
		bundle:   bundle,
		store:    s,
		players:  players,
		punish:   p,
		tick:     tick,
		enabled:  true,
		reply:    map[string]string{},
		cooldown: map[string]int64{},
		last:     map[string]string{},
		counters: map[uuid.UUID]*counter{},
	}
}

func (m *Manager) Enabled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.enabled
}

func (m *Manager) SetEnabled(v bool) {
	m.mu.Lock()
	m.enabled = v
	m.mu.Unlock()
}

func (m *Manager) SetReply(target, from string) {
	m.mu.Lock()
	m.reply[target] = from
	m.mu.Unlock()
}

func (m *Manager) ReplyTarget(name string) (*state.Online, bool) {
	m.mu.Lock()
	to, ok := m.reply[name]
	m.mu.Unlock()
	if !ok {
		return nil, false
	}
	o, ok := m.players.ByName(to)
	if !ok {
		m.mu.Lock()
		delete(m.reply, name)
		m.mu.Unlock()
	}
	return o, ok
}

func (m *Manager) Forget(o *state.Online) {
	m.mu.Lock()
	delete(m.cooldown, o.Name)
	delete(m.last, o.Name)
	delete(m.reply, o.Name)
	delete(m.counters, o.UUID)
	m.mu.Unlock()
}

func (m *Manager) profile(o *state.Online) store.Profile {
	p, ok := m.store.ByXUID(o.XUID)
	if !ok {
		p.Rank = rank.Default
	}
	return p
}

func (m *Manager) NameTag(o *state.Online) string {
	return device.Icon(device.OSName(o.DeviceOS, o.DeviceModel)) + " " + rank.NameTag(m.profile(o).Rank, o.Name)
}

func (m *Manager) HasTag(o *state.Online) bool {
	return m.profile(o).Cosmetics.Tag != ""
}

func (m *Manager) TagColored(o *state.Online) string {
	return tags.Colored(m.profile(o).Cosmetics.Tag)
}

func (m *Manager) TagFormat(o *state.Online) string {
	prof := m.profile(o)
	if !rank.Has(prof.Rank, rank.CosmeticsTagsSilver) {
		return ""
	}
	return "§l" + tags.Colored(prof.Cosmetics.Tag) + "§r "
}

func (m *Manager) Take(id uuid.UUID) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.counters[id]
	now := m.tick()
	if !ok || c.tick != now {
		c = &counter{tick: now, left: MessagesPerTick}
		m.counters[id] = c
	}
	return c.left
}

func (m *Manager) Consume(id uuid.UUID) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.counters[id]
	if !ok {
		return false
	}
	c.left--
	return c.left >= 0
}

func (m *Manager) Send(o *state.Online, p *player.Player, message string) {
	l := o.Lang()
	prof := m.profile(o)
	privileged := rank.Priority(prof.Rank) >= rank.Priority(rank.Manager)

	if !m.Enabled() && !privileged {
		o.Message(m.bundle.Text(l, "chatCurrentlyDisabled"))
		return
	}

	if m.punish.Muted(o.Name) {
		expiry := m.punish.RawMuteExpiry(o.Name, o.XUID)
		if time.Now().Unix() >= expiry && expiry != 0 {
			m.punish.RemoveMute(o.Name, o.XUID)
		} else {
			m.punish.SyncMutedNickname(o.Name, o.XUID)
			o.Message(m.punish.MuteMessage(l, o.Name, o.XUID))
			return
		}
	}

	if text.Len(message) > MessageLengthLimit && !privileged {
		o.Message(m.bundle.Text(l, "chatLongMessage"))
		return
	}

	now := time.Now().Unix()
	m.mu.Lock()
	last, ok := m.cooldown[o.Name]
	if ok && now-last <= Cooldown && !privileged {
		m.mu.Unlock()
		o.Message(m.bundle.Text(l, "chatCooldown"))
		return
	}
	m.cooldown[o.Name] = now
	prev, ok := m.last[o.Name]
	if ok && strings.ToLower(prev) == strings.ToLower(message) && !privileged {
		m.mu.Unlock()
		o.Message(m.bundle.Text(l, "chatCooldown"))
		return
	}
	m.last[o.Name] = message
	m.mu.Unlock()

	message = emoji.Replace(text.Trim(text.Clean(message, true)))
	format := rank.ChatFormat(prof.Rank, o.Name)
	final := format + message
	if prof.Cosmetics.Tag != "" {
		final = m.TagFormat(o) + format + message
	}

	for _, r := range m.players.All() {
		r.Raw(final)
	}
	m.log.Info(text.Clean(final, true))
}
