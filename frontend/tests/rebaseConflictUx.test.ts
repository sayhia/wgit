import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const rebaseModalFile = fileURLToPath(
  new URL("../src/components/RebaseModal.vue", import.meta.url)
);
const conflictBannerFile = fileURLToPath(
  new URL("../src/components/workspace/ConflictBanner.vue", import.meta.url)
);
const workspaceViewFile = fileURLToPath(
  new URL("../src/components/WorkspaceView.vue", import.meta.url)
);

describe("Rebase Lifecycle & Conflict Ribbon UX", () => {
  describe("RebaseModal contract", () => {
    const src = readFileSync(rebaseModalFile, "utf8");

    test("shows step progression using rebase.progress.step", () => {
      expect(src).toContain("t(\"rebase.progress.step\"");
    });

    test("contains stylized progress bar with width percentage", () => {
      expect(src).toContain("progressPercent");
      expect(src).toContain(":style=\"`width: ${progressPercent}%;`\"");
    });

    test("displays paused commit hash and subject when available", () => {
      expect(src).toContain("pausedCommitInfo");
      expect(src).toContain("pausedCommitInfo.hash");
      expect(src).toContain("pausedCommitInfo.subject");
    });

    test("lists conflicted files with view conflict in workspace button", () => {
      expect(src).toContain("conflictedFilesList");
      expect(src).toContain("handleViewConflictInWorkspace");
      expect(src).toContain("t(\"rebase.conflict.view.file\")");
      expect(src).toContain("wgit:open-file");
    });

    test("prompts with confirmation before aborting", () => {
      expect(src).toContain("confirmAction");
      expect(src).toContain("t(\"rebase.abort.confirm\")");
      expect(src).toContain("gitService.abortConflictOperation");
    });

    test("prompts with confirmation before skipping", () => {
      expect(src).toContain("t(\"rebase.skip.confirm\")");
      expect(src).toContain("gitService.skipConflictOperation");
    });
  });

  describe("ConflictBanner contract", () => {
    const src = readFileSync(conflictBannerFile, "utf8");

    test("shows distinct badges for rebasing, merging, and cherry-picking", () => {
      expect(src).toContain("currentBadge");
      expect(src).toContain("t(\"workspace.conflict.badge.rebasing\")");
      expect(src).toContain("t(\"workspace.conflict.badge.merging\")");
      expect(src).toContain("t(\"workspace.conflict.badge.cherry_picking\")");
    });

    test("shows step progression for rebasing", () => {
      expect(src).toContain("stepProgressionText");
      expect(src).toContain("t(\"rebase.progress.step\"");
    });

    test("provides localized help text and confirmation on abort", () => {
      expect(src).toContain("helpText");
      expect(src).toContain("confirmAction");
      expect(src).toContain("handleAbort");
      expect(src).toContain("t(\"rebase.abort.confirm\")");
    });
  });

  describe("WorkspaceView integration", () => {
    const src = readFileSync(workspaceViewFile, "utf8");

    test("supports opening conflicted diff view via wgit:open-file", () => {
      expect(src).toContain("detail.view === \"diff\"");
      expect(src).toContain("selectChangeFile");
    });

    test("handles abort without double-confirming from ConflictBanner", () => {
      expect(src).toContain("handleAbortConflict(true)");
    });
  });

  describe("i18n key parity for rebase & conflict ribbon", () => {
    const requiredKeys = [
      "rebase.progress.step",
      "rebase.conflict.view.file",
      "rebase.abort.confirm",
      "rebase.skip.confirm",
      "workspace.conflict.badge.rebasing",
      "workspace.conflict.badge.merging",
      "workspace.conflict.badge.cherry_picking",
      "workspace.conflict.abort.confirm",
      "workspace.conflict.help.unresolved",
      "workspace.conflict.help.resolved",
    ];

    for (const key of requiredKeys) {
      test(`key "${key}" exists in zh-CN and en-US with non-empty translation`, () => {
        const zh = (zhCN as Record<string, string>)[key];
        const en = (enUS as Record<string, string>)[key];
        expect(zh).toBeDefined();
        expect(en).toBeDefined();
        expect(zh?.trim().length).toBeGreaterThan(0);
        expect(en?.trim().length).toBeGreaterThan(0);
      });
    }
  });
});
