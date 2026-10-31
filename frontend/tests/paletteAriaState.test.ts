import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

const read = (path: string) => readFileSync(fileURLToPath(new URL(path, import.meta.url)), "utf8");

describe("palette combobox state", () => {
  test("command palette only names an active option when results exist", () => {
    const src = read("../src/components/CommandPalette.vue");
    expect(src).toContain(
      ':aria-activedescendant="filteredCommands.length > 0 ? \'palette-item-\' + selectedIndex : undefined"'
    );
  });

  test("quick open only names an active option when results exist", () => {
    const src = read("../src/components/QuickOpen.vue");
    expect(src).toContain(
      ':aria-activedescendant="resultsCount > 0 ? \'quickopen-item-\' + selectedIndex : undefined"'
    );
  });
});
