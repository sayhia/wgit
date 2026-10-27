/**
 * WCAG 2.1 AA text-contrast audit across every theme x accent preset.
 *
 * The palette is entirely CSS-variable driven (`data-theme` + `data-accent`), so
 * flipping those attributes re-paints the app without a reload. For every
 * visible text node we resolve the *effective* foreground and background
 * (compositing translucent layers down to the canvas) and compare against
 * 4.5:1 (normal) / 3:1 (large text). The measurement itself lives in
 * `audit-lib.cjs` so the deep pass uses the exact same rules.
 */
const { chromium } = require("playwright");
const { AUDIT, freezeMotion, applyTheme } = require("./audit-lib.cjs");

const VIEWS = [
  { name: "workspace", label: "Workspace & Changes" },
  { name: "history", label: "Commit Graph & History" },
  { name: "branches", label: "Branches & Tags" },
  { name: "review", label: "Change Review" },
  { name: "tools", label: "Tools & Diagnostics" },
  { name: "settings", label: "Settings" },
];

const THEMES = ["dark", "light"];
const ACCENTS = ["blue", "violet", "teal", "green", "amber", "rose"];

(async () => {
  const browser = await chromium.launch({ channel: "chrome" });
  const page = await browser.newPage({ viewport: { width: 1500, height: 1000 } });
  const errors = [];
  page.on("pageerror", (e) => errors.push(e.message));

  // Pin the locale so the dock labels below match.
  await page.addInitScript(() => localStorage.setItem("wgit.locale", "en-US"));
  await page.goto("http://127.0.0.1:9245/", { waitUntil: "networkidle" });
  await page.waitForTimeout(1600);
  await freezeMotion(page);

  const results = [];

  for (const theme of THEMES) {
    for (const accent of ACCENTS) {
      await applyTheme(page, theme, accent);

      for (const view of VIEWS) {
        const btn = page.getByRole("button", { name: view.label, exact: true }).first();
        if (await btn.count()) {
          await btn.click({ timeout: 3000 }).catch(() => {});
          await page.waitForTimeout(500);
        }
        const found = await page.evaluate(AUDIT);
        if (found.length) results.push({ theme, accent, view: view.name, items: found });
      }
    }
  }

  console.log(JSON.stringify({ errors, results }, null, 1));
  await browser.close();

  // A green run that found violations is worse than no run at all.
  const offenders = results.reduce((sum, bucket) => sum + bucket.items.length, 0);
  process.exit(offenders > 0 || errors.length > 0 ? 1 : 0);
})();
