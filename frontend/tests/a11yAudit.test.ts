/**
 * Accessibility audit regression suite (plan §2.1 follow-up round).
 *
 * Two kinds of assertions:
 *  1. Fix-point locks — every pure-attribute a11y fix applied in this audit
 *     round is pinned so a future refactor cannot silently drop it.
 *  2. Spot checks — the modal/focus infrastructure (useModalA11y, UiModal,
 *     ariaLabelFallback) and the highest-traffic interactive surface
 *     (CommandPalette) keep their contract, and the decorative-icon
 *     aria-hidden count in the touched files never decreases.
 *
 * The audit scope was src/components/** (except the four hot files
 * WorkspaceView.vue / DiffViewer.vue / HistoryView.vue / aiService.ts, which
 * were audited read-only and reported as backlog) plus src/composables/**.
 */
import { describe, test, expect } from "bun:test";
import { readFileSync } from "fs";
import { resolve } from "path";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const read = (rel: string) => readFileSync(resolve(__dirname, "..", rel), "utf-8");

// ---------------------------------------------------------------------------
// Fix points from the audit round (pure attribute additions only)
// ---------------------------------------------------------------------------

describe("a11y audit: fix-point locks", () => {
  test("StashView clear-search (✕) button has an accessible name", () => {
    const src = read("src/components/StashView.vue");
    expect(src).toContain(':aria-label="t(\'explorer.search.clear\')"');
  });

  test("StashView comparator ref1/ref2 inputs are programmatically labelled", () => {
    const src = read("src/components/StashView.vue");
    expect(src).toContain('for="wgit-comparator-ref1"');
    expect(src).toContain('id="wgit-comparator-ref1"');
    expect(src).toContain('for="wgit-comparator-ref2"');
    expect(src).toContain('id="wgit-comparator-ref2"');
  });

  test("MergeBranchModal strategy select and message input are labelled", () => {
    const src = read("src/components/branches/modals/MergeBranchModal.vue");
    expect(src).toContain('for="wgit-merge-strategy"');
    expect(src).toContain('id="wgit-merge-strategy"');
    expect(src).toContain('for="wgit-merge-message"');
    expect(src).toContain('id="wgit-merge-message"');
  });

  test("ShortcutCheatSheetModal: named dialog, named close and clear buttons", () => {
    const src = read("src/components/ShortcutCheatSheetModal.vue");
    // dialog gets a real accessible name instead of the common.confirm fallback
    expect(src).toContain(':label="t(\'shortcuts.cheatsheet.title\')"');
    expect(src).toContain(':aria-label="t(\'common.close\')"');
    expect(src).toContain(':aria-label="t(\'explorer.search.clear\')"');
  });

  test("ShortcutCheatSheetModal search input has an explicit aria-label", () => {
    const src = read("src/components/ShortcutCheatSheetModal.vue");
    expect(src).toContain(':aria-label="t(\'shortcuts.cheatsheet.search.placeholder\')"');
  });

  test("HeaderBar save-context-session icon button has an accessible name", () => {
    const src = read("src/components/HeaderBar.vue");
    const buttonStart = src.indexOf('data-testid="save-context-session-btn"');
    expect(buttonStart).toBeGreaterThan(0);
    const tagStart = src.lastIndexOf("<button", buttonStart);
    const tagEnd = src.indexOf(">", buttonStart);
    const openTag = src.slice(tagStart, tagEnd);
    expect(openTag).toContain(":aria-label=");
  });

  test("BranchesView sort select exposes an accessible name (title alone is hover-only)", () => {
    const src = read("src/components/BranchesView.vue");
    expect(src).toContain(':aria-label="t(\'branch.sort.title\')"');
  });

  test("SettingsView search input has an explicit aria-label", () => {
    const src = read("src/components/SettingsView.vue");
    expect(src).toContain(':aria-label="t(\'settings.search.placeholder\')"');
  });

  test("every i18n key reused by the fixes exists in both locales", () => {
    const keys = [
      "common.close",
      "common.save",
      "explorer.search.clear",
      "branch.sort.title",
      "settings.search.placeholder",
      "shortcuts.cheatsheet.title",
      "shortcuts.cheatsheet.search.placeholder",
    ];
    for (const key of keys) {
      expect(zhCN[key as keyof typeof zhCN]).toBeDefined();
      expect(enUS[key as keyof typeof enUS]).toBeDefined();
    }
  });
});

// ---------------------------------------------------------------------------
// Spot checks: infrastructure + highest-traffic surfaces
// ---------------------------------------------------------------------------

describe("a11y audit: infrastructure spot checks", () => {
  test("UiModal keeps role=dialog + aria-modal and a labelled close button", () => {
    const src = read("src/components/ui/UiModal.vue");
    expect(src).toContain('role="dialog"');
    expect(src).toContain('aria-modal="true"');
    expect(src).toContain(':aria-label="label || title');
    expect(src).toContain(`:aria-label="t('common.close')"`);
  });

  test("useModalA11y keeps the focus trap (Escape close + Tab wrap) and focus restore", () => {
    const src = read("src/composables/useModalA11y.ts");
    expect(src).toContain('"Escape"');
    expect(src).toContain('"Tab"');
    expect(src).toContain("previousFocus.focus()");
    expect(src).toContain("addEventListener");
  });

  test("ariaLabelFallback still covers placeholder-only controls app-wide", () => {
    const src = read("src/composables/ariaLabelFallback.ts");
    expect(src).toContain("installAriaLabelFallback");
    expect(src).toContain('setAttribute("aria-label", placeholder)');
  });

  test("ConfirmDialog labels its type-to-confirm input", () => {
    const src = read("src/components/ui/ConfirmDialog.vue");
    expect(src).toContain('for="confirm-require-text"');
    expect(src).toContain('id="confirm-require-text"');
  });

  test("UiContextMenu keeps menu semantics (role=menu / menuitem / aria-disabled)", () => {
    const src = read("src/components/ui/UiContextMenu.vue");
    expect(src).toContain('role="menu"');
    expect(src).toContain('role="menuitem"');
    expect(src).toContain(":aria-disabled=");
  });

  test("Toast keeps a live region for async status announcements", () => {
    const src = read("src/components/ui/Toast.vue");
    expect(src).toContain('aria-live="polite"');
  });
});

describe("a11y audit: high-traffic component spot checks", () => {
  test("CommandPalette keeps combobox/listbox wiring inside a labelled dialog", () => {
    const src = read("src/components/CommandPalette.vue");
    expect(src).toContain('role="combobox"');
    expect(src).toContain('aria-autocomplete="list"');
    expect(src).toContain(':aria-controls="\'palette-listbox\'"');
    expect(src).toContain(":aria-activedescendant=");
    expect(src).toContain('role="listbox"');
    expect(src).toContain(`:label="t('cmd.aria')"`);
    // every palette row action button keeps an explicit accessible name
    expect(src).toContain(`:aria-label="t('cmd.pin.up')"`);
    expect(src).toContain(`:aria-label="t('cmd.pin.down')"`);
    expect(src).toContain(`:aria-label="isPinned(entry.cmd.id) ? t('cmd.pin.remove') : t('cmd.pin.add')"`);
  });

  test("BranchesView refresh/drift icon buttons keep accessible names", () => {
    const src = read("src/components/BranchesView.vue");
    expect(src).toContain(`:aria-label="t('branch.refresh')"`);
    expect(src).toContain('data-testid="branch-drift-inspect-btn"');
    expect(src).toContain('role="alert"');
  });

  test("ReviewView header icon buttons keep accessible names and pressed state", () => {
    const src = read("src/components/ReviewView.vue");
    expect(src).toContain(`:aria-label="t('review.batch.approve')"`);
    expect(src).toContain(`:aria-label="t('review.refresh')"`);
    expect(src).toContain(":aria-pressed=");
  });

  test("StashView header icon buttons keep accessible names", () => {
    const src = read("src/components/StashView.vue");
    expect(src).toContain(`:aria-label="t('stash.refresh')"`);
    expect(src).toContain(`:aria-label="t('stash.snapshot.btn')"`);
    expect(src).toContain(`:aria-label="t('stash.comparator.title')"`);
    expect(src).toContain(`:aria-label="t('stash.clear.title')"`);
  });

  test("ToolsView primary tabs and more-menu trigger keep names/expanded state", () => {
    const src = read("src/components/ToolsView.vue");
    expect(src).toContain(':aria-label="t(def.labelKey)"');
    expect(src).toContain(":aria-expanded=");
  });
});

describe("a11y audit: decorative icon aria-hidden counts never decrease", () => {
  const floors: Record<string, number> = {
    // files edited in this round: the fix added aria-hidden markers
    "src/components/BranchesView.vue": 4,
    "src/components/branches/modals/MergeBranchModal.vue": 2,
    "src/components/ShortcutCheatSheetModal.vue": 3,
    "src/components/HeaderBar.vue": 1,
    // file already carrying the convention before this round
    "src/components/ui/Toast.vue": 1,
  };

  for (const [file, floor] of Object.entries(floors)) {
    if (floor === 0) continue;
    test(`${file} keeps >= ${floor} aria-hidden decorative markers`, () => {
      const src = read(file);
      const count = (src.match(/aria-hidden/g) ?? []).length;
      expect(count).toBeGreaterThanOrEqual(floor);
    });
  }
});
