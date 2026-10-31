import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

/**
 * F1: AI commit-message generation (flag aiCommitMsg, advisor plan §3.2).
 *
 * Asserts the wiring end to end at source level (the suite's established
 * pattern for components that need a live Wails backend): the FeatureGate
 * wrapper, the click call chain, the empty-input/overwrite-confirm branches,
 * the loading state, the RPC plumbing, the Go gate/truncation/post-processing,
 * and bilingual i18n coverage.
 */

const composerFile = fileURLToPath(
  new URL("../src/components/workspace/CommitComposer.vue", import.meta.url)
);
const gitServiceFile = fileURLToPath(
  new URL("../src/services/gitService.ts", import.meta.url)
);
const aiServiceFile = fileURLToPath(
  new URL("../src/services/aiService.ts", import.meta.url)
);
const goAiServiceFile = fileURLToPath(
  new URL("../../git_ai_service.go", import.meta.url)
);

describe("F1: AI commit message generation", () => {
  const composer = readFileSync(composerFile, "utf8");
  const gitService = readFileSync(gitServiceFile, "utf8");
  const aiService = readFileSync(aiServiceFile, "utf8");
  const goSource = readFileSync(goAiServiceFile, "utf8");

  describe("CommitComposer: FeatureGate gating", () => {
    test("the AI button is wrapped in <FeatureGate flag=\"aiCommitMsg\"> and mounts the gate", () => {
      expect(composer).toContain('import FeatureGate from "../ui/FeatureGate.vue"');
      expect(composer).toContain('<FeatureGate flag="aiCommitMsg">');
      expect(composer).toContain("</FeatureGate>");
      // The button lives inside the gate: the gate opens before the button
      // marker and closes after it in the template source order.
      const gateStart = composer.indexOf('<FeatureGate flag="aiCommitMsg">');
      const buttonStart = composer.indexOf('data-testid="ai-generate-commit-msg-btn"');
      const gateEnd = composer.indexOf("</FeatureGate>", gateStart);
      expect(gateStart).toBeGreaterThan(-1);
      expect(buttonStart).toBeGreaterThan(gateStart);
      expect(buttonStart).toBeLessThan(gateEnd);
    });

    test("the unconfigured path notifies ai.notConfigured instead of raising an error stack", () => {
      expect(composer).toContain("isAIConfigured()");
      expect(composer).toContain('t("ai.notConfigured")');
      expect(aiService).toContain("export async function isAIConfigured");
      // Availability is mode-aware: API needs an endpoint (key optional),
      // CLI needs a command.
      expect(aiService).toContain('cfg.mode === "cli"');
      expect(aiService).toContain('cfg.baseUrl.trim() !== ""');
      expect(aiService).toContain('cfg.cliCommand.trim() !== ""');
    });
  });

  describe("CommitComposer: click call chain", () => {
    test("collects the staged diff via the existing per-file diff RPCs", () => {
      expect(composer).toContain("getRepoStatus");
      expect(composer).toContain("getWorkingDiffAdvanced");
      expect(composer).toContain("startAIStream(");
      expect(composer).toContain('emit("update:subject", subject)');
      expect(composer).toContain("startAIStream(");
    });

    test("WorkspaceView passes the repository path to the composer", () => {
      const workspace = readFileSync(
        new URL("../src/components/WorkspaceView.vue", import.meta.url),
        "utf8"
      );
      expect(workspace).toContain(':repo-path="currentRepoPath"');
    });

    test("fills an empty subject directly, confirms before overwriting a draft", () => {
      // The overwrite branch is guarded by a non-empty subject/body check...
      expect(composer).toContain("if (props.subject.trim() || props.body.trim())");
      // ...and routed through the promise-based confirmAction dialog.
      expect(composer).toContain('import { confirmAction } from "../../composables/useConfirm"');
      expect(composer).toContain("confirmAction({ message: t(\"ai.generate.confirmOverwrite\"), tone: \"warning\" })");
      // Cancel keeps the user's draft: the fill only happens after the guard.
      const guard = composer.indexOf("if (props.subject.trim() || props.body.trim())");
      const fill = composer.indexOf('emit("update:subject", subject)');
      expect(guard).toBeGreaterThan(-1);
      expect(fill).toBeGreaterThan(guard);
    });

    test("streamed subject/body output is split without clearing an untouched body", () => {
      expect(composer).toContain("message.indexOf(");
      expect(composer).toContain('emit("update:body", body)');
      expect(composer).toContain("if (body) emit");
    });

    test("outbound payload preview is available before generating", () => {
      expect(composer).toContain("previewAIRequest");
      expect(composer).toContain('data-testid="ai-preview-payload-btn"');
      expect(composer).toContain("AiPayloadPreviewModal");
      expect(aiService).toContain("export function previewAIRequest");
    });

    test("only unconfigured/disabled outcomes may fall back to local rules", () => {
      expect(composer).toContain('| { kind: "failed"; message: string }');
      expect(composer).toContain('if (outcome.kind === "unconfigured" || outcome.kind === "disabled")');
      expect(composer).toContain("await runLocalGeneration()");
      expect(composer).not.toContain("if (await handleAIGenerate()) return;");
    });

    test("failures notify ai.generate.failed with the loading state reset", () => {
      expect(composer).toContain('t("ai.generate.failed")');
      expect(composer).toContain("finally {");
    });
  });

  describe("CommitComposer: loading state", () => {
    test("the button disables and spins its icon while generating", () => {
      expect(composer).toContain("const aiGenerating = ref(false)");
      expect(composer).toContain(':disabled="aiGenerating"');
      expect(composer).toContain('{ \'animate-spin\': aiGenerating }');
      expect(composer).toContain('aiGenerating ? t("ai.generating") : t("ai.generate")');
      expect(composer).toContain("if (aiGenerating.value) return");
      expect(composer).toContain('data-testid="ai-stream-preview"');
      expect(composer).toContain('data-testid="ai-stream-cancel"');
      expect(composer).toContain("aiStreamSession");
      expect(composer).toContain("session.cancel()");
    });
  });

  describe("RPC plumbing", () => {
    test("gitService wraps the AiService RPC by its full name (RPC parity +1)", () => {
      expect(gitService).toContain("generateCommitMessage: (repoPath: string, diff: string): Promise<string>");
      expect(gitService).toContain('Call.ByName("main.AiService.GenerateCommitMessage", repoPath, diff)');
    });

    test("aiService is the thin re-export layer for the composer", () => {
      expect(aiService).toContain('Call.ByName("main.AiService.GetAIConfig")');
      expect(aiService).toContain("aiCommitApi.generateCommitMessage(repoPath, diff)");
      expect(aiService).toContain("export function generateCommitMessage");
    });
  });

  describe("Go backend: git_ai_service.go", () => {
    test("GenerateCommitMessage is exported and flag-gated with a wire-English error", () => {
      expect(goSource).toContain("func (AiService) GenerateCommitMessage(repoPath, diff string) (string, error)");
      expect(goSource).toContain('errors.New("feature disabled (aiCommitMsg flag is off)")');
      expect(goSource).toContain("LoadConfig().Flags.AICommitMsg");
    });

    test("empty diff is rejected and oversized diffs are truncated with a note", () => {
      expect(goSource).toContain("aiCommitMsgMaxDiffBytes = 200 << 10");
      expect(goSource).toContain('errors.New("staged diff 为空，请先暂存改动再生成提交信息")');
      expect(goSource).toContain("[diff truncated to first %d KB]");
      expect(goSource).toContain("func truncateUTF8Safe(");
    });

    test("the fixed prompt template and post-processing live in Go", () => {
      expect(goSource).toContain("const aiCommitMsgPromptTemplate");
      expect(goSource).toContain("conventional-commit style subject line");
      expect(goSource).toContain("func postProcessCommitMessage(");
      // The diff is embedded into the prompt that flows through complete()
      // (whose redactAIPayload scrubs the outbound payload).
      expect(goSource).toContain("prompt += aiCommitMsgDiffEndMarker");
      expect(goSource).toContain('AiService{}.complete(context.Background(), repoPath, prompt, "commit")');
    });
  });

  describe("i18n completeness", () => {
    // Catalog literals follow the lowercase naming contract
    // (tests/i18n.test.ts); the task-specified camelCase spellings are
    // provided by non-enumerable aliases declared at the catalog tails.
    const canonicalKeys = [
      "ai.generate",
      "ai.generating",
      "ai.notconfigured",
      "ai.generate.failed",
      "ai.generate.confirmoverwrite",
      "ai.preview.title",
      "ai.preview.btn",
      "ai.preview.hint",
      "ai.preview.empty",
      "ai.preview.failed",
    ];
    const camelAliases = ["ai.notConfigured", "ai.generate.confirmOverwrite"];

    test("all ai.* keys exist in both zh-CN and en-US catalogs with content", () => {
      for (const key of canonicalKeys) {
        const zh = (zhCN as Record<string, string>)[key];
        const en = (enUS as Record<string, string>)[key];
        expect(zh).toBeDefined();
        expect(zh.length).toBeGreaterThan(0);
        expect(en).toBeDefined();
        expect(en.length).toBeGreaterThan(0);
      }
    });

    test("the camelCase aliases used by the composer resolve in both locales", () => {
      for (const key of camelAliases) {
        expect((zhCN as Record<string, string>)[key]).toBeDefined();
        expect((enUS as Record<string, string>)[key]).toBeDefined();
        expect(composer).toContain(`t("${key}")`);
      }
    });

    test("zh and en localize the button label distinctly", () => {
      expect((zhCN as Record<string, string>)["ai.generate"]).toContain("AI");
      expect((enUS as Record<string, string>)["ai.generate"]).toContain("AI");
      expect((zhCN as Record<string, string>)["ai.generate"]).not.toBe((enUS as Record<string, string>)["ai.generate"]);
    });
  });
});
