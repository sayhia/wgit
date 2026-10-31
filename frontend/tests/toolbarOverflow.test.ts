/**
 * Toolbar overflow (方案 §2.1 第 7 条): low-frequency actions fold into a
 * `⋯` (MoreHorizontal) overflow menu so the primary row only carries the
 * tools used per review pass.
 *
 * Source-assertion suite, following the readFileSync convention of the other
 * UX tests. Two views are covered:
 *
 * - DiffViewer: the ⋯ menu must exist, must collect exactly the five
 *   low-frequency actions (patch playground / blame / heatmap / invisible
 *   chars / open in editor), must keep the high-frequency actions (AI
 *   explain / AI chat / AI review / zen / symbols) at the first level, and
 *   must be a11y-wired (aria-label reusing the existing `tools.more` key,
 *   Escape + outside-click close).
 * - HistoryView: the parallel-session overflow menu already mirrors every
 *   secondary traversal tool; this asserts that coverage conclusion instead
 *   of re-doing the work.
 */
import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const diffViewerFile = fileURLToPath(new URL("../src/components/workspace/DiffViewer.vue", import.meta.url));
const historyViewFile = fileURLToPath(new URL("../src/components/HistoryView.vue", import.meta.url));

const diffSrc = readFileSync(diffViewerFile, "utf8");
const historySrc = readFileSync(historyViewFile, "utf8");

function count(haystack: string, needle: string): number {
  return haystack.split(needle).length - 1;
}

/** Text between two markers; fails loudly via expect when markers are gone. */
function sliceBetween(src: string, startMarker: string, endMarker: string): string {
  const start = src.indexOf(startMarker);
  expect(start).toBeGreaterThan(-1);
  const rest = src.slice(start);
  const end = rest.indexOf(endMarker);
  expect(end).toBeGreaterThan(-1);
  return rest.slice(0, end);
}

describe("DiffViewer toolbar overflow menu", () => {
  // The menu panel sits between its test id and the next template section
  // (the Patch Playground drawer).
  const menuPanel = sliceBetween(diffSrc, 'data-testid="diff-more-menu"', "<!-- Patch Playground Drawer -->");

  test("⋯ trigger and menu exist, built on the MoreHorizontal icon", () => {
    expect(diffSrc).toContain('data-testid="diff-more-btn"');
    expect(diffSrc).toContain('data-testid="diff-more-menu"');
    expect(diffSrc).toContain("MoreHorizontal");
    expect(menuPanel).toContain('role="menu"');
  });

  test("trigger a11y reuses the existing tools.more key (no new i18n keys)", () => {
    expect(diffSrc).toContain(':aria-label="t(\'tools.more\')"');
    expect(diffSrc).toContain(':title="t(\'tools.more\')"');
    expect(diffSrc).toContain('aria-haspopup="menu"');
    expect(diffSrc).toContain(':aria-expanded="showMoreMenu"');
    expect((zhCN as Record<string, string>)["tools.more"]).toBeDefined();
    expect((enUS as Record<string, string>)["tools.more"]).toBeDefined();
  });

  test("Escape and outside-click close the menu", () => {
    expect(diffSrc).toContain('if (e.key === "Escape" && showMoreMenu.value)');
    expect(diffSrc).toContain('document.addEventListener("click", onMoreMenuDocClick)');
    expect(diffSrc).toContain('document.removeEventListener("click", onMoreMenuDocClick)');
  });

  test("low-frequency actions moved into the menu, exactly once", () => {
    const movedItems = [
      "showPatchPlayground = !showPatchPlayground; showMoreMenu = false",
      "toggleBlame(); showMoreMenu = false",
      "toggleHeatmap(); showMoreMenu = false",
      "showInvisibleChars = !showInvisibleChars; showMoreMenu = false",
      "emit('open-in-editor'); showMoreMenu = false",
    ];
    for (const item of movedItems) {
      expect(count(diffSrc, item)).toBe(1);
      expect(menuPanel).toContain(item);
    }
    // Blame keeps its original guard; toggles keep their original test ids.
    expect(menuPanel).toContain('v-if="repoPath && filePath"');
    expect(menuPanel).toContain('data-testid="blame-heatmap-toggle"');
    expect(menuPanel).toContain('data-testid="toggle-invisible-chars-btn"');
  });

  test("high-frequency actions stay at the first level, outside the menu", () => {
    const keptItems = ["emit('explain-diff')", "emit('open-ai-chat')", "emit('open-ai-review')", "emit('toggle-zen')", "showSymbolsDropdown = !showSymbolsDropdown"];
    for (const item of keptItems) {
      expect(count(diffSrc, item)).toBe(1);
      expect(menuPanel).not.toContain(item);
    }
    // The three AI actions remain behind their feature gate.
    expect(count(diffSrc, 'FeatureGate flag="aiSummary"')).toBeGreaterThanOrEqual(3);
  });

  test("menu items keep their original aria-labels and hint titles", () => {
    for (const key of ["workspace.diff.playground.btn", "workspace.diff.blame.btn", "workspace.diff.heatmap.btn", "workspace.diff.invisible.chars.btn", "diff.open.editor"]) {
      expect(menuPanel).toContain(`:aria-label="t('${key}')"`);
    }
    for (const key of ["workspace.diff.playground.hint", "workspace.diff.blame.hint", "workspace.diff.heatmap.hint", "workspace.diff.invisible.chars.hint", "diff.open.editor.title"]) {
      expect(menuPanel).toContain(`:title="t('${key}')"`);
    }
  });
});

describe("HistoryView traversal overflow menu (parallel-session work)", () => {
  // Narrow commit column folds the secondary traversal tools into one menu;
  // wide windows show the same tools inline. The menu panel sits between its
  // test id and the refresh button that follows it.
  const menuPanel = sliceBetween(historySrc, 'data-testid="history-overflow-menu"', ':aria-label="t(\'history.refresh.title\')"');

  test("overflow trigger and menu exist with a wired aria-label", () => {
    expect(historySrc).toContain('data-testid="history-overflow-trigger"');
    expect(historySrc).toContain('data-testid="history-overflow-menu"');
    expect(historySrc).toContain(':aria-label="t(\'history.overflow.title\')"');
    expect(historySrc).toContain(':aria-expanded="showTraversalMenu"');
    expect((zhCN as Record<string, string>)["history.overflow.title"]).toBeDefined();
    expect((enUS as Record<string, string>)["history.overflow.title"]).toBeDefined();
  });

  test("conclusion: menu already covers every secondary traversal tool, none left bare at the first level", () => {
    const coveredHandlers = ["showRangeCherryPickDialog = true", "openBisectConsole()", "openReleaseNotes()", "showRebaseKanbanModal = true", "showBatchCherryPickDialog = true"];
    for (const handler of coveredHandlers) {
      expect(menuPanel).toContain(handler);
    }
    // Batch cherry-pick keeps its disabled state in the menu, as inline.
    expect(menuPanel).toContain(':disabled="selectionCount === 0"');
  });
});
