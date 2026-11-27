package duel

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/EraseMC/GoPractice/internal/combat"
	"github.com/EraseMC/GoPractice/internal/ffa"
	"github.com/EraseMC/GoPractice/internal/state"
	"github.com/EraseMC/GoPractice/internal/worlds"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

type mode struct {
	id, display, icon string
	profile           combat.Profile
	kit               ffa.Kind
}

var modes = []mode{
	{"nodebuff", "§cNoDebuff", "potion_bottle_splash_heal", combat.DuelNoDebuff, ffa.NoDebuff},
	{"classic_nodebuff", "§cClassic NoDebuff", "potion_bottle_splash_harm", combat.DuelNoDebuff, ffa.NoDebuff},
	{"boxing", "§9Boxing", "diamond_sword", combat.DuelNoDebuff, ffa.NoDebuff},
	{"soup", "§8Soup", "mushroom_stew", combat.DuelSoup, ffa.Soup},
	{"sumo", "§aSumo", "slimeball", combat.DuelSumo, ffa.Sumo},
	{"fist", "§6Fist", "beef_cooked", combat.DuelFist, ffa.Fist},
	{"gapple", "§eGApple", "apple_golden", combat.DuelGApple, ffa.GApple},
}

func ModeIDs() []string {
	ids := make([]string, 0, len(modes))
	for _, m := range modes {
		ids = append(ids, m.id)
	}
	return ids
}

func findMode(id string) (mode, bool) {
	for _, m := range modes {
		if m.id == strings.ToLower(id) {
			return m, true
		}
	}
	return mode{}, false
}

func Profile(o *state.Online) (combat.Profile, bool) {
	place, id := o.Place()
	m, ok := findMode(id)
	return m.profile, ok && place == state.PlaceDuel
}

func (m mode) noDamage() bool {
	return m.id == "boxing" || m.id == "sumo"
}

type spawn struct {
	X, Y, Z, Yaw, Pitch float64
}

func (s spawn) position() mgl64.Vec3 {
	return mgl64.Vec3{s.X, s.Y, s.Z}
}

type arena struct {
	spawns   []spawn
	snapshot *worlds.Snapshot
}

func loadMaps(log *slog.Logger, dir string) (map[string][]arena, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	pool := make(map[string][]arena)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(filepath.Join(path, "config.json"))
		if err != nil {
			return nil, err
		}
		var config struct {
			Name   string
			Modes  []string
			Spawns []spawn
		}
		if err := json.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("duel map %s: %w", path, err)
		}
		if config.Name == "" || len(config.Spawns) < 2 || len(config.Modes) == 0 {
			return nil, fmt.Errorf("duel map %s needs a name, two spawns and at least one mode", path)
		}
		snapshot, err := worlds.LoadSnapshot(log, filepath.Join(path, "map"))
		if err != nil {
			return nil, err
		}
		for _, id := range config.Modes {
			m, ok := findMode(id)
			if !ok {
				return nil, fmt.Errorf("duel map %s: unknown mode %q", path, id)
			}
			pool[m.id] = append(pool[m.id], arena{config.Spawns, snapshot})
		}
	}
	return pool, nil
}

func playerIn(tx *world.Tx, o *state.Online) (*player.Player, bool) {
	e, ok := o.Handle.Entity(tx)
	if !ok {
		return nil, false
	}
	p, ok := e.(*player.Player)
	return p, ok
}
