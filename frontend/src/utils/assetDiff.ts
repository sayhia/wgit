/**
 * Pure helpers for the binary-asset pixel diff.
 *
 * Extracted from AssetDiffModal so the thresholds and eligibility rules are
 * testable without a DOM or a canvas.
 */

/** Sum of |Δr| + |Δg| + |Δb| above which a pixel counts as changed. */
export const DIFF_PIXEL_THRESHOLD = 24;

/** Above this many pixels the O(w*h) JS loop is skipped. */
export const DIFF_MAX_PIXELS = 4_000_000;

export interface DiffEligibility {
  ok: boolean;
  reason: string;
}

/**
 * A pixel diff only makes sense when both sides share a canvas, and only when
 * the image is small enough to walk in JS.
 */
export function checkDiffEligibility(
  oldWidth: number,
  oldHeight: number,
  newWidth: number,
  newHeight: number
): DiffEligibility {
  if (oldWidth <= 0 || oldHeight <= 0 || newWidth <= 0 || newHeight <= 0) {
    return { ok: false, reason: "缺少尺寸信息，无法逐像素对比" };
  }
  if (oldWidth !== newWidth || oldHeight !== newHeight) {
    return {
      ok: false,
      reason: `尺寸不同（${oldWidth}×${oldHeight} vs ${newWidth}×${newHeight}），无法逐像素对比`,
    };
  }
  if (oldWidth * oldHeight > DIFF_MAX_PIXELS) {
    return {
      ok: false,
      reason: `图像较大（${oldWidth}×${oldHeight}，超过 ${Math.round(DIFF_MAX_PIXELS / 10000)} 万像素），已跳过逐像素对比`,
    };
  }
  return { ok: true, reason: "" };
}

export function pixelDelta(
  r0: number,
  g0: number,
  b0: number,
  r1: number,
  g1: number,
  b1: number
): number {
  return Math.abs(r0 - r1) + Math.abs(g0 - g1) + Math.abs(b0 - b1);
}

export function isChangedPixel(delta: number): boolean {
  return delta > DIFF_PIXEL_THRESHOLD;
}

/**
 * Dimmed grayscale base for unchanged pixels, so the magenta highlights read
 * clearly against it instead of competing with full-brightness artwork.
 */
export function dimmedGray(r: number, g: number, b: number): number {
  return (r * 0.3 + g * 0.59 + b * 0.11) * 0.3 + 38;
}

/** Highlight colour written into changed pixels. */
export const DIFF_HIGHLIGHT = { r: 255, g: 46, b: 99 } as const;
