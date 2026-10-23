<script setup lang="ts">
import { confirmAction } from "../../composables/useConfirm";
import { ref, watch, onMounted, watchEffect } from "vue";
import { Filter, CheckCircle2, FolderTree, Folder } from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import { toolsStatus } from "../../composables/toolsStatus";
import type { ReflogItem, RemoteDetail, WorktreeInfo, LfsLockInfo, LfsTrackPattern, SubmoduleInfo, DiskBreakdownResult, LargeFileInfo, RepoMaintenanceStats, LfsPruneResult, GitFsckResult } from "../../types";

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

// -------------------------------------------------------------
// 6. SPARSE CHECKOUT (稀疏检出 / Monorepo 部分拉取)
// -------------------------------------------------------------
const sparseFolders = ref<string[]>([]);
const newSparseFolder = ref("");
const isLoadingSparse = ref(false);
const isSavingSparse = ref(false);

async function loadSparseCheckout() {
  if (!props.currentRepoPath) return;
  isLoadingSparse.value = true;
  try {
    const list = await gitService.getSparseCheckoutFolders(props.currentRepoPath);
    sparseFolders.value = list || [];
  } catch (err) {
    console.error("Failed to load sparse checkout:", err);
  } finally {
    isLoadingSparse.value = false;
  }
}

function addSparseFolder(path?: string) {
  const target = (path ?? newSparseFolder.value).trim();
  if (!target) return;
  if (!sparseFolders.value.includes(target)) {
    sparseFolders.value.push(target);
  }
  newSparseFolder.value = "";
}

function removeSparseFolder(index: number) {
  sparseFolders.value.splice(index, 1);
}

async function handleApplySparseCheckout() {
  if (!props.currentRepoPath) return;
  isSavingSparse.value = true;
  try {
    await gitService.setSparseCheckoutFolders(props.currentRepoPath, sparseFolders.value);
    notify(`稀疏检出规则已应用 (${sparseFolders.value.length} 个目录)`);
    await loadSparseCheckout();
  } catch (err: any) {
    notify(`应用稀疏检出失败: ${err?.message || err}`);
  } finally {
    isSavingSparse.value = false;
  }
}

async function handleResetSparseCheckout() {
  if (!props.currentRepoPath) return;
  if (!(await confirmAction("确定要取消稀疏检出限制并检出全部文件吗？大型仓库可能需要较长时间。"))) return;
  isSavingSparse.value = true;
  try {
    await gitService.setSparseCheckoutFolders(props.currentRepoPath, []);
    notify("已恢复全量目录检出");
    await loadSparseCheckout();
  } catch (err: any) {
    notify(`重置稀疏检出失败: ${err?.message || err}`);
  } finally {
    isSavingSparse.value = false;
  }
}


watchEffect(() => {
  toolsStatus.loading.sparse = isLoadingSparse.value || isSavingSparse.value;
});

watchEffect(() => {
  toolsStatus.counts.sparse = sparseFolders.value.length;
});

onMounted(() => {
  void loadSparseCheckout();
});

watch(
  () => props.refreshNonce,
  () => {
    void loadSparseCheckout();
  }
);
</script>

<template>
      <!-- ============================================== -->
      <!-- TAB 6: 稀疏检出与 Monorepo (Sparse Checkout)    -->
      <!-- ============================================== -->
      <div class="space-y-5">
        <!-- Banner -->
        <div class="p-4 rounded-xl border border-purple/30 bg-purple/10 flex items-start gap-3">
          <Filter class="w-5 h-5 text-purple shrink-0 mt-0.5" />
          <div class="text-xs space-y-1">
            <div class="font-bold text-sm text-ink">大型单体仓库 (Monorepo) 稀疏检出与 Cone 模式</div>
            <div class="text-ink-muted leading-relaxed">
              针对大型工程、超大型游戏仓库，稀疏检出可让本地工作区仅保留你关注的代码或资产目录，减少 80%~95% 本地磁盘空间，极大加速 Git 状态更新、分支切换及代码编辑器索引。
            </div>
          </div>
        </div>

        <!-- Section: Active Folders List -->
        <div class="space-y-3" data-tool-section="sparse-folders">
          <div class="flex items-center justify-between pb-2 border-b border-line">
            <h3 class="text-sm font-bold text-ink flex items-center gap-2">
              <FolderTree class="w-4 h-4 text-accent" />
              <span>当前检出的目录白名单 (Active Sparse Folders)</span>
            </h3>
            <span class="text-xs text-ink-muted">共 {{ sparseFolders.length }} 个目录受保护检出</span>
          </div>

          <!-- Tags list -->
          <div class="flex flex-wrap gap-2 p-4 rounded-xl border border-line bg-surface/30 min-h-[90px]">
            <div
              v-for="(folder, idx) in sparseFolders"
              :key="idx"
              class="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-surface border border-line font-mono text-xs text-ink shadow-2xs group"
            >
              <Folder class="w-3.5 h-3.5 text-accent shrink-0" />
              <span>{{ folder }}</span>
              <button
                @click="removeSparseFolder(idx)"
                class="text-ink-muted hover:text-danger cursor-pointer ml-1"
                title="移除该目录"
              >
                ×
              </button>
            </div>

            <div
              v-if="sparseFolders.length === 0"
              class="w-full py-4 text-center text-xs text-ink-muted flex items-center justify-center gap-2"
            >
              <CheckCircle2 class="w-4 h-4 text-success" />
              <span>未启用目录过滤（当前全量检出所有仓库文件）</span>
            </div>
          </div>

          <!-- Add Directory Form & Presets -->
          <div class="flex items-center gap-2">
            <div class="relative flex-1">
              <input
                v-model="newSparseFolder"
                type="text"
                @keydown.enter="addSparseFolder()"
                placeholder="输入相对路径添加到白名单，例如：src/components 或 frontend 或 Assets/Scripts..."
                class="w-full px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
              />
            </div>
            <button
              @click="addSparseFolder()"
              :disabled="!newSparseFolder.trim()"
              class="px-4 py-2 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-50"
            >
              添加目录
            </button>
          </div>

          <!-- Preset Helpers -->
          <div class="flex items-center gap-2 text-xs text-ink-muted pt-1">
            <span class="font-medium">快捷预设:</span>
            <button
              @click="addSparseFolder('frontend')"
              class="px-2 py-0.5 rounded bg-surface hover:bg-raised border border-line text-[11px] font-mono cursor-pointer"
            >
              + frontend
            </button>
            <button
              @click="addSparseFolder('src')"
              class="px-2 py-0.5 rounded bg-surface hover:bg-raised border border-line text-[11px] font-mono cursor-pointer"
            >
              + src
            </button>
            <button
              @click="addSparseFolder('docs')"
              class="px-2 py-0.5 rounded bg-surface hover:bg-raised border border-line text-[11px] font-mono cursor-pointer"
            >
              + docs
            </button>
          </div>
        </div>

        <!-- Action Buttons -->
        <div class="flex items-center justify-between pt-4 border-t border-line">
          <button
            @click="handleResetSparseCheckout"
            :disabled="isSavingSparse || sparseFolders.length === 0"
            class="px-4 py-2 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink-muted hover:text-ink transition cursor-pointer disabled:opacity-50"
          >
            取消限制 / 恢复检出全部文件
          </button>

          <button
            @click="handleApplySparseCheckout"
            :disabled="isSavingSparse || sparseFolders.length === 0"
            class="flex items-center gap-2 px-5 py-2 rounded-lg bg-purple-strong hover:bg-purple-dark text-white text-xs font-semibold transition cursor-pointer disabled:opacity-50 shadow-xs"
          >
            <Filter class="w-3.5 h-3.5" :class="{ 'animate-spin': isSavingSparse }" />
            <span>{{ isSavingSparse ? "应用规则中..." : "保存并生效稀疏检出" }}</span>
          </button>
        </div>
      </div>

</template>
