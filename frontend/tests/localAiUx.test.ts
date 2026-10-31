import { describe, expect, test } from "bun:test";
import {
  clearLegacyLocalAiKey,
  extractConflictBlocks,
  extractFirstConflictBlock,
  heuristicConflictAdvice,
  parseConflictAdvice,
  parseConflictFileBlocks,
  readLegacyLocalAiConfig,
  requestConflictAdvice,
  setConflictAdviceBackendForTest,
} from "../src/services/localAiService";
import { apiOrigin, isSameApiOrigin } from "../src/services/aiService";
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

  test("parseConflictFileBlocks preserves raw file text, CRLF and diff3 base", () => {
    const raw = "head\r\n<<<<<<< HEAD\r\n+const a = 1;\r\n const b = 2;\r\n||||||| base\r\nconst old = 0;\r\n=======\r\nconst c = 3;\r\n>>>>>>> incoming\r\n";
    const blocks = parseConflictFileBlocks(raw);
    expect(blocks.length).toBe(1);
    expect(blocks[0]?.ours).toBe("+const a = 1;\n const b = 2;");
    expect(blocks[0]?.theirs).toBe("const c = 3;");
    expect(blocks[0]?.base).toBe("const old = 0;");
  });

  test("backend failure is explicit and the heuristic fallback is labeled as such", async () => {
    setConflictAdviceBackendForTest(async () => {
      throw new Error("feature disabled (aiConflictBatch flag is off)");
    });
    try {
      await expect(
        requestConflictAdvice({
          ours: "import { ref } from 'vue';\nconst a = 1;",
          theirs: "import { reactive } from 'vue';\nconst b = 2;",
          filePath: "src/App.vue",
        })
      ).rejects.toThrow("feature disabled");
    } finally {
      setConflictAdviceBackendForTest(null);
    }

    const fallback = heuristicConflictAdvice({
      ours: "import { ref } from 'vue';\nconst a = 1;",
      theirs: "import { reactive } from 'vue';\nconst b = 2;",
    });
    expect(fallback.source).toBe("heuristic");
    expect(fallback.recommendedResolution).toContain("const a = 1;");
    expect(fallback.recommendedResolution).toContain("const b = 2;");
    expect(fallback.oursSummary).toBeDefined();
    expect(fallback.theirsSummary).toBeDefined();
    expect(fallback).not.toHaveProperty("confidence");
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
      "workspace.conflict.ai.block.status",
      "workspace.conflict.ai.source.ai",
      "workspace.conflict.ai.source.heuristic",
      "workspace.conflict.ai.heuristic.notice",
      "workspace.conflict.ai.error",
      "workspace.conflict.ai.retry",
      "workspace.conflict.ai.use.heuristic",
      "workspace.conflict.ai.no.advice",
      "workspace.conflict.ai.ours",
      "workspace.conflict.ai.theirs",
      "workspace.conflict.ai.recommended",
      "workspace.conflict.ai.apply.block",
      "ai.stream.cancel",
      "ai.stream.waiting",
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

describe("Phase F5 step 1: conflict advice migrates to the backend channel", () => {
  test("requestConflictAdvice prefers the backend channel and parses the raw suggestion", async () => {
    const seen: Array<[string, string, string]> = [];
    setConflictAdviceBackendForTest(async (ours, theirs, base) => {
      seen.push([ours, theirs, base]);
      return "```\nmerged code line\nconst x = 1;\n```\nRisk: ordering may change.\nRisk: verify imports.";
    });
    try {
      const advice = await requestConflictAdvice({
        ours: "const a = 1;",
        theirs: "const b = 2;",
        base: "const base = 0;",
      });

      // The backend suggestion wins over the deterministic synthesizer.
      expect(advice.recommendedResolution).toBe("merged code line\nconst x = 1;");
      expect(advice.explanation).toContain("Risk: ordering may change.");
      expect(advice.explanation).toContain("Risk: verify imports.");
      expect(advice.source).toBe("ai");
      expect(advice).not.toHaveProperty("confidence");
      expect(advice.oursSummary).toBeDefined();
      expect(advice.theirsSummary).toBeDefined();

      // ours/theirs/base reach the backend RPC by its full AiService name.
      expect(seen.length).toBe(1);
      expect(seen[0]?.[0]).toBe("const a = 1;");
      expect(seen[0]?.[1]).toBe("const b = 2;");
      expect(seen[0]?.[2]).toBe("const base = 0;");
    } finally {
      setConflictAdviceBackendForTest(null);
    }
  });

  test("parseConflictAdvice extracts the fenced block and risk lines; unfenced text degrades to the whole body", () => {
    const fenced = parseConflictAdvice("```ts\nconst x = 1;\n```\nRisk one.\nRisk two.");
    expect(fenced.recommendedResolution).toBe("const x = 1;");
    expect(fenced.explanation).toBe("Risk one.\nRisk two.");

    // A completion ignoring the output contract still yields usable code.
    const unfenced = parseConflictAdvice("plain merged result");
    expect(unfenced.recommendedResolution).toBe("plain merged result");
    expect(unfenced.explanation).toBe("");
  });

  test("legacy localStorage keys are read explicitly and removed without touching other fields", () => {
    localStorage.setItem(
      "wgit.localai.config.v1",
      JSON.stringify({ endpoint: "http://localhost:11434/v1", model: "llama3", apiKey: "legacy-plaintext-key" })
    );
    const legacy = readLegacyLocalAiConfig();
    expect(legacy).toEqual({
      endpoint: "http://localhost:11434/v1",
      model: "llama3",
      apiKey: "legacy-plaintext-key",
    });

    clearLegacyLocalAiKey();
    expect(localStorage.getItem("wgit.localai.config.v1")).toBe(
      JSON.stringify({ endpoint: "http://localhost:11434/v1", model: "llama3" })
    );
    expect(readLegacyLocalAiConfig()).toBeNull();
    localStorage.removeItem("wgit.localai.config.v1");
  });

  test("API origin identity ignores paths but distinguishes host, scheme and port", () => {
    expect(apiOrigin("http://127.0.0.1:18080/v1")).toBe("http://127.0.0.1:18080");
    expect(apiOrigin("http://127.0.0.1:18080/v2")).toBe("http://127.0.0.1:18080");
    expect(apiOrigin("https://API.Example.com/v1")).toBe("https://api.example.com:443");
    expect(isSameApiOrigin("http://127.0.0.1:18080/v1", "http://127.0.0.1:18080/v2")).toBe(true);
    expect(isSameApiOrigin("http://127.0.0.1:18080/v1", "http://127.0.0.1:18081/v1")).toBe(false);
    expect(isSameApiOrigin("http://127.0.0.1:18080/v1", "https://127.0.0.1:18080/v1")).toBe(false);
    expect(isSameApiOrigin("localhost:11434", "http://localhost:11434")).toBe(false);
  });
});

describe("Advisor apply safety wiring", () => {
  test("WorkspaceView uses the block-scoped RPC and handles stale files", () => {
    const src = readFileSync(resolve(__dirname, "../src/components/WorkspaceView.vue"), "utf-8");
    expect(src).toContain("parseConflictFileBlocks");
    expect(src).toContain("resolveConflictBlockWithCustomContent");
    expect(src).toContain("if (res.stale)");
    expect(src).toContain("loadAiAdvisorBlock");
    expect(src).not.toContain("resolveConflictWithCustomContent(\n      props.currentRepoPath,\n      filePath,\n      code");
  });

  test("DiffViewer passes the active block index and gates the advisor by flag", () => {
    const src = readFileSync(resolve(__dirname, "../src/components/workspace/DiffViewer.vue"), "utf-8");
    expect(src).toContain('<FeatureGate flag="aiConflictBatch">');
    expect(src).toContain("emit('open-ai-advisor', activeConflictIndex)");
  });

  test("Advisor streams its suggestion and exposes an explicit stop action", () => {
    const src = readFileSync(resolve(__dirname, "../src/components/workspace/AiConflictAdvisor.vue"), "utf-8");
    expect(src).toContain('kind: "conflict"');
    expect(src).toContain("editedCode.value += delta");
    expect(src).toContain("cancelAdviceStream");
    expect(src).not.toContain("requestConflictAdvice(");
  });
});
