/**
 * Pure geometry for the window chrome.
 *
 * The window drag region used to rely on `-webkit-app-region: drag`, which
 * WebKit resolves from how an element paints. Every variant of the glass
 * material broke that resolution on macOS, so dragging is now implemented in
 * JS: `mousedown` records the window position, `mousemove` moves it through the
 * Wails window API. That makes the maths below load-bearing, and testable
 * without a browser.
 */

export interface Point {
  x: number;
  y: number;
}

export interface Rect {
  x: number;
  y: number;
  width: number;
  height: number;
}

/** How much of the window must stay reachable on a screen. */
export const MIN_VISIBLE = 96;

/** Smallest window we accept from persisted geometry (matches `MinWidth/MinHeight`). */
export const MIN_SIZE = { width: 960, height: 640 };

/**
 * Window position implied by a pointer move, given the drag origin.
 *
 * Screen coordinates are used (`screenX`/`screenY`) rather than client ones so
 * the maths stays correct while the window itself is moving under the pointer.
 */
export function dragTarget(base: Point, from: Point, to: Point): Point {
  return {
    x: Math.round(base.x + (to.x - from.x)),
    y: Math.round(base.y + (to.y - from.y)),
  };
}

/**
 * Whether any element between the pointer target and the drag root opted out.
 *
 * The markup already marks controls with `-webkit-app-region: no-drag`; that
 * value is read back from computed style, so a plain list of region strings is
 * all the decision needs.
 */
export function blockedByRegion(regions: (string | null | undefined)[]): boolean {
  return regions.some((region) => (region ?? "").trim().toLowerCase() === "no-drag");
}

/**
 * Fit a persisted window rect back onto the current screens.
 *
 * Returns `null` when the saved position would leave less than `minVisible`
 * pixels reachable on every screen — the display it lived on is gone, so the
 * caller should fall back to the default geometry instead of restoring a window
 * the user cannot reach.
 */
export function keepOnScreen(rect: Rect, screens: Rect[], minVisible = MIN_VISIBLE): Rect | null {
  if (!screens.length) return null;

  for (const screen of screens) {
    const overlapX =
      Math.min(rect.x + rect.width, screen.x + screen.width) - Math.max(rect.x, screen.x);
    const overlapY =
      Math.min(rect.y + rect.height, screen.y + screen.height) - Math.max(rect.y, screen.y);
    if (overlapX < minVisible || overlapY < minVisible) continue;

    return {
      // Never let the title bar slide under the menu bar, and never let the
      // window drift so far right/bottom that only a sliver stays grabbable.
      x: Math.min(Math.max(rect.x, screen.x - rect.width + minVisible), screen.x + screen.width - minVisible),
      y: Math.min(Math.max(rect.y, screen.y), screen.y + screen.height - minVisible),
      width: rect.width,
      height: rect.height,
    };
  }
  return null;
}

/** Parse persisted geometry, rejecting anything malformed or absurd. */
export function parseGeometry(raw: string | null): Rect | null {
  if (!raw) return null;
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch {
    return null;
  }
  if (!parsed || typeof parsed !== "object") return null;

  const value = parsed as Record<string, unknown>;
  const nums = ["x", "y", "width", "height"].map((key) => value[key]);
  if (nums.some((n) => typeof n !== "number" || !Number.isFinite(n))) return null;

  const [x, y, width, height] = nums as number[];
  if (width! < MIN_SIZE.width || height! < MIN_SIZE.height) return null;
  // A window larger than any plausible display is a corrupted record.
  if (width! > 20000 || height! > 20000) return null;

  return { x: Math.round(x!), y: Math.round(y!), width: Math.round(width!), height: Math.round(height!) };
}
