import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const terminalTabFile = fileURLToPath(
  new URL("../src/components/tools/TerminalTab.vue", import.meta.url)
);

describe("Embedded Terminal & CLI Experience UX (Round 13)", () => {
  const src = readFileSync(terminalTabFile, "utf8");

  describe("Keyboard shortcuts & selection handling", () => {
    test("wires attachCustomKeyEventHandler for Cmd+C copy and Cmd+K clear", () => {
      expect(src).toContain("attachCustomKeyEventHandler");
      expect(src).toContain("term?.hasSelection()");
      expect(src).toContain("writeClipboardText(sel)");
      expect(src).toContain("term?.clear()");
    });
  });

  describe("Terminal Toolbar & Directory Sync", () => {
    test("provides restartTerminal functionality", () => {
      expect(src).toContain("restartTerminal");
      expect(src).toContain("stopTerminal()");
      expect(src).toContain("startTerminal()");
      expect(src).toContain("terminal.restart");
    });

    test("provides cdToCurrentRepo path synchronization", () => {
      expect(src).toContain("cdToCurrentRepo");
      expect(src).toContain("gitService.writeTerminal(`cd ${safePath}\\n`)");
      expect(src).toContain("terminal.cd.repo");
    });

    test("renders quick command presets chips bar", () => {
      expect(src).toContain("QUICK_COMMANDS");
      expect(src).toContain("runQuickCommand");
      expect(src).toContain("git status");
      expect(src).toContain("git diff");
      expect(src).toContain("git branch -av");
      expect(src).toContain("terminal.quick.commands");
    });
  });

  describe("i18n completeness for Terminal enhancements", () => {
    const requiredKeys = [
      "terminal.start",
      "terminal.stop",
      "terminal.restart",
      "terminal.restart.title",
      "terminal.clear",
      "terminal.cd.repo",
      "terminal.cd.repo.title",
      "terminal.quick.commands",
      "terminal.toast.cd.synced",
      "terminal.toast.restarted",
    ];

    test("zh-CN contains all required Round 13 keys", () => {
      const catalog = zhCN as Record<string, string>;
      for (const key of requiredKeys) {
        expect(catalog[key]).toBeDefined();
        expect(catalog[key].length).toBeGreaterThan(0);
      }
    });

    test("en-US contains all required Round 13 keys", () => {
      const catalog = enUS as Record<string, string>;
      for (const key of requiredKeys) {
        expect(catalog[key]).toBeDefined();
        expect(catalog[key].length).toBeGreaterThan(0);
      }
    });
  });
});
