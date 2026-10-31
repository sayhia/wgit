import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

const read = (path: string) => readFileSync(fileURLToPath(new URL(path, import.meta.url)), "utf8");

describe("shared AI settings style tokens", () => {
  test("the component layer defines the AI card, field and input primitives", () => {
    const css = read("../src/style.css");
    for (const token of [".ai-card", ".ai-card-title", ".ai-card-desc", ".ai-badge", ".ai-field-label", ".ai-input", ".ai-hint"]) {
      expect(css).toContain(token);
    }
  });

  test("quick setup, usage and control plane consume the shared tokens", () => {
    for (const path of [
      "../src/components/settings/AiQuickSetup.vue",
      "../src/components/settings/AiUsageCard.vue",
      "../src/components/settings/AiControlPlaneSection.vue",
    ]) {
      const source = read(path);
      expect(source).toContain("ai-card");
      expect(source).toContain("ai-input");
      expect(source).toContain("ai-field-label");
    }
  });

  test("the duplicates the tokens replaced do not creep back in the settings cards", () => {
    for (const path of [
      "../src/components/settings/AiQuickSetup.vue",
      "../src/components/settings/AiUsageCard.vue",
      "../src/components/settings/AiControlPlaneSection.vue",
    ]) {
      const source = read(path);
      expect(source).not.toContain(
        'class="w-full px-3 py-2 rounded-xl bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent"'
      );
      expect(source).not.toContain('class="text-[10px] font-semibold text-ink-muted"');
    }
  });
});
