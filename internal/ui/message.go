package ui

import (
	"time"

	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/player/title"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const tick = 50 * time.Millisecond

func Message(p *player.Player, msg string) {
	p.SendPacket(&packet.Text{TextType: packet.TextTypeSystem, Message: msg})
}

func Raw(p *player.Player, msg string) {
	p.Message(msg)
}

func ActionBar(p *player.Player, msg string) {
	p.SendPacket(&packet.SetTitle{ActionType: packet.TitleActionSetActionBar, Text: msg})
}

func Title(p *player.Player, text, subtitle string) {
	TitleTimed(p, text, subtitle, 10, 18, 10)
}

func TitleTimed(p *player.Player, text, subtitle string, fadeIn, stay, fadeOut int) {
	t := title.New(text).
		WithFadeInDuration(time.Duration(fadeIn) * tick).
		WithDuration(time.Duration(stay) * tick).
		WithFadeOutDuration(time.Duration(fadeOut) * tick)
	if subtitle != "" {
		t = t.WithSubtitle(subtitle)
	}
	p.SendTitle(t)
}
