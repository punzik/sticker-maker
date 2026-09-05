package texteng

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// Params describe the text block geometry and typography.
type Params struct {
	Width    int     // block width in px
	Height   int     // block height in px
	ScaleX   float64 // horizontal scale factor
	Wrap     string  // word | char | word_char | none
	Align    string  // left | center | right
	Valign   string  // top | center | bottom
	Overflow string  // error | clip
}

// RenderResult is a black-and-white bitmap of exactly Params.Width x
// Params.Height with the text drawn inside.
type RenderResult struct {
	Image *image.Gray
}

// Render lays out and rasterizes text into a block bitmap.
// It returns an error when the text does not fit and Overflow is "error",
// or when the font is missing a glyph for some rune.
func (f *Face) Render(text string, p Params) (*RenderResult, error) {
	if err := f.CheckGlyphs(text); err != nil {
		return nil, err
	}
	maxNatural := float64(p.Width) / p.ScaleX
	lines := Wrap(text, maxNatural, p.Wrap, f.Measure)

	// Check horizontal fit of each line.
	for _, ln := range lines {
		if ln.width*p.ScaleX > float64(p.Width)+1e-6 {
			if p.Overflow == "error" {
				return nil, fmt.Errorf("text line wider than block: %d px > %d px", int(math.Ceil(ln.width*p.ScaleX)), p.Width)
			}
		}
	}
	totalH := len(lines) * f.LineH
	if totalH > p.Height {
		if p.Overflow == "error" {
			return nil, fmt.Errorf("text does not fit vertically: %d px > %d px", totalH, p.Height)
		}
	}

	out := newWhiteImage(p.Width, p.Height)

	// Vertical start of the first line box.
	yOff := 0
	switch p.Valign {
	case "center":
		yOff = (p.Height - totalH) / 2
	case "bottom":
		yOff = p.Height - totalH
	}

	for i, ln := range lines {
		y := yOff + i*f.LineH
		if y >= p.Height {
			break // clip: nothing more fits
		}
		// Horizontal start of the line inside the block.
		scaledW := int(math.Round(ln.width * p.ScaleX))
		x := 0
		switch p.Align {
		case "center":
			x = (p.Width - scaledW) / 2
		case "right":
			x = p.Width - scaledW
		}
		f.drawLine(out, ln.text, x, y, p.ScaleX)
	}
	return &RenderResult{Image: out}, nil
}

// line is one wrapped line with its natural (unscaled) width in px.
type line struct {
	text  string
	width float64
}

// Wrap splits text into lines no wider than maxNatural (px, unscaled).
// Explicit newlines are always preserved. measure returns the natural
// width of a string.
func Wrap(text string, maxNatural float64, mode string, measure func(string) (float64, error)) []line {
	if text == "" {
		return nil
	}
	var lines []line
	for _, para := range splitParagraphs(text) {
		if para == "" {
			lines = append(lines, line{})
			continue
		}
		switch mode {
		case "none":
			w, _ := measure(para)
			lines = append(lines, line{para, w})
		case "word":
			lines = append(lines, wrapWords(para, maxNatural, measure, false)...)
		case "char":
			lines = append(lines, wrapChars(para, maxNatural, measure)...)
		case "word_char":
			lines = append(lines, wrapWords(para, maxNatural, measure, true)...)
		}
	}
	return lines
}

func splitParagraphs(text string) []string {
	var paras []string
	start := 0
	for i, r := range text {
		if r == '\n' {
			paras = append(paras, text[start:i])
			start = i + 1
		}
	}
	paras = append(paras, text[start:])
	return paras
}

// wrapWords does greedy word wrapping. When breakLong is true, a word that
// does not fit on an empty line is broken character by character.
func wrapWords(para string, maxW float64, measure func(string) (float64, error), breakLong bool) []line {
	var lines []line
	var cur []rune
	curW := 0.0
	flush := func() {
		if len(cur) == 0 {
			return
		}
		w, _ := measure(string(cur))
		lines = append(lines, line{string(cur), w})
		cur = nil
		curW = 0
	}
	runes := []rune(para)
	i := 0
	for i < len(runes) {
		if runes[i] == ' ' {
			// Spaces are consumed here and re-added exactly once when the
			// following word is appended; leading spaces of a line drop out.
			i++
			continue
		}
		// Read one word.
		j := i
		for j < len(runes) && runes[j] != ' ' {
			j++
		}
		word := string(runes[i:j])
		wordW, _ := measure(word)
		if len(cur) == 0 {
			if wordW <= maxW {
				cur = append(cur, []rune(word)...)
				curW += wordW
			} else if breakLong {
				lines = append(lines, wrapChars(word, maxW, measure)...)
			} else {
				// Does not fit; emit it alone (overflow is reported by the caller).
				lines = append(lines, line{word, wordW})
			}
			i = j
			continue
		}
		spaceW, _ := measure(" ")
		if curW+spaceW+wordW <= maxW {
			cur = append(cur, ' ')
			cur = append(cur, []rune(word)...)
			curW += spaceW + wordW
			i = j
		} else {
			// The word starts a new line; reprocess it with an empty line.
			flush()
		}
	}
	flush()
	return lines
}

func wrapChars(para string, maxW float64, measure func(string) (float64, error)) []line {
	var lines []line
	var cur []rune
	curW := 0.0
	for _, r := range para {
		rw, _ := measure(string(r))
		if len(cur) > 0 && curW+rw > maxW {
			w, _ := measure(string(cur))
			lines = append(lines, line{string(cur), w})
			cur = nil
			curW = 0
		}
		cur = append(cur, r)
		curW += rw
	}
	if len(cur) > 0 {
		w, _ := measure(string(cur))
		lines = append(lines, line{string(cur), w})
	}
	return lines
}

// drawLine rasterizes one line at natural scale, scales it horizontally and
// composites the thresholded pixels into dst at block coordinates.
func (f *Face) drawLine(dst *image.Gray, s string, x, y int, scaleX float64) {
	if s == "" {
		return
	}
	natW, _ := f.Measure(s)
	natPx := int(math.Ceil(natW))
	// Padding absorbs negative side bearings and rasterization overshoot.
	padX, padY := 8, f.LineH
	w := natPx + 2*padX
	h := padY*2 + f.LineH
	buf := image.NewGray(image.Rect(0, 0, w, h))
	for yy := 0; yy < h; yy++ {
		for xx := 0; xx < w; xx++ {
			buf.SetGray(xx, yy, color.Gray{255})
		}
	}
	dr := &font.Drawer{
		Dst:  buf,
		Src:  image.NewUniform(color.Black),
		Face: f.face,
		Dot:  fixed.Point26_6{X: fixed.I(padX), Y: fixed.I(padY + f.Ascent)},
	}
	dr.DrawString(s)

	// Horizontal bilinear scale + threshold into dst.
	outW := int(math.Round(float64(w) * scaleX))
	for oy := 0; oy < h; oy++ {
		ty := y - padY + oy
		if ty < 0 || ty >= dst.Rect.Dy() {
			continue
		}
		for ox := 0; ox < outW; ox++ {
			tx := x - int(math.Round(float64(padX)*scaleX)) + ox
			if tx < 0 || tx >= dst.Rect.Dx() {
				continue
			}
			sx := float64(ox) / scaleX
			i0 := int(sx)
			if i0 >= w {
				continue
			}
			frac := sx - float64(i0)
			var g float64
			if i0+1 < w {
				g = float64(buf.GrayAt(i0, oy).Y)*(1-frac) + float64(buf.GrayAt(i0+1, oy).Y)*frac
			} else {
				g = float64(buf.GrayAt(i0, oy).Y)
			}
			if g < Threshold {
				dst.SetGray(tx, ty, color.Gray{0})
			}
		}
	}
}

// newWhiteImage creates an all-white gray image of the given size.
func newWhiteImage(w, h int) *image.Gray {
	img := image.NewGray(image.Rect(0, 0, w, h))
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			img.SetGray(x, y, color.Gray{255})
		}
	}
	return img
}
