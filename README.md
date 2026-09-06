# sticker-maker

`sticker-maker` is a Linux command-line tool that renders sticker images from a JSON layout and named field values. Layouts can contain text, Data Matrix ECC 200 and line blocks. The output is an opaque black-and-white PNG with pixel-based dimensions and coordinates.

The tool only generates images; it does not communicate with a printer.

## Requirements

- Go (see `go.mod` for the required version)
- fontconfig (`fc-match` and `fc-list`) and installed system fonts when text blocks select fonts by family

A Nix development environment is included:

```sh
nix-shell
```

It provides Go, fontconfig, DejaVu fonts, and `dmtx-utils`.

## Build

```sh
go build -o sticker-maker .
```

## Usage

```sh
./sticker-maker \
  --layout layouts/combined.json \
  --field "title=Resistor 10 kOhm 1/4W" \
  --field "code=PART-00123" \
  --output label.png
```

| Option | Description |
|---|---|
| `--layout FILE` | JSON layout file. Required when rendering. |
| `--output FILE` | Output PNG file. Required when rendering. |
| `--field NAME=VALUE` | Named field value. May be repeated. |
| `--list-fonts` | List available font families, styles, and files, then exit. |
| `--help`, `-h` | Show command usage. |

A block's content is a static `text` value when `text` is set; otherwise the block takes the named field matching its `id`. A block cannot interpolate multiple fields.

Invalid layouts, missing fields, unavailable fonts, text overflow, unsupported Data Matrix data, out-of-bounds blocks, and — when `forbid_overlap` is set — overlapping blocks are reported on standard error.

## Layout format

Layouts are parsed strictly: unknown properties and unsupported format versions are rejected.

```json
{
  "version": 1,
  "image": { "width": 384, "height": 200 },
  "blocks": [
    {
      "id": "title",
      "type": "text",
      "x": 12,
      "y": 12,
      "width": 228,
      "height": 120,
      "font": {
        "family": "DejaVu Sans",
        "style": "Book",
        "size_px": 24
      },
      "wrap": "word_char",
      "align": "left",
      "valign": "top",
      "overflow": "error"
    },
    {
      "id": "code",
      "type": "datamatrix",
      "x": 246,
      "y": 12,
      "symbol": { "rows": 24, "columns": 24 },
      "module_px": 5,
      "quiet_zone_modules": 1
    }
  ]
}
```

### Top-level properties

| Property | Type | Description |
|---|---|---|
| `version` | integer | Layout format version. Must be `1`. |
| `image.width`, `image.height` | positive integer | Output dimensions in pixels. |
| `blocks` | array | Text and Data Matrix blocks. |
| `forbid_overlap` | boolean | Default: `false`. When `true`, overlapping blocks are rejected. |

### Common block properties

| Property | Required | Description |
|---|---|---|
| `id` | yes | Unique block identifier used in error messages. |
| `type` | yes | `text`, `datamatrix`, or `line`. |
| `x`, `y` | yes* | Non-negative position of the rotated block's top-left corner. Line blocks use `x1`/`y1`/`x2`/`y2` instead and do not support `rotation`. |
| `rotation` | no | Clockwise rotation: `0`, `90`, `180`, or `270`. Default: `0`. |
| `text` | no | Static content. When omitted, the block takes the named field matching its `id`. |

Coordinates start at the image's top-left corner, with X increasing to the right and Y increasing downward. A block occupies its final `width` x `height` rectangle at `(x, y)`, regardless of rotation: it is rendered first, rotated without interpolation, and then placed there. For a text block rotated by 90 or 270 degrees, the text is laid out in a transposed `height` x `width` box before rotation.

Blocks must remain inside the image. Overlapping blocks are allowed by default; set `forbid_overlap` to `true` to reject them. Edge contact is allowed. A Data Matrix block's quiet zone is part of its bounds. Line blocks are bounds-checked against their stroke but do not take part in the overlap check.

## Text blocks

| Property | Required | Default | Description |
|---|---|---|---|
| `width`, `height` | yes | — | Positive final block dimensions on the canvas (after rotation). |
| `font.family` | one of family/file | — | Exact system font family resolved through fontconfig. |
| `font.style` | no | — | Font style, such as `Book`, `Bold`, or `Oblique`. |
| `font.file` | one of family/file | — | Direct path to a TTF or OTF file. |
| `font.size_px` | yes | — | Positive font size in pixels. |
| `scale_x` | no | `1.0` | Positive horizontal scale factor. |
| `wrap` | no | `word_char` | `word`, `char`, `word_char`, or `none`. |
| `align` | no | `left` | `left`, `center`, or `right`. |
| `valign` | no | `top` | `top`, `center`, or `bottom`. |
| `overflow` | no | `error` | `error` or `clip`. |

`word_char` wraps at word boundaries and splits words that are too wide. Explicit newlines are preserved. `scale_x` affects measurement, wrapping, and alignment.

With `overflow: error`, rendering fails if the text does not fit. `clip` allows content outside the block to be cropped. Font size is never reduced automatically.

Fontconfig fallback is rejected when it does not match the requested family or style. Missing glyphs are also reported as errors. Use `font.file` when reproducible font selection is required.

List fonts recognized by the tool with:

```sh
./sticker-maker --list-fonts
```

## Data Matrix blocks

Data Matrix blocks use ECC 200, fixed symbol dimensions, integer module scaling, and no antialiasing. Content must be non-empty ASCII. GS1 DataMatrix and automatic symbol-size selection are not supported.

| Property | Required | Default | Description |
|---|---|---|---|
| `symbol.rows`, `symbol.columns` | yes | — | Symbol dimensions in modules, excluding the quiet zone. |
| `module_px` | yes | — | Positive number of pixels per module. |
| `quiet_zone_modules` | no | `1` | Quiet-zone width in modules on each side; must be at least `1`. |

Supported square sizes are:

```text
10, 12, 14, 16, 18, 20, 22, 24, 26, 32, 36, 40, 44, 48,
52, 64, 72, 80, 88, 96, 104, 120, 132, 144
```

Supported rectangular sizes (`rows × columns`) are:

```text
8×18, 8×32, 12×26, 12×36, 16×36, 16×48
```

The rendered size, including the quiet zone, is:

```text
width_px  = (columns + 2 * quiet_zone_modules) * module_px
height_px = (rows    + 2 * quiet_zone_modules) * module_px
```

Rendering fails if the content does not fit in the selected symbol.

## Line blocks

A line block draws a solid black line between two points, without antialiasing and without content. It does not support `rotation` and does not take part in the overlap check.

| Property | Required | Description |
|---|---|---|
| `x1`, `y1`, `x2`, `y2` | yes | Non-negative stroke endpoints in pixel coordinates. |
| `width` | yes | Positive stroke thickness in pixels. |

The stroke is centered on the segment and has flat caps: a pixel is drawn when its center is projected onto the segment and its perpendicular distance to the segment is within `(width-1)/2`. An odd `width` is symmetric about the line; an even `width` covers exactly `width` pixel rows (columns), skewed by one pixel to the right of the direction of travel (down for a horizontal line, left for a downward vertical line). A `width` of `1` is centered with a half-pixel band so diagonals stay connected. The direction of travel goes from the endpoint with the smaller x (then smaller y), so the result does not depend on the endpoint order.

The `layouts/` directory contains:

- `combined.json` — text and a square Data Matrix symbol
- `text.json` — centered and horizontally scaled text
- `datamatrix.json` — a square Data Matrix symbol
- `rotated.json` — rotated text and a rectangular Data Matrix symbol

## Tests

```sh
go test ./...
```

To check a generated Data Matrix image with an independent decoder:

```sh
nix-shell -c 'dmtxread label.png'
```

## Limitations

The tool does not provide direct printing, a GUI, physical units or DPI conversion, arbitrary-angle rotation, color output, automatic font fitting, multiple-field interpolation, or automatic font fallback.
