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
	"path/filepath"
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
	var ue *usageError
	switch {
	case errors.Is(err, flag.ErrHelp):
		printUsage(os.Stdout) // -h/--help: print usage and exit 0
	case err == errUsage, errors.As(err, &ue):
		// Usage error: show the explanation (if any), the usage text,
		// and the conventional exit code.
		if ue != nil && ue.msg != "" {
			fmt.Fprintln(os.Stderr, "sticker-maker:", ue.msg)
		}
		printUsage(os.Stderr)
		os.Exit(2)
	default:
		fmt.Fprintln(os.Stderr, "sticker-maker:", err)
		os.Exit(1) // runtime error
	}
}

var errUsage = fmt.Errorf("usage error")

// usageError is a usage error with a user-facing explanation to print
// above the usage text.
type usageError struct{ msg string }

func (e *usageError) Error() string { return e.msg }

// version is the tool version; override at build time with
// -ldflags "-X main.version=...".
var version = "0.1.0"

// usageText follows the CLI guidelines: description first, usage lines,
// flag descriptions, an example, and a pointer to --help.
const usageText = `sticker-maker renders sticker images (black-and-white PNG) from a JSON
layout and named field values.

Usage:
  sticker-maker --layout L.json --output out.png [--field name=value]...
  sticker-maker --list-fonts

Options:
  --layout FILE       path to the JSON layout file (required when rendering)
  --output FILE       path of the output PNG (required when rendering)
  --field NAME=VALUE  field value; may be repeated
  --list-fonts        list available fonts (family, style, file) and exit
  --version           print version information and exit
  -h, --help          show this help

Example:
  sticker-maker --layout layouts/combined.json \
    --field "title=Resistor 10 kOhm" --field "code=PART-00123" \
    --output label.png
`

func printUsage(w io.Writer) {
	fmt.Fprint(w, usageText)
}

// flagParseMessage renders a flag package parse error and, for an
// unknown flag, appends a "did you mean" suggestion for the closest
// known flag name.
func flagParseMessage(fs *flag.FlagSet, err error) string {
	msg := err.Error()
	const prefix = "flag provided but not defined: "
	i := strings.Index(msg, prefix)
	if i < 0 {
		return msg
	}
	name := strings.TrimLeft(strings.TrimSpace(msg[i+len(prefix):]), "-")
	if hint := suggestFlag(fs, name); hint != "" {
		msg += fmt.Sprintf(" (did you mean --%s?)", hint)
	}
	return msg
}

// suggestFlag returns the closest flag name within a small edit distance.
func suggestFlag(fs *flag.FlagSet, name string) string {
	best, bestDist := "", 1<<32
	fs.VisitAll(func(f *flag.Flag) {
		if d := editDistance(f.Name, name); d < bestDist {
			best, bestDist = f.Name, d
		}
	})
	if bestDist <= 2 {
		return best
	}
	return ""
}

func editDistance(a, b string) int {
	dist := make([]int, len(b)+1)
	for j := range dist {
		dist[j] = j
	}
	for i := 1; i <= len(a); i++ {
		prev := dist[0]
		dist[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			prev, dist[j] = dist[j], min3(dist[j]+1, dist[j-1]+1, prev+cost)
		}
	}
	return dist[len(b)]
}

func min3(a, b, c int) int {
	if b < a {
		a = b
	}
	if c < a {
		a = c
	}
	return a
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
	// The flag package never prints here (SetOutput(nil) would reset the
	// destination back to stderr, so discard it explicitly); all messages
	// are printed by main, and flag documentation lives in usageText.
	fs.SetOutput(io.Discard)
	fs.Usage = func() {} // usage is printed by main, not by the flag package
	var fields fieldList
	layoutPath := fs.String("layout", "", "")
	outputPath := fs.String("output", "", "")
	listFonts := fs.Bool("list-fonts", false, "")
	showVersion := fs.Bool("version", false, "")
	fs.Var(&fields, "field", "")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return err // -h/--help: let main print usage and exit 0
		}
		return &usageError{msg: flagParseMessage(fs, err)}
	}
	if *showVersion {
		fmt.Fprintf(os.Stdout, "sticker-maker %s\n", version)
		return nil
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
			// width/height describe the block's final rectangle after
			// rotation, so for a 90/270 rotation the text is laid out in a
			// transposed box that becomes width x height after rotating.
				rw, rh := b.Width, b.Height
				if b.Rotation%180 == 90 {
					rw, rh = b.Height, b.Width
				}
				res, err := face.Render(content, texteng.Params{
					Width: rw, Height: rh, ScaleX: *b.ScaleX,
					Wrap: *b.Wrap, Align: *b.Align, Valign: *b.Valign, Overflow: *b.Overflow,
				})
				if err != nil {
					return fmt.Errorf("block %q: %w", b.ID, err)
				}
				blk = res.Image
				bw, bh = rw, rh
		case "datamatrix":
			bm, err := dm.Encode(content, b.Symbol.Rows, b.Symbol.Columns)
			if err != nil {
				return fmt.Errorf("block %q: %w", b.ID, err)
			}
			blk = dm.Render(bm, b.ModulePx, *b.QuietZoneModules)
			bw, bh = dm.Size(b.Symbol.Rows, b.Symbol.Columns, b.ModulePx, *b.QuietZoneModules)
		default:
			// Config validation rejects other types; guard anyway so a
			// future caller cannot nil-deref blk.
			return fmt.Errorf("block %q: unsupported type %q", b.ID, b.Type)
		}
		rot, err := compose.Rotate(blk, b.Rotation)
		if err != nil {
			return fmt.Errorf("block %q: %w", b.ID, err)
		}
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
	if err := writePNG(*outputPath, canvas); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "wrote %s (%dx%d)\n", *outputPath, layout.Image.Width, layout.Image.Height)
	return nil
}

// writePNG encodes the canvas into a temporary file in the destination
// directory and renames it into place, so a failure while encoding or
// writing never leaves a truncated result file behind.
func writePNG(path string, canvas image.Image) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".sticker-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err := png.Encode(f, canvas); err != nil {
		f.Close()
		return fmt.Errorf("write %s: %w", path, err)
	}
	return finishTemp(f, path, tmp)
}

// finishTemp fsyncs, closes and renames a fully written temp file.
func finishTemp(f *os.File, path, tmp string) error {
	if err := f.Sync(); err != nil {
		f.Close()
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
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
