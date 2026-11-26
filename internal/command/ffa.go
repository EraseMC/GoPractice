package command

import (
	"github.com/EraseMC/GoPractice/internal/rank"
)

func ReKit() *Command {
	return &Command{
		Name:           "rekit",
		Permission:     rank.CmdReKit,
		Description:    "Get a new kit",
		DescriptionKey: "reKitCommandDescription",
		Usage:          "Usage: /rekit",
		Overloads:      EmptyOverload(),
		Run: func(c *Ctx) {
			if !c.OnlyPlayers() {
				return
			}
			o, p, tx := c.Player.O, c.Player.P, c.Player.Tx
			if !c.check(c.InFFA, o) {
				c.Reply(c.T("notInFFA"))
				return
			}
			switch {
			case c.check(c.InCombat, o):
				c.Reply(c.T("noReKitInCombat"))
			case c.check(c.HasRespawnDelay, o):
				c.Reply(c.T("noReKitInRespawnDelay"))
			default:
				if c.Rekit != nil {
					c.Rekit(o, p, tx)
				}
				c.Reply(c.T("successfulReKit"))
			}
		},
	}
}

func Surrender() *Command {
	return &Command{
		Name:           "surrender",
		Permission:     rank.CmdSurrender,
		Description:    "Leave from a fight",
		DescriptionKey: "surrenderCommandDescription",
		Usage:          "Usage: /surrender",
		Overloads:      EmptyOverload(),
		Run: func(c *Ctx) {
			if !c.OnlyPlayers() {
				return
			}
			o, p, tx := c.Player.O, c.Player.P, c.Player.Tx
			switch {
			case c.check(c.InFFA, o):
				if c.SurrenderFFA != nil && c.SurrenderFFA(o, p, tx) {
					c.Reply(c.T("successfulSurrender"))
					return
				}
				c.Reply(c.T("notInCombat"))
			case c.check(c.InDuel, o):
				if c.SurrenderDuel != nil && c.SurrenderDuel(o, p, tx) {
					c.Reply(c.T("successfulSurrender"))
				}
			default:
				c.Reply(c.T("notInFFAOrDuel"))
			}
		},
	}
}
