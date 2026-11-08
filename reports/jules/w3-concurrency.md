w3-concurrency

## Verdict
Concurrency gaps identified regarding ffmpeg process lifecycle, lack of context cancellation in terminal input loops, and uncancelled worker queues.

## Evidence

### 1. Audio `ffplay` process lifecycle block
- **Path:Line**: `internal/audio/audio.go:133`
- **Quote**: `_ = p.cmd.Process.Kill()`
- **Severity**: Medium
- **Effort**: S
- **Why it helps**: `Stop()` waits indefinitely on `<-p.done` (line 134). If `ffplay` resists `Kill()` or there's a zombie process, `cmd.Wait()` in the goroutine won't return, causing `Stop()` to hang. Using a timeout or a non-blocking select prevents the consumer from freezing on media transition.
- **Change**: Introduce a context or timeout when waiting on `p.done` during `Stop()`, or do not block on it after killing the process.
- **Smallest next step**: Modify `Player.Stop()` to use a `select` with a timeout when waiting on `p.done`.

### 2. Browser terminal input loop lacks cancellation
- **Path:Line**: `cmd/browser.go:261`
- **Quote**: `go func() {` (with `for { n, err := os.Stdin.Read(buf)` at line 263-264)
- **Severity**: Low
- **Effort**: M
- **Why it helps**: The goroutine blocks in `os.Stdin.Read` and loops forever until an error. It cannot be cleanly cancelled, potentially leaking if the viewer exits or if used as a library. Cancelling allows clean testing and shutdown.
- **Change**: Pass the main context to the input loop and use `os.Stdin.SetReadDeadline` inside the loop to wake up and check context cancellation.
- **Smallest next step**: In `browser()`, add a context to the input goroutine and a ticker or deadline for `Stdin.Read` to poll `ctx.Done()`.

### 3. Interactive viewer input loop lacks cancellation
- **Path:Line**: `cmd/interactive.go:80`
- **Quote**: `go func() {` (with `for { n, err := os.Stdin.Read(buf)` at line 82-83)
- **Severity**: Low
- **Effort**: M
- **Why it helps**: Similar to the browser, the interactive viewer's `os.Stdin` reading goroutine has no way to exit cleanly if the parent returns without an EOF. Clean shutdown prevents leaks in integrations and tests.
- **Change**: Implement a deadline-based read loop checking a context or `done` channel.
- **Smallest next step**: Refactor the shared input reader in `interactiveWithChan` to accept a `context.Context` and poll it using short read deadlines.

### 4. Worker context cancellation is not checked when dequeuing
- **Path:Line**: `cmd/thumbqueue.go:119`
- **Quote**: `job, ok := q.next()`
- **Severity**: Low
- **Effort**: S
- **Why it helps**: `thumbWorker` calls `q.next()` which blocks on a condition variable. While `q.stop()` broadcasts, a worker pulling jobs doesn't check `ctx.Done()` (from `workerCtx`), relying purely on the defer `q.stop()`. Explicitly checking context ensures faster unblocking on cancellation.
- **Change**: Make `q.next()` accept a context, or add a `select` in the worker to prioritize `ctx.Done()`.
- **Smallest next step**: Add `if ctx.Err() != nil { return }` inside the `thumbWorker` loop, and wake waiters when context cancels.

## Ticket corrections
none
