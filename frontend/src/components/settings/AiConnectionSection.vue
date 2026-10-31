<script setup lang="ts">
/**
 * AiConnectionSection — settings leaf for the connection surface.
 *
 * Composes the connection editor, the saved profiles card and the stored keys
 * card, plus the one-time legacy localStorage migration banner. The manual
 * status card, mode switch and connection form live in `AiConnectionForm`; this
 * leaf never duplicates them.
 *
 * The form owns its draft state, so migrating a legacy key targets the
 * persisted config (`GetAIConfig`) rather than unsaved form edits. Successful
 * migration and child changes emit `changed`; the shell bumps `refreshKey`, and
 * the same value is forwarded to every child so they reload against one
 * backend state (no second store).
 */
import { computed, onMounted, ref, watch } from "vue";
import { KeyRound } from "lucide-vue-next";
import { t } from "../../i18n";
import {
  getAIConfig,
  isSameApiOrigin,
  saveAISettings,
  type AiConfigNoSecret,
} from "../../services/aiService";
import {
  clearLegacyLocalAiKey,
  readLegacyLocalAiConfig,
  type LegacyLocalAiConfig,
} from "../../services/localAiService";
import { pushToast } from "../../composables/useToast";
import { confirmAction } from "../../composables/useConfirm";
import AiConnectionForm from "./AiConnectionForm.vue";
import AiProfilesCard from "./AiProfilesCard.vue";
import AiKeysCard from "./AiKeysCard.vue";
import SettingsPageHeader from "./SettingsPageHeader.vue";

const props = withDefaults(defineProps<{ refreshKey?: number }>(), { refreshKey: 0 });
const emit = defineEmits<{ (e: "changed"): void }>();

const legacyConfig = ref<LegacyLocalAiConfig | null>(readLegacyLocalAiConfig());
const legacyBusy = ref(false);
const savedConfig = ref<AiConfigNoSecret | null>(null);
const configLoading = ref(true);
const isCliMode = computed(() => savedConfig.value?.mode === "cli");

const errorMessage = (err: unknown): string => (err instanceof Error ? err.message : String(err));

function notifyChanged(): void {
  emit("changed");
}

async function refreshSavedConfig(): Promise<void> {
  configLoading.value = true;
  try {
    savedConfig.value = await getAIConfig();
  } catch {
    savedConfig.value = null;
  } finally {
    configLoading.value = false;
  }
}

onMounted(() => {
  void refreshSavedConfig();
});

watch(
  () => props.refreshKey,
  (next, previous) => {
    if (next !== previous) void refreshSavedConfig();
  }
);

async function importLegacyKey(): Promise<void> {
  const legacy = legacyConfig.value;
  if (!legacy || legacyBusy.value) return;
  if (isCliMode.value) {
    pushToast(t("settings.localai.legacy.climode"), { tone: "warning" });
    return;
  }
  const saved = savedConfig.value;
  // The form draft is not visible here; persist into the active endpoint,
  // falling back to whatever the legacy copy recorded.
  const targetBase = saved?.baseUrl?.trim() || legacy.endpoint.trim();
  const targetModel = saved?.model?.trim() || legacy.model.trim();
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
      timeoutSeconds: saved?.timeoutSeconds || 60,
      cliCommand: "",
      cliArgs: [],
      cliInput: "stdin",
      apiKey: legacy.apiKey,
    });
    clearLegacyLocalAiKey();
    legacyConfig.value = null;
    await refreshSavedConfig();
    emit("changed");
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
</script>

<template>
  <div class="space-y-4" data-testid="ai-connection-section">
    <SettingsPageHeader
      :title="t('settings.ai.connection.title')"
      :description="t('settings.ai.connection.subtitle')"
    />
    <AiConnectionForm :refresh-key="refreshKey" @saved="notifyChanged" />
    <AiProfilesCard :refresh-key="refreshKey" @changed="notifyChanged" />
    <AiKeysCard :refresh-key="refreshKey" @changed="notifyChanged" />

    <div
  v-if="legacyConfig"
  data-testid="localai-legacy-banner"
  class="p-3 rounded-xl border bg-warning-bg border-warning/30 text-warning text-xs flex items-start gap-2"
    >
      <KeyRound class="w-4 h-4 shrink-0 mt-0.5" />
      <div class="min-w-0 flex-1 space-y-2">
        <div class="space-y-1">
          <p class="text-[11px] font-semibold">{{ t("settings.localai.legacy.title") }}</p>
          <p class="text-[10px] leading-relaxed opacity-90">{{ t("settings.localai.legacy.desc") }}</p>
          <p class="text-[10px] leading-relaxed opacity-80">{{ t("settings.localai.legacy.target.note") }}</p>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <button
            type="button"
            data-testid="localai-legacy-import"
            :disabled="legacyBusy || isCliMode || configLoading"
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
        <p v-if="isCliMode" class="text-[10px] leading-relaxed opacity-80">
          {{ t("settings.localai.legacy.climode") }}
        </p>
      </div>
    </div>
  </div>
</template>
