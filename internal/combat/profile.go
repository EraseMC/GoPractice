package combat

import "math"

type Profile struct {
	XZ       float64
	Y        float64
	Immunity int
	NoCrit   bool
	ZeroY    func(y float64) bool
}

var (
	Default = Profile{XZ: 0.4, Y: 0.4, Immunity: 10}

	DuelNoDebuff = Profile{XZ: 0.3855, Y: 0.385, Immunity: 9}
	DuelSoup     = Profile{XZ: 0.386, Y: 0.386, Immunity: 9}
	DuelSumo     = Profile{XZ: 0.384, Y: 0.383, Immunity: 9}
	DuelFist     = Profile{XZ: 0.377, Y: 0.382, Immunity: 8}
	DuelGApple   = Profile{XZ: 0.377, Y: 0.381, Immunity: 8}

	FFANoDebuff   = Profile{XZ: 0.3855, Y: 0.385, Immunity: 9}
	FFASoup       = Profile{XZ: 0.386, Y: 0.386, Immunity: 9}
	FFASumo       = Profile{XZ: 0.378, Y: 0.382, Immunity: 8, NoCrit: true}
	FFAFist       = Profile{XZ: 0.377, Y: 0.382, Immunity: 8}
	FFAGApple     = Profile{XZ: 0.377, Y: 0.381, Immunity: 8}
	FFAResistance = Profile{XZ: 0.377, Y: 0.382, Immunity: 8, NoCrit: true}
)

func FFACombo(arenaY float64) Profile {
	return Profile{
		XZ:       0.245,
		Y:        0.21,
		Immunity: 3,
		ZeroY:    func(y float64) bool { return math.Abs(arenaY-y) >= 2.5 },
	}
}

func (p Profile) Velocity(dx, dz, y float64) (x, vy, z float64, ok bool) {
	f := math.Sqrt(dx*dx + dz*dz)
	if f <= 0 {
		return 0, 0, 0, false
	}
	f = 1 / f
	x, vy, z = dx*f*p.XZ, p.Y, dz*f*p.XZ
	if vy > p.Y {
		vy = p.Y
	}
	if p.ZeroY != nil && p.ZeroY(y) {
		vy = 0
	}
	return x, vy, z, true
}
