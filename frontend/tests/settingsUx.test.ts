import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const signingSectionFile = fileURLToPath(
  new URL("../src/components/settings/SigningSection.vue", import.meta.url)
);
const advancedConfigFile = fileURLToPath(
  new URL("../src/components/settings/AdvancedConfigSection.vue", import.meta.url)
);

describe("Global Settings & Security Governance UX (Round 17)", () => {
  const signingSrc = readFileSync(signingSectionFile, "utf8");
  const configSrc = readFileSync(advancedConfigFile, "utf8");

  describe("Commit Signing Guide & Audit", () => {
    test("renders selectable SSH key chips for fast configuration", () => {
      expect(signingSrc).toContain("signingFormat === 'ssh'");
      expect(signingSrc).toContain("@click=\"signingKeyPath = key\"");
      expect(signingSrc).toContain("settings.signing.ssh.detected.title");
    });

    test("contains signature compliance dashboard and recent commit audit", () => {
      expect(signingSrc).toContain("complianceRate");
      expect(signingSrc).toContain("compliantCount");
      expect(signingSrc).toContain("recentCommits");
      expect(signingSrc).toContain("settings.signing.audit.title");
      expect(signingSrc).toContain("settings.signing.badge.valid");
      expect(signingSrc).toContain("settings.signing.badge.unsigned");
    });
  });

  describe("safe.directory Management & Auto-fix", () => {
    test("contains safe directory state and auto-fix functions", () => {
      expect(configSrc).toContain("safeDirectories");
      expect(configSrc).toContain("isCurrentRepoSafe");
      expect(configSrc).toContain("handleAddCurrentToSafe");
      expect(configSrc).toContain("handleRemoveSafe");
      expect(configSrc).toContain("gitService.addSafeDirectory(props.currentRepoPath)");
      expect(configSrc).toContain("gitService.removeSafeDirectory(dir)");
    });

    test("renders safe.directory card, trust badge, and 1-click fix button", () => {
      expect(configSrc).toContain("settings.safe.title");
      expect(configSrc).toContain("settings.safe.trusted");
      expect(configSrc).toContain("settings.safe.untrusted");
      expect(configSrc).toContain("settings.safe.fix.action");
    });
  });

  describe("i18n completeness for Settings & Governance", () => {
    const requiredKeys = [
      "settings.signing.ssh.detected.title",
      "settings.signing.audit.title",
      "settings.signing.audit.desc",
      "settings.signing.badge.valid",
      "settings.signing.badge.unsigned",
      "settings.safe.title",
      "settings.safe.desc",
      "settings.safe.trusted",
      "settings.safe.untrusted",
      "settings.safe.fix.action",
      "settings.safe.adding",
      "settings.safe.added",
      "settings.safe.add.failed",
      "settings.safe.configured.title",
      "settings.safe.none",
      "settings.safe.remove.title",
      "settings.safe.remove.confirm",
      "settings.safe.removed",
      "settings.safe.remove.failed",
    ];

    test("zh-CN contains all required Round 17 keys", () => {
      for (const k of requiredKeys) {
        expect((zhCN as Record<string, string>)[k]).toBeDefined();
        expect((zhCN as Record<string, string>)[k].length).toBeGreaterThan(0);
      }
    });

    test("en-US contains all required Round 17 keys", () => {
      for (const k of requiredKeys) {
        expect((enUS as Record<string, string>)[k]).toBeDefined();
        expect((enUS as Record<string, string>)[k].length).toBeGreaterThan(0);
      }
    });
  });
});
