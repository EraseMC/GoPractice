package cosmetics

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/EraseMC/GoPractice/internal/handler"
	"github.com/EraseMC/GoPractice/internal/lang"
	"github.com/EraseMC/GoPractice/internal/netmw"
	"github.com/EraseMC/GoPractice/internal/netver"
	"github.com/EraseMC/GoPractice/internal/rank"
	"github.com/EraseMC/GoPractice/internal/state"
	"github.com/EraseMC/GoPractice/internal/store"
	"github.com/df-mc/dragonfly/server"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func TestLegacyClientsReceivePreparedSkin(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 35*time.Second)
	defer cancel()
	socket, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := socket.LocalAddr().String()
	_ = socket.Close()

	profiles, players := store.NewMemory(), state.NewPlayers()
	profiles.Login(store.Identity{XUID: "subject", Name: "SkinSubject", Locale: "en"})
	profiles.Update("subject", func(p *store.Profile) { p.Rank = rank.Owner; p.Cosmetics.Cape = "Oldtralia" })
	m, err := New(Deps{Store: profiles, Players: players})
	if err != nil {
		t.Fatal(err)
	}
	mw := netmw.New()
	mw.PrepareClientData = func(id login.IdentityData, data login.ClientData) login.ClientData {
		if id.DisplayName == "SkinSubject" {
			return m.PrepareClientData("subject", data)
		}
		return m.PrepareClientData("", data)
	}
	mw.OnWrite(m.Outbound)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	selected := map[string]minecraft.Protocol{}
	conf := server.Config{
		Log: log, AuthDisabled: true, DisableResourceBuilding: true, MaxChunkRadius: 4,
		Listeners: mw.Wrap([]func(server.Config) (server.Listener, error){
			func(conf server.Config) (server.Listener, error) {
				return server.ListenNetwork(conf, minecraft.RakNet{}, addr)
			},
		}),
		AcceptedProtocolsProvider: func(blocks world.BlockRegistry) ([]minecraft.Protocol, error) {
			all, err := netver.Provider(nil)(blocks)
			for _, p := range all {
				if p.ID() == 766 {
					selected["1.21.50"] = p
				}
				if p.ID() == 419 {
					selected["1.16.100"] = p
				}
			}
			return all, err
		},
	}
	srv := conf.New()
	srv.Listen()
	t.Cleanup(func() { _ = srv.Close() })
	router := handler.NewRouter()
	router.Add(m.Handler())
	go func() {
		for p := range srv.Accept() {
			connection, _ := mw.Conn(p.UUID())
			xuid := ""
			if p.Name() == "SkinSubject" {
				xuid = "subject"
			}
			o := players.Add(p, xuid, connection, lang.EN)
			p.Handle(router.Handler())
			m.Join(o, p)
		}
	}()
	join := func(version, network, name string, persona bool) *minecraft.Conn {
		t.Helper()
		p := selected[version]
		if p == nil {
			t.Fatalf("protocol %s is unavailable", version)
		}
		d := minecraft.Dialer{Protocol: p, IdentityData: login.IdentityData{DisplayName: name}, ClientData: login.ClientData{PersonaSkin: persona, GameVersion: version}}
		c, err := d.DialContext(ctx, network, addr)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = c.Close() })
		if err := c.DoSpawnContext(ctx); err != nil {
			t.Fatal(err)
		}
		_ = c.SetReadDeadline(time.Now().Add(10 * time.Second))
		return c
	}
	subject := join("1.21.50", "raknet", "SkinSubject", true)
	id := uuid.MustParse(subject.IdentityData().Identity)
	viewer := join("1.16.100", "raknet-v10", "SkinViewer", false)

	check := func(s protocol.Skin) {
		t.Helper()
		if s.PersonaSkin || s.PremiumSkin || s.CapeImageWidth != 32 || s.CapeImageHeight != 64 || len(s.CapeData) != 8192 || s.FullID == "" || s.FullID == "Oldtralia_Default" {
			t.Fatalf("invalid prepared skin: persona=%t premium=%t cape=%dx%d fullID=%q", s.PersonaSkin, s.PremiumSkin, s.CapeImageWidth, s.CapeImageHeight, s.FullID)
		}
	}
	for _, client := range []*minecraft.Conn{subject, viewer} {
		registered, visible := false, client == subject
		for !registered || !visible {
			p, err := client.ReadPacket()
			if err != nil {
				t.Fatalf("protocol %d: registered=%t visible=%t: %v", client.Proto().ID(), registered, visible, err)
			}
			switch v := p.(type) {
			case *packet.PlayerList:
				for _, entry := range v.Entries {
					if entry.UUID == id && entry.ActionType == protocol.PlayerListActionAdd {
						check(entry.Skin)
						registered = true
					}
				}
			case *packet.AddPlayer:
				if v.UUID == id {
					if !registered || v.EntityMetadata.Flag(protocol.EntityDataKeyFlags, protocol.EntityDataFlagInvisible) {
						t.Fatal("observer received an invisible or unregistered actor")
					}
					visible = true
				}
			case *packet.PlayerSkin:
				if v.UUID == id {
					check(v.Skin)
				}
			}
		}
	}
}
