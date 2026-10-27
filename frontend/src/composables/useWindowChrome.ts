import { onBeforeUnmount, onMounted, type Ref } from "vue";
import { Screens, Window } from "@wailsio/runtime";
import {
  blockedByRegion,
  dragTarget,
  keepOnScreen,
  parseGeometry,
  type Point,
  type Rect,
} from "../utils/windowChrome";

const GEOMETRY_KEY = "wgit.window.v1";

/** Runtime environment, or `undefined` when the page is running in a browser. */
function runtimeOS(): string | undefined {
  const wails = (window as unknown as { _wails?: { environment?: { OS?: string } } })._wails;
  return wails?.environment?.OS;
}

/** True inside the desktop shell (as opposed to `vite dev` in a browser tab). */
export function hasWindowRuntime(): boolean {
  return Boolean((window as unknown as { _wails?: { environment?: unknown } })._wails?.environment);
}

/**
 * Manual window dragging.
 *
 * `-webkit-app-region: drag` is resolved by WebKit from how the element paints,
 * and the glass material broke that resolution on macOS: with the material on
 * the header the window simply stopped moving. Rather than keep guessing which
 * paint property is at fault, the drag is driven explicitly — press records the
 * window position, move translates it through the Wails window API. That also
 * means the header can wear any material we like.
 */
export function useWindowDrag(root: Ref<HTMLElement | null>): void {
  let pressed: { from: Point; last: Point; base: Point | null } | null = null;
  let pending: Point | null = null;
  let frame = 0;
  let inFlight = false;

  const flush = () => {
    frame = 0;
    if (!pending || inFlight) return;
    const next = pending;
    pending = null;
    inFlight = true;
    // Fire-and-forget: the next frame may already carry a newer position, so a
    // rejection (window closing, API unavailable) is not worth reporting.
    Promise.resolve(Window.SetPosition(next.x, next.y))
      .catch(() => undefined)
      .finally(() => {
        inFlight = false;
        if (pending) frame = requestAnimationFrame(flush);
      });
  };

  const schedule = (next: Point) => {
    pending = next;
    if (!frame) frame = requestAnimationFrame(flush);
  };

  /** Regions between the pointer target and the drag root that opted out. */
  const regionsBetween = (target: EventTarget | null, host: HTMLElement): string[] => {
    const regions: string[] = [];
    let node = target instanceof Element ? target : null;
    while (node && node !== host) {
      regions.push(getComputedStyle(node).getPropertyValue("-webkit-app-region"));
      node = node.parentElement;
    }
    return regions;
  };

  const onPointerDown = (event: PointerEvent) => {
    if (event.button !== 0 || event.pointerType === "touch") return;
    const host = root.value;
    if (!host) return;
    if (blockedByRegion(regionsBetween(event.target, host))) return;

    const from = { x: event.screenX, y: event.screenY };
    pressed = { from, last: from, base: null };

    // The position arrives asynchronously; until it does, moves are recorded but
    // not applied, so the window never jumps to a stale origin.
    Window.Position()
      .then((position) => {
        if (!pressed) return; // released before the round-trip finished
        pressed.base = { x: position.x, y: position.y };
        if (pressed.last.x !== from.x || pressed.last.y !== from.y) {
          schedule(dragTarget(pressed.base, pressed.from, pressed.last));
        }
      })
      .catch(() => {
        pressed = null;
      });
  };

  const onPointerMove = (event: PointerEvent) => {
    if (!pressed) return;
    pressed.last = { x: event.screenX, y: event.screenY };
    if (!pressed.base) return; // still waiting for the origin
    schedule(dragTarget(pressed.base, pressed.from, pressed.last));
  };

  const finish = () => {
    if (!pressed) return;
    pressed = null;
    pending = null;
    if (frame) {
      cancelAnimationFrame(frame);
      frame = 0;
    }
    saveGeometry();
  };

  const onDoubleClick = (event: MouseEvent) => {
    const host = root.value;
    if (!host) return;
    if (blockedByRegion(regionsBetween(event.target, host))) return;
    Promise.resolve(Window.ToggleMaximise()).catch(() => undefined);
  };

  let resizeTimer = 0;

  const onResize = () => {
    window.clearTimeout(resizeTimer);
    resizeTimer = window.setTimeout(saveGeometry, 400);
  };

  onMounted(() => {
    if (runtimeOS() !== "darwin") return;
    const host = root.value;
    if (!host) return;
    host.addEventListener("pointerdown", onPointerDown);
    host.addEventListener("dblclick", onDoubleClick);
    window.addEventListener("pointermove", onPointerMove, { passive: true });
    window.addEventListener("pointerup", finish);
    window.addEventListener("pointercancel", finish);
    window.addEventListener("resize", onResize);
  });

  onBeforeUnmount(() => {
    const host = root.value;
    if (host) {
      host.removeEventListener("pointerdown", onPointerDown);
      host.removeEventListener("dblclick", onDoubleClick);
    }
    window.removeEventListener("pointermove", onPointerMove);
    window.removeEventListener("pointerup", finish);
    window.removeEventListener("pointercancel", finish);
    window.removeEventListener("resize", onResize);
    window.clearTimeout(resizeTimer);
    if (frame) cancelAnimationFrame(frame);
  });
}

/** Persist the current window rect so the next launch reopens in place. */
export async function saveGeometry(): Promise<void> {
  if (!hasWindowRuntime()) return;
  try {
    const [position, size] = await Promise.all([Window.Position(), Window.Size()]);
    const rect: Rect = { x: position.x, y: position.y, width: size.width, height: size.height };
    localStorage.setItem(GEOMETRY_KEY, JSON.stringify(rect));
  } catch {
    // Persisting chrome geometry is a nicety; never surface a failure.
  }
}

/** Screen work areas in the shape `keepOnScreen` expects. */
function screenRects(screens: Awaited<ReturnType<typeof Screens.GetAll>>): Rect[] {
  const out: Rect[] = [];
  for (const screen of screens) {
    const area =
      (screen as unknown as { WorkArea?: { X: number; Y: number; Size?: { width: number; height: number } } })
        .WorkArea ?? screen;
    const size =
      (area as { Size?: { width: number; height: number } }).Size ??
      (area as unknown as { width: number; height: number });
    const x = (area as unknown as { X?: number }).X;
    const y = (area as unknown as { Y?: number }).Y;
    if (typeof x !== "number" || typeof y !== "number") continue;
    if (typeof size?.width !== "number" || typeof size?.height !== "number") continue;
    out.push({ x, y, width: size.width, height: size.height });
  }
  return out;
}

/**
 * Restore the window rect saved by `saveGeometry`, but only when it still lands
 * on a connected display. A laptop that was docked to a 5K monitor yesterday
 * must not reopen the window at coordinates that no longer exist.
 */
export async function restoreGeometry(): Promise<void> {
  if (!hasWindowRuntime()) return;
  const saved = parseGeometry(localStorage.getItem(GEOMETRY_KEY));
  if (!saved) return;
  try {
    const rect = keepOnScreen(saved, screenRects(await Screens.GetAll()));
    if (!rect) return;
    await Window.SetSize(rect.width, rect.height);
    await Window.SetPosition(rect.x, rect.y);
  } catch {
    // Same reasoning as `saveGeometry`: chrome restoration is best-effort.
  }
}
