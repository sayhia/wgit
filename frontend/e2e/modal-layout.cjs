#!/usr/bin/env node
/** Long-reference modal layout regression; previews only, never runs Git actions. */
const { chromium } = require("playwright");
const fs = require("node:fs");
const path = require("node:path");
const BASE = process.env.BASE_URL || "http://127.0.0.1:9245/";
const OUT = path.join(__dirname, "out");
const REF = `feature/gameplay/season-2026/refactor-${"multiplayer-synchronization-assets-pipeline-".repeat(4)}`;

async function panelMetrics(page, modal) {
  return page.locator(`[data-testid="${modal}-modal-footer"]`).evaluate((footer) => {
    const panel = footer.parentElement;
    const box = panel.getBoundingClientRect();
    const buttons = Array.from(footer.querySelectorAll("button"));
    return {
      overflow: panel.scrollWidth - panel.clientWidth,
      headerHeight: panel.firstElementChild.getBoundingClientRect().height,
      bodyHeight: panel.querySelector('[data-testid$="-modal-body"]')?.getBoundingClientRect().height,
      toastReserve: parseFloat(getComputedStyle(document.documentElement).getPropertyValue("--ug-toast-reserve")) || 0,
      insideViewport: box.left >= 0 && box.top >= 0 && box.right <= innerWidth && box.bottom <= innerHeight,
      footerInside: footer.getBoundingClientRect().bottom <= box.bottom,
      actions: buttons.map((button) => {
        const rect = button.getBoundingClientRect();
        const label = button.lastElementChild?.tagName === "SPAN" ? button.lastElementChild : button;
        const labelStyle = getComputedStyle(label);
        return {
          text: button.textContent.trim(),
          oneLine: (label.getBoundingClientRect().height - (label === button
            ? parseFloat(labelStyle.paddingTop) + parseFloat(labelStyle.paddingBottom) + parseFloat(labelStyle.borderTopWidth) + parseFloat(labelStyle.borderBottomWidth)
            : 0)) <= parseFloat(labelStyle.lineHeight) + 2,
          inside: rect.left >= box.left && rect.right <= box.right,
          reachable: button.contains(document.elementFromPoint(rect.left + rect.width / 2, rect.top + rect.height / 2)),
        };
      }),
    };
  });
}

(async () => {
  fs.mkdirSync(OUT, { recursive: true });
  const browser = await chromium.launch({ channel: "chrome" });
  const failures = [];
  const rows = [];
  try {
    for (const locale of ["zh-CN", "en-US"]) {
      for (const [width, height] of [[960, 640], [390, 640], [960, 400]]) {
        const page = await browser.newPage({ viewport: { width, height } });
        await page.addInitScript(() => localStorage.setItem("wgit.locale", "zh-CN"));
        await page.goto(BASE, { waitUntil: "networkidle" });
        if (locale === "en-US") {
          await page.getByRole("button", { name: "设置", exact: true }).click();
          await page.getByRole("button", { name: "English", exact: true }).click();
          await page.getByRole("button", { name: "Workspace & Changes", exact: true }).waitFor({ state: "visible" });
        }
        // The split push button remains visible at every shell breakpoint.
        const trigger = page.locator('header button[title]').filter({ visible: true });
        const candidate = await trigger.evaluateAll((buttons) => buttons.map((button) => button.title).find((title) => /高级推送设置|Advanced push/i.test(title)));
        if (!candidate) throw new Error(`No advanced push trigger for ${locale}`);
        await page.getByRole("button", { name: candidate, exact: true }).click();
        const header = page.getByTestId("sync-modal-header");
        await header.waitFor({ state: "visible" });
        // Put a representative legal long ref into the real rendered header.
        // No form field or Git action is changed: this only exercises layout.
        await header.locator("strong").evaluate((element, value) => { element.textContent = value; element.title = value; }, REF);
        await header.locator(".text-accent.font-mono[title]").evaluate((element, value) => { element.textContent = `→ origin/${value}`; element.title = `origin/${value}`; }, REF);
        const pullTab = locale === "zh-CN" ? "拉取 (Pull)" : "Pull";
        await page.getByTestId("sync-modal-tabs").getByRole("button", { name: new RegExp(`^${pullTab.replace(/[()]/g, "\\$&")}`) }).click();
        const sync = await panelMetrics(page, "sync");
        if (sync.overflow > 1 || sync.headerHeight > 100 || !sync.insideViewport || !sync.footerInside || sync.bodyHeight < Math.max(24, 120 - sync.toastReserve) || sync.actions.some((action) => !action.oneLine || !action.inside || !action.reachable)) {
          failures.push({ locale, width, height, modal: "sync", metrics: sync });
        }
        rows.push({ locale, width, height, modal: "sync", metrics: sync });
        if (width === 960 && height === 400 && locale === "en-US") await page.screenshot({ path: path.join(OUT, "modal-layout-sync-long-ref-en-960x400.png") });
        await page.keyboard.press("Escape");

        await page.keyboard.press("Control+r");
        const overview = page.getByTestId("rebase-target-overview");
        await overview.waitFor({ state: "visible" });
        await overview.locator("div").first().locator("span").last().evaluate((element, value) => { element.textContent = value; element.title = value; }, REF);
        const suggested = page.getByTestId("rebase-suggested-branch").first();
        if (await suggested.count()) await suggested.locator("span").first().evaluate((element, value) => { element.textContent = `origin/${value}`; }, REF);
        const rebase = await panelMetrics(page, "rebase");
        const overviewHeight = await overview.evaluate((element) => element.getBoundingClientRect().height);
        if (rebase.overflow > 1 || overviewHeight > 90 || !rebase.insideViewport || !rebase.footerInside || rebase.actions.some((action) => !action.oneLine || !action.inside || !action.reachable)) {
          failures.push({ locale, width, height, modal: "rebase", overviewHeight, metrics: rebase });
        }
        rows.push({ locale, width, height, modal: "rebase", overviewHeight, metrics: rebase });
        if (width === 390 && locale === "en-US") await page.screenshot({ path: path.join(OUT, "modal-layout-rebase-long-ref-en-390.png") });
        await page.keyboard.press("Escape");
        await page.close();
        console.log(`  ${failures.some((failure) => failure.locale === locale && failure.width === width && failure.height === height) ? "✗" : "✓"} ${locale} ${width}×${height}: bounded refs, readable actions, scrolling bodies`);
      }
    }
    fs.writeFileSync(path.join(OUT, "modal-layout-report.json"), JSON.stringify({ rows, failures }, null, 2));
  } finally { await browser.close(); }
  if (failures.length) throw new Error(`Modal layout findings:\n${JSON.stringify(failures, null, 2)}`);
  console.log("modal layout passed: sync and rebase, two locales, three viewports");
})().catch((error) => { console.error(error.message || error); process.exitCode = 1; });
