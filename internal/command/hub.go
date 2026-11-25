package command

import (
	"github.com/EraseMC/GoPractice/internal/rank"
)

func Spawn() *Command {
	return &Command{
		Name:           "spawn",
		Aliases:        []string{"hub", "lobby"},
		Permission:     rank.CmdSpawn,
		Description:    "Teleport to spawn",
		DescriptionKey: "spawnCommandDescription",
		Usage:          "Usage: /spawn",
		Overloads:      EmptyOverload(),
		Run: func(c *Ctx) {
			if !c.OnlyPlayers() || c.Hub == nil {
				return
			}
			o, p, tx := c.Player.O, c.Player.P, c.Player.Tx
			switch {
			case c.check(c.InFFA, o):
				switch {
				case c.check(c.InCombat, o):
					c.Reply(c.T("noSpawnInCombat"))
				case c.check(c.HasRespawnDelay, o):
					c.Reply(c.T("noSpawnInRespawnDelay"))
				default:
					if c.LeaveFFA != nil {
						c.LeaveFFA(o, p, tx)
					}
					c.Reply(c.T("teleportedToSpawn"))
				}
			case c.check(c.InDuel, o):
				c.Reply(c.T("noSpawnInDuel"))
			case c.check(c.IsSpectator, o):
				c.Reply(c.T("noSpawnInSpectator"))
			case c.check(c.InQueue, o):
				c.Hub.Teleport(p, tx)
				c.Reply(c.T("teleportedToSpawn"))
			default:
				c.Hub.Reset(o, p, tx)
				c.Reply(c.T("teleportedToSpawn"))
			}
		},
	}
}

func formCommand(name, perm, description, key, usage string, aliases []string, send func(c *Ctx)) *Command {
	return &Command{
		Name:           name,
		Aliases:        aliases,
		Permission:     perm,
		Description:    description,
		DescriptionKey: key,
		Usage:          usage,
		Overloads:      EmptyOverload(),
		Run: func(c *Ctx) {
			if !c.OnlyPlayers() || c.Hub == nil {
				return
			}
			send(c)
		},
	}
}

func Settings() *Command {
	return formCommand("settings", rank.CmdSettings, "Open settings", "settingsCommandDescription", "Usage: /settings", []string{"options"}, func(c *Ctx) {
		c.Hub.SendSettings(c.Player.P)
	})
}

func Info() *Command {
	return formCommand("info", rank.CmdInfo, "Information about the server", "infoCommandDescription", "Usage: /info", nil, func(c *Ctx) {
		c.Hub.SendInfo(c.Player.P)
	})
}

func Rules() *Command {
	return formCommand("rules", rank.CmdRules, "Server's rules", "rulesCommandDescription", "Usage: /rules", nil, func(c *Ctx) {
		c.Hub.SendRules(c.Player.P)
	})
}

func Reconnect() *Command {
	return &Command{
		Name:           "reconnect",
		Aliases:        []string{"rejoin"},
		Permission:     rank.CmdReconnect,
		Description:    "Reconnect to the server",
		DescriptionKey: "reconnectCommandDescription",
		Usage:          "Usage: /reconnect",
		Overloads:      EmptyOverload(),
		Run: func(c *Ctx) {
			if !c.OnlyPlayers() {
				return
			}
			c.Reply(c.T("successfulReconnect"))
			if c.TransferAddress == nil {
				return
			}
			p := c.Player.P
			if addr := c.TransferAddress(p); addr != "" {
				_ = p.Transfer(addr)
			}
		},
	}
}
