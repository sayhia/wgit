<script setup lang="ts">
import { ref, computed, watch } from "vue";
import { GitCompare, RefreshCw, FileText, ArrowLeftRight, Copy, Download, Archive } from "lucide-vue-next";
import UiModal from "../ui/UiModal.vue";
import UiButton from "../ui/UiButton.vue";
import Toast from "../ui/Toast.vue";
import { useToast } from "../../composables/useToast";
import { gitService } from "../../services/gitService";
import { t } from "../../i18n";
import type { CommitItem, DiffComparisonResult, FileStatus } from "../../types";

const props = defineProps<{
  isOpen: boolean;
  repoPath: string;
  baseCommit: CommitItem | null;
  targetCommit: CommitItem | null;
}>();

const emit = defineEmits<{
  (e: "close"): void;
  (e: "swap", base: CommitItem | null, target: CommitItem | null): void;
}>();

const currentBaseCommit = ref<CommitItem | null>(props.baseCommit);
const currentTargetCommit = ref<CommitItem | null>(props.targetCommit);

watch(
  () => [props.baseCommit, props.targetCommit],
  ([b, t]) => {
    currentBaseCommit.value = (b as CommitItem) ?? null;
    currentTargetCommit.value = (t as CommitItem) ?? null;
  },
  { immediate: true }
);

const { toastText, showToast: toastVisible, notify: showToast } = useToast(2000);

function handleSwapBaseTarget() {
  const tmp = currentBaseCommit.value;
  currentBaseCommit.value = currentTargetCommit.value;
  currentTargetCommit.value = tmp;
  emit("swap", currentBaseCommit.value, currentTargetCommit.value);
  void loadDiff();
}

async function handleCopyPatch() {
  const base = currentBaseCommit.value;
  const target = currentTargetCommit.value;
  if (!props.repoPath || !base?.hash || !target?.hash) return;
  try {
    let diff = diffResult.value?.diff || "";
    if (!diff) {
      diff = await gitService.generatePatchRange(props.repoPath, base.hash, target.hash);
    }
    if (diff) {
      await navigator.clipboard.writeText(diff);
      showToast(t("history.compare.toast.patch.copied"));
    }
  } catch (err: any) {
    showToast(t("history.compare.toast.patch.export.failed", { error: err?.message || err }));
  }
}

async function handleExportPatch() {
  const base = currentBaseCommit.value;
  const target = currentTargetCommit.value;
  if (!props.repoPath || !base?.hash || !target?.hash) return;
  try {
    const dir = await gitService.pickFolderDialog();
    if (!dir) return;
    const startShort = base.shortHash || base.hash.slice(0, 7);
    const endShort = target.shortHash || target.hash.slice(0, 7);
    const outputPath = `${dir.replace(/[/\\]+$/, "")}/${startShort}..${endShort}.patch`;
    await gitService.exportPatchRange(
      props.repoPath,
      base.hash,
      target.hash,
      outputPath
    );
    showToast(t("history.compare.toast.patch.exported", { path: outputPath }));
  } catch (err: any) {
    showToast(t("history.compare.toast.patch.export.failed", { error: err?.message || err }));
  }
}

const isExportingArchive = ref(false);

async function handleExportChangedFilesArchive() {
  const base = currentBaseCommit.value;
  const target = currentTargetCommit.value;
  if (!props.repoPath || !base?.hash || !target?.hash || isExportingArchive.value) return;
  isExportingArchive.value = true;
  try {
    const dir = await gitService.pickFolderDialog();
    if (!dir) return;
    const startShort = base.shortHash || base.hash.slice(0, 7);
    const endShort = target.shortHash || target.hash.slice(0, 7);
    const outputPath = `${dir.replace(/[/\\]+$/, "")}/changed-files-${startShort}..${endShort}.zip`;
    const res = await gitService.exportChangedFilesArchive(
      props.repoPath,
      base.hash,
      target.hash,
      "zip",
      outputPath
    );
    showToast(t("history.compare.toast.archive.exported", { count: res.fileCount, size: res.formattedSize }));
  } catch (err: any) {
    showToast(t("history.compare.toast.archive.failed", { error: err?.message || err }));
  } finally {
    isExportingArchive.value = false;
  }
}

const loading = ref(false);
const error = ref<string | null>(null);
const diffResult = ref<DiffComparisonResult | null>(null);
const selectedFilePath = ref<string | null>(null);
const viewMode = ref<"file" | "unified">("file");
const patchWrap = ref(false);
const fileSearch = ref("");
const fileDiffOverrides = ref<Record<string, string>>({});
const isFetchingFileDiff = ref(false);

const PATCH_META_PREFIXES = [
  "diff --git",
  "index ",
  "new file mode",
  "deleted file mode",
  "old mode",
  "new mode",
  "similarity index",
  "dissimilarity index",
  "rename from",
  "rename to",
  "copy from",
  "copy to",
  "Binary files ",
  "\\ No newline",
];

type PatchLineKind = "hunk" | "meta" | "add" | "del" | "context";

function classifyPatchLine(text: string): PatchLineKind {
  if (text.startsWith("@@")) return "hunk";
  if (text.startsWith("+++") || text.startsWith("---")) return "meta";
  if (PATCH_META_PREFIXES.some((prefix) => text.startsWith(prefix))) return "meta";
  if (text.startsWith("+")) return "add";
  if (text.startsWith("-")) return "del";
  return "context";
}

function patchLineClass(kind: PatchLineKind): string {
  switch (kind) {
    case "add":
      return "bg-success/10 text-success";
    case "del":
      return "bg-danger/10 text-danger";
    case "hunk":
      return "bg-accent/10 text-accent border-y border-accent/20";
    case "meta":
      return "text-ink-muted";
    default:
      return "text-ink-secondary";
  }
}

function getFileStatusBadge(file: FileStatus) {
  const s = file.stagedStatus || file.unstagedStatus;
  if (s) {
    switch (s) {
      case "A":
        return { label: "A", bg: "bg-success/20 text-success" };
      case "D":
        return { label: "D", bg: "bg-danger/20 text-danger" };
      case "R":
        return { label: "R", bg: "bg-purple/20 text-purple" };
      default:
        return { label: "M", bg: "bg-warning/20 text-warning" };
    }
  }
  if (file.additions > 0 && file.deletions === 0) {
    return { label: "A", bg: "bg-success/20 text-success" };
  }
  if (file.deletions > 0 && file.additions === 0) {
    return { label: "D", bg: "bg-danger/20 text-danger" };
  }
  return { label: "M", bg: "bg-warning/20 text-warning" };
}

function extractFileDiff(fullDiff: string, filePath: string): string {
  if (!fullDiff || !filePath) return "";
  const chunks = fullDiff.split(/(?=^diff --git )/m);
  for (const chunk of chunks) {
    const firstLine = chunk.split("\n")[0] || "";
    if (
      firstLine.includes(`a/${filePath} `) ||
      firstLine.endsWith(`b/${filePath}`) ||
      chunk.includes(`--- a/${filePath}`) ||
      chunk.includes(`+++ b/${filePath}`)
    ) {
      return chunk;
    }
  }
  return "";
}

async function selectFile(path: string) {
  selectedFilePath.value = path;
  viewMode.value = "file";
  const extracted = extractFileDiff(diffResult.value?.diff || "", path);
  const base = currentBaseCommit.value ?? props.baseCommit;
  const target = currentTargetCommit.value ?? props.targetCommit;
  if (
    !extracted &&
    !fileDiffOverrides.value[path] &&
    base &&
    target &&
    props.repoPath
  ) {
    isFetchingFileDiff.value = true;
    try {
      const fDiff = await gitService.getFileDiffBetween(
        props.repoPath,
        base.hash,
        target.hash,
        path,
        false
      );
      if (fDiff) {
        fileDiffOverrides.value[path] = fDiff;
      }
    } catch {
      // fallback to whatever is available
    } finally {
      isFetchingFileDiff.value = false;
    }
  }
}

async function loadDiff() {
  const base = currentBaseCommit.value ?? props.baseCommit;
  const target = currentTargetCommit.value ?? props.targetCommit;
  if (!props.isOpen || !props.repoPath || !base?.hash || !target?.hash) {
    return;
  }
  loading.value = true;
  error.value = null;
  fileDiffOverrides.value = {};
  try {
    const res = await gitService.getDiffBetween(
      props.repoPath,
      base.hash,
      target.hash
    );
    diffResult.value = res;
    if (res?.files && res.files.length > 0 && res.files[0]) {
      selectedFilePath.value = res.files[0].path;
    } else {
      selectedFilePath.value = null;
    }
  } catch (err: any) {
    error.value = err?.message || String(err);
    diffResult.value = null;
  } finally {
    loading.value = false;
  }
}

watch(
  () => [props.isOpen, props.repoPath, props.baseCommit?.hash, props.targetCommit?.hash],
  ([open]) => {
    if (open) {
      loadDiff();
    } else {
      diffResult.value = null;
      selectedFilePath.value = null;
      error.value = null;
      fileDiffOverrides.value = {};
    }
  },
  { immediate: true }
);

const filteredFiles = computed(() => {
  const files = diffResult.value?.files || [];
  const q = fileSearch.value.trim().toLowerCase();
  if (!q) return files;
  return files.filter((f) => f.path.toLowerCase().includes(q));
});

const currentDiffText = computed<string>(() => {
  if (viewMode.value === "unified") {
    return diffResult.value?.diff ?? "";
  }
  if (!selectedFilePath.value) {
    return diffResult.value?.diff ?? "";
  }
  if (fileDiffOverrides.value[selectedFilePath.value]) {
    return fileDiffOverrides.value[selectedFilePath.value] ?? "";
  }
  return extractFileDiff(diffResult.value?.diff || "", selectedFilePath.value);
});

const patchIsBinary = computed(() => {
  const text = currentDiffText.value;
  return /^Binary files /m.test(text) || text.includes("GIT binary patch");
});

const patchLines = computed(() => {
  const text = currentDiffText.value;
  if (!text) return [];
  const raw = text.split("\n");
  if (raw.length > 0 && raw[raw.length - 1] === "") raw.pop();
  return raw.map((line) => ({ text: line, kind: classifyPatchLine(line) }));
});
</script>

<template>
  <UiModal
    :open="isOpen"
    :title="t('history.compare.modal.title')"
    width="max-w-5xl"
    body-class="flex flex-col gap-3 p-5 max-h-[85vh] overflow-hidden"
    @update:open="emit('close')"
  >
    <!-- Header comparing Base Commit vs Target Commit -->
    <div class="flex flex-col md:flex-row items-stretch md:items-center gap-3 p-3 rounded-xl border border-line bg-surface/50 shrink-0">
      <!-- Base Commit (Older) -->
      <div class="flex flex-col gap-1 min-w-0 flex-1">
        <div class="flex items-center gap-2">
          <span
            class="px-1.5 py-0.5 rounded text-[10px] font-bold uppercase tracking-wider bg-accent/15 text-accent border border-accent/30 shrink-0"
          >
            {{ t("history.compare.modal.base") }}
          </span>
          <span class="font-mono text-xs font-bold text-accent shrink-0">
            {{ currentBaseCommit?.shortHash || baseCommit?.shortHash || currentBaseCommit?.hash?.slice(0, 7) }}
          </span>
          <span class="text-[10px] text-ink-muted font-mono truncate">
            {{ (currentBaseCommit ?? baseCommit)?.relativeDate || (currentBaseCommit ?? baseCommit)?.date?.split("T")[0] }}
          </span>
        </div>
        <div class="text-xs font-medium text-ink truncate" :title="(currentBaseCommit ?? baseCommit)?.subject">
          {{ (currentBaseCommit ?? baseCommit)?.subject }}
        </div>
        <div class="text-[10px] text-ink-muted truncate">
          {{ (currentBaseCommit ?? baseCommit)?.authorName }}
        </div>
      </div>

      <!-- Swap Base <-> Target Action -->
      <div class="flex items-center justify-center shrink-0">
        <button
          type="button"
          @click="handleSwapBaseTarget"
          class="flex items-center gap-1 px-2.5 py-1.5 rounded-lg border border-line bg-canvas hover:bg-raised text-xs font-medium text-ink transition cursor-pointer shadow-xs active:scale-95"
          :title="t('history.compare.modal.swap.hint')"
        >
          <ArrowLeftRight class="w-3.5 h-3.5 text-accent" />
          <span class="text-[11px] font-semibold">{{ t("history.compare.modal.swap") }}</span>
        </button>
      </div>

      <!-- Target Commit (Newer) -->
      <div class="flex flex-col gap-1 min-w-0 flex-1">
        <div class="flex items-center gap-2">
          <span
            class="px-1.5 py-0.5 rounded text-[10px] font-bold uppercase tracking-wider bg-cyan/15 text-cyan border border-cyan/30 shrink-0"
          >
            {{ t("history.compare.modal.target") }}
          </span>
          <span class="font-mono text-xs font-bold text-cyan shrink-0">
            {{ currentTargetCommit?.shortHash || targetCommit?.shortHash || currentTargetCommit?.hash?.slice(0, 7) }}
          </span>
          <span class="text-[10px] text-ink-muted font-mono truncate">
            {{ (currentTargetCommit ?? targetCommit)?.relativeDate || (currentTargetCommit ?? targetCommit)?.date?.split("T")[0] }}
          </span>
        </div>
        <div class="text-xs font-medium text-ink truncate" :title="(currentTargetCommit ?? targetCommit)?.subject">
          {{ (currentTargetCommit ?? targetCommit)?.subject }}
        </div>
        <div class="text-[10px] text-ink-muted truncate">
          {{ (currentTargetCommit ?? targetCommit)?.authorName }}
        </div>
      </div>
    </div>

    <!-- Comparison Summary & Actions Bar -->
    <div class="flex flex-wrap items-center justify-between gap-2 shrink-0 px-0.5">
      <div class="flex items-center gap-2">
        <span class="text-xs font-semibold text-ink">
          {{ t("history.compare.modal.files", { count: diffResult?.files?.length || 0 }) }}
        </span>
        <span v-if="diffResult" class="text-xs font-mono text-success font-semibold">
          +{{ diffResult.additions }}
        </span>
        <span v-if="diffResult" class="text-xs font-mono text-danger font-semibold">
          -{{ diffResult.deletions }}
        </span>
      </div>

      <div class="flex items-center gap-2 flex-wrap">
        <!-- Copy & Export Patch shortcuts -->
        <button
          type="button"
          @click="handleCopyPatch"
          class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-canvas hover:bg-raised text-[11px] font-medium text-ink transition cursor-pointer border border-line"
          :title="t('history.compare.modal.copy.patch.hint')"
        >
          <Copy class="w-3.5 h-3.5 text-purple" />
          <span>{{ t("history.compare.modal.copy.patch") }}</span>
        </button>

        <button
          type="button"
          @click="handleExportPatch"
          class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-canvas hover:bg-raised text-[11px] font-medium text-ink transition cursor-pointer border border-line"
          :title="t('history.compare.modal.export.patch.hint')"
        >
          <Download class="w-3.5 h-3.5 text-success" />
          <span>{{ t("history.compare.modal.export.patch") }}</span>
        </button>

        <button
          type="button"
          @click="handleExportChangedFilesArchive"
          :disabled="isExportingArchive"
          class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-canvas hover:bg-raised text-[11px] font-medium text-ink transition cursor-pointer border border-line disabled:opacity-50"
          :title="t('history.compare.modal.export.archive.hint')"
          data-testid="export-archive-btn"
        >
          <Archive class="w-3.5 h-3.5 text-accent" />
          <span>{{ isExportingArchive ? t("history.compare.modal.exporting.archive") : t("history.compare.modal.export.archive") }}</span>
        </button>

        <div class="flex items-center bg-canvas p-0.5 rounded-lg border border-line">
          <button
            @click="viewMode = 'file'"
            class="px-2.5 py-0.5 rounded text-[11px] font-medium transition cursor-pointer"
            :class="viewMode === 'file' ? 'bg-raised text-accent font-semibold shadow-xs' : 'text-ink-muted hover:text-ink'"
          >
            {{ t("history.compare.modal.view.file") }}
          </button>
          <button
            @click="viewMode = 'unified'"
            class="px-2.5 py-0.5 rounded text-[11px] font-medium transition cursor-pointer"
            :class="viewMode === 'unified' ? 'bg-raised text-accent font-semibold shadow-xs' : 'text-ink-muted hover:text-ink'"
          >
            {{ t("history.compare.modal.view.unified") }}
          </button>
        </div>

        <button
          @click="patchWrap = !patchWrap"
          class="px-2 py-0.5 rounded text-[10px] font-mono border border-line bg-canvas hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer"
        >
          Wrap: {{ patchWrap ? "ON" : "OFF" }}
        </button>
      </div>
    </div>

    <!-- Main Content Area -->
    <div v-if="loading" class="flex flex-col items-center justify-center py-16 gap-3 text-ink-muted">
      <RefreshCw class="w-6 h-6 animate-spin text-accent" />
      <span class="text-xs">{{ t("history.compare.modal.loading") }}</span>
    </div>

    <div v-else-if="error" class="p-4 rounded-xl border border-danger/30 bg-danger/10 text-danger text-xs">
      {{ error }}
    </div>

    <div
      v-else-if="!diffResult || diffResult.files.length === 0"
      class="py-16 text-center text-xs text-ink-muted"
    >
      {{ t("history.compare.modal.no.diff") }}
    </div>

    <div v-else class="flex-1 min-h-0 grid grid-cols-1 md:grid-cols-12 gap-3 overflow-hidden">
      <!-- File list column -->
      <div
        class="md:col-span-4 flex flex-col gap-2 min-h-0 overflow-hidden rounded-xl border border-line bg-surface/30 p-2.5"
      >
        <div class="relative shrink-0">
          <input
            v-model="fileSearch"
            type="text"
            spellcheck="false"
            :placeholder="t('history.compare.modal.files', { count: diffResult.files.length })"
            class="w-full px-2.5 py-1 rounded-lg bg-canvas border border-line text-[11px] text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
          />
        </div>

        <div class="flex-1 min-h-0 overflow-y-auto space-y-1 pr-1">
          <div
            v-for="file in filteredFiles"
            :key="file.path"
            @click="selectFile(file.path)"
            class="flex items-center gap-2 px-2.5 py-1.5 rounded-lg text-xs cursor-pointer transition"
            :class="
              selectedFilePath === file.path && viewMode === 'file'
                ? 'bg-accent/15 border border-accent/40 font-medium text-ink'
                : 'hover:bg-raised text-ink-secondary border border-transparent'
            "
          >
            <span
              class="shrink-0 px-1 py-0.5 rounded text-[9px] font-bold font-mono"
              :class="getFileStatusBadge(file).bg"
            >
              {{ getFileStatusBadge(file).label }}
            </span>
            <span class="truncate flex-1 font-mono text-[11px]" :title="file.path">
              {{ file.path }}
            </span>
            <span v-if="file.additions > 0" class="text-[10px] font-mono text-success shrink-0">
              +{{ file.additions }}
            </span>
            <span v-if="file.deletions > 0" class="text-[10px] font-mono text-danger shrink-0">
              -{{ file.deletions }}
            </span>
          </div>

          <div
            v-if="filteredFiles.length === 0"
            class="py-6 text-center text-xs text-ink-muted"
          >
            {{ t("history.compare.modal.no.diff") }}
          </div>
        </div>
      </div>

      <!-- Diff preview column -->
      <div
        class="md:col-span-8 flex flex-col gap-2 min-h-0 overflow-hidden rounded-xl border border-line bg-surface/30 p-2.5"
      >
        <div class="flex items-center justify-between gap-2 px-1 shrink-0">
          <span class="text-xs font-mono font-medium text-ink truncate" :title="selectedFilePath || ''">
            {{ viewMode === "unified" ? t("history.compare.modal.view.unified") : (selectedFilePath || "") }}
          </span>
          <RefreshCw
            v-if="isFetchingFileDiff"
            class="w-3.5 h-3.5 animate-spin text-accent shrink-0"
          />
        </div>

        <div
          class="flex-1 min-h-0 overflow-auto rounded-lg border border-line bg-canvas/70 font-mono text-[11px] leading-relaxed select-text"
        >
          <div v-if="patchIsBinary" class="py-12 text-center text-xs text-ink-muted">
            {{ t("history.inspector.patch.binary") }}
          </div>
          <div v-else-if="patchLines.length === 0" class="py-12 text-center text-xs text-ink-muted">
            {{ t("history.compare.modal.no.diff") }}
          </div>
          <div
            v-else
            v-for="(line, lineIdx) in patchLines"
            :key="lineIdx"
            class="flex items-start px-2 py-px"
            :class="patchLineClass(line.kind)"
          >
            <span class="w-10 shrink-0 select-none text-right pr-2 text-ink-muted text-[10px]">
              {{ lineIdx + 1 }}
            </span>
            <span
              class="flex-1 min-w-0"
              :class="patchWrap ? 'whitespace-pre-wrap break-all' : 'whitespace-pre'"
            >{{ line.text || " " }}</span>
          </div>
        </div>
      </div>
    </div>

    <template #footer>
      <UiButton variant="secondary" @click="emit('close')">
        {{ t("common.close") }}
      </UiButton>
    </template>
  </UiModal>
</template>
