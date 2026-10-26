import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const historyViewFile = fileURLToPath(
  new URL("../src/components/HistoryView.vue", import.meta.url)
);
const batchToolbarFile = fileURLToPath(
  new URL("../src/components/history/BatchActionToolbar.vue", import.meta.url)
);
const compareModalFile = fileURLToPath(
  new URL("../src/components/history/CompareCommitsModal.vue", import.meta.url)
);
const graphListFile = fileURLToPath(
  new URL("../src/components/history/CommitGraphList.vue", import.meta.url)
);

describe("Commit DAG Graph UX & Range Selection (Round 22)", () => {
  describe("HistoryView Context Menu Standardization", () => {
    const src = readFileSync(historyViewFile, "utf8");

    test("adopts useContextMenu and mounts UiContextMenu", () => {
      expect(src).toContain('import UiContextMenu from "./ui/UiContextMenu.vue";');
      expect(src).toContain('import { useContextMenu, type ContextMenuItem } from "../composables/useContextMenu";');
      expect(src).toContain("<UiContextMenu");
      expect(src).toContain(':open="contextMenuOpen"');
      expect(src).toContain(':items="contextMenuItems"');
      expect(src).toContain('@select="handleContextMenuSelect"');
      expect(src).toContain('@close="closeContextMenu"');
    });

    test("includes single-commit context menu items: checkout detached, revert to worktree, export patch", () => {
      expect(src).toContain('id: "checkout-detached"');
      expect(src).toContain('id: "revert-no-commit"');
      expect(src).toContain('id: "export-patch"');
      expect(src).toContain("handleCheckoutDetached");
      expect(src).toContain("handleRevertNoCommit");
      expect(src).toContain("handleExportCommitPatch");
      expect(src).toContain("gitService.exportPatch");
    });

    test("preserves all original single-commit actions", () => {
      expect(src).toContain('id: "rebase-onto"');
      expect(src).toContain('id: "rebase-interactive"');
      expect(src).toContain('id: "cherry-pick"');
      expect(src).toContain('id: "revert"');
      expect(src).toContain('id: "branch"');
      expect(src).toContain('id: "tag"');
      expect(src).toContain('id: "reset"');
      expect(src).toContain('id: "copy-hash"');
      expect(src).toContain('id: "copy-subject"');
      expect(src).toContain('id: "open-web"');
    });

    test("provides multi-select context menu for selectionCount > 1", () => {
      expect(src).toContain("selectedHashes.value.has(commit.hash) && selectedHashes.value.size > 1");
      expect(src).toContain("buildMultiSelectMenuItems");
      expect(src).toContain('id: "compare-range"');
      expect(src).toContain('id: "export-patch-range"');
      expect(src).toContain('id: "copy-diff-range"');
      expect(src).toContain('id: "batch-cherry-pick"');
      expect(src).toContain('id: "copy-all-hashes"');
      expect(src).toContain('id: "clear-selection"');
      expect(src).toContain("handleExportSelectedRangePatch");
      expect(src).toContain("gitService.exportPatchRange");
      expect(src).toContain("handleCopySelectedRangeDiff");
      expect(src).toContain("gitService.generatePatchRange");
      expect(src).toContain("handleCopyAllSelectedHashes");
      expect(src).toContain("clearSelection");
    });
  });

  describe("Reset Branch Modal & Safety Defense", () => {
    const src = readFileSync(historyViewFile, "utf8");

    test("clearly displays currentBranchName and target commit", () => {
      expect(src).toContain("currentBranchName");
      expect(src).toContain("t(\"history.dialog.reset.branch.hint\"");
      expect(src).toContain("selectedCommit?.shortHash");
      expect(src).toContain("selectedCommit.subject");
    });

    test("eliminates double modal (modal-on-modal) for hard reset", () => {
      // In handleResetBranch, no confirmAction popup is triggered for hard reset
      const resetFnBody = src.slice(src.indexOf("async function handleResetBranch()"), src.indexOf("function handleCopyBatchDetails"));
      expect(resetFnBody).not.toContain("confirmAction");
      expect(resetFnBody).toContain("gitService.resetBranch");
    });

    test("renders inline danger warning and working copy dirty defense in hard mode", () => {
      expect(src).toContain("v-if=\"resetMode === 'hard'\"");
      expect(src).toContain("t(\"history.reset.hard.inline.warn\")");
      expect(src).toContain("!isWorkingTreeClean");
      expect(src).toContain("t(\"history.reset.hard.dirty.warn\")");
      expect(src).toContain("gitService.getRepoStatus");
    });
  });

  describe("Multi-Select Toolbar & Compare Commits Modal", () => {
    const toolbarSrc = readFileSync(batchToolbarFile, "utf8");
    const compareModalSrc = readFileSync(compareModalFile, "utf8");

    test("BatchActionToolbar relaxes compare to selectionCount >= 2 and adds export-patch", () => {
      expect(toolbarSrc).toContain("selectionCount >= 2");
      expect(toolbarSrc).toContain('(e: "export-patch"): void;');
      expect(toolbarSrc).toContain("@click=\"emit('export-patch')\"");
      expect(toolbarSrc).toContain('t("history.batch.export.patch")');
    });

    test("CompareCommitsModal supports Swap Base <-> Target", () => {
      expect(compareModalSrc).toContain("handleSwapBaseTarget");
      expect(compareModalSrc).toContain('(e: "swap"');
      expect(compareModalSrc).toContain('t("history.compare.modal.swap")');
    });

    test("CompareCommitsModal supports Export Patch and Copy Patch shortcuts", () => {
      expect(compareModalSrc).toContain("handleExportPatch");
      expect(compareModalSrc).toContain("handleCopyPatch");
      expect(compareModalSrc).toContain("gitService.exportPatchRange");
      expect(compareModalSrc).toContain("gitService.generatePatchRange");
      expect(compareModalSrc).toContain('t("history.compare.modal.export.patch")');
      expect(compareModalSrc).toContain('t("history.compare.modal.copy.patch")');
    });
  });

  describe("Commit DAG Graph Lane Rendering", () => {
    const graphSrc = readFileSync(graphListFile, "utf8");

    test("adaptive Bezier curve vertical shoulder clears intervening nodes", () => {
      expect(graphSrc).toContain("const laneDiff = Math.abs(fromLane - toLane);");
      expect(graphSrc).toContain("Math.min(laneDiff * 3, 16)");
      expect(graphSrc).toContain("push(`M ${x1} ${midY} C ${x1} ${shoulder}, ${x2} ${shoulder}, ${x2} ${ROW_HEIGHT}`, color);");
    });

    test("enforces reasonable graph gutter width upper bound against excessive squeezing", () => {
      expect(graphSrc).toContain("MAX_GRAPH_GUTTER_WIDTH");
      expect(graphSrc).toContain("Math.min(calculated, MAX_GRAPH_GUTTER_WIDTH)");
    });
  });

  describe("i18n Symmetry & Coverage for Round 22 Keys", () => {
    const requiredKeys = [
      "history.menu.checkout.detached",
      "history.menu.revert.nocommit",
      "history.menu.export.patch",
      "history.menu.range.compare",
      "history.menu.range.export.patch",
      "history.menu.range.copy.diff",
      "history.menu.range.batch.cherry.pick",
      "history.menu.range.copy.hashes",
      "history.menu.range.clear.selection",
      "history.batch.compare.range",
      "history.batch.compare.range.hint",
      "history.batch.export.patch",
      "history.batch.export.patch.hint",
      "history.compare.modal.swap",
      "history.compare.modal.swap.hint",
      "history.compare.modal.export.patch",
      "history.compare.modal.export.patch.hint",
      "history.compare.modal.copy.patch",
      "history.compare.modal.copy.patch.hint",
      "history.compare.toast.patch.copied",
      "history.compare.toast.patch.exported",
      "history.compare.toast.patch.export.failed",
      "history.dialog.reset.branch.hint",
      "history.dialog.reset.target.hint",
      "history.reset.hard.inline.warn",
      "history.reset.hard.dirty.warn",
      "history.toast.patch.exported",
      "history.toast.patch.export.failed",
      "history.toast.patch.range.exported",
      "history.toast.patch.range.copied",
      "history.toast.hashes.copied",
      "history.toast.copy.failed",
    ];

    for (const key of requiredKeys) {
      test(`key "${key}" exists symmetrically in zh-CN and en-US with non-empty translation`, () => {
        const zh = (zhCN as Record<string, string>)[key];
        const en = (enUS as Record<string, string>)[key];
        expect(zh).toBeDefined();
        expect(en).toBeDefined();
        expect(zh!.trim().length).toBeGreaterThan(0);
        expect(en!.trim().length).toBeGreaterThan(0);
      });
    }
  });
});
