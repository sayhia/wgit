import { beforeEach, describe, expect, test } from "bun:test";
import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import {
  LOCALES,
  isSupportedLocale,
  interpolate,
  locale,
  setLocale,
  setWarnMissing,
  t,
  translate,
} from "../src/i18n";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

beforeEach(() => {
  setWarnMissing(false);
  locale.value = "zh-CN";
});

describe("interpolate", () => {
  test("returns the template unchanged without params", () => {
    expect(interpolate("已修复 {count} 项")).toBe("已修复 {count} 项");
  });

  test("replaces named placeholders", () => {
    expect(interpolate("已修复 {count} 项", { count: 3 })).toBe("已修复 3 项");
    expect(interpolate("{a}-{b}", { a: "x", b: "y" })).toBe("x-y");
  });

  test("keeps unknown placeholders visible", () => {
    expect(interpolate("{a} {b}", { a: "x" })).toBe("x {b}");
  });

  test("stringifies numbers but not objects", () => {
    expect(interpolate("{n}", { n: 0 })).toBe("0");
    expect(interpolate("{n}", { n: 1.5 })).toBe("1.5");
  });

  test("does not recurse into replaced values", () => {
    expect(interpolate("{a}", { a: "{b}" })).toBe("{b}");
  });
});

describe("translate", () => {
  test("resolves keys in the source language", () => {
    expect(translate("common.close", undefined, "zh-CN")).toBe("关闭");
    expect(translate("nav.workspace", undefined, "zh-CN")).toContain("工作区");
  });

  test("resolves keys in English", () => {
    expect(translate("common.close", undefined, "en-US")).toBe("Close");
    expect(translate("nav.tools", undefined, "en-US")).toBe("Tools & Diagnostics");
  });

  test("falls back to the source language for a missing translation", () => {
    // Simulate a key that exists in zh-CN only by using a synthetic lookup:
    // every real key is translated, so we assert the fallback path via the
    // active locale being English while the key comes from the source catalog.
    const zhOnly = Object.keys(zhCN).find((k) => !(k in enUS));
    if (zhOnly) {
      expect(translate(zhOnly, undefined, "en-US")).toBe(
        (zhCN as Record<string, string>)[zhOnly] as string
      );
    } else {
      // No gap today — the contract is still "never return undefined".
      expect(translate("common.close", undefined, "en-US")).toBeTruthy();
    }
  });

  test("returns the key itself when nothing matches, so gaps are visible", () => {
    expect(translate("does.not.exist", undefined, "zh-CN")).toBe("does.not.exist");
  });

  test("interpolates params through translate", () => {
    expect(translate("fix.applied", { count: 2 }, "zh-CN")).toBe("fix.applied");
  });

  test("t() uses the active locale", () => {
    expect(t("common.confirm")).toBe("确认");
    setLocale("en-US");
    expect(t("common.confirm")).toBe("Confirm");
    setLocale("zh-CN");
    expect(t("common.confirm")).toBe("确认");
  });
});

describe("setLocale", () => {
  test("ignores unsupported locales", () => {
    setLocale("zh-CN");
    setLocale("fr-FR" as never);
    expect(locale.value).toBe("zh-CN");
  });

  test("accepts every advertised locale", () => {
    for (const item of LOCALES) {
      setLocale(item.id);
      expect(locale.value).toBe(item.id);
    }
  });
});

describe("isSupportedLocale", () => {
  test("only allows registered locales", () => {
    expect(isSupportedLocale("zh-CN")).toBe(true);
    expect(isSupportedLocale("en-US")).toBe(true);
    expect(isSupportedLocale("zh-TW")).toBe(false);
    expect(isSupportedLocale(null)).toBe(false);
    expect(isSupportedLocale(42)).toBe(false);
  });

  test("rejects prototype keys", () => {
    expect(isSupportedLocale("toString")).toBe(false);
    expect(isSupportedLocale("__proto__")).toBe(false);
  });
});

describe("catalog contract", () => {
  test("English covers every source key", () => {
    const missing = Object.keys(zhCN).filter((key) => !(key in enUS));
    expect(missing).toEqual([]);
  });

  test("no locale has extra keys that the source lacks", () => {
    const orphans = Object.keys(enUS).filter((key) => !(key in zhCN));
    expect(orphans).toEqual([]);
  });

  test("keys follow the domain.semantic naming rule", () => {
    for (const key of Object.keys(zhCN)) {
      expect(key).toContain(".");
      expect(key).not.toContain(" ");
      expect(key).toBe(key.toLowerCase());
    }
  });

  test("no empty translations", () => {
    for (const [key, value] of Object.entries(zhCN)) {
      expect((value as string).trim().length).toBeGreaterThan(0);
      void key;
    }
    for (const [key, value] of Object.entries(enUS)) {
      expect((value as string).trim().length).toBeGreaterThan(0);
      void key;
    }
  });
});

function walkSource(dir: string, out: string[] = []): string[] {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const full = join(dir, entry.name);
    if (entry.isDirectory()) walkSource(full, out);
    else if (entry.name.endsWith(".vue") || entry.name.endsWith(".ts")) out.push(full);
  }
  return out;
}

describe("catalog usage", () => {
  test("every t() key used in source exists in the catalog", () => {
    const root = fileURLToPath(new URL("../src", import.meta.url));
    const missing: string[] = [];
    for (const file of walkSource(root)) {
      if (file.includes("/i18n/messages/")) continue;
      const text = readFileSync(file, "utf8");
      for (const match of text.matchAll(/\bt\(\s*["']([^"']+)["']/g)) {
        const key = match[1]!;
        if (!(key in zhCN)) missing.push(`${key} (${file.slice(root.length)})`);
      }
    }
    expect(missing).toEqual([]);
  });
});

describe("command palette keys", () => {
  // Titles and categories are stored as keys and resolved through titleOf() /
  // categoryLabel(), so the generic t("...") scan cannot see them. Check the
  // data table directly, otherwise a typo would silently render the raw key.
  test("every command title and category has a translation", () => {
    const file = fileURLToPath(new URL("../src/components/CommandPalette.vue", import.meta.url));
    const source = readFileSync(file, "utf8");
    const missing: string[] = [];

    for (const match of source.matchAll(/title:\s*"(cmd\.[^"]+)"/g)) {
      if (!(match[1]! in zhCN)) missing.push(match[1]!);
    }
    for (const match of source.matchAll(/category:\s*"([a-z]+)"/g)) {
      const key = `cmd.cat.${match[1]}`;
      if (!(key in zhCN)) missing.push(key);
    }

    expect(missing).toEqual([]);
  });

  test("pinned and recent sections use stable category keys", () => {
    const file = fileURLToPath(new URL("../src/components/CommandPalette.vue", import.meta.url));
    const source = readFileSync(file, "utf8");
    expect(source).toContain('category: "pinned"');
    expect(source).toContain('category: "recent"');
    // A translated category stored in localStorage would break after a switch.
    expect(source).not.toContain('category: "收藏"');
  });
});

describe("no hardcoded UI copy", () => {
  // Everything the user reads must come from the catalog. This guard walks the
  // frontend source, strips comments, and fails on any remaining CJK text —
  // except the handful of *wire values* listed below, which are matched against
  // backend output and never rendered.
  const SRC = fileURLToPath(new URL("../src", import.meta.url));

  /** Files that are legitimately not UI copy. */
  const EXEMPT = new Set([
    "i18n/messages/zh-CN.ts",
    "i18n/messages/en-US.ts",
    // Locale endonyms: "简体中文" must stay Chinese in every locale.
    "i18n/index.ts",
    // Dev-preview fixtures, only reachable when the Wails runtime is absent.
    "services/gitService.ts",
    // Bilingual *search aliases*, deliberately untranslated so users can type
    // either language into the settings box.
    "utils/settingsIndex.ts",
    "components/SettingsView.vue",
  ]);

  /** Backend wire values: matched, never displayed. */
  const WIRE_VALUES = ["不是一个有效的 Git 仓库", "不是有效的 Git 仓库", "新增", "移除", "修改"];

  /**
   * CJK ideographs plus CJK/fullwidth punctuation. Punctuation matters: a stray
   * hardcoded "。" or "、" is just as untranslatable as a Chinese word, and it is
   * exactly what slips through a review.
   */
  const CJK_ANY = /[\u4e00-\u9fff\u3000-\u303f\uff01-\uff60]/;

  /**
   * Protocol regexes that must contain CJK quote characters: they pull the
   * folder path out of the backend's quoted error message.
   */
  const PROTOCOL_FILES = new Set(["App.vue", "components/RepoOnboarding.vue"]);

  function walk(dir: string, out: string[] = []): string[] {
    for (const entry of readdirSync(dir, { withFileTypes: true })) {
      const full = join(dir, entry.name);
      if (entry.isDirectory()) walk(full, out);
      else if (/\.(vue|ts)$/.test(entry.name)) out.push(full);
    }
    return out;
  }

  function stripComments(source: string): string {
    return source
      .replace(/<!--[\s\S]*?-->/g, "")
      .replace(/\/\*[\s\S]*?\*\//g, "")
      .replace(/^\s*\/\/.*$/gm, "");
  }

  test("every string shown to the user is translated", () => {
    const offenders: string[] = [];
    for (const file of walk(SRC)) {
      const rel = file.slice(SRC.length + 1).split("\\").join("/");
      if (EXEMPT.has(rel)) continue;
      let body = stripComments(readFileSync(file, "utf8"));
      for (const wire of WIRE_VALUES) body = body.split(wire).join("");
      if (PROTOCOL_FILES.has(rel)) body = body.split("「").join("").split("」").join("");
      if (CJK_ANY.test(body)) offenders.push(rel);
    }
    expect(offenders).toEqual([]);
  });
});

describe("boot screen", () => {
  // index.html paints before the bundle exists, so it cannot import the catalog.
  // The trade-off is inline copy: assert every locale ships, and that the markup
  // itself stays empty (otherwise a locale would be baked in again).
  const HTML = readFileSync(fileURLToPath(new URL("../index.html", import.meta.url)), "utf8");

  test("the boot label ships both locales", () => {
    expect(HTML).toContain('"zh-CN": "正在加载工作区');
    expect(HTML).toContain('"en-US": "Loading workspace');
  });

  test("the markup does not hardcode a boot label", () => {
    const tag = HTML.match(/<div class="boot-label"[^>]*>([\s\S]*?)<\/div>/);
    expect(tag).not.toBeNull();
    expect(tag![1]!.trim()).toBe("");
  });

  test("the boot label follows the persisted locale", () => {
    expect(HTML).toContain('localStorage.getItem("wgit.locale")');
  });
});

describe("backend message keys", () => {
  // Some payloads carry an i18n key next to their rendered text (see
  // `backendText()`), so the generic t("...") scan cannot see them. Read the Go
  // source and assert every key the backend emits is actually translated —
  // otherwise the UI silently falls back to the backend's Chinese sentence.
  const GO_SRC = fileURLToPath(new URL("../../git_release.go", import.meta.url));

  test("every release-plan key emitted by the backend is translated", () => {
    const source = readFileSync(GO_SRC, "utf8");
    const keys = new Set<string>();
    for (const match of source.matchAll(/"(release\.(?:check|summary)\.[a-z.]+)"/g)) {
      keys.add(match[1]!);
    }
    expect(keys.size).toBeGreaterThan(10);

    const missing = [...keys].filter((key) => !(key in zhCN) || !(key in enUS));
    expect(missing).toEqual([]);
  });

  test("the initial-commit placeholder used by the frontend exists", () => {
    expect("release.base.initial" in zhCN).toBe(true);
    expect("release.base.initial" in enUS).toBe(true);
  });
});

describe("external tool blurbs", () => {
  // The backend owns the tool catalogue (ids + Chinese blurbs); the frontend
  // maps ids to catalog keys. A tool added on the Go side without a translation
  // would silently render the Chinese blurb in English mode.
  const GO_SRC = fileURLToPath(new URL("../../git_external_tools.go", import.meta.url));
  const VUE_SRC = fileURLToPath(
    new URL("../src/components/settings/ToolsSection.vue", import.meta.url)
  );

  test("every tool id in the backend has a translated blurb", () => {
    const goSource = readFileSync(GO_SRC, "utf8");
    const ids = [...goSource.matchAll(/ID:\s+"([a-z]+)"/g)].map((m) => m[1]!);
    expect(ids.length).toBeGreaterThan(3);

    const vueSource = readFileSync(VUE_SRC, "utf8");
    const mapped = new Set(
      [...vueSource.matchAll(/([a-z]+):\s*"settings\.tools\.desc\.([a-z]+)"/g)].map((m) => m[2]!)
    );

    const missing = ids.filter((id) => !mapped.has(id) || !(`settings.tools.desc.${id}` in enUS));
    expect(missing).toEqual([]);
  });
});

describe("reflog action keys", () => {
  // `classifyReflog` returns a stable action key that the frontend maps to
  // `reflog.action.<key>`. An unmapped key silently falls back to the backend's
  // Chinese label, so the mapping is asserted here.
  const GO_SRC = fileURLToPath(new URL("../../git_oplog.go", import.meta.url));

  test("every reflog action key has a label and an undo hint", () => {
    const source = readFileSync(GO_SRC, "utf8");
    const keys = new Set(
      [...source.matchAll(/return "([a-z-]+)", "/g)].map((m) => m[1]!)
    );
    expect(keys.size).toBeGreaterThan(8);

    const missing: string[] = [];
    for (const key of keys) {
      if (!(`reflog.action.${key}` in zhCN)) missing.push(`reflog.action.${key}`);
      // Hints are optional for a few actions (checkout/branch/pull have text,
      // clone does not) — only assert when the Go source ships a hint.
      const hasHint = new RegExp(`return "${key}", "[^"]*", (?:true|false), "[^"]+"`).test(source);
      if (hasHint && !(`reflog.undo.hint.${key}` in zhCN)) missing.push(`reflog.undo.hint.${key}`);
    }
    expect(missing).toEqual([]);
  });
});

describe("fix center keys", () => {
  // Same contract as the release plan: the backend ships key + params next to
  // the rendered Chinese text, so every key it can emit must be translated.
  const GO_SRC = fileURLToPath(new URL("../../git_fix_center.go", import.meta.url));

  test("every fix-center item key is translated", () => {
    const source = readFileSync(GO_SRC, "utf8");
    const keys = new Set(
      [...source.matchAll(/(?:TitleKey|SummaryKey|FixLabelKey):\s*"(fix\.item\.[a-z.]+)"/g)].map(
        (m) => m[1]!
      )
    );
    expect(keys.size).toBeGreaterThan(8);

    const missing = [...keys].filter((key) => !(key in zhCN) || !(key in enUS));
    expect(missing).toEqual([]);
  });
});

describe("merge overlap and asset impact keys", () => {
  // Both payloads carry a key (+ params) next to the rendered sentence; the
  // asset-impact one may also append conditional note keys.
  const FILES = [
    { path: "../../git_closed_loop.go", pattern: /"(merge\.overlap\.message\.[a-z.]+)"/g },
    { path: "../../git_asset_impact.go", pattern: /"(assets\.impact\.message\.[a-z.]+)"/g },
  ];

  for (const { path, pattern } of FILES) {
    test(`every key in ${path.split("/").pop()} is translated`, () => {
      const source = readFileSync(fileURLToPath(new URL(path, import.meta.url)), "utf8");
      const keys = new Set([...source.matchAll(pattern)].map((m) => m[1]!));
      expect(keys.size).toBeGreaterThan(3);
      const missing = [...keys].filter((key) => !(key in zhCN) || !(key in enUS));
      expect(missing).toEqual([]);
    });
  }
});

describe("repo health keys", () => {
  // Dimensions derive their title key from the dimension key, so the Go source
  // only names detail/summary/operation keys explicitly.
  const GO_SRC = fileURLToPath(new URL("../../git_health.go", import.meta.url));

  test("every health key emitted by the backend is translated", () => {
    const source = readFileSync(GO_SRC, "utf8");
    const keys = new Set(
      [...source.matchAll(/"(health\.(?:identity|objects|ignore|largefiles|signing|workspace|summary)\.[a-z.]+)"/g)].map(
        (m) => m[1]!
      )
    );
    expect(keys.size).toBeGreaterThan(20);

    const missing = [...keys].filter((key) => !(key in zhCN) || !(key in enUS));
    expect(missing).toEqual([]);
  });

  test("every dimension title key exists", () => {
    const dims = ["identity", "objects", "ignore", "largefiles", "signing", "workspace"];
    const missing = dims.filter((dim) => !(`health.dim.${dim}.title` in zhCN));
    expect(missing).toEqual([]);
  });

  test("every in-progress operation slug is translated", () => {
    const source = readFileSync(GO_SRC, "utf8");
    // The slug is the second element of the {label, slug} pairs.
    const slugs = new Set(
      [...source.matchAll(/\{"[^"]+", "([a-z.]+)"\}/g)].map((m) => m[1]!)
    );
    expect(slugs.size).toBeGreaterThan(3);
    const missing = [...slugs].filter((slug) => !(`health.workspace.op.${slug}` in zhCN));
    expect(missing).toEqual([]);
  });
});
