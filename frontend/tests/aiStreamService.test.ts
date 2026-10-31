import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const streamSrc = readFileSync(
  fileURLToPath(new URL("../src/services/aiStreamService.ts", import.meta.url)),
  "utf8"
);
const composerSrc = readFileSync(
  fileURLToPath(new URL("../src/components/workspace/CommitComposer.vue", import.meta.url)),
  "utf8"
);
const summarySrc = readFileSync(
  fileURLToPath(new URL("../src/components/history/CommitInspector.vue", import.meta.url)),
  "utf8"
);
const advisorSrc = readFileSync(
  fileURLToPath(new URL("../src/components/workspace/AiConflictAdvisor.vue", import.meta.url)),
  "utf8"
);
const aiServiceSrc = readFileSync(
  fileURLToPath(new URL("../src/services/aiService.ts", import.meta.url)),
  "utf8"
);
const explainSrc = readFileSync(
  fileURLToPath(new URL("../src/components/workspace/AiExplainDiffModal.vue", import.meta.url)),
  "utf8"
);
const batchSrc = readFileSync(
  fileURLToPath(new URL("../src/components/workspace/AiConflictBatchModal.vue", import.meta.url)),
  "utf8"
);
const studioSrc = readFileSync(
  fileURLToPath(new URL("../src/components/tools/StudioTab.vue", import.meta.url)),
  "utf8"
);
const settingsSrc = readFileSync(
  fileURLToPath(new URL("../src/components/settings/AiConnectionForm.vue", import.meta.url)),
  "utf8"
);

const dict = (m: object) => m as Record<string, string>;

describe("AI streaming transport", () => {
  test("subscribes to all three events before starting the backend request", () => {
    expect(streamSrc).toContain('import { Call, Events } from "@wailsio/runtime"');
    for (const event of ["ai:stream:delta", "ai:stream:done", "ai:stream:error"]) {
      expect(streamSrc).toContain(event);
    }
    expect(streamSrc).toContain("disposers.push(");
    const startCall = streamSrc.indexOf('"main.AiService.StartAIStream"');
    const firstListener = streamSrc.indexOf("Events.On(DELTA_EVENT");
    expect(firstListener).toBeGreaterThan(-1);
    expect(startCall).toBeGreaterThan(firstListener);
  });

  test("exposes a completion promise and cancels through CancelAIRequest", () => {
    expect(streamSrc).toContain("completion: Promise<AIStreamOutcome>");
    expect(streamSrc).toContain('"main.AiService.CancelAIRequest"');
    expect(streamSrc).toContain("finish({ kind: \"cancelled\" })");
    expect(streamSrc).toContain("onDelta?: (delta: string) => void");
    expect(streamSrc).toContain("includeFile: input.includeFile === true");
    expect(aiServiceSrc).toContain("includeFile: input.includeFile === true");
  });

  test("commit, summary and conflict surfaces use the streaming entry point", () => {
    expect(composerSrc).toContain('kind: "commit"');
    expect(composerSrc).toContain("session.completion");
    expect(composerSrc).toContain("aiStreamText.value += delta");
    expect(summarySrc).toContain('kind: "summary"');
    expect(summarySrc).toContain("aiSummary.value += delta");
    expect(advisorSrc).toContain('kind: "conflict"');
    expect(advisorSrc).toContain("editedCode.value += delta");
  });

  test("explain-diff uses the stream transport with an explicit stop path", () => {
    expect(explainSrc).toContain('kind: "explain"');
    expect(explainSrc).toContain("startAIStream");
    expect(explainSrc).toContain("current.cancel()");
  });

  test("batch conflict advice streams each block and applies by content", () => {
    expect(batchSrc).toContain('kind: "conflict"');
    expect(batchSrc).toContain("startAIStream");
    expect(batchSrc).toContain("applyConflictSuggestionByContent");
    expect(batchSrc).toContain("runAll");
    expect(batchSrc).toContain("active.cancel()");
  });

  test("AI calls are filterable in the Studio log with mode/model metadata", () => {
    expect(studioSrc).toContain("logSourceFilter");
    expect(studioSrc).toContain("studio.logs.filter.ai");
    expect(studioSrc).toContain("log.source");
    expect(studioSrc).toContain("studio.logs.model");
    expect(studioSrc).toContain("studio.logs.cancelled");
  });

  test("provider protocol, custom headers and CLI env are exposed in settings", () => {
    expect(settingsSrc).toContain('data-testid="ai-connection-protocol"');
    expect(settingsSrc).toContain('data-testid="ai-connection-headers"');
    expect(settingsSrc).toContain('data-testid="localai-cli-env"');
    expect(settingsSrc).toContain("form.protocol");
    expect(aiServiceSrc).toContain("azureApiVersion");
    expect(aiServiceSrc).toContain("extraHeaders");
  });

  test("legacy non-stream AI exports remain available for compatibility", () => {
    expect(aiServiceSrc).toContain("export function generateCommitMessage");
    expect(aiServiceSrc).toContain("export function summarizeCommit");
  });
});

describe("AI streaming copy", () => {
  test("both locales carry the stop/waiting labels", () => {
    for (const key of [
      "ai.stream.cancel",
      "ai.stream.waiting",
      "workspace.diff.ai.explain.title",
      "workspace.conflict.batch.title",
      "studio.logs.filter.ai",
      "settings.localai.protocol.label",
      "settings.localai.commit.title",
    ]) {
      expect(typeof dict(zhCN)[key]).toBe("string");
      expect(dict(zhCN)[key].length).toBeGreaterThan(0);
      expect(typeof dict(enUS)[key]).toBe("string");
      expect(dict(enUS)[key].length).toBeGreaterThan(0);
    }
  });
});
