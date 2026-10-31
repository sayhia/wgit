#!/usr/bin/env node
/** Real-renderer workbench material contract; no screenshot baseline updates. */
const { chromium } = require("playwright");
const fs = require("node:fs");
const path = require("node:path");
const { measureSurface, settlePaint } = require("./glass-pixels.cjs");

const BASE = process.env.BASE_URL || "http://127.0.0.1:9245/";
const OUT = path.join(__dirname, "out");
const ACCENTS = ["blue", "teal", "green", "amber", "rose", "violet"];
const TABS = ["工作区 & 源码变更", "提交图谱 & 历史", "分支 & 标签", "储藏箱 & 快照", "评审", "效能工具 & 诊断", "设置"];
const PALETTE = '[role="dialog"][aria-label="命令面板"] > .glass-panel';

async function applyTheme(page, theme, accent) {
  await page.evaluate(([themeName, accentName]) => {
    const root = document.documentElement;
    root.dataset.theme = themeName;
    root.classList.toggle("dark", themeName === "dark");
    root.classList.toggle("light", themeName === "light");
    root.dataset.accent = accentName;
  }, [theme, accent]);
  await settlePaint(page);
}
async function chromeStyles(page) {
  return page.evaluate(() => [...new Set(document.querySelectorAll(".glass-bar, .surface-chrome"))].filter((element) => {
    const rect = element.getBoundingClientRect();
    return rect.width > 0 && rect.height > 0 && getComputedStyle(element).visibility !== "hidden";
  }).map((element, index) => {
    const style = getComputedStyle(element);
    return { index, tag: element.tagName, background: style.backgroundColor, image: style.backgroundImage, shadow: style.boxShadow, blur: style.backdropFilter, filter: style.filter, border: style.borderColor };
  }));
}
async function readingBlur(page) {
  return page.evaluate(() => Array.from(document.querySelectorAll("main, main *")).filter((element) => {
    const rect = element.getBoundingClientRect();
    if (rect.width <= 0 || rect.height <= 0 || rect.bottom < 0 || rect.top > innerHeight) return false;
    const style = getComputedStyle(element);
    if (style.visibility === "hidden" || style.display === "none") return false;
    if (element.closest('[role="dialog"], .glass-panel, .glass-popover, .surface-panel, .surface-popover')) return false;
    return style.backdropFilter !== "none" || style.filter.includes("blur(");
  }).map((element) => ({ tag: element.tagName, class: String(element.className).slice(0, 120), blur: getComputedStyle(element).backdropFilter })).slice(0, 8));
}
async function paletteState(page) {
  return page.locator(PALETTE).evaluate((panel) => {
    const visible = (element) => {
      const box = element.getBoundingClientRect();
      const style = getComputedStyle(element);
      return box.width > 0 && box.height > 0 && style.visibility !== "hidden" && style.display !== "none";
    };
    const style = getComputedStyle(panel);
    const box = panel.getBoundingClientRect();
    const blurred = Array.from(document.querySelectorAll("*")).filter((element) => visible(element) && getComputedStyle(element).backdropFilter !== "none");
    const probe = document.createElement("span");
    probe.style.visibility = "hidden";
    document.body.appendChild(probe);
    const resolve = (value, property = "backgroundColor") => {
      probe.style[property] = value;
      return getComputedStyle(probe)[property];
    };
    const overlay = resolve("var(--ug-bg-overlay)");
    const strongBorder = resolve("var(--ug-border-strong)", "color");
    const canvas = resolve("Canvas");
    const canvasText = resolve("CanvasText", "color");
    probe.remove();
    const points = [[box.left + 16, box.top + 16], [box.left + box.width / 2, box.top + box.height / 2], [box.right - 16, box.bottom - 16]];
    return {
      blur: style.backdropFilter, blurredCount: blurred.length, onlyPaletteBlurred: blurred.length === 1 && blurred[0] === panel,
      image: style.backgroundImage, shadow: style.boxShadow, background: style.backgroundColor, border: style.borderTopColor,
      color: style.color, opacity: style.opacity, filter: style.filter,
      overlay, strongBorder, canvas, canvasText,
      insideViewport: box.top >= 0 && box.left >= 0 && box.right <= innerWidth && box.bottom <= innerHeight,
      hitTest: points.every(([x, y]) => panel.contains(document.elementFromPoint(x, y))),
    };
  });
}
async function movePointerAndReadSheen(page) {
  await page.mouse.move(80, 20);
  await page.mouse.move(23, 180);
  await page.mouse.move(640, 220);
  return page.evaluate(() => Array.from(document.querySelectorAll("[data-glass]")).filter((element) => element.style.getPropertyValue("--glass-x") || element.style.getPropertyValue("--glass-y")).map((element) => ({ tag: element.tagName, style: element.getAttribute("style") })));
}

(async () => {
  fs.mkdirSync(OUT, { recursive: true });
  const browser = await chromium.launch({ channel: "chrome" });
  const failures = [];
  const rows = [];
  const materials = [];
  const readingViews = [];
  const fallbacks = [];
  const check = (ok, label, details) => { if (!ok) failures.push(`${label}: ${typeof details === "string" ? details : JSON.stringify(details)}`); };
  try {
    const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
    await page.addInitScript(() => localStorage.setItem("wgit.locale", "zh-CN"));
    await page.goto(BASE, { waitUntil: "networkidle" });
    await page.getByRole("button", { name: TABS[0], exact: true }).click();
    for (const theme of ["dark", "light"]) {
      let neutralChrome;
      for (const accent of ACCENTS) {
        const before = failures.length;
        await applyTheme(page, theme, accent);
        const chrome = await chromeStyles(page);
        check(chrome.length >= 3, `${theme}/${accent} chrome inventory`, chrome);
        for (const surface of chrome) {
          check(surface.blur === "none" && surface.filter === "none" && surface.image === "none" && surface.shadow === "none", `${theme}/${accent} flat chrome`, surface);
        }
        const paints = chrome.map(({ tag, background, border }) => ({ tag, background, border }));
        if (accent === "blue") neutralChrome = paints;
        else check(JSON.stringify(paints) === JSON.stringify(neutralChrome), `${theme}/${accent} accent-independent chrome`, { actual: paints, blue: neutralChrome });
        const reading = await readingBlur(page);
        check(reading.length === 0, `${theme}/${accent} opaque reading surfaces`, reading);
        const pointer = await movePointerAndReadSheen(page);
        check(pointer.length === 0, `${theme}/${accent} no pointer sheen writes`, pointer);
        for (const [name, selector, strip] of [["header", "header.glass-bar", "bottom"], ["dock", "aside.glass-bar", "right"]]) {
          const pixels = await measureSurface(page, `${theme}/${accent}/${name}`, selector, strip);
          failures.push(...pixels.failures);
          rows.push({ theme, accent, surface: name, contrast: pixels.worst });
        }
        if (accent === "blue") await page.screenshot({ path: path.join(OUT, `theme-surface-${theme}-workspace.png`) });
        await page.keyboard.press("Control+k");
        await page.locator(PALETTE).waitFor({ state: "visible" });
        await settlePaint(page);
        const palette = await paletteState(page);
        materials.push({ theme, accent, chrome, palette });
        check(palette.blur === "blur(12px)" && palette.onlyPaletteBlurred && palette.image === "none", `${theme}/${accent} one neutral palette blur`, palette);
        check(palette.opacity === "1" && palette.filter === "none" && palette.insideViewport && palette.hitTest, `${theme}/${accent} clear reachable palette`, palette);
        check((await movePointerAndReadSheen(page)).length === 0, `${theme}/${accent} no floating pointer sheen`, "inline --glass-x/y appeared");
        const pixels = await measureSurface(page, `${theme}/${accent}/palette`, `${PALETTE} #palette-listbox`);
        failures.push(...pixels.failures);
        rows.push({ theme, accent, surface: "palette", contrast: pixels.worst });
        if (accent === "blue") await page.screenshot({ path: path.join(OUT, `theme-surface-${theme}-palette.png`) });
        await page.keyboard.press("Escape");
        console.log(`  ${failures.length === before ? "✓" : "✗"} ${theme}/${accent}: flat neutral chrome, clear content/palette, rendered contrast`);
      }
      // Walk the other reading surfaces once per theme; no modal is open.
      await applyTheme(page, theme, "blue");
      for (const tab of TABS.slice(1)) {
        await page.getByRole("button", { name: tab, exact: true }).first().click();
        await page.waitForTimeout(120);
        const reading = await readingBlur(page);
        readingViews.push({ theme, view: tab, blurred: reading });
        check(reading.length === 0, `${theme}/${tab} reading surfaces`, reading);
      }
      await page.getByRole("button", { name: TABS[0], exact: true }).click();
    }

    const cdp = await page.context().newCDPSession(page);
    for (const theme of ["dark", "light"]) {
      await applyTheme(page, theme, "blue");
      await page.keyboard.press("Control+k");
      await page.locator(PALETTE).waitFor({ state: "visible" });
      for (const className of ["reduce-transparency", "high-contrast"]) {
        await page.evaluate((name) => document.documentElement.classList.add(name), className);
        await settlePaint(page);
        const state = await paletteState(page);
        fallbacks.push({ theme, mode: className, state });
        check(state.blur === "none" && state.background === state.overlay && state.hitTest, `${theme}/${className} opaque fallback`, state);
        if (className === "high-contrast") check(state.shadow === "none" && state.border === state.strongBorder, `${theme}/high-contrast strong border`, state);
        await page.evaluate((name) => document.documentElement.classList.remove(name), className);
        await settlePaint(page);
      }
      await cdp.send("Emulation.setEmulatedMedia", { features: [{ name: "prefers-reduced-transparency", value: "reduce" }] });
      check(await page.evaluate(() => matchMedia("(prefers-reduced-transparency: reduce)").matches), `${theme}/system reduced transparency`, "media emulation inactive");
      await settlePaint(page);
      const reduced = await paletteState(page);
      fallbacks.push({ theme, mode: "prefers-reduced-transparency", state: reduced });
      check(reduced.blur === "none" && reduced.background === reduced.overlay && reduced.hitTest, `${theme}/system reduced transparency fallback`, reduced);
      await cdp.send("Emulation.setEmulatedMedia", { features: [] });
      await page.emulateMedia({ forcedColors: "active", colorScheme: theme });
      await settlePaint(page);
      const forced = await paletteState(page);
      fallbacks.push({ theme, mode: "forced-colors", state: forced });
      check(await page.evaluate(() => matchMedia("(forced-colors: active)").matches), `${theme}/forced colors`, "media emulation inactive");
      check(forced.blur === "none" && forced.shadow === "none" && forced.background === forced.canvas && forced.color === forced.canvasText && forced.hitTest, `${theme}/forced colors fallback`, forced);
      await page.emulateMedia({ forcedColors: "none", colorScheme: "no-preference" });
      await settlePaint(page);
      await page.keyboard.press("Escape");
    }
    await cdp.detach();
    fs.writeFileSync(path.join(OUT, "theme-surfaces-report.json"), JSON.stringify({ base: BASE, materials, readingViews, fallbacks, rows, failures }, null, 2));
  } finally { await browser.close(); }
  if (failures.length) throw new Error(`Theme surface audit found ${failures.length} issue(s):\n${failures.join("\n")}`);
  console.log(`theme surfaces passed: 12 theme/accent combinations, 7 reading views, accessibility fallbacks and ${rows.length} pixel checks`);
})().catch((error) => { console.error(error.message || error); process.exitCode = 1; });
