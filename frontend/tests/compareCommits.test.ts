import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const batchToolbarFile = fileURLToPath(
  new URL("../src/components/history/BatchActionToolbar.vue", import.meta.url)
);
const compareModalFile = fileURLToPath(
  new URL("../src/components/history/CompareCommitsModal.vue", import.meta.url)
);
const historyViewFile = fileURLToPath(
  new URL("../src/components/HistoryView.vue", import.meta.url)
);

describe("Compare 2 Selected Commits UX & Feature", () => {
  describe("BatchActionToolbar contract", () => {
    const src = readFileSync(batchToolbarFile, "utf8");

    test("declares compare-two emit", () => {
      expect(src).toContain('(e: "compare-two"): void;');
    });

    test("renders compare button when selectionCount === 2", () => {
      expect(src).toContain('v-if="selectionCount === 2"');
      expect(src).toContain("@click=\"emit('compare-two')\"");
      expect(src).toContain('text-cyan');
      expect(src).toContain('t("history.batch.compare.two")');
    });
  });

  describe("CompareCommitsModal contract", () => {
    const src = readFileSync(compareModalFile, "utf8");

    test("declares expected props", () => {
      expect(src).toContain("isOpen: boolean");
      expect(src).toContain("repoPath: string");
      expect(src).toContain("baseCommit: CommitItem | null");
      expect(src).toContain("targetCommit: CommitItem | null");
    });

    test("declares close emit", () => {
      expect(src).toContain('(e: "close"): void');
    });

    test("uses UiModal and calls gitService.getDiffBetween", () => {
      expect(src).toContain("<UiModal");
      expect(src).toContain("gitService.getDiffBetween");
    });

    test("displays header comparison, file list with pills & status badge, and diff preview", () => {
      expect(src).toContain("baseCommit?.shortHash");
      expect(src).toContain("targetCommit?.shortHash");
      expect(src).toContain("getFileStatusBadge");
      expect(src).toContain("file.additions");
      expect(src).toContain("file.deletions");
      expect(src).toContain("patchLines");
      expect(src).toContain("viewMode");
    });
  });

  describe("HistoryView integration", () => {
    const src = readFileSync(historyViewFile, "utf8");

    test("imports CompareCommitsModal", () => {
      expect(src).toContain('import CompareCommitsModal from "./history/CompareCommitsModal.vue";');
    });

    test("wires @compare-two handler on BatchActionToolbar", () => {
      expect(src).toContain('@compare-two="handleCompareTwoCommits"');
    });

    test("orders selected commits chronologically in handleCompareTwoCommits", () => {
      expect(src).toContain("function handleCompareTwoCommits()");
      expect(src).toContain("selectedHashes.value.size !== 2");
      expect(src).toContain("new Date(c1.date).getTime()");
      expect(src).toContain("compareBaseCommit.value");
      expect(src).toContain("compareTargetCommit.value");
      expect(src).toContain("showCompareModal.value = true");
    });

    test("mounts CompareCommitsModal with required props and emits", () => {
      expect(src).toContain("<CompareCommitsModal");
      expect(src).toContain(':is-open="showCompareModal"');
      expect(src).toContain(':repo-path="currentRepoPath"');
      expect(src).toContain(':base-commit="compareBaseCommit"');
      expect(src).toContain(':target-commit="compareTargetCommit"');
      expect(src).toContain('@close="showCompareModal = false"');
    });
  });

  describe("i18n key parity for Compare Commits", () => {
    const requiredKeys = [
      "history.batch.compare.two",
      "history.batch.compare.two.hint",
      "history.compare.modal.title",
      "history.compare.modal.files",
      "history.compare.modal.no.diff",
      "history.compare.modal.loading",
    ];

    for (const key of requiredKeys) {
      test(`key "${key}" exists in zh-CN and en-US with non-empty translation`, () => {
        const zh = (zhCN as Record<string, string>)[key];
        const en = (enUS as Record<string, string>)[key];
        expect(zh).toBeDefined();
        expect(zh.trim().length).toBeGreaterThan(0);
        expect(en).toBeDefined();
        expect(en.trim().length).toBeGreaterThan(0);
      });
    }
  });
});
