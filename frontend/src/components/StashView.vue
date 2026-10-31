<script setup lang="ts">
import { confirmAction } from "../composables/useConfirm";
import { t } from "../i18n";
import { ref, computed, watch } from "vue";
import {
  Archive,
  Plus,
  Trash2,
  Play,
  RotateCcw,
  GitBranch,
  Search,
  RefreshCw,
  List,
  Columns,
  Clock,
  FileText,
  Copy,
  Sparkles,
  GitCompare,
  CheckCircle2,
} from "lucide-vue-next";
import { gitService } from "../services/gitService";
import { useContextMenu, type ContextMenuItem } from "../composables/useContextMenu";
import UiContextMenu from "./ui/UiContextMenu.vue";
import UiModal from "./ui/UiModal.vue";
import UiButton from "./ui/UiButton.vue";
import Toast from "./ui/Toast.vue";
import { useToast, reportError } from "../composables/useToast";
import UiSkeleton from "./ui/UiSkeleton.vue";
import UiEmpty from "./ui/UiEmpty.vue";
import type { StashInfo, StashFileEntry, StashDiffReport } from "../types";
import { useRovingList } from "../composables/useRovingList";

const props = defineProps<{
  currentRepoPath: string;
}>();

const emit = defineEmits<{
  (e: "refresh"): void;
}>();

const stashes = ref<StashInfo[]>([]);
const selectedStash = ref<StashInfo | null>(null);
const stashSearch = ref("");
const isLoading = ref(false);
const diffText = ref("");
const isLoadingDiff = ref(false);
const activeDiffMode = ref<"unified" | "split">("unified");

// Per-file restore
const stashFiles = ref<StashFileEntry[]>([]);
const isLoadingFiles = ref(false);
const showFileList = ref(false);
const restoringPath = ref("");
let repoEpoch = 0;

async function loadStashFiles(stash: StashInfo) {
  if (!props.currentRepoPath) return;
  const repoPath = props.currentRepoPath;
  const epoch = repoEpoch;
  isLoadingFiles.value = true;
  try {
    const files = await gitService.getStashFiles(repoPath, stash.index);
    if (epoch === repoEpoch && selectedStash.value?.index === stash.index) {
      stashFiles.value = files || [];
    }
  } catch (err: any) {
    if (epoch === repoEpoch) notify(t("stash.toast.read.files.failed", { error: err?.message || err }));
  } finally {
    if (epoch === repoEpoch) isLoadingFiles.value = false;
  }
}

function stashFileStatusClass(status: string) {
  if (status === "A") return "bg-success-bg text-success";
  if (status === "D") return "bg-danger-bg text-danger";
  if (status === "R") return "bg-purple-bg text-purple";
  return "bg-warning-bg text-warning";
}

async function handleRestoreFile(file: StashFileEntry) {
  if (!props.currentRepoPath || !selectedStash.value) return;
  if (!(await confirmAction({ message: t("stash.confirm.restore.file", { index: selectedStash.value.index, file: file.path }), tone: "danger" }))) return;
  restoringPath.value = file.path;
  try {
    const res = await gitService.applyStashFile(props.currentRepoPath, selectedStash.value.index, file.path);
    notify(res.message || t("stash.toast.file.restored", { file: file.path.split("/").pop() ?? file.path }));
    emit("refresh");
    await loadStashFiles(selectedStash.value);
  } catch (err: any) {
    notify(t("stash.toast.file.restore.failed", { error: err?.message || err }));
  } finally {
    restoringPath.value = "";
  }
}

// Modals
const showSaveModal = ref(false);
const saveMessage = ref("");
const saveIncludeUntracked = ref(true);
const saveIncludeAll = ref(false);
const saveKeepIndex = ref(false);
const saveStagedOnly = ref(false);
const isSaving = ref(false);

const showBranchModal = ref(false);
const newBranchName = ref("");
const isBranching = ref(false);

// Phase 57: Stash Sandbox & Comparator
const showComparatorModal = ref(false);
const comparatorRef1 = ref("stash@{0}");
const comparatorRef2 = ref("stash@{1}");
const comparatorReport = ref<StashDiffReport | null>(null);
const isComparingStashes = ref(false);
const isCreatingSnapshot = ref(false);
const isApplyingHunk = ref(false);

async function handleCreateMicroSnapshot() {
  if (!props.currentRepoPath || isCreatingSnapshot.value) return;
  isCreatingSnapshot.value = true;
  try {
    const res = await gitService.createMicroSnapshot(props.currentRepoPath);
    if (res) {
      notify(res.summary);
      emit("refresh");
      await loadStashes();
    }
  } catch (err: any) {
    /* best-effort snapshot */
    notify(String(err?.message || err));
  } finally {
    isCreatingSnapshot.value = false;
  }
}

async function runStashComparison() {
  if (!props.currentRepoPath || isComparingStashes.value) return;
  isComparingStashes.value = true;
  try {
    const rep = await gitService.compareStashes(
      props.currentRepoPath,
      comparatorRef1.value,
      comparatorRef2.value
    );
    comparatorReport.value = rep;
  } catch (err: any) {
    /* best-effort compare */
    notify(String(err?.message || err));
    comparatorReport.value = null;
  } finally {
    isComparingStashes.value = false;
  }
}

function openStashComparator() {
  showComparatorModal.value = true;
  if (stashes.value.length >= 2) {
    comparatorRef1.value = `stash@{${stashes.value[0]?.index ?? 0}}`;
    comparatorRef2.value = `stash@{${stashes.value[1]?.index ?? 1}}`;
  }
  void runStashComparison();
}

async function handleApplyStashHunk(filePath: string) {
  if (!props.currentRepoPath || isApplyingHunk.value) return;
  isApplyingHunk.value = true;
  try {
    const idx = selectedStash.value ? selectedStash.value.index : 0;
    const ok = await gitService.applyStashHunk(props.currentRepoPath, idx, filePath, 0);
    if (ok) {
      notify(t("stash.toast.file.restored", { file: filePath }));
      emit("refresh");
    }
  } catch (err: any) {
    /* best-effort apply hunk */
    notify(String(err?.message || err));
  } finally {
    isApplyingHunk.value = false;
  }
}

// Context Menu
const {
  isOpen: stashMenuOpen,
  position: stashMenuPos,
  items: stashMenuList,
  open: openMenuAt,
  close: closeStashMenu,
} = useContextMenu();
const menuStash = ref<StashInfo | null>(null);

function stashMenuItems(stash: StashInfo): ContextMenuItem[] {
  return [
    { id: "apply", label: t("stash.action.apply"), icon: RotateCcw },
    { id: "pop", label: t("stash.action.pop"), icon: Play },
    { id: "branch", label: t("stash.action.branch"), icon: GitBranch },
    { id: "copy", label: t("stash.action.copy"), icon: Copy, hint: `stash@{${stash.index}}`, separatorBefore: true },
    { id: "drop", label: t("stash.action.drop"), icon: Trash2, danger: true, separatorBefore: true },
  ];
}

function openStashMenu(stash: StashInfo, event: MouseEvent) {
  menuStash.value = stash;
  openMenuAt(event, stashMenuItems(stash));
}

async function handleStashMenuSelect(id: string) {
  const stash = menuStash.value;
  if (!stash) return;
  switch (id) {
    case "apply":
      await handleApplyStash(stash.index);
      break;
    case "pop":
      await handlePopStash(stash.index);
      break;
    case "branch":
      openBranchModal(stash);
      break;
    case "copy":
      await copyStashRef(stash.index);
      break;
    case "drop":
      await handleDropStash(stash.index);
      break;
  }
}

async function copyStashRef(index: number) {
  const ref = `stash@{${index}}`;
  await navigator.clipboard.writeText(ref);
  notify(t("stash.toast.copied", { ref }));
}


// Toast
const { toastText, showToast, notify } = useToast();
// Filtered stashes
const filteredStashes = computed(() => {
  if (!stashSearch.value.trim()) return stashes.value;
  const q = stashSearch.value.toLowerCase();
  return stashes.value.filter(
    (s) =>
      s.message.toLowerCase().includes(q) ||
      `stash@{${s.index}}`.includes(q) ||
      s.date.toLowerCase().includes(q)
  );
});

// Keyboard parity with StashesList: the cards carry their own action buttons
// (pop/apply/branch/copy/drop), so one card is tabbable at a time and ↑/↓
// moves between cards; Enter performs the row's primary action (select).
const {
  setItemRef: setStashItemRef,
  tabindexFor: stashTabindexFor,
  onKeydown: onStashRowKeydown,
  onFocus: onStashRowFocus,
} = useRovingList(() => filteredStashes.value, (s) => String(s.index));

// Load stashes
async function loadStashes() {
  if (!props.currentRepoPath) return;
  const repoPath = props.currentRepoPath;
  const epoch = repoEpoch;
  isLoading.value = true;
  try {
    const list = await gitService.getStashes(repoPath);
    if (epoch !== repoEpoch) return;
    stashes.value = list || [];
    if (stashes.value.length > 0) {
      if (
        !selectedStash.value ||
        !stashes.value.some((s) => s.index === selectedStash.value?.index)
      ) {
        const first = stashes.value[0];
        if (first) selectStash(first);
      } else {
        const found = stashes.value.find((s) => s.index === selectedStash.value?.index);
        if (found) selectStash(found);
      }
    } else {
      selectedStash.value = null;
      diffText.value = "";
    }
  } catch (err) {
    reportError("Failed to load stashes", err);
  } finally {
    if (epoch === repoEpoch) isLoading.value = false;
  }
}

// Select a stash and load diff
async function selectStash(stash: StashInfo) {
  selectedStash.value = stash;
  showFileList.value = false;
  stashFiles.value = [];
  if (!props.currentRepoPath) return;
  const repoPath = props.currentRepoPath;
  const epoch = repoEpoch;
  void loadStashFiles(stash);
  isLoadingDiff.value = true;
  try {
    const diff = await gitService.getStashDiff(repoPath, stash.index);
    if (epoch !== repoEpoch) return;
    diffText.value = diff || t("stash.diff.empty");
  } catch (err: any) {
    if (epoch === repoEpoch) diffText.value = t("stash.diff.failed", { error: err?.message || err });
  } finally {
    if (epoch === repoEpoch) isLoadingDiff.value = false;
  }
}

// Save Stash
async function handleSaveStash() {
  if (!props.currentRepoPath) return;
  isSaving.value = true;
  try {
    if (saveStagedOnly.value) {
      await gitService.stashStagedOnly(props.currentRepoPath, saveMessage.value.trim());
      notify(t("stash.toast.staged.only"));
    } else {
      await gitService.stashSaveAdvanced(
        props.currentRepoPath,
        saveMessage.value.trim(),
        saveKeepIndex.value,
        saveIncludeUntracked.value,
        saveIncludeAll.value
      );
      notify(t("stash.toast.saved"));
    }
    saveMessage.value = "";
    saveIncludeAll.value = false;
    showSaveModal.value = false;
    emit("refresh");
    await loadStashes();
  } catch (err: any) {
    notify(t("stash.toast.save.failed", { error: err?.message || err }));
  } finally {
    isSaving.value = false;
  }
}

// Apply Stash
async function handleApplyStash(index: number) {
  if (!props.currentRepoPath) return;
  try {
    await gitService.stashApply(props.currentRepoPath, index);
    notify(t("stash.toast.applied", { index }));
    emit("refresh");
  } catch (err: any) {
    notify(t("stash.toast.apply.failed", { error: err?.message || err }));
  }
}

// Pop Stash
async function handlePopStash(index: number) {
  if (!props.currentRepoPath) return;
  try {
    await gitService.stashPop(props.currentRepoPath, index);
    notify(t("stash.toast.popped", { index }));
    emit("refresh");
    await loadStashes();
  } catch (err: any) {
    notify(t("stash.toast.pop.failed", { error: err?.message || err }));
  }
}

// Drop Stash
async function handleDropStash(index: number) {
  if (!props.currentRepoPath) return;
  if (!(await confirmAction({ message: t("stash.confirm.drop", { index }), tone: "danger" }))) return;
  try {
    await gitService.stashDrop(props.currentRepoPath, index);
    notify(t("stash.toast.dropped", { index }));
    await loadStashes();
  } catch (err: any) {
    notify(t("stash.toast.drop.failed", { error: err?.message || err }));
  }
}

// Clear All Stashes
async function handleClearAll() {
  if (!props.currentRepoPath) return;
  if (
    !(await confirmAction({
      message: t("stash.confirm.clear"),
      tone: "danger",
      requireText: t("stash.confirm.clear.require"),
      requireTextHint: t("stash.confirm.clear.hint"),
      // Stashes are the one thing git cannot recover: say how many are going.
      affectedSummary: t("stash.confirm.clear.scope", { count: stashes.value.length }),
    }))
  )
    return;
  try {
    await gitService.stashClear(props.currentRepoPath);
    notify(t("stash.toast.cleared"));
    await loadStashes();
  } catch (err: any) {
    notify(t("stash.toast.clear.failed", { error: err?.message || err }));
  }
}

// Branch from Stash
function openBranchModal(stash: StashInfo) {
  selectedStash.value = stash;
  newBranchName.value = `stash-${stash.index}-recovery`;
  showBranchModal.value = true;
}

async function handleBranchFromStash() {
  if (!props.currentRepoPath || !selectedStash.value || !newBranchName.value.trim()) return;
  isBranching.value = true;
  try {
    await gitService.stashBranch(props.currentRepoPath, selectedStash.value.index, newBranchName.value.trim());
    notify(t("stash.toast.branch.created", { branch: newBranchName.value.trim() }));
    showBranchModal.value = false;
    emit("refresh");
    await loadStashes();
  } catch (err: any) {
    notify(t("stash.toast.branch.failed", { error: err?.message || err }));
  } finally {
    isBranching.value = false;
  }
}

// Parsed Diff Lines
const parsedDiffLines = computed(() => {
  if (!diffText.value) return [];
  const lines = diffText.value.split("\n");
  let oldLineNo = 0;
  let newLineNo = 0;

  return lines.map((text, idx) => {
    let type: "header" | "add" | "del" | "ctx" = "ctx";
    if (text.startsWith("@@")) {
      type = "header";
      const match = text.match(/@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@/);
      if (match && match[1] && match[2]) {
        oldLineNo = parseInt(match[1], 10);
        newLineNo = parseInt(match[2], 10);
      }
    } else if (text.startsWith("+") && !text.startsWith("+++")) {
      type = "add";
      newLineNo++;
    } else if (text.startsWith("-") && !text.startsWith("---")) {
      type = "del";
      oldLineNo++;
    } else if (!text.startsWith("---") && !text.startsWith("+++") && !text.startsWith("diff ")) {
      type = "ctx";
      oldLineNo++;
      newLineNo++;
    }
    return {
      index: idx,
      text,
      type,
      oldLineNo: type === "add" ? "" : oldLineNo,
      newLineNo: type === "del" ? "" : newLineNo,
    };
  });
});

// Parsed Split Diff Rows
interface SplitRow {
  index: number;
  isHeader: boolean;
  headerText?: string;
  left: { lineNo: number | ""; text: string; type: "del" | "ctx" | "empty" };
  right: { lineNo: number | ""; text: string; type: "add" | "ctx" | "empty" };
}

const parsedSplitRows = computed<SplitRow[]>(() => {
  if (!diffText.value) return [];
  const rawLines = diffText.value.split("\n");
  const rows: SplitRow[] = [];
  let oldNo = 0;
  let newNo = 0;
  let pendingDels: { lineNo: number; text: string }[] = [];
  let pendingAdds: { lineNo: number; text: string }[] = [];

  function flushPending() {
    const maxLen = Math.max(pendingDels.length, pendingAdds.length);
    for (let i = 0; i < maxLen; i++) {
      const del = pendingDels[i];
      const add = pendingAdds[i];
      rows.push({
        index: rows.length,
        isHeader: false,
        left: del ? { lineNo: del.lineNo, text: del.text, type: "del" } : { lineNo: "", text: "", type: "empty" },
        right: add ? { lineNo: add.lineNo, text: add.text, type: "add" } : { lineNo: "", text: "", type: "empty" },
      });
    }
    pendingDels = [];
    pendingAdds = [];
  }

  for (const text of rawLines) {
    if (text.startsWith("@@")) {
      flushPending();
      const match = text.match(/@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@/);
      if (match && match[1] && match[2]) {
        oldNo = parseInt(match[1], 10);
        newNo = parseInt(match[2], 10);
      }
      rows.push({
        index: rows.length,
        isHeader: true,
        headerText: text,
        left: { lineNo: "", text: "", type: "empty" },
        right: { lineNo: "", text: "", type: "empty" },
      });
    } else if (text.startsWith("+") && !text.startsWith("+++")) {
      pendingAdds.push({ lineNo: newNo, text: text.substring(1) });
      newNo++;
    } else if (text.startsWith("-") && !text.startsWith("---")) {
      pendingDels.push({ lineNo: oldNo, text: text.substring(1) });
      oldNo++;
    } else if (text.startsWith("diff ") || text.startsWith("index ") || text.startsWith("---") || text.startsWith("+++")) {
      flushPending();
    } else {
      flushPending();
      const ctxText = text.startsWith(" ") ? text.substring(1) : text;
      rows.push({
        index: rows.length,
        isHeader: false,
        left: { lineNo: oldNo, text: ctxText, type: "ctx" },
        right: { lineNo: newNo, text: ctxText, type: "ctx" },
      });
      oldNo++;
      newNo++;
    }
  }
  flushPending();
  return rows;
});

function resetRepoScopedState() {
  selectedStash.value = null;
  stashSearch.value = "";
  diffText.value = "";
  stashFiles.value = [];
  showFileList.value = false;
  showSaveModal.value = false;
  saveMessage.value = "";
  saveIncludeAll.value = false;
  showBranchModal.value = false;
  newBranchName.value = "";
  restoringPath.value = "";
  isLoading.value = false;
  isLoadingDiff.value = false;
  isLoadingFiles.value = false;
  isSaving.value = false;
  isBranching.value = false;
}

watch(
  () => props.currentRepoPath,
  () => {
    repoEpoch++;
    resetRepoScopedState();
    loadStashes();
  },
  { immediate: true }
);
</script>

<template>
  <div class="flex-1 flex overflow-hidden bg-canvas text-ink select-none font-sans">
    <!-- ============================================== -->
    <!-- LEFT COLUMN: 储藏快照列表 (Stash List)       -->
    <!-- ============================================== -->
    <div class="w-80 border-r border-line bg-surface flex flex-col overflow-hidden shrink-0">
      <!--
        Header Bar. Two rows on purpose: the sidebar is a fixed 320px, so a
        single row of title + count + four labelled actions used to wrap the
        title («储藏箱 & / 快照») and squeeze the refresh icon against the
        divider. Row 1 keeps identity + low-frequency icon actions, row 2 keeps
        search next to the primary action.
      -->
      <div class="p-3 border-b border-line bg-surface flex flex-col gap-2 shrink-0">
        <div class="flex items-center justify-between gap-2">
          <div class="flex items-center gap-2 min-w-0">
            <Archive class="w-4 h-4 text-accent shrink-0" />
            <h2 class="text-xs font-bold text-ink truncate">{{ t("stash.title") }}</h2>
            <span
              v-if="stashes.length > 0"
              class="px-1.5 py-0.2 rounded-full bg-accent-bg text-accent text-[10px] font-bold shrink-0"
            >
              {{ stashes.length }}
            </span>
          </div>

          <div class="flex items-center gap-0.5 shrink-0">
            <button
              v-if="stashes.length > 0"
              @click="handleClearAll"
              class="p-1.5 rounded-md hover:bg-danger-bg text-ink-muted hover:text-danger transition cursor-pointer"
              :title="t('stash.clear.title')"
              :aria-label="t('stash.clear.title')"
            >
              <Trash2 class="w-3.5 h-3.5" />
            </button>
            <button :aria-label="t('stash.refresh')"
              @click="loadStashes"
              class="p-1.5 rounded-md hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer"
              :title="t('stash.refresh')"
            >
              <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isLoading }" />
            </button>
          </div>
        </div>

        <!-- Search Bar + quick actions -->
        <div class="flex items-center gap-1.5">
          <div class="relative flex-1 min-w-0">
            <input
              v-model="stashSearch"
              type="text"
              :aria-label="t('stash.search.aria')"
              :placeholder="t('stash.search.placeholder')"
              class="w-full pl-7 pr-6 py-1 rounded-md bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent font-mono transition"
            />
            <Search class="w-3.5 h-3.5 text-ink-muted absolute left-2 top-2 pointer-events-none" />
            <button
              v-if="stashSearch"
              @click="stashSearch = ''"
              :aria-label="t('explorer.search.clear')"
              class="absolute right-1.5 top-1.5 text-[10px] text-ink-muted hover:text-ink cursor-pointer font-bold"
            >
              ✕
            </button>
          </div>

          <button
            type="button"
            data-testid="create-micro-snapshot-btn"
            @click="handleCreateMicroSnapshot"
            :disabled="isCreatingSnapshot"
            class="flex items-center justify-center w-7 h-7 rounded-md bg-surface hover:bg-raised border border-line text-ink transition cursor-pointer disabled:opacity-50 shrink-0"
            :title="t('stash.snapshot.btn')"
            :aria-label="t('stash.snapshot.btn')"
          >
            <Sparkles class="w-3.5 h-3.5 text-accent" :class="{ 'animate-spin': isCreatingSnapshot }" />
          </button>
          <button
            type="button"
            data-testid="stash-compare-btn"
            @click="openStashComparator"
            class="flex items-center justify-center w-7 h-7 rounded-md bg-surface hover:bg-raised border border-line text-ink transition cursor-pointer shrink-0"
            :title="t('stash.comparator.title')"
            :aria-label="t('stash.comparator.title')"
          >
            <GitCompare class="w-3.5 h-3.5 text-accent" />
          </button>
          <button
            @click="showSaveModal = true"
            class="flex items-center gap-1 px-2 py-1 rounded-md bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-[11px] font-semibold transition cursor-pointer shadow-xs whitespace-nowrap shrink-0"
            :title="t('stash.new.title')"
          >
            <Plus class="w-3.5 h-3.5" />
            <span>{{ t("stash.new") }}</span>
          </button>
        </div>
      </div>

      <!-- Stash List Cards -->
      <div class="flex-1 overflow-y-auto p-2 space-y-1.5">
        <div
          v-for="(s, index) in filteredStashes"
          :key="s.index"
          :ref="(el) => setStashItemRef(s, el)"
          :tabindex="stashTabindexFor(s, index)"
          @click="selectStash(s)"
          @focus="onStashRowFocus(s)"
          @keydown="onStashRowKeydown($event, index)"
          @keydown.enter.self="selectStash(s)"
          @contextmenu.prevent="openStashMenu(s, $event)"
          class="group p-2.5 rounded-xl border transition cursor-pointer select-none flex flex-col gap-1.5 focus:outline-none focus-visible:ring-2 focus-visible:ring-accent/45"
          :class="
            selectedStash?.index === s.index
              ? 'bg-raised border-accent/50 shadow-sm'
              : 'bg-surface hover:bg-surface border-line hover:border-line/80'
          "
        >
          <!-- Top Row: Index Badge & Actions -->
          <div class="flex items-center justify-between">
            <span
              class="px-1.5 py-0.2 rounded text-[10px] font-mono font-bold"
              :class="
                selectedStash?.index === s.index
                  ? 'bg-accent-strong text-accent-contrast'
                  : 'bg-line/60 text-ink-muted'
              "
            >
              stash@{{ s.index }}
            </span>

            <!-- Quick Action Hover Bar -->
            <div class="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition">
              <button :aria-label="t('stash.action.pop')"
                @click.stop="handlePopStash(s.index)"
                class="p-1 rounded hover:bg-success-bg text-ink-muted hover:text-success transition cursor-pointer"
                :title="t('stash.action.pop')"
              >
                <Play class="w-3 h-3" />
              </button>
              <button :aria-label="t('stash.action.apply')"
                @click.stop="handleApplyStash(s.index)"
                class="p-1 rounded hover:bg-accent-bg text-ink-muted hover:text-accent transition cursor-pointer"
                :title="t('stash.action.apply')"
              >
                <RotateCcw class="w-3 h-3" />
              </button>
              <button :aria-label="t('stash.action.branch')"
                @click.stop="openBranchModal(s)"
                class="p-1 rounded hover:bg-purple-bg text-ink-muted hover:text-purple transition cursor-pointer"
                :title="t('stash.action.branch')"
              >
                <GitBranch class="w-3 h-3" />
              </button>
              <button :aria-label="t('stash.action.copy')"
                @click.stop="copyStashRef(s.index)"
                class="p-1 rounded hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer"
                :title="t('stash.action.copy')"
              >
                <Copy class="w-3 h-3" />
              </button>
              <button :aria-label="t('stash.action.drop')"
                @click.stop="handleDropStash(s.index)"
                class="p-1 rounded hover:bg-danger-bg text-ink-muted hover:text-danger transition cursor-pointer"
                :title="t('stash.action.drop')"
              >
                <Trash2 class="w-3 h-3" />
              </button>
            </div>
          </div>

          <!-- Message -->
          <div class="text-xs font-semibold text-ink line-clamp-2 leading-snug">
            {{ s.message || t("stash.none.message") }}
          </div>

          <!-- Date -->
          <div class="flex items-center gap-1 text-[10px] text-ink-muted font-mono">
            <Clock class="w-3 h-3 shrink-0" />
            <span class="truncate">{{ s.date }}</span>
          </div>
        </div>

        <!-- Loading: card-shaped rows so the list does not jump when it fills -->
        <UiSkeleton v-if="isLoading && stashes.length === 0" variant="rows" :rows="6" :row-height="64" />

        <!-- Empty State in List -->
        <UiEmpty
          v-else-if="stashes.length === 0"
          :icon="Archive"
          :title="t('stash.empty.title')"
          :hint="t('stash.empty.hint')"
          bordered
        />

        <div
          v-else-if="filteredStashes.length === 0 && stashSearch"
          class="py-12 text-center text-xs text-ink-muted"
        >
          {{ t("stash.empty.filtered") }}
        </div>
      </div>
    </div>

    <!-- ============================================== -->
    <!-- RIGHT COLUMN: 储藏快照差异比对主区 (Diff View)  -->
    <!-- ============================================== -->
    <div class="@container/stash-detail flex-1 flex flex-col overflow-hidden bg-canvas min-w-0">
      <!-- Top Action Bar -->
      <div
        v-if="selectedStash"
  class="min-h-12 border-b border-line px-3 py-2 flex flex-wrap items-center justify-between gap-2 bg-surface shrink-0"
      >
        <div class="flex-1 basis-full @min-[1000px]/stash-detail:basis-0 flex items-center gap-3 truncate min-w-0">
          <span class="px-2 py-0.5 rounded bg-accent-bg text-accent font-mono font-bold text-xs shrink-0">
            stash@{{ selectedStash.index }}
          </span>
          <span class="font-bold text-xs text-ink truncate">{{ selectedStash.message }}</span>
        </div>

        <div class="flex flex-wrap items-center gap-2 min-w-0 max-w-full">
          <!-- Diff Mode Toggle -->
          <button
            @click="showFileList = !showFileList"
            class="flex items-center gap-1 px-2.5 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer"
            :title="t('stash.files.title')"
          >
            <FileText class="w-3.5 h-3.5 text-purple" />
            <span>{{ t("stash.files.count", { count: stashFiles.length }) }}</span>
          </button>

          <div class="flex items-center gap-0.5 bg-surface rounded-lg p-0.5 border border-line mr-2">
            <button
              @click="activeDiffMode = 'unified'"
              class="px-2 py-1 rounded-md text-[11px] transition cursor-pointer flex items-center gap-1"
              :class="activeDiffMode === 'unified' ? 'bg-raised text-ink font-semibold shadow-xs' : 'text-ink-muted hover:text-ink'"
            >
              <List class="w-3 h-3" /> Unified
            </button>
            <button
              @click="activeDiffMode = 'split'"
              class="px-2 py-1 rounded-md text-[11px] transition cursor-pointer flex items-center gap-1"
              :class="activeDiffMode === 'split' ? 'bg-raised text-ink font-semibold shadow-xs' : 'text-ink-muted hover:text-ink'"
            >
              <Columns class="w-3 h-3" /> Split
            </button>
          </div>

          <!-- Action Buttons -->
          <button
            @click="handleApplyStash(selectedStash.index)"
            class="flex items-center gap-1 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer"
            :title="t('stash.apply.title')"
          >
            <RotateCcw class="w-3.5 h-3.5 text-accent" />
            <span>{{ t("stash.short.apply") }}</span>
          </button>

          <button
            @click="handlePopStash(selectedStash.index)"
            class="flex items-center gap-1 px-3 py-1.5 rounded-lg bg-success-bg hover:bg-success-bg-strong text-success text-xs font-semibold transition cursor-pointer border border-success/30"
            :title="t('stash.pop.title')"
          >
            <Play class="w-3.5 h-3.5" />
            <span>{{ t("stash.short.pop") }}</span>
          </button>

          <button
            @click="openBranchModal(selectedStash)"
            class="flex items-center gap-1 px-2.5 py-1.5 rounded-lg hover:bg-raised text-ink-muted hover:text-ink text-xs transition cursor-pointer"
            :title="t('stash.branch.title')"
          >
            <GitBranch class="w-3.5 h-3.5" />
            <span>{{ t("stash.short.branch") }}</span>
          </button>

          <button :aria-label="t('stash.action.drop.short')"
            @click="handleDropStash(selectedStash.index)"
            class="flex items-center gap-1 px-2.5 py-1.5 rounded-lg hover:bg-danger-bg text-ink-muted hover:text-danger text-xs transition cursor-pointer"
            :title="t('stash.action.drop.short')"
          >
            <Trash2 class="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      <!-- Per-file restore panel -->
      <div
        v-if="selectedStash && showFileList"
        class="border-b border-line bg-surface px-5 py-2 shrink-0 max-h-40 overflow-y-auto"
      >
        <div v-if="isLoadingFiles" class="py-1 text-xs text-ink-muted">{{ t("stash.files.loading") }}</div>
        <div v-else-if="stashFiles.length === 0" class="py-1 text-xs text-ink-muted">
          {{ t("stash.files.empty") }}
        </div>
        <div v-else class="flex flex-wrap gap-2">
          <div
            v-for="file in stashFiles"
            :key="file.path"
            class="flex items-center gap-2 px-2 py-1 rounded-lg bg-surface border border-line max-w-full"
          >
            <span
              class="w-4 h-4 rounded text-[9px] font-bold flex items-center justify-center shrink-0"
              :class="stashFileStatusClass(file.status)"
            >
              {{ file.status }}
            </span>
            <span class="font-mono text-[11px] text-ink-secondary truncate max-w-[320px]" :title="file.path">
              {{ file.path }}
            </span>
            <span class="font-mono text-[10px] text-ink-muted shrink-0">+{{ file.additions }} -{{ file.deletions }}</span>
            <button
              @click="handleRestoreFile(file)"
              :disabled="restoringPath === file.path"
              class="px-2 py-0.5 rounded-md bg-canvas hover:bg-raised border border-line text-[10px] font-semibold text-accent transition cursor-pointer disabled:opacity-50 shrink-0"
              :title="t('stash.files.restore.title')"
              data-testid="stash-extract-file-btn"
            >
              {{ restoringPath === file.path ? t("stash.files.restoring") : t("stash.files.restore") }}
            </button>
          </div>
        </div>
      </div>

      <!-- Diff Canvas Area -->
      <div class="flex-1 overflow-auto bg-canvas relative font-mono text-[12px] leading-5 select-text">
        <div v-if="isLoadingDiff" class="flex items-center justify-center h-full text-ink-muted">
          <RefreshCw class="w-5 h-5 animate-spin mr-2" /> {{ t("stash.diff.loading") }}
        </div>

        <div
          v-else-if="!selectedStash"
          class="flex flex-col items-center justify-center h-full text-center p-8 select-none"
        >
          <div class="w-14 h-14 rounded-2xl bg-surface border border-line flex items-center justify-center text-accent mb-4 shadow-sm">
            <Archive class="w-7 h-7" />
          </div>
          <h3 class="text-sm font-bold text-ink mb-1">{{ t("stash.hero.title") }}</h3>
          <p class="text-xs text-ink-muted max-w-md leading-relaxed mb-4">
            {{ t("stash.hero.desc") }}
          </p>
          <button
            @click="showSaveModal = true"
            class="flex items-center gap-1.5 px-3.5 py-1.5 rounded-md bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-xs font-medium cursor-pointer transition"
          >
            <Plus class="w-3.5 h-3.5" />
            <span>{{ t("stash.hero.new") }}</span>
          </button>
        </div>

        <!-- 1. Unified Diff Table -->
        <div v-else-if="activeDiffMode === 'unified'" class="min-w-full pb-16">
          <div
            v-for="line in parsedDiffLines"
            :key="line.index"
            class="flex items-center px-2 py-0.5 transition-colors font-mono"
            :class="{
              'bg-success-bg text-success font-medium border-l-2 border-success': line.type === 'add',
              'bg-danger-bg text-danger font-medium border-l-2 border-danger line-through decoration-danger/40': line.type === 'del',
              'bg-accent-bg text-accent font-semibold py-1 border-y border-accent/20 select-none': line.type === 'header',
              'text-ink-secondary hover:bg-surface': line.type === 'ctx',
            }"
          >
            <span class="w-10 text-right pr-2 text-[10px] text-ink-muted select-none shrink-0 font-mono">
              {{ line.oldLineNo }}
            </span>
            <span class="w-10 text-right pr-2 text-[10px] text-ink-muted select-none shrink-0 font-mono">
              {{ line.newLineNo }}
            </span>
            <span class="w-4 text-center select-none font-bold shrink-0">
              {{ line.type === 'add' ? '+' : line.type === 'del' ? '-' : '' }}
            </span>
            <span class="flex-1 whitespace-pre overflow-hidden text-ellipsis pl-1">{{ line.text }}</span>
          </div>
        </div>

        <!-- 2. Split Side-by-Side Diff Table -->
        <div v-else-if="activeDiffMode === 'split'" class="min-w-full pb-16">
          <template v-for="row in parsedSplitRows" :key="row.index">
            <div
              v-if="row.isHeader"
              class="px-4 py-1 bg-accent-bg text-accent font-semibold text-[11px] border-y border-accent/20 select-none font-mono"
            >
              {{ row.headerText }}
            </div>

            <div v-else class="flex border-b border-line/20 hover:bg-surface">
              <div
                class="w-1/2 flex items-center px-2 py-0.5 border-r border-line/40 transition-colors min-w-0"
                :class="{
                  'bg-danger-bg text-danger font-medium': row.left.type === 'del',
                  'text-ink-secondary': row.left.type === 'ctx',
                  'bg-canvas': row.left.type === 'empty',
                }"
              >
                <span class="w-9 text-right pr-2 text-[10px] text-ink-muted select-none shrink-0 font-mono">
                  {{ row.left.lineNo }}
                </span>
                <span class="w-4 text-center select-none font-bold shrink-0">
                  {{ row.left.type === 'del' ? '-' : '' }}
                </span>
                <span class="flex-1 whitespace-pre overflow-hidden text-ellipsis pl-1">{{ row.left.text }}</span>
              </div>

              <div
                class="w-1/2 flex items-center px-2 py-0.5 transition-colors min-w-0"
                :class="{
                  'bg-success-bg text-success font-medium': row.right.type === 'add',
                  'text-ink-secondary': row.right.type === 'ctx',
                  'bg-canvas': row.right.type === 'empty',
                }"
              >
                <span class="w-9 text-right pr-2 text-[10px] text-ink-muted select-none shrink-0 font-mono">
                  {{ row.right.lineNo }}
                </span>
                <span class="w-4 text-center select-none font-bold shrink-0">
                  {{ row.right.type === 'add' ? '+' : '' }}
                </span>
                <span class="flex-1 whitespace-pre overflow-hidden text-ellipsis pl-1">{{ row.right.text }}</span>
              </div>
            </div>
          </template>
        </div>
      </div>
    </div>

    <!-- ============================================== -->
    <!-- MODAL 1: 新建储藏 (Save Stash Modal)          -->
    <!-- ============================================== -->
    <UiModal
      :open="showSaveModal"
      :label="t('stash.modal.save.title')"
      width="max-w-md"
      @update:open="showSaveModal = $event"
    >
      <template #header>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <Archive class="w-4 h-4 text-accent" />
          <span>{{ t("stash.modal.save.heading") }}</span>
        </h3>
      </template>

      <div class="space-y-3">
        <div class="flex flex-col gap-1.5">
          <label class="text-xs text-ink-secondary font-medium">{{ t("stash.modal.save.message") }}</label>
          <input
            v-model="saveMessage"
            type="text"
            :aria-label="t('stash.modal.save.message')"
            :placeholder="t('stash.modal.save.placeholder')"
            @keydown.enter="handleSaveStash"
            class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
          />
        </div>

        <div class="flex flex-col gap-2 p-3 rounded-xl bg-canvas border border-line/60 text-xs text-ink-secondary">
          <label class="flex items-center gap-2 cursor-pointer hover:text-ink">
            <input type="checkbox" v-model="saveIncludeUntracked" :disabled="saveIncludeAll" class="rounded accent-[var(--ug-accent-strong)]" />
            <span :class="{ 'opacity-50': saveIncludeAll }">{{ t("stash.modal.save.untracked") }}</span>
          </label>
          <label class="flex items-center gap-2 cursor-pointer hover:text-ink">
            <input type="checkbox" v-model="saveIncludeAll" class="rounded accent-[var(--ug-accent-strong)]" />
            <span>{{ t("stash.modal.save.all") }}</span>
          </label>
          <label class="flex items-center gap-2 cursor-pointer hover:text-ink">
            <input type="checkbox" v-model="saveKeepIndex" class="rounded accent-[var(--ug-accent-strong)]" />
            <span>{{ t("stash.modal.save.keep.index") }}</span>
          </label>
          <label class="flex items-center gap-2 cursor-pointer hover:text-ink">
            <input v-model="saveStagedOnly" type="checkbox" class="rounded accent-[var(--ug-accent-strong)]" />
            <span>{{ t("stash.modal.save.staged.only") }}</span>
          </label>
        </div>
      </div>

      <template #footer>
        <UiButton variant="ghost" @click="showSaveModal = false">{{ t("common.cancel") }}</UiButton>
        <UiButton variant="primary" :disabled="isSaving" @click="handleSaveStash">
          {{ isSaving ? t("stash.modal.save.saving") : t("stash.modal.save.confirm") }}
        </UiButton>
      </template>
    </UiModal>

    <!-- ============================================== -->
    <!-- MODAL 2: 基于储藏创建分支 (Branch Modal)       -->
    <!-- ============================================== -->
    <UiModal
      :open="showBranchModal"
      :label="t('stash.modal.branch.title')"
      width="max-w-md"
      @update:open="showBranchModal = $event"
    >
      <template #header>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <GitBranch class="w-4 h-4 text-purple" />
          <span>{{ t("stash.modal.branch.heading") }}</span>
        </h3>
      </template>

      <div class="space-y-3">
        <div class="flex flex-col gap-1.5">
          <label class="text-xs text-ink-secondary font-medium">{{ t("stash.modal.branch.label") }}</label>
          <input
            v-model="newBranchName"
            type="text"
            :aria-label="t('stash.modal.branch.label')"
            placeholder="feature/restore-stash"
            @keydown.enter="handleBranchFromStash"
            class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent font-mono"
          />
        </div>
        <p class="text-[11px] text-ink-muted leading-relaxed">
          {{ t("stash.modal.branch.hint", { index: selectedStash?.index ?? 0 }) }}
        </p>
      </div>

      <template #footer>
        <UiButton variant="ghost" @click="showBranchModal = false">{{ t("common.cancel") }}</UiButton>
        <UiButton variant="purple" :disabled="isBranching || !newBranchName.trim()" @click="handleBranchFromStash">
          {{ isBranching ? t("stash.modal.branch.creating") : t("stash.modal.branch.confirm") }}
        </UiButton>
      </template>
    </UiModal>

    <!-- Phase 57: Stash Diff Comparator Modal -->
    <UiModal
      :open="showComparatorModal"
      :title="t('stash.comparator.title')"
      width="max-w-3xl"
      @update:open="showComparatorModal = $event"
    >
      <div data-testid="stash-diff-comparator" class="space-y-4">
        <!-- Selectors -->
        <div class="grid grid-cols-2 gap-3 text-xs">
          <div class="space-y-1">
            <label class="font-bold text-ink" for="wgit-comparator-ref1">{{ t("stash.comparator.ref1") }}</label>
            <input
              id="wgit-comparator-ref1"
              v-model="comparatorRef1"
              type="text"
              class="w-full px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs font-mono text-ink focus:outline-none focus:border-accent"
              @change="runStashComparison"
            />
          </div>
          <div class="space-y-1">
            <label class="font-bold text-ink" for="wgit-comparator-ref2">{{ t("stash.comparator.ref2") }}</label>
            <input
              id="wgit-comparator-ref2"
              v-model="comparatorRef2"
              type="text"
              class="w-full px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs font-mono text-ink focus:outline-none focus:border-accent"
              @change="runStashComparison"
            />
          </div>
        </div>

        <!-- Comparison Summary -->
        <div v-if="comparatorReport" class="p-3 rounded-xl bg-canvas border border-line text-xs space-y-2">
          <div class="font-semibold text-ink flex items-center justify-between">
            <span>{{ comparatorReport.summary }}</span>
            <span class="font-mono text-ink-muted">+{{ comparatorReport.totalAdditions }} / -{{ comparatorReport.totalDeletions }}</span>
          </div>
          <div class="max-h-64 overflow-y-auto space-y-1.5 pt-2 border-t border-line">
            <div
              v-for="f in comparatorReport.files"
              :key="f.filePath"
              class="p-2 rounded-lg bg-surface border border-line flex items-center justify-between gap-2"
            >
              <div class="flex items-center gap-2 truncate">
                <span class="font-mono font-bold text-[11px] text-ink truncate">{{ f.filePath }}</span>
                <span class="text-[10px] text-ink-muted font-mono">(+{{ f.additions }}, -{{ f.deletions }})</span>
              </div>
              <button
                type="button"
                data-testid="stash-hunk-apply-btn"
                @click="handleApplyStashHunk(f.filePath)"
                class="px-2 py-0.5 rounded bg-accent text-accent-contrast text-[10px] font-semibold hover:opacity-90 transition cursor-pointer shrink-0"
              >
                {{ t("stash.hunk.apply") }}
              </button>
            </div>
          </div>
        </div>
      </div>
      <template #footer>
        <UiButton variant="secondary" @click="showComparatorModal = false">
          {{ t("common.close") }}
        </UiButton>
      </template>
    </UiModal>

    <UiContextMenu
      :open="stashMenuOpen"
      :x="stashMenuPos.x"
      :y="stashMenuPos.y"
      :items="stashMenuList"
      @select="handleStashMenuSelect"
      @close="closeStashMenu"
    />

  </div>
</template>
