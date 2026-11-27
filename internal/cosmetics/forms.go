package cosmetics

import (
	"github.com/EraseMC/GoPractice/internal/lang"
	"github.com/EraseMC/GoPractice/internal/rank"
	"github.com/EraseMC/GoPractice/internal/state"
	"github.com/EraseMC/GoPractice/internal/store"
	"github.com/EraseMC/GoPractice/internal/tags"
	"github.com/EraseMC/GoPractice/internal/ui"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
)

const backIcon = "textures/ui/arrow_left"
const removeIcon = "textures/blocks/barrier"

func (m *Manager) form(o *state.Online, p *player.Player, key, content string, icons []string, selectButton func(*player.Player, *world.Tx, int)) {
	lines := m.Lang.Lines(o.Lang(), key)
	start := 1
	form := ui.NewSimple(func(p *player.Player, tx *world.Tx, choice *int) {
		if choice != nil && *choice >= 0 && *choice < len(lines)-start && m.Hub.InHub(o) && tx.World() == m.Hub.World() {
			selectButton(p, tx, *choice)
		}
	})
	form.SetTitle(lines[0])
	if content != "" {
		form.SetContent(lang.Sprintf(lines[1], content))
		start++
	}
	for i, text := range lines[start:] {
		if i < len(icons) {
			form.AddButton(text, ui.ImagePath, icons[i])
		} else {
			form.AddButton(text, ui.ImageNone, "")
		}
	}
	m.Forms.Send(p, form)
}

type access struct {
	perm, denied string
}

func (m *Manager) allow(o *state.Online, access access) bool {
	if rank.Has(m.profile(o).Rank, access.perm) {
		return true
	}
	m.message(o, access.denied)
	return false
}

var cosmeticsAccess = access{rank.CosmeticsDefault, "cosmeticsNoPermission"}
var tagAccess = access{rank.CosmeticsTagsSilver, "tagsNoPermission"}

func (m *Manager) SendForm(o *state.Online, p *player.Player, tx *world.Tx) {
	if !m.Hub.InHub(o) || tx.World() != m.Hub.World() || !m.allow(o, cosmeticsAccess) {
		return
	}
	m.form(o, p, "cosmeticsForm", "", nil, func(p *player.Player, tx *world.Tx, choice int) {
		if !m.allow(o, cosmeticsAccess) {
			return
		}
		switch choice {
		case 0:
			m.choiceMenu(o, p, capeSetting)
		case 1:
			m.tagMenu(o, p)
		case 2:
			m.toggleForm(o, p, flySetting)
		case 3:
			m.choiceMenu(o, p, potionSetting)
		case 4:
			m.toggleForm(o, p, petSetting)
		case 5:
			m.Hub.SendPreferencesForm(p)
		}
	})
}

type choiceSetting struct {
	access
	key, menu, list, image, absent string
	values                         []string
	name, coloured                 lang.Section
	field                          func(*store.Cosmetics) *string
}

var capeSetting = choiceSetting{
	access: access{rank.CosmeticsCapes, "capesNoPermission"},
	key:    "cape", menu: "capesForm", list: "capesSelectionForm", image: "textures/items/banner_pattern", absent: "capeDoesNotSet",
	values: capes, name: lang.Cape, coloured: lang.CapeColored,
	field: func(c *store.Cosmetics) *string { return &c.Cape },
}

var potionSetting = choiceSetting{
	access: access{rank.CosmeticsPotionColor, "potionColorNoPermission"},
	key:    "potionColor", menu: "potionColorForm", list: "potionColorSelectionForm", image: "textures/items/potion_bottle_splash_heal", absent: "potionColorAlreadyDisabled",
	values: colourNames, name: lang.PotionColor, coloured: lang.PotionColorColored,
	field: func(c *store.Cosmetics) *string { return &c.PotionColor },
}

var tagSetting = choiceSetting{
	key: "tag", absent: "tagDoesNotSet",
	field: func(c *store.Cosmetics) *string { return &c.Tag },
}

type tagTier struct {
	access
	list   string
	values []string
}

var tagTiers = []tagTier{
	{tagAccess, "silverTagsForm", tags.Silver},
	{access{rank.CosmeticsTagsGolden, "goldenTagsNoPermission"}, "goldenTagsForm", tags.Golden},
	{access{rank.CosmeticsTagsPlatinum, "platinumTagsNoPermission"}, "platinumTagsForm", tags.Platinum},
}

func (m *Manager) choiceMenu(o *state.Online, p *player.Player, s choiceSetting) {
	if !m.allow(o, s.access) {
		return
	}
	profile := m.profile(o)
	content := m.Lang.In(o.Lang(), s.coloured, *s.field(&profile.Cosmetics))
	m.form(o, p, s.menu, content, []string{s.image, removeIcon, backIcon}, func(p *player.Player, tx *world.Tx, choice int) {
		if !m.allow(o, s.access) {
			return
		}
		switch choice {
		case 0:
			m.choiceList(o, p, s, func(p *player.Player) { m.choiceMenu(o, p, s) })
		case 1:
			m.saveChoice(o, p, s, "")
		case 2:
			m.SendForm(o, p, tx)
		}
	})
}

func (m *Manager) tagMenu(o *state.Online, p *player.Player) {
	if !m.allow(o, tagAccess) {
		return
	}
	content := tags.ColoredForForm(m.profile(o).Cosmetics.Tag)
	icons := []string{"textures/items/iron_ingot", "textures/items/gold_ingot", "textures/items/netherite_ingot", removeIcon, backIcon}
	m.form(o, p, "tagsForm", content, icons, func(p *player.Player, tx *world.Tx, choice int) {
		if !m.allow(o, tagAccess) {
			return
		}
		s := tagSetting
		switch choice {
		case 3:
			s.access = tagAccess
			m.saveChoice(o, p, s, "")
		case 4:
			m.SendForm(o, p, tx)
		default:
			tier := tagTiers[choice]
			s.access, s.list, s.values = tier.access, tier.list, tier.values
			m.choiceList(o, p, s, func(p *player.Player) { m.tagMenu(o, p) })
		}
	})
}

func (m *Manager) choiceList(o *state.Online, p *player.Player, s choiceSetting, back func(*player.Player)) {
	if !m.allow(o, s.access) {
		return
	}
	m.form(o, p, s.list, "", nil, func(p *player.Player, _ *world.Tx, choice int) {
		if choice < len(s.values) {
			m.saveChoice(o, p, s, s.values[choice])
		} else {
			back(p)
		}
	})
}

func (m *Manager) saveChoice(o *state.Online, p *player.Player, s choiceSetting, value string) {
	allowed, changed := false, false
	m.Store.Update(o.XUID, func(profile *store.Profile) {
		allowed = rank.Has(profile.Rank, s.perm)
		current := s.field(&profile.Cosmetics)
		if allowed && *current != value {
			*current = value
			changed = true
		}
	})
	switch {
	case !allowed:
		m.message(o, s.denied)
	case !changed && value == "":
		m.message(o, s.absent)
	case !changed:
		m.message(o, s.key+"AlreadySelected")
	default:
		switch s.key {
		case "cape":
			p.SetSkin(p.Skin())
		case "tag":
			m.Hub.UpdateNameTag(o, p)
		}
		if value == "" {
			m.message(o, s.key+"Disabled")
		} else {
			if s.name != "" {
				value = m.Lang.In(o.Lang(), s.name, value)
			}
			m.message(o, s.key+"Enabled", value)
		}
	}
}

type toggleSetting struct {
	access
	key, icon string
	field     func(*store.Cosmetics) *bool
}

var flySetting = toggleSetting{
	access{rank.CosmeticsFly, "flyNoPermission"}, "fly", "textures/items/feather",
	func(c *store.Cosmetics) *bool { return &c.Fly },
}

var petSetting = toggleSetting{
	access{rank.CosmeticsCapybara, "capybaraNoPermission"}, "capybara", "textures/items/egg_rabbit",
	func(c *store.Cosmetics) *bool { return &c.Capybara },
}

func (m *Manager) toggleForm(o *state.Online, p *player.Player, s toggleSetting) {
	if !m.allow(o, s.access) {
		return
	}
	profile := m.profile(o)
	status := "Disabled"
	if *s.field(&profile.Cosmetics) {
		status = "Enabled"
	}
	content := m.Lang.Text(o.Lang(), s.key+status+"ForForm")
	m.form(o, p, s.key+"Form", content, []string{s.icon, removeIcon, backIcon}, func(p *player.Player, tx *world.Tx, choice int) {
		if choice == 2 {
			m.SendForm(o, p, tx)
			return
		}
		m.saveToggle(o, p, tx, s, choice == 0)
	})
}

func (m *Manager) saveToggle(o *state.Online, p *player.Player, tx *world.Tx, s toggleSetting, enable bool) {
	allowed, changed := false, false
	m.Store.Update(o.XUID, func(profile *store.Profile) {
		allowed = rank.Has(profile.Rank, s.perm)
		current := s.field(&profile.Cosmetics)
		if allowed && *current != enable {
			*current = enable
			changed = true
		}
	})
	if !allowed {
		m.message(o, s.denied)
		return
	}
	status := "Disabled"
	if enable {
		status = "Enabled"
	}
	if !changed {
		m.message(o, s.key+"Already"+status)
		return
	}
	switch {
	case s.key == "fly":
		m.flight(p, enable)
	case enable:
		m.Enter(tx, p)
	default:
		m.Leave(tx, p)
	}
	m.message(o, s.key+status)
}
