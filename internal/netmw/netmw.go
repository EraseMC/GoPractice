package netmw

import (
	"sync"

	"github.com/df-mc/dragonfly/server"
	"github.com/df-mc/dragonfly/server/session"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type Inbound func(c *Conn, pk packet.Packet) bool

type Outbound func(c *Conn, pk packet.Packet) bool

type Middleware struct {
	mu    sync.RWMutex
	in    []Inbound
	out   []Outbound
	conns map[uuid.UUID]*Conn
}

func New() *Middleware {
	return &Middleware{conns: map[uuid.UUID]*Conn{}}
}

func (m *Middleware) OnRead(f Inbound) {
	m.mu.Lock()
	m.in = append(m.in, f)
	m.mu.Unlock()
}

func (m *Middleware) OnWrite(f Outbound) {
	m.mu.Lock()
	m.out = append(m.out, f)
	m.mu.Unlock()
}

func (m *Middleware) Conn(id uuid.UUID) (*Conn, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.conns[id]
	return c, ok
}

func (m *Middleware) Wrap(listeners []func(server.Config) (server.Listener, error)) []func(server.Config) (server.Listener, error) {
	out := make([]func(server.Config) (server.Listener, error), len(listeners))
	for i, f := range listeners {
		out[i] = func(conf server.Config) (server.Listener, error) {
			l, err := f(conf)
			if err != nil {
				return nil, err
			}
			return &listener{Listener: l, m: m}, nil
		}
	}
	return out
}

func (m *Middleware) register(c *Conn) {
	m.mu.Lock()
	if old, ok := m.conns[c.id]; ok && old != c {
		old.registered.Store(false)
	}
	m.conns[c.id] = c
	c.registered.Store(true)
	m.mu.Unlock()
}

func (m *Middleware) unregister(c *Conn) {
	m.mu.Lock()
	if cur, ok := m.conns[c.id]; ok && cur == c {
		delete(m.conns, c.id)
	}
	c.registered.Store(false)
	m.mu.Unlock()
}

func (m *Middleware) hooks() ([]Inbound, []Outbound) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.in, m.out
}

type listener struct {
	server.Listener
	m *Middleware
}

func (l *listener) Accept() (session.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	wrapped := newConn(c, l.m)
	l.m.register(wrapped)
	return wrapped, nil
}

func (l *listener) Disconnect(c session.Conn, reason string) error {
	if w, ok := c.(*Conn); ok {
		l.m.unregister(w)
		return l.Listener.Disconnect(w.Conn, reason)
	}
	return l.Listener.Disconnect(c, reason)
}
