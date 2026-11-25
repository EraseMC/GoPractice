package rank

import (
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

func TestPermissionsMatchPHP(t *testing.T) {
	path := filepath.Join("..", "..", "..", "Practice", "plugins", "Practice", "src", "antralia", "practice", "player", "rank", "RankManager.php")
	f, err := os.Open(path)
	if err != nil {
		t.Skip("php sources not available")
	}
	defer f.Close()
	data, _ := io.ReadAll(f)
	src := string(data)

	block := regexp.MustCompile(`(?s)public const (\w+)_PERMISSIONS = (\[.*?\]|self::(\w+)_PERMISSIONS);`)
	item := regexp.MustCompile(`"([^"]+)"`)
	php := map[string][]string{}
	for _, m := range block.FindAllStringSubmatch(src, -1) {
		if m[3] != "" {
			php[m[1]] = php[m[3]]
			continue
		}
		var list []string
		for _, it := range item.FindAllStringSubmatch(m[2], -1) {
			list = append(list, it[1])
		}
		php[m[1]] = list
	}
	names := map[string]string{
		Player: "PLAYER", Wayner: "WAYNER", Mystery: "MYSTERY", Gangster: "GANGSTER", Snowman: "SNOWMAN",
		Sosatel: "SOSATEL", Famous: "FAMOUS", FamousPlus: "FAMOUS_PLUS", Moderator: "MODERATOR", Admin: "ADMIN",
		Builder: "BUILDER", Staff: "STAFF", Manager: "MANAGER", Developer: "DEVELOPER", Owner: "OWNER",
	}
	for _, r := range All {
		want := php[names[r]]
		for i, p := range want {
			if p == "pocketmine.command.teleport" {
				want[i] = CmdTeleport
			}
		}
		if !slices.Equal(Permissions(r), want) {
			t.Errorf("%s: got %v want %v", r, Permissions(r), want)
		}
	}
}

func TestPriorityAndFormats(t *testing.T) {
	if Priority(Owner) != 100 || Priority(Famous) != 5 || Priority("Nope") != 0 {
		t.Fatal("priority")
	}
	if Color(Staff) != "§c" || Color(Developer) != "§m" {
		t.Fatal("rank colors must match chat and name tag")
	}
	if got := ChatFormat(Player, "Steve"); got != "§fSteve: §7" {
		t.Fatalf("player chat %q", got)
	}
	if got := ChatFormat(Sosatel, "Steve"); got != "§l§4S§co§6s§ga§et§de§5l§r §fSteve: §g" {
		t.Fatalf("sosatel chat %q", got)
	}
	if got := ChatFormat("Weird", "Steve"); got != "§8Steve: " {
		t.Fatalf("unknown chat %q", got)
	}
	if NameTag(FamousPlus, "Steve") != "§6Steve" || NameTag("Weird", "Steve") != "§8Steve" {
		t.Fatal("name tag")
	}
	if r, ok := Parse("MGR"); !ok || r != Manager {
		t.Fatal("parse alias")
	}
	if r, ok := Parse("famous_plus"); !ok || r != FamousPlus {
		t.Fatal("parse famous_plus")
	}
	if _, ok := Parse("god"); ok {
		t.Fatal("parse unknown")
	}
	if !Has(Player, CmdTell) || Has(Player, CmdRank) || !Has(Manager, CmdRank) || Has(Owner, CmdSneakPeek) {
		t.Fatal("has")
	}
	if strings.Join(EnumValues, ",") != "player,wayner,mystery,gangster,snowman,sosatel,famous,famous+,moderator,admin,builder,staff,manager,developer,owner" {
		t.Fatal("enum values")
	}
}
