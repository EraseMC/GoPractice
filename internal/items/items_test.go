package items

import (
	"image/color"
	"testing"
	"time"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/entity/effect"
	"github.com/df-mc/dragonfly/server/item/potion"
)

type recorder struct {
	got []effect.Effect
}

func (r *recorder) AddEffect(e effect.Effect) {
	r.got = append(r.got, e)
}

func TestApplyLasting(t *testing.T) {
	r := &recorder{}
	Apply(r, []effect.Effect{effect.New(effect.Speed, 1, 90*time.Second)}, MaxMiss)
	if len(r.got) != 1 {
		t.Fatalf("got %d effects", len(r.got))
	}
	if want := 1622 * 50 * time.Millisecond; r.got[0].Duration() != want {
		t.Fatalf("miss duration %v, want %v", r.got[0].Duration(), want)
	}

	r = &recorder{}
	Apply(r, []effect.Effect{effect.New(effect.Speed, 1, 90*time.Second)}, MaxHit)
	if want := 1762 * 50 * time.Millisecond; r.got[0].Duration() != want {
		t.Fatalf("hit duration %v, want %v", r.got[0].Duration(), want)
	}

	r = &recorder{}
	Apply(r, []effect.Effect{effect.New(effect.Speed, 1, time.Second/2)}, MaxHit)
	if len(r.got) != 0 {
		t.Fatal("short effect must be skipped")
	}
}

func TestApplyInstant(t *testing.T) {
	r := &recorder{}
	Apply(r, potion.StrongHealing().Effects(), MaxHit)
	if len(r.got) != 1 || r.got[0].Type() != effect.InstantHealth || r.got[0].Level() != 2 {
		t.Fatalf("unexpected effects %+v", r.got)
	}
}

func TestMix(t *testing.T) {
	c := Mix([]effect.Effect{effect.NewInstant(effect.InstantHealth, 2)})
	if c != effect.InstantHealth.RGBA() {
		t.Fatalf("got %v", c)
	}
	a, b := effect.Speed.RGBA(), effect.InstantHealth.RGBA()
	want := color.RGBA{
		R: uint8((int(a.R) + 2*int(b.R)) / 3),
		G: uint8((int(a.G) + 2*int(b.G)) / 3),
		B: uint8((int(a.B) + 2*int(b.B)) / 3),
		A: uint8((int(a.A) + 2*int(b.A)) / 3),
	}
	if got := Mix([]effect.Effect{effect.New(effect.Speed, 1, time.Second), effect.NewInstant(effect.InstantHealth, 2)}); got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
	if Mix(nil) != DefaultSplashColour {
		t.Fatal("empty mix must be default")
	}
}

func TestSeconds(t *testing.T) {
	for ticks, want := range map[int]int{300: 15, 299: 15, 282: 15, 281: 14, 20: 1, 3: 1, 1: 0} {
		if got := seconds(ticks); got != want {
			t.Errorf("%d ticks: got %d, want %d", ticks, got, want)
		}
	}
}

func TestDirectionStraightDown(t *testing.T) {
	d := Direction(cube.Rotation{0, 90})
	if d[1] > -0.999999 {
		t.Fatalf("looking straight down must throw down, got %v", d)
	}
	d = Direction(cube.Rotation{90, 0})
	if d[0] > -0.999999 {
		t.Fatalf("yaw 90 must point to -x, got %v", d)
	}
}
