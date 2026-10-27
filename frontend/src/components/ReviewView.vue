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
  Check,
  RotateCcw,
} from "lucide-vue-next";
import { gitService } from "../services/gitService";
import {
  createReviewSource,
  localNative,
  localReviewId,
  ReviewSourceError,
  type LocalReviewSource,
  type ReviewDetail,
} from "../services/reviewSource";
import { t } from "../i18n";
import Toast from "./ui/Toast.vue";
import { useToast } from "../composables/useToast";
import type { ChangeReview, ReviewComment, ReviewFileState, ReviewSessionSummary } from "../types";
import { verdictChipOf, verdictKeyOf, verdictTextOf } from "./review/meta";
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
  (e: "switch-mode", mode: "local" | "remote"): void;
}>();

/**
 * All local review I/O goes through the ReviewSource adapter. The view keeps its
 * own session bootstrap (base/head detection) because choosing a range is a
 * local-source affordance, but every read/write below is source-agnostic.
 */
let reviewSource: LocalReviewSource | null = null;
let reviewSourceRepo = "";
function source(): LocalReviewSource {
  if (!reviewSource || reviewSourceRepo !== props.currentRepoPath) {
    reviewSource = createReviewSource("local", props.currentRepoPath);
    reviewSourceRepo = props.currentRepoPath;
  }
  return reviewSource;
}

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
  return t("review.error.generic");
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
  // Switch on the stable key, not the localized label — the label changes with
  // the locale while the colour must not.
  switch (verdictKeyOf(review.value?.stats.verdict ?? "")) {
    case "review.verdict.ready":
      return "bg-success-strong";
    case "review.verdict.changes":
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
    const list = await source().list();
    if (seq !== sessionsSeq || token !== bootstrapSeq) return;
    sessions.value = Array.isArray(list.nativeItems) ? list.nativeItems : [];
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

function getDraftStorageKey(path: string): string {
  const repo = props.currentRepoPath;
  const current = review.value;
  const base = current?.base || baseRef.value;
  const head = current?.head || headRef.value;
  return `wgit.review.draft.${encodeURIComponent(repo)}.${encodeURIComponent(base)}.${encodeURIComponent(head)}.${encodeURIComponent(path)}`;
}

function loadSavedDraft(path: string, fallback: string): string {
  try {
    const saved = localStorage.getItem(getDraftStorageKey(path));
    if (saved !== null && saved.trim() !== "") return saved;
  } catch {}
  return fallback;
}

function saveDraftToStorage(path: string, content: string) {
  if (!path) return;
  try {
    const key = getDraftStorageKey(path);
    if (!content || content.trim() === "") {
      localStorage.removeItem(key);
    } else {
      localStorage.setItem(key, content);
    }
  } catch {}
}

function clearDraftFromStorage(path: string) {
  if (!path) return;
  try {
    localStorage.removeItem(getDraftStorageKey(path));
  } catch {}
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
  noteDraft.value = loadSavedDraft(target.path, target.note ?? "");
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
    reviewError.value = t("review.error.no.repo");
    return;
  }
  if (!base) {
    reviewError.value = t("review.error.base.required");
    return;
  }

  isLoadingReview.value = true;
  try {
    const model = await source().detail(localReviewId(base, head));
    const next = localNative(model);
    if (seq !== reviewSeq || token !== bootstrapSeq) return;
    if (!next) {
      reviewError.value = t("review.error.session.not.found");
      return;
    }
    baseRef.value = next.base || base;
    headRef.value = next.head || head;
    adoptReview(next);
    emit("refresh");
    if (refreshSessions) void loadSessions(token);
  } catch (err) {
    if (seq === reviewSeq && token === bootstrapSeq) {
      reviewError.value =
        err instanceof ReviewSourceError && err.code === "not_found"
          ? t("review.error.session.not.found")
          : errText(err);
    }
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
    const raw = await source().diff(localReviewId(current.base, current.head), path);
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
  mutator: (refs: SessionRefs) => Promise<ReviewDetail | null>
) {
  const refs = currentRefs();
  if (!refs || isMutating.value) return;
  const seq = ++actionSeq;
  const epoch = bootstrapSeq;
  isMutating.value = true;
  actionError.value = "";
  try {
    const model = await mutator(refs);
    if (seq !== actionSeq || epoch !== bootstrapSeq) return;
    const next = localNative(model);
    if (!next) {
      actionError.value = t("review.error.no.session");
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
  noteDraft.value = loadSavedDraft(file.path, file.note ?? "");
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
    approved: t("review.toast.approved"),
    changes: t("review.toast.changes"),
    pending: t("review.toast.pending"),
  };
  void runReviewMutation(labels[status] ?? t("review.toast.status.updated"), (refs) =>
    source().setFileVerdict(localReviewId(refs.base, refs.head), path, status, note)
  );
}

function selectAdjacentFile(delta: number) {
  const files = reviewFiles.value;
  if (files.length === 0) return;
  const currentIdx = files.findIndex((f) => f.path === selectedPath.value);
  const nextIdx = currentIdx === -1 ? 0 : Math.max(0, Math.min(files.length - 1, currentIdx + delta));
  const next = files[nextIdx];
  if (next && next.path !== selectedPath.value) {
    selectedPath.value = next.path;
    noteDraft.value = next.note ?? "";
    void loadDiff(next.path);
  }
}

function handleApproveAndNext() {
  const current = selectedFile.value;
  if (!current) return;
  setFileState("approved");
  selectAdjacentFile(1);
}

async function handleBatchSetStatus(status: "approved" | "pending") {
  const current = review.value;
  if (!current || isMutating.value) return;
  const confirmMsg =
    status === "approved"
      ? t("review.confirm.batch.approve", { count: reviewFiles.value.length })
      : t("review.confirm.batch.reset", { count: reviewFiles.value.length });
  if (!(await confirmAction({ message: confirmMsg, tone: "default" }))) return;

  isMutating.value = true;
  try {
    for (const file of reviewFiles.value) {
      if (file.status !== status) {
        await source().setFileVerdict(localReviewId(current.base, current.head), file.path, status, file.note ?? "");
      }
    }
    await loadReview(bootstrapSeq);
    showInfo(status === "approved" ? t("review.toast.batch.approved") : t("review.toast.batch.reset"));
  } catch (err) {
    actionError.value = errText(err);
  } finally {
    isMutating.value = false;
  }
}

function saveNote() {
  const file = selectedFile.value;
  if (!file) return;
  clearDraftFromStorage(file.path);
  void runReviewMutation(t("review.toast.note.saved"), (refs) =>
    source().setFileVerdict(
      localReviewId(refs.base, refs.head),
      file.path,
      (file.status || "pending") as "approved" | "changes" | "pending",
      noteDraft.value
    )
  );
}

watch(noteDraft, (val) => {
  if (selectedPath.value) {
    saveDraftToStorage(selectedPath.value, val);
  }
});

function addComment(payload: { body: string; line: number; side: "new" | "old" }) {
  const file = selectedFile.value;
  const body = payload.body.trim();
  if (!file || !body || isMutating.value) return;
  void runReviewMutation(t("review.toast.comment.added"), (refs) =>
    source().addComment(localReviewId(refs.base, refs.head), {
      body,
      path: file.path,
      line: payload.line,
      side: payload.side,
    })
  );
}

function toggleComment(comment: ReviewComment) {
  const nextResolved = !comment.resolved;
  void runReviewMutation(nextResolved ? t("review.toast.comment.resolved") : t("review.toast.comment.reopened"), (refs) =>
    source().resolveComment(localReviewId(refs.base, refs.head), comment.id, nextResolved)
  );
}

async function handleDeleteComment(comment: ReviewComment) {
  if (!(await confirmAction({ message: t("review.confirm.delete.comment"), tone: "danger" }))) return;
  void runReviewMutation(t("review.toast.comment.deleted"), (refs) =>
    source().deleteComment(localReviewId(refs.base, refs.head), comment.id)
  );
}

async function clearSession() {
  const refs = currentRefs();
  const current = review.value;
  if (!refs || !current) return;
  const label = `${current.baseLabel || current.base} → ${current.headLabel || current.head}`;
  if (!(await confirmAction({ message: t("review.confirm.clear.session", { label }), tone: "danger" }))) return;

  const seq = ++actionSeq;
  const epoch = bootstrapSeq;
  isMutating.value = true;
  actionError.value = "";
  try {
    const message = await source().clear(localReviewId(refs.base, refs.head));
    if (seq !== actionSeq || epoch !== bootstrapSeq) return;
    showInfo(typeof message === "string" && message ? message : t("review.toast.session.cleared"));
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
    const markdown = await source().exportMarkdown(localReviewId(refs.base, refs.head));
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
    return;
  }
  const tag = (e.target as HTMLElement)?.tagName?.toLowerCase();
  if (tag === "input" || tag === "textarea") return;

  if (e.altKey && e.key === "ArrowUp") {
    e.preventDefault();
    selectAdjacentFile(-1);
  } else if (e.altKey && e.key === "ArrowDown") {
    e.preventDefault();
    selectAdjacentFile(1);
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
        <span class="text-xs font-semibold text-ink whitespace-nowrap">{{ t("review.title") }}</span>
      </div>

      <!-- Source switch: local base...head sessions vs remote PR/MR -->
      <div class="flex items-center rounded-lg border border-line bg-raised/60 p-0.5 shrink-0" role="group" :aria-label="t('review.mode.label')">
        <button
          type="button"
          class="px-2 py-0.5 rounded-md text-[10px] font-semibold bg-surface text-accent shadow-sm cursor-default"
          :aria-pressed="true"
        >
          {{ t("review.mode.local") }}
        </button>
        <button
          type="button"
          class="px-2 py-0.5 rounded-md text-[10px] font-semibold text-ink-muted hover:text-ink transition cursor-pointer"
          :aria-pressed="false"
          @click="emit('switch-mode', 'remote')"
        >
          {{ t("review.mode.remote") }}
        </button>
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

        <div class="flex items-center gap-1.5 shrink-0" :title="t('review.progress', { progress: review.stats.progress })">
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

        <span class="flex items-center gap-1 text-[10px] text-ink-muted shrink-0" :title="t('review.open.comments')">
          <MessageSquare class="w-3.5 h-3.5" />
          {{ review.stats.openComments }}
        </span>

        <span class="text-[10px] text-ink-muted shrink-0 hidden xl:inline">
          {{ t("review.files.approved", { approved: review.stats.approved, total: review.stats.total }) }}
        </span>
      </template>

      <div class="ml-auto flex items-center gap-1.5 shrink-0">
        <button
          v-if="review"
          @click="handleBatchSetStatus('approved')"
          :disabled="isMutating"
          class="flex items-center gap-1 px-2 py-1 rounded-lg bg-surface border border-line text-[11px] text-success hover:bg-success-bg transition cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
          :title="t('review.batch.approve.title')"
        >
          <Check class="w-3.5 h-3.5" />
          <span>{{ t("review.batch.approve") }}</span>
        </button>
        <button
          v-if="review"
          @click="handleBatchSetStatus('pending')"
          :disabled="isMutating"
          class="flex items-center gap-1 px-2 py-1 rounded-lg bg-surface border border-line text-[11px] text-ink-secondary hover:text-ink hover:bg-raised transition cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
          :title="t('review.batch.reset.title')"
        >
          <RotateCcw class="w-3.5 h-3.5" />
          <span>{{ t("review.batch.reset") }}</span>
        </button>
        <button
          v-if="review"
          @click="exportReport"
          :disabled="isExporting"
          class="flex items-center gap-1 px-2 py-1 rounded-lg bg-surface border border-line text-[11px] text-ink-secondary hover:text-ink hover:bg-raised transition cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
          :title="t('review.export.title')"
        >
          <Loader2 v-if="isExporting" class="w-3.5 h-3.5 animate-spin" />
          <Download v-else class="w-3.5 h-3.5" />
          {{ t("review.export") }}
        </button>
        <button
          v-if="review"
          @click="clearSession"
          :disabled="isMutating"
          class="flex items-center gap-1 px-2 py-1 rounded-lg bg-surface border border-line text-[11px] text-danger hover:bg-danger-bg transition cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
          :title="t('review.clear.title')"
        >
          <Trash2 class="w-3.5 h-3.5" />
          {{ t("review.clear") }}
        </button>
        <button
          @click="refreshAll"
          :disabled="isLoadingReview || isLoadingSessions || !currentRepoPath"
          class="flex items-center gap-1 px-2 py-1 rounded-lg bg-surface border border-line text-[11px] text-ink-secondary hover:text-ink hover:bg-raised transition cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
          :title="t('review.refresh.title')"
        >
          <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isLoadingReview || isLoadingSessions }" />
          {{ t("review.refresh") }}
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
      <button :aria-label="t('common.close')" @click="clearErrors" class="shrink-0 p-0.5 rounded hover:bg-danger/15 cursor-pointer" :title="t('common.close')">
        <X class="w-3.5 h-3.5" />
      </button>
    </div>

    <!-- No repo placeholder -->
    <div v-if="!currentRepoPath" class="flex-1 flex flex-col items-center justify-center gap-2.5 text-ink-muted">
      <FolderGit2 class="w-10 h-10 opacity-40" />
      <div class="text-sm font-medium text-ink-secondary">{{ t("review.error.no.repo") }}</div>
      <div class="text-xs">{{ t("review.empty.hint") }}</div>
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
        @approve-and-next="handleApproveAndNext"
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
