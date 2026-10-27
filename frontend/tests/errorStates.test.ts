import { describe, expect, test } from "bun:test";
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";

const ROOT = join(import.meta.dir, "..");
const SRC = join(ROOT, "src");

function walk(dir: string): string[] {
  const out: string[] = [];
  for (const entry of readdirSync(dir)) {
    const path = join(dir, entry);
    if (statSync(path).isDirectory()) out.push(...walk(path));
    else if (/\.(vue|ts)$/.test(entry)) out.push(path);
  }
  return out;
}

const FILES = walk(SRC);

/**
 * Catch blocks whose body has no nested braces.
 *
 * Deliberately shallow: a `try` inside a `catch` is rare enough that skipping it
 * is better than a brace matcher that trips over braces inside strings and
 * template literals — which is exactly how the first version of this guard
 * reported 205 false positives.
 */
function catchBlocks(source: string): { body: string; line: number }[] {
  const out: { body: string; line: number }[] = [];
  const re = /catch\s*(?:\([^)]*\))?\s*\{([^{}]*)\}/g;
  for (const match of source.matchAll(re)) {
    out.push({
      body: match[1] ?? "",
      line: source.slice(0, match.index!).split("\n").length,
    });
  }
  return out;
}

/** A catch may stay quiet, but only when it says why. */
const LOG_ONLY = /^\s*console\.\w+\([\s\S]*?\)\s*;?\s*$/;

describe("error handling contract", () => {
  test("no catch stops at a log line, and none is left empty", () => {
    // The history view used to do `catch (err) { console.error(...) }` and then
    // render the empty state, so a failed load looked like an empty repository.
    // Logging is not handling: either the user hears about it, the code sets
    // state, or it says why silence is correct ("best-effort").
    //
    // Anything richer than a bare log is left alone — this guard is deliberately
    // narrow so it never argues with a judgement call it cannot see.
    const offenders: string[] = [];
    for (const file of FILES) {
      const source = readFileSync(file, "utf8");
      for (const block of catchBlocks(source)) {
        const body = block.body.trim();
        const annotated = /best-effort/i.test(body);
        if (annotated) continue;
        if (body === "" || LOG_ONLY.test(body)) {
          offenders.push(`${file.slice(SRC.length + 1)}:${block.line} ${body.replace(/\s+/g, " ").slice(0, 70)}`);
        }
      }
    }
    expect(offenders).toEqual([]);
  });

  test("failures are surfaced through the shared toast store", () => {
    // `reportError` both logs and shows the message, so a failed action inside a
    // tools tab is no longer invisible.
    const users = FILES.filter((file) => readFileSync(file, "utf8").includes("reportError("));
    expect(users.length).toBeGreaterThanOrEqual(10);
  });

  test("only one Toast is mounted", () => {
    // Eleven mounts used to exist, one per view; two views firing at once drew
    // two panels in the same corner.
    const mounts: string[] = [];
    for (const file of FILES) {
      if (!file.endsWith(".vue")) continue;
      if (file.endsWith("components/ui/Toast.vue")) continue;
      if (/<Toast[\s/>]/.test(readFileSync(file, "utf8"))) mounts.push(file.slice(SRC.length + 1));
    }
    expect(mounts).toEqual(["App.vue"]);
  });

  test("the loading list distinguishes failure from emptiness", () => {
    // The error block must come before the empty block, or
    // `commits.length === 0` wins the race and the failure never renders.
    const view = readFileSync(join(SRC, "components/history/CommitGraphList.vue"), "utf8");
    const errorAt = view.indexOf('v-else-if="error"');
    const emptyAt = view.indexOf('v-else-if="commits.length === 0"');
    expect(errorAt).toBeGreaterThan(-1);
    expect(emptyAt).toBeGreaterThan(-1);
    expect(errorAt).toBeLessThan(emptyAt);
  });
});
