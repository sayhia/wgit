import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const sparseTabFile = fileURLToPath(
  new URL("../src/components/tools/SparseTab.vue", import.meta.url)
);
const commitGraphListFile = fileURLToPath(
  new URL("../src/components/history/CommitGraphList.vue", import.meta.url)
);
const diffViewerFile = fileURLToPath(
  new URL("../src/components/workspace/DiffViewer.vue", import.meta.url)
);

describe("Phase M3: Extreme Monorepo Scalability & Minimap Navigation", () => {
  describe("Partial Clone & On-Demand Hydration (SparseTab)", () => {
    const src = readFileSync(sparseTabFile, "utf8");

    test("queries getPartialCloneStatus and exposes hydrateBlobs", () => {
      expect(src).toContain("getPartialCloneStatus");
      expect(src).toContain("hydrateBlobs");
      expect(src).toContain("partialCloneStatus");
      expect(src).toContain("handleHydrate");
    });

    test("renders Partial Clone promisor indicator and hydrate controls", () => {
      expect(src).toContain("sparse.partial.title");
      expect(src).toContain("sparse.partial.active");
      expect(src).toContain("sparse.partial.hydrate");
      expect(src).toContain("hydratePath");
    });
  });

  describe("Commit Graph Minimap Navigation", () => {
    const src = readFileSync(commitGraphListFile, "utf8");

    test("calculates minimap visibility and slider thumb geometry", () => {
      expect(src).toContain("showMinimap");
      expect(src).toContain("minimapSlider");
      expect(src).toContain("handleMinimapClick");
      expect(src).toContain("handleMinimapMouseDown");
    });

    test("renders minimap navigation strip with thumb", () => {
      expect(src).toContain("minimapEl");
      expect(src).toContain("history.graph.minimap.hint");
      expect(src).toContain("minimapSlider.top");
      expect(src).toContain("minimapSlider.height");
    });
  });

  describe("Virtualized Diff Line Rendering for Large Files", () => {
    const src = readFileSync(diffViewerFile, "utf8");

    test("implements visibleUnifiedWindow with topPadding and bottomPadding spacers", () => {
      expect(src).toContain("isVirtualizedDiff");
      expect(src).toContain("visibleUnifiedWindow");
      expect(src).toContain("topPadding");
      expect(src).toContain("bottomPadding");
      expect(src).toContain("DIFF_LINE_HEIGHT");
    });

    test("detects scrollable ancestor and handles scroll updates", () => {
      expect(src).toContain("getScrollParent");
      expect(src).toContain("updateScrollPosition");
      expect(src).toContain("scrollTarget.addEventListener");
      expect(src).toContain("scrollTarget.removeEventListener");
    });

    test("renders spacer blocks when padding is positive", () => {
      expect(src).toContain("visibleUnifiedWindow.topPadding > 0");
      expect(src).toContain("visibleUnifiedWindow.bottomPadding > 0");
    });
  });

  describe("Phase M3 i18n Completeness", () => {
    const requiredKeys = [
      "sparse.partial.title",
      "sparse.partial.active",
      "sparse.partial.standard",
      "sparse.partial.filter",
      "sparse.partial.hydrate",
      "sparse.partial.hydrate.btn",
      "sparse.partial.hydrating",
      "sparse.partial.hydrated",
      "history.graph.minimap.hint",
    ];

    test("all Phase M3 keys exist in zh-CN and en-US catalogs", () => {
      for (const key of requiredKeys) {
        expect((zhCN as Record<string, string>)[key]).toBeDefined();
        expect((enUS as Record<string, string>)[key]).toBeDefined();
      }
    });
  });
});
