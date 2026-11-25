package gamemode

import (
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type adventureFly struct{}

var AdventureFly adventureFly

func (adventureFly) AllowsEditing() bool       { return false }
func (adventureFly) AllowsTakingDamage() bool  { return true }
func (adventureFly) CreativeInventory() bool   { return false }
func (adventureFly) HasCollision() bool        { return true }
func (adventureFly) AllowsFlying() bool        { return true }
func (adventureFly) AllowsInteraction() bool   { return true }
func (adventureFly) Visible() bool             { return true }
func (adventureFly) InstantPortalTravel() bool { return false }
func (adventureFly) GameType() int32           { return packet.GameTypeAdventure }

func IsAdventure(m world.GameMode) bool {
	return m == world.GameModeAdventure || m == AdventureFly
}

func WithFlight(m world.GameMode, allow bool) world.GameMode {
	switch {
	case allow && m == world.GameModeAdventure:
		return AdventureFly
	case !allow && m == AdventureFly:
		return world.GameModeAdventure
	}
	return m
}
