# Performance Optimization: Reusable Byte Buffer ANSI Formatting in Sparkline Renderer

## Overview
In high-throughput terminal image rendering using the `sparkline` package, formatting ANSI true-color escape sequences (`\x1b[38;2;r;g;bm` and `\x1b[48;2;r;g;bm`) using `fmt.Sprintf` and accumulating output strings with `strings.Builder` created excessive heap allocations (~2,261 allocs/op) per frame.

By introducing direct byte-slice ANSI code formatting (`appendFgRGB` and `appendBgRGB` via `strconv.AppendUint`) and writing lines directly as `[]byte` via `utf8.AppendRune` into a line buffer in `sparkline.Render`, heap object allocations are reduced by 98.6% (from 2,261 to 32 allocs/op), and total allocated memory per frame drops from 245 KB to 25.8 KB (~89.5% reduction), cutting rendering latency from ~1.92ms to ~1.08ms (~43.8% speedup).

## Hardware Target
- **Topology & Limits**: Multi-core x86_64, worker pool / benchmark parallelism capped at 10 cores (`min(runtime.NumCPU(), 10)`).
- **Execution Profile**: Compute & string-heavy terminal rendering loop.

## Topology / Data Flow

```
[Scaled Image Grid]
        │
        ▼
[Cell Analysis / Selection]
        │
        ▼
[Reusable Line []byte Buffer] ───► [appendBgRGB / appendFgRGB via strconv]
        │
        ▼
[utf8.AppendRune + ansiReset]
        │
        ▼
[Stdout / Writer Write(buf)] (Single write per row)
```

## Benchmark Evidence

### `go test -bench=BenchmarkRenderSerial ./v1/sparkline -benchmem -cpu=1,4,10`

| Metric | Before Optimization | After Optimization | Improvement |
| :--- | :--- | :--- | :--- |
| **ns/op (cpu=1)** | 1,928,722 ns/op | 1,082,437 ns/op | **~43.8% faster** |
| **B/op** | 244,820 B/op | 25,832 B/op | **~89.5% reduction** |
| **allocs/op** | 2,261 allocs/op | 32 allocs/op | **~98.6% reduction** |

## Verification
- Unit test suite: `go test -v ./v1/sparkline/...`
- Full test suite: `go test -v ./v1/...`
- Comparative benchmarks: `go test -bench=BenchmarkRenderSerial -benchmem -cpu=1,4,10 -count=5 ./v1/sparkline`
