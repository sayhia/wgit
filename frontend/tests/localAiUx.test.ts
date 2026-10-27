import { describe, expect, test } from "bun:test";
import {
  getLocalAiConfig,
  saveLocalAiConfig,
  extractConflictBlocks,
  extractFirstConflictBlock,
  requestConflictAdvice,
} from "../src/services/localAiService";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";
import { readFileSync, existsSync } from "fs";
import { resolve } from "path";

// Headless polyfill for localStorage
if (typeof (globalThis as any).localStorage === "undefined" || !(globalThis as any).localStorage?.getItem) {
  const store = new Map<string, string>();
  (globalThis as any).localStorage = {
    getItem: (k: string) => store.get(k) ?? null,
    setItem: (k: string, v: string) => store.set(k, String(v)),
    removeItem: (k: string) => store.delete(k),
    clear: () => store.clear(),
  };
}

describe("Phase E4: Local AI & Merge Conflict Advisor UX", () => {
  test("getLocalAiConfig and saveLocalAiConfig persist preferences", () => {
    const initial = getLocalAiConfig();
    expect(initial.endpoint).toBeDefined();
    expect(initial.model).toBeDefined();

    const custom = {
      ...initial,
      model: "qwen2.5-coder:7b",
      temperature: 0.1,
    };
    saveLocalAiConfig(custom);
    const retrieved = getLocalAiConfig();
    expect(retrieved.model).toBe("qwen2.5-coder:7b");
    expect(retrieved.temperature).toBe(0.1);
  });

  test("extractConflictBlocks correctly parses standard Git 2-way conflict markers", () => {
    const raw = `const x = 1;
<<<<<<< HEAD
const greeting = "Hello from local";
=======
const greeting = "Hello from remote";
>>>>>>> feat/branch
return greeting;`;

    const blocks = extractConflictBlocks(raw);
    expect(blocks.length).toBe(1);
    expect(blocks[0].ours.trim()).toBe('const greeting = "Hello from local";');
    expect(blocks[0].theirs.trim()).toBe('const greeting = "Hello from remote";');
    expect(blocks[0].base).toBeUndefined();
  });

  test("extractConflictBlocks correctly parses diff3 format with base", () => {
    const raw = `<<<<<<< HEAD
func Compute() int { return 10 }
||||||| base
func Compute() int { return 0 }
=======
func Compute() int { return 20 }
>>>>>>> feat/fast`;

    const blocks = extractConflictBlocks(raw);
    expect(blocks.length).toBe(1);
    expect(blocks[0].ours.trim()).toBe("func Compute() int { return 10 }");
    expect(blocks[0].base?.trim()).toBe("func Compute() int { return 0 }");
    expect(blocks[0].theirs.trim()).toBe("func Compute() int { return 20 }");
  });

  test("extractFirstConflictBlock returns first conflict or falls back to diff lines", () => {
    const single = `<<<<<<< HEAD
ours
=======
theirs
>>>>>>> incoming`;
    const res = extractFirstConflictBlock(single);
    expect(res).not.toBeNull();
    expect(res?.ours).toBe("ours");
    expect(res?.theirs).toBe("theirs");

    // Diff fallback
    const diff = `@@ -1,2 +1,2 @@
-removed line
+added line`;
    const diffRes = extractFirstConflictBlock(diff);
    expect(diffRes).not.toBeNull();
    expect(diffRes?.ours).toBe("removed line");
    expect(diffRes?.theirs).toBe("added line");
  });

  test("requestConflictAdvice offline deterministic synthesizer merges both sides non-destructively", async () => {
    const advice = await requestConflictAdvice({
      ours: "import { ref } from 'vue';\nconst a = 1;",
      theirs: "import { reactive } from 'vue';\nconst b = 2;",
      filePath: "src/App.vue",
    });

    expect(advice.recommendedResolution).toContain("const a = 1;");
    expect(advice.recommendedResolution).toContain("const b = 2;");
    expect(advice.confidence).toBeGreaterThanOrEqual(80);
    expect(advice.oursSummary).toBeDefined();
    expect(advice.theirsSummary).toBeDefined();
    expect(advice.explanation).toBeDefined();
  });

  test("i18n catalogs contain complete local AI keys in both languages", () => {
    const requiredKeys = [
      "settings.category.localai",
      "settings.localai.title",
      "settings.localai.intro",
      "settings.localai.switch.title",
      "settings.localai.switch.desc",
      "settings.localai.endpoint.label",
      "settings.localai.model.label",
      "settings.localai.test.btn",
      "workspace.conflict.ai.advisor",
      "workspace.conflict.ai.title",
      "workspace.conflict.ai.confidence",
      "workspace.conflict.ai.ours",
      "workspace.conflict.ai.theirs",
      "workspace.conflict.ai.recommended",
      "workspace.conflict.ai.apply",
    ];

    for (const key of requiredKeys) {
      expect((zhCN as Record<string, string>)[key]).toBeDefined();
      expect((enUS as Record<string, string>)[key]).toBeDefined();
    }
  });

  test("LocalAiSection and AiConflictAdvisor components exist and have no hardcoded Chinese in templates", () => {
    const advisorPath = resolve(__dirname, "../src/components/workspace/AiConflictAdvisor.vue");
    const sectionPath = resolve(__dirname, "../src/components/settings/LocalAiSection.vue");

    expect(existsSync(advisorPath)).toBe(true);
    expect(existsSync(sectionPath)).toBe(true);

    const advisorSrc = readFileSync(advisorPath, "utf-8");
    const sectionSrc = readFileSync(sectionPath, "utf-8");

    // In templates (between <template> and </template>), no direct Chinese characters outside t(...) calls
    const extractTemplate = (src: string) => {
      const match = src.match(/<template>([\s\S]*?)<\/template>/);
      return match ? match[1] : "";
    };

    const advisorTemplate = extractTemplate(advisorSrc);
    const sectionTemplate = extractTemplate(sectionSrc);

    // Stripping comments and strings within t("...")
    const stripI18n = (tpl: string) =>
      tpl.replace(/t\(['"][^'"]+['"]\)/g, "").replace(/<!--[\s\S]*?-->/g, "");

    const chineseRegex = /[\u4e00-\u9fa5]/;
    expect(chineseRegex.test(stripI18n(advisorTemplate))).toBe(false);
    expect(chineseRegex.test(stripI18n(sectionTemplate))).toBe(false);
  });
});
