<script setup lang="ts">
import { computed, ref } from "vue";
import { Loader2, FileText, MessageSquare, Check, AlertCircle, RefreshCw, Send, GitPullRequest, Trash2, ArrowDown } from "lucide-vue-next";
import type { ChangeReview, ReviewFileState, ReviewComment } from "../../types";
import { changeTypeMeta, fileStatus } from "./meta";
import { t } from "../../i18n";

const props = defineProps<{
  review: ChangeReview | null;
  selectedFile: ReviewFileState | null;
  filesCount: number;
  diffText: string;
  diffError: string;
  isLoadingDiff: boolean;
  isLoadingReview: boolean;
  isMutating: boolean;
  comments: ReviewComment[];
  noteDraft: string;
  noteDirty: boolean;
}>();

const emit = defineEmits<{
  (e: "update:noteDraft", value: string): void;
  (e: "set-state", status: "approved" | "changes" | "pending"): void;
  (e: "approve-and-next"): void;
  (e: "save-note"): void;
  (e: "add-comment", payload: { body: string; line: number; side: "new" | "old" }): void;
  (e: "toggle-comment", comment: ReviewComment): void;
  (e: "delete-comment", comment: ReviewComment): void;
}>();

const noteModel = computed({
  get: () => props.noteDraft,
  set: (value: string) => emit("update:noteDraft", value),
});

const showAllDiff = ref(false);
const commentLine = ref(1);
const commentSide = ref<"new" | "old">("new");
const commentBody = ref("");

const openCommentsForSelection = computed(() => props.comments.filter((comment) => !comment.resolved).length);

type DiffKind = "add" | "del" | "context" | "hunk" | "meta";
interface DiffLine {
  kind: DiffKind;
  text: string;
  oldLine?: number;
  newLine?: number;
}
const MAX_DIFF_LINES = 4000;

// -------------------------------------------------------------- diff ----
function parseUnifiedDiff(raw: string): DiffLine[] {
  if (!raw) return [];
  const rawLines = raw.split(/\r?\n/);
  if (rawLines.length > 0 && rawLines[rawLines.length - 1] === "") rawLines.pop();

  const metaPrefixes = [
    "index ",
    "new file mode",
    "deleted file mode",
    "old mode",
    "new mode",
    "similarity index",
    "rename from",
    "rename to",
    "Binary files",
    "GIT binary patch",
    "\\ No newline",
  ];
  const hunkPattern = /^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@/;
  const parsed: DiffLine[] = [];
  let oldLine = 0;
  let newLine = 0;
  // 进入 hunk 后，"--- "/"+++ " 开头的行可能是以 "-- "/"++ " 开头的内容删除/新增行，
  // 因此文件头只能出现在第一个 hunk 之前（或下一个 diff --git 之后）。
  let inHunk = false;

  for (const text of rawLines) {
    const hunk = hunkPattern.exec(text);
    if (hunk) {
      inHunk = true;
      oldLine = Number(hunk[1] ?? 0);
      newLine = Number(hunk[3] ?? 0);
      parsed.push({ kind: "hunk", text });
      continue;
    }
    if (text.startsWith("diff --git")) {
      inHunk = false;
      parsed.push({ kind: "meta", text });
      continue;
    }
    if (!inHunk && (text.startsWith("--- ") || text.startsWith("+++ "))) {
      parsed.push({ kind: "meta", text });
      continue;
    }
    if (metaPrefixes.some((prefix) => text.startsWith(prefix))) {
      parsed.push({ kind: "meta", text });
      continue;
    }
    if (text.startsWith("+")) {
      parsed.push({ kind: "add", text, newLine });
      newLine += 1;
      continue;
    }
    if (text.startsWith("-")) {
      parsed.push({ kind: "del", text, oldLine });
      oldLine += 1;
      continue;
    }
    parsed.push({
      kind: "context",
      text,
      oldLine: oldLine > 0 ? oldLine : undefined,
      newLine: newLine > 0 ? newLine : undefined,
    });
    oldLine += 1;
    newLine += 1;
  }
  return parsed;
}

const diffLines = computed<DiffLine[]>(() => parseUnifiedDiff(props.diffText));
const isDiffTruncated = computed(() => diffLines.value.length > MAX_DIFF_LINES && !showAllDiff.value);
const renderedDiffLines = computed<DiffLine[]>(() =>
  showAllDiff.value ? diffLines.value : diffLines.value.slice(0, MAX_DIFF_LINES)
);

function diffLineClass(line: DiffLine): string {
  switch (line.kind) {
    case "add":
      return "bg-success/10 text-success";
    case "del":
      return "bg-danger/10 text-danger";
    case "hunk":
      return "bg-surface/80 text-accent font-semibold";
    case "meta":
      return "text-ink-muted";
    default:
      return "text-ink-secondary";
  }
}

function diffPrefix(line: DiffLine): string {
  switch (line.kind) {
    case "add":
      return "+";
    case "del":
      return "-";
    case "hunk":
      return "@";
    case "meta":
      return "";
    default:
      return " ";
  }
}

function isCommentTarget(line: DiffLine): boolean {
  if (line.kind === "meta" || line.kind === "hunk") return false;
  if (commentSide.value === "new") return line.newLine === commentLine.value;
  return line.oldLine === commentLine.value;
}

function handleDiffLineClick(line: DiffLine) {
  if (line.kind === "add" || (line.kind === "context" && typeof line.newLine === "number")) {
    commentSide.value = "new";
    commentLine.value = line.newLine ?? 1;
  } else if (line.kind === "del" && typeof line.oldLine === "number") {
    commentSide.value = "old";
    commentLine.value = line.oldLine;
  }
}


function addCommentLocal() {
  const file = props.selectedFile;
  const body = commentBody.value.trim();
  if (!file || !body || props.isMutating) return;
  const parsedLine = Number(commentLine.value);
  const line = Number.isFinite(parsedLine) && parsedLine > 0 ? Math.floor(parsedLine) : 1;
  emit("add-comment", { body, line, side: commentSide.value });
  commentBody.value = "";
}
</script>

<template>
      <!-- Column 3: file detail (verdict + diff) and comments -->
      <main class="flex-1 flex flex-col min-w-0 min-h-0">
        <!-- Empty / loading state when no session loaded -->
        <div
          v-if="!review"
          class="flex-1 flex flex-col items-center justify-center gap-2.5 text-ink-muted"
        >
          <Loader2 v-if="isLoadingReview" class="w-8 h-8 animate-spin opacity-50" />
          <GitPullRequest v-else class="w-10 h-10 opacity-30" />
          <div class="text-sm font-medium text-ink-secondary">
            {{ isLoadingReview ? t("review.file.loading") : t("review.file.empty") }}
          </div>
          <div class="text-xs">{{ t("review.file.empty.hint") }}</div>
        </div>

        <template v-else>
          <!-- File detail header: path, status, verdict actions, note -->
          <div v-if="selectedFile" class="shrink-0 border-b border-line bg-surface/30 px-3 py-2 space-y-2">
            <div class="flex items-center gap-1.5 min-w-0">
              <FileText class="w-3.5 h-3.5 text-ink-muted shrink-0" />
              <span class="text-[11px] font-mono text-ink truncate" :title="selectedFile.path">{{ selectedFile.path }}</span>
              <span class="shrink-0 px-1 py-0.5 rounded border text-[9px] font-mono" :class="changeTypeMeta(selectedFile.changeType).chip">
                {{ t(changeTypeMeta(selectedFile.changeType).labelKey) }}
              </span>
              <span class="shrink-0 px-1 py-0.5 rounded border text-[9px]" :class="fileStatus(selectedFile.status).chip">
                {{ t(fileStatus(selectedFile.status).labelKey) }}
              </span>
              <span class="shrink-0 text-[9px] font-mono text-success">+{{ selectedFile.additions }}</span>
              <span class="shrink-0 text-[9px] font-mono text-danger">-{{ selectedFile.deletions }}</span>
              <span
                class="flex items-center gap-0.5 shrink-0 text-[9px] text-ink-muted"
                :title="
                  t('review.file.comments.tooltip', {
                    count: selectedFile.commentCount,
                    open: selectedFile.openCommentCount,
                  })
                "
              >
                <MessageSquare class="w-3 h-3" />{{ selectedFile.commentCount }}
              </span>
              <span v-if="selectedFile.updatedAt" class="ml-auto shrink-0 text-[9px] font-mono text-ink-muted">
                {{ t("review.file.updated", { date: selectedFile.updatedAt }) }}
              </span>
            </div>

            <div class="flex items-start gap-2">
              <textarea
                v-model="noteModel"
                rows="2"
                :placeholder="t('review.file.note.placeholder')"
                class="flex-1 min-w-0 resize-none bg-canvas/60 border border-line rounded-lg px-2 py-1 text-[11px] text-ink placeholder-ink-muted focus:outline-none focus:border-accent-border"
              ></textarea>
              <div class="flex flex-col gap-1 shrink-0">
                <div class="flex items-center gap-1">
                  <button
                    @click="emit('set-state', 'approved')"
                    :disabled="isMutating"
                    class="flex items-center gap-1 px-2 py-1 rounded-md border text-[11px] font-medium transition cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
                    :class="
                      selectedFile.status === 'approved'
                        ? 'bg-success-bg text-success border-success-border'
                        : 'bg-surface border-line text-ink-secondary hover:text-ink hover:bg-raised'
                    "
                  >
                    <Check class="w-3 h-3" />{{ t("review.file.approve") }}
                  </button>
                  <button
                    @click="emit('approve-and-next')"
                    :disabled="isMutating"
                    :title="t('review.file.approve.next.title')"
                    class="flex items-center gap-1 px-2 py-1 rounded-md border text-[11px] font-medium transition cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed border-line bg-surface text-ink-secondary hover:text-ink hover:bg-raised"
                  >
                    <ArrowDown class="w-3 h-3 text-success" />{{ t("review.file.approve.next") }}
                  </button>
                  <button
                    @click="emit('set-state', 'changes')"
                    :disabled="isMutating"
                    class="flex items-center gap-1 px-2 py-1 rounded-md border text-[11px] font-medium transition cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
                    :class="
                      selectedFile.status === 'changes'
                        ? 'bg-danger-bg text-danger border-danger-border'
                        : 'bg-surface border-line text-ink-secondary hover:text-ink hover:bg-raised'
                    "
                  >
                    <AlertCircle class="w-3 h-3" />{{ t("review.file.changes") }}
                  </button>
                  <button
                    @click="emit('set-state', 'pending')"
                    :disabled="isMutating"
                    class="flex items-center gap-1 px-2 py-1 rounded-md border border-line bg-surface text-[11px] text-ink-secondary hover:text-ink hover:bg-raised transition cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
                  >
                    <RefreshCw class="w-3 h-3" />{{ t("review.file.pending") }}
                  </button>
                </div>
                <button
                  @click="emit('save-note')"
                  :disabled="isMutating || !noteDirty"
                  class="flex items-center justify-center gap-1 px-2 py-1 rounded-md border border-line bg-surface text-[10px] text-ink-secondary hover:text-ink hover:bg-raised transition cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
                >
                  {{ t("review.file.save.note") }}
                </button>
              </div>
            </div>
          </div>

          <!-- Diff viewer -->
          <div class="flex-1 overflow-auto bg-canvas/40 min-h-0">
            <div v-if="isLoadingDiff" class="h-full flex flex-col items-center justify-center gap-2 text-ink-muted">
              <Loader2 class="w-5 h-5 animate-spin" />
              <span class="text-[11px]">{{ t("review.file.diff.loading") }}</span>
            </div>
            <div v-else-if="diffError" class="p-3 text-[11px] text-danger flex items-center gap-1.5">
              <AlertCircle class="w-3.5 h-3.5 shrink-0" />
              <span class="break-words">{{ diffError }}</span>
            </div>
            <div v-else-if="!selectedFile" class="h-full flex items-center justify-center text-[11px] text-ink-muted">
              {{ filesCount === 0 ? t("review.file.diff.no.files") : t("review.file.diff.select") }}
            </div>
            <div v-else-if="selectedFile.isBinary" class="h-full flex items-center justify-center text-[11px] text-ink-muted">
              {{ t("review.file.diff.binary") }}
            </div>
            <div v-else-if="diffLines.length === 0" class="h-full flex items-center justify-center text-[11px] text-ink-muted">
              {{ t("review.file.diff.empty") }}
            </div>
            <template v-else>
              <div
                v-if="isDiffTruncated"
                class="sticky top-0 z-10 flex items-center justify-between gap-2 px-3 py-1.5 bg-warning-bg border-b border-warning-border text-[10px] text-warning"
              >
                <span class="flex items-center gap-1.5">
                  <AlertCircle class="w-3.5 h-3.5 shrink-0" />
                  {{ t("review.file.diff.truncated", { total: diffLines.length, limit: MAX_DIFF_LINES }) }}
                </span>
                <button
                  @click="showAllDiff = true"
                  class="shrink-0 px-2 py-0.5 rounded border border-warning-border bg-surface/60 hover:bg-raised transition cursor-pointer"
                >
                  {{ t("review.file.diff.show.all") }}
                </button>
              </div>
              <div v-else-if="diffLines.length > MAX_DIFF_LINES && showAllDiff" class="sticky top-0 z-10 flex items-center justify-between gap-2 px-3 py-1.5 bg-surface/90 border-b border-line text-[10px] text-ink-muted">
                <span>{{ t("review.file.diff.expanded", { total: diffLines.length }) }}</span>
                <button
                  @click="showAllDiff = false"
                  class="shrink-0 px-2 py-0.5 rounded border border-line bg-surface hover:bg-raised transition cursor-pointer"
                >
                  {{ t("review.file.diff.collapse") }}
                </button>
              </div>
              <div class="font-mono text-[11px] leading-5 min-w-max py-1">
                <div
                  v-for="(line, idx) in renderedDiffLines"
                  :key="idx"
                  @click="handleDiffLineClick(line)"
                  class="flex items-start hover:bg-surface/60 transition-colors cursor-pointer"
                  :class="[diffLineClass(line), isCommentTarget(line) ? 'ring-1 ring-inset ring-accent/50' : '']"
                >
                  <span class="w-11 shrink-0 text-right pr-2 text-[10px] text-ink-muted select-none font-mono">
                    {{ line.oldLine ?? "" }}
                  </span>
                  <span class="w-11 shrink-0 text-right pr-2 text-[10px] text-ink-muted select-none border-r border-line/40 font-mono">
                    {{ line.newLine ?? "" }}
                  </span>
                  <span class="w-4 shrink-0 text-center select-none font-bold">{{ diffPrefix(line) }}</span>
                  <span class="pl-2 whitespace-pre">{{ line.text }}</span>
                </div>
              </div>
            </template>
          </div>

          <!-- Comments panel for the selected file -->
          <section class="h-56 shrink-0 border-t border-line flex flex-col bg-surface/20 min-h-0">
            <div class="h-8 shrink-0 px-3 border-b border-line flex items-center gap-1.5">
              <MessageSquare class="w-3.5 h-3.5 text-ink-muted" />
              <span class="text-[11px] font-semibold text-ink">{{ t("review.file.comments.title") }}</span>
              <span class="text-[10px] font-mono text-ink-muted">
                {{ t("review.file.comments.count", { count: comments.length }) }}
                <template v-if="openCommentsForSelection > 0">
                  · {{ t("review.file.comments.open", { count: openCommentsForSelection }) }}
                </template>
              </span>
              <span class="ml-auto text-[9px] text-ink-muted hidden lg:inline">{{ t("review.file.comments.hint") }}</span>
            </div>
            <div class="flex-1 flex min-h-0">
              <div class="flex-1 overflow-y-auto p-2 space-y-1.5 min-w-0">
                <div v-if="comments.length === 0" class="h-full flex items-center justify-center text-[11px] text-ink-muted">
                  {{ t("review.file.comments.empty") }}
                </div>
                <div
                  v-for="comment in comments"
                  :key="comment.id"
                  class="px-2.5 py-1.5 rounded-lg border min-w-0"
                  :class="comment.resolved ? 'bg-surface/30 border-line/60 opacity-75' : 'bg-surface/60 border-line'"
                >
                  <div class="flex items-center gap-1.5 text-[9px] text-ink-muted min-w-0">
                    <span class="shrink-0 px-1 py-0.5 rounded bg-raised border border-line font-mono text-accent">
                      {{ comment.side === "old" ? t("review.file.side.old") : t("review.file.side.new") }} L{{ comment.line }}
                    </span>
                    <span class="truncate">{{ comment.author }}</span>
                    <span class="shrink-0 hidden md:inline">{{ comment.createdAt }}</span>
                    <span v-if="comment.resolved" class="shrink-0 px-1 py-0.5 rounded bg-success-bg text-success border border-success-border">
                      {{ t("review.file.comments.resolved") }}
                    </span>
                    <button
                      @click="emit('toggle-comment', comment)"
                      :disabled="isMutating"
                      class="ml-auto shrink-0 px-1.5 py-0.5 rounded border text-[9px] transition cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
                      :class="
                        comment.resolved
                          ? 'border-line bg-surface text-ink-secondary hover:text-ink'
                          : 'border-success-border bg-success-bg text-success hover:bg-raised'
                      "
                    >
                      {{ comment.resolved ? t("review.file.comments.reopen") : t("review.file.comments.resolve") }}
                    </button>
                    <button :aria-label="t('review.file.comments.delete')"
                      @click="emit('delete-comment', comment)"
                      :disabled="isMutating"
                      class="shrink-0 p-1 rounded border border-line bg-surface text-ink-muted hover:text-danger hover:border-danger-border transition cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
                      :title="t('review.file.comments.delete')"
                    >
                      <Trash2 class="w-3 h-3" />
                    </button>
                  </div>
                  <div class="mt-1 text-[11px] text-ink-secondary whitespace-pre-wrap break-words">{{ comment.body }}</div>
                </div>
              </div>

              <!-- Add comment form -->
              <div class="w-56 xl:w-64 shrink-0 border-l border-line p-2 flex flex-col gap-1.5">
                <div class="flex items-center gap-1.5">
                  <label class="flex items-center gap-1 text-[10px] text-ink-muted shrink-0">
                    {{ t("review.file.comment.line") }}
                    <input
                      v-model.number="commentLine"
                      type="number"
                      min="1"
                      class="w-14 bg-canvas/60 border border-line rounded px-1 py-0.5 text-[11px] font-mono text-ink focus:outline-none focus:border-accent-border"
                    />
                  </label>
                  <select
                    v-model="commentSide"
                    class="flex-1 min-w-0 bg-canvas/60 border border-line rounded px-1 py-0.5 text-[10px] text-ink focus:outline-none focus:border-accent-border"
                  >
                    <option value="new">{{ t("review.file.comment.side.new") }}</option>
                    <option value="old">{{ t("review.file.comment.side.old") }}</option>
                  </select>
                </div>
                <textarea
                  v-model="commentBody"
                  rows="2"
                  :placeholder="t('review.file.comment.placeholder')"
                  class="flex-1 min-h-[40px] resize-none bg-canvas/60 border border-line rounded-lg px-2 py-1 text-[11px] text-ink placeholder-ink-muted focus:outline-none focus:border-accent-border"
                ></textarea>
                <button
                  @click="addCommentLocal"
                  :disabled="isMutating || !commentBody.trim() || !selectedFile"
                  class="w-full flex items-center justify-center gap-1 py-1 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-[11px] font-medium transition cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
                >
                  <Loader2 v-if="isMutating" class="w-3 h-3 animate-spin" />
                  <Send v-else class="w-3 h-3" />
                  {{ t("review.file.comment.add") }}
                </button>
              </div>
            </div>
          </section>
        </template>
      </main>
</template>
