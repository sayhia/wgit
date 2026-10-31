import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

const read = (path: string) => readFileSync(fileURLToPath(new URL(path, import.meta.url)), "utf8");

describe("timer hygiene", () => {
  test("dashboard auto-refresh skips ticks while the window is hidden", () => {
    const src = read("../src/components/RepoDashboard.vue");
    expect(src).toMatch(
      /setInterval\(\(\) => \{\s*if \(document\.visibilityState === "visible"\) loadRepoStatuses\(true\);\s*\}, AUTO_REFRESH_INTERVAL_MS\)/
    );
  });
});
