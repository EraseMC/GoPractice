package info

import (
	"github.com/EraseMC/GoPractice/internal/lang"
	"github.com/EraseMC/GoPractice/internal/state"
	"github.com/EraseMC/GoPractice/internal/ui"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
)

type Manager struct {
	bundle  *lang.Bundle
	players *state.Players
	forms   *ui.Forms

	Back func(p *player.Player, tx *world.Tx)
}

func New(bundle *lang.Bundle, players *state.Players, forms *ui.Forms) *Manager {
	return &Manager{bundle: bundle, players: players, forms: forms}
}

func (m *Manager) SendInfoForm(p *player.Player, back bool) {
	m.send(p, "infoForm", back)
}

func (m *Manager) SendRulesForm(p *player.Player, back bool) {
	m.send(p, "rulesForm", back)
}

func (m *Manager) send(p *player.Player, key string, back bool) {
	o, ok := m.players.Get(p.UUID())
	if !ok {
		return
	}
	form := ui.NewSimple(func(p *player.Player, tx *world.Tx, data *int) {
		if back && data != nil && *data == 0 && m.Back != nil {
			m.Back(p, tx)
		}
	})
	contents := m.bundle.Lines(o.Lang(), key)
	form.SetTitle(contents[0])
	form.SetContent(contents[1])
	if back {
		form.AddButton(contents[2], ui.ImagePath, "textures/ui/arrow_left")
	}
	m.forms.Send(p, form)
}
