// Command sticker-maker renders sticker images (black-and-white PNG) from a
// JSON layout and named fields.
package main

import (
	"errors"
	"flag"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"sort"
	"strings"

	"sticker-maker/internal/compose"
	"sticker-maker/internal/config"
	"sticker-maker/internal/dm"
	"sticker-maker/internal/texteng"
)

func main() {
	err := run(os.Args[1:])
	if err == nil {
		return
	}
	if errors.Is(err, flag.ErrHelp) {
		printUsage(os.Stdout)
		return // --help/-h: print usage and exit 0
	}
	if err == errUsage {
		printUsage(os.Stderr)
	} else {
		fmt.Fprintln(os.Stderr, "sticker-maker:", err)
	}
	os.Exit(1)
}

var errUsage = fmt.Errorf("usage error")

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: sticker-maker --layout L.json --output out.png --field name=value ...")
	fmt.Fprintln(w, "       sticker-maker --list-fonts")
}

type fieldList []string

func (f *fieldList) String() string { return strings.Join(*f, ", ") }

func (f *fieldList) Set(s string) error {
	if _, _, err := config.ParseField(s); err != nil {
		return err
	}
	*f = append(*f, s)
	return nil
}

func run(args []string) error {
	fs := flag.NewFlagSet("sticker-maker", flag.ContinueOnError)
	fs.SetOutput(nil)
	fs.Usage = func() {} // usage is printed by main, not by the flag package
	var fields fieldList
	layoutPath := fs.String("layout", "", "path to the JSON layout file (required)")
	outputPath := fs.String("output", "", "path of the output PNG (required)")
	listFonts := fs.Bool("list-fonts", false, "list system fonts (family, style, file) and exit")
	fs.Var(&fields, "field", "field value, format name=value (repeatable)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return err // -h/--help: let main print usage and exit 0
		}
		return errUsage
	}
	if *listFonts {
		return printFonts(os.Stdout)
	}
	if *layoutPath == "" || *outputPath == "" {
		return errUsage
	}

	layout, err := config.Load(*layoutPath)
	if err != nil {
		return err
	}
	vals, err := parseFields(fields)
	if err != nil {
		return err
	}

	type placedBlock struct {
		name string
		img  *image.Gray
		rect compose.Rect
	}
	var placed []placedBlock

	for i := range layout.Blocks {
		b := &layout.Blocks[i]
		content, err := b.Content(vals)
		if err != nil {
			return err
		}
		if b.Type == "text" && strings.TrimSpace(content) == "" {
			// An empty text block would print as blank; reject it the way
			// datamatrix rejects empty content, per the README.
			return fmt.Errorf("block %q: content must not be empty", b.ID)
		}
		var blk *image.Gray
		var bw, bh int
		switch b.Type {
		case "text":
			face, err := texteng.Load(texteng.Options{
				Family: b.Font.Family,
				Style:  b.Font.Style,
				File:   b.Font.File,
				SizePx: b.Font.SizePx,
			})
			if err != nil {
				return fmt.Errorf("block %q: %w", b.ID, err)
			}
			res, err := face.Render(content, texteng.Params{
				Width: b.Width, Height: b.Height, ScaleX: *b.ScaleX,
				Wrap: *b.Wrap, Align: *b.Align, Valign: *b.Valign, Overflow: *b.Overflow,
			})
			if err != nil {
				return fmt.Errorf("block %q: %w", b.ID, err)
			}
			blk = res.Image
			bw, bh = b.Width, b.Height
		case "datamatrix":
			bm, err := dm.Encode(content, b.Symbol.Rows, b.Symbol.Columns)
			if err != nil {
				return fmt.Errorf("block %q: %w", b.ID, err)
			}
			blk = dm.Render(bm, b.ModulePx, *b.QuietZoneModules)
			bw, bh = dm.Size(b.Symbol.Rows, b.Symbol.Columns, b.ModulePx, *b.QuietZoneModules)
		}
		rot := compose.Rotate(blk, b.Rotation)
		rw, rh := bw, bh
		if b.Rotation%180 == 90 {
			rw, rh = bh, bw
		}
		placed = append(placed, placedBlock{
			name: b.ID,
			img:  rot,
			rect: compose.Rect{X: b.X, Y: b.Y, W: rw, H: rh},
		})
	}

	named := make([]compose.NamedRect, len(placed))
	for i, p := range placed {
		named[i] = compose.NamedRect{Name: p.name, R: p.rect}
	}
	if err := compose.CheckBoundsAndOverlaps(named, compose.Rect{W: layout.Image.Width, H: layout.Image.Height}); err != nil {
		return err
	}
	canvas := compose.NewCanvas(layout.Image.Width, layout.Image.Height)
	for _, p := range placed {
		compose.Place(canvas, p.img, p.rect.X, p.rect.Y)
	}
	if err := compose.Finalize(canvas); err != nil {
		return err
	}
	f, err := os.Create(*outputPath)
	if err != nil {
		return err
	}
	if err := png.Encode(f, canvas); err != nil {
		f.Close()
		return fmt.Errorf("write %s: %w", *outputPath, err)
	}
	return f.Close()
}

func parseFields(list []string) (config.Fields, error) {
	vals := make(config.Fields, len(list))
	for _, s := range list {
		name, value, err := config.ParseField(s)
		if err != nil {
			return nil, err
		}
		if _, dup := vals[name]; dup {
			return nil, fmt.Errorf("field %q given more than once", name)
		}
		vals[name] = value
	}
	return vals, nil
}

func printFonts(w io.Writer) error {
	lines, err := texteng.List()
	if err != nil {
		return err
	}
	sort.Strings(lines)
	for _, l := range lines {
		if l != "" {
			fmt.Fprintln(w, l)
		}
	}
	return nil
}
