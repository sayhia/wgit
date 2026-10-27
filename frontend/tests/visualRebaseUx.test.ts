import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const historyViewFile = fileURLToPath(new URL("../src/components/HistoryView.vue", import.meta.url));
const batchCherryPickModalFile = fileURLToPath(new URL("../src/components/history/BatchCherryPickModal.vue", import.meta.url));
const interactiveRebaseModalFile = fileURLToPath(new URL("../src/components/history/InteractiveRebaseModal.vue", import.meta.url));

describe("Phase E2: Visual Graph Rewriting & Batch Cherry-Pick Planner UX", () => {
  describe("BatchCherryPickModal Contract & Features", () => {
    const src = readFileSync(batchCherryPickModalFile, "utf8");

    test("declares expected props and emits", () => {
      expect(src).toContain("isOpen: boolean");
      expect(src).toContain("repoPath: string");
      expect(src).toContain("commits: CommitItem[]");
      expect(src).toContain('(e: "close"): void');
      expect(src).toContain('(e: "success"): void');
      expect(src).toContain('(e: "notify", message: string): void');
    });

    test("implements chronological initialization and reverse order", () => {
      expect(src).toContain("initializeCommits");
      expect(src).toContain("new Date(a.date).getTime() - new Date(b.date).getTime()");
      expect(src).toContain("function reverseOrder()");
    });

    test("implements moveUp and moveDown commit reordering sequencer", () => {
      expect(src).toContain("function moveUp(index: number)");
      expect(src).toContain("function moveDown(index: number)");
    });

    test("integrates predictive conflict pre-check using gitService.predictMergeConflicts", () => {
      expect(src).toContain("gitService.predictMergeConflicts");
      expect(src).toContain("runConflictPreCheck");
      expect(src).toContain("prediction.hasConflicts");
      expect(src).toContain("prediction.riskScore");
    });

    test("executes batch cherry-pick with gitService.cherryPickMultiple", () => {
      expect(src).toContain("gitService.cherryPickMultiple");
      expect(src).toContain("handleExecute");
    });
  });

  describe("HistoryView Context Menu & Rebase Integration", () => {
    const src = readFileSync(historyViewFile, "utf8");

    test("imports and mounts BatchCherryPickModal", () => {
      expect(src).toContain('import BatchCherryPickModal from "./history/BatchCherryPickModal.vue";');
      expect(src).toContain("<BatchCherryPickModal");
      expect(src).toContain(':is-open="showBatchCherryPickDialog"');
      expect(src).toContain(':commits="selectedCommits"');
    });

    test("provides visual history rewrite actions in commit context menu", () => {
      expect(src).toContain('id: "rebase-squash-parent"');
      expect(src).toContain('id: "rebase-fixup-parent"');
      expect(src).toContain('id: "rebase-drop"');
      expect(src).toContain('t("history.menu.rebase.squash")');
      expect(src).toContain('t("history.menu.rebase.fixup")');
      expect(src).toContain('t("history.menu.rebase.drop")');
    });

    test("implements openRebaseDialogWithAction for single-click rewrites", () => {
      expect(src).toContain("function openRebaseDialogWithAction");
      expect(src).toContain("openRebaseDialog(base)");
    });
  });

  describe("InteractiveRebaseModal Visual Reordering", () => {
    const src = readFileSync(interactiveRebaseModalFile, "utf8");

    test("provides actions for pick, reword, edit, squash, fixup, drop", () => {
      expect(src).toContain('value: "pick"');
      expect(src).toContain('value: "reword"');
      expect(src).toContain('value: "edit"');
      expect(src).toContain('value: "squash"');
      expect(src).toContain('value: "fixup"');
      expect(src).toContain('value: "drop"');
    });

    test("supports moveUp and moveDown for commit reordering", () => {
      expect(src).toContain("function moveUp(index: number)");
      expect(src).toContain("function moveDown(index: number)");
    });
  });

  describe("i18n Parity for Visual Rebase and Cherry-Pick Planner", () => {
    const requiredKeys = [
      "history.menu.rebase.squash",
      "history.menu.rebase.fixup",
      "history.menu.rebase.drop",
      "history.batch.reverse",
      "history.batch.reverse.hint",
      "history.batch.risk.conflict",
      "history.batch.risk.low",
      "history.batch.risk.clean",
    ];

    test("zh-CN and en-US contain all required keys", () => {
      for (const k of requiredKeys) {
        expect(zhCN[k as keyof typeof zhCN]).toBeDefined();
        expect(enUS[k as keyof typeof enUS]).toBeDefined();
      }
    });
  });
});
