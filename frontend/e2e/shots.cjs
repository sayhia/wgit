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
  for (const theme of THEMES) {
    for (const accent of ACCENTS) {
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
      await page.waitForTimeout(260);

      const name = `${theme}-${accent}.png`;
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
  }

  await browser.close();

  if (CHECK) {
    if (diffs.length) {
      console.log(`visual diff against e2e/baselines (${diffs.length}):`);
      for (const line of diffs) console.log(`  ${line}`);
      console.log(`captures written to ${OUT}`);
      process.exit(1);
    }
    console.log(`visual check passed (${THEMES.length * ACCENTS.length} combinations)`);
    return;
  }
  console.log(`wrote ${THEMES.length * ACCENTS.length} baselines to ${BASELINES}`);
})();
