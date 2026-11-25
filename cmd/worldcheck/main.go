package main

import (
	"cmp"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"

	"github.com/EraseMC/GoPractice/internal/worlds"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
)

const radius = 48

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	dirs, _ := filepath.Glob("worlds/*")
	maps, _ := filepath.Glob("duels/*/map")
	failed := false
	for _, dir := range append(dirs, maps...) {
		if err := check(log, dir); err != nil {
			fmt.Printf("%-28s FAIL %v\n", dir, err)
			failed = true
		}
	}
	if failed {
		os.Exit(1)
	}
}

func check(log *slog.Logger, dir string) error {
	snap, err := worlds.LoadSnapshot(log, dir)
	if err != nil {
		return err
	}
	w := snap.New(log)
	defer w.Close()

	spawn := w.Spawn()
	counts := map[string]int{}
	<-w.Do(func(tx *world.Tx) {
		for x := spawn.X() - radius; x <= spawn.X()+radius; x++ {
			for z := spawn.Z() - radius; z <= spawn.Z()+radius; z++ {
				for y := tx.Range().Min(); y <= tx.Range().Max(); y++ {
					name, _ := tx.Block(cube.Pos{x, y, z}).EncodeBlock()
					if name != "minecraft:air" {
						counts[name]++
					}
				}
			}
		}
	}).Done()
	type entry struct {
		name string
		n    int
	}
	var list []entry
	total := 0
	for k, v := range counts {
		list = append(list, entry{k, v})
		total += v
	}
	slices.SortFunc(list, func(a, b entry) int { return cmp.Compare(b.n, a.n) })
	fmt.Printf("%-28s name=%q spawn=%v blocks=%d kinds=%d\n", dir, w.Name(), spawn, total, len(list))
	for i, e := range list {
		if i == 6 {
			break
		}
		fmt.Printf("    %6d %s\n", e.n, e.name)
	}
	if total == 0 {
		return fmt.Errorf("no blocks around spawn %v", spawn)
	}
	return nil
}
