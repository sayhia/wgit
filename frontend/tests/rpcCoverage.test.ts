import { describe, expect, test } from "bun:test";
import { readFileSync, readdirSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { join } from "node:path";

/**
 * Guards the "every backend RPC has a frontend entry" claim.
 *
 * docs/architecture-audit.md asserts that back-end-only methods are zero, but
 * that number was counted by hand and drifts as soon as someone adds a method
 * and forgets the wrapper. This test reads both sides and fails when they
 * disagree — in either direction:
 *   · new Go method without a wrapper  → the feature is unreachable from the UI
 *   · wrapper without a Go method      → dead code that fails at runtime
 */

const REPO_ROOT = fileURLToPath(new URL("../..", import.meta.url));
const SERVICE_TS = join(REPO_ROOT, "frontend/src/services/gitService.ts");

/** `func (GitService) X` and `func (s GitService) X` are both valid receivers. */
const GO_RPC = /^func \([^)]*GitService\) ([A-Z]\w*)/gm;
/** Generic and non-generic callBackend forms both appear in the service. */
const FE_CALL = /callBackend(?:<[^>]+>)?\(\s*"(\w+)"/g;

function backendRpcs(): Set<string> {
  const names = new Set<string>();
  for (const entry of readdirSync(REPO_ROOT, { withFileTypes: true })) {
    if (!entry.isFile() || !entry.name.endsWith(".go")) continue;
    if (entry.name.endsWith("_test.go")) continue;
    const source = readFileSync(join(REPO_ROOT, entry.name), "utf8");
    for (const match of source.matchAll(GO_RPC)) {
      names.add(match[1]!);
    }
  }
  return names;
}

function frontendWrapped(): Set<string> {
  const source = readFileSync(SERVICE_TS, "utf8");
  const names = new Set<string>();
  for (const match of source.matchAll(FE_CALL)) {
    names.add(match[1]!);
  }
  return names;
}

describe("RPC coverage", () => {
  test("the scanner actually finds the service surface", () => {
    // A regex that silently matches nothing would make the assertions below
    // pass for the wrong reason.
    expect(backendRpcs().size).toBeGreaterThan(300);
    expect(frontendWrapped().size).toBeGreaterThan(300);
  });

  test("every backend RPC is wrapped by the frontend service", () => {
    const missing = [...backendRpcs()].filter((name) => !frontendWrapped().has(name)).sort();
    expect(missing).toEqual([]);
  });

  test("every frontend wrapper points at a real backend method", () => {
    const orphans = [...frontendWrapped()].filter((name) => !backendRpcs().has(name)).sort();
    expect(orphans).toEqual([]);
  });

  test("the two sides agree on the exact count", () => {
    expect(frontendWrapped().size).toBe(backendRpcs().size);
  });
});
