<script setup lang="ts">
import { confirmAction } from "../composables/useConfirm";
import { ref, computed, watch, onMounted, onUnmounted } from "vue";
import {
  GitPullRequest,
  ArrowRight,
  MessageSquare,
  Download,
  Trash2,
  RefreshCw,
  AlertCircle,
  Copy,
  X,
  Loader2,
  FolderGit2,
} from "lucide-vue-next";
import { gitService } from "../services/gitService";
import Toast from "./ui/Toast.vue";
import { useToast } from "../composables/useToast";
import type { ChangeReview, ReviewComment, ReviewFileState, ReviewSessionSummary } from "../types";
import { verdictChipOf, verdictTextOf } from "./review/meta";
import SessionList from "./review/SessionList.vue";
import ChangeList from "./review/ChangeList.vue";
import FileDetailPanel from "./review/FileDetailPanel.vue";
import ExportModal from "./review/ExportModal.vue";

/**
 * Change Review workspace: review a base...head range file-by-file,
 * record per-file verdicts + notes, anchor comments to lines, export a report.
 */
const props = defineProps<{
  currentRepoPath: string;
}>();

const emit = defineEmits<{
  (e: "refresh"): void;
}>();

const MAX_DIFF_LINES = 4000;
const DEFAULT_BASE_REFS = ["origin/main", "origin/master", "origin/develop", "main", "master", "develop"];

type DiffKind = "add" | "del" | "context" | "hunk" | "meta";
interface DiffLine {
  kind: DiffKind;
  text: string;
  oldLine?: number;
  newLine?: number;
}
interface SessionRefs {
  repo: string;
  base: string;
  head: string;
}
interface FileStatusMeta {
  label: string;
  chip: string;
}

// ---------------------------------------------------------------- state ----
const baseRef = ref("");
const headRef = ref("HEAD");
const sessions = ref<ReviewSessionSummary[]>([]);
const review = ref<ChangeReview | null>(null);
const selectedPath = ref("");
const diffText = ref("");
const noteDraft = ref("");
const commitsOpen = ref(true);

const isLoadingSessions = ref(false);
const isLoadingReview = ref(false);
const isLoadingDiff = ref(false);
const isMutating = ref(false);
const isExporting = ref(false);

const sessionsError = ref("");
const reviewError = ref("");
const diffError = ref("");
const actionError = ref("");
const { toastText: actionInfo, showToast: actionToastVisible, notify: showInfo } = useToast(2600);

const exportOpen = ref(false);
const exportMarkdown = ref("");
const exportCopied = ref(false);

let bootstrapSeq = 0;
let sessionsSeq = 0;
let reviewSeq = 0;
let diffSeq = 0;
let actionSeq = 0;
let exportSeq = 0;

// ------------------------------------------------------------- helpers ----
function errText(err: unknown): string {
  if (err instanceof Error && err.message) return err.message;
  if (typeof err === "string" && err) return err;
  if (err && typeof err === "object" && "message" in err) {
    const message = (err as { message?: unknown }).message;
    if (typeof message === "string" && message) return message;
  }
  return "操作失败，请稍后重试";
}

function clearErrors() {
  reviewError.value = "";
  actionError.value = "";
}

// ----------------------------------------------------------- computeds ----
const reviewFiles = computed<ReviewFileState[]>(() => {
  const current = review.value;
  return current && Array.isArray(current.files) ? current.files : [];
});

const reviewCommits = computed(() => {
  const current = review.value;
  return current && Array.isArray(current.commits) ? current.commits : [];
});

const selectedFile = computed<ReviewFileState | null>(() => {
  const path = selectedPath.value;
  if (!path) return null;
  return reviewFiles.value.find((file) => file.path === path) ?? null;
});

const noteDirty = computed(() => {
  const file = selectedFile.value;
  if (!file) return false;
  return noteDraft.value !== (file.note ?? "");
});

const fileComments = computed<ReviewComment[]>(() => {
  const current = review.value;
  const path = selectedPath.value;
  if (!current || !path || !Array.isArray(current.comments)) return [];
  return current.comments
    .filter((comment) => comment.path === path)
    .slice()
    .sort((a, b) => Number(a.resolved) - Number(b.resolved) || a.line - b.line);
});


const verdictText = computed(() => verdictTextOf(review.value?.stats.verdict ?? ""));
const verdictChipClass = computed(() => verdictChipOf(review.value?.stats.verdict ?? ""));
const progressBarClass = computed(() => {
  switch (verdictText.value) {
    case "可以合并":
      return "bg-success-strong";
    case "需修改":
      return "bg-danger-strong";
    default:
      return "bg-accent-strong";
  }
});

// ------------------------------------------------------------- loading ----
async function loadSessions(token = bootstrapSeq) {
  const repo = props.currentRepoPath;
  const seq = ++sessionsSeq;
  if (!repo) {
    sessions.value = [];
    isLoadingSessions.value = false;
    return;
  }
  isLoadingSessions.value = true;
  sessionsError.value = "";
  try {
    const list = await gitService.listChangeReviews(repo);
    if (seq !== sessionsSeq || token !== bootstrapSeq) return;
    sessions.value = Array.isArray(list) ? list : [];
  } catch (err) {
    if (seq === sessionsSeq && token === bootstrapSeq) sessionsError.value = errText(err);
  } finally {
    if (seq === sessionsSeq && token === bootstrapSeq) isLoadingSessions.value = false;
  }
}

async function detectDefaultBase(token: number) {
  const repo = props.currentRepoPath;
  let detected = "HEAD~20";
  if (repo) {
    try {
      const branches = await gitService.getBranches(repo);
      if (token !== bootstrapSeq) return;
      const names = (Array.isArray(branches) ? branches : []).map((branch) => branch.name);
      const preferred = DEFAULT_BASE_REFS.find((candidate) => names.includes(candidate));
      if (preferred) detected = preferred;
    } catch {
      // 检测失败时回退到 HEAD~20。
      detected = "HEAD~20";
    }
  }
  if (token !== bootstrapSeq) return;
  if (!baseRef.value.trim()) baseRef.value = detected;
}

function adoptReview(next: ChangeReview) {
  review.value = next;
  const files = Array.isArray(next.files) ? next.files : [];
  const current = selectedPath.value ? files.find((file) => file.path === selectedPath.value) : undefined;
  const target = current ?? files[0];
  if (!target) {
    selectedPath.value = "";
    diffText.value = "";
    noteDraft.value = "";
    return;
  }
  selectedPath.value = target.path;
  noteDraft.value = target.note ?? "";
  void loadDiff(target.path);
}

function applyRemoteReview(next: ChangeReview) {
  review.value = next;
  const files = Array.isArray(next.files) ? next.files : [];
  if (!selectedPath.value) return;
  const current = files.find((file) => file.path === selectedPath.value);
  if (!current) {
    selectedPath.value = "";
    diffText.value = "";
    const first = files[0];
    if (first) {
      selectedPath.value = first.path;
      noteDraft.value = first.note ?? "";
      void loadDiff(first.path);
    }
    return;
  }
  if (!noteDirty.value) noteDraft.value = current.note ?? "";
}

async function loadReview(token = bootstrapSeq, refreshSessions = false) {
  const repo = props.currentRepoPath;
  const base = baseRef.value.trim();
  const head = headRef.value.trim() || "HEAD";
  const seq = ++reviewSeq;
  actionSeq += 1;
  reviewError.value = "";
  actionError.value = "";

  if (!repo) {
    reviewError.value = "请先打开仓库";
    return;
  }
  if (!base) {
    reviewError.value = "请输入基准引用（base），例如 origin/main 或 HEAD~20";
    return;
  }

  isLoadingReview.value = true;
  try {
    const next = await gitService.getChangeReview(repo, base, head);
    if (seq !== reviewSeq || token !== bootstrapSeq) return;
    if (!next) {
      reviewError.value = "未找到评审会话，请检查 base / head 引用是否存在";
      return;
    }
    baseRef.value = next.base || base;
    headRef.value = next.head || head;
    adoptReview(next);
    emit("refresh");
    if (refreshSessions) void loadSessions(token);
  } catch (err) {
    if (seq === reviewSeq && token === bootstrapSeq) reviewError.value = errText(err);
  } finally {
    if (seq === reviewSeq && token === bootstrapSeq) isLoadingReview.value = false;
  }
}

async function loadDiff(path: string) {
  const repo = props.currentRepoPath;
  const current = review.value;
  const seq = ++diffSeq;
  if (!repo || !current || !path) {
    diffText.value = "";
    isLoadingDiff.value = false;
    return;
  }
  isLoadingDiff.value = true;
  diffError.value = "";
  diffText.value = "";
  try {
    const raw = await gitService.getReviewFileDiff(repo, current.base, current.head, path);
    if (seq !== diffSeq) return;
    diffText.value = typeof raw === "string" ? raw : "";
  } catch (err) {
    if (seq === diffSeq) {
      diffText.value = "";
      diffError.value = errText(err);
    }
  } finally {
    if (seq === diffSeq) isLoadingDiff.value = false;
  }
}

// ------------------------------------------------------------ bootstrap ----
async function bootstrap() {
  const token = ++bootstrapSeq;
  sessionsSeq += 1;
  reviewSeq += 1;
  diffSeq += 1;
  actionSeq += 1;
  sessions.value = [];
  sessionsError.value = "";
  review.value = null;
  selectedPath.value = "";
  diffText.value = "";
  noteDraft.value = "";
  reviewError.value = "";
  diffError.value = "";
  actionError.value = "";
  isLoadingReview.value = false;
  isLoadingDiff.value = false;
  isMutating.value = false;
  exportOpen.value = false;
  commitsOpen.value = true;

  if (!props.currentRepoPath) {
    baseRef.value = "";
    headRef.value = "HEAD";
    return;
  }

  baseRef.value = "";
  headRef.value = "HEAD";
  await Promise.all([detectDefaultBase(token), loadSessions(token)]);
  if (token !== bootstrapSeq) return;

  const latest = sessions.value[0];
  if (latest && !review.value && !isLoadingReview.value) {
    baseRef.value = latest.base;
    headRef.value = latest.head;
    void loadReview(token, false);
  }
}

// -------------------------------------------------------------- actions ----
function currentRefs(): SessionRefs | null {
  const current = review.value;
  if (!props.currentRepoPath || !current) return null;
  return { repo: props.currentRepoPath, base: current.base, head: current.head };
}

async function runReviewMutation(
  successText: string,
  mutator: (refs: SessionRefs) => Promise<ChangeReview | null>
) {
  const refs = currentRefs();
  if (!refs || isMutating.value) return;
  const seq = ++actionSeq;
  const epoch = bootstrapSeq;
  isMutating.value = true;
  actionError.value = "";
  try {
    const next = await mutator(refs);
    if (seq !== actionSeq || epoch !== bootstrapSeq) return;
    if (!next) {
      actionError.value = "后端未返回最新评审会话";
      return;
    }
    applyRemoteReview(next);
    showInfo(successText);
    void loadSessions(epoch);
    emit("refresh");
  } catch (err) {
    if (seq === actionSeq && epoch === bootstrapSeq) actionError.value = errText(err);
  } finally {
    if (epoch === bootstrapSeq) isMutating.value = false;
  }
}

function selectFile(file: ReviewFileState) {
  selectedPath.value = file.path;
  noteDraft.value = file.note ?? "";
  void loadDiff(file.path);
}

function openSession(session: ReviewSessionSummary) {
  if (isLoadingReview.value) return;
  if (review.value && review.value.key === session.key) return;
  baseRef.value = session.base;
  headRef.value = session.head;
  void loadReview(bootstrapSeq, false);
}

function startReview() {
  if (isLoadingReview.value) return;
  void loadReview(bootstrapSeq, true);
}

function refreshAll() {
  if (!props.currentRepoPath) return;
  clearErrors();
  void loadSessions(bootstrapSeq);
  if (review.value) void loadReview(bootstrapSeq, false);
}

function setFileState(status: "approved" | "changes" | "pending") {
  const file = selectedFile.value;
  if (!file) return;
  const path = file.path;
  const note = noteDraft.value;
  const labels: Record<string, string> = {
    approved: "已标记为通过",
    changes: "已标记为需修改",
    pending: "已重置为待评审",
  };
  void runReviewMutation(labels[status] ?? "评审状态已更新", (refs) =>
    gitService.setReviewFileState(refs.repo, refs.base, refs.head, path, status, note)
  );
}

function saveNote() {
  const file = selectedFile.value;
  if (!file) return;
  void runReviewMutation("评审备注已保存", (refs) =>
    gitService.setReviewFileState(refs.repo, refs.base, refs.head, file.path, file.status || "pending", noteDraft.value)
  );
}

function addComment(payload: { body: string; line: number; side: "new" | "old" }) {
  const file = selectedFile.value;
  const body = payload.body.trim();
  if (!file || !body || isMutating.value) return;
  void runReviewMutation("评论已添加", (refs) =>
    gitService.addReviewComment(refs.repo, refs.base, refs.head, file.path, payload.line, payload.side, body)
  );
}

function toggleComment(comment: ReviewComment) {
  const nextResolved = !comment.resolved;
  void runReviewMutation(nextResolved ? "评论已解决" : "评论已重新打开", (refs) =>
    gitService.resolveReviewComment(refs.repo, refs.base, refs.head, comment.id, nextResolved)
  );
}

async function handleDeleteComment(comment: ReviewComment) {
  if (!(await confirmAction({ message: "确定删除该评论吗？", tone: "danger" }))) return;
  void runReviewMutation("评论已删除", (refs) =>
    gitService.deleteReviewComment(refs.repo, refs.base, refs.head, comment.id)
  );
}

async function clearSession() {
  const refs = currentRefs();
  const current = review.value;
  if (!refs || !current) return;
  const label = `${current.baseLabel || current.base} → ${current.headLabel || current.head}`;
  if (!(await confirmAction({ message: `确定清除评审会话「${label}」的全部文件结论与评论吗？此操作不可撤销。`, tone: "danger" }))) return;

  const seq = ++actionSeq;
  const epoch = bootstrapSeq;
  isMutating.value = true;
  actionError.value = "";
  try {
    const message = await gitService.clearChangeReview(refs.repo, refs.base, refs.head);
    if (seq !== actionSeq || epoch !== bootstrapSeq) return;
    showInfo(typeof message === "string" && message ? message : "评审会话已清除");
    await loadReview(epoch, true);
  } catch (err) {
    if (seq === actionSeq && epoch === bootstrapSeq) actionError.value = errText(err);
  } finally {
    if (epoch === bootstrapSeq) isMutating.value = false;
  }
}

async function exportReport() {
  const refs = currentRefs();
  if (!refs || isExporting.value) return;
  const seq = ++exportSeq;
  const epoch = bootstrapSeq;
  isExporting.value = true;
  actionError.value = "";
  try {
    const markdown = await gitService.exportChangeReviewMarkdown(refs.repo, refs.base, refs.head);
    if (seq !== exportSeq || epoch !== bootstrapSeq) return;
    exportMarkdown.value = typeof markdown === "string" ? markdown : "";
    exportCopied.value = false;
    exportOpen.value = true;
    void copyExportToClipboard();
  } catch (err) {
    if (seq === exportSeq && epoch === bootstrapSeq) actionError.value = errText(err);
  } finally {
    if (seq === exportSeq && epoch === bootstrapSeq) isExporting.value = false;
  }
}

async function copyExportToClipboard() {
  const text = exportMarkdown.value;
  if (!text) return;
  try {
    await navigator.clipboard.writeText(text);
    exportCopied.value = true;
    return;
  } catch {
    // 回退：某些 WebView 不开放 clipboard API。
  }
  try {
    const textarea = document.createElement("textarea");
    textarea.value = text;
    textarea.style.position = "fixed";
    textarea.style.opacity = "0";
    document.body.appendChild(textarea);
    textarea.select();
    const ok = document.execCommand("copy");
    textarea.remove();
    exportCopied.value = ok;
  } catch {
    exportCopied.value = false;
  }
}

function handleGlobalKeydown(e: KeyboardEvent) {
  if (e.key === "Escape" && exportOpen.value) {
    exportOpen.value = false;
  }
}

// ------------------------------------------------------------- lifecycle ----
watch(
  () => props.currentRepoPath,
  () => {
    void bootstrap();
  },
  { immediate: true }
);

onMounted(() => {
  window.addEventListener("keydown", handleGlobalKeydown);
});

onUnmounted(() => {
  window.removeEventListener("keydown", handleGlobalKeydown);
  bootstrapSeq += 1;
  sessionsSeq += 1;
  reviewSeq += 1;
  diffSeq += 1;
  actionSeq += 1;
  exportSeq += 1;
});
</script>

<template>
  <div class="flex-1 flex flex-col bg-canvas text-ink overflow-hidden select-none font-sans relative min-h-0">
    <!-- Header: session summary + global actions -->
    <header class="h-12 shrink-0 border-b border-line px-4 flex items-center gap-3 bg-surface/40 backdrop-blur-sm">
      <div class="flex items-center gap-2 shrink-0">
        <div class="w-7 h-7 rounded-lg bg-accent/15 text-accent flex items-center justify-center">
          <GitPullRequest class="w-4 h-4" />
        </div>
        <span class="text-xs font-semibold text-ink whitespace-nowrap">变更评审</span>
      </div>

      <template v-if="review">
        <div class="flex items-center gap-1.5 min-w-0">
          <span class="text-[10px] font-mono text-ink-muted truncate" :title="review.base">
            {{ review.baseLabel || review.base }}
          </span>
          <ArrowRight class="w-3 h-3 text-ink-muted shrink-0" />
          <span class="text-[10px] font-mono text-ink-muted truncate" :title="review.head">
            {{ review.headLabel || review.head }}
          </span>
        </div>

        <span
          class="px-1.5 py-0.5 rounded-md border text-[10px] font-medium whitespace-nowrap shrink-0"
          :class="verdictChipClass"
          :title="review.stats.verdictDetail"
        >
          {{ verdictText }}
        </span>

        <div class="flex items-center gap-1.5 shrink-0" :title="`评审进度 ${review.stats.progress}%`">
          <div class="w-16 h-1.5 rounded-full bg-raised overflow-hidden">
            <div
              class="h-full rounded-full transition-all duration-300"
              :class="progressBarClass"
              :style="{ width: `${review.stats.progress}%` }"
            ></div>
          </div>
          <span class="text-[10px] font-mono text-ink-muted">{{ review.stats.progress }}%</span>
        </div>

        <span class="text-[10px] font-mono text-success shrink-0">+{{ review.stats.additions }}</span>
        <span class="text-[10px] font-mono text-danger shrink-0">-{{ review.stats.deletions }}</span>

        <span class="flex items-center gap-1 text-[10px] text-ink-muted shrink-0" title="未解决评论">
          <MessageSquare class="w-3.5 h-3.5" />
          {{ review.stats.openComments }}
        </span>

        <span class="text-[10px] text-ink-muted shrink-0 hidden xl:inline">
          {{ review.stats.approved }}/{{ review.stats.total }} 文件已通过
        </span>
      </template>

      <div class="ml-auto flex items-center gap-1.5 shrink-0">
        <button
          v-if="review"
          @click="exportReport"
          :disabled="isExporting"
          class="flex items-center gap-1 px-2 py-1 rounded-lg bg-surface border border-line text-[11px] text-ink-secondary hover:text-ink hover:bg-raised transition cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
          title="导出评审报告 Markdown 并复制"
        >
          <Loader2 v-if="isExporting" class="w-3.5 h-3.5 animate-spin" />
          <Download v-else class="w-3.5 h-3.5" />
          导出 Markdown
        </button>
        <button
          v-if="review"
          @click="clearSession"
          :disabled="isMutating"
          class="flex items-center gap-1 px-2 py-1 rounded-lg bg-surface border border-line text-[11px] text-danger hover:bg-danger-bg transition cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
          title="清除该会话的全部结论与评论"
        >
          <Trash2 class="w-3.5 h-3.5" />
          清除会话
        </button>
        <button
          @click="refreshAll"
          :disabled="isLoadingReview || isLoadingSessions || !currentRepoPath"
          class="flex items-center gap-1 px-2 py-1 rounded-lg bg-surface border border-line text-[11px] text-ink-secondary hover:text-ink hover:bg-raised transition cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
          title="刷新评审会话与历史列表"
        >
          <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isLoadingReview || isLoadingSessions }" />
          刷新
        </button>
      </div>
    </header>

    <!-- Error banner -->
    <div
      v-if="reviewError || actionError"
      class="shrink-0 px-4 py-1.5 border-b border-danger-border bg-danger-bg text-[11px] text-danger flex items-center gap-2"
    >
      <AlertCircle class="w-3.5 h-3.5 shrink-0" />
      <span class="flex-1 truncate">{{ reviewError || actionError }}</span>
      <button aria-label="关闭" @click="clearErrors" class="shrink-0 p-0.5 rounded hover:bg-danger/15 cursor-pointer" title="关闭">
        <X class="w-3.5 h-3.5" />
      </button>
    </div>

    <!-- No repo placeholder -->
    <div v-if="!currentRepoPath" class="flex-1 flex flex-col items-center justify-center gap-2.5 text-ink-muted">
      <FolderGit2 class="w-10 h-10 opacity-40" />
      <div class="text-sm font-medium text-ink-secondary">请先打开仓库</div>
      <div class="text-xs">打开仓库后可基于 base / head 创建变更评审会话</div>
    </div>

    <!-- Main 3-column workspace -->
    <div v-else class="flex-1 flex overflow-hidden min-h-0">
      <SessionList
        :base="baseRef"
        :head="headRef"
        :sessions="sessions"
        :sessions-error="sessionsError"
        :is-loading-sessions="isLoadingSessions"
        :is-loading-review="isLoadingReview"
        :current-repo-path="currentRepoPath"
        :current-review-key="review?.key ?? ''"
        @update:base="baseRef = $event"
        @update:head="headRef = $event"
        @start="startReview"
        @open="openSession"
      />
      <ChangeList
        :review="review"
        :files="reviewFiles"
        :commits="reviewCommits"
        :selected-path="selectedPath"
        :commits-open="commitsOpen"
        :is-loading-review="isLoadingReview"
        @select-file="selectFile"
        @update:commits-open="commitsOpen = $event"
      />
      <FileDetailPanel
        v-model:note-draft="noteDraft"
        :review="review"
        :selected-file="selectedFile"
        :files-count="reviewFiles.length"
        :diff-text="diffText"
        :diff-error="diffError"
        :is-loading-diff="isLoadingDiff"
        :is-loading-review="isLoadingReview"
        :is-mutating="isMutating"
        :comments="fileComments"
        :note-dirty="noteDirty"
        @set-state="setFileState"
        @save-note="saveNote"
        @add-comment="addComment"
        @toggle-comment="toggleComment"
        @delete-comment="handleDeleteComment"
      />
    </div>

    <Toast :text="actionInfo" :visible="actionToastVisible" />

    <ExportModal
      v-model:open="exportOpen"
      :markdown="exportMarkdown"
      :copied="exportCopied"
      @copy="copyExportToClipboard"
    />
  </div>
</template>
