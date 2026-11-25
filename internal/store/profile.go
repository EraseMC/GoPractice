package store

const DefaultRank = "Player"

type Settings struct {
	Scoreboard   bool
	CPS          bool
	AutoSprint   bool
	Chat         bool
	HidePlayers  bool
	ArenaRespawn bool
	Lightning    bool
}

type Cosmetics struct {
	Cape        string
	Tag         string
	Fly         bool
	PotionColor string
	Capybara    bool
}

type Profile struct {
	Nickname string
	XUID     string
	IP       string
	DeviceID string
	Rank     string
	Locale   string

	Kills    int
	Deaths   int
	PlayTime int

	Settings  Settings
	Cosmetics Cosmetics

	seq uint64
}

var Missing = Profile{Nickname: "Steve", XUID: "0", IP: "0.0.0.0", DeviceID: "0"}

func defaults(xuid, nickname, ip, deviceID, rank, locale string) Profile {
	return Profile{
		Nickname: nickname,
		XUID:     xuid,
		IP:       ip,
		DeviceID: deviceID,
		Rank:     rank,
		Locale:   locale,
		Settings: Settings{
			Scoreboard:  true,
			Chat:        true,
			HidePlayers: true,
			Lightning:   true,
		},
	}
}

type column struct {
	table string
	name  string
	value func(*Profile) any
}

func flag(b bool) any {
	if b {
		return 1
	}
	return 0
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

var columns = []column{
	{"players", "name", func(p *Profile) any { return p.Nickname }},
	{"players", "ip", func(p *Profile) any { return p.IP }},
	{"players", "device_id", func(p *Profile) any { return p.DeviceID }},
	{"players", "player_rank", func(p *Profile) any { return p.Rank }},
	{"players", "language", func(p *Profile) any { return p.Locale }},

	{"statistics", "kills", func(p *Profile) any { return max(0, p.Kills) }},
	{"statistics", "deaths", func(p *Profile) any { return max(0, p.Deaths) }},
	{"statistics", "play_time", func(p *Profile) any { return max(0, p.PlayTime) }},

	{"settings", "scoreboard", func(p *Profile) any { return flag(p.Settings.Scoreboard) }},
	{"settings", "cps", func(p *Profile) any { return flag(p.Settings.CPS) }},
	{"settings", "autosprint", func(p *Profile) any { return flag(p.Settings.AutoSprint) }},
	{"settings", "chat", func(p *Profile) any { return flag(p.Settings.Chat) }},
	{"settings", "hide_players", func(p *Profile) any { return flag(p.Settings.HidePlayers) }},
	{"settings", "arena_respawn", func(p *Profile) any { return flag(p.Settings.ArenaRespawn) }},
	{"settings", "lightning", func(p *Profile) any { return flag(p.Settings.Lightning) }},

	{"cosmetics", "cape", func(p *Profile) any { return nullable(p.Cosmetics.Cape) }},
	{"cosmetics", "tag", func(p *Profile) any { return nullable(p.Cosmetics.Tag) }},
	{"cosmetics", "fly", func(p *Profile) any { return flag(p.Cosmetics.Fly) }},
	{"cosmetics", "potion_color", func(p *Profile) any { return nullable(p.Cosmetics.PotionColor) }},
	{"cosmetics", "capybara", func(p *Profile) any { return flag(p.Cosmetics.Capybara) }},
}

func (c column) statement(v any, xuid string) (string, []any) {
	if c.table == "players" {
		return "UPDATE players SET " + c.name + " = ? WHERE xuid = ?", []any{v, xuid}
	}
	return "UPDATE " + c.table + " t JOIN players p ON p.id = t.player_id SET t." + c.name + " = ? WHERE p.xuid = ?", []any{v, xuid}
}
