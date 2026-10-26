import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";
import { translate } from "../src/i18n";

const commitComposerFile = fileURLToPath(
  new URL("../src/components/workspace/CommitComposer.vue", import.meta.url)
);
const workspaceViewFile = fileURLToPath(
  new URL("../src/components/WorkspaceView.vue", import.meta.url)
);
const maintenanceTabFile = fileURLToPath(
  new URL("../src/components/tools/MaintenanceTab.vue", import.meta.url)
);

describe("Git Hooks Client-side Interception & Management UX (Round 24)", () => {
  const composerSrc = readFileSync(commitComposerFile, "utf8");
  const workspaceSrc = readFileSync(workspaceViewFile, "utf8");
  const maintenanceSrc = readFileSync(maintenanceTabFile, "utf8");

  describe("Commit Composer --no-verify controls", () => {
    test("declares noVerify prop and emits update:noVerify and commit with boolean", () => {
      expect(composerSrc).toContain("noVerify?: boolean;");
      expect(composerSrc).toContain('(e: "update:noVerify", value: boolean): void;');
      expect(composerSrc).toContain('(e: "commit", noVerify: boolean): void;');
    });

    test("renders --no-verify checkbox with tooltip in bottom action bar", () => {
      expect(composerSrc).toContain("data-testid=\"no-verify-checkbox\"");
      expect(composerSrc).toContain("t('workspace.composer.noVerify.hint')");
      expect(composerSrc).toContain("--no-verify");
      expect(composerSrc).toContain("v-model=\"noVerifyModel\"");
    });

    test("passes noVerify state on shortcut and button click commit triggers", () => {
      expect(composerSrc).toContain("emit(\"commit\", noVerifyModel.value);");
    });
  });

  describe("WorkspaceView commit integration & hook blockage guidance", () => {
    test("declares isNoVerify ref and binds v-model:noVerify on CommitComposer", () => {
      expect(workspaceSrc).toContain("const isNoVerify = ref(false);");
      expect(workspaceSrc).toContain('v-model:noVerify="isNoVerify"');
    });

    test("handleCommit receives noVerify and calls gitService.commitAdvanced", () => {
      expect(workspaceSrc).toContain("async function handleCommit(noVerifyParam?: boolean)");
      expect(workspaceSrc).toContain(
        "const noVerify = typeof noVerifyParam === \"boolean\" ? noVerifyParam : isNoVerify.value;"
      );
      expect(workspaceSrc).toContain(
        "await gitService.commitAdvanced(props.currentRepoPath, fullMsg, isAmend.value, noVerify);"
      );
    });

    test("provides error notification with advice when commit is blocked by git hooks", () => {
      expect(workspaceSrc).toContain("const isHookBlocked =");
      expect(workspaceSrc).toContain("/pre-commit/i.test(rawError)");
      expect(workspaceSrc).toContain("/commit-msg/i.test(rawError)");
      expect(workspaceSrc).toContain("isHookBlocked && !noVerify");
      expect(workspaceSrc).toContain("t(\"ws.toast.commit.failed.hook\"");
    });
  });

  describe("Third-party hooks framework detection banner", () => {
    test("calls gitService.detectHooksFramework inside loadGitHooks", () => {
      expect(maintenanceSrc).toContain("gitService.detectHooksFramework(props.currentRepoPath)");
      expect(maintenanceSrc).toContain("hooksFramework.value = framework;");
    });

    test("renders framework banner when type is not native", () => {
      expect(maintenanceSrc).toContain("data-testid=\"hooks-framework-banner\"");
      expect(maintenanceSrc).toContain("hooksFramework.type !== 'native'");
      expect(maintenanceSrc).toContain("hooks.framework.banner.title");
      expect(maintenanceSrc).toContain("hooks.framework.banner.desc");
      expect(maintenanceSrc).toContain("hooks.framework.config");
      expect(maintenanceSrc).toContain("hooks.framework.path");
    });

    test("formats framework labels properly", () => {
      expect(maintenanceSrc).toContain("function frameworkDisplayName");
      expect(maintenanceSrc).toContain('"Husky"');
      expect(maintenanceSrc).toContain('"Lefthook"');
      expect(maintenanceSrc).toContain('"pre-commit"');
    });
  });

  describe("Hook cards quick test run & sample restore/initialization", () => {
    test("renders quick test run button calling handleRunHookCard", () => {
      expect(maintenanceSrc).toContain("data-testid=\"hook-card-test-btn\"");
      expect(maintenanceSrc).toContain("@click.stop=\"handleRunHookCard(hook)\"");
      expect(maintenanceSrc).toContain("gitService.runGitHook(props.currentRepoPath, hook.name)");
      expect(maintenanceSrc).toContain("hooks.toast.run.success");
    });

    test("renders restore / init from sample button calling handleInstallSampleHook", () => {
      expect(maintenanceSrc).toContain("data-testid=\"hook-card-sample-btn\"");
      expect(maintenanceSrc).toContain("@click.stop=\"handleInstallSampleHook(hook)\"");
      expect(maintenanceSrc).toContain("gitService.installSampleHook(props.currentRepoPath, hook.name)");
      expect(maintenanceSrc).toContain("hooks.toast.sample.installed");
    });
  });

  describe("Enhanced Hook Editor Modal & real-time terminal console", () => {
    test("provides preset templates dropdown with standard git hook scripts", () => {
      expect(maintenanceSrc).toContain("data-testid=\"hook-template-select\"");
      expect(maintenanceSrc).toContain("HOOK_TEMPLATES");
      expect(maintenanceSrc).toContain("conventional-commits");
      expect(maintenanceSrc).toContain("secret-scan");
      expect(maintenanceSrc).toContain("large-files");
      expect(maintenanceSrc).toContain("protect-main");
      expect(maintenanceSrc).toContain("handleApplyHookTemplate");
    });

    test("provides in-modal test run trigger and live output terminal", () => {
      expect(maintenanceSrc).toContain("data-testid=\"hook-modal-test-btn\"");
      expect(maintenanceSrc).toContain("@click=\"handleRunHookInModal\"");
      expect(maintenanceSrc).toContain("data-testid=\"hook-test-terminal\"");
      expect(maintenanceSrc).toContain("hooks.terminal.title");
      expect(maintenanceSrc).toContain("modalTestResult.exitCode");
      expect(maintenanceSrc).toContain("modalTestResult.durationMs");
      expect(maintenanceSrc).toContain("modalTestResult.stdout");
      expect(maintenanceSrc).toContain("modalTestResult.stderr");
    });
  });

  describe("i18n completeness for Git Hooks & no-verify controls", () => {
    const requiredHooksKeys = [
      "hooks.framework.banner.title",
      "hooks.framework.banner.desc",
      "hooks.framework.config",
      "hooks.framework.path",
      "hooks.framework.custom",
      "hooks.test.run",
      "hooks.test.running",
      "hooks.sample.init",
      "hooks.sample.restore",
      "hooks.template.select",
      "hooks.template.placeholder",
      "hooks.template.conventional",
      "hooks.template.secrets",
      "hooks.template.largefiles",
      "hooks.template.protectmain",
      "hooks.terminal.title",
      "hooks.terminal.running",
      "hooks.terminal.no_output",
      "hooks.toast.run.success",
      "hooks.toast.run.failed",
      "hooks.toast.run.error",
      "hooks.toast.sample.installed",
      "hooks.toast.sample.failed",
      "hooks.toast.template.applied",
      "workspace.composer.noverify",
      "workspace.composer.noverify.hint",
      "workspace.composer.noverify.bypass",
      "ws.toast.commit.failed.hook",
    ];

    test("zh-CN contains all required Round 24 hooks & no-verify keys", () => {
      const catalog = zhCN as Record<string, string>;
      for (const key of requiredHooksKeys) {
        expect(catalog[key]).toBeDefined();
        expect(catalog[key].length).toBeGreaterThan(0);
      }
    });

    test("en-US contains all required Round 24 hooks & no-verify keys", () => {
      const catalog = enUS as Record<string, string>;
      for (const key of requiredHooksKeys) {
        expect(catalog[key]).toBeDefined();
        expect(catalog[key].length).toBeGreaterThan(0);
      }
    });

    test("resolves noVerify camelCase and bypass hints via translation helper", () => {
      const hintZh = translate("workspace.composer.noVerify.hint", undefined, "zh-CN");
      expect(hintZh).toContain("--no-verify");
      expect(hintZh).toContain("pre-commit");

      const hintEn = translate("workspace.composer.noVerify.hint", undefined, "en-US");
      expect(hintEn).toContain("--no-verify");
      expect(hintEn).toContain("pre-commit");

      const hookBlockedZh = translate("ws.toast.commit.failed.hook", { error: "pre-commit failed" }, "zh-CN");
      expect(hookBlockedZh).toContain("--no-verify");
      expect(hookBlockedZh).toContain("pre-commit failed");

      const hookBlockedEn = translate("ws.toast.commit.failed.hook", { error: "pre-commit failed" }, "en-US");
      expect(hookBlockedEn).toContain("--no-verify");
      expect(hookBlockedEn).toContain("pre-commit failed");
    });
  });
});
