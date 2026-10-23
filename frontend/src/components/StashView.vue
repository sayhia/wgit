<script setup lang="ts">
import { confirmAction } from "../composables/useConfirm";
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
} from "lucide-vue-next";
import { gitService } from "../services/gitService";
import UiModal from "./ui/UiModal.vue";
import UiButton from "./ui/UiButton.vue";
import Toast from "./ui/Toast.vue";
import { useToast } from "../composables/useToast";
import { useModalA11y } from "../composables/useModalA11y";
import type { StashInfo, StashFileEntry } from "../types";

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
    if (epoch === repoEpoch) notify(`读取储藏文件失败: ${err?.message || err}`);
  } finally {
    if (epoch === repoEpoch) isLoadingFiles.value = false;
  }
}

function stashFileStatusClass(status: string) {
  if (status === "A") return "bg-success/15 text-success";
  if (status === "D") return "bg-danger/15 text-danger";
  if (status === "R") return "bg-purple/15 text-purple";
  return "bg-warning/15 text-warning";
}

async function handleRestoreFile(file: StashFileEntry) {
  if (!props.currentRepoPath || !selectedStash.value) return;
  if (!(await confirmAction({ message: `确定从 stash@{${selectedStash.value.index}} 恢复 ${file.path} 吗？同名文件将被覆盖。`, tone: "danger" }))) return;
  restoringPath.value = file.path;
  try {
    await gitService.applyFileFromStash(props.currentRepoPath, selectedStash.value.index, file.path);
    notify(`已从储藏恢复并暂存: ${file.path.split("/").pop()}`);
    emit("refresh");
    await loadStashFiles(selectedStash.value);
  } catch (err: any) {
    notify(`恢复文件失败: ${err?.message || err}`);
  } finally {
    restoringPath.value = "";
  }
}

// Modals
const showSaveModal = ref(false);
const saveMessage = ref("");
const saveIncludeUntracked = ref(true);
const saveKeepIndex = ref(false);
const saveStagedOnly = ref(false);
const isSaving = ref(false);

const showBranchModal = ref(false);
const newBranchName = ref("");
const isBranching = ref(false);


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
    console.error("Failed to load stashes:", err);
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
    diffText.value = diff || "(该储藏快照无文本变更或仅包含二进制文件)";
  } catch (err: any) {
    if (epoch === repoEpoch) diffText.value = `获取储藏差异失败: ${err?.message || err}`;
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
      notify("已仅储藏暂存区改动");
    } else {
      await gitService.stashSave(
        props.currentRepoPath,
        saveMessage.value.trim(),
        saveKeepIndex.value,
        saveIncludeUntracked.value
      );
      notify("工作区改动已成功保存至储藏箱");
    }
    saveMessage.value = "";
    showSaveModal.value = false;
    emit("refresh");
    await loadStashes();
  } catch (err: any) {
    notify(`保存储藏失败: ${err?.message || err}`);
  } finally {
    isSaving.value = false;
  }
}

// Apply Stash
async function handleApplyStash(index: number) {
  if (!props.currentRepoPath) return;
  try {
    await gitService.stashApply(props.currentRepoPath, index);
    notify(`已应用 stash@{${index}} 到当前工作区`);
    emit("refresh");
  } catch (err: any) {
    notify(`应用储藏失败: ${err?.message || err}`);
  }
}

// Pop Stash
async function handlePopStash(index: number) {
  if (!props.currentRepoPath) return;
  try {
    await gitService.stashPop(props.currentRepoPath, index);
    notify(`已弹出并应用 stash@{${index}}，并从储藏列表移除`);
    emit("refresh");
    await loadStashes();
  } catch (err: any) {
    notify(`弹出储藏失败: ${err?.message || err}`);
  }
}

// Drop Stash
async function handleDropStash(index: number) {
  if (!props.currentRepoPath) return;
  if (!(await confirmAction({ message: `确定要彻底删除 stash@{${index}} 储藏快照吗？此操作无法撤销。`, tone: "danger" }))) return;
  try {
    await gitService.stashDrop(props.currentRepoPath, index);
    notify(`已删除 stash@{${index}}`);
    await loadStashes();
  } catch (err: any) {
    notify(`删除储藏失败: ${err?.message || err}`);
  }
}

// Clear All Stashes
async function handleClearAll() {
  if (!props.currentRepoPath) return;
  if (!(await confirmAction({ message: "⚠️ 确定要清空储藏箱中的所有快照吗？这将永久删除所有历史储藏！", tone: "danger", requireText: "清空", requireTextHint: "输入「清空」以确认永久删除全部储藏" }))) return;
  try {
    await gitService.stashClear(props.currentRepoPath);
    notify("储藏箱已清空");
    await loadStashes();
  } catch (err: any) {
    notify(`清空储藏失败: ${err?.message || err}`);
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
    notify(`已基于储藏快照创建并检出分支: ${newBranchName.value.trim()}`);
    showBranchModal.value = false;
    emit("refresh");
    await loadStashes();
  } catch (err: any) {
    notify(`创建分支失败: ${err?.message || err}`);
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
    <div class="w-80 border-r border-line bg-surface/30 flex flex-col overflow-hidden shrink-0">
      <!-- Header Bar -->
      <div class="p-3 border-b border-line bg-surface/50 flex flex-col gap-2 shrink-0">
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-2">
            <Archive class="w-4 h-4 text-accent" />
            <h2 class="text-xs font-bold text-ink">储藏箱 & 快照</h2>
            <span
              v-if="stashes.length > 0"
              class="px-1.5 py-0.2 rounded-full bg-accent/20 text-accent text-[10px] font-bold"
            >
              {{ stashes.length }}
            </span>
          </div>

          <div class="flex items-center gap-1.5">
            <button
              @click="showSaveModal = true"
              class="flex items-center gap-1 px-2 py-1 rounded-md bg-accent-strong hover:bg-accent-strong-hover text-white text-[11px] font-semibold transition cursor-pointer shadow-xs"
              title="将当前工作区改动保存至储藏箱"
            >
              <Plus class="w-3.5 h-3.5" />
              <span>新建储藏</span>
            </button>
            <button
              v-if="stashes.length > 0"
              @click="handleClearAll"
              class="p-1 rounded-md hover:bg-danger/20 text-ink-muted hover:text-danger transition cursor-pointer"
              title="清空所有储藏"
            >
              <Trash2 class="w-3.5 h-3.5" />
            </button>
            <button aria-label="刷新列表"
              @click="loadStashes"
              class="p-1 rounded-md hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer"
              title="刷新列表"
            >
              <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isLoading }" />
            </button>
          </div>
        </div>

        <!-- Search Bar -->
        <div class="relative">
          <input
            v-model="stashSearch"
            type="text"
            aria-label="搜索储藏说明或编号"
            placeholder="搜索储藏说明或 stash@{n}..."
            class="w-full pl-7 pr-6 py-1 rounded-md bg-canvas/80 border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent font-mono transition"
          />
          <Search class="w-3.5 h-3.5 text-ink-muted absolute left-2 top-2 pointer-events-none" />
          <button
            v-if="stashSearch"
            @click="stashSearch = ''"
            class="absolute right-1.5 top-1.5 text-[10px] text-ink-muted hover:text-ink cursor-pointer font-bold"
          >
            ✕
          </button>
        </div>
      </div>

      <!-- Stash List Cards -->
      <div class="flex-1 overflow-y-auto p-2 space-y-1.5">
        <div
          v-for="s in filteredStashes"
          :key="s.index"
          @click="selectStash(s)"
          class="group p-2.5 rounded-xl border transition cursor-pointer select-none flex flex-col gap-1.5"
          :class="
            selectedStash?.index === s.index
              ? 'bg-raised border-accent/50 shadow-sm'
              : 'bg-surface/40 hover:bg-surface/80 border-line hover:border-line/80'
          "
        >
          <!-- Top Row: Index Badge & Actions -->
          <div class="flex items-center justify-between">
            <span
              class="px-1.5 py-0.2 rounded text-[10px] font-mono font-bold"
              :class="
                selectedStash?.index === s.index
                  ? 'bg-accent-strong text-white'
                  : 'bg-line/60 text-ink-muted'
              "
            >
              stash@{{ s.index }}
            </span>

            <!-- Quick Action Hover Bar -->
            <div class="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition">
              <button aria-label="弹出并应用到工作区 (Pop)"
                @click.stop="handlePopStash(s.index)"
                class="p-1 rounded hover:bg-success/20 text-ink-muted hover:text-success transition cursor-pointer"
                title="弹出并应用到工作区 (Pop)"
              >
                <Play class="w-3 h-3" />
              </button>
              <button aria-label="应用到工作区 (Apply，不删除储藏)"
                @click.stop="handleApplyStash(s.index)"
                class="p-1 rounded hover:bg-accent/20 text-ink-muted hover:text-accent transition cursor-pointer"
                title="应用到工作区 (Apply，不删除储藏)"
              >
                <RotateCcw class="w-3 h-3" />
              </button>
              <button aria-label="以此储藏创建新分支 (Branch)"
                @click.stop="openBranchModal(s)"
                class="p-1 rounded hover:bg-purple/20 text-ink-muted hover:text-purple transition cursor-pointer"
                title="以此储藏创建新分支 (Branch)"
              >
                <GitBranch class="w-3 h-3" />
              </button>
              <button aria-label="删除该储藏 (Drop)"
                @click.stop="handleDropStash(s.index)"
                class="p-1 rounded hover:bg-danger/20 text-ink-muted hover:text-danger transition cursor-pointer"
                title="删除该储藏 (Drop)"
              >
                <Trash2 class="w-3 h-3" />
              </button>
            </div>
          </div>

          <!-- Message -->
          <div class="text-xs font-semibold text-ink line-clamp-2 leading-snug">
            {{ s.message || "(无说明储藏快照)" }}
          </div>

          <!-- Date -->
          <div class="flex items-center gap-1 text-[10px] text-ink-muted font-mono">
            <Clock class="w-3 h-3 shrink-0" />
            <span class="truncate">{{ s.date }}</span>
          </div>
        </div>

        <!-- Empty State in List -->
        <div
          v-if="stashes.length === 0 && !isLoading"
          class="py-12 px-4 text-center text-xs text-ink-muted flex flex-col items-center gap-2"
        >
          <Archive class="w-8 h-8 text-ink-muted/40 mb-1" />
          <span class="font-semibold text-ink/80">暂无储藏快照</span>
          <span class="text-[11px] leading-relaxed">
            当您需要临时切换分支时，点击上方「新建储藏」即可将当前改动安全存档。
          </span>
        </div>

        <div
          v-else-if="filteredStashes.length === 0 && stashSearch"
          class="py-12 text-center text-xs text-ink-muted"
        >
          无匹配的储藏快照
        </div>
      </div>
    </div>

    <!-- ============================================== -->
    <!-- RIGHT COLUMN: 储藏快照差异比对主区 (Diff View)  -->
    <!-- ============================================== -->
    <div class="flex-1 flex flex-col overflow-hidden bg-canvas min-w-0">
      <!-- Top Action Bar -->
      <div
        v-if="selectedStash"
        class="h-12 border-b border-line px-5 flex items-center justify-between bg-surface/40 backdrop-blur-sm shrink-0"
      >
        <div class="flex items-center gap-3 truncate min-w-0">
          <span class="px-2 py-0.5 rounded bg-accent/20 text-accent font-mono font-bold text-xs shrink-0">
            stash@{{ selectedStash.index }}
          </span>
          <span class="font-bold text-xs text-ink truncate">{{ selectedStash.message }}</span>
        </div>

        <div class="flex items-center gap-2 shrink-0">
          <!-- Diff Mode Toggle -->
          <button
            @click="showFileList = !showFileList"
            class="flex items-center gap-1 px-2.5 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer"
            title="查看该储藏包含的文件，并可单文件恢复"
          >
            <FileText class="w-3.5 h-3.5 text-purple" />
            <span>文件 {{ stashFiles.length }}</span>
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
            title="将此储藏应用到当前分支（保留在储藏列表中）"
          >
            <RotateCcw class="w-3.5 h-3.5 text-accent" />
            <span>应用 (Apply)</span>
          </button>

          <button
            @click="handlePopStash(selectedStash.index)"
            class="flex items-center gap-1 px-3 py-1.5 rounded-lg bg-success/20 hover:bg-success/30 text-success text-xs font-semibold transition cursor-pointer border border-success/30"
            title="应用此储藏并从储藏箱弹出删除"
          >
            <Play class="w-3.5 h-3.5" />
            <span>弹出 (Pop)</span>
          </button>

          <button
            @click="openBranchModal(selectedStash)"
            class="flex items-center gap-1 px-2.5 py-1.5 rounded-lg hover:bg-raised text-ink-muted hover:text-ink text-xs transition cursor-pointer"
            title="以此储藏快照创建独立恢复分支"
          >
            <GitBranch class="w-3.5 h-3.5" />
            <span>分支</span>
          </button>

          <button aria-label="删除该储藏"
            @click="handleDropStash(selectedStash.index)"
            class="flex items-center gap-1 px-2.5 py-1.5 rounded-lg hover:bg-danger/20 text-ink-muted hover:text-danger text-xs transition cursor-pointer"
            title="删除该储藏"
          >
            <Trash2 class="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      <!-- Per-file restore panel -->
      <div
        v-if="selectedStash && showFileList"
        class="border-b border-line bg-surface/30 px-5 py-2 shrink-0 max-h-40 overflow-y-auto"
      >
        <div v-if="isLoadingFiles" class="py-1 text-xs text-ink-muted">正在读取储藏文件...</div>
        <div v-else-if="stashFiles.length === 0" class="py-1 text-xs text-ink-muted">
          该储藏没有可列出的文件。
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
              title="仅恢复该文件到工作区并暂存"
            >
              {{ restoringPath === file.path ? "恢复中..." : "恢复" }}
            </button>
          </div>
        </div>
      </div>

      <!-- Diff Canvas Area -->
      <div class="flex-1 overflow-auto bg-canvas relative font-mono text-[12px] leading-5 select-text">
        <div v-if="isLoadingDiff" class="flex items-center justify-center h-full text-ink-muted">
          <RefreshCw class="w-5 h-5 animate-spin mr-2" /> 正在加载储藏差异快照...
        </div>

        <div
          v-else-if="!selectedStash"
          class="flex flex-col items-center justify-center h-full text-center p-8 select-none"
        >
          <div class="w-14 h-14 rounded-2xl bg-surface border border-line flex items-center justify-center text-accent mb-4 shadow-sm">
            <Archive class="w-7 h-7" />
          </div>
          <h3 class="text-sm font-bold text-ink mb-1">Git 储藏箱 & 快照管理中心</h3>
          <p class="text-xs text-ink-muted max-w-md leading-relaxed mb-4">
            随时保存当前未完成的代码草稿，保持工作区干净整洁，自由切换或拉取分支，之后一键恢复上下文。
          </p>
          <button
            @click="showSaveModal = true"
            class="flex items-center gap-1.5 px-3.5 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-white text-xs font-semibold shadow-md shadow-accent/20 cursor-pointer transition"
          >
            <Plus class="w-3.5 h-3.5" />
            <span>新建快照储藏</span>
          </button>
        </div>

        <!-- 1. Unified Diff Table -->
        <div v-else-if="activeDiffMode === 'unified'" class="min-w-full pb-16">
          <div
            v-for="line in parsedDiffLines"
            :key="line.index"
            class="flex items-center px-2 py-0.5 transition-colors font-mono"
            :class="{
              'bg-success/15 text-success font-medium border-l-2 border-success': line.type === 'add',
              'bg-danger/15 text-danger font-medium border-l-2 border-danger line-through decoration-danger/40': line.type === 'del',
              'bg-accent/10 text-accent font-semibold py-1 border-y border-accent/20 select-none': line.type === 'header',
              'text-ink-secondary hover:bg-surface/30': line.type === 'ctx',
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
              class="px-4 py-1 bg-accent/10 text-accent font-semibold text-[11px] border-y border-accent/20 select-none font-mono"
            >
              {{ row.headerText }}
            </div>

            <div v-else class="flex border-b border-line/20 hover:bg-surface/30">
              <div
                class="w-1/2 flex items-center px-2 py-0.5 border-r border-line/40 transition-colors min-w-0"
                :class="{
                  'bg-danger/10 text-danger font-medium': row.left.type === 'del',
                  'text-ink-secondary': row.left.type === 'ctx',
                  'bg-canvas/30': row.left.type === 'empty',
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
                  'bg-success/10 text-success font-medium': row.right.type === 'add',
                  'text-ink-secondary': row.right.type === 'ctx',
                  'bg-canvas/30': row.right.type === 'empty',
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
      label="新建工作区储藏快照"
      width="max-w-md"
      @update:open="showSaveModal = $event"
    >
      <template #header>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <Archive class="w-4 h-4 text-accent" />
          <span>新建工作区储藏快照 (git stash push)</span>
        </h3>
      </template>

      <div class="space-y-3">
        <div class="flex flex-col gap-1.5">
          <label class="text-xs text-ink-secondary font-medium">储藏说明 (可选):</label>
          <input
            v-model="saveMessage"
            type="text"
            aria-label="储藏说明"
            placeholder="例如: WIP: 临时保存重构中代码..."
            @keydown.enter="handleSaveStash"
            class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
          />
        </div>

        <div class="flex flex-col gap-2 p-3 rounded-xl bg-canvas/60 border border-line/60 text-xs text-ink-secondary">
          <label class="flex items-center gap-2 cursor-pointer hover:text-ink">
            <input type="checkbox" v-model="saveIncludeUntracked" class="rounded accent-[var(--ug-accent-strong)]" />
            <span>包含未跟踪的新增文件 (-u / --include-untracked)</span>
          </label>
          <label class="flex items-center gap-2 cursor-pointer hover:text-ink">
            <input type="checkbox" v-model="saveKeepIndex" class="rounded accent-[var(--ug-accent-strong)]" />
            <span>保留暂存区索引状态 (--keep-index)</span>
          </label>
          <label class="flex items-center gap-2 cursor-pointer hover:text-ink">
            <input v-model="saveStagedOnly" type="checkbox" class="rounded accent-[var(--ug-accent-strong)]" />
            <span>仅储藏已暂存改动 (--staged)，未暂存内容保留</span>
          </label>
        </div>
      </div>

      <template #footer>
        <UiButton variant="ghost" @click="showSaveModal = false">取消</UiButton>
        <UiButton variant="primary" :disabled="isSaving" @click="handleSaveStash">
          {{ isSaving ? "储藏中..." : "确认储藏" }}
        </UiButton>
      </template>
    </UiModal>

    <!-- ============================================== -->
    <!-- MODAL 2: 基于储藏创建分支 (Branch Modal)       -->
    <!-- ============================================== -->
    <UiModal
      :open="showBranchModal"
      label="基于储藏创建新分支"
      width="max-w-md"
      @update:open="showBranchModal = $event"
    >
      <template #header>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <GitBranch class="w-4 h-4 text-purple" />
          <span>基于储藏创建新分支 (git stash branch)</span>
        </h3>
      </template>

      <div class="space-y-3">
        <div class="flex flex-col gap-1.5">
          <label class="text-xs text-ink-secondary font-medium">新分支名称:</label>
          <input
            v-model="newBranchName"
            type="text"
            aria-label="新分支名称"
            placeholder="feature/restore-stash"
            @keydown.enter="handleBranchFromStash"
            class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent font-mono"
          />
        </div>
        <p class="text-[11px] text-ink-muted leading-relaxed">
          将自动创建新分支、检出并在其上应用 stash@{{ selectedStash?.index }} 的全部改动。若成功，该储藏将被自动清除。
        </p>
      </div>

      <template #footer>
        <UiButton variant="ghost" @click="showBranchModal = false">取消</UiButton>
        <UiButton variant="purple" :disabled="isBranching || !newBranchName.trim()" @click="handleBranchFromStash">
          {{ isBranching ? "创建中..." : "创建并检出" }}
        </UiButton>
      </template>
    </UiModal>

    <Toast :text="toastText" :visible="showToast" />
  </div>
</template>
