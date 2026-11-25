package skins

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/fs"

	"github.com/df-mc/dragonfly/server/player/skin"
)

func FromPNG(fsys fs.FS, path string) (skin.Skin, error) {
	f, err := fsys.Open(path)
	if err != nil {
		return skin.Skin{}, err
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return skin.Skin{}, fmt.Errorf("decode %s: %w", path, err)
	}
	return FromImage(img), nil
}

func FromImage(img image.Image) skin.Skin {
	b := img.Bounds()
	s := skin.New(b.Dx(), b.Dy())
	i := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			s.Pix[i], s.Pix[i+1], s.Pix[i+2], s.Pix[i+3] = c.R, c.G, c.B, c.A&^1
			i += 4
		}
	}
	s.ModelConfig = skin.ModelConfig{Default: "geometry.humanoid.custom"}
	return s
}
