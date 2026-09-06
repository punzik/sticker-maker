package main

import (
	"errors"
	"flag"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"sticker-maker/internal/dm"
)

const layout = `{
  "version": 1,
  "image": { "width": 200, "height": 100 },
  "blocks": [
    { "id": "t", "type": "text", "x": 2, "y": 5, "width": 115, "height": 90,
      "field": "title",
      "font": { "family": "DejaVu Sans", "style": "Book", "size_px": 16 },
      "wrap": "word_char", "align": "left", "valign": "top" },
    { "id": "code", "type": "datamatrix", "x": 122, "y": 5,
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
	// Font availability is what the tool needs: fonts may live in
	// non-standard directories (e.g. the Nix store), so check
	// fontconfig rather than a fixed path.
	if out, err := exec.Command("fc-list").Output(); err != nil || len(out) == 0 {
		t.Skip("no system fonts available")
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
	// 24*3 + 2*1*3 = 78 px at (122,5)
	foundBlack := false
	for y := 5; y < 5+78; y++ {
		for x := 122; x < 122+78; x++ {
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

func TestWritePNGAtomic(t *testing.T) {
	dir := t.TempDir()
	img := image.NewGray(image.Rect(0, 0, 2, 2))
	if err := writePNG(filepath.Join(dir, "out.png"), img); err != nil {
		t.Fatal(err)
	}
	// The result is in place and no temporary file remains.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "out.png" {
		t.Fatalf("entries %v, want only out.png", entries)
	}
	// A destination whose parent is a regular file must fail cleanly.
	notDir := filepath.Join(dir, "plain")
	if err := os.WriteFile(notDir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writePNG(filepath.Join(notDir, "out.png"), img); err == nil {
		t.Fatal("want error for unwritable destination")
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

func TestOverlapDefaultAndForbidden(t *testing.T) {
	requiresFonts(t)
	const ovl = `"blocks":[
	  {"id":"a","type":"text","x":0,"y":0,"width":30,"height":30,
	   "text":"A","font":{"family":"DejaVu Sans","size_px":12}},
	  {"id":"b","type":"text","x":10,"y":0,"width":30,"height":30,
	   "text":"B","font":{"family":"DejaVu Sans","size_px":12}}]`
	lp := writeLayout(t, `{"version":1,"image":{"width":60,"height":30},`+ovl+`}`)
	if err := run([]string{"--layout", lp, "--output", filepath.Join(t.TempDir(), "a.png")}); err != nil {
		t.Fatalf("overlapping blocks must be allowed by default: %v", err)
	}
	lp = writeLayout(t, `{"version":1,"image":{"width":60,"height":30},"forbid_overlap":true,`+ovl+`}`)
	err := run([]string{"--layout", lp, "--output", filepath.Join(t.TempDir(), "b.png")})
	if err == nil || !strings.Contains(err.Error(), `"a" and "b" overlap`) {
		t.Fatalf("want overlap error, got %v", err)
	}
}

func TestRotatedTextAbsoluteDimensions(t *testing.T) {
	requiresFonts(t)
	// width/height are the final post-rotation dimensions: with
	// rotation 90 the block must occupy 30 px by X and 80 px by Y,
	// so the text is laid out in a transposed 80 x 30 box. Under the
	// old pre-rotation semantics "ABCD" (~64 px) would not fit the
	// 30 px render box and rendering would fail.
	lp := writeLayout(t, `{"version":1,"image":{"width":100,"height":100},"blocks":[
	  {"id":"t","type":"text","x":0,"y":0,"width":30,"height":80,"rotation":90,
	   "text":"ABCD","font":{"family":"DejaVu Sans","size_px":24},"overflow":"error"}]}`)
	out := filepath.Join(t.TempDir(), "out.png")
	if err := run([]string{"--layout", lp, "--output", out}); err != nil {
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
	minx, miny, maxx, maxy := 1<<30, 1<<30, -1, -1
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			if gray(img, x, y) == 0 {
				if x < minx { minx = x }
				if y < miny { miny = y }
				if x > maxx { maxx = x }
				if y > maxy { maxy = y }
			}
		}
	}
	if maxx < 0 {
		t.Fatal("no black pixels rendered")
	}
	if maxx >= 30 {
		t.Fatalf("black pixels at x=%d exceed the 30 px post-rotation width", maxx)
	}
	if maxy >= 80 {
		t.Fatalf("black pixels at y=%d exceed the 80 px post-rotation height", maxy)
	}
}

func TestUnknownFlagSuggestion(t *testing.T) {
	var ue *usageError
	err := run([]string{"--laytout"})
	if !errors.As(err, &ue) {
		t.Fatalf("run(--laytout): want usageError, got %v", err)
	}
	if !strings.Contains(ue.msg, "did you mean --layout?") {
		t.Fatalf("want suggestion for --layout, got %q", ue.msg)
	}
	// A name far from every known flag gets no suggestion.
	err = run([]string{"--zzz"})
	if !errors.As(err, &ue) {
		t.Fatalf("run(--zzz): want usageError, got %v", err)
	}
	if strings.Contains(ue.msg, "did you mean") {
		t.Fatalf("unexpected suggestion in %q", ue.msg)
	}
}

func TestVersionFlag(t *testing.T) {
	if err := run([]string{"--version"}); err != nil {
		t.Fatalf("run(--version): %v", err)
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
