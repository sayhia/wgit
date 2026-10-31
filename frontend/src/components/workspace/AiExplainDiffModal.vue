<script setup lang="ts">
import { onUnmounted, ref, watch } from "vue";
import { AlertTriangle, RefreshCw, Sparkles } from "lucide-vue-next";
import UiModal from "../ui/UiModal.vue";
import UiButton from "../ui/UiButton.vue";
import AiReasoningPanel from "../ui/AiReasoningPanel.vue";
import { t } from "../../i18n";
import { startAIStream, type AIStreamSession } from "../../services/aiStreamService";

const props = defineProps<{
  isOpen: boolean;
  repoPath?: string;
  filePath?: string;
  diff: string;
}>();

const emit = defineEmits<{ (e: "close"): void }>();

const isLoading = ref(false);
const text = ref("");
const reasoning = ref("");
const errorMessage = ref("");
let session: AIStreamSession | null = null;
let requestSeq = 0;

function stop(): void {
  requestSeq++;
  const current = session;
  session = null;
  if (current) void current.cancel().finally(() => current.dispose());
  isLoading.value = false;
}

async function fetchExplain(): Promise<void> {
  if (!props.isOpen) return;
  stop();
  const seq = ++requestSeq;
  isLoading.value = true;
  text.value = "";
  reasoning.value = "";
  errorMessage.value = "";
  try {
    const next = await startAIStream(
      { kind: "explain", repoPath: props.repoPath, filePath: props.filePath, diff: props.diff },
      {
        onDelta: (delta) => { if (seq === requestSeq) text.value += delta; },
        onReasoning: (delta) => { if (seq === requestSeq) reasoning.value += delta; },
      }
    );
    session = next;
    const outcome = await next.completion;
    session = null;
    if (seq !== requestSeq) return;
    if (outcome.kind === "done") {
      text.value = outcome.text;
    } else if (outcome.kind === "cancelled") {
      text.value = "";
    } else {
      text.value = "";
      errorMessage.value = outcome.message;
    }
  } catch (err) {
    if (seq !== requestSeq) return;
    errorMessage.value = err instanceof Error ? err.message : String(err);
  } finally {
    if (seq === requestSeq) isLoading.value = false;
  }
}

watch(
  () => [props.isOpen, props.diff] as const,
  ([open]) => {
    if (open) void fetchExplain();
    else stop();
  },
  { immediate: true }
);

onUnmounted(stop);
</script>

<template>
  <UiModal
    :open="isOpen"
    :title="t('workspace.diff.ai.explain.title')"
    width="max-w-2xl"
    @update:open="emit('close')"
  >
    <div class="space-y-3">
      <div class="flex items-start gap-2 p-2.5 rounded-xl border border-line bg-surface text-xs">
        <Sparkles class="w-4 h-4 text-accent shrink-0 mt-0.5" />
        <div class="min-w-0">
          <p class="text-[11px] font-semibold text-ink">{{ t("workspace.diff.ai.explain.hint") }}</p>
          <p v-if="filePath" class="text-[10px] text-ink-muted font-mono truncate mt-0.5">
            {{ t("workspace.diff.ai.explain.file") }}: {{ filePath }}
          </p>
        </div>
      </div>

      <div v-if="errorMessage" class="p-3 rounded-xl border border-danger/30 bg-danger/10 text-danger" role="alert">
        <div class="flex items-start gap-2">
          <AlertTriangle class="w-4 h-4 shrink-0 mt-0.5" />
          <div class="min-w-0">
            <p class="text-[11px] font-semibold">{{ t("workspace.diff.ai.explain.error") }}</p>
            <p class="text-[10px] font-mono opacity-80 mt-0.5 break-all">{{ errorMessage }}</p>
          </div>
        </div>
      </div>

      <div v-else-if="isLoading || text" class="space-y-1.5">
        <div class="flex items-center gap-2 text-[10px] text-ink-muted">
          <RefreshCw v-if="isLoading" class="w-3 h-3 animate-spin text-accent" />
          <span>{{ isLoading ? t("workspace.diff.ai.explain.loading") : t("workspace.diff.ai.explain.title") }}</span>
        </div>
        <AiReasoningPanel :text="reasoning" />
        <pre class="max-h-[55vh] overflow-y-auto p-3 rounded-xl bg-canvas border border-line text-xs text-ink whitespace-pre-wrap select-text">{{ text || t("workspace.diff.ai.explain.empty") }}</pre>
      </div>

      <div v-else class="py-10 text-center text-xs text-ink-muted">
        {{ t("workspace.diff.ai.explain.empty") }}
      </div>
    </div>

    <template #footer>
      <UiButton variant="secondary" @click="isLoading ? stop() : emit('close')">
        {{ isLoading ? t("ai.stream.cancel") : t("common.cancel") }}
      </UiButton>
      <UiButton v-if="!isLoading && errorMessage" variant="primary" @click="fetchExplain">
        {{ t("workspace.diff.ai.explain.retry") }}
      </UiButton>
    </template>
  </UiModal>
</template>
