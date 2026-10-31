import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

/**
 * Semantic spacing tokens (roadmap §2.1 item 5).
 *
 * WorkspaceView carried ~50 raw `gap-1`-style utilities with no vocabulary
 * for *why* a gap is 4px vs 8px. styles/spacing.css introduces four intent
 * names under one hard constraint: **zero visual change**. Every token must
 * alias an existing Tailwind scale step exactly — the pass names the current
 * rhythm, it must never retune it (no 6px/10px "improvements" smuggled in
 * under a semantic name).
 *
 * These tests pin that contract:
 *   1. the four tokens exist with the exact 4/8/12/16px values;
 *   2. each token equals the Tailwind step it replaces (gap-1/2/3/4);
 *   3. no extra --space-* token invents an off-scale value;
 *   4. the entry stylesheet actually imports the token file;
 *   5. the WorkspaceView pilot really uses the vocabulary (≥10 sites);
 *   6. the wave-2 rollout (History/Branches/Review/Settings) keeps its
 *      per-file semantic-use floor, so the vocabulary cannot silently
 *      regress out of a promoted view.
 */

const ROOT = fileURLToPath(new URL("..", import.meta.url));
const read = (rel: string) => readFileSync(join(ROOT, rel), "utf8");

const spacingCss = read("src/styles/spacing.css");
const entryCss = read("src/style.css");
const wsSrc = read("src/components/WorkspaceView.vue");

/** Strips CSS comments so assertions see declarations, not prose. */
function stripComments(css: string): string {
  return css.replace(/\/\*[\s\S]*?\*\//g, "");
}

/** Every `--space-*: <value>` declaration in the file. */
function spaceTokens(css: string): Map<string, string> {
  const tokens = new Map<string, string>();
  for (const m of stripComments(css).matchAll(/--space-[a-z-]+\s*:\s*([^;]+);/g)) {
    tokens.set(m[0]!.slice(0, m[0]!.indexOf(":")).trim(), m[1]!.trim());
  }
  return tokens;
}

/** Token → the Tailwind scale step it must alias, in px. */
const TAILWIND_SCALE: Record<string, { utility: string; px: number }> = {
  "--space-tight": { utility: "gap-1", px: 4 },
  "--space-base": { utility: "gap-2", px: 8 },
  "--space-loose": { utility: "gap-3", px: 12 },
  "--space-wide": { utility: "gap-4", px: 16 },
};

describe("spacing tokens", () => {
  test("defines the four tokens with exact 4/8/12/16px values", () => {
    const tokens = spaceTokens(spacingCss);
    for (const [name, spec] of Object.entries(TAILWIND_SCALE)) {
      expect(tokens.has(name)).toBe(true);
      expect(tokens.get(name)).toBe(`${spec.px}px`);
    }
    expect(tokens.size).toBe(4);
  });

  test("zero visual change: every token equals the Tailwind step it replaces", () => {
    const tokens = spaceTokens(spacingCss);
    for (const [name, spec] of Object.entries(TAILWIND_SCALE)) {
      // Exact alias, not a close value: 4px↔gap-1, 8px↔gap-2, 12px↔gap-3, 16px↔gap-4.
      expect(`${name}=${tokens.get(name)}`).toBe(`${name}=${spec.px}px`);
      expect(spec.px % 4).toBe(0); // sanity: the aliased step is on the Tailwind 4px grid
    }
  });

  test("invents no off-scale value (no 6px/10px under a semantic name)", () => {
    const onScale = new Set(Object.values(TAILWIND_SCALE).map((s) => `${s.px}px`));
    for (const [name, value] of spaceTokens(spacingCss)) {
      expect(onScale.has(value)).toBe(true);
      expect(TAILWIND_SCALE[name]).toBeDefined();
    }
  });

  test("registers the tokens in the Tailwind spacing namespace via var() indirection", () => {
    const theme = stripComments(spacingCss).match(/@theme\s+inline\s*\{([\s\S]*?)\}/);
    expect(theme).not.toBeNull();
    for (const key of ["tight", "base", "loose", "wide"]) {
      expect(theme![1]).toContain(`--spacing-${key}: var(--space-${key})`);
    }
  });

  test("the entry stylesheet imports the spacing tokens", () => {
    expect(entryCss).toContain('@import "./styles/spacing.css";');
  });
});

describe("WorkspaceView spacing pilot", () => {
  /** Semantic utilities counted at use sites: gap-*, p-*, … on any axis. */
  const semanticUses = [...wsSrc.matchAll(/(?:gap-[xy]-)?(?:gap|p|px|py|pt|pb|m|mx|my|space-[xy])-(?:tight|base|loose|wide)\b/g)];

  test("uses the semantic vocabulary at 10+ sites", () => {
    expect(semanticUses.length).toBeGreaterThanOrEqual(10);
  });

  test("every semantic use resolves to an on-scale token (conservative guard)", () => {
    for (const m of semanticUses) {
      const scale = m[0]!.match(/-(tight|base|loose|wide)$/)![1]!;
      expect(TAILWIND_SCALE[`--space-${scale}`]).toBeDefined();
    }
  });

  test("the pinned commit bar (composer-pin-bar) opts in", () => {
    const pinBar = wsSrc.slice(wsSrc.indexOf('data-testid="composer-pin-bar"'));
    expect(pinBar.slice(0, 400)).toContain("gap-base");
  });
});

describe("spacing rollout wave 2: History/Branches/Review/Settings", () => {
  /**
   * Second promotion slice of the semantic spacing vocabulary (roadmap §2.1
   * item 5, wave 2): same zero-visual-change rule as the WorkspaceView pilot —
   * each replacement aliases its Tailwind step exactly (gap-1↔tight,
   * gap-2↔base, gap-3↔loose, gap-4↔x-wide), off-scale micro-gaps
   * (gap-0.5/gap-1.5/gap-2.5) stay raw, and literals pinned by
   * tests/shellLayout.test.ts keep their class strings (e.g. the HistoryView
   * `@min-[900px]` traversal cluster, the BranchesView 5xl toolbar row and the
   * ReviewView `gap-x-3 gap-y-1.5` header wrap).
   *
   * The floor per file is the count this wave actually landed; going below it
   * means semantic names were edited back to raw pixels.
   */

  /** Same counting rule as the WorkspaceView pilot above, per file. */
  const semanticUsesIn = (rel: string) =>
    [...read(rel).matchAll(/(?:gap-[xy]-)?(?:gap|p|px|py|pt|pb|m|mx|my|space-[xy])-(?:tight|base|loose|wide)\b/g)].length;

  test.each([
    ["src/components/HistoryView.vue", 14],
    ["src/components/BranchesView.vue", 16],
    ["src/components/ReviewView.vue", 8],
    ["src/components/settings/SettingsNav.vue", 1],
    ["src/components/settings/SettingsSearch.vue", 1],
  ])("%s keeps >=%i semantic spacing uses", (file, floor) => {
    expect(semanticUsesIn(file)).toBeGreaterThanOrEqual(floor);
  });
});
