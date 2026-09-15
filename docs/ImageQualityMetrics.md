---
title: Image Quality & Error Metrics
weight: 62
---

# Image Quality & Error Metrics — Photographic & Pixel Art Assessment

This document provides a comprehensive reference on image quality assessment (IQA) and error metrics for both continuous-tone photographic images and low-resolution, discretized terminal graphics and pixel art. It establishes mathematical formulations, perceptual foundations, computational complexity profiles, and architectural guidelines for choosing and implementing metrics in Cati.

---

## 1. Executive Summary & Problem Space

Evaluating image fidelity in terminal graphics presents unique challenges that distinguish it from traditional photographic compression benchmarks. Terminal graphics operate under strict hardware and protocol constraints:
* **Spatial Discretization**: Images are mapped to a coarse grid of character cells (e.g., $80 \times 24$ or $120 \times 40$).
* **Subpixel Block Packing**: Each cell is partitioned into a small subpixel matrix ($1 \times 2$ half-blocks, $2 \times 2$ quadrants, $2 \times 3$ sextants, or $1 \times 8$ sparkline fractions).
* **Two-Color Cell Budget**: ANSI escape sequences restrict each character cell to at most two simultaneous 24-bit RGB colors (one foreground and one background).
* **Anisotropic Geometry**: Standard monospace terminal fonts have an aspect ratio of roughly $1:2$ (width is half of height), requiring horizontal aspect-ratio compensation.

Because of these constraints, a metric that excels for continuous-tone photography (such as PSNR or raw MSE) often fails when evaluating terminal pixel art—for example, penalizing high-frequency dither patterns that human vision blends into smooth gradients, or rewarding blurry downscaling over crisp edge preservation.

```text
┌────────────────────────────────────────────────────────────────────────┐
│                        Original Source Image                           │
└──────────────────────────────────┬─────────────────────────────────────┘
                                   │
                    Downsampling & Quantization
                                   │
                                   ▼
┌────────────────────────────────────────────────────────────────────────┐
│                   Terminal Cell & Subpixel Solver                      │
│      (2-color partition, glyph mask codebook matching, aspect fix)      │
└──────────────────────────────────┬─────────────────────────────────────┘
                                   │
                     Terminal Reconstruction Grid
                                   │
                                   ▼
┌────────────────────────────────────────────────────────────────────────┐
│                        Quality Evaluation                              │
│  ├── End-to-End Metric (E_src, SSIM): Output vs Original Source        │
│  └── Fitting Metric    (E_fit):       Output vs Intermediate Grid      │
└────────────────────────────────────────────────────────────────────────┘
```

---

## 2. Foundations of Image Quality Assessment

### 2.1 Full-Reference vs. No-Reference IQA

* **Full-Reference (FR-IQA)**: Compares a distorted or reconstructed image against a pristine ground-truth reference image. All core metrics evaluated in Cati (SSIM, PSNR, MSE, CIEDE2000, Sobel continuity) operate in the FR-IQA paradigm.
* **Reduced-Reference (RR-IQA)**: Uses extracted features (e.g., edge histograms, wavelet coefficients) rather than the complete image.
* **No-Reference / Blind (NR-IQA)**: Evaluates natural scene statistics or neural representations without any reference image (e.g., BRISQUE, NIQE). Useful for arbitrary user input grading, but not for compiler/solver fidelity verification.

### 2.2 The Two-Stage Terminal Information Loss Model

As defined in [`Glossary.md`](Glossary.md), terminal rendering loss occurs across two distinct boundaries:

1. **Downsampling & Geometric Loss**: Transition from source resolution $(W_{\text{src}} \times H_{\text{src}})$ to the terminal cell grid $(W_{\text{cell}} \times H_{\text{cell}})$. High-frequency details beyond the Nyquist limit of the terminal grid are eliminated.
2. **Quantization & Codebook Discretization Loss**: Transition from continuous downscaled color patches to a discrete Unicode glyph mask and a foreground/background color pair $(\mathbf{c}_{\text{fg}}, \mathbf{c}_{\text{bg}})$.

$$\text{Total Fidelity Loss} = \text{Downsampling Error} + \text{Quantization Error}$$

* **End-to-End Reconstruction Error ($E_{\text{src}}$)**: Measures the visual difference between the high-resolution reconstructed terminal raster and the original source image $I_{\text{src}}$.
* **Representation / Fitting Error ($E_{\text{fit}}$)**: Measures how well the glyph codebook fits the intermediate downscaled bitmap $I_{\text{down}}$.

---

## 3. Continuous-Tone & Photographic IQA Metrics

### 3.1 Mean Squared Error (MSE) & Sum of Squared Errors (SSE)

MSE and SSE measure the arithmetic $L_2$ Euclidean distance across color channels.

$$\text{MSE}(x, y) = \frac{1}{N \cdot C} \sum_{i=1}^{N} \sum_{c=1}^{C} \big(x_{i,c} - y_{i,c}\big)^2, \qquad \text{SSE}(x, y) = \sum_{i=1}^{N} \sum_{c=1}^{C} \big(x_{i,c} - y_{i,c}\big)^2$$

* **Mathematical Properties**: Convex, strictly additive, differentiable, $O(N)$ computational complexity.
* **Strengths**: Extremely fast; optimal loss function for linear regression and k-means color quantization; zero branching.
* **Limitations**: Poor correlation with human visual perception. Treats a 1-pixel spatial shift identically to an extreme uniform blur; sensitive to global luminance offsets that the human visual system (HVS) adapts to.

### 3.2 Peak Signal-to-Noise Ratio (PSNR)

PSNR expresses MSE logarithmically relative to the maximum possible signal power ($MAX_I = 255$ for 8-bit channels):

$$\text{PSNR} = 10 \cdot \log_{10} \left( \frac{MAX_I^2}{\text{MSE}} \right) = 20 \cdot \log_{10}(MAX_I) - 10 \cdot \log_{10}(\text{MSE})$$

* **Scale**: Decibels ($\text{dB}$). Identical images yield $+\infty$; typical lossy compression ranges from $25\,\text{dB}$ (low) to $45\,\text{dB}$ (high).
* **Strengths**: Standard benchmark in legacy video/image compression codecs; easy to compute.
* **Limitations**: Inherits all weaknesses of MSE. High-contrast edge distortions and subtle low-frequency color shifts produce disproportionate decibel swings that do not match human subjective ratings.

### 3.3 Structural Similarity Index (SSIM) & Multi-Scale SSIM (MS-SSIM)

Proposed by Wang et al. (2004), SSIM models visual perception by decomposing similarity into three orthogonal components: luminance $l(x,y)$, contrast $c(x,y)$, and structure $s(x,y)$.

$$\text{SSIM}(x, y) = \left[ l(x,y) \right]^\alpha \cdot \left[ c(x,y) \right]^\beta \cdot \left[ s(x,y) \right]^\gamma$$

With $\alpha = \beta = \gamma = 1$, computed over local spatial windows (typically $8 \times 8$ or Gaussian $\sigma=1.5$):

$$\text{SSIM}(x, y) = \frac{(2\mu_x\mu_y + C_1)(2\sigma_{xy} + C_2)}{(\mu_x^2 + \mu_y^2 + C_1)(\sigma_x^2 + \sigma_y^2 + C_2)}$$

where $\mu_x, \mu_y$ are local sample means, $\sigma_x^2, \sigma_y^2$ are sample variances, $\sigma_{xy}$ is sample covariance, and $C_1 = (K_1 L)^2, C_2 = (K_2 L)^2$ (with $K_1=0.01, K_2=0.03, L=255$ or $1.0$) prevent division by zero in flat regions.

* **Multi-Scale SSIM (MS-SSIM)**: Evaluates luminance at the coarsest scale $M$ and contrast/structure across $M$ iteratively downsampled scales, accounting for varying viewing distances and display DPI:
  $$\text{MS-SSIM}(x, y) = \left[ l_M(x,y) \right]^{\alpha_M} \cdot \prod_{j=1}^{M} \left[ c_j(x,y) \right]^{\beta_j} \left[ s_j(x,y) \right]^{\gamma_j}$$
* **Strengths**: Strong correlation with human structural perception; penalizes structural blur and contour corruption far more than uniform brightness shifts.
* **Limitations**: Standard SSIM on non-overlapping $8 \times 8$ blocks can suffer from window boundary discontinuities; sensitive to subpixel translation unless computed with sliding Gaussian windows or Complex Wavelets (CW-SSIM).

### 3.4 Complex Wavelet SSIM (CW-SSIM)

CW-SSIM performs structural similarity assessment in the complex wavelet transform domain (using steerable pyramids or dual-tree complex wavelets).
* **Mechanism**: Compares the local phase and magnitude of complex wavelet coefficients.
* **Advantage**: Invariant to small spatial translations, rotations, and minor geometric scalings that destroy standard pixel-space MSE and SSIM scores without diminishing subjective visual quality.
* **Complexity**: High computation overhead due to multi-scale multi-orientation wavelet decomposition.

### 3.5 Feature Similarity Index (FSIM / FSIMc)

Zhang et al. (2011) observed that the HVS perceives images primarily through low-level salient features:
1. **Phase Congruency (PC)**: A dimensionless measure of feature significance invariant to contrast, identifying salient edges, lines, and corners.
2. **Gradient Magnitude (GM)**: Captures luminance contrast intensity via Scharr or Sobel operators.

$$\text{FSIM} = \frac{\sum_{x} S_L(x) \cdot PC_m(x)}{\sum_{x} PC_m(x)}, \qquad S_L(x) = S_{PC}(x) \cdot S_G(x)$$

* **FSIMc**: Extends FSIM to color by evaluating chromatic channels in $I_1 I_2 I_3$ or $YIQ$ space.
* **Strengths**: Exceptionally high correlation with human Mean Opinion Scores (MOS) on continuous photographic databases (TID2008, LIVE).
* **Limitations**: Expensive 2D Log-Gabor filtering steps make it unsuitable for real-time per-frame rendering loops.

### 3.6 Visual Information Fidelity (VIF)

Sheikh and Bovik (2006) framed image quality assessment in an information-theoretic framework using natural scene statistics (NSS) in the wavelet domain:
* Quantifies the Shannon mutual information between the source reference and the brain's internal neural representation, modeled through a Gaussian Scale Mixture (GSM) and human visual neural noise.
* **Score**: $\text{VIF} = \frac{\text{Information Extracted from Distorted}}{\text{Information Extracted from Reference}}$. A score of $1.0$ represents identical fidelity; $>1.0$ can occur if contrast enhancement adds perceptual information.
* **Use Case**: Deep analysis of transmission channels and codec artifacts; computationally heavy.

### 3.7 Haar Wavelet-Based Perceptual Similarity Index (HaarPSI)

Reisenhofer et al. (2018) developed HaarPSI to provide high perceptual correlation while drastically reducing the computation cost of wavelet/feature metrics:
* Uses 2D discrete Haar wavelet filters at 2 or 3 scales to extract horizontal and vertical high-frequency responses.
* Combines local similarity maps weighted by high-frequency energy.
* **Performance**: Computes $\approx 5\text{--}10\times$ faster than FSIM and SSIM while achieving comparable or superior Pearson correlation to subjective human scores.

### 3.8 Psychovisual Color Metrics: Butteraugli & SSIMULACRA 2

Developed for next-generation image codecs (JPEG XL, WebP 2), these metrics abandon RGB/YUV in favor of physiologically validated color spaces:

* **XYB Color Space**: Models human retinal cones (L, M, S) and opponent channels:
  * $X$: Red-green opponent signal.
  * $Y$: Luminance (sum of L and M cones with non-linear response).
  * $B$: Blue-yellow opponent signal.
* **Butteraugli**: Estimates the psychovisual Just-Noticeable-Difference (JND) threshold. Designed to guide encoder optimization loops by identifying where bitrate can be cut without human detection.
* **SSIMULACRA 2**: Evaluation-time verifier combining multi-scale structural metrics, asymmetric edge sharpness weighting, and XYB color difference pooling. Scores range from $-\infty$ to $100$ ($>90$ is visually lossless).
* **Applicability**: Gold standard for high-end lossy compression benchmarking; too heavy for real-time terminal CLI rendering ($>50\text{ms}$ per frame).

### 3.9 Learned Perceptual Image Patch Similarity (LPIPS)

Zhang et al. (2018) established LPIPS by extracting feature activations from deep convolutional networks (VGG-16, AlexNet, SqueezeNet) pre-trained on ImageNet:

$$d(x, x_0) = \sum_{l} \frac{1}{H_l W_l} \sum_{h,w} \left\| w_l \odot \big( \hat{y}^l_{hw} - \hat{y}_{0,hw}^l \big) \right\|_2^2$$

* **Strengths**: Captures deep semantic and texture similarity; highly resilient to minor pixel jitter; aligns closely with human judgements on generative AI (GANs, diffusion models).
* **Limitations**: Requires GPU acceleration or heavy ONNX/PyTorch runtimes ($\approx 50\text{--}200\text{ms}$ per image); non-interpretable feature weights; poor fit for lightweight CLI utilities.

---

## 4. Low-Resolution, Pixel Art, & Terminal Graphics Metrics

Low-resolution terminal rendering imposes discrete block boundaries and strict 2-color palettes. Standard continuous-tone metrics produce severe pathologies when applied naively. Specialized metrics are required to assess edge fidelity, blockiness, and palette quantization.

```text
┌────────────────────────────────────────────────────────────────────────┐
│               Terminal Cell 2-Color Partition Problem                  │
│                                                                        │
│   Source Subpixel Bitmap (e.g. 2x3)        Unicode Character Choice    │
│           ┌───┬───┐                              ┌───┬───┐             │
│           │ 1 │ 0 │                              │ █ │   │  Foreground │
│           ├───┼───┤                              ├───┼───┤  Color (C_fg)│
│           │ 1 │ 1 │   ─── Codebook Match ───>    │ █ │ █ │             │
│           ├───┼───┤                              ├───┼───┤  Background │
│           │ 0 │ 1 │                              │   │ █ │  Color (C_bg)│
│           └───┴───┘                              └───┴───┘             │
│                                                                        │
│   Matching Distance: D(Cell) = || Bitmask - Target ||_H + DeltaE(Colors)│
└────────────────────────────────────────────────────────────────────────┘
```

### 4.1 Subpixel Bitmask Matching & Hamming Distance

In binary subpixel block modes (half-block $1\times 2$, quadrant $2\times 2$, sextant $2\times 3$, braille $2\times 4$), each terminal cell represents a binary bitmask of size $K = W_{\text{sub}} \times H_{\text{sub}}$:

$$D_{\text{Hamming}}(\mathbf{b}_{\text{target}}, \mathbf{b}_{\text{glyph}}) = \sum_{k=0}^{K-1} \big( b_{\text{target}}[k] \oplus b_{\text{glyph}}[k] \big) = \text{popcount}(\mathbf{b}_{\text{target}} \oplus \mathbf{b}_{\text{glyph}})$$

* **Optimization**: Bitwise `XOR` and CPU `POPCNT` instructions evaluate candidate glyphs in $<1\text{ nanosecond}$.
* **Lookup Table (LUT) Acceleration**: For 6-bit sextants ($2^6 = 64$ states) and 4-bit quadrants ($2^4 = 16$ states), exact matching is performed via an $O(1)$ direct array lookup, reducing fitting error $E_{\text{fit}}$ to zero for fully covered glyph sets.

### 4.2 Perceptual Color Spaces & Color Difference ($\Delta E$)

When quantizing continuous colors to terminal ANSI palettes or 2-color cell foreground/background pairs, Euclidean distance in non-linear sRGB creates major color shifts (e.g., greens appear disproportionately brighter than blues):

```text
       Euclidean RGB:  ΔE_RGB  = sqrt((ΔR)² + (ΔG)² + (ΔB)²)           [Poor perceptual uniformity]
       CIELAB 1976:    ΔE*76   = sqrt((ΔL*)² + (Δa*)² + (Δb*)²)        [Simple, overestimates blue/yellow]
       CIEDE2000:      ΔE00    = Complex formula with chroma/hue rotation [Industry gold standard]
       Oklab:          ΔE_ok   = sqrt((ΔL)² + (Δa)² + (Δb)²)           [Fast, excellent uniformity]
```

#### 4.2.1 CIELAB & CIEDE2000 ($\Delta E_{00}$)
* **CIELAB ($\Delta E^*_{76}$)**: Evaluates distance in lightness ($L^*$), green-red ($a^*$), and blue-yellow ($b^*$). Fast, but non-uniform in high-chroma regions.
* **CIEDE2000 ($\Delta E_{00}$)**: Incorporates chroma-dependent weighting $S_C$, hue-dependent weighting $S_H$, lightness weighting $S_L$, and an interactive rotation term $R_T$ for the blue region ($\approx 275^\circ$):

$$\Delta E_{00} = \sqrt{ \left(\frac{\Delta L'}{k_L S_L}\right)^2 + \left(\frac{\Delta C'}{k_C S_C}\right)^2 + \left(\frac{\Delta H'}{k_H S_H}\right)^2 + R_T \left(\frac{\Delta C'}{k_C S_C}\right) \left(\frac{\Delta H'}{k_H S_H}\right) }$$

* **Thresholds**: $\Delta E_{00} \approx 1.0$ is a Just-Noticeable Difference (JND). Values $< 2.0$ are indistinguishable under normal conditions; values $> 5.0$ represent noticeable color divergence.

#### 4.2.2 Oklab & $\Delta E_{\text{ok}}$
Developed by Björn Ottosson (2020), Oklab uses a 3-stage transformation: $\text{sRGB} \to \text{Linear LMS} \to \text{Non-linear } L^{1/3} M^{1/3} S^{1/3} \to \text{Oklab }(L, a, b)$.
* **Advantage**: Achieves perceptual uniformity comparable to CIEDE2000 using simple Euclidean distance $\Delta E_{\text{ok}} = \sqrt{\Delta L^2 + \Delta a^2 + \Delta b^2}$.
* **Suitability**: Ideal balance of precision and speed for real-time terminal color quantization.

### 4.3 Spatial-Color Energy Optimization (2-Color-per-Cell Partition)

In modes like `spark` (Set 86/88), `quad` (Set 4), and `six` (Set 6), the solver partitions the $W_c \times H_c$ subpixel block into two disjoint sets: $\Omega_{\text{fg}}$ (foreground mask) and $\Omega_{\text{bg}}$ (background mask).

The Total Squared Error for a candidate partition is:

$$E(\Omega_{\text{fg}}, \Omega_{\text{bg}}) = \sum_{p \in \Omega_{\text{fg}}} \|\mathbf{I}(p) - \mathbf{c}_{\text{fg}}\|^2 + \sum_{q \in \Omega_{\text{bg}}} \|\mathbf{I}(q) - \mathbf{c}_{\text{bg}}\|^2$$

Optimal colors $\mathbf{c}_{\text{fg}}^*$ and $\mathbf{c}_{\text{bg}}^*$ are the centroid averages:

$$\mathbf{c}_{\text{fg}}^* = \frac{1}{|\Omega_{\text{fg}}|} \sum_{p \in \Omega_{\text{fg}}} \mathbf{I}(p), \qquad \mathbf{c}_{\text{bg}}^* = \frac{1}{|\Omega_{\text{bg}}|} \sum_{q \in \Omega_{\text{bg}}} \mathbf{I}(q)$$

Substituting centroids into the error equation yields the optimal split criterion:

$$E_{\text{total}} = \text{Var}(\text{Cell}) - \left( |\Omega_{\text{fg}}| \cdot \|\mathbf{c}_{\text{fg}}^* - \mathbf{\mu}_{\text{cell}}\|^2 + |\Omega_{\text{bg}}| \cdot \|\mathbf{c}_{\text{bg}}^* - \mathbf{\mu}_{\text{cell}}\|^2 \right)$$

Minimizing reconstruction error is mathematically equivalent to **maximizing between-cluster variance** (Otsu's criterion).

### 4.4 Boundary Artifacts, Blockiness, & Edge Continuity

Low-resolution terminal rendering can introduce severe boundary artifacts:
* **False Grid Edges (Excess Blockiness)**: High-contrast boundaries introduced at cell boundaries that do not exist in the source image.
* **Edge Fragmentation (Broken Hairlines/Horns)**: In 1D sparkline/bar rendering, diagonal contours are stair-stepped into discontinuous horizontal shelves (see [`GlyphEdgeArtifacts.md`](GlyphEdgeArtifacts.md)).

#### 4.4.1 Sobel Edge Gradient Magnitude
Computed over the BT.709 luminance grid $g(y,x)$:

$$G_x = \begin{bmatrix} -1 & 0 & +1 \\ -2 & 0 & +2 \\ -1 & 0 & +1 \end{bmatrix} * g, \qquad G_y = \begin{bmatrix} -1 & -2 & -1 \\ 0 & 0 & 0 \\ +1 & +2 & +1 \end{bmatrix} * g, \qquad S(y, x) = \frac{\sqrt{G_x^2 + G_y^2}}{4}$$

#### 4.4.2 Excess Blockiness ($B_{\text{score}}$)
Measures the average gradient overshoot introduced across cell borders:

$$\text{Excess}(y, x) = \max\big(0, S_{\text{rendered}}(y, x) - S_{\text{reference}}(y, x)\big)$$

$$B_{\text{score}} = \max\left(0,\, 1.0 - \frac{1}{N \cdot E_{\text{max}}} \sum_{(y,x) \in \text{Borders}} \text{Excess}(y, x)\right)$$

A score of $1.0$ indicates that the rendered output introduces zero artificial cell seams.

#### 4.4.3 Edge Continuity / Edge Recall ($E_{\text{cont}}$)
Evaluates whether strong edges in the reference image ($S_{\text{ref}} > \theta_{\text{edge}}$) are retained in the rendered terminal reconstruction within a $\pm 1$ pixel spatial neighborhood:

$$E_{\text{cont}} = \frac{ \sum_{(y,x) \in \text{Edges}} S_{\text{ref}}(y, x) \cdot \min\left(1.0,\, \frac{\max_{\delta \in \mathcal{N}(y,x)} S_{\text{rend}}(\delta)}{S_{\text{ref}}(y, x)}\right) }{ \sum_{(y,x) \in \text{Edges}} S_{\text{ref}}(y, x) }$$

### 4.5 Dithering & Frequency-Domain Texture Metrics

Dithering algorithms (Floyd-Steinberg, Atkinson, Bayer ordered dither, Blue Noise) intentionally introduce high-frequency quantization noise to eliminate low-frequency banding.

* **The Dither Paradox**: High-frequency dither drastically *increases* MSE/PSNR error (because individual pixels deviate from the reference), yet human vision applies a low-pass optical transfer function (Contrast Sensitivity Function, CSF) that filters out the high frequencies, perceiving smooth gradients.
* **Low-Pass Filtered Error (CSF-MSE)**:
  $$E_{\text{CSF}} = \text{MSE}\big(\text{CSF} * I_{\text{src}},\, \text{CSF} * I_{\text{rendered}}\big)$$
  Filtering both images with a Gaussian blur ($\sigma \approx 1.2\text{--}1.8$ subpixels) before calculating MSE or SSIM accurately reflects perceived dither quality.

---

## 5. Comprehensive Metric Comparison Matrix

| Metric | Primary Target | Mathematical Basis | Perceptual Alignment | Computational Complexity | Sensitivity to Shifts | Recommended Usage in Terminal Renderers |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **MSE / SSE** | Pixel distance | $L_2$ Euclidean mean | **Low** | $O(N)$ (Ultra-fast, SIMD) | High | Internal cell solver color fitting; Otsu split optimization |
| **PSNR** | Continuous signal | Logarithmic $L_2$ | **Low** | $O(N)$ (Fast) | High | Legacy codec comparisons; `--smart` width optimizer baseline |
| **SSIM** | Structure & contrast | Mean/variance covariance | **High** | $O(N)$ (Fast, $8 \times 8$ windows) | Moderate | CLI mode sorting (`cati modes --sort ssim`), interactive viewer quality bar |
| **MS-SSIM** | Multi-scale structure | Multi-resolution pyramid | **Very High** | $O(N \log N)$ (Moderate) | Moderate | Offline benchmarking across terminal font sizes / zoom levels |
| **CW-SSIM** | Non-rigid geometry | Complex wavelet phase | **High** | $O(N \log N)$ (Heavy) | **Very Low** | Terminal font aspect ratio & font metric distortion testing |
| **HaarPSI** | Edges & gradients | 2D Haar wavelets | **Very High** | $O(N)$ (Fast) | Low | High-throughput offline golden verification suites |
| **Butteraugli** | JND psychovisual | XYB retinal cone model | **Exceptional** | $O(N)$ (Heavy) | Low | Tuning custom glyph sets & quantizer color curves |
| **SSIMULACRA 2**| Compression artifacts | Multi-scale XYB + edges | **Exceptional** | $O(N)$ (Very Heavy) | Low | Golden standard offline regression testing |
| **LPIPS** | Deep semantic features | VGG / AlexNet features | **State of the Art** | $O(N \cdot W_{\text{deep}})$ (Heavy, GPU) | **Very Low** | Research & perceptual validation of generative terminal art |
| **CIEDE2000** | Color difference | Perceptual Lab $\Delta E_{00}$ | **Very High** | $O(N)$ (Transcendental ops) | None (Color only) | High-fidelity ANSI palette quantization & LUT generation |
| **Oklab $\Delta E$** | Color difference | Linear LMS cube root | **Very High** | $O(N)$ (Fast, 3 roots) | None (Color only) | Real-time 256-color & truecolor terminal color matching |
| **Edge Continuity**| Low-res contours | Sobel gradient recall | **High** (Pixel Art) | $O(N)$ (Fast, $3 \times 3$ kernel) | Low ($\pm 1$ px tolerance) | Hairline & thin-contour integrity tests ([`GlyphEdgeArtifacts.md`](GlyphEdgeArtifacts.md)) |
| **Blockiness** | Cell boundaries | Step-stride border gradient | **High** (Pixel Art) | $O(N)$ (Fast) | High at boundaries | Measuring artificial block seams across quad/half/spark modes |

---

## 6. Real-Time vs. Offline Benchmark Architecture

### 6.1 Real-Time Execution Path ($< 1\,\text{ms}$ Budget)

In interactive playback, video streaming, and responsive CLI browsing (`cati modes`, `cati view`), metric computation must not exceed a fraction of a millisecond.

```text
┌────────────────────────────────────────────────────────────────────────┐
│                   Interactive CLI Quality Pipeline                     │
│                                                                        │
│   Render Frame (e.g. 1.2ms)                                            │
│            │                                                           │
│            ▼                                                           │
│   Fast Downscaled Reconstruction Canvas (k=2 or k=4)                   │
│            │                                                           │
│            ├──> Fast SSIM (BT.709 Luma, non-overlapping 8x8)  [0.3ms]  │
│            ├──> Sobel Edge Continuity (3x3 kernel)            [0.2ms]  │
│            └──> Efficiency Score: (1 - SSIM) * T_render       [0.01ms] │
└────────────────────────────────────────────────────────────────────────┘
```

1. **Reconstruction Factor $K$**: Reconstruct terminal output onto a standard subpixel canvas ($K=4$ via `internal/metrics.GridK`).
2. **Fast Luminance Extraction**: Convert RGB to BT.709 luminance in integer arithmetic or single-precision float:
   $$Y = 0.2126 R + 0.7152 G + 0.0722 B$$
3. **Non-Overlapping Block SSIM**: Compute SSIM over non-overlapping $8 \times 8$ tiles (`internal/metrics.SSIMLuminance`), eliminating sliding-window redundant passes.
4. **Algorithmic Efficiency Index ($\text{Eff}$)**:
   $$\text{Score} = (1.0 - \text{SSIM}) \times T_{\text{render}}$$
   Balances visual quality against computation latency. Modes that achieve high SSIM in $\le 1\text{ms}$ (such as native direct-table `six` and `quad`) achieve the lowest (best) scores.

### 6.2 Offline & Golden Testing Path ($> 10\,\text{ms}$ Budget)

In CI regression test suites (`cmd/golden_render_test.go`, `testhelper`):
1. **Full Reference Multi-Scale Verification**: Combine SSIM with Sobel Edge Continuity and Excess Blockiness into a composite composite score:
   $$Q_{\text{composite}} = 0.6 \cdot \text{SSIM} + 0.25 \cdot E_{\text{cont}} + 0.15 \cdot B_{\text{score}}$$
2. **Metadata Embedding in Golden PNGs**: Embed algorithm name, downscale mode, PSNR, SSIM, and edge continuity scores directly into PNG custom `tEXt` chunks.
3. **Regression Thresholds**: Tests fail if a code modification reduces $Q_{\text{composite}}$ by $>0.005$ without an intentional, documented algorithmic justification.

---

## 7. Implementation Recommendations for Cati

1. **Inner Solver Loops (Character & Color Selection)**:
   * **Rule**: Never use complex perceptual metrics (SSIM, CIEDE2000) inside per-cell candidate loops.
   * **Recommendation**: Use SSE with centroid luminance or Otsu between-cluster variance maximization. For palette quantization, use Euclidean distance in **Oklab** color space.

2. **CLI Ranking & Hint Bar Display**:
   * **Rule**: Keep metrics lightweight, deterministic, and bounded in $[0.0, 1.0]$.
   * **Recommendation**: Standardize on `SSIM` (as computed by `internal/metrics.SSIMLuminance`) and `Eff` (`--sort eff`).

3. **Smart Mode Width Selection (`--smart` / `+smart`)**:
   * **Rule**: Must accurately penalize aliasing caused by bad aspect-ratio column decimation.
   * **Recommendation**: Probe candidate widths within $\pm 4$ columns using `PyramidDownscale` and pick the width that maximizes `PSNR` or `SSIM`.

4. **Edge Artifact & Line Art Diagnostics**:
   * **Rule**: Pure block MSE misses thin diagonal line breaks.
   * **Recommendation**: Use `EdgeContinuityFromGrids` and `SobelGrid` from `internal/metrics` when testing hairline and curved contour rendering.

---

## 8. Related Documentation & References

* [`Glossary.md`](Glossary.md) — Metric taxonomy ($E_{\text{src}}$, $E_{\text{fit}}$, $\text{SSIM}$, $\text{Eff}$), subpixel geometry, and smart rendering.
* [`GlyphEdgeArtifacts.md`](GlyphEdgeArtifacts.md) — 1D bar vs 2D block boundary dynamics, hairlines, and edge artifacts.
* [`SparklinePixelArt.md`](SparklinePixelArt.md) — Optimal split algorithms, 2-color candidate scoring, and scanning traversal.
* [`QuadPixelArt.md`](QuadPixelArt.md) — Aspect ratio compensation and quadrant character lookup tables.
* [`RenderingBugPlaybook.md`](RenderingBugPlaybook.md) — Golden image verification, diagnosing visual bugs, and metric logging.
* **Wang, Z., Bovik, A. C., Sheikh, H. R., & Simoncelli, E. P. (2004)**. *Image quality assessment: from error visibility to structural similarity*. IEEE Transactions on Image Processing, 13(4), 600-612.
* **Reisenhofer, R., Bosse, S., Barthel, D. U., & Wiegand, T. (2018)**. *A Haar Wavelet-Based Perceptual Similarity Index for Image Quality Assessment*. Signal Processing: Image Communication, 61, 33-43.
* **Ottosson, B. (2020)**. *A perceptual color space for image processing (Oklab)*.
* **Sharma, G., Wu, W., & Dalal, E. N. (2005)**. *The CIEDE2000 color-difference formula: Implementation notes, supplementary test data, and mathematical observations*. Color Research & Application, 30(1), 21-30.
