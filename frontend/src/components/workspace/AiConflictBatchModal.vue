<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from "vue";
import { AlertTriangle, CheckCircle2, RefreshCw, Sparkles } from "lucide-vue-next";
import UiModal from "../ui/UiModal.vue";
import UiButton from "../ui/UiButton.vue";
import AiReasoningPanel from "../ui/AiReasoningPanel.vue";
import { t } from "../../i18n";
import {
  parseConflictAdvice,
  type ParsedConflictBlock,
} from "../../services/localAiService";
import { startAIStream, type AIStreamSession } from "../../services/aiStreamService";
import { applyConflictSuggestionByContent } from "../../utils/conflictApply";

type SuggestionStatus = "pending" | "streaming" | "done" | "error" | "applied";

export interface BatchTarget {
  filePath: string;
  blocks: ParsedConflictBlock[];
}

interface BatchSuggestion {
  blockIndex: number;
  filePath: string;
  ours: string;
  theirs: string;
  base: string;
  status: SuggestionStatus;
  text: string;
  reasoning: string;
  explanation: string;
  error: string;
}

const props = defineProps<{
  isOpen: boolean;
  repoPath?: string;
  filePath?: string;
  blocks: ParsedConflictBlock[];
  targets?: BatchTarget[];
}>();

const emit = defineEmits<{
  (e: "close"): void;
  (e: "refresh", filePath: string): void;
  (e: "notify", message: string): void;
}>();

const suggestions = ref<BatchSuggestion[]>([]);
const isRunning = ref(false);
const currentIndex = ref(-1);
let session: AIStreamSession | null = null;
let runSeq = 0;

const hasSuggestions = computed(() => suggestions.value.length > 0);
const targetList = computed<BatchTarget[]>(() => {
  if (props.targets && props.targets.length > 0) return props.targets;
  if (props.filePath) return [{ filePath: props.filePath, blocks: props.blocks }];
  return [];
});
const targetFileCount = computed(() => targetList.value.filter((target) => target.blocks.length > 0).length);

function resetSuggestions(): void {
  suggestions.value = targetList.value.flatMap((target) =>
    target.blocks.map((block, index) => ({
      blockIndex: index,
      filePath: target.filePath,
      ours: block.ours,
      theirs: block.theirs,
      base: block.base ?? "",
      status: "pending" as SuggestionStatus,
      text: "",
      reasoning: "",
      explanation: "",
      error: "",
    }))
  );
}

function stopRun(): void {
  runSeq++;
  const active = session;
  session = null;
  if (active) void active.cancel().finally(() => active.dispose());
  isRunning.value = false;
  currentIndex.value = -1;
  for (const item of suggestions.value) {
    if (item.status === "streaming") item.status = "pending";
  }
}

async function runAll(): Promise<void> {
  if (!props.isOpen || !props.repoPath || targetList.value.length === 0) return;
  stopRun();
  const seq = ++runSeq;
  resetSuggestions();
  isRunning.value = true;
  try {
    for (const item of suggestions.value) {
      if (seq !== runSeq) return;
      item.status = "streaming";
      currentIndex.value = item.blockIndex;
      item.text = "";
      item.reasoning = "";
      item.error = "";
      try {
        const next = await startAIStream(
          {
            kind: "conflict",
            ours: item.ours,
            theirs: item.theirs,
            base: item.base,
          },
          {
            onDelta: (delta) => {
              if (seq === runSeq) item.text += delta;
            },
            onReasoning: (delta) => {
              if (seq === runSeq) item.reasoning += delta;
            },
          }
        );
        session = next;
        const outcome = await next.completion;
        session = null;
        if (seq !== runSeq) return;
        if (outcome.kind === "done") {
          const parsed = parseConflictAdvice(outcome.text);
          item.text = parsed.recommendedResolution;
          item.explanation = parsed.explanation;
          item.status = item.text.trim() ? "done" : "error";
          if (!item.text.trim()) item.error = t("workspace.conflict.ai.empty.result");
        } else if (outcome.kind === "cancelled") {
          item.status = "pending";
          return;
        } else {
          item.status = "error";
          item.error = outcome.message;
        }
      } catch (err) {
        item.status = "error";
        item.error = err instanceof Error ? err.message : String(err);
      }
    }
  } finally {
    if (seq === runSeq) {
      isRunning.value = false;
      currentIndex.value = -1;
    }
  }
}

function stop(): void {
  stopRun();
}

async function applySuggestion(item: BatchSuggestion): Promise<void> {
  if (!props.repoPath || !item.filePath || item.status !== "done") return;
  try {
    const res = await applyConflictSuggestionByContent(props.repoPath, item.filePath, {
      content: item.text,
      expectedOurs: item.ours,
      expectedTheirs: item.theirs,
      expectedBase: item.base,
    });
    if (!res) {
      item.status = "error";
      item.error = t("workspace.conflict.ai.stale");
      return;
    }
    if (res.stale) {
      item.status = "error";
      item.error = t("workspace.conflict.ai.stale");
      return;
    }
    if (!res.success) {
      item.status = "error";
      item.error = res.message;
      return;
    }
    item.status = "applied";
    emit("notify", t("workspace.conflict.batch.applied", { block: item.blockIndex + 1 }));
    emit("refresh", item.filePath);
  } catch (err) {
    item.status = "error";
    item.error = err instanceof Error ? err.message : String(err);
  }
}

watch(
  () => [props.isOpen, props.filePath, props.targets] as const,
  ([open]) => {
    if (open) void runAll();
    else stopRun();
  },
  { immediate: true }
);

onUnmounted(stopRun);
</script>

<template>
  <UiModal
    :open="isOpen"
    :title="t('workspace.conflict.batch.title')"
    width="max-w-3xl"
    @update:open="emit('close')"
  >
    <div class="space-y-3">
      <div class="p-2.5 rounded-xl border border-line bg-surface flex items-start gap-2 text-xs">
        <Sparkles class="w-4 h-4 text-accent shrink-0 mt-0.5" />
        <div class="min-w-0">
          <p class="text-[11px] font-semibold text-ink">{{ t("workspace.conflict.batch.desc") }}</p>
          <p class="text-[10px] text-ink-muted font-mono truncate mt-0.5">
            {{ filePath || t("workspace.conflict.batch.files", { count: targetFileCount }) }}
          </p>
        </div>
      </div>

      <div v-if="!hasSuggestions" class="py-10 text-center text-xs text-ink-muted">
        {{ t("workspace.conflict.ai.no.blocks") }}
      </div>

      <div v-else class="space-y-2 max-h-[55vh] overflow-y-auto pr-1">
        <div
          v-for="item in suggestions"
          :key="item.filePath + ':' + item.blockIndex"
          class="rounded-xl border border-line bg-canvas/40 p-2.5 space-y-1.5"
        >
          <div class="flex flex-wrap items-center gap-2 text-[11px]">
            <span class="font-bold text-ink">{{ t("workspace.conflict.batch.block", { block: item.blockIndex + 1 }) }}</span>
            <span v-if="!filePath" class="text-[10px] text-ink-muted font-mono truncate max-w-[260px]">{{ item.filePath }}</span>
            <span
              class="px-1.5 py-0.5 rounded text-[9px] font-bold border"
              :class="{
                'bg-canvas text-ink-muted border-line': item.status === 'pending',
                'bg-accent/15 text-accent border-accent/30': item.status === 'streaming',
                'bg-success/15 text-success border-success/30': item.status === 'done' || item.status === 'applied',
                'bg-danger/15 text-danger border-danger/30': item.status === 'error',
              }"
            >
              {{ t(`workspace.conflict.batch.status.${item.status}`) }}
            </span>
            <span class="text-ink-muted">{{ t("workspace.conflict.batch.stats", { ours: item.ours.split("\n").length, theirs: item.theirs.split("\n").length }) }}</span>
            <UiButton
              v-if="item.status === 'done'"
              variant="secondary"
              @click="applySuggestion(item)"
            >
              <CheckCircle2 class="w-3.5 h-3.5" />
              {{ t("workspace.conflict.batch.apply") }}
            </UiButton>
          </div>
          <AiReasoningPanel :text="item.reasoning" />
          <pre v-if="item.text" class="max-h-28 overflow-y-auto p-2 rounded-lg bg-canvas border border-line text-[10px] font-mono text-ink whitespace-pre-wrap select-text">{{ item.text }}</pre>
          <div v-if="item.error" class="flex items-start gap-1 text-[10px] text-danger">
            <AlertTriangle class="w-3 h-3 shrink-0 mt-0.5" />
            <span class="break-all">{{ item.error }}</span>
          </div>
          <p v-if="item.explanation" class="text-[10px] text-ink-muted leading-relaxed line-clamp-2">{{ item.explanation }}</p>
        </div>
      </div>
    </div>

    <template #footer>
      <UiButton variant="secondary" @click="isRunning ? stop() : emit('close')">
        {{ isRunning ? t("ai.stream.cancel") : t("common.cancel") }}
      </UiButton>
      <UiButton variant="primary" :disabled="isRunning || !hasSuggestions" @click="runAll">
        <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isRunning }" />
        {{ t("workspace.conflict.batch.regenerate") }}
      </UiButton>
    </template>
  </UiModal>
</template>
