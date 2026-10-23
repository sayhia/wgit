import { describe, expect, test } from "bun:test";
import { parseUnifiedPatch } from "../src/utils/prPatch";
import {
  checksDot,
  fileStatusChip,
  fileStatusLabel,
  formatRelativeTime,
  prStateChip,
  prStateLabel,
} from "../src/components/pr/meta";
import {
  canConfirm,
  confirmAction,
  confirmState,
  resetConfirm,
  settleConfirm,
  type ConfirmState,
} from "../src/composables/useConfirm";

const SAMPLE = [
  "diff --git a/Assets/Water.mat b/Assets/Water.mat",
  "index 111..222 100644",
  "--- a/Assets/Water.mat",
  "+++ b/Assets/Water.mat",
  "@@ -10,3 +10,4 @@",
  " context line",
  "-old line",
  "+new line",
  "+added line",
  " trailing context",
].join("\n");

describe("parseUnifiedPatch", () => {
  test("counts additions and deletions from the body, not the header", () => {
    const model = parseUnifiedPatch(SAMPLE);
    expect(model.additions).toBe(2);
    expect(model.deletions).toBe(1);
  });

  test("numbers old and new sides independently across a hunk", () => {
    const model = parseUnifiedPatch(SAMPLE);
    const body = model.lines.filter((line) => line.kind !== "meta");
    expect(body.map((line) => [line.oldLine, line.newLine])).toEqual([
      [10, 10],
      [11, null],
      [null, 11],
      [null, 12],
      [12, 13],
    ]);
  });

  test("restarts numbering at each hunk header", () => {
    const twoHunks = [
      "@@ -1,1 +1,1 @@",
      "-a",
      "+b",
      "@@ -20,1 +21,1 @@",
      "-c",
      "+d",
    ].join("\n");
    const model = parseUnifiedPatch(twoHunks);
    const body = model.lines.filter((line) => line.kind !== "meta");
    expect(body.map((line) => [line.oldLine, line.newLine])).toEqual([
      [1, null],
      [null, 1],
      [20, null],
      [null, 21],
    ]);
  });

  test("treats the no-newline marker and file headers as meta", () => {
    const model = parseUnifiedPatch("@@ -1 +1 @@\n-a\n\\ No newline at end of file\n+b");
    const meta = model.lines.filter((line) => line.kind === "meta");
    expect(meta).toHaveLength(2);
    expect(meta[1]!.text).toContain("No newline");
    expect(model.additions).toBe(1);
    expect(model.deletions).toBe(1);
  });

  test("returns an empty model for an empty patch", () => {
    const model = parseUnifiedPatch("");
    expect(model.lines).toEqual([]);
    expect(model.additions).toBe(0);
  });
});

describe("PR meta", () => {
  test("draft open PRs are labelled as drafts but keep the open chip family", () => {
    expect(prStateLabel("open", true)).not.toBe(prStateLabel("open", false));
    expect(prStateChip("open", true)).toContain("text-ink-muted");
    expect(prStateChip("open", false)).toContain("text-success");
  });

  test("merged and closed use distinct semantic colours", () => {
    expect(prStateChip("merged")).toContain("text-purple");
    expect(prStateChip("closed")).toContain("text-danger");
  });

  test("file status labels cover the GitHub vocabulary", () => {
    expect(fileStatusLabel("added")).toBe("新增");
    expect(fileStatusLabel("removed")).toBe("删除");
    expect(fileStatusLabel("renamed")).toBe("重命名");
    expect(fileStatusLabel("modified")).toBe("修改");
    expect(fileStatusChip("removed")).toContain("text-danger");
  });

  test("checks dot maps pending/failure/success to different tokens", () => {
    const dots = new Set([
      checksDot("success"),
      checksDot("failure"),
      checksDot("pending"),
      checksDot(""),
    ]);
    expect(dots.size).toBe(4);
  });

  test("relative time degrades to buckets and raw strings", () => {
    const now = Date.parse("2026-10-23T12:00:00Z");
    expect(formatRelativeTime("2026-10-23T11:59:40Z", now)).toBe("刚刚");
    expect(formatRelativeTime("2026-10-23T11:30:00Z", now)).toBe("30 分钟前");
    expect(formatRelativeTime("2026-10-23T08:00:00Z", now)).toBe("4 小时前");
    expect(formatRelativeTime("2026-10-20T12:00:00Z", now)).toBe("3 天前");
    expect(formatRelativeTime("not-a-date", now)).toBe("not-a-date");
    expect(formatRelativeTime("", now)).toBe("");
  });
});

function state(overrides: Partial<ConfirmState>): ConfirmState {
  return {
    open: true,
    message: "x",
    requireTextValue: "",
    busy: false,
    ...overrides,
  };
}

describe("confirmAction", () => {
  test("requires an exact typed phrase when requireText is set", () => {
    expect(canConfirm(state({ requireText: "wgit" }))).toBe(false);
    expect(canConfirm(state({ requireText: "wgit", requireTextValue: "wgi" }))).toBe(false);
    expect(canConfirm(state({ requireText: "wgit", requireTextValue: " wgit " }))).toBe(true);
    expect(canConfirm(state({}))).toBe(true);
  });

  test("never confirms while busy or closed", () => {
    expect(canConfirm(state({ busy: true }))).toBe(false);
    expect(canConfirm(state({ open: false }))).toBe(false);
  });

  test("settleConfirm resolves the pending promise", async () => {
    resetConfirm();
    const pending = confirmAction({ message: "删除？", tone: "danger" });
    expect(confirmState.open).toBe(true);
    expect(confirmState.tone).toBe("danger");
    settleConfirm(true);
    await expect(pending).resolves.toBe(true);
    expect(confirmState.open).toBe(false);
  });

  test("a second prompt cancels the first instead of leaking it", async () => {
    resetConfirm();
    const first = confirmAction("first");
    const second = confirmAction("second");
    await expect(first).resolves.toBe(false);
    settleConfirm(false);
    await expect(second).resolves.toBe(false);
  });

  test("resetConfirm drops state between cases", () => {
    confirmState.requireText = "abc";
    confirmState.requireTextValue = "abc";
    resetConfirm();
    expect(confirmState.requireText).toBeUndefined();
    expect(confirmState.requireTextValue).toBe("");
    expect(confirmState.open).toBe(false);
  });
});
