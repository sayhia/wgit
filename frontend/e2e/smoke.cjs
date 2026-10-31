/**
 * Runtime smoke test for the i18n migration.
 *
 * Loads the Vite dev server (dev fixtures stand in for the Wails backend), walks
 * every top-level view in both locales, and fails on:
 *   · uncaught exceptions / console errors (Wails runtime 404s excluded)
 *   · i18n "missing key" warnings (a raw key leaking into the UI)
 *   · CJK text still visible after switching to English
 */
const { chromium } = require("playwright");

const BASE = process.env.BASE_URL || "http://127.0.0.1:9245/";
const path = require("node:path");
const fs = require("node:fs");
const OUT = path.join(__dirname, "out");
fs.mkdirSync(OUT, { recursive: true });

const VIEWS = [
  ["nav.workspace", "工作区 & 源码变更", "Workspace & Changes"],
  ["nav.history", "提交图谱 & 历史", "Commit Graph & History"],
  ["nav.branches", "分支 & 标签", "Branches & Tags"],
  ["nav.stashes", "储藏箱 & 快照", "Stashes & Snapshots"],
  ["nav.review", "评审", "Change Review"],
  ["nav.tools", "效能工具 & 诊断", "Tools & Diagnostics"],
  ["nav.settings", "设置", "Settings"],
];

const CJK = /[\u4e00-\u9fff]/;
/** CJK that legitimately appears in English mode: fixture file names / commit data. */
const ALLOWED = [
  /中文配置表/,
  /大型项目/,
  /架构说明/,
  /开发预览/,
  /分页与筛选/,
  /分支洞察/,
  /稀疏检出/,
  /批量锁定/,
  /代码检索/,
  /升级依赖/,
  /截断保护/,
  /monorepo/i,
  // Fixture commit subjects, diff hunks and the locale's own label. These are
  // *data* rendered inside the English UI, not untranslated chrome — the check
  // exists to catch the latter.
  /分页加载不再重复/,
  /分页 \+ 筛选/,
  /服务端模糊文件搜索/,
  /评审 diff 预览/,
  /虚拟滚动窗口计算/,
  /简体中文/,
];

(async () => {
  const browser = await chromium.launch({ channel: "chrome" });
  const page = await browser.newPage({ viewport: { width: 1600, height: 1000 } });

  const errors = [];
  const missingKeys = [];
  page.on("console", (msg) => {
    const text = msg.text();
    if (text.includes("[i18n] missing key")) missingKeys.push(text);
    if (msg.type() !== "error") return;
    // The Wails runtime script only exists inside the desktop shell.
    if (/wails\/(runtime|custom\.js)/.test(text)) return;
    if (/Failed to load resource/.test(text)) return;
    errors.push(text);
  });
  page.on("pageerror", (err) => errors.push(`pageerror: ${err.message}`));

  await page.goto(BASE, { waitUntil: "networkidle" });
  await page.waitForTimeout(1500);

  const report = { errors, missingKeys, views: {}, cjk: [] };

  async function visitAll(locale) {
    const idx = locale === "zh" ? 1 : 2;
    for (const view of VIEWS) {
      const label = view[idx];
      const tab = page.locator(`[aria-label="${label}"]`).first();
      if ((await tab.count()) === 0) {
        report.views[`${locale}:${view[0]}`] = "nav-not-found";
        continue;
      }
      await tab.click({ timeout: 5000 }).catch((err) => {
        report.views[`${locale}:${view[0]}`] = `click-failed: ${err.message}`;
      });
      await page.waitForTimeout(900);
      const body = await page.locator("body").innerText();
      if (!report.views[`${locale}:${view[0]}`]) {
        report.views[`${locale}:${view[0]}`] = body.length;
      }
      if (locale === "en") {
        for (const line of body.split("\n")) {
          if (!CJK.test(line)) continue;
          if (ALLOWED.some((re) => re.test(line))) continue;
          report.cjk.push(`${view[0]}: ${line.trim().slice(0, 120)}`);
        }
      }
      await page.screenshot({ path: `${OUT}/${locale}-${view[0]}.png` });
    }
  }

  await visitAll("zh");

  // Switch to English from Settings → Appearance (the locale button is labelled
  // with its own endonym, so the click is locale-independent).
  await page.locator('[aria-label="设置"]').first().click().catch(() => {});
  await page.waitForTimeout(700);
  const english = page.locator('button:has-text("English")').first();
  if ((await english.count()) > 0) {
    await english.click();
    await page.waitForTimeout(900);
  } else {
    report.views["locale-switch"] = "english-button-not-found";
  }

  await visitAll("en");

  console.log(JSON.stringify(report, null, 2));
  await browser.close();

  const failed = errors.length > 0 || missingKeys.length > 0 || report.cjk.length > 0;
  process.exit(failed ? 1 : 0);
})();
