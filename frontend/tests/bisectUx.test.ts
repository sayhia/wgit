import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const bisectPanelFile = fileURLToPath(
  new URL("../src/components/BisectPanel.vue", import.meta.url)
);

describe("Git Bisect Regression Hunting UX (Round 16)", () => {
  const panelSrc = readFileSync(bisectPanelFile, "utf8");

  describe("Candidate Commit Details", () => {
    test("displays candidate commit subject, author, and relative date", () => {
      expect(panelSrc).toContain("status?.currentSubject");
      expect(panelSrc).toContain("status.currentAuthor");
      expect(panelSrc).toContain("status.currentDate");
    });
  });

  describe("Automated Bisect Script Runner", () => {
    test("defines autoCommand, isRunningAuto, and autoRunMessage state", () => {
      expect(panelSrc).toContain("const autoCommand = ref");
      expect(panelSrc).toContain("const isRunningAuto = ref");
      expect(panelSrc).toContain("const autoRunMessage = ref");
    });

    test("contains handleAutoRun calling gitService.bisectRun", () => {
      expect(panelSrc).toContain("handleAutoRun");
      expect(panelSrc).toContain("gitService.bisectRun(props.repoPath, autoCommand.value.trim())");
      expect(panelSrc).toContain('t("bisect.error.auto.failed"');
    });

    test("renders auto runner input, presets, and trigger button", () => {
      expect(panelSrc).toContain("bisect.auto.title");
      expect(panelSrc).toContain("bisect.auto.placeholder");
      expect(panelSrc).toContain("bisect.auto.presets");
      expect(panelSrc).toContain("bisect.auto.run");
      expect(panelSrc).toContain("bun test");
      expect(panelSrc).toContain("go test ./...");
    });
  });

  describe("i18n completeness for Bisect enhancements", () => {
    const requiredKeys = [
      "bisect.auto.title",
      "bisect.auto.desc",
      "bisect.auto.placeholder",
      "bisect.auto.run",
      "bisect.auto.running",
      "bisect.auto.presets",
      "bisect.error.auto.failed",
    ];

    test("zh-CN contains all required Round 16 keys", () => {
      for (const k of requiredKeys) {
        expect((zhCN as Record<string, string>)[k]).toBeDefined();
        expect((zhCN as Record<string, string>)[k].length).toBeGreaterThan(0);
      }
    });

    test("en-US contains all required Round 16 keys", () => {
      for (const k of requiredKeys) {
        expect((enUS as Record<string, string>)[k]).toBeDefined();
        expect((enUS as Record<string, string>)[k].length).toBeGreaterThan(0);
      }
    });
  });
});
