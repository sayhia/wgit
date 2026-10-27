import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const terminalTabFile = fileURLToPath(
  new URL("../src/components/tools/TerminalTab.vue", import.meta.url)
);
const diffViewerFile = fileURLToPath(
  new URL("../src/components/workspace/DiffViewer.vue", import.meta.url)
);
const gitServiceFile = fileURLToPath(
  new URL("../src/services/gitService.ts", import.meta.url)
);

describe("P4: Terminal Auto-Follow & External Editor Interop UX", () => {
  const terminalSrc = readFileSync(terminalTabFile, "utf8");
  const diffViewerSrc = readFileSync(diffViewerFile, "utf8");
  const gitServiceSrc = readFileSync(gitServiceFile, "utf8");

  test("TerminalTab supports auto-following active repository directory", () => {
    expect(terminalSrc).toContain("autoFollowRepo");
    expect(terminalSrc).toContain("toggleAutoFollow");
    expect(terminalSrc).toContain("wgit.terminal.auto-follow");
    expect(terminalSrc).toContain("terminal.auto.follow");
    expect(terminalSrc).toContain("cdToCurrentRepo");

    // Watcher triggers cdToCurrentRepo when path changes and autoFollow is active
    expect(terminalSrc).toContain("autoFollowRepo.value && isRunning.value && newPath");

    expect((zhCN as Record<string, string>)["terminal.auto.follow"]).toBeDefined();
    expect((enUS as Record<string, string>)["terminal.auto.follow"]).toBeDefined();
    expect((zhCN as Record<string, string>)["terminal.auto.follow.title"]).toBeDefined();
    expect((enUS as Record<string, string>)["terminal.auto.follow.title"]).toBeDefined();
  });

  test("DiffViewer provides Open in Editor action", () => {
    expect(diffViewerSrc).toContain("open-in-editor");
    expect(diffViewerSrc).toContain("diff.open.editor");
    expect(diffViewerSrc).toContain("ExternalLink");

    expect((zhCN as Record<string, string>)["diff.open.editor"]).toBeDefined();
    expect((enUS as Record<string, string>)["diff.open.editor"]).toBeDefined();
    expect((zhCN as Record<string, string>)["diff.open.editor.title"]).toBeDefined();
    expect((enUS as Record<string, string>)["diff.open.editor.title"]).toBeDefined();
  });

  test("gitService exposes openFileInEditor and getAppVersion RPCs", () => {
    expect(gitServiceSrc).toContain("openFileInEditor:");
    expect(gitServiceSrc).toContain("getAppVersion:");
  });
});
