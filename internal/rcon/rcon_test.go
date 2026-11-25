package rcon

import (
	"io"
	"log/slog"
	"net"
	"testing"
)

func TestServer(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	s, err := Listen(log, "127.0.0.1:0", "secret", func(cmd string) string {
		return "ran " + cmd + "\nsecond\n"
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	bad, err := net.Dial("tcp", s.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	_ = writePacket(bad, 7, typeAuth, "nope")
	id, kind, _, err := readPacket(bad)
	if err != nil || id != -1 || kind != typeAuthOK {
		t.Fatalf("bad auth: %d %d %v", id, kind, err)
	}
	if _, _, _, err := readPacket(bad); err == nil {
		t.Fatal("connection should be closed after wrong password")
	}
	_ = bad.Close()

	c, err := net.Dial("tcp", s.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	_ = writePacket(c, 1, typeAuth, "secret")
	if id, kind, _, err := readPacket(c); err != nil || id != 1 || kind != typeAuthOK {
		t.Fatalf("auth: %d %d %v", id, kind, err)
	}
	_ = writePacket(c, 2, typeCommand, "  list")
	id, kind, payload, err := readPacket(c)
	if err != nil || id != 2 || kind != typeResponse || payload != "ran list\r\nsecond" {
		t.Fatalf("command: %d %d %q %v", id, kind, payload, err)
	}
}
