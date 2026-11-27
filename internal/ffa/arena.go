package ffa

import (
	"math"
	"time"

	"github.com/EraseMC/GoPractice/internal/combat"
	"github.com/df-mc/dragonfly/server/entity/effect"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/item/enchantment"
	"github.com/df-mc/dragonfly/server/item/potion"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

type Kind int

const (
	NoDebuff Kind = iota
	Soup
	Sumo
	Fist
	GApple
	Resistance
	Combo
)

type Arena struct {
	Kind    Kind
	Folder  string
	Name    string
	Display string
	Icon    string
	Y       float64
	KillKey string
	Stats   bool
	NoScore bool
	Profile combat.Profile

	world *world.World
	spawn mgl64.Vec3
}

func (a *Arena) World() *world.World {
	return a.world
}

func (a *Arena) Spawn() mgl64.Vec3 {
	return a.spawn
}

func Arenas() []*Arena {
	return []*Arena{
		{Kind: NoDebuff, Folder: "ffa_nodebuff", Name: "NoDebuff", Display: "§cNoDebuff", Icon: "textures/items/potion_bottle_splash_heal", Y: 66, KillKey: "ffaNoDebuffKill", Stats: true, Profile: combat.FFANoDebuff},
		{Kind: Soup, Folder: "ffa_soup", Name: "Soup", Display: "§7Soup", Icon: "textures/items/mushroom_stew", Y: 48, KillKey: "ffaSoupKill", Stats: true, Profile: combat.FFASoup},
		{Kind: Sumo, Folder: "ffa_sumo", Name: "Sumo", Display: "§aSumo", Icon: "textures/items/slimeball", Y: 65, KillKey: "ffaSumoKill", Stats: true, NoScore: true, Profile: combat.FFASumo},
		{Kind: Fist, Folder: "ffa_fist", Name: "Fist", Display: "§6Fist", Icon: "textures/items/beef_cooked", Y: 66, KillKey: "ffaFistKill", Stats: true, Profile: combat.FFAFist},
		{Kind: GApple, Folder: "ffa_gapple", Name: "GApple", Display: "§eGApple", Icon: "textures/items/apple_golden", Y: 66, KillKey: "ffaGAppleKill", Stats: true, Profile: combat.FFAGApple},
		{Kind: Resistance, Folder: "ffa_resistance", Name: "Resistance", Display: "§4Resistance", Icon: "textures/ui/resistance_effect", Y: 66, KillKey: "ffaResistanceKill", Stats: false, NoScore: true, Profile: combat.FFAResistance},
		{Kind: Combo, Folder: "ffa_combo", Name: "Combo", Display: "§bCombo", Icon: "textures/items/feather", Y: 66, KillKey: "ffaComboKill", Stats: true, Profile: combat.FFACombo(66)},
	}
}

const effectDuration = time.Duration(math.MaxInt32) * 50 * time.Millisecond

func lasting(t effect.LastingType, level int) effect.Effect {
	return effect.New(t, level, effectDuration).WithoutParticles()
}

func enchanted(s item.Stack, unbreakable bool, e ...item.Enchantment) item.Stack {
	if unbreakable {
		s = s.AsUnbreakable()
	}
	return s.WithEnchantments(e...)
}

func armour(tier item.ArmourTier, unbreakable bool, unbreaking int) (h, c, l, b item.Stack) {
	e := []item.Enchantment{item.NewEnchantment(enchantment.Protection, 1), item.NewEnchantment(enchantment.Unbreaking, unbreaking)}
	h = enchanted(item.NewStack(item.Helmet{Tier: tier}, 1), unbreakable, e...)
	c = enchanted(item.NewStack(item.Chestplate{Tier: tier}, 1), unbreakable, e...)
	l = enchanted(item.NewStack(item.Leggings{Tier: tier}, 1), unbreakable, e...)
	b = enchanted(item.NewStack(item.Boots{Tier: tier}, 1), unbreakable, e...)
	return
}

func tool(it world.Item) item.Stack {
	return enchanted(item.NewStack(it, 1), true, item.NewEnchantment(enchantment.Unbreaking, 10))
}

func GiveNoDebuffKit(p *player.Player) {
	inv := p.Inventory()
	p.Armour().Set(armour(item.ArmourTierDiamond{}, true, 10))
	_, _ = inv.AddItem(tool(item.Sword{Tier: item.ToolTierDiamond}))
	_, _ = inv.AddItem(item.NewStack(item.EnderPearl{}, 16))
	pot := item.NewStack(item.SplashPotion{Type: potion.StrongHealing()}, 1)
	for i := 0; i <= inv.Size()-2; i++ {
		_, _ = inv.AddItem(pot)
	}
}

func (a *Arena) GiveKit(p *player.Player) {
	inv := p.Inventory()
	switch a.Kind {
	case NoDebuff:
		GiveNoDebuffKit(p)
		p.AddEffect(lasting(effect.Speed, 1))
	case Soup:
		p.Armour().Set(armour(item.ArmourTierIron{}, true, 10))
		_, _ = inv.AddItem(tool(item.Sword{Tier: item.ToolTierStone}))
		for i := 0; i < 16; i++ {
			_, _ = inv.AddItem(item.NewStack(item.MushroomStew{}, 1))
		}
	case Sumo:
		_, _ = inv.AddItem(item.NewStack(item.Stick{}, 1))
		p.AddEffect(lasting(effect.Resistance, 256))
	case Fist:
		_, _ = inv.AddItem(item.NewStack(item.Beef{Cooked: true}, 1))
	case GApple:
		p.Armour().Set(armour(item.ArmourTierDiamond{}, true, 10))
		_, _ = inv.AddItem(tool(item.Sword{Tier: item.ToolTierDiamond}))
		_, _ = inv.AddItem(item.NewStack(item.GoldenApple{}, 8))
	case Resistance:
		_, _ = inv.AddItem(tool(item.Axe{Tier: item.ToolTierDiamond}))
		p.AddEffect(lasting(effect.Resistance, 256))
	case Combo:
		p.Armour().Set(armour(item.ArmourTierDiamond{}, false, 3))
		_, _ = inv.AddItem(tool(item.Sword{Tier: item.ToolTierDiamond}))
		_, _ = inv.AddItem(item.NewStack(item.EnchantedApple{}, 3))
	}
}

func count(p *player.Player, match func(world.Item) bool) int {
	n := 0
	for _, s := range p.Inventory().Items() {
		if match(s.Item()) {
			n++
		}
	}
	return n
}

func Potions(p *player.Player) int {
	return count(p, func(it world.Item) bool { _, ok := it.(item.SplashPotion); return ok })
}

func Soups(p *player.Player) int {
	return count(p, func(it world.Item) bool { _, ok := it.(item.MushroomStew); return ok })
}

func health(p *player.Player) int {
	return int(math.Round(p.Health()))
}

func (a *Arena) killArgs(dead, killer string, deadP, killerP *player.Player) []any {
	switch a.Kind {
	case NoDebuff:
		return []any{dead, Potions(deadP), killer, Potions(killerP), health(killerP)}
	case Soup:
		return []any{dead, Soups(deadP), killer, Soups(killerP), health(killerP)}
	case Sumo, Resistance:
		return []any{dead, killer}
	}
	return []any{dead, killer, health(killerP)}
}
