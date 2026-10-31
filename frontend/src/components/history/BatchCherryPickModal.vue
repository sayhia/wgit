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
  Sparkles,
} from "lucide-vue-next";
import UiModal from "../ui/UiModal.vue";
import UiButton from "../ui/UiButton.vue";
import { t } from "../../i18n";
import { gitService } from "../../services/gitService";
import type {
  CommitItem,
  BatchCherryPickResult,
  MergeConflictPrediction,
  CherryPickSimulationReport,
  CherryPickBatchResult,
} from "../../types";

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
const shuttleReport = ref<CherryPickSimulationReport | null>(null);
const recordOrigin = ref(true);
const isExecuting = ref(false);
const result = ref<CherryPickBatchResult | null>(null);
const executionError = ref<string | null>(null);

function initializeCommits() {
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
    const hashes = orderedCommits.value.map((c) => c.hash);
    const [pred, sim] = await Promise.all([
      orderedCommits.value[0]
        ? gitService.predictMergeConflicts(props.repoPath, orderedCommits.value[0].hash).catch(() => null)
        : null,
      gitService.simulateCherryPick(props.repoPath, hashes).catch(() => null),
    ]);
    prediction.value = pred;
    shuttleReport.value = sim;
  } catch (err) {
    /* best-effort pre-check */
    prediction.value = null;
    shuttleReport.value = null;
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
    const res = await gitService.applyCherryPickBatch(
      props.repoPath,
      hashes,
      recordOrigin.value,
      false
    ).catch(async () => {
      /* best-effort fallback */
      return (await gitService.cherryPickMultiple(props.repoPath, hashes)) as any;
    });
    result.value = res;
    if (res && res.success) {
      emit("notify", t("history.batch.done"));
      emit("success");
    } else if (res && !res.success) {
      executionError.value = res.errorMessage || res.summary;
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
    <div class="space-y-4" data-testid="cherry-pick-shuttle-modal">
      <!-- Sequence Explainer & Reverse Button & Record Origin Toggle -->
      <div class="flex flex-wrap items-center justify-between gap-2 text-xs text-ink-muted">
        <p class="leading-relaxed">
          {{ t("history.batch.desc.before") }}
          <span class="text-accent font-semibold">{{ t("history.batch.desc.order") }}</span>
          {{ t("history.batch.desc.after") }}
          <span class="text-ink font-mono font-bold">{{ orderedCommits.length }}</span>
          {{ t("history.batch.desc.tail") }}
        </p>

        <div class="flex items-center gap-2">
          <label class="flex items-center gap-1 text-[11px] text-ink-secondary cursor-pointer select-none">
            <input
              v-model="recordOrigin"
              type="checkbox"
              data-testid="cherry-pick-record-origin-toggle"
              class="rounded border-line bg-surface text-accent focus:ring-0"
            />
            <span>-x ({{ t("history.batch.origin") }})</span>
          </label>

          <button
            type="button"
            @click="reverseOrder"
            :disabled="isExecuting"
            class="flex items-center gap-1 px-2 py-1 rounded-lg border border-line bg-surface hover:bg-raised text-[10px] font-medium text-ink-secondary hover:text-ink transition cursor-pointer shrink-0 disabled:opacity-40"
            :title="t('history.batch.reverse.hint')"
          >
            <ArrowUpDown class="w-3 h-3 text-accent" aria-hidden="true" />
            <span>{{ t("history.batch.reverse") }}</span>
          </button>
        </div>
      </div>

      <!-- Phase 53: Cherry-pick Shuttle Simulation Card -->
      <div
        v-if="shuttleReport || prediction"
        class="p-2.5 rounded-xl border text-xs flex items-center justify-between gap-3 transition-colors"
        :class="
          shuttleReport
            ? (shuttleReport.viabilityScore >= 80 ? 'bg-success-bg border-success/30 text-success' : shuttleReport.viabilityScore >= 50 ? 'bg-warning-bg border-warning/30 text-warning' : 'bg-danger-bg border-danger/30 text-danger')
            : (prediction && !prediction.hasConflicts ? 'bg-success-bg border-success/30 text-success' : 'bg-warning-bg border-warning/30 text-warning')
        "
      >
        <div class="flex items-center gap-2 min-w-0">
          <ShieldCheck v-if="shuttleReport ? shuttleReport.viabilityScore >= 80 : (prediction && !prediction.hasConflicts)" class="w-4 h-4 shrink-0 text-success" />
          <ShieldAlert v-else-if="shuttleReport ? shuttleReport.viabilityScore >= 50 : (prediction && prediction.riskScore <= 50)" class="w-4 h-4 shrink-0 text-warning" />
          <AlertTriangle v-else class="w-4 h-4 shrink-0 text-danger" />
          <div class="min-w-0">
            <div class="font-semibold text-[11px] truncate">
              {{ t("history.batch.shuttle.title") }}
            </div>
            <div class="text-[10px] opacity-80 truncate">
              {{ shuttleReport ? shuttleReport.summary : prediction?.summary }}
            </div>
          </div>
        </div>
        <div class="flex items-center gap-1.5 shrink-0">
          <span
            data-testid="cherry-pick-viability-chip"
            class="font-mono text-[10px] font-bold px-1.5 py-0.5 rounded bg-surface border border-current"
          >
            {{ shuttleReport ? shuttleReport.viabilityScore : (prediction ? (100 - prediction.riskScore) : 100) }}%
          </span>
          <button
            type="button"
            data-testid="cherry-pick-simulate-btn"
            @click="runConflictPreCheck"
            :disabled="isCheckingRisk"
            class="p-1 rounded hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer"
            :title="t('history.batch.resimulate')"
          >
            <RefreshCw class="w-3 h-3" :class="isCheckingRisk ? 'animate-spin' : ''" aria-hidden="true" />
          </button>
        </div>
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

          <span
            v-if="shuttleReport && shuttleReport.steps[index]"
            class="text-[9px] font-mono px-1.5 py-0.2 rounded shrink-0"
            :class="shuttleReport.steps[index]?.canApplyCleanly ? 'bg-success-bg text-success' : 'bg-warning-bg text-warning'"
          >
            {{ shuttleReport.steps[index]?.canApplyCleanly ? t("history.batch.step.clean") : t("history.batch.step.conflict") }}
          </span>

          <div class="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition shrink-0">
            <button
              type="button"
              @click="moveUp(index)"
              :disabled="index === 0 || isExecuting"
              class="p-1 rounded hover:bg-raised text-ink-muted hover:text-ink disabled:opacity-30 cursor-pointer"
              :title="t('history.rebase.move.up')"
            >
              <ArrowUp class="w-3.5 h-3.5" aria-hidden="true" />
            </button>
            <button
              type="button"
              @click="moveDown(index)"
              :disabled="index === orderedCommits.length - 1 || isExecuting"
              class="p-1 rounded hover:bg-raised text-ink-muted hover:text-ink disabled:opacity-30 cursor-pointer"
              :title="t('history.rebase.move.down')"
            >
              <ArrowDown class="w-3.5 h-3.5" aria-hidden="true" />
            </button>
          </div>
        </div>
      </div>

      <!-- Execution Result Feedback -->
      <div
        v-if="result"
        class="rounded-xl border p-3 text-xs flex flex-col gap-1.5"
        :class="
          !result.success
            ? 'bg-warning-bg border-warning/30'
            : 'bg-success-bg border-success/30'
        "
      >
        <div class="flex items-center gap-2 font-semibold">
          <AlertTriangle v-if="!result.success" class="w-4 h-4 text-warning" />
          <CheckCircle2 v-else class="w-4 h-4 text-success" />
          <span :class="!result.success ? 'text-warning' : 'text-success'">
            {{
              !result.success
                ? t("history.batch.interrupted")
                : t("history.batch.done")
            }}
          </span>
        </div>
        <p class="text-ink-secondary font-mono text-[11px]">
          {{ result.summary }}
        </p>
        <template v-if="!result.success">
          <p class="text-danger font-mono text-[11px] break-all">
            {{ result.errorMessage }}
          </p>
        </template>
      </div>

      <div
        v-if="executionError"
        class="rounded-xl border border-danger/30 bg-danger-bg p-3 text-xs text-danger whitespace-pre-wrap"
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
        data-testid="cherry-pick-apply-btn"
        :disabled="isExecuting || orderedCommits.length === 0"
        @click="handleExecute"
      >
        <RefreshCw v-if="isExecuting" class="w-3.5 h-3.5 animate-spin" />
        <span>{{ isExecuting ? t("history.batch.running") : t("history.batch.confirm") }}</span>
      </UiButton>
    </template>
  </UiModal>
</template>
