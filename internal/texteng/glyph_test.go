package texteng

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/image/font/gofont/goregular"
)

// loadTestFace loads the Go Regular TTF from x/image so the test does not
// depend on system fonts.
func loadTestFace(t *testing.T) *Face {
	t.Helper()
	p := filepath.Join(t.TempDir(), "go-reg.ttf")
	if err := os.WriteFile(p, goregular.TTF, 0o644); err != nil {
		t.Fatal(err)
	}
	face, err := Load(Options{File: p, SizePx: 16})
	if err != nil {
		t.Fatal(err)
	}
	return face
}

func TestCheckGlyphs(t *testing.T) {
	face := loadTestFace(t)
	if err := face.CheckGlyphs("PART-001 =5"); err != nil {
		t.Fatalf("ascii: %v", err)
	}
	if err := face.CheckGlyphs("中"); err == nil {
		t.Fatal("want missing glyph error for U+4E2D")
	} else if !strings.Contains(err.Error(), "U+4E2D") {
		t.Fatalf("error %q does not name the missing rune", err)
	}
}

func TestRenderMissingGlyphErrors(t *testing.T) {
	face := loadTestFace(t)
	p := Params{Width: 200, Height: 20, ScaleX: 1, Overflow: "error"}
	if _, err := face.Render("hello", p); err != nil {
		t.Fatalf("ascii: %v", err)
	}
	// Go Regular has no CJK glyph: must fail, not silently drop the rune.
	if _, err := face.Render("hello 中", p); err == nil {
		t.Fatal("want missing glyph error")
	}
}
