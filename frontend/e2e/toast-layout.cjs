#!/usr/bin/env node
/**
 * Toast/modal coordination against a Vite dev server. Imports the same store
 * URLs as the rendered components to create deterministic local-only notices;
 * it never runs repository mutations. Main production-compatible modal-layout
 * checks also cover the genuine language-switch notification.
 */
const { chromium } = require("playwright");
const fs = require("node:fs");
const path = require("node:path");
const BASE = process.env.BASE_URL || "http://127.0.0.1:9245/";
const OUT = path.join(__dirname, "out");
const CASES = ["command", "quick", "help", "sync", "rebase", "confirm"];
async function moduleUrl(page, component, module) {
  const source = await (await page.request.get(`${BASE.replace(/\/$/, "")}/src/components/${component}.vue`)).text();
  const escaped = module.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const match = source.match(new RegExp(`from "([^"\\n]*${escaped}[^"\\n]*)"`));
  if (!match) throw new Error(`toast-layout requires the Vite dev server; could not locate ${module}`);
  return match[1];
}
async function waitForLayout(page) {
  await page.evaluate(async () => {
    await new Promise(requestAnimationFrame);
    await new Promise(requestAnimationFrame);
    await Promise.all(document.getAnimations().filter((animation) => animation instanceof CSSTransition).map((animation) => animation.finished.catch(() => {})));
  });
}
async function open(page, name) {
  if (name === "sync") {
    const title = await page.locator('header button[title]').filter({ visible: true }).evaluateAll((buttons) => buttons.map((button) => button.title).find((value) => /高级推送设置|Advanced push/i.test(value)));
    await page.getByRole("button", { name: title, exact: true }).click();
  } else if (name === "confirm") {
    await page.evaluate(() => {
      void window.__toastLayout.confirm.confirmAction({ title: "Read-only preview", message: "Confirm layout preview", detail: "Long read-only context. ".repeat(40), confirmLabel: "Preview confirmation", requireText: "preview" });
    });
  } else await page.keyboard.press(({ command: "Control+k", quick: "Control+p", help: "Control+/", rebase: "Control+r" })[name]);
  await page.locator('[role="dialog"]').waitFor({ state: "visible" });
  await waitForLayout(page);
}
async function measure(page) {
  return page.locator('[role="dialog"] > div').evaluate((panel) => {
    const box = panel.getBoundingClientRect();
    const stack = document.querySelector('[data-testid="toast-stack"]');
    const stackBox = stack.getBoundingClientRect();
    const newest = Array.from(stack.querySelectorAll('[data-toast-id]')).at(-1);
    const newestBox = newest?.getBoundingClientRect();
    const undo = newest?.querySelector('[data-testid="toast-undo-btn"]');
    const undoBox = undo?.getBoundingClientRect();
    const footer = Array.from(panel.children).findLast((element) => String(element.className).includes("border-t"));
    const buttons = footer ? Array.from(footer.querySelectorAll("button")) : [];
    const actions = buttons.map((button) => {
      const rect = button.getBoundingClientRect();
      return { text: button.textContent.trim(), hit: button.contains(document.elementFromPoint(rect.left + rect.width / 2, rect.top + rect.height / 2)), inside: rect.top >= box.top && rect.bottom <= box.bottom };
    });
    return {
      panel: { top: box.top, bottom: box.bottom, height: box.height },
      stack: { top: stackBox.top, height: stackBox.height, client: stack.clientHeight, scroll: stack.scrollHeight, overflow: getComputedStyle(stack).overflowY },
      reserve: parseFloat(getComputedStyle(document.documentElement).getPropertyValue("--ug-toast-reserve")) || 0,
      actions,
      inside: box.top >= 0 && box.bottom <= innerHeight && box.right <= innerWidth && box.left >= 0,
      errorCount: stack.querySelectorAll('[role="alert"]').length,
      newestStartVisible: !newestBox || newestBox.top >= stackBox.top - 1 && newestBox.top < stackBox.bottom,
      undoInitiallyVisible: !undoBox || undoBox.top >= stackBox.top && undoBox.bottom <= stackBox.bottom,
    };
  });
}

(async () => {
  fs.mkdirSync(OUT, { recursive: true });
  const browser = await chromium.launch({ channel: "chrome" });
  const rows = [];
  const failures = [];
  try {
    for (const [width, height] of [[960, 400], [390, 640]]) {
      const page = await browser.newPage({ viewport: { width, height } });
      await page.goto(BASE, { waitUntil: "networkidle" });
      const toastUrl = await moduleUrl(page, "ui/Toast", "useToast.ts");
      const confirmUrl = await moduleUrl(page, "ui/ConfirmDialog", "useConfirm.ts");
      await page.evaluate(async ({ toastUrl, confirmUrl }) => {
        window.__toastLayout = { toast: await import(toastUrl), confirm: await import(confirmUrl), undone: false };
        window.__toastLayout.toast.resetToasts();
      }, { toastUrl, confirmUrl });
      for (const name of CASES) {
        await open(page, name);
        const baseline = await measure(page);
        if (baseline.reserve !== 0) throw new Error("Empty toast stack reserved modal space");
        await page.evaluate(() => window.__toastLayout.toast.pushToast("A short status notice", { duration: 0 }));
        await page.waitForFunction(() => parseFloat(document.documentElement.style.getPropertyValue("--ug-toast-reserve")) > 0);
        await waitForLayout(page);
        const single = await measure(page);
        if (!single.inside || single.panel.bottom > single.stack.top - 7 || single.actions.some((action) => !action.hit || !action.inside)) failures.push({ name, width, height, stage: "single", single });

        await page.evaluate(() => {
          const { toast } = window.__toastLayout;
          toast.resetToasts();
          for (let index = 0; index < 3; index += 1) toast.pushToast(`Provider failure ${index + 1}: ${"A long diagnostic with a complete cause and recovery hint. ".repeat(16)}`, {
            tone: "danger", duration: 0,
            action: index === 2 ? { label: "Undo", run: () => { window.__toastLayout.undone = true; } } : undefined,
          });
        });
        await page.waitForFunction(() => document.querySelectorAll('[data-testid="toast-item"][role="alert"]').length === 3);
        await waitForLayout(page);
        const multi = await measure(page);
        if (!multi.inside || multi.panel.bottom > multi.stack.top - 7 || multi.stack.height > height * 0.3 + 1 || multi.stack.scroll <= multi.stack.client || multi.errorCount !== 3 || !multi.newestStartVisible || !multi.undoInitiallyVisible || multi.actions.some((action) => !action.hit || !action.inside)) failures.push({ name, width, height, stage: "three-long-errors", multi });
        if (name === "sync" && width === 960 || name === "quick" && width === 390) await page.screenshot({ path: path.join(OUT, `toast-layout-${name}-${width}x${height}.png`) });
        await page.getByTestId("toast-undo-btn").click();
        if (!(await page.evaluate(() => window.__toastLayout.undone))) throw new Error("Toast Undo stopped working");
        await page.evaluate(() => { window.__toastLayout.undone = false; window.__toastLayout.toast.resetToasts(); });
        await page.waitForFunction(() => document.querySelectorAll('[data-testid="toast-item"]').length === 0 && !document.documentElement.style.getPropertyValue("--ug-toast-reserve"));
        await waitForLayout(page);
        const restored = await measure(page);
        if (Math.abs(restored.panel.top - baseline.panel.top) > 1 || Math.abs(restored.panel.bottom - baseline.panel.bottom) > 1) failures.push({ name, width, height, stage: "restored", baseline, restored });
        rows.push({ name, width, height, baseline, single, multi, restored });
        await page.keyboard.press("Escape");
        console.log(`  ✓ ${width}×${height} ${name}: no overlap, bounded stack, Undo, restored layout`);
      }
      // Real pointer entry must continue freezing the dismiss timer while read.
      await page.evaluate(() => window.__toastLayout.toast.pushToast("Hover keeps this notice visible", { duration: 500 }));
      const timed = page.getByTestId("toast-item").last();
      await timed.hover();
      await page.waitForTimeout(650);
      if (!(await timed.count())) throw new Error("Hover did not pause Toast dismissal");
      await page.mouse.move(2, 2);
      await timed.waitFor({ state: "detached", timeout: 2000 });
      await page.close();
    }
    fs.writeFileSync(path.join(OUT, "toast-layout-report.json"), JSON.stringify({ rows, failures }, null, 2));
  } finally { await browser.close(); }
  if (failures.length) throw new Error(`Toast/modal layout findings:\n${JSON.stringify(failures, null, 2)}`);
  console.log("toast layout passed: default/bare, top/center, Sync/Rebase/Confirm, short/narrow windows");
})().catch((error) => { console.error(error.message || error); process.exitCode = 1; });
