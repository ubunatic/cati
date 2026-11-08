## Verdict

I reviewed the codebase for error handling issues (swallowed errors, panics, missing `%w`, ignored `Close()`). I found instances of ignored `Close()` errors, panics used for flow control, and swallowed process termination errors.

## Proposals

### 1. Ignored `Close()` Errors

**Location:** `cmd/input_tester.go:42`
```go
			_ = logFile.Close()
```

**Location:** `cmd/media_meta.go:131`
```go
			_ = f.Close()
```

**Severity:** Medium, **Effort:** S
**Change:** Check the error returned by `Close()` instead of ignoring it with `_ =`.
**Why it helps:** Ignoring `Close()` errors can lead to silent data loss (if the final write/flush fails) or resource leaks. Handling the error ensures any failure to flush buffers to disk is caught.
**Smallest next step:** Replace `_ = logFile.Close()` and `_ = f.Close()` with error logging or returning the error appropriately.

### 2. Panics in Library Code (`cmd` wrappers)

**Location:** `cmd/render_pipeline.go:53`
```go
	img, err := prepareRenderedImageChecked(orig, state, termCols, termRows, rc, initialZoom)
	if err != nil {
		panic(err)
	}
```

**Severity:** Low, **Effort:** M
**Change:** Remove the panic wrapper and propagate the error up the call stack, or handle it gracefully where called.
**Why it helps:** Using `panic` for standard error propagation in Go is an anti-pattern. It can crash the application unexpectedly and makes the code harder to test and use as a library. Returning the error allows callers to decide how to handle failures.
**Smallest next step:** Refactor callers of `prepareRenderedImage` to use the `prepareRenderedImageChecked` variant directly and handle the error, then remove the panicking wrapper.

### 3. Panics in Tests

**Location:** `v1/halfblock/image_async_test.go:91`
```go
	legacy := testing.AllocsPerRun(5, func() {
		if _, err := LoadImageWithTarget(path, 0, 0); err != nil {
			panic(err)
		}
	})
```

**Severity:** Low, **Effort:** S
**Change:** Replace `panic(err)` with proper test failure mechanisms.
**Why it helps:** Using `panic` inside tests (especially within helper closures like `testing.AllocsPerRun`) can cause the test suite to abort abruptly or provide poor stack traces. Proper error handling provides better diagnostics.
**Smallest next step:** Since it's inside `testing.AllocsPerRun`, `t.Fatal` might not be safe to call directly from the goroutine if it spawns one. If it doesn't, use `t.Fatal(err)`. Alternatively, use a boolean flag and `t.Fatalf` outside the loop.

### 4. Swallowed Errors during Process Termination

**Location:** `internal/audio/audio.go:133`
```go
	_ = p.cmd.Process.Kill()
```

**Location:** `v1/halfblock/video.go:269`
```go
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
```

**Severity:** Low, **Effort:** S
**Change:** Check and log the errors returned by `cmd.Process.Kill()` and `cmd.Wait()` instead of ignoring them.
**Why it helps:** Swallowing these errors can hide underlying issues with process management, such as zombie processes or permission problems when trying to terminate subprocesses like `ffmpeg` or audio players.
**Smallest next step:** Introduce error logging (e.g., to standard error or a debug output) for the `Kill()` and `Wait()` calls.
