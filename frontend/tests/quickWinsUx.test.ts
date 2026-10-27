import { describe, test, expect } from "bun:test";
import { readFileSync } from "fs";
import { resolve } from "path";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

describe("Phase Q: Quick Wins & Fine-Grained Adjustments Suite", () => {
  describe("SourceExplorer & ChangesList Deep Link Integration", () => {
    const sourceExplorerContent = readFileSync(
      resolve(__dirname, "../src/components/workspace/SourceExplorer.vue"),
      "utf-8"
    );
    const changesListContent = readFileSync(
      resolve(__dirname, "../src/components/workspace/ChangesList.vue"),
      "utf-8"
    );

    test("SourceExplorer.vue imports generateDeepLink and provides copy deep link action", () => {
      expect(sourceExplorerContent).toContain("generateDeepLink");
      expect(sourceExplorerContent).toContain("copyDeepLinkPath");
      expect(sourceExplorerContent).toContain("explorer.menu.copy.deeplink");
    });

    test("ChangesList.vue provides copy deep link button on change rows", () => {
      expect(changesListContent).toContain("generateDeepLink");
      expect(changesListContent).toContain("copyFileDeepLink");
      expect(changesListContent).toContain("changes.copy.deeplink");
    });

    test("ChangesList.vue provides starter configuration template card in clean state", () => {
      expect(changesListContent).toContain("handleApplyTemplate");
      expect(changesListContent).toContain("generateGitignoreTemplate");
      expect(changesListContent).toContain("changes.starter.templates.title");
      expect(changesListContent).toContain("changes.starter.templates.desc");
    });
  });

  describe("Maintenance Frontend Cache Management", () => {
    const maintenanceContent = readFileSync(
      resolve(__dirname, "../src/components/tools/MaintenanceTab.vue"),
      "utf-8"
    );

    test("MaintenanceTab.vue displays in-memory cache metrics and clear action", () => {
      expect(maintenanceContent).toContain("commitDetailCache");
      expect(maintenanceContent).toContain("handleClearFrontendCache");
      expect(maintenanceContent).toContain("clearCommitDetailCache");
      expect(maintenanceContent).toContain("maint.cache.title");
      expect(maintenanceContent).toContain("maint.cache.clear.btn");
    });
  });

  describe("Shortcut Cheat Sheet Modal", () => {
    const cheatSheetContent = readFileSync(
      resolve(__dirname, "../src/components/ShortcutCheatSheetModal.vue"),
      "utf-8"
    );
    const appContent = readFileSync(resolve(__dirname, "../src/App.vue"), "utf-8");

    test("ShortcutCheatSheetModal renders shortcuts registry and fixed bindings", () => {
      expect(cheatSheetContent).toContain("SHORTCUT_ACTIONS");
      expect(cheatSheetContent).toContain("effectiveBinding");
      expect(cheatSheetContent).toContain("formatBinding");
      expect(cheatSheetContent).toContain("shortcuts.cheatsheet.title");
    });

    test("App.vue mounts ShortcutCheatSheetModal and handles keyboard toggle", () => {
      expect(appContent).toContain("ShortcutCheatSheetModal");
      expect(appContent).toContain("showCheatSheet");
    });
  });

  describe("i18n Completeness for Phase Q", () => {
    const requiredKeys = [
      "explorer.menu.copy.deeplink",
      "explorer.toast.deeplink.copied",
      "changes.copy.deeplink",
      "changes.toast.deeplink.copied",
      "changes.starter.templates.title",
      "changes.starter.templates.desc",
      "changes.toast.template.applied",
      "maint.cache.title",
      "maint.cache.desc",
      "maint.cache.clear.btn",
      "maint.cache.cleared",
      "maint.cache.detail.label",
      "maint.cache.detail.hint",
      "maint.cache.svg.label",
      "maint.cache.svg.hint",
      "shortcuts.cheatsheet.title",
      "shortcuts.cheatsheet.section.global",
      "shortcuts.cheatsheet.section.builtin",
      "shortcuts.fixed.repo.tabs.label",
      "shortcuts.fixed.repo.tabs.desc",
      "shortcuts.fixed.escape.label",
      "shortcuts.fixed.escape.desc",
      "shortcuts.fixed.help.label",
      "shortcuts.fixed.help.desc",
    ];

    test("zh-CN contains all required Phase Q keys", () => {
      const zhDict = zhCN as Record<string, string>;
      for (const key of requiredKeys) {
        expect(zhDict[key]).toBeDefined();
        expect(zhDict[key].length).toBeGreaterThan(0);
      }
    });

    test("en-US contains all required Phase Q keys", () => {
      const enDict = enUS as Record<string, string>;
      for (const key of requiredKeys) {
        expect(enDict[key]).toBeDefined();
        expect(enDict[key].length).toBeGreaterThan(0);
      }
    });
  });
});
