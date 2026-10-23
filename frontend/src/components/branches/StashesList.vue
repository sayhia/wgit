<script setup lang="ts">
import { confirmAction } from "../../composables/useConfirm";
import { ref } from "vue";
import { Archive, Play, RotateCcw, Trash2, ArrowDownCircle, ChevronDown, FileCode, RefreshCw, Copy, FileDiff } from "lucide-vue-next";
import { useContextMenu, type ContextMenuItem } from "../../composables/useContextMenu";
import UiContextMenu from "../ui/UiContextMenu.vue";
import { gitService } from "../../services/gitService";
import type { StashInfo, StashFileEntry } from "../../types";

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
  (e: "show-more"): void;
  (e: "notify", msg: string): void;
  (e: "refresh"): void;
}>();

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
    { id: "apply", label: "应用（保留储藏）", icon: Play },
    { id: "pop", label: "弹出（应用并删除）", icon: RotateCcw },
    { id: "files", label: expandedIndex.value === stash.index ? "收起文件清单" : "查看文件清单", icon: FileDiff, separatorBefore: true },
    { id: "copy", label: "复制储藏编号", icon: Copy, hint: `stash@{${stash.index}}` },
    { id: "drop", label: "删除该储藏…", icon: Trash2, danger: true, separatorBefore: true },
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
    case "files":
      await toggleFiles(stash);
      break;
    case "copy":
      await navigator.clipboard.writeText(`stash@{${stash.index}}`);
      emit("notify", `已复制 stash@{${stash.index}}`);
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
    emit("notify", `读取储藏文件失败: ${err?.message || err}`);
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
  if (!(await confirmAction({ message: `确定从 stash@{${index}} 恢复 ${file.path} 吗？同名文件将被覆盖。`, tone: "danger" }))) return;
  restoringPath.value = file.path;
  try {
    await gitService.applyFileFromStash(props.repoPath, index, file.path);
    emit("notify", `已从储藏恢复并暂存: ${file.path.split("/").pop()}`);
    emit("refresh");
  } catch (err: any) {
    emit("notify", `恢复文件失败: ${err?.message || err}`);
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
          {{ total === 0 ? '储藏架为空。您可点击右上角 "储藏修改" 临时备份工作区未提交内容。' : '没有匹配的储藏。' }}
        </div>

        <div
          v-for="s in visible"
          :key="s.index"
          :aria-label="`储藏 ${s.index}：${s.message || '无说明'}`"
          class="rounded-xl bg-surface/60 border border-line hover:border-line-strong transition"
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
                :title="expandedIndex === s.index ? '收起文件列表' : '查看此储藏包含的文件'"
              >
                <FileCode class="w-3 h-3 text-purple" />
                <span>文件</span>
                <ChevronDown
                  class="w-3 h-3 transition-transform"
                  :class="{ 'rotate-180': expandedIndex === s.index }"
                />
              </button>
              <button
                @click="emit('apply', s.index)"
                class="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-raised hover:bg-hover text-ink text-xs font-medium transition cursor-pointer border border-line"
                title="应用此储藏，保留储藏记录"
              >
                <Play class="w-3 h-3 text-accent" />
                <span>应用</span>
              </button>
              <button
                @click="emit('pop', s.index)"
                class="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-raised hover:bg-hover text-ink text-xs font-medium transition cursor-pointer border border-line"
                title="弹出并删除此储藏"
              >
                <RotateCcw class="w-3 h-3 text-warning" />
                <span>弹出 (Pop)</span>
              </button>
              <button aria-label="丢弃此储藏"
                @click="emit('drop', s.index)"
                class="p-1.5 rounded-lg hover:bg-danger/20 text-ink-muted hover:text-danger transition cursor-pointer"
                title="丢弃此储藏"
              >
                <Trash2 class="w-3.5 h-3.5" />
              </button>
            </div>
          </div>

          <div v-if="expandedIndex === s.index" class="border-t border-line/60 px-3.5 py-2">
            <div v-if="isLoadingFiles" class="flex items-center gap-2 py-2 text-xs text-ink-muted">
              <RefreshCw class="w-3.5 h-3.5 animate-spin" />
              <span>正在读取储藏文件...</span>
            </div>
            <div v-else-if="stashFiles.length === 0" class="py-2 text-xs text-ink-muted">
              该储藏没有可列出的文件。
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
                  title="仅恢复该文件到工作区并暂存"
                >
                  {{ restoringPath === file.path ? "恢复中..." : "恢复" }}
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
          <ArrowDownCircle class="w-3.5 h-3.5" />
          <span>显示更多 (+{{ moreCount }})</span>
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
