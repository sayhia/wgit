import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const rebaseModalFile = fileURLToPath(
  new URL("../src/components/history/InteractiveRebaseModal.vue", import.meta.url)
);
const historyViewFile = fileURLToPath(
  new URL("../src/components/HistoryView.vue", import.meta.url)
);

describe("Interactive Rebase Planner & Reordering UX", () => {
  describe("InteractiveRebaseModal contract", () => {
    const src = readFileSync(rebaseModalFile, "utf8");

    test("declares all required props", () => {
      expect(src).toContain("isOpen: boolean");
      expect(src).toContain("repoPath: string");
      expect(src).toContain("rebaseBase: string");
      expect(src).toContain("rebaseBaseOptions: { value: string; label: string }[]");
      expect(src).toContain("initialPlan: RebaseTodoItem[]");
      expect(src).toContain("isLoadingPlan: boolean");
      expect(src).toContain("planError: string | null");
      expect(src).toContain("selectionCount: number");
      expect(src).toContain("isHashSelected: (hash: string) => boolean");
      expect(src).toContain("isExecuting: boolean");
      expect(src).toContain("executionError: string | null");
    });

    test("declares all required emits", () => {
      expect(src).toContain('(e: "close"): void');
      expect(src).toContain('(e: "reload-plan"): void');
      expect(src).toContain('(e: "update:rebaseBase", base: string): void');
      expect(src).toContain('(e: "execute", plan: RebaseTodoItem[]): void');
    });

    test("implements commit reordering with Move Up / Move Down buttons and drag-and-drop", () => {
      expect(src).toContain("function moveUp");
      expect(src).toContain("function moveDown");
      expect(src).toContain("ArrowUp");
      expect(src).toContain("ArrowDown");
      expect(src).toContain("GripVertical");
      expect(src).toContain("handleDragStart");
      expect(src).toContain("handleDragOver");
      expect(src).toContain("handleDrop");
      expect(src).toContain("handleDragEnd");
    });

    test("implements safety validation preventing squash/fixup on row 0 with tooltip", () => {
      expect(src).toContain("ensureFirstCommitValid");
      expect(src).toContain("history.rebase.first.cannot.squash");
      expect(src).toContain("action.value === 'squash' || action.value === 'fixup'");
    });

    test("implements quick action buttons: All Pick, Reverse, Reset Plan", () => {
      expect(src).toContain("function setAllPick");
      expect(src).toContain("function reversePlan");
      expect(src).toContain("function resetPlan");
      expect(src).toContain('t("history.rebase.all.pick")');
      expect(src).toContain('t("history.rebase.reverse")');
      expect(src).toContain('t("history.rebase.reset.plan")');
    });

    test("renders execution spinner and error block", () => {
      expect(src).toContain("executionError");
      expect(src).toContain("animate-spin");
      expect(src).toContain('t("history.rebase.running")');
    });
  });

  describe("HistoryView integration", () => {
    const src = readFileSync(historyViewFile, "utf8");

    test("imports InteractiveRebaseModal", () => {
      expect(src).toContain(
        'import InteractiveRebaseModal from "./history/InteractiveRebaseModal.vue";'
      );
    });

    test("mounts InteractiveRebaseModal with required props and emits", () => {
      expect(src).toContain("<InteractiveRebaseModal");
      expect(src).toContain(':is-open="showRebaseDialog"');
      expect(src).toContain(':repo-path="currentRepoPath"');
      expect(src).toContain(':rebase-base="rebaseBase"');
      expect(src).toContain(':rebase-base-options="rebaseBaseOptions"');
      expect(src).toContain(':initial-plan="rebasePlan"');
      expect(src).toContain(':is-loading-plan="isLoadingRebasePlan"');
      expect(src).toContain(':plan-error="rebasePlanError"');
      expect(src).toContain(':selection-count="selectionCount"');
      expect(src).toContain(':is-hash-selected="isHashSelected"');
      expect(src).toContain(':is-executing="isExecutingRebase"');
      expect(src).toContain(':execution-error="rebaseError"');
      expect(src).toContain('@close="closeRebaseDialog"');
      expect(src).toContain('@reload-plan="loadRebasePlan"');
      expect(src).toContain('@update:rebase-base="rebaseBase = $event"');
      expect(src).toContain('@execute="executeRebase"');
    });

    test("executeRebase accepts reordered plan", () => {
      expect(src).toContain("async function executeRebase(planToExecute?: RebaseTodoItem[])");
      expect(src).toContain("planToExecute && planToExecute.length > 0 ? planToExecute : rebasePlan.value");
    });
  });

  describe("i18n key parity for Interactive Rebase", () => {
    const requiredKeys = [
      "history.rebase.move.up",
      "history.rebase.move.down",
      "history.rebase.all.pick",
      "history.rebase.reverse",
      "history.rebase.reset.plan",
      "history.rebase.first.cannot.squash",
    ];

    for (const key of requiredKeys) {
      test(`key "${key}" exists in zh-CN and en-US with non-empty translation`, () => {
        const zh = (zhCN as Record<string, string>)[key];
        const en = (enUS as Record<string, string>)[key];
        expect(zh).toBeDefined();
        expect(zh?.trim().length).toBeGreaterThan(0);
        expect(en).toBeDefined();
        expect(en?.trim().length).toBeGreaterThan(0);
      });
    }
  });
});
