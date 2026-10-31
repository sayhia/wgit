#!/usr/bin/env node
/**
 * Workspace reading controls must stay reachable with long source lines.
 * Requires Vite dev: fixtures only replace this browser page's service methods.
 * No real repository operations are performed.
 */
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const { chromium } = require("playwright");

const BASE = process.env.BASE_URL || "http://127.0.0.1:9245/";
const OUT = path.join(__dirname, "out", "workspace-layout");
const CASES = [
  { width: 390, height: 844 },
  { width: 960, height: 640 },
  { width: 1280, height: 720 },
];

async function assertFileControls(page, label) {
  const result = await page.locator('[data-testid="ws-file-toolbar"]').evaluate((element) => {
    const bounds = element.getBoundingClientRect();
    return {
      pathCount: element.querySelectorAll('[data-testid="ws-file-path"]').length,
      outside: [...element.querySelectorAll("button")].filter((button) => {
        const rect = button.getBoundingClientRect();
        return rect.left < bounds.left || rect.right > bounds.right + 1 ||
          rect.bottom > bounds.bottom + 1 || rect.right > innerWidth + 1;
      }).map((button) => button.getAttribute("aria-label") || button.textContent.trim()),
      obscured: [...element.querySelectorAll("button")].filter((button) => {
        const rect = button.getBoundingClientRect();
        return !button.contains(document.elementFromPoint(rect.x + rect.width / 2, rect.y + rect.height / 2));
      }).map((button) => button.getAttribute("aria-label") || button.textContent.trim()),
    };
  });
  assert.equal(result.pathCount, 1, `${label}: file identity must appear once`);
  assert.deepEqual(result.outside, [], `${label}: file actions leave the panel`);
  assert.deepEqual(result.obscured, [], `${label}: file actions are covered`);
}

async function assertMiniMap(page, label) {
  const viewport = await page.locator('[data-testid="ws-diff-scroll"]').boundingBox();
  const map = await page.locator('[data-testid="diff-minimap-gutter"]').boundingBox();
  assert.ok(map && viewport, `${label}: minimap is visible`);
  assert.ok(map.x >= viewport.x && map.x + map.width <= viewport.x + viewport.width,
    `${label}: minimap leaves the diff width`);
  assert.ok(map.y >= viewport.y && map.y + map.height <= viewport.y + viewport.height,
    `${label}: minimap overlaps file actions or commit fields`);
}

async function assertModalMiniMap(page, label) {
  const result = await page.locator('[role="dialog"] [data-testid="diff-minimap-gutter"]').evaluate((element) => {
    let parent = element.parentElement;
    while (parent && !["auto", "scroll"].includes(getComputedStyle(parent).overflowY)) parent = parent.parentElement;
    const viewport = parent.getBoundingClientRect();
    const map = element.getBoundingClientRect();
    return {
      contained: map.left >= viewport.left && map.right <= viewport.right &&
        map.top >= viewport.top && map.bottom <= viewport.bottom,
      reachable: element.contains(document.elementFromPoint(map.x + map.width / 2, map.y + map.height / 2)),
    };
  });
  assert.ok(result.contained, `${label}: modal containing block offsets the minimap`);
  assert.ok(result.reachable, `${label}: modal minimap is clipped or covered`);
}

(async () => {
  fs.mkdirSync(OUT, { recursive: true });
  const browser = await chromium.launch({ channel: "chrome" });
  try {
    for (const viewport of CASES) {
      for (const locale of ["zh-CN", "en-US"]) {
        const page = await browser.newPage({ viewport });
        const label = `${viewport.width}-${locale}`;
        await page.addInitScript((value) => {
          localStorage.setItem("wgit.locale", value);
          localStorage.setItem("wgit.active-tab", "workspace");
        }, locale);
        await page.goto(BASE, { waitUntil: "networkidle" });
        await page.evaluate(async () => {
          const { gitService } = await import("/src/services/gitService.ts");
          gitService.getWorkingDiffAdvanced = async () => [
            "diff --git a/vite.config.ts b/vite.config.ts",
            "index 0000..1111 100644", "--- a/vite.config.ts", "+++ b/vite.config.ts",
            "@@ -1,80 +1,80 @@",
            ...Array.from({ length: 80 }, (_, index) =>
              `-${index}: ${"old long source; ".repeat(50)}\n+${index}: ${"new long source; ".repeat(50)}`), "",
          ].join("\n");
        });
        const paneSwitch = page.locator('[data-testid="ws-stacked-pane-switch"]');
        if (await paneSwitch.isVisible()) await paneSwitch.locator("button").first().click();
        await page.getByText("vite.config.ts", { exact: true }).last().click();
        await page.locator('[data-testid="diff-minimap-gutter"]').waitFor({ state: "visible" });
        await assertFileControls(page, label);
        await assertMiniMap(page, label);
        const hunkActions = await page.locator('[data-testid="stage-hunk-btn"], [data-testid="discard-hunk-btn"]').evaluateAll((buttons) => buttons.map((button) => {
          const rect = button.getBoundingClientRect();
          return rect.left >= 52 && rect.right <= innerWidth &&
            button.contains(document.elementFromPoint(rect.x + rect.width / 2, rect.y + rect.height / 2));
        }));
        assert.ok(hunkActions.length === 2 && hunkActions.every(Boolean), `${label}: hunk actions leave the reading viewport`);
        const toolbar = page.locator('[data-testid="ws-file-toolbar"]');
        const initial = await toolbar.boundingBox();
        const scroller = page.locator('[data-testid="ws-diff-scroll"]');
        const scroll = await scroller.evaluate((element) => {
          element.scrollLeft = 1000;
          return { width: element.clientWidth, scrollWidth: element.scrollWidth };
        });
        assert.ok(scroll.scrollWidth > scroll.width + 1000, `${label}: long-line fixture is active`);
        await page.waitForTimeout(50);
        assert.deepEqual(await toolbar.boundingBox(), initial, `${label}: code scrolling moves file actions`);
        await assertFileControls(page, label);
        await assertMiniMap(page, label);
        const map = page.locator('[data-testid="diff-minimap-gutter"]');
        const mapBounds = await map.boundingBox();
        await map.click({ position: { x: mapBounds.width / 2, y: mapBounds.height * 0.85 } });
        assert.ok(await scroller.evaluate((element) => element.scrollTop > 0), `${label}: minimap navigation works`);
        await page.locator('[data-testid="composer-pin-toggle"]').click();
        await page.waitForTimeout(80);
        await assertMiniMap(page, `${label} collapsed composer`);
        await page.screenshot({ path: path.join(OUT, `workspace-${label}.png`) });
        console.log(`  ✓ ${label}: fixed file actions, long-line scrolling and contained minimap`);
        await page.close();
      }
    }

    // A blurred modal creates its own fixed-position containing block. The
    // minimap must convert viewport coordinates into that block's coordinates.
    const modalPage = await browser.newPage({ viewport: { width: 960, height: 640 } });
    await modalPage.addInitScript(() => localStorage.setItem("wgit.locale", "en-US"));
    await modalPage.goto(BASE, { waitUntil: "networkidle" });
    await modalPage.evaluate(async () => {
      const { gitService } = await import("/src/services/gitService.ts");
      const diff = ["diff --git a/layout.ts b/layout.ts", "--- a/layout.ts", "+++ b/layout.ts",
        "@@ -1,100 +1,100 @@", ...Array.from({ length: 100 }, (_, index) => `-old ${index}\n+new ${index}`)].join("\n");
      gitService.compareBranches = async () => ({ baseBranch: "main", targetBranch: "feature/studio-ui",
        aheadCount: 1, behindCount: 0, aheadCommits: [], behindCommits: [] });
      const details = { refA: "main", refB: "feature/studio-ui", additions: 100, deletions: 100,
        files: [{ path: "layout.ts", additions: 100, deletions: 100, status: "M" }], diff };
      gitService.getDiffBetween = async () => details;
      gitService.getMergeBaseDiff = async () => details;
      gitService.getFileDiffBetween = async () => diff;
    });
    await modalPage.getByLabel("Branches & Tags", { exact: true }).click();
    await modalPage.getByRole("button", { name: "Compare refs", exact: true }).click();
    await modalPage.getByRole("button", { name: "Recompute", exact: true }).click();
    await modalPage.getByRole("button", { name: "Show diff details", exact: true }).click();
    const fileDiff = modalPage.getByTitle("Show this file's diff between the two versions", { exact: true });
    await fileDiff.click();
    const modalMap = modalPage.locator('[role="dialog"] [data-testid="diff-minimap-gutter"]');
    await modalMap.waitFor({ state: "visible" });
    await modalPage.waitForTimeout(80);
    await assertModalMiniMap(modalPage, "960px file diff");
    const modalMapBounds = await modalMap.boundingBox();
    await modalMap.click({ position: { x: modalMapBounds.width / 2, y: modalMapBounds.height * 0.85 } });
    await modalPage.waitForTimeout(80);
    await assertModalMiniMap(modalPage, "scrolled file diff");
    await modalPage.setViewportSize({ width: 768, height: 540 });
    await modalPage.waitForTimeout(80);
    await assertModalMiniMap(modalPage, "resized file diff");
    await modalPage.keyboard.press("Escape");
    await fileDiff.click();
    await modalMap.waitFor({ state: "visible" });
    await modalPage.waitForTimeout(80);
    await assertModalMiniMap(modalPage, "reopened file diff");
    await modalPage.screenshot({ path: path.join(OUT, "modal-file-diff-768.png") });
    await modalPage.close();
    console.log("  ✓ embedded file diff: minimap survives modal offset, scroll, resize and reopen");

    const page = await browser.newPage({ viewport: { width: 960, height: 400 } });
    await page.addInitScript(() => localStorage.setItem("wgit.locale", "zh-CN"));
    await page.goto(BASE, { waitUntil: "networkidle" });
    await page.evaluate(() => {
      for (const kind of ["fetch", "pull", "lfs", "maintenance"]) {
        window.dispatchEvent(new CustomEvent("wgit:activity-task", {
          detail: { id: `layout-${kind}`, kind, status: "running", startedAt: Date.now() },
        }));
      }
    });
    const settings = page.locator('aside button[aria-label="设置"]');
    const settingsBounds = await settings.boundingBox();
    assert.ok(settingsBounds && settingsBounds.y + settingsBounds.height <= 400,
      "short window: background tasks push settings out of view");
    const tools = page.locator('aside button[aria-label="效能工具 & 诊断"]');
    await tools.scrollIntoViewIfNeeded();
    const nav = await page.locator('[data-testid="activity-dock-nav"]').boundingBox();
    const toolBounds = await tools.boundingBox();
    assert.ok(toolBounds.y >= nav.y && toolBounds.y + toolBounds.height <= nav.y + nav.height + 1,
      "short window: main navigation must remain reachable by scrolling");
    await page.screenshot({ path: path.join(OUT, "dock-960x400-tasks.png") });
    await page.close();
    console.log("  ✓ 960×400: navigation scrolls and settings stays visible with four tasks");
  } finally {
    await browser.close();
  }
  console.log("workspace layout audit passed");
})().catch((error) => {
  console.error(error);
  process.exit(1);
});
