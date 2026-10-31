import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

const read = (path: string) => readFileSync(fileURLToPath(new URL(path, import.meta.url)), "utf8");

describe("clickable surfaces are keyboard reachable", () => {
  test("copy targets are real buttons instead of bare spans", () => {
    for (const path of [
      "../src/components/tools/SubmodulesTab.vue",
      "../src/components/tools/ReflogTab.vue",
      "../src/components/workspace/SourcePanorama.vue",
    ]) {
      const src = read(path);
      expect(src).not.toMatch(/<span\s+@click[^>]*copyToClipboard/);
      expect(src).not.toMatch(/<span\s+@click\.stop="emit\('copy-hash'/);
      expect(src).toContain('type="button"');
      expect(src).toContain("focus-visible:ring-1");
    }
  });

  test("crash rows expose an expanded state on a native button", () => {
    const src = read("../src/components/CrashReportModal.vue");
    expect(src).toContain(':aria-expanded="expandedIds.has(item.id)"');
    expect(src).toMatch(/<button\s+type="button"[\s\S]{0,800}@click="toggleExpand\(item.id\)"/);
    expect(src).toContain('aria-hidden="true"');
  });

  test("rebase upstream branches behave as a keyboard listbox", () => {
    const src = read("../src/components/RebaseModal.vue");
    expect(src).toContain('role="listbox"');
    expect(src).toContain(":aria-label=\"t('rebase.base.label')\"");
    expect(src).toContain('role="option"');
    expect(src).toContain(':aria-selected="selectedUpstream === b.name"');
    expect(src).toContain('@keydown.enter.prevent="selectedUpstream = b.name"');
    expect(src).toContain('@keydown.space.prevent="selectedUpstream = b.name"');
  });

  test("explorer group files and panorama history rows handle Enter and Space", () => {
    const explorer = read("../src/components/workspace/SourceExplorer.vue");
    expect(explorer).toContain('role="button"');
    expect(explorer).toContain('@keydown.enter.prevent="emit(\'open-file\', file.path)"');
    expect(explorer).toContain('@keydown.space.prevent="emit(\'open-file\', file.path)"');

    const panorama = read("../src/components/workspace/SourcePanorama.vue");
    expect(panorama).toContain('@keydown.enter.prevent="emit(\'view-revision\', c.commitHash)"');
    expect(panorama).toContain('@keydown.space.prevent="emit(\'view-revision\', c.commitHash)"');
    expect(panorama).toContain("focus-visible:ring-2");
  });
});
