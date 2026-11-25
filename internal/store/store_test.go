package store

import (
	"slices"
	"testing"

	"github.com/EraseMC/GoPractice/internal/db"
)

type captured struct {
	q     db.Query
	shard string
}

func testStore() (*Store, *[]captured) {
	s := newStore()
	s.Punishments = &Punishments{}
	var log []captured
	s.sink = func(q db.Query, shard string) { log = append(log, captured{q, shard}) }
	return s, &log
}

func TestLoginRegistersNewPlayer(t *testing.T) {
	s, log := testStore()
	p := s.Login(Identity{XUID: "100", Name: "Steve_X", IP: "1.2.3.4", DeviceID: "dev", Locale: "ru_RU"})
	if p.Nickname != "steve_x" || p.Locale != "ru" || p.Rank != DefaultRank || !p.Settings.HidePlayers || p.Settings.CPS {
		t.Fatalf("unexpected profile %+v", p)
	}
	if len(*log) != 1 || !(*log)[0].q.Tx || len((*log)[0].q.Stmts) != 4 || (*log)[0].shard != "100" {
		t.Fatalf("unexpected queries %+v", *log)
	}
	if got, _ := s.ByNickname("STEVE_X"); got.XUID != "100" {
		t.Fatal("nickname index")
	}
}

func TestLoginSyncsChangedIdentity(t *testing.T) {
	s, log := testStore()
	s.Login(Identity{XUID: "100", Name: "a", IP: "1.1.1.1", DeviceID: "d1"})
	*log = nil
	s.Login(Identity{XUID: "100", Name: "B", IP: "1.1.1.1", DeviceID: "d2"})
	var sqls []string
	for _, c := range *log {
		sqls = append(sqls, c.q.Stmts[0].SQL)
	}
	want := []string{
		"INSERT IGNORE INTO statistics (player_id) VALUES ((SELECT id FROM players WHERE xuid = ?))",
		"UPDATE players SET name = ? WHERE xuid = ?",
		"UPDATE players SET device_id = ? WHERE xuid = ?",
	}
	if !slices.Equal(sqls, want) {
		t.Fatalf("got %q", sqls)
	}
	if s.Registered("a") || !s.Registered("b") {
		t.Fatal("nickname reindex")
	}
}

func TestUpdateWritesOnlyChangedColumns(t *testing.T) {
	s, log := testStore()
	s.Login(Identity{XUID: "1", Name: "x", IP: "ip", DeviceID: "d"})
	*log = nil
	s.Update("1", func(p *Profile) {
		p.Kills++
		p.Settings.Chat = false
		p.Cosmetics.Cape = "Oldtralia"
	})
	s.Update("1", func(p *Profile) { p.Cosmetics.Cape = "" })
	got := []captured(*log)
	if len(got) != 4 {
		t.Fatalf("got %d queries", len(got))
	}
	check := func(i int, sql string, v any) {
		st := got[i].q.Stmts[0]
		if st.SQL != sql || st.Args[0] != v || st.Args[1] != "1" {
			t.Errorf("query %d = %q %v", i, st.SQL, st.Args)
		}
	}
	check(0, "UPDATE statistics t JOIN players p ON p.id = t.player_id SET t.kills = ? WHERE p.xuid = ?", 1)
	check(1, "UPDATE settings t JOIN players p ON p.id = t.player_id SET t.chat = ? WHERE p.xuid = ?", 0)
	check(2, "UPDATE cosmetics t JOIN players p ON p.id = t.player_id SET t.cape = ? WHERE p.xuid = ?", "Oldtralia")
	check(3, "UPDATE cosmetics t JOIN players p ON p.id = t.player_id SET t.cape = ? WHERE p.xuid = ?", nil)
}

func TestOtherNicknames(t *testing.T) {
	s, _ := testStore()
	s.Login(Identity{XUID: "1", Name: "main", IP: "ip1", DeviceID: "d1"})
	s.Login(Identity{XUID: "2", Name: "alt1", IP: "ip1", DeviceID: "d2"})
	s.Login(Identity{XUID: "3", Name: "alt2", IP: "ip3", DeviceID: "d1"})
	s.Login(Identity{XUID: "4", Name: "other", IP: "ip4", DeviceID: "d4"})
	if got := s.OtherNicknames("MAIN"); !slices.Equal(got, []string{"alt1", "alt2"}) {
		t.Fatalf("got %v", got)
	}
}

func TestPunishmentsFirstMatch(t *testing.T) {
	p := &Punishments{}
	p.AddBan(Ban{Nickname: "a", XUID: "1", Expiry: 5})
	p.AddBan(Ban{Nickname: "a", XUID: "1", Expiry: 9})
	if b, _ := p.BanByXUID("1"); b.Expiry != 5 {
		t.Fatal("first match")
	}
	p.RenameBans("1", "b")
	if _, ok := p.BanByNickname("b"); !ok {
		t.Fatal("rename")
	}
	p.RemoveBans("b")
	if p.Count() != 0 {
		t.Fatal("remove")
	}
}
