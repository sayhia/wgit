import { describe, expect, test } from "bun:test";
import { readFileSync, readdirSync, existsSync } from "node:fs";
import { join } from "node:path";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const REPO_ROOT = join(__dirname, "../..");
const SERVICE_TS = join(REPO_ROOT, "frontend/src/services/gitService.ts");
const DIST_INDEX = join(REPO_ROOT, "frontend/dist/index.html");

const GO_RPC = /^func \([^)]*GitService\) ([A-Z]\w*)/gm;
const FE_CALL = /callBackend(?:<[^>]+>)?\(\s*"(\w+)"/g;

function getGoRpcs(): Set<string> {
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

function getTsRpcs(): Set<string> {
  const source = readFileSync(SERVICE_TS, "utf8");
  const names = new Set<string>();
  for (const match of source.matchAll(FE_CALL)) {
    names.add(match[1]!);
  }
  return names;
}

describe("Final Delivery & System Verification Suite (Round 20)", () => {
  test("100% RPC parity between Go backend GitService and TypeScript gitService", () => {
    const goRpcs = getGoRpcs();
    const tsRpcs = getTsRpcs();

    expect(goRpcs.size).toBeGreaterThanOrEqual(374);
    expect(tsRpcs.size).toBeGreaterThanOrEqual(374);

    const missingInTs = [...goRpcs].filter((rpc) => !tsRpcs.has(rpc));
    const extraInTs = [...tsRpcs].filter((rpc) => !goRpcs.has(rpc));

    expect(missingInTs).toEqual([]);
    expect(extraInTs).toEqual([]);
    expect(tsRpcs.size).toBe(goRpcs.size);
  });

  test("i18n catalogs zh-CN and en-US are strictly symmetric with 0 untranslated entries", () => {
    const zhKeys = Object.keys(zhCN);
    const enKeys = Object.keys(enUS);

    expect(zhKeys.length).toBeGreaterThanOrEqual(1000);
    expect(enKeys.length).toBe(zhKeys.length);

    const missingInEn = zhKeys.filter((k) => !(k in enUS));
    const missingInZh = enKeys.filter((k) => !(k in zhCN));

    expect(missingInEn).toEqual([]);
    expect(missingInZh).toEqual([]);

    for (const [key, val] of Object.entries(zhCN)) {
      expect((val as string).trim().length).toBeGreaterThan(0);
    }
    for (const [key, val] of Object.entries(enUS)) {
      expect((val as string).trim().length).toBeGreaterThan(0);
    }
  });

  test("production frontend build artifacts exist and are valid", () => {
    expect(existsSync(DIST_INDEX)).toBe(true);
    const html = readFileSync(DIST_INDEX, "utf8");
    expect(html).toContain("<!doctype html>");
    expect(html).toContain('<div id="app"');
    expect(html).toContain("assets/");
  });

  test("core architecture invariant: zero 'mygo' references in codebase", () => {
    const forbidden = "mygo";
    for (const file of ["main.go", "git_service.go", "package.json"]) {
      const p = join(REPO_ROOT, file);
      if (existsSync(p)) {
        const text = readFileSync(p, "utf8");
        expect(text.toLowerCase()).not.toContain(forbidden);
      }
    }
  });
});
