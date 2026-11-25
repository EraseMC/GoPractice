package skins

import (
	"image"
	"image/color"
	"testing"

	"github.com/EraseMC/GoPractice/assets"
)

func TestAlphaMatchesGD(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	img.SetNRGBA(0, 0, color.NRGBA{1, 2, 3, 255})
	img.SetNRGBA(1, 0, color.NRGBA{4, 5, 6, 0})
	s := FromImage(img)
	want := []uint8{1, 2, 3, 254, 4, 5, 6, 0}
	for i, v := range want {
		if s.Pix[i] != v {
			t.Fatalf("pix = %v, want %v", s.Pix, want)
		}
	}
}

func TestNPCSkin(t *testing.T) {
	s, err := FromPNG(assets.FS, "skins/npc/npc.png")
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Pix) != 64*64*4 || s.ModelConfig.Default != "geometry.humanoid.custom" {
		t.Fatalf("unexpected skin: %d bytes, model %q", len(s.Pix), s.ModelConfig.Default)
	}
}
