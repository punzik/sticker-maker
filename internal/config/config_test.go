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
	if *b.ScaleX != 1.0 || *b.Wrap != "word_char" || *b.Align != "left" ||
		*b.Valign != "top" || *b.Overflow != "error" || b.Rotation != 0 {
		t.Fatalf("bad defaults: %+v", b)
	}
}

func TestForbidOverlap(t *testing.T) {
	l := mustParse(t, `{"version":1,"image":{"width":1,"height":1},"blocks":[],"forbid_overlap":true}`)
	if !l.ForbidOverlap {
		t.Fatal("forbid_overlap not parsed")
	}
	if mustParse(t, minimal).ForbidOverlap {
		t.Fatal("forbid_overlap default must be false")
	}
}

func TestLineBlock(t *testing.T) {
	l := mustParse(t, `{"version":1,"image":{"width":10,"height":10},"blocks":[
		{"id":"ln","type":"line","x1":0,"y1":5,"x2":9,"y2":5,"width":3}]}`)
	b := l.Blocks[0]
	if b.X1 != 0 || b.Y1 != 5 || b.X2 != 9 || b.Y2 != 5 || b.Width != 3 {
		t.Fatalf("bad line: %+v", b)
	}
}

func TestErrors(t *testing.T) {
	cases := map[string]string{
		"unknown field":         `{"version":1,"image":{"width":1,"height":1},"blocks":[],"extra":true}`,
		"wrong version":         `{"version":2,"image":{"width":1,"height":1},"blocks":[]}`,
		"zero image":            `{"version":1,"image":{"width":0,"height":1},"blocks":[]}`,
		"no id":                 `{"version":1,"image":{"width":1,"height":1},"blocks":[{"type":"text","x":0,"y":0,"width":1,"height":1,"text":"a","font":{"family":"F","size_px":1}}]}`,
		"dup id":                `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"text","x":0,"y":0,"width":1,"height":1,"text":"a","font":{"family":"F","size_px":1}},{"id":"a","type":"text","x":0,"y":0,"width":1,"height":1,"text":"a","font":{"family":"F","size_px":1}}]}`,
		"negative x":            `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"text","x":-1,"y":0,"width":1,"height":1,"text":"a","font":{"family":"F","size_px":1}}]}`,
		"bad rotation":          `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"text","x":0,"y":0,"width":1,"height":1,"rotation":45,"text":"a","font":{"family":"F","size_px":1}}]}`,
		"no font":               `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"text","x":0,"y":0,"width":1,"height":1,"text":"a"}]}`,
		"font both+none":        `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"text","x":0,"y":0,"width":1,"height":1,"text":"a","font":{"family":"F","file":"/x.ttf","size_px":1}}]}`,
		"font no family/file":   `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"text","x":0,"y":0,"width":1,"height":1,"text":"a","font":{"size_px":1}}]}`,
		"zero size_px":          `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"text","x":0,"y":0,"width":1,"height":1,"text":"a","font":{"family":"F","size_px":0}}]}`,
		"zero scale_x":          `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"text","x":0,"y":0,"width":1,"height":1,"scale_x":0,"text":"a","font":{"family":"F","size_px":1}}]}`,
		"negative scale_x":      `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"text","x":0,"y":0,"width":1,"height":1,"scale_x":-1,"text":"a","font":{"family":"F","size_px":1}}]}`,
		"empty wrap":            `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"text","x":0,"y":0,"width":1,"height":1,"wrap":"","text":"a","font":{"family":"F","size_px":1}}]}`,
		"bad wrap":              `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"text","x":0,"y":0,"width":1,"height":1,"wrap":"words","text":"a","font":{"family":"F","size_px":1}}]}`,
		"bad align":             `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"text","x":0,"y":0,"width":1,"height":1,"align":"middle","text":"a","font":{"family":"F","size_px":1}}]}`,
		"bad overflow":          `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"text","x":0,"y":0,"width":1,"height":1,"overflow":"shrink","text":"a","font":{"family":"F","size_px":1}}]}`,
		"unknown block type":    `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"qr","x":0,"y":0,"text":"a"}]}`,
		"dm no symbol":          `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"datamatrix","x":0,"y":0,"text":"a","module_px":2}]}`,
		"dm zero module":        `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"datamatrix","x":0,"y":0,"text":"a","symbol":{"rows":24,"columns":24}}]}`,
		"dm zero quiet":         `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"datamatrix","x":0,"y":0,"text":"a","module_px":2,"quiet_zone_modules":0}]}`,
		"text block w/o size":   `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"text","x":0,"y":0,"text":"a","font":{"family":"F","size_px":1}}]}`,
		"dm field in text":      `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"text","x":0,"y":0,"width":1,"height":1,"text":"a","font":{"family":"F","size_px":1},"module_px":2}]}`,
		"quiet zone in text":    `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"text","x":0,"y":0,"width":1,"height":1,"text":"a","font":{"family":"F","size_px":1},"quiet_zone_modules":2}]}`,
		"symbol in text":        `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"text","x":0,"y":0,"width":1,"height":1,"text":"a","font":{"family":"F","size_px":1},"symbol":{"rows":10,"columns":10}}]}`,
		"width in datamatrix":   `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"datamatrix","x":0,"y":0,"text":"a","width":40,"symbol":{"rows":10,"columns":10},"module_px":2}]}`,
		"font in datamatrix":    `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"datamatrix","x":0,"y":0,"text":"a","font":{"family":"F","size_px":1},"symbol":{"rows":10,"columns":10},"module_px":2}]}`,
		"scale_x in datamatrix": `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"datamatrix","x":0,"y":0,"text":"a","scale_x":1,"symbol":{"rows":10,"columns":10},"module_px":2}]}`,
		"line zero width":       `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"line","x1":0,"y1":0,"x2":0,"y2":0}]}`,
		"line negative coord":   `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"line","x1":-1,"y1":0,"x2":0,"y2":0,"width":1}]}`,
		"line rotation":         `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"line","x1":0,"y1":0,"x2":0,"y2":0,"width":1,"rotation":90}]}`,
		"line x":                `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"line","x":1,"y":0,"x1":0,"y1":0,"x2":0,"y2":0,"width":1}]}`,
		"line height":           `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"line","x1":0,"y1":0,"x2":0,"y2":0,"width":1,"height":2}]}`,
		"line text":             `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"line","x1":0,"y1":0,"x2":0,"y2":0,"width":1,"text":"x"}]}`,
		"line font":             `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"line","x1":0,"y1":0,"x2":0,"y2":0,"width":1,"font":{"family":"F","size_px":1}}]}`,
		"line symbol":           `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"line","x1":0,"y1":0,"x2":0,"y2":0,"width":1,"symbol":{"rows":10,"columns":10}}]}`,
		"x2 in text":            `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"text","x":0,"y":0,"width":1,"height":1,"text":"a","x1":0,"y1":0,"x2":5,"y2":0,"font":{"family":"F","size_px":1}}]}`,
		"x2 in datamatrix":      `{"version":1,"image":{"width":1,"height":1},"blocks":[{"id":"a","type":"datamatrix","x":0,"y":0,"text":"a","x1":0,"y1":0,"x2":5,"y2":0,"symbol":{"rows":10,"columns":10},"module_px":2}]}`,
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
		{"id":"b","type":"text","x":0,"y":0,"width":1,"height":1,"font":{"family":"F","size_px":1}}
	]}`+`)`)
	fields := Fields{"b": "42"}
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
