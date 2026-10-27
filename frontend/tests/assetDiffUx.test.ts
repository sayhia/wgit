import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const diffViewerFile = fileURLToPath(
  new URL("../src/components/workspace/DiffViewer.vue", import.meta.url)
);
const workspaceViewFile = fileURLToPath(
  new URL("../src/components/WorkspaceView.vue", import.meta.url)
);
const changesListFile = fileURLToPath(
  new URL("../src/components/workspace/ChangesList.vue", import.meta.url)
);
const commitInspectorFile = fileURLToPath(
  new URL("../src/components/history/CommitInspector.vue", import.meta.url)
);
const historyViewFile = fileURLToPath(
  new URL("../src/components/HistoryView.vue", import.meta.url)
);
const assetDiffModalFile = fileURLToPath(
  new URL("../src/components/assets/AssetDiffModal.vue", import.meta.url)
);

describe("S1: Asset Diff UX & Conflict Resolution Suite", () => {
  describe("AssetDiffModal 4 Comparison Modes", () => {
    const src = readFileSync(assetDiffModalFile, "utf8");

    test("implements 4 comparison modes: side, overlay, swipe, diff", () => {
      expect(src).toContain('type Mode = "side" | "overlay" | "swipe" | "diff"');
      expect(src).toContain("mode === 'side'");
      expect(src).toContain("mode === 'overlay'");
      expect(src).toContain("mode === 'swipe'");
      expect(src).toContain("computeDiff");
      expect(src).toContain("checkDiffEligibility");
    });

    test("supports opacity range slider for overlay and swipe percentage for swipe mode", () => {
      expect(src).toContain("opacity");
      expect(src).toContain("swipePct");
      expect(src).toContain("clipPath");
    });
  });

  describe("DiffViewer Binary Asset Compare Integration", () => {
    const src = readFileSync(diffViewerFile, "utf8");

    test("declares canCompareAsset prop and open-asset-diff emit", () => {
      expect(src).toContain("canCompareAsset?: boolean");
      expect(src).toContain('(e: "open-asset-diff"): void');
    });

    test("renders visual compare button inside binary detection banner", () => {
      expect(src).toContain("isBinaryDiff");
      expect(src).toContain("emit('open-asset-diff')");
      expect(src).toContain('t("ws.asset.compare")');
    });
  });

  describe("CommitInspector Historical Asset Diff Integration", () => {
    const src = readFileSync(commitInspectorFile, "utf8");

    test("declares open-asset-diff emit and checks asset file paths", () => {
      expect(src).toContain('(e: "open-asset-diff", filePath: string): void');
      expect(src).toContain("isAssetFilePath");
      expect(src).toContain("firstAssetFile");
    });

    test("renders asset compare action in files list and binary notice", () => {
      expect(src).toContain("emit('open-asset-diff', f.path)");
      expect(src).toContain("emit('open-asset-diff', firstAssetFile)");
    });
  });

  describe("HistoryView AssetDiffModal Integration", () => {
    const src = readFileSync(historyViewFile, "utf8");

    test("imports and mounts AssetDiffModal", () => {
      expect(src).toContain('import AssetDiffModal from "./assets/AssetDiffModal.vue"');
      expect(src).toContain("<AssetDiffModal");
      expect(src).toContain("v-model:open=\"showAssetDiff\"");
      expect(src).toContain("@open-asset-diff=\"handleOpenAssetDiff\"");
    });
  });

  describe("ChangesList & WorkspaceView Bulk Meta Conflict Resolution", () => {
    const changesSrc = readFileSync(changesListFile, "utf8");
    const wsSrc = readFileSync(workspaceViewFile, "utf8");

    test("ChangesList computes metaConflictedFiles and emits auto-resolve-meta", () => {
      expect(changesSrc).toContain("metaConflictedFiles");
      expect(changesSrc).toContain("hasMetaConflicts");
      expect(changesSrc).toContain('(e: "auto-resolve-meta", strategy: "theirs" | "backup_theirs"): void');
      expect(changesSrc).toContain("emit('auto-resolve-meta', 'theirs')");
    });

    test("WorkspaceView wires autoResolveMetaConflicts handler", () => {
      expect(wsSrc).toContain("handleAutoResolveMetaConflicts");
      expect(wsSrc).toContain("gitService.autoResolveMetaConflicts");
      expect(wsSrc).toContain('@auto-resolve-meta="handleAutoResolveMetaConflicts"');
    });
  });

  describe("i18n completeness for S1", () => {
    const requiredKeys = [
      "ws.toast.meta.conflict.none",
      "ws.toast.meta.conflict.resolved",
      "ws.conflict.auto.meta",
      "ws.conflict.auto.meta.desc",
    ];

    for (const key of requiredKeys) {
      test(`zh-CN has ${key}`, () => {
        expect((zhCN as Record<string, string>)[key]).toBeDefined();
        expect((zhCN as Record<string, string>)[key]!.length).toBeGreaterThan(0);
      });

      test(`en-US has ${key}`, () => {
        expect((enUS as Record<string, string>)[key]).toBeDefined();
        expect((enUS as Record<string, string>)[key]!.length).toBeGreaterThan(0);
      });
    }
  });
});
