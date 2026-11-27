package skins

import (
	"testing"

	"github.com/EraseMC/GoPractice/assets"
	"github.com/df-mc/dragonfly/server/player/skin"
)

func TestAdapter(t *testing.T) {
	a, err := NewAdapter(assets.FS)
	if err != nil {
		t.Fatal(err)
	}
	for _, size := range [][2]int{{64, 32}, {64, 64}, {128, 128}} {
		s := skin.New(size[0], size[1])
		s.ModelConfig.Default = "geometry.humanoid.custom"
		s.FullID = "custom"
		for i := 0; i < len(s.Pix); i += 4 {
			s.Pix[i], s.Pix[i+3] = byte(i), 255
		}
		if got := a.Adapt(s); got.FullID != s.FullID {
			t.Fatalf("replaced opaque %v skin", size)
		}
		for _, unsupported := range []skin.Skin{{Persona: true}, {Premium: true}, {PersonaCapeOnClassic: true}} {
			candidate := s
			candidate.Persona, candidate.Premium, candidate.PersonaCapeOnClassic = unsupported.Persona, unsupported.Premium, unsupported.PersonaCapeOnClassic
			if got := a.Adapt(candidate); got.FullID != "Oldtralia_Default" {
				t.Fatalf("kept unsupported skin flags at %v", size)
			}
		}
		w := size[0]
		scale := w / 64
		for i := range s.Pix {
			if i%4 == 3 && i >= w*8*4*scale && i < w*16*4*scale {
				s.Pix[i] = 0
			}
		}
		if got := a.Adapt(s); got.FullID != "Oldtralia_Default" {
			t.Fatalf("kept transparent %v skin", size)
		}
		clear(s.Pix)
		if got := a.Adapt(s); got.FullID != "Oldtralia_Default" {
			t.Fatalf("kept solid placeholder at %v", size)
		}
	}
}
