<script setup lang="ts">
import { confirmAction } from "../../composables/useConfirm";
import { ref, watch, onMounted, watchEffect } from "vue";
import {
  Trash2,
  Globe,
  GitBranch,
  RefreshCw,
  Plus,
  Copy,
  FolderGit2,
  Lock,
  Unlock,
  FolderPlus,
  Wrench,
  Filter,
  Folder,
  Eraser,
  Terminal,
  ExternalLink,
  Pencil,
  Zap,
} from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import UiModal from "../ui/UiModal.vue";
import UiButton from "../ui/UiButton.vue";
import { toolsStatus } from "../../composables/toolsStatus";
import { useModalA11y } from "../../composables/useModalA11y";
import type { ReflogItem, RemoteDetail, WorktreeInfo, LfsLockInfo, LfsTrackPattern, SubmoduleInfo, DiskBreakdownResult, LargeFileInfo, RepoMaintenanceStats, LfsPruneResult, GitFsckResult, MultiRemoteStatusResult } from "../../types";

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
// 3. REMOTES & WORKTREES
// -------------------------------------------------------------
const remotes = ref<RemoteDetail[]>([]);
const worktrees = ref<WorktreeInfo[]>([]);
const multiRemoteStatus = ref<MultiRemoteStatusResult | null>(null);
const remoteLatency = ref<Record<string, string>>({});
const isLoadingRemotes = ref(false);
const remoteBusy = ref<string>("");
const isFetchingAllRemotes = ref(false);

function remoteSyncStatus(name: string) {
  return multiRemoteStatus.value?.remotes?.find((r) => r.name === name);
}

function remoteWebURL(url: string): string {
  if (/^https?:\/\//i.test(url)) return url;
  const sshMatch = url.match(/^git@([^:]+):(.+?)(?:\.git)?$/);
  if (sshMatch) return `https://${sshMatch[1]}/${sshMatch[2]}`;
  const sshProtoMatch = url.match(/^ssh:\/\/git@([^/]+)\/(.+?)(?:\.git)?$/);
  if (sshProtoMatch) return `https://${sshProtoMatch[1]}/${sshProtoMatch[2]}`;
  return "";
}

async function handleOpenRemoteWeb(name: string, url: string) {
  const target = remoteWebURL(url);
  if (!target) {
    notify(`远程 [${name}] 不是 HTTP(S) / SSH 地址，无法在浏览器打开`);
    return;
  }
  try {
    await gitService.openWebURL(target);
  } catch (err: any) {
    notify(`打开远程网页失败: ${err?.message || err}`);
  }
}

async function handleTestRemote(name: string) {
  if (!props.currentRepoPath || remoteBusy.value) return;
  remoteBusy.value = `test:${name}`;
  try {
    const res = await gitService.testRemoteConnection(props.currentRepoPath, name);
    if (res?.success) {
      remoteLatency.value = { ...remoteLatency.value, [name]: res.latency };
      notify(`远程 [${name}] 连接正常，延迟 ${res.latency}，可见 ${res.heads?.length ?? 0} 个分支`);
    } else {
      notify(`远程 [${name}] 连接失败: ${res?.error || "未知错误"}`);
    }
  } catch (err: any) {
    notify(`远程 [${name}] 连通性测试失败: ${err?.message || err}`);
  } finally {
    remoteBusy.value = "";
  }
}

const showAddRemoteModal = ref(false);
const newRemoteName = ref("origin");
const newRemoteUrl = ref("");
const isAddingRemote = ref(false);

const showEditRemoteModal = ref(false);
const editRemoteOriginalName = ref("");
const editRemoteOriginalUrl = ref("");
const editRemoteName = ref("");
const editRemoteUrl = ref("");
const isSavingRemote = ref(false);

function openEditRemote(remote: RemoteDetail) {
  editRemoteOriginalName.value = remote.name;
  editRemoteOriginalUrl.value = remote.url;
  editRemoteName.value = remote.name;
  editRemoteUrl.value = remote.url;
  showEditRemoteModal.value = true;
}

async function handleSaveRemoteEdit() {
  if (!props.currentRepoPath || isSavingRemote.value) return;
  const name = editRemoteName.value.trim();
  const url = editRemoteUrl.value.trim();
  if (!name || !url) return;
  isSavingRemote.value = true;
  try {
    if (name !== editRemoteOriginalName.value) {
      await gitService.renameRemote(props.currentRepoPath, editRemoteOriginalName.value, name);
    }
    if (url !== editRemoteOriginalUrl.value) {
      await gitService.setRemoteUrl(props.currentRepoPath, name, url);
    }
    notify(`远程仓库 ${name} 已更新`);
    showEditRemoteModal.value = false;
    await loadRemotesAndWorktrees();
    emit("refresh");
  } catch (err: any) {
    notify(`更新远程仓库失败: ${err?.message || err}`);
  } finally {
    isSavingRemote.value = false;
  }
}

const showAddWorktreeModal = ref(false);
const newWorktreePath = ref("");
const newWorktreeBranch = ref("");
const isAddingWorktree = ref(false);

async function loadRemotesAndWorktrees() {
  if (!props.currentRepoPath) return;
  isLoadingRemotes.value = true;
  try {
    const [rList, wList, status] = await Promise.all([
      gitService.getRemoteDetails(props.currentRepoPath).catch(() => []),
      gitService.getWorktrees(props.currentRepoPath).catch(() => []),
      gitService.getMultiRemoteStatus(props.currentRepoPath).catch(() => null),
    ]);
    remotes.value = rList || [];
    worktrees.value = wList || [];
    multiRemoteStatus.value = status;
  } catch (err) {
    console.error("Failed to load remotes/worktrees:", err);
  } finally {
    isLoadingRemotes.value = false;
  }
}

async function handleAddRemote() {
  if (!props.currentRepoPath || !newRemoteName.value.trim() || !newRemoteUrl.value.trim()) return;
  isAddingRemote.value = true;
  try {
    await gitService.addRemote(props.currentRepoPath, newRemoteName.value.trim(), newRemoteUrl.value.trim());
    notify(`已添加远程仓库: ${newRemoteName.value.trim()}`);
    showAddRemoteModal.value = false;
    newRemoteUrl.value = "";
    await loadRemotesAndWorktrees();
  } catch (err: any) {
    notify(`添加远程仓库失败: ${err?.message || err}`);
  } finally {
    isAddingRemote.value = false;
  }
}

async function handleRemoveRemote(name: string) {
  if (!props.currentRepoPath) return;
  if (!(await confirmAction({ message: `确定要移除远程仓库配置「${name}」吗？`, tone: "danger" }))) return;
  try {
    await gitService.removeRemote(props.currentRepoPath, name);
    notify(`已移除远程仓库: ${name}`);
    await loadRemotesAndWorktrees();
  } catch (err: any) {
    notify(`移除失败: ${err?.message || err}`);
  }
}

/** Fetch one remote with --prune: the usual large-project flow for multi-remote setups. */
async function handleFetchRemote(name: string) {
  if (!props.currentRepoPath || remoteBusy.value) return;
  remoteBusy.value = `fetch:${name}`;
  try {
    await gitService.fetch(props.currentRepoPath, name);
    notify(`远程 [${name}] 已更新并清理失效引用`);
    emit("refresh");
    await loadRemotesAndWorktrees();
  } catch (err: any) {
    notify(`获取远程 [${name}] 失败: ${err?.message || err}`);
  } finally {
    remoteBusy.value = "";
  }
}

/** git remote prune <name>: drop stale remote-tracking refs (huge repos accumulate them). */
async function handlePruneRemote(name: string) {
  if (!props.currentRepoPath || remoteBusy.value) return;
  remoteBusy.value = `prune:${name}`;
  try {
    const out = await gitService.pruneRemote(props.currentRepoPath, name);
    notify(out && out.trim() ? `远程 [${name}] 失效引用已清理` : `远程 [${name}] 无需要清理的引用`);
    await loadRemotesAndWorktrees();
  } catch (err: any) {
    notify(`清理远程 [${name}] 失效引用失败: ${err?.message || err}`);
  } finally {
    remoteBusy.value = "";
  }
}

/** Fetch every configured remote at once (git fetch --all --prune). */
async function handleFetchAllRemotes() {
  if (!props.currentRepoPath || isFetchingAllRemotes.value || remotes.value.length === 0) return;
  isFetchingAllRemotes.value = true;
  try {
    const res = await gitService.syncMultiRemotes(props.currentRepoPath, "fetch-all");
    if (res?.success) {
      notify(`已从全部 ${remotes.value.length} 个远程获取最新分支与标签索引`);
    } else {
      notify(res?.messages?.[0] || "多远程获取失败");
    }
    emit("refresh");
    await loadRemotesAndWorktrees();
  } catch (err: any) {
    notify(`全部远程获取失败: ${err?.message || err}`);
  } finally {
    isFetchingAllRemotes.value = false;
  }
}

const isWorktreeNewBranch = ref(false);
const openAfterCreateWorktree = ref(true);
const showLockWorktreeModal = ref(false);
const targetLockWorktreePath = ref("");
const lockWorktreeReason = ref("");
const isWorktreeOperation = ref(false);


const isWorktreeMaintenance = ref(false);

async function handlePruneWorktrees() {
  if (!props.currentRepoPath || isWorktreeMaintenance.value) return;
  if (!(await confirmAction({ message: "清理目录已不存在的失效工作树记录？\n\n只会移除 .git/worktrees 下的过期条目，不会删除任何文件。", tone: "warning" }))) return;
  isWorktreeMaintenance.value = true;
  try {
    const res = await gitService.pruneWorktrees(props.currentRepoPath);
    notify(res.message);
    if (res.details.length) {
      console.info("[worktree prune]", res.details.join("\n"));
    }
    await loadRemotesAndWorktrees();
    emit("refresh");
  } catch (err: any) {
    notify(`清理失效记录失败: ${err?.message || err}`);
  } finally {
    isWorktreeMaintenance.value = false;
  }
}

async function handleRepairWorktrees() {
  if (!props.currentRepoPath || isWorktreeMaintenance.value) return;
  isWorktreeMaintenance.value = true;
  try {
    const res = await gitService.repairWorktrees(props.currentRepoPath, []);
    notify(res.message);
    await loadRemotesAndWorktrees();
    emit("refresh");
  } catch (err: any) {
    notify(`修复工作树链接失败: ${err?.message || err}`);
  } finally {
    isWorktreeMaintenance.value = false;
  }
}

async function handleAddWorktree() {
  if (!props.currentRepoPath || !newWorktreePath.value.trim()) return;
  isAddingWorktree.value = true;
  const targetPath = newWorktreePath.value.trim();
  try {
    await gitService.addWorktree(
      props.currentRepoPath,
      targetPath,
      newWorktreeBranch.value.trim(),
      isWorktreeNewBranch.value
    );
    notify(`多工作树已成功挂载: ${targetPath.split("/").pop()}`);
    showAddWorktreeModal.value = false;
    newWorktreePath.value = "";
    newWorktreeBranch.value = "";
    await loadRemotesAndWorktrees();
    if (openAfterCreateWorktree.value) {
      emit("open-repo", targetPath);
    }
  } catch (err: any) {
    notify(`创建工作树失败: ${err?.message || err}`);
  } finally {
    isAddingWorktree.value = false;
  }
}

async function handleRemoveWorktree(path: string, force = false) {
  if (!props.currentRepoPath) return;
  if (!(await confirmAction({ message: `确定要移除工作树「${path}」吗？`, tone: "danger" }))) return;
  try {
    await gitService.removeWorktree(props.currentRepoPath, path, force);
    notify("工作树已安全移除");
    await loadRemotesAndWorktrees();
  } catch (err: any) {
    if (!force && await confirmAction({ message: `普通移除失败: ${err?.message || err}\n是否尝试强制移除 (--force)？`, tone: "danger" })) {
      handleRemoveWorktree(path, true);
    } else {
      notify(`移除工作树失败: ${err?.message || err}`);
    }
  }
}

function openLockWorktreeModal(path: string) {
  targetLockWorktreePath.value = path;
  lockWorktreeReason.value = "";
  showLockWorktreeModal.value = true;
}

async function handleLockWorktree() {
  if (!props.currentRepoPath || !targetLockWorktreePath.value) return;
  isWorktreeOperation.value = true;
  try {
    await gitService.lockWorktree(props.currentRepoPath, targetLockWorktreePath.value, lockWorktreeReason.value.trim());
    notify("工作树已成功锁定，防止误删");
    showLockWorktreeModal.value = false;
    await loadRemotesAndWorktrees();
  } catch (err: any) {
    notify(`锁定工作树失败: ${err?.message || err}`);
  } finally {
    isWorktreeOperation.value = false;
  }
}

async function handleUnlockWorktree(path: string) {
  if (!props.currentRepoPath) return;
  try {
    await gitService.unlockWorktree(props.currentRepoPath, path);
    notify("工作树锁定已解除");
    await loadRemotesAndWorktrees();
  } catch (err: any) {
    notify(`解除锁定失败: ${err?.message || err}`);
  }
}

function handleOpenWorktreeAsTab(path: string) {
  emit("open-repo", path);
}

async function handleOpenWorktreeInTerminal(path: string) {
  try {
    await gitService.openInTerminal(path);
  } catch (err: any) {
    notify(`打开终端失败: ${err?.message || err}`);
  }
}

async function handleOpenWorktreeInFinder(path: string) {
  try {
    await gitService.openInFinder(path);
  } catch (err: any) {
    notify(`打开文件管理器失败: ${err?.message || err}`);
  }
}


watchEffect(() => {
  toolsStatus.loading.remotes = isLoadingRemotes.value || isFetchingAllRemotes.value || isWorktreeOperation.value;
});

watchEffect(() => {
  toolsStatus.counts.worktrees = worktrees.value.length;
});

onMounted(() => {
  void loadRemotesAndWorktrees();
});

watch(
  () => props.refreshNonce,
  () => {
    void loadRemotesAndWorktrees();
  }
);
</script>

<template>
      <!-- ============================================== -->
      <!-- TAB 3: 远程仓库与多工作树协作 (Remotes & Trees)   -->
      <!-- ============================================== -->
      <div v-if="mode === 'remotes'" class="space-y-6" data-tool-section="remotes">
        <!-- Section A: 远程仓库 Remotes -->
        <div class="space-y-3">
          <div class="flex items-center justify-between pb-2 border-b border-line">
            <div>
              <h3 class="text-sm font-bold text-ink flex items-center gap-2">
                <Globe class="w-4 h-4 text-accent" />
                <span>已配置远程仓库 (Remotes)</span>
              </h3>
            </div>
            <div class="flex items-center gap-2">
              <button
                @click="handleFetchAllRemotes"
                :disabled="isFetchingAllRemotes || remotes.length === 0"
                class="flex items-center gap-1 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs text-ink-secondary hover:text-ink font-medium transition cursor-pointer shadow-xs disabled:opacity-50 disabled:cursor-not-allowed"
                title="从所有远程获取最新分支与标签索引 (git fetch --all --prune)"
              >
                <RefreshCw class="w-3.5 h-3.5 text-cyan" :class="{ 'animate-spin': isFetchingAllRemotes }" />
                <span>全部 Fetch</span>
              </button>
              <button
                @click="showAddRemoteModal = true"
                class="flex items-center gap-1 px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-white text-xs font-semibold transition cursor-pointer shadow-xs"
              >
                <Plus class="w-3.5 h-3.5" />
                <span>添加远程仓库</span>
              </button>
            </div>
          </div>

          <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
            <div
              v-for="r in remotes"
              :key="r.name"
              class="p-3.5 rounded-xl border border-line bg-surface/40 flex items-center justify-between shadow-xs"
            >
              <div class="flex flex-col gap-1 min-w-0 pr-2">
                <div class="flex items-center gap-2">
                  <span class="font-bold text-xs text-ink">{{ r.name }}</span>
                  <span class="px-1.5 py-0.2 rounded bg-accent/20 text-accent font-mono text-[9px] uppercase font-bold">
                    {{ r.type }}
                  </span>
                  <span
                    v-if="remoteSyncStatus(r.name)"
                    class="px-1.5 py-0.2 rounded bg-surface border border-line text-ink-muted font-mono text-[9px] shrink-0"
                    :title="`当前分支 ${remoteSyncStatus(r.name)?.branch || ''} 相对该远程的领先/落后`"
                  >
                    ↑{{ remoteSyncStatus(r.name)?.ahead ?? 0 }} ↓{{ remoteSyncStatus(r.name)?.behind ?? 0 }}
                  </span>
                  <span
                    v-if="remoteLatency[r.name]"
                    class="px-1.5 py-0.2 rounded bg-cyan/15 text-cyan font-mono text-[9px] shrink-0"
                    title="最近一次连通性测试延迟"
                  >
                    {{ remoteLatency[r.name] }}
                  </span>
                </div>
                <span class="font-mono text-[11px] text-ink-muted truncate" :title="r.url">{{ r.url }}</span>
              </div>
              <div class="flex items-center gap-1 shrink-0">
                <button aria-label="获取该远程最新索引并清理失效引用 (Fetch --prune)"
                  @click="handleFetchRemote(r.name)"
                  :disabled="!!remoteBusy"
                  class="p-1.5 rounded-lg hover:bg-cyan/20 text-ink-muted hover:text-cyan transition cursor-pointer disabled:opacity-50"
                  title="获取该远程最新索引并清理失效引用 (Fetch --prune)"
                >
                  <RefreshCw class="w-4 h-4" :class="{ 'animate-spin': remoteBusy === `fetch:${r.name}` }" />
                </button>
                <button aria-label="清理该远程已失效的分支引用 (git remote prune)"
                  @click="handlePruneRemote(r.name)"
                  :disabled="!!remoteBusy"
                  class="p-1.5 rounded-lg hover:bg-warning/20 text-ink-muted hover:text-warning transition cursor-pointer disabled:opacity-50"
                  title="清理该远程已失效的分支引用 (git remote prune)"
                >
                  <Eraser class="w-4 h-4" />
                </button>
                <button aria-label="在浏览器中打开远程仓库网页"
                  @click="handleOpenRemoteWeb(r.name, r.url)"
                  class="p-1.5 rounded-lg hover:bg-accent/20 text-ink-muted hover:text-accent transition cursor-pointer"
                  title="在浏览器中打开远程仓库网页"
                >
                  <ExternalLink class="w-4 h-4" />
                </button>
                <button aria-label="测试远程连通性与延迟 (git ls-remote)"
                  @click="handleTestRemote(r.name)"
                  :disabled="!!remoteBusy"
                  class="p-1.5 rounded-lg hover:bg-cyan/20 text-ink-muted hover:text-cyan transition cursor-pointer disabled:opacity-50"
                  title="测试远程连通性与延迟 (git ls-remote)"
                >
                  <Zap class="w-4 h-4" :class="{ 'animate-pulse': remoteBusy === `test:${r.name}` }" />
                </button>
                <button aria-label="编辑远程名称或 URL"
                  @click="openEditRemote(r)"
                  class="p-1.5 rounded-lg hover:bg-accent/20 text-ink-muted hover:text-accent transition cursor-pointer"
                  title="编辑远程名称或 URL"
                >
                  <Pencil class="w-4 h-4" />
                </button>
                <button aria-label="移除此远程仓库"
                  @click="handleRemoveRemote(r.name)"
                  class="p-1.5 rounded-lg hover:bg-danger/20 text-ink-muted hover:text-danger transition cursor-pointer shrink-0"
                  title="移除此远程仓库"
                >
                  <Trash2 class="w-4 h-4" />
                </button>
              </div>
            </div>

            <div v-if="remotes.length === 0" class="col-span-2 py-6 text-center text-xs text-ink-muted border border-dashed border-line rounded-xl">
              暂无配置任何远程仓库地址
            </div>
          </div>
        </div>
      </div>

      <!-- ============================================== -->
      <!-- TAB: 多工作树工作台 (Git Worktrees Switchboard) -->
      <!-- ============================================== -->
      <div v-if="mode === 'worktrees'" class="space-y-6" data-tool-section="worktrees">
        <!-- Overview Header -->
        <div class="flex items-center justify-between pb-3 border-b border-line">
          <div>
            <h3 class="text-sm font-bold text-ink flex items-center gap-2">
              <FolderGit2 class="w-4 h-4 text-purple" />
              <span>多工作树并行开发工作台 (Git Worktree Switchboard)</span>
            </h3>
            <p class="text-xs text-ink-muted mt-0.5">
              在不影响主工程的前提下，于独立的物理目录并行检出多个分支协同开发，无需切换分支、无需重编依赖或重刷引擎缓存。
            </p>
          </div>

          <div class="flex items-center gap-2">
            <button
              @click="handlePruneWorktrees"
              :disabled="isWorktreeMaintenance"
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-40"
              title="清理目录已被删除的失效工作树记录（--expire=now，立即生效）"
            >
              <Trash2 class="w-3.5 h-3.5 text-warning" />
              <span>清理失效记录</span>
            </button>
            <button
              @click="handleRepairWorktrees"
              :disabled="isWorktreeMaintenance"
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-40"
              title="仓库目录被移动后，重新链接各工作树的 .git 指针"
            >
              <Wrench class="w-3.5 h-3.5 text-accent" />
              <span>修复链接</span>
            </button>
            <button
              @click="showAddWorktreeModal = true"
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-purple-strong hover:bg-purple-dark text-white text-xs font-semibold transition cursor-pointer shadow-xs"
            >
              <FolderPlus class="w-3.5 h-3.5" />
              <span>挂载新工作树</span>
            </button>
          </div>
        </div>

        <!-- Metrics Overview Cards -->
        <div class="grid grid-cols-3 gap-4">
          <div class="p-4 rounded-xl border border-line bg-surface/40 flex items-center gap-3">
            <div class="w-10 h-10 rounded-xl bg-purple/15 text-purple flex items-center justify-center shrink-0">
              <FolderGit2 class="w-5 h-5" />
            </div>
            <div>
              <span class="text-[10px] text-ink-muted block uppercase tracking-wider">工作树总数</span>
              <span class="text-base font-bold text-ink font-mono">{{ worktrees.length }} 处</span>
            </div>
          </div>

          <div class="p-4 rounded-xl border border-line bg-surface/40 flex items-center gap-3">
            <div class="w-10 h-10 rounded-xl bg-accent/15 text-accent flex items-center justify-center shrink-0">
              <GitBranch class="w-5 h-5" />
            </div>
            <div>
              <span class="text-[10px] text-ink-muted block uppercase tracking-wider">关联工作树 (Linked)</span>
              <span class="text-base font-bold text-ink font-mono">
                {{ worktrees.filter(w => !w.isBare && !w.isMain).length }} 处
              </span>
            </div>
          </div>

          <div class="p-4 rounded-xl border border-line bg-surface/40 flex items-center gap-3">
            <div class="w-10 h-10 rounded-xl bg-warning/15 text-warning flex items-center justify-center shrink-0">
              <Lock class="w-5 h-5" />
            </div>
            <div>
              <span class="text-[10px] text-ink-muted block uppercase tracking-wider">锁定保护状态</span>
              <span class="text-base font-bold text-ink font-mono">
                {{ worktrees.filter(w => w.isLocked).length }} 处锁定
              </span>
            </div>
          </div>
        </div>

        <!-- Worktrees List -->
        <div class="space-y-3">
          <div
            v-for="w in worktrees"
            :key="w.path"
            class="p-4 rounded-xl border border-line bg-surface/40 flex items-center justify-between gap-4 shadow-xs hover:border-line-strong transition"
          >
            <div class="flex items-center gap-3 min-w-0 flex-1">
              <div
                class="w-9 h-9 rounded-xl flex items-center justify-center shrink-0 font-bold text-xs"
                :class="w.isMain ? 'bg-accent/15 text-accent' : 'bg-purple/15 text-purple'"
              >
                <FolderGit2 class="w-4 h-4" />
              </div>

              <div class="flex flex-col min-w-0 flex-1">
                <div class="flex items-center gap-2">
                  <span class="px-2 py-0.5 rounded font-mono font-bold text-xs shrink-0 bg-purple/20 text-purple">
                    {{ w.branch || "HEAD (Detached)" }}
                  </span>
                  <span
                    v-if="w.isMain"
                    class="px-2 py-0.2 rounded text-[10px] font-bold bg-accent/15 text-accent border border-accent/20"
                  >
                    当前主工作区 (Main)
                  </span>
                  <span
                    v-else
                    class="px-2 py-0.2 rounded text-[10px] font-medium bg-line/60 text-ink-muted"
                  >
                    Linked Worktree
                  </span>
                  <span
                    v-if="w.isLocked"
                    class="px-2 py-0.2 rounded text-[10px] font-bold bg-warning/15 text-warning border border-warning/20 flex items-center gap-1"
                  >
                    <Lock class="w-2.5 h-2.5" /> {{ w.lockReason || '已锁定防误删' }}
                  </span>
                </div>

                <div class="flex items-center gap-2 mt-1 text-xs font-mono text-ink-muted truncate">
                  <span class="text-ink font-medium truncate" :title="w.path">{{ w.path }}</span>
                  <button aria-label="复制路径"
                    @click="copyToClipboard(w.path, '工作树路径')"
                    class="p-0.5 rounded hover:bg-raised text-ink-muted hover:text-ink cursor-pointer shrink-0"
                    title="复制路径"
                  >
                    <Copy class="w-3 h-3" />
                  </button>
                  <span class="shrink-0">• Commit: <strong class="text-accent">{{ w.head ? w.head.substring(0, 8) : 'N/A' }}</strong></span>
                </div>
              </div>
            </div>

            <!-- Actions -->
            <div class="flex items-center gap-2 shrink-0">
              <button
                v-if="!w.isMain"
                @click="handleOpenWorktreeAsTab(w.path)"
                class="flex items-center gap-1 px-3 py-1.5 rounded-lg bg-accent/15 hover:bg-accent/25 text-accent text-xs font-semibold transition cursor-pointer border border-accent/30"
                title="在 wgit 顶栏多仓库标签页中打开此工作树"
              >
                <ExternalLink class="w-3.5 h-3.5" />
                <span>在新标签页打开</span>
              </button>

              <button aria-label="在终端 (Terminal) 中打开"
                @click="handleOpenWorktreeInTerminal(w.path)"
                class="p-1.5 rounded-lg hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer"
                title="在终端 (Terminal) 中打开"
              >
                <Terminal class="w-4 h-4" />
              </button>

              <button aria-label="在文件管理器 (Finder) 中显示"
                @click="handleOpenWorktreeInFinder(w.path)"
                class="p-1.5 rounded-lg hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer"
                title="在文件管理器 (Finder) 中显示"
              >
                <Folder class="w-4 h-4" />
              </button>

              <button aria-label="锁定此工作树，防止误删"
                v-if="!w.isMain && !w.isLocked"
                @click="openLockWorktreeModal(w.path)"
                class="p-1.5 rounded-lg hover:bg-warning/20 text-ink-muted hover:text-warning transition cursor-pointer"
                title="锁定此工作树，防止误删"
              >
                <Lock class="w-4 h-4" />
              </button>

              <button aria-label="解除工作树锁定"
                v-if="!w.isMain && w.isLocked"
                @click="handleUnlockWorktree(w.path)"
                class="p-1.5 rounded-lg hover:bg-success/20 text-warning hover:text-success transition cursor-pointer"
                title="解除工作树锁定"
              >
                <Unlock class="w-4 h-4" />
              </button>

              <button aria-label="移除该工作树 (git worktree remove)"
                v-if="!w.isMain"
                @click="handleRemoveWorktree(w.path, false)"
                class="p-1.5 rounded-lg hover:bg-danger/20 text-ink-muted hover:text-danger transition cursor-pointer"
                title="移除该工作树 (git worktree remove)"
              >
                <Trash2 class="w-4 h-4" />
              </button>
            </div>
          </div>

          <div v-if="worktrees.length === 0" class="py-12 text-center text-xs text-ink-muted border border-dashed border-line rounded-xl">
            暂无已挂载的工作树
          </div>
        </div>
      </div>


    <!-- Modal: Add Remote -->
    <UiModal
      :open="showAddRemoteModal"
      label="添加远程仓库"
      width="max-w-md"
      @update:open="showAddRemoteModal = $event"
    >
      <template #header>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <Globe class="w-4 h-4 text-accent" />
          <span>添加远程仓库 (git remote add)</span>
        </h3>
      </template>

      <div class="space-y-3">
        <div class="flex flex-col gap-1.5">
          <label class="text-xs text-ink-secondary font-medium">远程别名 (Remote Name):</label>
          <input
            v-model="newRemoteName"
            type="text"
            aria-label="远程别名"
            placeholder="origin / upstream / backup"
            class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
          />
        </div>
        <div class="flex flex-col gap-1.5">
          <label class="text-xs text-ink-secondary font-medium">远程 URL 地址 (Git / HTTPS / SSH):</label>
          <input
            v-model="newRemoteUrl"
            type="text"
            aria-label="远程 URL"
            placeholder="git@github.com:user/repo.git"
            class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
          />
        </div>
      </div>

      <template #footer>
        <UiButton variant="ghost" @click="showAddRemoteModal = false">取消</UiButton>
        <UiButton
          variant="primary"
          :disabled="isAddingRemote || !newRemoteName.trim() || !newRemoteUrl.trim()"
          @click="handleAddRemote"
        >
          {{ isAddingRemote ? "添加中..." : "确认添加" }}
        </UiButton>
      </template>
    </UiModal>

    <!-- Modal: Edit Remote -->
    <UiModal
      :open="showEditRemoteModal"
      label="编辑远程仓库"
      width="max-w-md"
      @update:open="showEditRemoteModal = $event"
    >
      <template #header>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <Pencil class="w-4 h-4 text-accent" />
          <span>编辑远程仓库 (git remote rename / set-url)</span>
        </h3>
      </template>

      <div class="space-y-3">
        <div class="flex flex-col gap-1.5">
          <label class="text-xs text-ink-secondary font-medium">远程别名 (Remote Name):</label>
          <input
            v-model="editRemoteName"
            type="text"
            aria-label="远程别名"
            placeholder="origin / upstream / backup"
            class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
          />
        </div>
        <div class="flex flex-col gap-1.5">
          <label class="text-xs text-ink-secondary font-medium">远程 URL 地址 (Fetch URL):</label>
          <input
            v-model="editRemoteUrl"
            type="text"
            aria-label="远程 URL"
            placeholder="git@github.com:user/repo.git"
            class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
          />
        </div>
      </div>

      <template #footer>
        <UiButton variant="ghost" @click="showEditRemoteModal = false">取消</UiButton>
        <UiButton
          variant="primary"
          :disabled="isSavingRemote || !editRemoteName.trim() || !editRemoteUrl.trim()"
          @click="handleSaveRemoteEdit"
        >
          {{ isSavingRemote ? "保存中..." : "保存修改" }}
        </UiButton>
      </template>
    </UiModal>

    <!-- Modal: Add Worktree -->
    <UiModal
      :open="showAddWorktreeModal"
      label="挂载新工作树"
      width="max-w-md"
      @update:open="showAddWorktreeModal = $event"
    >
      <template #header>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <FolderPlus class="w-4 h-4 text-purple" />
          <span>挂载新工作树 (git worktree add)</span>
        </h3>
      </template>

      <div class="space-y-3">
        <div class="flex flex-col gap-1.5">
          <label class="text-xs text-ink-secondary font-medium">本地工作树目录路径:</label>
          <input
            v-model="newWorktreePath"
            type="text"
            aria-label="工作树目录路径"
            placeholder="../project-worktree-feat"
            class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
          />
        </div>
        <div class="flex flex-col gap-1.5">
          <label class="text-xs text-ink-secondary font-medium">关联分支名称 (可选，留空将自动根据路径生成):</label>
          <input
            v-model="newWorktreeBranch"
            type="text"
            aria-label="关联分支名称"
            placeholder="feature/parallel-work"
            class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
          />
        </div>
        <div class="flex flex-col gap-2 pt-1 border-t border-line/60">
          <label class="flex items-center gap-2 text-xs text-ink cursor-pointer">
            <input v-model="isWorktreeNewBranch" type="checkbox" class="w-4 h-4 rounded accent-[var(--ug-purple-strong)] cursor-pointer" />
            <span>基于此分支名称创建并检出全新分支 (-b)</span>
          </label>
          <label class="flex items-center gap-2 text-xs text-ink cursor-pointer">
            <input v-model="openAfterCreateWorktree" type="checkbox" class="w-4 h-4 rounded accent-[var(--ug-accent-strong)] cursor-pointer" />
            <span>挂载成功后立即在 wgit 顶栏多仓库新标签页中打开</span>
          </label>
        </div>
      </div>

      <template #footer>
        <UiButton variant="ghost" @click="showAddWorktreeModal = false">取消</UiButton>
        <UiButton
          variant="purple"
          :disabled="isAddingWorktree || !newWorktreePath.trim()"
          @click="handleAddWorktree"
        >
          {{ isAddingWorktree ? "创建中..." : "确认创建" }}
        </UiButton>
      </template>
    </UiModal>

    <!-- Modal: Lock Worktree -->
    <UiModal
      :open="showLockWorktreeModal"
      label="锁定工作树"
      width="max-w-md"
      @update:open="showLockWorktreeModal = $event"
    >
      <template #header>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <Lock class="w-4 h-4 text-warning" />
          <span>锁定工作树 (git worktree lock)</span>
        </h3>
      </template>

      <div class="space-y-3">
        <div class="flex flex-col gap-1 text-xs">
          <span class="text-ink-secondary">目标工作树路径:</span>
          <span class="p-2 rounded-lg bg-canvas border border-line font-mono text-ink text-[11px] truncate">
            {{ targetLockWorktreePath }}
          </span>
        </div>
        <div class="flex flex-col gap-1.5">
          <label class="text-xs text-ink-secondary font-medium">锁定原因备注 (可选):</label>
          <input
            v-model="lockWorktreeReason"
            type="text"
            aria-label="锁定原因"
            placeholder="正在运行仿真构建 / 保留备查"
            @keydown.enter="handleLockWorktree"
            class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink focus:outline-none focus:border-accent"
          />
        </div>
      </div>

      <template #footer>
        <UiButton variant="ghost" @click="showLockWorktreeModal = false">取消</UiButton>
        <UiButton variant="warning" :disabled="isWorktreeOperation" @click="handleLockWorktree">
          {{ isWorktreeOperation ? "锁定中..." : "确认锁定" }}
        </UiButton>
      </template>
    </UiModal>

</template>
