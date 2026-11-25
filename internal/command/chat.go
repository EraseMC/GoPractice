package command

import (
	"fmt"
	"strings"

	"github.com/EraseMC/GoPractice/internal/chat"
	"github.com/EraseMC/GoPractice/internal/rank"
	"github.com/EraseMC/GoPractice/internal/state"
	"github.com/EraseMC/GoPractice/internal/telegram"
	"github.com/EraseMC/GoPractice/internal/text"
)

func (s *Services) Info(msg string) {
	s.Log.Info(text.Clean(msg, true))
}

func (s *Services) Notify(perm string, msg func(o *state.Online) string) {
	for _, o := range s.Players.All() {
		if s.Has(o, perm) {
			o.Message(msg(o))
		}
	}
}

func (s *Services) Report(format string, args ...any) {
	s.Telegram.Send(fmt.Sprintf("[%s] "+format, append([]any{telegram.Date()}, args...)...))
}

func Tell() *Command {
	return &Command{
		Name:           "tell",
		Aliases:        []string{"whisper", "message", "msg", "w", "m"},
		Permission:     rank.CmdTell,
		Description:    "Send a message to player",
		DescriptionKey: "tellCommandDescription",
		Usage:          "Usage: /tell <player> <message>",
		Overloads:      [][]Param{{Target("player", false), RawText("message")}},
		Run: func(c *Ctx) {
			if len(c.Args) < 2 {
				if c.Player != nil {
					c.Reply(c.T("tellUsage"))
				} else {
					c.Usage()
				}
				return
			}
			nickname := c.Args[0]
			message := strings.Join(c.Args[1:], " ")
			target, found := c.Players.Find(nickname)
			if !found {
				if c.Player != nil {
					if c.Punish.Muted(c.Player.Name()) {
						c.Reply(c.Punish.MuteMessage(c.Player.Lang(), c.Player.Name(), c.Player.O.XUID))
						return
					}
					c.Reply(c.T("playerNotFound"))
				} else {
					c.Reply("§cPlayer not found!")
				}
				return
			}
			if c.Player != nil {
				if c.Punish.Muted(c.Player.Name()) {
					c.Reply(c.Punish.MuteMessage(c.Player.Lang(), c.Player.Name(), c.Player.O.XUID))
					return
				}
				if c.Player.Name() == target.Name {
					c.Reply(c.T("tellSelfError"))
					return
				}
				if text.Len(message) > chat.MessageLengthLimit && c.Player.Rank() != rank.Owner {
					c.Reply(c.T("chatLongMessage"))
					return
				}
				c.Reply(c.Tf("tellSelf", target.Name, message))
				target.Raw(c.Lang.Textf(target.Lang(), "tellAnother", c.Sender.Name(), message))
				c.Chat.SetReply(target.Name, c.Sender.Name())
			} else {
				c.Reply("§eYour message to " + target.Name + ": " + message)
				target.Raw(c.Lang.Textf(target.Lang(), "tellAnother", c.Sender.Name(), message))
			}
			c.Info(fmt.Sprintf("%s to %s: %s", c.Sender.Name(), target.Name, message))
		},
	}
}

func Reply() *Command {
	return &Command{
		Name:           "reply",
		Aliases:        []string{"r"},
		Permission:     rank.CmdReply,
		Description:    "Reply to player's message",
		DescriptionKey: "replyCommandDescription",
		Usage:          "Usage: /reply <message>",
		Overloads:      [][]Param{{RawText("message")}},
		Run: func(c *Ctx) {
			if !c.OnlyPlayers() {
				return
			}
			if len(c.Args) == 0 {
				c.Reply(c.T("replyUsage"))
				return
			}
			target, ok := c.Chat.ReplyTarget(c.Player.Name())
			message := strings.Join(c.Args, " ")
			if !ok {
				c.Reply(c.T("replyNoOneTo"))
				return
			}
			if c.Punish.Muted(c.Player.Name()) {
				c.Reply(c.Punish.MuteMessage(c.Player.Lang(), c.Player.Name(), c.Player.O.XUID))
				return
			}
			if text.Len(message) > chat.MessageLengthLimit && c.Player.Rank() != rank.Owner {
				c.Reply(c.T("chatLongMessage"))
				return
			}
			c.Chat.SetReply(target.Name, c.Player.Name())
			c.Reply(c.Tf("replySelf", target.Name, message))
			target.Raw(c.Lang.Textf(target.Lang(), "replyAnother", c.Player.Name(), message))
			c.Info(fmt.Sprintf("%s to %s: %s", c.Player.Name(), target.Name, message))
		},
	}
}

func ToggleChat() *Command {
	return &Command{
		Name:           "togglechat",
		Permission:     rank.CmdToggleChat,
		Description:    "Enable or disable the chat",
		DescriptionKey: "toggleChatCommandDescription",
		Usage:          "Usage: /togglechat",
		Overloads:      EmptyOverload(),
		Run: func(c *Ctx) {
			enable := !c.Chat.Enabled()
			c.Chat.SetEnabled(enable)
			now, notification, success, done, english := "chatNowDisabled", "disabledChatNotification", "disableChatSuccess", "disabled", "§aYou have disabled the chat!"
			if enable {
				now, notification, success, done, english = "chatNowEnabled", "enabledChatNotification", "enableChatSuccess", "enabled", "§aYou have enabled the chat!"
			}
			for _, o := range c.Players.All() {
				l := o.Lang()
				o.Message(c.Lang.Text(l, now))
				if c.Has(o, rank.ChannelOwners) {
					o.Message(c.Lang.Textf(l, notification, c.Sender.Name()))
				}
			}
			if c.Player != nil {
				c.Reply(c.T(success))
			} else {
				c.Reply(english)
			}
			c.Info(fmt.Sprintf("%s has %s the chat", c.Sender.Name(), done))
			c.Report("%s has %s the chat", c.Sender.Name(), done)
		},
	}
}

func ClearChat() *Command {
	return &Command{
		Name:           "clearchat",
		Permission:     rank.CmdClearChat,
		Description:    "Clear the chat",
		DescriptionKey: "clearChatCommandDescription",
		Usage:          "Usage: /clearchat",
		Overloads:      EmptyOverload(),
		Run: func(c *Ctx) {
			for _, o := range c.Players.All() {
				l := o.Lang()
				for i := 0; i <= 100; i++ {
					o.Message("\n")
				}
				o.Message(c.Lang.Text(l, "clearedChat"))
				if c.Has(o, rank.ChannelAdministrative) {
					o.Message(c.Lang.Textf(l, "clearedChatNotification", c.Sender.Name()))
				}
			}
			if c.Player != nil {
				c.Reply(c.T("clearChatSuccess"))
			} else {
				c.Reply("§aYou have cleared the chat!")
			}
			c.Info(fmt.Sprintf("%s has cleared the chat", c.Sender.Name()))
			c.Report("%s has cleared the chat", c.Sender.Name())
		},
	}
}

func Emoji() *Command {
	return &Command{
		Name:           "emoji",
		Permission:     rank.CmdEmoji,
		Description:    "Emoji list",
		DescriptionKey: "emojiCommandDescription",
		Usage:          "Usage: /emoji",
		Overloads:      EmptyOverload(),
		Run: func(c *Ctx) {
			if !c.OnlyPlayers() {
				return
			}
			c.Reply(c.T("emojiSuccess"))
		},
	}
}

func Say() *Command {
	return &Command{
		Name:           "say",
		Permission:     rank.CmdSay,
		Description:    "Broadcast a message",
		DescriptionKey: "sayCommandDescription",
		Usage:          "Usage: /say <message>",
		Overloads:      [][]Param{{RawText("message")}},
		Run: func(c *Ctx) {
			if !c.TestConsole() || !c.TestRcon() {
				return
			}
			if len(c.Args) == 0 {
				c.Usage()
				return
			}
			message := "§cSERVER: §e" + strings.Join(c.Args, " ")
			c.Reply(message)
			for _, o := range c.Players.All() {
				o.Raw(message)
			}
		},
	}
}
