package worlds

import (
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"maps"
	"os"
	"path/filepath"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/df-mc/dragonfly/server/world/mcdb"
	"github.com/df-mc/goleveldb/leveldb"
	"github.com/google/uuid"
)

type column struct {
	data          chunk.SerialisedData
	blockEntities []chunk.BlockEntity
}

type Snapshot struct {
	Name       string
	Spawn      cube.Pos
	Time       int64
	Difficulty world.Difficulty
	GameMode   world.GameMode
	blocks     world.BlockRegistry
	columns    map[world.ChunkPos]column
}

func LoadSnapshot(log *slog.Logger, dir string) (*Snapshot, error) {
	tmp, err := os.MkdirTemp("", "gopractice-world-*")
	if err != nil {
		return nil, fmt.Errorf("snapshot %s: %w", dir, err)
	}
	defer os.RemoveAll(tmp)
	if err := copyDir(dir, tmp); err != nil {
		return nil, fmt.Errorf("snapshot %s: copy: %w", dir, err)
	}
	db, err := mcdb.Config{Log: log}.Open(tmp)
	if err != nil {
		return nil, fmt.Errorf("snapshot %s: open: %w", dir, err)
	}
	defer db.Close()

	set := db.Settings()
	s := &Snapshot{
		Name:       set.Name,
		Spawn:      set.Spawn,
		Time:       set.Time,
		Difficulty: set.Difficulty,
		GameMode:   set.DefaultGameMode,
		blocks:     world.DefaultBlockRegistry,
		columns:    map[world.ChunkPos]column{},
	}
	it := db.NewColumnIterator(nil)
	for it.Next() {
		if it.Dimension() != world.Overworld {
			continue
		}
		col := it.Column()
		s.columns[it.Position()] = column{
			data:          chunk.Encode(col.Chunk, chunk.DiskEncoding),
			blockEntities: col.BlockEntities,
		}
	}
	it.Release()
	if err := it.Error(); err != nil {
		return nil, fmt.Errorf("snapshot %s: iterate: %w", dir, err)
	}
	return s, nil
}

func (s *Snapshot) Columns() int {
	return len(s.columns)
}

func (s *Snapshot) Provider() world.Provider {
	return &provider{snap: s}
}

func (s *Snapshot) New(log *slog.Logger) *world.World {
	return world.Config{
		Log:                 log,
		Provider:            s.Provider(),
		Generator:           world.NopGenerator{},
		ReadOnly:            true,
		RandomTickSpeed:     -1,
		DisableFireTick:     true,
		DisableBlockTicks:   true,
		DisableRedstoneTick: true,
		DisableHopperTick:   true,
	}.New()
}

type provider struct {
	snap *Snapshot
}

func (p *provider) Settings() *world.Settings {
	return &world.Settings{
		Name:            p.snap.Name,
		Spawn:           p.snap.Spawn,
		Time:            p.snap.Time,
		DefaultGameMode: p.snap.GameMode,
		Difficulty:      p.snap.Difficulty,
		TickRange:       6,
	}
}

func (p *provider) SaveSettings(*world.Settings) {}

func (p *provider) LoadPlayerSpawnPosition(uuid.UUID) (cube.Pos, bool, error) {
	return cube.Pos{}, false, nil
}

func (p *provider) SavePlayerSpawnPosition(uuid.UUID, cube.Pos) error {
	return nil
}

func (p *provider) LoadColumn(pos world.ChunkPos, dim world.Dimension) (*chunk.Column, error) {
	c, ok := p.snap.columns[pos]
	if !ok || dim != world.Overworld {
		return nil, leveldb.ErrNotFound
	}
	ch, err := chunk.DiskDecode(p.snap.blocks, c.data, dim.Range())
	if err != nil {
		return nil, fmt.Errorf("decode column %v: %w", pos, err)
	}
	bes := make([]chunk.BlockEntity, len(c.blockEntities))
	for i, be := range c.blockEntities {
		bes[i] = chunk.BlockEntity{Pos: be.Pos, Data: maps.Clone(be.Data)}
	}
	return &chunk.Column{Chunk: ch, BlockEntities: bes}, nil
}

func (p *provider) StoreColumn(world.ChunkPos, world.Dimension, *chunk.Column) error {
	return nil
}

func (p *provider) Close() error {
	return nil
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if d.Name() == "LOCK" {
			return nil
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			out.Close()
			return err
		}
		return out.Close()
	})
}
