package skins

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"image"
	"io/fs"

	"github.com/df-mc/dragonfly/server/player/skin"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

var vanillaHashes = map[string]struct{}{
	"2de9f1efb4cf85f020c68465acf41f8211e2402e7425ffc902537b0e7046e4f0": {},
	"953a988290ae02e3df91d49b15a0f97523c0c6d302079ae89222b8d76b79287f": {},
	"2da730fdd07fa9b3033428a4ee600542ac3cc1e036176fde71d09c17f24a6f51": {},
}

type Adapter struct {
	fallback skin.Skin
	bounds   []image.Rectangle
}

func NewAdapter(fsys fs.FS) (*Adapter, error) {
	fallback, err := FromPNG(fsys, "skins/default.png")
	if err != nil {
		return nil, err
	}
	fallback.FullID = "Oldtralia_Default"
	fallback.ArmSize = protocol.ArmSizeWide
	data, err := fs.ReadFile(fsys, "skins/default.geo.json")
	if err != nil {
		return nil, err
	}
	var model map[string]struct {
		Bones []struct {
			Cubes []struct {
				Size [3]int
				UV   [2]int
			}
		}
	}
	if err := json.Unmarshal(data, &model); err != nil {
		return nil, fmt.Errorf("decode default skin geometry: %w", err)
	}
	a := &Adapter{fallback: fallback}
	for _, bone := range model["geometry.humanoid"].Bones {
		for _, cube := range bone.Cubes {
			x, y, z := cube.Size[0], cube.Size[1], cube.Size[2]
			u, v := cube.UV[0], cube.UV[1]
			a.bounds = append(a.bounds,
				image.Rect(u+z, v, u+z+2*x, v+z),
				image.Rect(u, v+z, u+2*(z+x), v+z+y))
		}
	}
	if len(a.bounds) == 0 {
		return nil, fmt.Errorf("default skin geometry has no cubes")
	}
	return a, nil
}

func (a *Adapter) Adapt(s skin.Skin) skin.Skin {
	if s.Persona || s.Premium || s.PersonaCapeOnClassic || s.ModelConfig.Default == "" || a.invalid(s) {
		return a.fallback
	}
	return s
}

func (a *Adapter) invalid(s skin.Skin) bool {
	w, h := s.Bounds().Dx(), s.Bounds().Dy()
	if !((w == 64 && (h == 32 || h == 64)) || (w == 128 && h == 128)) || len(s.Pix) != w*h*4 {
		return true
	}
	if _, vanilla := vanillaHashes[fmt.Sprintf("%x", sha256.Sum256(s.Pix))]; vanilla {
		return true
	}
	solid := true
	for i := 4; i < len(s.Pix) && solid; i += 4 {
		solid = bytes.Equal(s.Pix[:4], s.Pix[i:i+4])
	}
	if solid {
		return true
	}
	scale := w / 64
	transparent, total := 0, 0
	for _, box := range a.bounds {
		box = image.Rect(box.Min.X*scale, box.Min.Y*scale, box.Max.X*scale, box.Max.Y*scale)
		if !box.In(s.Bounds()) {
			continue
		}
		total += box.Dx() * box.Dy()
		for y := box.Min.Y; y < box.Max.Y; y++ {
			for x := box.Min.X; x < box.Max.X; x++ {
				if s.Pix[(y*w+x)*4+3] < 127 {
					transparent++
				}
			}
		}
	}
	return total == 0 || (transparent*100+total/2)/total > 4
}
