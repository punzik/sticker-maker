package config

import (
	"strings"
	"testing"
)

func mustParse(t *testing.T, s string) *Layout {
	t.Helper()
	l, err := Parse(strings.NewReader(s))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return l
}

func wantErr(t *testing.T, s string) {
	t.Helper()
	if _, err := Parse(strings.NewReader(s)); err == nil {
		t.Fatalf("Parse(%q) succeeded, want error", s)
	}
}

const minimal = `{
  "version": 1,
  "image": {"width": 100, "height": 50},
  "blocks": [
    {"id": "a", "type": "text", "x": 0, "y": 0, "width": 40, "height": 20,
     "text": "hi", "font": {"family": "DejaVu Sans", "size_px": 12}}
  ]
}`

func TestMinimal(t *testing.T) {
	l := mustParse(t, minimal)
	if l.Version != 1 || l.Image.Width != 100 || l.Image.Height != 50 {
		t.Fatalf("bad image: %+v", l.Image)
	}
	b := l.Blocks[0]
	// defaults
	if b.ScaleX != 1.0 || b.Wrap != "word_char" || b.Align != "left" ||
		b.Valign != "top" || b.Overflow != "error" || b.Rotation != 0 {
		t.Fatalf("bad defaults: %+v", b)
	}
}

func TestErrors(t *testing.T) {
	cases := map[string]string{
		"unknown field":       `{"version":1,"image":{"width":1,"height":1},"blocks":[],"extra":true}`,
		"wrong version":       `{"version":2,"image":{"width":1,"height":1},"blocks":[]}`,
		"zero image":          `{"version":1,"image":{"width":0,"height":1},"blocks":[]}`,
		"no id":               `{"version":1,"image":{"width":1,"height":1},"blocks":[{"type":"text","x":0,"y":0,"width":1,"height":1,"text":"a","font":{"family":"F","size_px":1}}]}`,
		"dup id":              `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"text","x":0,"y":0,"width":1,"height":1,"text":"a","font":{"family":"F","size_px":1}},{"id":"a","type":"text","x":0,"y":0,"width":1,"height":1,"text":"a","font":{"family":"F","size_px":1}}]}`,
		"both sources":        `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"text","x":0,"y":0,"width":1,"height":1,"text":"a","field":"f","font":{"family":"F","size_px":1}}]}`,
		"no source":           `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"text","x":0,"y":0,"width":1,"height":1,"font":{"family":"F","size_px":1}}]}`,
		"negative x":          `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"text","x":-1,"y":0,"width":1,"height":1,"text":"a","font":{"family":"F","size_px":1}}]}`,
		"bad rotation":        `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"text","x":0,"y":0,"width":1,"height":1,"rotation":45,"text":"a","font":{"family":"F","size_px":1}}]}`,
		"no font":             `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"text","x":0,"y":0,"width":1,"height":1,"text":"a"}]}`,
		"font both+none":      `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"text","x":0,"y":0,"width":1,"height":1,"text":"a","font":{"family":"F","file":"/x.ttf","size_px":1}}]}`,
		"font no family/file": `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"text","x":0,"y":0,"width":1,"height":1,"text":"a","font":{"size_px":1}}]}`,
		"zero size_px":        `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"text","x":0,"y":0,"width":1,"height":1,"text":"a","font":{"family":"F","size_px":0}}]}`,
		"zero scale_x":        `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"text","x":0,"y":0,"width":1,"height":1,"scale_x":-1,"text":"a","font":{"family":"F","size_px":1}}]}`,
		"bad wrap":            `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"text","x":0,"y":0,"width":1,"height":1,"wrap":"words","text":"a","font":{"family":"F","size_px":1}}]}`,
		"bad align":           `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"text","x":0,"y":0,"width":1,"height":1,"align":"middle","text":"a","font":{"family":"F","size_px":1}}]}`,
		"bad overflow":        `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"text","x":0,"y":0,"width":1,"height":1,"overflow":"shrink","text":"a","font":{"family":"F","size_px":1}}]}`,
		"unknown block type":  `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"qr","x":0,"y":0,"text":"a"}]}`,
		"dm no symbol":        `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"datamatrix","x":0,"y":0,"text":"a","module_px":2}]}`,
		"dm zero module":      `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"datamatrix","x":0,"y":0,"text":"a","symbol":{"rows":24,"columns":24}}]}`,
		"dm zero quiet":       `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"datamatrix","x":0,"y":0,"text":"a","module_px":2,"quiet_zone_modules":0}]}`,
		"text block w/o size": `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"text","x":0,"y":0,"text":"a","font":{"family":"F","size_px":1}}]}`,
	}
	for name, s := range cases {
		t.Run(name, func(t *testing.T) { wantErr(t, s) })
	}
}

func TestFields(t *testing.T) {
	if _, _, err := ParseField("name"); err == nil {
		t.Fatal("ParseField(name) should fail")
	}
	if _, _, err := ParseField("=value"); err == nil {
		t.Fatal("ParseField(=value) should fail")
	}
	n, v, err := ParseField("name=value=with=equals")
	if err != nil || n != "name" || v != "value=with=equals" {
		t.Fatalf("ParseField: %q %q %v", n, v, err)
	}

	l := mustParse(t, `{"version":1,"image":{"width":1,"height":1},"blocks":[
		{"id":"a","type":"text","x":0,"y":0,"width":1,"height":1,"text":"static","font":{"family":"F","size_px":1}},
		{"id":"b","type":"text","x":0,"y":0,"width":1,"height":1,"field":"f","font":{"family":"F","size_px":1}}
	]}`+`)`)
	fields := Fields{"f": "42"}
	if got, err := l.Blocks[0].Content(fields); err != nil || got != "static" {
		t.Fatalf("static content: %q %v", got, err)
	}
	if got, err := l.Blocks[1].Content(fields); err != nil || got != "42" {
		t.Fatalf("field content: %q %v", got, err)
	}
	if _, err := l.Blocks[1].Content(Fields{}); err == nil {
		t.Fatal("missing field should error")
	}
}
