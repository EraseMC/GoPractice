package app

import (
	"bufio"
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/EraseMC/GoPractice/internal/chat"
	"github.com/EraseMC/GoPractice/internal/command"
	"github.com/EraseMC/GoPractice/internal/lang"
	"github.com/EraseMC/GoPractice/internal/netmw"
	"github.com/EraseMC/GoPractice/internal/rank"
	"github.com/EraseMC/GoPractice/internal/rcon"
	"github.com/EraseMC/GoPractice/internal/state"
	"github.com/EraseMC/GoPractice/internal/text"
	"github.com/EraseMC/GoPractice/internal/toggles"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func (a *App) setupCommands() {
	svc := &command.Services{
		Log:      a.Log,
		Lang:     a.Lang,
		Store:    a.Store,
		Players:  a.Players,
		Chat:     a.Chat,
		Punish:   a.Punish,
		Telegram: a.Telegram,
		Started:  time.Now(),
		Status:   a.serverStatus,
		Stop: func() {
			if a.stop != nil {
				a.stop()
			}
		},
		RankChanged:     a.rankChanged,
		CleanCosmetics:  a.Cosmetics.NormalizeProfile,
		Hub:             hubCommands{a},
		InFFA:           a.FFA.InFFA,
		InCombat:        a.FFA.InCombat,
		HasRespawnDelay: a.FFA.HasRespawnDelay,
		LeaveFFA:        a.FFA.RemoveFromFFA,
		Rekit:           func(o *state.Online, p *player.Player, _ *world.Tx) { a.FFA.ReKit(o, p) },
		SurrenderFFA:    a.FFA.Surrender,
		InDuel:          func(o *state.Online) bool { return o.In(state.PlaceDuel) },
		InQueue:         a.Duel.InQueue,
		SurrenderDuel:   a.Duel.Surrender,
		Duel:            a.Duel.Command,
		TransferAddress: a.TransferAddress,
	}
	a.Commands = command.NewMap(svc)
	a.Commands.Register(
		command.ClearChat(),
		command.Emoji(),
		command.GameMode(),
		command.List(),
		command.Ping(),
		command.Platform(),
		command.Rank(),
		command.Reply(),
		command.Say(),
		command.Status(),
		command.Tell(),
		command.ToggleChat(),
		command.Teleport(),
		command.Stop(),
		command.Spawn(),
		command.Settings(),
		command.Info(),
		command.Rules(),
		command.Reconnect(),
		command.ReKit(),
		command.Surrender(),
		command.Duel(),
	)

	a.Net.OnRead(func(c *netmw.Conn, pk packet.Packet) bool {
		req, ok := pk.(*packet.CommandRequest)
		if !ok {
			return true
		}
		c.Inject(&packet.Text{TextType: packet.TextTypeChat, Message: req.CommandLine})
		return false
	})
	a.Net.OnWrite(func(c *netmw.Conn, pk packet.Packet) bool {
		if ac, ok := pk.(*packet.AvailableCommands); ok {
			*ac = *a.availableCommands(c)
		}
		return true
	})
	a.Router.OnChat(a.handleChat)
	a.Language.OnChange(func(p *player.Player, _ *world.Tx, _ lang.Lang) {
		a.SyncCommands(p)
	})
}

func (a *App) availableCommands(c *netmw.Conn) *packet.AvailableCommands {
	xuid := c.IdentityData().XUID
	o, online := a.Players.Get(c.UUID())
	if online {
		xuid = o.XUID
	}
	r := rank.Default
	l := lang.FromLocale(c.ClientData().LanguageCode)
	if prof, ok := a.Store.ByXUID(xuid); ok {
		r = prof.Rank
		if pl, ok := lang.Parse(prof.Locale); ok {
			l = pl
		}
	}
	if online {
		l = o.Lang()
	}
	return a.Commands.Packet(l, func(perm string) bool { return rank.Has(r, perm) })
}

func (a *App) SyncCommands(p *player.Player) {
	p.SendPacket(&packet.AvailableCommands{})
}

func (a *App) rankChanged(o *state.Online) {
	o.Do(func(_ *world.Tx, p *player.Player) {
		a.Cosmetics.RankChanged(o, p)
		if a.Hub.InHub(o) {
			a.Hub.UpdateNameTag(o, p)
		} else {
			p.SetNameTag(a.Chat.NameTag(o))
		}
		a.SyncCommands(p)
	})
	for _, f := range a.OnRankChange {
		f(o)
	}
}

func (a *App) handleChat(ctx *player.Context, message *string) {
	ctx.Cancel()
	p := ctx.Player()
	o, ok := a.Players.Get(p.UUID())
	if !ok {
		return
	}
	left := a.Chat.Take(o.UUID)
	if left <= 0 || len(*message) > left*(chat.MaxByteLength+1) {
		return
	}
	msg := text.Clean(*message, false)
	for _, part := range strings.SplitN(msg, "\n", left+1) {
		if text.Trim(part) == "" || len(part) > chat.MaxByteLength || text.Len(part) > chat.MaxCharLength || !a.Chat.Consume(o.UUID) {
			continue
		}
		if strings.HasPrefix(part, "./") {
			part = part[1:]
		}
		if strings.HasPrefix(part, "/") {
			a.Commands.Dispatch(a.Commands.Player(o, p, p.Tx()), part[1:])
			continue
		}
		a.Chat.Send(o, p, part)
	}
}

func (a *App) Console(r io.Reader) {
	sender := &command.ConsoleSender{Print: func(line string) { a.Log.Info(line) }}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := text.Trim(sc.Text())
		if line == "" {
			continue
		}
		a.Commands.Dispatch(sender, line)
	}
}

func (a *App) startRcon() {
	password := a.Env.Get("OLDTRALIA_RCON_PASSWORD", "")
	if password == "" {
		return
	}
	addr := "0.0.0.0:" + strconv.Itoa(a.Env.Int("OLDTRALIA_RCON_PORT", 25575))
	a.Log.Info("RCON running on " + addr)
	srv, err := rcon.Listen(a.Log, addr, password, func(line string) string {
		sender := &command.RconSender{}
		a.Commands.Dispatch(sender, line)
		return sender.Output()
	})
	if err != nil {
		a.Log.Error("Failed to start RCON: " + err.Error())
		return
	}
	a.rcon = srv
}

func (a *App) updateNameTags() {
	for _, o := range a.Players.All() {
		place, _ := o.Place()
		o.Do(func(_ *world.Tx, p *player.Player) {
			switch place {
			case state.PlaceHub:
				tag := a.Chat.NameTag(o)
				if a.Chat.HasTag(o) {
					tag += "\n" + a.Chat.TagColored(o)
				}
				p.SetNameTag(tag)
				p.SetScoreTag("")
			case state.PlaceFFA:
				a.FFA.UpdateNameTag(o, p)
			case state.PlaceDuel:
			default:
				p.SetNameTag(a.Chat.NameTag(o))
				p.SetScoreTag("")
			}
		})
	}
}

type hubCommands struct {
	a *App
}

func (h hubCommands) Reset(o *state.Online, p *player.Player, tx *world.Tx) {
	h.a.Hub.Reset(o, p, tx)
}

func (h hubCommands) Teleport(p *player.Player, tx *world.Tx) {
	h.a.Hub.Teleport(p, tx)
}

func (h hubCommands) SendSettings(p *player.Player) {
	h.a.Toggles.SendForm(p, toggles.NoBack)
}

func (h hubCommands) SendInfo(p *player.Player) {
	h.a.Info.SendInfoForm(p, false)
}

func (h hubCommands) SendRules(p *player.Player) {
	h.a.Info.SendRulesForm(p, false)
}

func (a *App) serverStatus() command.ServerStatus {
	s := a.Scheduler.Stats()
	st := command.ServerStatus{TPS: s.TPS, TickUsage: s.TickUsage, AverageTPS: s.AverageTPS, AverageTickUsage: s.AverageTickUsage}
	ws := []*world.World{a.Hub.World()}
	for _, ar := range a.FFA.List() {
		ws = append(ws, ar.World())
	}
	for _, w := range ws {
		if w == nil {
			continue
		}
		st.TPS = min(st.TPS, round2(w.TicksPerSecond()))
		st.TickUsage = max(st.TickUsage, round2(w.TickUsage()))
		st.AverageTPS = min(st.AverageTPS, round2(w.TicksPerSecondAverage()))
		st.AverageTickUsage = max(st.AverageTickUsage, round2(w.TickUsageAverage()))
	}
	return st
}

func round2(f float64) float64 {
	return math.Round(f*100) / 100
}
