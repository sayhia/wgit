import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { gitService } from "../src/services/gitService";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

describe("Phase F4: Local AI Sentinel & Semantic Release Changelog UX", () => {
  test("gitService exposes generateReleaseChangelog RPC wrapper", () => {
    expect(typeof gitService.generateReleaseChangelog).toBe("function");
  });

  test("CommitComposer integrates audit-sentinel security check action", () => {
    const filePath = resolve(__dirname, "../src/components/workspace/CommitComposer.vue");
    const content = readFileSync(filePath, "utf-8");

    expect(content).toContain("audit-sentinel");
    expect(content).toContain("workspace.composer.audit.sentinel");
  });

  test("ReleaseChangelogModal exists with clean template and zero hardcoded Chinese", () => {
    const filePath = resolve(__dirname, "../src/components/tools/ReleaseChangelogModal.vue");
    const content = readFileSync(filePath, "utf-8");

    expect(content).toContain("generateReleaseChangelog");
    expect(content).toContain("release.changelog.modal.title");
    expect(content).toContain("release.changelog.stat.breaking");

    // Zero hardcoded Chinese in template
    const templateMatch = content.match(/<template>([\s\S]*?)<\/template>/);
    expect(templateMatch).not.toBeNull();
    const templateContent = templateMatch![1];
    const chineseChars = templateContent.match(/[\u4e00-\u9fa5]/g);
    expect(chineseChars).toBeNull();
  });

  test("ReleaseWizard integrates ReleaseChangelogModal", () => {
    const filePath = resolve(__dirname, "../src/components/ReleaseWizard.vue");
    const content = readFileSync(filePath, "utf-8");

    expect(content).toContain("ReleaseChangelogModal");
    expect(content).toContain("showChangelogModal");
    expect(content).toContain("release.changelog.modal.btn");
  });

  test("i18n catalogs contain complete Phase F4 keys in both languages", () => {
    const requiredKeys = [
      "workspace.composer.audit.sentinel",
      "workspace.composer.audit.sentinel.hint",
      "ws.composer.audit.no.staged",
      "ws.composer.audit.scanning",
      "ws.composer.audit.clean",
      "ws.composer.audit.issues",
      "ws.composer.audit.failed",
      "release.changelog.modal.title",
      "release.changelog.modal.btn",
      "release.changelog.from.label",
      "release.changelog.to.label",
      "release.changelog.range.desc",
      "release.changelog.btn.generate",
      "release.changelog.stat.commits",
      "release.changelog.stat.breaking",
      "release.changelog.stat.contributors",
      "release.changelog.breaking.alert",
      "release.changelog.preview.label",
      "release.changelog.toast.copied",
      "release.changelog.toast.failed",
    ];

    for (const key of requiredKeys) {
      expect((zhCN as Record<string, string>)[key]).toBeDefined();
      expect((enUS as Record<string, string>)[key]).toBeDefined();
      expect((zhCN as Record<string, string>)[key].length).toBeGreaterThan(0);
      expect((enUS as Record<string, string>)[key].length).toBeGreaterThan(0);
    }
  });
});
