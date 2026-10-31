<script setup lang="ts">
import { computed, ref, watch } from "vue";
import {
  FileX,
  Plus,
  Trash2,
  Loader2,
  RefreshCw,
  ShieldAlert,
  Folder,
  Hash,
  Pencil,
  Check,
  X,
  ChevronUp,
  ChevronDown,
} from "lucide-vue-next";
import UiModal from "./ui/UiModal.vue";
import { gitService } from "../services/gitService";
import { t } from "../i18n";

const props = defineProps<{
  isOpen: boolean;
  repoPath: string;
}>();

const emit = defineEmits<{
  (e: "close"): void;
  (e: "notify", msg: string): void;
  (e: "refresh"): void;
}>();

const patterns = ref<string[]>([]);
const isLoading = ref(false);
const busyPattern = ref("");
const newPattern = ref("");
const errorMessage = ref("");
const editingPattern = ref<string | null>(null);
const editDraft = ref("");

/** Last successful mutation, undoable in-place without leaving the panel. */
const undoAction = ref<{ label: string; run: () => Promise<void> } | null>(null);

function setUndo(label: string, run: () => Promise<void>) {
  undoAction.value = { label, run };
}

async function undoLast() {
  const action = undoAction.value;
  if (!action || !props.repoPath || busyPattern.value) return;
  busyPattern.value = "__undo__";
  try {
    await action.run();
    undoAction.value = null;
    emit("notify", t("gitignore.toast.undone", { label: action.label }));
    await loadPatterns();
    emit("refresh");
  } catch (err: any) {
    emit("notify", t("gitignore.toast.undo.failed", { error: err?.message || err }));
  } finally {
    busyPattern.value = "";
  }
}

/** Group rules for readability: directory rules, file/glob rules and comments. */
const groupedPatterns = computed(() => {
  const dirs: string[] = [];
  const globs: string[] = [];
  const comments: string[] = [];
  for (const pattern of patterns.value) {
    if (pattern.startsWith("#")) comments.push(pattern);
    else if (pattern.endsWith("/")) dirs.push(pattern);
    else globs.push(pattern);
  }
  return [
    { title: t("gitignore.group.dirs"), items: dirs, icon: Folder, iconClass: "text-accent", comment: false },
    { title: t("gitignore.group.globs"), items: globs, icon: FileX, iconClass: "text-warning", comment: false },
    { title: t("gitignore.group.comments"), items: comments, icon: Hash, iconClass: "text-ink-muted", comment: true },
  ].filter((group) => group.items.length > 0);
});

async function loadPatterns() {
  if (!props.repoPath) return;
  isLoading.value = true;
  errorMessage.value = "";
  try {
    patterns.value = (await gitService.getGitignorePatterns(props.repoPath)) || [];
  } catch (err: any) {
    errorMessage.value = t("gitignore.error.load", { error: err?.message || err });
  } finally {
    isLoading.value = false;
  }
}

watch(
  () => props.isOpen,
  (open) => {
    if (!open) return;
    newPattern.value = "";
    errorMessage.value = "";
    editingPattern.value = null;
    undoAction.value = null;
    void loadPatterns();
  },
  { immediate: true }
);

watch(
  () => props.repoPath,
  () => {
    if (props.isOpen) void loadPatterns();
  }
);

async function addPattern() {
  const pattern = newPattern.value.trim();
  if (!pattern || !props.repoPath || busyPattern.value) return;
  const existed = patterns.value.includes(pattern);
  busyPattern.value = pattern;
  try {
    await gitService.addGitignorePattern(props.repoPath, pattern);
    if (!existed) {
      setUndo(t("gitignore.undo.add", { pattern }), async () => {
        await gitService.removeGitignorePattern(props.repoPath, pattern);
      });
    }
    emit("notify", t("gitignore.toast.added", { pattern }));
    newPattern.value = "";
    await loadPatterns();
    emit("refresh");
  } catch (err: any) {
    emit("notify", t("gitignore.toast.add.failed", { error: err?.message || err }));
  } finally {
    busyPattern.value = "";
  }
}

async function removePattern(pattern: string) {
  if (!props.repoPath || busyPattern.value) return;
  busyPattern.value = pattern;
  try {
    await gitService.removeGitignorePattern(props.repoPath, pattern);
    setUndo(t("gitignore.undo.remove", { pattern }), async () => {
      await gitService.addGitignorePattern(props.repoPath, pattern);
    });
    emit("notify", t("gitignore.toast.removed", { pattern }));
    if (editingPattern.value === pattern) cancelEdit();
    await loadPatterns();
    emit("refresh");
  } catch (err: any) {
    emit("notify", t("gitignore.toast.remove.failed", { error: err?.message || err }));
  } finally {
    busyPattern.value = "";
  }
}

function startEdit(pattern: string) {
  if (pattern.startsWith("#")) return; // comments are metadata, not editable rules
  editingPattern.value = pattern;
  editDraft.value = pattern;
}

function cancelEdit() {
  editingPattern.value = null;
  editDraft.value = "";
}

/** Swap two rules inside the same display group to change ignore precedence. */
async function movePattern(groupItems: string[], index: number, delta: number) {
  const targetIndex = index + delta;
  if (!props.repoPath || busyPattern.value) return;
  if (targetIndex < 0 || targetIndex >= groupItems.length) return;
  const current = groupItems[index]!;
  const neighbour = groupItems[targetIndex]!;
  busyPattern.value = current;
  try {
    await gitService.swapGitignorePatterns(props.repoPath, current, neighbour);
    setUndo(t("gitignore.undo.swap", { current, neighbour }), async () => {
      await gitService.swapGitignorePatterns(props.repoPath, current, neighbour);
    });
    emit("notify", t("gitignore.toast.swapped", { current, neighbour }));
    if (editingPattern.value === current || editingPattern.value === neighbour) cancelEdit();
    await loadPatterns();
    emit("refresh");
  } catch (err: any) {
    emit("notify", t("gitignore.toast.swap.failed", { error: err?.message || err }));
  } finally {
    busyPattern.value = "";
  }
}

async function saveEdit() {
  const oldPattern = editingPattern.value;
  const next = editDraft.value.trim();
  if (!oldPattern || !props.repoPath || busyPattern.value) return;
  if (!next || next === oldPattern) {
    cancelEdit();
    return;
  }
  busyPattern.value = oldPattern;
  try {
    await gitService.updateGitignorePattern(props.repoPath, oldPattern, next);
    setUndo(t("gitignore.undo.edit", { from: oldPattern, to: next }), async () => {
      await gitService.updateGitignorePattern(props.repoPath, next, oldPattern);
    });
    emit("notify", t("gitignore.toast.updated", { from: oldPattern, to: next }));
    cancelEdit();
    await loadPatterns();
    emit("refresh");
  } catch (err: any) {
    emit("notify", t("gitignore.toast.update.failed", { error: err?.message || err }));
  } finally {
    busyPattern.value = "";
  }
}
</script>

<template>
  <UiModal
    :open="isOpen"
    :title="t('gitignore.title')"
    width="max-w-lg"
    @update:open="!$event && emit('close')"
  >
    <template #header>
      <div class="flex items-center gap-3">
        <div class="w-8 h-8 rounded-xl flex items-center justify-center bg-warning-bg text-warning">
          <FileX class="w-4 h-4" />
        </div>
        <div>
          <h2 class="text-sm font-bold text-ink flex items-center gap-2">
            {{ t("gitignore.title") }}
            <span class="px-1.5 py-0.5 rounded bg-raised border border-line text-[10px] font-mono font-normal text-ink-muted">
              {{ t("gitignore.count", { count: patterns.length }) }}
            </span>
          </h2>
          <p class="text-[11px] text-ink-muted mt-0.5">{{ t("gitignore.subtitle") }}</p>
        </div>
      </div>
    </template>

    <!-- Grouped pattern list -->
    <div class="space-y-2">
        <div v-if="isLoading" class="flex items-center justify-center gap-2 py-10 text-xs text-ink-muted">
          <Loader2 class="w-4 h-4 animate-spin text-accent" /> {{ t("gitignore.loading") }}
        </div>
        <div v-else-if="errorMessage" class="rounded-lg border border-danger/30 bg-danger-bg px-3 py-2 text-[11px] text-danger">
          {{ errorMessage }}
        </div>
        <template v-else>
          <div v-for="group in groupedPatterns" :key="group.title" class="space-y-1.5">
            <div class="flex items-center gap-2 px-1 pt-1">
              <component :is="group.icon" class="w-3 h-3 shrink-0" :class="group.iconClass" />
              <span class="text-[10px] font-semibold text-ink-muted tracking-wider">{{ group.title }}</span>
              <span class="h-px flex-1 bg-line/60"></span>
              <span class="text-[9px] font-mono text-ink-muted">{{ group.items.length }}</span>
            </div>

            <div
              v-for="(pattern, patternIndex) in group.items"
              :key="pattern"
              class="group flex items-center gap-2 px-2.5 py-1.5 rounded-lg border border-line bg-surface hover:bg-raised transition"
            >
              <component
                :is="group.comment ? Hash : FileX"
                class="w-3.5 h-3.5 shrink-0"
                :class="group.iconClass"
              />
              <template v-if="editingPattern === pattern">
                <input
                  v-model="editDraft"
                  type="text"
                  spellcheck="false"
                  class="flex-1 min-w-0 px-2 py-0.5 rounded-md bg-canvas border border-accent text-xs font-mono text-ink focus:outline-none"
                  @keydown.enter.prevent="saveEdit"
                  @keydown.esc.prevent="cancelEdit"
                />
                <button :aria-label="t('gitignore.save')"
                  @click="saveEdit"
                  :disabled="!!busyPattern"
                  class="shrink-0 p-1 rounded text-success hover:bg-success-bg transition cursor-pointer disabled:opacity-40"
                  :title="t('gitignore.save')"
                >
                  <Check class="w-3.5 h-3.5" />
                </button>
                <button :aria-label="t('gitignore.cancel.edit')"
                  @click="cancelEdit"
                  class="shrink-0 p-1 rounded text-ink-muted hover:bg-raised transition cursor-pointer"
                  :title="t('gitignore.cancel.edit')"
                >
                  <X class="w-3.5 h-3.5" />
                </button>
              </template>
              <template v-else>
                <span
                  class="text-xs font-mono truncate"
                  :class="group.comment ? 'text-ink-muted italic' : 'text-ink'"
                  :title="pattern"
                >
                  {{ pattern }}
                </span>
                <div class="ml-auto flex items-center gap-0.5 shrink-0">
                  <template v-if="!group.comment">
                    <button :aria-label="t('gitignore.move.up')"
                      @click="movePattern(group.items, patternIndex, -1)"
                      :disabled="patternIndex === 0 || !!busyPattern"
                      class="p-1 rounded text-ink-muted hover:text-accent hover:bg-accent-bg transition cursor-pointer disabled:opacity-25 disabled:cursor-not-allowed"
                      :title="t('gitignore.move.up')"
                    >
                      <ChevronUp class="w-3.5 h-3.5" />
                    </button>
                    <button :aria-label="t('gitignore.move.down')"
                      @click="movePattern(group.items, patternIndex, 1)"
                      :disabled="patternIndex === group.items.length - 1 || !!busyPattern"
                      class="p-1 rounded text-ink-muted hover:text-accent hover:bg-accent-bg transition cursor-pointer disabled:opacity-25 disabled:cursor-not-allowed"
                      :title="t('gitignore.move.down')"
                    >
                      <ChevronDown class="w-3.5 h-3.5" />
                    </button>
                  </template>
                  <button :aria-label="t('gitignore.edit.hint', { pattern })"
                    v-if="!group.comment"
                    @click="startEdit(pattern)"
                    :disabled="!!busyPattern"
                    class="p-1 rounded text-ink-muted hover:text-accent hover:bg-accent-bg transition cursor-pointer disabled:opacity-40"
                    :title="t('gitignore.edit.hint', { pattern })"
                  >
                    <Pencil class="w-3.5 h-3.5" />
                  </button>
                  <button :aria-label="t('gitignore.remove.hint', { pattern })"
                    @click="removePattern(pattern)"
                    :disabled="!!busyPattern"
                    class="p-1 rounded text-ink-muted hover:text-danger hover:bg-danger-bg transition cursor-pointer disabled:opacity-40"
                    :title="t('gitignore.remove.hint', { pattern })"
                  >
                    <Trash2 v-if="busyPattern !== pattern" class="w-3.5 h-3.5" />
                    <Loader2 v-else class="w-3.5 h-3.5 animate-spin" />
                  </button>
                </div>
              </template>
            </div>
          </div>

          <div
            v-if="patterns.length === 0"
            class="flex flex-col items-center justify-center gap-2 py-10 text-xs text-ink-muted"
          >
            <ShieldAlert class="w-6 h-6 opacity-40" />
            <span>{{ t("gitignore.empty") }}</span>
            <span class="text-[10px] text-ink-muted">{{ t("gitignore.empty.hint") }}</span>
          </div>
        </template>
      </div>

      <!-- Add pattern -->
      <div class="shrink-0 border-t border-line p-3 space-y-2 bg-canvas">
        <div v-if="undoAction" class="flex items-center gap-2 pb-0.5">
          <span class="text-[10px] text-ink-muted truncate">{{ t("gitignore.undo.last", { label: undoAction.label }) }}</span>
          <button
            @click="undoLast"
            :disabled="!!busyPattern"
            class="ml-auto shrink-0 flex items-center gap-1 px-2 py-0.5 rounded-md border border-line bg-surface hover:bg-raised text-[10px] font-semibold text-ink-secondary hover:text-ink transition cursor-pointer disabled:opacity-40"
            :title="t('gitignore.undo.hint')"
          >
            <RefreshCw class="w-3 h-3" /> {{ t("gitignore.undo") }}
          </button>
        </div>
        <div class="flex items-center gap-2">
          <input
            v-model="newPattern"
            type="text"
            spellcheck="false"
            :aria-label="t('gitignore.placeholder')"
            :placeholder="t('gitignore.placeholder')"
            class="flex-1 min-w-0 px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs font-mono text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
            @keydown.enter.prevent="addPattern"
          />
          <button
            @click="addPattern"
            :disabled="!newPattern.trim() || !!busyPattern"
            class="flex items-center gap-1 px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-xs font-semibold transition cursor-pointer disabled:opacity-40"
          >
            <Plus class="w-3.5 h-3.5" /> {{ t("gitignore.add") }}
          </button>
          <button :aria-label="t('gitignore.refresh')"
            @click="loadPatterns"
            :disabled="isLoading"
            class="p-1.5 rounded-lg border border-line bg-surface hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer disabled:opacity-40"
            :title="t('gitignore.refresh')"
          >
            <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isLoading }" />
          </button>
        </div>
        <p class="text-[10px] text-ink-muted">
          {{ t("gitignore.footnote") }}
        </p>
    </div>
  </UiModal>
</template>
