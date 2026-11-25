package stats

import (
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/EraseMC/GoPractice/internal/lang"
	"github.com/EraseMC/GoPractice/internal/state"
	"github.com/EraseMC/GoPractice/internal/store"
	"github.com/EraseMC/GoPractice/internal/ui"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/google/uuid"
)

const TopSize = 10

type Manager struct {
	bundle  *lang.Bundle
	store   *store.Store
	players *state.Players
	forms   *ui.Forms
	now     func() time.Time

	mu       sync.Mutex
	joined   map[uuid.UUID]int64
	killers  []store.TopEntry
	playTime []store.TopEntry
}

func New(bundle *lang.Bundle, s *store.Store, players *state.Players, forms *ui.Forms) *Manager {
	return &Manager{bundle: bundle, store: s, players: players, forms: forms, now: time.Now, joined: map[uuid.UUID]int64{}}
}

func (m *Manager) profile(o *state.Online) store.Profile {
	p, _ := m.store.ByXUID(o.XUID)
	return p
}

func (m *Manager) Kills(o *state.Online) int  { return m.profile(o).Kills }
func (m *Manager) Deaths(o *state.Online) int { return m.profile(o).Deaths }

func (m *Manager) AddKill(o *state.Online, n int) {
	m.store.Update(o.XUID, func(p *store.Profile) { p.Kills += n })
}

func (m *Manager) AddDeath(o *state.Online, n int) {
	m.store.Update(o.XUID, func(p *store.Profile) { p.Deaths += n })
}

func Division(kills int) string {
	switch {
	case kills < 0:
		return "§0Bedrock"
	case kills < 50:
		return "§6Bronze"
	case kills < 100:
		return "§7Silver"
	case kills < 150:
		return "§eGold"
	case kills < 200:
		return "§dPlatinum"
	case kills < 300:
		return "§bDiamond"
	case kills < 400:
		return "§aEmerald"
	case kills < 500:
		return "§8Netherite"
	case kills < 1000:
		return "§5Sapphire"
	case kills < 2000:
		return "§cMaster"
	}
	return "§cMaster§e+"
}

func (m *Manager) DivisionColored(o *state.Online) string {
	return Division(m.Kills(o))
}

func KDR(kills, deaths int) string {
	if deaths == 0 {
		return strconv.Itoa(kills)
	}
	r := float64(kills) / float64(deaths)
	r = math.Round(r*100) / 100
	return strconv.FormatFloat(r, 'f', -1, 64)
}

func Hours(seconds int) int {
	return seconds / 3600
}

func (m *Manager) Join(o *state.Online) {
	m.mu.Lock()
	m.joined[o.UUID] = m.now().Unix()
	m.mu.Unlock()
}

func (m *Manager) session(id uuid.UUID) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.joined[id]
	if !ok {
		return 0
	}
	return int(m.now().Unix() - t)
}

func (m *Manager) Quit(o *state.Online) {
	played := m.session(o.UUID)
	m.store.Update(o.XUID, func(p *store.Profile) { p.PlayTime += played })
	m.mu.Lock()
	delete(m.joined, o.UUID)
	m.mu.Unlock()
}

func (m *Manager) PlayTimeHours(o *state.Online) int {
	return Hours(m.profile(o).PlayTime + m.session(o.UUID))
}

func (m *Manager) UpdateTops() {
	kills := m.store.FetchTop(store.StatKills, TopSize)
	played := m.store.FetchTop(store.StatPlayTime, TopSize)
	go func() {
		k, p := <-kills, <-played
		m.mu.Lock()
		if k != nil {
			m.killers = k
		}
		if p != nil {
			m.playTime = p
		}
		m.mu.Unlock()
	}()
}

func (m *Manager) tops() ([]store.TopEntry, []store.TopEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.killers, m.playTime
}

func rowColor(row int) string {
	switch row {
	case 1:
		return "§6"
	case 2:
		return "§g"
	case 3:
		return "§e"
	}
	return "§7"
}

func topText(format string, entries []store.TopEntry, value func(int) int) string {
	var b strings.Builder
	for i, e := range entries {
		b.WriteString(lang.Sprintf(format, rowColor(i+1), i+1, e.Nickname, value(e.Value)))
		b.WriteByte('\n')
	}
	if b.Len() == 0 {
		return ":("
	}
	return b.String()
}

func (m *Manager) online(p *player.Player) (*state.Online, bool) {
	return m.players.Get(p.UUID())
}

func (m *Manager) SendStatisticsForm(p *player.Player) {
	o, ok := m.online(p)
	if !ok {
		return
	}
	form := ui.NewSimple(func(p *player.Player, _ *world.Tx, data *int) {
		if data != nil && *data == 0 {
			m.SendTopsForm(p)
		}
	})
	contents := m.bundle.Lines(o.Lang(), "statisticsForm")
	prof := m.profile(o)
	form.SetTitle(contents[0])
	form.SetContent(lang.Sprintf(contents[1],
		Division(prof.Kills),
		prof.Kills,
		prof.Deaths,
		KDR(prof.Kills, prof.Deaths),
		m.PlayTimeHours(o),
	))
	form.AddButton(contents[2], ui.ImageNone, "")
	m.forms.Send(p, form)
}

func (m *Manager) SendTopsForm(p *player.Player) {
	o, ok := m.online(p)
	if !ok {
		return
	}
	form := ui.NewSimple(func(p *player.Player, _ *world.Tx, data *int) {
		if data == nil {
			return
		}
		switch *data {
		case 0:
			m.SendTopKillersForm(p)
		case 1:
			m.SendTopPlayTimeForm(p)
		case 2:
			m.SendStatisticsForm(p)
		}
	})
	contents := m.bundle.Lines(o.Lang(), "topsForm")
	form.SetTitle(contents[0])
	form.AddButton(contents[1], ui.ImageNone, "")
	form.AddButton(contents[2], ui.ImageNone, "")
	form.AddButton(contents[3], ui.ImageNone, "")
	m.forms.Send(p, form)
}

func (m *Manager) sendTop(p *player.Player, key string, entries []store.TopEntry, value func(int) int) {
	o, ok := m.online(p)
	if !ok {
		return
	}
	form := ui.NewSimple(func(p *player.Player, _ *world.Tx, data *int) {
		if data != nil && *data == 0 {
			m.SendTopsForm(p)
		}
	})
	contents := m.bundle.Lines(o.Lang(), key)
	form.SetTitle(contents[0])
	form.SetContent(topText(contents[1], entries, value))
	form.AddButton(contents[2], ui.ImageNone, "")
	m.forms.Send(p, form)
}

func (m *Manager) SendTopKillersForm(p *player.Player) {
	killers, _ := m.tops()
	m.sendTop(p, "topKillersForm", killers, func(v int) int { return v })
}

func (m *Manager) SendTopPlayTimeForm(p *player.Player) {
	_, played := m.tops()
	m.sendTop(p, "topPlayTimeForm", played, Hours)
}
