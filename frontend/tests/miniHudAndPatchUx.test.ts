import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { SHORTCUT_ACTIONS } from "../src/utils/shortcuts";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

describe("Phase Polish: Mini Commit HUD & Diff Patch Playground UX", () => {
  test("SHORTCUT_ACTIONS registers mini-commit-hud with Alt+Meta+C", () => {
    const action = SHORTCUT_ACTIONS.find((a) => a.id === "mini-commit-hud");
    expect(action).toBeDefined();
    expect(action?.defaultBinding).toBe("Alt+Meta+C");
    expect(action?.customizable).toBe(true);
    expect(action?.owner).toBe("app");
  });

  test("App.vue mounts MiniCommitHudModal and handles shortcut action", () => {
    const filePath = resolve(__dirname, "../src/App.vue");
    const content = readFileSync(filePath, "utf-8");

    expect(content).toContain("MiniCommitHudModal");
    expect(content).toContain("showMiniCommitHud");
    expect(content).toContain('case "mini-commit-hud"');
  });

  test("MiniCommitHudModal has clean template and zero hardcoded Chinese", () => {
    const filePath = resolve(__dirname, "../src/components/workspace/MiniCommitHudModal.vue");
    const content = readFileSync(filePath, "utf-8");

    expect(content).toContain("ws.hud.title");
    expect(content).toContain("ws.hud.staged.count");
    expect(content).toContain("handleCommit");
    expect(content).toContain("handleStageAll");

    const templateMatch = content.match(/<template>([\s\S]*?)<\/template>/);
    expect(templateMatch).not.toBeNull();
    const templateContent = templateMatch![1];
    const chineseChars = templateContent.match(/[\u4e00-\u9fa5]/g);
    expect(chineseChars).toBeNull();
  });

  test("DiffViewer integrates Patch Playground drawer and copy action", () => {
    const filePath = resolve(__dirname, "../src/components/workspace/DiffViewer.vue");
    const content = readFileSync(filePath, "utf-8");

    expect(content).toContain("showPatchPlayground");
    expect(content).toContain("copyFullPatch");
    expect(content).toContain("workspace.diff.playground.title");
    expect(content).toContain("workspace.diff.playground.btn");
  });

  test("i18n catalogs contain complete Phase Polish keys in both languages", () => {
    const requiredKeys = [
      "shortcuts.action.mini-commit-hud.label",
      "shortcuts.action.mini-commit-hud.description",
      "ws.hud.title",
      "ws.hud.staged.count",
      "ws.hud.stage.all",
      "ws.hud.subject.placeholder",
      "ws.hud.add.body",
      "ws.hud.hide.body",
      "ws.hud.body.placeholder",
      "ws.hud.shortcut.hint",
      "ws.hud.committing",
      "ws.hud.commit",
      "ws.hud.toast.staged.all",
      "ws.hud.toast.no.staged",
      "ws.hud.toast.success",
      "ws.hud.toast.failed",
      "workspace.diff.playground.btn",
      "workspace.diff.playground.hint",
      "workspace.diff.playground.title",
      "workspace.diff.playground.hunks",
      "workspace.diff.playground.lines",
      "workspace.diff.playground.copy",
      "workspace.diff.playground.toast.copied",
    ];

    for (const key of requiredKeys) {
      expect((zhCN as Record<string, string>)[key]).toBeDefined();
      expect((enUS as Record<string, string>)[key]).toBeDefined();
      expect((zhCN as Record<string, string>)[key].length).toBeGreaterThan(0);
      expect((enUS as Record<string, string>)[key].length).toBeGreaterThan(0);
    }
  });
});
