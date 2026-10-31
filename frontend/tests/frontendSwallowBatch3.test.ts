import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

// Source-assertion suite for "swallowed catch" triage batch 3:
// HistoryView.vue (6 bare catches), workspace/DiffViewer.vue (5),
// tools/TerminalTab.vue (4 + 2 fire-and-forget writes). Fix points escalate
// swallowed service failures to an existing toast channel (reportError) or
// console.error while keeping the graceful state marking; keep points document
// why localStorage/clipboard/xterm-write catches stay best-effort.

const historyViewFile = fileURLToPath(
  new URL("../src/components/HistoryView.vue", import.meta.url)
);
const diffViewerFile = fileURLToPath(
  new URL("../src/components/workspace/DiffViewer.vue", import.meta.url)
);
const terminalTabFile = fileURLToPath(
  new URL("../src/components/tools/TerminalTab.vue", import.meta.url)
);
const zhFile = fileURLToPath(new URL("../src/i18n/messages/zh-CN.ts", import.meta.url));
const enFile = fileURLToPath(new URL("../src/i18n/messages/en-US.ts", import.meta.url));

function count(src: string, needle: string): number {
  return src.split(needle).length - 1;
}

describe("Swallowed catch triage batch 3: fix points (user-visible wiring)", () => {
  const historySrc = readFileSync(historyViewFile, "utf8");
  const diffSrc = readFileSync(diffViewerFile, "utf8");
  const terminalSrc = readFileSync(terminalTabFile, "utf8");

  test("HistoryView loadTopologyStats logs failures instead of silently hiding the topology badge", () => {
    expect(historySrc).toContain('console.error("Failed to load commit topology stats:", err);');
    // State marking kept: the badge still hides on failure (graceful hide).
    expect(historySrc).toContain("topologyStats.value = null;");
  });

  test("HistoryView reset-dialog status probe logs failures instead of silently dropping the dirty warning", () => {
    expect(historySrc).toContain('console.error("Failed to check working-tree cleanliness:", err);');
    // State marking kept: still degrades to "clean"; the reset itself fails
    // safely through the existing history.toast.reset.failed channel.
    expect(historySrc).toContain("isWorkingTreeClean.value = true;");
    expect(historySrc).toContain('t("history.toast.reset.failed"');
  });

  test("DiffViewer loadBlame surfaces failures via reportError like its loadHeatmap sibling", () => {
    // A swallowed failure rendered an empty gutter that read as "this file has
    // no history" — the priority fake-empty state of this batch.
    expect(diffSrc).toContain('reportError("Failed to load blame", err);');
    // State marking kept: the gutter still clears on failure.
    expect(diffSrc).toContain("blameLines.value = [];");
  });

  test("DiffViewer loadSemanticDiff logs failures instead of a bare /* ignore */", () => {
    expect(diffSrc).toContain('console.error("Failed to load semantic diff outline:", err);');
    // Both bare ignores (semantic outline + patch copy) were replaced with
    // rationale comments; no silent /* ignore */ remains in the file.
    expect(diffSrc).not.toContain("/* ignore */");
  });

  test("TerminalTab refreshStatus logs probe failures while degrading to not-running", () => {
    expect(terminalSrc).toContain('console.error("Failed to read terminal status:", err);');
    // State marking kept: disconnect still reads as stopped.
    expect(terminalSrc).toContain("isRunning.value = false;");
  });

  test("no new i18n keys were introduced (fixes wire to reportError/console.error, not new keys)", () => {
    const zhSrc = readFileSync(zhFile, "utf8");
    const enSrc = readFileSync(enFile, "utf8");
    for (const key of [
      "history.toast.topology.failed",
      "history.toast.blame.failed",
      "workspace.diff.semantic.failed",
      "terminal.status.failed",
    ]) {
      expect(zhSrc).not.toContain(`"${key}"`);
      expect(enSrc).not.toContain(`"${key}"`);
    }
    // The pre-existing toast channels these files rely on are untouched.
    expect(count(historySrc, 't("history.toast.export.failed"')).toBe(1);
    expect(diffSrc).toContain('t("workspace.diff.playground.toast.copied")');
  });
});

describe("Swallowed catch triage batch 3: keep points (documented benign catches)", () => {
  const historySrc = readFileSync(historyViewFile, "utf8");
  const diffSrc = readFileSync(diffViewerFile, "utf8");
  const terminalSrc = readFileSync(terminalTabFile, "utf8");

  test("HistoryView localStorage and clipboard keeps carry rationale comments", () => {
    expect(historySrc).toContain(
      "Benign: unreadable/corrupt graph prefs just default the graph on."
    );
    // The pre-existing write-side annotation was not disturbed.
    expect(historySrc).toContain("/* storage unavailable — keep the in-memory preference */");
    // Export report's clipboard fallback keeps its WebView comment and the
    // execCommand failure still routes to the outer failed toast.
    expect(historySrc).toContain("Fallback for WebViews that do not expose the clipboard API.");
    expect(historySrc).toContain(
      "Benign: clipboard API rejected — fall back to execCommand below."
    );
  });

  test("DiffViewer clipboard keeps carry rationale comments; fallback chain returns false", () => {
    expect(diffSrc).toContain("Benign: clipboard is best-effort here");
    expect(diffSrc).toContain("// fallback to execCommand below");
    expect(diffSrc).toContain("return false;");
  });

  test("TerminalTab xterm write/disconnect keeps stay fire-and-forget with rationale comments", () => {
    expect(terminalSrc).toContain("Benign: theme probing is best-effort");
    expect(terminalSrc).toContain("/* the host may not be laid out yet */");
    expect(terminalSrc).toContain("/* ignore transient layout errors */");
    // Keystroke writes keep the disconnect contract: a failed write means the
    // session died, which the status probe reports.
    expect(terminalSrc).toContain("Keystrokes are fire-and-forget");
    expect(terminalSrc).toContain("Fire-and-forget like keystrokes");
    expect(count(terminalSrc, ".catch(() => {})")).toBe(2);
  });
});
