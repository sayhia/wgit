import { describe, expect, test } from "bun:test";
import { readFileSync, existsSync } from "node:fs";
import { fileURLToPath } from "node:url";

const stateMatrixFile = fileURLToPath(
  new URL("../../docs/state-matrix.md", import.meta.url)
);
const commitGraphListFile = fileURLToPath(
  new URL("../src/components/history/CommitGraphList.vue", import.meta.url)
);
const commitInspectorFile = fileURLToPath(
  new URL("../src/components/history/CommitInspector.vue", import.meta.url)
);
const useToastFile = fileURLToPath(
  new URL("../src/composables/useToast.ts", import.meta.url)
);

describe("P1: State Matrix & Anti-Silent Failure Guard", () => {
  test("docs/state-matrix.md exists and covers 6 main views and 14 tool subtabs", () => {
    expect(existsSync(stateMatrixFile)).toBe(true);
    const matrixSrc = readFileSync(stateMatrixFile, "utf8");

    // 6 Main views
    const mainViews = [
      "WorkspaceView",
      "HistoryView",
      "BranchesView",
      "StashView",
      "PrView",
      "ReviewView",
    ];
    for (const view of mainViews) {
      expect(matrixSrc).toContain(view);
    }

    // 14 Tool subtabs
    const toolTabs = [
      "TerminalTab",
      "ReflogTab",
      "CleanTab",
      "RemotesTab",
      "WorktreesTab",
      "LfsTab",
      "SubmodulesTab",
      "SparseTab",
      "DiskTab",
      "MaintenanceTab",
      "StudioTab",
      "EngineTab",
      "BisectPanel",
      "ReleaseWizard",
    ];
    for (const tab of toolTabs) {
      expect(matrixSrc).toContain(tab);
    }
  });

  test("CommitGraphList renders error block strictly BEFORE empty block", () => {
    const src = readFileSync(commitGraphListFile, "utf8");
    const errorIdx = src.indexOf('v-else-if="error"');
    const emptyIdx = src.indexOf("history.graph.empty");
    expect(errorIdx).toBeGreaterThan(0);
    expect(emptyIdx).toBeGreaterThan(0);
    expect(errorIdx).toBeLessThan(emptyIdx);
  });

  test("CommitInspector defines error prop and renders error block before empty block", () => {
    const src = readFileSync(commitInspectorFile, "utf8");
    expect(src).toContain("error?: string | null");
    const errorIdx = src.indexOf('v-else-if="error"');
    const emptyIdx = src.indexOf("history.inspector.empty");
    expect(errorIdx).toBeGreaterThan(0);
    expect(emptyIdx).toBeGreaterThan(0);
    expect(errorIdx).toBeLessThan(emptyIdx);
  });

  test("useToast exports reportError to enforce visible technical error reporting", () => {
    const src = readFileSync(useToastFile, "utf8");
    expect(src).toContain("export function reportError");
    expect(src).toContain('tone: "danger"');
  });
});
