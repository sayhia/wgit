<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from "vue";
import {
  Sparkles,
  Bot,
  Check,
  Copy,
  RefreshCw,
  Settings,
  FileCheck2,
  AlertTriangle,
} from "lucide-vue-next";
import UiModal from "../ui/UiModal.vue";
import UiButton from "../ui/UiButton.vue";
import AiReasoningPanel from "../ui/AiReasoningPanel.vue";
import { t } from "../../i18n";
import {
  conflictBranchSummaries,
  heuristicConflictAdvice,
  parseConflictAdvice,
  type AiConflictAdvice,
} from "../../services/localAiService";
import { getAIConfig, type AiConfigNoSecret } from "../../services/aiService";
import { startAIStream, type AIStreamSession } from "../../services/aiStreamService";

const props = withDefaults(
  defineProps<{
    isOpen: boolean;
    filePath: string;
    blockIndex: number;
    blockTotal: number;
    reloadNonce: number;
    applying: boolean;
    ours: string;
    theirs: string;
    base?: string;
  }>(),
  { base: "", applying: false }
);

const emit = defineEmits<{
  (e: "close"): void;
  (
    e: "apply",
    payload: {
      blockIndex: number;
      content: string;
      expectedOurs: string;
      expectedTheirs: string;
      expectedBase: string;
    }
  ): void;
  (e: "open-settings"): void;
}>();

const isLoading = ref(false);
const advice = ref<AiConflictAdvice | null>(null);
const reasoning = ref("");
const editedCode = ref("");
const copied = ref(false);
const config = ref<AiConfigNoSecret | null>(null);
const errorMessage = ref("");
let requestSeq = 0;
let adviceSession: AIStreamSession | null = null;

function stopAdviceStream(): void {
  const session = adviceSession;
  adviceSession = null;
  if (session) void session.cancel().finally(() => session.dispose());
}

onUnmounted(stopAdviceStream);

const modalTitle = computed(
  () => `${t("workspace.conflict.ai.title")} · ${t("workspace.conflict.ai.block.status", {
    current: props.blockIndex + 1,
    total: props.blockTotal,
  })}`
);

const engineLabel = computed(() => {
  const cfg = config.value;
  if (!cfg) return t("workspace.conflict.ai.engine.unknown");
  if (cfg.mode === "cli") return cfg.cliCommand || t("settings.localai.mode.cli");
  return cfg.model || cfg.baseUrl || t("workspace.conflict.ai.engine.unknown");
});

function resetState(): void {
  requestSeq++;
  stopAdviceStream();
  isLoading.value = false;
  advice.value = null;
  editedCode.value = "";
  reasoning.value = "";
  copied.value = false;
  errorMessage.value = "";
}

async function fetchAdvice(): Promise<void> {
  if (!props.isOpen) return;
  const seq = ++requestSeq;
  const blockIndex = props.blockIndex;
  isLoading.value = true;
  advice.value = null;
  editedCode.value = "";
  reasoning.value = "";
  errorMessage.value = "";

  try {
    config.value = await getAIConfig();
  } catch {
    config.value = null;
  }

  try {
    stopAdviceStream();
    const session = await startAIStream(
      {
        kind: "conflict",
        ours: props.ours,
        theirs: props.theirs,
        base: props.base,
      },
      {
        onDelta: (delta) => {
          if (seq === requestSeq) editedCode.value += delta;
        },
        onReasoning: (delta) => {
          if (seq === requestSeq) reasoning.value += delta;
        },
      }
    );
    adviceSession = session;
    if (seq !== requestSeq || !props.isOpen) {
      adviceSession = null;
      await session.cancel();
      session.dispose();
      return;
    }
    const outcome = await session.completion;
    adviceSession = null;
    if (seq !== requestSeq || !props.isOpen || props.blockIndex !== blockIndex) return;
    if (outcome.kind === "done") {
      const parsed = parseConflictAdvice(outcome.text);
      if (!parsed.recommendedResolution.trim()) {
        errorMessage.value = t("workspace.conflict.ai.empty.result");
        return;
      }
      advice.value = {
        ...conflictBranchSummaries(props.ours, props.theirs),
        recommendedResolution: parsed.recommendedResolution,
        explanation: parsed.explanation,
        source: "ai",
      };
      editedCode.value = parsed.recommendedResolution;
    } else if (outcome.kind === "cancelled") {
      advice.value = null;
      editedCode.value = "";
    } else {
      errorMessage.value = outcome.message;
      editedCode.value = "";
    }
  } catch (err) {
    if (seq !== requestSeq || !props.isOpen || props.blockIndex !== blockIndex) return;
    editedCode.value = "";
    errorMessage.value = err instanceof Error ? err.message : String(err);
  } finally {
    if (seq === requestSeq) isLoading.value = false;
  }
}

async function cancelAdviceStream(): Promise<void> {
  const session = adviceSession;
  if (!session) return;
  await session.cancel();
}

function useHeuristic(): void {
  requestSeq++;
  stopAdviceStream();
  const res = heuristicConflictAdvice({ ours: props.ours, theirs: props.theirs });
  advice.value = res;
  editedCode.value = res.recommendedResolution;
  errorMessage.value = "";
  isLoading.value = false;
}

watch(
  () => [props.isOpen, props.reloadNonce, props.blockIndex] as const,
  ([open]) => {
    if (open) {
      void fetchAdvice();
    } else {
      resetState();
    }
  },
  { immediate: true }
);

function handleApply(): void {
  if (isLoading.value || props.applying || !advice.value || !editedCode.value.trim()) return;
  emit("apply", {
    blockIndex: props.blockIndex,
    content: editedCode.value,
    expectedOurs: props.ours,
    expectedTheirs: props.theirs,
    expectedBase: props.base,
  });
}

async function copyCode(): Promise<void> {
  if (!editedCode.value) return;
  try {
    await navigator.clipboard.writeText(editedCode.value);
    copied.value = true;
    setTimeout(() => {
      copied.value = false;
    }, 2000);
  } catch {
    /* clipboard unavailable */
  }
}
</script>

<template>
  <UiModal
    :open="isOpen"
    :title="modalTitle"
    width="max-w-2xl"
    @update:open="emit('close')"
  >
    <div class="space-y-4">
      <!-- AI Engine & Model Info Pill -->
      <div class="flex items-center justify-between gap-2 p-2.5 rounded-xl bg-surface border border-line text-xs">
        <div class="flex items-center gap-2 min-w-0">
          <div class="w-6 h-6 rounded-lg bg-accent/15 flex items-center justify-center text-accent shrink-0">
            <Sparkles class="w-3.5 h-3.5" />
          </div>
          <div class="min-w-0 truncate">
            <span class="text-ink font-semibold mr-1.5">{{ t("workspace.conflict.ai.engine") }}:</span>
            <span class="text-accent font-mono text-[11px]">{{ engineLabel }}</span>
            <span v-if="config?.mode !== 'cli' && config?.baseUrl" class="text-ink-muted text-[10px] ml-1.5">({{ config.baseUrl }})</span>
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

      <AiReasoningPanel :text="reasoning" />

      <!-- Loading State -->
      <div v-if="isLoading" class="space-y-3">
        <div class="py-6 flex flex-col items-center justify-center gap-3 text-ink-muted">
          <RefreshCw class="w-6 h-6 animate-spin text-accent" />
          <span class="text-xs font-medium">{{ t("workspace.conflict.ai.analyzing") }}</span>
        </div>
        <pre
          v-if="editedCode"
          class="max-h-48 overflow-y-auto p-2.5 rounded-xl bg-canvas border border-line text-xs font-mono text-ink whitespace-pre-wrap select-text"
        >{{ editedCode }}</pre>
      </div>

      <!-- Explicit error state: never leave a blank modal with an enabled Apply. -->
      <div
        v-else-if="errorMessage"
        class="p-4 rounded-xl border border-danger/30 bg-danger/10 text-danger space-y-3"
        role="alert"
      >
        <div class="flex items-start gap-2">
          <AlertTriangle class="w-4 h-4 shrink-0 mt-0.5" />
          <div class="min-w-0 space-y-1">
            <p class="text-[11px] font-semibold leading-relaxed">{{ t("workspace.conflict.ai.error") }}</p>
            <p class="text-[10px] font-mono opacity-80 break-all">{{ errorMessage }}</p>
          </div>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <UiButton variant="secondary" @click="fetchAdvice">{{ t("workspace.conflict.ai.retry") }}</UiButton>
          <UiButton variant="secondary" @click="emit('open-settings')">{{ t("workspace.conflict.ai.settings") }}</UiButton>
          <UiButton variant="secondary" @click="useHeuristic">{{ t("workspace.conflict.ai.use.heuristic") }}</UiButton>
        </div>
      </div>

      <template v-else-if="advice">
        <!-- Branch Comparison Side Cards -->
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 text-xs">
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
          <div class="flex items-center justify-between gap-2 text-xs">
            <div class="flex items-center gap-1.5 text-ink font-semibold min-w-0">
              <Bot class="w-3.5 h-3.5 text-accent shrink-0" />
              <span class="truncate">{{ t("workspace.conflict.ai.recommended") }}</span>
              <span
                class="px-1.5 py-0.5 rounded text-[9px] font-mono font-bold border shrink-0"
                :class="advice.source === 'heuristic'
                  ? 'bg-warning/15 text-warning border-warning/30'
                  : 'bg-accent/15 text-accent border-accent/30'"
              >
                {{ advice.source === "heuristic" ? t("workspace.conflict.ai.source.heuristic") : t("workspace.conflict.ai.source.ai") }}
              </span>
            </div>
            <button
              type="button"
              @click="copyCode"
              class="flex items-center gap-1 text-[10px] text-ink-muted hover:text-ink transition cursor-pointer shrink-0"
            >
              <Check v-if="copied" class="w-3 h-3 text-success" />
              <Copy v-else class="w-3 h-3" />
              <span>{{ copied ? t("ws.copy.done") : t("ws.copy") }}</span>
            </button>
          </div>

          <p
            v-if="advice.source === 'heuristic'"
            class="text-[10px] text-warning leading-relaxed"
          >
            {{ t("workspace.conflict.ai.heuristic.notice") }}
          </p>

          <label for="ai-conflict-resolution" class="sr-only">{{ t("workspace.conflict.ai.resolution.label") }}</label>
          <textarea
            id="ai-conflict-resolution"
            v-model="editedCode"
            rows="6"
            class="w-full p-2.5 rounded-xl bg-canvas border border-line text-xs font-mono text-ink focus:outline-none focus:border-accent transition resize-y leading-relaxed"
          ></textarea>

          <p v-if="advice.explanation" class="text-[10px] text-ink-muted leading-relaxed">
            {{ advice.explanation }}
          </p>
        </div>
      </template>

      <div v-else class="py-10 flex flex-col items-center gap-3 text-ink-muted">
        <span class="text-xs">{{ t("workspace.conflict.ai.no.advice") }}</span>
        <UiButton variant="secondary" @click="fetchAdvice">{{ t("workspace.conflict.ai.retry") }}</UiButton>
      </div>
    </div>

    <template #footer>
      <UiButton variant="secondary" @click="isLoading ? cancelAdviceStream() : emit('close')">
        {{ isLoading ? t("ai.stream.cancel") : t("common.cancel") }}
      </UiButton>
      <UiButton
        variant="primary"
        :disabled="isLoading || applying || !advice || !editedCode.trim()"
        @click="handleApply"
      >
        <RefreshCw v-if="applying" class="w-3.5 h-3.5 animate-spin" />
        <FileCheck2 v-else class="w-3.5 h-3.5" />
        <span>{{ applying ? t("common.loading") : t("workspace.conflict.ai.apply.block", { block: blockIndex + 1 }) }}</span>
      </UiButton>
    </template>
  </UiModal>
</template>
