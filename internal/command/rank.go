package command

import (
	"fmt"
	"strings"

	"github.com/EraseMC/GoPractice/internal/rank"
	"github.com/EraseMC/GoPractice/internal/state"
	"github.com/EraseMC/GoPractice/internal/store"
)

var sqlFilter = strings.NewReplacer("'", "", "`", "", `"`, "", ";", "")

func FilterSQL(s string) string {
	return sqlFilter.Replace(s)
}

func rconProtected(r string) bool {
	return r == rank.Owner || r == rank.Developer || r == rank.Manager || r == rank.Staff
}

func (s *Services) SetRank(target *state.Online, nickname, r string) {
	apply := func(p *store.Profile) {
		p.Rank = r
		if s.CleanCosmetics != nil {
			s.CleanCosmetics(p)
		}
	}
	if target != nil {
		s.Store.Update(target.XUID, apply)
		if s.RankChanged != nil {
			s.RankChanged(target)
		}
		return
	}
	s.Store.UpdateByNickname(nickname, apply)
}

func (s *Services) rankByNickname(nickname string) string {
	if p, ok := s.Store.ByNickname(nickname); ok {
		return p.Rank
	}
	return rank.Default
}

func Rank() *Command {
	return &Command{
		Name:           "rank",
		Permission:     rank.CmdRank,
		Description:    "Change player's rank",
		DescriptionKey: "rankCommandDescription",
		Usage:          "Usage: /rank <player> <rank>",
		Overloads:      [][]Param{{Target("player", false), Enum("rank", rank.EnumValues...)}},
		Run: func(c *Ctx) {
			if c.Player != nil {
				runRankPlayer(c)
				return
			}
			runRankConsole(c)
		},
	}
}

func (c *Ctx) announceRank(name, r string) {
	c.Info(fmt.Sprintf("§e%s had set %s's rank to %s", c.Sender.Name(), name, r))
	c.Notify(rank.ChannelOwners, func(o *state.Online) string {
		return c.Lang.Textf(o.Lang(), "playerRankChangedNotification", c.Sender.Name(), name, r)
	})
	c.Report("%s had set %s's rank to %s", c.Sender.Name(), name, r)
}

func runRankPlayer(c *Ctx) {
	if len(c.Args) < 2 {
		c.Reply(c.T("rankUsage"))
		return
	}
	target, online := c.Players.ByName(c.Args[0])
	nickname := FilterSQL(strings.ToLower(c.Args[0]))
	r, ok := rank.Parse(c.Args[1])
	if !ok {
		c.Reply(c.T("rankNotFound"))
		return
	}
	senderPriority := rank.Priority(c.Player.Rank())

	if online {
		if !c.Store.RegisteredXUID(target.XUID) {
			c.Reply(c.T("playerNotRegistered"))
			return
		}
		if c.Player.Name() == target.Name {
			c.Reply(c.T("rankSelf"))
			return
		}
		current := c.OnlineRank(target)
		switch {
		case senderPriority <= rank.Priority(current):
			c.Reply(c.T("rankCannotBeChanged"))
			return
		case senderPriority <= rank.Priority(r):
			c.Reply(c.T("rankCannotBeSet"))
			return
		case current == r:
			c.Reply(c.T("rankAlreadySet"))
			return
		}
		c.SetRank(target, "", r)
		c.Reply(c.Tf("rankChanged", target.Name, r))
		target.Message(c.Lang.Textf(target.Lang(), "rankChangedAnother", r))
		c.announceRank(target.Name, r)
		return
	}

	if !c.Store.Registered(nickname) {
		c.Reply(c.T("playerNotRegistered"))
		return
	}
	current := c.rankByNickname(nickname)
	switch {
	case senderPriority <= rank.Priority(current):
		c.Reply(c.T("rankCannotBeChanged"))
		return
	case senderPriority <= rank.Priority(r):
		c.Reply(c.T("rankCannotBeSet"))
		return
	case current == r:
		c.Reply(c.T("rankAlreadySet"))
		return
	}
	c.SetRank(nil, nickname, r)
	c.Reply(c.Tf("rankChanged", nickname, r))
	c.announceRank(nickname, r)
}

func runRankConsole(c *Ctx) {
	if len(c.Args) < 2 || len(c.Args) > 3 {
		c.Usage()
		return
	}
	target, online := c.Players.ByName(c.Args[0])
	nickname := FilterSQL(strings.ToLower(c.Args[0]))
	r, ok := rank.Parse(c.Args[1])
	if !ok {
		c.Reply("§cRank not found!")
		return
	}
	store := len(c.Args) > 2 && strings.ToLower(c.Args[2]) == "store"

	name := nickname
	var current string
	if online {
		if !c.Store.RegisteredXUID(target.XUID) {
			c.Reply("§cPlayer is not registered!")
			return
		}
		name = target.Name
		current = c.OnlineRank(target)
	} else {
		if !c.Store.Registered(nickname) {
			c.Reply("§cPlayer is not registered!")
			return
		}
		current = c.rankByNickname(nickname)
		target = nil
	}

	if c.Sender.Kind() == KindRcon {
		if rconProtected(current) {
			c.Reply("§cYou can't change this player's rank!")
			return
		}
		if rconProtected(r) {
			c.Reply("§cYou can't set this rank!")
			return
		}
		if store && rank.Priority(r) <= rank.Priority(current) {
			c.Reply("§cYou can't change player's rank to this!")
			return
		}
	}
	if current == r {
		c.Reply("§cPlayer already have this rank!")
		return
	}
	c.SetRank(target, nickname, r)
	c.Reply("§a" + name + "'s rank has been changed to " + r)
	if target != nil {
		target.Message(c.Lang.Textf(target.Lang(), "rankChangedAnother", r))
	}
	c.announceRank(name, r)
}
