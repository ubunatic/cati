import sys

def main():
    report = """## Verdict
still valid. The issue states that ffprobe is called for all images, causing performance issues, and `cmd/media_meta.go` still unconditionally calls `halfblock.ProbeMediaMeta` for all files.

## Evidence
- "`cmd/media_meta.go:loadMediaMeta` calls `halfblock.ProbeMediaMeta(path)` for **every** file type" - True. `cmd/media_meta.go:126`: `pr := halfblock.ProbeMediaMeta(path)`
- "Implement a `shouldProbeRich(ext string) bool` helper in `media_meta.go`." - Missing. A search for `shouldProbeRich` using `grep -ri "shouldproberich" .` returned no results in the Go code. It would go near `cmd/media_meta.go:183`.

## Smallest next step
Implement `shouldProbeRich(ext string) bool` in `cmd/media_meta.go` to return true for "jpeg", "jpg", "tiff", "heic". Update `loadMediaMeta` to only call `halfblock.ProbeMediaMeta(path)` if `isVideo` is true or `shouldProbeRich(m.Ext)` is true.

## Ticket corrections
none
"""
    with open("reports/jules/007.md", "w") as f:
        f.write(report)

if __name__ == "__main__":
    main()
