package duel

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"time"

	"github.com/EraseMC/GoPractice/internal/ffa"
	"github.com/EraseMC/GoPractice/internal/hub"
	"github.com/EraseMC/GoPractice/internal/lang"
	"github.com/EraseMC/GoPractice/internal/state"
	"github.com/EraseMC/GoPractice/internal/toggles"
	"github.com/EraseMC/GoPractice/internal/worlds"
	"github.com/df-mc/dragonfly/server/entity/effect"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/item/enchantment"
	"github.com/df-mc/dragonfly/server/item/potion"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/player/title"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const kitEffectDuration = time.Duration(math.MaxInt32) * 50 * time.Millisecond

type phase int

const (
	waiting phase = iota
	running
	ended
)

type match struct {
	mode              mode
	world             *world.World
	players           [2]*state.Online
	joined, gone      [2]bool
	hits, supplies    [2]int
	phase             phase
	countdown         int
	released          bool
	started, finished time.Time
	winner            int
}

func (m mode) giveKit(p *player.Player) {
	inv := p.Inventory()
	switch m.id {
	case "sumo", "boxing":
		if m.id == "boxing" {
			sword := item.NewStack(item.Sword{Tier: item.ToolTierDiamond}, 1).AsUnbreakable().
				WithEnchantments(item.NewEnchantment(enchantment.Unbreaking, 10))
			_ = inv.SetItem(0, sword)
			p.AddEffect(effect.New(effect.Speed, 1, kitEffectDuration).WithoutParticles())
		}
		p.AddEffect(effect.New(effect.Resistance, 256, kitEffectDuration).WithoutParticles())
	case "classic_nodebuff":
		ffa.GiveNoDebuffKit(p)
		swift := item.NewStack(item.Potion{Type: potion.StrongSwiftness()}, 1)
		for _, slot := range []int{2, 17, 26} {
			_ = inv.SetItem(slot, swift)
		}
		_ = inv.SetItem(8, item.NewStack(item.GoldenCarrot{}, 64))
	default:
		kit := ffa.Arena{Kind: m.kit}
		kit.GiveKit(p)
	}
}

func (m *Manager) current(p *player.Player) (*match, int) {
	m.mu.Lock()
	match := m.matches[p.Tx().World()]
	m.mu.Unlock()
	if match != nil {
		for i, o := range match.players {
			if o.Handle == p.H() {
				return match, i
			}
		}
	}
	return nil, 0
}

func (m *Manager) Tick() {
	now := time.Now()
	m.mu.Lock()
	m.invites = slices.DeleteFunc(m.invites, func(i invite) bool { return !now.Before(i.expires) })
	queue := slices.Clone(m.queue)
	matches := make([]*match, 0, len(m.matches))
	for _, match := range m.matches {
		matches = append(matches, match)
	}
	m.mu.Unlock()
	if len(queue) != 0 {
		m.Hub.World().Do(func(_ *world.Tx) {
			m.mu.Lock()
			active := slices.Clone(m.queue)
			m.mu.Unlock()
			for _, entry := range active {
				m.queueBoard(entry, len(active), now)
			}
		})
	}
	for _, match := range matches {
		match.world.Do(func(tx *world.Tx) { m.tickMatch(tx, match) })
	}
}

func (m *Manager) tickMatch(tx *world.Tx, match *match) {
	if match.released || !match.joined[0] || !match.joined[1] {
		return
	}
	if match.gone[0] && match.gone[1] || match.phase == ended && time.Since(match.finished) >= 4*time.Second {
		m.release(tx, match)
		return
	}
	if match.phase != ended {
		switch {
		case match.gone[0]:
			m.finish(tx, match, 0, nil, false)
		case match.gone[1]:
			m.finish(tx, match, 1, nil, false)
		case match.phase == running && time.Since(match.started) >= 30*time.Minute:
			loser := rand.IntN(2)
			p, _ := playerIn(tx, match.players[loser])
			m.finish(tx, match, loser, p, false)
		case match.phase == waiting && match.countdown == 0:
			match.phase = running
			match.started = time.Now()
			for _, o := range match.players {
				if p, ok := playerIn(tx, o); ok {
					if match.mode.id == "sumo" {
						p.SetMobile()
						match.mode.giveKit(p)
					}
					p.SendPacket(&packet.SetTitle{ActionType: packet.TitleActionClear})
					p.Message(m.Lang.Text(o.Lang(), "duelStarted"))
				}
			}
		case match.phase == waiting:
			colours := [...]string{"", "§c", "§6", "§e"}
			for _, o := range match.players {
				if p, ok := playerIn(tx, o); ok {
					p.SendTitle(title.New(colours[match.countdown] + fmt.Sprint(match.countdown)).
						WithFadeInDuration(0).WithFadeOutDuration(0).WithDuration(time.Second))
				}
			}
			match.countdown--
		}
	}
	for i, o := range match.players {
		if p, ok := playerIn(tx, o); ok {
			m.updateTag(match, i, p)
			m.matchBoard(match, i)
		}
	}
}

func (m *Manager) finish(tx *world.Tx, match *match, loser int, dead *player.Player, toWinner bool) bool {
	if match.phase == ended || !match.joined[0] || !match.joined[1] {
		return false
	}
	winner := 1 - loser
	won, lost := match.players[winner], match.players[loser]
	victor, ok := playerIn(tx, won)
	if !ok || match.gone[winner] {
		return false
	}
	if dead != nil {
		match.supplies[loser] = remaining(match.mode, dead)
	}
	match.supplies[winner] = remaining(match.mode, victor)
	match.winner = winner
	match.phase = ended
	match.finished = time.Now()
	m.Stats.AddKill(won, 1)
	m.Stats.AddDeath(lost, 1)
	m.Items.ResetPearlCooldown(victor)
	m.Combat.ResetImmunity(won.UUID)
	elapsed := time.Duration(0)
	if !match.started.IsZero() {
		elapsed = match.finished.Sub(match.started)
	}
	key := "duelEndedDefault"
	base := []any{match.mode.display, duration(elapsed), won.Name}
	hp := int(math.Round(victor.Health()))
	var result []any
	switch match.mode.id {
	case "nodebuff", "classic_nodebuff":
		key = "duelEndedNoDebuff"
		result = []any{match.supplies[winner], hp, lost.Name, match.supplies[loser]}
	case "soup":
		key = "duelEndedSoup"
		result = []any{match.supplies[winner], hp, lost.Name, match.supplies[loser]}
	case "boxing":
		key = "duelEndedBoxing"
		result = []any{match.hits[winner], lost.Name, match.hits[loser]}
	case "sumo":
		key = "duelEndedNoDamage"
		result = []any{lost.Name}
	default:
		result = []any{hp, lost.Name}
	}
	for i, o := range match.players {
		if p, ok := playerIn(tx, o); ok && !match.gone[i] {
			p.SetMobile()
			p.SendPacket(&packet.SetTitle{ActionType: packet.TitleActionClear})
			p.SetScoreTag("")
			p.Message(m.Lang.Textf(o.Lang(), key, append(base, result...)...))
			m.matchBoard(match, i)
		}
	}
	if dead != nil {
		m.Combat.DeathAnimation(tx, dead, victor.Position())
	}
	if dead != nil && !match.gone[loser] {
		m.Items.ResetPearlCooldown(dead)
		m.Combat.ResetImmunity(lost.UUID)
		hub.BasicProperties(dead, world.GameModeSpectator)
		pos := dead.Position()
		if toWinner {
			pos = victor.Position()
		}
		pos[1] += 3
		dead.Teleport(pos)
	}
	return true
}

func remaining(mode mode, p *player.Player) int {
	switch mode.id {
	case "nodebuff", "classic_nodebuff":
		return ffa.Potions(p)
	case "soup":
		return ffa.Soups(p)
	default:
		return 0
	}
}

func (m *Manager) release(tx *world.Tx, match *match) {
	if match.released {
		return
	}
	match.released = true
	m.mu.Lock()
	delete(m.matches, match.world)
	m.mu.Unlock()
	for i, o := range match.players {
		if p, ok := playerIn(tx, o); ok && !match.gone[i] {
			m.returnToHub(o, p, tx)
		}
	}
	go match.world.Close()
}

func (m *Manager) returnToHub(o *state.Online, p *player.Player, tx *world.Tx) {
	p.SetMobile()
	m.Items.ResetPearlCooldown(p)
	m.Combat.ResetImmunity(o.UUID)
	worlds.MoveThen(tx, p, m.Hub.World(), m.Hub.Spawn(), hub.Yaw, hub.Pitch, func(tx *world.Tx, p *player.Player) {
		m.Hub.Reset(o, p, tx)
	})
}

func (m *Manager) Close() {
	m.mu.Lock()
	m.closing = true
	matches := make([]*match, 0, len(m.matches))
	for _, match := range m.matches {
		matches = append(matches, match)
	}
	m.mu.Unlock()
	for _, match := range matches {
		task := match.world.Do(func(tx *world.Tx) { m.release(tx, match) })
		_ = task.Wait(context.Background())
		_ = match.world.Close()
	}
}

func (m *Manager) Surrender(_ *state.Online, p *player.Player, tx *world.Tx) bool {
	match, i := m.current(p)
	return match != nil && m.finish(tx, match, i, p, false)
}

func duration(d time.Duration) string {
	total := max(0, int(d/time.Second))
	return fmt.Sprintf("%02d:%02d", total/60, total%60)
}

func (m *Manager) queueBoard(e entry, count int, now time.Time) {
	if !m.Toggles.Get(e.o, toggles.Scoreboard) {
		return
	}
	lines := slices.Clone(m.Lang.Lines(e.o.Lang(), "duelsScoreboardQueue"))
	lines[2] = lang.Sprintf(lines[2], e.mode.display)
	lines[3] = lang.Sprintf(lines[3], duration(now.Sub(e.since)))
	lines[5] = lang.Sprintf(lines[5], count)
	m.Boards.SetLines(e.o.UUID, lines)
}

func (m *Manager) matchBoard(match *match, i int) {
	o, opponent := match.players[i], match.players[1-i]
	if !m.Toggles.Get(o, toggles.Scoreboard) {
		return
	}
	var lines []string
	switch {
	case match.phase == waiting:
		lines = slices.Clone(m.Lang.Lines(o.Lang(), "duelsScoreboardWaiting"))
		lines[2] = lang.Sprintf(lines[2], opponent.Name)
		lines[3] = lang.Sprintf(lines[3], match.mode.display)
	case match.phase == ended:
		lines = slices.Clone(m.Lang.Lines(o.Lang(), "duelsScoreboardEnd"))
		lines[3] = lang.Sprintf(lines[3], match.players[match.winner].Name)
	case match.mode.id == "boxing":
		lines = slices.Clone(m.Lang.Lines(o.Lang(), "duelsScoreboardRunningBoxing"))
		self, other := match.hits[i], match.hits[1-i]
		colour, sign, difference := "§e", "", self-other
		if difference > 0 {
			colour, sign = "§a", "+"
		} else if difference < 0 {
			colour, sign, difference = "§c", "-", -difference
		}
		lines[2] = lang.Sprintf(lines[2], colour, sign, difference)
		lines[3] = lang.Sprintf(lines[3], self)
		lines[4] = lang.Sprintf(lines[4], other)
		lines[6] = lang.Sprintf(lines[6], o.Ping())
		lines[7] = lang.Sprintf(lines[7], opponent.Ping())
	default:
		lines = slices.Clone(m.Lang.Lines(o.Lang(), "duelsScoreboardRunning"))
		lines[2] = lang.Sprintf(lines[2], o.Ping())
		lines[3] = lang.Sprintf(lines[3], opponent.Ping())
	}
	m.Boards.SetLines(o.UUID, lines)
}

func (m *Manager) updateTag(match *match, i int, p *player.Player) {
	p.SetNameTag(m.Chat.NameTag(match.players[i]))
	if match.mode.noDamage() || match.phase == ended {
		p.SetScoreTag("")
		return
	}
	p.SetScoreTag(fmt.Sprintf("§f%d", int(math.Round(p.Health()))))
}
