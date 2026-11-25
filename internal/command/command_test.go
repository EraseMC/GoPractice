package command

import (
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/EraseMC/GoPractice/internal/chat"
	"github.com/EraseMC/GoPractice/internal/lang"
	"github.com/EraseMC/GoPractice/internal/punish"
	"github.com/EraseMC/GoPractice/internal/rank"
	"github.com/EraseMC/GoPractice/internal/state"
	"github.com/EraseMC/GoPractice/internal/store"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

func testMap(t *testing.T) (*Map, *store.Store) {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	bundle := lang.MustLoad()
	st := store.NewMemory()
	players := state.NewPlayers()
	pm := punish.New(bundle, st)
	svc := &Services{
		Log:     log,
		Lang:    bundle,
		Store:   st,
		Players: players,
		Chat:    chat.New(log, bundle, st, players, pm, func() uint64 { return 0 }),
		Punish:  pm,
		Started: time.Now().Add(-(2*86400 + 3*3600 + 4*60 + 5) * time.Second),
	}
	m := NewMap(svc)
	m.Register(ClearChat(), Emoji(), GameMode(), List(), Ping(), Platform(), Rank(), Reply(), Say(), Status(), Tell(), ToggleChat(), Teleport(), Stop())
	return m, st
}

func console(m *Map, line string) []string {
	var out []string
	m.Dispatch(&ConsoleSender{Print: func(l string) { out = append(out, l) }}, line)
	return out
}

func rconRun(m *Map, line string) string {
	s := &RconSender{}
	m.Dispatch(s, line)
	return s.Output()
}

func TestConsoleUnknownCommand(t *testing.T) {
	m, _ := testMap(t)
	got := console(m, "HELP me")
	if len(got) != 1 || got[0] != "Unknown command: help. Please check that the command exists and that you have permission to use it." {
		t.Fatalf("got %q", got)
	}
}

func TestSayRestrictions(t *testing.T) {
	m, _ := testMap(t)
	if got := rconRun(m, "say hi"); got != "Command can be executed only from console!\n" {
		t.Fatalf("rcon say %q", got)
	}
	if got := console(m, "say"); len(got) != 1 || got[0] != "Usage: /say <message>" {
		t.Fatalf("console say %q", got)
	}
	if got := console(m, "SAY hello there"); len(got) != 1 || got[0] != "SERVER: hello there" {
		t.Fatalf("console say %q", got)
	}
}

func TestConsoleList(t *testing.T) {
	m, _ := testMap(t)
	if got := console(m, "online"); len(got) != 2 || got[0] != "There are 0 players online:" || got[1] != "" {
		t.Fatalf("got %q", got)
	}
}

func TestRankFromConsoleAndRcon(t *testing.T) {
	m, st := testMap(t)
	st.Login(store.Identity{XUID: "1", Name: "Alice"})
	st.Login(store.Identity{XUID: "2", Name: "Boss"})
	st.Update("2", func(p *store.Profile) { p.Rank = rank.Manager })

	if got := console(m, "rank alice"); got[0] != "Usage: /rank <player> <rank>" {
		t.Fatalf("usage %q", got)
	}
	if got := console(m, "rank alice god"); got[0] != "Rank not found!" {
		t.Fatalf("not found %q", got)
	}
	if got := console(m, "rank nobody mod"); got[0] != "Player is not registered!" {
		t.Fatalf("not registered %q", got)
	}
	if got := rconRun(m, "rank boss player"); got != "You can't change this player's rank!\n" {
		t.Fatalf("rcon protected %q", got)
	}
	if got := rconRun(m, "rank alice mgr"); got != "You can't set this rank!\n" {
		t.Fatalf("rcon protected rank %q", got)
	}
	if got := rconRun(m, "rank alice mod"); got != "alice's rank has been changed to Moderator\n" {
		t.Fatalf("rcon set %q", got)
	}
	if got := rconRun(m, "rank alice wayner store"); got != "You can't change player's rank to this!\n" {
		t.Fatalf("rcon store %q", got)
	}
	if got := rconRun(m, "rank alice mod"); got != "Player already have this rank!\n" {
		t.Fatalf("rcon already %q", got)
	}
	if p, _ := st.ByNickname("alice"); p.Rank != rank.Moderator {
		t.Fatalf("rank not stored: %s", p.Rank)
	}
	if got := console(m, "rank boss owner"); got[0] != "boss's rank has been changed to Owner" {
		t.Fatalf("console set %q", got)
	}
}

func TestStatusFormat(t *testing.T) {
	m, st := testMap(t)
	st.Login(store.Identity{XUID: "1", Name: "Alice"})
	m.svc.Status = func() ServerStatus {
		return ServerStatus{TPS: 20, TickUsage: 1.5, AverageTPS: 19.95, AverageTickUsage: 0.25}
	}
	got := console(m, "status")
	want := []string{"Server status:", "Online: 0 (1 registered)", "Uptime: 2 days 3 hours 4 minutes 5 seconds", "Current TPS: 20 (1.5%)", "Average TPS: 19.95 (0.25%)"}
	for i, w := range want {
		if got[i] != w {
			t.Fatalf("line %d: got %q want %q", i, got[i], w)
		}
	}
	if !strings.HasPrefix(got[5], "Memory used: ") || !strings.HasSuffix(got[5], " MB") {
		t.Fatalf("memory %q", got[5])
	}
}

func TestTellAndPingFromConsole(t *testing.T) {
	m, _ := testMap(t)
	if got := console(m, "msg"); got[0] != "Usage: /tell <player> <message>" {
		t.Fatalf("tell usage %q", got)
	}
	if got := console(m, "w nobody hi"); got[0] != "Player not found!" {
		t.Fatalf("tell missing %q", got)
	}
	if got := console(m, "ping"); got[0] != "Usage: /ping <player>" {
		t.Fatalf("ping usage %q", got)
	}
	if got := console(m, "reply hi"); got[0] != "Only for players!" {
		t.Fatalf("reply %q", got)
	}
	if got := console(m, "tp a b"); got[0] != `Cannot find a player matching "a"` {
		t.Fatalf("tp %q", got)
	}
	if got := console(m, "tp 1 2 3"); got[0] != "Usage: /tp [target player] <destination player> OR /tp [target player] <x> <y> <z> [<y-rot> <x-rot>]" {
		t.Fatalf("tp usage %q", got)
	}
}

func TestToggleChat(t *testing.T) {
	m, _ := testMap(t)
	if got := console(m, "togglechat"); got[0] != "You have disabled the chat!" || m.svc.Chat.Enabled() {
		t.Fatalf("disable %q", got)
	}
	if got := console(m, "togglechat"); got[0] != "You have enabled the chat!" || !m.svc.Chat.Enabled() {
		t.Fatalf("enable %q", got)
	}
}

func TestAvailableCommandsPacket(t *testing.T) {
	m, _ := testMap(t)
	pk := m.Packet(lang.RU, func(perm string) bool { return rank.Has(rank.Player, perm) })
	names := map[string]protocol.Command{}
	for _, c := range pk.Commands {
		names[c.Name] = c
	}
	for _, hidden := range []string{"rank", "say", "status", "tp", "stop", "gamemode", "clearchat", "togglechat"} {
		if _, ok := names[hidden]; ok {
			t.Errorf("%s visible to player", hidden)
		}
	}
	tell, ok := names["tell"]
	if !ok {
		t.Fatal("tell missing")
	}
	if tell.Description != m.svc.Lang.Text(lang.RU, "tellCommandDescription") {
		t.Fatalf("description %q", tell.Description)
	}
	alias := pk.Enums[tell.AliasesOffset]
	var aliases []string
	for _, i := range alias.ValueIndices {
		aliases = append(aliases, pk.EnumValues[i])
	}
	if alias.Type != "TellAliases" || strings.Join(aliases, ",") != "whisper,message,msg,w,m,tell" {
		t.Fatalf("aliases %s %v", alias.Type, aliases)
	}
	if len(tell.Overloads) != 1 || len(tell.Overloads[0].Parameters) != 2 || tell.Overloads[0].Parameters[0].Type != protocol.CommandArgValid|protocol.CommandArgTypeTarget {
		t.Fatalf("overloads %+v", tell.Overloads)
	}
	emoji := names["emoji"]
	if len(emoji.Overloads) != 1 || len(emoji.Overloads[0].Parameters) != 0 {
		t.Fatalf("emoji overloads %+v", emoji.Overloads)
	}

	owner := m.Packet(lang.EN, func(perm string) bool { return rank.Has(rank.Owner, perm) })
	for _, c := range owner.Commands {
		if c.Name == "rank" {
			p := c.Overloads[0].Parameters[1]
			if p.Type&protocol.CommandArgEnum == 0 || owner.Enums[p.Type&0xffff].Type != "rank" {
				t.Fatalf("rank enum %+v", p)
			}
			return
		}
	}
	t.Fatal("rank missing for owner")
}

func TestHelpers(t *testing.T) {
	if PrettyUptime(59*time.Second) != "59 seconds" || PrettyUptime(61*time.Second) != "1 minutes 1 seconds" {
		t.Fatal("uptime")
	}
	if NumberFormat(1234567.891, 2) != "1,234,567.89" || NumberFormat(12.5, 2) != "12.50" {
		t.Fatal("number format")
	}
	if phpFloatval("12abc") != 12 || phpFloatval("abc") != 0 || phpFloatval("-1.5e1") != -15 {
		t.Fatal("floatval")
	}
	if FilterSQL(`a'b"c;d`+"`") != "abcd" {
		t.Fatal("sql filter")
	}
}
