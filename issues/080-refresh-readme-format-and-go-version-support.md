# 080 — Refresh README format and Go version support

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Documentation
**Related**: [README](../README.md), [go.mod](../go.mod), [Image decoder support](../internal)

---

/goal Update the README's supported-format and Go-version guidance to match the current implementation, or stop and report if compatibility cannot be established from the code.

## 1. Problem & Motivation
The README says users need Go 1.21+ and lists PNG, JPEG, and SVG as supported formats, while `go.mod` requires Go 1.26.0 and `website/index.html` advertises GIF, WebP, and SVG. The README also describes adding raster decoders as future work, although decoder support changed in commit `ea51211`.

## 2. Technical Specification / Findings
The install instructions and format table are user-facing compatibility claims. Verify the decoder registrations and any external SVG dependency before revising them; keep the README and website consistent.

## 3. Implementation & Verification Plan
Update the README based on the module's declared Go version and actual decoder registrations. Confirm every listed format against the code and make the README, website, and install instructions agree.
