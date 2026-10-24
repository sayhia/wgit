<script setup lang="ts">
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
} from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import UiModal from "../ui/UiModal.vue";
import UiButton from "../ui/UiButton.vue";
import { toolsStatus } from "../../composables/toolsStatus";
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
  notify(t("sub.toast.copied", { label }));
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
    notify(out ? t("sub.toast.updated.detail", { detail: out.slice(0, 60) }) : t("sub.toast.updated"));
    await loadSubmodules();
  } catch (err: any) {
    notify(t("sub.toast.update.failed", { error: err?.message || err }));
  } finally {
    isUpdatingSubmodules.value = false;
  }
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
              <span>{{ t("sub.title") }}</span>
            </h3>
            <p class="text-xs text-ink-muted mt-0.5">
              {{ t("sub.desc") }}
            </p>
          </div>

          <div class="flex items-center gap-2">
            <button
              @click="handleUpdateSubmodules(true)"
              :disabled="isUpdatingSubmodules"
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-50"
              :title="t('sub.sync.all.title')"
            >
              <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isUpdatingSubmodules }" />
              <span>{{ t("sub.sync.all") }}</span>
            </button>
            <button
              @click="handleUpdateSubmoduleRemote()"
              :disabled="isUpdatingSubmodules"
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-50"
              :title="t('sub.pull.remote.title')"
            >
              <RotateCcw class="w-3.5 h-3.5" />
              <span>{{ t("sub.pull.remote") }}</span>
            </button>
            <button
              @click="showAddSubmoduleModal = true"
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-xs font-semibold transition cursor-pointer shadow-xs"
            >
              <Plus class="w-3.5 h-3.5" />
              <span>{{ t("sub.add") }}</span>
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

          <div
            v-if="submodules.length === 0"
            class="py-12 text-center text-xs text-ink-muted border border-dashed border-line rounded-xl flex flex-col items-center gap-3"
          >
            <Boxes class="w-8 h-8 text-ink-muted" />
            <div>{{ t("sub.empty") }}</div>
            <button
              @click="showAddSubmoduleModal = true"
              class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-accent transition cursor-pointer"
            >
              {{ t("sub.bind") }}
            </button>
          </div>
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
              </div>
            </div>

            <div v-if="subtrees.length === 0" class="py-8 text-center text-xs text-ink-muted border border-dashed border-line rounded-xl">
              {{ isLoadingSubtrees ? t("sub.subtree.loading") : t("sub.subtree.empty") }}
            </div>
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

</template>
