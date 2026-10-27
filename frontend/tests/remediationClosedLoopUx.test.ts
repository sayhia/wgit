import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const lockModalFile = fileURLToPath(
  new URL("../src/components/lfs/LockNotificationModal.vue", import.meta.url)
);
const lfsTabFile = fileURLToPath(
  new URL("../src/components/tools/LfsTab.vue", import.meta.url)
);
const toolsSectionFile = fileURLToPath(
  new URL("../src/components/settings/ToolsSection.vue", import.meta.url)
);
const workspaceViewFile = fileURLToPath(
  new URL("../src/components/WorkspaceView.vue", import.meta.url)
);

describe("S2: Remediation Closed-Loop & LFS Communication Suite", () => {
  describe("LockNotificationModal Contracts", () => {
    const src = readFileSync(lockModalFile, "utf8");

    test("implements 3 communication formats: markdown, email, webhook", () => {
      expect(src).toContain("activeTab");
      expect(src).toContain("template.markdown");
      expect(src).toContain("template.emailSubject");
      expect(src).toContain("template.emailBody");
      expect(src).toContain("template.webhookJson");
    });

    test("calls gitService.generateLfsLockNotification with note", () => {
      expect(src).toContain("gitService.generateLfsLockNotification");
      expect(src).toContain("props.repoPath");
      expect(src).toContain("props.filePath");
    });

    test("provides one-click copy logic with copied state feedback", () => {
      expect(src).toContain("copyCurrentContent");
      expect(src).toContain("navigator.clipboard.writeText");
      expect(src).toContain("copied.value = true");
    });
  });

  describe("LfsTab Zombie Notification Integration", () => {
    const src = readFileSync(lfsTabFile, "utf8");

    test("imports and mounts LockNotificationModal", () => {
      expect(src).toContain('import LockNotificationModal from "../lfs/LockNotificationModal.vue"');
      expect(src).toContain("<LockNotificationModal");
      expect(src).toContain("showLockNotifyModal");
    });

    test("renders contact owner button in zombie lock banner and lock list", () => {
      expect(src).toContain("openLockNotification(zombieLocks[0].path, zombieLocks[0].owner)");
      expect(src).toContain("openLockNotification(lock.path, lock.owner)");
      expect(src).toContain('t("lfs.activity.zombie.notify")');
    });
  });

  describe("ToolsSection UnityYAMLMerge Driver Setup", () => {
    const src = readFileSync(toolsSectionFile, "utf8");

    test("implements configureUnityMerge calling gitService", () => {
      expect(src).toContain("configureUnityMerge");
      expect(src).toContain("gitService.configureUnityYamlMerge");
      expect(src).toContain("unityMergeCustomPath");
      expect(src).toContain("unityApplyGitattributes");
    });

    test("renders Unity YAML Merge driver configuration card", () => {
      expect(src).toContain('t("settings.tools.unity.yaml.title")');
      expect(src).toContain('t("settings.tools.unity.yaml.configure")');
    });
  });

  describe("WorkspaceView Staged Non-LFS Large Files Alert", () => {
    const src = readFileSync(workspaceViewFile, "utf8");

    test("computes stagedNonLfsLargeFiles and handles conversion", () => {
      expect(src).toContain("stagedNonLfsLargeFiles");
      expect(src).toContain("handleConvertStagedNonLfs");
      expect(src).toContain("gitService.trackStagedLargeFilesAsLfs");
    });

    test("renders warning banner and one-click convert button", () => {
      expect(src).toContain("v-if=\"stagedNonLfsLargeFiles.length > 0\"");
      expect(src).toContain('t("ws.staged.large.warning"');
      expect(src).toContain('t("ws.staged.large.convert")');
    });
  });

  describe("i18n completeness for S2", () => {
    const requiredKeys = [
      "lfs.activity.zombie.notify",
      "lfs.notify.modal.title",
      "lfs.notify.modal.note.label",
      "lfs.notify.modal.note.placeholder",
      "lfs.notify.modal.tab.email",
      "lfs.notify.modal.hint",
      "lfs.notify.modal.copy",
      "settings.tools.unity.yaml.title",
      "settings.tools.unity.yaml.desc",
      "settings.tools.unity.yaml.placeholder",
      "settings.tools.unity.yaml.attributes",
      "settings.tools.unity.yaml.configure",
      "settings.tools.unity.yaml.configuring",
      "settings.tools.unity.yaml.success",
      "settings.tools.unity.yaml.failed",
      "ws.staged.large.warning",
      "ws.staged.large.convert",
      "ws.staged.large.success",
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
