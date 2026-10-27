<script setup lang="ts">
import { ref, computed, watch } from "vue";
import {
  Layers,
  Scissors,
  AlertTriangle,
  CheckCircle2,
  MoveUp,
  MoveDown,
  RefreshCw,
  Plus,
  Trash2,
  Sparkles,
  FileText,
} from "lucide-vue-next";
import UiModal from "../ui/UiModal.vue";
import UiButton from "../ui/UiButton.vue";
import { gitService } from "../../services/gitService";
import { t } from "../../i18n";
import type {
  RebaseKanbanItem,
  RebasePlanPreview,
  CommitPartition,
  CommitSplitResult,
} from "../../types";

const props = defineProps<{
  open: boolean;
  repoPath: string;
  baseRef?: string;
  headRef?: string;
}>();

const emit = defineEmits<{
  (e: "update:open", value: boolean): void;
  (e: "applied", preview: RebasePlanPreview): void;
}>();

const isLoading = ref(false);
const error = ref<string | null>(null);
const planPreview = ref<RebasePlanPreview | null>(null);
const kanbanItems = ref<RebaseKanbanItem[]>([]);

// Split workbench state
const activeSplitCommit = ref<RebaseKanbanItem | null>(null);
const splitPartitions = ref<CommitPartition[]>([]);
const isSplitting = ref(false);
const splitError = ref<string | null>(null);
const splitSuccessMsg = ref<string | null>(null);

const actions = [
  { value: "pick", label: t("rebase.kanban.action.pick") },
  { value: "squash", label: t("rebase.kanban.action.squash") },
  { value: "fixup", label: t("rebase.kanban.action.fixup") },
  { value: "reword", label: t("rebase.kanban.action.reword") },
  { value: "drop", label: t("rebase.kanban.action.drop") },
  { value: "split", label: t("rebase.kanban.action.split") },
];

async function loadTopology() {
  if (!props.repoPath) return;
  isLoading.value = true;
  error.value = null;
  try {
    const base = props.baseRef || "HEAD~5";
    const head = props.headRef || "HEAD";
    const res = await gitService.previewRebaseTopology(props.repoPath, base, head);
    planPreview.value = res;
    kanbanItems.value = (res.items || []).map((item, idx) => ({
      ...item,
      currentOrder: idx,
    }));
  } catch (err: unknown) {
    error.value = err instanceof Error ? err.message : String(err);
  } finally {
    isLoading.value = false;
  }
}

watch(
  () => props.open,
  (isOpen) => {
    if (isOpen) {
      activeSplitCommit.value = null;
      splitSuccessMsg.value = null;
      splitError.value = null;
      void loadTopology();
    }
  },
  { immediate: true }
);

function moveItem(index: number, direction: -1 | 1) {
  const targetIndex = index + direction;
  if (targetIndex < 0 || targetIndex >= kanbanItems.value.length) return;
  const items = [...kanbanItems.value];
  const [removed] = items.splice(index, 1);
  if (!removed) return;
  items.splice(targetIndex, 0, removed);
  kanbanItems.value = items.map((item, idx) => ({ ...item, currentOrder: idx }));
}

function openSplitWorkbench(item: RebaseKanbanItem) {
  activeSplitCommit.value = item;
  splitError.value = null;
  splitSuccessMsg.value = null;
  const files = item.overlappingFiles && item.overlappingFiles.length > 0
    ? [...item.overlappingFiles]
    : ["file1.txt", "file2.txt"];
  const half = Math.max(1, Math.ceil(files.length / 2));
  splitPartitions.value = [
    {
      subject: `${item.subject} (part 1)`,
      files: files.slice(0, half),
    },
    {
      subject: `${item.subject} (part 2)`,
      files: files.slice(half),
    },
  ];
}

function addPartition() {
  splitPartitions.value.push({
    subject: `Sub-commit ${splitPartitions.value.length + 1}`,
    files: [],
  });
}

function removePartition(index: number) {
  if (splitPartitions.value.length <= 1) return;
  splitPartitions.value.splice(index, 1);
}

async function executeSplit() {
  if (!activeSplitCommit.value || !props.repoPath) return;
  isSplitting.value = true;
  splitError.value = null;
  try {
    const res = await gitService.splitCommitSurgically(
      props.repoPath,
      activeSplitCommit.value.hash,
      splitPartitions.value
    );
    if (res.success) {
      splitSuccessMsg.value = t("rebase.kanban.split.success", {
        count: res.generatedCommits?.length ?? 0,
      });
      setTimeout(() => {
        activeSplitCommit.value = null;
        void loadTopology();
      }, 1200);
    } else {
      splitError.value = res.message;
    }
  } catch (err: unknown) {
    splitError.value = err instanceof Error ? err.message : String(err);
  } finally {
    isSplitting.value = false;
  }
}

function handleApply() {
  if (!planPreview.value) return;
  const updatedPreview: RebasePlanPreview = {
    ...planPreview.value,
    items: kanbanItems.value,
  };
  emit("applied", updatedPreview);
  emit("update:open", false);
}
</script>

<template>
  <UiModal
    :open="open"
    :title="t('rebase.kanban.title')"
    width="max-w-4xl"
    @update:open="emit('update:open', $event)"
  >
    <div class="space-y-4 text-xs text-neutral-300">
      <!-- Subtitle & Controls -->
      <div class="flex items-center justify-between pb-2 border-b border-neutral-700/60">
        <p class="text-neutral-400">
          {{ t('rebase.kanban.subtitle') }}
        </p>
        <UiButton
          size="sm"
          variant="secondary"
          :disabled="isLoading"
          class="flex items-center gap-1.5"
          @click="loadTopology"
        >
          <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isLoading }" />
          <span>{{ t('rebase.kanban.load') }}</span>
        </UiButton>
      </div>

      <!-- Loading State -->
      <div v-if="isLoading" class="py-12 flex flex-col items-center justify-center gap-3">
        <RefreshCw class="w-7 h-7 text-indigo-400 animate-spin" />
        <span class="text-neutral-400">{{ t('rebase.kanban.loading') }}</span>
      </div>

      <!-- Error State -->
      <div
        v-else-if="error"
        class="p-3 bg-red-950/40 border border-red-500/30 rounded-lg text-red-300 flex items-center gap-2"
      >
        <AlertTriangle class="w-4 h-4 shrink-0 text-red-400" />
        <span>{{ error }}</span>
      </div>

      <!-- Main Content -->
      <div v-else class="space-y-4">
        <!-- Topology Summary Bar -->
        <div
          v-if="planPreview"
          class="grid grid-cols-3 gap-3 p-3 bg-neutral-900/60 border border-neutral-800 rounded-lg"
        >
          <div class="flex flex-col gap-0.5">
            <span class="text-neutral-400">{{ t('rebase.kanban.total_commits') }}</span>
            <span class="text-base font-semibold text-neutral-100">
              {{ kanbanItems.length }}
            </span>
          </div>
          <div class="flex flex-col gap-0.5">
            <span class="text-neutral-400">{{ t('rebase.kanban.conflict_risk') }}</span>
            <span
              class="text-base font-semibold"
              :class="planPreview.estimatedConflictCount > 0 ? 'text-amber-400' : 'text-emerald-400'"
            >
              {{ planPreview.estimatedConflictCount }}
            </span>
          </div>
          <div class="flex flex-col gap-0.5">
            <span class="text-neutral-400">{{ t('rebase.kanban.conflict_risk') }}</span>
            <span
              v-if="planPreview.canAutoLinearize"
              class="text-emerald-400 flex items-center gap-1 font-medium"
            >
              <CheckCircle2 class="w-3.5 h-3.5" />
              {{ t('rebase.kanban.linearize_ready') }}
            </span>
            <span
              v-else
              class="text-amber-400 flex items-center gap-1 font-medium"
            >
              <AlertTriangle class="w-3.5 h-3.5" />
              {{ t('rebase.kanban.linearize_warning') }}
            </span>
          </div>
        </div>

        <!-- Kanban Commit List -->
        <div v-if="kanbanItems.length === 0" class="py-8 text-center text-neutral-500">
          {{ t('rebase.kanban.empty') }}
        </div>

        <div v-else class="space-y-2 max-h-[380px] overflow-y-auto pr-1">
          <div
            v-for="(item, idx) in kanbanItems"
            :key="item.hash"
            class="p-3 bg-neutral-800/60 hover:bg-neutral-800/90 border rounded-lg transition-colors flex items-center justify-between gap-3"
            :class="{
              'border-emerald-500/30': item.conflictRisk === 'low',
              'border-amber-500/40': item.conflictRisk === 'medium',
              'border-rose-500/50 bg-rose-950/20': item.conflictRisk === 'high',
              'opacity-60': item.action === 'drop',
            }"
          >
            <!-- Left Info -->
            <div class="flex items-center gap-3 min-w-0 flex-1">
              <span class="text-neutral-500 font-mono w-5 text-right shrink-0">
                {{ idx + 1 }}
              </span>
              <span class="font-mono text-indigo-400 bg-indigo-950/50 px-1.5 py-0.5 rounded shrink-0">
                {{ item.shortHash || item.hash.slice(0, 7) }}
              </span>
              <div class="min-w-0 flex-1">
                <div class="text-neutral-200 font-medium truncate">
                  {{ item.subject }}
                </div>
                <div class="text-[11px] text-neutral-400 flex items-center gap-2 mt-0.5">
                  <span>{{ item.author }}</span>
                  <span>·</span>
                  <span>{{ item.authorDate }}</span>
                  <span
                    v-if="item.overlappingFiles && item.overlappingFiles.length > 0"
                    class="text-amber-400 bg-amber-950/40 px-1 rounded text-[10px]"
                  >
                    {{ item.overlappingFiles.length }} overlap
                  </span>
                </div>
              </div>
            </div>

            <!-- Risk Badge -->
            <div class="shrink-0">
              <span
                v-if="item.conflictRisk === 'low'"
                class="px-2 py-0.5 rounded text-[10px] font-medium bg-emerald-950/60 text-emerald-300 border border-emerald-500/30"
              >
                {{ t('rebase.kanban.risk.low') }}
              </span>
              <span
                v-else-if="item.conflictRisk === 'medium'"
                class="px-2 py-0.5 rounded text-[10px] font-medium bg-amber-950/60 text-amber-300 border border-amber-500/30"
              >
                {{ t('rebase.kanban.risk.medium') }}
              </span>
              <span
                v-else
                class="px-2 py-0.5 rounded text-[10px] font-medium bg-rose-950/60 text-rose-300 border border-rose-500/30"
              >
                {{ t('rebase.kanban.risk.high') }}
              </span>
            </div>

            <!-- Action Select & Split Workbench Trigger -->
            <div class="flex items-center gap-1.5 shrink-0">
              <select
                v-model="item.action"
                class="bg-neutral-900 border border-neutral-700 rounded px-2 py-1 text-xs text-neutral-200 focus:outline-none focus:border-indigo-500"
              >
                <option v-for="act in actions" :key="act.value" :value="act.value">
                  {{ act.label }}
                </option>
              </select>

              <button
                type="button"
                data-testid="surgical-split-trigger-btn"
                class="p-1.5 text-neutral-400 hover:text-indigo-300 hover:bg-neutral-700/60 rounded transition-colors"
                :title="t('rebase.kanban.btn.split_workbench')"
                @click="openSplitWorkbench(item)"
              >
                <Scissors class="w-3.5 h-3.5" />
              </button>

              <!-- Order Buttons -->
              <div class="flex flex-col gap-0.5">
                <button
                  type="button"
                  :disabled="idx === 0"
                  class="p-0.5 text-neutral-400 hover:text-neutral-200 disabled:opacity-30 disabled:pointer-events-none"
                  @click="moveItem(idx, -1)"
                >
                  <MoveUp class="w-3 h-3" />
                </button>
                <button
                  type="button"
                  :disabled="idx === kanbanItems.length - 1"
                  class="p-0.5 text-neutral-400 hover:text-neutral-200 disabled:opacity-30 disabled:pointer-events-none"
                  @click="moveItem(idx, 1)"
                >
                  <MoveDown class="w-3 h-3" />
                </button>
              </div>
            </div>
          </div>
        </div>

        <!-- Surgical Split Workbench Sub-Panel -->
        <div
          v-if="activeSplitCommit"
          class="mt-4 p-4 bg-indigo-950/20 border border-indigo-500/40 rounded-xl space-y-3"
        >
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2">
              <Scissors class="w-4 h-4 text-indigo-400" />
              <span class="font-semibold text-neutral-100">
                {{ t('rebase.kanban.split.title') }}
              </span>
              <span class="font-mono text-indigo-300 text-[11px] bg-indigo-950/60 px-1.5 py-0.5 rounded">
                {{ activeSplitCommit.shortHash || activeSplitCommit.hash.slice(0, 7) }}
              </span>
            </div>
            <UiButton
              size="sm"
              variant="secondary"
              @click="activeSplitCommit = null"
            >
              ✕
            </UiButton>
          </div>

          <p class="text-neutral-400 text-[11px]">
            {{ t('rebase.kanban.split.desc') }}
          </p>

          <!-- Partition List -->
          <div class="space-y-2">
            <div
              v-for="(part, pIdx) in splitPartitions"
              :key="pIdx"
              class="p-2.5 bg-neutral-900/80 border border-neutral-700/60 rounded-lg space-y-2"
            >
              <div class="flex items-center justify-between gap-2">
                <span class="text-neutral-400 font-medium shrink-0">
                  {{ t('rebase.kanban.split.partition') }} {{ pIdx + 1 }}
                </span>
                <input
                  v-model="part.subject"
                  type="text"
                  :placeholder="t('rebase.kanban.split.subject_placeholder')"
                  class="flex-1 bg-neutral-800 border border-neutral-700 rounded px-2 py-1 text-xs text-neutral-100 focus:outline-none focus:border-indigo-500"
                />
                <button
                  type="button"
                  :disabled="splitPartitions.length <= 1"
                  class="text-neutral-500 hover:text-red-400 disabled:opacity-20 p-1"
                  @click="removePartition(pIdx)"
                >
                  <Trash2 class="w-3.5 h-3.5" />
                </button>
              </div>

              <!-- Files in partition -->
              <div class="flex flex-wrap gap-1.5 pl-6">
                <span
                  v-for="(f, fIdx) in part.files"
                  :key="fIdx"
                  class="inline-flex items-center gap-1 text-[11px] font-mono bg-neutral-800 text-neutral-300 px-2 py-0.5 rounded border border-neutral-700"
                >
                  <FileText class="w-3 h-3 text-neutral-400" />
                  {{ f }}
                </span>
                <span class="text-neutral-500 text-[10px] self-center">
                  ({{ t('rebase.kanban.split.file_count', { count: part.files.length }) }})
                </span>
              </div>
            </div>
          </div>

          <!-- Split Workbench Actions -->
          <div class="flex items-center justify-between pt-2">
            <UiButton
              size="sm"
              variant="secondary"
              class="flex items-center gap-1"
              @click="addPartition"
            >
              <Plus class="w-3.5 h-3.5" />
              <span>{{ t('rebase.kanban.split.add_partition') }}</span>
            </UiButton>

            <div class="flex items-center gap-2">
              <span v-if="splitError" class="text-red-400 text-xs">
                {{ splitError }}
              </span>
              <span v-if="splitSuccessMsg" class="text-emerald-400 text-xs">
                {{ splitSuccessMsg }}
              </span>
              <UiButton
                size="sm"
                variant="primary"
                :disabled="isSplitting"
                class="flex items-center gap-1.5"
                @click="executeSplit"
              >
                <Scissors class="w-3.5 h-3.5" :class="{ 'animate-pulse': isSplitting }" />
                <span>
                  {{ isSplitting ? t('rebase.kanban.split.executing') : t('rebase.kanban.split.execute') }}
                </span>
              </UiButton>
            </div>
          </div>
        </div>

        <!-- Dialog Footer -->
        <div class="flex items-center justify-end gap-2 pt-4 border-t border-neutral-800">
          <UiButton
            size="sm"
            variant="secondary"
            @click="emit('update:open', false)"
          >
            {{ t('common.cancel') }}
          </UiButton>
          <UiButton
            size="sm"
            variant="primary"
            data-testid="apply-kanban-rebase-btn"
            :disabled="kanbanItems.length === 0"
            @click="handleApply"
          >
            {{ t('rebase.kanban.btn.apply') }}
          </UiButton>
        </div>
      </div>
    </div>
  </UiModal>
</template>
