import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

const source = readFileSync(
  fileURLToPath(new URL("../src/components/RepoRail.vue", import.meta.url)),
  "utf8"
);

describe("repo rail health badge", () => {
  test("is a native, keyboard-reachable button with a padded hit area", () => {
    const start = source.indexOf('data-testid="repo-health-badge"');
    expect(start).toBeGreaterThan(-1);
    const tagStart = source.lastIndexOf("<", start);
    const tagEnd = source.indexOf(">", start);
    const tag = source.slice(tagStart, tagEnd + 1);

    expect(tag.startsWith("<button")).toBe(true);
    expect(tag).toContain('type="button"');
    expect(tag).toContain("focus-visible:ring");
    expect(tag).toContain("before:-inset-1.5");
    expect(tag).toContain('@click.stop="openRepoHealth(repo)"');
    expect(tag).not.toContain('role="button"');
  });
});
