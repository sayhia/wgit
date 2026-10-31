<script setup lang="ts">
/**
 * AiBehaviorSection — settings leaf for AI behavior.
 *
 * Owns four surfaces that share one save action: commit generation
 * preferences (persisted through `SaveAISettings`, like the connection leaf),
 * per-task routing and reliability (advanced config), and the five staged
 * rollout flags (immediate save, with rollback on failure).
 *
 * The two cards are presentational; this section is the only reader/writer of
 * the advanced config so routing and reliability cannot diverge into separate
 * save paths. Fields owned elsewhere (pricing/budget, repository overrides,
 * profile labels, connection details) are re-read immediately before writing
 * so this surface never rolls them back.
 *
 * Loading is split per surface (advanced config, connection record, flags) so
 * one failing read only degrades its own area and reports its own alert.
 */
import { onMounted, reactive, ref, watch } from "vue";
import { Save, ShieldCheck } from "lucide-vue-next";
import SettingsCard from "./SettingsCard.vue";
import SettingsPageHeader from "./SettingsPageHeader.vue";
import UiButton from "../ui/UiButton.vue";
import AiTaskRoutingCard from "./AiTaskRoutingCard.vue";
import AiReliabilityCard from "./AiReliabilityCard.vue";
import { t } from "../../i18n";
import {
  getAIAdvancedConfig,
  getAIConfig,
  saveAIAdvancedConfig,
  saveAISettings,
  type AIAdvancedConfig,
  type AISaveInput,
  type AITaskKind,
  type AiConfigNoSecret,
} from "../../services/aiService";
import { featureFlagsApi } from "../../services/gitService";
import type { FeatureFlags } from "../../types";
import { featureFlags as appFeatureFlags } from "../../composables/useFeatureFlags";
import { pushToast } from "../../composables/useToast";

const props = withDefaults(defineProps<{ refreshKey?: number }>(), { refreshKey: 0 });
const emit = defineEmits<{ (e: "changed"): void }>();

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

const commit = reactive<{
  language: "auto" | "zh" | "en";
  includeBody: boolean;
  instructions: string;
}>({ language: "auto", includeBody: false, instructions: "" });
/** Snapshot of what the backend already holds, for the commit-prefs dirty check. */
const loadedCommit = reactive({ language: "auto" as "auto" | "zh" | "en", includeBody: false, instructions: "" });

const FLAG_DEFS: Array<{ key: keyof FeatureFlags; labelKey: string; descKey: string }> = [
  { key: "aiCommitMsg", labelKey: "settings.flags.aicommitmsg.label", descKey: "settings.flags.aicommitmsg.desc" },
  { key: "aiSummary", labelKey: "settings.flags.aisummary.label", descKey: "settings.flags.aisummary.desc" },
  { key: "aiConflictBatch", labelKey: "settings.flags.aiconflictbatch.label", descKey: "settings.flags.aiconflictbatch.desc" },
  { key: "reviewFlow", labelKey: "settings.flags.reviewflow.label", descKey: "settings.flags.reviewflow.desc" },
  { key: "assetBoard", labelKey: "settings.flags.assetboard.label", descKey: "settings.flags.assetboard.desc" },
];

const flags = reactive<FeatureFlags>({
  aiCommitMsg: true,
  aiSummary: true,
  aiConflictBatch: true,
  reviewFlow: false,
  assetBoard: false,
});
const flagsLoaded = ref(false);
const flagsLoading = ref(false);
const flagsError = ref("");
const savingFlagKey = ref("");
const isLoading = ref(false);
/** Routing/reliability could not be read; the draft is not authoritative. */
const advancedError = ref("");
/** Commit preferences could not be read; the settings write is skipped. */
const configError = ref("");
/** True once the connection record has been read at least once. */
const configLoaded = ref(false);
const isSaving = ref(false);

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

function applyCommit(next: AiConfigNoSecret | null): void {
  if (!next) return;
  commit.language =
  next.commitLanguage === "zh" || next.commitLanguage === "en"
      ? next.commitLanguage
      : "auto";
  commit.includeBody = next.commitIncludeBody === true;
  commit.instructions = next.commitInstructions || "";
  loadedCommit.language = commit.language;
  loadedCommit.includeBody = commit.includeBody;
  loadedCommit.instructions = commit.instructions.trim();
}

function commitPreferencesChanged(): boolean {
  return (
  commit.language !== loadedCommit.language ||
  commit.includeBody !== loadedCommit.includeBody ||
  commit.instructions.trim() !== loadedCommit.instructions
  );
}

function updateTaskModel(kind: AITaskKind, value: string): void {
  advanced.taskModels = { ...advanced.taskModels, [kind]: value };
}

/**
 * The three reads are independent on purpose: one failing surface must not
 * blank the others. Each records its own error and keeps the last good draft.
 */
async function loadAdvanced(): Promise<void> {
  try {
    applyAdvanced(await getAIAdvancedConfig());
    advancedError.value = "";
  } catch (err) {
    advancedError.value = errorText(err);
  }
}

async function loadCommitPreferences(): Promise<void> {
  try {
    applyCommit(await getAIConfig());
    configLoaded.value = true;
    configError.value = "";
  } catch (err) {
    configError.value = errorText(err);
  }
}

async function loadBehavior(): Promise<void> {
  isLoading.value = true;
  try {
    await Promise.all([loadAdvanced(), loadCommitPreferences()]);
  } finally {
    isLoading.value = false;
  }
}

async function loadFlags(): Promise<void> {
  flagsLoading.value = true;
  flagsError.value = "";
  try {
    const next = await featureFlagsApi.getFeatureFlags();
    Object.assign(flags, next || {});
    flagsLoaded.value = true;
  } catch (err) {
    // Never render or persist a fabricated default: the rows stay hidden behind
    // a visible alert with its own retry until a real read succeeds.
    flagsLoaded.value = false;
    flagsError.value = errorText(err);
  } finally {
    flagsLoading.value = false;
  }
}

async function loadAll(): Promise<void> {
  await Promise.all([loadBehavior(), loadFlags()]);
}

function advancedSavePayload(current: AIAdvancedConfig): AIAdvancedConfig {
  return {
    ...current,
    taskModels: { ...advanced.taskModels },
    retryCount: Number(advanced.retryCount) || 0,
    rateLimitPerMinute: Number(advanced.rateLimitPerMinute) || 0,
    fallbackBaseUrl: advanced.fallbackBaseUrl,
    fallbackModel: advanced.fallbackModel,
    fallbackProtocol: advanced.fallbackProtocol || "openai",
  };
}

function settingsSaveInput(current: AiConfigNoSecret): AISaveInput {
  return {
    mode: current.mode,
    baseUrl: current.baseUrl,
    model: current.model,
    timeoutSeconds: current.timeoutSeconds,
    cliCommand: current.cliCommand,
    cliArgs: current.cliArgs ?? [],
    cliInput: current.cliInput === "arg" ? "arg" : "stdin",
    commitLanguage: commit.language,
    commitIncludeBody: commit.includeBody,
    commitInstructions: commit.instructions.trim(),
  protocol:
  current.protocol === "azure" ||
  current.protocol === "anthropic" ||
  current.protocol === "gemini"
        ? current.protocol
        : "openai",
    azureApiVersion: current.azureApiVersion || "",
    extraHeaders: current.extraHeaders ?? {},
    cliEnvMode: current.cliEnvMode === "minimal" ? "minimal" : "inherit",
    // No apiKey: the connection editor owns the secret; an empty value keeps
    // the stored key (AISaveInput's documented keep-key contract).
  };
}

async function saveBehavior(): Promise<void> {
  if (isSaving.value) return;
  isSaving.value = true;
  let commitPersisted = false;
  try {
    // Order matters: commit preferences (the settings write) go first, and the
    // advanced config is only written once that write succeeded, so a failure
    // never leaves advanced routing/reliability persisted behind a "failed"
    // toast.
    //
    // The settings write is skipped when the record has never loaded (its
    // draft would be fabricated defaults) or when nothing changed: it would
    // otherwise be rejected on a fresh install with no connection, blocking a
    // routing-only save.
    if (configLoaded.value && commitPreferencesChanged()) {
      // Re-read the connection record so fields edited elsewhere survive.
      const currentConfig = await getAIConfig();
      await saveAISettings(settingsSaveInput(currentConfig));
      loadedCommit.language = commit.language;
      loadedCommit.includeBody = commit.includeBody;
      loadedCommit.instructions = commit.instructions.trim();
      commitPersisted = true;
    }

    // Re-read the advanced record so pricing/budget, repository overrides and
    // profile labels edited elsewhere survive this save.
    const currentAdvanced = await getAIAdvancedConfig();
    await saveAIAdvancedConfig(advancedSavePayload(currentAdvanced));

    pushToast(t("settings.localai.save.succeeded"), { tone: "success" });
    emit("changed");
  } catch (err) {
    if (commitPersisted) {
      // Commit preferences did persist; the advanced write is what failed, so
      // name that part and still tell the shell the AI settings changed.
      pushToast(t("settings.ai.control.failed", { error: errorText(err) }), {
        tone: "danger",
      });
      emit("changed");
    } else {
      pushToast(t("settings.localai.save.failed", { message: errorText(err) }), {
        tone: "danger",
      });
    }
  } finally {
    isSaving.value = false;
  }
}

async function toggleFlag(key: keyof FeatureFlags): Promise<void> {
  if (savingFlagKey.value !== "" || !flagsLoaded.value) return;
  savingFlagKey.value = key;
  const snapshot = { ...flags };
  flags[key] = !flags[key];
  try {
    await featureFlagsApi.saveAppFlags({ ...flags });
    Object.assign(appFeatureFlags, flags);
    emit("changed");
  } catch (err) {
    Object.assign(flags, snapshot);
    pushToast(t("settings.flags.save.failed", { message: errorText(err) }), {
      tone: "danger",
    });
  } finally {
    savingFlagKey.value = "";
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
  <div class="space-y-4" :aria-busy="isLoading" data-testid="ai-behavior-section">
    <SettingsPageHeader
      :title="t('settings.ai.behavior.title')"
      :description="t('settings.ai.behavior.subtitle')"
    >
      <template #actions>
        <UiButton
  variant="primary"
          :disabled="isSaving || isLoading || Boolean(advancedError)"
  data-testid="ai-behavior-save"
  @click="saveBehavior"
        >
          <Save class="w-3.5 h-3.5" aria-hidden="true" />
          {{ isSaving ? t("common.saving") : t("settings.localai.save.btn") }}
        </UiButton>
      </template>
    </SettingsPageHeader>
    <div
      v-if="advancedError"
      data-testid="ai-behavior-load-error"
      role="alert"
      class="flex flex-wrap items-center gap-2 p-2.5 rounded-xl border border-danger/30 bg-danger-bg text-[11px] text-danger"
    >
      <span class="min-w-0 flex-1 break-words">
        {{ t("settings.ai.control.loadfailed", { error: advancedError }) }}
      </span>
      <UiButton variant="secondary" data-testid="ai-behavior-retry" @click="loadAll">
        {{ t("common.retry") }}
      </UiButton>
    </div>

    <SettingsCard data-testid="commit-prefs-card">
      <template #header>
        <div>
          <h3 class="ai-card-title">{{ t("settings.localai.commit.title") }}</h3>
          <p class="ai-card-desc">{{ t("settings.localai.commit.desc") }}</p>
        </div>
      </template>

      <div class="space-y-3">
        <div
          v-if="configError"
          data-testid="ai-behavior-config-error"
          role="alert"
          class="flex flex-wrap items-center gap-2 p-2.5 rounded-xl border border-danger/30 bg-danger-bg text-[11px] text-danger"
        >
          <span class="min-w-0 flex-1 break-words">
            {{ t("ai.quick.load.failed", { error: configError }) }}
          </span>
          <UiButton
  variant="secondary"
  data-testid="ai-behavior-config-retry"
  @click="loadCommitPreferences"
          >
            {{ t("common.retry") }}
          </UiButton>
        </div>
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div class="space-y-1">
            <label class="text-xs font-semibold text-ink" for="localai-commit-language">
              {{ t("settings.localai.commit.language") }}
            </label>
            <select
              id="localai-commit-language"
              v-model="commit.language"
              data-testid="localai-commit-language"
              :disabled="!configLoaded"
              class="w-full px-3 py-2 rounded-xl bg-canvas border border-line text-xs text-ink focus:outline-none focus:border-accent disabled:opacity-50"
            >
              <option value="auto">{{ t("settings.localai.commit.auto") }}</option>
              <option value="zh">{{ t("settings.localai.commit.zh") }}</option>
              <option value="en">{{ t("settings.localai.commit.en") }}</option>
            </select>
          </div>
          <label class="flex items-start gap-2 p-2 rounded-xl border border-line bg-canvas cursor-pointer">
            <input
              v-model="commit.includeBody"
              type="checkbox"
              :disabled="!configLoaded"
              class="accent-accent scale-90 mt-0.5 disabled:opacity-50"
              data-testid="localai-commit-body"
            />
            <span class="text-[11px] leading-relaxed">
              <span class="block font-semibold text-ink">{{ t("settings.localai.commit.body") }}</span>
              <span class="block text-ink-muted">{{ t("settings.localai.commit.bodyhint") }}</span>
            </span>
          </label>
        </div>
        <div class="space-y-1">
          <label class="text-xs font-semibold text-ink" for="localai-commit-instructions">
            {{ t("settings.localai.commit.instructions") }}
          </label>
          <textarea
            id="localai-commit-instructions"
            v-model="commit.instructions"
            rows="2"
            maxlength="2000"
            data-testid="localai-commit-instructions"
            :disabled="!configLoaded"
            :placeholder="t('settings.localai.commit.instructionshint')"
            class="w-full px-3 py-2 rounded-xl bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent disabled:opacity-50"
          ></textarea>
        </div>
      </div>
    </SettingsCard>

    <AiTaskRoutingCard :task-models="advanced.taskModels" @update:task-model="updateTaskModel" />

    <AiReliabilityCard
      v-model:retry-count="advanced.retryCount"
      v-model:rate-limit-per-minute="advanced.rateLimitPerMinute"
      v-model:fallback-base-url="advanced.fallbackBaseUrl"
      v-model:fallback-model="advanced.fallbackModel"
      v-model:fallback-protocol="advanced.fallbackProtocol"
    />

    <SettingsCard data-testid="feature-flags-card">
      <template #header>
        <div class="flex items-start gap-2">
          <ShieldCheck class="w-4 h-4 mt-0.5 text-accent shrink-0" aria-hidden="true" />
          <div class="min-w-0">
            <h3 class="ai-card-title">{{ t("settings.flags.title") }}</h3>
            <p class="ai-card-desc">{{ t("settings.flags.desc") }}</p>
          </div>
        </div>
      </template>

      <div
        v-if="flagsError"
        data-testid="ai-behavior-flags-error"
        role="alert"
        class="flex flex-wrap items-center gap-2 p-2.5 rounded-xl border border-danger/30 bg-danger-bg text-[11px] text-danger"
      >
        <span class="min-w-0 flex-1 break-words">
          {{ t("settings.flags.load.failed", { message: flagsError }) }}
        </span>
        <UiButton
  variant="secondary"
  data-testid="ai-behavior-flags-retry"
          :disabled="flagsLoading"
  @click="loadFlags"
        >
          {{ t("common.retry") }}
        </UiButton>
      </div>
      <div v-else-if="!flagsLoaded" class="text-[11px] text-ink-muted">
        {{ t("common.loading") }}
      </div>
      <div v-else>
        <div
          v-for="def in FLAG_DEFS"
          :key="def.key"
          class="flex items-center justify-between gap-4 py-2 border-t border-line/60 first:border-t-0 first:pt-0"
          :data-testid="`flags-row-${def.key}`"
        >
          <div class="min-w-0">
            <div class="text-xs font-semibold text-ink">{{ t(def.labelKey) }}</div>
            <div class="text-[11px] text-ink-muted mt-0.5 leading-relaxed">{{ t(def.descKey) }}</div>
          </div>
          <button
            type="button"
            role="switch"
            :aria-checked="flags[def.key]"
            :aria-label="t(def.labelKey)"
            :data-testid="`flags-toggle-${def.key}`"
            :disabled="savingFlagKey !== '' || !flagsLoaded"
            class="relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-accent/50 disabled:opacity-40 disabled:cursor-wait"
            :class="flags[def.key] ? 'bg-accent' : 'bg-line'"
            @click="toggleFlag(def.key)"
          >
            <span
              class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow-lg ring-0 transition duration-200 ease-in-out"
              :class="flags[def.key] ? 'translate-x-5' : 'translate-x-0'"
            />
          </button>
        </div>
      </div>
    </SettingsCard>
  </div>
</template>
