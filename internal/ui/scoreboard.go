package ui

import (
	"sort"
	"sync"

	"github.com/EraseMC/GoPractice/internal/state"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const (
	objectiveName = "objective"
	criteriaName  = "dummy"
	minLine       = 1
	maxLine       = 15
)

type board struct {
	title    string
	shown    *string
	lines    map[int]string
	rendered map[int]string
}

type Scoreboards struct {
	players *state.Players
	mu      sync.Mutex
	boards  map[uuid.UUID]*board
}

func NewScoreboards(players *state.Players) *Scoreboards {
	return &Scoreboards{players: players, boards: map[uuid.UUID]*board{}}
}

func (s *Scoreboards) Set(id uuid.UUID, title string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.boards[id]
	if !ok {
		b = &board{rendered: map[int]string{}}
		s.boards[id] = b
	}
	b.title = title
	b.lines = map[int]string{}
}

func (s *Scoreboards) SetLine(id uuid.UUID, line int, text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.boards[id]
	if !ok || line < minLine || line > maxLine {
		return
	}
	b.lines[line] = text
}

func (s *Scoreboards) SetLines(id uuid.UUID, lines []string) {
	if len(lines) == 0 {
		return
	}
	s.Set(id, lines[0])
	for i := 1; i < len(lines); i++ {
		s.SetLine(id, i, lines[i])
	}
}

func (s *Scoreboards) Has(id uuid.UUID) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.boards[id]
	return ok
}

func (s *Scoreboards) Remove(p *player.Player) {
	s.mu.Lock()
	b, ok := s.boards[p.UUID()]
	delete(s.boards, p.UUID())
	s.mu.Unlock()
	if ok && b.shown != nil {
		p.SendPacket(&packet.RemoveObjective{ObjectiveName: objectiveName})
	}
}

func (s *Scoreboards) Forget(id uuid.UUID) {
	s.mu.Lock()
	delete(s.boards, id)
	s.mu.Unlock()
}

func (s *Scoreboards) Flush() {
	type batch struct {
		id      uuid.UUID
		packets []packet.Packet
	}
	var out []batch
	s.mu.Lock()
	for id, b := range s.boards {
		if _, ok := s.players.Get(id); !ok {
			delete(s.boards, id)
			continue
		}
		if pks := b.diff(); len(pks) > 0 {
			out = append(out, batch{id, pks})
		}
	}
	s.mu.Unlock()

	for _, bt := range out {
		o, ok := s.players.Get(bt.id)
		if !ok {
			continue
		}
		pks := bt.packets
		o.Do(func(_ *world.Tx, p *player.Player) {
			for _, pk := range pks {
				p.SendPacket(pk)
			}
		})
	}
}

func (b *board) diff() []packet.Packet {
	var pks []packet.Packet
	if b.shown == nil || *b.shown != b.title {
		if b.shown != nil {
			pks = append(pks, &packet.RemoveObjective{ObjectiveName: objectiveName})
		}
		pks = append(pks, &packet.SetDisplayObjective{
			DisplaySlot:   packet.ScoreboardSlotSidebar,
			ObjectiveName: objectiveName,
			DisplayName:   b.title,
			CriteriaName:  criteriaName,
			SortOrder:     packet.ScoreboardSortOrderAscending,
		})
		title := b.title
		b.shown = &title
		b.rendered = map[int]string{}
	}

	lines := unique(b.lines)
	var removed, changed []protocol.ScoreboardEntry
	for _, line := range sortedKeys(b.rendered) {
		text := b.rendered[line]
		if cur, ok := lines[line]; !ok || cur != text {
			removed = append(removed, entry(line, text, protocol.ScoreboardIdentityRemove))
		}
	}
	for _, line := range sortedKeys(lines) {
		text := lines[line]
		if prev, ok := b.rendered[line]; !ok || prev != text {
			changed = append(changed, entry(line, text, protocol.ScoreboardIdentityFakePlayer))
		}
	}
	if len(removed) > 0 {
		pks = append(pks, &packet.SetScore{Entries: removed})
	}
	if len(changed) > 0 {
		pks = append(pks, &packet.SetScore{Entries: changed})
	}
	b.rendered = lines
	return pks
}

func unique(lines map[int]string) map[int]string {
	out := make(map[int]string, len(lines))
	seen := map[string]bool{}
	for _, line := range sortedKeys(lines) {
		text := lines[line]
		for seen[text] {
			text += "§r"
		}
		seen[text] = true
		out[line] = text
	}
	return out
}

func sortedKeys(m map[int]string) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}

func entry(line int, text string, identity byte) protocol.ScoreboardEntry {
	return protocol.ScoreboardEntry{
		EntryID:       int64(line),
		ObjectiveName: objectiveName,
		Score:         int32(line),
		IdentityType:  identity,
		DisplayName:   text,
	}
}
