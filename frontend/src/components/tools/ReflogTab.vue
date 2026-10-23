<script setup lang="ts">
import { confirmAction } from "../../composables/useConfirm";
import { ref, computed, watch, onMounted, watchEffect } from "vue";
import { GitBranch, Search, Play, Filter, Undo2, History } from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import { toolsStatus } from "../../composables/toolsStatus";
import { useModalA11y } from "../../composables/useModalA11y";
import type { ReflogItem, RemoteDetail, WorktreeInfo, LfsLockInfo, LfsTrackPattern, SubmoduleInfo, DiskBreakdownResult, LargeFileInfo, RepoMaintenanceStats, LfsPruneResult, GitFsckResult, OperationLogResult } from "../../types";

const props = defineProps<{
  currentRepoPath: string;
  refreshNonce: number;
  mode?: "remotes" | "worktrees";
}>();

const emit = defineEmits<{
  (e: "refresh"): void;
  (e: "notify", msg: string): void;
  (e: "open-repo", path: string): void;
}>();

function notify(msg: string) {
  emit("notify", msg);
}

function copyToClipboard(text: string, label: string) {
  navigator.clipboard.writeText(text);
  notify(`已复制 ${label}`);
}

// -------------------------------------------------------------
// 1. REFLOG & RECOVERY
// -------------------------------------------------------------
const reflogItems = ref<ReflogItem[]>([]);
const reflogSearch = ref("");
const isLoadingReflog = ref(false);

const showReflogBranchModal = ref(false);
const reflogSelectedCommit = ref("");
const reflogBranchName = ref("");
const isCreatingReflogBranch = ref(false);

const { containerRef: reflogBranchDialogRef } = useModalA11y(
  () => showReflogBranchModal.value,
  () => { if (!isCreatingReflogBranch.value) showReflogBranchModal.value = false; }
);

async function loadReflog() {
  if (!props.currentRepoPath) return;
  isLoadingReflog.value = true;
  try {
    const list = await gitService.getReflog(props.currentRepoPath, 100);
    reflogItems.value = list || [];
  } catch (err) {
    console.error("Failed to load reflog:", err);
  } finally {
    isLoadingReflog.value = false;
  }
  void loadOplog();
}

// ---- 操作日志与撤销 -------------------------------------------------------
const oplog = ref<OperationLogResult | null>(null);
const isUndoing = ref(false);

async function loadOplog() {
  if (!props.currentRepoPath) return;
  try {
    oplog.value = await gitService.getOperationLog(props.currentRepoPath, 8);
  } catch (err) {
    console.error("Failed to load operation log:", err);
  }
}

async function handleUndo() {
  const newest = oplog.value?.entries[0];
  const target = oplog.value?.entries[1];
  if (!props.currentRepoPath || !newest || !oplog.value?.canUndo || isUndoing.value) return;

  const confirmed = await confirmAction(
    { message: `撤销最近的「${newest.actionLabel}」？\n\n` +
      `${newest.subject}\n\n` +
      `· HEAD 回到 ${target?.shortHash || "上一个位置"}\n` +
      `· 全部改动保留在暂存区，不丢任何文件\n` +
      `· 撤销本身也可通过下方 reflog 再次找回`, tone: "danger" }
  );
  if (!confirmed) return;

  isUndoing.value = true;
  try {
    const res = await gitService.undoLastOperation(props.currentRepoPath);
    notify(res.message);
    await loadReflog();
    emit("refresh");
  } catch (err: any) {
    notify(`撤销失败: ${err?.message || err}`);
  } finally {
    isUndoing.value = false;
  }
}

const filteredReflog = computed(() => {
  if (!reflogSearch.value.trim()) return reflogItems.value;
  const q = reflogSearch.value.toLowerCase();
  return reflogItems.value.filter(
    (item) =>
      item.selector.toLowerCase().includes(q) ||
      item.action.toLowerCase().includes(q) ||
      item.message.toLowerCase().includes(q) ||
      item.commitHash.toLowerCase().includes(q)
  );
});

async function handleCheckoutReflog(selector: string) {
  if (!props.currentRepoPath) return;
  if (!(await confirmAction(`确定要直接检出到 ${selector} 吗？这可能会进入分离 HEAD 状态 (Detached HEAD)。`))) return;
  try {
    await gitService.checkoutReflog(props.currentRepoPath, selector);
    notify(`已检出至 ${selector}`);
    emit("refresh");
  } catch (err: any) {
    notify(`检出失败: ${err?.message || err}`);
  }
}

function openReflogBranchModal(commitHash: string, selector: string) {
  reflogSelectedCommit.value = commitHash;
  reflogBranchName.value = `recovered-${selector.replace(/[^a-zA-Z0-9]/g, "-")}`;
  showReflogBranchModal.value = true;
}

async function handleCreateBranchFromReflog() {
  if (!props.currentRepoPath || !reflogBranchName.value.trim()) return;
  isCreatingReflogBranch.value = true;
  try {
    await gitService.createBranchFromReflog(
      props.currentRepoPath,
      reflogBranchName.value.trim(),
      reflogSelectedCommit.value
    );
    notify(`已从历史记录创建恢复分支: ${reflogBranchName.value.trim()}（当前分支保持不变）`);
    showReflogBranchModal.value = false;
    emit("refresh");
  } catch (err: any) {
    notify(`创建恢复分支失败: ${err?.message || err}`);
  } finally {
    isCreatingReflogBranch.value = false;
  }
}


watchEffect(() => {
  toolsStatus.loading.reflog = isLoadingReflog.value;
});

onMounted(() => {
  void loadReflog();
});

watch(
  () => props.refreshNonce,
  () => {
    void loadReflog();
  }
);
</script>

<template>
      <!-- ============================================== -->
      <!-- TAB 1: 引用日志与救砖恢复 (Reflog & Recovery)   -->
      <!-- ============================================== -->
      <div class="space-y-4">
        <!-- Sub Header & Tips -->
        <div class="flex items-center justify-between pb-3 border-b border-line">
          <div>
            <h3 class="text-sm font-bold text-ink flex items-center gap-2">
              <span>HEAD 引用日志与时光倒流救砖</span>
              <span class="text-xs text-ink-muted font-normal">(完整记录所有的 HEAD 移动，误删分支与错误回退的克星)</span>
            </h3>
          </div>

          <div class="relative w-64">
            <input
              v-model="reflogSearch"
              type="text"
              aria-label="搜索 Reflog 操作、说明或哈希"
              placeholder="搜索操作、说明或哈希..."
              class="w-full pl-7 pr-6 py-1 rounded-md bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent font-mono"
            />
            <Search class="w-3.5 h-3.5 text-ink-muted absolute left-2 top-2 pointer-events-none" />
          </div>
        </div>

        <!-- Recent operations + one-click undo -->
        <div
          v-if="oplog"
          class="rounded-xl border p-3 space-y-2"
          :class="oplog.canUndo ? 'border-accent/30 bg-accent/5' : 'border-line bg-surface/30'"
        >
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0">
              <div class="text-xs font-bold text-ink flex items-center gap-2">
                <History class="w-3.5 h-3.5 text-accent" />
                <span>最近操作与撤销</span>
              </div>
              <p class="text-[11px] text-ink-muted mt-1 leading-relaxed">{{ oplog.message }}</p>
            </div>
            <button
              @click="handleUndo"
              :disabled="!oplog.canUndo || isUndoing"
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-white text-xs font-semibold transition cursor-pointer disabled:opacity-30 shrink-0"
              :title="oplog.canUndo ? '软重置到上一个位置，改动保留在暂存区' : '当前没有可一键撤销的操作'"
            >
              <Undo2 class="w-3.5 h-3.5" :class="{ 'animate-pulse': isUndoing }" />
              <span>{{ isUndoing ? "撤销中..." : "撤销上一步" }}</span>
            </button>
          </div>

          <div v-if="oplog.entries.length > 1" class="flex flex-wrap items-center gap-1.5 pt-1 border-t border-line/50">
            <span class="text-[10px] text-ink-muted shrink-0">时间线</span>
            <span
              v-for="entry in oplog.entries.slice(0, 6)"
              :key="entry.selector"
              class="px-1.5 py-0.5 rounded text-[10px] font-mono"
              :class="entry.undoable ? 'bg-surface border border-line text-ink-muted' : 'bg-surface/50 text-ink-muted/60 line-through'"
              :title="`${entry.actionLabel} · ${entry.subject}`"
            >
              {{ entry.actionLabel }}
            </span>
          </div>
        </div>

        <!-- Reflog List Table -->
        <div class="border border-line rounded-xl bg-surface/30 overflow-hidden shadow-xs">
          <div class="divide-y divide-line/40 font-mono text-[11px]">
            <div
              v-for="item in filteredReflog"
              :key="item.selector"
              class="flex items-center justify-between p-2.5 hover:bg-surface/70 transition-colors group"
            >
              <div class="flex items-center gap-3 min-w-0">
                <span class="px-2 py-0.5 rounded bg-accent/20 text-accent font-bold shrink-0">
                  {{ item.selector }}
                </span>
                <span
                  @click="copyToClipboard(item.commitHash, 'Commit Hash')"
                  class="text-ink-muted hover:text-ink cursor-pointer hover:underline shrink-0"
                  title="点击复制 Commit Hash"
                >
                  {{ item.commitHash.substring(0, 8) }}
                </span>
                <span class="px-1.5 py-0.2 rounded bg-line/60 text-ink-secondary text-[10px] shrink-0 uppercase font-sans font-semibold">
                  {{ item.action }}
                </span>
                <span class="font-sans font-medium text-ink truncate max-w-lg">
                  {{ item.message }}
                </span>
              </div>

              <!-- Action Buttons -->
              <div class="flex items-center gap-2 shrink-0 opacity-0 group-hover:opacity-100 transition">
                <button
                  @click="openReflogBranchModal(item.commitHash, item.selector)"
                  class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-purple/20 hover:bg-purple/30 text-purple text-[11px] font-semibold transition cursor-pointer border border-purple/30"
                  title="以此历史状态为基准创建新恢复分支"
                >
                  <GitBranch class="w-3 h-3" />
                  <span>恢复为新分支</span>
                </button>
                <button
                  @click="handleCheckoutReflog(item.selector)"
                  class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-surface hover:bg-raised border border-line text-[11px] font-medium text-ink transition cursor-pointer"
                  title="直接检出至该历史时刻 (git checkout)"
                >
                  <Play class="w-3 h-3 text-accent" />
                  <span>检出</span>
                </button>
              </div>
            </div>

            <div v-if="filteredReflog.length === 0" class="py-12 text-center text-xs text-ink-muted font-sans">
              暂无匹配的引用日志记录
            </div>
          </div>
        </div>
      </div>


    <!-- Modal: Create Branch from Reflog -->
    <div
      v-if="showReflogBranchModal"
      ref="reflogBranchDialogRef"
      role="dialog"
      aria-modal="true"
      aria-label="从历史时刻创建恢复分支"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-xs p-4"
    >
      <div class="w-full max-w-md rounded-2xl bg-surface border border-line p-5 shadow-2xl flex flex-col gap-4 animate-in fade-in zoom-in-95 duration-150">
        <div class="flex items-center gap-2 pb-2 border-b border-line">
          <GitBranch class="w-4 h-4 text-purple" />
          <h3 class="text-sm font-bold text-ink">从历史时刻创建恢复分支</h3>
        </div>

        <div class="flex flex-col gap-1.5">
          <label class="text-xs text-ink-secondary font-medium">基准 Commit:</label>
          <span class="px-2.5 py-1.5 rounded-lg bg-canvas border border-line font-mono text-xs text-ink">
            {{ reflogSelectedCommit }}
          </span>
        </div>

        <div class="flex flex-col gap-1.5">
          <label class="text-xs text-ink-secondary font-medium">新分支名称:</label>
          <input
            v-model="reflogBranchName"
            type="text"
            @keydown.enter="handleCreateBranchFromReflog"
            class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
          />
        </div>

        <div class="flex items-center justify-end gap-2 pt-2 border-t border-line">
          <button
            @click="showReflogBranchModal = false"
            class="px-3 py-1.5 rounded-lg hover:bg-raised text-xs text-ink-muted hover:text-ink transition cursor-pointer"
          >
            取消
          </button>
          <button
            @click="handleCreateBranchFromReflog"
            :disabled="isCreatingReflogBranch || !reflogBranchName.trim()"
            class="px-4 py-1.5 rounded-lg bg-purple-strong hover:bg-purple-dark text-white text-xs font-semibold transition cursor-pointer disabled:opacity-50"
          >
            {{ isCreatingReflogBranch ? "创建中..." : "创建分支" }}
          </button>
        </div>
      </div>
    </div>

</template>
