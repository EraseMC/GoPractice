package netmw

import (
	"sync"
	"sync/atomic"

	"github.com/df-mc/dragonfly/server/session"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const (
	InputModeUndefined = packet.InputModeUndefined
	InputModeMouse     = packet.InputModeMouse
	InputModeTouch     = packet.InputModeTouch
	InputModeGamePad   = packet.InputModeGamePad
	InputModeMotion    = packet.InputModeMotionController
)

type Conn struct {
	session.Conn
	m          *Middleware
	id         uuid.UUID
	protocol   int32
	inputMode  atomic.Uint32
	registered atomic.Bool
	closeOnce  sync.Once

	valuesMu sync.Mutex
	values   map[string]any
}

func newConn(c session.Conn, m *Middleware) *Conn {
	w := &Conn{Conn: c, m: m, protocol: c.Proto().ID(), values: map[string]any{}}
	w.id, _ = uuid.Parse(c.IdentityData().Identity)
	w.inputMode.Store(uint32(c.ClientData().CurrentInputMode))
	return w
}

func (c *Conn) UUID() uuid.UUID {
	return c.id
}

func (c *Conn) Protocol() int32 {
	return c.protocol
}

func (c *Conn) GameVersion() string {
	return c.ClientData().GameVersion
}

func (c *Conn) InputMode() uint32 {
	return c.inputMode.Load()
}

func (c *Conn) Value(key string) (any, bool) {
	c.valuesMu.Lock()
	defer c.valuesMu.Unlock()
	v, ok := c.values[key]
	return v, ok
}

func (c *Conn) SetValue(key string, v any) {
	c.valuesMu.Lock()
	c.values[key] = v
	c.valuesMu.Unlock()
}

func (c *Conn) ReadPacket() (packet.Packet, error) {
	for {
		pk, err := c.Conn.ReadPacket()
		if err != nil {
			return nil, err
		}
		if in, ok := pk.(*packet.PlayerAuthInput); ok && in.InputMode != packet.InputModeUndefined && in.InputMode < packet.InputModeCount {
			c.inputMode.Store(in.InputMode)
		}
		hooks, _ := c.m.hooks()
		pass := true
		for _, h := range hooks {
			if !h(c, pk) {
				pass = false
				break
			}
		}
		if pass {
			return pk, nil
		}
	}
}

func (c *Conn) WritePacket(pk packet.Packet) error {
	_, hooks := c.m.hooks()
	for _, h := range hooks {
		if !h(c, pk) {
			return nil
		}
	}
	return c.Conn.WritePacket(pk)
}

func (c *Conn) Close() error {
	c.closeOnce.Do(func() { c.m.unregister(c) })
	return c.Conn.Close()
}
