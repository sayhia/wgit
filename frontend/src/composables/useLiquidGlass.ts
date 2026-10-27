import { onMounted, onUnmounted } from "vue";

/**
 * Pointer-reactive specular for Liquid Glass surfaces.
 *
 * A single delegated listener updates `--glass-x` / `--glass-y` on the nearest
 * `[data-glass]` element, which the stylesheet turns into a radial highlight in
 * the element's own `background-image`. Those two custom properties are
 * registered with `@property`, so the browser can *interpolate* them — the
 * highlight glides between pointer samples instead of jumping, which is what
 * sells the material as a liquid rather than a static gradient.
 *
 * Deliberate limits: one rAF-batched listener for the whole app, no work on
 * touch input (no hover to track), nothing at all when the user asked for
 * reduced motion, and **no work on bars** — `.glass-bar` paints no highlight
 * (see `glass.css`), so tracking the pointer there would cost frames for a
 * gradient that is never drawn.
 */
export function useLiquidGlass(): void {
  let frame = 0;
  let pending: { x: number; y: number; target: Element } | null = null;

  const flush = () => {
    frame = 0;
    const sample = pending;
    pending = null;
    if (!sample) return;

    const host = sample.target.closest("[data-glass]");
    if (!(host instanceof HTMLElement)) return;
    // Bars have no specular layer; skip them instead of writing unused vars.
    if (host.classList.contains("glass-bar")) return;

    const rect = host.getBoundingClientRect();
    if (rect.width < 8 || rect.height < 8) return;

    const x = ((sample.x - rect.left) / rect.width) * 100;
    const y = ((sample.y - rect.top) / rect.height) * 100;
    host.style.setProperty("--glass-x", `${x.toFixed(1)}%`);
    host.style.setProperty("--glass-y", `${y.toFixed(1)}%`);
  };

  const onPointerMove = (event: PointerEvent) => {
    // Touch has no hover state to track; skip the work entirely.
    if (event.pointerType === "touch") return;
    if (!(event.target instanceof Element)) return;
    pending = { x: event.clientX, y: event.clientY, target: event.target };
    if (!frame) frame = requestAnimationFrame(flush);
  };

  onMounted(() => {
    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
    window.addEventListener("pointermove", onPointerMove, { passive: true });
  });

  onUnmounted(() => {
    window.removeEventListener("pointermove", onPointerMove);
    if (frame) cancelAnimationFrame(frame);
  });
}
