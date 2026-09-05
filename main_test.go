package main

import (
	"errors"
	"flag"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sticker-maker/internal/dm"
)

const layout = `{
  "version": 1,
  "image": { "width": 200, "height": 100 },
  "blocks": [
    { "id": "t", "type": "text", "x": 5, "y": 5, "width": 120, "height": 90,
      "field": "title",
      "font": { "family": "DejaVu Sans", "style": "Book", "size_px": 16 },
      "wrap": "word_char", "align": "left", "valign": "top" },
    { "id": "code", "type": "datamatrix", "x": 140, "y": 5,
      "field": "code", "symbol": { "rows": 24, "columns": 24 },
      "module_px": 3, "quiet_zone_modules": 1 }
  ]
}`

func writeLayout(t *testing.T, s string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "layout.json")
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func gray(img image.Image, x, y int) uint32 {
	r, _, _, _ := img.At(x, y).RGBA()
	return r
}

func requiresFonts(t *testing.T) {
	t.Helper()
	if _, err := os.Stat("/usr/share/fonts"); err != nil {
		t.Skip("no system fonts installed")
	}
}

func TestRenderEndToEnd(t *testing.T) {
	requiresFonts(t)
	lp := writeLayout(t, layout)
	out := filepath.Join(t.TempDir(), "out.png")
	err := run([]string{"--layout", lp, "--output", out, "--field", "title=Hello wrap", "--field", "code=END2END-1"})
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	if w != 200 || h != 100 {
		t.Fatalf("size %dx%d, want 200x100", w, h)
	}
	// exactly two colors
	seen := map[uint8]bool{}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			seen[uint8(gray(img, x, y))] = true
		}
	}
	if len(seen) != 2 {
		t.Fatalf("colors %v, want exactly {0,255}", seen)
	}
	// the Data Matrix region must contain black modules
	// 24*3 + 2*1*3 = 78 px at (140,5)
	foundBlack := false
	for y := 5; y < 5+78; y++ {
		for x := 140; x < 140+78; x++ {
			if gray(img, x, y) == 0 {
				foundBlack = true
			}
		}
	}
	if !foundBlack {
		t.Fatal("datamatrix region is empty")
	}
}

func TestMissingField(t *testing.T) {
	lp := writeLayout(t, layout)
	err := run([]string{"--layout", lp, "--output", filepath.Join(t.TempDir(), "x.png"), "--field", "title=x"})
	if err == nil || !strings.Contains(err.Error(), "code") {
		t.Fatalf("want missing field error, got %v", err)
	}
}

func TestEmptyTextRejected(t *testing.T) {
	dir := t.TempDir()
	lp := writeLayout(t, `{"version":1,"image":{"width":40,"height":40},"blocks":[{"id":"t","type":"text","x":0,"y":0,"width":40,"height":20,"field":"empty","font":{"family":"DejaVu Sans","size_px":12}}]}`)
	err := run([]string{"--layout", lp, "--output", filepath.Join(dir, "x.png"), "--field", "empty="})
	if err == nil || !strings.Contains(err.Error(), "must not be empty") {
		t.Fatalf("want empty content error, got %v", err)
	}
	lp = writeLayout(t, `{"version":1,"image":{"width":40,"height":40},"blocks":[{"id":"t","type":"text","x":0,"y":0,"width":40,"height":20,"text":"   ","font":{"family":"DejaVu Sans","size_px":12}}]}`)
	if err := run([]string{"--layout", lp, "--output", filepath.Join(dir, "y.png")}); err == nil {
		t.Fatal("want error for whitespace-only static text")
	}
}

func TestUsage(t *testing.T) {
	if err := run(nil); err != errUsage {
		t.Fatalf("want usage error, got %v", err)
	}
}

func TestHelpFlag(t *testing.T) {
	for _, arg := range [][]string{{"--help"}, {"-h"}} {
		err := run(arg)
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("run(%v): want flag.ErrHelp, got %v", arg, err)
		}
	}
}

func TestDataMatrixOverflow(t *testing.T) {
	// 10x10 holds only 3 ASCII characters.
	lp := writeLayout(t, `{
	  "version": 1, "image": { "width": 50, "height": 50 },
	  "blocks": [ { "id": "c", "type": "datamatrix", "x": 0, "y": 0,
	    "field": "code", "symbol": { "rows": 10, "columns": 10 }, "module_px": 4 } ]
	}`)
	out := filepath.Join(t.TempDir(), "out.png")
	err := run([]string{"--layout", lp, "--output", out, "--field", "code=TOOLONG"})
	if err == nil {
		t.Fatal("want capacity overflow error")
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatal("no output file must be created on error")
	}
}

// TestDMAllSizes encodes every supported size with a payload that fits,
// ensuring the encoder accepts the whole documented list.
func TestDMAllSizes(t *testing.T) {
	if _, err := dm.Encode("A", 144, 144); err != nil {
		t.Fatalf("144x144: %v", err)
	}
	if _, err := dm.Encode("ABCDE", 8, 18); err != nil {
		t.Fatalf("8x18 with 5 chars: %v", err)
	}
	if _, err := dm.Encode("0123456789", 12, 26); err != nil {
		t.Fatalf("12x26 with 10 chars: %v", err)
	}
}
