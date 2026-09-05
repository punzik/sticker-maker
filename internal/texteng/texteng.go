// Package texteng renders Latin text with a system font file into
// a black-and-white bitmap, with horizontal scaling, line wrapping and
// alignment.
package texteng

import (
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// Threshold is the fixed gray level (0..255) at which an antialiased pixel
// becomes black in the final image.
const Threshold = 128

// Options selects the typeface and rasterization size.
type Options struct {
	Family string
	Style  string
	File   string
	SizePx float64
}

// Face is a loaded font with its metrics.
type Face struct {
	f      *opentype.Font
	face   font.Face
	LineH  int // line height in pixels (metrics height, rounded up)
	Ascent int // ascent in pixels (rounded up)
}

// Load loads a font either from an explicit file or from the system
// (fontconfig) by family and style.
func Load(o Options) (*Face, error) {
	path, err := resolvePath(o)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("font %q: %w", path, err)
	}
	f, err := opentype.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("font %q: %w", path, err)
	}
	fc, err := opentype.NewFace(f, &opentype.FaceOptions{Size: o.SizePx, DPI: 72})
	if err != nil {
		return nil, fmt.Errorf("font %q: %w", path, err)
	}
	m := fc.Metrics()
	if m.Height <= 0 || m.Ascent <= 0 {
		return nil, fmt.Errorf("font %q: invalid metrics", path)
	}
	return &Face{
		f:      f,
		face:   fc,
		LineH:  int(math.Ceil(float64(m.Height) / 64.0)),
		Ascent: int(math.Ceil(float64(m.Ascent) / 64.0)),
	}, nil
}

// Measure returns the natural (unscaled) width of s in pixels.
// It fails if the face lacks a glyph for some rune.
func (f *Face) Measure(s string) (float64, error) {
	var w fixed.Int26_6
	for _, r := range s {
		adv, ok := f.face.GlyphAdvance(r)
		if !ok {
			return 0, fmt.Errorf("font is missing a glyph for U+%04X", r)
		}
		w += adv
	}
	return float64(w.Round()), nil
}

// CheckGlyphs reports the first rune of s for which the face has no glyph.
// font.Drawer skips such runes silently, so Render checks up front to
// guarantee that no content is lost.
func (f *Face) CheckGlyphs(s string) error {
	for _, r := range s {
		if _, ok := f.face.GlyphAdvance(r); !ok {
			return fmt.Errorf("font is missing a glyph for U+%04X", r)
		}
	}
	return nil
}

// List returns "family|style|file" lines for all fonts known to fontconfig.
func List() ([]string, error) {
	out, err := exec.Command("fc-list", "--format", "%{family}\t%{style}\t%{file}\n").Output()
	if err != nil {
		return nil, fmt.Errorf("fc-list: %w (is fontconfig installed?)", err)
	}
	return strings.Split(strings.TrimRight(string(out), "\n"), "\n"), nil
}

// fontEntry is one row of the system font database.
type fontEntry struct {
	Family string
	Style  string
	File   string
}

// listFonts returns every font known to fontconfig in one query.
// Matching is then done in Go instead of building fc-match patterns,
// which keeps family/style values from ever being interpreted as
// pattern syntax.
func listFonts() ([]fontEntry, error) {
	out, err := exec.Command("fc-list", "--format", "%{family}\t%{style}\t%{file}\n").Output()
	if err != nil {
		return nil, fmt.Errorf("fc-list: %w (is fontconfig installed?)", err)
	}
	var fonts []fontEntry
	for _, ln := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		f := parseFontLine(ln)
		if f != nil {
			fonts = append(fonts, *f)
		}
	}
	if len(fonts) == 0 {
		return nil, errors.New("no system fonts found")
	}
	return fonts, nil
}

func parseFontLine(ln string) *fontEntry {
	parts := strings.SplitN(ln, "\t", 3)
	if len(parts) != 3 || parts[2] == "" {
		return nil
	}
	return &fontEntry{Family: strings.TrimSpace(parts[0]), Style: strings.TrimSpace(parts[1]), File: parts[2]}
}

// familyNames splits the fc-list family column: fontconfig separates
// families by ":" and aliases inside a family by ",".
func familyNames(col string) []string {
	var names []string
	for _, grp := range strings.Split(col, ":") {
		for _, n := range strings.Split(grp, ",") {
			if n = strings.TrimSpace(n); n != "" {
				names = append(names, n)
			}
		}
	}
	return names
}

// familyMatches reports whether the font family contains the requested
// name (case-insensitively). Comparing list elements instead of the whole
// colon-joined string avoids false "not found" for fonts with aliases.
func familyMatches(font fontEntry, want string) bool {
	for _, n := range familyNames(font.Family) {
		if strings.EqualFold(n, want) {
			return true
		}
	}
	return false
}

// resolvePath maps the font selection to a font file, verifying that the
// fontconfig fallback did not silently substitute another face.
func resolvePath(o Options) (string, error) {
	if o.File != "" {
		return o.File, nil
	}
	fonts, err := listFonts()
	if err != nil {
		return "", err
	}
	f, err := matchFont(fonts, o.Family, o.Style)
	if err != nil {
		return "", err
	}
	return f.File, nil
}

// matchFont selects the font for family (and style, when non-empty) from
// the database. Both values are matched as opaque strings: family against
// the individual names of each entry, style case-insensitively as a whole
// column value. A missing style is an error — no silent substitution, as
// per the README.
//
// An entry whose primary family (first name) equals the request beats one
// that only carries the name as an alias, and among those, the earlier
// database entry wins. This mirrors fontconfig's preference for exact
// families, so "DejaVu Sans:Bold" never resolves to a condensed alias
// face regardless of database order.
func matchFont(fonts []fontEntry, family, style string) (*fontEntry, error) {
	var best *fontEntry
	bestFam := 2
	for i := range fonts {
		f := &fonts[i]
		names := familyNames(f.Family)
		if len(names) == 0 {
			continue
		}
		famScore := 2 // alias match
		if strings.EqualFold(names[0], family) {
			famScore = 1 // primary family match
		} else if !familyMatches(*f, family) {
			continue
		}
		if style != "" && !strings.EqualFold(f.Style, style) {
			continue
		}
		if best == nil || famScore < bestFam {
			best, bestFam = f, famScore
		}
	}
	if best == nil && availableStyles(fonts, family) != "" {
		return nil, fmt.Errorf("font style %q not found for family %q (available styles: %s)",
			style, family, availableStyles(fonts, family))
	}
	if best == nil {
		return nil, fmt.Errorf("font family %q not found (use --list-fonts to see installed families)", family)
	}
	return best, nil
}

// availableStyles returns the distinct styles of the requested family,
// in install order, for error messages.
func availableStyles(fonts []fontEntry, family string) string {
	var seen []string
	for _, f := range fonts {
		if !familyMatches(f, family) || f.Style == "" {
			continue
		}
		dup := false
		for _, s := range seen {
			if strings.EqualFold(s, f.Style) {
				dup = true
				break
			}
		}
		if !dup {
			seen = append(seen, f.Style)
		}
	}
	return strings.Join(seen, ", ")
}
