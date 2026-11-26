package combat

import (
	"math"
	"testing"

	"github.com/df-mc/dragonfly/server/item"
)

func near(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

func TestAttackPoints(t *testing.T) {
	cases := []struct {
		it   item.Sword
		want float64
	}{
		{item.Sword{Tier: item.ToolTierWood}, 5},
		{item.Sword{Tier: item.ToolTierGold}, 5},
		{item.Sword{Tier: item.ToolTierStone}, 6},
		{item.Sword{Tier: item.ToolTierIron}, 7},
		{item.Sword{Tier: item.ToolTierDiamond}, 8},
		{item.Sword{Tier: item.ToolTierNetherite}, 9},
	}
	for _, c := range cases {
		if got := AttackPoints(c.it); got != c.want {
			t.Errorf("%v: got %v, want %v", c.it.Tier.Name, got, c.want)
		}
	}
	if got := AttackPoints(item.Axe{Tier: item.ToolTierDiamond}); got != 7 {
		t.Errorf("diamond axe: got %v", got)
	}
	if got := AttackPoints(item.Pickaxe{Tier: item.ToolTierDiamond}); got != 6 {
		t.Errorf("diamond pickaxe: got %v", got)
	}
	if got := AttackPoints(item.Shovel{Tier: item.ToolTierDiamond}); got != 5 {
		t.Errorf("diamond shovel: got %v", got)
	}
	if got := AttackPoints(item.Stick{}); got != 1 {
		t.Errorf("stick: got %v", got)
	}
}

func TestProtectionFactor(t *testing.T) {
	for lvl, want := range map[int]int{1: 1, 2: 2, 3: 3, 4: 5} {
		if got := ProtectionFactor(lvl); got != want {
			t.Errorf("level %d: got %d, want %d", lvl, got, want)
		}
	}
}

func TestDamage(t *testing.T) {
	diamond := Defence{ArmourPoints: 20, EPF: 4, Roll: 1}
	cases := []struct {
		name string
		a    Attack
		d    Defence
		want float64
	}{
		{"fist no armour", Attack{Base: 1}, Defence{Roll: 1}, 1},
		{"sword vs prot1 full roll", Attack{Base: 8}, diamond, 1.344},
		{"sword vs prot1 half roll", Attack{Base: 8}, Defence{ArmourPoints: 20, EPF: 4, Roll: 0.5}, 1.472},
		{"crit", Attack{Base: 8, Critical: true}, diamond, 2.016},
		{"sharpness crit", Attack{Base: 8, Sharpness: 1, Critical: true}, Defence{Roll: 1}, 13.5},
		{"strength", Attack{Base: 8, Strength: 2}, Defence{Roll: 1}, 12.8},
		{"weakness", Attack{Base: 8, Weakness: 1}, Defence{Roll: 1}, 6.4},
		{"resistance 255", Attack{Base: 8, Critical: true}, Defence{Resistance: 255, Roll: 1}, 0},
		{"resistance 1", Attack{Base: 10}, Defence{Resistance: 1, Roll: 1}, 8},
		{"epf cap", Attack{Base: 10}, Defence{EPF: 40, Roll: 1}, 2},
	}
	for _, c := range cases {
		if got := Damage(c.a, c.d); !near(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestRoll(t *testing.T) {
	for i := 0; i < 1000; i++ {
		r := Roll()
		if r < 0.5 || r > 1 || !near(math.Round(r*100), r*100) {
			t.Fatalf("roll out of range: %v", r)
		}
	}
}

func TestVelocity(t *testing.T) {
	x, y, z, ok := FFANoDebuff.Velocity(3, 4, 70)
	if !ok || !near(x, 0.6*0.3855) || !near(z, 0.8*0.3855) || !near(y, 0.385) {
		t.Fatalf("nodebuff: %v %v %v %v", x, y, z, ok)
	}
	if _, _, _, ok := Default.Velocity(0, 0, 0); ok {
		t.Fatal("zero direction must not knock back")
	}
	combo := FFACombo(66)
	if _, y, _, _ := combo.Velocity(1, 0, 67); !near(y, 0.21) {
		t.Fatalf("combo near floor: %v", y)
	}
	if _, y, _, _ := combo.Velocity(1, 0, 68.5); y != 0 {
		t.Fatalf("combo high: %v", y)
	}
	if _, y, _, _ := combo.Velocity(1, 0, 63.5); y != 0 {
		t.Fatalf("combo low: %v", y)
	}
}
