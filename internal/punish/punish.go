package punish

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/EraseMC/GoPractice/internal/lang"
	"github.com/EraseMC/GoPractice/internal/store"
)

type Manager struct {
	bundle *lang.Bundle
	store  *store.Store

	mu    sync.Mutex
	muted map[string]bool
}

func New(bundle *lang.Bundle, s *store.Store) *Manager {
	return &Manager{bundle: bundle, store: s, muted: map[string]bool{}}
}

func (m *Manager) OnJoin(name, xuid string) {
	_, byName := m.store.Punishments.MuteByNickname(strings.ToLower(name))
	_, byXUID := m.store.Punishments.MuteByXUID(xuid)
	if byName || byXUID {
		m.mu.Lock()
		m.muted[name] = true
		m.mu.Unlock()
	}
}

func (m *Manager) OnQuit(name string) {
	m.mu.Lock()
	delete(m.muted, name)
	m.mu.Unlock()
}

func (m *Manager) Muted(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.muted[name]
}

func (m *Manager) mute(name, xuid string) (store.Mute, bool) {
	if mu, ok := m.store.Punishments.MuteByNickname(strings.ToLower(name)); ok {
		return mu, true
	}
	return m.store.Punishments.MuteByXUID(xuid)
}

func (m *Manager) RawMuteExpiry(name, xuid string) int64 {
	mu, ok := m.mute(name, xuid)
	if !ok {
		return 0
	}
	return mu.Expiry
}

func (m *Manager) RemoveMute(name, xuid string) {
	if _, ok := m.store.Punishments.MuteByNickname(strings.ToLower(name)); ok {
		m.store.Punishments.RemoveMutes(strings.ToLower(name))
	} else if _, ok := m.store.Punishments.MuteByXUID(xuid); ok {
		m.store.Punishments.RemoveMutesByXUID(xuid)
	} else {
		m.store.Punishments.RemoveMutes(strings.ToLower(name))
	}
	m.mu.Lock()
	delete(m.muted, name)
	m.mu.Unlock()
}

func (m *Manager) SyncMutedNickname(name, xuid string) {
	mu, ok := m.store.Punishments.MuteByXUID(xuid)
	if ok && mu.Nickname != strings.ToLower(name) {
		m.store.Punishments.RenameMutes(xuid, strings.ToLower(name))
	}
}

func (m *Manager) MuteMessage(l lang.Lang, name, xuid string) string {
	mutedNickname := "Steve"
	if mu, ok := m.store.Punishments.MuteByXUID(xuid); ok {
		mutedNickname = mu.Nickname
	}
	date, reason, expiry := "00.00.00", "Unknown", m.bundle.Text(l, "muteNeverExpires")
	if mu, ok := m.mute(name, xuid); ok {
		date = time.Unix(mu.When, 0).Format("02.01.06")
		reason = mu.Reason
		expiry = m.Expiry(l, mu.Expiry)
	}
	return m.bundle.Textf(l, "muteMessage", mutedNickname, date, reason, expiry)
}

func (m *Manager) Expiry(l lang.Lang, expiry int64) string {
	if expiry == 0 {
		return m.bundle.Text(l, "muteNeverExpires")
	}
	d, h, i := Remaining(expiry - time.Now().Unix())
	return m.bundle.Textf(l, "muteExpiry", d, h, i)
}

func Remaining(left int64) (days, hours, minutes string) {
	if left < 0 {
		left = 0
	}
	hours = fmt.Sprintf("%02d", left%86400/3600)
	minutes = fmt.Sprintf("%02d", left%3600/60)
	if left < 86400 {
		return "0", hours, minutes
	}
	return fmt.Sprintf("%02d", left/86400), hours, minutes
}
