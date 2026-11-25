package hub

import (
	"log/slog"

	"github.com/EraseMC/GoPractice/internal/chat"
	"github.com/EraseMC/GoPractice/internal/gamemode"
	"github.com/EraseMC/GoPractice/internal/info"
	"github.com/EraseMC/GoPractice/internal/lang"
	"github.com/EraseMC/GoPractice/internal/rank"
	"github.com/EraseMC/GoPractice/internal/state"
	"github.com/EraseMC/GoPractice/internal/stats"
	"github.com/EraseMC/GoPractice/internal/store"
	"github.com/EraseMC/GoPractice/internal/text"
	"github.com/EraseMC/GoPractice/internal/toggles"
	"github.com/EraseMC/GoPractice/internal/ui"
	"github.com/EraseMC/GoPractice/internal/worlds"
	"github.com/df-mc/dragonfly/server/entity"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/sound"
	"github.com/go-gl/mathgl/mgl64"
)

const (
	Yaw          = 180
	Pitch        = 0
	MaxDistance  = 160
	ItemLore     = "§r§aoldtralia.ru"
	itemKey      = "practice:hub_item"
	itemDuels    = 0
	itemFFA      = 1
	itemSettings = 2
	itemStats    = 3
)

var textPos = mgl64.Vec3{801.5, 52.4, 793.5}

type Hooks struct {
	InQueue       func(o *state.Online) bool
	Fighting      func() int
	QueueForm     func(o *state.Online, p *player.Player, tx *world.Tx)
	FFAForm       func(o *state.Online, p *player.Player, tx *world.Tx)
	CosmeticsForm func(o *state.Online, p *player.Player, tx *world.Tx)
	InviteForm    func(from *state.Online, p *player.Player, to *state.Online, tx *world.Tx)
}

type Manager struct {
	Hooks

	log      *slog.Logger
	bundle   *lang.Bundle
	store    *store.Store
	players  *state.Players
	forms    *ui.Forms
	boards   *ui.Scoreboards
	chat     *chat.Manager
	toggles  *toggles.Manager
	stats    *stats.Manager
	info     *info.Manager
	language interface {
		SendLanguageForm(p *player.Player)
	}

	world *world.World
	spawn mgl64.Vec3
	npc   *npc
	text  *world.EntityHandle
}

type Deps struct {
	Log      *slog.Logger
	Lang     *lang.Bundle
	Store    *store.Store
	Players  *state.Players
	Forms    *ui.Forms
	Boards   *ui.Scoreboards
	Chat     *chat.Manager
	Toggles  *toggles.Manager
	Stats    *stats.Manager
	Info     *info.Manager
	Language interface {
		SendLanguageForm(p *player.Player)
	}
}

func New(d Deps) *Manager {
	return &Manager{
		log:      d.Log,
		bundle:   d.Lang,
		store:    d.Store,
		players:  d.Players,
		forms:    d.Forms,
		boards:   d.Boards,
		chat:     d.Chat,
		toggles:  d.Toggles,
		stats:    d.Stats,
		info:     d.Info,
		language: d.Language,
	}
}

func (m *Manager) Setup(w *world.World) error {
	m.world = w
	m.spawn = w.Spawn().Vec3()
	n, err := newNPC(m)
	if err != nil {
		return err
	}
	m.npc = n
	m.text = entity.NewText("", textPos)
	<-w.Do(func(tx *world.Tx) {
		n.spawn(tx)
		tx.AddEntity(m.text)
	}).Done()
	return nil
}

func (m *Manager) World() *world.World {
	return m.world
}

func (m *Manager) InHub(o *state.Online) bool {
	return o.In(state.PlaceHub)
}

func (m *Manager) inQueue(o *state.Online) bool {
	return m.InQueue != nil && m.InQueue(o)
}

func (m *Manager) profile(o *state.Online) store.Profile {
	p, ok := m.store.ByXUID(o.XUID)
	if !ok {
		p.Rank = rank.Default
	}
	return p
}

func (m *Manager) Teleport(p *player.Player, tx *world.Tx) {
	worlds.Move(tx, p, m.world, m.spawn, Yaw, Pitch)
}

func BasicProperties(p *player.Player, mode world.GameMode) {
	p.SetGameMode(mode)
	p.StopFlying()
	p.SetMaxHealth(20)
	if missing := p.MaxHealth() - p.Health(); missing > 0 {
		p.Heal(missing, resetHealing{})
	}
	p.SetFood(20)
	p.Saturate(0, 20)
	p.SetExperienceLevel(0)
	p.SetExperienceProgress(0)
	p.Inventory().Clear()
	p.Armour().Clear()
	p.SetHeldItems(item.Stack{}, item.Stack{})
	for _, e := range p.Effects() {
		p.RemoveEffect(e.Type())
	}
	p.Extinguish()
}

type resetHealing struct{}

func (resetHealing) HealingSource() {}

func (m *Manager) HasFly(o *state.Online) bool {
	return m.profile(o).Cosmetics.Fly
}

func (m *Manager) CheckFly(o *state.Online, p *player.Player) {
	prof := m.profile(o)
	if !prof.Cosmetics.Fly || rank.Has(prof.Rank, rank.CosmeticsFly) {
		return
	}
	m.store.Update(o.XUID, func(p *store.Profile) { p.Cosmetics.Fly = false })
	if p != nil {
		p.StopFlying()
		p.SetGameMode(gamemode.WithFlight(p.GameMode(), false))
	}
}

func (m *Manager) SetPlayerSettings(o *state.Online, p *player.Player, tx *world.Tx) {
	if m.inQueue(o) {
		m.Teleport(p, tx)
		return
	}
	BasicProperties(p, world.GameModeAdventure)
	o.SetPlace(state.PlaceHub, "")
	m.Teleport(p, tx)
	if m.HasFly(o) {
		p.SetGameMode(gamemode.WithFlight(p.GameMode(), true))
	}
}

func (m *Manager) SendMenuItems(o *state.Online, p *player.Player) {
	if m.inQueue(o) {
		return
	}
	names := m.bundle.Lines(o.Lang(), "hubItems")
	inv := p.Inventory()
	inv.Clear()
	_ = inv.SetItem(0, hubItem(item.Sword{Tier: item.ToolTierDiamond}, names[0], itemDuels).AsUnbreakable())
	_ = inv.SetItem(1, hubItem(item.Sword{Tier: item.ToolTierIron}, names[1], itemFFA).AsUnbreakable())
	_ = inv.SetItem(4, hubItem(item.Clock{}, names[2], itemSettings))
	_ = inv.SetItem(8, hubItem(item.Book{}, names[3], itemStats))
}

func hubItem(it world.Item, name string, id int) item.Stack {
	return item.NewStack(it, 1).WithCustomName(name).WithLore(ItemLore).WithValue(itemKey, id)
}

func heldHubItem(p *player.Player) (int, bool) {
	held, _ := p.HeldItems()
	v, ok := held.Value(itemKey)
	if !ok {
		return 0, false
	}
	id, ok := v.(int)
	return id, ok
}

func (m *Manager) UpdateScoreboard(o *state.Online) {
	if !m.toggles.Get(o, toggles.Scoreboard) || m.inQueue(o) {
		return
	}
	lines := m.bundle.Lines(o.Lang(), "hubScoreboard")
	prof := m.profile(o)
	fighting := 0
	if m.Fighting != nil {
		fighting = m.Fighting()
	}
	m.boards.Set(o.UUID, lines[0])
	m.boards.SetLine(o.UUID, 1, lines[1])
	m.boards.SetLine(o.UUID, 2, lang.Sprintf(lines[2], rank.Colored(prof.Rank)))
	m.boards.SetLine(o.UUID, 3, lang.Sprintf(lines[3], stats.Division(prof.Kills)))
	m.boards.SetLine(o.UUID, 4, lines[4])
	m.boards.SetLine(o.UUID, 5, lang.Sprintf(lines[5], m.players.Count()))
	m.boards.SetLine(o.UUID, 6, lang.Sprintf(lines[6], fighting))
	m.boards.SetLine(o.UUID, 7, lines[7])
	m.boards.SetLine(o.UUID, 8, lines[8])
}

func (m *Manager) NameTag(o *state.Online) string {
	tag := m.chat.NameTag(o)
	if m.chat.HasTag(o) {
		tag += "\n" + m.chat.TagColored(o)
	}
	return tag
}

func (m *Manager) UpdateNameTag(o *state.Online, p *player.Player) {
	p.SetNameTag(m.NameTag(o))
	p.SetScoreTag("")
}

func (m *Manager) Reset(o *state.Online, p *player.Player, tx *world.Tx) {
	m.SetPlayerSettings(o, p, tx)
	m.SendMenuItems(o, p)
	m.UpdateScoreboard(o)
	m.UpdateNameTag(o, p)
}

func (m *Manager) floatingText(o *state.Online) string {
	t := m.bundle.Lines(o.Lang(), "hubFloatingText")
	if t[1] == "" {
		return t[0]
	}
	return t[0] + "\n" + t[1]
}

func (m *Manager) UpdateFloatingText(o *state.Online, p *player.Player, tx *world.Tx) {
	if e, ok := m.text.Entity(tx); ok {
		p.ViewNameTag(e, m.floatingText(o))
	}
}

func joinColor(r string) string {
	if c := rank.Color(r); c != "§7" {
		return c
	}
	return "§f"
}

func (m *Manager) broadcast(msg string) {
	m.log.Info(text.Clean(msg, true))
	for _, o := range m.players.All() {
		o.Message(msg)
	}
}

func (m *Manager) Join(o *state.Online, p *player.Player, tx *world.Tx) {
	m.CheckFly(o, nil)
	m.SetPlayerSettings(o, p, tx)
	m.SendMenuItems(o, p)
	m.UpdateScoreboard(o)
	m.UpdateNameTag(o, p)
	m.UpdateFloatingText(o, p, tx)

	l := o.Lang()
	o.Message(m.bundle.Textf(l, "joinMessage", m.players.Count()))
	t := m.bundle.Lines(l, "joinTitle")
	ui.Title(p, t[0], t[1])
	p.PlaySound(sound.Note{Instrument: sound.Piano(), Pitch: 10})

	m.broadcast("§a+ §7| §f" + joinColor(m.profile(o).Rank) + o.Name)
}

func (m *Manager) Quit(o *state.Online) {
	o.SetPlace(state.PlaceNone, "")
	m.broadcast("§c- §7| §f" + joinColor(m.profile(o).Rank) + o.Name)
}

func (m *Manager) LanguageChanged(o *state.Online, p *player.Player, tx *world.Tx) {
	if !m.InHub(o) {
		return
	}
	m.SendMenuItems(o, p)
	m.UpdateScoreboard(o)
	m.UpdateFloatingText(o, p, tx)
}

func (m *Manager) Tick() {
	for _, o := range m.players.All() {
		if m.InHub(o) {
			m.UpdateScoreboard(o)
		}
	}
}

func (m *Manager) SendPreferencesForm(p *player.Player) {
	o, ok := m.players.Get(p.UUID())
	if !ok {
		return
	}
	form := ui.NewSimple(func(p *player.Player, tx *world.Tx, data *int) {
		if data == nil {
			return
		}
		switch *data {
		case 0:
			m.language.SendLanguageForm(p)
		case 1:
			m.toggles.SendForm(p, toggles.WithBack)
		case 2:
			if m.CosmeticsForm != nil {
				m.CosmeticsForm(o, p, tx)
			}
		}
	})
	contents := m.bundle.Lines(o.Lang(), "preferencesForm")
	form.SetTitle(contents[0])
	form.AddButton(contents[1], ui.ImageNone, "")
	form.AddButton(contents[2], ui.ImageNone, "")
	form.AddButton(contents[3], ui.ImageNone, "")
	m.forms.Send(p, form)
}

func (m *Manager) SendMenuForm(p *player.Player) {
	o, ok := m.players.Get(p.UUID())
	if !ok {
		return
	}
	form := ui.NewSimple(func(p *player.Player, _ *world.Tx, data *int) {
		if data == nil {
			return
		}
		switch *data {
		case 0:
			m.info.SendInfoForm(p, true)
		case 1:
			m.info.SendRulesForm(p, true)
		}
	})
	contents := m.bundle.Lines(o.Lang(), "menuForm")
	form.SetTitle(contents[0])
	form.AddButton(contents[1], ui.ImageNone, "")
	form.AddButton(contents[2], ui.ImageNone, "")
	m.forms.Send(p, form)
}
