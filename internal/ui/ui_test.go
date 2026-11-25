package ui

import (
	"encoding/json"
	"testing"

	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func TestBoardDiff(t *testing.T) {
	b := &board{rendered: map[int]string{}, title: "T", lines: map[int]string{1: "a", 2: "a", 3: "b"}}
	pks := b.diff()
	if len(pks) != 2 {
		t.Fatalf("first flush: %d packets", len(pks))
	}
	if _, ok := pks[0].(*packet.SetDisplayObjective); !ok {
		t.Fatal("expected display objective first")
	}
	set := pks[1].(*packet.SetScore)
	if len(set.Entries) != 3 || set.Entries[1].DisplayName != "a§r" {
		t.Fatalf("unique lines: %+v", set.Entries)
	}

	if pks := b.diff(); len(pks) != 0 {
		t.Fatalf("unchanged board sent %d packets", len(pks))
	}

	b.lines = map[int]string{1: "a", 2: "c"}
	pks = b.diff()
	if len(pks) != 2 {
		t.Fatalf("diff: %d packets", len(pks))
	}
	removed, changed := pks[0].(*packet.SetScore), pks[1].(*packet.SetScore)
	if len(removed.Entries) != 2 || removed.Entries[0].IdentityType != protocol.ScoreboardIdentityRemove {
		t.Fatalf("removed: %+v", removed.Entries)
	}
	if len(changed.Entries) != 1 || changed.Entries[0].DisplayName != "c" || changed.Entries[0].Score != 2 {
		t.Fatalf("changed: %+v", changed.Entries)
	}

	b.title = "U"
	pks = b.diff()
	if _, ok := pks[0].(*packet.RemoveObjective); !ok || len(pks) != 3 {
		t.Fatalf("title change: %#v", pks)
	}
}

func TestSimpleJSONMatchesPHP(t *testing.T) {
	f := NewSimple(nil)
	f.SetTitle("t")
	f.AddButton("a", ImagePath, "textures/x")
	f.AddButton("b", ImageNone, "")
	got, _ := json.Marshal(f)
	want := `{"type":"form","title":"t","content":"","buttons":[{"text":"a","image":{"type":"path","data":"textures/x"}},{"text":"b"}]}`
	if string(got) != want {
		t.Fatalf("got %s", got)
	}
}

func TestSimpleSubmit(t *testing.T) {
	var got *int
	called := false
	f := NewSimple(func(_ *player.Player, _ *world.Tx, data *int) { called, got = true, data })
	f.AddButton("a", ImageNone, "")
	f.AddButton("b", ImageNone, "")
	if err := f.submit(nil, nil, []byte("1")); err != nil || !called || got == nil || *got != 1 {
		t.Fatalf("submit 1: err=%v got=%v", err, got)
	}
	called = false
	if err := f.submit(nil, nil, nil); err != nil || !called || got != nil {
		t.Fatal("close should call with nil")
	}
	called = false
	if err := f.submit(nil, nil, []byte("5")); err == nil || called {
		t.Fatal("out of range accepted")
	}
}

func TestCustomSubmitWithOmittedLabels(t *testing.T) {
	var got []any
	f := NewCustom(func(_ *player.Player, _ *world.Tx, data []any) { got = data })
	f.AddLabel("info")
	f.AddToggle("t", nil)
	f.AddDropdown("d", []string{"x", "y"}, nil)
	f.AddInput("i", "", nil)
	if err := f.submit(nil, nil, []byte(`[true,1,"hi"]`)); err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 || got[0] != nil || got[1] != true || got[2] != 1 || got[3] != "hi" {
		t.Fatalf("got %#v", got)
	}
	if err := f.submit(nil, nil, []byte(`[null,true,2,"hi"]`)); err == nil {
		t.Fatal("invalid dropdown index accepted")
	}
}
