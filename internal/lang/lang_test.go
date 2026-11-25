package lang

import (
	"slices"
	"strings"
	"testing"
)

func TestEveryKeyInEveryLanguage(t *testing.T) {
	b, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range Sections {
		refText, refLines := b.Keys(EN, s)
		slices.Sort(refText)
		slices.Sort(refLines)
		for _, l := range Languages[1:] {
			text, lines := b.Keys(l, s)
			slices.Sort(text)
			slices.Sort(lines)
			if !slices.Equal(refText, text) {
				t.Errorf("%s/%s: string keys differ from %s", l, s, EN)
			}
			if !slices.Equal(refLines, lines) {
				t.Errorf("%s/%s: list keys differ from %s", l, s, EN)
			}
			for _, k := range refText {
				if a, c := strings.Count(b.In(EN, s, k), "%s"), strings.Count(b.In(l, s, k), "%s"); a != c {
					t.Errorf("%s/%s.%s: %d placeholders, %s has %d", l, s, k, c, EN, a)
				}
			}
			if s != Messages {
				continue
			}
			for _, k := range refLines {
				en, other := b.Lines(EN, k), b.Lines(l, k)
				if len(en) != len(other) {
					t.Errorf("%s/%s.%s: %d lines, %s has %d", l, s, k, len(other), EN, len(en))
					continue
				}
				for i := range en {
					if a, c := strings.Count(en[i], "%s"), strings.Count(other[i], "%s"); a != c {
						t.Errorf("%s/%s.%s[%d]: %d placeholders, %s has %d", l, s, k, i, c, EN, a)
					}
				}
			}
		}
	}
}

func TestFallbacks(t *testing.T) {
	b := MustLoad()
	if got := b.Text(EN, "missingKey"); got != "Unknown message" {
		t.Errorf("en fallback = %q", got)
	}
	if got := b.Text(RU, "missingKey"); got != "Неизвестное сообщение" {
		t.Errorf("ru fallback = %q", got)
	}
	if got := b.Text("de", "joinMessage"); got != "Unknown language" {
		t.Errorf("unknown language = %q", got)
	}
	if got := b.Textf(EN, "pingSelf", 42); got != "§aYour ping: §e42ms" {
		t.Errorf("pingSelf = %q", got)
	}
}
