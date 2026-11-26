package entities

import (
	"time"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/entity"
	"github.com/df-mc/dragonfly/server/world"
)

type scaledType struct {
	name  string
	scale float64
	box   cube.BBox
}

func (t scaledType) Open(tx *world.Tx, handle *world.EntityHandle, data *world.EntityData) world.Entity {
	return &Scaled{Ent: entity.Open(tx, handle, data), scale: t.scale}
}

func (t scaledType) EncodeEntity() string                        { return t.name }
func (t scaledType) BBox(world.Entity) cube.BBox                 { return t.box }
func (t scaledType) DecodeNBT(map[string]any, *world.EntityData) {}
func (t scaledType) EncodeNBT(*world.EntityData) map[string]any  { return nil }

type Scaled struct {
	*entity.Ent
	scale float64
}

func (s *Scaled) Scale() float64 {
	return s.scale
}

func box(width, height float64) cube.BBox {
	half := width / 2
	return cube.Box(-half, 0, -half, half, height, half)
}

var (
	PearlType = scaledType{
		name:  "minecraft:ender_pearl",
		scale: 0.29,
		box:   cube.Box(-0.029/25, 0, -0.029/25, 0.029/25, 0.029, 0.029/25),
	}
	SplashType = scaledType{
		name:  "minecraft:splash_potion",
		scale: 0.4,
		box:   box(0.1, 0.1),
	}
	LightningType = scaledType{
		name:  "minecraft:lightning_bolt",
		scale: 1,
		box:   box(0.1, 0.1),
	}
)

const Tick = 50 * time.Millisecond

func Lightning(opts world.EntitySpawnOpts) *world.EntityHandle {
	return opts.New(LightningType, entity.StationaryBehaviourConfig{ExistenceDuration: 24 * Tick})
}
