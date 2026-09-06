// Package config defines the JSON layout schema, strict parsing and
// validation for the sticker maker.
package config

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
)

// SupportedVersion is the only accepted layout format version.
const SupportedVersion = 1

// Layout is the top-level document.
type Layout struct {
	Version int     `json:"version"`
	Image   Image   `json:"image"`
	Blocks  []Block `json:"blocks"`
	// ForbidOverlap rejects overlapping blocks; by default blocks may
	// overlap.
	ForbidOverlap bool `json:"forbid_overlap"`
}

// Image describes the output canvas size in pixels.
type Image struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// Block is a single labeled element: a text block or a Data Matrix block.
type Block struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	X        int    `json:"x"`
	Y        int    `json:"y"`
	Rotation int    `json:"rotation"`

	// Text blocks only.
	Width    int      `json:"width,omitempty"`
	Height   int      `json:"height,omitempty"`
	Font     *Font    `json:"font,omitempty"`
	ScaleX   *float64 `json:"scale_x,omitempty"`
	Wrap     *string  `json:"wrap,omitempty"`
	Align    *string  `json:"align,omitempty"`
	Valign   *string  `json:"valign,omitempty"`
	Overflow *string  `json:"overflow,omitempty"`

	// Data Matrix blocks only.
	Symbol           *Symbol `json:"symbol,omitempty"`
	ModulePx         int     `json:"module_px,omitempty"`
	QuietZoneModules *int    `json:"quiet_zone_modules,omitempty"`

	// Line blocks only: stroke endpoints in pixel coordinates.
	X1 int `json:"x1,omitempty"`
	Y1 int `json:"y1,omitempty"`
	X2 int `json:"x2,omitempty"`
	Y2 int `json:"y2,omitempty"`

	// Content: static text when set; otherwise the named field matching
	// the block's ID. Not used by line blocks.
	Text *string `json:"text,omitempty"`
}

// Font selects the typeface for a text block.
//
// Exactly one of Family (+ optional Style) or File must be set.
type Font struct {
	Family string  `json:"family,omitempty"`
	Style  string  `json:"style,omitempty"`
	File   string  `json:"file,omitempty"`
	SizePx float64 `json:"size_px"`
}

// Symbol fixes the Data Matrix symbol dimensionality (in modules),
// without the quiet zone.
type Symbol struct {
	Rows    int `json:"rows"`
	Columns int `json:"columns"`
}

// applyDefaults fills omitted optional fields with documented defaults.
// Optional fields are pointers so that "absent" is distinguishable from an
// explicit zero/empty value: explicit values survive here and are rejected
// by Validate when they violate the documented constraints. Only fields of
// the block's own type are defaulted, so validateApplicability can still
// see foreign fields as explicitly set.
func (b *Block) applyDefaults() {
	if b.Type == "text" {
		if b.ScaleX == nil {
			v := 1.0
			b.ScaleX = &v
		}
		if b.Wrap == nil {
			v := "word_char"
			b.Wrap = &v
		}
		if b.Align == nil {
			v := "left"
			b.Align = &v
		}
		if b.Valign == nil {
			v := "top"
			b.Valign = &v
		}
		if b.Overflow == nil {
			v := "error"
			b.Overflow = &v
		}
		return
	}
	if b.Type == "datamatrix" && b.QuietZoneModules == nil {
		v := 1
		b.QuietZoneModules = &v
	}
}

// Load reads and strictly parses a layout file.
func Load(path string) (*Layout, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("layout: %w", err)
	}
	defer f.Close()
	return Parse(f)
}

// Parse strictly parses a layout document. Unknown fields are rejected.
func Parse(r io.Reader) (*Layout, error) {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	var l Layout
	if err := dec.Decode(&l); err != nil {
		return nil, fmt.Errorf("layout: %w", err)
	}
	if err := l.Validate(); err != nil {
		return nil, err
	}
	return &l, nil
}

func (l *Layout) Validate() error {
	if l.Version != SupportedVersion {
		return fmt.Errorf("layout: unsupported version %d (want %d)", l.Version, SupportedVersion)
	}
	if l.Image.Width < 1 || l.Image.Height < 1 {
		return fmt.Errorf("layout: image size must be at least 1x1, got %dx%d", l.Image.Width, l.Image.Height)
	}
	seen := make(map[string]bool, len(l.Blocks))
	for i := range l.Blocks {
		b := &l.Blocks[i]
		b.applyDefaults()
		if err := b.Validate(i); err != nil {
			return err
		}
		if seen[b.ID] {
			return fmt.Errorf("layout: duplicate block id %q", b.ID)
		}
		seen[b.ID] = true
	}
	return nil
}

func (b *Block) Validate(i int) error {
	if b.ID == "" {
		return fmt.Errorf("layout: block %d: missing id", i)
	}
	switch b.Rotation {
	case 0, 90, 180, 270:
	default:
		return fmt.Errorf("block %q: rotation must be 0, 90, 180 or 270, got %d", b.ID, b.Rotation)
	}
	switch b.Type {
	case "text", "datamatrix", "line":
	default:
		return fmt.Errorf("block %q: unknown type %q (want \"text\", \"datamatrix\" or \"line\")", b.ID, b.Type)
	}
	if err := b.validateApplicability(); err != nil {
		return err
	}
	switch b.Type {
	case "text":
		return b.validateText()
	case "line":
		return b.validateLine()
	default: // "datamatrix"
		return b.validateDataMatrix()
	}
}

// validateApplicability rejects fields that belong to the other block type:
// strict parsing covers unknown field names only, and both types share one
// struct, so a "width" inside a datamatrix block would otherwise be
// silently ignored.
func (b *Block) validateApplicability() error {
	var fields []struct {
		name string
		set  bool
	}
	switch b.Type {
	case "text":
		fields = []struct {
			name string
			set  bool
		}{
			{"symbol", b.Symbol != nil},
			{"module_px", b.ModulePx != 0},
			{"quiet_zone_modules", b.QuietZoneModules != nil},
			{"x1", b.X1 != 0},
			{"y1", b.Y1 != 0},
			{"x2", b.X2 != 0},
			{"y2", b.Y2 != 0},
		}
	case "datamatrix":
		fields = []struct {
			name string
			set  bool
		}{
			{"width", b.Width != 0},
			{"height", b.Height != 0},
			{"font", b.Font != nil},
			{"scale_x", b.ScaleX != nil},
			{"wrap", b.Wrap != nil},
			{"align", b.Align != nil},
			{"valign", b.Valign != nil},
			{"overflow", b.Overflow != nil},
			{"x1", b.X1 != 0},
			{"y1", b.Y1 != 0},
			{"x2", b.X2 != 0},
			{"y2", b.Y2 != 0},
		}
	default: // "line"
		fields = []struct {
			name string
			set  bool
		}{
			{"x", b.X != 0},
			{"y", b.Y != 0},
			{"height", b.Height != 0},
			{"rotation", b.Rotation != 0},
			{"text", b.Text != nil},
			{"font", b.Font != nil},
			{"scale_x", b.ScaleX != nil},
			{"wrap", b.Wrap != nil},
			{"align", b.Align != nil},
			{"valign", b.Valign != nil},
			{"overflow", b.Overflow != nil},
			{"symbol", b.Symbol != nil},
			{"module_px", b.ModulePx != 0},
			{"quiet_zone_modules", b.QuietZoneModules != nil},
		}
	}
	for _, f := range fields {
		if f.set {
			return fmt.Errorf("block %q: field %q does not apply to a %s block", b.ID, f.name, b.Type)
		}
	}
	return nil
}

func (b *Block) validateLine() error {
	if b.X1 < 0 || b.Y1 < 0 || b.X2 < 0 || b.Y2 < 0 {
		return fmt.Errorf("block %q: x1, y1, x2, y2 must be non-negative", b.ID)
	}
	if b.Width < 1 {
		return fmt.Errorf("block %q: line width must be positive", b.ID)
	}
	return nil
}

func (b *Block) validateText() error {
	if b.X < 0 || b.Y < 0 {
		return fmt.Errorf("block %q: x and y must be non-negative", b.ID)
	}
	if b.Width < 1 || b.Height < 1 {
		return fmt.Errorf("block %q: width and height must be positive", b.ID)
	}
	if b.Font == nil {
		return fmt.Errorf("block %q: text block requires \"font\"", b.ID)
	}
	if b.Font.SizePx <= 0 || math.IsNaN(b.Font.SizePx) {
		return fmt.Errorf("block %q: font.size_px must be positive", b.ID)
	}
	hasFamily := b.Font.Family != ""
	hasFile := b.Font.File != ""
	if hasFamily == hasFile {
		return fmt.Errorf("block %q: font requires exactly one of \"family\" or \"file\"", b.ID)
	}
	if *b.ScaleX <= 0 || math.IsNaN(*b.ScaleX) {
		return fmt.Errorf("block %q: scale_x must be positive", b.ID)
	}
	switch *b.Wrap {
	case "word", "char", "word_char", "none":
	default:
		return fmt.Errorf("block %q: invalid wrap %q", b.ID, *b.Wrap)
	}
	switch *b.Align {
	case "left", "center", "right":
	default:
		return fmt.Errorf("block %q: invalid align %q", b.ID, *b.Align)
	}
	switch *b.Valign {
	case "top", "center", "bottom":
	default:
		return fmt.Errorf("block %q: invalid valign %q", b.ID, *b.Valign)
	}
	switch *b.Overflow {
	case "error", "clip":
	default:
		return fmt.Errorf("block %q: invalid overflow %q", b.ID, *b.Overflow)
	}
	return nil
}

func (b *Block) validateDataMatrix() error {
	if b.X < 0 || b.Y < 0 {
		return fmt.Errorf("block %q: x and y must be non-negative", b.ID)
	}
	if b.Symbol == nil {
		return fmt.Errorf("block %q: datamatrix block requires \"symbol\"", b.ID)
	}
	if b.Symbol.Rows < 1 || b.Symbol.Columns < 1 {
		return fmt.Errorf("block %q: symbol rows and columns must be positive", b.ID)
	}
	if b.ModulePx < 1 {
		return fmt.Errorf("block %q: module_px must be a positive integer", b.ID)
	}
	if *b.QuietZoneModules < 1 {
		return fmt.Errorf("block %q: quiet_zone_modules must be at least 1", b.ID)
	}
	return nil
}

// Fields holds named values passed on the command line.
type Fields map[string]string

// ParseField parses one "name=value" command-line field.
func ParseField(s string) (string, string, error) {
	name, value, ok := strings.Cut(s, "=")
	if !ok || name == "" {
		return "", "", fmt.Errorf("field %q: expected format name=value", s)
	}
	return name, value, nil
}

// Content returns the block content: static text when set, otherwise the
// named field matching the block's ID.
func (b *Block) Content(fields Fields) (string, error) {
	if b.Text != nil {
		return *b.Text, nil
	}
	v, ok := fields[b.ID]
	if !ok {
		return "", fmt.Errorf("block %q: field %q was not provided", b.ID, b.ID)
	}
	return v, nil
}
