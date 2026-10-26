import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const maintenanceTabFile = fileURLToPath(
  new URL("../src/components/tools/MaintenanceTab.vue", import.meta.url)
);

describe("Repo Diagnostics & Maintenance UX (Round 12)", () => {
  const src = readFileSync(maintenanceTabFile, "utf8");

  describe("Object Store Tasks & Prune", () => {
    test("includes prune task in OBJECT_STORE_TASKS", () => {
      expect(src).toContain('key: "prune"');
      expect(src).toContain("maint.store.task.prune");
      expect(src).toContain("maint.store.task.prune.hint");
    });

    test("contains handlePruneDangling calling optimizeObjectStore with prune", () => {
      expect(src).toContain("handlePruneDangling");
      expect(src).toContain('gitService.optimizeObjectStore(props.currentRepoPath, ["prune"])');
      expect(src).toContain("maint.fsck.prune.confirm");
      expect(src).toContain("maint.fsck.prune.action");
    });
  });

  describe("FSCK Dangling Objects Remediation & Error Details", () => {
    test("renders dangling objects remediation action bar", () => {
      expect(src).toContain("fsckResult.danglingBlob > 0 || fsckResult.danglingTree > 0 || fsckResult.danglingCommit > 0");
      expect(src).toContain("maint.fsck.dangling.hint");
      expect(src).toContain("@click=\"handlePruneDangling\"");
    });

    test("renders copy button for FSCK raw errors", () => {
      expect(src).toContain("fsckResult.rawErrors && fsckResult.rawErrors.length > 0");
      expect(src).toContain("copyToClipboard(fsckResult.rawErrors.join('\\n'))");
    });
  });

  describe("i18n completeness for Maintenance & Diagnostics", () => {
    const requiredKeys = [
      "maint.store.task.prune",
      "maint.store.task.prune.hint",
      "maint.fsck.dangling.hint",
      "maint.fsck.prune.action",
      "maint.fsck.pruning",
      "maint.fsck.prune.confirm",
      "maint.toast.prune.dangling.failed",
      "maint.toast.errors.copied",
    ];

    test("zh-CN contains all required Round 12 keys", () => {
      const catalog = zhCN as Record<string, string>;
      for (const key of requiredKeys) {
        expect(catalog[key]).toBeDefined();
        expect(catalog[key].length).toBeGreaterThan(0);
      }
    });

    test("en-US contains all required Round 12 keys", () => {
      const catalog = enUS as Record<string, string>;
      for (const key of requiredKeys) {
        expect(catalog[key]).toBeDefined();
        expect(catalog[key].length).toBeGreaterThan(0);
      }
    });
  });
});
