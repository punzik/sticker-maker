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

// resolvePath maps the font selection to a font file, verifying that the
// fontconfig fallback did not silently substitute another face.
func resolvePath(o Options) (string, error) {
	if o.File != "" {
		return o.File, nil
	}
	pattern := o.Family
	if o.Style != "" {
		pattern += ":style=" + o.Style
	}
	file, err := fcMatch(pattern, "%{file}")
	if err != nil {
		return "", err
	}
	fam, err := fcMatch(pattern, "%{family}")
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(strings.TrimSpace(fam), o.Family) {
		return "", errors.New("font family " + quote(o.Family) + " not found (fontconfig fell back to " + quote(fam) + ")")
	}
	if o.Style != "" {
		st, err := fcMatch(pattern, "%{style}")
		if err != nil {
			return "", err
		}
		if !strings.EqualFold(strings.TrimSpace(st), o.Style) {
			return "", errors.New("font style " + quote(o.Style) + " not found for family " + quote(o.Family) + " (fontconfig fell back to " + quote(st) + ")")
		}
	}
	return file, nil
}

func fcMatch(pattern, format string) (string, error) {
	out, err := exec.Command("fc-match", "-f", format+"\n", pattern).Output()
	if err != nil {
		return "", fmt.Errorf("fc-match %q: %w", pattern, err)
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		return "", fmt.Errorf("fc-match %q: empty result", pattern)
	}
	return s, nil
}

func quote(s string) string { return "\"" + s + "\"" }
