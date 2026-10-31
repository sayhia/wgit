<script setup lang="ts">
import { reportError } from "../../composables/useToast";
import { t } from "../../i18n";
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
  GitFork,
  Settings2,
  Link2Off,
} from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import UiModal from "../ui/UiModal.vue";
import UiButton from "../ui/UiButton.vue";
import UiEmpty from "../ui/UiEmpty.vue";
import { toolsStatus } from "../../composables/toolsStatus";
import { useRovingList } from "../../composables/useRovingList";
import type { ReflogItem, RemoteDetail, WorktreeInfo, LfsLockInfo, LfsTrackPattern, SubmoduleInfo, DiskBreakdownResult, LargeFileInfo, RepoMaintenanceStats, LfsPruneResult, GitFsckResult, SubtreeItem, SubmoduleMatrixItem } from "../../types";

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
  notify(t("sub.toast.copied", { label }));
}

// -------------------------------------------------------------
// 5. SUBMODULES (大型多仓协同)
// -------------------------------------------------------------
const submodules = ref<SubmoduleInfo[]>([]);

// Keyboard parity with the branch/tag/stash lists: one row tabbable at a time,
// ↑/↓ between rows. Rows keep their own buttons, so no listbox role.
const {
  setItemRef: setSubRowRef,
  tabindexFor: subRowTabindex,
  onKeydown: onSubRowKeydown,
  onFocus: onSubRowFocus,
} = useRovingList(() => submodules.value, (sub) => sub.path);
const submoduleMatrix = ref<SubmoduleMatrixItem[]>([]);
const isLoadingSubmodules = ref(false);
const isUpdatingSubmodules = ref(false);
const isSyncingRecursive = ref(false);
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
    const [list, matrix] = await Promise.all([
      gitService.getSubmodules(props.currentRepoPath),
      gitService.getSubmoduleMatrix(props.currentRepoPath).catch((err) => {
        reportError("Failed to load submodule matrix", err);
        return [];
      }),
    ]);
    submodules.value = list || [];
    submoduleMatrix.value = matrix || [];
  } catch (err) {
    reportError("Failed to load submodules", err);
  } finally {
    isLoadingSubmodules.value = false;
  }
}

async function handleSyncRecursiveMatrix() {
  if (!props.currentRepoPath || isSyncingRecursive.value) return;
  isSyncingRecursive.value = true;
  try {
    const res = await gitService.syncSubmodulesRecursive(props.currentRepoPath);
    notify(res.message);
    await loadSubmodules();
  } catch (err: any) {
    notify(String(err?.message || err));
  } finally {
    isSyncingRecursive.value = false;
  }
}

async function handleUpdateSubmodules(recursive = true) {
  if (!props.currentRepoPath) return;
  isUpdatingSubmodules.value = true;
  try {
    const out = recursive
      ? await gitService.updateSubmodulesRecursive(props.currentRepoPath)
      : await gitService.updateSubmodules(props.currentRepoPath, true, false);
    notify(out ? t("sub.toast.updated.detail", { detail: out.slice(0, 60) }) : t("sub.toast.updated"));
    await loadSubmodules();
  } catch (err: any) {
    notify(t("sub.toast.update.failed", { error: err?.message || err }));
  } finally {
    isUpdatingSubmodules.value = false;
  }
}

async function handleInitSubmodules() {
  if (!props.currentRepoPath) return;
  isUpdatingSubmodules.value = true;
  try {
    const out = await gitService.updateSubmodules(props.currentRepoPath, true, false);
    notify(out ? t("sub.toast.init.done") + `: ${out.slice(0, 60)}` : t("sub.toast.init.done"));
    await loadSubmodules();
  } catch (err: any) {
    notify(t("sub.toast.init.failed", { error: err?.message || err }));
  } finally {
    isUpdatingSubmodules.value = false;
  }
}

function handleOpenSubmodule(sub: SubmoduleInfo) {
  const cleanBase = props.currentRepoPath.replace(/\/+$/, "");
  const cleanSub = sub.path.replace(/^\/+/, "");
  const fullPath = sub.path.startsWith("/") ? sub.path : `${cleanBase}/${cleanSub}`;
  emit("open-repo", fullPath);
}

async function handleSyncSubmodule(submodulePath: string) {
  if (!props.currentRepoPath) return;
  isUpdatingSubmodules.value = true;
  try {
    const out = await gitService.syncSubmodule(props.currentRepoPath, submodulePath);
    notify(out ? t("sub.toast.url.detail", { detail: out.slice(0, 60) }) : t("sub.toast.url.synced"));
    await loadSubmodules();
  } catch (err: any) {
    notify(t("sub.toast.url.failed", { error: err?.message || err }));
  } finally {
    isUpdatingSubmodules.value = false;
  }
}

async function handleUpdateSubmoduleRemote(submodulePath = "") {
  if (!props.currentRepoPath) return;
  isUpdatingSubmodules.value = true;
  try {
    const out = await gitService.updateSubmoduleRemote(props.currentRepoPath, submodulePath);
    notify(out ? t("sub.toast.remote.detail", { detail: out.slice(0, 60) }) : t("sub.toast.remote.synced"));
    await loadSubmodules();
  } catch (err: any) {
    notify(t("sub.toast.remote.failed", { error: err?.message || err }));
  } finally {
    isUpdatingSubmodules.value = false;
  }
}

async function handleAddSubmodule() {
  if (!props.currentRepoPath || !newSubmoduleUrl.value.trim() || !newSubmodulePath.value.trim()) return;
  try {
    await gitService.addSubmodule(props.currentRepoPath, newSubmoduleUrl.value.trim(), newSubmodulePath.value.trim());
    notify(t("sub.toast.bound", { path: newSubmodulePath.value }));
    newSubmoduleUrl.value = "";
    newSubmodulePath.value = "";
    showAddSubmoduleModal.value = false;
    await loadSubmodules();
  } catch (err: any) {
    notify(t("sub.toast.add.failed", { error: err?.message || err }));
  }
}

async function handleRemoveSubmodule(path: string) {
  if (!props.currentRepoPath || !path) return;
  if (!(await confirmAction({ message: t("sub.confirm.remove", { path }), tone: "danger" }))) return;
  try {
    await gitService.removeSubmodule(props.currentRepoPath, path);
    notify(t("sub.toast.removed", { path }));
    await loadSubmodules();
  } catch (err: any) {
    notify(t("sub.toast.remove.failed", { error: err?.message || err }));
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
    notify(t("sub.toast.branch.set", { name: selectedSubmodule.value.name, branch: targetSubmoduleBranch.value }));
    showSetSubmoduleBranchModal.value = false;
    await loadSubmodules();
  } catch (err: any) {
    notify(t("sub.toast.branch.failed", { error: err?.message || err }));
  }
}


// Git subtree (vendored repository under a prefix)
const subtrees = ref<SubtreeItem[]>([]);
const isLoadingSubtrees = ref(false);

const {
  setItemRef: setSubtreeRowRef,
  tabindexFor: subtreeRowTabindex,
  onKeydown: onSubtreeRowKeydown,
  onFocus: onSubtreeRowFocus,
} = useRovingList(() => subtrees.value, (item) => item.prefix);
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
    reportError("Failed to load subtrees", err);
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
    notify(res?.message || t("sub.toast.subtree.added", { prefix }));
    showAddSubtreeModal.value = false;
    newSubtreePrefix.value = "";
    newSubtreeUrl.value = "";
    await loadSubtrees();
    emit("refresh");
  } catch (err: any) {
    notify(t("sub.toast.subtree.add.failed", { error: err?.message || err }));
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
    notify(res?.message || t("sub.toast.subtree.updated", { prefix: item.prefix }));
    await loadSubtrees();
    emit("refresh");
  } catch (err: any) {
    notify(t("sub.toast.subtree.pull.failed", { error: err?.message || err }));
  } finally {
    subtreeBusy.value = "";
  }
}

async function handlePushSubtree(item: SubtreeItem) {
  if (!props.currentRepoPath || subtreeBusy.value) return;
  if (!(await confirmAction(t("sub.confirm.subtree.push", { prefix: item.prefix, branch: item.branch })))) return;
  subtreeBusy.value = `push:${item.prefix}`;
  try {
    const res = await gitService.pushSubtree(
      props.currentRepoPath,
      item.prefix,
      item.remoteOrUrl,
      item.branch
    );
    notify(res?.message || t("sub.toast.subtree.pushed", { prefix: item.prefix }));
    emit("refresh");
  } catch (err: any) {
    notify(t("sub.toast.subtree.push.failed", { error: err?.message || err }));
  } finally {
    subtreeBusy.value = "";
  }
}

// Split Subtree
const showSplitSubtreeModal = ref(false);
const selectedSubtreeForSplit = ref<SubtreeItem | null>(null);
const splitBranchName = ref("");
const splitSquash = ref(false);
const splitRejoin = ref(false);

function openSplitSubtreeModal(item: SubtreeItem) {
  selectedSubtreeForSplit.value = item;
  splitBranchName.value = `${item.prefix.replace(/[\/\\]+/g, "-")}-split`;
  splitSquash.value = false;
  splitRejoin.value = false;
  showSplitSubtreeModal.value = true;
}

async function handleSplitSubtree() {
  if (!props.currentRepoPath || !selectedSubtreeForSplit.value || subtreeBusy.value) return;
  const item = selectedSubtreeForSplit.value;
  const branch = splitBranchName.value.trim();
  if (!branch) return;
  subtreeBusy.value = `split:${item.prefix}`;
  try {
    const res = await gitService.splitSubtree(
      props.currentRepoPath,
      item.prefix,
      branch,
      splitSquash.value,
      splitRejoin.value
    );
    notify(res?.message || t("sub.toast.subtree.split.success", { prefix: item.prefix, commit: res?.newCommit || "" }));
    showSplitSubtreeModal.value = false;
    emit("refresh");
  } catch (err: any) {
    notify(t("sub.toast.subtree.split.failed", { error: err?.message || err }));
  } finally {
    subtreeBusy.value = "";
  }
}

// Edit Subtree Metadata
const showEditSubtreeModal = ref(false);
const selectedSubtreeForEdit = ref<SubtreeItem | null>(null);
const editSubtreeUrl = ref("");
const editSubtreeBranch = ref("");
const editSubtreeSquash = ref(true);

function openEditSubtreeModal(item: SubtreeItem) {
  selectedSubtreeForEdit.value = item;
  editSubtreeUrl.value = item.remoteOrUrl || "";
  editSubtreeBranch.value = item.branch || "main";
  editSubtreeSquash.value = true;
  showEditSubtreeModal.value = true;
}

async function handleEditSubtree() {
  if (!props.currentRepoPath || !selectedSubtreeForEdit.value || subtreeBusy.value) return;
  const item = selectedSubtreeForEdit.value;
  const url = editSubtreeUrl.value.trim();
  const branch = editSubtreeBranch.value.trim();
  if (!url || !branch) return;
  subtreeBusy.value = `edit:${item.prefix}`;
  try {
    await gitService.updateSubtreeMetadata(
      props.currentRepoPath,
      item.prefix,
      url,
      branch,
      editSubtreeSquash.value
    );
    notify(t("sub.toast.subtree.edit.success", { prefix: item.prefix }));
    showEditSubtreeModal.value = false;
    await loadSubtrees();
  } catch (err: any) {
    notify(t("sub.toast.subtree.edit.failed", { error: err?.message || err }));
  } finally {
    subtreeBusy.value = "";
  }
}

// Unlink Subtree
async function handleUnlinkSubtree(item: SubtreeItem) {
  if (!props.currentRepoPath || subtreeBusy.value) return;
  if (!(await confirmAction(t("sub.confirm.subtree.unlink", { prefix: item.prefix })))) return;
  subtreeBusy.value = `unlink:${item.prefix}`;
  try {
    await gitService.unlinkSubtree(props.currentRepoPath, item.prefix);
    notify(t("sub.toast.subtree.unlink.success", { prefix: item.prefix }));
    await loadSubtrees();
  } catch (err: any) {
    notify(t("sub.toast.subtree.unlink.failed", { error: err?.message || err }));
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
        <!--
          Same header rule as the worktrees tab: actions keep nowrap labels and
          never shrink; the row wraps when the five actions + description cannot
          share one line (they were squeezed onto two lines at 960/1024px).
        -->
        <div class="flex flex-wrap items-center justify-between gap-x-3 gap-y-2 pb-3 border-b border-line">
          <div class="min-w-0 flex-1">
            <h3 class="text-sm font-bold text-ink flex items-center gap-2">
              <Boxes class="w-4 h-4 text-accent" />
              <span>{{ t("sub.title") }}</span>
            </h3>
            <p class="text-xs text-ink-muted mt-0.5">
              {{ t("sub.desc") }}
            </p>
          </div>

          <div class="ml-auto flex flex-wrap items-center gap-2 shrink-0">
            <button
              @click="handleSyncRecursiveMatrix"
              :disabled="isSyncingRecursive"
              class="flex items-center gap-1.5 whitespace-nowrap px-3 py-1.5 rounded-lg bg-accent text-accent-contrast text-xs font-semibold hover:opacity-90 transition cursor-pointer disabled:opacity-50"
              :title="t('sub.matrix.sync.title')"
              data-testid="submodule-sync-recursive-btn"
            >
              <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isSyncingRecursive }" />
              <span>{{ isSyncingRecursive ? t("sub.matrix.syncing") : t("sub.matrix.sync") }}</span>
            </button>
            <button
              @click="handleUpdateSubmodules(true)"
              :disabled="isUpdatingSubmodules"
              class="flex items-center gap-1.5 whitespace-nowrap px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-50"
              :title="t('sub.sync.all.title')"
            >
              <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isUpdatingSubmodules }" />
              <span>{{ t("sub.sync.all") }}</span>
            </button>
            <button
              @click="handleInitSubmodules"
              :disabled="isUpdatingSubmodules"
              class="flex items-center gap-1.5 whitespace-nowrap px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-50"
              :title="t('sub.init.all.title')"
            >
              <Boxes class="w-3.5 h-3.5 text-accent" />
              <span>{{ t("sub.init.all") }}</span>
            </button>
            <button
              @click="handleUpdateSubmoduleRemote()"
              :disabled="isUpdatingSubmodules"
              class="flex items-center gap-1.5 whitespace-nowrap px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-50"
              :title="t('sub.pull.remote.title')"
            >
              <RotateCcw class="w-3.5 h-3.5" />
              <span>{{ t("sub.pull.remote") }}</span>
            </button>
            <button
              @click="showAddSubmoduleModal = true"
              class="flex items-center gap-1.5 whitespace-nowrap px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-xs font-semibold transition cursor-pointer shadow-xs"
            >
              <Plus class="w-3.5 h-3.5" />
              <span>{{ t("sub.add") }}</span>
            </button>
          </div>
        </div>

        <!-- Submodules List -->
        <div class="space-y-3">
          <div
            v-for="(sub, index) in submodules"
            :key="sub.path"
            :ref="(el) => setSubRowRef(sub, el)"
            :tabindex="subRowTabindex(sub, index)"
            class="p-4 rounded-xl border border-line bg-surface/30 hover:bg-surface/50 transition flex flex-col gap-3 shadow-xs focus:outline-none focus-visible:ring-2 focus-visible:ring-accent/45"
            data-testid="submodule-matrix-card"
            @focus="onSubRowFocus(sub)"
            @keydown="onSubRowKeydown($event, index)"
          >
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-3 min-w-0">
                <span class="font-bold text-sm text-ink truncate">{{ sub.name }}</span>
                <span
                  class="px-2 py-0.5 rounded text-[10px] font-bold uppercase tracking-wider font-mono whitespace-nowrap shrink-0"
                  :class="{
                    'bg-success/15 text-success border border-success/30': sub.status === 'synced',
                    'bg-warning/15 text-warning border border-warning/30': sub.status === 'modified',
                    'bg-purple/15 text-purple border border-purple/30': sub.status === 'uninitialized',
                    'bg-danger/15 text-danger border border-danger/30': sub.status === 'conflict',
                  }"
                >
                  {{
                    sub.status === 'synced' ? t('sub.status.synced') :
                    sub.status === 'modified' ? t('sub.status.modified') :
                    sub.status === 'uninitialized' ? t('sub.status.uninitialized') : t('sub.status.conflict')
                  }}
                </span>
                <span class="text-xs text-ink-muted font-mono truncate" :title="sub.path">
                  📁 {{ sub.path }}
                </span>
              </div>

              <!-- Action buttons -->
              <div class="flex items-center gap-2 shrink-0">
                <button
                  @click="handleOpenSubmodule(sub)"
                  class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-surface hover:bg-raised border border-line text-xs font-medium text-ink transition cursor-pointer"
                  :title="t('sub.open.workspace')"
                >
                  <FolderTree class="w-3 h-3 text-accent" />
                  <span>{{ t("sub.open.workspace") }}</span>
                </button>
                <button
                  @click="handleUpdateSubmoduleRemote(sub.path)"
                  class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-surface hover:bg-raised border border-line text-xs font-medium text-ink transition cursor-pointer"
                  :title="t('sub.sync.one.title')"
                >
                  <RefreshCw class="w-3 h-3 text-accent" />
                  <span>{{ t("sub.sync.one") }}</span>
                </button>
                <button
                  @click="handleSyncSubmodule(sub.path)"
                  class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-surface hover:bg-raised border border-line text-xs font-medium text-ink transition cursor-pointer"
                  :title="t('sub.sync.url.title')"
                >
                  <RotateCcw class="w-3 h-3 text-warning" />
                  <span>{{ t("sub.sync.url") }}</span>
                </button>
                <button
                  @click="openSetSubmoduleBranchModal(sub)"
                  class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-surface hover:bg-raised border border-line text-xs font-medium text-ink transition cursor-pointer"
                  :title="t('sub.branch.edit.title')"
                >
                  <GitBranch class="w-3 h-3 text-purple" />
                  <span>{{ t("sub.branch.tracking", { branch: sub.branch || "HEAD" }) }}</span>
                </button>
                <button :aria-label="t('sub.remove.aria')"
                  @click="handleRemoveSubmodule(sub.path)"
                  class="p-1.5 rounded-lg hover:bg-danger/20 text-ink-muted hover:text-danger transition cursor-pointer"
                  :title="t('sub.remove.aria')"
                >
                  <Trash2 class="w-3.5 h-3.5" />
                </button>
              </div>
            </div>

            <!-- Details: URL and Commit -->
            <div class="grid grid-cols-2 gap-4 pt-2 border-t border-line/40 text-xs font-mono">
              <div class="flex items-center gap-2 truncate text-ink-muted">
                <span class="text-ink-secondary shrink-0 font-sans font-medium">{{ t("sub.url.label") }}</span>
                <span
                  @click="copyToClipboard(sub.url, t('sub.url.label'))"
                  class="text-ink hover:text-accent hover:underline cursor-pointer truncate"
                  :title="sub.url"
                >
                  {{ sub.url }}
                </span>
              </div>
              <div class="flex items-center gap-2 text-ink-muted">
                <span class="text-ink-secondary shrink-0 font-sans font-medium">{{ t("sub.commit.label") }}</span>
                <span
                  @click="copyToClipboard(sub.commit, 'Commit Hash')"
                  class="text-accent hover:underline cursor-pointer font-bold shrink-0"
                  :title="t('sub.commit.copy.title')"
                >
                  {{ sub.commit ? sub.commit.substring(0, 10) : 'N/A' }}
                </span>
              </div>
            </div>
          </div>

          <UiEmpty
            v-if="submodules.length === 0"
            :icon="Boxes"
            :title="t('sub.empty')"
            bordered
          >
            <UiButton variant="secondary" size="sm" @click="showAddSubmoduleModal = true">
              {{ t("sub.bind") }}
            </UiButton>
          </UiEmpty>
        </div>

        <!-- Git Subtree (vendored repositories) -->
        <div class="space-y-3 pt-4 border-t border-line" data-tool-section="subtree-list">
          <div class="flex items-center justify-between pb-2 border-b border-line">
            <div>
              <h3 class="text-sm font-bold text-ink flex items-center gap-2">
                <FolderTree class="w-4 h-4 text-purple" />
                <span>{{ t("sub.subtree.title") }}</span>
              </h3>
              <p class="text-xs text-ink-muted mt-0.5">
                {{ t("sub.subtree.desc") }}
              </p>
            </div>
            <button
              @click="showAddSubtreeModal = true"
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-purple-strong hover:bg-purple-dark text-white text-xs font-semibold transition cursor-pointer shadow-xs"
            >
              <Plus class="w-3.5 h-3.5" />
              <span>{{ t("sub.subtree.add") }}</span>
            </button>
          </div>

          <div class="space-y-2">
            <div
              v-for="(item, index) in subtrees"
              :key="item.prefix"
              :ref="(el) => setSubtreeRowRef(item, el)"
              :tabindex="subtreeRowTabindex(item, index)"
              @focus="onSubtreeRowFocus(item)"
              @keydown="onSubtreeRowKeydown($event, index)"
              class="p-3 rounded-xl border border-line bg-surface/30 flex items-center justify-between gap-3 focus:outline-none focus-visible:ring-2 focus-visible:ring-accent/45"
            >
              <div class="min-w-0 space-y-1">
                <div class="flex items-center gap-2">
                  <span class="font-mono text-xs font-bold text-ink truncate">{{ item.prefix }}</span>
                  <span class="px-1.5 py-0.5 rounded bg-purple/15 text-purple text-[9px] font-bold shrink-0">
                    {{ item.branch }}
                  </span>
                </div>
                <div class="text-[10px] text-ink-muted font-mono truncate" :title="item.remoteOrUrl">
                  {{ item.remoteOrUrl || t("sub.subtree.no.remote") }}
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
                  :title="t('sub.subtree.pull.title')"
                >
                  <RefreshCw class="w-3 h-3" :class="{ 'animate-spin': subtreeBusy === `pull:${item.prefix}` }" />
                  <span>{{ t("sub.subtree.pull") }}</span>
                </button>
                <button
                  @click="handlePushSubtree(item)"
                  :disabled="!!subtreeBusy || !item.remoteOrUrl"
                  class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-surface hover:bg-raised border border-line text-xs font-medium text-purple transition cursor-pointer disabled:opacity-40"
                  :title="t('sub.subtree.push.title')"
                >
                  <ArrowUpCircle class="w-3 h-3" :class="{ 'animate-pulse': subtreeBusy === `push:${item.prefix}` }" />
                  <span>{{ t("sub.subtree.push") }}</span>
                </button>
                <button
                  @click="openSplitSubtreeModal(item)"
                  :disabled="!!subtreeBusy"
                  class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-surface hover:bg-raised border border-line text-xs font-medium text-ink transition cursor-pointer disabled:opacity-40"
                  :title="t('sub.subtree.split.title')"
                  data-action="split-subtree"
                >
                  <GitFork class="w-3 h-3" />
                  <span>{{ t("sub.subtree.split") }}</span>
                </button>
                <button
                  @click="openEditSubtreeModal(item)"
                  :disabled="!!subtreeBusy"
                  class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-surface hover:bg-raised border border-line text-xs font-medium text-ink transition cursor-pointer disabled:opacity-40"
                  :title="t('sub.subtree.edit.title')"
                  data-action="edit-subtree"
                >
                  <Settings2 class="w-3 h-3" />
                  <span>{{ t("sub.subtree.edit") }}</span>
                </button>
                <button
                  @click="handleUnlinkSubtree(item)"
                  :disabled="!!subtreeBusy"
                  class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-surface hover:bg-raised border border-line text-xs font-medium text-danger transition cursor-pointer disabled:opacity-40"
                  :title="t('sub.subtree.unlink.title')"
                  data-action="unlink-subtree"
                >
                  <Link2Off class="w-3 h-3" />
                  <span>{{ t("sub.subtree.unlink") }}</span>
                </button>
              </div>
            </div>

            <UiEmpty
              v-if="subtrees.length === 0"
              :icon="FolderTree"
              :title="isLoadingSubtrees ? t('sub.subtree.loading') : t('sub.subtree.empty')"
              bordered
            />
          </div>
        </div>
      </div>


    <!-- Modal: Add Subtree -->
    <UiModal
      :open="showAddSubtreeModal"
      :label="t('sub.modal.subtree.label')"
      width="max-w-md"
      @update:open="showAddSubtreeModal = $event"
    >
      <template #header>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <FolderTree class="w-4 h-4 text-purple" />
          <span>{{ t("sub.modal.subtree.title") }}</span>
        </h3>
      </template>

      <div class="space-y-3">
        <div class="flex flex-col gap-1.5">
          <label class="text-xs text-ink-secondary font-medium">{{ t("sub.modal.prefix") }}</label>
          <input
            v-model="newSubtreePrefix"
            type="text"
            :aria-label="t('sub.modal.prefix.aria')"
            placeholder="libs/shared-core"
            class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
          />
        </div>
        <div class="flex flex-col gap-1.5">
          <label class="text-xs text-ink-secondary font-medium">{{ t("sub.modal.upstream.url") }}</label>
          <input
            v-model="newSubtreeUrl"
            type="text"
            :aria-label="t('sub.modal.upstream.url.aria')"
            :placeholder="t('sub.modal.upstream.url.placeholder')"
            class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
          />
        </div>
        <div class="flex flex-col gap-1.5">
          <label class="text-xs text-ink-secondary font-medium">{{ t("sub.modal.branch") }}</label>
          <input
            v-model="newSubtreeBranch"
            type="text"
            :aria-label="t('sub.modal.branch.aria')"
            placeholder="main"
            class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
          />
        </div>
        <label class="flex items-center gap-2 text-[11px] text-ink-secondary cursor-pointer select-none">
          <input v-model="newSubtreeSquash" type="checkbox" class="rounded accent-[var(--ug-purple-strong)]" />
          <span>{{ t("sub.modal.squash") }}</span>
        </label>
      </div>

      <template #footer>
        <UiButton variant="ghost" @click="showAddSubtreeModal = false">{{ t("common.cancel") }}</UiButton>
        <UiButton
          variant="purple"
          :disabled="subtreeBusy === 'add' || !newSubtreePrefix.trim() || !newSubtreeUrl.trim() || !newSubtreeBranch.trim()"
          @click="handleAddSubtree"
        >
          {{ subtreeBusy === "add" ? t("sub.adding") : t("sub.modal.confirm.add") }}
        </UiButton>
      </template>
    </UiModal>

    <!-- Modal: Add Submodule -->
    <UiModal
      :open="showAddSubmoduleModal"
      :label="t('sub.modal.submodule.label')"
      width="max-w-md"
      @update:open="showAddSubmoduleModal = $event"
    >
      <template #header>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <Boxes class="w-4 h-4 text-accent" />
          <span>{{ t("sub.modal.submodule.title") }}</span>
        </h3>
      </template>

      <div class="space-y-3">
        <div class="flex flex-col gap-1.5">
          <label class="text-xs text-ink-secondary font-medium">{{ t("sub.modal.submodule.url") }}</label>
          <input
            v-model="newSubmoduleUrl"
            type="text"
            :aria-label="t('sub.modal.submodule.url.aria')"
            placeholder="https://github.com/org/repo.git"
            class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
          />
        </div>
        <div class="flex flex-col gap-1.5">
          <label class="text-xs text-ink-secondary font-medium">{{ t("sub.modal.submodule.path") }}</label>
          <input
            v-model="newSubmodulePath"
            type="text"
            :aria-label="t('sub.modal.submodule.path.aria')"
            :placeholder="t('sub.modal.submodule.path.placeholder')"
            class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
          />
        </div>
      </div>

      <template #footer>
        <UiButton variant="ghost" @click="showAddSubmoduleModal = false">{{ t("common.cancel") }}</UiButton>
        <UiButton
          variant="primary"
          :disabled="!newSubmoduleUrl.trim() || !newSubmodulePath.trim()"
          @click="handleAddSubmodule"
        >
          {{ t("sub.modal.confirm.add") }}
        </UiButton>
      </template>
    </UiModal>

    <!-- Modal: Set Submodule Branch -->
    <UiModal
      :open="showSetSubmoduleBranchModal && Boolean(selectedSubmodule)"
      :label="t('sub.modal.branch.label')"
      width="max-w-md"
      @update:open="showSetSubmoduleBranchModal = $event"
    >
      <template #header>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <GitBranch class="w-4 h-4 text-purple" />
          <span>{{ t("sub.modal.branch.title") }}</span>
        </h3>
      </template>

      <div v-if="selectedSubmodule" class="space-y-3">
        <div class="flex flex-col gap-1 text-xs">
          <span class="text-ink-muted">{{ t("sub.modal.branch.current") }}</span>
          <span class="font-bold text-ink font-mono">{{ selectedSubmodule.name }} ({{ selectedSubmodule.path }})</span>
        </div>
        <div class="flex flex-col gap-1.5">
          <label class="text-xs text-ink-secondary font-medium">{{ t("sub.modal.branch.tracked") }}</label>
          <input
            v-model="targetSubmoduleBranch"
            type="text"
            :aria-label="t('sub.modal.branch.tracked.aria')"
            placeholder="main / master / release/v1.0"
            class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
          />
        </div>
      </div>

      <template #footer>
        <UiButton variant="ghost" @click="showSetSubmoduleBranchModal = false">{{ t("common.cancel") }}</UiButton>
        <UiButton
          variant="purple"
          :disabled="!targetSubmoduleBranch.trim()"
          @click="handleSaveSubmoduleBranch"
        >
          {{ t("sub.modal.save") }}
        </UiButton>
      </template>
    </UiModal>

    <!-- Modal: Split Subtree -->
    <UiModal
      :open="showSplitSubtreeModal && Boolean(selectedSubtreeForSplit)"
      :label="t('sub.modal.subtree.split.label')"
      width="max-w-md"
      @update:open="showSplitSubtreeModal = $event"
    >
      <template #header>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <GitFork class="w-4 h-4 text-accent" />
          <span>{{ t("sub.modal.subtree.split.title") }}</span>
        </h3>
      </template>

      <div v-if="selectedSubtreeForSplit" class="space-y-3">
        <div class="flex flex-col gap-1 text-xs">
          <span class="text-ink-muted">{{ t("sub.modal.prefix") }}</span>
          <span class="font-bold text-ink font-mono">{{ selectedSubtreeForSplit.prefix }}</span>
        </div>
        <div class="flex flex-col gap-1.5">
          <label class="text-xs text-ink-secondary font-medium">{{ t("sub.modal.subtree.split.branch") }}</label>
          <input
            v-model="splitBranchName"
            type="text"
            :aria-label="t('sub.modal.subtree.split.branch.aria')"
            :placeholder="t('sub.modal.subtree.split.branch.placeholder')"
            class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
          />
        </div>
        <div class="space-y-1.5 pt-1">
          <label class="flex items-center gap-2 text-[11px] text-ink-secondary cursor-pointer select-none">
            <input v-model="splitSquash" type="checkbox" class="rounded accent-accent" />
            <span>{{ t("sub.modal.subtree.split.squash") }}</span>
          </label>
          <label class="flex items-center gap-2 text-[11px] text-ink-secondary cursor-pointer select-none">
            <input v-model="splitRejoin" type="checkbox" class="rounded accent-accent" />
            <span>{{ t("sub.modal.subtree.split.rejoin") }}</span>
          </label>
        </div>
      </div>

      <template #footer>
        <UiButton variant="ghost" @click="showSplitSubtreeModal = false">{{ t("common.cancel") }}</UiButton>
        <UiButton
          variant="primary"
          :disabled="!splitBranchName.trim() || !!subtreeBusy"
          @click="handleSplitSubtree"
        >
          {{ t("sub.modal.subtree.split.btn") }}
        </UiButton>
      </template>
    </UiModal>

    <!-- Modal: Edit Subtree Metadata -->
    <UiModal
      :open="showEditSubtreeModal && Boolean(selectedSubtreeForEdit)"
      :label="t('sub.modal.subtree.edit.label')"
      width="max-w-md"
      @update:open="showEditSubtreeModal = $event"
    >
      <template #header>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <Settings2 class="w-4 h-4 text-purple" />
          <span>{{ t("sub.modal.subtree.edit.title") }}</span>
        </h3>
      </template>

      <div v-if="selectedSubtreeForEdit" class="space-y-3">
        <div class="flex flex-col gap-1 text-xs">
          <span class="text-ink-muted">{{ t("sub.modal.prefix") }}</span>
          <span class="font-bold text-ink font-mono">{{ selectedSubtreeForEdit.prefix }}</span>
        </div>
        <div class="flex flex-col gap-1.5">
          <label class="text-xs text-ink-secondary font-medium">{{ t("sub.modal.upstream.url") }}</label>
          <input
            v-model="editSubtreeUrl"
            type="text"
            :aria-label="t('sub.modal.upstream.url.aria')"
            :placeholder="t('sub.modal.upstream.url.placeholder')"
            class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
          />
        </div>
        <div class="flex flex-col gap-1.5">
          <label class="text-xs text-ink-secondary font-medium">{{ t("sub.modal.branch") }}</label>
          <input
            v-model="editSubtreeBranch"
            type="text"
            :aria-label="t('sub.modal.branch.aria')"
            placeholder="main"
            class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
          />
        </div>
        <label class="flex items-center gap-2 text-[11px] text-ink-secondary cursor-pointer select-none">
          <input v-model="editSubtreeSquash" type="checkbox" class="rounded accent-[var(--ug-purple-strong)]" />
          <span>{{ t("sub.modal.squash") }}</span>
        </label>
      </div>

      <template #footer>
        <UiButton variant="ghost" @click="showEditSubtreeModal = false">{{ t("common.cancel") }}</UiButton>
        <UiButton
          variant="purple"
          :disabled="!editSubtreeUrl.trim() || !editSubtreeBranch.trim() || !!subtreeBusy"
          @click="handleEditSubtree"
        >
          {{ t("sub.modal.save") }}
        </UiButton>
      </template>
    </UiModal>

</template>
