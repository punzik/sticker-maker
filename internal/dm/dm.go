// Package dm encodes fixed-size Data Matrix ECC 200 symbols and renders
// them to black-and-white bitmaps.
package dm

import (
	"fmt"
	"image"
	"image/color"

	zx "github.com/makiuchi-d/gozxing"
	zxdm "github.com/makiuchi-d/gozxing/datamatrix"
	zxenc "github.com/makiuchi-d/gozxing/datamatrix/encoder"
)

// Squares are the supported square symbol sizes (modules).
var Squares = []int{10, 12, 14, 16, 18, 20, 22, 24, 26, 32, 36, 40, 44, 48, 52, 64, 72, 80, 88, 96, 104, 120, 132, 144}

// Rectangles are the supported rectangular symbol sizes, rows x columns.
// Rectangular Data Matrix symbols are landscape: rows < columns.
var Rectangles = [][2]int{
	{8, 18}, {8, 32}, {12, 26}, {12, 36}, {16, 36}, {16, 48},
}

// Supported reports whether rows x columns is a supported symbol size.
func Supported(rows, columns int) bool {
	if rows == columns {
		for _, s := range Squares {
			if s == rows {
				return true
			}
		}
		return false
	}
	for _, r := range Rectangles {
		if r[0] == rows && r[1] == columns {
			return true
		}
	}
	return false
}

// Encode encodes content (ASCII, non-empty) into a fixed-size Data Matrix
// symbol of rows x columns modules.
func Encode(content string, rows, columns int) (*zx.BitMatrix, error) {
	if content == "" {
		return nil, fmt.Errorf("datamatrix content must not be empty")
	}
	for _, r := range content {
		if r > 0x7f {
			return nil, fmt.Errorf("datamatrix content must be ASCII (found U+%04X)", r)
		}
	}
	if !Supported(rows, columns) {
		return nil, fmt.Errorf("unsupported datamatrix size %dx%d (rows x columns)", rows, columns)
	}
	dim, err := zx.NewDimension(columns, rows)
	if err != nil {
		return nil, err
	}
	hints := map[zx.EncodeHintType]interface{}{
		zx.EncodeHintType_MIN_SIZE: dim,
		zx.EncodeHintType_MAX_SIZE: dim,
	}
	if rows != columns {
		hints[zx.EncodeHintType_DATA_MATRIX_SHAPE] = zxenc.SymbolShapeHint_FORCE_RECTANGLE
	}
	bm, err := zxdm.NewDataMatrixWriter().Encode(content, zx.BarcodeFormat_DATA_MATRIX, 0, 0, hints)
	if err != nil {
		return nil, fmt.Errorf("datamatrix encode: %w", err)
	}
	if bm.GetWidth() != columns || bm.GetHeight() != rows {
		return nil, fmt.Errorf("datamatrix encode: expected %dx%d, got %dx%d", columns, rows, bm.GetWidth(), bm.GetHeight())
	}
	// The writer does not reliably reject content that does not fit a
	// forced size: it may emit a symbol that no decoder can read. Verify
	// every symbol by decoding it before returning.
	//
	// Exception: gozxing's own reader has a Reed-Solomon bug on the
	// 144x144 size (the symbols are valid — an independent decoder reads
	// them), so for that size we fall back to the ASCII capacity guard.
	// Pure ASCII content uses exactly one data codeword per character.
	if rows == 144 && columns == 144 {
		if len(content) > dataCodewords144 {
			return nil, fmt.Errorf("datamatrix content %q does not fit into a %dx%d symbol", content, rows, columns)
		}
		return bm, nil
	}
	if got, err := decode(bm); err != nil || got != content {
		return nil, fmt.Errorf("datamatrix content %q does not fit into a %dx%d symbol (or produced an invalid symbol)", content, rows, columns)
	}
	return bm, nil
}

// dataCodewords144 is the data capacity (in codewords) of a 144x144 symbol
// per ISO/IEC 16022. Used as the overflow guard for that size.
const dataCodewords144 = 1304

// decode renders the symbol with a quiet zone and decodes it, returning
// the recovered text.
func decode(bm *zx.BitMatrix) (string, error) {
	qz := 4
	img := image.NewGray(image.Rect(0, 0, bm.GetWidth()+2*qz, bm.GetHeight()+2*qz))
	for y := 0; y < img.Rect.Dy(); y++ {
		for x := 0; x < img.Rect.Dx(); x++ {
			img.SetGray(x, y, color.Gray{0xff})
		}
	}
	for y := 0; y < bm.GetHeight(); y++ {
		for x := 0; x < bm.GetWidth(); x++ {
			if bm.Get(x, y) {
				img.SetGray(x+qz, y+qz, color.Gray{0})
			}
		}
	}
	bb, err := zx.NewBinaryBitmapFromImage(img)
	if err != nil {
		return "", err
	}
	// PURE_BARCODE is required: without it the reader's symbol detector
	// mis-parses these images and fails Reed-Solomon.
	hints := map[zx.DecodeHintType]interface{}{
		zx.DecodeHintType_PURE_BARCODE: true,
		zx.DecodeHintType_TRY_HARDER:   true,
	}
	res, err := zxdm.NewDataMatrixReader().Decode(bb, hints)
	if err != nil {
		return "", err
	}
	return res.GetText(), nil
}

// Render renders the symbol with the given pixel size per module and a
// quiet zone of the given number of modules on every side.
func Render(bm *zx.BitMatrix, modulePx, quietModules int) *image.Gray {
	qz := quietModules * modulePx
	w := bm.GetWidth()*modulePx + 2*qz
	h := bm.GetHeight()*modulePx + 2*qz
	img := image.NewGray(image.Rect(0, 0, w, h))
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			img.SetGray(x, y, color.Gray{255})
		}
	}
	for y := 0; y < bm.GetHeight(); y++ {
		for x := 0; x < bm.GetWidth(); x++ {
			if !bm.Get(x, y) {
				continue
			}
			x0, y0 := qz+x*modulePx, qz+y*modulePx
			for yy := y0; yy < y0+modulePx; yy++ {
				for xx := x0; xx < x0+modulePx; xx++ {
					img.SetGray(xx, yy, color.Gray{0})
				}
			}
		}
	}
	return img
}

// Size returns the rendered pixel size (including quiet zone) for a symbol.
func Size(rows, columns, modulePx, quietModules int) (w, h int) {
	return columns*modulePx + 2*quietModules*modulePx,
		rows*modulePx + 2*quietModules*modulePx
}
