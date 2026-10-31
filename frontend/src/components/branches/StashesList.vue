<script setup lang="ts">
import { confirmAction } from "../../composables/useConfirm";
import { ref } from "vue";
import { Archive, Play, RotateCcw, Trash2, ArrowDownCircle, ChevronDown, FileCode, RefreshCw, Copy, FileDiff, GitBranch } from "lucide-vue-next";
import { useContextMenu, type ContextMenuItem } from "../../composables/useContextMenu";
import { useRovingList } from "../../composables/useRovingList";
import UiContextMenu from "../ui/UiContextMenu.vue";
import { gitService } from "../../services/gitService";
import type { StashInfo, StashFileEntry } from "../../types";
import { t } from "../../i18n";

const props = defineProps<{
  repoPath: string;
  total: number;
  filteredCount: number;
  visible: StashInfo[];
  moreCount: number;
}>();

const emit = defineEmits<{
  (e: "apply", index: number): void;
  (e: "pop", index: number): void;
  (e: "drop", index: number): void;
  (e: "branch", stash: StashInfo): void;
  (e: "show-more"): void;
  (e: "notify", msg: string): void;
  (e: "refresh"): void;
}>();

const { setItemRef, tabindexFor, onKeydown: onRowKeydown, onFocus: onRowFocus } = useRovingList(
  () => props.visible,
  (stash) => String(stash.index)
);

const {
  isOpen: stashMenuOpen,
  position: stashMenuPos,
  items: stashMenuList,
  open: openMenuAt,
  close: closeStashMenu,
} = useContextMenu();
const menuStash = ref<StashInfo | null>(null);

function stashMenuItems(stash: StashInfo): ContextMenuItem[] {
  return [
    { id: "apply", label: t("branches.stash.menu.apply"), icon: Play },
    { id: "pop", label: t("branches.stash.menu.pop"), icon: RotateCcw },
    { id: "branch", label: t("branches.stash.menu.branch"), icon: GitBranch },
    {
      id: "files",
      label:
        expandedIndex.value === stash.index
          ? t("branches.stash.menu.files.collapse")
          : t("branches.stash.menu.files"),
      icon: FileDiff,
      separatorBefore: true,
    },
    { id: "copy", label: t("branches.stash.menu.copy"), icon: Copy, hint: `stash@{${stash.index}}` },
    { id: "drop", label: t("branches.stash.menu.drop"), icon: Trash2, danger: true, separatorBefore: true },
  ];
}

function openStashMenu(stash: StashInfo, event: MouseEvent) {
  menuStash.value = stash;
  openMenuAt(event, stashMenuItems(stash));
}

async function handleStashMenuSelect(id: string) {
  const stash = menuStash.value;
  if (!stash) return;
  switch (id) {
    case "apply":
      emit("apply", stash.index);
      break;
    case "pop":
      emit("pop", stash.index);
      break;
    case "branch":
      emit("branch", stash);
      break;
    case "files":
      await toggleFiles(stash);
      break;
    case "copy":
      await navigator.clipboard.writeText(`stash@{${stash.index}}`);
      emit("notify", t("branches.stash.toast.copied", { index: stash.index }));
      break;
    case "drop":
      emit("drop", stash.index);
      break;
  }
}

const expandedIndex = ref<number | null>(null);
const stashFiles = ref<StashFileEntry[]>([]);
const isLoadingFiles = ref(false);
const restoringPath = ref("");

async function toggleFiles(stash: StashInfo) {
  if (expandedIndex.value === stash.index) {
    expandedIndex.value = null;
    return;
  }
  expandedIndex.value = stash.index;
  stashFiles.value = [];
  isLoadingFiles.value = true;
  try {
    const files = await gitService.getStashFiles(props.repoPath, stash.index);
    if (expandedIndex.value === stash.index) {
      stashFiles.value = files || [];
    }
  } catch (err: any) {
    emit("notify", t("branches.stash.toast.files.failed", { error: err?.message || err }));
  } finally {
    isLoadingFiles.value = false;
  }
}

function statusColor(status: string) {
  if (status === "A") return "bg-success/15 text-success";
  if (status === "D") return "bg-danger/15 text-danger";
  if (status === "R") return "bg-purple/15 text-purple";
  return "bg-warning/15 text-warning";
}

async function restoreFile(file: StashFileEntry) {
  const index = expandedIndex.value;
  if (index === null) return;
  if (
    !(await confirmAction({
      message: t("branches.stash.confirm.restore", { index, path: file.path }),
      tone: "danger",
    }))
  )
    return;
  restoringPath.value = file.path;
  try {
    await gitService.applyFileFromStash(props.repoPath, index, file.path);
    emit("notify", t("branches.stash.toast.restored", { name: file.path.split("/").pop() ?? file.path }));
    emit("refresh");
  } catch (err: any) {
    emit("notify", t("branches.stash.toast.restore.failed", { error: err?.message || err }));
  } finally {
    restoringPath.value = "";
  }
}
</script>

<template>
        <div
          v-if="filteredCount === 0"
          class="py-12 text-center text-xs text-ink-muted border border-dashed border-line rounded-2xl"
        >
          {{ total === 0 ? t("branches.stash.empty") : t("branches.stash.empty.filtered") }}
        </div>

        <div
          v-for="(s, index) in visible"
          :key="s.index"
          :ref="(el) => setItemRef(s, el)"
          :tabindex="tabindexFor(s, index)"
          :aria-label="
            t('branches.stash.row.aria', {
              index: s.index,
              message: s.message || t('branches.stash.no.message'),
            })
          "
          class="rounded-xl bg-surface/60 border border-line hover:border-line-strong transition focus:outline-none focus-visible:ring-2 focus-visible:ring-accent/45"
          @focus="onRowFocus(s)"
          @keydown="onRowKeydown($event, index)"
          @keydown.enter.self="emit('apply', s.index)"
          @contextmenu="openStashMenu(s, $event)"
        >
          <div class="flex items-center justify-between p-3.5">
            <div class="flex items-center gap-3 min-w-0">
              <div class="w-8 h-8 rounded-lg bg-warning/10 text-warning flex items-center justify-center shrink-0">
                <Archive class="w-4 h-4" />
              </div>
              <div class="flex flex-col min-w-0">
                <div class="flex items-center gap-2">
                  <span class="font-mono text-xs font-bold text-ink">stash@{{ s.index }}</span>
                </div>
                <span class="text-xs text-ink-secondary mt-0.5 truncate">{{ s.message }}</span>
              </div>
            </div>

            <div class="flex items-center gap-2 shrink-0">
              <button
                @click="toggleFiles(s)"
                class="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-raised hover:bg-hover text-ink text-xs font-medium transition cursor-pointer border border-line"
                :title="
                  expandedIndex === s.index
                    ? t('branches.stash.files.collapse.hint')
                    : t('branches.stash.files.hint')
                "
              >
                <FileCode class="w-3 h-3 text-purple" aria-hidden="true" />
                <span>{{ t("branches.stash.files") }}</span>
                <ChevronDown
                  class="w-3 h-3 transition-transform"
                  :class="{ 'rotate-180': expandedIndex === s.index }"
                aria-hidden="true" />
              </button>
              <button
                @click="emit('apply', s.index)"
                class="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-raised hover:bg-hover text-ink text-xs font-medium transition cursor-pointer border border-line"
                :title="t('branches.stash.apply.hint')"
              >
                <Play class="w-3 h-3 text-accent" aria-hidden="true" />
                <span>{{ t("branches.stash.apply") }}</span>
              </button>
              <button
                @click="emit('pop', s.index)"
                class="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-raised hover:bg-hover text-ink text-xs font-medium transition cursor-pointer border border-line"
                :title="t('branches.stash.pop.hint')"
              >
                <RotateCcw class="w-3 h-3 text-warning" aria-hidden="true" />
                <span>{{ t("branches.stash.pop") }}</span>
              </button>
              <button
                @click="emit('branch', s)"
                class="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-raised hover:bg-hover text-ink text-xs font-medium transition cursor-pointer border border-line"
                :title="t('branches.stash.menu.branch')"
              >
                <GitBranch class="w-3 h-3 text-purple" aria-hidden="true" />
                <span>{{ t("branches.stash.menu.branch") }}</span>
              </button>
              <button :aria-label="t('branches.stash.drop')"
                @click="emit('drop', s.index)"
                class="p-1.5 rounded-lg hover:bg-danger/20 text-ink-muted hover:text-danger transition cursor-pointer"
                :title="t('branches.stash.drop')"
              >
                <Trash2 class="w-3.5 h-3.5" aria-hidden="true" />
              </button>
            </div>
          </div>

          <div v-if="expandedIndex === s.index" class="border-t border-line/60 px-3.5 py-2">
            <div v-if="isLoadingFiles" class="flex items-center gap-2 py-2 text-xs text-ink-muted">
              <RefreshCw class="w-3.5 h-3.5 animate-spin" />
              <span>{{ t("branches.stash.files.loading") }}</span>
            </div>
            <div v-else-if="stashFiles.length === 0" class="py-2 text-xs text-ink-muted">
              {{ t("branches.stash.files.empty") }}
            </div>
            <div v-else class="divide-y divide-line/40">
              <div
                v-for="file in stashFiles"
                :key="file.path"
                class="flex items-center gap-3 py-1.5 text-xs"
              >
                <span
                  class="w-5 h-5 rounded text-[10px] font-bold flex items-center justify-center shrink-0"
                  :class="statusColor(file.status)"
                >
                  {{ file.status }}
                </span>
                <span class="flex-1 font-mono text-ink-secondary truncate" :title="file.path">{{ file.path }}</span>
                <span class="font-mono text-[10px] text-ink-muted shrink-0">
                  +{{ file.additions }} -{{ file.deletions }}
                </span>
                <button
                  @click="restoreFile(file)"
                  :disabled="restoringPath === file.path"
                  class="px-2 py-0.5 rounded-md bg-surface hover:bg-raised border border-line text-[10px] font-semibold text-accent transition cursor-pointer disabled:opacity-50 shrink-0"
                  :title="t('branches.stash.restore.hint')"
                >
                  {{ restoringPath === file.path ? t("branches.stash.restoring") : t("branches.stash.restore") }}
                </button>
              </div>
            </div>
          </div>
        </div>

        <button
          v-if="moreCount > 0"
          @click="emit('show-more')"
          class="w-full py-2 rounded-xl border border-dashed border-line text-xs text-ink-secondary hover:text-ink hover:border-line-strong transition cursor-pointer flex items-center justify-center gap-1.5"
        >
          <ArrowDownCircle class="w-3.5 h-3.5" aria-hidden="true" />
          <span>{{ t("branches.stash.show.more", { count: moreCount }) }}</span>
        </button>

    <UiContextMenu
      :open="stashMenuOpen"
      :x="stashMenuPos.x"
      :y="stashMenuPos.y"
      :items="stashMenuList"
      @select="handleStashMenuSelect"
      @close="closeStashMenu"
    />
</template>
