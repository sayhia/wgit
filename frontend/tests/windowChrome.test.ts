import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import {
  blockedByRegion,
  dragTarget,
  keepOnScreen,
  parseGeometry,
  MIN_VISIBLE,
  MIN_SIZE,
  type Rect,
} from "../src/utils/windowChrome";

describe("P0 & P2.1: Window Chrome, Drag Geometry & Size Persistence", () => {
  const SRC = join(import.meta.dir, "../src");

  test("dragTarget calculates screen coordinate offsets accurately", () => {
    const base = { x: 100, y: 200 };
    const from = { x: 150, y: 250 };
    const to = { x: 180, y: 300 };

    const target = dragTarget(base, from, to);
    expect(target).toEqual({ x: 130, y: 250 });
  });

  test("blockedByRegion identifies no-drag markers", () => {
    expect(blockedByRegion(["no-drag"])).toBe(true);
    expect(blockedByRegion(["drag", "NO-DRAG"])).toBe(true);
    expect(blockedByRegion(["drag", null, undefined])).toBe(false);
    expect(blockedByRegion([])).toBe(false);
  });

  test("keepOnScreen keeps windows accessible on displays", () => {
    const screens: Rect[] = [
      { x: 0, y: 0, width: 1920, height: 1080 },
    ];

    // Window nicely inside display
    const normalWindow: Rect = { x: 100, y: 100, width: 1200, height: 800 };
    expect(keepOnScreen(normalWindow, screens)).toEqual(normalWindow);

    // Window drifted off-screen
    const offScreenWindow: Rect = { x: 5000, y: 5000, width: 1200, height: 800 };
    expect(keepOnScreen(offScreenWindow, screens)).toBeNull();

    // Window partially off left edge: clamped to remain reachable
    const partialLeft: Rect = { x: -1100, y: 50, width: 1200, height: 800 };
    const clamped = keepOnScreen(partialLeft, screens);
    expect(clamped).not.toBeNull();
    expect(clamped!.x).toBeGreaterThanOrEqual(0 - 1200 + MIN_VISIBLE);
  });

  test("parseGeometry parses valid JSON and rejects corrupted records", () => {
    const valid = JSON.stringify({ x: 100, y: 150, width: 1280, height: 800 });
    expect(parseGeometry(valid)).toEqual({ x: 100, y: 150, width: 1280, height: 800 });

    // Rejects below min size
    const tooSmall = JSON.stringify({ x: 10, y: 10, width: 100, height: 100 });
    expect(parseGeometry(tooSmall)).toBeNull();

    // Rejects absurd dimensions
    const absurd = JSON.stringify({ x: 0, y: 0, width: 999999, height: 999999 });
    expect(parseGeometry(absurd)).toBeNull();

    // Rejects null / non-JSON
    expect(parseGeometry(null)).toBeNull();
    expect(parseGeometry("invalid json")).toBeNull();
  });

  test("keepOnScreen never restores a window under the menu bar", () => {
    const laptop: Rect = { x: 0, y: 25, width: 1512, height: 945 };
    const fitted = keepOnScreen({ x: 200, y: -300, width: 1380, height: 880 }, [laptop]);
    expect(fitted!.y).toBe(laptop.y);
  });

  test("keepOnScreen accepts a secondary display", () => {
    const laptop: Rect = { x: 0, y: 25, width: 1512, height: 945 };
    const external: Rect = { x: 1512, y: 0, width: 2560, height: 1440 };
    const rect: Rect = { x: 1700, y: 200, width: 1380, height: 880 };
    expect(keepOnScreen(rect, [laptop, external])).toEqual(rect);
  });

  test("parseGeometry rounds fractional coordinates", () => {
    expect(parseGeometry('{"x":10.6,"y":20.2,"width":1380.4,"height":880.9}')).toEqual({
      x: 11,
      y: 20,
      width: 1380,
      height: 881,
    });
  });

  test("blockedByRegion ignores casing and padding", () => {
    expect(blockedByRegion(["  No-Drag "])).toBe(true);
    expect(blockedByRegion([null, undefined, ""])).toBe(false);
  });

  test("the manual drag reads declared blockers, not inherited computed style", () => {
    // `-webkit-app-region` inherits, and the macOS header is `no-drag` on
    // purpose. Reading it back made every descendant look blocked, so the bar
    // could not move the window at all.
    const composable = readFileSync(join(SRC, "composables/useWindowChrome.ts"), "utf8");
    expect(composable).not.toContain("getComputedStyle");
    expect(composable).toContain("INTERACTIVE_SELECTOR");
  });

  test("no header group re-blocks the drag strip", () => {
    // A `no-drag` group spanning the bar (the flex-1 centre group especially)
    // leaves only the 12px padding draggable.
    const header = readFileSync(join(SRC, "components/HeaderBar.vue"), "utf8");
    expect(header).not.toMatch(/class="[^"]*\bno-drag\b/);
    expect(header).not.toContain("-webkit-app-region: no-drag;");
  });
});
