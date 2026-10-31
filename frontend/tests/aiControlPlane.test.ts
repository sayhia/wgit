import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const read = (path: string): string =>
  readFileSync(fileURLToPath(new URL(path, import.meta.url)), "utf8");

const aiServiceSrc = read("../src/services/aiService.ts");
const streamSrc = read("../src/services/aiStreamService.ts");
const behaviorSrc = read("../src/components/settings/AiBehaviorSection.vue");
const insightsSrc = read("../src/components/settings/AiInsightsSection.vue");
const routingSrc = read("../src/components/settings/AiTaskRoutingCard.vue");
const reliabilitySrc = read("../src/components/settings/AiReliabilityCard.vue");
const profilesSrc = read("../src/components/settings/AiProfilesCard.vue");
const keysSrc = read("../src/components/settings/AiKeysCard.vue");
const diagnosticsSrc = read("../src/components/settings/AiDiagnosticsCard.vue");
const transferSrc = read("../src/components/settings/AiTransferActions.vue");
const usageSrc = read("../src/components/settings/AiUsageCard.vue");
const quickSrc = read("../src/components/settings/AiConnectionForm.vue");
const setupModalSrc = read("../src/components/settings/AiQuickSetupModal.vue");
const repoModalSrc = read("../src/components/workspace/AiRepoOverrideModal.vue");
const composerSrc = read("../src/components/workspace/CommitComposer.vue");
const settingsSrc = read("../src/components/settings/AiConnectionSection.vue");
const settingsIndexSrc = read("../src/utils/settingsIndex.ts");
const settingsViewSrc = read("../src/components/SettingsView.vue");
const chatSrc = read("../src/components/workspace/AiChatModal.vue");
const reviewSrc = read("../src/components/workspace/AiReviewModal.vue");
const releaseSrc = read("../src/components/workspace/AiReleaseNotesModal.vue");
const workspaceSrc = read("../src/components/WorkspaceView.vue");
const diffSrc = read("../src/components/workspace/DiffViewer.vue");
const inspectorSrc = read("../src/components/history/CommitInspector.vue");
const historySrc = read("../src/components/HistoryView.vue");
const studioSrc = read("../src/components/tools/StudioTab.vue");
const batchSrc = read("../src/components/workspace/AiConflictBatchModal.vue");
const bindingsSrc = read("../bindings/wgit/aiservice.ts");
const modelsSrc = read("../bindings/wgit/models.ts");

const zh = zhCN as Record<string, string>;
const en = enUS as Record<string, string>;

describe("AI control plane", () => {
  test("service exposes durable logs, usage, advanced config and origin keys", () => {
    for (const method of [
      "GetAIAdvancedConfig",
      "SaveAIAdvancedConfig",
      "ListAIKeys",
      "SaveAIKeyForOrigin",
      "DeleteAIKeyForOrigin",
      "GetAICallLogs",
      "ClearAICallLogs",
      "ExportAICallLogs",
      "GetAIUsageSummary",
      "GetAIProviderCatalog",
      "ListAIProfiles",
      "ActivateAIProfile",
      "DeleteAIProfile",
      "GetAIRepoOverride",
      "SaveAIRepoOverride",
      "DeleteAIRepoOverride",
    ]) {
      expect(aiServiceSrc).toContain(`"${method}"`);
      expect(bindingsSrc).toContain(`export function ${method}`);
    }
    expect(modelsSrc).toContain("export interface AIUsageSummary");
    expect(modelsSrc).toContain("export interface AICallLog");
    expect(modelsSrc).toContain("export interface AIKeyEntry");
    expect(modelsSrc).toContain("export interface AIAdvancedConfig");
  });

  test("the three settings leaves retain routing, reliability, keys and profiles under their actual owners", () => {
    expect(behaviorSrc).toContain('data-testid="ai-behavior-section"');
    for (const kind of ["commit", "summary", "conflict", "explain", "chat", "review", "release"]) {
      expect(routingSrc).toContain("`ai-task-model-${kind}`");
      expect(zh[`settings.ai.tasks.${kind}`]).toBeTruthy();
      expect(en[`settings.ai.tasks.${kind}`]).toBeTruthy();
    }
    for (const field of ["retryCount", "rateLimitPerMinute", "fallbackBaseUrl", "fallbackModel"]) {
      expect(reliabilitySrc).toContain(`v-model${field === "retryCount" || field === "rateLimitPerMinute" ? ".number" : ""}="${field}"`);
      expect(behaviorSrc).toContain(`advanced.${field}`);
    }
    for (const method of ["saveAIKeyForOrigin", "deleteAIKeyForOrigin"]) expect(keysSrc).toContain(method);
    for (const method of ["listAIProfiles", "activateAIProfile", "deleteAIProfile"]) expect(profilesSrc).toContain(method);
    const behaviorTemplate = behaviorSrc.split("<template>")[1] || "";
    expect(behaviorTemplate).not.toContain("inputUsdPerMTok");
    expect(behaviorTemplate).not.toContain("monthlyBudgetUsd");
    expect(behaviorSrc).toContain("<AiTaskRoutingCard");
    expect(behaviorSrc).toContain("<AiReliabilityCard");
    expect(settingsSrc).toContain("<AiConnectionForm");
    expect(settingsSrc).toContain("<AiProfilesCard");
    expect(settingsSrc).toContain("<AiKeysCard");
    expect(insightsSrc).toContain("<AiUsageCard");
    expect(insightsSrc).toContain("<AiDiagnosticsCard");
    expect(insightsSrc).toContain("<AiTransferActions");
    for (const leaf of ["AiConnectionSection", "AiBehaviorSection", "AiInsightsSection"]) expect(settingsViewSrc).toContain(`<${leaf}`);
  });

  test("usage dashboard shows budget, cost and unknown-token labels", () => {
    expect(usageSrc).toContain('data-testid="ai-usage-card"');
    expect(usageSrc).toContain("usage.budgetPercent");
    expect(usageSrc).toContain("usage.overBudget");
    expect(usageSrc).toContain("costUnknownCalls");
    expect(usageSrc).toContain('data-testid="ai-usage-budget-input"');
    expect(usageSrc).toContain('t("ai.usage.unknown")');
    expect(studioSrc).toContain('data-testid="studio-ai-usage"');
    expect(studioSrc).toContain("getAIUsageSummary");
    expect(studioSrc).toContain("getAICallLogs");
    expect(studioSrc).toContain("log.usageKnown");
    expect(studioSrc).toContain('t("ai.usage.unknown")');
  });

  test("quick setup keeps the first screen to preset, model and key/command", () => {
    expect(quickSrc).toContain('data-testid="ai-quick-setup"');
    expect(quickSrc).toContain("ai-quick-preset-${provider.id}");
    expect(quickSrc).toContain('data-testid="ai-quick-model"');
    expect(quickSrc).toContain('data-testid="ai-quick-key"');
    expect(quickSrc).toContain('data-testid="ai-quick-command"');
    expect(quickSrc).toContain('data-testid="ai-quick-save"');
    expect(quickSrc).toContain("getAIProviderCatalog");
    expect(quickSrc).toContain("findModelPrice");
    // Profiles have one dedicated card, shared by neither connection form nor
    // compact first-run wizard.
    expect(quickSrc).not.toContain("listAIProfiles");
    expect(quickSrc).not.toContain("activateAIProfile");
    expect(profilesSrc).toContain('data-testid="ai-profiles-card"');
    // Transport details stay folded inside the single connection form.
    expect(quickSrc).toContain('data-testid="ai-connection-advanced"');
    expect(quickSrc).toContain('data-testid="ai-quick-endpoint"');
    expect(quickSrc).toContain("<details");
    expect(quickSrc).toContain("form.protocol === 'azure'");
  });

  test("quick setup surfaces load failures and exposes accessible selection state", () => {
    expect(quickSrc).toContain("loadError");
    expect(quickSrc).toContain('data-testid="ai-quick-load-error"');
    expect(quickSrc).toContain('data-testid="ai-quick-retry"');
    expect(quickSrc).toContain('data-testid="ai-quick-loading"');
    expect(quickSrc).toContain('role="alert"');
    expect(quickSrc).toContain('aria-pressed="apiPresetId === provider.id"');
    expect(quickSrc).not.toContain("ai-quick-profile-");
    expect(quickSrc).toContain('for="ai-quick-model-input"');
    expect(quickSrc).toContain('id="ai-quick-model-input"');
    expect(quickSrc).toContain('for="ai-quick-key-input"');
    expect(quickSrc).toContain('for="ai-quick-command-input"');
  });

  test("extracted settings cards retain retryable errors and labelled icon buttons", () => {
    for (const [source, errorId, retryId] of [
      [behaviorSrc, "ai-behavior-load-error", "ai-behavior-retry"],
      [profilesSrc, "role=\"alert\"", "ai-profiles-retry"],
      [diagnosticsSrc, "role=\"alert\"", "ai-diagnostics-retry"],
    ]) {
      expect(source).toContain(errorId);
      expect(source).toContain(`data-testid="${retryId}"`);
    }
    expect(profilesSrc).toContain(':aria-label="t(\'ai.profile.delete\')"');
    expect(keysSrc).toContain(':aria-label="t(\'settings.ai.keys.delete\')"');
  });

  test("settings search indexes the advanced AI concepts", () => {
    for (const keyword of ["配置档案", "profile", "预算", "budget", "fallback", "多 key", "仓库 ai"]) {
      expect(settingsIndexSrc).toContain(`"${keyword}"`);
    }
  });

  test("runtime setup wizard replaces the detour to the full settings page", () => {
    expect(setupModalSrc).toContain("AiConnectionForm");
    expect(setupModalSrc).toContain("useAiSetup");
    expect(composerSrc).toContain("openAiSetup");
    expect(inspectorSrc).toContain("openAiSetup");
    expect(workspaceSrc).toContain("ensureAIConfigured");
    expect(historySrc).toContain("openReleaseNotes");
    expect(workspaceSrc).toContain("AiRepoOverrideModal");
    expect(workspaceSrc).toContain('data-testid="repo-ai-btn"');
  });

  test("repository overrides live in the repository menu, not global settings", () => {
    expect(repoModalSrc).toContain('data-testid="ai-repo-modal"');
    expect(repoModalSrc).toContain("getAIRepoOverride");
    expect(repoModalSrc).toContain("saveAIRepoOverride");
    expect(repoModalSrc).toContain("deleteAIRepoOverride");
    expect(behaviorSrc).not.toContain("saveRepoOverride");
    expect(settingsSrc).not.toContain("ai-repo-overrides-card");
  });

  test("model prices come from the catalog instead of manual per-million inputs", () => {
    expect(aiServiceSrc).toContain("findModelPrice");
    expect(quickSrc).toContain('t("ai.quick.price"');
    expect(usageSrc).toContain('t("ai.usage.price.note")');
    expect(usageSrc).not.toContain("inputUsdPerMTok");
    expect(en["ai.quick.price"]).toContain("1M tokens");
  });
});

describe("read-only AI assistants", () => {
  test("stream service carries chat/review/release plus reasoning deltas", () => {
    for (const kind of ["chat", "review", "release", "explain", "conflict"]) {
      expect(streamSrc).toContain(`"${kind}"`);
    }
    expect(streamSrc).toContain("onReasoning");
    expect(streamSrc).toContain("data.reasoning");
    expect(streamSrc).toContain("history: input.history ?? []");
    expect(streamSrc).toContain("maxCommits: input.maxCommits ?? 0");
  });

  test("chat modal keeps a bounded conversation and cancels streams", () => {
    expect(chatSrc).toContain('kind: "chat"');
    expect(chatSrc).toContain("onReasoning");
    expect(chatSrc).toContain("stopStream");
    expect(chatSrc).toContain('data-testid="ai-chat-input"');
    expect(chatSrc).toContain('data-testid="ai-chat-send"');
    expect(chatSrc).toContain("whitespace-pre-wrap");
    expect(chatSrc).toContain("messages.value.push(userMessage, assistant)");
    expect(diffSrc).toContain("open-ai-chat");
    expect(workspaceSrc).toContain("<AiChatModal");
    expect(workspaceSrc).toContain("openAiRepoChat");
    expect(workspaceSrc).toContain('selectedChangeFile.value?.path ?? selectedSourceFile.value ?? ""');
    expect(chatSrc).toContain("includeFile");
    expect(chatSrc).toContain("previewOutbound");
    expect(chatSrc).toContain('data-testid="ai-chat-include-file"');
  });

  test("review modal is reachable from diff and commit inspector", () => {
    expect(reviewSrc).toContain('kind: "review"');
    expect(reviewSrc).toContain('data-testid="ai-review-modal"');
    expect(diffSrc).toContain("open-ai-review");
    expect(inspectorSrc).toContain('data-testid="ai-review-commit-btn"');
    expect(inspectorSrc).toContain("<AiReviewModal");
    expect(workspaceSrc).toContain("<AiReviewModal");
  });

  test("release notes use a real ref range and show in history", () => {
    expect(releaseSrc).toContain('kind: "release"');
    expect(releaseSrc).toContain("fromRef");
    expect(releaseSrc).toContain("maxCommits");
    expect(historySrc).toContain('data-testid="ai-release-notes-btn"');
    expect(historySrc).toContain("<AiReleaseNotesModal");
  });

  test("whole-merge conflict batch applies per file with stable keys", () => {
    expect(batchSrc).toContain("export interface BatchTarget");
    expect(batchSrc).toContain("targetList");
    expect(batchSrc).toContain("filePath: target.filePath");
    expect(batchSrc).toContain('emit("refresh", item.filePath)');
    expect(batchSrc).toContain("item.filePath + ':' + item.blockIndex");
    expect(workspaceSrc).toContain("openAiConflictBatchAll");
    expect(workspaceSrc).toContain(":targets=\"aiConflictBatchTargets\"");
    expect(diffSrc).toContain("open-ai-batch-all");
    expect(zh["workspace.conflict.batch.all"]).toBeTruthy();
    expect(en["workspace.conflict.batch.all"]).toBeTruthy();
  });
});

describe("AI i18n parity", () => {
  test("every new control-plane key exists in both locales", () => {
    const keys = Object.keys(zh).filter(
      (key) =>
        key.startsWith("settings.ai.") ||
        key.startsWith("ai.usage.") ||
        key.startsWith("ai.chat.") ||
        key.startsWith("ai.review.") ||
        key.startsWith("ai.release.") ||
        key.startsWith("ai.quick.") ||
        key.startsWith("ai.setup.") ||
        key.startsWith("ai.preset.") ||
        key.startsWith("ai.profile.") ||
        key.startsWith("ai.repo.") ||
        key.startsWith("studio.aiusage.") ||
        key.startsWith("workspace.conflict.batch.")
    );
    expect(keys.length).toBeGreaterThan(60);
    for (const key of keys) {
      expect(en[key], `missing en translation for ${key}`).toBeTruthy();
    }
  });
});
