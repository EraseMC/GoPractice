package rank

import "strings"

const (
	Player     = "Player"
	Wayner     = "Wayner"
	Mystery    = "Mystery"
	Gangster   = "Gangster"
	Snowman    = "Snowman"
	Sosatel    = "Sosatel"
	Famous     = "Famous"
	FamousPlus = "Famous+"
	Moderator  = "Moderator"
	Admin      = "Admin"
	Builder    = "Builder"
	Staff      = "Staff"
	Manager    = "Manager"
	Developer  = "Developer"
	Owner      = "Owner"

	Default = Player
)

var All = []string{Player, Wayner, Mystery, Gangster, Snowman, Sosatel, Famous, FamousPlus, Moderator, Admin, Builder, Staff, Manager, Developer, Owner}

type info struct {
	priority  int
	colored   string
	color     string
	chat      string
	chatColor string
	tag       string
	perms     []string
}

var ranks = map[string]info{
	Player:     {1, "§7Player", "§7", "", "§7", "§f", nil},
	Wayner:     {2, "§2Wayner", "§2", "§l§2Wayner§r ", "§2", "§2", wayner},
	Mystery:    {3, "§5Mystery", "§5", "§l§5Mystery§r ", "§5", "§5", mystery},
	Gangster:   {4, "§aGangster", "§a", "§l§aGangster§r ", "§a", "§a", gangster},
	Snowman:    {5, "§fSnow§9man", "§9", "§l§fSnow§9man§r ", "§9", "§9", gangster},
	Sosatel:    {5, "§4S§co§6s§ga§et§de§5l", "§g", "§l§4S§co§6s§ga§et§de§5l§r ", "§g", "§g", gangster},
	Famous:     {5, "§6Famous", "§6", "§l§6Famous§r ", "§6", "§6", gangster},
	FamousPlus: {6, "§6Famous§c+", "§6", "§l§6Famous§c+§r ", "§6", "§6", famousPlus},
	Moderator:  {7, "§bModerator", "§b", "§l§bModerator§r ", "§b", "§b", moderator},
	Admin:      {8, "§3Admin", "§3", "§l§3Admin§r ", "§3", "§3", admin},
	Builder:    {9, "§eBuilder", "§e", "§l§eBuilder§r ", "§e", "§e", builder},
	Staff:      {10, "§cStaff", "§c", "§l§cStaff§r ", "§c", "§c", builder},
	Manager:    {50, "§dManager", "§d", "§l§dManager§r ", "§d", "§d", manager},
	Developer:  {75, "§mDeveloper", "§m", "§l§mDeveloper§r ", "§m", "§m", developer},
	Owner:      {100, "§4Owner", "§4", "§l§4Owner§r ", "§4", "§4", owner},
}

func Valid(r string) bool {
	_, ok := ranks[r]
	return ok
}

func Priority(r string) int {
	return ranks[r].priority
}

func Colored(r string) string {
	if i, ok := ranks[r]; ok {
		return i.colored
	}
	return "§0Unknown"
}

func Color(r string) string {
	if i, ok := ranks[r]; ok {
		return i.color
	}
	return "§0"
}

func ChatFormat(r, nickname string) string {
	i, ok := ranks[r]
	if !ok {
		return "§8" + nickname + ": "
	}
	return i.chat + "§f" + nickname + ": " + i.chatColor
}

func NameTag(r, nickname string) string {
	i, ok := ranks[r]
	if !ok {
		return "§8" + nickname
	}
	return i.tag + nickname
}

func Permissions(r string) []string {
	return ranks[r].perms
}

func Has(r, perm string) bool {
	if everyone[perm] {
		return true
	}
	for _, p := range ranks[r].perms {
		if p == perm {
			return true
		}
	}
	return false
}

func Parse(arg string) (string, bool) {
	switch strings.ToLower(arg) {
	case "player", "default":
		return Player, true
	case "wayner":
		return Wayner, true
	case "mystery":
		return Mystery, true
	case "gangster":
		return Gangster, true
	case "snowman":
		return Snowman, true
	case "sosatel":
		return Sosatel, true
	case "famous":
		return Famous, true
	case "famous+", "famous_plus":
		return FamousPlus, true
	case "moderator", "mod":
		return Moderator, true
	case "admin", "adm":
		return Admin, true
	case "builder", "anb":
		return Builder, true
	case "staff":
		return Staff, true
	case "manager", "mgr":
		return Manager, true
	case "developer", "dev":
		return Developer, true
	case "owner", "own":
		return Owner, true
	}
	return "Unknown", false
}

var EnumValues = []string{"player", "wayner", "mystery", "gangster", "snowman", "sosatel", "famous", "famous+", "moderator", "admin", "builder", "staff", "manager", "developer", "owner"}
