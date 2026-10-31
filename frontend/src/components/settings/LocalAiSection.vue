<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from "vue";
import {
  AlertTriangle,
  CheckCircle2,
  ChevronDown,
  Cpu,
  KeyRound,
  RefreshCw,
  Save,
  Server,
  ShieldCheck,
  Sparkles,
  Terminal,
  Trash2,
  Zap,
} from "lucide-vue-next";
import { t } from "../../i18n";
import {
  clearAIKey,
  detectAICLIs,
  apiOrigin,
  getAIConfig,
  isSameApiOrigin,
  listAIModels,
  saveAISettings,
  testAIConnectionResult,
  testAIDeepResult,
  type AICLIOption,
  type AIMode,
  type AiConfigNoSecret,
  type AITestResult,
} from "../../services/aiService";
import {
  clearLegacyLocalAiKey,
  readLegacyLocalAiConfig,
  type LegacyLocalAiConfig,
} from "../../services/localAiService";
import { featureFlagsApi } from "../../services/gitService";
import type { FeatureFlags } from "../../types";
import { featureFlags as appFeatureFlags } from "../../composables/useFeatureFlags";
import { pushToast } from "../../composables/useToast";
import { confirmAction } from "../../composables/useConfirm";
import AiControlPlaneSection from "./AiControlPlaneSection.vue";
import AiQuickSetup from "./AiQuickSetup.vue";
import AiUsageCard from "./AiUsageCard.vue";

// ---------------------------------------------------------------------------
// One AI configuration, two transports. The backend owns the secret; this
// component only edits the non-secret fields and optionally replaces the key.
// ---------------------------------------------------------------------------

const backend = ref<AiConfigNoSecret | null>(null);
const cliOptions = ref<AICLIOption[]>([]);
const selectedPreset = ref("custom");
const isSaving = ref(false);
const isTesting = ref(false);
const isDeepTesting = ref(false);
const clearingKey = ref(false);
const legacyConfig = ref<LegacyLocalAiConfig | null>(null);
const legacyBusy = ref(false);
const apiModels = ref<string[]>([]);
const modelsEndpoint = ref("");
const modelsLoading = ref(false);
const modelsMessage = ref("");
const customModelMode = ref(false);
const testResult = ref<(AITestResult & { detail?: string }) | null>(null);
const quickRefreshKey = ref(0);
const usageRefreshKey = ref(0);

const form = reactive({
  mode: "api" as AIMode,
  baseUrl: "",
  apiModel: "",
  cliModel: "",
  timeoutSeconds: 60,
  apiKey: "",
  cliCommand: "",
  cliArgsText: "",
  cliInput: "stdin" as "stdin" | "arg",
  commitLanguage: "auto" as "auto" | "zh" | "en",
  commitIncludeBody: false,
  commitInstructions: "",
  protocol: "openai" as "openai" | "azure" | "anthropic" | "gemini",
  azureApiVersion: "2024-10-21",
  extraHeadersText: "{}",
  cliEnvMode: "inherit" as "inherit" | "minimal",
});

const errorMessage = (err: unknown): string => (err instanceof Error ? err.message : String(err));
const displayedTimeout = (seconds: number): number => (seconds > 0 ? seconds : 60);

const mode = computed<AIMode>(() => form.mode);
const configured = computed(() => {
  if (!backend.value) return false;
  return backend.value.mode === "cli"
    ? backend.value.cliCommand.trim() !== ""
    : backend.value.baseUrl.trim() !== "";
});
const detectedOptions = computed(() => cliOptions.value.filter((option) => option.available));
const originWillChange = computed(() => {
  const saved = backend.value;
  if (!saved?.hasApiKey) return false;
  const next = form.baseUrl.trim();
  if (!next) return false;
  return !isSameApiOrigin(saved.baseUrl, next);
});
const hasModelOptions = computed(() => apiModels.value.length > 0);

function syncCustomModelMode(): void {
  const current = form.apiModel.trim();
  customModelMode.value =
    !hasModelOptions.value || (current !== "" && !apiModels.value.includes(current));
}

watch(
  () => form.baseUrl,
  (next) => {
    if (!modelsEndpoint.value || apiOrigin(next) !== modelsEndpoint.value) {
      apiModels.value = [];
      modelsMessage.value = "";
      customModelMode.value = false;
    }
  }
);

watch(
  () => [form.protocol, form.azureApiVersion] as const,
  () => {
    apiModels.value = [];
    modelsMessage.value = "";
    customModelMode.value = false;
  }
);

async function refreshBackend(): Promise<void> {
  try {
    backend.value = await getAIConfig();
  } catch {
    backend.value = null;
  }
}

function fillFormFromBackend(saved: AiConfigNoSecret | null): void {
  if (!saved) return;
  form.mode = saved.mode === "cli" ? "cli" : "api";
  form.baseUrl = saved.baseUrl;
  if (saved.mode === "cli") form.cliModel = saved.model;
  else form.apiModel = saved.model;
  form.timeoutSeconds = displayedTimeout(saved.timeoutSeconds);
  form.cliCommand = saved.cliCommand;
  form.cliArgsText = (saved.cliArgs || []).join(" ");
  form.cliInput = saved.cliInput === "arg" ? "arg" : "stdin";
  form.commitLanguage =
    saved.commitLanguage === "zh" || saved.commitLanguage === "en" ? saved.commitLanguage : "auto";
  form.commitIncludeBody = saved.commitIncludeBody === true;
  form.commitInstructions = saved.commitInstructions || "";
  form.protocol =
    saved.protocol === "azure" || saved.protocol === "anthropic" || saved.protocol === "gemini"
      ? saved.protocol
      : "openai";
  form.azureApiVersion = saved.azureApiVersion || "2024-10-21";
  form.extraHeadersText = JSON.stringify(saved.extraHeaders ?? {});
  form.cliEnvMode = saved.cliEnvMode === "minimal" ? "minimal" : "inherit";
}

async function detectCLIs(): Promise<void> {
  try {
    cliOptions.value = (await detectAICLIs()) || [];
  } catch {
    cliOptions.value = [];
  }
  // Keep the picker pointing at the command the config already uses.
  const matches = cliOptions.value.find(
    (option) => option.available && option.command === form.cliCommand
  );
  if (matches) selectedPreset.value = matches.id;
}

function applyPreset(id: string): void {
  selectedPreset.value = id;
  if (id === "custom") return;
  const option = cliOptions.value.find((item) => item.id === id);
  if (!option) return;
  form.mode = "cli";
  form.cliCommand = option.command;
  form.cliArgsText = (option.args || []).join(" ");
  form.cliInput = option.input === "arg" ? "arg" : "stdin";
}

onMounted(async () => {
  await refreshBackend();
  const saved = backend.value;
  fillFormFromBackend(saved);
  legacyConfig.value = readLegacyLocalAiConfig();
  await detectCLIs();
  if (saved?.mode === "cli") {
    const match = cliOptions.value.find((option) => option.available && option.command === saved.cliCommand);
    if (match) selectedPreset.value = match.id;
  }

  try {
    const flags = await featureFlagsApi.getFeatureFlags();
    if (flags) Object.assign(flagState, flags);
    flagsLoaded.value = true;
  } catch {
    flagsLoaded.value = false;
  }
});

function savePayload() {
  return {
    mode: form.mode,
    baseUrl: form.baseUrl.trim(),
    model: (form.mode === "cli" ? form.cliModel : form.apiModel).trim(),
    timeoutSeconds: Number(form.timeoutSeconds) || 0,
    cliCommand: form.cliCommand.trim(),
    cliArgs: form.cliArgsText.split(/\s+/).map((item) => item.trim()).filter(Boolean),
    cliInput: form.cliInput,
    commitLanguage: form.commitLanguage,
    commitIncludeBody: form.commitIncludeBody,
    commitInstructions: form.commitInstructions.trim(),
    protocol: form.protocol,
    azureApiVersion: form.azureApiVersion.trim(),
    cliEnvMode: form.cliEnvMode,
    apiKey: form.apiKey.trim(),
  };
}

function validate(): string {
  if (form.mode === "api") {
    const base = form.baseUrl.trim();
    if (base === "") return t("settings.localai.endpoint.required");
    let parsed: URL;
    try {
      parsed = new URL(base);
    } catch {
      return t("settings.localai.endpoint.invalid");
    }
    if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
      return t("settings.localai.endpoint.invalid");
    }
    if (parsed.username || parsed.password) {
      return t("settings.localai.endpoint.credentials");
    }
    if (form.apiModel.trim() === "") return t("settings.localai.model.required");
  }
  if (form.mode === "cli" && form.cliCommand.trim() === "") {
    return t("settings.localai.cli.required");
  }
  return "";
}

async function saveBackendConfig(silent = false): Promise<boolean> {
  const invalid = validate();
  if (invalid) {
    pushToast(invalid, { tone: "warning" });
    return false;
  }
  let extraHeaders: Record<string, string> = {};
  const rawHeaders = form.extraHeadersText.trim();
  if (rawHeaders) {
    try {
      const parsed = JSON.parse(rawHeaders);
      if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) throw new Error("object");
      const entries = Object.entries(parsed);
      if (entries.some(([key, value]) => !key.trim() || typeof value !== "string")) {
        throw new Error("values");
      }
      extraHeaders = Object.fromEntries(entries.map(([key, value]) => [key.trim(), String(value)]));
    } catch {
      pushToast(t("settings.localai.headers.invalid"), { tone: "warning" });
      return false;
    }
  }
  isSaving.value = true;
  try {
    await saveAISettings({ ...savePayload(), extraHeaders });
    form.apiKey = ""; // never echo the secret back into the input
    await refreshBackend();
    fillFormFromBackend(backend.value);
    quickRefreshKey.value++;
    usageRefreshKey.value++;
    if (!silent) pushToast(t("settings.localai.save.succeeded"), { tone: "success" });
    return true;
  } catch (err) {
    pushToast(t("settings.localai.save.failed", { message: errorMessage(err) }), {
      tone: "danger",
    });
    return false;
  } finally {
    isSaving.value = false;
  }
}

async function fetchModels(): Promise<void> {
  if (modelsLoading.value) return;
  const base = form.baseUrl.trim();
  if (!base) {
    pushToast(t("settings.localai.endpoint.required"), { tone: "warning" });
    return;
  }
  modelsLoading.value = true;
  modelsMessage.value = "";
  try {
    const res = await listAIModels(base, form.apiKey.trim(), form.protocol, form.azureApiVersion.trim());
    modelsMessage.value = res?.message ?? "";
    if (!res?.ok) {
      apiModels.value = [];
      syncCustomModelMode();
      pushToast(res?.message || t("settings.localai.model.fetchfailed"), { tone: "warning" });
      return;
    }
    apiModels.value = Array.from(
      new Set((res.models ?? []).map((model) => model.trim()).filter(Boolean))
    ).sort((a, b) => a.localeCompare(b));
    modelsEndpoint.value = apiOrigin(base);
    syncCustomModelMode();
  } catch (err) {
    apiModels.value = [];
    syncCustomModelMode();
    pushToast(t("settings.localai.model.fetchfailed"), { tone: "danger" });
  } finally {
    modelsLoading.value = false;
  }
}

function handleModelSelect(event: Event): void {
  const value = (event.target as HTMLSelectElement).value;
  if (value === "__custom__") {
    customModelMode.value = true;
    form.apiModel = "";
  } else {
    customModelMode.value = false;
    form.apiModel = value;
  }
}

async function runTest(): Promise<void> {
  if (isTesting.value) return;
  isTesting.value = true;
  testResult.value = null;
  try {
    // The probe runs against the persisted config, so save the form first.
    if (!(await saveBackendConfig(true))) return;
    const result = await testAIConnectionResult();
    testResult.value = result;
    if (result.ok && result.models?.length) {
      apiModels.value = Array.from(
        new Set(result.models.map((model) => model.trim()).filter(Boolean))
      ).sort((a, b) => a.localeCompare(b));
      modelsEndpoint.value = apiOrigin(form.baseUrl);
      modelsMessage.value = result.message;
      syncCustomModelMode();
    }
  } catch (err) {
    testResult.value = { ok: false, mode: form.mode, message: errorMessage(err) };
  } finally {
    isTesting.value = false;
  }
}

async function runDeepTest(): Promise<void> {
  if (isDeepTesting.value) return;
  isDeepTesting.value = true;
  testResult.value = null;
  try {
    if (!(await saveBackendConfig(true))) return;
    testResult.value = await testAIDeepResult();
  } catch (err) {
    testResult.value = { ok: false, mode: form.mode, message: errorMessage(err) };
  } finally {
    isDeepTesting.value = false;
  }
}

async function removeKey(): Promise<void> {
  if (clearingKey.value) return;
  if (!(await confirmAction({ message: t("settings.localai.key.clearconfirm"), tone: "warning" }))) return;
  clearingKey.value = true;
  try {
    await clearAIKey();
    await refreshBackend();
    pushToast(t("settings.localai.key.cleared"), { tone: "success" });
  } catch (err) {
    pushToast(t("settings.localai.save.failed", { message: errorMessage(err) }), { tone: "danger" });
  } finally {
    clearingKey.value = false;
  }
}

async function importLegacyKey(): Promise<void> {
  const legacy = legacyConfig.value;
  if (!legacy || legacyBusy.value) return;
  if (form.mode === "cli") {
    pushToast(t("settings.localai.legacy.climode"), { tone: "warning" });
    return;
  }
  const targetBase = form.baseUrl.trim() || backend.value?.baseUrl?.trim() || legacy.endpoint.trim();
  const targetModel = form.apiModel.trim() || backend.value?.model?.trim() || legacy.model.trim();
  if (!targetBase || !targetModel) {
    pushToast(t("settings.localai.legacy.incomplete"), { tone: "warning" });
    return;
  }
  if (legacy.endpoint.trim() && !isSameApiOrigin(legacy.endpoint, targetBase)) {
    const ok = await confirmAction({
      message: t("settings.localai.legacy.originwarning"),
      tone: "warning",
    });
    if (!ok) return;
  }
  legacyBusy.value = true;
  try {
    await saveAISettings({
      mode: "api",
      baseUrl: targetBase,
      model: targetModel,
      timeoutSeconds: backend.value?.timeoutSeconds || 60,
      cliCommand: "",
      cliArgs: [],
      cliInput: "stdin",
      apiKey: legacy.apiKey,
    });
    clearLegacyLocalAiKey();
    legacyConfig.value = null;
    await refreshBackend();
    fillFormFromBackend(backend.value);
    syncCustomModelMode();
    pushToast(t("settings.localai.legacy.imported"), { tone: "success" });
  } catch (err) {
    pushToast(t("settings.localai.save.failed", { message: errorMessage(err) }), { tone: "danger" });
  } finally {
    legacyBusy.value = false;
  }
}

async function discardLegacyKey(): Promise<void> {
  if (!legacyConfig.value || legacyBusy.value) return;
  if (!(await confirmAction({ message: t("settings.localai.legacy.discardconfirm"), tone: "warning" }))) {
    return;
  }
  clearLegacyLocalAiKey();
  legacyConfig.value = null;
  pushToast(t("settings.localai.legacy.discarded"), { tone: "success" });
}

function switchMode(next: AIMode): void {
  form.mode = next;
  testResult.value = null;
}

async function handleQuickSaved(): Promise<void> {
  await refreshBackend();
  fillFormFromBackend(backend.value);
  syncCustomModelMode();
  quickRefreshKey.value++;
  usageRefreshKey.value++;
}

async function handleAdvancedChanged(): Promise<void> {
  await refreshBackend();
  fillFormFromBackend(backend.value);
  syncCustomModelMode();
  quickRefreshKey.value++;
  usageRefreshKey.value++;
}

// ---------------------------------------------------------------------------
// Feature flags remain a separate, user-visible rollout surface.
// ---------------------------------------------------------------------------

const FLAG_DEFS: Array<{ key: keyof FeatureFlags; labelKey: string; descKey: string }> = [
  { key: "aiCommitMsg", labelKey: "settings.flags.aicommitmsg.label", descKey: "settings.flags.aicommitmsg.desc" },
  { key: "aiSummary", labelKey: "settings.flags.aisummary.label", descKey: "settings.flags.aisummary.desc" },
  { key: "aiConflictBatch", labelKey: "settings.flags.aiconflictbatch.label", descKey: "settings.flags.aiconflictbatch.desc" },
  { key: "reviewFlow", labelKey: "settings.flags.reviewflow.label", descKey: "settings.flags.reviewflow.desc" },
  { key: "assetBoard", labelKey: "settings.flags.assetboard.label", descKey: "settings.flags.assetboard.desc" },
];

const flagState = reactive<FeatureFlags>({
  aiCommitMsg: true,
  aiSummary: true,
  aiConflictBatch: true,
  reviewFlow: false,
  assetBoard: false,
});
const flagsLoaded = ref(false);
const savingFlagKey = ref("");

async function toggleFlag(key: keyof FeatureFlags) {
  if (savingFlagKey.value !== "") return;
  savingFlagKey.value = key;
  const snapshot = { ...flagState };
  flagState[key] = !flagState[key];
  try {
    await featureFlagsApi.saveAppFlags({ ...flagState });
    Object.assign(appFeatureFlags, flagState);
  } catch (err) {
    Object.assign(flagState, snapshot);
    pushToast(t("settings.flags.save.failed", { message: errorMessage(err) }), { tone: "danger" });
  } finally {
    savingFlagKey.value = "";
  }
}
</script>

<template>
  <section class="space-y-6">
    <div>
      <h2 class="text-sm font-bold text-ink">{{ t("settings.localai.title") }}</h2>
      <p class="text-xs text-ink-secondary mt-1">{{ t("settings.localai.intro") }}</p>
    </div>

    <AiQuickSetup :refresh-key="quickRefreshKey" @saved="handleQuickSaved" />

    <details class="group rounded-2xl border border-line bg-surface/20" data-testid="ai-advanced-settings">
      <summary class="flex cursor-pointer list-none items-center justify-between gap-3 p-4 select-none rounded-2xl focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/50">
        <div class="min-w-0">
          <h3 class="text-xs font-bold text-ink">{{ t("settings.localai.advanced.title") }}</h3>
          <p class="text-[10px] text-ink-muted mt-0.5 leading-relaxed">{{ t("settings.localai.advanced.desc") }}</p>
        </div>
        <ChevronDown class="w-4 h-4 text-ink-muted shrink-0 transition-transform group-open:rotate-180" aria-hidden="true" />
      </summary>
      <div class="px-4 pb-4 space-y-5">

    <!-- Status -->
    <div class="p-4 rounded-2xl bg-surface border border-line flex items-start gap-3" data-testid="localai-backend-status">
      <div class="w-9 h-9 rounded-xl bg-accent/10 border border-accent/20 flex items-center justify-center text-accent shrink-0">
        <KeyRound class="w-4 h-4" />
      </div>
      <div class="min-w-0 flex-1">
        <div class="flex items-center gap-2 flex-wrap">
          <h3 class="text-xs font-bold text-ink">{{ t("settings.localai.backend.title") }}</h3>
          <span class="px-1.5 py-0.5 rounded text-[9px] font-mono font-bold border border-line bg-canvas text-ink-secondary">
            {{ backend?.mode === "cli" ? t("settings.localai.mode.cli") : t("settings.localai.mode.api") }}
          </span>
          <span
            class="px-1.5 py-0.5 rounded text-[9px] font-mono font-bold"
            :class="configured ? 'bg-accent/20 text-accent border border-accent/30' : 'bg-canvas text-ink-muted border border-line'"
          >
            {{ configured ? t("settings.localai.status.configured") : t("settings.localai.status.unconfigured") }}
          </span>
          <span
            v-if="backend?.mode !== 'cli' && backend?.keyAvailable"
            class="px-1.5 py-0.5 rounded text-[9px] font-mono bg-success/10 text-success border border-success/30"
          >
            {{ t("settings.localai.status.keysaved") }}
          </span>
        </div>
        <p class="text-[11px] text-ink-muted mt-0.5 leading-relaxed">
          {{ backend?.mode === "cli" ? t("settings.localai.mode.clihint") : t("settings.localai.mode.apihint") }}
        </p>
        <p v-if="backend?.cliCommand" class="text-[10px] font-mono text-ink-muted mt-1 truncate">
          {{ backend.cliCommand }} {{ (backend.cliArgs || []).join(" ") }}
        </p>
      </div>
    </div>

    <div
      v-if="backend?.notice"
      class="p-3 rounded-xl border bg-warning/10 border-warning/30 text-warning text-xs flex items-start gap-2"
    >
      <AlertTriangle class="w-4 h-4 shrink-0 mt-0.5" />
      <span class="text-[11px] font-medium leading-relaxed break-all">{{ backend.notice }}</span>
    </div>

    <div
      v-if="legacyConfig"
      data-testid="localai-legacy-banner"
      class="p-3 rounded-xl border bg-warning/10 border-warning/30 text-warning text-xs flex items-start gap-2"
    >
      <KeyRound class="w-4 h-4 shrink-0 mt-0.5" />
      <div class="min-w-0 flex-1 space-y-2">
        <div class="space-y-1">
          <p class="text-[11px] font-semibold">{{ t("settings.localai.legacy.title") }}</p>
          <p class="text-[10px] leading-relaxed opacity-90">{{ t("settings.localai.legacy.desc") }}</p>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <button
            type="button"
            data-testid="localai-legacy-import"
            :disabled="legacyBusy || form.mode === 'cli'"
            class="px-2.5 py-1.5 rounded-lg border border-warning/40 bg-surface text-warning text-[11px] font-semibold hover:bg-raised transition cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
            @click="importLegacyKey"
          >
            {{ legacyBusy ? t("common.loading") : t("settings.localai.legacy.import") }}
          </button>
          <button
            type="button"
            data-testid="localai-legacy-discard"
            :disabled="legacyBusy"
            class="px-2.5 py-1.5 rounded-lg border border-line bg-surface text-ink-muted text-[11px] font-semibold hover:text-danger hover:bg-raised transition cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
            @click="discardLegacyKey"
          >
            {{ t("settings.localai.legacy.discard") }}
          </button>
        </div>
        <p v-if="form.mode === 'cli'" class="text-[10px] leading-relaxed opacity-80">
          {{ t("settings.localai.legacy.climode") }}
        </p>
      </div>
    </div>

    <!-- Mode switch -->
    <div class="inline-flex p-1 rounded-xl border border-line bg-canvas" data-testid="ai-mode-switch">
      <button
        type="button"
        data-testid="ai-mode-api"
        class="px-3 py-1.5 rounded-lg text-xs font-semibold transition cursor-pointer"
        :class="mode === 'api' ? 'bg-accent-strong text-accent-contrast shadow-xs' : 'text-ink-muted hover:text-ink'"
        @click="switchMode('api')"
      >
        <Server class="w-3.5 h-3.5 inline mr-1" aria-hidden="true" />{{ t("settings.localai.mode.api") }}
      </button>
      <button
        type="button"
        data-testid="ai-mode-cli"
        class="px-3 py-1.5 rounded-lg text-xs font-semibold transition cursor-pointer"
        :class="mode === 'cli' ? 'bg-accent-strong text-accent-contrast shadow-xs' : 'text-ink-muted hover:text-ink'"
        @click="switchMode('cli')"
      >
        <Terminal class="w-3.5 h-3.5 inline mr-1" aria-hidden="true" />{{ t("settings.localai.mode.cli") }}
      </button>
    </div>

    <div class="p-5 rounded-2xl bg-surface border border-line space-y-4">
      <!-- API fields -->
      <template v-if="mode === 'api'">
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div class="space-y-1">
            <label class="text-xs font-semibold text-ink" for="localai-protocol">{{ t("settings.localai.protocol.label") }}</label>
            <select
              id="localai-protocol"
              v-model="form.protocol"
              data-testid="localai-protocol"
              class="w-full px-3 py-2 rounded-xl bg-canvas border border-line text-xs text-ink focus:outline-none focus:border-accent"
            >
              <option value="openai">OpenAI-compatible</option>
              <option value="azure">Azure OpenAI</option>
              <option value="anthropic">Anthropic</option>
              <option value="gemini">Google Gemini</option>
            </select>
          </div>
          <div v-if="form.protocol === 'azure'" class="space-y-1">
            <label class="text-xs font-semibold text-ink" for="localai-azure-version">{{ t("settings.localai.protocol.azureversion") }}</label>
            <input
              id="localai-azure-version"
              v-model="form.azureApiVersion"
              type="text"
              data-testid="localai-azure-version"
              placeholder="2024-10-21"
              class="w-full px-3 py-2 rounded-xl bg-canvas border border-line text-xs font-mono text-ink focus:outline-none focus:border-accent"
            />
          </div>
        </div>

        <div class="space-y-1">
          <label class="text-xs font-semibold text-ink flex items-center gap-1.5">
            <Server class="w-3.5 h-3.5 text-accent" aria-hidden="true" />
            <span>{{ t("settings.localai.endpoint.label") }}</span>
          </label>
          <input
            v-model="form.baseUrl"
            type="text"
            data-testid="localai-endpoint-input"
            placeholder="http://localhost:11434/v1"
            class="w-full px-3 py-2 rounded-xl bg-canvas border border-line text-xs font-mono text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
          />
          <p class="text-[10px] text-ink-muted">{{ t(`settings.localai.protocol.${form.protocol}.hint`) }}</p>
          <p v-if="originWillChange" class="text-[10px] text-ink-muted leading-relaxed">
            {{ t("settings.localai.key.originchanged") }}
          </p>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div class="space-y-1 min-w-0">
            <label class="text-xs font-semibold text-ink flex items-center gap-1.5">
              <Cpu class="w-3.5 h-3.5 text-accent" aria-hidden="true" />
              <span>{{ t("settings.localai.model.label") }}</span>
            </label>
            <div class="flex items-center gap-1.5">
              <select
                v-if="hasModelOptions"
                :value="customModelMode ? '__custom__' : form.apiModel"
                :aria-label="t('settings.localai.model.label')"
                data-testid="localai-model-select"
                class="flex-1 min-w-0 px-3 py-2 rounded-xl bg-canvas border border-line text-xs font-mono text-ink focus:outline-none focus:border-accent"
                @change="handleModelSelect"
              >
                <option value="" disabled>{{ t("settings.localai.model.pick") }}</option>
                <option v-for="model in apiModels" :key="model" :value="model">{{ model }}</option>
                <option value="__custom__">{{ t("settings.localai.model.custom") }}</option>
              </select>
              <input
                v-else
                v-model="form.apiModel"
                type="text"
                :aria-label="t('settings.localai.model.label')"
                data-testid="localai-model-input"
                placeholder="llama3"
                class="flex-1 min-w-0 px-3 py-2 rounded-xl bg-canvas border border-line text-xs font-mono text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
              />
              <button
                type="button"
                data-testid="localai-model-refresh"
                :disabled="modelsLoading || !form.baseUrl.trim()"
                class="shrink-0 p-2 rounded-xl border border-line bg-surface text-ink-muted hover:text-accent hover:bg-raised transition cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
                :title="t('settings.localai.model.fetch')"
                :aria-label="t('settings.localai.model.fetch')"
                @click="fetchModels"
              >
                <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': modelsLoading }" aria-hidden="true" />
              </button>
            </div>
            <input
              v-if="hasModelOptions && customModelMode"
              v-model="form.apiModel"
              type="text"
              :aria-label="t('settings.localai.model.custom')"
              data-testid="localai-model-custom"
              placeholder="llama3"
              class="w-full px-3 py-2 rounded-xl bg-canvas border border-line text-xs font-mono text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
            />
            <p class="text-[10px] text-ink-muted leading-relaxed">
              {{
                modelsLoading
                  ? t("settings.localai.model.fetching")
                  : modelsMessage ||
                    (hasModelOptions
                      ? t("settings.localai.model.found", { count: apiModels.length })
                      : t("settings.localai.model.hint"))
              }}
            </p>
          </div>
          <div class="space-y-1">
            <label class="text-xs font-semibold text-ink flex items-center gap-1.5">
              <Zap class="w-3.5 h-3.5 text-accent" aria-hidden="true" />
              <span>{{ t("settings.localai.timeout.label") }}</span>
            </label>
            <input
              v-model.number="form.timeoutSeconds"
              type="number"
              min="0"
              max="600"
              data-testid="localai-timeout-input"
              class="w-full px-3 py-2 rounded-xl bg-canvas border border-line text-xs font-mono text-ink focus:outline-none focus:border-accent"
            />
          </div>
        </div>

        <div class="space-y-1">
          <label class="text-xs font-semibold text-ink flex items-center gap-1.5">
            <KeyRound class="w-3.5 h-3.5 text-accent" aria-hidden="true" />
            <span>{{ t("settings.localai.apikey.label") }}</span>
          </label>
          <div class="flex items-center gap-2">
            <input
              v-model="form.apiKey"
              type="password"
              data-testid="localai-apikey-input"
              :placeholder="backend?.keyAvailable ? t('settings.localai.key.kept') : 'sk-...'"
              class="flex-1 px-3 py-2 rounded-xl bg-canvas border border-line text-xs font-mono text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
            />
            <button
              v-if="backend?.keyAvailable"
              type="button"
              data-testid="localai-key-clear"
              :disabled="clearingKey"
              class="px-2.5 py-2 rounded-xl border border-line bg-surface hover:bg-raised text-ink-muted hover:text-danger text-[11px] font-semibold transition cursor-pointer disabled:opacity-40 flex items-center gap-1"
              :title="t('settings.localai.key.clear')"
              @click="removeKey"
            >
              <Trash2 class="w-3.5 h-3.5" aria-hidden="true" />
              <span>{{ clearingKey ? t("common.loading") : t("settings.localai.key.clear") }}</span>
            </button>
          </div>
          <p class="text-[10px] text-ink-muted">{{ t("settings.localai.key.stored") }}</p>
        </div>

        <details class="rounded-xl border border-line bg-canvas/50 px-3 py-2">
          <summary class="text-[11px] font-semibold text-ink cursor-pointer select-none">{{ t("settings.localai.headers.title") }}</summary>
          <div class="pt-2 space-y-1">
            <textarea
              v-model="form.extraHeadersText"
              rows="3"
              spellcheck="false"
              data-testid="localai-extra-headers"
              placeholder='{"X-Title": "wgit"}'
              class="w-full px-2.5 py-2 rounded-lg bg-canvas border border-line text-[11px] font-mono text-ink focus:outline-none focus:border-accent"
            ></textarea>
            <p class="text-[10px] text-ink-muted leading-relaxed">{{ t("settings.localai.headers.hint") }}</p>
          </div>
        </details>
      </template>

      <!-- CLI fields -->
      <template v-else>
        <div class="space-y-1">
          <label class="text-xs font-semibold text-ink flex items-center gap-1.5">
            <Terminal class="w-3.5 h-3.5 text-accent" aria-hidden="true" />
            <span>{{ t("settings.localai.cli.preset") }}</span>
          </label>
          <div class="flex items-center gap-2">
            <select
              :value="selectedPreset"
              data-testid="localai-cli-preset"
              class="flex-1 min-w-0 w-full px-3 py-2 rounded-xl bg-canvas border border-line text-xs font-mono text-ink focus:outline-none focus:border-accent"
              @change="applyPreset(($event.target as HTMLSelectElement).value)"
            >
              <option value="custom">{{ t("settings.localai.cli.custom") }}</option>
              <option v-for="option in cliOptions" :key="option.id" :value="option.id" :disabled="!option.available">
                {{ option.name }}{{ option.available ? (option.version ? ` · ${option.version}` : " · " + t("settings.localai.cli.detected")) : " · " + t("settings.localai.cli.notfound") }}
              </option>
            </select>
            <button
              type="button"
              data-testid="localai-cli-refresh"
              class="shrink-0 px-2.5 py-2 rounded-xl border border-line bg-surface hover:bg-raised text-ink-muted hover:text-ink text-[11px] font-semibold transition cursor-pointer flex items-center gap-1"
              :title="t('settings.localai.cli.refresh')"
              :aria-label="t('settings.localai.cli.refresh')"
              @click="detectCLIs"
            >
              <RefreshCw class="w-3.5 h-3.5" aria-hidden="true" />
              <span class="hidden sm:inline">{{ t("settings.localai.cli.refresh") }}</span>
            </button>
          </div>
          <p class="text-[10px] text-ink-muted">
            {{ detectedOptions.length > 0 ? t("settings.localai.cli.detectedhint", { count: detectedOptions.length }) : t("settings.localai.cli.nonehint") }}
          </p>
        </div>

        <div class="space-y-1">
          <label class="text-xs font-semibold text-ink">{{ t("settings.localai.cli.command") }}</label>
          <input
            v-model="form.cliCommand"
            type="text"
            data-testid="localai-cli-command"
            placeholder="claude"
            class="w-full px-3 py-2 rounded-xl bg-canvas border border-line text-xs font-mono text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
          />
        </div>

        <div class="space-y-1">
          <label class="text-xs font-semibold text-ink">{{ t("settings.localai.cli.args") }}</label>
          <input
            v-model="form.cliArgsText"
            type="text"
            data-testid="localai-cli-args"
            placeholder="-p --output-format text"
            class="w-full px-3 py-2 rounded-xl bg-canvas border border-line text-xs font-mono text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
          />
          <p class="text-[10px] text-ink-muted">{{ t("settings.localai.cli.argshint") }}</p>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div class="space-y-1">
            <label class="text-xs font-semibold text-ink">{{ t("settings.localai.cli.input") }}</label>
            <div class="inline-flex p-1 rounded-xl border border-line bg-canvas">
              <button
                type="button"
                class="px-2.5 py-1 rounded-lg text-[11px] font-semibold cursor-pointer"
                :class="form.cliInput === 'stdin' ? 'bg-accent-strong text-accent-contrast' : 'text-ink-muted hover:text-ink'"
                @click="form.cliInput = 'stdin'"
              >
                {{ t("settings.localai.cli.stdin") }}
              </button>
              <button
                type="button"
                class="px-2.5 py-1 rounded-lg text-[11px] font-semibold cursor-pointer"
                :class="form.cliInput === 'arg' ? 'bg-accent-strong text-accent-contrast' : 'text-ink-muted hover:text-ink'"
                @click="form.cliInput = 'arg'"
              >
                {{ t("settings.localai.cli.arg") }}
              </button>
            </div>
          </div>
          <div class="space-y-1">
            <label class="text-xs font-semibold text-ink flex items-center gap-1.5">
              <Cpu class="w-3.5 h-3.5 text-accent" aria-hidden="true" />
              <span>{{ t("settings.localai.model.label") }}</span>
            </label>
            <input
              v-model="form.cliModel"
              type="text"
              placeholder="sonnet"
              class="w-full px-3 py-2 rounded-xl bg-canvas border border-line text-xs font-mono text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
            />
          </div>
        </div>

        <div class="grid grid-cols-1 gap-1">
          <label class="text-xs font-semibold text-ink flex items-center gap-1.5">
            <Zap class="w-3.5 h-3.5 text-accent" aria-hidden="true" />
            <span>{{ t("settings.localai.timeout.label") }}</span>
          </label>
          <input
            v-model.number="form.timeoutSeconds"
            type="number"
            min="0"
            max="600"
            class="w-40 px-3 py-2 rounded-xl bg-canvas border border-line text-xs font-mono text-ink focus:outline-none focus:border-accent"
          />
        </div>

        <div class="grid grid-cols-1 gap-1">
          <label class="text-xs font-semibold text-ink" for="localai-cli-env">{{ t("settings.localai.env.label") }}</label>
          <select
            id="localai-cli-env"
            v-model="form.cliEnvMode"
            data-testid="localai-cli-env"
            class="w-56 px-3 py-2 rounded-xl bg-canvas border border-line text-xs text-ink focus:outline-none focus:border-accent"
          >
            <option value="inherit">{{ t("settings.localai.env.inherit") }}</option>
            <option value="minimal">{{ t("settings.localai.env.minimal") }}</option>
          </select>
          <p class="text-[10px] text-ink-muted">{{ t("settings.localai.env.hint") }}</p>
        </div>
      </template>

      <div class="pt-2 flex flex-col gap-2">
        <div class="flex items-center gap-2 flex-wrap">
          <button
            type="button"
            data-testid="localai-save-btn"
            @click="saveBackendConfig(false)"
            :disabled="isSaving"
            class="w-fit flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-accent-strong text-accent-contrast text-xs font-semibold hover:bg-accent-strong-hover transition cursor-pointer disabled:opacity-40"
          >
            <Save class="w-3.5 h-3.5" aria-hidden="true" />
            <span>{{ isSaving ? t("common.loading") : t("settings.localai.save.btn") }}</span>
          </button>
          <button
            type="button"
            data-testid="localai-test-btn"
            @click="runTest"
            :disabled="isTesting || isSaving || isDeepTesting"
            class="w-fit flex items-center gap-1.5 px-3 py-1.5 rounded-xl border border-line bg-surface hover:bg-raised text-xs font-semibold text-ink-secondary hover:text-ink transition cursor-pointer disabled:opacity-40"
          >
            <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isTesting }" aria-hidden="true" />
            <span>{{ isTesting ? t("common.loading") : t("settings.localai.test.btn") }}</span>
          </button>
          <button
            type="button"
            data-testid="localai-deep-test-btn"
            @click="runDeepTest"
            :disabled="isDeepTesting || isSaving || isTesting"
            class="w-fit flex items-center gap-1.5 px-3 py-1.5 rounded-xl border border-warning/40 bg-warning/10 hover:bg-warning/15 text-xs font-semibold text-warning transition cursor-pointer disabled:opacity-40"
          >
            <Zap class="w-3.5 h-3.5" :class="{ 'animate-pulse': isDeepTesting }" aria-hidden="true" />
            <span>{{ isDeepTesting ? t("common.loading") : t("settings.localai.deep.btn") }}</span>
          </button>
          <span class="text-[10px] text-ink-muted">{{ t("settings.localai.test.savesfirst") }}</span>
        </div>
        <p class="text-[10px] text-ink-muted leading-relaxed">{{ t("settings.localai.deep.hint") }}</p>

        <div
          v-if="testResult"
          class="p-3 rounded-xl border text-xs flex items-start gap-2 transition"
          :class="testResult.ok ? 'bg-success/10 border-success/30 text-success' : 'bg-danger/10 border-danger/30 text-danger'"
        >
          <CheckCircle2 v-if="testResult.ok" class="w-4 h-4 shrink-0 text-success mt-0.5" />
          <AlertTriangle v-else class="w-4 h-4 shrink-0 text-danger mt-0.5" />
          <div class="min-w-0">
            <div class="text-[11px] font-medium leading-relaxed">{{ testResult.message }}</div>
            <div v-if="testResult.detail" class="text-[10px] font-mono opacity-80 mt-0.5 break-all">{{ testResult.detail }}</div>
            <div v-if="testResult.models?.length" class="text-[10px] font-mono opacity-80 mt-0.5 break-all">
              {{ t("settings.localai.test.models", { count: testResult.models.length }) }}: {{ testResult.models.slice(0, 8).join(", ") }}
            </div>
          </div>
        </div>
      </div>
    </div>

    <div class="p-5 rounded-2xl bg-surface border border-line space-y-3" data-testid="commit-prefs-card">
      <div>
        <h3 class="text-xs font-bold text-ink">{{ t("settings.localai.commit.title") }}</h3>
        <p class="text-[11px] text-ink-muted mt-0.5 leading-relaxed">{{ t("settings.localai.commit.desc") }}</p>
      </div>
      <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <div class="space-y-1">
          <label class="text-xs font-semibold text-ink" for="localai-commit-language">{{ t("settings.localai.commit.language") }}</label>
          <select
            id="localai-commit-language"
            v-model="form.commitLanguage"
            data-testid="localai-commit-language"
            class="w-full px-3 py-2 rounded-xl bg-canvas border border-line text-xs text-ink focus:outline-none focus:border-accent"
          >
            <option value="auto">{{ t("settings.localai.commit.auto") }}</option>
            <option value="zh">{{ t("settings.localai.commit.zh") }}</option>
            <option value="en">{{ t("settings.localai.commit.en") }}</option>
          </select>
        </div>
        <label class="flex items-start gap-2 p-2 rounded-xl border border-line bg-canvas cursor-pointer">
          <input v-model="form.commitIncludeBody" type="checkbox" class="accent-accent scale-90 mt-0.5" data-testid="localai-commit-body" />
          <span class="text-[11px] leading-relaxed">
            <span class="block font-semibold text-ink">{{ t("settings.localai.commit.body") }}</span>
            <span class="block text-ink-muted">{{ t("settings.localai.commit.bodyhint") }}</span>
          </span>
        </label>
      </div>
      <div class="space-y-1">
        <label class="text-xs font-semibold text-ink" for="localai-commit-instructions">{{ t("settings.localai.commit.instructions") }}</label>
        <textarea
          id="localai-commit-instructions"
          v-model="form.commitInstructions"
          rows="2"
          maxlength="2000"
          data-testid="localai-commit-instructions"
          :placeholder="t('settings.localai.commit.instructionshint')"
          class="w-full px-3 py-2 rounded-xl bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
        ></textarea>
      </div>
    </div>

    <AiControlPlaneSection :refresh-key="quickRefreshKey" @changed="handleAdvancedChanged" />

    <!-- Feature flags (staged rollout) -->
    <div class="p-5 rounded-2xl bg-surface border border-line space-y-3" data-testid="feature-flags-card">
      <div class="flex items-start gap-3">
        <div class="w-9 h-9 rounded-xl bg-accent/10 border border-accent/20 flex items-center justify-center text-accent shrink-0">
          <ShieldCheck class="w-4 h-4 text-accent" />
        </div>
        <div class="min-w-0">
          <h3 class="text-xs font-bold text-ink">{{ t("settings.flags.title") }}</h3>
          <p class="text-[11px] text-ink-muted mt-0.5 leading-relaxed">{{ t("settings.flags.desc") }}</p>
        </div>
      </div>

      <div
        v-for="def in FLAG_DEFS"
        :key="def.key"
        class="flex items-center justify-between gap-4 py-2 border-t border-line/60"
        :data-testid="`flags-row-${def.key}`"
      >
        <div class="min-w-0">
          <div class="text-xs font-semibold text-ink">{{ t(def.labelKey) }}</div>
          <div class="text-[11px] text-ink-muted mt-0.5 leading-relaxed">{{ t(def.descKey) }}</div>
        </div>
        <button
          type="button"
          role="switch"
          :aria-checked="flagState[def.key]"
          :aria-label="t(def.labelKey)"
          :data-testid="`flags-toggle-${def.key}`"
          :disabled="savingFlagKey !== '' || !flagsLoaded"
          @click="toggleFlag(def.key)"
          class="relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-accent/50 disabled:opacity-40 disabled:cursor-wait"
          :class="flagState[def.key] ? 'bg-accent' : 'bg-line'"
        >
          <span
            class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow-lg ring-0 transition duration-200 ease-in-out"
            :class="flagState[def.key] ? 'translate-x-5' : 'translate-x-0'"
          />
        </button>
      </div>
    </div>
      </div>
    </details>

    <AiUsageCard :refresh-key="usageRefreshKey" />
  </section>
</template>
