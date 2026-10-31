<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from "vue";
import {
  AlertTriangle,
  CheckCircle2,
  KeyRound,
  RefreshCw,
  Server,
  Sparkles,
  Terminal,
  Zap,
} from "lucide-vue-next";
import UiButton from "../ui/UiButton.vue";
import { t } from "../../i18n";
import { pushToast } from "../../composables/useToast";
import {
  activateAIProfile,
  apiOrigin,
  detectAICLIs,
  findModelPrice,
  getAIConfig,
  getAIProviderCatalog,
  isSameApiOrigin,
  listAIKeys,
  listAIProfiles,
  listAIModels,
  saveAISettings,
  testAIConnectionResult,
  type AICLIOption,
  type AIKeyEntry,
  type AIModelPrice,
  type AIProfile,
  type AIProviderCatalog,
  type AIProviderPreset,
  type AITestResult,
  type AiConfigNoSecret,
  type AIMode,
} from "../../services/aiService";

const props = withDefaults(
  defineProps<{ refreshKey?: number; compact?: boolean }>(),
  { refreshKey: 0, compact: false }
);
const emit = defineEmits<{ (e: "saved"): void }>();

const catalog = ref<AIProviderCatalog>({ providers: [], models: [] });
const saved = ref<AiConfigNoSecret | null>(null);
const profiles = ref<AIProfile[]>([]);
const keys = ref<AIKeyEntry[]>([]);
const cliOptions = ref<AICLIOption[]>([]);
const isLoading = ref(false);
const loadError = ref("");
const loadWarning = ref("");
const isInitialLoading = computed(
  () => isLoading.value && (catalog.value.providers || []).length === 0
);
const isSaving = ref(false);
const isTesting = ref(false);
const cliLoading = ref(false);
const modelsBusy = ref(false);
const modelsMessage = ref("");
const result = ref<AITestResult | null>(null);
const apiPresetId = ref("");
const cliPresetId = ref("");
const endpointEditing = ref(false);
const fetchedModels = ref<string[]>([]);

const form = reactive({
  mode: "api" as AIMode,
  baseUrl: "",
  protocol: "openai" as "openai" | "azure" | "anthropic" | "gemini",
  apiModel: "",
  cliModel: "",
  cliCommand: "",
  cliArgs: [] as string[],
  cliInput: "stdin" as "stdin" | "arg",
  apiKey: "",
  azureApiVersion: "2024-10-21",
  timeoutSeconds: 60,
});

const customProvider: AIProviderPreset = {
  id: "custom",
  name: t("ai.quick.custom"),
  mode: "api",
  protocol: "openai",
  baseUrl: "",
  models: [],
  defaultModel: "",
  keyRequired: false,
};

const providers = computed<AIProviderPreset[]>(() => [
  ...(catalog.value.providers || []),
  customProvider,
]);
const cliCards = computed(() => cliOptions.value);
const selectedProvider = computed(() =>
  providers.value.find((provider) => provider.id === apiPresetId.value) || null
);
const selectedCLI = computed(() =>
  cliOptions.value.find((option) => option.id === cliPresetId.value.replace(/^cli:/, "")) || null
);
const selectedPreset = computed(() => (form.mode === "api" ? apiPresetId.value : cliPresetId.value));
const isCustomAPI = computed(() => form.mode === "api" && apiPresetId.value === "custom");
const isCustomCLI = computed(() => form.mode === "cli" && cliPresetId.value === "custom-cli");
const modelValue = computed({
  get: () => (form.mode === "api" ? form.apiModel : form.cliModel),
  set: (value: string) => {
    if (form.mode === "api") form.apiModel = value;
    else form.cliModel = value;
  },
});
const modelOptions = computed(() => {
  const presetModels = selectedProvider.value?.models || [];
  return Array.from(new Set([...fetchedModels.value, ...presetModels].filter(Boolean)));
});
const price = computed<AIModelPrice | null>(() =>
  findModelPrice(catalog.value, modelValue.value)
);
const keyAvailableForEndpoint = computed(() =>
  keys.value.some(
    (entry) => entry.available && entry.origin === apiOrigin(form.baseUrl)
  )
);
const endpointKnown = computed(() => form.baseUrl.trim() !== "");
const showEndpointInput = computed(
  () =>
    form.mode === "api" &&
    (isCustomAPI.value || form.protocol === "azure" || endpointEditing.value || !endpointKnown.value)
);
const presetDesc = computed(() => {
  if (form.mode === "api") {
    if (apiPresetId.value === "custom") return t("ai.preset.custom.desc");
    if (!apiPresetId.value) return "";
    const key = `ai.preset.${apiPresetId.value}.desc`;
    const text = t(key);
    return text === key ? "" : text;
  }
  if (cliPresetId.value === "custom-cli") return t("ai.preset.customcli.desc");
  const option = selectedCLI.value;
  if (!option) return "";
  return option.available
    ? `${t("ai.quick.detected")}${option.version ? ` · ${option.version}` : ""}`
    : t("ai.quick.notdetected");
});
const availableCLIs = computed(() => cliOptions.value.filter((option) => option.available));
const activeProfileId = computed(() => saved.value?.activeProfile || "");
const configured = computed(() => {
  const value = saved.value;
  if (!value) return false;
  return value.mode === "cli" ? value.cliCommand.trim() !== "" : value.baseUrl.trim() !== "";
});

const errorText = (err: unknown): string => (err instanceof Error ? err.message : String(err));

function formatContext(value: number): string {
  if (!value) return "—";
  if (value >= 1_000_000) return `${Math.round(value / 100_000) / 10}M`;
  if (value >= 1_000) return `${Math.round(value / 1_000)}k`;
  return String(value);
}

function applyConfig(value: AiConfigNoSecret | null): void {
  saved.value = value;
  if (!value) {
    form.mode = "api";
    apiPresetId.value = "";
    cliPresetId.value = "";
    form.baseUrl = "";
    form.apiModel = "";
    form.cliModel = "";
    form.cliCommand = "";
    form.apiKey = "";
    return;
  }
  form.mode = value.mode === "cli" ? "cli" : "api";
  form.timeoutSeconds = value.timeoutSeconds > 0 ? value.timeoutSeconds : 60;
  form.azureApiVersion = value.azureApiVersion || "2024-10-21";
  form.protocol =
    value.protocol === "azure" || value.protocol === "anthropic" || value.protocol === "gemini"
      ? value.protocol
      : "openai";
  if (value.mode === "cli") {
    form.cliCommand = value.cliCommand;
    form.cliArgs = [...(value.cliArgs || [])];
    form.cliInput = value.cliInput === "arg" ? "arg" : "stdin";
    form.cliModel = value.model;
    const match = cliOptions.value.find(
      (option) => option.command === value.cliCommand || option.id === value.cliCommand
    );
    cliPresetId.value = match ? `cli:${match.id}` : "custom-cli";
  } else {
    form.baseUrl = value.baseUrl;
    form.apiModel = value.model;
    const match = providers.value.find(
      (provider) =>
        provider.id !== "custom" &&
        provider.baseUrl &&
        provider.protocol === value.protocol &&
        isSameApiOrigin(provider.baseUrl, value.baseUrl)
    );
    apiPresetId.value = match ? match.id : "custom";
  }
  endpointEditing.value = false;
}

async function loadSecretsAndProfiles(): Promise<string[]> {
  const warnings: string[] = [];
  const [profileResult, keyResult] = await Promise.allSettled([listAIProfiles(), listAIKeys()]);
  if (profileResult.status === "fulfilled") {
    profiles.value = profileResult.value || [];
  } else {
    warnings.push(`${t("ai.quick.load.profiles")}: ${errorText(profileResult.reason)}`);
  }
  if (keyResult.status === "fulfilled") {
    keys.value = keyResult.value || [];
  } else {
    warnings.push(`${t("ai.quick.load.keys")}: ${errorText(keyResult.reason)}`);
  }
  return warnings;
}

async function loadAll(): Promise<void> {
  isLoading.value = true;
  loadError.value = "";
  loadWarning.value = "";
  const errors: string[] = [];
  try {
    const [catalogResult, configResult] = await Promise.allSettled([
      getAIProviderCatalog(),
      getAIConfig(),
    ]);
    if (catalogResult.status === "fulfilled") {
      catalog.value = {
        providers: catalogResult.value.providers || [],
        models: catalogResult.value.models || [],
      };
    } else {
      errors.push(`${t("ai.quick.load.catalog")}: ${errorText(catalogResult.reason)}`);
    }
    if (configResult.status === "fulfilled") {
      applyConfig(configResult.value);
    } else {
      errors.push(`${t("ai.quick.load.config")}: ${errorText(configResult.reason)}`);
    }
    const warnings = await loadSecretsAndProfiles();
    loadWarning.value = warnings.join("; ");
  } finally {
    loadError.value = errors.join("; ");
    isLoading.value = false;
  }
}

async function detectCLIs(): Promise<void> {
  cliLoading.value = true;
  try {
    cliOptions.value = (await detectAICLIs()) || [];
    if (saved.value?.mode === "cli" && (cliPresetId.value === "custom-cli" || !cliPresetId.value)) {
      const match = cliOptions.value.find(
        (option) => option.command === saved.value?.cliCommand || option.id === saved.value?.cliCommand
      );
      if (match) cliPresetId.value = `cli:${match.id}`;
    }
  } catch (err) {
    cliOptions.value = [];
    loadWarning.value = [loadWarning.value, `${t("ai.quick.load.cli")}: ${errorText(err)}`]
      .filter(Boolean)
      .join("; ");
  } finally {
    cliLoading.value = false;
  }
}

async function retryLoad(): Promise<void> {
  await loadAll();
  await detectCLIs();
}

function setMode(mode: AIMode): void {
  form.mode = mode;
  result.value = null;
  endpointEditing.value = false;
}

function selectAPI(provider: AIProviderPreset): void {
  form.mode = "api";
  apiPresetId.value = provider.id;
  form.apiKey = "";
  endpointEditing.value = false;
  fetchedModels.value = [];
  modelsMessage.value = "";
  result.value = null;
  if (provider.id === "custom") {
    form.protocol = "openai";
    form.baseUrl = "";
    form.apiModel = "";
    return;
  }
  form.baseUrl = provider.baseUrl;
  form.protocol =
    provider.protocol === "azure" || provider.protocol === "anthropic" || provider.protocol === "gemini"
      ? provider.protocol
      : "openai";
  form.apiModel = provider.defaultModel;
  if (provider.id === "azure") endpointEditing.value = true;
}

function selectCLI(option: AICLIOption): void {
  form.mode = "cli";
  cliPresetId.value = `cli:${option.id}`;
  form.cliCommand = option.command;
  form.cliArgs = [...(option.args || [])];
  form.cliInput = option.input === "arg" ? "arg" : "stdin";
  form.apiKey = "";
  result.value = null;
}

function selectCustomCLI(): void {
  form.mode = "cli";
  cliPresetId.value = "custom-cli";
  form.cliCommand = "";
  form.cliArgs = [];
  form.cliInput = "stdin";
  form.apiKey = "";
  result.value = null;
}

function presetLabel(provider: AIProviderPreset): string {
  if (provider.id === "custom") return t("ai.quick.custom");
  return provider.name;
}

function validate(): string {
  if (form.mode === "cli") {
    if (!form.cliCommand.trim()) return t("ai.quick.err.command");
    return "";
  }
  const base = form.baseUrl.trim();
  if (!base) return t("ai.quick.err.endpoint");
  try {
    const parsed = new URL(base);
    if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
      return t("settings.localai.endpoint.invalid");
    }
  } catch {
    return t("settings.localai.endpoint.invalid");
  }
  if (!form.apiModel.trim()) return t("ai.quick.err.model");
  if (selectedProvider.value?.keyRequired && !keyAvailableForEndpoint.value && !form.apiKey.trim()) {
    return t("ai.quick.err.key");
  }
  return "";
}

function savePayload() {
  const isCLI = form.mode === "cli";
  const sameOrigin =
    Boolean(saved.value?.baseUrl) &&
    Boolean(form.baseUrl) &&
    isSameApiOrigin(saved.value?.baseUrl || "", form.baseUrl);
  return {
    mode: form.mode,
    baseUrl: isCLI ? saved.value?.baseUrl || "" : form.baseUrl.trim(),
    model: isCLI ? form.cliModel.trim() : form.apiModel.trim(),
    timeoutSeconds: form.timeoutSeconds,
    cliCommand: isCLI ? form.cliCommand.trim() : saved.value?.cliCommand || "",
    cliArgs: isCLI ? form.cliArgs : saved.value?.cliArgs || [],
    cliInput: isCLI ? form.cliInput : saved.value?.cliInput === "arg" ? ("arg" as const) : ("stdin" as const),
    commitLanguage: (saved.value?.commitLanguage as "auto" | "zh" | "en") || "auto",
    commitIncludeBody: saved.value?.commitIncludeBody === true,
    commitInstructions: saved.value?.commitInstructions || "",
    protocol: form.protocol,
    azureApiVersion: form.azureApiVersion,
    extraHeaders: !isCLI && sameOrigin ? saved.value?.extraHeaders || {} : {},
    cliEnvMode: (saved.value?.cliEnvMode as "inherit" | "minimal") || "inherit",
    apiKey: isCLI ? "" : form.apiKey.trim(),
  };
}

async function persist(withTest: boolean): Promise<void> {
  if (isSaving.value) return;
  const invalid = validate();
  if (invalid) {
    pushToast(invalid, { tone: "warning" });
    return;
  }
  isSaving.value = true;
  result.value = null;
  try {
    await saveAISettings(savePayload());
    form.apiKey = "";
    await loadAll();
    if (withTest) await testConnection();
    emit("saved");
    pushToast(t("ai.quick.saved"), { tone: "success" });
  } catch (err) {
    pushToast(t("settings.localai.save.failed", { message: errorText(err) }), { tone: "danger" });
  } finally {
    isSaving.value = false;
  }
}

async function testConnection(): Promise<void> {
  if (isTesting.value) return;
  isTesting.value = true;
  result.value = null;
  try {
    const next = await testAIConnectionResult();
    result.value = next;
    if (next.ok && next.models?.length) {
      fetchedModels.value = Array.from(new Set(next.models.map((model) => model.trim()).filter(Boolean)));
      modelsMessage.value = next.message;
    }
  } catch (err) {
    result.value = { ok: false, mode: form.mode, message: errorText(err) };
  } finally {
    isTesting.value = false;
  }
}

async function fetchModels(): Promise<void> {
  if (modelsBusy.value || form.mode !== "api") return;
  if (!form.baseUrl.trim()) {
    pushToast(t("ai.quick.err.endpoint"), { tone: "warning" });
    return;
  }
  modelsBusy.value = true;
  modelsMessage.value = "";
  try {
    const next = await listAIModels(
      form.baseUrl.trim(),
      form.apiKey.trim(),
      form.protocol,
      form.azureApiVersion
    );
    modelsMessage.value = next?.message || "";
    if (next?.ok) {
      fetchedModels.value = Array.from(
        new Set((next.models || []).map((model) => model.trim()).filter(Boolean))
      ).sort((a, b) => a.localeCompare(b));
      if (!form.apiModel.trim() && fetchedModels.value.length > 0) {
        form.apiModel = fetchedModels.value[0]!;
      }
    } else {
      pushToast(next?.message || t("ai.quick.model.none"), { tone: "warning" });
    }
  } catch (err) {
    pushToast(errorText(err), { tone: "danger" });
  } finally {
    modelsBusy.value = false;
  }
}

async function switchProfile(profile: AIProfile): Promise<void> {
  if (profile.id === activeProfileId.value) return;
  try {
    await activateAIProfile(profile.id);
    await loadAll();
    emit("saved");
    pushToast(t("ai.quick.saved"), { tone: "success" });
  } catch (err) {
    pushToast(t("ai.quick.profile.switchfailed", { error: errorText(err) }), { tone: "danger" });
  }
}

watch(
  () => props.refreshKey,
  (next, previous) => {
    if (next !== previous) void loadAll();
  }
);

onMounted(() => {
  void loadAll();
  void detectCLIs();
});
</script>

<template>
  <div
    class="ai-card space-y-4"
    :class="compact ? 'p-4' : 'p-5'"
    :aria-busy="isLoading"
    data-testid="ai-quick-setup"
  >
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div class="min-w-0">
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <Zap class="w-4 h-4 text-accent" />
          {{ t("ai.quick.title") }}
        </h3>
        <p class="text-[11px] text-ink-muted mt-0.5 leading-relaxed">{{ t("ai.quick.desc") }}</p>
      </div>
      <span
        class="px-2 py-1 rounded-lg text-[10px] font-semibold border"
        :class="configured ? 'bg-success/15 text-success border-success/30' : 'bg-warning/15 text-warning border-warning/30'"
      >
        {{ configured ? t("settings.localai.status.configured") : t("settings.localai.status.unconfigured") }}
      </span>
    </div>

    <div
      v-if="loadError"
      data-testid="ai-quick-load-error"
      role="alert"
      class="flex flex-wrap items-center gap-2 p-2.5 rounded-xl border border-danger/30 bg-danger/10 text-[11px] text-danger"
    >
      <AlertTriangle class="w-4 h-4 shrink-0" />
      <span class="min-w-0 flex-1 break-words">{{ t("ai.quick.load.failed", { error: loadError }) }}</span>
      <button
        type="button"
        data-testid="ai-quick-retry"
        class="px-2 py-1 rounded-lg border border-danger/40 bg-surface text-danger font-semibold cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-danger/50"
        @click="retryLoad"
      >
        {{ t("ai.quick.load.retry") }}
      </button>
    </div>
    <div
      v-else-if="loadWarning"
      data-testid="ai-quick-load-warning"
      role="status"
      class="flex items-start gap-2 p-2.5 rounded-xl border border-warning/30 bg-warning/10 text-[10px] text-warning"
    >
      <AlertTriangle class="w-3.5 h-3.5 shrink-0 mt-0.5" />
      <span class="min-w-0 break-words">{{ t("ai.quick.load.warning", { error: loadWarning }) }}</span>
    </div>

    <div v-if="isInitialLoading" data-testid="ai-quick-loading" aria-busy="true" class="space-y-2 py-1">
      <div class="h-3 w-1/3 rounded bg-line/50 animate-pulse"></div>
      <div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-1.5">
        <div v-for="i in 5" :key="i" class="h-9 rounded-xl bg-line/30 animate-pulse"></div>
      </div>
    </div>

    <div v-if="profiles.length" class="space-y-1.5" role="group" :aria-label="t('ai.quick.profile')">
      <div class="ai-field-label">{{ t("ai.quick.profile") }}</div>
      <div class="flex flex-wrap gap-1.5">
        <button
          v-for="profile in profiles"
          :key="profile.id"
          type="button"
          :data-testid="`ai-quick-profile-${profile.id}`"
          :aria-pressed="profile.id === activeProfileId"
          class="px-2.5 py-1 rounded-lg border text-[10px] font-semibold transition cursor-pointer max-w-[240px] truncate focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/50"
          :class="profile.id === activeProfileId
            ? 'bg-accent/15 text-accent border-accent/40'
            : 'bg-canvas text-ink-muted border-line hover:text-ink hover:bg-raised'"
          :title="profile.name"
          @click="switchProfile(profile)"
        >
          {{ profile.name }}
          <span v-if="profile.id === activeProfileId">· {{ t("ai.quick.profile.current") }}</span>
        </button>
      </div>
    </div>

    <div
      class="inline-flex p-1 rounded-xl border border-line bg-canvas"
      data-testid="ai-quick-mode-switch"
      role="group"
      :aria-label="t('ai.quick.mode.label')"
    >
      <button
        type="button"
        data-testid="ai-quick-mode-api"
        :aria-pressed="form.mode === 'api'"
        class="px-3 py-1.5 rounded-lg text-[11px] font-semibold transition cursor-pointer flex items-center gap-1 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/50"
        :class="form.mode === 'api' ? 'bg-raised text-accent shadow-xs' : 'text-ink-muted hover:text-ink'"
        @click="setMode('api')"
      >
        <Server class="w-3.5 h-3.5" />
        {{ t("ai.quick.mode.api") }}
      </button>
      <button
        type="button"
        data-testid="ai-quick-mode-cli"
        :aria-pressed="form.mode === 'cli'"
        class="px-3 py-1.5 rounded-lg text-[11px] font-semibold transition cursor-pointer flex items-center gap-1 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/50"
        :class="form.mode === 'cli' ? 'bg-raised text-accent shadow-xs' : 'text-ink-muted hover:text-ink'"
        @click="setMode('cli')"
      >
        <Terminal class="w-3.5 h-3.5" />
        {{ t("ai.quick.mode.cli") }}
      </button>
    </div>

    <div class="space-y-2">
      <div id="ai-quick-providers-label" class="ai-field-label">{{ t("ai.quick.providers") }}</div>
      <div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-1.5" role="group" aria-labelledby="ai-quick-providers-label">
        <template v-if="form.mode === 'api'">
          <button
            v-for="provider in providers"
            :key="provider.id"
            type="button"
            :data-testid="`ai-quick-preset-${provider.id}`"
            :aria-pressed="apiPresetId === provider.id"
            class="text-left px-2.5 py-2 rounded-xl border transition cursor-pointer min-w-0 flex items-center justify-between gap-1.5 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/50"
            :class="apiPresetId === provider.id
              ? 'bg-accent/10 border-accent/40'
              : 'bg-canvas border-line hover:bg-raised'"
            @click="selectAPI(provider)"
          >
            <span class="text-[11px] font-bold text-ink truncate">{{ presetLabel(provider) }}</span>
            <span v-if="provider.local" class="text-[9px] text-success shrink-0">{{ t("ai.quick.local") }}</span>
          </button>
        </template>
        <template v-else>
          <button
            v-for="option in cliCards"
            :key="option.id"
            type="button"
            :data-testid="`ai-quick-preset-cli-${option.id}`"
            :aria-pressed="cliPresetId === `cli:${option.id}`"
            class="text-left px-2.5 py-2 rounded-xl border transition cursor-pointer min-w-0 flex items-center justify-between gap-1.5 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/50"
            :class="cliPresetId === `cli:${option.id}`
              ? 'bg-accent/10 border-accent/40'
              : 'bg-canvas border-line hover:bg-raised'"
            @click="selectCLI(option)"
          >
            <span class="text-[11px] font-bold text-ink truncate">{{ option.name }}</span>
            <span class="text-[9px] shrink-0" :class="option.available ? 'text-success' : 'text-ink-muted'">
              {{ option.available ? t("ai.quick.detected") : t("ai.quick.notdetected") }}
            </span>
          </button>
          <button
            type="button"
            data-testid="ai-quick-preset-customcli"
            :aria-pressed="cliPresetId === 'custom-cli'"
            class="text-left px-2.5 py-2 rounded-xl border transition cursor-pointer min-w-0 flex items-center justify-between gap-1.5 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/50"
            :class="cliPresetId === 'custom-cli' ? 'bg-accent/10 border-accent/40' : 'bg-canvas border-line hover:bg-raised'"
            @click="selectCustomCLI"
          >
            <span class="text-[11px] font-bold text-ink truncate">{{ t("ai.quick.customcli") }}</span>
          </button>
        </template>
      </div>
      <p v-if="presetDesc" class="ai-hint">{{ presetDesc }}</p>
      <p v-if="form.mode === 'cli' && cliLoading" class="ai-hint">{{ t("common.loading") }}</p>
    </div>

    <div v-if="selectedPreset" class="space-y-3 pt-1" data-testid="ai-quick-fields">
      <div v-if="form.mode === 'api'" class="space-y-1">
        <div class="flex items-center justify-between gap-2">
          <label
            class="ai-field-label"
            :for="showEndpointInput ? 'ai-quick-endpoint-input' : undefined"
          >{{ t("ai.quick.endpoint") }}</label>
          <button
            v-if="!isCustomAPI && form.protocol !== 'azure'"
            type="button"
            :aria-label="t('ai.quick.endpoint.change')"
            class="text-[10px] font-semibold text-accent hover:text-accent-strong cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/50 rounded"
            @click="endpointEditing = !endpointEditing"
          >
            {{ t("ai.quick.endpoint.change") }}
          </button>
        </div>
        <input
          v-if="showEndpointInput"
          id="ai-quick-endpoint-input"
          v-model="form.baseUrl"
          type="url"
          data-testid="ai-quick-endpoint"
          placeholder="https://api.example.com/v1"
          class="ai-input"
        />
        <div v-else class="px-3 py-2 rounded-xl bg-canvas border border-line text-xs font-mono text-ink-muted truncate">
          {{ form.baseUrl || "—" }}
        </div>
      </div>

      <div class="space-y-1">
        <div class="flex items-center justify-between gap-2">
          <label class="ai-field-label" for="ai-quick-model-input">
            {{ form.mode === "api" ? t("ai.quick.model") : t("ai.quick.model.optional") }}
          </label>
          <button
            v-if="form.mode === 'api'"
            type="button"
            :disabled="modelsBusy"
            :aria-label="modelsBusy ? t('ai.quick.model.fetching') : t('ai.quick.model.fetch')"
            data-testid="ai-quick-model-fetch"
            class="flex items-center gap-1 text-[10px] font-semibold text-accent hover:text-accent-strong cursor-pointer disabled:opacity-40"
            @click="fetchModels"
          >
            <RefreshCw class="w-3 h-3" :class="{ 'animate-spin': modelsBusy }" />
            {{ modelsBusy ? t("ai.quick.model.fetching") : t("ai.quick.model.fetch") }}
          </button>
        </div>
        <input
          id="ai-quick-model-input"
          v-model="modelValue"
          type="text"
          list="ai-quick-model-options"
          data-testid="ai-quick-model"
          :placeholder="form.mode === 'api' ? 'gpt-4.1-mini' : t('ai.quick.model.optional')"
          class="ai-input"
        />
        <datalist id="ai-quick-model-options">
          <option v-for="model in modelOptions" :key="model" :value="model" />
        </datalist>
        <p v-if="price && form.mode === 'api'" class="ai-hint">
          <template v-if="price.provider === 'local'">{{ t("ai.quick.price.local") }}</template>
          <template v-else>
            {{ t("ai.quick.price", {
              input: price.inputUsdPerMTok.toFixed(2),
              output: price.outputUsdPerMTok.toFixed(2),
              context: formatContext(price.contextWindow),
            }) }}
          </template>
        </p>
        <p v-else-if="modelsMessage" class="ai-hint">{{ modelsMessage }}</p>
        <p v-else-if="form.mode === 'api' && form.apiModel && !price" class="ai-hint">
          {{ t("ai.quick.price.unknown") }}
        </p>
      </div>

      <div v-if="form.mode === 'api'" class="space-y-1">
        <label class="ai-field-label" for="ai-quick-key-input">{{ t("ai.quick.key") }}</label>
        <div class="relative">
          <KeyRound class="w-3.5 h-3.5 text-ink-muted absolute left-3 top-2.5" />
          <input
            id="ai-quick-key-input"
            v-model="form.apiKey"
            type="password"
            autocomplete="off"
            data-testid="ai-quick-key"
            :placeholder="keyAvailableForEndpoint ? t('ai.quick.key.saved') : 'sk-...'"
            class="w-full pl-8 pr-3 py-2 rounded-xl bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
          />
        </div>
        <p class="ai-hint">
          {{ keyAvailableForEndpoint ? t("ai.quick.key.saved") : selectedProvider?.local ? t("ai.quick.key.local") : t("settings.localai.key.stored") }}
        </p>
      </div>

      <div v-else class="space-y-1">
        <label class="ai-field-label" for="ai-quick-command-input">{{ t("ai.quick.command") }}</label>
        <input
          id="ai-quick-command-input"
          v-model="form.cliCommand"
          type="text"
          list="ai-quick-cli-options"
          data-testid="ai-quick-command"
          placeholder="claude / codex / ollama"
          class="w-full px-3 py-2 rounded-xl bg-canvas border border-line text-xs font-mono text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
        />
        <datalist id="ai-quick-cli-options">
          <option v-for="option in availableCLIs" :key="option.id" :value="option.command">{{ option.name }}</option>
        </datalist>
        <p class="ai-hint">{{ t("ai.quick.command.hint") }}</p>
      </div>

      <div
        class="space-y-2"
        :class="compact ? 'sticky bottom-0 z-10 -mx-1 rounded-xl bg-surface/95 px-1 py-2 backdrop-blur-sm' : ''"
      >
        <div
          v-if="result"
          data-testid="ai-quick-test-result"
          class="flex items-start gap-2 p-2.5 rounded-xl border text-[11px]"
          :class="result.ok ? 'bg-success/10 border-success/30 text-success' : 'bg-danger/10 border-danger/30 text-danger'"
        >
          <CheckCircle2 v-if="result.ok" class="w-4 h-4 shrink-0 mt-0.5" />
          <AlertTriangle v-else class="w-4 h-4 shrink-0 mt-0.5" />
          <div class="min-w-0">
            <div class="font-semibold">{{ result.ok ? t("ai.quick.testok") : t("ai.quick.testfail") }}</div>
            <div class="text-[10px] opacity-90 break-words">{{ result.message || result.detail || "" }}</div>
          </div>
        </div>

        <div class="flex flex-wrap items-center gap-2">
          <UiButton variant="primary" :disabled="isSaving || isTesting" data-testid="ai-quick-save" @click="persist(true)">
            <Sparkles class="w-3.5 h-3.5" :class="{ 'animate-spin': isSaving || isTesting }" />
            {{ isSaving || isTesting ? t("ai.quick.testing") : t("ai.quick.save") }}
          </UiButton>
          <UiButton variant="secondary" :disabled="isSaving" data-testid="ai-quick-save-only" @click="persist(false)">
            {{ t("ai.quick.saveonly") }}
          </UiButton>
        </div>
      </div>
    </div>
  </div>
</template>
