// Package compose rotates block bitmaps, checks block geometry and
// composes the final black-and-white image.
package compose

import (
	"fmt"
	"image"
	"image/color"
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

// Rotate rotates a bitmap clockwise by deg (0, 90, 180 or 270).
func Rotate(img *image.Gray, deg int) *image.Gray {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	switch deg % 360 {
	case 0:
		return copyGray(img)
	case 90: // clockwise: (x,y) -> (h-1-y, x)
		out := image.NewGray(image.Rect(0, 0, h, w))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				out.SetGray(h-1-y, x, img.GrayAt(x, y))
			}
		}
		return out
	case 180: // (x,y) -> (w-1-x, h-1-y)
		out := image.NewGray(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				out.SetGray(w-1-x, h-1-y, img.GrayAt(x, y))
			}
		}
		return out
	case 270: // clockwise: (x,y) -> (y, w-1-x)
		out := image.NewGray(image.Rect(0, 0, h, w))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				out.SetGray(y, w-1-x, img.GrayAt(x, y))
			}
		}
		return out
	default:
		panic(fmt.Sprintf("rotate: unsupported angle %d", deg))
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

// CheckBoundsAndOverlaps verifies that all placed rectangles are inside the
// canvas and do not overlap each other.
func CheckBoundsAndOverlaps(placed []NamedRect, canvas Rect) error {
	for i := range placed {
		if !placed[i].R.Inside(canvas) {
			return fmt.Errorf("block %q: rectangle (%d,%d %dx%d) is outside the %dx%d canvas",
				placed[i].Name, placed[i].R.X, placed[i].R.Y, placed[i].R.W, placed[i].R.H, canvas.W, canvas.H)
		}
		for j := i + 1; j < len(placed); j++ {
			if placed[i].R.Intersects(placed[j].R) {
				return fmt.Errorf("blocks %q and %q overlap", placed[i].Name, placed[j].Name)
			}
		}
	}
	return nil
}

// Finalize verifies that the image contains exactly two colors.
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
