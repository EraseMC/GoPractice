package command

import (
	"math"
	"strings"
	"unicode"

	"github.com/EraseMC/GoPractice/internal/lang"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type enumTable struct {
	pk      *packet.AvailableCommands
	values  map[string]uint32
	indices map[string]uint32
}

func (t *enumTable) add(name string, values []string) uint32 {
	if i, ok := t.indices[name]; ok {
		return i
	}
	e := protocol.CommandEnum{Type: name}
	for _, v := range values {
		idx, ok := t.values[v]
		if !ok {
			idx = uint32(len(t.pk.EnumValues))
			t.values[v] = idx
			t.pk.EnumValues = append(t.pk.EnumValues, v)
		}
		e.ValueIndices = append(e.ValueIndices, idx)
	}
	i := uint32(len(t.pk.Enums))
	t.indices[name] = i
	t.pk.Enums = append(t.pk.Enums, e)
	return i
}

func (m *Map) Packet(l lang.Lang, has func(perm string) bool) *packet.AvailableCommands {
	pk := &packet.AvailableCommands{}
	t := &enumTable{pk: pk, values: map[string]uint32{}, indices: map[string]uint32{}}
	for _, c := range m.Commands() {
		if c.Permission != "" && !has(c.Permission) {
			continue
		}
		desc := c.Description
		if c.DescriptionKey != "" {
			desc = m.svc.Lang.Text(l, c.DescriptionKey)
		}
		aliasOffset := uint32(math.MaxUint32)
		if len(c.Aliases) > 0 {
			aliases := append([]string{}, c.Aliases...)
			found := false
			for _, a := range aliases {
				if a == c.Name {
					found = true
				}
			}
			if !found {
				aliases = append(aliases, c.Name)
			}
			aliasOffset = t.add(ucfirst(c.Name)+"Aliases", aliases)
		}
		pk.Commands = append(pk.Commands, protocol.Command{
			Name:            c.Name,
			Description:     desc,
			PermissionLevel: protocol.CommandPermissionLevelAny,
			AliasesOffset:   aliasOffset,
			Overloads:       overloads(t, c),
		})
	}
	return pk
}

func overloads(t *enumTable, c *Command) []protocol.CommandOverload {
	if c.Overloads == nil {
		return []protocol.CommandOverload{{Parameters: []protocol.CommandParameter{{
			Name:     "args",
			Type:     protocol.CommandArgValid | protocol.CommandArgTypeRawText,
			Optional: true,
		}}}}
	}
	out := make([]protocol.CommandOverload, 0, len(c.Overloads))
	for _, params := range c.Overloads {
		o := protocol.CommandOverload{Parameters: []protocol.CommandParameter{}}
		for _, p := range params {
			typ := protocol.CommandArgValid | p.Type
			if p.Enum != "" && len(p.Values) > 0 {
				typ = protocol.CommandArgValid | protocol.CommandArgEnum | t.add(p.Enum, p.Values)
			}
			o.Parameters = append(o.Parameters, protocol.CommandParameter{Name: p.Name, Type: typ, Optional: p.Optional})
		}
		out = append(out, o)
	}
	if len(out) == 0 {
		out = append(out, protocol.CommandOverload{Parameters: []protocol.CommandParameter{}})
	}
	return out
}

func ucfirst(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

func EmptyOverload() [][]Param {
	return [][]Param{{}}
}

func lower(s string) string {
	return strings.ToLower(s)
}
