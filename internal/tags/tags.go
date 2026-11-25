package tags

var (
	Silver   = []string{"E-GIRL", "BESTWW", "GOD", "LIT", "LUNAR", "STARDUST", "CHOPPA", "CLOUTLORD"}
	Golden   = []string{"OLDTRALIA", "JUGG", "FINESSE", "MEMPHIS", "LEANDOER", "CAPYBARA", "SHOOTA", "L", "W"}
	Platinum = []string{"SANTA", "TRIPPY", "MAFIA", "ALCOHOLIC", "BLATANT", "JUICY", "OG", "BROKE", "INDIAN", "GOAT", "INVALIDCOLOR"}
)

var colored = map[string]string{
	"E-GIRL":       "§dE-GIRL",
	"BESTWW":       "§cBESTWW",
	"GOD":          "§6GOD",
	"LIT":          "§eLIT",
	"LUNAR":        "§dLUNAR",
	"STARDUST":     "§7STARDUST",
	"CHOPPA":       "§8CHOPPA",
	"CLOUTLORD":    "§fCLOUTLORD",
	"OLDTRALIA":    "§aOLDTRALIA",
	"JUGG":         "§2JUGG",
	"FINESSE":      "§bFINESSE",
	"MEMPHIS":      "§cMEMPHIS",
	"LEANDOER":     "§5LEANDOER",
	"CAPYBARA":     "§gCAPYBARA",
	"SHOOTA":       "§3SHOOTA",
	"L":            "§6L",
	"W":            "§4W",
	"SANTA":        "§cSANTA",
	"TRIPPY":       "§dTRIPPY",
	"MAFIA":        "§4MAFIA",
	"ALCOHOLIC":    "§3ALCOHOLIC",
	"BLATANT":      "§bBLATANT",
	"JUICY":        "§eJUICY",
	"OG":           "§6OG",
	"BROKE":        "§gBROKE",
	"INDIAN":       "§2INDIAN",
	"GOAT":         "§5GOAT",
	"INVALIDCOLOR": "§0INVALIDCOLOR",
}

func Name(stored string) string {
	if stored == "" {
		return "default"
	}
	return stored
}

func Colored(stored string) string {
	if c, ok := colored[Name(stored)]; ok {
		return c
	}
	return "§0???"
}

func ColoredForForm(stored string) string {
	if c, ok := colored[Name(stored)]; ok {
		return c
	}
	return "§7-"
}
