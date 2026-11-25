package lang

import (
	"embed"
	"fmt"

	"github.com/pelletier/go-toml"
)

//go:embed en.toml ru.toml
var files embed.FS

type Lang string

const (
	EN Lang = "en"
	RU Lang = "ru"
)

var Languages = []Lang{EN, RU}

type Section string

const (
	Messages           Section = "messages"
	Ban                Section = "ban"
	BanFirst           Section = "ban_first"
	Cape               Section = "cape"
	CapeColored        Section = "cape_colored"
	Language           Section = "language"
	InputMode          Section = "input_mode"
	AntiCheat          Section = "anticheat"
	PotionColor        Section = "potion_color"
	PotionColorColored Section = "potion_color_colored"
)

var Sections = []Section{Messages, Ban, BanFirst, Cape, CapeColored, Language, InputMode, AntiCheat, PotionColor, PotionColorColored}

const unknownLanguage = "Unknown language"

type table struct {
	text     map[string]string
	lines    map[string][]string
	fallback string
}

type Bundle struct {
	tables map[Lang]map[Section]*table
}

func Parse(s string) (Lang, bool) {
	switch Lang(s) {
	case EN, RU:
		return Lang(s), true
	}
	return "", false
}

func FromLocale(locale string) Lang {
	if locale == "ru_RU" {
		return RU
	}
	return EN
}

func Load() (*Bundle, error) {
	b := &Bundle{tables: make(map[Lang]map[Section]*table, len(Languages))}
	for _, l := range Languages {
		data, err := files.ReadFile(string(l) + ".toml")
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", l, err)
		}
		tree, err := toml.LoadBytes(data)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", l, err)
		}
		sections := make(map[Section]*table, len(Sections))
		for _, s := range Sections {
			t := &table{text: map[string]string{}, lines: map[string][]string{}}
			fallback, ok := tree.GetPath([]string{"defaults", string(s)}).(string)
			if !ok {
				return nil, fmt.Errorf("%s: missing default for %s", l, s)
			}
			t.fallback = fallback
			sub, ok := tree.Get(string(s)).(*toml.Tree)
			if !ok {
				return nil, fmt.Errorf("%s: missing section %s", l, s)
			}
			for _, key := range sub.Keys() {
				switch v := sub.GetPath([]string{key}).(type) {
				case string:
					t.text[key] = v
				case []any:
					lines := make([]string, len(v))
					for i, item := range v {
						str, ok := item.(string)
						if !ok {
							return nil, fmt.Errorf("%s: %s.%s[%d] is not a string", l, s, key, i)
						}
						lines[i] = str
					}
					t.lines[key] = lines
				default:
					return nil, fmt.Errorf("%s: %s.%s has unsupported type %T", l, s, key, v)
				}
			}
			sections[s] = t
		}
		b.tables[l] = sections
	}
	return b, nil
}

func MustLoad() *Bundle {
	b, err := Load()
	if err != nil {
		panic(err)
	}
	return b
}

func (b *Bundle) table(l Lang, s Section) *table {
	sections, ok := b.tables[l]
	if !ok {
		return nil
	}
	return sections[s]
}

func (b *Bundle) In(l Lang, s Section, key string) string {
	t := b.table(l, s)
	if t == nil {
		return unknownLanguage
	}
	if v, ok := t.text[key]; ok {
		return v
	}
	return t.fallback
}

func (b *Bundle) Inf(l Lang, s Section, key string, args ...any) string {
	return Sprintf(b.In(l, s, key), args...)
}

func (b *Bundle) Text(l Lang, key string) string {
	return b.In(l, Messages, key)
}

func (b *Bundle) Textf(l Lang, key string, args ...any) string {
	return Sprintf(b.Text(l, key), args...)
}

func (b *Bundle) Lines(l Lang, key string) []string {
	t := b.table(l, Messages)
	if t == nil {
		return nil
	}
	return t.lines[key]
}

func (b *Bundle) Keys(l Lang, s Section) (text []string, lines []string) {
	t := b.table(l, s)
	if t == nil {
		return nil, nil
	}
	for k := range t.text {
		text = append(text, k)
	}
	for k := range t.lines {
		lines = append(lines, k)
	}
	return text, lines
}

func Sprintf(format string, args ...any) string {
	if len(args) == 0 {
		return format
	}
	conv := make([]any, len(args))
	for i, a := range args {
		switch v := a.(type) {
		case string:
			conv[i] = v
		case fmt.Stringer:
			conv[i] = v.String()
		default:
			conv[i] = fmt.Sprint(v)
		}
	}
	return fmt.Sprintf(format, conv...)
}
