# w3-alloc

## 1. Eliminate `image.At` boxing allocations in halfblock fallback rendering
**Severity:** High
**Effort:** M
**Why:** In `v1/halfblock/render.go` at `func (p *RenderParams) scaleImage(...)` and other pixel loops, the reference rendering uses `img.At(srcX, srcY)` which triggers interface boxing on `color.Color`, creating thousands of heap allocations per frame.
**Smallest next step:** Implement typed type-switches (for `*image.RGBA`, `*image.NRGBA`, `*image.YCbCr`) in `v1/halfblock/render.go` at lines `218` and `278` to retrieve pixels from the underlying slice arrays directly.
**Evidence:**
- `v1/halfblock/render.go:218`: `				dst.Set(x, y, img.At(srcX, srcY))`
- `v1/halfblock/render.go:278`: `				dst.Set(x, y, img.At(srcX, srcY))`

## 2. Eliminate `image.At` boxing allocations in sparkline rendering
**Severity:** High
**Effort:** M
**Why:** Similar to halfblock, the sparkline renderer uses `img.At` across its grid layout in loops. Every call dynamically dispatches via the `image.Image` interface, allocating heap memory.
**Smallest next step:** Add typed fast-paths similar to how other packages check for `*image.NRGBA` or `*image.RGBA` to read raw bytes.
**Evidence:**
- `v1/sparkline/render.go:109`: `				dst.Set(x, y, img.At(srcX, srcY))`
- `v1/sparkline/render.go:1031`: `				p = append(p, toRGBA(img.At(bx, by)))`

## 3. Replace `strings.Builder` with zero-allocation `[]byte` appends in halfblock ANSI formatting
**Severity:** Medium
**Effort:** M
**Why:** While performance tuning in other paths (like sextant) moved to direct `[]byte` appending (`strconv.AppendUint`), the halfblock ANSI fallback formatting still uses `strings.Builder` and `fmt.Sprintf` style logic, causing significant GC pressure per frame string construction.
**Smallest next step:** Refactor the halfblock render loop to build lines into a pre-sized reusable `[]byte` buffer, appending ANSI sequences with `strconv.AppendInt`.
**Evidence:**
- `v1/halfblock/render.go:512`: `	var b strings.Builder`

## 4. Cache duplicate spec loads in `spec/load.go`
**Severity:** Medium
**Effort:** S
**Why:** Each call to load specs (e.g. `LoadStyle`, `LoadControls`) currently reads the file from `fs.FS` and parses it via `yaml.Unmarshal`, which incurs I/O and allocation overhead every time it is invoked.
**Smallest next step:** Introduce a package-level cache, such as `sync.OnceValues`, to parse YAML configuration schemas exactly once across the application lifecycle.
**Evidence:**
- `spec/load.go:123`: `	err = yaml.Unmarshal(data, &spec)`
- `spec/load.go:885`: `	data, err := fs.ReadFile(fsys, "controls.yaml")`

## 5. Expand type-switching to avoid `img.At` in quadblock fallback
**Severity:** Low
**Effort:** S
**Why:** The fast-path in `quadblock` checks for `*image.NRGBA`, but other common types like `*image.RGBA` or `*image.YCbCr` will fall through to `img.At()`, generating heap allocations.
**Smallest next step:** Add `*image.RGBA` block to the type assertion logic in `toRGBA`.
**Evidence:**
- `v1/quadblock/render.go:806`: `		return toRGBA(img.At(x, y))`
