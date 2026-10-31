import { describe, expect, test } from "bun:test";
import { readFileSync, readdirSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

/** Workbench material policy: solid chrome and neutral, dense floating layers.
 * Backdrop effects must degrade safely without introducing pointer lighting
 * or compositing the native window drag region.
 */

const ROOT = fileURLToPath(new URL("..", import.meta.url));
const css = readFileSync(join(ROOT, "src/styles/glass.css"), "utf8");

/** Declarations of the first block whose selector list contains `needle`. */
function block(needle: string): string {
  const re = /([^{}]+)\{([^{}]*)\}/g;
  const withoutComments = css.replace(/\/\*[\s\S]*?\*\//g, "");
  for (const m of withoutComments.matchAll(re)) {
    if (m[1]!.includes(needle)) return m[2]!;
  }
  throw new Error(`missing CSS block: ${needle}`);
}

/** Declarations of the block whose selector is exactly `selector`. */
function blockOf(selector: string): string {
  const re = /([^{}]+)\{([^{}]*)\}/g;
  const withoutComments = css.replace(/\/\*[\s\S]*?\*\//g, "");
  for (const m of withoutComments.matchAll(re)) {
    if (m[1]!.trim().split(",").some((part) => part.trim() === selector)) return m[2]!;
  }
  throw new Error(`missing CSS block: ${selector}`);
}

/** Alpha of a `color-mix(... N%, transparent)` tint, as 0..1. */
function tintOpacity(themeBlock: string): number {
  const decl = themeBlock.match(/--glass-tint:\s*([\s\S]*?);/);
  if (!decl) throw new Error("missing --glass-tint");
  const alphas = [...decl[1]!.matchAll(/(\d+(?:\.\d+)?)%\s*,\s*transparent\s*\)/g)];
  if (!alphas.length) throw new Error("tint is not a color-mix with transparent");
  return Number(alphas[alphas.length - 1]![1]) / 100;
}

describe("glass material", () => {
  const dark = block('[data-theme="dark"]');
  const light = block('[data-theme="light"]');

  test("dark tint stays at or above the 94% readability floor", () => {
    expect(tintOpacity(dark)).toBeGreaterThanOrEqual(0.94);
  });

  test("light tint stays at or above the 94% readability floor", () => {
    expect(tintOpacity(light)).toBeGreaterThanOrEqual(0.94);
  });

  test("material colors and elevation derive from the theme", () => {
    for (const theme of [dark, light]) {
      expect(theme).toContain("var(--ug-bg-overlay)");
      expect(theme).toContain("var(--ug-border-default)");
      expect(theme).toContain("var(--ug-shadow-modal)");
      expect(theme).not.toContain("--glass-sheen");
      expect(theme).not.toContain("--glass-bloom");
    }
  });

  test("floating material has one blur and no decorative paint layers", () => {
    const glass = blockOf(".glass");
    expect(glass).toContain("backdrop-filter: blur(var(--glass-blur))");
    expect(glass).toContain("-webkit-backdrop-filter:");
    expect(glass).toContain("background-image: none");
    expect(css).not.toContain("radial-gradient");
    expect(css).not.toContain("saturate(");
    expect(css).not.toContain("--glass-x");
    expect(css).not.toContain("--glass-y");
  });

  test("the window-frame variant keeps the drag strip free of compositing", () => {
    // `-webkit-app-region: drag` is resolved from how the element paints. Every
    // one of these has been implicated in breaking it, so the bar variant is
    // deliberately "boring": flat colour + shadows only.
    const bar = blockOf(".glass-bar");
    expect(bar).toContain("backdrop-filter: none");
    expect(bar).toContain("-webkit-backdrop-filter: none");
    expect(bar).toContain("background-image: none");
    expect(bar).toContain("transition: none");
    expect(bar).toContain("box-shadow: none");
    expect(bar).toContain("var(--ug-bg-chrome)");
    expect(blockOf(".glass")).not.toContain("position: relative");
  });

  test("no full-cover pseudo-element (it breaks window dragging)", () => {
    // WKWebView resolves `-webkit-app-region: drag` by hit-testing. A pseudo
    // element covering the whole surface becomes the topmost box in the drag
    // strip and carries no region, so the window stops moving. The sheen must
    // stay on `background-image`.
    expect(css).not.toContain(".glass::before");
    expect(blockOf(".glass")).not.toContain("isolation: isolate");
  });

  test("opaque fallbacks exist for reduced transparency and missing support", () => {
    expect(css).toContain("@supports not ((backdrop-filter: blur(1px))");
    expect(css).toContain("@media (prefers-reduced-transparency: reduce)");
    const reduced = css.slice(css.indexOf("@media (prefers-reduced-transparency: reduce)"));
    expect(reduced).toContain("background-color: var(--ug-bg-overlay)");
    expect(reduced).toContain("backdrop-filter: none");
  });

  test("forced-colors collapses the material to a system surface", () => {
    // Windows High Contrast replaces every colour with a system pair; a
    // translucent tint then lets the page bleed through and the panel loses its
    // edge. The block must repaint to Canvas/CanvasText and drop the blur.
    const start = css.indexOf("@media (forced-colors: active)");
    expect(start).toBeGreaterThan(-1);
    const blockBody = css.slice(start);
    expect(blockBody).toContain("background-color: Canvas");
    expect(blockBody).toContain("border-color: CanvasText");
    expect(blockBody).toContain("backdrop-filter: none");
    expect(blockBody).toContain('[aria-current="page"]');
    expect(blockBody).toContain("outline: 1px solid Highlight");
  });

  test("pointer lighting is absent from the material and app startup", () => {
    const app = readFileSync(join(ROOT, "src/App.vue"), "utf8");
    expect(app).not.toContain("useLiquidGlass");
    expect(css).not.toContain("--glass-x");
    expect(css).not.toContain("--glass-y");
  });
});

describe("single source of truth", () => {
  test("style.css does not redefine the glass classes", () => {
    // A legacy `.glass-panel` block lived in style.css and silently overrode the
    // new material (0.75 opaque, blur 20px) — the popover bled the content
    // behind it. Only styles/glass.css may define these.
    const appCss = readFileSync(join(ROOT, "src/style.css"), "utf8");
    const withoutComments = appCss.replace(/\/\*[\s\S]*?\*\//g, "");
    for (const selector of [".glass-panel", ".glass-pill", ".glass-bar", ".glass {"]) {
      expect(withoutComments.includes(selector)).toBe(false);
    }
  });
});

describe("glass usage", () => {
  const SRC = fileURLToPath(new URL("../src", import.meta.url));

  function walk(dir: string, out: string[] = []): string[] {
    for (const entry of readdirSync(dir, { withFileTypes: true })) {
      const full = join(dir, entry.name);
      if (entry.isDirectory()) walk(full, out);
      else if (entry.name.endsWith(".vue")) out.push(full);
    }
    return out;
  }

  /**
   * The `glass` class token, but not the `--glass-*` custom properties that
   * appear inside `bg-[var(--glass-raise)]` — those are values, not classes.
   */
  const GLASS_CLASS = /(?<![-\w])glass(?![\w-])/;
  const GLASS_VARIANT = /(?<![-\w])glass-(bar|panel)(?![\w-])/;

  test("every glass surface picks an intensity variant", () => {
    const offenders: string[] = [];
    for (const file of walk(SRC)) {
      const source = readFileSync(file, "utf8");
      for (const m of source.matchAll(/(?::|v-bind:)?class="([^"]*)"/g)) {
        const body = m[1]!;
        if (!GLASS_CLASS.test(body)) continue;
        if (!GLASS_VARIANT.test(body)) {
          offenders.push(`${file.slice(SRC.length + 1)}: ${body.trim().slice(0, 70)}`);
        }
      }
    }
    expect(offenders).toEqual([]);
  });

  test("the window drag strip does not rely on -webkit-app-region", () => {
    // The material broke WebKit's drag-region resolution on macOS, so the drag is
    // driven in JS (`useWindowChrome`) and the native region is switched off
    // there. The header can therefore wear `.glass-bar` again — but it must keep
    // binding the manual drag, or the window becomes immovable.
    const header = readFileSync(join(SRC, "components/HeaderBar.vue"), "utf8");
    const tag = header.match(/<header[\s\S]*?>/);
    expect(tag).not.toBeNull();
    expect(tag![0]).toContain('ref="headerRef"');
    expect(tag![0]).toContain(':style="dragStyle"');
    expect(header).toContain("useWindowDrag(headerRef)");
    // On macOS the native region must be `no-drag`, otherwise the window moves
    // twice (once natively, once through SetPosition).
    expect(header).toMatch(/===\s+"darwin"[\s\S]{0,40}"no-drag"/);
  });

  test("no glass surface keeps an opaque background utility", () => {
    // `bg-surface` in the same class list would win over the tint (utilities
    // layer) and silently turn the material back into a flat panel.
    const offenders: string[] = [];
    for (const file of walk(SRC)) {
      const source = readFileSync(file, "utf8");
      for (const m of source.matchAll(/(?::|v-bind:)?class="([^"]*)"/g)) {
        const body = m[1]!;
        if (!GLASS_CLASS.test(body)) continue;
        if (/\bbg-(surface|canvas|overlay|raised)\b(?!\/)/.test(body)) {
          offenders.push(`${file.slice(SRC.length + 1)}: ${body.trim().slice(0, 70)}`);
        }
      }
    }
    expect(offenders).toEqual([]);
  });

  test("every transition stays inside the motion budget", () => {
    // 300 ms is the ceiling for anything that answers a user action. The one
    // exception is the specular glide (`--glass-x` / `--glass-y`): it is a
    // painted highlight chasing the pointer, not a state change, and shortening
    // it makes the material read as a hard snap.
    const offenders: string[] = [];
    for (const file of walk(SRC)) {
      if (!file.endsWith(".css")) continue;
      const css = readFileSync(file, "utf8");
      for (const m of css.matchAll(/transition:\s*([^;]+);/g)) {
        const value = m[1]!;
        if (/^\s*none\b/.test(value)) continue;
        const isSheen = /--glass-[xy]/.test(value);
        const limit = isSheen ? 400 : 300;
        for (const duration of value.matchAll(/([0-9.]+)(m?s)\b/g)) {
          const ms = duration[2] === "s" ? Number(duration[1]) * 1000 : Number(duration[1]);
          if (ms > limit) {
            offenders.push(`${file.slice(SRC.length + 1)}: ${value.trim().slice(0, 70)} (${ms}ms > ${limit}ms)`);
          }
        }
      }
    }
    expect(offenders).toEqual([]);
  });
});
