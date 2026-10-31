#!/usr/bin/env node
/**
 * Responsive page-width audit.
 *
 * Phone and tablet widths are the easiest place to regress layout silently:
 * a nowrap row or a fixed-width panel pushes the whole document wider than the
 * viewport, but the app still "works" on desktop. This walks every main tab
 * at phone, tablet and desktop minimum widths, then exercises a restored
 * source pane and manual commit-body resizing. Header controls are checked
 * independently of clipping ancestors, so overflow:hidden cannot mask them.
 *
 *   bun run audit:responsive
 *   BASE_URL=http://127.0.0.1:4173/ bun run audit:responsive
 */
const { chromium } = require("playwright");

const BASE = process.env.BASE_URL || "http://127.0.0.1:9245/";
const VIEWPORTS = [
  { width: 390, height: 844 },
  { width: 768, height: 900 },
  { width: 960, height: 640 },
  { width: 1280, height: 800 },
];
const TABS = [
  "工作区 & 源码变更",
  "提交图谱 & 历史",
  "分支 & 标签",
  "储藏箱 & 快照",
  "评审",
  "效能工具 & 诊断",
  "设置",
];

(async () => {
  const browser = await chromium.launch({ channel: "chrome" });
  const failures = [];
  for (const viewport of VIEWPORTS) {
    const page = await browser.newPage({ viewport });
    await page.addInitScript(() => localStorage.setItem("wgit.locale", "zh-CN"));
    await page.goto(BASE, { waitUntil: "networkidle" });
    await page.waitForTimeout(1500);
    // overflow:hidden on the app root concealed clipped header actions from
    // the old document-width check. Check the visible controls themselves.
    const headerFindings = await page.evaluate(() => [...document.querySelectorAll('header button')]
      .filter(el => el.getClientRects().length && !el.closest('[data-header-menu]'))
      .filter(el => {
        const r = el.getBoundingClientRect();
        return r.left < -1 || r.right > innerWidth + 1 || r.top < 0 || r.bottom > 49;
      }).map(el => el.getAttribute('aria-label') || el.title || el.textContent.trim()));
    if (headerFindings.length) failures.push(`${viewport.width}px header controls clipped: ${headerFindings.join(', ')}`);
    for (const tab of TABS) {
      const trigger = page.locator(`[aria-label="${tab}"]`).first();
      if (!(await trigger.count())) {
        failures.push(`${viewport.width}px ${tab}: nav trigger missing`);
        continue;
      }
      await trigger.click();
      await page.waitForTimeout(500);
      const result = await page.evaluate(() => {
        const width = window.innerWidth;
        const offenders = [];
        for (const el of document.querySelectorAll("main *")) {
          const rect = el.getBoundingClientRect();
          if (rect.width <= 0 || rect.height <= 0) continue;
          const style = getComputedStyle(el);
          if (style.display === "none" || style.visibility === "hidden") continue;
          if (rect.right <= width + 1 && rect.left >= -1) continue;
          let parent = el.parentElement;
          let clipped = false;
          while (parent) {
            const parentStyle = getComputedStyle(parent);
            if (["auto", "scroll", "hidden"].includes(parentStyle.overflowX)) {
              clipped = true;
              break;
            }
            parent = parent.parentElement;
          }
          if (!clipped) {
            offenders.push({
              tag: el.tagName,
              cls: String(el.className || "").slice(0, 80),
              left: Math.round(rect.left),
              right: Math.round(rect.right),
            });
          }
          if (offenders.length >= 3) break;
        }
        return { width, scrollWidth: document.documentElement.scrollWidth, offenders };
      });
      if (result.scrollWidth > result.width + 1 || result.offenders.length > 0) {
        failures.push(
          `${viewport.width}px ${tab}: scrollWidth=${result.scrollWidth} offenders=${JSON.stringify(result.offenders)}`
        );
      } else {
        console.log(`  ✓ ${viewport.width}px ${tab}`);
      }
    }
    await page.close();
  }

  // A restored, expanded source tree must be included in the pane budget.
  // The old width test allocated changes+diff from the entire workspace and
  // left the diff only ~270px wide at the desktop minimum.
  const workspace = await browser.newPage({ viewport: { width: 960, height: 640 } });
  await workspace.addInitScript(() => {
    localStorage.setItem('wgit.locale', 'en-US');
    localStorage.setItem('wgit.active-tab', 'workspace');
    localStorage.setItem('wgit.workspace.sourcePane.v1', 'shown');
    localStorage.setItem('wgit.workspace.leftPane.collapsed.v1', 'false');
  });
  await workspace.goto(BASE, { waitUntil: 'networkidle' });
  const paneSwitch = workspace.locator('[data-testid="ws-stacked-pane-switch"]');
  if (!(await paneSwitch.isVisible())) failures.push('960px source tree: changes/diff must stack when the tree consumes their width');
  else {
    await paneSwitch.locator('button').last().click();
    const dimensions = await workspace.locator('#wgit-main main').evaluate(el => ({ width: el.clientWidth, scrollWidth: el.scrollWidth }));
    if (dimensions.width < 500 || dimensions.scrollWidth > dimensions.width + 1) failures.push(`960px source tree: unusable diff column ${JSON.stringify(dimensions)}`);
  }
  const body = workspace.locator('[data-testid="composer-pin-bar"] textarea');
  await body.evaluate(el => { el.style.height = '1000px'; });
  const composer = await workspace.locator('[data-testid="composer-pin-bar"]').boundingBox();
  if (!composer || composer.y < 250 || composer.y + composer.height > 641) failures.push('resizing commit body must leave space for changes/diff and keep commit controls visible');
  await workspace.setViewportSize({ width: 390, height: 844 });
  const backdrop = workspace.locator('[data-testid="ws-source-pane-backdrop"]');
  await backdrop.waitFor({ state: 'visible' });
  const bounds = await backdrop.boundingBox();
  await backdrop.click({ position: { x: bounds.width - 8, y: 16 } });
  if (await backdrop.isVisible()) failures.push('390px source tree overlay cannot be dismissed');
  const narrowSwitch = await paneSwitch.boundingBox();
  if (!narrowSwitch || narrowSwitch.x < 52 || narrowSwitch.x + narrowSwitch.width > 391) failures.push('390px workspace must keep changes/diff switching reachable');
  await workspace.close();
  await browser.close();
  if (failures.length) {
    console.error("\nResponsive overflow findings:");
    for (const failure of failures) console.error(`  ✗ ${failure}`);
    process.exit(1);
  }
  console.log("responsive audit passed: 390/768/960/1280 widths, source pane and commit resizing");
})().catch((error) => {
  console.error(error);
  process.exit(1);
});
