package command

import (
	"fmt"
	"math"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/EraseMC/GoPractice/internal/device"
	"github.com/EraseMC/GoPractice/internal/lang"
	"github.com/EraseMC/GoPractice/internal/rank"
	"github.com/EraseMC/GoPractice/internal/state"
)

func List() *Command {
	return &Command{
		Name:           "list",
		Aliases:        []string{"online"},
		Permission:     rank.CmdInfo,
		Description:    "Online players list",
		DescriptionKey: "listCommandDescription",
		Usage:          "Usage: /list",
		Overloads:      EmptyOverload(),
		Run: func(c *Ctx) {
			all := c.Players.All()
			names := make([]string, len(all))
			for i, o := range all {
				names[i] = o.Name
			}
			sort.Strings(names)
			if c.Player != nil {
				c.Reply(c.Tf("listSuccess", len(names), strings.Join(names, ", ")))
			} else {
				c.Reply(fmt.Sprintf("§eThere are %d players online:\n%s", len(names), strings.Join(names, ", ")))
			}
		},
	}
}

func Ping() *Command {
	return &Command{
		Name:           "ping",
		Aliases:        []string{"connection"},
		Permission:     rank.CmdPing,
		Description:    "Check your or another player's ping",
		DescriptionKey: "pingCommandDescription",
		Usage:          "Usage: /ping <player>",
		Overloads:      [][]Param{{Target("player", true)}},
		Run: func(c *Ctx) {
			if len(c.Args) == 0 {
				if c.Player != nil {
					c.Reply(c.Tf("pingSelf", c.Player.O.Ping()))
				} else {
					c.Usage()
				}
				return
			}
			target, ok := c.Players.Find(c.Args[0])
			switch {
			case ok && c.Player != nil:
				c.Reply(c.Tf("pingAnother", target.Name, target.Ping()))
			case ok:
				c.Reply(fmt.Sprintf("§a%s's ping: %dms", target.Name, target.Ping()))
			case c.Player != nil:
				c.Reply(c.T("playerNotFound"))
			default:
				c.Reply("§cPlayer not found!")
			}
		},
	}
}

func osName(o *state.Online) string {
	return device.OSName(o.DeviceOS, o.DeviceModel)
}

func Platform() *Command {
	return &Command{
		Name:           "platform",
		Permission:     rank.CmdPlatform,
		Description:    "Check your or another player's platform",
		DescriptionKey: "platformCommandDescription",
		Usage:          "Usage: /platform <player>",
		Overloads:      [][]Param{{Target("player", true)}},
		Run: func(c *Ctx) {
			if len(c.Args) == 0 {
				if c.Player != nil {
					o := c.Player.O
					c.Reply(c.Tf("platformSelf", osName(o), c.Lang.In(c.Player.Lang(), lang.InputMode, device.InputModeName(o.InputMode()))))
				} else {
					c.Usage()
				}
				return
			}
			target, ok := c.Players.Find(c.Args[0])
			switch {
			case ok && c.Player != nil:
				c.Reply(c.Tf("platformAnother", target.Name, osName(target), c.Lang.In(c.Player.Lang(), lang.InputMode, device.InputModeName(target.InputMode()))))
			case ok:
				c.Reply("§a" + target.Name + "'s platform: " + osName(target) + " (" + device.InputModeName(target.InputMode()) + ")")
			case c.Player != nil:
				c.Reply(c.T("playerNotFound"))
			default:
				c.Reply("§cPlayer not found!")
			}
		},
	}
}

func Status() *Command {
	return &Command{
		Name:           "status",
		Permission:     rank.CmdStatus,
		Description:    "Check server's status",
		DescriptionKey: "statusCommandDescription",
		Usage:          "Usage: /status",
		Overloads:      EmptyOverload(),
		Run: func(c *Ctx) {
			st := ServerStatus{TPS: 20, AverageTPS: 20}
			if c.Services.Status != nil {
				st = c.Services.Status()
			}
			format := "§eServer status:\nOnline: %s (%s registered)\nUptime: %s\nCurrent TPS: %s (%sPERCENT)\nAverage TPS: %s (%sPERCENT)\nMemory used: %s MB"
			if c.Player != nil {
				format = c.T("statusSuccess")
			}
			out := lang.Sprintf(format,
				c.Players.Count(),
				c.Store.Count(),
				PrettyUptime(time.Since(c.Started)),
				phpFloat(st.TPS),
				phpFloat(st.TickUsage),
				phpFloat(st.AverageTPS),
				phpFloat(st.AverageTickUsage),
				memoryUsage(),
			)
			c.Reply(strings.ReplaceAll(out, "PERCENT", "%"))
		},
	}
}

func PrettyUptime(d time.Duration) string {
	t := int64(d.Seconds())
	out := strconv.FormatInt(t%60, 10) + " seconds"
	if t < 60 {
		return out
	}
	out = strconv.FormatInt(t%3600/60, 10) + " minutes " + out
	if t < 3600 {
		return out
	}
	out = strconv.FormatInt(t%86400/3600, 10) + " hours " + out
	if t < 86400 {
		return out
	}
	return strconv.FormatInt(t/86400, 10) + " days " + out
}

func phpFloat(f float64) string {
	if f == math.Trunc(f) && math.Abs(f) < 1e15 {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

func memoryUsage() string {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return NumberFormat(math.Round(float64(ms.Sys)/1024/1024*100)/100, 2)
}

func NumberFormat(f float64, decimals int) string {
	s := strconv.FormatFloat(math.Abs(f), 'f', decimals, 64)
	whole, frac, _ := strings.Cut(s, ".")
	var b strings.Builder
	if f < 0 {
		b.WriteByte('-')
	}
	for i, r := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	if decimals > 0 {
		b.WriteByte('.')
		b.WriteString(frac)
	}
	return b.String()
}
