<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from "vue";
import { KeyRound, Plus, Trash2 } from "lucide-vue-next";
import SettingsCard from "./SettingsCard.vue";
import UiButton from "../ui/UiButton.vue";
import { t } from "../../i18n";
import { pushToast } from "../../composables/useToast";
import { confirmAction } from "../../composables/useConfirm";
import {
  deleteAIKeyForOrigin,
  listAIKeys,
  saveAIKeyForOrigin,
  type AIKeyEntry,
} from "../../services/aiService";

const props = withDefaults(defineProps<{ refreshKey?: number }>(), { refreshKey: 0 });
const emit = defineEmits<{ (e: "changed"): void }>();

const keys = ref<AIKeyEntry[]>([]);
const isLoading = ref(false);
const loadError = ref("");
const keyForm = reactive({ origin: "", apiKey: "" });
const isAddingKey = ref(false);
const canAddKey = computed(() => Boolean(keyForm.origin.trim() && keyForm.apiKey.trim()));

const errorText = (err: unknown): string => (err instanceof Error ? err.message : String(err));

async function loadKeys(): Promise<void> {
  isLoading.value = true;
  loadError.value = "";
  try {
    keys.value = (await listAIKeys()) || [];
  } catch (err) {
    loadError.value = errorText(err);
  } finally {
    isLoading.value = false;
  }
}

async function addKey(): Promise<void> {
  if (isAddingKey.value || !canAddKey.value) return;
  isAddingKey.value = true;
  try {
    await saveAIKeyForOrigin(keyForm.origin.trim(), keyForm.apiKey);
    keyForm.origin = "";
    keyForm.apiKey = "";
    keys.value = (await listAIKeys()) || [];
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
    await loadKeys();
    pushToast(t("settings.ai.keys.deleted"), { tone: "success" });
    emit("changed");
  } catch (err) {
    pushToast(errorText(err), { tone: "danger" });
  }
}

watch(
  () => props.refreshKey,
  (next, previous) => {
    if (next !== previous) void loadKeys();
  }
);

onMounted(loadKeys);
</script>

<template>
  <SettingsCard id="ai-section-keys" :aria-busy="isLoading" data-testid="ai-keys-card">
    <template #header>
      <div class="flex items-center gap-2">
        <KeyRound class="w-4 h-4 text-accent shrink-0" />
        <div>
          <h3 class="ai-card-title">{{ t("settings.ai.keys.title") }}</h3>
          <p class="ai-card-desc">{{ t("settings.ai.keys.desc") }}</p>
        </div>
      </div>
    </template>

    <div class="space-y-3">
      <div
  v-if="loadError"
  role="alert"
  class="flex flex-wrap items-center gap-2 p-2.5 rounded-xl border border-danger/30 bg-danger-bg text-[11px] text-danger"
      >
        <span class="min-w-0 flex-1 break-words">
          {{ t("settings.ai.control.loadfailed", { error: loadError }) }}
        </span>
        <UiButton variant="secondary" data-testid="ai-keys-retry" @click="loadKeys">
          {{ t("ai.quick.load.retry") }}
        </UiButton>
      </div>

      <p v-if="isLoading && !keys.length" class="text-xs text-ink-muted" role="status">
        {{ t("settings.ai.control.loading") }}
      </p>
      <p v-else-if="!keys.length && !loadError" class="text-xs text-ink-muted">
        {{ t("settings.ai.keys.empty") }}
      </p>
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
          <span v-if="entry.current" class="px-1.5 py-0.5 rounded bg-accent-bg text-accent text-[9px] font-bold">
            {{ t("settings.ai.keys.current") }}
          </span>
          <button
            type="button"
            class="p-1.5 rounded-lg text-ink-muted hover:text-danger hover:bg-danger-bg"
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
          <input
  v-model="keyForm.origin"
  type="url"
  placeholder="https://api.example.com/v1"
  class="ai-input"
  data-testid="ai-key-origin"
          />
        </label>
        <label class="space-y-1">
          <span class="ai-field-label">{{ t("settings.ai.keys.key") }}</span>
          <input
  v-model="keyForm.apiKey"
  type="password"
  autocomplete="off"
  class="ai-input"
  data-testid="ai-key-value"
          />
        </label>
        <UiButton
  variant="secondary"
          :disabled="isAddingKey || !canAddKey"
  data-testid="ai-key-add"
  @click="addKey"
        >
          <Plus class="w-3.5 h-3.5" aria-hidden="true" />
          {{ t("settings.ai.keys.add") }}
        </UiButton>
      </div>
      <p class="ai-card-desc">{{ t("settings.ai.keys.hint") }}</p>
    </div>
  </SettingsCard>
</template>
