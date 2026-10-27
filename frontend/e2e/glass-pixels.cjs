/**
 * Rendered-pixel contrast check for the glass surfaces.
 *
 * The CSS audit can only reason about declared colours, and `backdrop-filter`
 * makes the *painted* background a blend of the tint, the aurora and whatever
 * is behind the surface. So this script screenshots the actual pixels of each
 * glass surface (a text-free strip), takes the lightest and darkest pixel as
 * the worst case, and checks the token text colours against them.
 *
 * Surfaces: header bar, activity dock, command palette panel.
 */
const { chromium } = require("playwright");
const { PNG } = require("pngjs");

const TEXT = {
  primary: { dark: "#eeeef0", light: "#1e1f24" },
  secondary: { dark: "#b2b3bd", light: "#62636c" },
  muted: { dark: "#8b8d98", light: "#63646d" },
};

const parse = (hex) => {
  const h = hex.replace("#", "");
  return { r: parseInt(h.slice(0, 2), 16), g: parseInt(h.slice(2, 4), 16), b: parseInt(h.slice(4, 6), 16) };
};
const lum = ({ r, g, b }) => {
  const f = (v) => {
    const x = v / 255;
    return x <= 0.03928 ? x / 12.92 : Math.pow((x + 0.055) / 1.055, 2.4);
  };
  return 0.2126 * f(r) + 0.7152 * f(g) + 0.0722 * f(b);
};
const ratio = (a, b) => {
  const [hi, lo] = [lum(a), lum(b)].sort((x, y) => y - x);
  return (hi + 0.05) / (lo + 0.05);
};

/** Extreme pixels of a clipped screenshot (ignores fully transparent pixels). */
function extremes(buffer) {
  const png = PNG.sync.read(buffer);
  let min = { r: 255, g: 255, b: 255 };
  let max = { r: 0, g: 0, b: 0 };
  let minL = Infinity;
  let maxL = -Infinity;
  let count = 0;
  for (let i = 0; i < png.data.length; i += 4) {
    const px = { r: png.data[i], g: png.data[i + 1], b: png.data[i + 2] };
    if (png.data[i + 3] < 250) continue;
    const l = lum(px);
    count++;
    if (l < minL) {
      minL = l;
      min = px;
    }
    if (l > maxL) {
      maxL = l;
      max = px;
    }
  }
  const hex = (c) => `#${[c.r, c.g, c.b].map((v) => v.toString(16).padStart(2, "0")).join("")}`;
  return { min, max, minHex: hex(min), maxHex: hex(max), count };
}

(async () => {
  const browser = await chromium.launch({ channel: "chrome" });
  const page = await browser.newPage({ viewport: { width: 1500, height: 950 } });
  await page.addInitScript(() => localStorage.setItem("wgit.locale", "en-US"));
  await page.goto("http://127.0.0.1:9245/", { waitUntil: "networkidle" });
  await page.waitForTimeout(1800);

  const results = [];

  const measure = async (label, clip, theme) => {
    const buf = await page.screenshot({ clip });
    const { min, max, minHex, maxHex, count } = extremes(buf);
    const row = { label, theme, minHex, maxHex, count, worst: {} };
    for (const [name, colors] of Object.entries(TEXT)) {
      const fg = parse(colors[theme]);
      // Worst case is the background closest in luminance to the text colour.
      const candidates = [min, max];
      const worst = candidates.reduce(
        (acc, bg) => {
          const r = ratio(fg, bg);
          return r < acc.ratio ? { ratio: r, bg } : acc;
        },
        { ratio: Infinity, bg: null }
      );
      row.worst[name] = Math.round(worst.ratio * 100) / 100;
    }
    results.push(row);
  };

  /** Strip inside a glass element that is guaranteed to be free of text. */
  const stripOf = (box, where) => {
    if (where === "bottom") {
      return { x: box.x + 8, y: box.y + box.height - 7, width: box.width - 16, height: 4 };
    }
    if (where === "left") {
      return { x: box.x + 1, y: box.y + box.height * 0.45, width: 3, height: box.height * 0.35 };
    }
    return { x: box.x + 10, y: box.y + box.height - 16, width: box.width - 20, height: 6 };
  };

  const measureEl = async (label, selector, where, theme) => {
    const el = page.locator(selector).first();
    const box = await el.boundingBox();
    if (!box) {
      results.push({ label, theme, error: "no box" });
      return;
    }
    const buf = await page.screenshot({ clip: stripOf(box, where) });
    const { min, max, minHex, maxHex, count } = extremes(buf);
    const row = { label, theme, minHex, maxHex, count, worst: {} };
    for (const [name, colors] of Object.entries(TEXT)) {
      const fg = parse(colors[theme]);
      const worst = [min, max].reduce(
        (acc, bg) => {
          const r = ratio(fg, bg);
          return r < acc.ratio ? { ratio: r, bg } : acc;
        },
        { ratio: Infinity, bg: null }
      );
      row.worst[name] = Math.round(worst.ratio * 100) / 100;
    }
    results.push(row);
  };

  for (const theme of ["dark", "light"]) {
    await page.evaluate((t) => {
      const r = document.documentElement;
      r.dataset.theme = t;
      r.classList.toggle("light", t === "light");
      r.classList.toggle("dark", t === "dark");
      delete r.dataset.accent;
    }, theme);
    await page.waitForTimeout(500);
    await measureEl("header", "header[data-glass]", "bottom", theme);
    await measureEl("dock", "aside[data-glass]", "left", theme);
  }

  await page.evaluate(() => {
    const r = document.documentElement;
    r.dataset.theme = "dark";
    r.classList.add("dark");
    r.classList.remove("light");
  });
  await page.keyboard.press("Meta+k");
  await page.waitForTimeout(900);
  await measureEl("palette", ".glass-panel", "bottom", "dark");
  await page.keyboard.press("Escape");

  console.log(JSON.stringify(results, null, 1));
  await browser.close();
})();
