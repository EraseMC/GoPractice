package store

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/EraseMC/GoPractice/internal/db"
)

type Identity struct {
	XUID     string
	Name     string
	IP       string
	DeviceID string
	Locale   string
}

type Store struct {
	pool *db.Pool
	sink func(q db.Query, shard string)

	mu         sync.RWMutex
	values     map[string]*Profile
	byNickname map[string]string
	byIP       map[string]map[string]struct{}
	byDevice   map[string]map[string]struct{}
	seq        uint64

	Punishments *Punishments
}

func newStore() *Store {
	return &Store{
		values:     map[string]*Profile{},
		byNickname: map[string]string{},
		byIP:       map[string]map[string]struct{}{},
		byDevice:   map[string]map[string]struct{}{},
	}
}

func Load(ctx context.Context, d *sql.DB) (*Store, error) {
	s := newStore()
	if err := s.loadPlayers(ctx, d); err != nil {
		return nil, fmt.Errorf("load players: %w", err)
	}
	p, err := loadPunishments(ctx, d)
	if err != nil {
		return nil, fmt.Errorf("load punishments: %w", err)
	}
	s.Punishments = p
	return s, nil
}

func (s *Store) Attach(pool *db.Pool) {
	s.mu.Lock()
	s.pool = pool
	s.mu.Unlock()
	s.Punishments.attach(pool)
}

func (s *Store) loadPlayers(ctx context.Context, d *sql.DB) error {
	rows, err := d.QueryContext(ctx,
		"SELECT p.xuid, p.name, p.ip, p.device_id, p.player_rank, p.language, "+
			"s.kills, s.deaths, s.play_time, "+
			"st.scoreboard, st.cps, st.autosprint, st.chat, st.hide_players, st.arena_respawn, st.lightning, "+
			"c.cape, c.tag, c.fly, c.potion_color, c.capybara "+
			"FROM players p "+
			"LEFT JOIN statistics s ON s.player_id = p.id "+
			"LEFT JOIN settings st ON st.player_id = p.id "+
			"LEFT JOIN cosmetics c ON c.player_id = p.id")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			xuid, name, ip, device, rank, locale string
			kills, deaths, playTime              sql.NullInt64
			settings                             [7]sql.NullInt64
			cape, tag, potion                    sql.NullString
			fly, capybara                        sql.NullInt64
		)
		if err := rows.Scan(&xuid, &name, &ip, &device, &rank, &locale,
			&kills, &deaths, &playTime,
			&settings[0], &settings[1], &settings[2], &settings[3], &settings[4], &settings[5], &settings[6],
			&cape, &tag, &fly, &potion, &capybara); err != nil {
			return err
		}
		p := defaults(xuid, name, ip, device, rank, locale)
		p.Kills, p.Deaths, p.PlayTime = int(kills.Int64), int(deaths.Int64), int(playTime.Int64)
		targets := [7]*bool{&p.Settings.Scoreboard, &p.Settings.CPS, &p.Settings.AutoSprint, &p.Settings.Chat, &p.Settings.HidePlayers, &p.Settings.ArenaRespawn, &p.Settings.Lightning}
		for i, v := range settings {
			if v.Valid {
				*targets[i] = v.Int64 == 1
			}
		}
		p.Cosmetics = Cosmetics{
			Cape:        cape.String,
			Tag:         tag.String,
			Fly:         fly.Int64 == 1,
			PotionColor: potion.String,
			Capybara:    capybara.Int64 == 1,
		}
		s.remember(&p)
	}
	return rows.Err()
}

func (s *Store) remember(p *Profile) {
	s.seq++
	p.seq = s.seq
	s.values[p.XUID] = p
	s.byNickname[p.Nickname] = p.XUID
	addIndex(s.byIP, p.IP, p.XUID)
	addIndex(s.byDevice, p.DeviceID, p.XUID)
}

func addIndex(m map[string]map[string]struct{}, key, xuid string) {
	set, ok := m[key]
	if !ok {
		set = map[string]struct{}{}
		m[key] = set
	}
	set[xuid] = struct{}{}
}

func removeIndex(m map[string]map[string]struct{}, key, xuid string) {
	set, ok := m[key]
	if !ok {
		return
	}
	delete(set, xuid)
	if len(set) == 0 {
		delete(m, key)
	}
}

func (s *Store) reindex(xuid string, old, cur *Profile) {
	if old.Nickname != cur.Nickname {
		if s.byNickname[old.Nickname] == xuid {
			delete(s.byNickname, old.Nickname)
		}
		s.byNickname[cur.Nickname] = xuid
	}
	if old.IP != cur.IP {
		removeIndex(s.byIP, old.IP, xuid)
		addIndex(s.byIP, cur.IP, xuid)
	}
	if old.DeviceID != cur.DeviceID {
		removeIndex(s.byDevice, old.DeviceID, xuid)
		addIndex(s.byDevice, cur.DeviceID, xuid)
	}
}

func (s *Store) execute(q db.Query, shard string) {
	if s.sink != nil {
		s.sink(q, shard)
		return
	}
	if s.pool != nil {
		s.pool.Execute(q, shard)
	}
}

const ownerSubquery = "(SELECT id FROM players WHERE xuid = ?)"

func satellites(xuid string) []db.Stmt {
	return []db.Stmt{
		{SQL: "INSERT IGNORE INTO statistics (player_id) VALUES (" + ownerSubquery + ")", Args: []any{xuid}},
		{SQL: "INSERT IGNORE INTO settings (player_id) VALUES (" + ownerSubquery + ")", Args: []any{xuid}},
		{SQL: "INSERT IGNORE INTO cosmetics (player_id) VALUES (" + ownerSubquery + ")", Args: []any{xuid}},
	}
}

func (s *Store) Login(id Identity) Profile {
	s.mu.Lock()
	defer s.mu.Unlock()

	nickname := strings.ToLower(id.Name)
	p, ok := s.values[id.XUID]
	if ok {
		s.execute(db.Transaction(satellites(id.XUID)...), id.XUID)
	} else {
		locale := "en"
		if id.Locale == "ru_RU" {
			locale = "ru"
		}
		np := defaults(id.XUID, nickname, id.IP, id.DeviceID, DefaultRank, locale)
		s.remember(&np)
		p = &np
		stmts := append([]db.Stmt{{
			SQL: "INSERT INTO players (xuid, name, ip, device_id, player_rank, language) VALUES (?, ?, ?, ?, ?, ?) " +
				"ON DUPLICATE KEY UPDATE name = VALUES(name), ip = VALUES(ip), device_id = VALUES(device_id)",
			Args: []any{np.XUID, np.Nickname, np.IP, np.DeviceID, np.Rank, np.Locale},
		}}, satellites(id.XUID)...)
		s.execute(db.Transaction(stmts...), id.XUID)
	}

	s.updateLocked(id.XUID, func(p *Profile) {
		p.Nickname = nickname
		p.IP = id.IP
		p.DeviceID = id.DeviceID
	})
	return *p
}

func (s *Store) Update(xuid string, f func(p *Profile)) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.updateLocked(xuid, f)
}

func (s *Store) UpdateByNickname(nickname string, f func(p *Profile)) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	xuid, ok := s.byNickname[strings.ToLower(nickname)]
	if !ok {
		return false
	}
	return s.updateLocked(xuid, f)
}

func (s *Store) updateLocked(xuid string, f func(p *Profile)) bool {
	p, ok := s.values[xuid]
	if !ok {
		return false
	}
	old := *p
	f(p)
	p.XUID, p.seq = old.XUID, old.seq
	s.reindex(xuid, &old, p)
	for _, c := range columns {
		before, after := c.value(&old), c.value(p)
		if before == after {
			continue
		}
		sqlText, args := c.statement(after, xuid)
		s.execute(db.Make(sqlText, args...), xuid)
	}
	return true
}

func (s *Store) ByXUID(xuid string) (Profile, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.values[xuid]
	if !ok {
		return Missing, false
	}
	return *p, true
}

func (s *Store) ByNickname(nickname string) (Profile, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	xuid, ok := s.byNickname[strings.ToLower(nickname)]
	if !ok {
		return Missing, false
	}
	return *s.values[xuid], true
}

func (s *Store) Registered(nickname string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.byNickname[strings.ToLower(nickname)]
	return ok
}

func (s *Store) RegisteredXUID(xuid string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.values[xuid]
	return ok
}

func (s *Store) XUIDByNickname(nickname string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if xuid, ok := s.byNickname[strings.ToLower(nickname)]; ok {
		return xuid
	}
	return "0"
}

func (s *Store) IPByNickname(nickname string) string {
	p, _ := s.ByNickname(nickname)
	return p.IP
}

func (s *Store) OtherNicknames(nickname string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	nickname = strings.ToLower(nickname)
	xuid, ok := s.byNickname[nickname]
	if !ok {
		return nil
	}
	p := s.values[xuid]
	related := map[string]*Profile{}
	for x := range s.byIP[p.IP] {
		related[x] = s.values[x]
	}
	for x := range s.byDevice[p.DeviceID] {
		related[x] = s.values[x]
	}
	list := make([]*Profile, 0, len(related))
	for _, r := range related {
		if r != nil {
			list = append(list, r)
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].seq < list[j].seq })
	seen := map[string]bool{}
	var out []string
	for _, r := range list {
		if r.Nickname == "" || r.Nickname == nickname || seen[r.Nickname] {
			continue
		}
		seen[r.Nickname] = true
		out = append(out, r.Nickname)
	}
	return out
}

func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.values)
}

func (s *Store) Each(f func(p Profile)) {
	s.mu.RLock()
	list := make([]Profile, 0, len(s.values))
	for _, p := range s.values {
		list = append(list, *p)
	}
	s.mu.RUnlock()
	for _, p := range list {
		f(p)
	}
}

type Stat string

const (
	StatKills    Stat = "kills"
	StatDeaths   Stat = "deaths"
	StatPlayTime Stat = "play_time"
)

type TopEntry struct {
	Nickname string
	Value    int
}

func (s *Store) FetchTop(stat Stat, limit int) <-chan []TopEntry {
	out := make(chan []TopEntry, 1)
	switch stat {
	case StatKills, StatDeaths, StatPlayTime:
	default:
		panic(fmt.Sprintf("unknown statistic %s", stat))
	}
	if s.pool == nil {
		out <- nil
		return out
	}
	res := s.pool.Query(db.Make(fmt.Sprintf(
		"SELECT p.name AS nickname, s.%s AS value FROM statistics s JOIN players p ON p.id = s.player_id ORDER BY s.%s DESC LIMIT %d",
		stat, stat, max(1, limit))), "")
	go func() {
		r := <-res
		if r.Err != nil {
			out <- nil
			return
		}
		entries := make([]TopEntry, 0, len(r.Rows))
		for _, row := range r.Rows {
			entries = append(entries, TopEntry{Nickname: fmt.Sprint(row["nickname"]), Value: toInt(row["value"])})
		}
		out <- entries
	}()
	return out
}

func toInt(v any) int {
	switch n := v.(type) {
	case int64:
		return int(n)
	case uint64:
		return int(n)
	case int:
		return n
	case string:
		var i int
		_, _ = fmt.Sscan(n, &i)
		return i
	}
	return 0
}
