import { describe, expect, test } from "bun:test";
import { readFileSync, readdirSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

/**
 * Contrast contract for the design tokens.
 *
 * The palette is CSS-only, so nothing in the TS layer can catch a "white label
 * on a too-light solid" regression. This test parses the two stylesheets the
 * app actually ships, resolves the variables per theme x accent the same way
 * the cascade does, and asserts the WCAG AA ratios for the pairs that get
 * painted together.
 *
 * It also guards the source: a component may not put `text-white` on an accent
 * fill, because the label ink has to follow `--ug-accent-contrast` (teal, green
 * and amber use a dark label in dark mode).
 */

const ROOT = fileURLToPath(new URL("..", import.meta.url));

type Rgb = { r: number; g: number; b: number };

function parseHex(value: string): Rgb | null {
  const m = value.match(/^#([0-9a-f]{3}|[0-9a-f]{6})$/i);
  if (!m) return null;
  const hex = m[1]!.length === 3 ? m[1]!.split("").map((c) => c + c).join("") : m[1]!;
  return {
    r: parseInt(hex.slice(0, 2), 16),
    g: parseInt(hex.slice(2, 4), 16),
    b: parseInt(hex.slice(4, 6), 16),
  };
}

function luminance({ r, g, b }: Rgb): number {
  const channel = (v: number) => {
    const x = v / 255;
    return x <= 0.03928 ? x / 12.92 : Math.pow((x + 0.055) / 1.055, 2.4);
  };
  return 0.2126 * channel(r) + 0.7152 * channel(g) + 0.0722 * channel(b);
}

function contrast(a: Rgb, b: Rgb): number {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x) as [number, number];
  return (hi + 0.05) / (lo + 0.05);
}

/** Collect `--name: #hex` declarations from a CSS block body. */
function declarations(body: string): Record<string, string> {
  const out: Record<string, string> = {};
  for (const m of body.matchAll(/(--[a-z0-9-]+)\s*:\s*(#[0-9a-fA-F]{3,6})\s*;/g)) {
    out[m[1]!] = m[2]!;
  }
  return out;
}

/**
 * Split a stylesheet into `{ selector, body }` chunks.
 *
 * Comments are stripped first: the banner above the first preset would
 * otherwise be folded into its selector and the block would never match.
 */
function blocks(css: string): { selector: string; body: string }[] {
  const out: { selector: string; body: string }[] = [];
  const withoutComments = css.replace(/\/\*[\s\S]*?\*\//g, "");
  const re = /([^{}]+)\{([^{}]*)\}/g;
  for (const m of withoutComments.matchAll(re)) {
    out.push({ selector: m[1]!.trim().replace(/\s+/g, " "), body: m[2]! });
  }
  return out;
}

const tokensCss = readFileSync(join(ROOT, "design-system/tokens.css"), "utf8");
const presetsCss = readFileSync(join(ROOT, "src/accent-presets.css"), "utf8");

const tokenBlocks = blocks(tokensCss);
const darkBase = declarations(
  tokenBlocks.find((b) => b.selector.includes('[data-theme="dark"]'))!.body
);
const lightBase = declarations(
  tokenBlocks.find((b) => b.selector.startsWith('[data-theme="light"]'))!.body
);

const ACCENTS = ["blue", "violet", "teal", "green", "amber", "rose"] as const;

function preset(accent: string, theme: "dark" | "light"): Record<string, string> {
  if (accent === "blue") return {}; // blue is the token default
  const selector =
    theme === "dark"
      ? `:root[data-accent="${accent}"]`
      : `:root[data-accent="${accent}"][data-theme="light"]`;
  const block = tokenBlocks.length && blocks(presetsCss).find((b) => b.selector === selector);
  if (!block) throw new Error(`missing preset block: ${selector}`);
  return declarations(block.body);
}

function palette(theme: "dark" | "light", accent: string): Record<string, string> {
  return { ...(theme === "dark" ? darkBase : lightBase), ...preset(accent, theme) };
}

/** Pairs that are actually painted together, with their WCAG AA threshold. */
const PAIRS: [string, string, string][] = [
  ["accent solid + label", "--ug-accent-contrast", "--ug-accent-strong"],
  ["accent solid hover + label", "--ug-accent-contrast", "--ug-accent-strong-hover"],
  ["success solid + label", "--ug-success-contrast", "--ug-success-strong"],
  ["danger solid + label", "#ffffff", "--ug-danger-strong"],
  ["warning solid + label", "--ug-warning-contrast", "--ug-warning-strong"],
  ["purple solid + label", "#ffffff", "--ug-purple-strong"],
  ["cyan solid + label", "#ffffff", "--ug-cyan-strong"],
  ["primary ink on canvas", "--ug-text-primary", "--ug-bg-canvas"],
  ["primary ink on surface", "--ug-text-primary", "--ug-bg-surface"],
  ["primary ink on raised", "--ug-text-primary", "--ug-bg-raised"],
  ["secondary ink on surface", "--ug-text-secondary", "--ug-bg-surface"],
  ["secondary ink on raised", "--ug-text-secondary", "--ug-bg-raised"],
  ["muted ink on canvas", "--ug-text-muted", "--ug-bg-canvas"],
  ["muted ink on surface", "--ug-text-muted", "--ug-bg-surface"],
  ["muted ink on raised", "--ug-text-muted", "--ug-bg-raised"],
  ["accent ink on surface", "--ug-accent", "--ug-bg-surface"],
  ["accent ink on raised", "--ug-accent", "--ug-bg-raised"],
  ["success ink on raised", "--ug-success", "--ug-bg-raised"],
  ["cyan ink on raised", "--ug-cyan", "--ug-bg-raised"],
  ["warning ink on raised", "--ug-warning", "--ug-bg-raised"],
  ["danger ink on raised", "--ug-danger", "--ug-bg-raised"],
  ["purple ink on raised", "--ug-purple", "--ug-bg-raised"],
];

describe("design token contrast", () => {
  for (const theme of ["dark", "light"] as const) {
    for (const accent of ACCENTS) {
      test(`${theme} / ${accent} meets WCAG AA on every painted pair`, () => {
        const vars = palette(theme, accent);
        const failures: string[] = [];

        for (const [label, fgRef, bgRef] of PAIRS) {
          const fgValue = fgRef.startsWith("#") ? fgRef : vars[fgRef];
          const bgValue = bgRef.startsWith("#") ? bgRef : vars[bgRef];
          const fg = fgValue ? parseHex(fgValue) : null;
          const bg = bgValue ? parseHex(bgValue) : null;
          if (!fg || !bg) {
            failures.push(`${label}: unresolved (${fgRef}=${fgValue}, ${bgRef}=${bgValue})`);
            continue;
          }
          const ratio = contrast(fg, bg);
          if (ratio + 0.005 < 4.5) {
            failures.push(`${label}: ${ratio.toFixed(2)} < 4.5 (${fgValue} on ${bgValue})`);
          }
        }

        expect(failures).toEqual([]);
      });
    }
  }

  test("every accent preset defines a label ink", () => {
    for (const theme of ["dark", "light"] as const) {
      for (const accent of ACCENTS) {
        expect(palette(theme, accent)["--ug-accent-contrast"]).toBeTruthy();
      }
    }
  });
});

describe("solid accent fills use the contrast token", () => {
  // `text-white` on `bg-accent*` breaks as soon as a preset declares a dark
  // label ink, so the pairing is rejected at the source level.
  const SRC = fileURLToPath(new URL("../src", import.meta.url));

  function walk(dir: string, out: string[] = []): string[] {
    for (const entry of readdirSync(dir, { withFileTypes: true })) {
      const full = join(dir, entry.name);
      if (entry.isDirectory()) walk(full, out);
      else if (/\.(vue|ts)$/.test(entry.name)) out.push(full);
    }
    return out;
  }

  test("muted text is never faded with an opacity modifier", () => {
    // `text-ink-muted/70` resolves to color-mix(… 70%, transparent), which lands
    // around 3.1:1 on a raised surface. The audit could not see those at first
    // (computed value serialises as oklab), so the rule is enforced here.
    const offenders: string[] = [];
    for (const file of walk(SRC)) {
      const source = readFileSync(file, "utf8");
      for (const m of source.matchAll(/text-ink-muted\/(\d+)/g)) {
        // `/0` is the hover-reveal trick: invisible until the group is hovered.
        if (m[1] === "0") continue;
        offenders.push(`${file.slice(SRC.length + 1)}: ${m[0]}`);
      }
    }
    expect(offenders).toEqual([]);
  });

  test("no component hardcodes a white label on an accent fill", () => {
    const offenders: string[] = [];
    const accentFill = /\bbg-accent-strong\b|\bbg-accent(?=[\s"'])/;

    for (const file of walk(SRC)) {
      const source = readFileSync(file, "utf8");
      for (const m of source.matchAll(/(?::|v-bind:)?class="([^"]*)"/g)) {
        const body = m[1]!;
        if (!accentFill.test(body)) continue;
        if (/\btext-white\b/.test(body)) {
          offenders.push(`${file.slice(SRC.length + 1)}: ${body.trim().slice(0, 80)}`);
        }
      }
    }

    expect(offenders).toEqual([]);
  });
});
