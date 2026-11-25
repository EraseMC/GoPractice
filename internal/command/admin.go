package command

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/EraseMC/GoPractice/internal/rank"
	"github.com/EraseMC/GoPractice/internal/state"
	"github.com/EraseMC/GoPractice/internal/worlds"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

func GameMode() *Command {
	return &Command{
		Name:           "gamemode",
		Aliases:        []string{"gm"},
		Permission:     rank.CmdGameMode,
		Description:    "Change your game mode",
		DescriptionKey: "gameModeCommandDescription",
		Usage:          "Usage: /gamemode <game mode>",
		Overloads:      [][]Param{{Enum("gamemode", "survival", "creative", "adventure", "spectator", "0", "1", "2", "3")}},
		Run: func(c *Ctx) {
			if !c.OnlyPlayers() {
				return
			}
			if len(c.Args) == 0 {
				c.Reply(c.T("gameModeUsage"))
				return
			}
			o := c.Player.O
			switch {
			case c.check(c.InCombat, o):
				c.Reply(c.T("noGameModeInCombat"))
				return
			case c.check(c.IsSpectator, o):
				c.Reply(c.T("noGameModeInSpectator"))
				return
			case c.check(c.HasRespawnDelay, o):
				c.Reply(c.T("noGameModeInRespawnDelay"))
				return
			}
			var mode world.GameMode
			var key string
			switch strings.ToLower(c.Args[0]) {
			case "survival", "0":
				mode, key = world.GameModeSurvival, "gameModeChangedToSurvival"
			case "creative", "1":
				mode, key = world.GameModeCreative, "gameModeChangedToCreative"
			case "adventure", "2":
				mode, key = world.GameModeAdventure, "gameModeChangedToAdventure"
			case "spectator", "3":
				mode, key = world.GameModeSpectator, "gameModeChangedToSpectator"
			default:
				c.Reply(c.T("unknownGameMode"))
				return
			}
			if c.Player.P.GameMode() == mode {
				c.Reply(c.T("sameGameMode"))
				return
			}
			c.Player.P.SetGameMode(mode)
			c.Reply(c.T(key))
		},
	}
}

const (
	tpUsage  = "Usage: /tp [target player] <destination player> OR /tp [target player] <x> <y> <z> [<y-rot> <x-rot>]"
	minCoord = -30000000
	maxCoord = 30000000
	minY     = -64
	maxY     = 320
)

func (c *Ctx) adminBroadcast(msg string) {
	c.Reply(msg)
	c.Info("[" + c.Sender.Name() + ": " + msg + "]")
}

func (c *Ctx) findPlayer(name string) (*state.Online, bool) {
	o, ok := c.Players.Find(name)
	if !ok {
		c.Reply(`§cCannot find a player matching "` + name + `"`)
	}
	return o, ok
}

func Teleport() *Command {
	return &Command{
		Name:        "tp",
		Aliases:     []string{"teleport"},
		Permission:  rank.CmdTeleport,
		Description: "Teleports the given player (or yourself) to another player or coordinates",
		Usage:       tpUsage,
		Vanilla:     true,
		Run: func(c *Ctx) {
			args := c.Args
			var subjectName string
			self := false
			switch len(args) {
			case 1, 3, 5:
				self = true
			case 2, 4, 6:
				subjectName, args = args[0], args[1:]
			default:
				c.Reply(tpUsage)
				return
			}
			var subject *state.Online
			if !self && subjectName != "@s" {
				o, ok := c.findPlayer(subjectName)
				if !ok {
					return
				}
				subject = o
			} else if c.Player != nil {
				subject = c.Player.O
			} else {
				c.Reply(tpUsage)
				return
			}

			if len(args) == 1 {
				target, ok := c.findPlayer(args[0])
				if !ok {
					return
				}
				var (
					w          *world.World
					pos        mgl64.Vec3
					yaw, pitch float64
				)
				c.With(target, func(tx *world.Tx, p *player.Player) {
					w, pos = tx.World(), p.Position()
					yaw, pitch = p.Rotation().Yaw(), p.Rotation().Pitch()
					Within(tx, subject, func(tx *world.Tx, p *player.Player) {
						worlds.Move(tx, p, w, pos, yaw, pitch)
					})
				})
				c.adminBroadcast("Teleported " + subject.Name + " to " + target.Name)
				return
			}

			c.With(subject, func(tx *world.Tx, p *player.Player) {
				base := p.Position()
				yaw, pitch := p.Rotation().Yaw(), p.Rotation().Pitch()
				if len(args) == 5 {
					yaw, pitch = phpFloatval(args[3]), phpFloatval(args[4])
				}
				x := relative(base.X(), args[0], minCoord, maxCoord)
				y := relative(base.Y(), args[1], minY, maxY)
				z := relative(base.Z(), args[2], minCoord, maxCoord)
				worlds.Move(tx, p, tx.World(), mgl64.Vec3{x, y, z}, yaw, pitch)
				c.adminBroadcast(fmt.Sprintf("Teleported %s to %s, %s, %s", subject.Name, round2(x), round2(y), round2(z)))
			})
		},
	}
}

func relative(original float64, input string, min, max float64) float64 {
	if strings.HasPrefix(input, "~") {
		return original + clamp(phpFloatval(input[1:]), minCoord, maxCoord)
	}
	return clamp(phpFloatval(input), min, max)
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

var leadingFloat = regexp.MustCompile(`^[ \t\n\r\v\f]*[+-]?(\d+(\.\d*)?|\.\d+)([eE][+-]?\d+)?`)

func phpFloatval(s string) float64 {
	m := strings.TrimLeft(leadingFloat.FindString(s), " \t\n\r\v\f")
	f, _ := strconv.ParseFloat(m, 64)
	return f
}

func round2(f float64) string {
	return phpFloat(math.Round(f*100) / 100)
}

func Stop() *Command {
	return &Command{
		Name:        "stop",
		Permission:  rank.CmdStop,
		Description: "Stops the server",
		Usage:       "/stop",
		Vanilla:     true,
		Run: func(c *Ctx) {
			c.adminBroadcast("Stopping the server")
			if c.Services.Stop != nil {
				c.Services.Stop()
			}
		},
	}
}
