<script setup lang="ts">
import { reportError } from "../../composables/useToast";
import { ref, watch } from "vue";
import {
  Sparkles,
  Bot,
  Check,
  Copy,
  ArrowRight,
  ShieldCheck,
  RefreshCw,
  Settings,
  Code,
  FileCheck2,
} from "lucide-vue-next";
import UiModal from "../ui/UiModal.vue";
import UiButton from "../ui/UiButton.vue";
import { t } from "../../i18n";
import {
  requestConflictAdvice,
  getLocalAiConfig,
  type AiConflictAdvice,
} from "../../services/localAiService";

const props = defineProps<{
  isOpen: boolean;
  filePath: string;
  ours: string;
  theirs: string;
  base?: string;
}>();

const emit = defineEmits<{
  (e: "close"): void;
  (e: "apply", resolvedText: string): void;
  (e: "open-settings"): void;
}>();

const isLoading = ref(false);
const advice = ref<AiConflictAdvice | null>(null);
const editedCode = ref("");
const copied = ref(false);

const config = ref(getLocalAiConfig());

async function fetchAdvice() {
  if (!props.isOpen) return;
  isLoading.value = true;
  advice.value = null;
  config.value = getLocalAiConfig();

  try {
    const res = await requestConflictAdvice({
      ours: props.ours,
      theirs: props.theirs,
      base: props.base,
      filePath: props.filePath,
    });
    advice.value = res;
    editedCode.value = res.recommendedResolution;
  } catch (err) {
    reportError("Failed to get conflict advice:", err);
  } finally {
    isLoading.value = false;
  }
}

watch(
  () => props.isOpen,
  (open) => {
    if (open) {
      void fetchAdvice();
    }
  },
  { immediate: true }
);

function handleApply() {
  if (!editedCode.value) return;
  emit("apply", editedCode.value);
  emit("close");
}

async function copyCode() {
  if (!editedCode.value) return;
  try {
    await navigator.clipboard.writeText(editedCode.value);
    copied.value = true;
    setTimeout(() => {
      copied.value = false;
    }, 2000);
  } catch {
    /* ignore */
  }
}
</script>

<template>
  <UiModal
    :open="isOpen"
    :title="t('workspace.conflict.ai.title')"
    width="max-w-2xl"
    @update:open="emit('close')"
  >
    <div class="space-y-4">
      <!-- AI Engine & Model Info Pill -->
      <div class="flex items-center justify-between p-2.5 rounded-xl bg-surface border border-line text-xs">
        <div class="flex items-center gap-2 min-w-0">
          <div class="w-6 h-6 rounded-lg bg-accent/15 flex items-center justify-center text-accent shrink-0">
            <Sparkles class="w-3.5 h-3.5" />
          </div>
          <div class="min-w-0 truncate">
            <span class="text-ink font-semibold mr-1.5">{{ t("workspace.conflict.ai.engine") }}:</span>
            <span class="text-accent font-mono text-[11px]">{{ config.model || "Local Llama" }}</span>
            <span class="text-ink-muted text-[10px] ml-1.5">({{ config.endpoint }})</span>
          </div>
        </div>
        <button
          type="button"
          @click="emit('open-settings')"
          class="flex items-center gap-1 text-[10px] text-ink-muted hover:text-accent font-medium transition cursor-pointer shrink-0"
          :title="t('workspace.conflict.ai.settings.hint')"
        >
          <Settings class="w-3 h-3" />
          <span>{{ t("workspace.conflict.ai.settings") }}</span>
        </button>
      </div>

      <!-- Loading State -->
      <div v-if="isLoading" class="py-12 flex flex-col items-center justify-center gap-3 text-ink-muted">
        <RefreshCw class="w-6 h-6 animate-spin text-accent" />
        <span class="text-xs font-medium">{{ t("workspace.conflict.ai.analyzing") }}</span>
      </div>

      <template v-else-if="advice">
        <!-- Branch Comparison Side Cards -->
        <div class="grid grid-cols-2 gap-3 text-xs">
          <div class="p-3 rounded-xl bg-accent/5 border border-accent/20 space-y-1">
            <div class="font-semibold text-accent text-[11px] flex items-center gap-1">
              <span>{{ t("workspace.conflict.ai.ours") }}</span>
            </div>
            <p class="text-[11px] text-ink leading-relaxed">
              {{ advice.oursSummary }}
            </p>
          </div>
          <div class="p-3 rounded-xl bg-warning/5 border border-warning/20 space-y-1">
            <div class="font-semibold text-warning text-[11px] flex items-center gap-1">
              <span>{{ t("workspace.conflict.ai.theirs") }}</span>
            </div>
            <p class="text-[11px] text-ink leading-relaxed">
              {{ advice.theirsSummary }}
            </p>
          </div>
        </div>

        <!-- Synthesized Code Resolution Block -->
        <div class="space-y-1.5">
          <div class="flex items-center justify-between text-xs">
            <div class="flex items-center gap-1.5 text-ink font-semibold">
              <Bot class="w-3.5 h-3.5 text-accent" />
              <span>{{ t("workspace.conflict.ai.recommended") }}</span>
              <span class="px-1.5 py-0.5 rounded text-[9px] font-mono font-bold bg-success/15 text-success border border-success/30">
                {{ advice.confidence }}% {{ t("workspace.conflict.ai.confidence") }}
              </span>
            </div>
            <button
              type="button"
              @click="copyCode"
              class="flex items-center gap-1 text-[10px] text-ink-muted hover:text-ink transition cursor-pointer"
            >
              <Check v-if="copied" class="w-3 h-3 text-success" />
              <Copy v-else class="w-3 h-3" />
              <span>{{ copied ? t("ws.copy.done") : t("ws.copy") }}</span>
            </button>
          </div>

          <textarea
            v-model="editedCode"
            rows="6"
            class="w-full p-2.5 rounded-xl bg-canvas border border-line text-xs font-mono text-ink focus:outline-none focus:border-accent transition resize-y leading-relaxed"
          ></textarea>

          <p class="text-[10px] text-ink-muted leading-relaxed">
            {{ advice.explanation }}
          </p>
        </div>
      </template>
    </div>

    <template #footer>
      <UiButton variant="secondary" @click="emit('close')">
        {{ t("common.cancel") }}
      </UiButton>
      <UiButton
        variant="primary"
        :disabled="isLoading || !editedCode.trim()"
        @click="handleApply"
      >
        <FileCheck2 class="w-3.5 h-3.5" />
        <span>{{ t("workspace.conflict.ai.apply") }}</span>
      </UiButton>
    </template>
  </UiModal>
</template>
