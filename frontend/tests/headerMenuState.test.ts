import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

const source = readFileSync(
  fileURLToPath(new URL("../src/components/HeaderBar.vue", import.meta.url)),
  "utf8"
);

describe("header dropdown disclosure state", () => {
  test("every dropdown trigger exposes its expanded state", () => {
    expect(source).toContain(':aria-expanded="showRepoMenu"');
    expect(source).toContain(':aria-expanded="showBranchMenu"');
    expect(source).toContain(':aria-expanded="showContextSessionMenu"');
    expect(source).toContain(':aria-expanded="showActionMenu"');
  });
});
