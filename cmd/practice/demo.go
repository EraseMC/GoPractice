package main

import (
	"github.com/EraseMC/GoPractice/internal/app"
	"github.com/EraseMC/GoPractice/internal/lang"
	"github.com/EraseMC/GoPractice/internal/state"
	"github.com/EraseMC/GoPractice/internal/store"
	"github.com/EraseMC/GoPractice/internal/ui"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
)

func installDemo(a *app.App) {
	board := func(o *state.Online) {
		l := o.Lang()
		lines := a.Lang.Lines(l, "hubScoreboard")
		if len(lines) < 9 {
			return
		}
		prof, ok := a.Store.ByXUID(o.XUID)
		if !ok {
			prof = store.Missing
			prof.Rank = store.DefaultRank
		}
		a.Boards.Set(o.UUID, lines[0])
		a.Boards.SetLine(o.UUID, 1, lines[1])
		a.Boards.SetLine(o.UUID, 2, lang.Sprintf(lines[2], prof.Rank))
		a.Boards.SetLine(o.UUID, 3, lang.Sprintf(lines[3], "-"))
		a.Boards.SetLine(o.UUID, 4, lines[4])
		a.Boards.SetLine(o.UUID, 5, lang.Sprintf(lines[5], a.Players.Count()))
		a.Boards.SetLine(o.UUID, 6, lang.Sprintf(lines[6], 0))
		a.Boards.SetLine(o.UUID, 7, lines[7])
		a.Boards.SetLine(o.UUID, 8, lines[8])
	}

	a.Scheduler.Repeat(20, func() {
		for _, o := range a.Players.All() {
			board(o)
		}
	})

	a.Language.OnChange(func(p *player.Player, _ *world.Tx, _ lang.Lang) {
		if o, ok := a.Players.Get(p.UUID()); ok {
			board(o)
		}
	})

	a.OnJoin = append(a.OnJoin, func(o *state.Online, p *player.Player, _ *world.Tx) {
		ui.Message(p, a.Lang.Textf(o.Lang(), "joinMessage", a.Players.Count()))
		board(o)
		a.Scheduler.After(40, func() {
			o.Do(func(_ *world.Tx, p *player.Player) {
				a.Language.SendLanguageForm(p)
			})
		})
	})
}
