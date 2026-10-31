import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

// Source-assertion suite for "swallowed catch" triage batch 1:
// WorkspaceView.vue (11 bare catches), App.vue (7), CommitComposer.vue (5).
// Fix points escalate user-decision-affecting swallowed service failures to an
// existing toast/notify channel or console.error; keep points document why a
// benign catch stays silent.

const workspaceViewFile = fileURLToPath(
  new URL("../src/components/WorkspaceView.vue", import.meta.url)
);
const appFile = fileURLToPath(new URL("../src/App.vue", import.meta.url));
const commitComposerFile = fileURLToPath(
  new URL("../src/components/workspace/CommitComposer.vue", import.meta.url)
);
const zhFile = fileURLToPath(new URL("../src/i18n/messages/zh-CN.ts", import.meta.url));
const enFile = fileURLToPath(new URL("../src/i18n/messages/en-US.ts", import.meta.url));

function count(src: string, needle: string): number {
  return src.split(needle).length - 1;
}

describe("Swallowed catch triage batch 1: fix points (user-visible wiring)", () => {
  const appSrc = readFileSync(appFile, "utf8");
  const composerSrc = readFileSync(commitComposerFile, "utf8");

  test("App.vue refreshAllStatuses surfaces per-repo failures via the existing refresh-failed toast", () => {
    // refreshStatus already toasted on failure; the batch refresh must not stay
    // silent (a swallowed getRepoStatus leaves the tab badge looking "clean").
    expect(count(appSrc, 'notify(t("app.toast.refresh.failed"')).toBe(2);
    expect(appSrc).toContain("Failed to refresh status for");
    // The batch catch still records the null status as the marked state.
    expect(appSrc).toContain("console.error(`Failed to refresh status for ${p}:`, err);");
    expect(appSrc).toContain("return [p, null] as const;");
  });

  test("App.vue restoreTabs logs skipped saved repos instead of dropping tabs silently", () => {
    expect(appSrc).toContain("Skipping saved repo ${p}: status check failed");
    // Graceful-degradation contract kept: the repo is still excluded from restore.
    expect(appSrc).toContain("return { p, ok: false, st: null };");
  });

  test("CommitComposer checkGpg failure is logged, not silently ignored", () => {
    expect(composerSrc).toContain('console.error("Failed to check commit GPG health:", err);');
  });

  test("CommitComposer lint watcher clears badges and logs on RPC failure", () => {
    expect(composerSrc).toContain('console.error("Failed to lint commit message:", err);');
    // State marking kept: both lint badges reset to null on failure.
    expect(composerSrc).toContain("lintResult.value = null;");
    expect(composerSrc).toContain("commitLintReport.value = null;");
  });

  test("no new i18n keys were introduced (reused app.toast.refresh.failed exists in both locales)", () => {
    expect((zhCN as Record<string, string>)["app.toast.refresh.failed"]).toBeDefined();
    expect((enUS as Record<string, string>)["app.toast.refresh.failed"]).toBeDefined();
    const zhSrc = readFileSync(zhFile, "utf8");
    const enSrc = readFileSync(enFile, "utf8");
    expect(count(zhSrc, '"app.toast.refresh.failed"')).toBe(1);
    expect(count(enSrc, '"app.toast.refresh.failed"')).toBe(1);
  });
});

describe("Swallowed catch triage batch 1: keep points (documented benign catches)", () => {
  const wsSrc = readFileSync(workspaceViewFile, "utf8");
  const appSrc = readFileSync(appFile, "utf8");
  const composerSrc = readFileSync(commitComposerFile, "utf8");

  test("WorkspaceView benign catches carry rationale comments", () => {
    expect(wsSrc).toContain("Benign: unreadable/corrupt pane prefs just fall back to the default widths.");
    expect(wsSrc).toContain("Benign: unreadable layout presets just start the list empty.");
    // The three clipboard execCommand fallbacks each got the comment.
    expect(count(wsSrc, "Benign: clipboard API rejected")).toBe(3);
  });

  test("WorkspaceView pre-existing commented benign catches were not disturbed", () => {
    expect(count(wsSrc, "/* ignore quota / privacy-mode errors */")).toBe(2);
  });

  test("App.vue benign catches keep their rationale comments", () => {
    expect(count(appSrc, "/* storage disabled — preference just won't persist */")).toBe(2);
    expect(appSrc).toContain("/* running outside Wails runtime */");
    expect(appSrc).toContain("/* Benign: storage disabled/corrupt — tab restore just keeps the current view */");
    expect(appSrc).toContain("/* Benign: deep-link probe is best-effort");
  });

  test("CommitComposer benign catches keep their rationale comments", () => {
    expect(composerSrc).toContain("Benign: unreadable/corrupt draft history just starts empty.");
    expect(composerSrc).toContain("skip an unreadable single file");
    expect(composerSrc).toContain("// ignore storage issues");
  });
});
