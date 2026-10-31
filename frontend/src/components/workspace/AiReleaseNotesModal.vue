<script setup lang="ts">
import { computed, nextTick, onUnmounted, ref, watch } from "vue";
import { AlertTriangle, Copy, RefreshCw, Sparkles, Square } from "lucide-vue-next";
import UiModal from "../ui/UiModal.vue";
import UiButton from "../ui/UiButton.vue";
import AiReasoningPanel from "../ui/AiReasoningPanel.vue";
import { t } from "../../i18n";
import { pushToast } from "../../composables/useToast";
import { startAIStream, type AIStreamSession } from "../../services/aiStreamService";

const props = defineProps<{
  isOpen: boolean;
  repoPath?: string;
}>();

const emit = defineEmits<{ (e: "close"): void }>();

const fromRef = ref("");
const toRef = ref("");
const maxCommits = ref(30);
const result = ref("");
const reasoning = ref("");
const error = ref("");
const isRunning = ref(false);
const scrollArea = ref<HTMLElement | null>(null);
let session: AIStreamSession | null = null;
let runSeq = 0;

const canRun = computed(() => Boolean(props.repoPath) && !isRunning.value);

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
        kind: "release",
        repoPath: props.repoPath,
        from: fromRef.value.trim(),
        to: toRef.value.trim(),
        maxCommits: Number(maxCommits.value) || 30,
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
    pushToast(t("ai.release.copied"), { tone: "success" });
  } catch {
    pushToast(t("ai.release.copied"), { tone: "danger" });
  }
}

watch(
  () => props.isOpen,
  (open) => {
    if (!open) stop();
  }
);

onUnmounted(stop);
</script>

<template>
  <UiModal :open="isOpen" :title="t('ai.release.title')" width="max-w-3xl" @update:open="emit('close')">
    <div class="space-y-3" data-testid="ai-release-modal">
      <div class="flex items-start gap-2 p-2.5 rounded-xl border border-line bg-canvas text-[11px]">
        <Sparkles class="w-4 h-4 text-accent shrink-0 mt-0.5" />
        <div class="min-w-0">
          <p class="font-semibold text-ink">{{ t("ai.release.desc") }}</p>
          <p class="text-[10px] text-ink-muted">{{ t("ai.release.hint") }}</p>
        </div>
      </div>

      <div class="grid grid-cols-1 sm:grid-cols-3 gap-2">
        <label class="space-y-1">
          <span class="text-[10px] font-semibold text-ink-muted">{{ t("ai.release.from") }}</span>
          <input v-model="fromRef" type="text" placeholder="v1.2.0" class="w-full px-3 py-2 rounded-xl bg-canvas border border-line text-xs font-mono text-ink placeholder-ink-muted focus:outline-none focus:border-accent" />
        </label>
        <label class="space-y-1">
          <span class="text-[10px] font-semibold text-ink-muted">{{ t("ai.release.to") }}</span>
          <input v-model="toRef" type="text" placeholder="HEAD" class="w-full px-3 py-2 rounded-xl bg-canvas border border-line text-xs font-mono text-ink placeholder-ink-muted focus:outline-none focus:border-accent" />
        </label>
        <label class="space-y-1">
          <span class="text-[10px] font-semibold text-ink-muted">{{ t("ai.release.count") }}</span>
          <input v-model.number="maxCommits" type="number" min="1" max="100" class="w-full px-3 py-2 rounded-xl bg-canvas border border-line text-xs text-ink focus:outline-none focus:border-accent" />
        </label>
      </div>

      <AiReasoningPanel :text="reasoning" />

      <div ref="scrollArea" class="max-h-[48vh] min-h-[180px] overflow-y-auto rounded-xl border border-line bg-canvas p-3">
        <p v-if="!result && !error" class="py-10 text-center text-xs text-ink-muted">
          {{ isRunning ? t("ai.summarizing") : t("ai.release.empty") }}
        </p>
        <pre v-if="result" class="text-xs text-ink leading-relaxed whitespace-pre-wrap break-words select-text font-sans">{{ result }}</pre>
        <div v-if="error" class="flex items-start gap-1.5 text-xs text-danger" role="alert">
          <AlertTriangle class="w-3.5 h-3.5 shrink-0 mt-0.5" />
          <span class="break-words">{{ t("ai.release.failed", { message: error }) }}</span>
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
        {{ t("ai.release.copy") }}
      </UiButton>
      <UiButton variant="primary" :disabled="!canRun" data-testid="ai-release-run" @click="run">
        <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isRunning }" />
        {{ result ? t("ai.release.retry") : t("ai.release.run") }}
      </UiButton>
    </template>
  </UiModal>
</template>
