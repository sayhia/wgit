<script setup lang="ts">
/**
 * AiTransferActions — portable AI configuration export/import.
 *
 * Export copies a sanitized bundle (no keys, fingerprints or repository paths)
 * to the clipboard. Import validates the file, asks for confirmation, writes
 * the connection and advanced records through the existing RPCs, and preserves
 * machine-local repository overrides. Files that carry secret fields are
 * rejected by the parser before anything is written.
 */
import { onMounted, ref, watch } from "vue";
import { Download, Upload } from "lucide-vue-next";
import SettingsCard from "./SettingsCard.vue";
import UiButton from "../ui/UiButton.vue";
import { t } from "../../i18n";
import { pushToast } from "../../composables/useToast";
import { confirmAction } from "../../composables/useConfirm";
import {
  getAIAdvancedConfig,
  getAIConfig,
  saveAIAdvancedConfig,
  saveAISettings,
  type AIAdvancedConfig,
  type AiConfigNoSecret,
} from "../../services/aiService";
import {
  buildAIConfigBundle,
  parseAIConfigBundle,
  serializeAIConfigBundle,
  toAdvancedSaveInput,
  toAISaveInput,
} from "../../utils/aiConfigTransfer";

const props = withDefaults(defineProps<{ refreshKey?: number }>(), { refreshKey: 0 });
const emit = defineEmits<{ (e: "changed"): void }>();

const config = ref<AiConfigNoSecret | null>(null);
const advanced = ref<AIAdvancedConfig | null>(null);
const importInput = ref<HTMLInputElement | null>(null);
const isTransferring = ref(false);
const loadError = ref("");

const errorText = (err: unknown): string => (err instanceof Error ? err.message : String(err));

async function loadTransferState(): Promise<void> {
  try {
  const [configResult, advancedResult] = await Promise.all([
  getAIConfig(),
  getAIAdvancedConfig(),
    ]);
    config.value = configResult;
    advanced.value = advancedResult;
    loadError.value = "";
  } catch (err) {
    // Keep the last good copy: export stays available only when both records
    // loaded at least once, and the alert names the failure.
    loadError.value = errorText(err);
  }
}

async function exportConfig(): Promise<void> {
  if (isTransferring.value || !config.value || !advanced.value) return;
  isTransferring.value = true;
  try {
    const bundle = buildAIConfigBundle(config.value, advanced.value);
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
    await loadTransferState();
    pushToast(t("settings.ai.transfer.imported"), { tone: "success" });
    emit("changed");
  } catch (err) {
    pushToast(t("settings.ai.transfer.importfailed", { error: errorText(err) }), { tone: "danger" });
  } finally {
    isTransferring.value = false;
  }
}

watch(
  () => props.refreshKey,
  (next, previous) => {
    if (next !== previous) void loadTransferState();
  }
);

onMounted(loadTransferState);
</script>

<template>
  <div class="space-y-4">
    <div
      v-if="loadError"
      data-testid="ai-transfer-error"
      role="alert"
      class="flex flex-wrap items-center gap-2 p-2.5 rounded-xl border border-danger/30 bg-danger-bg text-[11px] text-danger"
    >
      <span class="min-w-0 flex-1 break-words">
        {{ t("settings.ai.control.loadfailed", { error: loadError }) }}
      </span>
      <UiButton variant="secondary" data-testid="ai-transfer-retry" @click="loadTransferState">
        {{ t("common.retry") }}
      </UiButton>
    </div>

    <SettingsCard data-testid="ai-transfer-actions">
      <template #header>
        <div>
          <h3 class="ai-card-title">{{ t("settings.ai.transfer.title") }}</h3>
          <p class="ai-card-desc">{{ t("settings.ai.transfer.desc") }}</p>
        </div>
      </template>
      <div class="flex flex-wrap items-center justify-end gap-1.5">
        <UiButton
  variant="secondary"
          :disabled="isTransferring || !config || !advanced"
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
      </div>
    </SettingsCard>
  </div>
</template>
