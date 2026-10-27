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
 * `--check` compares byte-for-byte. That is blunt — a single antialiased pixel
 * counts as a diff — but it catches the changes that matter (palette, radius,
 * material) without pulling in an image-diff dependency.
 */
const { chromium } = require("playwright");
const path = require("node:path");
const fs = require("node:fs");

const BASE = process.env.BASE_URL || "http://127.0.0.1:9245/";
const BASELINES = path.join(__dirname, "baselines");
const OUT = path.join(__dirname, "out");
const CHECK = process.argv.includes("--check");

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
        } else if (!fs.readFileSync(baseline).equals(buffer)) {
          diffs.push(`${name}: differs`);
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
