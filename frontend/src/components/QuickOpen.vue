<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted, onUnmounted } from "vue";
import { Search, FileText, FileCode2, Loader2 } from "lucide-vue-next";
import UiModal from "./ui/UiModal.vue";
import { gitService } from "../services/gitService";
import type { ProjectFileMatch } from "../services/gitService";
import type { GrepMatchItem } from "../types";
import { t } from "../i18n";

/** 快速打开 / 全仓库代码搜索：大型项目导航（⌘P / ⌘⇧F）。 */
const props = defineProps<{
  currentRepoPath: string;
}>();

type Mode = "files" | "code";
type QuickOpenDetail = { mode?: Mode } | undefined;

const FILE_LIMIT = 80;
const CODE_LIMIT = 200;
const FILES_DEBOUNCE_MS = 200;
const CODE_DEBOUNCE_MS = 300;

const isOpen = ref(false);
const mode = ref<Mode>("files");
const query = ref("");
const ignoreCase = ref(true);
const files = ref<ProjectFileMatch[]>([]);
const codeMatches = ref<GrepMatchItem[]>([]);
const selectedIndex = ref(0);
const isLoading = ref(false);
const inputRef = ref<HTMLInputElement | null>(null);
const listRef = ref<HTMLElement | null>(null);

let debounceTimer: ReturnType<typeof setTimeout> | null = null;
let requestSeq = 0;

const hasRepo = computed(() => Boolean(props.currentRepoPath));
const resultsCount = computed(() => (mode.value === "files" ? files.value.length : codeMatches.value.length));

const emptyHint = computed(() => {
  if (!hasRepo.value) return t("quickopen.hint.no.repo");
  if (mode.value === "files") {
    return query.value.trim() ? t("quickopen.hint.no.match") : t("quickopen.hint.no.files");
  }
  if (!query.value.trim()) return t("quickopen.hint.need.query");
  return t("quickopen.hint.no.match");
});

function clearDebounce() {
  if (debounceTimer !== null) {
    clearTimeout(debounceTimer);
    debounceTimer = null;
  }
}

function focusInput() {
  void nextTick(() => inputRef.value?.focus());
}

function scrollSelectedIntoView() {
  const el = listRef.value?.querySelector<HTMLElement>(`[data-idx="${selectedIndex.value}"]`);
  el?.scrollIntoView({ block: "nearest" });
}

function scheduleSearch(delay: number) {
  clearDebounce();
  if (!isOpen.value) return;
  debounceTimer = setTimeout(() => {
    debounceTimer = null;
    void runSearch();
  }, delay);
}

async function runSearch() {
  const repoPath = props.currentRepoPath;
  const seq = ++requestSeq;

  if (!repoPath) {
    files.value = [];
    codeMatches.value = [];
    isLoading.value = false;
    return;
  }

  // `git grep` with an empty pattern would be catastrophic on a huge repo.
  if (mode.value === "code" && !query.value.trim()) {
    codeMatches.value = [];
    isLoading.value = false;
    return;
  }

  isLoading.value = true;
  try {
    if (mode.value === "files") {
      const res = await gitService.searchProjectFiles(repoPath, query.value.trim(), FILE_LIMIT);
      if (seq !== requestSeq) return;
      files.value = Array.isArray(res) ? res.slice(0, FILE_LIMIT) : [];
    } else {
      const res = await gitService.searchRepoCode(
        repoPath,
        query.value.trim(),
        ignoreCase.value,
        CODE_LIMIT
      );
      if (seq !== requestSeq) return;
      codeMatches.value = Array.isArray(res) ? res.slice(0, CODE_LIMIT) : [];
    }
    selectedIndex.value = 0;
  } catch {
    // 后端失败时静默降级为空结果，不打断输入。
    if (seq === requestSeq) {
      files.value = [];
      codeMatches.value = [];
    }
  } finally {
    if (seq === requestSeq) {
      isLoading.value = false;
      void nextTick(scrollSelectedIntoView);
    }
  }
}

function open(nextMode: Mode = "files") {
  if (isOpen.value && mode.value === nextMode) {
    focusInput();
    return;
  }

  mode.value = nextMode;
  query.value = "";
  ignoreCase.value = true;
  files.value = [];
  codeMatches.value = [];
  selectedIndex.value = 0;
  isLoading.value = false;
  isOpen.value = true;
  clearDebounce();
  requestSeq++;
  void runSearch();
  focusInput();
}

function close() {
  isOpen.value = false;
  clearDebounce();
  requestSeq++;
  isLoading.value = false;
}

function switchMode(nextMode: Mode) {
  if (mode.value === nextMode) {
    focusInput();
    return;
  }
  mode.value = nextMode;
  selectedIndex.value = 0;
  clearDebounce();
  requestSeq++;
  isLoading.value = false;
  if (nextMode === "files" || query.value.trim()) {
    void runSearch();
  } else {
    codeMatches.value = [];
  }
  focusInput();
}

function handleQueryInput(e: Event) {
  const value = (e.target as HTMLInputElement).value;
  query.value = value;
  selectedIndex.value = 0;
  scheduleSearch(mode.value === "files" ? FILES_DEBOUNCE_MS : CODE_DEBOUNCE_MS);
}

function toggleIgnoreCase() {
  ignoreCase.value = !ignoreCase.value;
  if (mode.value === "code" && query.value.trim()) scheduleSearch(0);
  focusInput();
}

function moveSelection(delta: number) {
  const len = resultsCount.value;
  if (len === 0) return;
  selectedIndex.value = (selectedIndex.value + delta + len) % len;
  void nextTick(scrollSelectedIntoView);
}

function selectFile(path: string, line?: number) {
  window.dispatchEvent(new CustomEvent("wgit:open-file", { detail: { path, line } }));
  close();
}

function selectCurrent() {
  if (mode.value === "files") {
    const file = files.value[selectedIndex.value];
    if (file) selectFile(file.path);
  } else {
    const match = codeMatches.value[selectedIndex.value];
    if (match) selectFile(match.path, match.lineNumber);
  }
}

import { isActionCustomized } from "../utils/shortcuts";

function handleGlobalKeydown(e: KeyboardEvent) {
  const mod = e.metaKey || e.ctrlKey;
  const key = e.key.toLowerCase();

  if (mod && !e.altKey && !e.shiftKey && key === "p" && !isActionCustomized("quick-open-files")) {
    e.preventDefault();
    open("files");
    return;
  }
  if (mod && !e.altKey && e.shiftKey && key === "f" && !isActionCustomized("code-search")) {
    e.preventDefault();
    open("code");
    return;
  }
  if (!isOpen.value) return;
  if (e.isComposing) return;

  if (e.key === "Escape") {
    e.preventDefault();
    close();
    return;
  }
  if (e.key === "ArrowDown") {
    e.preventDefault();
    moveSelection(1);
  } else if (e.key === "ArrowUp") {
    e.preventDefault();
    moveSelection(-1);
  } else if (e.key === "Enter") {
    e.preventDefault();
    selectCurrent();
  }
}

function handleQuickOpenEvent(e: Event) {
  const detail = (e as CustomEvent<QuickOpenDetail>).detail;
  open(detail?.mode === "code" ? "code" : "files");
}

watch(
  () => props.currentRepoPath,
  () => {
    if (isOpen.value) {
      selectedIndex.value = 0;
      scheduleSearch(0);
    }
  }
);

onMounted(() => {
  window.addEventListener("keydown", handleGlobalKeydown);
  window.addEventListener("wgit:quick-open", handleQuickOpenEvent as EventListener);
});

onUnmounted(() => {
  window.removeEventListener("keydown", handleGlobalKeydown);
  window.removeEventListener("wgit:quick-open", handleQuickOpenEvent as EventListener);
  clearDebounce();
});
</script>

<template>
  <UiModal
    :open="isOpen"
    variant="bare"
    align="top"
    width="max-w-[640px]"
    :label="t('quickopen.title')"
    panel-class="select-none animate-in fade-in duration-150"
    close-on-backdrop
    @update:open="(open) => { if (!open) close(); }"
  >
      <!-- Mode Tabs -->
      <div class="flex items-center gap-1 px-2 pt-2">
        <button
          type="button"
          @click="switchMode('files')"
          class="flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg text-[11px] border transition-colors"
          :class="
            mode === 'files'
              ? 'bg-raised text-accent border-line font-medium'
              : 'text-ink-muted border-transparent hover:text-ink-secondary hover:bg-raised/50'
          "
        >
          <FileText class="w-3.5 h-3.5" />
          <span>{{ t("quickopen.mode.files") }}</span>
          <kbd
            class="px-1 py-0.5 rounded bg-canvas/60 border border-line text-[9px] text-ink-muted font-mono"
          >
            ⌘P
          </kbd>
        </button>
        <button
          type="button"
          @click="switchMode('code')"
          class="flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg text-[11px] border transition-colors"
          :class="
            mode === 'code'
              ? 'bg-raised text-accent border-line font-medium'
              : 'text-ink-muted border-transparent hover:text-ink-secondary hover:bg-raised/50'
          "
        >
          <FileCode2 class="w-3.5 h-3.5" />
          <span>{{ t("quickopen.mode.code") }}</span>
          <kbd
            class="px-1 py-0.5 rounded bg-canvas/60 border border-line text-[9px] text-ink-muted font-mono"
          >
            ⌘⇧F
          </kbd>
        </button>
        <span class="ml-auto pr-1 text-[10px] text-ink-muted font-mono">
          {{ mode === "files" ? t("quickopen.badge.files") : t("quickopen.badge.code") }}
        </span>
      </div>

      <!-- Search Input -->
      <div class="p-3 border-b border-line flex items-center gap-2.5">
        <Search class="w-4 h-4 text-ink-muted ml-1 shrink-0" />
        <input
          ref="inputRef"
          :value="query"
          @input="handleQueryInput"
          type="text"
          role="combobox"
          aria-autocomplete="list"
          aria-expanded="true"
          :aria-controls="'quickopen-listbox'"
          :aria-activedescendant="'quickopen-item-' + selectedIndex"
          spellcheck="false"
          autocomplete="off"
          :placeholder="
            mode === 'files' ? t('quickopen.placeholder.files') : t('quickopen.placeholder.code')
          "
          autofocus
          class="flex-1 min-w-0 bg-transparent border-none text-sm text-ink placeholder-ink-muted focus:outline-none font-sans"
        />
        <Loader2 v-if="isLoading" class="w-3.5 h-3.5 text-ink-muted animate-spin shrink-0" />
        <span
          v-if="mode === 'code' && !isLoading"
          class="text-[10px] text-ink-muted font-mono whitespace-nowrap shrink-0"
        >
          {{ t("quickopen.results", { count: resultsCount }) }}
        </span>
        <button
          v-if="mode === 'code'"
          type="button"
          @click="toggleIgnoreCase"
          :title="ignoreCase ? t('quickopen.case.insensitive.hint') : t('quickopen.case.sensitive.hint')"
          class="shrink-0 px-1.5 py-0.5 rounded text-[10px] font-mono border transition-colors"
          :class="
            ignoreCase
              ? 'bg-accent-bg text-accent border-accent-border'
              : 'bg-raised text-ink-muted border-line hover:text-ink-secondary'
          "
        >
          Aa
        </button>
        <kbd
          class="px-1.5 py-0.5 rounded bg-raised border border-line text-[10px] text-ink-muted font-mono shrink-0"
        >
          ESC
        </kbd>
      </div>

      <!-- Results -->
      <div
        id="quickopen-listbox"
        ref="listRef"
        role="listbox"
        class="max-h-[420px] min-h-[88px] overflow-y-auto p-2 space-y-0.5"
      >
        <template v-if="mode === 'files'">
          <div
            v-for="(file, idx) in files"
            :key="file.path"
            :id="'quickopen-item-' + idx"
            role="option"
            :aria-selected="selectedIndex === idx"
            :data-idx="idx"
            @click="selectFile(file.path)"
            @mouseenter="selectedIndex = idx"
            class="flex items-baseline gap-2.5 px-3 py-2 rounded-xl transition cursor-pointer min-w-0"
            :class="
              selectedIndex === idx
                ? 'bg-raised text-accent font-medium shadow-xs'
                : 'text-ink-secondary hover:text-ink'
            "
          >
            <span class="text-xs font-semibold truncate shrink-0 max-w-[45%]">{{ file.name }}</span>
            <span class="text-[11px] text-ink-muted truncate flex-1 min-w-0">{{
              file.dir || t("quickopen.root")
            }}</span>
            <span
              v-if="file.isBinary"
              class="shrink-0 text-[9px] font-mono text-ink-muted px-1.5 py-0.5 rounded bg-canvas/60 border border-line"
            >
              BIN
            </span>
          </div>
        </template>

        <template v-else>
          <div
            v-for="(match, idx) in codeMatches"
            :key="`${match.path}:${match.lineNumber}:${idx}`"
            :id="'quickopen-item-' + idx"
            role="option"
            :aria-selected="selectedIndex === idx"
            :data-idx="idx"
            @click="selectFile(match.path, match.lineNumber)"
            @mouseenter="selectedIndex = idx"
            class="px-3 py-1.5 rounded-xl transition cursor-pointer flex flex-col gap-0.5 min-w-0"
            :class="
              selectedIndex === idx
                ? 'bg-raised text-accent font-medium shadow-xs'
                : 'text-ink-secondary hover:text-ink'
            "
          >
            <span class="text-[11px] font-mono text-accent truncate">{{ match.path }}:{{ match.lineNumber }}</span>
            <div class="min-w-0 truncate">
              <span class="whitespace-pre font-mono text-[11px] text-ink-secondary">{{ match.lineContent }}</span>
            </div>
          </div>
        </template>

        <div v-if="!isLoading && resultsCount === 0" class="py-8 text-center text-xs text-ink-muted">
          {{ emptyHint }}
        </div>
        <div v-else-if="isLoading && resultsCount === 0" class="py-8 text-center text-xs text-ink-muted">
          {{ t("quickopen.searching") }}
        </div>
      </div>

      <!-- Bottom Hint Bar -->
      <div
        class="p-2 border-t border-line bg-canvas/30 text-[10px] text-ink-muted flex items-center justify-between px-3 font-mono"
      >
        <span>{{ t("quickopen.hints") }}</span>
        <span>{{
          mode === "files"
            ? t("quickopen.footer.files")
            : ignoreCase
              ? t("quickopen.footer.insensitive")
              : t("quickopen.footer.sensitive")
        }}</span>
      </div>
  </UiModal>
</template>
