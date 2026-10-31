import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

const read = (path: string) => readFileSync(fileURLToPath(new URL(path, import.meta.url)), "utf8");

describe("search controls have accessible names", () => {
  test("command palette and quick open expose a label beyond the placeholder", () => {
    const palette = read("../src/components/CommandPalette.vue");
    expect(palette).toContain(':aria-label="t(\'cmd.placeholder\')"');

    const quick = read("../src/components/QuickOpen.vue");
    expect(quick).toContain(':aria-label="');
    expect(quick).toContain("quickopen.placeholder.files");
  });

  test("header search and filter inputs are named", () => {
    const header = read("../src/components/HeaderBar.vue");
    expect(header).toContain(':aria-label="t(\'header.repo.search\')"');
    expect(header).toContain(':aria-label="t(\'header.branch.filter\')"');
  });
});
