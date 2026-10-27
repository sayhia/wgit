import { describe, expect, test } from "bun:test";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";
import { readFileSync, existsSync } from "fs";
import { resolve } from "path";

describe("Phase F2: Visual Git Bisect & Heatmap UX", () => {
  test("HistoryView integrates Bisect in toolbar and commit context menu", () => {
    const historyPath = resolve(__dirname, "../src/components/HistoryView.vue");
    expect(existsSync(historyPath)).toBe(true);

    const src = readFileSync(historyPath, "utf-8");
    expect(src).toContain("history.bisect.btn.title");
    expect(src).toContain("history.menu.bisect.bad");
    expect(src).toContain("history.menu.bisect.good");
    expect(src).toContain("openBisectConsole");
    expect(src).toContain("wgit:tools-tab");
  });

  test("BisectPanel exposes setRefs and integrates with ToolsView", () => {
    const panelPath = resolve(__dirname, "../src/components/BisectPanel.vue");
    const toolsPath = resolve(__dirname, "../src/components/ToolsView.vue");

    expect(existsSync(panelPath)).toBe(true);
    expect(existsSync(toolsPath)).toBe(true);

    const panelSrc = readFileSync(panelPath, "utf-8");
    const toolsSrc = readFileSync(toolsPath, "utf-8");

    expect(panelSrc).toContain("setRefs");
    expect(toolsSrc).toContain("bisectPanelRef.value?.setRefs");
  });

  test("SourcePanorama contains blame heatmap color classes", () => {
    const panoramaPath = resolve(__dirname, "../src/components/workspace/SourcePanorama.vue");
    const src = readFileSync(panoramaPath, "utf-8");

    expect(src).toContain("getBlameHeatmapClass");
    expect(src).toContain("showBlame");
    expect(src).toContain("onBlameCellClick");
  });

  test("i18n catalogs contain complete bisect keys in both languages", () => {
    const requiredKeys = [
      "history.menu.bisect.bad",
      "history.menu.bisect.good",
      "history.toast.bisect.bad.set",
      "history.toast.bisect.good.set",
      "history.bisect.btn.title",
    ];

    for (const key of requiredKeys) {
      expect((zhCN as Record<string, string>)[key]).toBeDefined();
      expect((enUS as Record<string, string>)[key]).toBeDefined();
    }
  });
});
