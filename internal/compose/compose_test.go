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
		got := Rotate(src, c.deg)
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

func TestCheckBoundsAndOverlaps(t *testing.T) {
	canvas := Rect{W: 100, H: 50}
	ok := []NamedRect{
		{Name: "a", R: Rect{X: 0, Y: 0, W: 50, H: 50}},
		{Name: "b", R: Rect{X: 50, Y: 0, W: 50, H: 50}}, // touches a
	}
	if err := CheckBoundsAndOverlaps(ok, canvas); err != nil {
		t.Fatalf("valid layout: %v", err)
	}
	out := append([]NamedRect{}, ok...)
	out = append(out, NamedRect{Name: "c", R: Rect{X: 90, Y: 0, W: 20, H: 10}})
	if err := CheckBoundsAndOverlaps(out, canvas); err == nil {
		t.Fatal("out-of-bounds must be reported")
	}
	ovl := []NamedRect{
		{Name: "a", R: Rect{X: 0, Y: 0, W: 50, H: 50}},
		{Name: "b", R: Rect{X: 49, Y: 0, W: 50, H: 50}},
	}
	if err := CheckBoundsAndOverlaps(ovl, canvas); err == nil {
		t.Fatal("overlap must be reported")
	}
}
