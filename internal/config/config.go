package config

import (
	"fmt"
	"os"

	"github.com/df-mc/dragonfly/server"
	"github.com/pelletier/go-toml"
)

func Default() server.UserConfig {
	c := server.DefaultConfig()
	c.Network.Address = ":19132"
	c.Network.Transport = []string{"raknet"}
	c.Network.NetherNet.KeyFile = "nethernet/nethernet.key"
	c.Network.NetherNet.Domain = "oldtralia.ru"
	c.Network.NetherNet.UDPPorts = "19134"
	c.Server.Name = "§l§aOldtralia"
	c.Server.AuthEnabled = true
	c.Server.DisableJoinQuitMessages = true
	c.Server.MuteEmoteChat = true
	c.World.SaveData = false
	c.World.Folder = "worlds/lobby"
	c.World.DisableNether = true
	c.World.DisableEnd = true
	c.World.DisableFireTick = true
	c.World.DisableHopperTick = true
	c.World.DisableRedstoneTick = true
	c.World.DisableBlockTicks = true
	c.World.RandomTickSpeed = -1
	c.World.DisableVanillaRecipes = true
	c.Players.MaxCount = 100
	c.Players.MaximumChunkRadius = 8
	c.Players.SaveData = false
	c.Resources.AutoBuildPack = false
	c.Resources.Folder = "resource_packs"
	c.Resources.Required = true
	return c
}

func Load(path string) (server.UserConfig, error) {
	c := Default()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		out, err := toml.Marshal(c)
		if err != nil {
			return c, fmt.Errorf("encode default config: %w", err)
		}
		if err := os.WriteFile(path, out, 0o644); err != nil {
			return c, fmt.Errorf("write default config: %w", err)
		}
		return c, nil
	}
	if err != nil {
		return c, fmt.Errorf("read config: %w", err)
	}
	if err := toml.Unmarshal(data, &c); err != nil {
		return c, fmt.Errorf("decode config: %w", err)
	}
	return c, nil
}
