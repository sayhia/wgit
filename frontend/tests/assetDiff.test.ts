import { describe, expect, test } from "bun:test";
import {
  DIFF_MAX_PIXELS,
  DIFF_PIXEL_THRESHOLD,
  checkDiffEligibility,
  dimmedGray,
  isChangedPixel,
  pixelDelta,
} from "../src/utils/assetDiff";

describe("checkDiffEligibility", () => {
  test("accepts two sides with identical dimensions", () => {
    const res = checkDiffEligibility(512, 512, 512, 512);
    expect(res.ok).toBe(true);
    expect(res.reason).toBe("");
  });

  test("rejects different dimensions and names both", () => {
    const res = checkDiffEligibility(512, 512, 1024, 1024);
    expect(res.ok).toBe(false);
    expect(res.reason).toContain("512×512");
    expect(res.reason).toContain("1024×1024");
  });

  test("rejects missing dimensions (0 width)", () => {
    const res = checkDiffEligibility(0, 0, 0, 0);
    expect(res.ok).toBe(false);
    expect(res.reason).toContain("缺少尺寸");
  });

  test("rejects images above the pixel budget", () => {
    // 2001 * 2000 = 4,002,000 — just over DIFF_MAX_PIXELS.
    const res = checkDiffEligibility(2001, 2000, 2001, 2000);
    expect(res.ok).toBe(false);
    expect(res.reason).toContain("像素");

    const boundary = checkDiffEligibility(2000, 2000, 2000, 2000);
    expect(boundary.ok).toBe(true);
    expect(2000 * 2000).toBeLessThanOrEqual(DIFF_MAX_PIXELS);
  });
});

describe("pixelDelta", () => {
  test("is zero for identical colours", () => {
    expect(pixelDelta(10, 20, 30, 10, 20, 30)).toBe(0);
  });

  test("sums the three channel deltas", () => {
    expect(pixelDelta(0, 0, 0, 1, 2, 3)).toBe(6);
    expect(pixelDelta(255, 255, 255, 0, 0, 0)).toBe(765);
  });

  test("is symmetric", () => {
    expect(pixelDelta(1, 2, 3, 4, 5, 6)).toBe(pixelDelta(4, 5, 6, 1, 2, 3));
  });
});

describe("isChangedPixel", () => {
  test("uses a strict greater-than threshold", () => {
    expect(isChangedPixel(DIFF_PIXEL_THRESHOLD)).toBe(false);
    expect(isChangedPixel(DIFF_PIXEL_THRESHOLD + 1)).toBe(true);
    expect(isChangedPixel(0)).toBe(false);
  });

  test("treats a one-channel 25-step shift as changed", () => {
    expect(isChangedPixel(pixelDelta(0, 0, 0, 25, 0, 0))).toBe(true);
  });

  test("ignores a subtle 8-step shift", () => {
    expect(isChangedPixel(pixelDelta(0, 0, 0, 8, 0, 0))).toBe(false);
  });
});

describe("dimmedGray", () => {
  test("stays a valid byte for black and white", () => {
    const black = dimmedGray(0, 0, 0);
    const white = dimmedGray(255, 255, 255);
    expect(black).toBeGreaterThanOrEqual(0);
    expect(white).toBeLessThanOrEqual(255);
    expect(white).toBeGreaterThan(black);
  });

  test("is dimmer than the source luminance", () => {
    const r = 200;
    const g = 200;
    const b = 200;
    const luminance = r * 0.3 + g * 0.59 + b * 0.11;
    expect(dimmedGray(r, g, b)).toBeLessThan(luminance);
  });
});
