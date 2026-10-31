<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from "vue";
import {
  GitMerge,
  Check,
  RotateCcw,
  ChevronUp,
  ChevronDown,
  Save,
  FileCode,
  AlertTriangle,
  Layers,
  ArrowRight,
  ShieldCheck,
  Sparkles,
  ExternalLink,
} from "lucide-vue-next";
import UiModal from "./ui/UiModal.vue";
import UiButton from "./ui/UiButton.vue";
import { gitService } from "../services/gitService";
import { t } from "../i18n";
import type { ThreeWayMergeData, ThreeWayMergeBlock } from "../types";

const props = defineProps<{
  open: boolean;
  repoPath: string;
  filePath: string;
}>();

const emit = defineEmits<{
  (e: "update:open", value: boolean): void;
  (e: "resolved", filePath: string): void;
  (e: "notify", msg: string): void;
}>();

const isLoading = ref(false);
const isSaving = ref(false);
const mergeData = ref<ThreeWayMergeData | null>(null);
const currentResult = ref("");
const autoStage = ref(true);
const activeBlockIndex = ref(0);

// Track resolution status for each block: "ours" | "theirs" | "base" | "both" | "custom"
const blockResolutions = ref<Map<number, "ours" | "theirs" | "base" | "both" | "custom">>(new Map());

const totalBlocks = computed(() => mergeData.value?.blocks.length || 0);
const remainingUnresolved = computed(() => {
  const total = totalBlocks.value;
  const resolved = blockResolutions.value.size;
  return Math.max(0, total - resolved);
});

const isFullyResolved = computed(() => {
  if (!currentResult.value) return false;
  return !currentResult.value.includes("<<<<<<<") && !currentResult.value.includes(">>>>>>>");
});

async function loadData() {
  if (!props.repoPath || !props.filePath || !props.open) return;
  isLoading.value = true;
  blockResolutions.value.clear();
  activeBlockIndex.value = 0;
  try {
    const data = await gitService.getThreeWayMergeData(props.repoPath, props.filePath);
    mergeData.value = data;
    currentResult.value = data?.currentContent || "";
  } catch (err: any) {
    emit("notify", err?.message || String(err));
  } finally {
    isLoading.value = false;
  }
}

function getBlockStart(b: ThreeWayMergeBlock): number {
  return b.startLine ?? (b as any).StartLine ?? 0;
}

function getBlockEnd(b: ThreeWayMergeBlock): number {
  return b.endLine ?? (b as any).EndLine ?? 0;
}

function getOursLines(b?: ThreeWayMergeBlock | null): string[] {
  if (!b) return [];
  return b.oursLines ?? (b as any).OursLines ?? [];
}

function getBaseLines(b?: ThreeWayMergeBlock | null): string[] {
  if (!b) return [];
  return b.baseLines ?? (b as any).BaseLines ?? [];
}

function getTheirsLines(b?: ThreeWayMergeBlock | null): string[] {
  if (!b) return [];
  return b.theirsLines ?? (b as any).TheirsLines ?? [];
}

function assembleResult() {
  if (!mergeData.value) return;
  const rawLines = mergeData.value.currentContent.replace(/\r\n/g, "\n").split("\n");
  const blocks = mergeData.value.blocks;
  if (!blocks || blocks.length === 0) return;

  const resultLines: string[] = [];
  let lastLine = 0;

  for (let i = 0; i < blocks.length; i++) {
    const b = blocks[i]!;
    const start = getBlockStart(b);
    const end = getBlockEnd(b);
    if (start > lastLine) {
      resultLines.push(...rawLines.slice(lastLine, start));
    }

    const res = blockResolutions.value.get(i);
    switch (res) {
      case "ours":
        resultLines.push(...getOursLines(b));
        break;
      case "theirs":
        resultLines.push(...getTheirsLines(b));
        break;
      case "base":
        resultLines.push(...getBaseLines(b));
        break;
      case "both":
        resultLines.push(...getOursLines(b), ...getTheirsLines(b));
        break;
      default:
        // Keep original conflict marker
        resultLines.push(...rawLines.slice(start, end + 1));
        break;
    }

    lastLine = end + 1;
  }

  if (lastLine < rawLines.length) {
    resultLines.push(...rawLines.slice(lastLine));
  }

  currentResult.value = resultLines.join("\n");
}

function resolveBlock(index: number, choice: "ours" | "theirs" | "base" | "both") {
  blockResolutions.value.set(index, choice);
  assembleResult();
  if (index + 1 < totalBlocks.value) {
    activeBlockIndex.value = index + 1;
  }
}

function resolveAll(choice: "ours" | "theirs") {
  if (!mergeData.value) return;
  for (let i = 0; i < mergeData.value.blocks.length; i++) {
    blockResolutions.value.set(i, choice);
  }
  assembleResult();
}

function resetAll() {
  blockResolutions.value.clear();
  if (mergeData.value) {
    currentResult.value = mergeData.value.currentContent;
  }
}

function prevConflict() {
  if (activeBlockIndex.value > 0) {
    activeBlockIndex.value--;
  }
}

function nextConflict() {
  if (activeBlockIndex.value + 1 < totalBlocks.value) {
    activeBlockIndex.value++;
  }
}

async function handleSave() {
  if (!props.repoPath || !props.filePath || isSaving.value) return;
  isSaving.value = true;
  try {
    const ok = await gitService.saveThreeWayMergeResult(
      props.repoPath,
      props.filePath,
      currentResult.value,
      autoStage.value
    );
    if (ok) {
      emit("notify", t("merge.3way.toast.saved", { file: props.filePath.split("/").pop() || props.filePath }));
      emit("resolved", props.filePath);
      emit("update:open", false);
    }
  } catch (err: any) {
    emit("notify", err?.message || String(err));
  } finally {
    isSaving.value = false;
  }
}

function handleKeydown(e: KeyboardEvent) {
  if (!props.open) return;
  if (e.altKey && e.key === "[") {
    e.preventDefault();
    prevConflict();
  } else if (e.altKey && e.key === "]") {
    e.preventDefault();
    nextConflict();
  }
}

watch(
  () => [props.open, props.filePath],
  ([isOpen]) => {
    if (isOpen) {
      void loadData();
    }
  }
);

onMounted(() => {
  window.addEventListener("keydown", handleKeydown);
  if (props.open) void loadData();
});

onUnmounted(() => {
  window.removeEventListener("keydown", handleKeydown);
});
</script>

<template>
  <UiModal
    :open="open"
    :title="t('merge.3way.title')"
    width="max-w-6xl"
    data-testid="three-way-merge-modal"
    @update:open="emit('update:open', $event)"
  >
    <div class="space-y-3 font-sans">
      <!-- Toolbar Header -->
      <div class="p-3 rounded-xl border border-line bg-surface flex flex-wrap items-center justify-between gap-3 text-xs">
        <div class="flex items-center gap-2 min-w-0">
          <FileCode class="w-4 h-4 text-accent shrink-0" />
          <span class="font-mono font-bold text-ink truncate">{{ filePath }}</span>
          <span
            class="px-2 py-0.5 rounded text-[10px] font-bold shrink-0"
            :class="isFullyResolved ? 'bg-success-bg text-success border border-success/30' : 'bg-danger-bg text-danger border border-danger/30'"
          >
            {{ isFullyResolved ? t('merge.3way.status.resolved') : t('merge.3way.status.conflicts', { count: remainingUnresolved }) }}
          </span>
        </div>

        <!-- Quick bulk and navigation controls -->
        <div class="flex items-center gap-1.5 flex-wrap">
          <div class="flex items-center gap-1 px-2 py-1 rounded-lg border border-line bg-surface">
            <span class="text-[11px] font-mono text-ink-muted">
              {{ activeBlockIndex + 1 }} / {{ Math.max(1, totalBlocks) }}
            </span>
            <button
              type="button"
              :disabled="activeBlockIndex <= 0"
              @click="prevConflict"
              class="p-0.5 rounded hover:bg-raised text-ink disabled:opacity-30 cursor-pointer"
              :title="t('merge.3way.prev.hint')"
            >
              <ChevronUp class="w-3.5 h-3.5" />
            </button>
            <button
              type="button"
              :disabled="activeBlockIndex >= totalBlocks - 1"
              @click="nextConflict"
              class="p-0.5 rounded hover:bg-raised text-ink disabled:opacity-30 cursor-pointer"
              :title="t('merge.3way.next.hint')"
            >
              <ChevronDown class="w-3.5 h-3.5" />
            </button>
          </div>

          <button
            type="button"
            @click="resolveAll('ours')"
            class="px-2.5 py-1 rounded-lg border border-blue/30 bg-blue/10 hover:bg-blue/20 text-blue font-semibold text-[11px] cursor-pointer transition"
          >
            {{ t("merge.3way.accept.all.ours") }}
          </button>
          <button
            type="button"
            @click="resolveAll('theirs')"
            class="px-2.5 py-1 rounded-lg border border-purple/30 bg-purple-bg hover:bg-purple-bg text-purple font-semibold text-[11px] cursor-pointer transition"
          >
            {{ t("merge.3way.accept.all.theirs") }}
          </button>
          <button
            type="button"
            @click="resetAll"
            class="px-2 py-1 rounded-lg border border-line bg-surface hover:bg-raised text-ink-muted text-[11px] cursor-pointer transition"
            :title="t('merge.3way.reset')"
          >
            <RotateCcw class="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      <!-- 3-Pane Comparison Columns -->
      <div class="grid grid-cols-1 md:grid-cols-3 gap-2 text-xs">
        <!-- Pane 1: Ours (Local) -->
        <div class="p-3 rounded-xl border border-blue/30 bg-blue/5 flex flex-col h-64 overflow-hidden">
          <div class="flex items-center justify-between pb-2 border-b border-blue/20 mb-2">
            <span class="font-bold text-blue flex items-center gap-1.5">
              <span class="w-2 h-2 rounded-full bg-blue" />
              <span>{{ t("merge.3way.pane.ours") }}</span>
            </span>
            <button
              v-if="mergeData && mergeData.blocks[activeBlockIndex]"
              type="button"
              @click="resolveBlock(activeBlockIndex, 'ours')"
              class="px-2 py-0.5 rounded bg-blue text-white text-[10px] font-semibold hover:bg-blue-600 transition cursor-pointer"
            >
              {{ t("merge.3way.accept.block") }}
            </button>
          </div>
          <div class="flex-1 overflow-y-auto font-mono text-[11px] leading-relaxed p-2 bg-canvas rounded-lg border border-line text-ink">
            <template v-if="mergeData && mergeData.blocks[activeBlockIndex]">
              <div v-for="(line, idx) in getOursLines(mergeData.blocks[activeBlockIndex])" :key="idx" class="whitespace-pre">
                {{ line || " " }}
              </div>
            </template>
            <div v-else class="text-ink-muted text-center py-8">
              {{ t("common.empty") }}
            </div>
          </div>
        </div>

        <!-- Pane 2: Base (Common Ancestor) -->
        <div class="p-3 rounded-xl border border-line bg-surface flex flex-col h-64 overflow-hidden">
          <div class="flex items-center justify-between pb-2 border-b border-line mb-2">
            <span class="font-bold text-ink-muted flex items-center gap-1.5">
              <span class="w-2 h-2 rounded-full bg-ink-muted" />
              <span>{{ t("merge.3way.pane.base") }}</span>
            </span>
            <button
              v-if="mergeData && mergeData.blocks[activeBlockIndex] && getBaseLines(mergeData.blocks[activeBlockIndex]).length > 0"
              type="button"
              @click="resolveBlock(activeBlockIndex, 'base')"
              class="px-2 py-0.5 rounded bg-surface hover:bg-raised border border-line text-ink text-[10px] font-semibold transition cursor-pointer"
            >
              {{ t("merge.3way.accept.block") }}
            </button>
          </div>
          <div class="flex-1 overflow-y-auto font-mono text-[11px] leading-relaxed p-2 bg-canvas rounded-lg border border-line text-ink-muted">
            <template v-if="mergeData && mergeData.blocks[activeBlockIndex] && getBaseLines(mergeData.blocks[activeBlockIndex]).length > 0">
              <div v-for="(line, idx) in getBaseLines(mergeData.blocks[activeBlockIndex])" :key="idx" class="whitespace-pre">
                {{ line || " " }}
              </div>
            </template>
            <div v-else class="text-ink-muted text-center py-8">
              {{ t("merge.3way.base.unavailable") }}
            </div>
          </div>
        </div>

        <!-- Pane 3: Theirs (Remote / Incoming) -->
        <div class="p-3 rounded-xl border border-purple/30 bg-purple-bg flex flex-col h-64 overflow-hidden">
          <div class="flex items-center justify-between pb-2 border-b border-purple/20 mb-2">
            <span class="font-bold text-purple flex items-center gap-1.5">
              <span class="w-2 h-2 rounded-full bg-purple" />
              <span>{{ t("merge.3way.pane.theirs") }}</span>
            </span>
            <button
              v-if="mergeData && mergeData.blocks[activeBlockIndex]"
              type="button"
              @click="resolveBlock(activeBlockIndex, 'theirs')"
              class="px-2 py-0.5 rounded bg-purple text-white text-[10px] font-semibold hover:bg-purple-600 transition cursor-pointer"
            >
              {{ t("merge.3way.accept.block") }}
            </button>
          </div>
          <div class="flex-1 overflow-y-auto font-mono text-[11px] leading-relaxed p-2 bg-canvas rounded-lg border border-line text-ink">
            <template v-if="mergeData && mergeData.blocks[activeBlockIndex]">
              <div v-for="(line, idx) in getTheirsLines(mergeData.blocks[activeBlockIndex])" :key="idx" class="whitespace-pre">
                {{ line || " " }}
              </div>
            </template>
            <div v-else class="text-ink-muted text-center py-8">
              {{ t("common.empty") }}
            </div>
          </div>
        </div>
      </div>

      <!-- Accept Both Action Bar for current conflict block -->
      <div v-if="mergeData && mergeData.blocks[activeBlockIndex]" class="flex items-center justify-center gap-3 py-1">
        <button
          type="button"
          @click="resolveBlock(activeBlockIndex, 'both')"
          class="px-3 py-1 rounded-lg border border-line bg-surface hover:bg-raised text-xs font-semibold text-ink flex items-center gap-1.5 transition cursor-pointer shadow-xs"
        >
          <Layers class="w-3.5 h-3.5 text-accent" />
          <span>{{ t("merge.3way.accept.both") }}</span>
        </button>
      </div>

      <!-- Result Pane (Live Output & Direct Manual Touch-up) -->
      <div class="space-y-1.5">
        <div class="flex items-center justify-between text-xs">
          <label class="font-bold text-ink flex items-center gap-1.5">
            <Check class="w-4 h-4 text-success" />
            <span>{{ t("merge.3way.pane.result") }}</span>
          </label>
          <span class="text-[11px] text-ink-muted">
            {{ t("merge.3way.editor.hint") }}
          </span>
        </div>
        <textarea
          v-model="currentResult"
          rows="10"
          spellcheck="false"
          class="w-full p-3 rounded-xl bg-canvas border border-line font-mono text-[11px] leading-relaxed text-ink focus:outline-none focus:border-accent transition select-text resize-y"
          :aria-label="t('merge.3way.editor.placeholder')"
          :placeholder="t('merge.3way.editor.placeholder')"
          data-testid="three-way-result-textarea"
        />
      </div>
    </div>

    <!-- Modal Footer -->
    <template #footer>
      <div class="flex items-center justify-between w-full">
        <label class="flex items-center gap-2 text-xs text-ink cursor-pointer select-none">
          <input
            v-model="autoStage"
            type="checkbox"
            class="rounded border-line text-accent focus:ring-accent"
          />
          <span>{{ t("merge.3way.auto.stage") }}</span>
        </label>

        <div class="flex items-center gap-2">
          <UiButton variant="secondary" @click="emit('update:open', false)">
            {{ t("common.cancel") }}
          </UiButton>
          <UiButton
            variant="primary"
            :disabled="isSaving"
            @click="handleSave"
            data-testid="three-way-save-btn"
          >
            <Save class="w-3.5 h-3.5" />
            <span>{{ isSaving ? t("common.loading") : t("merge.3way.save.resolve") }}</span>
          </UiButton>
        </div>
      </div>
    </template>
  </UiModal>
</template>
