package command

import (
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/EraseMC/GoPractice/internal/chat"
	"github.com/EraseMC/GoPractice/internal/lang"
	"github.com/EraseMC/GoPractice/internal/punish"
	"github.com/EraseMC/GoPractice/internal/rank"
	"github.com/EraseMC/GoPractice/internal/state"
	"github.com/EraseMC/GoPractice/internal/store"
	"github.com/EraseMC/GoPractice/internal/telegram"
	"github.com/EraseMC/GoPractice/internal/text"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
)

const Cooldown = 2

type ServerStatus struct {
	TPS, TickUsage, AverageTPS, AverageTickUsage float64
}

type Services struct {
	Log      *slog.Logger
	Lang     *lang.Bundle
	Store    *store.Store
	Players  *state.Players
	Chat     *chat.Manager
	Punish   *punish.Manager
	Telegram *telegram.Client
	Started  time.Time

	Status          func() ServerStatus
	Stop            func()
	CleanCosmetics  func(p *store.Profile)
	RankChanged     func(o *state.Online)
	InCombat        func(o *state.Online) bool
	IsSpectator     func(o *state.Online) bool
	HasRespawnDelay func(o *state.Online) bool
	InFFA           func(o *state.Online) bool
	Duel            func(o *state.Online, p *player.Player, tx *world.Tx, args []string)
	InDuel          func(o *state.Online) bool
	InQueue         func(o *state.Online) bool
	LeaveFFA        func(o *state.Online, p *player.Player, tx *world.Tx)
	Rekit           func(o *state.Online, p *player.Player, tx *world.Tx)
	SurrenderFFA    func(o *state.Online, p *player.Player, tx *world.Tx) bool
	SurrenderDuel   func(o *state.Online, p *player.Player, tx *world.Tx) bool
	Hub             Hub
	TransferAddress func(p *player.Player) string
}

type Hub interface {
	Reset(o *state.Online, p *player.Player, tx *world.Tx)
	Teleport(p *player.Player, tx *world.Tx)
	SendSettings(p *player.Player)
	SendInfo(p *player.Player)
	SendRules(p *player.Player)
}

func (s *Services) RankOf(xuid string) string {
	if p, ok := s.Store.ByXUID(xuid); ok {
		return p.Rank
	}
	return rank.Default
}

func (s *Services) OnlineRank(o *state.Online) string {
	return s.RankOf(o.XUID)
}

func (s *Services) Has(o *state.Online, perm string) bool {
	return rank.Has(s.OnlineRank(o), perm)
}

func (s *Services) check(f func(o *state.Online) bool, o *state.Online) bool {
	return f != nil && f(o)
}

type Map struct {
	svc *Services

	mu      sync.RWMutex
	list    []*Command
	byLabel map[string]*Command

	cdMu     sync.Mutex
	cooldown map[string]int64
}

func NewMap(svc *Services) *Map {
	return &Map{svc: svc, byLabel: map[string]*Command{}, cooldown: map[string]int64{}}
}

func (m *Map) Services() *Services {
	return m.svc
}

func (m *Map) Register(cmds ...*Command) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range cmds {
		c.Name = strings.ToLower(c.Name)
		m.list = append(m.list, c)
		for _, l := range c.Labels() {
			m.byLabel[strings.ToLower(l)] = c
		}
	}
}

func (m *Map) Lookup(label string) (*Command, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.byLabel[label]
	return c, ok
}

func (m *Map) Commands() []*Command {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Command, len(m.list))
	copy(out, m.list)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (m *Map) Player(o *state.Online, p *player.Player, tx *world.Tx) *PlayerSender {
	return &PlayerSender{O: o, P: p, Tx: tx, rank: func() string { return m.svc.OnlineRank(o) }}
}

func (m *Map) Forget(name string) {
	m.cdMu.Lock()
	delete(m.cooldown, name)
	m.cdMu.Unlock()
}

func (m *Map) Dispatch(s Sender, line string) {
	ps, _ := s.(*PlayerSender)
	if ps != nil {
		now := time.Now().Unix()
		m.cdMu.Lock()
		last, ok := m.cooldown[ps.Name()]
		if ok && now-last <= Cooldown && ps.Rank() != rank.Owner {
			m.cdMu.Unlock()
			ps.Message(m.svc.Lang.Text(ps.Lang(), "commandCooldown"))
			return
		}
		m.cooldown[ps.Name()] = now
		m.cdMu.Unlock()
	}

	parts := strings.Split(line, " ")
	label := strings.ToLower(parts[0])
	line = label + " " + strings.Join(parts[1:], " ")
	cmd, ok := m.Lookup(label)
	if !ok {
		m.unknown(s, ps, label)
		return
	}

	args := text.SplitQuoteAware(line)
	if len(args) == 0 {
		m.unknown(s, ps, label)
		return
	}
	sent := args[0]
	if cmd, ok = m.Lookup(sent); !ok {
		m.unknown(s, ps, sent)
		return
	}
	c := &Ctx{Services: m.svc, Sender: s, Player: ps, Label: sent, Args: args[1:], Cmd: cmd}
	if !m.TestPermission(c) {
		return
	}
	cmd.Run(c)
}

func (m *Map) unknown(s Sender, ps *PlayerSender, label string) {
	if ps != nil {
		ps.Message(m.svc.Lang.Textf(ps.Lang(), "commandNotFoundOrNoPermission", label))
		return
	}
	s.Message(lang.Sprintf("§cUnknown command: %s. Please check that the command exists and that you have permission to use it.", label))
}

func (m *Map) TestPermission(c *Ctx) bool {
	if c.Cmd.Permission == "" || c.Sender.HasPermission(c.Cmd.Permission) {
		return true
	}
	if c.Cmd.Vanilla {
		c.Reply("§cYou don't have permission to use the command: " + c.Cmd.Name)
		return false
	}
	m.unknown(c.Sender, c.Player, c.Cmd.Name)
	return false
}
