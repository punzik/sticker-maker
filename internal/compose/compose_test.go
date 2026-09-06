package compose

import (
	"image"
	"image/color"
	"testing"
)

// L-shaped 3x2 bitmap to make orientation checks unambiguous:
//
//	X .
//	X X
func lShape() *image.Gray {
	img := image.NewGray(image.Rect(0, 0, 3, 2))
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			img.SetGray(x, y, color.Gray{0xff})
		}
	}
	img.SetGray(0, 0, color.Gray{0})
	img.SetGray(0, 1, color.Gray{0})
	img.SetGray(1, 1, color.Gray{0})
	return img
}

func pix(img *image.Gray, x, y int) bool {
	return img.GrayAt(x, y).Y == 0
}

func TestRotate(t *testing.T) {
	src := lShape()
	cases := []struct {
		deg   int
		w, h  int
		black [][2]int // black pixel positions (x, y)
	}{
		// L shape: (0,0),(0,1),(1,1); w=3,h=2
		{0, 3, 2, [][2]int{{0, 0}, {0, 1}, {1, 1}}},
		// 90 clockwise: (x,y) -> (h-1-y, x)
		{90, 2, 3, [][2]int{{1, 0}, {0, 0}, {0, 1}}},
		{180, 3, 2, [][2]int{{2, 1}, {2, 0}, {1, 0}}},
		// 270 clockwise: (x,y) -> (y, w-1-x)
		{270, 2, 3, [][2]int{{0, 2}, {1, 2}, {1, 1}}},
	}
	for _, c := range cases {
		got, err := Rotate(src, c.deg)
		if err != nil {
			t.Fatalf("rot %d: %v", c.deg, err)
		}
		if got.Rect.Dx() != c.w || got.Rect.Dy() != c.h {
			t.Fatalf("rot %d: size %dx%d want %dx%d", c.deg, got.Rect.Dx(), got.Rect.Dy(), c.w, c.h)
		}
		black := map[[2]int]bool{}
		for y := 0; y < c.h; y++ {
			for x := 0; x < c.w; x++ {
				if pix(got, x, y) {
					black[[2]int{x, y}] = true
				}
			}
		}
		want := map[[2]int]bool{}
		for _, p := range c.black {
			want[[2]int{p[0], p[1]}] = true
		}
		if len(black) != len(want) {
			t.Fatalf("rot %d: %d black px, want %d (%v)", c.deg, len(black), len(want), black)
		}
		for p := range want {
			if !black[p] {
				t.Fatalf("rot %d: missing black px %v (got %v)", c.deg, p, black)
			}
		}
	}
}

func TestRotateBadAngle(t *testing.T) {
	if _, err := Rotate(lShape(), 45); err == nil {
		t.Fatal("want error for non-multiple-of-90 angle")
	}
}

func TestRect(t *testing.T) {
	a := Rect{X: 0, Y: 0, W: 10, H: 10}
	if !a.Inside(Rect{W: 10, H: 10}) {
		t.Fatal("exact fit must be inside")
	}
	if a.Inside(Rect{W: 9, H: 10}) {
		t.Fatal("wider canvas check")
	}
	if (Rect{X: 5, Y: 5, W: 10, H: 10}).Inside(a) {
		t.Fatal("protruding rect must not be inside")
	}
	// touching edges: no intersection
	if a.Intersects(Rect{X: 10, Y: 0, W: 5, H: 5}) {
		t.Fatal("touching edge must not intersect")
	}
	if !a.Intersects(Rect{X: 9, Y: 0, W: 5, H: 5}) {
		t.Fatal("1px overlap must intersect")
	}
	if a.Intersects(Rect{X: 20, Y: 20, W: 5, H: 5}) {
		t.Fatal("disjoint must not intersect")
	}
}

func TestCheckBounds(t *testing.T) {
	canvas := Rect{W: 100, H: 50}
	ok := []NamedRect{
		{Name: "a", R: Rect{X: 0, Y: 0, W: 50, H: 50}},
		{Name: "b", R: Rect{X: 50, Y: 0, W: 50, H: 50}}, // touches a
	}
	if err := CheckBounds(ok, canvas); err != nil {
		t.Fatalf("valid layout: %v", err)
	}
	out := append([]NamedRect{}, ok...)
	out = append(out, NamedRect{Name: "c", R: Rect{X: 90, Y: 0, W: 20, H: 10}})
	if err := CheckBounds(out, canvas); err == nil {
		t.Fatal("out-of-bounds must be reported")
	}
}

func TestCheckOverlaps(t *testing.T) {
	ovl := []NamedRect{
		{Name: "a", R: Rect{X: 0, Y: 0, W: 50, H: 50}},
		{Name: "b", R: Rect{X: 49, Y: 0, W: 50, H: 50}},
	}
	if err := CheckOverlaps(ovl, true); err == nil {
		t.Fatal("overlap must be reported")
	}
	if err := CheckOverlaps(ovl, false); err != nil {
		t.Fatalf("overlap must be allowed by default: %v", err)
	}
}

func TestLineExtent(t *testing.T) {
	// Horizontal line: x span is exactly the endpoints, y is centered.
	if r, got := LineExtent(0, 5, 9, 5, 3); !got || r != (Rect{X: 0, Y: 4, W: 10, H: 3}) {
		t.Fatalf("horizontal: %+v %v", r, got)
	}
	// Reversed endpoints and a single-pixel width.
	if r, got := LineExtent(9, 5, 0, 5, 1); !got || r != (Rect{X: 0, Y: 5, W: 10, H: 1}) {
		t.Fatalf("reversed: %+v %v", r, got)
	}
	// Vertical line.
	if r, got := LineExtent(3, 1, 3, 4, 2); !got || r != (Rect{X: 3, Y: 1, W: 1, H: 4}) {
		t.Fatalf("vertical: %+v %v", r, got)
	}
	// Even width: the stroke is centered on the segment, so a width of
	// 4 covers 3 pixel rows perpendicular to it.
	if r, got := LineExtent(0, 5, 9, 5, 4); !got || r != (Rect{X: 0, Y: 4, W: 10, H: 3}) {
		t.Fatalf("even width: %+v %v", r, got)
	}
	// Degenerate point: a filled square of width x width.
	if r, got := LineExtent(4, 4, 4, 4, 3); !got || r != (Rect{X: 3, Y: 3, W: 3, H: 3}) {
		t.Fatalf("point: %+v %v", r, got)
	}
}

func TestDrawLine(t *testing.T) {
	dst := NewCanvas(12, 12)
	DrawLine(dst, 1, 5, 10, 5, 3)
	for y := 4; y <= 6; y++ {
		for x := 1; x <= 10; x++ {
			if !pix(dst, x, y) {
				t.Fatalf("missing px (%d,%d)", x, y)
			}
		}
	}
	if pix(dst, 0, 5) {
		t.Fatal("flat cap: pixel before the start must stay white")
	}
	if pix(dst, 11, 5) {
		t.Fatal("flat cap: pixel after the end must stay white")
	}
	if pix(dst, 5, 3) || pix(dst, 5, 7) {
		t.Fatal("line wider than requested")
	}

	// A 1px diagonal connects through corner-adjacent pixels.
	dst = NewCanvas(6, 6)
	DrawLine(dst, 0, 0, 5, 5, 1)
	for i := 0; i < 6; i++ {
		if !pix(dst, i, i) {
			t.Fatalf("missing diagonal px (%d,%d)", i, i)
		}
	}

	// A line fully outside the canvas must not panic or draw.
	dst = NewCanvas(4, 4)
	DrawLine(dst, 10, 10, 20, 20, 3)
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			if pix(dst, x, y) {
				t.Fatal("out-of-canvas line drew something")
			}
		}
	}
}
