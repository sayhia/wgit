import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import {
  confirmAction,
  confirmState,
  resetConfirm,
  settleConfirm,
} from "../src/composables/useConfirm";

const appFile = fileURLToPath(new URL("../src/App.vue", import.meta.url));
const historyFile = fileURLToPath(new URL("../src/components/HistoryView.vue", import.meta.url));
const confirmDialogFile = fileURLToPath(
  new URL("../src/components/ui/ConfirmDialog.vue", import.meta.url)
);

describe("P2: Desktop UX - Drop Folder, Confirm Scope, and Keyboard Nav", () => {
  test("confirmAction captures affectedSummary scope description", async () => {
    resetConfirm();
    const promise = confirmAction({
      message: "Discard all changes?",
      affectedSummary: "Permanently discards 5 untracked files (240 lines)",
      tone: "danger",
    });

    expect(confirmState.open).toBe(true);
    expect(confirmState.affectedSummary).toBe("Permanently discards 5 untracked files (240 lines)");
    expect(confirmState.tone).toBe("danger");

    settleConfirm(true);
    const result = await promise;
    expect(result).toBe(true);
    expect(confirmState.open).toBe(false);
  });

  test("ConfirmDialog renders affectedSummary when provided", () => {
    const dialogSrc = readFileSync(confirmDialogFile, "utf8");
    expect(dialogSrc).toContain("confirmState.affectedSummary");
  });

  test("App.vue opens dropped folders through the native event, not DataTransfer", () => {
    // The HTML5 route looked implemented but could never work: WKWebView leaves
    // `File.path` undefined, so `handleWindowDrop` was a no-op on macOS. The
    // path now arrives from Go via `wgit:open-repo-path`.
    const appSrc = readFileSync(appFile, "utf8");
    expect(appSrc).toContain('addEventListener("wgit:open-repo-path"');
    expect(appSrc).toContain("data-file-drop-target");
    expect(appSrc).toContain("app.drop.repo.title");
    expect(appSrc).not.toContain("isDraggingFolder");
    expect(appSrc).not.toContain("as any)?.path");
  });

  test("HistoryView handles j/k/ArrowDown/ArrowUp and Enter/Space navigation", () => {
    const historySrc = readFileSync(historyFile, "utf8");
    expect(historySrc).toContain('e.key === "ArrowDown" || e.key === "j"');
    expect(historySrc).toContain('e.key === "ArrowUp" || e.key === "k"');
    expect(historySrc).toContain('e.key === " "');
    expect(historySrc).toContain('e.key === "Enter"');
    expect(historySrc).toContain("togglePatchPreview");
  });
});
