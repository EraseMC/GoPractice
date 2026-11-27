package cosmetics

import (
	"encoding/base64"
	"fmt"
	"image/color"
	"io/fs"
	"slices"
	"sync"

	"github.com/EraseMC/GoPractice/assets"
	"github.com/EraseMC/GoPractice/internal/gamemode"
	"github.com/EraseMC/GoPractice/internal/hub"
	"github.com/EraseMC/GoPractice/internal/lang"
	"github.com/EraseMC/GoPractice/internal/netmw"
	"github.com/EraseMC/GoPractice/internal/rank"
	"github.com/EraseMC/GoPractice/internal/skins"
	"github.com/EraseMC/GoPractice/internal/state"
	"github.com/EraseMC/GoPractice/internal/store"
	"github.com/EraseMC/GoPractice/internal/tags"
	"github.com/EraseMC/GoPractice/internal/ui"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/player/skin"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
)

var capes = []string{"Oldtralia", "Lunarelly", "Birthday", "Cake", "Enderman", "Firework", "Iron_Golem", "Pickaxe", "Green_Creeper", "Red_Creeper", "Turtle", "Migrator"}
var colourNames = []string{"Red", "Dark_Red", "Blue", "Light_Blue", "Green", "Purple", "Yellow", "White", "Black"}

var colours = map[string]color.RGBA{
	"Red": {R: 255, A: 255}, "Dark_Red": {R: 139, A: 255}, "Blue": {B: 255, A: 255},
	"Light_Blue": {G: 255, B: 255, A: 255}, "Green": {G: 255, A: 255}, "Purple": {R: 153, G: 50, B: 204, A: 255},
	"Yellow": {R: 255, G: 255, A: 255}, "White": {R: 255, G: 255, B: 255, A: 255}, "Black": {A: 255},
}

type Deps struct {
	Lang    *lang.Bundle
	Store   *store.Store
	Players *state.Players
	Forms   *ui.Forms
	Hub     *hub.Manager
}

type Manager struct {
	Deps
	adapter    *skins.Adapter
	capeImages map[string]skin.Cape
	petSkin    skin.Skin

	mu       sync.Mutex
	pets     map[uuid.UUID]*pet
	petsByID map[uuid.UUID]*pet
	views    map[*netmw.Conn]map[int64]*pet
}

func New(d Deps) (*Manager, error) {
	adapter, err := skins.NewAdapter(assets.FS)
	if err != nil {
		return nil, err
	}
	petSkin, err := skins.FromPNG(assets.FS, "skins/capybara/capybara.png")
	if err != nil {
		return nil, err
	}
	petSkin.FullID = "Capybara"
	petSkin.ModelConfig.Default = "geometry.capybara"
	petSkin.Model, err = fs.ReadFile(assets.FS, "skins/capybara/capybara.geo.json")
	if err != nil {
		return nil, err
	}
	m := &Manager{
		Deps: d, adapter: adapter, capeImages: make(map[string]skin.Cape), petSkin: petSkin,
		pets: make(map[uuid.UUID]*pet), petsByID: make(map[uuid.UUID]*pet), views: make(map[*netmw.Conn]map[int64]*pet),
	}
	for _, name := range capes {
		image, err := skins.FromPNG(assets.FS, "skins/capes/"+name+".png")
		if err != nil {
			return nil, err
		}
		if len(image.Pix) != 8192 {
			return nil, fmt.Errorf("invalid cape image size for %s", name)
		}
		cape := skin.NewCape(32, 64)
		copy(cape.Pix, image.Pix)
		m.capeImages[name] = cape
	}
	return m, nil
}

func (m *Manager) profile(o *state.Online) store.Profile {
	p, _ := m.Store.ByXUID(o.XUID)
	return p
}

func (m *Manager) message(o *state.Online, key string, args ...any) {
	o.Message(m.Lang.Textf(o.Lang(), key, args...))
}

func tagPermission(name string) string {
	switch {
	case slices.Contains(tags.Silver, name):
		return rank.CosmeticsTagsSilver
	case slices.Contains(tags.Golden, name):
		return rank.CosmeticsTagsGolden
	case slices.Contains(tags.Platinum, name):
		return rank.CosmeticsTagsPlatinum
	default:
		return ""
	}
}

func (m *Manager) NormalizeProfile(profile *store.Profile) {
	r, c := profile.Rank, &profile.Cosmetics
	if _, valid := m.capeImages[c.Cape]; !valid || !rank.Has(r, rank.CosmeticsCapes) {
		c.Cape = ""
	}
	if perm := tagPermission(c.Tag); perm == "" || !rank.Has(r, perm) {
		c.Tag = ""
	}
	if _, valid := colours[c.PotionColor]; !valid || !rank.Has(r, rank.CosmeticsPotionColor) {
		c.PotionColor = ""
	}
	c.Fly = c.Fly && rank.Has(r, rank.CosmeticsFly)
	c.Capybara = c.Capybara && rank.Has(r, rank.CosmeticsCapybara)
}

func (m *Manager) Join(o *state.Online, p *player.Player) {
	m.Store.Update(o.XUID, m.NormalizeProfile)
	if o.Conn == nil || p.Skin().CapeID != m.profile(o).Cosmetics.Cape {
		p.SetSkin(p.Skin())
	}
}

func (m *Manager) PrepareClientData(xuid string, data login.ClientData) login.ClientData {
	data = m.adapter.AdaptLogin(data)
	p, _ := m.Store.ByXUID(xuid)
	m.NormalizeProfile(&p)
	cape := m.capeImages[p.Cosmetics.Cape]
	data.CapeID = p.Cosmetics.Cape
	data.CapeImageWidth, data.CapeImageHeight = cape.Bounds().Dx(), cape.Bounds().Dy()
	data.CapeData = base64.StdEncoding.EncodeToString(cape.Pix)
	data.ProfileHash = ""
	return data
}

func (m *Manager) RankChanged(o *state.Online, p *player.Player) {
	c := m.profile(o).Cosmetics
	if p.Skin().CapeID != c.Cape {
		p.SetSkin(p.Skin())
	}
	if !c.Capybara {
		m.Leave(p.Tx(), p)
	}
	if m.Hub.InHub(o) {
		m.flight(p, c.Fly)
	}
}

func (m *Manager) flight(p *player.Player, enabled bool) {
	mode := gamemode.WithFlight(p.GameMode(), enabled)
	if mode == p.GameMode() {
		return
	}
	if !enabled {
		p.StopFlying()
	}
	p.SetGameMode(mode)
}

func (m *Manager) PotionColour(o *state.Online) (color.RGBA, bool) {
	p := m.profile(o)
	c, ok := colours[p.Cosmetics.PotionColor]
	return c, ok && rank.Has(p.Rank, rank.CosmeticsPotionColor)
}

func (m *Manager) skin(o *state.Online, s skin.Skin) skin.Skin {
	s = m.adapter.Adapt(s)
	p := m.profile(o)
	s.Cape = skin.Cape{}
	s.CapeID = ""
	s.ProfileHash = ""
	if cape, ok := m.capeImages[p.Cosmetics.Cape]; ok && rank.Has(p.Rank, rank.CosmeticsCapes) {
		s.Cape = cape
		s.CapeID = p.Cosmetics.Cape
	}
	return s
}
