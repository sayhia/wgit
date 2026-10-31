import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

// Source-assertion suite for "swallowed catch" triage batch 5:
// tools/WorktreesTab.vue (2), tools/SparseTab.vue (1), tools/CleanTab.vue (1),
// tools/MaintenanceTab.vue (3), tools/StudioTab.vue (1). TerminalTab.vue was
// fully triaged in batch 3 and is out of scope here. Fix points escalate
// swallowed service failures to an existing toast channel (reportError) or
// console.error while keeping the graceful state marking; keep points document
// why clipboard fallbacks / best-effort prechecks / secondary cleanup stay
// silent-but-annotated.

const worktreesFile = fileURLToPath(
  new URL("../src/components/tools/WorktreesTab.vue", import.meta.url)
);
const sparseFile = fileURLToPath(
  new URL("../src/components/tools/SparseTab.vue", import.meta.url)
);
const cleanFile = fileURLToPath(
  new URL("../src/components/tools/CleanTab.vue", import.meta.url)
);
const maintenanceFile = fileURLToPath(
  new URL("../src/components/tools/MaintenanceTab.vue", import.meta.url)
);
const studioFile = fileURLToPath(
  new URL("../src/components/tools/StudioTab.vue", import.meta.url)
);
const zhFile = fileURLToPath(new URL("../src/i18n/messages/zh-CN.ts", import.meta.url));
const enFile = fileURLToPath(new URL("../src/i18n/messages/en-US.ts", import.meta.url));

function count(src: string, needle: string): number {
  return src.split(needle).length - 1;
}

describe("Swallowed catch triage batch 5: fix points (user-visible wiring)", () => {
  const worktreesSrc = readFileSync(worktreesFile, "utf8");
  const sparseSrc = readFileSync(sparseFile, "utf8");

  test("WorktreesTab branch-validation watcher logs failures instead of silently clearing the conflict warning", () => {
    expect(worktreesSrc).toContain('console.error("Failed to validate worktree branch:", err);');
    // State marking kept: the warning still clears on failure (graceful
    // degrade), and the create itself re-validates server-side.
    expect(worktreesSrc).toContain("branchConflictWarning.value = \"\";");
    // The eventual failure channel for the create is untouched.
    expect(worktreesSrc).toContain('t("worktrees.toast.add.failed"');
  });

  test("SparseTab partial-clone probe surfaces failures via reportError like its sparse siblings", () => {
    // A swallowed failure rendered the "standard clone" badge that read as
    // "this repo is not a partial clone" — the fake state of this batch.
    expect(sparseSrc).toContain('reportError("Failed to load partial clone status", err);');
    // State marking kept: still degrades to the standard-clone badge.
    expect(sparseSrc).toContain("partialCloneStatus.value = null;");
  });

  test("no new i18n keys were introduced (fixes wire to reportError/console.error, not new keys)", () => {
    const zhSrc = readFileSync(zhFile, "utf8");
    const enSrc = readFileSync(enFile, "utf8");
    for (const key of [
      "worktrees.toast.validate.failed",
      "sparse.partial.status.failed",
    ]) {
      expect(zhSrc).not.toContain(`"${key}"`);
      expect(enSrc).not.toContain(`"${key}"`);
    }
  });
});

describe("Swallowed catch triage batch 5: keep points (documented benign catches)", () => {
  const worktreesSrc = readFileSync(worktreesFile, "utf8");
  const cleanSrc = readFileSync(cleanFile, "utf8");
  const maintenanceSrc = readFileSync(maintenanceFile, "utf8");
  const studioSrc = readFileSync(studioFile, "utf8");

  test("MaintenanceTab clipboard keeps its execCommand fallback with rationale comments", () => {
    expect(
      count(maintenanceSrc, "Benign: clipboard API rejected — fall back to execCommand below.")
    ).toBe(2);
    // Both copy sites still report success through their existing toasts.
    expect(maintenanceSrc).toContain('t("maint.toast.errors.copied")');
    expect(maintenanceSrc).toContain('t("maint.toast.hotfix.copied")');
  });

  test("MaintenanceTab archive dir picker failure already routes to a notify toast", () => {
    expect(maintenanceSrc).toContain('notify(t("maint.toast.dir.picker.failed"))');
  });

  test("WorktreesTab dirty precheck stays best-effort with a documented rationale", () => {
    expect(worktreesSrc).toContain("Benign: the dirty precheck is best-effort");
  });

  test("CleanTab post-remediation rescan keeps previous findings with its rationale comment", () => {
    expect(cleanSrc).toContain("Keep the previous findings");
    expect(cleanSrc).toContain("A manual rescan is always available.");
  });

  test("StudioTab durable log cleanup stays best-effort with its rationale comment", () => {
    expect(studioSrc).toContain("/* durable history may be absent */");
  });
});
