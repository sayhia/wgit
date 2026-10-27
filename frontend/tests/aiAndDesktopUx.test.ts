import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const workspaceViewFile = fileURLToPath(
  new URL("../src/components/WorkspaceView.vue", import.meta.url)
);
const rebaseModalFile = fileURLToPath(
  new URL("../src/components/RebaseModal.vue", import.meta.url)
);
const mainGoFile = fileURLToPath(
  new URL("../../main.go", import.meta.url)
);

describe("Phase M4: AI Assistance & Desktop Ecosystem", () => {
  describe("Conventional Commit Smart Generator", () => {
    const src = readFileSync(workspaceViewFile, "utf8");

    test("implements generateSmartCommitMessage with conventional commit scopes", () => {
      expect(src).toContain("generateSmartCommitMessage");
      expect(src).toContain("prefix");
      expect(src).toContain("scope");
      expect(src).toContain("commitSubject.value =");
      expect(src).toContain("commitBody.value =");
    });
  });

  describe("Predictive Merge Conflict Detector", () => {
    const src = readFileSync(rebaseModalFile, "utf8");

    test("queries predictMergeConflicts and tracks conflictPrediction", () => {
      expect(src).toContain("predictMergeConflicts");
      expect(src).toContain("conflictPrediction");
      expect(src).toContain("loadConflictPrediction");
    });

    test("renders predictive conflict risk badge with score", () => {
      expect(src).toContain("conflictPrediction.hasConflicts");
      expect(src).toContain("conflictPrediction.riskScore");
      expect(src).toContain("rebase.predict.risk");
    });
  });

  describe("Desktop System Tray & Notifications", () => {
    const src = readFileSync(mainGoFile, "utf8");

    test("configures SystemTray and tray context menu in main.go", () => {
      expect(src).toContain("app.SystemTray.New()");
      expect(src).toContain("tray.SetIcon(appIcon)");
      expect(src).toContain("tray.SetTooltip");
      expect(src).toContain("tray.SetMenu(trayMenu)");
      expect(src).toContain("tray.OnClick");
    });
  });

  describe("Phase M4 i18n Completeness", () => {
    const requiredKeys = [
      "rebase.predict.risk",
      "rebase.predict.title",
    ];

    test("all Phase M4 keys exist in zh-CN and en-US catalogs", () => {
      for (const key of requiredKeys) {
        expect((zhCN as Record<string, string>)[key]).toBeDefined();
        expect((enUS as Record<string, string>)[key]).toBeDefined();
      }
    });
  });
});
