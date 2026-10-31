import { describe, expect, test } from "bun:test";
import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("../src/components", import.meta.url));

function vueFiles(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) return vueFiles(path);
    return entry.name.endsWith(".vue") ? [path] : [];
  });
}

const openTags = (src: string, tag: string) =>
  src.match(new RegExp(`<${tag}\\b(?:[^>"']|"[^"]*"|'[^']*')*>`, "g")) ?? [];

describe("markup semantics", () => {
  test("new-tab links always drop the opener reference", () => {
    for (const path of vueFiles(root)) {
      const src = readFileSync(path, "utf8");
      for (const tag of openTags(src, "a")) {
        if (!tag.includes('target="_blank"')) continue;
        expect(tag).toContain('rel="noopener');
      }
    }
  });

  test("tables and header cells expose their structure to assistive tech", () => {
    for (const path of vueFiles(root)) {
      const src = readFileSync(path, "utf8");
      for (const tag of openTags(src, "table")) {
        expect(tag.includes("aria-label") || src.includes("<caption")).toBe(true);
      }
      for (const tag of openTags(src, "th")) {
        expect(tag).toContain("scope=");
      }
    }
  });
});
