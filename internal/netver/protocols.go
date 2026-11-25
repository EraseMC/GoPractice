package netver

import (
	"fmt"
	"slices"

	"github.com/df-mc/dragonfly/server"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft"
	multiversion "github.com/shawtymarco/go-multiversion"
)

var DefaultBlocked = []int32{475}

func Provider(blocked []int32) func(world.BlockRegistry) ([]minecraft.Protocol, error) {
	return func(blocks world.BlockRegistry) ([]minecraft.Protocol, error) {
		all, err := multiversion.Config{}.ProtocolsWithRegistries(blocks, server.VanillaItemEntries())
		if err != nil {
			return nil, fmt.Errorf("build multiversion protocols: %w", err)
		}
		return slices.DeleteFunc(all, func(p minecraft.Protocol) bool {
			return slices.Contains(blocked, p.ID())
		}), nil
	}
}
