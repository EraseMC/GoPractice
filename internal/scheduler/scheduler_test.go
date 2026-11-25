package scheduler

import (
	"log/slog"
	"testing"
)

func TestRepeatAndAfter(t *testing.T) {
	s := New(slog.New(slog.DiscardHandler))
	var every20, once, cancelled int
	s.Repeat(20, func() { every20++ })
	s.After(5, func() { once++ })
	c := s.Repeat(1, func() { cancelled++ })
	for range 3 {
		s.step()
	}
	c.Cancel()
	for range 57 {
		s.step()
	}
	if every20 != 3 || once != 1 || cancelled != 3 {
		t.Fatalf("every20=%d once=%d cancelled=%d", every20, once, cancelled)
	}
}

func TestPanicIsContained(t *testing.T) {
	s := New(slog.New(slog.DiscardHandler))
	ran := false
	s.After(1, func() { panic("boom") })
	s.After(1, func() { ran = true })
	s.step()
	if !ran {
		t.Fatal("second task did not run")
	}
}
