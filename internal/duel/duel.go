package duel

import (
	"log/slog"
	"math/rand/v2"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/EraseMC/GoPractice/internal/chat"
	"github.com/EraseMC/GoPractice/internal/combat"
	"github.com/EraseMC/GoPractice/internal/hub"
	"github.com/EraseMC/GoPractice/internal/items"
	"github.com/EraseMC/GoPractice/internal/lang"
	"github.com/EraseMC/GoPractice/internal/state"
	"github.com/EraseMC/GoPractice/internal/stats"
	"github.com/EraseMC/GoPractice/internal/toggles"
	"github.com/EraseMC/GoPractice/internal/ui"
	"github.com/EraseMC/GoPractice/internal/worlds"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
)

type Deps struct {
	Log     *slog.Logger
	Lang    *lang.Bundle
	Players *state.Players
	Forms   *ui.Forms
	Boards  *ui.Scoreboards
	Chat    *chat.Manager
	Toggles *toggles.Manager
	Stats   *stats.Manager
	Combat  *combat.Manager
	Items   *items.Manager
	Hub     *hub.Manager
}

type Manager struct {
	Deps
	pool map[string][]arena

	mu      sync.Mutex
	queue   []entry
	invites []invite
	matches map[*world.World]*match
	closing bool
}

type entry struct {
	o     *state.Online
	mode  mode
	since time.Time
}

type invite struct {
	from, to *state.Online
	mode     mode
	expires  time.Time
}

func New(d Deps) *Manager {
	return &Manager{Deps: d, matches: make(map[*world.World]*match)}
}

func (m *Manager) Load(dir string) error {
	pool, err := loadMaps(m.Log, dir)
	m.pool = pool
	return err
}

func (m *Manager) message(o *state.Online, key string, args ...any) {
	o.Message(m.Lang.Textf(o.Lang(), key, args...))
}

func (m *Manager) InQueue(o *state.Online) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.ContainsFunc(m.queue, func(e entry) bool { return e.o == o })
}

func (m *Manager) available(o *state.Online, tx *world.Tx) (*player.Player, bool) {
	if tx.World() != m.Hub.World() || !m.Hub.InHub(o) || m.InQueue(o) {
		return nil, false
	}
	p, ok := playerIn(tx, o)
	return p, ok && !p.Dead()
}

func (m *Manager) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.matches) * 2
}

func (m *Manager) QueueForm(o *state.Online, p *player.Player, tx *world.Tx) {
	if _, ok := m.available(o, tx); !ok {
		return
	}
	lines := m.Lang.Lines(o.Lang(), "duelsFormQueue")
	form := ui.NewSimple(func(p *player.Player, tx *world.Tx, choice *int) {
		if choice != nil && *choice >= 0 && *choice < len(modes) {
			m.enqueue(o, p, tx, modes[*choice])
		}
	})
	form.SetTitle(lines[0])
	m.mu.Lock()
	for _, mode := range modes {
		playing, waiting := 0, 0
		for _, match := range m.matches {
			if match.mode.id == mode.id {
				playing += 2
			}
		}
		for _, entry := range m.queue {
			if entry.mode.id == mode.id {
				waiting++
			}
		}
		form.AddButton(lang.Sprintf(lines[1], mode.display, playing, waiting), ui.ImagePath, "textures/items/"+mode.icon)
	}
	m.mu.Unlock()
	m.Forms.Send(p, form)
}

func (m *Manager) enqueue(o *state.Online, p *player.Player, tx *world.Tx, mode mode) {
	if _, ok := m.available(o, tx); !ok || len(m.pool[mode.id]) == 0 {
		return
	}
	entry := entry{o, mode, time.Now()}
	m.mu.Lock()
	m.queue = append(m.queue, entry)
	queue := slices.Clone(m.queue)
	m.mu.Unlock()
	p.Inventory().Clear()
	p.Armour().Clear()
	m.queueItem(o, p)
	m.queueBoard(entry, len(queue), time.Now())
	m.message(o, "duelJoinedQueue", mode.display)
	for _, other := range queue {
		if other.o == o || other.mode.id != mode.id {
			continue
		}
		if target, ok := playerIn(tx, other.o); ok && !target.Dead() && m.Hub.InHub(other.o) {
			if m.start(tx, mode, o, p, other.o, target) {
				return
			}
		}
	}
}

func (m *Manager) queueItem(o *state.Online, p *player.Player) {
	stack := item.NewStack(item.RedstoneWire{}, 1).
		WithCustomName(m.Lang.Text(o.Lang(), "duelsItemLeaveQueue")).
		WithLore(hub.ItemLore).WithValue("duel-leave-queue", true)
	_ = p.Inventory().SetItem(8, stack)
}

func (m *Manager) leaveQueue(o *state.Online, p *player.Player) {
	m.mu.Lock()
	m.queue = slices.DeleteFunc(m.queue, func(e entry) bool { return e.o == o })
	m.mu.Unlock()
	m.Hub.SendMenuItems(o, p)
	m.Hub.UpdateScoreboard(o)
	m.message(o, "duelLeftQueue")
}

func (m *Manager) InviteForm(o *state.Online, p *player.Player, target *state.Online, tx *world.Tx) {
	if !m.canInvite(o, target, tx) {
		return
	}
	lines := m.Lang.Lines(o.Lang(), "duelsFormInvite")
	form := ui.NewSimple(func(_ *player.Player, tx *world.Tx, choice *int) {
		if choice != nil && *choice >= 0 && *choice < len(modes) && m.canInvite(o, target, tx) {
			m.sendInvite(o, target, modes[*choice])
		}
	})
	form.SetTitle(lang.Sprintf(lines[0], target.Name))
	for _, mode := range modes {
		form.AddButton(lang.Sprintf(lines[1], mode.display), ui.ImagePath, "textures/items/"+mode.icon)
	}
	m.Forms.Send(p, form)
}

func (m *Manager) canInvite(o, target *state.Online, tx *world.Tx) bool {
	if _, ok := m.available(o, tx); !ok {
		m.message(o, "duelCommandNotInHub")
		return false
	}
	if o == target {
		m.message(o, "duelCommandSelf")
		return false
	}
	if current, ok := m.Players.Get(target.UUID); !ok || current != target {
		m.message(o, "playerNotFound")
		return false
	}
	if _, ok := m.available(target, tx); !ok {
		m.message(o, "duelCommandTargetNotInHub")
		return false
	}
	m.mu.Lock()
	duplicate := slices.ContainsFunc(m.invites, func(i invite) bool {
		return i.from == o && i.to == target && time.Now().Before(i.expires)
	})
	m.mu.Unlock()
	if duplicate {
		m.message(o, "duelCommandInviteAlreadySent")
	}
	return !duplicate
}

func (m *Manager) sendInvite(from, to *state.Online, mode mode) {
	if len(m.pool[mode.id]) == 0 {
		m.message(from, "duelCommandModeNotFound")
		return
	}
	m.mu.Lock()
	m.invites = append(m.invites, invite{from, to, mode, time.Now().Add(30 * time.Second)})
	m.mu.Unlock()
	m.message(from, "duelCommandInviteSuccess", to.Name, mode.display)
	m.message(to, "duelCommandInviteNotification", from.Name, mode.display)
}

func (m *Manager) Command(o *state.Online, p *player.Player, tx *world.Tx, args []string) {
	if _, ok := m.available(o, tx); !ok {
		m.message(o, "duelCommandNotInHub")
		return
	}
	if len(args) == 0 {
		m.message(o, "duelCommandUsage")
		return
	}
	var target *state.Online
	if len(args) > 1 {
		target, _ = m.Players.Find(args[1])
	}
	switch strings.ToLower(args[0]) {
	case "invite":
		switch {
		case len(args) == 1:
			m.message(o, "duelCommandInviteUsage")
		case target == nil:
			m.message(o, "playerNotFound")
		case len(args) == 2:
			m.InviteForm(o, p, target, tx)
		case m.canInvite(o, target, tx):
			mode, ok := findMode(args[2])
			if !ok {
				m.message(o, "duelCommandModeNotFound")
				return
			}
			m.sendInvite(o, target, mode)
		}
	case "accept":
		if len(args) > 1 && target == nil {
			m.message(o, "playerNotFound")
			return
		}
		m.accept(o, p, tx, target)
	default:
		m.message(o, "duelCommandUsage")
	}
}

func (m *Manager) accept(o *state.Online, p *player.Player, tx *world.Tx, target *state.Online) {
	m.mu.Lock()
	var request invite
	for _, i := range slices.Backward(m.invites) {
		if i.to == o && (target == nil || i.from == target) && time.Now().Before(i.expires) {
			request = i
			break
		}
	}
	m.mu.Unlock()
	if target == nil && request.from == nil {
		m.message(o, "duelCommandInviteNotFound")
		return
	}
	if target == nil {
		target = request.from
	}
	opponent, ok := m.available(target, tx)
	switch {
	case !ok:
		m.message(o, "duelCommandTargetNotInHub")
	case request.from == nil:
		m.message(o, "duelCommandInviteNotFoundFrom")
	case m.start(tx, request.mode, target, opponent, o, p):
		m.message(o, "duelCommandInviteAccepted", target.Name)
		m.message(target, "duelCommandInviteAcceptedBy", o.Name)
	}
}

func (m *Manager) start(tx *world.Tx, mode mode, first *state.Online, p *player.Player, second *state.Online, opponent *player.Player) bool {
	m.mu.Lock()
	if m.closing {
		m.mu.Unlock()
		return false
	}
	options := m.pool[mode.id]
	arena := options[rand.IntN(len(options))]
	w := arena.snapshot.New(m.Log)
	w.SetTime(1000)
	w.StopTime()
	if mode.id == "classic_nodebuff" && w.Difficulty().FoodRegenerates() {
		w.SetDifficulty(world.DifficultyNormal)
	}
	match := &match{
		mode: mode, world: w, players: [2]*state.Online{first, second}, countdown: 3,
	}
	m.matches[w] = match
	m.queue = slices.DeleteFunc(m.queue, func(e entry) bool { return e.o == first || e.o == second })
	m.invites = slices.DeleteFunc(m.invites, func(i invite) bool {
		return i.from == first || i.from == second || i.to == first || i.to == second
	})
	m.mu.Unlock()
	order := rand.Perm(len(arena.spawns))
	for i, participant := range []*player.Player{p, opponent} {
		o := match.players[i]
		m.Forms.Reset(o.UUID)
		spawn := arena.spawns[order[i]]
		hub.BasicProperties(participant, world.GameModeAdventure)
		m.Items.ResetPearlCooldown(participant)
		m.Combat.ResetImmunity(o.UUID)
		o.SetPlace(state.PlaceDuel, mode.id)
		worlds.MoveThen(tx, participant, w, spawn.position(), spawn.Yaw, spawn.Pitch, func(tx *world.Tx, p *player.Player) {
			if match.released {
				m.returnToHub(o, p, tx)
				return
			}
			if mode.id == "sumo" {
				p.SetImmobile()
			} else {
				mode.giveKit(p)
			}
			match.joined[i] = true
			p.Message(m.Lang.Textf(o.Lang(), "duelStarting", mode.display, match.players[1-i].Name))
			m.updateTag(match, i, p)
			m.matchBoard(match, i)
		})
	}
	return true
}
