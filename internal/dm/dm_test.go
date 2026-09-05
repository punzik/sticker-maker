package dm

import (
	"strings"
	"testing"
)

func TestSupported(t *testing.T) {
	for _, s := range Squares {
		if !Supported(s, s) {
			t.Errorf("square %dx%d should be supported", s, s)
		}
	}
	for _, r := range Rectangles {
		if !Supported(r[0], r[1]) {
			t.Errorf("rectangle %dx%d should be supported", r[0], r[1])
		}
		if Supported(r[1], r[0]) {
			t.Errorf("transposed rectangle %dx%d should not be supported", r[1], r[0])
		}
	}
	for _, bad := range [][2]int{{11, 11}, {6, 10}, {16, 64}, {10, 12}, {0, 0}} {
		if Supported(bad[0], bad[1]) {
			t.Errorf("%dx%d should not be supported", bad[0], bad[1])
		}
	}
}

func TestEncodeErrors(t *testing.T) {
	if _, err := Encode("", 24, 24); err == nil {
		t.Fatal("empty content should fail")
	}
	if _, err := Encode("Привет", 24, 24); err == nil {
		t.Fatal("non-ASCII content should fail")
	}
	if _, err := Encode("ABC", 11, 11); err == nil {
		t.Fatal("non-standard size should fail")
	}
	// 10x10 holds 3 ASCII characters; 4 must fail.
	if _, err := Encode("ABCD", 10, 10); err == nil {
		t.Fatal("overflow must fail")
	}
	// 10x10 holds exactly 3.
	if _, err := Encode("ABC", 10, 10); err != nil {
		t.Fatalf("3 chars in 10x10: %v", err)
	}
}

// TestAllSizesRoundTrip encodes a payload that fits each supported size and
// confirms Encode's built-in decoder verification accepts it.
func TestAllSizesRoundTrip(t *testing.T) {
	payload := func(rows, cols int) string {
		switch {
		case rows == 10 && cols == 10:
			return "ABC"
		default:
			return "A"
		}
	}
	for _, s := range Squares {
		if _, err := Encode(payload(s, s), s, s); err != nil {
			t.Errorf("%dx%d: %v", s, s, err)
		}
	}
	for _, r := range Rectangles {
		if _, err := Encode(payload(r[0], r[1]), r[0], r[1]); err != nil {
			t.Errorf("%dx%d: %v", r[0], r[1], err)
		}
	}
	// 144x144 overflow guard (ASCII capacity 1304 codewords).
	if _, err := Encode(strings.Repeat("A", dataCodewords144+1), 144, 144); err == nil {
		t.Error("144x144 overflow should be rejected")
	}
}

// TestEncodeOverflowDetected checks that content that physically does not
// fit is rejected, including the case where the upstream writer silently
// emits an undecodable symbol.
func TestEncodeOverflowDetected(t *testing.T) {
	for _, c := range []string{"ABCD", "TOOLONG", "ab-cd", "0123456789"} {
		if _, err := Encode(c, 10, 10); err == nil {
			t.Errorf("%q in 10x10: expected overflow error", c)
		}
	}
}

func TestEncodeSizes(t *testing.T) {
	if bm, err := Encode("PART-00123", 24, 24); err != nil {
		t.Fatal(err)
	} else if bm.GetWidth() != 24 || bm.GetHeight() != 24 {
		t.Fatalf("got %dx%d", bm.GetWidth(), bm.GetHeight())
	}
	if bm, err := Encode("HELLO", 8, 18); err != nil {
		t.Fatal(err)
	} else if bm.GetWidth() != 18 || bm.GetHeight() != 8 {
		t.Fatalf("got %dx%d", bm.GetWidth(), bm.GetHeight())
	}
}

func TestRender(t *testing.T) {
	bm, err := Encode("ABC", 10, 10)
	if err != nil {
		t.Fatal(err)
	}
	img := Render(bm, 4, 2)
	w, h := img.Rect.Dx(), img.Rect.Dy()
	if ew, eh := Size(10, 10, 4, 2); w != ew || h != eh {
		t.Fatalf("size %dx%d, want %dx%d", w, h, ew, eh)
	}
	// quiet zone must be white: corner pixels and the whole border.
	if img.GrayAt(0, 0).Y != 0xff || img.GrayAt(w-1, h-1).Y != 0xff {
		t.Fatal("quiet zone corners must be white")
	}
	for x := 0; x < w; x++ {
		if img.GrayAt(x, 0).Y != 0xff || img.GrayAt(x, h-1).Y != 0xff {
			t.Fatal("quiet zone rows must be white")
		}
	}
	for y := 0; y < h; y++ {
		if img.GrayAt(0, y).Y != 0xff || img.GrayAt(w-1, y).Y != 0xff {
			t.Fatal("quiet zone columns must be white")
		}
	}
	// only two colors overall
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := img.GrayAt(x, y).Y
			if v != 0 && v != 0xff {
				t.Fatalf("pixel (%d,%d) = %d, want 0 or 255", x, y, v)
			}
		}
	}
	// Data Matrix has a solid bottom row of the symbol area.
	qz := 2 * 4
	symBottom := qz + 10*4 - 1
	for x := qz; x < qz+10*4; x++ {
		if img.GrayAt(x, symBottom).Y != 0 {
			t.Fatalf("solid bottom row broken at x=%d", x)
		}
	}
}
