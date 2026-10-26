<script setup lang="ts">
import { t } from "../../i18n";
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
  KeyRound,
  UploadCloud,
} from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import UiModal from "../ui/UiModal.vue";
import UiButton from "../ui/UiButton.vue";
import { toolsStatus } from "../../composables/toolsStatus";
import type { ReflogItem, RemoteDetail, WorktreeInfo, LfsLockInfo, LfsTrackPattern, SubmoduleInfo, DiskBreakdownResult, LargeFileInfo, RepoMaintenanceStats, LfsPruneResult, GitFsckResult, MultiRemoteStatusResult, AuthDiagnosticResult } from "../../types";

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
  notify(t("remotes.toast.copied", { label }));
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

function extractHostFromUrl(url: string): string {
  const trimmed = url.trim();
  const sshMatch = trimmed.match(/@([^:/]+)/);
  if (sshMatch?.[1]) return sshMatch[1];
  const httpMatch = trimmed.match(/https?:\/\/([^/]+)/);
  if (httpMatch?.[1]) return httpMatch[1];
  return "github.com";
}

function convertRemoteUrlProtocol(url: string, toType: "ssh" | "https"): string {
  const trimmed = url.trim();
  if (!trimmed) return "";
  if (toType === "ssh") {
    const m = trimmed.match(/^https?:\/\/([^/]+)\/(.+?)(?:\.git)?$/i);
    if (m) {
      return `git@${m[1]}:${m[2]}.git`;
    }
  } else {
    const m = trimmed.match(/^(?:ssh:\/\/)?git@([^:/]+)(?::|\/)(.+?)(?:\.git)?$/i);
    if (m) {
      return `https://${m[1]}/${m[2]}.git`;
    }
  }
  return trimmed;
}

async function handleOpenRemoteWeb(name: string, url: string) {
  const target = remoteWebURL(url);
  if (!target) {
    notify(t("remotes.toast.not.http", { name }));
    return;
  }
  try {
    await gitService.openWebURL(target);
  } catch (err: any) {
    notify(t("remotes.toast.open.web.failed", { error: err?.message || err }));
  }
}

async function handleTestRemote(name: string) {
  if (!props.currentRepoPath || remoteBusy.value) return;
  remoteBusy.value = `test:${name}`;
  try {
    const res = await gitService.testRemoteConnection(props.currentRepoPath, name);
    if (res?.success) {
      remoteLatency.value = { ...remoteLatency.value, [name]: res.latency };
      notify(t("remotes.toast.test.ok", { name, latency: res.latency, count: res.heads?.length ?? 0 }));
    } else {
      notify(t("remotes.toast.test.failed", { name, error: res?.error || t("remotes.unknown.error") }));
    }
  } catch (err: any) {
    notify(t("remotes.toast.test.error", { error: err?.message || err }));
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
    notify(t("remotes.toast.updated", { name }));
    showEditRemoteModal.value = false;
    await loadRemotesAndWorktrees();
    emit("refresh");
  } catch (err: any) {
    notify(t("remotes.toast.update.failed", { error: err?.message || err }));
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
    const [rRes, wRes, sRes] = await Promise.allSettled([
      gitService.getRemoteDetails(props.currentRepoPath),
      gitService.getWorktrees(props.currentRepoPath),
      gitService.getMultiRemoteStatus(props.currentRepoPath),
    ]);
    const errors: string[] = [];
    if (rRes.status === "fulfilled") {
      remotes.value = rRes.value || [];
    } else {
      errors.push(String((rRes.reason as any)?.message || rRes.reason));
    }
    if (wRes.status === "fulfilled") {
      worktrees.value = wRes.value || [];
    } else {
      errors.push(String((wRes.reason as any)?.message || wRes.reason));
    }
    if (sRes.status === "fulfilled") {
      multiRemoteStatus.value = sRes.value;
    } else {
      errors.push(String((sRes.reason as any)?.message || sRes.reason));
    }
    if (errors.length > 0) {
      notify(t("remotes.toast.load.failed", { error: errors.join("; ") }));
    }
  } finally {
    isLoadingRemotes.value = false;
  }
}

async function handleAddRemote() {
  if (!props.currentRepoPath || !newRemoteName.value.trim() || !newRemoteUrl.value.trim()) return;
  isAddingRemote.value = true;
  try {
    await gitService.addRemote(props.currentRepoPath, newRemoteName.value.trim(), newRemoteUrl.value.trim());
    notify(t("remotes.toast.added", { name: newRemoteName.value.trim() }));
    showAddRemoteModal.value = false;
    newRemoteUrl.value = "";
    await loadRemotesAndWorktrees();
  } catch (err: any) {
    notify(t("remotes.toast.add.failed", { error: err?.message || err }));
  } finally {
    isAddingRemote.value = false;
  }
}

async function handleRemoveRemote(name: string) {
  if (!props.currentRepoPath) return;
  if (!(await confirmAction({ message: t("remotes.confirm.remove", { name }), tone: "danger" }))) return;
  try {
    await gitService.removeRemote(props.currentRepoPath, name);
    notify(t("remotes.toast.removed", { name }));
    await loadRemotesAndWorktrees();
  } catch (err: any) {
    notify(t("remotes.toast.remove.failed", { error: err?.message || err }));
  }
}

/** Fetch one remote with --prune: the usual large-project flow for multi-remote setups. */
async function handleFetchRemote(name: string) {
  if (!props.currentRepoPath || remoteBusy.value) return;
  remoteBusy.value = `fetch:${name}`;
  try {
    await gitService.fetch(props.currentRepoPath, name);
    notify(t("remotes.toast.fetched.pruned", { name }));
    emit("refresh");
    await loadRemotesAndWorktrees();
  } catch (err: any) {
    notify(t("remotes.toast.fetch.one.failed", { error: err?.message || err }));
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
    notify(out && out.trim() ? t("remotes.toast.pruned", { name }) : t("remotes.toast.prune.none", { name }));
    await loadRemotesAndWorktrees();
  } catch (err: any) {
    notify(t("remotes.toast.prune.failed", { error: err?.message || err }));
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
      notify(t("remotes.toast.fetch.all.done", { count: remotes.value.length }));
    } else {
      notify(res?.messages?.[0] || t("remotes.toast.fetch.all.failed.short"));
    }
    emit("refresh");
    await loadRemotesAndWorktrees();
  } catch (err: any) {
    notify(t("remotes.toast.fetch.all.failed", { error: err?.message || err }));
  } finally {
    isFetchingAllRemotes.value = false;
  }
}

/** Push current branch to all configured remotes (git push <remote> <branch>). */
const isPushingAllRemotes = ref(false);

async function handlePushAllRemotes() {
  if (!props.currentRepoPath || isPushingAllRemotes.value || remotes.value.length === 0) return;
  const branch = multiRemoteStatus.value?.currentBranch || "";
  const confirmed = await confirmAction({
    message: t("remotes.confirm.push.all", { branch: branch || "HEAD", count: remotes.value.length }),
    tone: "default",
  });
  if (!confirmed) return;

  isPushingAllRemotes.value = true;
  try {
    const res = await gitService.syncMultiRemotes(props.currentRepoPath, "push-all");
    if (res?.success) {
      notify(t("remotes.toast.push.all.done", { count: remotes.value.length }));
    } else {
      notify(res?.messages?.join("; ") || t("remotes.toast.push.all.failed.short"));
    }
    emit("refresh");
    await loadRemotesAndWorktrees();
  } catch (err: any) {
    notify(t("remotes.toast.push.all.failed", { error: err?.message || err }));
  } finally {
    isPushingAllRemotes.value = false;
  }
}

// SSH / Credentials Diagnosis
const showAuthDiagModal = ref(false);
const authDiagHost = ref("github.com");
const isRunningAuthDiag = ref(false);
const authDiagResult = ref<AuthDiagnosticResult | null>(null);

async function handleRunAuthDiag(hostOverride?: string) {
  if (!props.currentRepoPath || isRunningAuthDiag.value) return;
  const targetHost = (hostOverride || authDiagHost.value || "github.com").trim();
  authDiagHost.value = targetHost;
  isRunningAuthDiag.value = true;
  try {
    const res = await gitService.diagnoseGitAuth(props.currentRepoPath, targetHost);
    authDiagResult.value = res;
    if (res?.sshAuthSuccess) {
      notify(t("remotes.toast.auth.ok", { host: targetHost }));
    } else {
      notify(t("remotes.toast.auth.warn", { host: targetHost }));
    }
  } catch (err: any) {
    notify(t("remotes.toast.auth.failed", { error: err?.message || err }));
  } finally {
    isRunningAuthDiag.value = false;
  }
}

function openAuthDiagModal(host?: string) {
  if (host) {
    authDiagHost.value = host;
  } else if (remotes.value.length > 0 && remotes.value[0]?.url) {
    authDiagHost.value = extractHostFromUrl(remotes.value[0].url);
  } else {
    authDiagHost.value = "github.com";
  }
  showAuthDiagModal.value = true;
  void handleRunAuthDiag(authDiagHost.value);
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
  if (!(await confirmAction({ message: t("remotes.confirm.prune.stale"), tone: "warning" })))
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
    notify(t("remotes.toast.prune.stale.failed", { error: err?.message || err }));
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
    notify(t("remotes.toast.repair.failed", { error: err?.message || err }));
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
    notify(t("remotes.toast.worktree.added", { name: targetPath.split("/").pop() ?? targetPath }));
    showAddWorktreeModal.value = false;
    newWorktreePath.value = "";
    newWorktreeBranch.value = "";
    await loadRemotesAndWorktrees();
    if (openAfterCreateWorktree.value) {
      emit("open-repo", targetPath);
    }
  } catch (err: any) {
    notify(t("remotes.toast.worktree.add.failed", { error: err?.message || err }));
  } finally {
    isAddingWorktree.value = false;
  }
}

async function handleRemoveWorktree(path: string, force = false) {
  if (!props.currentRepoPath) return;
  if (!(await confirmAction({ message: t("remotes.confirm.worktree.remove", { path }), tone: "danger" }))) return;
  try {
    await gitService.removeWorktree(props.currentRepoPath, path, force);
    notify(t("remotes.toast.worktree.removed"));
    await loadRemotesAndWorktrees();
  } catch (err: any) {
    if (!force && await confirmAction({ message: t("remotes.confirm.worktree.force", { error: err?.message || err }), tone: "danger" })) {
      handleRemoveWorktree(path, true);
    } else {
      notify(t("remotes.toast.worktree.remove.failed", { error: err?.message || err }));
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
    notify(t("remotes.toast.worktree.locked"));
    showLockWorktreeModal.value = false;
    await loadRemotesAndWorktrees();
  } catch (err: any) {
    notify(t("remotes.toast.worktree.lock.failed", { error: err?.message || err }));
  } finally {
    isWorktreeOperation.value = false;
  }
}

async function handleUnlockWorktree(path: string) {
  if (!props.currentRepoPath) return;
  try {
    await gitService.unlockWorktree(props.currentRepoPath, path);
    notify(t("remotes.toast.worktree.unlocked"));
    await loadRemotesAndWorktrees();
  } catch (err: any) {
    notify(t("remotes.toast.worktree.unlock.failed", { error: err?.message || err }));
  }
}

function handleOpenWorktreeAsTab(path: string) {
  emit("open-repo", path);
}

async function handleOpenWorktreeInTerminal(path: string) {
  try {
    await gitService.openInTerminal(path);
  } catch (err: any) {
    notify(t("remotes.toast.terminal.failed", { error: err?.message || err }));
  }
}

async function handleOpenWorktreeInFinder(path: string) {
  try {
    await gitService.openInFinder(path);
  } catch (err: any) {
    notify(t("remotes.toast.finder.failed", { error: err?.message || err }));
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
                <span>{{ t("remotes.title") }}</span>
              </h3>
            </div>
            <div class="flex items-center gap-2">
              <button
                @click="handleFetchAllRemotes"
                :disabled="isFetchingAllRemotes || remotes.length === 0"
                class="flex items-center gap-1 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs text-ink-secondary hover:text-ink font-medium transition cursor-pointer shadow-xs disabled:opacity-50 disabled:cursor-not-allowed"
                :title="t('remotes.fetch.all.title')"
              >
                <RefreshCw class="w-3.5 h-3.5 text-cyan" :class="{ 'animate-spin': isFetchingAllRemotes }" />
                <span>{{ t("remotes.fetch.all") }}</span>
              </button>
              <button
                @click="handlePushAllRemotes"
                :disabled="isPushingAllRemotes || remotes.length === 0"
                class="flex items-center gap-1 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs text-ink-secondary hover:text-ink font-medium transition cursor-pointer shadow-xs disabled:opacity-50 disabled:cursor-not-allowed"
                :title="t('remotes.push.all.title')"
              >
                <UploadCloud class="w-3.5 h-3.5 text-accent" :class="{ 'animate-pulse': isPushingAllRemotes }" />
                <span>{{ isPushingAllRemotes ? t("common.loading") : t("remotes.push.all") }}</span>
              </button>
              <button
                @click="openAuthDiagModal()"
                class="flex items-center gap-1 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs text-ink-secondary hover:text-ink font-medium transition cursor-pointer shadow-xs"
                :title="t('remotes.auth.diag.title')"
              >
                <KeyRound class="w-3.5 h-3.5 text-warning" />
                <span>{{ t("remotes.auth.diag") }}</span>
              </button>
              <button
                @click="showAddRemoteModal = true"
                class="flex items-center gap-1 px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-xs font-semibold transition cursor-pointer shadow-xs"
              >
                <Plus class="w-3.5 h-3.5" />
                <span>{{ t("remotes.add") }}</span>
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
                    class="px-1.5 py-0.2 rounded bg-surface border border-line font-mono text-[9px] shrink-0"
                    :class="remoteSyncStatus(r.name)?.statusWarning ? 'text-warning cursor-help' : 'text-ink-muted'"
                    :title="remoteSyncStatus(r.name)?.statusWarning || t('remotes.ahead.behind', { branch: remoteSyncStatus(r.name)?.branch || '' })"
                    :aria-label="remoteSyncStatus(r.name)?.statusWarning || undefined"
                  >
                    <template v-if="remoteSyncStatus(r.name)?.statusWarning">!</template>
                    <template v-else>↑{{ remoteSyncStatus(r.name)?.ahead ?? 0 }} ↓{{ remoteSyncStatus(r.name)?.behind ?? 0 }}</template>
                  </span>
                  <span
                    v-if="remoteLatency[r.name]"
                    class="px-1.5 py-0.2 rounded bg-cyan/15 text-cyan font-mono text-[9px] shrink-0"
                    :title="t('remotes.latency.title')"
                  >
                    {{ remoteLatency[r.name] }}
                  </span>
                </div>
                <span class="font-mono text-[11px] text-ink-muted truncate" :title="r.url">{{ r.url }}</span>
              </div>
              <div class="flex items-center gap-1 shrink-0">
                <button :aria-label="t('remotes.action.copy.url')"
                  @click="copyToClipboard(r.url, r.name)"
                  class="p-1.5 rounded-lg hover:bg-surface text-ink-muted hover:text-ink transition cursor-pointer"
                  :title="t('remotes.action.copy.url')"
                >
                  <Copy class="w-4 h-4" />
                </button>
                <button :aria-label="t('remotes.action.fetch.prune')"
                  @click="handleFetchRemote(r.name)"
                  :disabled="!!remoteBusy"
                  class="p-1.5 rounded-lg hover:bg-cyan/20 text-ink-muted hover:text-cyan transition cursor-pointer disabled:opacity-50"
                  :title="t('remotes.action.fetch.prune')"
                >
                  <RefreshCw class="w-4 h-4" :class="{ 'animate-spin': remoteBusy === `fetch:${r.name}` }" />
                </button>
                <button :aria-label="t('remotes.action.prune')"
                  @click="handlePruneRemote(r.name)"
                  :disabled="!!remoteBusy"
                  class="p-1.5 rounded-lg hover:bg-warning/20 text-ink-muted hover:text-warning transition cursor-pointer disabled:opacity-50"
                  :title="t('remotes.action.prune')"
                >
                  <Eraser class="w-4 h-4" />
                </button>
                <button :aria-label="t('remotes.action.open.web')"
                  @click="handleOpenRemoteWeb(r.name, r.url)"
                  class="p-1.5 rounded-lg hover:bg-accent/20 text-ink-muted hover:text-accent transition cursor-pointer"
                  :title="t('remotes.action.open.web')"
                >
                  <ExternalLink class="w-4 h-4" />
                </button>
                <button :aria-label="t('remotes.action.test')"
                  @click="handleTestRemote(r.name)"
                  :disabled="!!remoteBusy"
                  class="p-1.5 rounded-lg hover:bg-cyan/20 text-ink-muted hover:text-cyan transition cursor-pointer disabled:opacity-50"
                  :title="t('remotes.action.test')"
                >
                  <Zap class="w-4 h-4" :class="{ 'animate-pulse': remoteBusy === `test:${r.name}` }" />
                </button>
                <button :aria-label="t('remotes.action.diagnose')"
                  @click="openAuthDiagModal(extractHostFromUrl(r.url))"
                  class="p-1.5 rounded-lg hover:bg-warning/20 text-ink-muted hover:text-warning transition cursor-pointer"
                  :title="t('remotes.action.diagnose')"
                >
                  <KeyRound class="w-4 h-4" />
                </button>
                <button :aria-label="t('remotes.action.edit')"
                  @click="openEditRemote(r)"
                  class="p-1.5 rounded-lg hover:bg-accent/20 text-ink-muted hover:text-accent transition cursor-pointer"
                  :title="t('remotes.action.edit')"
                >
                  <Pencil class="w-4 h-4" />
                </button>
                <button :aria-label="t('remotes.action.remove')"
                  @click="handleRemoveRemote(r.name)"
                  class="p-1.5 rounded-lg hover:bg-danger/20 text-ink-muted hover:text-danger transition cursor-pointer shrink-0"
                  :title="t('remotes.action.remove')"
                >
                  <Trash2 class="w-4 h-4" />
                </button>
              </div>
            </div>

            <div v-if="remotes.length === 0" class="col-span-2 py-6 text-center text-xs text-ink-muted border border-dashed border-line rounded-xl">
              {{ t("remotes.empty") }}
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
              <span>{{ t("remotes.worktree.title") }}</span>
            </h3>
            <p class="text-xs text-ink-muted mt-0.5">
              {{ t("remotes.worktree.desc") }}
            </p>
          </div>

          <div class="flex items-center gap-2">
            <button
              @click="handlePruneWorktrees"
              :disabled="isWorktreeMaintenance"
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-40"
              :title="t('remotes.worktree.prune.title')"
            >
              <Trash2 class="w-3.5 h-3.5 text-warning" />
              <span>{{ t("remotes.worktree.prune") }}</span>
            </button>
            <button
              @click="handleRepairWorktrees"
              :disabled="isWorktreeMaintenance"
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-40"
              :title="t('remotes.worktree.repair.title')"
            >
              <Wrench class="w-3.5 h-3.5 text-accent" />
              <span>{{ t("remotes.worktree.repair") }}</span>
            </button>
            <button
              @click="showAddWorktreeModal = true"
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-purple-strong hover:bg-purple-dark text-white text-xs font-semibold transition cursor-pointer shadow-xs"
            >
              <FolderPlus class="w-3.5 h-3.5" />
              <span>{{ t("remotes.worktree.add") }}</span>
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
              <span class="text-[10px] text-ink-muted block uppercase tracking-wider">{{ t("remotes.worktree.total") }}</span>
              <span class="text-base font-bold text-ink font-mono">{{ t("remotes.worktree.count", { count: worktrees.length }) }}</span>
            </div>
          </div>

          <div class="p-4 rounded-xl border border-line bg-surface/40 flex items-center gap-3">
            <div class="w-10 h-10 rounded-xl bg-accent/15 text-accent flex items-center justify-center shrink-0">
              <GitBranch class="w-5 h-5" />
            </div>
            <div>
              <span class="text-[10px] text-ink-muted block uppercase tracking-wider">{{ t("remotes.worktree.linked") }}</span>
              <span class="text-base font-bold text-ink font-mono">
                {{ t("remotes.worktree.count", { count: worktrees.filter(w => !w.isBare && !w.isMain).length }) }}
              </span>
            </div>
          </div>

          <div class="p-4 rounded-xl border border-line bg-surface/40 flex items-center gap-3">
            <div class="w-10 h-10 rounded-xl bg-warning/15 text-warning flex items-center justify-center shrink-0">
              <Lock class="w-5 h-5" />
            </div>
            <div>
              <span class="text-[10px] text-ink-muted block uppercase tracking-wider">{{ t("remotes.worktree.locked.state") }}</span>
              <span class="text-base font-bold text-ink font-mono">
                {{ t("remotes.worktree.locked.count", { count: worktrees.filter(w => w.isLocked).length }) }}
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
                    {{ t("remotes.worktree.main") }}
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
                    <Lock class="w-2.5 h-2.5" /> {{ w.lockReason || t('remotes.worktree.locked.default') }}
                  </span>
                </div>

                <div class="flex items-center gap-2 mt-1 text-xs font-mono text-ink-muted truncate">
                  <span class="text-ink font-medium truncate" :title="w.path">{{ w.path }}</span>
                  <button :aria-label="t('remotes.worktree.copy.path')"
                    @click="copyToClipboard(w.path, t('remotes.worktree.path.label'))"
                    class="p-0.5 rounded hover:bg-raised text-ink-muted hover:text-ink cursor-pointer shrink-0"
                    :title="t('remotes.worktree.copy.path')"
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
                :title="t('remotes.worktree.open.tab.title')"
              >
                <ExternalLink class="w-3.5 h-3.5" />
                <span>{{ t("remotes.worktree.open.tab") }}</span>
              </button>

              <button :aria-label="t('remotes.worktree.open.terminal')"
                @click="handleOpenWorktreeInTerminal(w.path)"
                class="p-1.5 rounded-lg hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer"
                :title="t('remotes.worktree.open.terminal')"
              >
                <Terminal class="w-4 h-4" />
              </button>

              <button :aria-label="t('remotes.worktree.open.finder')"
                @click="handleOpenWorktreeInFinder(w.path)"
                class="p-1.5 rounded-lg hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer"
                :title="t('remotes.worktree.open.finder')"
              >
                <Folder class="w-4 h-4" />
              </button>

              <button :aria-label="t('remotes.worktree.lock.aria')"
                v-if="!w.isMain && !w.isLocked"
                @click="openLockWorktreeModal(w.path)"
                class="p-1.5 rounded-lg hover:bg-warning/20 text-ink-muted hover:text-warning transition cursor-pointer"
                :title="t('remotes.worktree.lock.aria')"
              >
                <Lock class="w-4 h-4" />
              </button>

              <button :aria-label="t('remotes.worktree.unlock.aria')"
                v-if="!w.isMain && w.isLocked"
                @click="handleUnlockWorktree(w.path)"
                class="p-1.5 rounded-lg hover:bg-success/20 text-warning hover:text-success transition cursor-pointer"
                :title="t('remotes.worktree.unlock.aria')"
              >
                <Unlock class="w-4 h-4" />
              </button>

              <button :aria-label="t('remotes.worktree.remove.aria')"
                v-if="!w.isMain"
                @click="handleRemoveWorktree(w.path, false)"
                class="p-1.5 rounded-lg hover:bg-danger/20 text-ink-muted hover:text-danger transition cursor-pointer"
                :title="t('remotes.worktree.remove.aria')"
              >
                <Trash2 class="w-4 h-4" />
              </button>
            </div>
          </div>

          <div v-if="worktrees.length === 0" class="py-12 text-center text-xs text-ink-muted border border-dashed border-line rounded-xl">
            {{ t("remotes.worktree.empty") }}
          </div>
        </div>
      </div>


    <!-- Modal: Add Remote -->
    <UiModal
      :open="showAddRemoteModal"
      :label="t('remotes.modal.add.label')"
      width="max-w-md"
      @update:open="showAddRemoteModal = $event"
    >
      <template #header>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <Globe class="w-4 h-4 text-accent" />
          <span>{{ t("remotes.modal.add.title") }}</span>
        </h3>
      </template>

      <div class="space-y-3">
        <div class="flex flex-col gap-1.5">
          <label class="text-xs text-ink-secondary font-medium">{{ t("remotes.modal.name") }}</label>
          <input
            v-model="newRemoteName"
            type="text"
            :aria-label="t('remotes.modal.name.aria')"
            placeholder="origin / upstream / backup"
            class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
          />
        </div>
        <div class="flex flex-col gap-1.5">
          <label class="text-xs text-ink-secondary font-medium">{{ t("remotes.modal.url") }}</label>
          <input
            v-model="newRemoteUrl"
            type="text"
            :aria-label="t('remotes.modal.url.aria')"
            placeholder="git@github.com:user/repo.git"
            class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
          />
          <div class="flex items-center gap-2 pt-0.5">
            <span class="text-[10px] text-ink-muted">{{ t("remotes.modal.convert.protocol") }}:</span>
            <button
              type="button"
              @click="newRemoteUrl = convertRemoteUrlProtocol(newRemoteUrl, 'ssh')"
              class="px-2 py-0.5 rounded text-[10px] font-mono border border-line bg-surface hover:bg-raised text-ink cursor-pointer transition"
            >
              SSH (git@...)
            </button>
            <button
              type="button"
              @click="newRemoteUrl = convertRemoteUrlProtocol(newRemoteUrl, 'https')"
              class="px-2 py-0.5 rounded text-[10px] font-mono border border-line bg-surface hover:bg-raised text-ink cursor-pointer transition"
            >
              HTTPS (https://...)
            </button>
          </div>
        </div>
      </div>

      <template #footer>
        <UiButton variant="ghost" @click="showAddRemoteModal = false">{{ t("common.cancel") }}</UiButton>
        <UiButton
          variant="primary"
          :disabled="isAddingRemote || !newRemoteName.trim() || !newRemoteUrl.trim()"
          @click="handleAddRemote"
        >
          {{ isAddingRemote ? t("remotes.modal.adding") : t("remotes.modal.add.confirm") }}
        </UiButton>
      </template>
    </UiModal>

    <!-- Modal: Edit Remote -->
    <UiModal
      :open="showEditRemoteModal"
      :label="t('remotes.modal.edit.label')"
      width="max-w-md"
      @update:open="showEditRemoteModal = $event"
    >
      <template #header>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <Pencil class="w-4 h-4 text-accent" />
          <span>{{ t("remotes.modal.edit.title") }}</span>
        </h3>
      </template>

      <div class="space-y-3">
        <div class="flex flex-col gap-1.5">
          <label class="text-xs text-ink-secondary font-medium">{{ t("remotes.modal.name") }}</label>
          <input
            v-model="editRemoteName"
            type="text"
            :aria-label="t('remotes.modal.name.aria')"
            placeholder="origin / upstream / backup"
            class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
          />
        </div>
        <div class="flex flex-col gap-1.5">
          <label class="text-xs text-ink-secondary font-medium">{{ t("remotes.modal.fetch.url") }}</label>
          <input
            v-model="editRemoteUrl"
            type="text"
            :aria-label="t('remotes.modal.url.aria')"
            placeholder="git@github.com:user/repo.git"
            class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
          />
          <div class="flex items-center gap-2 pt-0.5">
            <span class="text-[10px] text-ink-muted">{{ t("remotes.modal.convert.protocol") }}:</span>
            <button
              type="button"
              @click="editRemoteUrl = convertRemoteUrlProtocol(editRemoteUrl, 'ssh')"
              class="px-2 py-0.5 rounded text-[10px] font-mono border border-line bg-surface hover:bg-raised text-ink cursor-pointer transition"
            >
              SSH (git@...)
            </button>
            <button
              type="button"
              @click="editRemoteUrl = convertRemoteUrlProtocol(editRemoteUrl, 'https')"
              class="px-2 py-0.5 rounded text-[10px] font-mono border border-line bg-surface hover:bg-raised text-ink cursor-pointer transition"
            >
              HTTPS (https://...)
            </button>
          </div>
        </div>
      </div>

      <template #footer>
        <UiButton variant="ghost" @click="showEditRemoteModal = false">{{ t("common.cancel") }}</UiButton>
        <UiButton
          variant="primary"
          :disabled="isSavingRemote || !editRemoteName.trim() || !editRemoteUrl.trim()"
          @click="handleSaveRemoteEdit"
        >
          {{ isSavingRemote ? t("remotes.modal.saving") : t("remotes.modal.save") }}
        </UiButton>
      </template>
    </UiModal>

    <!-- Modal: Add Worktree -->
    <UiModal
      :open="showAddWorktreeModal"
      :label="t('remotes.worktree.modal.add.label')"
      width="max-w-md"
      @update:open="showAddWorktreeModal = $event"
    >
      <template #header>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <FolderPlus class="w-4 h-4 text-purple" />
          <span>{{ t("remotes.worktree.modal.add.title") }}</span>
        </h3>
      </template>

      <div class="space-y-3">
        <div class="flex flex-col gap-1.5">
          <label class="text-xs text-ink-secondary font-medium">{{ t("remotes.worktree.modal.path") }}</label>
          <input
            v-model="newWorktreePath"
            type="text"
            :aria-label="t('remotes.worktree.modal.path.aria')"
            placeholder="../project-worktree-feat"
            class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
          />
        </div>
        <div class="flex flex-col gap-1.5">
          <label class="text-xs text-ink-secondary font-medium">{{ t("remotes.worktree.modal.branch") }}</label>
          <input
            v-model="newWorktreeBranch"
            type="text"
            :aria-label="t('remotes.worktree.modal.branch.aria')"
            placeholder="feature/parallel-work"
            class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
          />
        </div>
        <div class="flex flex-col gap-2 pt-1 border-t border-line/60">
          <label class="flex items-center gap-2 text-xs text-ink cursor-pointer">
            <input v-model="isWorktreeNewBranch" type="checkbox" class="w-4 h-4 rounded accent-[var(--ug-purple-strong)] cursor-pointer" />
            <span>{{ t("remotes.worktree.modal.new.branch") }}</span>
          </label>
          <label class="flex items-center gap-2 text-xs text-ink cursor-pointer">
            <input v-model="openAfterCreateWorktree" type="checkbox" class="w-4 h-4 rounded accent-[var(--ug-accent-strong)] cursor-pointer" />
            <span>{{ t("remotes.worktree.modal.open.tab") }}</span>
          </label>
        </div>
      </div>

      <template #footer>
        <UiButton variant="ghost" @click="showAddWorktreeModal = false">{{ t("common.cancel") }}</UiButton>
        <UiButton
          variant="purple"
          :disabled="isAddingWorktree || !newWorktreePath.trim()"
          @click="handleAddWorktree"
        >
          {{ isAddingWorktree ? t("remotes.worktree.modal.creating") : t("remotes.worktree.modal.confirm") }}
        </UiButton>
      </template>
    </UiModal>

    <!-- Modal: Lock Worktree -->
    <UiModal
      :open="showLockWorktreeModal"
      :label="t('remotes.worktree.modal.lock.label')"
      width="max-w-md"
      @update:open="showLockWorktreeModal = $event"
    >
      <template #header>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <Lock class="w-4 h-4 text-warning" />
          <span>{{ t("remotes.worktree.modal.lock.title") }}</span>
        </h3>
      </template>

      <div class="space-y-3">
        <div class="flex flex-col gap-1 text-xs">
          <span class="text-ink-secondary">{{ t("remotes.worktree.modal.lock.target") }}</span>
          <span class="p-2 rounded-lg bg-canvas border border-line font-mono text-ink text-[11px] truncate">
            {{ targetLockWorktreePath }}
          </span>
        </div>
        <div class="flex flex-col gap-1.5">
          <label class="text-xs text-ink-secondary font-medium">{{ t("remotes.worktree.modal.lock.reason") }}</label>
          <input
            v-model="lockWorktreeReason"
            type="text"
            :aria-label="t('remotes.worktree.modal.lock.reason.aria')"
            :placeholder="t('remotes.worktree.modal.lock.reason.placeholder')"
            @keydown.enter="handleLockWorktree"
            class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink focus:outline-none focus:border-accent"
          />
        </div>
      </div>

      <template #footer>
        <UiButton variant="ghost" @click="showLockWorktreeModal = false">{{ t("common.cancel") }}</UiButton>
        <UiButton variant="warning" :disabled="isWorktreeOperation" @click="handleLockWorktree">
          {{ isWorktreeOperation ? t("remotes.worktree.modal.locking") : t("remotes.worktree.modal.lock.confirm") }}
        </UiButton>
      </template>
    </UiModal>

    <!-- Modal: SSH & Git Credentials Diagnosis -->
    <UiModal
      :open="showAuthDiagModal"
      :label="t('remotes.auth.modal.title')"
      width="max-w-lg"
      @update:open="showAuthDiagModal = $event"
    >
      <template #header>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <KeyRound class="w-4 h-4 text-warning" />
          <span>{{ t("remotes.auth.modal.title") }}</span>
        </h3>
      </template>

      <div class="space-y-4">
        <div class="flex items-center gap-2">
          <input
            v-model="authDiagHost"
            type="text"
            placeholder="github.com / gitlab.com"
            @keydown.enter="handleRunAuthDiag(authDiagHost)"
            class="flex-1 px-3 py-1.5 rounded-lg bg-canvas border border-line text-xs font-mono text-ink focus:outline-none focus:border-accent"
          />
          <UiButton
            variant="primary"
            :disabled="isRunningAuthDiag || !authDiagHost.trim()"
            @click="handleRunAuthDiag(authDiagHost)"
          >
            <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isRunningAuthDiag }" />
            <span>{{ isRunningAuthDiag ? t("remotes.auth.testing") : t("remotes.auth.test") }}</span>
          </UiButton>
        </div>

        <!-- Diagnosis Results -->
        <div v-if="authDiagResult" class="space-y-3">
          <div class="flex items-center justify-between p-2.5 rounded-lg bg-surface/50 border border-line">
            <div class="flex items-center gap-2">
              <span
                class="px-2 py-0.5 rounded text-[10px] font-bold font-mono"
                :class="authDiagResult.sshAuthSuccess ? 'bg-success/15 text-success border border-success/30' : 'bg-warning/15 text-warning border border-warning/30'"
              >
                {{ authDiagResult.sshAuthSuccess ? t("remotes.auth.status.ok") : t("remotes.auth.status.warn") }}
              </span>
              <span class="text-xs font-mono font-medium text-ink">{{ authDiagResult.host }}</span>
            </div>
          </div>

          <div class="grid grid-cols-2 gap-2 text-xs">
            <div class="p-2.5 rounded-lg bg-surface/30 border border-line">
              <span class="text-[10px] text-ink-muted block uppercase tracking-wider">{{ t("remotes.auth.keys.found") }}</span>
              <span class="font-bold text-ink font-mono">{{ authDiagResult.sshKeysFound?.length || 0 }}</span>
            </div>
            <div class="p-2.5 rounded-lg bg-surface/30 border border-line">
              <span class="text-[10px] text-ink-muted block uppercase tracking-wider">Credential Helper</span>
              <span class="font-mono text-ink truncate block text-[11px]">{{ authDiagResult.credentialHelper || t("remotes.auth.not.configured") }}</span>
            </div>
          </div>

          <div v-if="(authDiagResult.sshKeysFound || []).length > 0" class="flex flex-wrap gap-1">
            <span
              v-for="k in authDiagResult.sshKeysFound"
              :key="k"
              class="px-2 py-0.5 rounded bg-surface border border-line font-mono text-[10px] text-ink"
            >
              {{ k }}
            </span>
          </div>

          <div v-if="authDiagResult.sshAuthOutput" class="space-y-1">
            <span class="text-[10px] text-ink-muted block uppercase tracking-wider">{{ t("remotes.auth.output") }}</span>
            <pre class="p-2.5 rounded-lg bg-canvas border border-line text-[10px] text-ink-muted font-mono whitespace-pre-wrap max-h-36 overflow-y-auto">{{ authDiagResult.sshAuthOutput }}</pre>
          </div>
        </div>
      </div>

      <template #footer>
        <UiButton variant="ghost" @click="showAuthDiagModal = false">{{ t("common.close") }}</UiButton>
      </template>
    </UiModal>

</template>
