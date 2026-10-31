<script setup lang="ts">
import { reportError } from "../../composables/useToast";
import { confirmAction } from "../../composables/useConfirm";
import { ref, computed, watch, onMounted, watchEffect } from "vue";
import { GitBranch, Search, Play, Filter, Undo2, History } from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import { toolsStatus } from "../../composables/toolsStatus";
import UiModal from "../ui/UiModal.vue";
import UiButton from "../ui/UiButton.vue";
import { backendText, t } from "../../i18n";
import type { OperationLogEntry, ReflogItem, RemoteDetail, WorktreeInfo, LfsLockInfo, LfsTrackPattern, SubmoduleInfo, DiskBreakdownResult, LargeFileInfo, RepoMaintenanceStats, LfsPruneResult, GitFsckResult, OperationLogResult } from "../../types";

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

async function copyToClipboard(text: string, label: string) {
  try {
    await navigator.clipboard.writeText(text);
    notify(t("reflog.toast.copied", { label }));
  } catch (err) {
    notify(t("common.copy.failed", { error: err instanceof Error ? err.message : String(err) }));
  }
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

function onReflogModalToggle(open: boolean) {
  if (!open && isCreatingReflogBranch.value) return;
  showReflogBranchModal.value = open;
}

async function loadReflog() {
  if (!props.currentRepoPath) return;
  isLoadingReflog.value = true;
  try {
    const list = await gitService.getReflog(props.currentRepoPath, 100);
    reflogItems.value = list || [];
  } catch (err) {
    reportError("Failed to load reflog", err);
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
    reportError("Failed to load operation log", err);
  }
}

/**
 * The backend classifies every reflog entry into a stable `action` key
 * (commit / amend / merge / reset …) and also ships a Chinese label. Prefer the
 * catalog so the chip and the confirm dialog follow the UI language, and fall
 * back to the backend label for actions the catalog does not know yet.
 */
function actionLabel(entry: OperationLogEntry): string {
  return backendText(`reflog.action.${entry.action}`, entry.actionLabel);
}

function undoHint(entry: OperationLogEntry): string {
  return backendText(`reflog.undo.hint.${entry.action}`, entry.undoHint);
}

/**
 * The undo banner sentence is rebuilt from the entries instead of using the
 * backend's `message`, so it reads in the active language.
 */
const oplogMessage = computed(() => {
  const log = oplog.value;
  const newest = log?.entries[0];
  if (!log || !newest) return "";
  if (log.entries.length === 1) return t("reflog.oplog.message.single");
  if (log.canUndo) {
    return t("reflog.oplog.message.undoable", {
      action: actionLabel(newest),
      hash: log.entries[1]!.shortHash,
    });
  }
  return t("reflog.oplog.message.not.undoable", {
    action: actionLabel(newest),
    hint: undoHint(newest),
  });
});

async function handleUndo() {
  const newest = oplog.value?.entries[0];
  const target = oplog.value?.entries[1];
  if (!props.currentRepoPath || !newest || !oplog.value?.canUndo || isUndoing.value) return;

  const confirmed = await confirmAction({
    message: t("reflog.confirm.undo", {
      action: actionLabel(newest),
      subject: newest.subject,
      target: target?.shortHash || t("reflog.confirm.undo.previous"),
    }),
    tone: "danger",
  });
  if (!confirmed) return;

  isUndoing.value = true;
  try {
    const res = await gitService.undoLastOperation(props.currentRepoPath);
    // Compose the toast from the result + the entry we just undid: the backend
    // sentence is Chinese, but the facts (target hash, action) are structured.
    const base = t("reflog.undo.done", {
      action: actionLabel(newest),
      hash: res.toHash || target?.shortHash || "",
    });
    notify(newest.action === "reset" ? base + t("reflog.undo.done.hard") : base);
    await loadReflog();
    emit("refresh");
  } catch (err: any) {
    notify(t("reflog.toast.undo.failed", { error: err?.message || err }));
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
  if (!(await confirmAction(t("reflog.confirm.checkout", { selector })))) return;
  try {
    await gitService.checkoutReflog(props.currentRepoPath, selector);
    notify(t("reflog.toast.checkout", { selector }));
    emit("refresh");
  } catch (err: any) {
    notify(t("reflog.toast.checkout.failed", { error: err?.message || err }));
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
    notify(t("reflog.toast.branch.created", { name: reflogBranchName.value.trim() }));
    showReflogBranchModal.value = false;
    emit("refresh");
  } catch (err: any) {
    notify(t("reflog.toast.branch.failed", { error: err?.message || err }));
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
        <div class="flex flex-wrap items-start justify-between gap-3 pb-3 border-b border-line">
          <div class="min-w-0 flex-1 basis-72">
            <h3 class="text-sm font-bold text-ink">{{ t("reflog.title") }}</h3>
            <p class="mt-1 text-xs text-ink-muted leading-relaxed">{{ t("reflog.subtitle") }}</p>
          </div>

          <div class="relative w-full sm:w-64 min-w-0 shrink-0">
            <input
              v-model="reflogSearch"
              type="text"
              :aria-label="t('reflog.search.aria')"
              :placeholder="t('reflog.search.placeholder')"
              class="w-full pl-7 pr-6 py-1 rounded-md bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent font-mono"
            />
            <Search class="w-3.5 h-3.5 text-ink-muted absolute left-2 top-2 pointer-events-none" />
          </div>
        </div>

        <!-- Recent operations + one-click undo -->
        <div
          v-if="oplog"
          class="rounded-xl border p-3 space-y-2"
          :class="oplog.canUndo ? 'border-accent/30 bg-accent-bg' : 'border-line bg-surface'"
        >
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0">
              <div class="text-xs font-bold text-ink flex items-center gap-2">
                <History class="w-3.5 h-3.5 text-accent" />
                <span>{{ t("reflog.oplog.title") }}</span>
              </div>
              <p class="text-[11px] text-ink-muted mt-1 leading-relaxed">{{ oplogMessage }}</p>
            </div>
            <button
              @click="handleUndo"
              :disabled="!oplog.canUndo || isUndoing"
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-xs font-semibold transition cursor-pointer disabled:opacity-30 shrink-0"
              :title="oplog.canUndo ? t('reflog.undo.hint') : t('reflog.undo.disabled')"
            >
              <Undo2 class="w-3.5 h-3.5" :class="{ 'animate-pulse': isUndoing }" aria-hidden="true" />
              <span>{{ isUndoing ? t("reflog.undoing") : t("reflog.undo") }}</span>
            </button>
          </div>

          <div v-if="oplog.entries.length > 1" class="flex flex-wrap items-center gap-1.5 pt-1 border-t border-line/50">
            <span class="text-[10px] text-ink-muted shrink-0">{{ t("reflog.timeline") }}</span>
            <span
              v-for="entry in oplog.entries.slice(0, 6)"
              :key="entry.selector"
              class="px-1.5 py-0.5 rounded text-[10px] font-mono"
              :class="entry.undoable ? 'bg-surface border border-line text-ink-muted' : 'bg-surface text-ink-muted line-through'"
              :title="
                t('reflog.timeline.entry.hint', { action: actionLabel(entry), subject: entry.subject })
              "
            >
              {{ actionLabel(entry) }}
            </span>
          </div>
        </div>

        <!-- Reflog List Table -->
        <div class="border border-line rounded-xl bg-surface overflow-hidden shadow-xs">
          <div class="divide-y divide-line/40 font-mono text-[11px]">
            <div
              v-for="item in filteredReflog"
              :key="item.selector"
              class="flex items-center justify-between p-2.5 hover:bg-surface transition-colors group"
            >
              <div class="flex items-center gap-3 min-w-0">
                <span class="px-2 py-0.5 rounded bg-accent-bg text-accent font-bold shrink-0">
                  {{ item.selector }}
                </span>
            <button
              type="button"
                  @click="copyToClipboard(item.commitHash, t('reflog.copy.hash.label'))"
              class="text-ink-muted hover:text-ink cursor-pointer hover:underline shrink-0 bg-transparent p-0 font-mono rounded-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent"
                  :title="t('reflog.copy.hash.hint')"
                >
                  {{ item.commitHash.substring(0, 8) }}
            </button>
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
                  class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-purple-bg hover:bg-purple-bg-strong text-purple text-[11px] font-semibold transition cursor-pointer border border-purple/30"
                  :title="t('reflog.branch.hint')"
                >
                  <GitBranch class="w-3 h-3" aria-hidden="true" />
                  <span>{{ t("reflog.branch") }}</span>
                </button>
                <button
                  @click="handleCheckoutReflog(item.selector)"
                  class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-surface hover:bg-raised border border-line text-[11px] font-medium text-ink transition cursor-pointer"
                  :title="t('reflog.checkout.hint')"
                >
                  <Play class="w-3 h-3 text-accent" aria-hidden="true" />
                  <span>{{ t("reflog.checkout") }}</span>
                </button>
              </div>
            </div>

            <div v-if="filteredReflog.length === 0" class="py-12 text-center text-xs text-ink-muted font-sans">
              {{ t("reflog.empty") }}
            </div>
          </div>
        </div>
      </div>


    <!-- Modal: Create Branch from Reflog -->
    <UiModal
      :open="showReflogBranchModal"
      :title="t('reflog.modal.title')"
      width="max-w-md"
      @update:open="onReflogModalToggle"
    >
      <template #header>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <GitBranch class="w-4 h-4 text-purple" />
          {{ t("reflog.modal.title") }}
        </h3>
      </template>

      <div class="space-y-3">
        <div class="flex flex-col gap-1.5">
          <label class="text-xs text-ink-secondary font-medium">{{ t("reflog.modal.base") }}</label>
          <span class="px-2.5 py-1.5 rounded-lg bg-canvas border border-line font-mono text-xs text-ink">
            {{ reflogSelectedCommit }}
          </span>
        </div>

        <div class="flex flex-col gap-1.5">
          <label class="text-xs text-ink-secondary font-medium">{{ t("reflog.modal.name") }}</label>
          <input
            v-model="reflogBranchName"
            type="text"
          :aria-label="t('reflog.modal.name')"
            @keydown.enter="handleCreateBranchFromReflog"
            class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
          />
        </div>
      </div>

      <template #footer>
        <UiButton variant="secondary" @click="showReflogBranchModal = false">{{ t("reflog.modal.cancel") }}</UiButton>
        <UiButton
          variant="purple"
          :disabled="isCreatingReflogBranch || !reflogBranchName.trim()"
          @click="handleCreateBranchFromReflog"
        >
          {{ isCreatingReflogBranch ? t("reflog.modal.creating") : t("reflog.modal.create") }}
        </UiButton>
      </template>
    </UiModal>

</template>
