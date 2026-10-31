#!/usr/bin/env node
/**
 * Settings navigation and tools header layout regression checks.
 * Run against the fixture-backed Vite server:
 *   node e2e/settings-layout.cjs
 *   BASE_URL=http://127.0.0.1:4173/ node e2e/settings-layout.cjs
 */
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const { chromium } = require("playwright");

const BASE = process.env.BASE_URL || "http://127.0.0.1:9245/";

// Find the actual scrolling ancestor, independent of the section's markup.
const SCROLL_ANCESTOR = 'xpath=ancestor::*[contains(concat(" ", normalize-space(@class), " "), " overflow-y-auto ")][1]';

async function openEnglishPage(browser, viewport) {
  const page = await browser.newPage({ viewport });
  page.setDefaultTimeout(10000);
  await page.addInitScript(() => {
    localStorage.setItem("wgit.locale", "en-US");
    localStorage.setItem("wgit.active-tab", "workspace");
  });
  await page.goto(BASE, { waitUntil: "networkidle" });
  return page;
}

async function settingsNavigation(browser) {
  const page = await openEnglishPage(browser, { width: 960, height: 640 });
  try {
    await page.getByRole("button", { name: "Settings", exact: true }).click();
    const search = page.getByRole("textbox", { name: "Search settings…", exact: true });
    await search.fill("i");
    const rail = page.getByTestId("settings-navigation-scroll");
    const geometry = await rail.evaluate((el) => ({
      height: el.clientHeight,
      scrollHeight: el.scrollHeight,
      overflow: getComputedStyle(el).overflowY,
    }));
    assert(geometry.scrollHeight > geometry.height + 40, "search must exercise a navigation list taller than the pane");
    assert.equal(geometry.overflow, "auto", "overflowing settings navigation must be scrollable");

    // The last category used to lie below the window in a clipped, non-scrolling
    // rail. Clicking must scroll it into view and open the requested section.
    const about = page.locator('[data-settings-leaf="about"]');
    await about.click();
    await page.getByRole("heading", { name: "About wgit", exact: true }).waitFor();
    assert(await rail.evaluate((el) => el.scrollTop > 0), "last category must be reached by scrolling the rail");
    assert(await about.evaluate((el) => {
      const r = el.getBoundingClientRect();
      const hit = document.elementFromPoint(r.left + r.width / 2, r.top + r.height / 2);
      return r.top >= 0 && r.bottom <= innerHeight && hit && el.contains(hit);
    }), "last settings category must be visible and hit-testable");
    const searchBox = await search.boundingBox();
    const sidebarBox = await page.getByTestId("settings-sidebar").boundingBox();
    assert(searchBox && sidebarBox && searchBox.y >= sidebarBox.y && searchBox.y + searchBox.height <= sidebarBox.y + sidebarBox.height,
      "selecting a low category must keep settings search visible");

    await search.fill("i");
    const results = page.getByTestId("settings-search-results");
    for (let index = 0; index < 5; index++) await search.press("ArrowDown");
    const selectedResult = await results.locator('[data-search-selected="true"]').boundingBox();
    const resultsBox = await results.boundingBox();
    assert(selectedResult && resultsBox && selectedResult.y >= resultsBox.y - 1 && selectedResult.y + selectedResult.height <= resultsBox.y + resultsBox.height + 1,
      "keyboard search selection must scroll into the bounded result list");

    await search.fill("");
    await page.locator('[data-settings-leaf="appearance"]').click();
    const appearance = page.getByRole("heading", { name: "Appearance", exact: true });
    const panel = await appearance.locator(SCROLL_ANCESTOR).elementHandle();
    assert(panel, "settings content must have a scrolling panel");
    await panel.evaluate((el) => { el.scrollTop = el.scrollHeight; });
    assert(await panel.evaluate((el) => el.scrollTop > 0), "appearance panel must be scrolled before testing a category change");
    await page.locator('[data-settings-leaf="ai-connection"]').click();
    await page.getByRole("heading", { name: "Connections", exact: true }).waitFor();
    assert.equal(await panel.evaluate((el) => el.scrollTop), 0, "a newly selected settings section must start at the top");
    console.log("  ✓ 960×640 English settings: search rail scrolls, final category reachable, new section starts at top");
  } finally {
    await page.close();
  }
}

async function toolsHeader(browser, width) {
  const page = await openEnglishPage(browser, { width, height: 640 });
  try {
    await page.getByRole("button", { name: "Tools & Diagnostics", exact: true }).click();
    const more = page.getByTestId("tools-more-btn");
    const geometry = await more.evaluate((el) => {
      const strip = el.closest('[role="tablist"]').parentElement;
      const header = strip.parentElement;
      return {
        headerWidth: header.clientWidth,
        headerScrollWidth: header.scrollWidth,
        stripWidth: strip.clientWidth,
        stripScrollWidth: strip.scrollWidth,
      };
    });
    assert(geometry.headerScrollWidth <= geometry.headerWidth + 1, "tool actions must fit inside the pane");
    assert(geometry.stripWidth >= 150, "tool actions must leave a usable tab strip");
    if (width >= 960) {
      assert(geometry.stripScrollWidth <= geometry.stripWidth + 1, "all primary tool tabs must fit at desktop minimum");
    }
    for (const label of ["Run Sandbox CI", "Air-Gapped Delta Pack Hub", "Refresh"]) {
      const button = page.getByRole("button", { name: label, exact: true });
      assert(await button.isVisible(), `${label} must keep its accessible name when its text is collapsed`);
      const box = await button.boundingBox();
      assert(box && box.x >= 0 && box.x + box.width <= width + 1, `${label} must stay within the viewport`);
    }
    // Narrow screens may horizontally scroll tabs, but More must remain usable.
    await more.click();
    const menu = page.getByTestId("tools-more-menu");
    const box = await menu.boundingBox();
    assert(box && box.x >= 0 && box.y >= 0 && box.x + box.width <= width + 1 && box.y + box.height <= 641,
      "the tools menu must open entirely inside the viewport");
    assert(await menu.evaluate((el) => {
      const r = el.getBoundingClientRect();
      const hit = document.elementFromPoint(r.left + r.width / 2, r.top + r.height / 2);
      return hit && el.contains(hit);
    }), "the tools menu must be hit-testable");
    console.log(`  ✓ ${width}px English tools: visible actions, usable tab strip, reachable More menu`);
  } finally {
    await page.close();
  }
}

async function everySettingsPage(browser) {
  const out = path.join(__dirname, "out", "settings-refactor");
  fs.mkdirSync(out, { recursive: true });
  for (const width of [390, 960, 1280]) {
    for (const locale of ["zh-CN", "en-US"]) {
      const page = await browser.newPage({ viewport: { width, height: width === 390 ? 844 : 640 } });
      const errors = [];
      page.on("pageerror", error => errors.push(error.message));
      await page.addInitScript(value => {
        localStorage.setItem("wgit.locale", value);
        localStorage.setItem("wgit.active-tab", "workspace");
      }, locale);
      try {
        await page.goto(BASE, { waitUntil: "networkidle" });
        await page.getByRole("button", { name: locale === "zh-CN" ? "设置" : "Settings", exact: true }).click();
        await page.locator("[data-settings-nav]").waitFor({ state: "visible" });
        const leaves = await page.locator("[data-settings-leaf]").evaluateAll(elements => elements.map(el => el.getAttribute("data-settings-leaf")));
        assert.equal(leaves.length, 13, "all settings destinations must remain available");
        for (const leaf of leaves) {
          await page.locator(`[data-settings-leaf="${leaf}"]`).click();
          await page.getByTestId("settings-page-title").waitFor({ state: "visible" });
          await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
          const layout = await page.getByTestId("settings-content").evaluate(element => {
            const headings = [...element.querySelectorAll("h2")];
            const title = headings[0];
            return {
              headings: headings.length,
              title: title?.textContent.trim(),
              titleHeight: title?.getBoundingClientRect().height,
              titleOverflow: title ? title.scrollWidth > title.clientWidth + 1 : true,
              overflow: element.scrollWidth > element.clientWidth + 1,
            };
          });
          assert.equal(layout.headings, 1, `${width}px ${locale} ${leaf}: exactly one page heading`);
          assert.ok(layout.titleHeight <= 28 && !layout.titleOverflow, `${width}px ${locale} ${leaf}: heading must fit one short line (${layout.title})`);
          assert.equal(layout.overflow, false, `${width}px ${locale} ${leaf}: content escapes its column`);
          assert.equal(await page.locator('[data-settings-leaf][aria-current="page"]').count(), 1);
          if (width === 1280 && locale === "zh-CN" && ["appearance", "ai-connection", "ai-behavior"].includes(leaf)) {
            await page.screenshot({ path: path.join(out, `${leaf}.png`) });
          }
        }
        assert.deepEqual(errors, [], `${width}px ${locale}: uncaught settings errors`);
        console.log(`  ✓ ${width}px ${locale}: all 13 settings pages have one short heading and fit their column`);
      } finally {
        await page.close();
      }
    }
  }
}

(async () => {
  const browser = await chromium.launch({ channel: "chrome" });
  try {
    await settingsNavigation(browser);
    await toolsHeader(browser, 960);
    await toolsHeader(browser, 390);
    await everySettingsPage(browser);
  } finally {
    await browser.close();
  }
  console.log("settings/tools layout regression checks passed");
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
