<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from "vue";
import {
  AlertTriangle,
  Check,
  CheckCircle2,
  Cpu,
  Download,
  KeyRound,
  Layers,
  Pencil,
  Plus,
  Save,
  Server,
  Trash2,
  Upload,
  X,
  XCircle,
} from "lucide-vue-next";
import UiButton from "../ui/UiButton.vue";
import { t } from "../../i18n";
import {
  activateAIProfile,
  apiOrigin,
  deleteAIKeyForOrigin,
  deleteAIProfile,
  detectAICLIs,
  findModelPrice,
  getAIAdvancedConfig,
  getAIConfig,
  getAIProviderCatalog,
  isSameApiOrigin,
  listAIKeys,
  listAIProfiles,
  saveAIAdvancedConfig,
  saveAIKeyForOrigin,
  saveAISettings,
  type AICLIOption,
  type AIAdvancedConfig,
  type AIKeyEntry,
  type AIProfile,
  type AIProviderCatalog,
  type AiConfigNoSecret,
} from "../../services/aiService";
import { pushToast } from "../../composables/useToast";
import { confirmAction } from "../../composables/useConfirm";
import {
  buildAIConfigBundle,
  parseAIConfigBundle,
  serializeAIConfigBundle,
  toAdvancedSaveInput,
  toAISaveInput,
} from "../../utils/aiConfigTransfer";

const props = withDefaults(defineProps<{ refreshKey?: number }>(), { refreshKey: 0 });
const emit = defineEmits<{ (e: "changed"): void }>();

const TASK_KINDS = ["commit", "summary", "conflict", "explain", "chat", "review", "release"] as const;

const advanced = reactive<AIAdvancedConfig>({
  taskModels: {},
  repoOverrides: {},
  profileNames: {},
  inputUsdPerMTok: 0,
  outputUsdPerMTok: 0,
  monthlyBudgetUsd: 0,
  retryCount: 0,
  rateLimitPerMinute: 0,
  fallbackBaseUrl: "",
  fallbackModel: "",
  fallbackProtocol: "openai",
});
const keys = ref<AIKeyEntry[]>([]);
const profiles = ref<AIProfile[]>([]);
const config = ref<AiConfigNoSecret | null>(null);
const catalog = ref<AIProviderCatalog | null>(null);
const cliOptions = ref<AICLIOption[]>([]);
const isLoading = ref(false);
const loadError = ref("");
const isSaving = ref(false);
const isAddingKey = ref(false);
const renamingProfileId = ref("");
const renameDraft = ref("");
const isRenamingProfile = ref(false);
const importInput = ref<HTMLInputElement | null>(null);
const isTransferring = ref(false);
const keyForm = reactive({ origin: "", apiKey: "" });

const errorText = (err: unknown): string => (err instanceof Error ? err.message : String(err));

function applyAdvanced(next: AIAdvancedConfig | null): void {
  if (!next) return;
  advanced.taskModels = next.taskModels || {};
  advanced.repoOverrides = next.repoOverrides || {};
  advanced.profileNames = next.profileNames || {};
  advanced.inputUsdPerMTok = next.inputUsdPerMTok || 0;
  advanced.outputUsdPerMTok = next.outputUsdPerMTok || 0;
  advanced.monthlyBudgetUsd = next.monthlyBudgetUsd || 0;
  advanced.retryCount = next.retryCount || 0;
  advanced.rateLimitPerMinute = next.rateLimitPerMinute || 0;
  advanced.fallbackBaseUrl = next.fallbackBaseUrl || "";
  advanced.fallbackModel = next.fallbackModel || "";
  advanced.fallbackProtocol = next.fallbackProtocol || "openai";
}

type DiagnosticLevel = "ok" | "warn" | "fail";
interface DiagnosticRow {
  id: string;
  level: DiagnosticLevel;
  label: string;
  detail: string;
}

const diagnostics = computed<DiagnosticRow[]>(() => {
  const current = config.value;
  if (!current) return [];
  const rows: DiagnosticRow[] = [];
  const isCLI = current.mode === "cli";
  const command = current.cliCommand.trim();

  if (isCLI) {
    const option = cliOptions.value.find(
      (item) => item.command === command || item.id === command
    );
    const found = Boolean(command && option?.available);
    rows.push({
      id: "endpoint",
      level: found ? "ok" : "fail",
      label: t("settings.ai.diagnostics.cli"),
      detail: !command
        ? t("settings.ai.diagnostics.cli.missing", { command: "—" })
        : found
          ? t("settings.ai.diagnostics.cli.ok", { command, version: option?.version || "" })
          : t("settings.ai.diagnostics.cli.missing", { command }),
    });
  } else {
    let urlOK = false;
    try {
      const parsed = new URL(current.baseUrl);
      urlOK = parsed.protocol === "http:" || parsed.protocol === "https:";
    } catch {
      urlOK = false;
    }
    rows.push({
      id: "endpoint",
      level: urlOK ? "ok" : "fail",
      label: t("settings.ai.diagnostics.endpoint"),
      detail: urlOK
        ? t("settings.ai.diagnostics.endpoint.ok", {
            protocol: current.protocol || "openai",
            url: current.baseUrl,
          })
        : t("settings.ai.diagnostics.endpoint.invalid"),
    });

    const provider = (catalog.value?.providers || []).find(
      (item) => item.baseUrl && isSameApiOrigin(item.baseUrl, current.baseUrl)
    );
    if (provider?.keyRequired === false) {
      rows.push({
        id: "key",
        level: "ok",
        label: t("settings.ai.diagnostics.key"),
        detail: t("settings.ai.diagnostics.key.notrequired"),
      });
    } else if (current.keyAvailable) {
      rows.push({
        id: "key",
        level: "ok",
        label: t("settings.ai.diagnostics.key"),
        detail: t("settings.ai.diagnostics.key.ok"),
      });
    } else {
      rows.push({
        id: "key",
        level: "warn",
        label: t("settings.ai.diagnostics.key"),
        detail: t("settings.ai.diagnostics.key.missing"),
      });
    }

    if (!current.model.trim()) {
      rows.push({
        id: "model",
        level: "fail",
        label: t("settings.ai.diagnostics.model"),
        detail: t("settings.ai.diagnostics.model.missing"),
      });
    } else {
      const price = findModelPrice(catalog.value, current.model);
      rows.push({
        id: "model",
        level: price ? "ok" : "warn",
        label: t("settings.ai.diagnostics.model"),
        detail: price
          ? price.provider === "local"
            ? t("settings.ai.diagnostics.model.local", { model: current.model })
            : t("settings.ai.diagnostics.model.ok", { model: current.model })
          : t("settings.ai.diagnostics.model.noprice", { model: current.model }),
      });
    }
  }

  const fallbackBase = advanced.fallbackBaseUrl.trim();
  const fallbackModel = advanced.fallbackModel.trim();
  if (!fallbackBase && !fallbackModel) {
    rows.push({
      id: "fallback",
      level: "ok",
      label: t("settings.ai.diagnostics.fallback"),
      detail: t("settings.ai.diagnostics.fallback.off"),
    });
  } else if (!fallbackBase || !fallbackModel) {
    rows.push({
      id: "fallback",
      level: "fail",
      label: t("settings.ai.diagnostics.fallback"),
      detail: t("settings.ai.diagnostics.fallback.partial"),
    });
  } else {
    let fallbackOK = false;
    try {
      fallbackOK = Boolean(apiOrigin(fallbackBase));
    } catch {
      fallbackOK = false;
    }
    rows.push({
      id: "fallback",
      level: fallbackOK ? "ok" : "fail",
      label: t("settings.ai.diagnostics.fallback"),
      detail: fallbackOK
        ? t("settings.ai.diagnostics.fallback.ok", {
            url: fallbackBase,
            model: fallbackModel,
          })
        : t("settings.ai.diagnostics.endpoint.invalid"),
    });
  }

  if (current.notice.trim()) {
    rows.push({
      id: "notice",
      level: "warn",
      label: t("settings.ai.diagnostics.notice"),
      detail: current.notice,
    });
  }
  return rows;
});

const sectionLinks = computed(() =>
  [
    { id: "ai-section-diagnostics", labelKey: "settings.ai.nav.diagnostics", show: diagnostics.value.length > 0 },
    { id: "ai-section-routing", labelKey: "settings.ai.nav.routing", show: true },
    { id: "ai-section-reliability", labelKey: "settings.ai.nav.reliability", show: true },
    { id: "ai-section-profiles", labelKey: "settings.ai.nav.profiles", show: true },
    { id: "ai-section-keys", labelKey: "settings.ai.nav.keys", show: true },
  ].filter((section) => section.show)
);

function jumpToSection(id: string): void {
  const target = document.getElementById(id);
  if (target && typeof target.scrollIntoView === "function") {
    target.scrollIntoView({ behavior: "smooth", block: "start" });
  }
}

async function loadAll(): Promise<void> {
  isLoading.value = true;
  loadError.value = "";
  try {
    const [adv, keyList, profileList, configResult, catalogResult, cliResult] = await Promise.all([
      getAIAdvancedConfig(),
      listAIKeys(),
      listAIProfiles(),
      getAIConfig().catch(() => null),
      getAIProviderCatalog().catch(() => null),
      detectAICLIs().catch(() => []),
    ]);
    applyAdvanced(adv);
    keys.value = keyList || [];
    profiles.value = profileList || [];
    config.value = configResult;
    catalog.value = catalogResult;
    cliOptions.value = cliResult || [];
  } catch (err) {
    loadError.value = errorText(err);
  } finally {
    isLoading.value = false;
  }
}

async function saveAdvanced(): Promise<void> {
  if (isSaving.value) return;
  isSaving.value = true;
  try {
    // Budget/pricing and repository overrides are edited elsewhere; read the
    // current values so saving routing/reliability cannot roll them back.
    const current = await getAIAdvancedConfig();
    await saveAIAdvancedConfig({
      ...advanced,
      taskModels: { ...advanced.taskModels },
      profileNames:
        Object.keys(advanced.profileNames || {}).length > 0
          ? { ...advanced.profileNames }
          : { ...(current.profileNames || {}) },
      inputUsdPerMTok: current.inputUsdPerMTok || 0,
      outputUsdPerMTok: current.outputUsdPerMTok || 0,
      monthlyBudgetUsd: current.monthlyBudgetUsd || 0,
      repoOverrides: current.repoOverrides || {},
    });
    await loadAll();
    pushToast(t("settings.ai.control.saved"), { tone: "success" });
    emit("changed");
  } catch (err) {
    pushToast(t("settings.ai.control.failed", { error: errorText(err) }), { tone: "danger" });
  } finally {
    isSaving.value = false;
  }
}

async function exportConfig(): Promise<void> {
  if (isTransferring.value || !config.value) return;
  isTransferring.value = true;
  try {
    const bundle = buildAIConfigBundle(config.value, advanced);
    await navigator.clipboard.writeText(serializeAIConfigBundle(bundle));
    pushToast(t("settings.ai.transfer.exported"), { tone: "success" });
  } catch (err) {
    pushToast(t("settings.ai.transfer.exportfailed", { error: errorText(err) }), { tone: "danger" });
  } finally {
    isTransferring.value = false;
  }
}

function pickImportFile(): void {
  if (!isTransferring.value) importInput.value?.click();
}

async function handleImportFile(event: Event): Promise<void> {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  // Allow importing the same file twice in one session.
  input.value = "";
  if (!file || isTransferring.value) return;
  if (file.size > 1_000_000) {
    pushToast(t("settings.ai.transfer.toolarge"), { tone: "warning" });
    return;
  }
  isTransferring.value = true;
  try {
    const parsed = parseAIConfigBundle(await file.text());
    if (!parsed.ok) {
      pushToast(
        t("settings.ai.transfer.importinvalid", {
          error: t(`settings.ai.transfer.err.${parsed.reason}`),
        }),
        { tone: "warning" }
      );
      return;
    }
    const bundle = parsed.bundle;
    const target =
      bundle.config.mode === "cli"
        ? `${bundle.config.cliCommand} ${bundle.config.model}`.trim()
        : `${bundle.config.baseUrl} · ${bundle.config.model}`;
    if (!(await confirmAction({ message: t("settings.ai.transfer.importconfirm", { target }), tone: "warning" }))) {
      return;
    }
    await saveAISettings(toAISaveInput(bundle.config));
    // Repository overrides stay machine-local and are preserved, not imported.
    const current = await getAIAdvancedConfig();
    await saveAIAdvancedConfig(toAdvancedSaveInput(bundle.advanced, current.repoOverrides || {}));
    await loadAll();
    pushToast(t("settings.ai.transfer.imported"), { tone: "success" });
    emit("changed");
  } catch (err) {
    pushToast(t("settings.ai.transfer.importfailed", { error: errorText(err) }), { tone: "danger" });
  } finally {
    isTransferring.value = false;
  }
}

async function addKey(): Promise<void> {
  if (isAddingKey.value) return;
  if (!keyForm.origin.trim() || !keyForm.apiKey.trim()) return;
  isAddingKey.value = true;
  try {
    await saveAIKeyForOrigin(keyForm.origin.trim(), keyForm.apiKey);
    keyForm.apiKey = "";
    keys.value = await listAIKeys();
    pushToast(t("settings.ai.keys.added"), { tone: "success" });
    emit("changed");
  } catch (err) {
    pushToast(errorText(err), { tone: "danger" });
  } finally {
    isAddingKey.value = false;
  }
}

async function removeKey(origin: string): Promise<void> {
  if (!(await confirmAction({ message: `${t("settings.ai.keys.delete")} ${origin}?`, tone: "danger" }))) return;
  try {
    await deleteAIKeyForOrigin(origin);
    keys.value = await listAIKeys();
    pushToast(t("settings.ai.keys.deleted"), { tone: "success" });
    emit("changed");
  } catch (err) {
    pushToast(errorText(err), { tone: "danger" });
  }
}

async function switchProfile(profile: AIProfile): Promise<void> {
  if (profile.active) return;
  try {
    await activateAIProfile(profile.id);
    await loadAll();
    pushToast(t("ai.quick.saved"), { tone: "success" });
    emit("changed");
  } catch (err) {
    pushToast(t("ai.quick.profile.switchfailed", { error: errorText(err) }), { tone: "danger" });
  }
}

function startRenameProfile(profile: AIProfile): void {
  renamingProfileId.value = profile.id;
  renameDraft.value = profile.name;
}

function cancelRenameProfile(): void {
  renamingProfileId.value = "";
  renameDraft.value = "";
}

async function saveProfileName(profile: AIProfile): Promise<void> {
  if (isRenamingProfile.value) return;
  const name = renameDraft.value.trim();
  if (name.length > 80) {
    pushToast(t("ai.profile.rename.toolong"), { tone: "warning" });
    return;
  }
  isRenamingProfile.value = true;
  try {
    const current = await getAIAdvancedConfig();
    const labels = { ...(current.profileNames || {}) };
    if (name) labels[profile.id] = name;
    else delete labels[profile.id];
    await saveAIAdvancedConfig({ ...current, profileNames: labels });
    await loadAll();
    cancelRenameProfile();
    pushToast(t("ai.profile.renamed"), { tone: "success" });
    emit("changed");
  } catch (err) {
    pushToast(t("settings.ai.control.failed", { error: errorText(err) }), { tone: "danger" });
  } finally {
    isRenamingProfile.value = false;
  }
}

async function removeProfile(profile: AIProfile): Promise<void> {
  if (!(await confirmAction({ message: `${t("ai.profile.delete")} ${profile.name}?`, tone: "danger" }))) return;
  try {
    await deleteAIProfile(profile.id);
    await loadAll();
    pushToast(t("ai.profile.deleted"), { tone: "success" });
    emit("changed");
  } catch (err) {
    pushToast(errorText(err), { tone: "danger" });
  }
}

watch(
  () => props.refreshKey,
  (next, previous) => {
    if (next !== previous) void loadAll();
  }
);

onMounted(loadAll);
</script>

<template>
  <div class="space-y-5" :aria-busy="isLoading" data-testid="ai-control-plane">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div class="min-w-0">
        <h3 class="text-sm font-bold text-ink">{{ t("settings.ai.control.title") }}</h3>
        <p class="text-[11px] text-ink-muted mt-0.5 leading-relaxed">{{ t("settings.ai.control.desc") }}</p>
      </div>
      <div class="flex flex-wrap items-center justify-end gap-1.5">
        <UiButton
          variant="secondary"
          :disabled="isTransferring || !config"
          data-testid="ai-config-export"
          @click="exportConfig"
        >
          <Download class="w-3.5 h-3.5" aria-hidden="true" />
          {{ t("settings.ai.transfer.export") }}
        </UiButton>
        <UiButton
          variant="secondary"
          :disabled="isTransferring"
          data-testid="ai-config-import"
          @click="pickImportFile"
        >
          <Upload class="w-3.5 h-3.5" aria-hidden="true" />
          {{ t("settings.ai.transfer.import") }}
        </UiButton>
        <input
          ref="importInput"
          type="file"
          accept="application/json,.json"
          class="hidden"
          data-testid="ai-config-import-file"
          @change="handleImportFile"
        />
        <UiButton variant="primary" :disabled="isSaving || isLoading || isTransferring" data-testid="ai-advanced-save" @click="saveAdvanced">
          <Save class="w-3.5 h-3.5" aria-hidden="true" />
          {{ t("settings.ai.control.save") }}
        </UiButton>
      </div>
    </div>

    <div
      v-if="loadError"
      data-testid="ai-control-load-error"
      role="alert"
      class="flex flex-wrap items-center gap-2 p-2.5 rounded-xl border border-danger/30 bg-danger/10 text-[11px] text-danger"
    >
      <span class="min-w-0 flex-1 break-words">{{ t("settings.ai.control.loadfailed", { error: loadError }) }}</span>
      <UiButton variant="secondary" data-testid="ai-control-retry" @click="loadAll">
        {{ t("ai.quick.load.retry") }}
      </UiButton>
    </div>

    <nav
      v-if="sectionLinks.length"
      class="flex flex-wrap gap-1.5"
      :aria-label="t('settings.ai.nav.label')"
      data-testid="ai-advanced-nav"
    >
      <button
        v-for="section in sectionLinks"
        :key="section.id"
        type="button"
        class="px-2.5 py-1 rounded-lg border border-line bg-canvas text-[10px] font-semibold text-ink-muted hover:text-ink hover:bg-raised transition cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/50"
        :data-testid="`ai-nav-${section.id.replace('ai-section-', '')}`"
        @click="jumpToSection(section.id)"
      >
        {{ t(section.labelKey) }}
      </button>
    </nav>

    <!-- Local-only diagnostics: never sends a network request. -->
    <div
      v-if="config && diagnostics.length"
      id="ai-section-diagnostics"
      class="ai-card p-4 space-y-2.5"
      data-testid="ai-diagnostics-card"
    >
      <div class="flex items-center gap-2">
        <Cpu class="w-4 h-4 text-accent" />
        <div>
          <h4 class="ai-card-title">{{ t("settings.ai.diagnostics.title") }}</h4>
          <p class="ai-card-desc">{{ t("settings.ai.diagnostics.desc") }}</p>
        </div>
      </div>
      <ul class="space-y-1.5">
        <li
          v-for="row in diagnostics"
          :key="row.id"
          class="flex items-start gap-2 text-[11px]"
          :data-testid="`ai-diagnostic-${row.id}`"
        >
          <CheckCircle2 v-if="row.level === 'ok'" class="w-3.5 h-3.5 text-success shrink-0 mt-0.5" />
          <AlertTriangle v-else-if="row.level === 'warn'" class="w-3.5 h-3.5 text-warning shrink-0 mt-0.5" />
          <XCircle v-else class="w-3.5 h-3.5 text-danger shrink-0 mt-0.5" />
          <span class="text-ink-muted shrink-0">{{ row.label }}</span>
          <span class="min-w-0 text-ink break-all">{{ row.detail }}</span>
        </li>
      </ul>
    </div>

    <!-- Per-task models -->
    <div id="ai-section-routing" class="ai-card p-4 space-y-3" data-testid="ai-task-models-card">
      <div class="flex items-center gap-2">
        <Cpu class="w-4 h-4 text-accent" />
        <div>
          <h4 class="ai-card-title">{{ t("settings.ai.tasks.title") }}</h4>
          <p class="ai-card-desc">{{ t("settings.ai.tasks.desc") }}</p>
        </div>
      </div>
      <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
        <label v-for="kind in TASK_KINDS" :key="kind" class="space-y-1">
          <span class="ai-field-label">{{ t(`settings.ai.tasks.${kind}`) }}</span>
          <input
            v-model="advanced.taskModels[kind]"
            type="text"
            :placeholder="t('settings.ai.tasks.inherit')"
            :data-testid="`ai-task-model-${kind}`"
            class="ai-input"
          />
        </label>
      </div>
    </div>

    <!-- Reliability + fallback -->
    <div id="ai-section-reliability" class="ai-card p-4 space-y-3" data-testid="ai-reliability-card">
      <div class="flex items-center gap-2">
        <Server class="w-4 h-4 text-accent" />
        <div>
          <h4 class="ai-card-title">{{ t("settings.ai.reliability.title") }}</h4>
          <p class="ai-card-desc">{{ t("settings.ai.reliability.desc") }}</p>
        </div>
      </div>
      <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
        <label class="space-y-1">
          <span class="ai-field-label">{{ t("settings.ai.reliability.retry") }}</span>
          <select v-model.number="advanced.retryCount" class="ai-input">
            <option :value="0">{{ t("settings.ai.reliability.retry.off") }}</option>
            <option :value="1">1</option>
            <option :value="2">2</option>
            <option :value="3">3</option>
          </select>
        </label>
        <label class="space-y-1">
          <span class="ai-field-label">{{ t("settings.ai.reliability.rate") }}</span>
          <input
            v-model.number="advanced.rateLimitPerMinute"
            type="number"
            min="0"
            max="600"
            step="1"
            :placeholder="t('settings.ai.reliability.rate.off')"
            class="ai-input"
          />
        </label>
      </div>

      <div class="pt-3 border-t border-line/60 space-y-2">
        <div>
          <h5 class="text-[11px] font-bold text-ink">{{ t("settings.ai.fallback.title") }}</h5>
          <p class="ai-card-desc">{{ t("settings.ai.fallback.desc") }}</p>
        </div>
        <div class="grid grid-cols-1 sm:grid-cols-3 gap-2">
          <label class="space-y-1 sm:col-span-2">
            <span class="ai-field-label">{{ t("settings.ai.fallback.base") }}</span>
            <input v-model="advanced.fallbackBaseUrl" type="url" placeholder="https://api.example.com/v1" class="ai-input" />
          </label>
          <label class="space-y-1">
            <span class="ai-field-label">{{ t("settings.ai.fallback.protocol") }}</span>
            <select v-model="advanced.fallbackProtocol" class="ai-input">
              <option value="openai">OpenAI</option>
              <option value="azure">Azure</option>
              <option value="anthropic">Anthropic</option>
              <option value="gemini">Gemini</option>
            </select>
          </label>
          <label class="space-y-1 sm:col-span-2">
            <span class="ai-field-label">{{ t("settings.ai.fallback.model") }}</span>
            <input v-model="advanced.fallbackModel" type="text" placeholder="fallback-model" class="ai-input" />
          </label>
        </div>
        <p class="ai-card-desc">{{ t("settings.ai.fallback.hint") }}</p>
      </div>
    </div>

    <!-- Profiles -->
    <div id="ai-section-profiles" class="ai-card p-4 space-y-3" data-testid="ai-profiles-card">
      <div class="flex items-center gap-2">
        <Layers class="w-4 h-4 text-accent" />
        <div>
          <h4 class="ai-card-title">{{ t("ai.profile.title") }}</h4>
          <p class="ai-card-desc">{{ t("ai.profile.desc") }}</p>
        </div>
      </div>
      <p v-if="!profiles.length" class="text-xs text-ink-muted">{{ t("ai.profile.empty") }}</p>
      <div v-else class="space-y-1.5">
        <div
          v-for="profile in profiles"
          :key="profile.id"
          class="flex flex-wrap items-center gap-2 p-2 rounded-xl bg-canvas border border-line"
        >
          <div v-if="renamingProfileId === profile.id" class="flex flex-wrap items-center gap-1.5 min-w-0 flex-1">
            <input
              v-model="renameDraft"
              type="text"
              maxlength="80"
              data-testid="ai-profile-rename-input"
              :aria-label="t('ai.profile.rename')"
              :placeholder="t('ai.profile.rename.placeholder')"
              class="min-w-0 flex-1 px-2 py-1.5 rounded-lg bg-surface border border-accent/40 text-[11px] text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
              @keydown.enter.prevent="saveProfileName(profile)"
              @keydown.esc="cancelRenameProfile"
            />
            <button
              type="button"
              class="p-1.5 rounded-lg text-success hover:bg-success/10 disabled:opacity-40"
              :disabled="isRenamingProfile"
              :aria-label="t('common.save')"
              @click="saveProfileName(profile)"
            >
              <Check class="w-3.5 h-3.5" aria-hidden="true" />
            </button>
            <button
              type="button"
              class="p-1.5 rounded-lg text-ink-muted hover:text-ink hover:bg-raised"
              :aria-label="t('common.cancel')"
              @click="cancelRenameProfile"
            >
              <X class="w-3.5 h-3.5" aria-hidden="true" />
            </button>
          </div>
          <button v-else type="button" class="min-w-0 flex-1 text-left cursor-pointer" @click="switchProfile(profile)">
            <div class="text-[11px] font-semibold text-ink truncate">{{ profile.name }}</div>
            <div class="text-[10px] text-ink-muted truncate">
              {{ profile.mode === "cli" ? profile.cliCommand : profile.baseUrl }} · {{ profile.model || "—" }}
            </div>
          </button>
          <span v-if="profile.active" class="px-1.5 py-0.5 rounded bg-accent/15 text-accent text-[9px] font-bold flex items-center gap-1">
            <CheckCircle2 class="w-3 h-3" />
            {{ t("ai.profile.active") }}
          </span>
          <button
            v-if="renamingProfileId !== profile.id"
            type="button"
            class="p-1.5 rounded-lg text-ink-muted hover:text-accent hover:bg-accent/10"
            :title="t('ai.profile.rename')"
            :aria-label="t('ai.profile.rename')"
            data-testid="ai-profile-rename"
            @click="startRenameProfile(profile)"
          >
            <Pencil class="w-3.5 h-3.5" aria-hidden="true" />
          </button>
          <button
            type="button"
            class="p-1.5 rounded-lg text-ink-muted hover:text-danger hover:bg-danger/10"
            :title="t('ai.profile.delete')"
            :aria-label="t('ai.profile.delete')"
            @click="removeProfile(profile)"
          >
            <Trash2 class="w-3.5 h-3.5" aria-hidden="true" />
          </button>
        </div>
      </div>
    </div>

    <!-- Multi-origin keys -->
    <div id="ai-section-keys" class="ai-card p-4 space-y-3" data-testid="ai-keys-card">
      <div class="flex items-center gap-2">
        <KeyRound class="w-4 h-4 text-accent" />
        <div>
          <h4 class="ai-card-title">{{ t("settings.ai.keys.title") }}</h4>
          <p class="ai-card-desc">{{ t("settings.ai.keys.desc") }}</p>
        </div>
      </div>
      <p v-if="!keys.length" class="text-xs text-ink-muted">{{ t("settings.ai.keys.empty") }}</p>
      <div v-else class="space-y-1.5">
        <div
          v-for="entry in keys"
          :key="entry.origin"
          class="flex flex-wrap items-center gap-2 p-2 rounded-xl bg-canvas border border-line"
        >
          <div class="min-w-0 flex-1">
            <div class="text-[11px] font-mono text-ink break-all">{{ entry.origin }}</div>
            <div class="text-[10px] text-ink-muted font-mono">
              {{ entry.fingerprint || "—" }}
              <span v-if="!entry.available" class="text-warning"> · {{ t("settings.ai.keys.unavailable") }}</span>
            </div>
          </div>
          <span v-if="entry.current" class="px-1.5 py-0.5 rounded bg-accent/15 text-accent text-[9px] font-bold">
            {{ t("settings.ai.keys.current") }}
          </span>
          <button
            type="button"
            class="p-1.5 rounded-lg text-ink-muted hover:text-danger hover:bg-danger/10"
            :title="t('settings.ai.keys.delete')"
            :aria-label="t('settings.ai.keys.delete')"
            @click="removeKey(entry.origin)"
          >
            <Trash2 class="w-3.5 h-3.5" aria-hidden="true" />
          </button>
        </div>
      </div>
      <div class="grid grid-cols-1 sm:grid-cols-[1fr_1fr_auto] gap-2 items-end">
        <label class="space-y-1">
          <span class="ai-field-label">{{ t("settings.ai.keys.origin") }}</span>
          <input v-model="keyForm.origin" type="url" placeholder="https://api.example.com/v1" class="ai-input" />
        </label>
        <label class="space-y-1">
          <span class="ai-field-label">{{ t("settings.ai.keys.key") }}</span>
          <input v-model="keyForm.apiKey" type="password" autocomplete="off" class="ai-input" />
        </label>
        <UiButton variant="secondary" :disabled="isAddingKey || !keyForm.origin.trim() || !keyForm.apiKey.trim()" @click="addKey">
          <Plus class="w-3.5 h-3.5" aria-hidden="true" />
          {{ t("settings.ai.keys.add") }}
        </UiButton>
      </div>
      <p class="ai-card-desc">{{ t("settings.ai.keys.hint") }}</p>
    </div>
  </div>
</template>
