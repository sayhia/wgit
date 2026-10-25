import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import {
  parseFileModeChange,
  isBinaryDiffText,
  extractHunkText,
  cleanSelectedLineText,
} from "../src/utils/diffViewerUtils";
import { t, setLocale } from "../src/i18n";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

describe("diffViewerUtils", () => {
  describe("parseFileModeChange", () => {
    test("detects old mode to new mode change (executable)", () => {
      setLocale("zh-CN");
      const diff = [
        "diff --git a/scripts/run.sh b/scripts/run.sh",
        "old mode 100644",
        "new mode 100755",
        "index 111..222",
        "--- a/scripts/run.sh",
        "+++ b/scripts/run.sh",
        "@@ -1,2 +1,2 @@",
      ].join("\n");

      const info = parseFileModeChange(diff);
      expect(info).not.toBeNull();
      expect(info?.oldMode).toBe("100644");
      expect(info?.newMode).toBe("100755");
      expect(info?.desc).toContain("可执行文件");
      expect(info?.text).toContain("100644 → 100755");
    });

    test("detects new mode (regular file)", () => {
      setLocale("zh-CN");
      const diff = [
        "diff --git a/scripts/run.sh b/scripts/run.sh",
        "old mode 100755",
        "new mode 100644",
      ].join("\n");

      const info = parseFileModeChange(diff);
      expect(info).not.toBeNull();
      expect(info?.oldMode).toBe("100755");
      expect(info?.newMode).toBe("100644");
      expect(info?.desc).toContain("常规文件");
    });

    test("detects new file mode", () => {
      const diff = [
        "diff --git a/new-exec.sh b/new-exec.sh",
        "new file mode 100755",
        "index 0000000..e69de29",
      ].join("\n");

      const info = parseFileModeChange(diff);
      expect(info).not.toBeNull();
      expect(info?.oldMode).toBe("—");
      expect(info?.newMode).toBe("100755");
      expect(info?.desc).toContain("可执行文件");
    });

    test("detects deleted file mode", () => {
      const diff = [
        "diff --git a/deleted.txt b/deleted.txt",
        "deleted file mode 100644",
        "index e69de29..0000000",
      ].join("\n");

      const info = parseFileModeChange(diff);
      expect(info).not.toBeNull();
      expect(info?.oldMode).toBe("100644");
      expect(info?.newMode).toBe("—");
      expect(info?.desc).toContain("常规文件");
    });

    test("returns null for normal diff without mode change", () => {
      const diff = [
        "diff --git a/index.ts b/index.ts",
        "index abc..def 100644",
        "--- a/index.ts",
        "+++ b/index.ts",
        "@@ -1,2 +1,2 @@",
      ].join("\n");

      expect(parseFileModeChange(diff)).toBeNull();
    });

    test("renders in English when locale is en-US", () => {
      setLocale("en-US");
      const diff = [
        "diff --git a/run.sh b/run.sh",
        "old mode 100644",
        "new mode 100755",
      ].join("\n");

      const info = parseFileModeChange(diff);
      expect(info?.text).toBe("File mode changed: 100644 → 100755 (executable)");
      setLocale("zh-CN");
    });
  });

  describe("isBinaryDiffText", () => {
    test("detects binary diff marker", () => {
      expect(isBinaryDiffText("Binary files a/image.png and b/image.png differ")).toBe(true);
      expect(isBinaryDiffText("diff --git a/f b/f\nBinary files /dev/null and b/f differ\n")).toBe(true);
    });

    test("returns false for regular diff", () => {
      expect(isBinaryDiffText("diff --git a/a b/b\n+hello\n-world")).toBe(false);
      expect(isBinaryDiffText("")).toBe(false);
    });
  });

  describe("extractHunkText", () => {
    test("extracts only the requested hunk up to the next hunk header", () => {
      const lines = [
        "diff --git a/foo b/foo",
        "--- a/foo",
        "+++ b/foo",
        "@@ -1,3 +1,4 @@",
        " context line 1",
        "-old line",
        "+new line",
        "+added line",
        " context line 2",
        "@@ -10,2 +11,2 @@",
        " context line 3",
        "+extra line",
      ];

      const hunk1 = extractHunkText(lines, 3);
      expect(hunk1).toBe(
        [
          "@@ -1,3 +1,4 @@",
          " context line 1",
          "-old line",
          "+new line",
          "+added line",
          " context line 2",
        ].join("\n")
      );

      const hunk2 = extractHunkText(lines, 9);
      expect(hunk2).toBe(
        [
          "@@ -10,2 +11,2 @@",
          " context line 3",
          "+extra line",
        ].join("\n")
      );
    });

    test("returns empty string for invalid indices", () => {
      expect(extractHunkText([], 0)).toBe("");
      expect(extractHunkText(["a"], -1)).toBe("");
      expect(extractHunkText(["a"], 5)).toBe("");
    });
  });

  describe("cleanSelectedLineText", () => {
    test("strips leading + and preserves indentation", () => {
      expect(cleanSelectedLineText("+  const x = 1;")).toBe("  const x = 1;");
      expect(cleanSelectedLineText("+hello")).toBe("hello");
    });

    test("strips leading - and preserves indentation", () => {
      expect(cleanSelectedLineText("-  const y = 2;")).toBe("  const y = 2;");
      expect(cleanSelectedLineText("-old")).toBe("old");
    });

    test("keeps context lines unchanged", () => {
      expect(cleanSelectedLineText("  context")).toBe("  context");
    });
  });
});

describe("DiffViewer component contract", () => {
  const diffViewerPath = fileURLToPath(
    new URL("../src/components/workspace/DiffViewer.vue", import.meta.url)
  );
  const diffViewerSource = readFileSync(diffViewerPath, "utf8");

  test("does not clip split diff lines with overflow-hidden text-ellipsis", () => {
    // Both split pane columns must not use overflow-hidden text-ellipsis
    expect(diffViewerSource).not.toContain("overflow-hidden text-ellipsis");
  });

  test("provides wrap toggle and horizontal overflow scroll in split mode", () => {
    expect(diffViewerSource).toContain("isSplitWrap");
    expect(diffViewerSource).toContain("overflow-x-auto");
    expect(diffViewerSource).toContain("workspace.diff.split.wrap");
  });

  test("provides copy selected lines and copy hunk buttons", () => {
    expect(diffViewerSource).toContain("copySelectedLines");
    expect(diffViewerSource).toContain("copyHunk");
    expect(diffViewerSource).toContain("workspace.diff.copy.lines");
    expect(diffViewerSource).toContain("workspace.diff.copy.hunk");
    expect(diffViewerSource).toContain("workspace.diff.copy.lines.done");
    expect(diffViewerSource).toContain("workspace.diff.copy.hunk.done");
  });

  test("provides file mode change badge and binary diff banner", () => {
    expect(diffViewerSource).toContain("fileModeChange");
    expect(diffViewerSource).toContain("isBinaryDiff");
    expect(diffViewerSource).toContain("workspace.diff.binary.notice");
  });

  test("all required diff i18n keys exist and are non-empty in zh-CN and en-US", () => {
    const requiredKeys = [
      "workspace.diff.file.mode.changed",
      "workspace.diff.binary.notice",
      "workspace.diff.copy.lines",
      "workspace.diff.copy.lines.done",
      "workspace.diff.copy.hunk",
      "workspace.diff.copy.hunk.done",
      "workspace.diff.split.wrap",
    ] as const;

    for (const key of requiredKeys) {
      expect(key in zhCN).toBe(true);
      expect(key in enUS).toBe(true);
      expect((zhCN as Record<string, string>)[key]?.trim().length).toBeGreaterThan(0);
      expect((enUS as Record<string, string>)[key]?.trim().length).toBeGreaterThan(0);
    }
  });
});
