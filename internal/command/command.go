package command

import (
	"strings"
	"time"

	"github.com/EraseMC/GoPractice/internal/lang"
	"github.com/EraseMC/GoPractice/internal/rank"
	"github.com/EraseMC/GoPractice/internal/state"
	"github.com/EraseMC/GoPractice/internal/text"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

type Kind int

const (
	KindPlayer Kind = iota
	KindConsole
	KindRcon
)

type Sender interface {
	Name() string
	Kind() Kind
	Message(msg string)
	HasPermission(perm string) bool
}

type PlayerSender struct {
	O    *state.Online
	P    *player.Player
	Tx   *world.Tx
	rank func() string
}

func (s *PlayerSender) Name() string                   { return s.O.Name }
func (s *PlayerSender) Kind() Kind                     { return KindPlayer }
func (s *PlayerSender) Message(msg string)             { s.O.Message(msg) }
func (s *PlayerSender) Raw(msg string)                 { s.O.Raw(msg) }
func (s *PlayerSender) Rank() string                   { return s.rank() }
func (s *PlayerSender) Lang() lang.Lang                { return s.O.Lang() }
func (s *PlayerSender) HasPermission(perm string) bool { return rank.Has(s.rank(), perm) }

type ConsoleSender struct {
	Print func(line string)
}

func (s *ConsoleSender) Name() string              { return "CONSOLE" }
func (s *ConsoleSender) Kind() Kind                { return KindConsole }
func (s *ConsoleSender) HasPermission(string) bool { return true }
func (s *ConsoleSender) Message(msg string) {
	for _, line := range strings.Split(text.Clean(msg, true), "\n") {
		s.Print(line)
	}
}

type RconSender struct {
	out strings.Builder
}

func (s *RconSender) Name() string              { return "RCON" }
func (s *RconSender) Kind() Kind                { return KindRcon }
func (s *RconSender) HasPermission(string) bool { return true }
func (s *RconSender) Message(msg string) {
	s.out.WriteString(strings.Trim(msg, "\r\n"))
	s.out.WriteString("\n")
}

func (s *RconSender) Output() string {
	return text.Clean(s.out.String(), true)
}

type Param struct {
	Name     string
	Type     uint32
	Optional bool
	Enum     string
	Values   []string
}

func Target(name string, optional bool) Param {
	return Param{Name: name, Type: protocol.CommandArgTypeTarget, Optional: optional}
}

func RawText(name string) Param {
	return Param{Name: name, Type: protocol.CommandArgTypeRawText}
}

func Enum(name string, values ...string) Param {
	return Param{Name: name, Enum: name, Values: values}
}

type Command struct {
	Name           string
	Aliases        []string
	Permission     string
	Description    string
	DescriptionKey string
	Usage          string
	Overloads      [][]Param
	Vanilla        bool
	Run            func(c *Ctx)
}

func (c *Command) Labels() []string {
	return append([]string{c.Name}, c.Aliases...)
}

type Ctx struct {
	*Services
	Sender Sender
	Player *PlayerSender
	Label  string
	Args   []string
	Cmd    *Command
}

func (c *Ctx) T(key string) string {
	return c.Lang.Text(c.Player.Lang(), key)
}

func (c *Ctx) Tf(key string, args ...any) string {
	return c.Lang.Textf(c.Player.Lang(), key, args...)
}

func (c *Ctx) Reply(msg string) {
	c.Sender.Message(msg)
}

func (c *Ctx) Usage() {
	c.Sender.Message(c.Cmd.Usage)
}

func (c *Ctx) Arg(i int) (string, bool) {
	if i < len(c.Args) {
		return c.Args[i], true
	}
	return "", false
}

func (c *Ctx) TestConsole() bool {
	if c.Sender.Kind() == KindPlayer {
		c.Reply(c.T("commandOnlyConsole"))
		return false
	}
	return true
}

func (c *Ctx) TestRcon() bool {
	if c.Sender.Kind() == KindRcon {
		c.Reply("§cCommand can be executed only from console!")
		return false
	}
	return true
}

func (c *Ctx) With(o *state.Online, f func(tx *world.Tx, p *player.Player)) {
	if c.Player != nil {
		if e, ok := o.Handle.Entity(c.Player.Tx); ok {
			if p, ok := e.(*player.Player); ok {
				f(c.Player.Tx, p)
				return
			}
		}
		o.Do(f)
		return
	}
	done := make(chan struct{})
	o.Do(func(tx *world.Tx, p *player.Player) {
		defer close(done)
		f(tx, p)
	})
	select {
	case <-done:
	case <-time.After(5 * time.Second):
	}
}

func Within(tx *world.Tx, o *state.Online, f func(tx *world.Tx, p *player.Player)) {
	if e, ok := o.Handle.Entity(tx); ok {
		if p, ok := e.(*player.Player); ok {
			f(tx, p)
			return
		}
	}
	o.Do(f)
}

func (c *Ctx) OnlyPlayers() bool {
	if c.Player == nil {
		c.Reply("§cOnly for players!")
		return false
	}
	return true
}
