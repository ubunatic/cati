# 052 — Unified Composite Quality Metric and Efficiency Index

**Status**: Open  
**Priority**: P2 (Medium)  
**Severity**: Moderate  
**Category**: Feature  
**Related**: [`internal/metrics/metrics.go`](../internal/metrics/metrics.go), [`docs/ImageQualityMetrics.md`](../docs/ImageQualityMetrics.md), [`cmd/modes.go`](../cmd/modes.go), [`examples/imgbrowser/preview.go`](../examples/imgbrowser/preview.go)

---

## 1. Problem Description

1. **Luminance SSIM Limitations with Pixel Art**:
   - As documented in [`docs/ImageQualityMetrics.md`](../docs/ImageQualityMetrics.md#L44), pure luminance SSIM alone cannot distinguish between soft antialiased downsampling and sharp pixel-art contours (both can score ~0.70 despite `all`/`six` producing crisper edges).
   - We already have primitives for **Sobel Edge Continuity ($E_{\text{cont}}$)** and **Excess Blockiness ($B_{\text{score}}$)** in `internal/metrics`, but they are not combined into a single standardized quality score for mode ranking.

2. **Evaluating Quality vs Speed (Efficiency Index)**:
   - CLI users (`cati modes`) and interactive browsers need an objective way to find the Pareto-optimal rendering mode balancing visual quality against render execution time ($T_{\text{render}}$).

---

## 2. Proposed Changes & Grouped Work

1. **Composite Quality Metric ($Q_{\text{composite}}$)**:
   - Implement `metrics.CompositeQuality(ref, rend image.Image) float64`:
     $$Q_{\text{composite}} = 0.60 \cdot \text{SSIM} + 0.25 \cdot E_{\text{cont}} + 0.15 \cdot B_{\text{score}}$$
   - Provide helper to compute all three components in a single unified pass over shared luminance and gradient grids.

2. **Standardized Efficiency Index ($\text{Eff}$)**:
   - Formalize the Efficiency Index in `internal/metrics`:
     $$\text{Eff} = (1.0 - \text{Quality}) \times T_{\text{render}}$$
   - Add `--sort eff` support to `cati modes` and expose the efficiency metric in the image browser `#` info pane.

3. **Golden Regression Testing Integration**:
   - Update `cmd/golden_render_test.go` and `v1/sparkline/testhelper` to record $Q_{\text{composite}}$, $E_{\text{cont}}$, and $B_{\text{score}}$ in golden PNG `tEXt` chunks.

---

## 3. Success Criteria & Verification

- [ ] `metrics.CompositeQuality` implemented and covered by unit tests.
- [ ] `cati modes` supports `--sort eff` and displays composite quality scores.
- [ ] Info pane in `imgbrowser` displays $Q_{\text{composite}}$ and its component breakdown.
- [ ] Documentation updated in `docs/ImageQualityMetrics.md` and `docs/Glossary.md`.
- [ ] `make preflight && make test` passes cleanly.
