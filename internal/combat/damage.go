package combat

import (
	"math"
	"math/rand/v2"

	"github.com/df-mc/dragonfly/server/entity/effect"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/item/enchantment"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
)

type Attack struct {
	Base      float64
	Strength  int
	Weakness  int
	Sharpness int
	Critical  bool
}

type Defence struct {
	ArmourPoints float64
	Resistance   int
	EPF          int
	Roll         float64
}

func AttackPoints(it world.Item) float64 {
	switch t := it.(type) {
	case item.Sword:
		return t.Tier.BaseAttackDamage + 4
	case item.Axe:
		return t.Tier.BaseAttackDamage + 3
	case item.Pickaxe:
		return t.Tier.BaseAttackDamage + 2
	case item.Shovel:
		return t.Tier.BaseAttackDamage + 1
	}
	return 1
}

func ProtectionFactor(level int) int {
	return int(math.Floor(float64(6+level*level) * 0.75 / 3))
}

func Damage(a Attack, d Defence) float64 {
	final := a.Base
	if a.Strength > 0 {
		final += a.Base * 0.3 * float64(a.Strength)
	}
	if a.Weakness > 0 {
		final -= a.Base * 0.2 * float64(a.Weakness)
	}
	if a.Sharpness > 0 {
		final += 0.5 * float64(a.Sharpness+1)
	}
	if a.Critical {
		final += math.Max(0, final) / 2
	}

	final -= math.Max(0, final) * d.ArmourPoints * 0.04
	if d.Resistance > 0 {
		final -= math.Max(0, final) * math.Min(1, 0.2*float64(d.Resistance))
	}
	epf := math.Min(math.Ceil(math.Min(float64(d.EPF), 25)*d.Roll), 20)
	final -= math.Max(0, final) * epf * 0.04
	return math.Max(0, final)
}

func Roll() float64 {
	return float64(50+rand.IntN(51)) / 100
}

func AttackOf(attacker *player.Player, critical bool) Attack {
	held, _ := attacker.HeldItems()
	a := Attack{Critical: critical}
	if !held.Empty() {
		a.Base = AttackPoints(held.Item())
	} else {
		a.Base = 1
	}
	if e, ok := attacker.Effect(effect.Strength); ok {
		a.Strength = e.Level()
	}
	if e, ok := attacker.Effect(effect.Weakness); ok {
		a.Weakness = e.Level()
	}
	if s, ok := held.Enchantment(enchantment.Sharpness); ok {
		a.Sharpness = s.Level()
	}
	return a
}

func DefenceOf(victim *player.Player) Defence {
	d := Defence{Roll: Roll()}
	for _, it := range victim.Armour().Items() {
		if a, ok := it.Item().(item.Armour); ok {
			d.ArmourPoints += a.DefencePoints()
		}
		if e, ok := it.Enchantment(enchantment.Protection); ok {
			d.EPF += ProtectionFactor(e.Level())
		}
	}
	if e, ok := victim.Effect(effect.Resistance); ok {
		d.Resistance = e.Level()
	}
	return d
}

func Critical(p *player.Player) bool {
	if p.Sprinting() || p.Flying() || p.FallDistance() <= 0 {
		return false
	}
	if _, ok := p.Effect(effect.Blindness); ok {
		return false
	}
	return true
}
