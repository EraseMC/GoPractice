package command

import (
	"github.com/EraseMC/GoPractice/internal/duel"
	"github.com/EraseMC/GoPractice/internal/rank"
)

func Duel() *Command {
	mode := Enum("mode", duel.ModeIDs()...)
	mode.Optional = true
	return &Command{
		Name:           "duel",
		Permission:     rank.CmdDuel,
		Description:    "Duel a player",
		DescriptionKey: "duelCommandDescription",
		Usage:          "Usage: /duel invite <player> [mode] or /duel accept [player]",
		Overloads: [][]Param{
			{Enum("duel_invite", "invite"), Target("player", false), mode},
			{Enum("duel_accept", "accept"), Target("player", true)},
		},
		Run: func(c *Ctx) {
			if c.OnlyPlayers() && c.Duel != nil {
				c.Duel(c.Player.O, c.Player.P, c.Player.Tx, c.Args)
			}
		},
	}
}
