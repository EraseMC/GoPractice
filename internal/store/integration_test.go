package store

import (
	"context"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/EraseMC/GoPractice/internal/db"
)

func integrationConfig(t *testing.T) db.Config {
	raw := os.Getenv("OLDTRALIA_TEST_DB")
	if raw == "" {
		t.Skip("OLDTRALIA_TEST_DB=host:port:user:password:database is not set")
	}
	parts := strings.SplitN(raw, ":", 5)
	if len(parts) != 5 {
		t.Fatal("OLDTRALIA_TEST_DB must be host:port:user:password:database")
	}
	port, _ := strconv.Atoi(parts[1])
	return db.Config{Host: parts[0], Port: port, Username: parts[2], Password: parts[3], Database: parts[4], Workers: 2}
}

func TestIntegrationRoundTrip(t *testing.T) {
	conf := integrationConfig(t)
	ctx := context.Background()
	log := slog.New(slog.DiscardHandler)

	d, err := db.Open(ctx, conf)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	for _, table := range []string{"statistics", "settings", "cosmetics", "bans", "mutes", "players", "schema_migrations"} {
		if _, err := d.ExecContext(ctx, "DROP TABLE IF EXISTS "+table); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Migrate(ctx, d, log); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(ctx, d, log); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	var applied int
	if err := d.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations WHERE version = 1 AND name = 'init'").Scan(&applied); err != nil || applied != 1 {
		t.Fatalf("schema_migrations: %d %v", applied, err)
	}

	if _, err := d.ExecContext(ctx, "INSERT INTO players (xuid, name, ip, device_id, player_rank, language) VALUES ('900', 'legacy', '9.9.9.9', 'old', 'Owner', 'ru')"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.ExecContext(ctx, "INSERT INTO settings (player_id, chat, hide_players) VALUES ((SELECT id FROM players WHERE xuid = '900'), 0, 0)"); err != nil {
		t.Fatal(err)
	}

	s, err := Load(ctx, d)
	if err != nil {
		t.Fatal(err)
	}
	legacy, ok := s.ByXUID("900")
	if !ok || legacy.Rank != "Owner" || legacy.Locale != "ru" || legacy.Settings.Chat || legacy.Settings.HidePlayers || !legacy.Settings.Scoreboard || legacy.Cosmetics.Cape != "" {
		t.Fatalf("legacy profile %+v", legacy)
	}

	pool := db.NewPool(d, conf.Workers, log)
	s.Attach(pool)

	s.Login(Identity{XUID: "100", Name: "NewGuy", IP: "1.2.3.4", DeviceID: "dev", Locale: "ru_RU"})
	s.Login(Identity{XUID: "900", Name: "Legacy2", IP: "9.9.9.9", DeviceID: "new"})
	s.Update("100", func(p *Profile) {
		p.Kills = 7
		p.Deaths = 2
		p.Settings.CPS = true
		p.Cosmetics.Cape = "Oldtralia"
		p.Cosmetics.Fly = true
	})
	s.Update("900", func(p *Profile) { p.Cosmetics.Tag = "Pro" })
	s.Update("900", func(p *Profile) { p.Cosmetics.Tag = "" })
	s.Punishments.AddBan(Ban{Nickname: "newguy", XUID: "100", DeviceID: "dev", BannedBy: "console", Expiry: 0, When: 1700000000, Reason: "-"})
	s.Punishments.AddMute(Mute{Nickname: "legacy2", XUID: "900", MutedBy: "console", Expiry: 1800000000, When: 1700000000, Reason: "spam"})
	s.Punishments.RenameMutes("900", "legacy3")

	top := <-s.FetchTop(StatKills, 10)
	pool.Close()

	if len(top) == 0 || top[0].Nickname != "newguy" || top[0].Value != 7 {
		t.Fatalf("top %+v", top)
	}

	again, err := Load(ctx, d)
	if err != nil {
		t.Fatal(err)
	}
	p, ok := again.ByXUID("100")
	if !ok || p.Nickname != "newguy" || p.Locale != "ru" || p.Rank != DefaultRank || p.Kills != 7 || p.Deaths != 2 || !p.Settings.CPS || !p.Settings.HidePlayers || p.Cosmetics.Cape != "Oldtralia" || !p.Cosmetics.Fly {
		t.Fatalf("reloaded new profile %+v", p)
	}
	l, ok := again.ByXUID("900")
	if !ok || l.Nickname != "legacy2" || l.DeviceID != "new" || l.Rank != "Owner" || l.Cosmetics.Tag != "" || l.Settings.Chat {
		t.Fatalf("reloaded legacy profile %+v", l)
	}
	var nullTag int
	if err := d.QueryRowContext(ctx, "SELECT COUNT(*) FROM cosmetics c JOIN players p ON p.id = c.player_id WHERE p.xuid = '900' AND c.tag IS NULL").Scan(&nullTag); err != nil || nullTag != 1 {
		t.Fatalf("tag should be NULL: %d %v", nullTag, err)
	}
	if b, ok := again.Punishments.BanByDeviceID("dev"); !ok || b.When != 1700000000 {
		t.Fatalf("ban %+v", b)
	}
	if m, ok := again.Punishments.MuteByNickname("legacy3"); !ok || m.Reason != "spam" {
		t.Fatalf("mute %+v", m)
	}
}
