/**
 * Shared WCAG text-contrast measurement, evaluated inside the page.
 *
 * Resolves the *effective* foreground and background for every element that
 * owns visible text: translucent colours are composited down through the
 * ancestor chain to the canvas, and gradients are skipped (cannot be resolved
 * numerically). Returns only the failures, de-duplicated by colour pair.
 */
const AUDIT = () => {
  /**
   * Parse any computed colour Chrome may hand back.
   *
   * `color-mix(in oklab, …)` serialises as `oklab(L a b / alpha)` — without the
   * OKLab branch every glass surface would silently fall through to its
   * ancestor's colour and the audit would measure the wrong background.
   */
  const parse = (s) => {
    const m = s.match(/rgba?\(([^)]+)\)/);
    if (m) {
      const p = m[1].split(/[,\s/]+/).filter(Boolean).map(Number);
      return { r: p[0], g: p[1], b: p[2], a: p.length > 3 ? p[3] : 1 };
    }
    const ok = s.match(/oklab\(([^)]+)\)/);
    if (ok) {
      const parts = ok[1].split("/");
      const nums = parts[0].trim().split(/\s+/).map(Number);
      const a = parts[1] ? Number(parts[1]) : 1;
      const [L, A, B] = nums;
      const l_ = L + 0.3963377774 * A + 0.2158037573 * B;
      const m_ = L - 0.1055613458 * A - 0.0638541728 * B;
      const s_ = L - 0.0894841775 * A - 1.291485548 * B;
      const l3 = l_ ** 3, m3 = m_ ** 3, s3 = s_ ** 3;
      const lin = [
        4.0767416621 * l3 - 3.3077115913 * m3 + 0.2309699292 * s3,
        -1.2684380046 * l3 + 2.6097574011 * m3 - 0.3413193965 * s3,
        -0.0041960863 * l3 - 0.7034186147 * m3 + 1.707614701 * s3,
      ];
      const toSrgb = (v) => {
        const c = v <= 0.0031308 ? 12.92 * v : 1.055 * Math.pow(Math.max(v, 0), 1 / 2.4) - 0.055;
        return Math.min(255, Math.max(0, Math.round(c * 255)));
      };
      return { r: toSrgb(lin[0]), g: toSrgb(lin[1]), b: toSrgb(lin[2]), a };
    }
    const srgb = s.match(/color\(srgb\s+([^)]+)\)/);
    if (srgb) {
      const parts = srgb[1].split("/");
      const nums = parts[0].trim().split(/\s+/).map(Number);
      const a = parts[1] ? Number(parts[1]) : 1;
      return {
        r: Math.round(nums[0] * 255),
        g: Math.round(nums[1] * 255),
        b: Math.round(nums[2] * 255),
        a,
      };
    }
    return null;
  };
  const over = (top, bottom) => ({
    r: top.r * top.a + bottom.r * (1 - top.a),
    g: top.g * top.a + bottom.g * (1 - top.a),
    b: top.b * top.a + bottom.b * (1 - top.a),
    a: 1,
  });
  const lum = (c) => {
    const f = (v) => {
      const x = v / 255;
      return x <= 0.03928 ? x / 12.92 : Math.pow((x + 0.055) / 1.055, 2.4);
    };
    return 0.2126 * f(c.r) + 0.7152 * f(c.g) + 0.0722 * f(c.b);
  };
  const ratio = (a, b) => {
    const l1 = lum(a);
    const l2 = lum(b);
    return (Math.max(l1, l2) + 0.05) / (Math.min(l1, l2) + 0.05);
  };

  const canvas = parse(getComputedStyle(document.documentElement).backgroundColor) || {
    r: 17,
    g: 17,
    b: 19,
    a: 1,
  };

  const effBg = (el) => {
    const layers = [];
    let node = el;
    while (node) {
      const cs = getComputedStyle(node);
      // A gradient cannot be resolved numerically, but its base colour still
      // gives a usable approximation (the aurora layer is < 14 % alpha).
      const hasGradient = cs.backgroundImage && cs.backgroundImage !== "none";
      const c = parse(cs.backgroundColor);
      if (c && c.a > 0) {
        layers.push(c);
        if (c.a >= 0.999) break;
      } else if (hasGradient && !layers.length) {
        // Transparent gradient layer: treat it as the canvas colour and stop.
        layers.push({ ...canvas, a: 1 });
        break;
      }
      node = node.parentElement;
    }
    let base =
      layers.length && layers[layers.length - 1].a >= 0.999
        ? layers.pop()
        : { ...canvas, a: 1 };
    for (let i = layers.length - 1; i >= 0; i--) base = over(layers[i], base);
    return base;
  };

  const path = (el) => {
    const parts = [];
    let node = el;
    for (let i = 0; i < 4 && node && node.nodeType === 1; i++) {
      let s = node.tagName.toLowerCase();
      if (node.className && typeof node.className === "string") {
        const cls = node.className
          .split(/\s+/)
          .filter((c) => c && !c.includes(":"))
          .slice(0, 3);
        if (cls.length) s += "." + cls.join(".");
      }
      parts.unshift(s);
      node = node.parentElement;
    }
    return parts.join(" > ");
  };

  const offenders = [];
  const seen = new Set();

  for (const el of document.querySelectorAll("*")) {
    let text = "";
    for (const child of el.childNodes) {
      if (child.nodeType === 3) text += child.nodeValue;
    }
    text = text.trim().replace(/\s+/g, " ");
    if (!text) continue;

    const cs = getComputedStyle(el);
    if (cs.display === "none" || cs.visibility === "hidden" || Number(cs.opacity) < 0.1) continue;
    if (el.closest("[aria-hidden='true']")) continue;
    const rect = el.getBoundingClientRect();
    if (rect.width < 2 || rect.height < 2) continue;
    if (rect.bottom < -200 || rect.top > window.innerHeight + 4000) continue;

    const fg = parse(cs.color);
    if (!fg || fg.a === 0) continue;
    const bg = effBg(el);
    if (!bg) continue;

    const fgSolid = fg.a >= 0.999 ? fg : over(fg, bg);
    const r = ratio(fgSolid, bg);
    const size = parseFloat(cs.fontSize);
    const weight = Number(cs.fontWeight) || 400;
    const large = size >= 24 || (size >= 18.66 && weight >= 700);
    const need = large ? 3 : 4.5;
    if (r + 0.005 >= need) continue;

    const key = `${Math.round(fgSolid.r)},${Math.round(fgSolid.g)},${Math.round(fgSolid.b)}|${Math.round(
      bg.r
    )},${Math.round(bg.g)},${Math.round(bg.b)}|${size}/${weight}`;
    if (seen.has(key)) continue;
    seen.add(key);

    offenders.push({
      ratio: Math.round(r * 100) / 100,
      need,
      text: text.slice(0, 48),
      size,
      weight,
      fg: `rgb(${Math.round(fgSolid.r)},${Math.round(fgSolid.g)},${Math.round(fgSolid.b)})`,
      bg: `rgb(${Math.round(bg.r)},${Math.round(bg.g)},${Math.round(bg.b)})`,
      sel: path(el),
    });
  }
  return offenders;
};

/** Freeze transitions so a theme switch cannot be measured mid-interpolation. */
async function freezeMotion(page) {
  await page.addStyleTag({
    content: "*, *::before, *::after { transition: none !important; animation: none !important; }",
  });
  await page.emulateMedia({ reducedMotion: "reduce" });
}

async function applyTheme(page, theme, accent) {
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
  await page.waitForTimeout(220);
}

module.exports = { AUDIT, freezeMotion, applyTheme };
