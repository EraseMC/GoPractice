package language

import (
	"log/slog"
	"strings"
	"sync"

	"github.com/EraseMC/GoPractice/internal/lang"
	"github.com/EraseMC/GoPractice/internal/state"
	"github.com/EraseMC/GoPractice/internal/store"
	"github.com/EraseMC/GoPractice/internal/ui"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
)

type Manager struct {
	log     *slog.Logger
	bundle  *lang.Bundle
	store   *store.Store
	players *state.Players
	forms   *ui.Forms

	mu       sync.RWMutex
	back     func(p *player.Player, tx *world.Tx)
	onChange []func(p *player.Player, tx *world.Tx, l lang.Lang)
}

func New(log *slog.Logger, bundle *lang.Bundle, s *store.Store, players *state.Players, forms *ui.Forms) *Manager {
	return &Manager{log: log, bundle: bundle, store: s, players: players, forms: forms}
}

func (m *Manager) SetBack(f func(p *player.Player, tx *world.Tx)) {
	m.mu.Lock()
	m.back = f
	m.mu.Unlock()
}

func (m *Manager) OnChange(f func(p *player.Player, tx *world.Tx, l lang.Lang)) {
	m.mu.Lock()
	m.onChange = append(m.onChange, f)
	m.mu.Unlock()
}

func ClientLocale(p *player.Player) string {
	return strings.ReplaceAll(p.Locale().String(), "-", "_")
}

func (m *Manager) Of(p *player.Player) lang.Lang {
	if prof, ok := m.store.ByXUID(p.XUID()); ok {
		if l, ok := lang.Parse(prof.Locale); ok {
			return l
		}
		return lang.Lang(prof.Locale)
	}
	if o, ok := m.players.Get(p.UUID()); ok {
		if l := o.Lang(); l != "" {
			return l
		}
	}
	return lang.FromLocale(ClientLocale(p))
}

func (m *Manager) ByNickname(nickname string) (lang.Lang, bool) {
	prof, ok := m.store.ByNickname(nickname)
	if !ok {
		return "", false
	}
	return lang.Lang(prof.Locale), true
}

func (m *Manager) ByXUID(xuid string) (lang.Lang, bool) {
	prof, ok := m.store.ByXUID(xuid)
	if !ok {
		return "", false
	}
	return lang.Lang(prof.Locale), true
}

func (m *Manager) Set(p *player.Player, l lang.Lang) {
	if _, ok := lang.Parse(string(l)); !ok {
		m.log.Error("Undefined language: " + string(l))
		return
	}
	m.store.Update(p.XUID(), func(prof *store.Profile) { prof.Locale = string(l) })
	if o, ok := m.players.Get(p.UUID()); ok {
		o.SetLang(l)
	}
}

func (m *Manager) SetByNickname(nickname string, l lang.Lang) {
	if _, ok := lang.Parse(string(l)); !ok {
		m.log.Error("Undefined language: " + string(l))
		return
	}
	m.store.UpdateByNickname(nickname, func(prof *store.Profile) { prof.Locale = string(l) })
	if o, ok := m.players.ByName(nickname); ok {
		o.SetLang(l)
	}
}

func (m *Manager) T(p *player.Player, key string) string {
	return m.bundle.Text(m.Of(p), key)
}

func (m *Manager) Tf(p *player.Player, key string, args ...any) string {
	return m.bundle.Textf(m.Of(p), key, args...)
}

func (m *Manager) Lines(p *player.Player, key string) []string {
	return m.bundle.Lines(m.Of(p), key)
}

func (m *Manager) In(p *player.Player, s lang.Section, key string) string {
	return m.bundle.In(m.Of(p), s, key)
}

func (m *Manager) SendLanguageForm(p *player.Player) {
	form := ui.NewSimple(func(p *player.Player, tx *world.Tx, data *int) {
		if data == nil {
			return
		}
		var l lang.Lang
		switch *data {
		case 0:
			l = lang.EN
		case 1:
			l = lang.RU
		case 2:
			m.mu.RLock()
			back := m.back
			m.mu.RUnlock()
			if back != nil {
				back(p, tx)
			}
			return
		}
		if l == "" {
			return
		}
		if m.Of(p) == l {
			ui.Message(p, m.T(p, "languageAlreadySelected"))
			return
		}
		m.SendConfirmationForm(p, l)
	})

	contents := m.Lines(p, "languageForm")
	form.SetTitle(contents[0])
	form.AddButton(contents[1], ui.ImagePath, "textures/oldtralia/flags/english_flag")
	form.AddButton(contents[2], ui.ImagePath, "textures/oldtralia/flags/russian_flag")
	form.AddButton(contents[3], ui.ImagePath, "textures/ui/arrow_left")
	m.forms.Send(p, form)
}

func (m *Manager) SendConfirmationForm(p *player.Player, l lang.Lang) {
	form := ui.NewModal(func(p *player.Player, tx *world.Tx, data *bool) {
		if data == nil {
			return
		}
		if *data {
			m.Set(p, l)
			ui.Message(p, m.T(p, "languageSuccessfullySelected"))
			m.mu.RLock()
			hooks := m.onChange
			m.mu.RUnlock()
			for _, h := range hooks {
				h(p, tx, l)
			}
			return
		}
		m.SendLanguageForm(p)
	})

	contents := m.Lines(p, "languageConfirmationForm")
	form.SetTitle(contents[0])
	form.SetContent(lang.Sprintf(contents[1], m.In(p, lang.Language, string(l))))
	form.SetFirstButton(contents[2])
	form.SetSecondButton(contents[3])
	m.forms.Send(p, form)
}
