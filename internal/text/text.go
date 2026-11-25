package text

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	privateUse = regexp.MustCompile("[-]")
	format     = regexp.MustCompile("§[0-9a-w]")
	ansi       = regexp.MustCompile("\x1b[\\(\\][[0-9;\\[\\(]+[Bm]")
)

func Clean(s string, removeFormat bool) string {
	s = strings.ToValidUTF8(s, "?")
	s = privateUse.ReplaceAllString(s, "")
	if removeFormat {
		s = strings.ReplaceAll(format.ReplaceAllString(s, ""), "§", "")
	}
	return strings.ReplaceAll(ansi.ReplaceAllString(s, ""), "\x1b", "")
}

func Len(s string) int {
	return utf8.RuneCountInString(s)
}

const phpSpace = " \t\n\r\x00\x0b"

func Trim(s string) string {
	return strings.Trim(s, phpSpace)
}

func TrimLeft(s string) string {
	return strings.TrimLeft(s, phpSpace)
}

func SplitQuoteAware(line string) []string {
	var out []string
	for _, m := range quoteAware.FindAllStringSubmatch(line, -1) {
		for _, g := range m[1:] {
			if g != "" {
				out = append(out, unescape.ReplaceAllString(g, "$1"))
				break
			}
		}
	}
	return out
}

var (
	quoteAware = regexp.MustCompile(`"((?:\\.|[^\\"])*)"|(\S+)`)
	unescape   = regexp.MustCompile(`\\([\\"])`)
)
