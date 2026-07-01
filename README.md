# cati

**`cat` for images** — renders PNG/JPEG files in the terminal using Unicode half-block characters and 24-bit ANSI true-color.

```
./cati photo.png
./cati --play --fps 15 frames/
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
make install      # installs to ~/go/bin
```

Requires Go 1.21+.

> **Planned:** `go install ubunatic.com/cati@latest` once the module path migration is complete (see `issues/vanity-module-path.md`).

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
cati --play frames/            # loop at 15 fps (default)
cati --play --fps 24 frames/   # custom frame rate
cati -p --fps 8 frame_*.png    # glob pattern, shell-expanded
```

Press **`q`**, **`ESC`**, or **`Ctrl+C`** to stop playback.

---

## Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--ansi` | | `true` | 24-bit ANSI true-color output |
| `--play` | `-p` | off | animate frames in a loop |
| `--fps` | | `15` | frames per second for `--play` |
| `--recursive` | `-r` | off | recurse into subdirectories |
| `--no-header` | | off | suppress filename headers |

---

## Development

```bash
make build    # compile ./cati
make test     # go vet + go test
make demo     # play the bundled emojig animation
make install  # go install to ~/go/bin
make tidy     # go mod tidy
make clean    # remove ./cati binary
```

---

## Supported formats

| Format | Extension |
|--------|-----------|
| PNG    | `.png`    |
| JPEG   | `.jpg`, `.jpeg` |

Support for more formats (GIF, WebP, …) can be added by importing the relevant `image/*` decoder package.

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
