package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"sync"

	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/player/form"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/google/uuid"
)

type ImageType int

const (
	ImageNone ImageType = -1
	ImagePath ImageType = 0
	ImageURL  ImageType = 1
)

type Form interface {
	json.Marshaler
	submit(p *player.Player, tx *world.Tx, data []byte) error
}

type Forms struct {
	mu      sync.Mutex
	looking map[uuid.UUID]bool
}

func NewForms() *Forms {
	return &Forms{looking: map[uuid.UUID]bool{}}
}

func (f *Forms) Send(p *player.Player, fm Form) {
	id := p.UUID()
	f.mu.Lock()
	if f.looking[id] {
		f.mu.Unlock()
		return
	}
	f.looking[id] = true
	f.mu.Unlock()
	p.SendForm(guarded{forms: f, id: id, form: fm})
}

func (f *Forms) Looking(id uuid.UUID) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.looking[id]
}

func (f *Forms) Reset(id uuid.UUID) {
	f.mu.Lock()
	delete(f.looking, id)
	f.mu.Unlock()
}

type guarded struct {
	forms *Forms
	id    uuid.UUID
	form  Form
}

func (g guarded) MarshalJSON() ([]byte, error) {
	return g.form.MarshalJSON()
}

func (g guarded) SubmitJSON(b []byte, submitter form.Submitter, tx *world.Tx) error {
	g.forms.Reset(g.id)
	p, ok := submitter.(*player.Player)
	if !ok {
		return nil
	}
	_ = g.form.submit(p, tx, b)
	return nil
}

type button struct {
	Text  string `json:"text"`
	Image *image `json:"image,omitempty"`
}

type image struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

type Simple struct {
	title    string
	content  string
	buttons  []button
	callback func(p *player.Player, tx *world.Tx, data *int)
}

func NewSimple(callback func(p *player.Player, tx *world.Tx, data *int)) *Simple {
	return &Simple{callback: callback}
}

func (s *Simple) SetTitle(t string)   { s.title = t }
func (s *Simple) Title() string       { return s.title }
func (s *Simple) SetContent(c string) { s.content = c }
func (s *Simple) Content() string     { return s.content }

func (s *Simple) AddButton(text string, kind ImageType, path string) {
	b := button{Text: text}
	switch kind {
	case ImagePath:
		b.Image = &image{Type: "path", Data: path}
	case ImageURL:
		b.Image = &image{Type: "url", Data: path}
	case ImageNone:
	default:
		panic(fmt.Sprintf("unknown image type: %d", kind))
	}
	s.buttons = append(s.buttons, b)
}

func (s *Simple) ButtonCount() int { return len(s.buttons) }

func (s *Simple) MarshalJSON() ([]byte, error) {
	buttons := s.buttons
	if buttons == nil {
		buttons = []button{}
	}
	return json.Marshal(struct {
		Type    string   `json:"type"`
		Title   string   `json:"title"`
		Content string   `json:"content"`
		Buttons []button `json:"buttons"`
	}{"form", s.title, s.content, buttons})
}

func (s *Simple) submit(p *player.Player, tx *world.Tx, data []byte) error {
	var choice *int
	if v, ok := decode(data); ok && v != nil {
		n, ok := toInt(v)
		if !ok || n < 0 || n >= len(s.buttons) {
			return fmt.Errorf("button %v does not exist", v)
		}
		choice = &n
	}
	if s.callback != nil {
		s.callback(p, tx, choice)
	}
	return nil
}

type Modal struct {
	title    string
	content  string
	button1  string
	button2  string
	callback func(p *player.Player, tx *world.Tx, data *bool)
}

func NewModal(callback func(p *player.Player, tx *world.Tx, data *bool)) *Modal {
	return &Modal{callback: callback}
}

func (m *Modal) SetTitle(t string)        { m.title = t }
func (m *Modal) Title() string            { return m.title }
func (m *Modal) SetContent(c string)      { m.content = c }
func (m *Modal) Content() string          { return m.content }
func (m *Modal) SetFirstButton(t string)  { m.button1 = t }
func (m *Modal) FirstButton() string      { return m.button1 }
func (m *Modal) SetSecondButton(t string) { m.button2 = t }
func (m *Modal) SecondButton() string     { return m.button2 }

func (m *Modal) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type    string `json:"type"`
		Title   string `json:"title"`
		Content string `json:"content"`
		Button1 string `json:"button1"`
		Button2 string `json:"button2"`
	}{"modal", m.title, m.content, m.button1, m.button2})
}

func (m *Modal) submit(p *player.Player, tx *world.Tx, data []byte) error {
	var choice *bool
	if v, ok := decode(data); ok && v != nil {
		b := truthy(v)
		choice = &b
	}
	if m.callback != nil {
		m.callback(p, tx, choice)
	}
	return nil
}

type element struct {
	body     map[string]any
	validate func(v any) bool
	label    bool
}

type Custom struct {
	title    string
	elements []element
	callback func(p *player.Player, tx *world.Tx, data []any)
}

func NewCustom(callback func(p *player.Player, tx *world.Tx, data []any)) *Custom {
	return &Custom{callback: callback}
}

func (c *Custom) SetTitle(t string) { c.title = t }
func (c *Custom) Title() string     { return c.title }

func (c *Custom) AddLabel(text string) {
	c.elements = append(c.elements, element{
		body:     map[string]any{"type": "label", "text": text},
		validate: func(v any) bool { return v == nil },
		label:    true,
	})
}

func (c *Custom) AddToggle(text string, def *bool) {
	body := map[string]any{"type": "toggle", "text": text}
	if def != nil {
		body["default"] = *def
	}
	c.elements = append(c.elements, element{body: body, validate: func(v any) bool { _, ok := v.(bool); return ok }})
}

func (c *Custom) AddSlider(text string, min, max, step, def int) {
	body := map[string]any{"type": "slider", "text": text, "min": min, "max": max}
	if step != -1 {
		body["step"] = step
	}
	if def != -1 {
		body["default"] = def
	}
	c.elements = append(c.elements, element{body: body, validate: func(v any) bool {
		f, ok := toFloat(v)
		return ok && f >= float64(min) && f <= float64(max)
	}})
}

func (c *Custom) AddStepSlider(text string, steps []string, defIndex int) {
	body := map[string]any{"type": "step_slider", "text": text, "steps": nonNil(steps)}
	if defIndex != -1 {
		body["default"] = defIndex
	}
	c.elements = append(c.elements, element{body: body, validate: indexIn(len(steps))})
}

func (c *Custom) AddDropdown(text string, options []string, def *int) {
	body := map[string]any{"type": "dropdown", "text": text, "options": nonNil(options), "default": nil}
	if def != nil {
		body["default"] = *def
	}
	c.elements = append(c.elements, element{body: body, validate: indexIn(len(options))})
}

func (c *Custom) AddInput(text, placeholder string, def *string) {
	body := map[string]any{"type": "input", "text": text, "placeholder": placeholder, "default": nil}
	if def != nil {
		body["default"] = *def
	}
	c.elements = append(c.elements, element{body: body, validate: func(v any) bool { _, ok := v.(string); return ok }})
}

func (c *Custom) MarshalJSON() ([]byte, error) {
	content := make([]map[string]any, len(c.elements))
	for i, e := range c.elements {
		content[i] = e.body
	}
	return json.Marshal(struct {
		Type    string           `json:"type"`
		Title   string           `json:"title"`
		Content []map[string]any `json:"content"`
	}{"custom_form", c.title, content})
}

func (c *Custom) submit(p *player.Player, tx *world.Tx, data []byte) error {
	v, ok := decode(data)
	if !ok {
		return fmt.Errorf("invalid custom form response")
	}
	if v == nil {
		if c.callback != nil {
			c.callback(p, tx, nil)
		}
		return nil
	}
	arr, ok := v.([]any)
	if !ok {
		return fmt.Errorf("expected an array response, got %T", v)
	}
	if len(arr) != len(c.elements) {
		arr = c.expandLabels(arr)
	}
	if len(arr) != len(c.elements) {
		return fmt.Errorf("expected an array response with the size %d, got %d", len(c.elements), len(arr))
	}
	out := make([]any, len(arr))
	for i, val := range arr {
		if !c.elements[i].validate(val) {
			return fmt.Errorf("invalid type given for element %d", i)
		}
		if n, ok := val.(json.Number); ok {
			if iv, err := n.Int64(); err == nil {
				out[i] = int(iv)
			} else {
				f, _ := n.Float64()
				out[i] = f
			}
			continue
		}
		out[i] = val
	}
	if c.callback != nil {
		c.callback(p, tx, out)
	}
	return nil
}

func (c *Custom) expandLabels(arr []any) []any {
	inputs := 0
	for _, e := range c.elements {
		if !e.label {
			inputs++
		}
	}
	if len(arr) != inputs {
		return arr
	}
	out := make([]any, 0, len(c.elements))
	j := 0
	for _, e := range c.elements {
		if e.label {
			out = append(out, nil)
			continue
		}
		out = append(out, arr[j])
		j++
	}
	return out
}

func decode(data []byte) (any, bool) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil, true
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, false
	}
	return v, true
}

func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return int(i), true
		}
		if f, err := n.Float64(); err == nil {
			return int(f), true
		}
	case string:
		var i int
		if _, err := fmt.Sscan(n, &i); err == nil {
			return i, true
		}
	case bool:
		if n {
			return 1, true
		}
		return 0, true
	}
	return 0, false
}

func toFloat(v any) (float64, bool) {
	n, ok := v.(json.Number)
	if !ok {
		return 0, false
	}
	f, err := n.Float64()
	return f, err == nil && !math.IsNaN(f)
}

func indexIn(n int) func(v any) bool {
	return func(v any) bool {
		num, ok := v.(json.Number)
		if !ok {
			return false
		}
		i, err := num.Int64()
		return err == nil && i >= 0 && int(i) < n
	}
}

func truthy(v any) bool {
	switch b := v.(type) {
	case bool:
		return b
	case json.Number:
		f, _ := b.Float64()
		return f != 0
	case string:
		return b != "" && b != "0"
	}
	return v != nil
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
