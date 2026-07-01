# cati

**`cat` for images** — renders PNG/JPEG files in the terminal using Unicode half-block characters and 24-bit ANSI true-color.

```
./cati photo.png
./cati play --fps 15 frames/
./cati browse ~/Pictures/
```

![demo](docs/demo.gif)

---

## How it works

Each terminal cell encodes **two vertical pixel rows** using Unicode block characters:

| Char | Top half | Bottom half |
|------|----------|-------------|
| `▀`  | fg color | bg color    |
| `▄`  | bg color | fg color    |
| `█`  | fg color | fg color    |
| ` `  | —        | —           |

Combined with 24-bit ANSI true-color (`\x1b[38;2;R;G;Bm`) this gives effective resolution of **terminal-width × (2 × terminal-height)** pixels.

---

## Install

```bash
# From source
git clone https://codeberg.org/ubunatic/cati
cd cati
make install      # installs cati, catiplay, and catibrowse to ~/go/bin
```

Or install with Go 1.21+.

```bash
go install ubunatic.com/cati/cmd/cati@latest
go install ubunatic.com/cati/cmd/catiplay@latest
go install ubunatic.com/cati/cmd/catibrowse@latest
```

---

## Usage

### Show an image

```bash
cati image.png
cati photo.jpg
```

### Show a folder

```bash
cati ~/Pictures/           # all PNGs/JPEGs, sorted
cati -r ~/Pictures/        # recurse into subdirectories
cati --no-header dir/      # suppress ==> file <== headers
```

### Animate frames

```bash
cati play frames/            # loop at 15 fps (default)
cati play --fps 24 frames/   # custom frame rate
cati play --fps 8 frame_*.png # glob pattern, shell-expanded
```

Press **`q`**, **`ESC`**, or **`Ctrl+C`** to stop playback.

### Interactive media and browser

```bash
catiplay image.png           # interactive single-image viewer
catiplay video.mp4           # interactive video viewer
catibrowse ~/Pictures/       # file browser with previews
cati browse ~/Pictures/      # forwards to catibrowse
```

Legacy `cati --play`/`cati -p` and `cati -i` still work as forwarding
compatibility aliases when the companion binaries are installed.

---

## Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--ansi` | | `true` | 24-bit ANSI true-color output |
| `--recursive` | `-r` | off | recurse into subdirectories |
| `--no-header` | | off | suppress filename headers |

---

## Development

```bash
make build    # compile ./cati, ./catiplay, ./catibrowse
make test     # go vet + default tests
make test-all # default + player/browser/integration tests
make demo     # play the bundled emojig animation
make install  # go install to ~/go/bin
make tidy     # go mod tidy
make clean    # remove built binaries
```

---

## Supported formats

| Format | Extension |
|--------|-----------|
| PNG    | `.png`    |
| JPEG   | `.jpg`, `.jpeg` |
| SVG    | `.svg`    |

Support for more raster formats (GIF, WebP, …) can be added by importing the
relevant `image/*` decoder package. SVG is rasterized via `rsvg-convert`
(librsvg), which must be installed and on `$PATH`.

---

## Roadmap

- [ ] Quad-block mode (`--mode=quad`) for 2× horizontal pixel resolution
- [ ] Kitty graphics protocol (`--kitty`)
- [ ] Sixel output (`--sixel`)
- [ ] Per-frame delay from manifest (non-uniform animation)
- [ ] `--loop N` to play N times instead of forever

---

## License

AGPL-3.0-or-later
