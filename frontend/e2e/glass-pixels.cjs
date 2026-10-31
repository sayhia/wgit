#!/usr/bin/env node
/**
 * Rendered surface contrast. Ink comes from the currently resolved theme
 * tokens, and backgrounds come from text-free screenshot strips. This can be
 * run directly or reused by the theme/material browser audit.
 */
const { chromium } = require("playwright");
const { PNG } = require("pngjs");
const BASE = process.env.BASE_URL || "http://127.0.0.1:9245/";
const ACCENTS = ["blue", "teal", "green", "amber", "rose", "violet"];

function rgb(value) {
  const channels = value.match(/[\d.]+/g)?.slice(0, 3).map(Number);
  if (!channels || channels.length !== 3 || channels.some((channel) => !Number.isFinite(channel))) {
    throw new Error(`Cannot resolve text colour: ${value}`);
  }
  return { r: channels[0], g: channels[1], b: channels[2] };
}
function luminance({ r, g, b }) {
  const linear = (value) => {
    const channel = value / 255;
    return channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4;
  };
  return 0.2126 * linear(r) + 0.7152 * linear(g) + 0.0722 * linear(b);
}
function contrast(a, b) {
  const [high, low] = [luminance(a), luminance(b)].sort((left, right) => right - left);
  return (high + 0.05) / (low + 0.05);
}
function extremePixels(buffer) {
  const png = PNG.sync.read(buffer);
  let lightest;
  let darkest;
  let count = 0;
  for (let index = 0; index < png.data.length; index += 4) {
    if (png.data[index + 3] < 250) continue;
    const pixel = { r: png.data[index], g: png.data[index + 1], b: png.data[index + 2] };
    const light = luminance(pixel);
    if (!darkest || light < darkest.light) darkest = { ...pixel, light };
    if (!lightest || light > lightest.light) lightest = { ...pixel, light };
    count += 1;
  }
  if (!count) throw new Error("Surface screenshot contains no painted pixels");
  return { count, lightest, darkest };
}
async function resolvedInk(page) {
  return page.evaluate(() => {
    const probe = document.createElement("span");
    probe.style.position = "fixed";
    probe.style.visibility = "hidden";
    document.body.appendChild(probe);
    try {
      return Object.fromEntries(["primary", "secondary", "muted"].map((name) => {
        probe.style.color = `var(--ug-text-${name})`;
        return [name, getComputedStyle(probe).color];
      }));
    } finally { probe.remove(); }
  });
}

async function settlePaint(page) {
  await page.evaluate(async () => {
    await new Promise(requestAnimationFrame);
    const transitions = document.getAnimations().filter((animation) => animation instanceof CSSTransition);
    await Promise.all(transitions.map((animation) => animation.finished.catch(() => {})));
    await new Promise(requestAnimationFrame);
  });
}

async function measureSurface(page, label, selector, where = "left") {
  const element = page.locator(selector).first();
  const box = await element.boundingBox();
  if (!box || box.width < 24 || box.height < 32) throw new Error(`${label}: surface has no measurable box`);
  // Stay inside the 1px border and outside rounded corners and text. Chrome
  // bottom padding and floating-panel left padding are deliberately empty.
  const clip = where === "bottom"
    ? { x: box.x + 16, y: box.y + box.height - 5, width: box.width - 32, height: 2 }
    : { x: where === "right" ? box.x + box.width - 4 : box.x + 3, y: box.y + 16, width: 2, height: box.height - 32 };
  const pixels = extremePixels(await page.screenshot({ clip }));
  const ink = await resolvedInk(page);
  const worst = Object.fromEntries(Object.entries(ink).map(([name, value]) => [
    name, Math.min(contrast(rgb(value), pixels.lightest), contrast(rgb(value), pixels.darkest)),
  ]));
  const failures = Object.entries(worst).filter(([, value]) => value < 4.5).map(
    ([name, value]) => `${label}: ${name} rendered contrast ${value.toFixed(2)} < 4.5`
  );
  return { label, ink, worst, pixels, failures };
}

async function standalone() {
  const browser = await chromium.launch({ channel: "chrome" });
  const results = [];
  const failures = [];
  try {
    const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
    await page.addInitScript(() => localStorage.setItem("wgit.locale", "zh-CN"));
    await page.goto(BASE, { waitUntil: "networkidle" });
    for (const theme of ["dark", "light"]) {
      for (const accent of ACCENTS) {
        await page.evaluate(([themeName, accentName]) => {
          const root = document.documentElement;
          root.dataset.theme = themeName;
          root.classList.toggle("light", themeName === "light");
          root.classList.toggle("dark", themeName === "dark");
          root.dataset.accent = accentName;
        }, [theme, accent]);
        await settlePaint(page);
        for (const [label, selector, strip] of [["header", "header.glass-bar", "bottom"], ["dock", "aside.glass-bar", "right"]]) {
          const result = await measureSurface(page, `${theme}/${accent}/${label}`, selector, strip);
          results.push(result);
          failures.push(...result.failures);
        }
        await page.keyboard.press("Control+k");
        await page.getByRole("dialog", { name: "命令面板", exact: true }).waitFor({ state: "visible" });
        await settlePaint(page);
        const result = await measureSurface(page, `${theme}/${accent}/palette`, '[role="dialog"] > .glass-panel #palette-listbox');
        results.push(result);
        failures.push(...result.failures);
        await page.keyboard.press("Escape");
      }
    }
    console.log(JSON.stringify(results.map(({ label, ink, worst }) => ({ label, ink, worst })), null, 2));
    if (failures.length) throw new Error(failures.join("\n"));
    console.log(`rendered-pixel contrast passed: ${results.length} surfaces, actual theme tokens`);
  } finally { await browser.close(); }
}

module.exports = { measureSurface, settlePaint };
if (require.main === module) standalone().catch((error) => { console.error(error.message || error); process.exitCode = 1; });
