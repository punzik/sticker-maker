// Package compose rotates block bitmaps, checks block geometry and
// composes the final black-and-white image.
package compose

import (
	"fmt"
	"image"
	"image/color"
	"math"
)

// Rect is an axis-aligned rectangle with the top-left corner at (X, Y).
type Rect struct {
	X, Y, W, H int
}

// Inside reports whether r lies fully inside outer.
func (r Rect) Inside(outer Rect) bool {
	return r.X >= outer.X && r.Y >= outer.Y &&
		r.X+r.W <= outer.X+outer.W && r.Y+r.H <= outer.Y+outer.H
}

// Intersects reports whether the two rectangles overlap with area > 0.
// Touching edges do not count as intersection.
func (a Rect) Intersects(b Rect) bool {
	return a.X < b.X+b.W && b.X < a.X+a.W && a.Y < b.Y+b.H && b.Y < a.Y+a.H
}

// Rotate rotates a bitmap clockwise by deg. Only multiples of 90 are
// supported; anything else is an error (config validation makes this
// unreachable from the CLI, but the library should not panic).
func Rotate(img *image.Gray, deg int) (*image.Gray, error) {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	switch deg % 360 {
	case 0:
		return copyGray(img), nil
	case 90: // clockwise: (x,y) -> (h-1-y, x)
		out := image.NewGray(image.Rect(0, 0, h, w))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				out.SetGray(h-1-y, x, img.GrayAt(x, y))
			}
		}
		return out, nil
	case 180: // (x,y) -> (w-1-x, h-1-y)
		out := image.NewGray(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				out.SetGray(w-1-x, h-1-y, img.GrayAt(x, y))
			}
		}
		return out, nil
	case 270: // clockwise: (x,y) -> (y, w-1-x)
		out := image.NewGray(image.Rect(0, 0, h, w))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				out.SetGray(y, w-1-x, img.GrayAt(x, y))
			}
		}
		return out, nil
	default:
		return nil, fmt.Errorf("rotate: unsupported angle %d (want a multiple of 90)", deg)
	}
}

func copyGray(img *image.Gray) *image.Gray {
	out := image.NewGray(img.Rect)
	for y := 0; y < img.Rect.Dy(); y++ {
		for x := 0; x < img.Rect.Dx(); x++ {
			out.SetGray(x, y, img.GrayAt(x, y))
		}
	}
	return out
}

// Place draws the block bitmap into dst at the block's position.
// The bitmap must already be rotated.
func Place(dst *image.Gray, blk *image.Gray, x, y int) {
	for oy := 0; oy < blk.Rect.Dy(); oy++ {
		for ox := 0; ox < blk.Rect.Dx(); ox++ {
			if blk.GrayAt(ox, oy).Y == 0 {
				dst.SetGray(x+ox, y+oy, color.Gray{0})
			}
		}
	}
}

// NewCanvas creates the all-white output canvas.
func NewCanvas(w, h int) *image.Gray {
	img := image.NewGray(image.Rect(0, 0, w, h))
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			img.SetGray(x, y, color.Gray{255})
		}
	}
	return img
}

// NamedRect pairs a block name with its placed rectangle (after rotation).
type NamedRect struct {
	Name string
	R    Rect
}

// CheckBounds verifies that all placed rectangles are inside the canvas.
func CheckBounds(placed []NamedRect, canvas Rect) error {
	for i := range placed {
		if !placed[i].R.Inside(canvas) {
			return fmt.Errorf("block %q: rectangle (%d,%d %dx%d) is outside the %dx%d canvas",
				placed[i].Name, placed[i].R.X, placed[i].R.Y, placed[i].R.W, placed[i].R.H, canvas.W, canvas.H)
		}
	}
	return nil
}

// CheckOverlaps, when forbidOverlap is true, verifies that no two placed
// rectangles overlap each other.
func CheckOverlaps(placed []NamedRect, forbidOverlap bool) error {
	if !forbidOverlap {
		return nil
	}
	for i := range placed {
		for j := i + 1; j < len(placed); j++ {
			if placed[i].R.Intersects(placed[j].R) {
				return fmt.Errorf("blocks %q and %q overlap", placed[i].Name, placed[j].Name)
			}
		}
	}
	return nil
}

// lineSeg is a flat-capped line stroke in continuous coordinates.
type lineSeg struct {
	ax, ay, dx, dy, l2, r2 float64
	thin                   bool // width 1: nearest-pixel sweep for connectivity
}

// newLineSeg builds the stroke of width pixels around segment
// (x1, y1) - (x2, y2); endpoint coordinates are pixel centers.
func newLineSeg(x1, y1, x2, y2, width int) lineSeg {
	return lineSeg{
		ax: float64(x1) + 0.5, ay: float64(y1) + 0.5,
		dx: float64(x2 - x1), dy: float64(y2 - y1),
		l2:   float64(x2-x1)*float64(x2-x1) + float64(y2-y1)*float64(y2-y1),
		r2:   float64(width-1) * float64(width-1) / 4,
		thin: width == 1,
	}
}

// contains reports whether the center of pixel (px, py) belongs to the
// stroke: within (width-1)/2 of the segment and projected onto it.
func (s lineSeg) contains(px, py int) bool {
	cx, cy := float64(px)+0.5, float64(py)+0.5
	if s.l2 == 0 { // degenerate: a point
		dx, dy := cx-s.ax, cy-s.ay
		return dx*dx+dy*dy <= s.r2+1e-9
	}
	if s.thin {
		// A zero-radius band would hit only pixel centers exactly on the
		// line, leaving gaps in shallow diagonals; instead take the pixel
		// whose center is nearest to the line in the sweep direction
		// (Bresenham-style), which stays 8-connected.
		if math.Abs(s.dx) >= math.Abs(s.dy) {
			t := (cx - s.ax) / s.dx
			return t >= 0 && t <= 1 && py == int(math.Floor(s.ay+t*s.dy))
		}
		t := (cy - s.ay) / s.dy
		return t >= 0 && t <= 1 && px == int(math.Floor(s.ax+t*s.dx))
	}
	t := ((cx-s.ax)*s.dx + (cy-s.ay)*s.dy) / s.l2
	if t < 0 || t > 1 {
		return false // flat cap
	}
	nx, ny := s.ax+t*s.dx, s.ay+t*s.dy
	qx, qy := cx-nx, cy-ny
	return qx*qx+qy*qy <= s.r2+1e-9
}

// LineExtent returns the pixel bounds of a line stroke and whether it
// covers at least one pixel.
func LineExtent(x1, y1, x2, y2, width int) (Rect, bool) {
	seg := newLineSeg(x1, y1, x2, y2, width)
	minx, maxx := x1, x2
	if minx > maxx {
		minx, maxx = maxx, minx
	}
	miny, maxy := y1, y2
	if miny > maxy {
		miny, maxy = maxy, miny
	}
	pad := width
	var bnd Rect
	got := false
	for py := miny - pad; py <= maxy+pad; py++ {
		for px := minx - pad; px <= maxx+pad; px++ {
			if !seg.contains(px, py) {
				continue
			}
			if !got {
				bnd = Rect{X: px, Y: py, W: 1, H: 1}
				got = true
				continue
			}
			if px < bnd.X {
				bnd.W += bnd.X - px
				bnd.X = px
			}
			if px+1 > bnd.X+bnd.W {
				bnd.W = px + 1 - bnd.X
			}
			if py < bnd.Y {
				bnd.H += bnd.Y - py
				bnd.Y = py
			}
			if py+1 > bnd.Y+bnd.H {
				bnd.H = py + 1 - bnd.Y
			}
		}
	}
	return bnd, got
}

// DrawLine paints a line stroke (see LineExtent) onto dst in black,
// clipping to dst bounds and without antialiasing.
func DrawLine(dst *image.Gray, x1, y1, x2, y2, width int) {
	seg := newLineSeg(x1, y1, x2, y2, width)
	bnd, got := LineExtent(x1, y1, x2, y2, width)
	if !got {
		return
	}
	c := dst.Bounds()
	for y := max(c.Min.Y, bnd.Y); y < min(c.Max.Y, bnd.Y+bnd.H); y++ {
		for x := max(c.Min.X, bnd.X); x < min(c.Max.X, bnd.X+bnd.W); x++ {
			if seg.contains(x, y) {
				dst.SetGray(x, y, color.Gray{0})
			}
		}
	}
}

// Finalize verifies that the output contains no anti-aliased or gray
// pixels: every pixel is black (0) or white (255). An image using fewer
// than two colors is valid — for example a layout with no content.
func Finalize(img *image.Gray) error {
	seen := map[uint8]bool{}
	for y := 0; y < img.Rect.Dy(); y++ {
		for x := 0; x < img.Rect.Dx(); x++ {
			seen[img.GrayAt(x, y).Y] = true
		}
	}
	for v := range seen {
		if v != 0 && v != 255 {
			return fmt.Errorf("internal error: pixel value %d in output (want 0 or 255)", v)
		}
	}
	return nil
}
