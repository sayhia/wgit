<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from "vue";
import { AlertTriangle, Binary, Check, Copy, FileCode, WrapText, ChevronUp, ChevronDown, ScanEye, Sparkles, Code2, X } from "lucide-vue-next";
import { useToast } from "../../composables/useToast";
import Toast from "../ui/Toast.vue";
import { t } from "../../i18n";
import {
  parseFileModeChange,
  isBinaryDiffText,
  extractHunkText,
  cleanSelectedLineText,
  type FileModeChangeInfo,
} from "../../utils/diffViewerUtils";

const props = defineProps<{
  diffText: string;
  mode: "unified" | "split" | "word";
  canHunkAction?: boolean;
  hunkActionBusy?: boolean;
  canLineAction?: boolean;
  canCompareAsset?: boolean;
}>();

const emit = defineEmits<{
  (e: "stage-hunk", patch: string): void;
  (e: "discard-hunk", patch: string): void;
  (e: "stage-lines", patch: string): void;
  (e: "discard-lines", patch: string): void;
  (e: "resolve-conflict-block", blockIndex: number, choice: "ours" | "theirs" | "both"): void;
  (e: "open-ai-advisor"): void;
  (e: "open-asset-diff"): void;
}>();

// ---- Large diff guard: 超过 MAX_DIFF_LINES 时默认只解析/渲染前 MAX_DIFF_LINES 行 ----
// 折叠仅限制解析与渲染量，统计信息仍基于完整 diff 文本；图片/二进制/空状态不受影响。
const MAX_DIFF_LINES = 4000;
const showFullDiff = ref(false);

const diffRawLines = computed<string[]>(() =>
  props.diffText ? props.diffText.split("\n") : []
);
const diffTotalLineCount = computed(() => diffRawLines.value.length);
const isLargeDiff = computed(() => diffTotalLineCount.value > MAX_DIFF_LINES);
const isDiffFolded = computed(() => isLargeDiff.value && !showFullDiff.value);

const diffLinesToParse = computed<string[]>(() =>
  isDiffFolded.value ? diffRawLines.value.slice(0, MAX_DIFF_LINES) : diffRawLines.value
);

interface DiffHunk {
  startLine: number;
  patch: string;
}

// A hunk patch needs the full file header plus one @@ block. Plain modified
// files only: new/deleted/renamed/binary diffs cannot be applied hunk-wise
// through the backend's `git apply` path.
const diffHunks = computed<DiffHunk[]>(() => {
  const lines = diffRawLines.value;
  const firstHunk = lines.findIndex((text) => text.startsWith("@@"));
  if (firstHunk < 0) return [];

  const header = lines.slice(0, firstHunk);
  if (!header[0]?.startsWith("diff --git ")) return [];
  const hasOldPath = header.some((l) => l.startsWith("--- a/") || l.startsWith('--- "a/'));
  const hasNewPath = header.some((l) => l.startsWith("+++ b/") || l.startsWith('+++ "b/'));
  const nonPlain = header.some((l) =>
    /^(new file mode|deleted file mode|rename from|rename to|Binary files)/.test(l)
  );
  if (!hasOldPath || !hasNewPath || nonPlain) return [];

  const headerText = header.join("\n");
  const hunks: DiffHunk[] = [];
  let start = firstHunk;
  for (let i = firstHunk + 1; i <= lines.length; i++) {
    if (i === lines.length || lines[i]!.startsWith("@@")) {
      // git apply rejects a patch whose last line has no terminating newline.
      const body = lines.slice(start, i).join("\n");
      hunks.push({
        startLine: start,
        patch: `${headerText}\n${body.endsWith("\n") ? body : `${body}\n`}`,
      });
      start = i;
    }
  }
  return hunks;
});

interface LinePatchEntry {
  kind: "add" | "del";
  oldPos: number;
  newPos: number;
  text: string;
}

// Zero-context patch sources for per-line staging. Positions are captured from
// the diff stream, then rendered one hunk per selected line.
const linePatchData = computed(() => {
  const lines = diffRawLines.value;
  const firstHunk = lines.findIndex((text) => text.startsWith("@@"));
  const empty = { header: "", entries: new Map<number, LinePatchEntry>() };
  if (firstHunk < 0) return empty;

  const headerLines = lines.slice(0, firstHunk);
  if (!headerLines[0]?.startsWith("diff --git ")) return empty;
  const hasOldPath = headerLines.some((l) => l.startsWith("--- a/") || l.startsWith('--- "a/'));
  const hasNewPath = headerLines.some((l) => l.startsWith("+++ b/") || l.startsWith('+++ "b/'));
  const nonPlain = headerLines.some((l) =>
    /^(new file mode|deleted file mode|rename from|rename to|Binary files)/.test(l)
  );
  if (!hasOldPath || !hasNewPath || nonPlain) return empty;

  const entries = new Map<number, LinePatchEntry>();
  let oldNo = 0;
  let newNo = 0;
  for (let i = firstHunk; i < lines.length; i++) {
    const text = lines[i]!;
    const headerMatch = text.match(/@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@/);
    if (headerMatch) {
      oldNo = parseInt(headerMatch[1]!, 10);
      newNo = parseInt(headerMatch[2]!, 10);
      continue;
    }
    if (!text || text.startsWith("\\")) continue;
    if (text.startsWith("+")) {
      entries.set(i, { kind: "add", oldPos: oldNo, newPos: newNo, text });
      newNo++;
    } else if (text.startsWith("-")) {
      entries.set(i, { kind: "del", oldPos: oldNo, newPos: newNo, text });
      oldNo++;
    } else {
      oldNo++;
      newNo++;
    }
  }
  return { header: headerLines.join("\n"), entries };
});

const selectedLineIndexes = ref<Set<number>>(new Set());

const canApplyLineActions = computed(
  () =>
    props.canLineAction === true &&
    props.mode === "unified" &&
    !isDiffFolded.value &&
    linePatchData.value.entries.size > 0
);

const selectedLineCount = computed(() => {
  let count = 0;
  for (const idx of selectedLineIndexes.value) {
    if (linePatchData.value.entries.has(idx)) count++;
  }
  return count;
});

function toggleLineSelection(index: number) {
  const next = new Set(selectedLineIndexes.value);
  if (next.has(index)) next.delete(index);
  else next.add(index);
  selectedLineIndexes.value = next;
}

function clearLineSelection() {
  selectedLineIndexes.value = new Set();
}

function buildSelectedLinePatch(): string {
  const { header, entries } = linePatchData.value;
  const selected = [...selectedLineIndexes.value]
    .filter((idx) => entries.has(idx))
    .sort((a, b) => a - b);
  let body = "";
  for (const idx of selected) {
    const entry = entries.get(idx)!;
    body +=
      entry.kind === "del"
        ? `@@ -${entry.oldPos},1 +${entry.newPos},0 @@\n${entry.text}\n`
        : `@@ -${entry.oldPos},0 +${entry.newPos},1 @@\n${entry.text}\n`;
  }
  return body ? `${header}\n${body}` : "";
}

function emitLineAction(action: "stage" | "discard") {
  const patch = buildSelectedLinePatch();
  if (!patch) return;
  if (action === "stage") emit("stage-lines", patch);
  else emit("discard-lines", patch);
}

watch(
  () => props.diffText,
  () => clearLineSelection()
);
watch(
  () => [props.mode, props.canLineAction, isDiffFolded.value] as const,
  () => {
    if (!canApplyLineActions.value) clearLineSelection();
  }
);

const canApplyHunks = computed(
  () => props.canHunkAction === true && !isDiffFolded.value && diffHunks.value.length > 0
);

function hunkIndexForLine(lineIndex: number): number {
  for (let i = diffHunks.value.length - 1; i >= 0; i--) {
    if (lineIndex >= diffHunks.value[i]!.startLine) return i;
  }
  return -1;
}

function emitHunk(action: "stage-hunk" | "discard-hunk", hunkIndex: number | undefined) {
  if (hunkIndex === undefined || hunkIndex < 0) return;
  const hunk = diffHunks.value[hunkIndex];
  if (!hunk) return;
  if (action === "stage-hunk") emit("stage-hunk", hunk.patch);
  else emit("discard-hunk", hunk.patch);
}

const { toastText, showToast, notify } = useToast(2000);

// File mode changes detection (old/new mode, new file mode, deleted file mode)
const fileModeChange = computed<FileModeChangeInfo | null>(() => {
  return parseFileModeChange(props.diffText);
});

// Diff Patch Playground
const showPatchPlayground = ref(false);
const copiedPatch = ref(false);

async function copyFullPatch() {
  if (!props.diffText) return;
  try {
    await navigator.clipboard.writeText(props.diffText);
    copiedPatch.value = true;
    setTimeout(() => {
      copiedPatch.value = false;
    }, 2000);
    notify(t("workspace.diff.playground.toast.copied"));
  } catch {
    /* ignore */
  }
}

// Binary diff detection
const isBinaryDiff = computed(() => {
  return isBinaryDiffText(props.diffText);
});

// Split mode line wrap toggle
const isSplitWrap = ref(false);

// Clipboard helpers & feedback
const copiedHunkLine = ref<number | null>(null);
const copiedLines = ref(false);

async function copyToClipboard(text: string): Promise<boolean> {
  if (!text) return false;
  if (typeof navigator !== "undefined" && navigator?.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(text);
      return true;
    } catch {
      // fallback to execCommand below
    }
  }
  try {
    if (typeof document !== "undefined") {
      const textarea = document.createElement("textarea");
      textarea.value = text;
      textarea.style.position = "fixed";
      textarea.style.left = "-9999px";
      textarea.style.top = "-9999px";
      textarea.style.opacity = "0";
      document.body.appendChild(textarea);
      textarea.focus();
      textarea.select();
      const success = document.execCommand("copy");
      document.body.removeChild(textarea);
      return success;
    }
  } catch {
    // fallback
  }
  return false;
}

async function copySelectedLines() {
  const { entries } = linePatchData.value;
  const selected = [...selectedLineIndexes.value]
    .filter((idx) => entries.has(idx))
    .sort((a, b) => a - b);
  if (selected.length === 0) return;
  const content = selected
    .map((idx) => cleanSelectedLineText(entries.get(idx)!.text))
    .join("\n");
  const ok = await copyToClipboard(content);
  if (ok) {
    copiedLines.value = true;
    notify(t("workspace.diff.copy.lines.done"));
    setTimeout(() => {
      copiedLines.value = false;
    }, 1500);
  }
}

function getHunkText(startLineIdx: number): string {
  return extractHunkText(diffRawLines.value, startLineIdx);
}

async function copyHunk(lineIndex: number | undefined) {
  if (lineIndex === undefined || lineIndex < 0) return;
  const text = getHunkText(lineIndex);
  if (!text) return;
  const ok = await copyToClipboard(text);
  if (ok) {
    copiedHunkLine.value = lineIndex;
    notify(t("workspace.diff.copy.hunk.done"));
    setTimeout(() => {
      if (copiedHunkLine.value === lineIndex) {
        copiedHunkLine.value = null;
      }
    }, 1500);
  }
}

// 差异统计: 完整 diff 的行数与 +新增/-删除 行数（按 unified diff 前缀统计）
const diffStats = computed(() => {
  let additions = 0;
  let deletions = 0;
  for (const text of diffRawLines.value) {
    if (text.startsWith("+") && !text.startsWith("+++")) additions++;
    else if (text.startsWith("-") && !text.startsWith("---")) deletions++;
  }
  return { additions, deletions };
});

// 切换文件 / 仓库 / diff 内容时自动恢复折叠状态
watch(
  [() => props.diffText],
  () => {
    showFullDiff.value = false;
  }
);

// 1. Unified Diff Lines Parser
const totalConflictBlocks = computed(() => {
  if (!props.diffText) return 0;
  return (props.diffText.match(/^<{7}/gm) || []).length;
});
const activeConflictIndex = ref(0);

function scrollToConflict(index: number) {
  if (index < 0 || index >= totalConflictBlocks.value) return;
  activeConflictIndex.value = index;
  const el = document.getElementById(`conflict-block-${index}`);
  if (el) {
    el.scrollIntoView({ behavior: "smooth", block: "center" });
  }
}

const parsedUnifiedDiffLines = computed(() => {
  if (diffLinesToParse.value.length === 0) return [];
  const lines = diffLinesToParse.value;
  let oldLineNo = 0;
  let newLineNo = 0;
  let blockCounter = 0;

  return lines.map((text, idx) => {
    let type: "header" | "add" | "del" | "ctx" | "conflict-marker" = "ctx";
    let blockIndex: number | undefined = undefined;
    if (text.startsWith("<<<<<<<") || text.startsWith("=======") || text.startsWith(">>>>>>>")) {
      type = "conflict-marker";
      if (text.startsWith("<<<<<<<")) {
        blockIndex = blockCounter++;
      }
      oldLineNo++;
      newLineNo++;
    } else if (text.startsWith("@@")) {
      type = "header";
      const match = text.match(/@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@/);
      if (match && match[1] && match[2]) {
        oldLineNo = parseInt(match[1], 10);
        newLineNo = parseInt(match[2], 10);
      }
    } else if (text.startsWith("+") && !text.startsWith("+++")) {
      type = "add";
      newLineNo++;
    } else if (text.startsWith("-") && !text.startsWith("---")) {
      type = "del";
      oldLineNo++;
    } else if (!text.startsWith("---") && !text.startsWith("+++") && !text.startsWith("diff ")) {
      type = "ctx";
      oldLineNo++;
      newLineNo++;
    }
    return {
      index: idx,
      text,
      type,
      conflictBlockIndex: blockIndex,
      oldLineNo: type === "add" ? "" : oldLineNo,
      newLineNo: type === "del" ? "" : newLineNo,
    };
  });
});

// 2. Split (Side-by-Side) Diff Rows Parser
interface SplitRow {
  index: number;
  isHeader: boolean;
  headerText?: string;
  hunkIndex?: number;
  rawLineIndex?: number;
  left: { lineNo: number | ""; text: string; type: "del" | "ctx" | "empty" };
  right: { lineNo: number | ""; text: string; type: "add" | "ctx" | "empty" };
}

const parsedSplitDiffRows = computed<SplitRow[]>(() => {
  if (diffLinesToParse.value.length === 0) return [];
  const rawLines = diffLinesToParse.value;
  const rows: SplitRow[] = [];

  let oldNo = 0;
  let newNo = 0;
  let hunkNo = -1;
  let pendingDels: { lineNo: number; text: string }[] = [];
  let pendingAdds: { lineNo: number; text: string }[] = [];

  function flushPending() {
    const maxLen = Math.max(pendingDels.length, pendingAdds.length);
    for (let i = 0; i < maxLen; i++) {
      const del = pendingDels[i];
      const add = pendingAdds[i];
      rows.push({
        index: rows.length,
        isHeader: false,
        left: del
          ? { lineNo: del.lineNo, text: del.text, type: "del" }
          : { lineNo: "", text: "", type: "empty" },
        right: add
          ? { lineNo: add.lineNo, text: add.text, type: "add" }
          : { lineNo: "", text: "", type: "empty" },
      });
    }
    pendingDels = [];
    pendingAdds = [];
  }

  for (let rawIdx = 0; rawIdx < rawLines.length; rawIdx++) {
    const text = rawLines[rawIdx]!;
    if (text.startsWith("@@")) {
      flushPending();
      hunkNo++;
      const match = text.match(/@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@/);
      if (match && match[1] && match[2]) {
        oldNo = parseInt(match[1], 10);
        newNo = parseInt(match[2], 10);
      }
      rows.push({
        index: rows.length,
        isHeader: true,
        headerText: text,
        hunkIndex: hunkNo,
        rawLineIndex: rawIdx,
        left: { lineNo: "", text: "", type: "empty" },
        right: { lineNo: "", text: "", type: "empty" },
      });
    } else if (text.startsWith("+") && !text.startsWith("+++")) {
      pendingAdds.push({ lineNo: newNo, text: text.substring(1) });
      newNo++;
    } else if (text.startsWith("-") && !text.startsWith("---")) {
      pendingDels.push({ lineNo: oldNo, text: text.substring(1) });
      oldNo++;
    } else if (text.startsWith("diff ") || text.startsWith("index ") || text.startsWith("---") || text.startsWith("+++")) {
      // Skip raw header meta
      flushPending();
    } else {
      flushPending();
      const ctxText = text.startsWith(" ") ? text.substring(1) : text;
      rows.push({
        index: rows.length,
        isHeader: false,
        left: { lineNo: oldNo, text: ctxText, type: "ctx" },
        right: { lineNo: newNo, text: ctxText, type: "ctx" },
      });
      oldNo++;
      newNo++;
    }
  }
  flushPending();
  return rows;
});

// 3. Word-level diff (`git diff --word-diff=plain`): inline [-removed-] / {+added+}.
interface WordToken {
  kind: "plain" | "del" | "add";
  text: string;
}

function tokenizeWordLine(text: string): WordToken[] {
  const tokens: WordToken[] = [];
  const re = /\[-([\s\S]*?)-\]|\{\+([\s\S]*?)\+\}/g;
  let last = 0;
  let match: RegExpExecArray | null;
  while ((match = re.exec(text)) !== null) {
    if (match.index > last) {
      tokens.push({ kind: "plain", text: text.slice(last, match.index) });
    }
    if (match[1] !== undefined) tokens.push({ kind: "del", text: match[1] });
    else tokens.push({ kind: "add", text: match[2] ?? "" });
    last = re.lastIndex;
  }
  if (last < text.length) tokens.push({ kind: "plain", text: text.slice(last) });
  return tokens;
}

const parsedWordDiffLines = computed(() => {
  return diffLinesToParse.value.map((text, idx) => {
    let type: "meta" | "hunk" | "content" = "content";
    if (text.startsWith("diff ") || text.startsWith("index ") || text.startsWith("--- ") || text.startsWith("+++ ")) {
      type = "meta";
    } else if (text.startsWith("@@")) {
      type = "hunk";
    }
    return {
      index: idx,
      type,
      text,
      tokens: type === "content" ? tokenizeWordLine(text) : [],
    };
  });
});
const rootEl = ref<HTMLDivElement | null>(null);
const diffScrollTop = ref(0);
const diffViewportHeight = ref(800);
const DIFF_LINE_HEIGHT = 20;

function getScrollParent(node: HTMLElement | null): HTMLElement | Window {
  if (!node || typeof window === "undefined") return window;
  let parent = node.parentElement;
  while (parent) {
    const { overflowY } = window.getComputedStyle(parent);
    if (overflowY === "auto" || overflowY === "scroll") return parent;
    parent = parent.parentElement;
  }
  return window;
}

function updateScrollPosition() {
  const parent = getScrollParent(rootEl.value);
  if (parent === window) {
    diffScrollTop.value = window.scrollY;
    diffViewportHeight.value = window.innerHeight;
  } else {
    const el = parent as HTMLElement;
    diffScrollTop.value = el.scrollTop;
    diffViewportHeight.value = el.clientHeight || 800;
  }
}

const isVirtualizedDiff = computed(() => parsedUnifiedDiffLines.value.length > 800);

const visibleUnifiedWindow = computed(() => {
  const allLines = parsedUnifiedDiffLines.value;
  const total = allLines.length;
  if (!isVirtualizedDiff.value) {
    return {
      lines: allLines,
      topPadding: 0,
      bottomPadding: 0,
    };
  }
  const start = Math.max(0, Math.floor(diffScrollTop.value / DIFF_LINE_HEIGHT) - 50);
  const end = Math.min(total, start + Math.ceil(diffViewportHeight.value / DIFF_LINE_HEIGHT) + 100);
  const lines = allLines.slice(start, end);
  const topPadding = start * DIFF_LINE_HEIGHT;
  const bottomPadding = Math.max(0, (total - end) * DIFF_LINE_HEIGHT);
  return { lines, topPadding, bottomPadding };
});

let scrollTarget: HTMLElement | Window | null = null;

onMounted(() => {
  scrollTarget = getScrollParent(rootEl.value);
  scrollTarget.addEventListener("scroll", updateScrollPosition, { passive: true });
  updateScrollPosition();
});

onUnmounted(() => {
  if (scrollTarget) {
    scrollTarget.removeEventListener("scroll", updateScrollPosition);
    scrollTarget = null;
  }
});
</script>

<template>
  <div ref="rootEl" class="diff-viewer-root">
    <!-- File Mode Changes Detection Badge -->
    <div
      v-if="fileModeChange"
      class="mx-2 mt-2 px-3 py-1.5 rounded-lg border border-accent/40 bg-accent/10 flex items-center gap-2 text-[11px] select-none shrink-0"
    >
      <FileCode class="w-3.5 h-3.5 text-accent shrink-0" />
      <span class="font-medium text-accent">{{ fileModeChange.text }}</span>
    </div>

    <!-- Binary Files Detection Banner -->
    <div
      v-if="isBinaryDiff"
      class="m-4 p-4 rounded-xl border border-line bg-surface/60 flex items-center justify-between gap-3 select-none"
    >
      <div class="flex items-start gap-3 min-w-0">
        <Binary class="w-5 h-5 text-accent shrink-0 mt-0.5" />
        <div class="text-xs text-ink-secondary leading-relaxed font-sans">
          {{ t("workspace.diff.binary.notice") }}
        </div>
      </div>
      <button
        v-if="canCompareAsset"
        @click="emit('open-asset-diff')"
        class="shrink-0 flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-accent text-accent-contrast hover:bg-accent-strong text-xs font-semibold cursor-pointer transition shadow-xs"
        :title="t('ws.asset.compare.title')"
      >
        <ScanEye class="w-3.5 h-3.5" />
        <span>{{ t("ws.asset.compare") }}</span>
      </button>
    </div>

    <template v-else>
      <!-- Diff Patch Playground Bar & Drawer -->
      <div v-if="diffText && !isBinaryDiff" class="mx-2 mt-2 flex items-center justify-between gap-2">
        <button
          type="button"
          @click="showPatchPlayground = !showPatchPlayground"
          class="flex items-center gap-1.5 px-2 py-0.5 rounded text-[10px] font-semibold transition cursor-pointer border shadow-xs"
          :class="showPatchPlayground ? 'bg-accent/20 border-accent/40 text-accent' : 'bg-surface hover:bg-raised border-line text-ink-muted hover:text-ink'"
          :title="t('workspace.diff.playground.hint')"
        >
          <Code2 class="w-3 h-3 text-accent" />
          <span>{{ t("workspace.diff.playground.btn") }}</span>
        </button>
      </div>

      <!-- Patch Playground Drawer -->
      <div
        v-if="showPatchPlayground && diffText && !isBinaryDiff"
        class="mx-2 my-2 p-3 rounded-xl border border-line bg-surface/90 backdrop-blur-md shadow-lg space-y-2.5 shrink-0"
      >
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-2">
            <FileCode class="w-4 h-4 text-accent" />
            <span class="text-xs font-bold text-ink">{{ t("workspace.diff.playground.title") }}</span>
            <span class="px-1.5 py-0.5 rounded text-[10px] font-mono bg-raised border border-line text-ink-muted">
              {{ t("workspace.diff.playground.hunks", { count: diffHunks.length }) }}
            </span>
            <span class="px-1.5 py-0.5 rounded text-[10px] font-mono bg-raised border border-line text-ink-muted">
              {{ t("workspace.diff.playground.lines", { count: diffTotalLineCount }) }}
            </span>
          </div>
          <div class="flex items-center gap-1.5">
            <button
              type="button"
              @click="copyFullPatch"
              class="flex items-center gap-1 px-2.5 py-1 rounded-lg border border-line bg-surface hover:bg-raised text-ink text-xs font-semibold transition cursor-pointer"
            >
              <Check v-if="copiedPatch" class="w-3.5 h-3.5 text-success" />
              <Copy v-else class="w-3.5 h-3.5" />
              <span>{{ copiedPatch ? t("ws.copy.done") : t("workspace.diff.playground.copy") }}</span>
            </button>
            <button
              type="button"
              @click="showPatchPlayground = false"
              class="p-1 rounded-lg hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer"
            >
              <X class="w-3.5 h-3.5" />
            </button>
          </div>
        </div>
        <textarea
          :value="diffText"
          readonly
          rows="6"
          class="w-full p-2.5 rounded-lg bg-canvas border border-line font-mono text-[11px] text-ink leading-relaxed focus:outline-none resize-none select-all"
        />
      </div>

      <!-- Per-line selection toolbar (unified diff only) -->
      <div
        v-if="canApplyLineActions && selectedLineCount > 0"
        class="mx-2 mt-2 px-3 py-2 rounded-lg border border-accent/40 bg-accent/10 flex items-center justify-between gap-3 select-none shrink-0"
      >
        <span class="text-[11px] font-semibold text-accent">{{
          t("workspace.diff.selected.lines", { count: selectedLineCount })
        }}</span>
        <div class="flex items-center gap-1.5">
          <button
            @click="clearLineSelection"
            class="px-2 py-0.5 rounded text-[10px] text-ink-muted hover:text-ink transition cursor-pointer"
          >
            {{ t("workspace.diff.clear") }}
          </button>
          <button
            @click="copySelectedLines"
            class="px-2 py-0.5 rounded border border-line bg-surface hover:bg-raised text-ink text-[10px] font-semibold transition cursor-pointer flex items-center gap-1"
            :title="t('workspace.diff.copy.lines')"
          >
            <Check v-if="copiedLines" class="w-3 h-3 text-success" />
            <Copy v-else class="w-3 h-3 text-ink-muted" />
            <span>{{ t("workspace.diff.copy.lines") }}</span>
          </button>
          <button
            @click="emitLineAction('stage')"
            :disabled="hunkActionBusy"
            class="px-2.5 py-1 rounded-md bg-success-strong hover:bg-success-dark text-white text-[10px] font-semibold transition cursor-pointer disabled:opacity-50"
          >
            {{ t("workspace.diff.stage.lines") }}
          </button>
          <button
            @click="emitLineAction('discard')"
            :disabled="hunkActionBusy"
            class="px-2.5 py-1 rounded-md bg-danger-strong hover:bg-danger-strong-hover text-white text-[10px] font-semibold transition cursor-pointer disabled:opacity-50"
          >
            {{ t("workspace.diff.discard.lines") }}
          </button>
        </div>
      </div>

      <!-- Large Diff Guard Banner: 超大差异默认折叠，避免一次性渲染海量 DOM -->
      <div
        v-if="isLargeDiff"
        class="mx-2 mt-2 px-3 py-2 rounded-lg border border-warning/40 bg-warning/10 flex items-center justify-between gap-3 select-none shrink-0"
      >
        <div class="flex items-center gap-2 text-[11px] text-warning min-w-0">
          <AlertTriangle class="w-4 h-4 shrink-0" />
          <span class="truncate font-medium">
            <template v-if="isDiffFolded">
              {{
                t("workspace.diff.large.folded", {
                  total: diffTotalLineCount,
                  additions: diffStats.additions,
                  deletions: diffStats.deletions,
                  limit: MAX_DIFF_LINES,
                })
              }}
            </template>
            <template v-else>
              {{
                t("workspace.diff.large.expanded", {
                  total: diffTotalLineCount,
                  additions: diffStats.additions,
                  deletions: diffStats.deletions,
                })
              }}
            </template>
          </span>
        </div>

        <div class="flex items-center gap-2 shrink-0">
          <button
            v-if="isDiffFolded"
            @click="showFullDiff = true"
            class="px-2.5 py-1 rounded-md bg-warning/20 hover:bg-warning/30 text-warning text-[11px] font-semibold transition cursor-pointer border border-warning/40"
          >
            {{ t("workspace.diff.show.all") }}
          </button>
          <button
            v-else
            @click="showFullDiff = false"
            class="px-2.5 py-1 rounded-md bg-surface hover:bg-raised text-ink text-[11px] font-semibold transition cursor-pointer border border-line"
          >
            {{ t("workspace.diff.fold") }}
          </button>
        </div>
      </div>

      <!-- Conflict Block Floating Navigation Pill -->
      <div
        v-if="totalConflictBlocks > 0"
        class="sticky top-2 z-20 mx-auto w-fit px-3 py-1 bg-surface/95 backdrop-blur-md border border-warning/40 shadow-lg rounded-full flex items-center gap-2 text-xs select-none text-ink my-1"
      >
        <span class="w-2 h-2 rounded-full bg-warning animate-pulse"></span>
        <span class="font-medium text-warning font-sans">
          {{ t("workspace.diff.conflict.navigation", { current: activeConflictIndex + 1, total: totalConflictBlocks }) }}
        </span>
        <div class="flex items-center gap-1 border-l border-line/60 pl-2">
          <button
            type="button"
            @click="scrollToConflict(activeConflictIndex - 1)"
            :disabled="activeConflictIndex <= 0"
            class="p-0.5 rounded hover:bg-raised text-ink-muted hover:text-ink disabled:opacity-40 transition cursor-pointer"
            :title="t('workspace.diff.conflict.prev')"
          >
            <ChevronUp class="w-3.5 h-3.5" />
          </button>
          <button
            type="button"
            @click="scrollToConflict(activeConflictIndex + 1)"
            :disabled="activeConflictIndex >= totalConflictBlocks - 1"
            class="p-0.5 rounded hover:bg-raised text-ink-muted hover:text-ink disabled:opacity-40 transition cursor-pointer"
            :title="t('workspace.diff.conflict.next')"
          >
            <ChevronDown class="w-3.5 h-3.5" />
          </button>
        </div>
        <div class="border-l border-line/60 pl-2">
          <button
            type="button"
            @click="emit('open-ai-advisor')"
            class="flex items-center gap-1 px-2 py-0.5 rounded-full bg-accent/20 hover:bg-accent/30 text-accent font-semibold text-[10px] transition cursor-pointer border border-accent/40"
            :title="t('workspace.conflict.ai.hint')"
          >
            <Sparkles class="w-3 h-3 text-accent" />
            <span>{{ t("workspace.conflict.ai.advisor") }}</span>
          </button>
        </div>
      </div>

      <!-- 0. WORD-LEVEL DIFF MODE -->
      <div v-if="mode === 'word'" class="py-2 font-mono">
        <div class="mx-2 mb-1.5 px-3 py-1.5 rounded-lg border border-dashed border-line bg-surface/30 text-[10px] text-ink-muted font-sans">
          {{ t("workspace.diff.word.prefix") }}
          <span class="bg-danger/15 text-danger line-through">{{ t("workspace.diff.word.removed") }}</span>
          {{ t("workspace.diff.word.and") }}
          <span class="bg-success/15 text-success">{{ t("workspace.diff.word.added") }}</span>
          {{ t("workspace.diff.word.suffix") }}
        </div>
        <div
          v-for="line in parsedWordDiffLines"
          :key="line.index"
          class="px-3 py-0.5 whitespace-pre-wrap break-all"
          :class="{
            'text-ink-muted': line.type === 'meta',
            'bg-accent/10 text-accent font-semibold py-1 border-y border-accent/20': line.type === 'hunk',
            'text-ink-secondary': line.type === 'content',
          }"
        >
          <template v-if="line.type === 'content'">
            <span
              v-for="(token, ti) in line.tokens"
              :key="ti"
              :class="{
                'bg-danger/15 text-danger line-through decoration-danger/50': token.kind === 'del',
                'bg-success/15 text-success': token.kind === 'add',
              }"
            >{{ token.text }}</span>
          </template>
          <template v-else>{{ line.text }}</template>
        </div>
      </div>

      <!-- 1. UNIFIED DIFF MODE -->
      <div v-else-if="mode === 'unified'" class="divide-y divide-line/20 font-mono">
        <div v-if="visibleUnifiedWindow.topPadding > 0" :style="{ height: visibleUnifiedWindow.topPadding + 'px' }" />
        <template v-for="line in visibleUnifiedWindow.lines" :key="line.index">
          <!-- Conflict Marker Banner Row with Inline Block-Level Resolvers -->
          <template v-if="line.type === 'conflict-marker'">
            <div
              v-if="line.text.startsWith('<<<<<<<')"
              :id="'conflict-block-' + (line.conflictBlockIndex ?? 0)"
              class="px-3 py-1.5 bg-accent/10 border-y border-dashed border-accent/40 flex items-center justify-between text-xs select-none my-1"
            >
              <div class="flex items-center gap-2">
                <span class="px-1.5 py-0.5 rounded bg-accent/20 text-accent font-mono text-[10px] font-bold">
                  {{ t("workspace.diff.conflict.block.badge", { n: (line.conflictBlockIndex ?? 0) + 1 }) }}
                </span>
                <span class="text-ink-secondary text-[11px] font-sans">
                  {{ t("workspace.diff.conflict.block.title") }}
                </span>
              </div>
              <div class="flex items-center gap-1.5">
                <button
                  type="button"
                  @click="emit('resolve-conflict-block', line.conflictBlockIndex ?? 0, 'ours')"
                  class="px-2 py-0.5 rounded bg-accent/20 hover:bg-accent/30 text-accent text-[11px] font-medium transition cursor-pointer"
                  :title="t('workspace.diff.conflict.accept.ours.hint')"
                >
                  {{ t("workspace.diff.conflict.accept.ours") }}
                </button>
                <button
                  type="button"
                  @click="emit('resolve-conflict-block', line.conflictBlockIndex ?? 0, 'theirs')"
                  class="px-2 py-0.5 rounded bg-purple/20 hover:bg-purple/30 text-purple text-[11px] font-medium transition cursor-pointer"
                  :title="t('workspace.diff.conflict.accept.theirs.hint')"
                >
                  {{ t("workspace.diff.conflict.accept.theirs") }}
                </button>
                <button
                  type="button"
                  @click="emit('resolve-conflict-block', line.conflictBlockIndex ?? 0, 'both')"
                  class="px-2 py-0.5 rounded bg-surface hover:bg-raised border border-line text-ink text-[11px] font-medium transition cursor-pointer"
                  :title="t('workspace.diff.conflict.accept.both.hint')"
                >
                  {{ t("workspace.diff.conflict.accept.both") }}
                </button>
              </div>
            </div>

            <div
              class="px-3 py-1 font-bold text-xs flex items-center justify-between select-none my-0.5"
              :class="{
                'bg-accent/20 text-accent border-l-4 border-accent': line.text.startsWith('<<<<<<<'),
                'bg-warning/20 text-warning border-y border-dashed border-warning': line.text.startsWith('======='),
                'bg-purple/20 text-purple border-l-4 border-purple': line.text.startsWith('>>>>>>>'),
              }"
            >
              <span class="font-mono">{{ line.text }}</span>
              <span class="text-[10px] font-sans font-semibold uppercase opacity-90">
                {{
                  line.text.startsWith("<<<<<<<")
                    ? t("workspace.diff.conflict.ours")
                    : line.text.startsWith("=======")
                      ? t("workspace.diff.conflict.divider")
                      : t("workspace.diff.conflict.theirs")
                }}
              </span>
            </div>
          </template>

          <div
            v-else
            class="flex items-center hover:bg-surface/50 px-2 py-0.5 transition-colors"
            :class="{
              'bg-success/10 text-success font-medium': line.type === 'add',
              'bg-danger/10 text-danger': line.type === 'del',
              'bg-surface/80 text-accent font-semibold': line.type === 'header',
              'text-ink-secondary': line.type === 'ctx',
            }"
          >
            <!-- Per-line selection -->
            <span v-if="canApplyLineActions" class="w-5 flex items-center justify-center shrink-0" @click.stop>
              <input
                v-if="line.type === 'add' || line.type === 'del'"
                type="checkbox"
                :checked="selectedLineIndexes.has(line.index)"
                @change="toggleLineSelection(line.index)"
                class="w-3 h-3 rounded border-line accent-accent cursor-pointer"
                :title="t('workspace.diff.select.line.hint')"
              />
            </span>

            <!-- Line Numbers -->
            <span class="w-10 text-right pr-2 text-[10px] text-ink-muted select-none shrink-0 font-mono">
              {{ line.oldLineNo }}
            </span>
            <span class="w-10 text-right pr-3 text-[10px] text-ink-muted select-none shrink-0 border-r border-line/40 font-mono">
              {{ line.newLineNo }}
            </span>

            <!-- Prefix Indicator (+/-/@) -->
            <span class="w-6 text-center select-none font-bold shrink-0">
              <template v-if="line.type === 'add'">+</template>
              <template v-else-if="line.type === 'del'">-</template>
              <template v-else-if="line.type === 'header'">@</template>
              <template v-else>&nbsp;</template>
            </span>

            <!-- Code Text -->
            <span class="flex-1 whitespace-pre pl-1">{{ line.text }}</span>

            <!-- Per-hunk actions -->
            <span
              v-if="line.type === 'header'"
              class="flex items-center gap-1 pl-3 shrink-0 font-sans"
            >
              <button
                @click="copyHunk(line.index)"
                class="px-2 py-0.5 rounded border border-line bg-surface hover:bg-raised text-[10px] font-semibold text-ink-secondary hover:text-ink transition cursor-pointer flex items-center gap-1"
                :title="t('workspace.diff.copy.hunk')"
              >
                <Check v-if="copiedHunkLine === line.index" class="w-3 h-3 text-success" />
                <Copy v-else class="w-3 h-3 text-ink-muted" />
                <span>{{ t("workspace.diff.copy.hunk") }}</span>
              </button>
              <template v-if="canApplyHunks">
                <button
                  @click="emitHunk('stage-hunk', hunkIndexForLine(line.index))"
                  :disabled="hunkActionBusy"
                  class="px-2 py-0.5 rounded border border-line bg-surface hover:bg-raised text-[10px] font-semibold text-accent transition cursor-pointer disabled:opacity-50"
                  :title="t('workspace.diff.stage.hunk.hint')"
                >
                  {{ t("workspace.diff.stage.hunk") }}
                </button>
                <button
                  @click="emitHunk('discard-hunk', hunkIndexForLine(line.index))"
                  :disabled="hunkActionBusy"
                  class="px-2 py-0.5 rounded border border-line bg-surface hover:bg-raised text-[10px] font-semibold text-danger transition cursor-pointer disabled:opacity-50"
                  :title="t('workspace.diff.discard.hunk.hint')"
                >
                  {{ t("workspace.diff.discard.hunk") }}
                </button>
              </template>
            </span>
          </div>
        </template>
        <div v-if="visibleUnifiedWindow.bottomPadding > 0" :style="{ height: visibleUnifiedWindow.bottomPadding + 'px' }" />
      </div>

      <!-- 2. SPLIT DIFF MODE (SIDE-BY-SIDE) -->
      <div v-else class="divide-y divide-line/20 font-mono text-[11px]">
        <!-- Split Mode Toolbar: Wrap toggle -->
        <div class="px-3 py-1 bg-surface/50 border-b border-line/30 flex items-center justify-end select-none font-sans">
          <button
            @click="isSplitWrap = !isSplitWrap"
            class="px-2 py-0.5 rounded border text-[10px] font-semibold transition cursor-pointer flex items-center gap-1.5"
            :class="
              isSplitWrap
                ? 'border-accent/40 bg-accent/15 text-accent'
                : 'border-line bg-surface hover:bg-raised text-ink-muted hover:text-ink'
            "
            :title="t('workspace.diff.split.wrap')"
          >
            <WrapText class="w-3 h-3" />
            <span>{{ t("workspace.diff.split.wrap") }}</span>
          </button>
        </div>

        <template v-for="row in parsedSplitDiffRows" :key="row.index">
          <!-- Chunk Header spanning full width -->
          <div
            v-if="row.isHeader"
            class="px-4 py-1 bg-surface/80 text-accent font-semibold border-y border-line/40 text-xs flex items-center gap-3"
          >
            <span class="flex-1 whitespace-pre">{{ row.headerText }}</span>
            <span class="flex items-center gap-1 font-sans font-normal">
              <button
                @click="copyHunk(row.rawLineIndex)"
                class="px-2 py-0.5 rounded border border-line bg-surface hover:bg-raised text-[10px] font-semibold text-ink-secondary hover:text-ink transition cursor-pointer flex items-center gap-1"
                :title="t('workspace.diff.copy.hunk')"
              >
                <Check v-if="copiedHunkLine === row.rawLineIndex" class="w-3 h-3 text-success" />
                <Copy v-else class="w-3 h-3 text-ink-muted" />
                <span>{{ t("workspace.diff.copy.hunk") }}</span>
              </button>
              <template v-if="canApplyHunks">
                <button
                  @click="emitHunk('stage-hunk', row.hunkIndex)"
                  :disabled="hunkActionBusy"
                  class="px-2 py-0.5 rounded border border-line bg-surface hover:bg-raised text-[10px] font-semibold text-accent transition cursor-pointer disabled:opacity-50"
                  :title="t('workspace.diff.stage.hunk.hint')"
                >
                  {{ t("workspace.diff.stage.hunk") }}
                </button>
                <button
                  @click="emitHunk('discard-hunk', row.hunkIndex)"
                  :disabled="hunkActionBusy"
                  class="px-2 py-0.5 rounded border border-line bg-surface hover:bg-raised text-[10px] font-semibold text-danger transition cursor-pointer disabled:opacity-50"
                  :title="t('workspace.diff.discard.hunk.hint')"
                >
                  {{ t("workspace.diff.discard.hunk") }}
                </button>
              </template>
            </span>
          </div>

          <!-- Side-by-Side Dual Panes -->
          <div v-else class="flex items-stretch divide-x divide-line/30">
            <!-- Left: Old / Deleted -->
            <div
              class="w-1/2 flex items-start px-2 py-0.5 transition-colors min-w-0"
              :class="{
                'bg-danger/10 text-danger': row.left.type === 'del',
                'text-ink-secondary': row.left.type === 'ctx',
                'bg-canvas/30': row.left.type === 'empty',
              }"
            >
              <span class="w-9 text-right pr-2 text-[10px] text-ink-muted select-none shrink-0 font-mono leading-relaxed">
                {{ row.left.lineNo }}
              </span>
              <span class="w-4 text-center select-none font-bold shrink-0 leading-relaxed">
                {{ row.left.type === 'del' ? '-' : '' }}
              </span>
              <span
                class="flex-1 pl-1 leading-relaxed"
                :class="isSplitWrap ? 'whitespace-pre-wrap break-all' : 'whitespace-pre overflow-x-auto'"
              >{{ row.left.text }}</span>
            </div>

            <!-- Right: New / Added -->
            <div
              class="w-1/2 flex items-start px-2 py-0.5 transition-colors min-w-0"
              :class="{
                'bg-success/10 text-success font-medium': row.right.type === 'add',
                'text-ink-secondary': row.right.type === 'ctx',
                'bg-canvas/30': row.right.type === 'empty',
              }"
            >
              <span class="w-9 text-right pr-2 text-[10px] text-ink-muted select-none shrink-0 font-mono leading-relaxed">
                {{ row.right.lineNo }}
              </span>
              <span class="w-4 text-center select-none font-bold shrink-0 leading-relaxed">
                {{ row.right.type === 'add' ? '+' : '' }}
              </span>
              <span
                class="flex-1 pl-1 leading-relaxed"
                :class="isSplitWrap ? 'whitespace-pre-wrap break-all' : 'whitespace-pre overflow-x-auto'"
              >{{ row.right.text }}</span>
            </div>
          </div>
        </template>
      </div>
    </template>

    <!-- Toast feedback for copy actions -->
    <Toast :text="toastText" :visible="showToast" />
  </div>
</template>
