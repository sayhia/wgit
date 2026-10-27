import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const kanbanModalFile = fileURLToPath(
  new URL("../src/components/history/RebaseKanbanModal.vue", import.meta.url)
);
const historyViewFile = fileURLToPath(
  new URL("../src/components/HistoryView.vue", import.meta.url)
);
const interactiveRebaseModalFile = fileURLToPath(
  new URL("../src/components/history/InteractiveRebaseModal.vue", import.meta.url)
);
const gitServiceFile = fileURLToPath(
  new URL("../src/services/gitService.ts", import.meta.url)
);
const typesFile = fileURLToPath(
  new URL("../src/types.ts", import.meta.url)
);

describe("Phase H2: Rebase Kanban & Surgical Split Workbench UX", () => {
  const kanbanSrc = readFileSync(kanbanModalFile, "utf8");
  const historySrc = readFileSync(historyViewFile, "utf8");
  const interactiveModalSrc = readFileSync(interactiveRebaseModalFile, "utf8");
  const gitServiceSrc = readFileSync(gitServiceFile, "utf8");
  const typesSrc = readFileSync(typesFile, "utf8");

  test("RebaseKanbanModal renders topology preview, conflict risks, and surgical split controls", () => {
    expect(kanbanSrc).toContain("previewRebaseTopology");
    expect(kanbanSrc).toContain("splitCommitSurgically");
    expect(kanbanSrc).toContain("surgical-split-trigger-btn");
    expect(kanbanSrc).toContain("apply-kanban-rebase-btn");
    expect(kanbanSrc).toContain("rebase.kanban.title");
    expect(kanbanSrc).toContain("rebase.kanban.split.title");
    expect(kanbanSrc).toContain("rebase.kanban.split.execute");
  });

  test("HistoryView and InteractiveRebaseModal integrate Kanban launcher", () => {
    expect(historySrc).toContain("open-rebase-kanban-btn");
    expect(historySrc).toContain("RebaseKanbanModal");
    expect(historySrc).toContain("showRebaseKanbanModal");
    expect(interactiveModalSrc).toContain("open-kanban-mode-btn");
    expect(interactiveModalSrc).toContain("open-kanban");
  });

  test("gitService exposes Phase H2 RPC endpoints", () => {
    expect(gitServiceSrc).toContain("previewRebaseTopology:");
    expect(gitServiceSrc).toContain("splitCommitSurgically:");
  });

  test("types.ts defines Phase H2 interfaces", () => {
    expect(typesSrc).toContain("export interface RebaseKanbanItem");
    expect(typesSrc).toContain("export interface RebasePlanPreview");
    expect(typesSrc).toContain("export interface CommitPartition");
    expect(typesSrc).toContain("export interface CommitSplitResult");
  });

  test("i18n parity for rebase kanban and surgical split keys", () => {
    const requiredKeys = [
      "rebase.kanban.title",
      "rebase.kanban.subtitle",
      "rebase.kanban.load",
      "rebase.kanban.loading",
      "rebase.kanban.total_commits",
      "rebase.kanban.conflict_risk",
      "rebase.kanban.linearize_ready",
      "rebase.kanban.linearize_warning",
      "rebase.kanban.risk.low",
      "rebase.kanban.risk.medium",
      "rebase.kanban.risk.high",
      "rebase.kanban.action.pick",
      "rebase.kanban.action.squash",
      "rebase.kanban.action.fixup",
      "rebase.kanban.action.reword",
      "rebase.kanban.action.drop",
      "rebase.kanban.action.split",
      "rebase.kanban.btn.split_workbench",
      "rebase.kanban.split.title",
      "rebase.kanban.split.desc",
      "rebase.kanban.split.partition",
      "rebase.kanban.split.subject_placeholder",
      "rebase.kanban.split.file_count",
      "rebase.kanban.split.add_partition",
      "rebase.kanban.split.execute",
      "rebase.kanban.split.executing",
      "rebase.kanban.split.success",
      "rebase.kanban.split.failed",
      "rebase.kanban.btn.apply",
      "rebase.kanban.empty",
      "rebase.kanban.open",
    ];

    for (const key of requiredKeys) {
      expect((zhCN as Record<string, string>)[key]).toBeDefined();
      expect((zhCN as Record<string, string>)[key]!.length).toBeGreaterThan(0);
      expect((enUS as Record<string, string>)[key]).toBeDefined();
      expect((enUS as Record<string, string>)[key]!.length).toBeGreaterThan(0);
    }
  });
});
