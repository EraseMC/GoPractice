package state

import (
	"strings"
	"sync"

	"github.com/EraseMC/GoPractice/internal/lang"
	"github.com/EraseMC/GoPractice/internal/netmw"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/google/uuid"
)

type Place int

const (
	PlaceHub Place = iota
	PlaceFFA
	PlaceQueue
	PlaceDuel
	PlaceSpectator
)

type Online struct {
	UUID   uuid.UUID
	XUID   string
	Name   string
	Handle *world.EntityHandle
	Conn   *netmw.Conn

	mu    sync.Mutex
	lang  lang.Lang
	place Place
	arena string
}

func (o *Online) LowerName() string {
	return strings.ToLower(o.Name)
}

func (o *Online) Lang() lang.Lang {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.lang
}

func (o *Online) SetLang(l lang.Lang) {
	o.mu.Lock()
	o.lang = l
	o.mu.Unlock()
}

func (o *Online) Place() (Place, string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.place, o.arena
}

func (o *Online) SetPlace(p Place, arena string) {
	o.mu.Lock()
	o.place, o.arena = p, arena
	o.mu.Unlock()
}

func (o *Online) Do(f func(tx *world.Tx, p *player.Player)) {
	o.Handle.Do(func(tx *world.Tx, e world.Entity) {
		if p, ok := e.(*player.Player); ok {
			f(tx, p)
		}
	})
}

type Players struct {
	mu     sync.RWMutex
	byUUID map[uuid.UUID]*Online
	byName map[string]*Online
	order  []*Online
}

func NewPlayers() *Players {
	return &Players{byUUID: map[uuid.UUID]*Online{}, byName: map[string]*Online{}}
}

func (s *Players) Add(p *player.Player, conn *netmw.Conn, l lang.Lang) *Online {
	o := &Online{UUID: p.UUID(), XUID: p.XUID(), Name: p.Name(), Handle: p.H(), Conn: conn, lang: l}
	s.mu.Lock()
	if old, ok := s.byUUID[o.UUID]; ok {
		s.removeLocked(old)
	}
	s.byUUID[o.UUID] = o
	s.byName[o.LowerName()] = o
	s.order = append(s.order, o)
	s.mu.Unlock()
	return o
}

func (s *Players) Remove(id uuid.UUID) (*Online, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, ok := s.byUUID[id]
	if ok {
		s.removeLocked(o)
	}
	return o, ok
}

func (s *Players) removeLocked(o *Online) {
	delete(s.byUUID, o.UUID)
	if s.byName[o.LowerName()] == o {
		delete(s.byName, o.LowerName())
	}
	for i, x := range s.order {
		if x == o {
			s.order = append(s.order[:i], s.order[i+1:]...)
			break
		}
	}
}

func (s *Players) Get(id uuid.UUID) (*Online, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	o, ok := s.byUUID[id]
	return o, ok
}

func (s *Players) ByName(name string) (*Online, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	o, ok := s.byName[strings.ToLower(name)]
	return o, ok
}

func (s *Players) Find(prefix string) (*Online, bool) {
	prefix = strings.ToLower(prefix)
	s.mu.RLock()
	defer s.mu.RUnlock()
	if o, ok := s.byName[prefix]; ok {
		return o, true
	}
	var found *Online
	delta := int(^uint(0) >> 1)
	for _, o := range s.order {
		name := o.LowerName()
		if strings.HasPrefix(name, prefix) {
			if d := len(name) - len(prefix); d < delta {
				found, delta = o, d
				if d == 0 {
					break
				}
			}
		}
	}
	return found, found != nil
}

func (s *Players) All() []*Online {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Online, len(s.order))
	copy(out, s.order)
	return out
}

func (s *Players) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.order)
}
