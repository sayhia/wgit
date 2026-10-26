import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const lfsTabFile = fileURLToPath(
  new URL("../src/components/tools/LfsTab.vue", import.meta.url)
);
const gitServiceFile = fileURLToPath(
  new URL("../src/services/gitService.ts", import.meta.url)
);

describe("Git LFS & Large Asset Governance UX (Round 10)", () => {
  describe("gitService LFS extensions", () => {
    const src = readFileSync(gitServiceFile, "utf8");

    test("exposes verifyLfsPointers and lfsPrune", () => {
      expect(src).toContain("verifyLfsPointers: (repoPath: string)");
      expect(src).toContain("lfsPrune: (repoPath: string, dryRun = true, verbose = false)");
    });
  });

  describe("LfsTab Prune & FSCK Actions", () => {
    const src = readFileSync(lfsTabFile, "utf8");

    test("contains handleLfsPrune with dryRun and confirmation", () => {
      expect(src).toContain("handleLfsPrune");
      expect(src).toContain("gitService.lfsPrune(props.currentRepoPath, dryRun, false)");
      expect(src).toContain("lfs.prune.confirm");
      expect(src).toContain("lfs.prune.dryrun");
      expect(src).toContain("lfs.prune.exec");
    });

    test("contains handleVerifyLfs and status badge", () => {
      expect(src).toContain("handleVerifyLfs");
      expect(src).toContain("gitService.verifyLfsPointers(props.currentRepoPath)");
      expect(src).toContain("lfs.fsck.button");
      expect(src).toContain("lfs.fsck.status.clean");
      expect(src).toContain("lfs.fsck.status.issues");
    });
  });

  describe("i18n completeness for LFS Prune & FSCK", () => {
    const requiredKeys = [
      "lfs.prune.confirm",
      "lfs.prune.dryrun",
      "lfs.prune.dryrun.title",
      "lfs.prune.exec",
      "lfs.prune.exec.title",
      "lfs.prune.running",
      "lfs.fsck.title",
      "lfs.fsck.button",
      "lfs.fsck.running",
      "lfs.fsck.status.clean",
      "lfs.fsck.status.issues",
      "lfs.toast.prune.preview",
      "lfs.toast.prune.success",
      "lfs.toast.prune.failed",
      "lfs.toast.fsck.clean",
      "lfs.toast.fsck.issues",
      "lfs.toast.fsck.failed",
    ];

    test("zh-CN contains all required LFS prune & fsck keys", () => {
      const catalog = zhCN as Record<string, string>;
      for (const key of requiredKeys) {
        expect(catalog[key]).toBeDefined();
      }
    });

    test("en-US contains all required LFS prune & fsck keys", () => {
      const catalog = enUS as Record<string, string>;
      for (const key of requiredKeys) {
        expect(catalog[key]).toBeDefined();
      }
    });
  });
});
