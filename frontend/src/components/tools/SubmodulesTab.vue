<script setup lang="ts">
import { confirmAction } from "../../composables/useConfirm";
import { ref, watch, onMounted, watchEffect } from "vue";
import {
  RotateCcw,
  Trash2,
  GitBranch,
  RefreshCw,
  Plus,
  Boxes,
  FolderTree,
  ArrowUpCircle,
} from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import UiModal from "../ui/UiModal.vue";
import UiButton from "../ui/UiButton.vue";
import { toolsStatus } from "../../composables/toolsStatus";
import { useModalA11y } from "../../composables/useModalA11y";
import type { ReflogItem, RemoteDetail, WorktreeInfo, LfsLockInfo, LfsTrackPattern, SubmoduleInfo, DiskBreakdownResult, LargeFileInfo, RepoMaintenanceStats, LfsPruneResult, GitFsckResult, SubtreeItem } from "../../types";

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
// 5. SUBMODULES (大型多仓协同)
// -------------------------------------------------------------
const submodules = ref<SubmoduleInfo[]>([]);
const isLoadingSubmodules = ref(false);
const isUpdatingSubmodules = ref(false);
const showAddSubmoduleModal = ref(false);
const newSubmoduleUrl = ref("");
const newSubmodulePath = ref("");
const showSetSubmoduleBranchModal = ref(false);
const selectedSubmodule = ref<SubmoduleInfo | null>(null);
const targetSubmoduleBranch = ref("");

async function loadSubmodules() {
  if (!props.currentRepoPath) return;
  isLoadingSubmodules.value = true;
  try {
    const list = await gitService.getSubmodules(props.currentRepoPath);
    submodules.value = list || [];
  } catch (err) {
    console.error("Failed to load submodules:", err);
  } finally {
    isLoadingSubmodules.value = false;
  }
}

async function handleUpdateSubmodules(recursive = true) {
  if (!props.currentRepoPath) return;
  isUpdatingSubmodules.value = true;
  try {
    const out = recursive
      ? await gitService.updateSubmodulesRecursive(props.currentRepoPath)
      : await gitService.updateSubmodules(props.currentRepoPath, true, false);
    notify(out ? `子模块更新成功: ${out.slice(0, 60)}` : "所有子模块已成功递归同步与更新");
    await loadSubmodules();
  } catch (err: any) {
    notify(`更新子模块失败: ${err?.message || err}`);
  } finally {
    isUpdatingSubmodules.value = false;
  }
}

async function handleSyncSubmodule(submodulePath: string) {
  if (!props.currentRepoPath) return;
  isUpdatingSubmodules.value = true;
  try {
    const out = await gitService.syncSubmodule(props.currentRepoPath, submodulePath);
    notify(out ? `子模块 URL 同步完成: ${out.slice(0, 60)}` : "子模块 URL 已同步");
    await loadSubmodules();
  } catch (err: any) {
    notify(`子模块 URL 同步失败: ${err?.message || err}`);
  } finally {
    isUpdatingSubmodules.value = false;
  }
}

async function handleUpdateSubmoduleRemote(submodulePath = "") {
  if (!props.currentRepoPath) return;
  isUpdatingSubmodules.value = true;
  try {
    const out = await gitService.updateSubmoduleRemote(props.currentRepoPath, submodulePath);
    notify(out ? `远端同步完成: ${out.slice(0, 60)}` : "子模块已同步至远端跟踪分支最新版本");
    await loadSubmodules();
  } catch (err: any) {
    notify(`远端同步失败: ${err?.message || err}`);
  } finally {
    isUpdatingSubmodules.value = false;
  }
}

async function handleAddSubmodule() {
  if (!props.currentRepoPath || !newSubmoduleUrl.value.trim() || !newSubmodulePath.value.trim()) return;
  try {
    await gitService.addSubmodule(props.currentRepoPath, newSubmoduleUrl.value.trim(), newSubmodulePath.value.trim());
    notify(`已成功绑定子模块: ${newSubmodulePath.value}`);
    newSubmoduleUrl.value = "";
    newSubmodulePath.value = "";
    showAddSubmoduleModal.value = false;
    await loadSubmodules();
  } catch (err: any) {
    notify(`添加子模块失败: ${err?.message || err}`);
  }
}

async function handleRemoveSubmodule(path: string) {
  if (!props.currentRepoPath || !path) return;
  if (!(await confirmAction({ message: `确定要移除子模块 "${path}" 吗？此操作将解除绑定并从工程中移除该路径。`, tone: "danger" }))) return;
  try {
    await gitService.removeSubmodule(props.currentRepoPath, path);
    notify(`子模块 ${path} 已解除绑定`);
    await loadSubmodules();
  } catch (err: any) {
    notify(`移除子模块失败: ${err?.message || err}`);
  }
}

function openSetSubmoduleBranchModal(sub: SubmoduleInfo) {
  selectedSubmodule.value = sub;
  targetSubmoduleBranch.value = sub.branch || "main";
  showSetSubmoduleBranchModal.value = true;
}

async function handleSaveSubmoduleBranch() {
  if (!props.currentRepoPath || !selectedSubmodule.value || !targetSubmoduleBranch.value.trim()) return;
  try {
    await gitService.setSubmoduleBranch(props.currentRepoPath, selectedSubmodule.value.path, targetSubmoduleBranch.value.trim());
    notify(`子模块 ${selectedSubmodule.value.name} 跟踪分支已设置为 ${targetSubmoduleBranch.value}`);
    showSetSubmoduleBranchModal.value = false;
    await loadSubmodules();
  } catch (err: any) {
    notify(`设置跟踪分支失败: ${err?.message || err}`);
  }
}


// Git subtree (vendored repository under a prefix)
const subtrees = ref<SubtreeItem[]>([]);
const isLoadingSubtrees = ref(false);
const subtreeBusy = ref("");
const showAddSubtreeModal = ref(false);
const newSubtreePrefix = ref("");
const newSubtreeUrl = ref("");
const newSubtreeBranch = ref("main");
const newSubtreeSquash = ref(true);


async function loadSubtrees() {
  if (!props.currentRepoPath) return;
  isLoadingSubtrees.value = true;
  try {
    const res = await gitService.listSubtrees(props.currentRepoPath);
    subtrees.value = res?.subtrees || [];
  } catch (err) {
    console.error("Failed to load subtrees:", err);
  } finally {
    isLoadingSubtrees.value = false;
  }
}

async function handleAddSubtree() {
  if (!props.currentRepoPath || subtreeBusy.value) return;
  const prefix = newSubtreePrefix.value.trim();
  const url = newSubtreeUrl.value.trim();
  const branch = newSubtreeBranch.value.trim();
  if (!prefix || !url || !branch) return;
  subtreeBusy.value = "add";
  try {
    const res = await gitService.addSubtree(
      props.currentRepoPath,
      prefix,
      url,
      branch,
      newSubtreeSquash.value
    );
    notify(res?.message || `子树已添加: ${prefix}`);
    showAddSubtreeModal.value = false;
    newSubtreePrefix.value = "";
    newSubtreeUrl.value = "";
    await loadSubtrees();
    emit("refresh");
  } catch (err: any) {
    notify(`添加子树失败: ${err?.message || err}`);
  } finally {
    subtreeBusy.value = "";
  }
}

async function handlePullSubtree(item: SubtreeItem) {
  if (!props.currentRepoPath || subtreeBusy.value) return;
  subtreeBusy.value = `pull:${item.prefix}`;
  try {
    const res = await gitService.pullSubtree(
      props.currentRepoPath,
      item.prefix,
      item.remoteOrUrl,
      item.branch,
      true
    );
    notify(res?.message || `子树已更新: ${item.prefix}`);
    await loadSubtrees();
    emit("refresh");
  } catch (err: any) {
    notify(`拉取子树失败: ${err?.message || err}`);
  } finally {
    subtreeBusy.value = "";
  }
}

async function handlePushSubtree(item: SubtreeItem) {
  if (!props.currentRepoPath || subtreeBusy.value) return;
  if (!(await confirmAction(`确定将子树 ${item.prefix} 的变更推送回上游 ${item.branch} 吗？`))) return;
  subtreeBusy.value = `push:${item.prefix}`;
  try {
    const res = await gitService.pushSubtree(
      props.currentRepoPath,
      item.prefix,
      item.remoteOrUrl,
      item.branch
    );
    notify(res?.message || `子树已推送: ${item.prefix}`);
    emit("refresh");
  } catch (err: any) {
    notify(`推送子树失败: ${err?.message || err}`);
  } finally {
    subtreeBusy.value = "";
  }
}

watchEffect(() => {
  toolsStatus.loading.submodules =
    isLoadingSubmodules.value || isUpdatingSubmodules.value || isLoadingSubtrees.value || !!subtreeBusy.value;
});

watchEffect(() => {
  toolsStatus.counts.submodules = submodules.value.length;
});

onMounted(() => {
  void loadSubmodules();
  void loadSubtrees();
});

watch(
  () => props.refreshNonce,
  () => {
    void loadSubmodules();
    void loadSubtrees();
  }
);
</script>

<template>
      <!-- ============================================== -->
      <!-- TAB 5: 子模块协同 (Git Submodules)             -->
      <!-- ============================================== -->
      <div class="space-y-4" data-tool-section="submodule-list">
        <div class="flex items-center justify-between pb-3 border-b border-line">
          <div>
            <h3 class="text-sm font-bold text-ink flex items-center gap-2">
              <Boxes class="w-4 h-4 text-accent" />
              <span>Git 子模块协同与子工程管理 (Git Submodules)</span>
            </h3>
            <p class="text-xs text-ink-muted mt-0.5">
              跨多仓库代码共享、游戏插件与第三方基础库协同开发。
            </p>
          </div>

          <div class="flex items-center gap-2">
            <button
              @click="handleUpdateSubmodules(true)"
              :disabled="isUpdatingSubmodules"
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-50"
              title="递归检出并同步所有子模块 (git submodule update --init --recursive)"
            >
              <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isUpdatingSubmodules }" />
              <span>递归同步所有子模块</span>
            </button>
            <button
              @click="handleUpdateSubmoduleRemote()"
              :disabled="isUpdatingSubmodules"
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-50"
              title="拉取远端跟踪分支最新版本 (git submodule update --remote)"
            >
              <RotateCcw class="w-3.5 h-3.5" />
              <span>拉取最新远端</span>
            </button>
            <button
              @click="showAddSubmoduleModal = true"
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-white text-xs font-semibold transition cursor-pointer shadow-xs"
            >
              <Plus class="w-3.5 h-3.5" />
              <span>添加子模块</span>
            </button>
          </div>
        </div>

        <!-- Submodules List -->
        <div class="space-y-3">
          <div
            v-for="sub in submodules"
            :key="sub.path"
            class="p-4 rounded-xl border border-line bg-surface/30 hover:bg-surface/50 transition flex flex-col gap-3 shadow-xs"
          >
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-3 min-w-0">
                <span class="font-bold text-sm text-ink truncate">{{ sub.name }}</span>
                <span
                  class="px-2 py-0.5 rounded text-[10px] font-bold uppercase tracking-wider font-mono"
                  :class="{
                    'bg-success/15 text-success border border-success/30': sub.status === 'synced',
                    'bg-warning/15 text-warning border border-warning/30': sub.status === 'modified',
                    'bg-purple/15 text-purple border border-purple/30': sub.status === 'uninitialized',
                    'bg-danger/15 text-danger border border-danger/30': sub.status === 'conflict',
                  }"
                >
                  {{
                    sub.status === 'synced' ? '已同步 (Synced)' :
                    sub.status === 'modified' ? '提交已变更 (Modified)' :
                    sub.status === 'uninitialized' ? '未初始化 (Uninitialized)' : '冲突 (Conflict)'
                  }}
                </span>
                <span class="text-xs text-ink-muted font-mono truncate" :title="sub.path">
                  📁 {{ sub.path }}
                </span>
              </div>

              <!-- Action buttons -->
              <div class="flex items-center gap-2 shrink-0">
                <button
                  @click="handleUpdateSubmoduleRemote(sub.path)"
                  class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-surface hover:bg-raised border border-line text-xs font-medium text-ink transition cursor-pointer"
                  title="拉取并更新此子模块至追踪分支"
                >
                  <RefreshCw class="w-3 h-3 text-accent" />
                  <span>单仓同步</span>
                </button>
                <button
                  @click="handleSyncSubmodule(sub.path)"
                  class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-surface hover:bg-raised border border-line text-xs font-medium text-ink transition cursor-pointer"
                  title="同步 .gitmodules 中的 URL 并初始化更新 (git submodule sync + update)"
                >
                  <RotateCcw class="w-3 h-3 text-warning" />
                  <span>同步 URL</span>
                </button>
                <button
                  @click="openSetSubmoduleBranchModal(sub)"
                  class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-surface hover:bg-raised border border-line text-xs font-medium text-ink transition cursor-pointer"
                  title="修改跟踪分支"
                >
                  <GitBranch class="w-3 h-3 text-purple" />
                  <span>跟踪分支: {{ sub.branch || "HEAD" }}</span>
                </button>
                <button aria-label="解绑并移除此子模块"
                  @click="handleRemoveSubmodule(sub.path)"
                  class="p-1.5 rounded-lg hover:bg-danger/20 text-ink-muted hover:text-danger transition cursor-pointer"
                  title="解绑并移除此子模块"
                >
                  <Trash2 class="w-3.5 h-3.5" />
                </button>
              </div>
            </div>

            <!-- Details: URL and Commit -->
            <div class="grid grid-cols-2 gap-4 pt-2 border-t border-line/40 text-xs font-mono">
              <div class="flex items-center gap-2 truncate text-ink-muted">
                <span class="text-ink-secondary shrink-0 font-sans font-medium">仓库地址:</span>
                <span
                  @click="copyToClipboard(sub.url, '子模块 URL')"
                  class="text-ink hover:text-accent hover:underline cursor-pointer truncate"
                  :title="sub.url"
                >
                  {{ sub.url }}
                </span>
              </div>
              <div class="flex items-center gap-2 text-ink-muted">
                <span class="text-ink-secondary shrink-0 font-sans font-medium">基准 Commit:</span>
                <span
                  @click="copyToClipboard(sub.commit, 'Commit Hash')"
                  class="text-accent hover:underline cursor-pointer font-bold shrink-0"
                  title="点击复制 Commit"
                >
                  {{ sub.commit ? sub.commit.substring(0, 10) : 'N/A' }}
                </span>
              </div>
            </div>
          </div>

          <div
            v-if="submodules.length === 0"
            class="py-12 text-center text-xs text-ink-muted border border-dashed border-line rounded-xl flex flex-col items-center gap-3"
          >
            <Boxes class="w-8 h-8 text-ink-muted/50" />
            <div>当前工程未注册任何 Git 子模块 (.gitmodules 文件不存在或为空)</div>
            <button
              @click="showAddSubmoduleModal = true"
              class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-accent transition cursor-pointer"
            >
              绑定现有子工程仓库
            </button>
          </div>
        </div>

        <!-- Git Subtree (vendored repositories) -->
        <div class="space-y-3 pt-4 border-t border-line" data-tool-section="subtree-list">
          <div class="flex items-center justify-between pb-2 border-b border-line">
            <div>
              <h3 class="text-sm font-bold text-ink flex items-center gap-2">
                <FolderTree class="w-4 h-4 text-purple" />
                <span>Git Subtree 子树协同</span>
              </h3>
              <p class="text-xs text-ink-muted mt-0.5">
                将外部仓库嵌入为普通目录，无需 `.gitmodules`；支持拉取上游更新与推送本地变更。
              </p>
            </div>
            <button
              @click="showAddSubtreeModal = true"
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-purple-strong hover:bg-purple-dark text-white text-xs font-semibold transition cursor-pointer shadow-xs"
            >
              <Plus class="w-3.5 h-3.5" />
              <span>添加子树</span>
            </button>
          </div>

          <div class="space-y-2">
            <div
              v-for="item in subtrees"
              :key="item.prefix"
              class="p-3 rounded-xl border border-line bg-surface/30 flex items-center justify-between gap-3"
            >
              <div class="min-w-0 space-y-1">
                <div class="flex items-center gap-2">
                  <span class="font-mono text-xs font-bold text-ink truncate">{{ item.prefix }}</span>
                  <span class="px-1.5 py-0.5 rounded bg-purple/15 text-purple text-[9px] font-bold shrink-0">
                    {{ item.branch }}
                  </span>
                </div>
                <div class="text-[10px] text-ink-muted font-mono truncate" :title="item.remoteOrUrl">
                  {{ item.remoteOrUrl || "未记录上游地址（可通过拉取/推送时重新填写）" }}
                </div>
                <div class="text-[10px] text-ink-muted font-mono">
                  last: {{ item.lastCommit || item.lastSquash || "—" }}
                </div>
              </div>
              <div class="flex items-center gap-2 shrink-0">
                <button
                  @click="handlePullSubtree(item)"
                  :disabled="!!subtreeBusy || !item.remoteOrUrl"
                  class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-surface hover:bg-raised border border-line text-xs font-medium text-accent transition cursor-pointer disabled:opacity-40"
                  title="拉取上游子树变更 (git subtree pull --squash)"
                >
                  <RefreshCw class="w-3 h-3" :class="{ 'animate-spin': subtreeBusy === `pull:${item.prefix}` }" />
                  <span>拉取更新</span>
                </button>
                <button
                  @click="handlePushSubtree(item)"
                  :disabled="!!subtreeBusy || !item.remoteOrUrl"
                  class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-surface hover:bg-raised border border-line text-xs font-medium text-purple transition cursor-pointer disabled:opacity-40"
                  title="将本地子树变更推送回上游 (git subtree push)"
                >
                  <ArrowUpCircle class="w-3 h-3" :class="{ 'animate-pulse': subtreeBusy === `push:${item.prefix}` }" />
                  <span>推送回上游</span>
                </button>
              </div>
            </div>

            <div v-if="subtrees.length === 0" class="py-8 text-center text-xs text-ink-muted border border-dashed border-line rounded-xl">
              {{ isLoadingSubtrees ? "正在读取子树..." : "当前工程尚未使用 Git Subtree" }}
            </div>
          </div>
        </div>
      </div>


    <!-- Modal: Add Subtree -->
    <UiModal
      :open="showAddSubtreeModal"
      label="添加 Git Subtree"
      width="max-w-md"
      @update:open="showAddSubtreeModal = $event"
    >
      <template #header>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <FolderTree class="w-4 h-4 text-purple" />
          <span>添加 Git Subtree (git subtree add)</span>
        </h3>
      </template>

      <div class="space-y-3">
        <div class="flex flex-col gap-1.5">
          <label class="text-xs text-ink-secondary font-medium">本地前缀目录 (Prefix):</label>
          <input
            v-model="newSubtreePrefix"
            type="text"
            aria-label="本地前缀目录"
            placeholder="libs/shared-core"
            class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
          />
        </div>
        <div class="flex flex-col gap-1.5">
          <label class="text-xs text-ink-secondary font-medium">上游仓库地址 (URL / 本地路径):</label>
          <input
            v-model="newSubtreeUrl"
            type="text"
            aria-label="上游仓库地址"
            placeholder="https://github.com/org/lib.git 或 /path/to/lib"
            class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
          />
        </div>
        <div class="flex flex-col gap-1.5">
          <label class="text-xs text-ink-secondary font-medium">上游分支 (Branch):</label>
          <input
            v-model="newSubtreeBranch"
            type="text"
            aria-label="上游分支"
            placeholder="main"
            class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
          />
        </div>
        <label class="flex items-center gap-2 text-[11px] text-ink-secondary cursor-pointer select-none">
          <input v-model="newSubtreeSquash" type="checkbox" class="rounded accent-[var(--ug-purple-strong)]" />
          <span>压缩上游历史 (--squash，推荐用于大型仓库)</span>
        </label>
      </div>

      <template #footer>
        <UiButton variant="ghost" @click="showAddSubtreeModal = false">取消</UiButton>
        <UiButton
          variant="purple"
          :disabled="subtreeBusy === 'add' || !newSubtreePrefix.trim() || !newSubtreeUrl.trim() || !newSubtreeBranch.trim()"
          @click="handleAddSubtree"
        >
          {{ subtreeBusy === "add" ? "添加中..." : "确认添加" }}
        </UiButton>
      </template>
    </UiModal>

    <!-- Modal: Add Submodule -->
    <UiModal
      :open="showAddSubmoduleModal"
      label="添加 Git 子模块"
      width="max-w-md"
      @update:open="showAddSubmoduleModal = $event"
    >
      <template #header>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <Boxes class="w-4 h-4 text-accent" />
          <span>添加 Git 子模块 (git submodule add)</span>
        </h3>
      </template>

      <div class="space-y-3">
        <div class="flex flex-col gap-1.5">
          <label class="text-xs text-ink-secondary font-medium">子模块远程仓库地址 (Git URL / HTTPS / SSH):</label>
          <input
            v-model="newSubmoduleUrl"
            type="text"
            aria-label="子模块仓库地址"
            placeholder="https://github.com/org/repo.git"
            class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
          />
        </div>
        <div class="flex flex-col gap-1.5">
          <label class="text-xs text-ink-secondary font-medium">本地存放相对路径 (Path):</label>
          <input
            v-model="newSubmodulePath"
            type="text"
            aria-label="子模块存放路径"
            placeholder="libs/engine-core 或 submodules/shader-lib"
            class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
          />
        </div>
      </div>

      <template #footer>
        <UiButton variant="ghost" @click="showAddSubmoduleModal = false">取消</UiButton>
        <UiButton
          variant="primary"
          :disabled="!newSubmoduleUrl.trim() || !newSubmodulePath.trim()"
          @click="handleAddSubmodule"
        >
          确认添加
        </UiButton>
      </template>
    </UiModal>

    <!-- Modal: Set Submodule Branch -->
    <UiModal
      :open="showSetSubmoduleBranchModal && Boolean(selectedSubmodule)"
      label="设置子模块跟踪分支"
      width="max-w-md"
      @update:open="showSetSubmoduleBranchModal = $event"
    >
      <template #header>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <GitBranch class="w-4 h-4 text-purple" />
          <span>设置子模块跟踪分支</span>
        </h3>
      </template>

      <div v-if="selectedSubmodule" class="space-y-3">
        <div class="flex flex-col gap-1 text-xs">
          <span class="text-ink-muted">当前子模块:</span>
          <span class="font-bold text-ink font-mono">{{ selectedSubmodule.name }} ({{ selectedSubmodule.path }})</span>
        </div>
        <div class="flex flex-col gap-1.5">
          <label class="text-xs text-ink-secondary font-medium">跟踪分支名称 (Tracked Branch):</label>
          <input
            v-model="targetSubmoduleBranch"
            type="text"
            aria-label="跟踪分支名称"
            placeholder="main / master / release/v1.0"
            class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
          />
        </div>
      </div>

      <template #footer>
        <UiButton variant="ghost" @click="showSetSubmoduleBranchModal = false">取消</UiButton>
        <UiButton
          variant="purple"
          :disabled="!targetSubmoduleBranch.trim()"
          @click="handleSaveSubmoduleBranch"
        >
          保存配置
        </UiButton>
      </template>
    </UiModal>

</template>
