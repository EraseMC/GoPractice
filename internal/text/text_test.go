package text

import (
	"slices"
	"testing"
)

func TestSplitQuoteAware(t *testing.T) {
	cases := map[string][]string{
		`tell Steve hello  world`:     {"tell", "Steve", "hello", "world"},
		`ban "Big Steve" 1 "a \"b\""`: {"ban", "Big Steve", "1", `a "b"`},
		`say ""`:                      {"say"},
		``:                            nil,
	}
	for in, want := range cases {
		if got := SplitQuoteAware(in); !slices.Equal(got, want) {
			t.Errorf("%q: got %q want %q", in, got, want)
		}
	}
}

func TestClean(t *testing.T) {
	if got := Clean("§aHi §lthere§r §", true); got != "Hi there " {
		t.Fatalf("got %q", got)
	}
	if got := Clean("§aHi\x1b[31m", false); got != "§aHi" {
		t.Fatalf("got %q", got)
	}
	if Len("привет") != 6 {
		t.Fatal("len")
	}
}
