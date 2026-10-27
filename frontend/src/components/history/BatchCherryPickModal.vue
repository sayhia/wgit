<script setup lang="ts">
import { ref, computed, watch } from "vue";
import {
  GitCompare,
  RefreshCw,
  ArrowUp,
  ArrowDown,
  GripVertical,
  AlertTriangle,
  CheckCircle2,
  ShieldCheck,
  ShieldAlert,
  ArrowUpDown,
  FileCode,
} from "lucide-vue-next";
import UiModal from "../ui/UiModal.vue";
import UiButton from "../ui/UiButton.vue";
import { t } from "../../i18n";
import { gitService } from "../../services/gitService";
import type { CommitItem, BatchCherryPickResult, MergeConflictPrediction } from "../../types";

const props = defineProps<{
  isOpen: boolean;
  repoPath: string;
  commits: CommitItem[];
}>();

const emit = defineEmits<{
  (e: "close"): void;
  (e: "success"): void;
  (e: "notify", message: string): void;
}>();

const orderedCommits = ref<CommitItem[]>([]);
const isCheckingRisk = ref(false);
const prediction = ref<MergeConflictPrediction | null>(null);
const isExecuting = ref(false);
const result = ref<BatchCherryPickResult | null>(null);
const executionError = ref<string | null>(null);

function initializeCommits() {
  // Default: chronological oldest first for sequential cherry-picking
  const list = [...props.commits].sort(
    (a, b) => new Date(a.date).getTime() - new Date(b.date).getTime()
  );
  orderedCommits.value = list;
  result.value = null;
  executionError.value = null;
  if (props.isOpen && list.length > 0) {
    void runConflictPreCheck();
  }
}

watch(
  () => props.isOpen,
  (open) => {
    if (open) {
      initializeCommits();
    }
  },
  { immediate: true }
);

watch(
  () => props.commits,
  () => {
    if (props.isOpen) {
      initializeCommits();
    }
  },
  { deep: true }
);

function moveUp(index: number) {
  if (index <= 0 || index >= orderedCommits.value.length || isExecuting.value) return;
  const items = [...orderedCommits.value];
  const temp = items[index - 1]!;
  items[index - 1] = items[index]!;
  items[index] = temp;
  orderedCommits.value = items;
  void runConflictPreCheck();
}

function moveDown(index: number) {
  if (index < 0 || index >= orderedCommits.value.length - 1 || isExecuting.value) return;
  const items = [...orderedCommits.value];
  const temp = items[index + 1]!;
  items[index + 1] = items[index]!;
  items[index] = temp;
  orderedCommits.value = items;
  void runConflictPreCheck();
}

function reverseOrder() {
  if (isExecuting.value) return;
  orderedCommits.value = [...orderedCommits.value].reverse();
  void runConflictPreCheck();
}

async function runConflictPreCheck() {
  if (!props.repoPath || orderedCommits.value.length === 0) return;
  isCheckingRisk.value = true;
  try {
    // Check conflicts for the first commit in the sequence
    const firstCommit = orderedCommits.value[0];
    if (firstCommit) {
      const pred = await gitService.predictMergeConflicts(props.repoPath, firstCommit.hash);
      prediction.value = pred;
    }
  } catch (err) {
    console.debug("Failed to predict cherry-pick conflict:", err);
    prediction.value = null;
  } finally {
    isCheckingRisk.value = false;
  }
}

async function handleExecute() {
  if (!props.repoPath || orderedCommits.value.length === 0 || isExecuting.value) return;
  isExecuting.value = true;
  executionError.value = null;
  result.value = null;

  try {
    const hashes = orderedCommits.value.map((c) => c.hash);
    const res = await gitService.cherryPickMultiple(props.repoPath, hashes);
    result.value = res;
    if (res && !res.hasConflict && res.successCount === res.totalCount) {
      emit("notify", t("history.batch.done"));
      emit("success");
    }
  } catch (err: any) {
    executionError.value = err?.message || String(err);
  } finally {
    isExecuting.value = false;
  }
}
</script>

<template>
  <UiModal
    :open="isOpen"
    :title="t('history.batch.title')"
    width="max-w-xl"
    @update:open="!isExecuting && emit('close')"
  >
    <div class="space-y-4">
      <!-- Sequence Explainer & Reverse Button -->
      <div class="flex items-center justify-between text-xs text-ink-muted">
        <p class="leading-relaxed">
          {{ t("history.batch.desc.before") }}
          <span class="text-accent font-semibold">{{ t("history.batch.desc.order") }}</span>
          {{ t("history.batch.desc.after") }}
          <span class="text-ink font-mono font-bold">{{ orderedCommits.length }}</span>
          {{ t("history.batch.desc.tail") }}
        </p>
        <button
          type="button"
          @click="reverseOrder"
          :disabled="isExecuting"
          class="flex items-center gap-1 px-2 py-1 rounded-lg border border-line bg-surface hover:bg-raised text-[10px] font-medium text-ink-secondary hover:text-ink transition cursor-pointer shrink-0 disabled:opacity-40"
          :title="t('history.batch.reverse.hint')"
        >
          <ArrowUpDown class="w-3 h-3 text-accent" />
          <span>{{ t("history.batch.reverse") }}</span>
        </button>
      </div>

      <!-- Predictive Conflict Risk Card -->
      <div
        v-if="prediction"
        class="p-2.5 rounded-xl border text-xs flex items-center justify-between gap-3 transition-colors"
        :class="
          prediction.hasConflicts
            ? 'bg-danger/10 border-danger/30 text-danger'
            : prediction.conflictRisk === 'low'
            ? 'bg-warning/10 border-warning/30 text-warning'
            : 'bg-success/10 border-success/30 text-success'
        "
      >
        <div class="flex items-center gap-2 min-w-0">
          <AlertTriangle v-if="prediction.hasConflicts" class="w-4 h-4 shrink-0 text-danger" />
          <ShieldAlert v-else-if="prediction.conflictRisk === 'low'" class="w-4 h-4 shrink-0 text-warning" />
          <ShieldCheck v-else class="w-4 h-4 shrink-0 text-success" />
          <div class="min-w-0">
            <div class="font-semibold text-[11px] truncate">
              {{
                prediction.hasConflicts
                  ? t("history.batch.risk.conflict")
                  : prediction.conflictRisk === 'low'
                  ? t("history.batch.risk.low")
                  : t("history.batch.risk.clean")
              }}
            </div>
            <div class="text-[10px] opacity-80 truncate">
              {{ prediction.summary }}
            </div>
          </div>
        </div>
        <span class="font-mono text-[10px] font-bold px-1.5 py-0.5 rounded bg-surface/50 border border-current shrink-0">
          {{ prediction.riskScore }}%
        </span>
      </div>

      <!-- Commit Sequencer List -->
      <div v-if="!result" class="max-h-60 overflow-y-auto space-y-1.5 pr-1">
        <div
          v-for="(c, index) in orderedCommits"
          :key="c.hash"
          class="flex items-center gap-2 px-2.5 py-1.5 rounded-xl bg-canvas border border-line text-xs group hover:border-line-hover transition"
        >
          <span class="w-5 text-[10px] text-ink-muted font-mono text-right shrink-0">{{ index + 1 }}</span>
          <span class="font-mono text-[11px] text-accent font-semibold shrink-0">{{ c.shortHash }}</span>
          <span class="text-ink truncate flex-1">{{ c.subject }}</span>
          <div class="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition shrink-0">
            <button
              type="button"
              @click="moveUp(index)"
              :disabled="index === 0 || isExecuting"
              class="p-1 rounded hover:bg-raised text-ink-muted hover:text-ink disabled:opacity-30 cursor-pointer"
              :title="t('history.rebase.move.up')"
            >
              <ArrowUp class="w-3.5 h-3.5" />
            </button>
            <button
              type="button"
              @click="moveDown(index)"
              :disabled="index === orderedCommits.length - 1 || isExecuting"
              class="p-1 rounded hover:bg-raised text-ink-muted hover:text-ink disabled:opacity-30 cursor-pointer"
              :title="t('history.rebase.move.down')"
            >
              <ArrowDown class="w-3.5 h-3.5" />
            </button>
          </div>
        </div>
      </div>

      <!-- Execution Result Feedback -->
      <div
        v-if="result"
        class="rounded-xl border p-3 text-xs flex flex-col gap-1.5"
        :class="
          result.hasConflict || result.successCount < result.totalCount
            ? 'bg-warning/10 border-warning/30'
            : 'bg-success/10 border-success/30'
        "
      >
        <div class="flex items-center gap-2 font-semibold">
          <AlertTriangle v-if="result.hasConflict" class="w-4 h-4 text-warning" />
          <CheckCircle2 v-else class="w-4 h-4 text-success" />
          <span :class="result.hasConflict ? 'text-warning' : 'text-success'">
            {{
              result.hasConflict || result.successCount < result.totalCount
                ? t("history.batch.interrupted")
                : t("history.batch.done")
            }}
          </span>
        </div>
        <p class="text-ink-secondary font-mono text-[11px]">
          {{ t("history.batch.summary", { ok: result.successCount, total: result.totalCount }) }}
        </p>
        <template v-if="result.hasConflict || result.failedCommitHash">
          <p class="text-danger font-mono text-[11px] break-all">
            {{ t("history.batch.failed", { hash: result.failedCommitHash || t("history.unknown") }) }}
          </p>
          <p v-if="result.errorMessage" class="text-ink-muted text-[10px] whitespace-pre-wrap font-mono">
            {{ result.errorMessage }}
          </p>
          <p class="text-warning text-[11px]">
            {{ t("history.batch.hint") }}
          </p>
        </template>
      </div>

      <div
        v-if="executionError"
        class="rounded-xl border border-danger/30 bg-danger/10 p-3 text-xs text-danger whitespace-pre-wrap"
      >
        {{ executionError }}
      </div>
    </div>

    <template #footer>
      <span class="mr-auto text-[10px] text-ink-muted font-mono">
        {{ result || executionError ? t("history.batch.result") : t("history.batch.apply") }}
      </span>
      <UiButton variant="secondary" :disabled="isExecuting" @click="emit('close')">
        {{ result || executionError ? t("common.close") : t("common.cancel") }}
      </UiButton>
      <UiButton
        v-if="!result && !executionError"
        variant="primary"
        :disabled="isExecuting || orderedCommits.length === 0"
        @click="handleExecute"
      >
        <RefreshCw v-if="isExecuting" class="w-3.5 h-3.5 animate-spin" />
        <span>{{ isExecuting ? t("history.batch.running") : t("history.batch.confirm") }}</span>
      </UiButton>
    </template>
  </UiModal>
</template>
