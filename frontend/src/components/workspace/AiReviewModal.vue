<script setup lang="ts">
import { computed, nextTick, onUnmounted, ref, watch } from "vue";
import { AlertTriangle, Copy, RefreshCw, ShieldCheck, Square } from "lucide-vue-next";
import UiModal from "../ui/UiModal.vue";
import UiButton from "../ui/UiButton.vue";
import AiReasoningPanel from "../ui/AiReasoningPanel.vue";
import { t } from "../../i18n";
import { pushToast } from "../../composables/useToast";
import { startAIStream, type AIStreamSession } from "../../services/aiStreamService";

const props = defineProps<{
  isOpen: boolean;
  repoPath?: string;
  commitHash?: string;
  filePath?: string;
  diff?: string;
}>();

const emit = defineEmits<{ (e: "close"): void }>();

const result = ref("");
const reasoning = ref("");
const error = ref("");
const isRunning = ref(false);
const scrollArea = ref<HTMLElement | null>(null);
let session: AIStreamSession | null = null;
let runSeq = 0;

const title = computed(() => {
  if (props.commitHash) return `${t("ai.review.title")} · ${props.commitHash.slice(0, 8)}`;
  return props.filePath ? `${t("ai.review.title")} · ${props.filePath}` : t("ai.review.title");
});

function scrollToBottom(): void {
  void nextTick(() => {
    const el = scrollArea.value;
    if (el) el.scrollTop = el.scrollHeight;
  });
}

function stop(): void {
  runSeq++;
  const active = session;
  session = null;
  if (active) void active.cancel().finally(() => active.dispose());
  isRunning.value = false;
}

async function run(): Promise<void> {
  if (!props.repoPath || isRunning.value) return;
  stop();
  const seq = ++runSeq;
  result.value = "";
  reasoning.value = "";
  error.value = "";
  isRunning.value = true;
  try {
    const next = await startAIStream(
      {
        kind: "review",
        repoPath: props.repoPath,
        commitHash: props.commitHash || "",
        filePath: props.filePath || "",
        diff: props.diff || "",
      },
      {
        onDelta: (delta) => {
          if (seq !== runSeq) return;
          result.value += delta;
          scrollToBottom();
        },
        onReasoning: (delta) => {
          if (seq !== runSeq) return;
          reasoning.value += delta;
        },
      }
    );
    session = next;
    const outcome = await next.completion;
    session = null;
    if (seq !== runSeq) return;
    if (outcome.kind === "done") {
      result.value = outcome.text;
    } else if (outcome.kind === "error") {
      error.value = outcome.message;
    }
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err);
  } finally {
    if (seq === runSeq) isRunning.value = false;
    scrollToBottom();
  }
}

async function copyResult(): Promise<void> {
  if (!result.value.trim()) return;
  try {
    await navigator.clipboard.writeText(result.value);
    pushToast(t("ai.review.copied"), { tone: "success" });
  } catch {
    pushToast(t("ai.review.copied"), { tone: "danger" });
  }
}

watch(
  () => [props.isOpen, props.commitHash, props.filePath] as const,
  ([open]) => {
    if (open) void run();
    else stop();
  },
  { immediate: true }
);

onUnmounted(stop);
</script>

<template>
  <UiModal :open="isOpen" :title="title" width="max-w-3xl" @update:open="emit('close')">
    <div class="space-y-3" data-testid="ai-review-modal">
      <div class="flex items-start gap-2 p-2.5 rounded-xl border border-line bg-canvas text-[11px]">
        <ShieldCheck class="w-4 h-4 text-accent shrink-0 mt-0.5" />
        <div class="min-w-0">
          <p class="font-semibold text-ink">{{ t("ai.review.desc") }}</p>
          <p class="text-[10px] text-ink-muted font-mono truncate mt-0.5">{{ commitHash || filePath || repoPath }}</p>
        </div>
      </div>

      <AiReasoningPanel :text="reasoning" />

      <div
        ref="scrollArea"
        class="max-h-[55vh] min-h-[200px] overflow-y-auto rounded-xl border border-line bg-canvas p-3"
      >
        <p v-if="!result && !error" class="py-10 text-center text-xs text-ink-muted">
          {{ isRunning ? t("ai.summarizing") : t("ai.review.empty") }}
        </p>
        <pre v-if="result" class="text-xs text-ink leading-relaxed whitespace-pre-wrap break-words select-text font-sans">{{ result }}</pre>
        <div v-if="error" class="flex items-start gap-1.5 text-xs text-danger" role="alert">
          <AlertTriangle class="w-3.5 h-3.5 shrink-0 mt-0.5" />
          <span class="break-words">{{ t("ai.review.failed", { message: error }) }}</span>
        </div>
      </div>
    </div>

    <template #footer>
      <UiButton v-if="isRunning" variant="secondary" @click="stop">
        <Square class="w-3.5 h-3.5" />
        {{ t("ai.stream.cancel") }}
      </UiButton>
      <UiButton v-else variant="ghost" :disabled="!result" @click="copyResult">
        <Copy class="w-3.5 h-3.5" />
        {{ t("ai.review.copy") }}
      </UiButton>
      <UiButton variant="primary" :disabled="isRunning" @click="run">
        <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isRunning }" />
        {{ t("ai.review.retry") }}
      </UiButton>
    </template>
  </UiModal>
</template>
