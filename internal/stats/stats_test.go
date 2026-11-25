package stats

import (
	"testing"

	"github.com/EraseMC/GoPractice/internal/store"
)

func TestDivision(t *testing.T) {
	cases := map[int]string{
		-1: "§0Bedrock", 0: "§6Bronze", 49: "§6Bronze", 50: "§7Silver", 149: "§eGold", 150: "§dPlatinum",
		200: "§bDiamond", 300: "§aEmerald", 400: "§8Netherite", 500: "§5Sapphire", 1000: "§cMaster",
		1999: "§cMaster", 2000: "§cMaster§e+",
	}
	for kills, want := range cases {
		if got := Division(kills); got != want {
			t.Errorf("Division(%d) = %q, want %q", kills, got, want)
		}
	}
}

func TestKDR(t *testing.T) {
	cases := []struct {
		k, d int
		want string
	}{{7, 0, "7"}, {10, 5, "2"}, {3, 2, "1.5"}, {1, 3, "0.33"}, {2, 3, "0.67"}}
	for _, c := range cases {
		if got := KDR(c.k, c.d); got != c.want {
			t.Errorf("KDR(%d, %d) = %q, want %q", c.k, c.d, got, c.want)
		}
	}
}

func TestTopText(t *testing.T) {
	format := "%s%s. %s - %s"
	if got := topText(format, nil, Hours); got != ":(" {
		t.Fatalf("empty top = %q", got)
	}
	got := topText(format, []store.TopEntry{{"a", 7200}, {"b", 3600}, {"c", 0}, {"d", 1}}, Hours)
	want := "§61. a - 2\n§g2. b - 1\n§e3. c - 0\n§74. d - 0\n"
	if got != want {
		t.Fatalf("top = %q, want %q", got, want)
	}
}
