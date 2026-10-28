<script setup lang="ts">
import { ref, computed, watch } from "vue";
import {
  GitBranch,
  AlertTriangle,
  RefreshCw,
  ArrowUp,
  ArrowDown,
  GripVertical,
  CheckCheck,
  ArrowUpDown,
  RotateCcw,
  Layers,
  Radar,
} from "lucide-vue-next";
import UiModal from "../ui/UiModal.vue";
import UiButton from "../ui/UiButton.vue";
import { gitService } from "../../services/gitService";
import { t } from "../../i18n";
import type { RebaseTodoItem, RebaseSimulationReport, RebaseTodoValidationReport } from "../../types";

export interface RebaseBaseOption {
  value: string;
  label: string;
}

const props = defineProps<{
  isOpen: boolean;
  repoPath: string;
  rebaseBase: string;
  rebaseBaseOptions: { value: string; label: string }[];
  initialPlan: RebaseTodoItem[];
  isLoadingPlan: boolean;
  planError: string | null;
  selectionCount: number;
  isHashSelected: (hash: string) => boolean;
  isExecuting: boolean;
  executionError: string | null;
}>();

const emit = defineEmits<{
  (e: "close"): void;
  (e: "reload-plan"): void;
  (e: "update:rebaseBase", base: string): void;
  (e: "execute", plan: RebaseTodoItem[]): void;
  (e: "open-kanban"): void;
}>();

const REBASE_ACTIONS = computed(() => [
  { value: "pick", label: t("history.rebase.action.pick") },
  { value: "reword", label: t("history.rebase.action.reword") },
  { value: "edit", label: t("history.rebase.action.edit") },
  { value: "squash", label: t("history.rebase.action.squash") },
  { value: "fixup", label: t("history.rebase.action.fixup") },
  { value: "drop", label: t("history.rebase.action.drop") },
]);

const localPlan = ref<RebaseTodoItem[]>([]);
const draggedIndex = ref<number | null>(null);
const dragOverIndex = ref<number | null>(null);
const isSimulatingRadar = ref(false);
const radarReport = ref<RebaseSimulationReport | null>(null);
const todoValidation = ref<RebaseTodoValidationReport | null>(null);

async function runRebaseRadar() {
  if (!props.repoPath || isSimulatingRadar.value || localPlan.value.length === 0) return;
  isSimulatingRadar.value = true;
  try {
    const hashes = localPlan.value.map((i) => i.hash).filter(Boolean);
    const [sim, val] = await Promise.all([
      gitService.simulateRebaseOnto(props.repoPath, props.rebaseBase, hashes),
      gitService.validateRebaseTodoGraph(props.repoPath, localPlan.value),
    ]);
    radarReport.value = sim;
    todoValidation.value = val;
  } catch {
    /* ignore */
  } finally {
    isSimulatingRadar.value = false;
  }
}

function ensureFirstCommitValid() {
  const first = localPlan.value[0];
  if (first && (first.action === "squash" || first.action === "fixup")) {
    first.action = "pick";
  }
}

function syncPlanFromProps() {
  localPlan.value = (props.initialPlan || []).map((item) => ({ ...item }));
  ensureFirstCommitValid();
}

watch(
  () => props.initialPlan,
  () => {
    syncPlanFromProps();
  },
  { immediate: true, deep: true }
);

watch(
  () => props.isOpen,
  (open) => {
    if (open) {
      syncPlanFromProps();
    }
  }
);

function handleActionChange(index: number, newAction: string) {
  if (props.isExecuting) return;
  const item = localPlan.value[index];
  if (!item) return;
  if (index === 0 && (newAction === "squash" || newAction === "fixup")) {
    item.action = "pick";
    return;
  }
  item.action = newAction;
}

function moveUp(index: number) {
  if (index <= 0 || index >= localPlan.value.length || props.isExecuting) return;
  const items = [...localPlan.value];
  const current = items[index];
  const prev = items[index - 1];
  if (!current || !prev) return;
  items[index] = prev;
  items[index - 1] = current;
  localPlan.value = items;
  ensureFirstCommitValid();
}

function moveDown(index: number) {
  if (index < 0 || index >= localPlan.value.length - 1 || props.isExecuting) return;
  const items = [...localPlan.value];
  const current = items[index];
  const next = items[index + 1];
  if (!current || !next) return;
  items[index] = next;
  items[index + 1] = current;
  localPlan.value = items;
  ensureFirstCommitValid();
}

function handleDragStart(index: number, event: DragEvent) {
  if (props.isExecuting) return;
  draggedIndex.value = index;
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = "move";
    event.dataTransfer.setData("text/plain", String(index));
  }
}

function handleDragOver(index: number, event: DragEvent) {
  if (props.isExecuting || draggedIndex.value === null) return;
  event.preventDefault();
  if (event.dataTransfer) {
    event.dataTransfer.dropEffect = "move";
  }
  dragOverIndex.value = index;
}

function handleDrop(index: number, event: DragEvent) {
  if (props.isExecuting) return;
  event.preventDefault();
  const fromIndex = draggedIndex.value;
  if (fromIndex !== null && fromIndex !== index && fromIndex >= 0 && fromIndex < localPlan.value.length) {
    const items = [...localPlan.value];
    const [movedItem] = items.splice(fromIndex, 1);
    if (movedItem) {
      items.splice(index, 0, movedItem);
      localPlan.value = items;
      ensureFirstCommitValid();
    }
  }
  draggedIndex.value = null;
  dragOverIndex.value = null;
}

function handleDragEnd() {
  draggedIndex.value = null;
  dragOverIndex.value = null;
}

function setAllPick() {
  if (props.isExecuting || props.isLoadingPlan || localPlan.value.length === 0) return;
  for (const item of localPlan.value) {
    item.action = "pick";
  }
}

function reversePlan() {
  if (props.isExecuting || props.isLoadingPlan || localPlan.value.length <= 1) return;
  localPlan.value = [...localPlan.value].reverse();
  ensureFirstCommitValid();
}

function resetPlan() {
  if (props.isExecuting || props.isLoadingPlan) return;
  syncPlanFromProps();
}

function handleClose() {
  if (props.isExecuting) return;
  emit("close");
}

function handleExecute() {
  if (props.isExecuting || props.isLoadingPlan || localPlan.value.length === 0 || !props.rebaseBase) return;
  ensureFirstCommitValid();
  emit("execute", localPlan.value.map((item) => ({ ...item })));
}
</script>

<template>
  <UiModal
    :open="isOpen"
    :title="t('history.rebase.title')"
    width="max-w-3xl"
    @update:open="!isExecuting && handleClose()"
  >
    <template #header>
      <h3 class="text-sm font-bold text-ink flex items-center gap-2">
        <GitBranch class="w-4 h-4 text-cyan" />
        {{ t("history.rebase.title") }}
      </h3>
    </template>

    <div class="space-y-3">
      <!-- Warning explanation -->
      <div class="flex items-start gap-2 p-2.5 rounded-lg bg-warning/10 border border-warning/30 text-[11px] text-ink-secondary leading-relaxed">
        <AlertTriangle class="w-3.5 h-3.5 text-warning shrink-0 mt-0.5" />
        <span>{{ t("history.rebase.desc") }}</span>
      </div>

      <!-- Base selection & reload -->
      <div class="flex items-center gap-2 flex-wrap">
        <label class="text-[11px] text-ink-muted shrink-0">{{ t("history.rebase.base.label") }}</label>
        <select
          :value="rebaseBase"
          :disabled="isLoadingPlan || isExecuting"
          class="flex-1 min-w-[200px] px-2 py-1 rounded-md bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent cursor-pointer transition disabled:opacity-60"
          @change="emit('update:rebaseBase', ($event.target as HTMLSelectElement).value)"
        >
          <option v-if="rebaseBaseOptions.length === 0" value="">{{ t("history.rebase.base.none") }}</option>
          <option v-for="option in rebaseBaseOptions" :key="option.value" :value="option.value">
            {{ option.label }}
          </option>
        </select>
        <button
          type="button"
          :aria-label="t('history.rebase.reload.title')"
          :disabled="isLoadingPlan || isExecuting"
          class="p-1 rounded-md hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer disabled:opacity-50"
          :title="t('history.rebase.reload.title')"
          @click="emit('reload-plan')"
        >
          <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isLoadingPlan }" />
        </button>
      </div>

      <!-- Toolbar: count, highlight info, and Quick Actions -->
      <div class="flex items-center justify-between gap-2 text-[10px] text-ink-muted flex-wrap">
        <div class="flex items-center gap-1.5">
          <span>
            {{ t("history.rebase.plan.count") }}<span class="text-ink font-mono">{{ localPlan.length }}</span>
          </span>
          <span v-if="selectionCount > 0">· {{ t("history.rebase.plan.selected", { count: selectionCount }) }}</span>
        </div>

        <div class="flex items-center gap-1.5">
          <button
            type="button"
            :disabled="isLoadingPlan || isExecuting || localPlan.length === 0"
            class="px-2 py-0.5 rounded text-[11px] bg-canvas hover:bg-raised border border-line text-ink-secondary hover:text-ink transition cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-1"
            :title="t('history.rebase.all.pick')"
            @click="setAllPick"
          >
            <CheckCheck class="w-3 h-3" />
            <span>{{ t("history.rebase.all.pick") }}</span>
          </button>
          <button
            type="button"
            :disabled="isLoadingPlan || isExecuting || localPlan.length <= 1"
            class="px-2 py-0.5 rounded text-[11px] bg-canvas hover:bg-raised border border-line text-ink-secondary hover:text-ink transition cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-1"
            :title="t('history.rebase.reverse')"
            @click="reversePlan"
          >
            <ArrowUpDown class="w-3 h-3" />
            <span>{{ t("history.rebase.reverse") }}</span>
          </button>
          <button
            type="button"
            :disabled="isLoadingPlan || isExecuting || localPlan.length === 0"
            class="px-2 py-0.5 rounded text-[11px] bg-canvas hover:bg-raised border border-line text-ink-secondary hover:text-ink transition cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-1"
            :title="t('history.rebase.reset.plan')"
            @click="resetPlan"
          >
            <RotateCcw class="w-3 h-3" />
            <span>{{ t("history.rebase.reset.plan") }}</span>
          </button>
          <button
            type="button"
            data-testid="rebase-radar-btn"
            :disabled="isLoadingPlan || isExecuting || localPlan.length === 0 || isSimulatingRadar"
            class="px-2 py-0.5 rounded text-[11px] bg-accent/15 hover:bg-accent/25 border border-accent/30 text-accent font-medium transition cursor-pointer disabled:opacity-50 flex items-center gap-1"
            :title="t('history.rebase.radar.btn')"
            @click="runRebaseRadar"
          >
            <RefreshCw v-if="isSimulatingRadar" class="w-3 h-3 animate-spin" />
            <Radar v-else class="w-3 h-3" />
            <span>{{ isSimulatingRadar ? t("history.rebase.radar.simulating") : t("history.rebase.radar.btn") }}</span>
          </button>
          <button
            type="button"
            data-testid="open-kanban-mode-btn"
            class="px-2 py-0.5 rounded text-[11px] bg-indigo-950/40 hover:bg-indigo-900/60 border border-indigo-500/40 text-indigo-300 hover:text-indigo-200 transition cursor-pointer flex items-center gap-1"
            :title="t('rebase.kanban.open')"
            @click="emit('open-kanban')"
          >
            <Layers class="w-3 h-3" />
            <span>{{ t("rebase.kanban.open") }}</span>
          </button>
        </div>
      </div>

      <!-- Rebase Radar Collision Panel -->
      <div
        v-if="radarReport"
        data-testid="rebase-collision-panel"
        class="p-2.5 rounded-lg border text-xs space-y-1.5"
        :class="radarReport.conflictSteps > 0 ? 'bg-danger/10 border-danger/30 text-danger' : 'bg-success/10 border-success/30 text-success'"
      >
        <div class="flex items-center justify-between font-semibold">
          <span>{{ t("history.rebase.radar.title") }}</span>
          <span class="px-1.5 py-0.2 rounded font-mono text-[10px] bg-canvas border border-line">
            {{ radarReport.viabilityScore }}%
          </span>
        </div>
        <p class="text-[11px] opacity-90">{{ radarReport.summary }}</p>
        <div v-if="todoValidation && (!todoValidation.valid || todoValidation.warnings.length > 0)" class="text-[10px] space-y-0.5">
          <p v-for="err in todoValidation.errors" :key="err" class="text-danger font-medium">⚠️ {{ err }}</p>
          <p v-for="warn in todoValidation.warnings" :key="warn" class="text-warning">ℹ️ {{ warn }}</p>
        </div>
      </div>

      <!-- Plan Loading -->
      <div
        v-if="isLoadingPlan"
        class="py-10 text-center text-xs text-ink-muted flex flex-col items-center gap-2"
      >
        <RefreshCw class="w-4 h-4 animate-spin text-accent" />
        <span>{{ t("history.rebase.plan.loading") }}</span>
      </div>

      <!-- Plan Loading Error -->
      <div
        v-else-if="planError"
        class="rounded-lg border border-danger/30 bg-danger/10 p-2.5 text-[11px] text-danger whitespace-pre-wrap"
      >
        {{ planError }}
      </div>

      <!-- Empty Plan -->
      <div v-else-if="localPlan.length === 0" class="py-10 text-center text-xs text-ink-muted">
        {{ t("history.rebase.plan.empty") }}
      </div>

      <!-- Plan List -->
      <div v-else class="flex flex-col gap-1 max-h-[46vh] overflow-y-auto pr-1">
        <div
          v-for="(todo, index) in localPlan"
          :key="todo.hash + ':' + index"
          :draggable="!isExecuting"
          class="group flex items-center gap-2 px-2 py-1.5 rounded-lg border text-xs transition-colors"
          :class="[
            isHashSelected(todo.hash) ? 'bg-accent/10 border-accent/30' : 'bg-canvas border-line/60',
            dragOverIndex === index ? 'ring-2 ring-accent border-accent' : '',
            draggedIndex === index ? 'opacity-40' : ''
          ]"
          @dragstart="handleDragStart(index, $event)"
          @dragover="handleDragOver(index, $event)"
          @drop="handleDrop(index, $event)"
          @dragend="handleDragEnd"
        >
          <!-- Drag grip handle -->
          <GripVertical
            class="w-3.5 h-3.5 text-ink-muted group-hover:text-ink cursor-grab active:cursor-grabbing shrink-0"
          />

          <!-- Move Up / Down Buttons -->
          <div class="flex items-center gap-0.5 shrink-0">
            <button
              type="button"
              :disabled="index === 0 || isExecuting"
              :aria-label="t('history.rebase.move.up')"
              :title="t('history.rebase.move.up')"
              class="p-0.5 rounded hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer disabled:opacity-30 disabled:cursor-not-allowed"
              @click.stop="moveUp(index)"
            >
              <ArrowUp class="w-3 h-3" />
            </button>
            <button
              type="button"
              :disabled="index === localPlan.length - 1 || isExecuting"
              :aria-label="t('history.rebase.move.down')"
              :title="t('history.rebase.move.down')"
              class="p-0.5 rounded hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer disabled:opacity-30 disabled:cursor-not-allowed"
              @click.stop="moveDown(index)"
            >
              <ArrowDown class="w-3 h-3" />
            </button>
          </div>

          <!-- Index -->
          <span class="w-4 text-[10px] text-ink-muted font-mono text-right shrink-0">{{ index + 1 }}</span>

          <!-- Action dropdown with safety validation -->
          <select
            :value="todo.action"
            :disabled="isExecuting"
            :title="index === 0 && (todo.action === 'squash' || todo.action === 'fixup') ? t('history.rebase.first.cannot.squash') : undefined"
            class="px-1.5 py-0.5 rounded-md bg-surface border border-line text-[10px] text-ink font-mono focus:outline-none focus:border-accent cursor-pointer disabled:opacity-60 shrink-0"
            @change="handleActionChange(index, ($event.target as HTMLSelectElement).value)"
          >
            <option
              v-for="action in REBASE_ACTIONS"
              :key="action.value"
              :value="action.value"
              :disabled="index === 0 && (action.value === 'squash' || action.value === 'fixup')"
              :title="index === 0 && (action.value === 'squash' || action.value === 'fixup') ? t('history.rebase.first.cannot.squash') : undefined"
            >
              {{ action.label }}
            </option>
          </select>

          <!-- Short hash -->
          <span class="font-mono text-[10px] text-accent shrink-0">{{ todo.shortHash }}</span>

          <!-- Step Risk Indicator -->
          <span
            v-if="radarReport && radarReport.steps[index]"
            data-testid="rebase-step-risk"
            class="px-1.5 py-0.2 rounded text-[9px] font-mono shrink-0"
            :class="[
              radarReport.steps[index].status === 'conflict'
                ? 'bg-danger/20 text-danger border border-danger/30 font-bold'
                : radarReport.steps[index].status === 'trivial_merge'
                ? 'bg-warning/20 text-warning border border-warning/30'
                : 'bg-success/20 text-success border border-success/30'
            ]"
          >
            {{ radarReport.steps[index].status }}
          </span>

          <!-- Subject -->
          <span class="text-ink-secondary truncate flex-1" :title="todo.subject">{{ todo.subject }}</span>

          <!-- Author -->
          <span class="text-[10px] text-ink-muted truncate max-w-[110px] shrink-0" :title="todo.author">{{ todo.author }}</span>

          <!-- Selected badge -->
          <span
            v-if="isHashSelected(todo.hash)"
            class="px-1 py-0.5 rounded bg-accent/15 border border-accent/30 text-accent text-[9px] shrink-0"
          >
            {{ t("history.rebase.plan.badge") }}
          </span>
        </div>
      </div>

      <!-- Execution Error -->
      <div
        v-if="executionError"
        class="rounded-lg border border-danger/30 bg-danger/10 p-2.5 text-[11px] text-danger whitespace-pre-wrap max-h-32 overflow-y-auto"
      >
        {{ executionError }}
        <p class="mt-1 text-ink-muted">
          {{ t("history.rebase.hint") }}
        </p>
      </div>
    </div>

    <!-- Footer -->
    <template #footer>
      <span class="mr-auto text-[10px] text-ink-muted font-mono truncate max-w-[55%]" :title="rebaseBase">
        {{ t("history.rebase.base.current", { base: rebaseBase || t("history.rebase.base.unset") }) }}
      </span>
      <UiButton variant="secondary" :disabled="isExecuting" @click="handleClose">
        {{ t("history.rebase.cancel") }}
      </UiButton>
      <UiButton
        variant="primary"
        :disabled="isExecuting || isLoadingPlan || localPlan.length === 0 || !rebaseBase"
        @click="handleExecute"
      >
        <RefreshCw v-if="isExecuting" class="w-3.5 h-3.5 animate-spin" />
        <span>{{ isExecuting ? t("history.rebase.running") : t("history.rebase.execute") }}</span>
      </UiButton>
    </template>
  </UiModal>
</template>
