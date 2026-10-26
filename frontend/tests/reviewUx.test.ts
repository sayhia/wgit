import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const reviewViewFile = fileURLToPath(
  new URL("../src/components/ReviewView.vue", import.meta.url)
);
const changeListFile = fileURLToPath(
  new URL("../src/components/review/ChangeList.vue", import.meta.url)
);
const fileDetailPanelFile = fileURLToPath(
  new URL("../src/components/review/FileDetailPanel.vue", import.meta.url)
);

describe("Code Review & Changeset Session UX (Round 14)", () => {
  describe("ChangeList Filter Chips", () => {
    const src = readFileSync(changeListFile, "utf8");

    test("implements status filter chips and filteredFiles computed", () => {
      expect(src).toContain("filterStatus");
      expect(src).toContain("filteredFiles");
      expect(src).toContain("review.filter.all");
      expect(src).toContain("review.filter.pending");
      expect(src).toContain("review.filter.approved");
      expect(src).toContain("review.filter.changes");
    });
  });

  describe("FileDetailPanel Approve & Next", () => {
    const src = readFileSync(fileDetailPanelFile, "utf8");

    test("declares approve-and-next emit and action button", () => {
      expect(src).toContain('(e: "approve-and-next"): void;');
      expect(src).toContain("@click=\"emit('approve-and-next')\"");
      expect(src).toContain("review.file.approve.next");
    });
  });

  describe("ReviewView Batch Operations & Navigation", () => {
    const src = readFileSync(reviewViewFile, "utf8");

    test("implements batch approval and reset", () => {
      expect(src).toContain("handleBatchSetStatus");
      expect(src).toContain("review.batch.approve");
      expect(src).toContain("review.batch.reset");
      expect(src).toContain("review.confirm.batch.approve");
      expect(src).toContain("review.confirm.batch.reset");
    });

    test("implements keyboard navigation and adjacent file selection", () => {
      expect(src).toContain("selectAdjacentFile");
      expect(src).toContain("handleApproveAndNext");
      expect(src).toContain("e.altKey && e.key === \"ArrowUp\"");
      expect(src).toContain("e.altKey && e.key === \"ArrowDown\"");
    });
  });

  describe("i18n completeness for Code Review", () => {
    const requiredKeys = [
      "review.batch.approve",
      "review.batch.approve.title",
      "review.batch.reset",
      "review.batch.reset.title",
      "review.confirm.batch.approve",
      "review.confirm.batch.reset",
      "review.toast.batch.approved",
      "review.toast.batch.reset",
      "review.file.approve.next",
      "review.file.approve.next.title",
      "review.filter.all",
      "review.filter.pending",
      "review.filter.approved",
      "review.filter.changes",
    ];

    test("zh-CN contains all required Round 14 keys", () => {
      const catalog = zhCN as Record<string, string>;
      for (const key of requiredKeys) {
        expect(catalog[key]).toBeDefined();
        expect(catalog[key].length).toBeGreaterThan(0);
      }
    });

    test("en-US contains all required Round 14 keys", () => {
      const catalog = enUS as Record<string, string>;
      for (const key of requiredKeys) {
        expect(catalog[key]).toBeDefined();
        expect(catalog[key].length).toBeGreaterThan(0);
      }
    });
  });
});
