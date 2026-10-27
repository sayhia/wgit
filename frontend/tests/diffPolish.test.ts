import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { sendNativeNotification } from "../src/utils/nativeNotify";

const diffViewerFile = fileURLToPath(
  new URL("../src/components/workspace/DiffViewer.vue", import.meta.url)
);

describe("Polish: Diff MiniMap, Jump Gestures & Native Notify", () => {
  const diffSrc = readFileSync(diffViewerFile, "utf8");

  test("DiffViewer integrates MiniMap gutter and fast hunk/conflict jump handlers", () => {
    expect(diffSrc).toContain("diff-minimap-gutter");
    expect(diffSrc).toContain("miniMapTicks");
    expect(diffSrc).toContain("handleMiniMapClick");
    expect(diffSrc).toContain("scrollToHunk");
    expect(diffSrc).toContain("handleDiffKeydown");
    expect(diffSrc).toContain("workspace.diff.minimap.hint");
  });

  test("sendNativeNotification handles headless environments safely", async () => {
    const sent = await sendNativeNotification("Test Title", "Test Body");
    expect(typeof sent).toBe("boolean");
  });
});
