#!/usr/bin/env node
/**
 * Visual baselines.
 *
 * Captures the workspace in every theme × accent combination so a palette or
 * material change can be diffed instead of eyeballed. Deliberately *not* wired
 * into CI yet: without pixel comparison, all a CI run would prove is that the
 * screenshot step did not crash.
 *
 *   bun run shots                 # write e2e/baselines/*.png
 *   bun run shots -- --check      # capture to e2e/out/ and list which files differ
 *
 * `--check` compares pixels, tolerating a handful of stray ones. Byte equality
 * was the first implementation and it made the gate random: a live app renders a
 * couple of antialiased pixels differently between two runs of the same page
 * (measured: 2 px), which reported "differs" on files nobody had touched. The
 * allowance is small enough that a palette, radius or material change — the
 * things this gate exists for — still trips it by thousands of pixels.
 *
 * Beyond the theme × accent matrix, *state* captures pin down UI states a
 * plain page load cannot reach (the pinned commit bar, its collapsed summary,
 * the narrow-window left-pane rail, the diff overflow menu). Each one drives
 * the UI through data-testid handles and asserts the target state exists
 * before shooting, so a renamed handle or a broken toggle fails the run
 * instead of silently photographing the wrong thing.
 *
 * `--only=<name|group,...>` limits the run to those captures (groups:
 * `themes` for the 12 theme × accent shots, `states` for the state shots) —
 * how the state baselines are refreshed without rewriting the locked
 * theme × accent PNGs. Names are file stems: `dark-blue`,
 * `workspace-composer-collapsed`, ...
 */
const { chromium } = require("playwright");
const { PNG } = require("pngjs");
const path = require("node:path");
const fs = require("node:fs");

const BASE = process.env.BASE_URL || "http://127.0.0.1:9245/";
const BASELINES = path.join(__dirname, "baselines");
const OUT = path.join(__dirname, "out");
const CHECK = process.argv.includes("--check");
// Stray-pixel allowance per capture (see the header comment).
const PIXEL_TOLERANCE = 40;
const ONLY = (() => {
  const arg = process.argv.find((a) => a.startsWith("--only="));
  if (!arg) return null;
  return arg
    .slice("--only=".length)
    .split(",")
    .map((s) => s.trim())
    .filter(Boolean);
})();

/** Pixels that differ between two PNG buffers, or -1 when the sizes differ. */
function differingPixels(a, b) {
  const left = PNG.sync.read(a);
  const right = PNG.sync.read(b);
  if (left.width !== right.width || left.height !== right.height) return -1;
  let count = 0;
  for (let i = 0; i < left.data.length; i += 4) {
    if (
      left.data[i] !== right.data[i] ||
      left.data[i + 1] !== right.data[i + 1] ||
      left.data[i + 2] !== right.data[i + 2]
    ) {
      count++;
    }
  }
  return count;
}

const THEMES = ["dark", "light"];
const ACCENTS = ["blue", "teal", "green", "amber", "rose", "violet"];

/** Same palette switch the theme × accent loop performs, reused by states. */
async function applyTheme(page, theme, accent) {
  await page.evaluate(
    ([t, a]) => {
      const root = document.documentElement;
      root.dataset.theme = t;
      root.classList.toggle("light", t === "light");
      root.classList.toggle("dark", t === "dark");
      if (a === "blue") delete root.dataset.accent;
      else root.dataset.accent = a;
    },
    [theme, accent]
  );
}

const testid = (page, id) => page.locator(`[data-testid="${id}"]`);
async function expectVisible(page, id, timeout = 5000) {
  await testid(page, id).waitFor({ state: "visible", timeout });
}

/**
 * State captures beyond a plain page load. `arrange` drives the UI into the
 * state and asserts it was reached; `cleanup` undoes whatever would leak into
 * a later capture. All states shoot the default dark/blue palette: the
 * theme × accent matrix already covers palette drift.
 */
const STATE_CAPTURES = [
  {
    name: "workspace-composer-pinned",
    async arrange(page) {
      await page.setViewportSize({ width: 1440, height: 900 });
      await applyTheme(page, "dark", "blue");
      // Default state: the pinned bar is docked with its composer expanded.
      await expectVisible(page, "composer-pin-bar");
      await expectVisible(page, "composer-pin-toggle");
      // The expandable panel carries an id, not a test id.
      await page.locator("#wgit-composer-pin-panel").waitFor({ state: "visible", timeout: 5000 });
      // If a previous capture left it collapsed, toggle back open first.
      if (await testid(page, "composer-pin-subject-preview").count()) {
        await testid(page, "composer-pin-toggle").click();
      }
      await page.locator("#wgit-composer-pin-panel").waitFor({ state: "visible", timeout: 5000 });
    },
  },
  {
    name: "diff-overflow-open",
    async arrange(page) {
      await page.setViewportSize({ width: 1440, height: 900 });
      await applyTheme(page, "dark", "blue");
      await expectVisible(page, "diff-more-btn");
      await testid(page, "diff-more-btn").click();
      await expectVisible(page, "diff-more-menu");
    },
    async cleanup(page) {
      await testid(page, "diff-more-btn").click();
      await testid(page, "diff-more-menu").waitFor({ state: "detached", timeout: 5000 });
    },
  },
  {
    name: "workspace-composer-collapsed",
    async arrange(page) {
      await page.setViewportSize({ width: 1440, height: 900 });
      await applyTheme(page, "dark", "blue");
      await expectVisible(page, "composer-pin-toggle");
      await testid(page, "composer-pin-toggle").click();
      // Collapsed = one summary line carrying the subject preview.
      await expectVisible(page, "composer-pin-subject-preview");
    },
    async cleanup(page) {
      await testid(page, "composer-pin-toggle").click();
      await testid(page, "composer-pin-subject-preview").waitFor({ state: "detached", timeout: 5000 });
    },
  },
  {
    name: "workspace-left-pane-collapsed",
    async arrange(page) {
      await page.setViewportSize({ width: 1100, height: 900 });
      await applyTheme(page, "dark", "blue");
      // The tree starts hidden by default; the narrow-window collapse control
      // lives on its divider, so show the pane first.
      await expectVisible(page, "ws-source-pane-toggle");
      await testid(page, "ws-source-pane-toggle").click();
      await expectVisible(page, "ws-left-pane-collapse");
      await testid(page, "ws-left-pane-collapse").click();
      await expectVisible(page, "ws-left-pane-rail");
      const width = await testid(page, "ws-left-pane-rail").evaluate(
        (el) => el.getBoundingClientRect().width
      );
      if (Math.round(width) !== 36) {
        throw new Error(`left pane rail is ${width}px wide, expected 36`);
      }
    },
    async cleanup(page) {
      await testid(page, "ws-left-pane-expand").click();
      await testid(page, "ws-left-pane-rail").waitFor({ state: "detached", timeout: 5000 });
    },
  },
];

const GROUP_ALIASES = {
  themes: THEMES.flatMap((t) => ACCENTS.map((a) => `${t}-${a}`)),
  states: STATE_CAPTURES.map((c) => c.name),
};

function wants(name) {
  if (!ONLY) return true;
  if (ONLY.includes(name)) return true;
  return ONLY.some((entry) => (GROUP_ALIASES[entry] || []).includes(name));
}

/** Screenshot the current page state and write/compare it as `name`. */
async function capture(page, name, dir, diffs) {
  const buffer = await page.screenshot();
  const target = path.join(dir, name);
  if (CHECK) {
    const baseline = path.join(BASELINES, name);
    if (!fs.existsSync(baseline)) {
      diffs.push(`${name}: no baseline`);
    } else {
      const changed = differingPixels(fs.readFileSync(baseline), buffer);
      if (changed < 0) diffs.push(`${name}: size changed`);
      else if (changed > PIXEL_TOLERANCE) diffs.push(`${name}: ${changed} px differ`);
    }
  }
  fs.writeFileSync(target, buffer);
}

(async () => {
  const dir = CHECK ? OUT : BASELINES;
  fs.mkdirSync(dir, { recursive: true });

  const browser = await chromium.launch({ channel: "chrome" });
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 }, deviceScaleFactor: 1 });
  await page.addInitScript(() => localStorage.setItem("wgit.locale", "en-US"));
  await page.goto(BASE, { waitUntil: "networkidle" });
  await page.waitForTimeout(1600);

  // Freeze motion: a transition mid-flight makes two captures of the same UI
  // differ for no reason.
  await page.addStyleTag({
    content: "*, *::before, *::after { transition: none !important; animation: none !important; }",
  });

  const diffs = [];
  let ran = 0;

  for (const theme of THEMES) {
    for (const accent of ACCENTS) {
      if (!wants(`${theme}-${accent}`)) continue;
      await applyTheme(page, theme, accent);
      await page.waitForTimeout(260);
      await capture(page, `${theme}-${accent}.png`, dir, diffs);
      ran++;
    }
  }

  for (const cap of STATE_CAPTURES) {
    if (!wants(cap.name)) continue;
    await cap.arrange(page);
    await page.waitForTimeout(300);
    await capture(page, `${cap.name}.png`, dir, diffs);
    if (cap.cleanup) await cap.cleanup(page);
    await page.waitForTimeout(150);
    ran++;
  }

  await browser.close();

  if (CHECK) {
    if (diffs.length) {
      console.log(`visual diff against e2e/baselines (${diffs.length}):`);
      for (const line of diffs) console.log(`  ${line}`);
      console.log(`captures written to ${OUT}`);
      process.exit(1);
    }
    console.log(`visual check passed (${ran} captures)`);
    return;
  }
  console.log(`wrote ${ran} baselines to ${BASELINES}`);
})();
