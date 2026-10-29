<script setup lang="ts">
import { t } from "../../i18n";
import { confirmAction } from "../../composables/useConfirm";
import { ref, watch, onMounted, watchEffect } from "vue";
import {
  Trash2,
  GitBranch,
  FolderGit2,
  Lock,
  Unlock,
  FolderPlus,
  Wrench,
  Folder,
  Copy,
  ExternalLink,
  Terminal,
  AlertTriangle,
  Eraser,
  Move,
  ArrowRightLeft,
} from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import { mapWithConcurrency } from "../../utils/concurrency";
import UiModal from "../ui/UiModal.vue";
import UiButton from "../ui/UiButton.vue";
import UiEmpty from "../ui/UiEmpty.vue";
import { toolsStatus } from "../../composables/toolsStatus";
import { useRovingList } from "../../composables/useRovingList";
import type { WorktreeInfo, BranchInfo, AddWorktreeAdvancedOptions } from "../../types";

const props = defineProps<{
  currentRepoPath: string;
  refreshNonce?: number;
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
  notify(t("worktrees.toast.copied", { label }));
}

function normalizePath(p: string): string {
  if (!p) return "";
  let clean = p.trim().replace(/\\/g, "/");
  while (clean.length > 1 && clean.endsWith("/")) {
    clean = clean.slice(0, -1);
  }
  return clean;
}

function isCurrentActive(path: string): boolean {
  if (!props.currentRepoPath || !path) return false;
  return normalizePath(path) === normalizePath(props.currentRepoPath);
}

// -------------------------------------------------------------
// WORKTREES STATE & LIFECYCLE
// -------------------------------------------------------------
const worktrees = ref<WorktreeInfo[]>([]);

// Roving tabindex so ↑/↓ walks worktree cards and Tab enters the list once.
const {
  setItemRef: setWorktreeRowRef,
  tabindexFor: worktreeRowTabindex,
  onKeydown: onWorktreeRowKeydown,
  onFocus: onWorktreeRowFocus,
} = useRovingList(() => worktrees.value, (w) => w.path);
const branches = ref<BranchInfo[]>([]);
const isLoading = ref(false);
const isMaintenance = ref(false);
const isWorktreeOperation = ref(false);
const worktreeUsageMap = ref<Map<string, string>>(new Map());
const branchConflictWarning = ref("");

async function loadDiskUsage() {
  if (!props.currentRepoPath) return;
  // Four at a time: every entry costs a directory walk on the backend, so this
  // is the one serial loop in the app that is safe to parallelise (no git state
  // is written). Results land as they arrive rather than after the whole batch,
  // so the list fills in progressively instead of showing blanks until the last
  // worktree finishes.
  await mapWithConcurrency(
    worktrees.value,
    4,
    (w) => gitService.getWorktreeDiskUsage(props.currentRepoPath, w.path),
    (u, w) => {
      if (u?.formattedSize) {
        worktreeUsageMap.value.set(w.path, `${u.formattedSize} (${u.fileCount} files)`);
      }
    }
  );
}

async function loadWorktrees() {
  if (!props.currentRepoPath) return;
  isLoading.value = true;
  try {
    const [wRes, bRes] = await Promise.allSettled([
      gitService.getWorktrees(props.currentRepoPath),
      gitService.getBranches(props.currentRepoPath),
    ]);
    if (wRes.status === "fulfilled") {
      worktrees.value = wRes.value || [];
      void loadDiskUsage();
    } else {
      notify(t("worktrees.toast.load.failed", { error: String((wRes.reason as any)?.message || wRes.reason) }));
    }
    if (bRes.status === "fulfilled") {
      branches.value = (bRes.value || []).filter((b) => !b.isRemote);
    }
  } finally {
    isLoading.value = false;
  }
}

// -------------------------------------------------------------
// WORKTREE ACTIONS: PRUNE & REPAIR
// -------------------------------------------------------------
async function handlePruneWorktrees() {
  if (!props.currentRepoPath || isMaintenance.value) return;
  if (!(await confirmAction({ message: t("worktrees.confirm.prune.stale"), tone: "warning" }))) {
    return;
  }
  isMaintenance.value = true;
  try {
    const res = await gitService.pruneWorktrees(props.currentRepoPath);
    notify(res.message);
    await loadWorktrees();
    emit("refresh");
  } catch (err: any) {
    notify(t("worktrees.toast.prune.stale.failed", { error: err?.message || err }));
  } finally {
    isMaintenance.value = false;
  }
}

async function handleRepairWorktrees() {
  if (!props.currentRepoPath || isMaintenance.value) return;
  isMaintenance.value = true;
  try {
    const res = await gitService.repairWorktrees(props.currentRepoPath, []);
    notify(res.message);
    await loadWorktrees();
    emit("refresh");
  } catch (err: any) {
    notify(t("worktrees.toast.repair.failed", { error: err?.message || err }));
  } finally {
    isMaintenance.value = false;
  }
}

// -------------------------------------------------------------
// WORKTREE ACTIONS: ADD (ADVANCED + BROWSE FOLDER)
// -------------------------------------------------------------
const showAddModal = ref(false);
const newWorktreePath = ref("");
const addMode = ref<"existing" | "new" | "detach">("existing");
const newWorktreeBranch = ref("");
const newWorktreeBaseRef = ref("");
const openAfterCreate = ref(false);
const isAdding = ref(false);

watch(newWorktreeBranch, async (b) => {
  const branch = b.trim();
  if (!branch || !props.currentRepoPath) {
    branchConflictWarning.value = "";
    return;
  }
  try {
    const val = await gitService.validateWorktreeBranch(props.currentRepoPath, branch);
    if (!val.available) {
      branchConflictWarning.value = val.message;
    } else {
      branchConflictWarning.value = "";
    }
  } catch {
    branchConflictWarning.value = "";
  }
});

const showMoveModal = ref(false);
const moveSourcePath = ref("");
const moveTargetPath = ref("");
const isMovingWorktree = ref(false);

function openAddModal() {
  newWorktreePath.value = "";
  branchConflictWarning.value = "";
  addMode.value = "existing";
  newWorktreeBranch.value = branches.value[0]?.name || "";
  newWorktreeBaseRef.value = "";
  openAfterCreate.value = false;
  showAddModal.value = true;
}

async function handleBrowseWorktreePath() {
  try {
    const folder = await gitService.pickFolderDialog();
    if (folder) {
      newWorktreePath.value = folder;
    }
  } catch (err: any) {
    notify(t("worktrees.toast.pick.folder.failed", { error: err?.message || err }));
  }
}

async function handleAddWorktreeAdvanced() {
  if (!props.currentRepoPath || !newWorktreePath.value.trim() || isAdding.value) return;
  const targetPath = newWorktreePath.value.trim();
  isAdding.value = true;
  try {
    const opts: AddWorktreeAdvancedOptions = {
      path: targetPath,
    };
    if (addMode.value === "existing") {
      opts.branch = newWorktreeBranch.value.trim();
      opts.newBranch = false;
    } else if (addMode.value === "new") {
      opts.branch = newWorktreeBranch.value.trim();
      opts.newBranch = true;
      if (newWorktreeBaseRef.value.trim()) {
        opts.baseRef = newWorktreeBaseRef.value.trim();
      }
    } else if (addMode.value === "detach") {
      opts.detach = true;
      if (newWorktreeBaseRef.value.trim()) {
        opts.baseRef = newWorktreeBaseRef.value.trim();
      }
    }
    await gitService.addWorktreeAdvanced(props.currentRepoPath, opts);
    notify(t("worktrees.toast.added", { name: targetPath.split("/").pop() ?? targetPath }));
    showAddModal.value = false;
    newWorktreePath.value = "";
    newWorktreeBranch.value = "";
    newWorktreeBaseRef.value = "";
    await loadWorktrees();
    if (openAfterCreate.value) {
      emit("open-repo", targetPath);
    }
  } catch (err: any) {
    notify(t("worktrees.toast.add.failed", { error: err?.message || err }));
  } finally {
    isAdding.value = false;
  }
}

// -------------------------------------------------------------
// WORKTREE ACTIONS: MOVE
// -------------------------------------------------------------
function openMoveWorktreeModal(path: string) {
  moveSourcePath.value = path;
  moveTargetPath.value = "";
  showMoveModal.value = true;
}

async function handleMoveWorktree() {
  if (!props.currentRepoPath || isMovingWorktree.value) return;
  const source = moveSourcePath.value.trim();
  const target = moveTargetPath.value.trim();
  if (!source || !target) {
    notify(t("worktrees.toast.move.path.required"));
    return;
  }
  if (normalizePath(source) === normalizePath(target)) {
    notify(t("worktrees.toast.move.same"));
    return;
  }
  const ok = await confirmAction({
    message: t("worktrees.confirm.move", { path: source, newPath: target }),
    tone: "danger",
  });
  if (!ok) return;
  isMovingWorktree.value = true;
  try {
    await gitService.moveWorktree(props.currentRepoPath, source, target);
    notify(t("worktrees.toast.moved"));
    showMoveModal.value = false;
    await loadWorktrees();
    emit("refresh");
  } catch (err: any) {
    notify(t("worktrees.toast.move.failed", { error: err?.message || err }));
  } finally {
    isMovingWorktree.value = false;
  }
}

// -------------------------------------------------------------
// WORKTREE ACTIONS: REMOVE (DIRTY PRECHECK & ACTIVE PROTECTION)
// -------------------------------------------------------------
async function handleRemoveWorktree(path: string, force = false) {
  if (!props.currentRepoPath) return;
  if (isCurrentActive(path)) {
    notify(t("worktrees.toast.remove.active.forbidden"));
    return;
  }

  try {
    if (!force) {
      let isDirty = false;
      let dirtyCount = 0;
      try {
        const status = await gitService.getWorktreeStatus(props.currentRepoPath, path);
        if (status && status.isDirty && status.dirtyCount > 0) {
          isDirty = true;
          dirtyCount = status.dirtyCount;
        }
      } catch {
        // Continue to confirmation if status check throws
      }

      let confirmed = false;
      if (isDirty) {
        confirmed = await confirmAction({
          message: t("worktrees.confirm.remove.dirty", { path, count: dirtyCount }),
          tone: "danger",
        });
      } else {
        confirmed = await confirmAction({
          message: t("worktrees.confirm.remove", { path }),
          tone: "danger",
        });
      }
      if (!confirmed) return;
    }

    await gitService.removeWorktree(props.currentRepoPath, path, force);
    notify(t("worktrees.toast.removed"));
    await loadWorktrees();
    emit("refresh");
  } catch (err: any) {
    if (!force && (await confirmAction({
      message: t("worktrees.confirm.remove.force", { error: err?.message || err }),
      tone: "danger",
    }))) {
      await handleRemoveWorktree(path, true);
    } else {
      notify(t("worktrees.toast.remove.failed", { error: err?.message || err }));
    }
  }
}

// -------------------------------------------------------------
// WORKTREE ACTIONS: SINGLE PRUNE
// -------------------------------------------------------------
async function handlePruneSingle(path: string) {
  if (!props.currentRepoPath || isMaintenance.value) return;
  if (isCurrentActive(path)) {
    notify(t("worktrees.toast.remove.active.forbidden"));
    return;
  }
  if (!(await confirmAction({ message: t("worktrees.confirm.prune.single", { path }), tone: "warning" }))) {
    return;
  }
  isMaintenance.value = true;
  try {
    await gitService.pruneSingleWorktree(props.currentRepoPath, path);
    notify(t("worktrees.toast.prune.single.success", { path }));
    await loadWorktrees();
    emit("refresh");
  } catch (err: any) {
    notify(t("worktrees.toast.prune.single.failed", { error: err?.message || err }));
  } finally {
    isMaintenance.value = false;
  }
}

// -------------------------------------------------------------
// WORKTREE ACTIONS: LOCK & UNLOCK
// -------------------------------------------------------------
const showLockModal = ref(false);
const targetLockPath = ref("");
const lockReason = ref("");

function openLockWorktreeModal(path: string) {
  targetLockPath.value = path;
  lockReason.value = "";
  showLockModal.value = true;
}

async function handleLockWorktree() {
  if (!props.currentRepoPath || !targetLockPath.value) return;
  isWorktreeOperation.value = true;
  try {
    await gitService.lockWorktree(props.currentRepoPath, targetLockPath.value, lockReason.value.trim());
    notify(t("worktrees.toast.locked"));
    showLockModal.value = false;
    await loadWorktrees();
  } catch (err: any) {
    notify(t("worktrees.toast.lock.failed", { error: err?.message || err }));
  } finally {
    isWorktreeOperation.value = false;
  }
}

async function handleUnlockWorktree(path: string) {
  if (!props.currentRepoPath) return;
  isWorktreeOperation.value = true;
  try {
    await gitService.unlockWorktree(props.currentRepoPath, path);
    notify(t("worktrees.toast.unlocked"));
    await loadWorktrees();
  } catch (err: any) {
    notify(t("worktrees.toast.unlock.failed", { error: err?.message || err }));
  } finally {
    isWorktreeOperation.value = false;
  }
}

// -------------------------------------------------------------
// NAVIGATION & EXTERNAL INTEGRATIONS
// -------------------------------------------------------------
function handleOpenWorktreeAsTab(path: string) {
  emit("open-repo", path);
}

function handleSwitchWorktree(path: string) {
  if (!path || isCurrentActive(path)) return;
  window.dispatchEvent(new CustomEvent("wgit:open-repo-path", { detail: path }));
  emit("open-repo", path);
  notify(t("worktrees.switch.repo.success", { path }));
}

async function handleOpenWorktreeInTerminal(path: string) {
  try {
    await gitService.openInTerminal(path);
  } catch (err: any) {
    notify(t("worktrees.toast.terminal.failed", { error: err?.message || err }));
  }
}

async function handleOpenWorktreeInFinder(path: string) {
  try {
    await gitService.openInFinder(path);
  } catch (err: any) {
    notify(t("worktrees.toast.finder.failed", { error: err?.message || err }));
  }
}

// -------------------------------------------------------------
// TOOLS STATUS & WATCHERS
// -------------------------------------------------------------
watchEffect(() => {
  toolsStatus.loading.worktrees = isLoading.value || isMaintenance.value || isWorktreeOperation.value;
});

watchEffect(() => {
  toolsStatus.counts.worktrees = worktrees.value.length;
});

onMounted(() => {
  void loadWorktrees();
});

watch(
  () => props.refreshNonce,
  () => {
    void loadWorktrees();
  }
);

watch(
  () => props.currentRepoPath,
  () => {
    void loadWorktrees();
  }
);
</script>

<template>
  <div class="space-y-6" data-tool-section="worktrees">
    <!-- Header: Title & Global Actions -->
    <div class="flex items-center justify-between pb-3 border-b border-line">
      <div>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <FolderGit2 class="w-4 h-4 text-purple" />
          <span>{{ t("worktrees.title") }}</span>
        </h3>
        <p class="text-xs text-ink-muted mt-0.5">
          {{ t("worktrees.desc") }}
        </p>
      </div>

      <div class="flex items-center gap-2">
        <button
          @click="handlePruneWorktrees"
          :disabled="isMaintenance"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-40"
          :title="t('worktrees.prune.title')"
        >
          <Trash2 class="w-3.5 h-3.5 text-warning" />
          <span>{{ t("worktrees.prune") }}</span>
        </button>
        <button
          @click="handleRepairWorktrees"
          :disabled="isMaintenance"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-40"
          :title="t('worktrees.repair.title')"
        >
          <Wrench class="w-3.5 h-3.5 text-accent" />
          <span>{{ t("worktrees.repair") }}</span>
        </button>
        <button
          @click="openAddModal"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-purple-strong hover:bg-purple-dark text-white text-xs font-semibold transition cursor-pointer shadow-xs"
        >
          <FolderPlus class="w-3.5 h-3.5" />
          <span>{{ t("worktrees.add") }}</span>
        </button>
      </div>
    </div>

    <!-- Metrics Cards -->
    <div class="grid grid-cols-4 gap-4">
      <div class="p-4 rounded-xl border border-line bg-surface/40 flex items-center gap-3">
        <div class="w-10 h-10 rounded-xl bg-purple/15 text-purple flex items-center justify-center shrink-0">
          <FolderGit2 class="w-5 h-5" />
        </div>
        <div>
          <span class="text-[10px] text-ink-muted block uppercase tracking-wider">{{ t("worktrees.total") }}</span>
          <span class="text-base font-bold text-ink font-mono">{{ t("worktrees.count", { count: worktrees.length }) }}</span>
        </div>
      </div>

      <div class="p-4 rounded-xl border border-line bg-surface/40 flex items-center gap-3">
        <div class="w-10 h-10 rounded-xl bg-accent/15 text-accent flex items-center justify-center shrink-0">
          <GitBranch class="w-5 h-5" />
        </div>
        <div>
          <span class="text-[10px] text-ink-muted block uppercase tracking-wider">{{ t("worktrees.linked") }}</span>
          <span class="text-base font-bold text-ink font-mono">
            {{ t("worktrees.count", { count: worktrees.filter(w => !w.isBare && !w.isMain).length }) }}
          </span>
        </div>
      </div>

      <div class="p-4 rounded-xl border border-line bg-surface/40 flex items-center gap-3">
        <div class="w-10 h-10 rounded-xl bg-warning/15 text-warning flex items-center justify-center shrink-0">
          <Lock class="w-5 h-5" />
        </div>
        <div>
          <span class="text-[10px] text-ink-muted block uppercase tracking-wider">{{ t("worktrees.locked.state") }}</span>
          <span class="text-base font-bold text-ink font-mono">
            {{ t("worktrees.locked.count", { count: worktrees.filter(w => w.isLocked).length }) }}
          </span>
        </div>
      </div>

      <div class="p-4 rounded-xl border border-line bg-surface/40 flex items-center gap-3">
        <div class="w-10 h-10 rounded-xl bg-danger/15 text-danger flex items-center justify-center shrink-0">
          <AlertTriangle class="w-5 h-5" />
        </div>
        <div>
          <span class="text-[10px] text-ink-muted block uppercase tracking-wider">{{ t("worktrees.status.prunable") }}</span>
          <span class="text-base font-bold text-ink font-mono">
            {{ t("worktrees.prunable.count", { count: worktrees.filter(w => w.isPrunable).length }) }}
          </span>
        </div>
      </div>
    </div>

    <!-- Worktrees List -->
    <div class="space-y-3">
      <div
        v-for="(w, index) in worktrees"
        :key="w.path"
        :ref="(el) => setWorktreeRowRef(w, el)"
        :tabindex="worktreeRowTabindex(w, index)"
        class="p-4 rounded-xl border border-line bg-surface/40 flex items-center justify-between gap-4 shadow-xs hover:border-line-strong transition focus:outline-none focus-visible:ring-2 focus-visible:ring-accent/45"
        :class="{ 'ring-1 ring-emerald-500/30 border-emerald-500/40': isCurrentActive(w.path) }"
        @focus="onWorktreeRowFocus(w)"
        @keydown="onWorktreeRowKeydown($event, index)"
      >
        <div class="flex items-center gap-3 min-w-0 flex-1">
          <div
            class="w-9 h-9 rounded-xl flex items-center justify-center shrink-0 font-bold text-xs"
            :class="isCurrentActive(w.path) ? 'bg-emerald-500/15 text-emerald-500' : (w.isMain ? 'bg-accent/15 text-accent' : 'bg-purple/15 text-purple')"
          >
            <FolderGit2 class="w-4 h-4" />
          </div>

          <div class="flex flex-col min-w-0 flex-1">
            <div class="flex items-center gap-2 flex-wrap">
              <!-- Current Active Badge -->
              <span
                v-if="isCurrentActive(w.path)"
                class="px-2 py-0.5 rounded text-[10px] font-bold bg-emerald-500/15 text-emerald-500 border border-emerald-500/20 flex items-center gap-1"
              >
                {{ t("worktrees.status.active") }}
              </span>

              <!-- Branch / Detached tag -->
              <span class="px-2 py-0.5 rounded font-mono font-bold text-xs shrink-0 bg-purple/20 text-purple">
                {{ w.branch || "HEAD (Detached)" }}
              </span>

              <!-- Main vs Linked Worktree -->
              <span
                v-if="w.isMain"
                class="px-2 py-0.2 rounded text-[10px] font-bold bg-accent/15 text-accent border border-accent/20"
              >
                {{ t("worktrees.main") }}
              </span>
              <span
                v-else
                class="px-2 py-0.2 rounded text-[10px] font-medium bg-line/60 text-ink-muted"
              >
                Linked Worktree
              </span>

              <!-- Lock indicator -->
              <span
                v-if="w.isLocked"
                class="px-2 py-0.2 rounded text-[10px] font-bold bg-warning/15 text-warning border border-warning/20 flex items-center gap-1"
              >
                <Lock class="w-2.5 h-2.5" /> {{ w.lockReason || t('worktrees.locked.default') }}
              </span>

              <!-- Prunable stale warning indicator -->
              <span
                v-if="w.isPrunable"
                class="px-2 py-0.2 rounded text-[10px] font-bold bg-danger/15 text-danger border border-danger/20 flex items-center gap-1"
                :title="w.pruneReason || t('worktrees.prunable.desc')"
              >
                <AlertTriangle class="w-2.5 h-2.5" /> {{ t("worktrees.status.prunable") }}
              </span>
            </div>

            <!-- Path & Commit row -->
            <div class="flex items-center gap-2 mt-1 text-xs font-mono text-ink-muted truncate">
              <span class="text-ink font-medium truncate" :title="w.path">{{ w.path }}</span>
              <button
                :aria-label="t('worktrees.copy.path')"
                @click="copyToClipboard(w.path, t('worktrees.path.label'))"
                class="p-0.5 rounded hover:bg-raised text-ink-muted hover:text-ink cursor-pointer shrink-0"
                :title="t('worktrees.copy.path')"
              >
                <Copy class="w-3 h-3" />
              </button>
              <span class="shrink-0">• Commit: <strong class="text-accent">{{ w.head ? w.head.substring(0, 8) : 'N/A' }}</strong></span>
              <span v-if="worktreeUsageMap.get(w.path)" class="shrink-0 text-ink-muted">
                • {{ worktreeUsageMap.get(w.path) }}
              </span>
              <span
                v-if="w.dirtyCount && w.dirtyCount > 0"
                class="text-warning font-medium shrink-0"
              >
                ({{ t("worktrees.dirty.count", { count: w.dirtyCount }) }})
              </span>
            </div>
          </div>
        </div>

        <!-- Actions -->
        <div class="flex items-center gap-2 shrink-0">
          <!-- Open in Workspace: available for ALL worktrees that are not currently active -->
          <button
            v-if="!isCurrentActive(w.path)"
            @click="handleOpenWorktreeAsTab(w.path)"
            class="flex items-center gap-1 px-3 py-1.5 rounded-lg bg-accent/15 hover:bg-accent/25 text-accent text-xs font-semibold transition cursor-pointer border border-accent/30"
            :title="t('worktrees.open.tab.title')"
          >
            <ExternalLink class="w-3.5 h-3.5" />
            <span>{{ t("worktrees.open.tab") }}</span>
          </button>

          <!-- Open in Terminal -->
          <button
            :aria-label="t('worktrees.open.terminal')"
            @click="handleOpenWorktreeInTerminal(w.path)"
            class="p-1.5 rounded-lg hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer"
            :title="t('worktrees.open.terminal')"
          >
            <Terminal class="w-4 h-4" />
          </button>

          <!-- Open in Finder -->
          <button
            :aria-label="t('worktrees.open.finder')"
            @click="handleOpenWorktreeInFinder(w.path)"
            class="p-1.5 rounded-lg hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer"
            :title="t('worktrees.open.finder')"
          >
            <Folder class="w-4 h-4" />
          </button>

          <!-- Switch Active Repo to this Worktree -->
          <button
            v-if="!isCurrentActive(w.path)"
            :aria-label="t('worktrees.switch.repo.title')"
            @click="handleSwitchWorktree(w.path)"
            class="flex items-center gap-1 px-2 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-accent transition cursor-pointer"
            :title="t('worktrees.switch.repo.hint')"
            data-testid="switch-worktree-btn"
          >
            <ArrowRightLeft class="w-3.5 h-3.5 text-accent" />
            <span class="hidden sm:inline">{{ t("worktrees.switch.repo.btn") }}</span>
          </button>

          <!-- Single Prune button for stale worktrees -->
          <button
            v-if="w.isPrunable"
            :aria-label="t('worktrees.prune.single.title')"
            @click="handlePruneSingle(w.path)"
            class="p-1.5 rounded-lg hover:bg-warning/20 text-ink-muted hover:text-warning transition cursor-pointer"
            :title="t('worktrees.prune.single.title')"
          >
            <Eraser class="w-4 h-4" />
          </button>

          <!-- Move Action: hidden for main, locked or active worktrees -->
          <button
            v-if="!w.isMain && !w.isLocked && !isCurrentActive(w.path)"
            :aria-label="t('worktrees.move.aria')"
            @click="openMoveWorktreeModal(w.path)"
            class="p-1.5 rounded-lg hover:bg-accent/15 text-ink-muted hover:text-accent transition cursor-pointer"
            :title="t('worktrees.move.aria')"
          >
            <Move class="w-4 h-4" />
          </button>

          <!-- Lock Action -->
          <button
            v-if="!w.isMain && !w.isLocked"
            :aria-label="t('worktrees.lock.aria')"
            @click="openLockWorktreeModal(w.path)"
            class="p-1.5 rounded-lg hover:bg-warning/20 text-ink-muted hover:text-warning transition cursor-pointer"
            :title="t('worktrees.lock.aria')"
          >
            <Lock class="w-4 h-4" />
          </button>

          <!-- Unlock Action -->
          <button
            v-if="!w.isMain && w.isLocked"
            :aria-label="t('worktrees.unlock.aria')"
            @click="handleUnlockWorktree(w.path)"
            class="p-1.5 rounded-lg hover:bg-success/20 text-warning hover:text-success transition cursor-pointer"
            :title="t('worktrees.unlock.aria')"
          >
            <Unlock class="w-4 h-4" />
          </button>

          <!-- Remove Action with safety: disabled when active -->
          <template v-if="!w.isMain">
            <button
              v-if="isCurrentActive(w.path)"
              disabled
              :aria-label="t('worktrees.cannot.remove.active')"
              :title="t('worktrees.cannot.remove.active')"
              class="p-1.5 rounded-lg text-ink-muted opacity-30 cursor-not-allowed"
            >
              <Trash2 class="w-4 h-4" />
            </button>
            <button
              v-else
              :aria-label="t('worktrees.remove.aria')"
              @click="handleRemoveWorktree(w.path, false)"
              class="p-1.5 rounded-lg hover:bg-danger/20 text-ink-muted hover:text-danger transition cursor-pointer"
              :title="t('worktrees.remove.aria')"
            >
              <Trash2 class="w-4 h-4" />
            </button>
          </template>
        </div>
      </div>

      <UiEmpty
        v-if="worktrees.length === 0"
        :icon="FolderGit2"
        :title="t('worktrees.empty')"
        bordered
      />
    </div>

    <!-- Modal: Add Worktree (Advanced) -->
    <UiModal
      :open="showAddModal"
      :label="t('worktrees.modal.add.label')"
      width="max-w-md"
      @update:open="showAddModal = $event"
    >
      <template #header>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <FolderPlus class="w-4 h-4 text-purple" />
          <span>{{ t("worktrees.modal.add.title") }}</span>
        </h3>
      </template>

      <div class="space-y-4">
        <!-- Path Input & Browse Button -->
        <div class="flex flex-col gap-1.5">
          <label class="text-xs text-ink-secondary font-medium">{{ t("worktrees.modal.path") }}</label>
          <div class="flex items-center gap-2">
            <input
              v-model="newWorktreePath"
              type="text"
              :aria-label="t('worktrees.modal.path.aria')"
              :placeholder="t('worktrees.modal.path.placeholder')"
              class="flex-1 px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
            />
            <UiButton
              variant="ghost"
              class="shrink-0 flex items-center gap-1 border border-line text-xs"
              @click="handleBrowseWorktreePath"
            >
              <Folder class="w-3.5 h-3.5 text-accent" />
              <span>{{ t("worktrees.modal.browse") }}</span>
            </UiButton>
          </div>
        </div>

        <!-- Mode Selector -->
        <div class="flex flex-col gap-1.5">
          <label class="text-xs text-ink-secondary font-medium">{{ t("worktrees.modal.mode.label") }}</label>
          <div class="grid grid-cols-3 gap-1.5 p-1 bg-canvas border border-line rounded-lg">
            <button
              type="button"
              @click="addMode = 'existing'"
              class="px-2 py-1 text-xs font-semibold rounded-md transition cursor-pointer"
              :class="addMode === 'existing' ? 'bg-raised text-accent shadow-xs' : 'text-ink-muted hover:text-ink'"
            >
              {{ t("worktrees.modal.mode.existing") }}
            </button>
            <button
              type="button"
              @click="addMode = 'new'"
              class="px-2 py-1 text-xs font-semibold rounded-md transition cursor-pointer"
              :class="addMode === 'new' ? 'bg-raised text-accent shadow-xs' : 'text-ink-muted hover:text-ink'"
            >
              {{ t("worktrees.modal.mode.new") }}
            </button>
            <button
              type="button"
              @click="addMode = 'detach'"
              class="px-2 py-1 text-xs font-semibold rounded-md transition cursor-pointer"
              :class="addMode === 'detach' ? 'bg-raised text-accent shadow-xs' : 'text-ink-muted hover:text-ink'"
            >
              {{ t("worktrees.modal.mode.detach") }}
            </button>
          </div>
        </div>

        <!-- Mode: Existing Branch -->
        <div v-if="addMode === 'existing'" class="flex flex-col gap-1.5">
          <label class="text-xs text-ink-secondary font-medium">{{ t("worktrees.modal.branch.select") }}</label>
          <select
            v-model="newWorktreeBranch"
            class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
          >
            <option value="" disabled>{{ t("worktrees.modal.branch.select.placeholder") }}</option>
            <option v-for="b in branches" :key="b.name" :value="b.name">{{ b.name }}</option>
          </select>
        </div>

        <!-- Mode: New Branch -->
        <template v-else-if="addMode === 'new'">
          <div class="flex flex-col gap-1.5">
            <label class="text-xs text-ink-secondary font-medium">{{ t("worktrees.modal.branch.name") }}</label>
            <input
              v-model="newWorktreeBranch"
              type="text"
              :placeholder="t('worktrees.modal.branch.name.placeholder')"
              class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
            />
          </div>
          <div class="flex flex-col gap-1.5">
            <label class="text-xs text-ink-secondary font-medium">{{ t("worktrees.modal.base.ref") }}</label>
            <input
              v-model="newWorktreeBaseRef"
              type="text"
              :placeholder="t('worktrees.modal.base.ref.placeholder')"
              class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
            />
          </div>
        </template>

        <!-- Mode: Detached HEAD -->
        <template v-else-if="addMode === 'detach'">
          <div class="flex flex-col gap-1.5">
            <label class="text-xs text-ink-secondary font-medium">{{ t("worktrees.modal.base.ref") }}</label>
            <input
              v-model="newWorktreeBaseRef"
              type="text"
              :placeholder="t('worktrees.modal.base.ref.placeholder')"
              class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
            />
          </div>
        </template>

        <!-- Branch Conflict Warning -->
        <div
          v-if="branchConflictWarning"
          class="p-2.5 rounded-lg border border-warning/40 bg-warning/10 text-warning text-xs flex items-center gap-2"
        >
          <AlertTriangle class="w-4 h-4 shrink-0" />
          <span>{{ branchConflictWarning }}</span>
        </div>

        <!-- Post-Add Options -->
        <div class="flex flex-col gap-2 pt-2 border-t border-line/60">
          <label class="flex items-center gap-2 text-xs text-ink cursor-pointer">
            <input v-model="openAfterCreate" type="checkbox" class="w-4 h-4 rounded accent-[var(--ug-accent-strong)] cursor-pointer" />
            <span>{{ t("worktrees.modal.open.tab") }}</span>
          </label>
        </div>
      </div>

      <template #footer>
        <UiButton variant="ghost" @click="showAddModal = false">{{ t("common.cancel") }}</UiButton>
        <UiButton
          variant="purple"
          :disabled="isAdding || !newWorktreePath.trim() || (addMode === 'new' && !newWorktreeBranch.trim())"
          @click="handleAddWorktreeAdvanced"
        >
          {{ isAdding ? t("worktrees.modal.creating") : t("worktrees.modal.confirm") }}
        </UiButton>
      </template>
    </UiModal>

    <!-- Modal: Move Worktree -->
    <UiModal
      :open="showMoveModal"
      :label="t('worktrees.move.title')"
      width="max-w-md"
      @update:open="showMoveModal = $event"
    >
      <template #header>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <Move class="w-4 h-4 text-accent" />
          <span>{{ t("worktrees.move.title") }}</span>
        </h3>
      </template>

      <div class="space-y-3">
        <div class="flex flex-col gap-1.5">
          <label class="text-xs text-ink-secondary font-medium">{{ t("worktrees.move.source") }}</label>
          <input
            :value="moveSourcePath"
            type="text"
            readonly
            class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink-muted font-mono"
          />
        </div>
        <div class="flex flex-col gap-1.5">
          <label class="text-xs text-ink-secondary font-medium">{{ t("worktrees.move.target") }}</label>
          <input
            v-model="moveTargetPath"
            type="text"
            :aria-label="t('worktrees.move.target')"
            :placeholder="t('worktrees.move.target.placeholder')"
            class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
          />
        </div>
      </div>

      <template #footer>
        <UiButton variant="ghost" :disabled="isMovingWorktree" @click="showMoveModal = false">
          {{ t("common.cancel") }}
        </UiButton>
        <UiButton
          variant="warning"
          :disabled="isMovingWorktree || !moveTargetPath.trim()"
          @click="handleMoveWorktree"
        >
          {{ isMovingWorktree ? t("worktrees.move.moving") : t("worktrees.move.confirm") }}
        </UiButton>
      </template>
    </UiModal>

    <!-- Modal: Lock Worktree -->
    <UiModal
      :open="showLockModal"
      :label="t('worktrees.modal.lock.label')"
      width="max-w-md"
      @update:open="showLockModal = $event"
    >
      <template #header>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <Lock class="w-4 h-4 text-warning" />
          <span>{{ t("worktrees.modal.lock.title") }}</span>
        </h3>
      </template>

      <div class="space-y-3">
        <div class="flex flex-col gap-1 text-xs">
          <span class="text-ink-secondary">{{ t("worktrees.modal.lock.target") }}</span>
          <span class="p-2 rounded-lg bg-canvas border border-line font-mono text-ink text-[11px] truncate">
            {{ targetLockPath }}
          </span>
        </div>
        <div class="flex flex-col gap-1.5">
          <label class="text-xs text-ink-secondary font-medium">{{ t("worktrees.modal.lock.reason") }}</label>
          <input
            v-model="lockReason"
            type="text"
            :aria-label="t('worktrees.modal.lock.reason.aria')"
            :placeholder="t('worktrees.modal.lock.reason.placeholder')"
            @keydown.enter="handleLockWorktree"
            class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink focus:outline-none focus:border-accent"
          />
        </div>
      </div>

      <template #footer>
        <UiButton variant="ghost" @click="showLockModal = false">{{ t("common.cancel") }}</UiButton>
        <UiButton variant="warning" :disabled="isWorktreeOperation" @click="handleLockWorktree">
          {{ isWorktreeOperation ? t("worktrees.modal.locking") : t("worktrees.modal.lock.confirm") }}
        </UiButton>
      </template>
    </UiModal>
  </div>
</template>
