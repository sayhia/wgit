#!/usr/bin/env node
/**
 * Popover reachability audit.
 *
 * A unit test can assert that a menu uses `overflow-visible` or `Teleport`, but
 * only a real renderer can prove the panel is actually hit-testable. Three
 * menus had been silently clipped by an ancestor's overflow (the header's repo /
 * branch / context dropdowns and the tools "more" menu), so the buttons did
 * nothing. This walks the interactive popovers, checks `elementFromPoint` lands
 * inside the panel, and verifies the dismissal rules.
 *
 *   bun run audit:popovers
 *   BASE_URL=http://127.0.0.1:4173/ bun run audit:popovers
 */
const { chromium } = require("playwright");

const BASE = process.env.BASE_URL || "http://127.0.0.1:9245/";
const failures = [];

function fail(name, reason) {
  failures.push(`${name}: ${reason}`);
  console.log(`  ✗ ${name} — ${reason}`);
}

function pass(name) {
  console.log(`  ✓ ${name}`);
}

/** Panel is laid out inside the viewport and every probe point hits it. */
async function reachable(page, selector) {
  return page.evaluate((sel) => {
    const el = document.querySelector(sel);
    if (!el) return { ok: false, reason: "not in DOM" };
    const r = el.getBoundingClientRect();
    if (r.width < 10 || r.height < 10) return { ok: false, reason: "zero size" };
    if (r.left < 0 || r.top < 0 || r.right > window.innerWidth || r.bottom > window.innerHeight) {
      return { ok: false, reason: `outside viewport ${Math.round(r.left)},${Math.round(r.top)} ${Math.round(r.width)}x${Math.round(r.height)}` };
    }
    const probes = [
      [r.left + 12, r.top + 12],
      [r.left + r.width / 2, r.top + r.height / 2],
      [r.right - 12, r.bottom - 12],
    ];
    for (const [x, y] of probes) {
      const hit = document.elementFromPoint(x, y);
      if (!hit || !el.contains(hit)) {
        return { ok: false, reason: `clipped (probe hit ${hit ? hit.tagName : "nothing"})` };
      }
    }
    return { ok: true };
  }, selector);
}

/**
 * Every item label must be fully rendered: a nowrap row wider than its menu
 * silently spills past the panel border (the history menu's Chinese
 * 「智能挑选穿梭机 (Cherry-pick Shuttle)」 did exactly that in a w-52 panel).
 */
async function menuLabelsFit(page, selector) {
  return page.evaluate((sel) => {
    const menu = document.querySelector(sel);
    if (!menu) return { ok: false, reason: "not in DOM" };
    const bad = [];
    for (const btn of menu.querySelectorAll("button")) {
      if (btn.scrollWidth > btn.clientWidth + 1) {
        bad.push(`${btn.textContent.trim()}: ${btn.scrollWidth} > ${btn.clientWidth}`);
      }
    }
    return bad.length ? { ok: false, reason: `labels overflow — ${bad.join("; ")}` } : { ok: true };
  }, selector);
}

/** Panel is a floating menu, not a full-width sheet. */
async function panelWithin(page, selector, maxWidth) {
  return page.evaluate(([sel, max]) => {
    const el = document.querySelector(sel);
    if (!el) return { ok: false, reason: "not in DOM" };
    const r = el.getBoundingClientRect();
    return r.width <= max ? { ok: true } : { ok: false, reason: `panel is ${Math.round(r.width)}px wide (max ${max})` };
  }, [selector, maxWidth]);
}

const menuCount = (page) => page.evaluate(() => document.querySelectorAll("header [data-header-menu]").length);

(async () => {
  const browser = await chromium.launch({ channel: "chrome" });
  const page = await browser.newPage({ viewport: { width: 1280, height: 900 } });
  const errors = [];
  page.on("pageerror", (e) => errors.push(e.message));
  page.on("console", (m) => {
    if (m.type() === "error" && !/Failed to load resource/.test(m.text())) errors.push(m.text());
  });

  await page.addInitScript(() => localStorage.setItem("wgit.locale", "zh-CN"));
  await page.goto(BASE, { waitUntil: "networkidle" });
  await page.waitForTimeout(1600);
  await page.evaluate(() => {
    for (const el of document.querySelectorAll("*")) {
      if (el.scrollTop > 0) el.scrollTop = 0;
    }
    window.scrollTo(0, 0);
  });

  console.log("\n▶ header dropdowns");
  const headerButtons = [
    ["repo", 'header button[title="切换项目 / 打开已有仓库"]', "项目切换"],
    ["branch", 'header button[title="切换分支"]', "切换分支"],
    ["context", 'header button[title="工作流会话上下文盒"]', "工作流上下文"],
  ];
  for (const [name, trigger, heading] of headerButtons) {
    const btn = page.locator(trigger).first();
    if (!(await btn.count())) {
      fail(name, `trigger not found (${trigger})`);
      continue;
    }
    await btn.click();
    await page.waitForTimeout(350);
    if ((await menuCount(page)) !== 1) {
      fail(name, `expected exactly one open menu, got ${await menuCount(page)}`);
    } else {
      const panel = await page.locator("header [data-header-menu]").first();
      const text = (await panel.innerText()).slice(0, 200);
      if (!text.includes(heading)) fail(name, `panel text does not look like ${heading}`);
      const r = await reachable(page, "header [data-header-menu]");
      if (r.ok) pass(name);
      else fail(name, r.reason);
    }
    await page.keyboard.press("Escape");
    await page.waitForTimeout(250);
    if ((await menuCount(page)) !== 0) fail(name, "Escape did not close the panel");
    await page.waitForTimeout(150);
  }

  console.log("\n▶ outside click dismisses header menu");
  await page.locator('header button[title="切换项目 / 打开已有仓库"]').first().click();
  await page.waitForTimeout(300);
  // The repo menu is tall: a fixed (640,700) click lands on its own
  // "提交改动" action and hides the commit composer, so dispatch the outside
  // mousedown on <body> to test dismissal without activating a menu row.
  await page.evaluate(() => {
    document.body.dispatchEvent(
      new MouseEvent("mousedown", { bubbles: true, cancelable: true, view: window })
    );
  });
  await page.waitForTimeout(300);
  if ((await menuCount(page)) === 0) pass("outside click");
  else fail("outside click", `${await menuCount(page)} menu(s) still open`);

  console.log("\n▶ tools ▾ menu (nine secondary tools)");
  await page.locator('[aria-label="效能工具 & 诊断"]').first().click();
  await page.waitForTimeout(700);
  await page.locator('[data-testid="tools-more-btn"]').click({ force: true });
  await page.waitForTimeout(400);
  const toolsMenu = await reachable(page, '[data-testid="tools-more-menu"]');
  const toolsFit = await menuLabelsFit(page, '[data-testid="tools-more-menu"]');
  if (toolsMenu.ok && toolsFit.ok) pass("tools menu");
  else fail("tools menu", toolsMenu.ok ? toolsFit.reason : toolsMenu.reason);
  // The menu must actually navigate: pick the first secondary tool.
  const firstTool = page.locator('[data-testid="tools-more-menu"] button').first();
  if (await firstTool.count()) {
    const before = await page.locator("h3").first().innerText().catch(() => "");
    await firstTool.click();
    await page.waitForTimeout(800);
    const after = await page.locator("h3").first().innerText().catch(() => "");
    if (before !== after) pass("tools menu navigates");
    else fail("tools menu navigates", "heading did not change after selecting a tool");
  } else {
    fail("tools menu navigates", "no items in menu");
  }

  console.log("\n▶ commit composer draft menu");
  await page.locator('[aria-label="工作区 & 源码变更"]').first().click();
  await page.waitForTimeout(700);
  const draft = page.locator('[data-testid="commit-draft-history-btn"]').first();
  if (await draft.count()) {
    await draft.scrollIntoViewIfNeeded();
    await draft.click();
    await page.waitForTimeout(350);
    const r = await reachable(page, '[data-testid="commit-draft-menu"]');
    if (r.ok) pass("draft menu");
    else fail("draft menu", r.reason);
    await draft.click();
    await page.waitForTimeout(250);
  } else {
    fail("draft menu", "trigger not found");
  }

  console.log("\n▶ composer type / tools menus");
  const typeBtn = page.locator('[data-testid="commit-type-trigger"]').first();
  if (await typeBtn.count()) {
    await typeBtn.click();
    await page.waitForTimeout(350);
    const r = await reachable(page, '[data-testid="commit-type-menu"]');
    const fit = await menuLabelsFit(page, '[data-testid="commit-type-menu"]');
    const w = await panelWithin(page, '[data-testid="commit-type-menu"]', 420);
    if (r.ok && fit.ok && w.ok) pass("composer type menu");
    else fail("composer type menu", !r.ok ? r.reason : !fit.ok ? fit.reason : w.reason);
    await typeBtn.click();
    await page.waitForTimeout(250);
  } else fail("composer type menu", "trigger not found");

  const toolsBtn = page.locator('[data-testid="commit-tools-btn"]').first();
  if (await toolsBtn.count()) {
    await toolsBtn.click();
    await page.waitForTimeout(350);
    const r = await reachable(page, '[data-testid="commit-tools-menu"]');
    const w = await panelWithin(page, '[data-testid="commit-tools-menu"]', 360);
    if (r.ok && w.ok) pass("composer tools menu");
    else fail("composer tools menu", r.ok ? w.reason : r.reason);
    await toolsBtn.click();
    await page.waitForTimeout(250);
  } else fail("composer tools menu", "trigger not found");

  console.log("\n▶ changes extension menu and diff overflow menu");
  const extBtn = page.locator('[data-testid="changes-ext-trigger"]').first();
  if (await extBtn.count()) {
    await extBtn.click();
    await page.waitForTimeout(350);
    const r = await reachable(page, '[data-testid="changes-ext-menu"]');
    const w = await panelWithin(page, '[data-testid="changes-ext-menu"]', 320);
    if (r.ok && w.ok) pass("changes extension menu");
    else fail("changes extension menu", r.ok ? w.reason : r.reason);
    await page.keyboard.press("Escape");
    await page.waitForTimeout(200);
  } else fail("changes extension menu", "trigger not found");

  const diffMore = page.locator('[data-testid="diff-more-btn"]').first();
  if (await diffMore.count()) {
    await diffMore.click();
    await page.waitForTimeout(350);
    const r = await reachable(page, '[data-testid="diff-more-menu"]');
    const fit = await menuLabelsFit(page, '[data-testid="diff-more-menu"]');
    const w = await panelWithin(page, '[data-testid="diff-more-menu"]', 320);
    if (r.ok && fit.ok && w.ok) pass("diff overflow menu");
    else fail("diff overflow menu", !r.ok ? r.reason : !fit.ok ? fit.reason : w.reason);
    await page.keyboard.press("Escape");
    await page.waitForTimeout(200);
  } else fail("diff overflow menu", "trigger not found");

  console.log("\n▶ export modal body does not overflow its panel");
  await page.locator('[aria-label="评审"]').first().click();
  await page.waitForTimeout(900);
  const exportBtn = page.locator("button", { hasText: "导出" }).first();
  if (await exportBtn.count()) {
    await exportBtn.evaluate((el) => el.click());
    await page.waitForTimeout(700);
    const over = await page.evaluate(() => {
      const panel = document.querySelector('[role="dialog"] > [data-glass]');
      if (!panel) return { ok: false, reason: "panel not found" };
      return panel.scrollWidth <= panel.clientWidth + 2
        ? { ok: true }
        : { ok: false, reason: `panel scrollWidth ${panel.scrollWidth} > ${panel.clientWidth}` };
    });
    if (over.ok) pass("export modal body");
    else fail("export modal body", over.reason);
    await page.keyboard.press("Escape");
    await page.waitForTimeout(250);
  } else fail("export modal body", "export trigger not found");

  console.log("\n▶ history traversal overflow menu");
  await page.locator('[aria-label="提交图谱 & 历史"]').first().click();
  await page.waitForTimeout(900);
  const trigger = page.locator('[data-testid="history-overflow-trigger"]').first();
  if (await trigger.count()) {
    await trigger.click();
    await page.waitForTimeout(350);
    const r = await reachable(page, '[data-testid="history-overflow-menu"]');
    const fit = await menuLabelsFit(page, '[data-testid="history-overflow-menu"]');
    if (r.ok && fit.ok) pass("history overflow menu");
    else fail("history overflow menu", r.ok ? fit.reason : r.reason);
    await trigger.click();
    await page.waitForTimeout(250);
  } else {
    fail("history overflow menu", "trigger not found");
  }

  console.log("\n▶ search palettes stay inside short windows");
  for (const height of [640, 400]) {
    await page.setViewportSize({ width: 960, height });
    for (const [name, shortcut] of [["quick open", "Control+p"], ["command palette", "Control+k"]]) {
      await page.keyboard.press(shortcut);
      await page.waitForTimeout(200);
      const result = await reachable(page, '[role="dialog"] > div');
      const bottomGap = await page.locator('[role="dialog"] > div').first().evaluate(
        (panel) => window.innerHeight - panel.getBoundingClientRect().bottom
      );
      if (result.ok && bottomGap >= 15) pass(`${name} at 960×${height}`);
      else fail(`${name} at 960×${height}`, result.ok ? `only ${bottomGap}px below panel` : result.reason);
      await page.keyboard.press("Escape");
    }
  }

  console.log("\n▶ shortcut sheet has one header and one scrolling body");
  await page.setViewportSize({ width: 960, height: 640 });
  await page.keyboard.press("Control+/");
  await page.waitForTimeout(200);
  const sheet = page.getByRole("dialog", { name: "键盘快捷键速查清单" });
  const sheetLayout = await sheet.evaluate((dialog) => {
    const panel = dialog.firstElementChild;
    const scrolls = Array.from(panel.querySelectorAll("*")).filter(
      (el) => el.scrollHeight > el.clientHeight + 1 && ["auto", "scroll"].includes(getComputedStyle(el).overflowY)
    );
    return {
      closeButtons: dialog.querySelectorAll('[aria-label="关闭"]').length,
      emptyHeadings: Array.from(panel.querySelectorAll("h2,h3")).filter((el) => !el.textContent.trim()).length,
      scrollContainers: scrolls.length,
      width: panel.getBoundingClientRect().width,
    };
  });
  if (sheetLayout.closeButtons === 1 && sheetLayout.emptyHeadings === 0 && sheetLayout.scrollContainers === 1 && sheetLayout.width >= 640) {
    pass("shortcut sheet layout");
  } else fail("shortcut sheet layout", JSON.stringify(sheetLayout));
  await sheet.getByRole("button", { name: "关闭", exact: true }).click();
  if (!(await sheet.count())) pass("shortcut sheet close");
  else fail("shortcut sheet close", "close button did not dismiss the dialog");

  if (errors.length) {
    console.log("\nconsole errors:");
    for (const e of errors) console.log(`  - ${e}`);
    failures.push(...errors.map((e) => `console: ${e}`));
  }

  await browser.close();

  if (failures.length) {
    console.log(`\npopover audit found ${failures.length} problem(s):`);
    for (const f of failures) console.log(`  - ${f}`);
    process.exit(1);
  }
  console.log("\npopover audit passed: every menu is visible and dismissible");
})();
