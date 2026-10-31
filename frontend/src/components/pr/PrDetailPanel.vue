<script setup lang="ts">
import { computed, ref, watch } from "vue";
import {
  ArrowRight,
  Check,
  CircleSlash,
  ExternalLink,
  GitMerge,
  Loader2,
  MessageSquarePlus,
  RefreshCw,
  ShieldCheck,
  X,
} from "lucide-vue-next";
import type { PrAnchor, PrComment, PrDetail } from "../../types";
import UiButton from "../ui/UiButton.vue";
import UiModal from "../ui/UiModal.vue";
import PrFileDiff from "./PrFileDiff.vue";
import {
  checksDot,
  checksLabel,
  formatRelativeTime,
  prStateChip,
  prStateLabel,
} from "./meta";
import { t } from "../../i18n";

const props = defineProps<{
  detail: PrDetail | null;
  loading: boolean;
  commentBusy: boolean;
  reviewBusy: boolean;
  mergeBusy: boolean;
  anchor: PrAnchor | null;
  commentEpoch: number;
}>();

const emit = defineEmits<{
  (e: "comment", payload: { body: string; path: string; line: number; side: string }): void;
  (e: "review", payload: { event: string; body: string }): void;
  (e: "merge", payload: { method: string }): void;
  (e: "anchor", payload: PrAnchor | null): void;
  (e: "open-external", url: string): void;
  (e: "refresh"): void;
}>();

type TabId = "description" | "files" | "discussion" | "checks";
const activeTab = ref<TabId>("description");
const expanded = ref<Record<string, boolean>>({});
const commentBody = ref("");
const mergeMethod = ref("merge");
const reviewModalOpen = ref(false);
const reviewEvent = ref("request_changes");
const reviewBody = ref("");

const summary = computed(() => props.detail?.summary ?? null);
const isOpen = computed(() => summary.value?.state === "open");
const reviewComments = computed(() => (props.detail?.comments ?? []).filter((c) => c.kind === "review"));

const tabs = computed(() => [
  { id: "description" as TabId, label: t("pr.tab.description"), count: 0 },
  { id: "files" as TabId, label: t("pr.tab.files"), count: props.detail?.files?.length ?? 0 },
  { id: "discussion" as TabId, label: t("pr.tab.discussion"), count: props.detail?.comments?.length ?? 0 },
  { id: "checks" as TabId, label: t("pr.tab.checks"), count: props.detail?.checks?.length ?? 0 },
]);

watch(
  () => props.commentEpoch,
  () => {
    commentBody.value = "";
  }
);

watch(
  () => props.detail?.summary.number,
  () => {
    activeTab.value = "description";
    expanded.value = {};
    commentBody.value = "";
    mergeMethod.value = "merge";
  }
);

function commentsFor(path: string): PrComment[] {
  return reviewComments.value.filter((c) => c.path === path);
}

function toggleFile(path: string) {
  expanded.value = { ...expanded.value, [path]: !expanded.value[path] };
}

function anchorTo(payload: PrAnchor) {
  emit("anchor", payload);
  commentBody.value = commentBody.value || "";
}

function submitComment() {
  const body = commentBody.value.trim();
  if (!body || props.commentBusy) return;
  emit("comment", {
    body,
    path: props.anchor?.path ?? "",
    line: props.anchor?.line ?? 0,
    side: props.anchor?.side ?? "new",
  });
}

function onComposerKeydown(event: KeyboardEvent) {
  if ((event.metaKey || event.ctrlKey) && event.key === "Enter") {
    event.preventDefault();
    submitComment();
  }
}

function openReviewModal(event: string) {
  reviewEvent.value = event;
  reviewBody.value = "";
  reviewModalOpen.value = true;
}

function confirmReview() {
  const body = reviewBody.value.trim();
  if (reviewEvent.value === "request_changes" && !body) return;
  emit("review", { event: reviewEvent.value, body });
  reviewModalOpen.value = false;
}

function commentTime(iso: string): string {
  return formatRelativeTime(iso);
}
</script>

<template>
  <section class="flex-1 flex flex-col min-w-0 min-h-0 bg-canvas text-ink">
    <div v-if="loading && !detail" class="flex-1 flex items-center justify-center gap-2 text-xs text-ink-muted">
      <Loader2 class="w-4 h-4 animate-spin" />
      {{ t("pr.loading") }}
    </div>

    <div v-else-if="!detail" class="flex-1 flex items-center justify-center text-xs text-ink-muted">
      {{ t("pr.empty.no.selection") }}
    </div>

    <template v-else-if="summary">
      <!-- Detail header -->
      <header class="shrink-0 border-b border-line px-4 py-3 space-y-2 bg-surface">
        <div class="flex items-start gap-2">
          <span class="text-[11px] font-mono text-ink-muted shrink-0 mt-0.5">#{{ summary.number }}</span>
          <h2 class="flex-1 min-w-0 text-sm font-bold leading-snug break-words">{{ summary.title }}</h2>
          <span
            class="text-[10px] font-bold px-1.5 py-0.5 rounded shrink-0"
            :class="prStateChip(summary.state, summary.isDraft)"
          >
            {{ prStateLabel(summary.state, summary.isDraft) }}
          </span>
          <span
            v-if="summary.mergeableState === 'conflicting' || summary.mergeable === false"
            class="text-[10px] px-1.5 py-0.5 rounded bg-danger/12 text-danger shrink-0"
          >
            {{ t("pr.conflict") }}
          </span>
        </div>

        <div class="flex items-center gap-2 flex-wrap text-[10px] text-ink-muted">
          <span class="font-semibold text-ink">{{ summary.author || "—" }}</span>
          <span class="flex items-center gap-1 font-mono">
            <span class="truncate max-w-[160px]" :title="summary.sourceBranch">{{ summary.sourceBranch }}</span>
            <ArrowRight class="w-3 h-3" />
            <span class="truncate max-w-[160px]" :title="summary.targetBranch">{{ summary.targetBranch }}</span>
          </span>
          <span class="font-mono text-success">+{{ summary.additions }}</span>
          <span class="font-mono text-danger">-{{ summary.deletions }}</span>
          <span>{{ t("pr.stat.files", { count: summary.changedFiles }) }}</span>
          <span v-if="detail.commitCount">{{ t("pr.stat.commits", { count: detail.commitCount }) }}</span>
          <span>{{ t("pr.updated.at", { time: formatRelativeTime(summary.updatedAt) }) }}</span>
          <span class="flex items-center gap-1">
            <span class="w-1.5 h-1.5 rounded-full" :class="checksDot(summary.checksState)"></span>
            {{ checksLabel(summary.checksState) }}
          </span>
          <span v-for="label in summary.labels ?? []" :key="label" class="px-1.5 py-0.5 rounded bg-accent/12 text-accent">
            {{ label }}
          </span>
        </div>
      </header>

      <!-- Actions -->
      <div class="shrink-0 border-b border-line px-4 py-2 flex items-center gap-2 flex-wrap bg-surface">
        <template v-if="isOpen">
          <UiButton variant="primary" size="xs" :disabled="reviewBusy" @click="emit('review', { event: 'approve', body: '' })">
            <ShieldCheck class="w-3 h-3" />
            {{ t("pr.review.approve") }}
          </UiButton>
          <UiButton variant="warning" size="xs" :disabled="reviewBusy" @click="openReviewModal('request_changes')">
            <MessageSquarePlus class="w-3 h-3" />
            {{ t("pr.review.request.changes") }}
          </UiButton>
          <UiButton variant="secondary" size="xs" :disabled="reviewBusy" @click="openReviewModal('comment')">
            {{ t("pr.review.comment") }}
          </UiButton>
          <span class="flex-1"></span>
          <label class="text-[10px] text-ink-muted">{{ t("pr.merge.title") }}</label>
          <select
            v-model="mergeMethod"
            class="px-2 py-1 rounded-lg bg-surface border border-line text-[11px] text-ink focus:outline-none focus:border-accent-strong"
            :aria-label="t('pr.merge.title')"
          >
            <option value="merge">merge</option>
            <option value="squash">squash</option>
            <option value="rebase">rebase</option>
          </select>
          <UiButton variant="purple" size="xs" :disabled="mergeBusy" @click="emit('merge', { method: mergeMethod })">
            <GitMerge class="w-3 h-3" />
            {{ t("pr.merge.action") }}
          </UiButton>
        </template>
        <template v-else>
          <span class="text-[11px] text-ink-muted flex items-center gap-1">
            <CircleSlash class="w-3 h-3" />
            {{ prStateLabel(summary.state, summary.isDraft) }}
          </span>
        </template>

        <span class="flex-1"></span>
        <UiButton variant="ghost" size="xs" :disabled="loading" @click="emit('refresh')">
          <RefreshCw class="w-3 h-3" :class="{ 'animate-spin': loading }" />
          {{ t("pr.refresh") }}
        </UiButton>
        <UiButton v-if="summary.webUrl" variant="ghost" size="xs" @click="emit('open-external', summary.webUrl)">
          <ExternalLink class="w-3 h-3" />
          {{ t("pr.open.browser") }}
        </UiButton>
      </div>

      <!-- Tabs -->
      <div class="shrink-0 border-b border-line px-2 flex items-center gap-1 overflow-x-auto" role="tablist">
        <button
          v-for="tab in tabs"
          :key="tab.id"
          type="button"
          role="tab"
          :aria-selected="activeTab === tab.id"
          class="shrink-0 whitespace-nowrap px-3 py-2 text-[11px] font-semibold border-b-2 -mb-px transition cursor-pointer"
          :class="
            activeTab === tab.id
              ? 'border-accent-strong text-accent'
              : 'border-transparent text-ink-muted hover:text-ink'
          "
          @click="activeTab = tab.id"
        >
          {{ tab.label }}
          <span v-if="tab.count" class="ml-1 font-mono text-[10px] text-ink-muted">{{ tab.count }}</span>
        </button>
      </div>

      <!-- Content -->
      <div class="flex-1 overflow-y-auto px-4 py-3 space-y-2 min-h-0">
        <template v-if="activeTab === 'description'">
          <p v-if="detail.body" class="text-xs leading-relaxed whitespace-pre-wrap break-words text-ink">
            {{ detail.body }}
          </p>
          <p v-else class="text-xs text-ink-muted">{{ t("common.empty") }}</p>
          <p v-if="detail.message" class="text-[10px] text-ink-muted">{{ detail.message }}</p>
        </template>

        <template v-else-if="activeTab === 'files'">
          <p v-if="detail.filesTruncated" class="text-[10px] text-warning">{{ t("pr.files.truncated") }}</p>
          <p v-if="!detail.files?.length" class="text-xs text-ink-muted">{{ t("pr.files.empty") }}</p>
          <PrFileDiff
            v-for="file in detail.files ?? []"
            :key="file.path + file.status"
            :file="file"
            :comments="commentsFor(file.path)"
            :expanded="!!expanded[file.path]"
            @toggle="toggleFile(file.path)"
            @anchor="anchorTo"
          />
        </template>

        <template v-else-if="activeTab === 'discussion'">
          <p v-if="!detail.comments?.length" class="text-xs text-ink-muted">{{ t("common.empty") }}</p>
          <div v-for="comment in detail.comments ?? []" :key="comment.kind + comment.id" class="border border-line rounded-xl px-3 py-2 space-y-1">
            <div class="flex items-center gap-2 text-[10px] text-ink-muted flex-wrap">
              <span class="font-semibold text-ink">{{ comment.author }}</span>
              <span class="px-1 py-0.5 rounded" :class="comment.kind === 'review' ? 'bg-accent/12 text-accent' : 'bg-raised text-ink-muted'">
                {{ comment.kind === "review" ? t("pr.comment.kind.review") : t("pr.comment.kind.issue") }}
              </span>
              <span v-if="comment.path" class="font-mono truncate max-w-[280px]" :title="comment.path">
                {{ comment.path }}<template v-if="comment.line">:{{ comment.line }}</template>
              </span>
              <span>{{ commentTime(comment.createdAt) }}</span>
            </div>
            <p class="text-[11px] text-ink whitespace-pre-wrap break-words">{{ comment.body }}</p>
          </div>
        </template>

        <template v-else>
          <p v-if="!detail.checks?.length" class="text-xs text-ink-muted">{{ t("pr.checks.empty") }}</p>
          <div
            v-for="check in detail.checks ?? []"
            :key="check.name + check.url"
            class="flex items-center gap-2 border border-line rounded-xl px-3 py-2"
          >
            <span class="w-1.5 h-1.5 rounded-full shrink-0" :class="checksDot(check.conclusion || check.status)"></span>
            <div class="min-w-0 flex-1">
              <p class="text-[11px] font-semibold truncate">{{ check.name }}</p>
              <p v-if="check.detail" class="text-[10px] text-ink-muted truncate">{{ check.detail }}</p>
            </div>
            <span class="text-[10px] font-mono text-ink-muted shrink-0">{{ check.conclusion || check.status }}</span>
            <button
              v-if="check.url"
              type="button"
              class="p-1 rounded text-ink-muted hover:text-accent hover:bg-raised cursor-pointer"
              :aria-label="t('pr.open.browser')"
              @click="emit('open-external', check.url)"
            >
              <ExternalLink class="w-3 h-3" aria-hidden="true" />
            </button>
          </div>
        </template>
      </div>

      <!-- Composer -->
      <footer v-if="isOpen" class="shrink-0 border-t border-line px-4 py-2 space-y-2 bg-surface">
        <div v-if="anchor" class="flex items-center gap-1 text-[10px]">
          <span class="px-1.5 py-0.5 rounded bg-accent/12 text-accent font-mono truncate max-w-[320px]">
            {{ t("pr.comment.anchor", { path: anchor.path, line: anchor.line }) }}
          </span>
          <button
            type="button"
            class="p-0.5 rounded text-ink-muted hover:text-ink cursor-pointer"
            :aria-label="t('pr.comment.clear.anchor')"
            @click="emit('anchor', null)"
          >
            <X class="w-3 h-3" aria-hidden="true" />
          </button>
        </div>
        <textarea
          v-model="commentBody"
          rows="2"
          :placeholder="t('pr.comment.placeholder')"
          class="w-full px-2.5 py-2 rounded-lg bg-canvas border border-line text-[11px] text-ink resize-y focus:outline-none focus:border-accent-strong"
          @keydown="onComposerKeydown"
        ></textarea>
        <div class="flex items-center justify-end gap-2">
          <UiButton variant="primary" size="xs" :disabled="commentBusy || !commentBody.trim()" @click="submitComment">
            <Loader2 v-if="commentBusy" class="w-3 h-3 animate-spin" />
            <Check v-else class="w-3 h-3" />
            {{ t("pr.comment.submit") }}
          </UiButton>
        </div>
      </footer>
    </template>

    <!-- Review verdict modal -->
    <UiModal
      :open="reviewModalOpen"
      :title="reviewEvent === 'request_changes' ? t('pr.review.request.changes') : t('pr.review.comment')"
      width="max-w-md"
      @update:open="reviewModalOpen = $event"
    >
      <textarea
        v-model="reviewBody"
        rows="5"
        :placeholder="t('pr.review.body.placeholder')"
        class="w-full px-2.5 py-2 rounded-lg bg-canvas border border-line text-xs text-ink resize-y focus:outline-none focus:border-accent-strong"
      ></textarea>
      <template #footer>
        <UiButton variant="secondary" @click="reviewModalOpen = false">{{ t("common.cancel") }}</UiButton>
        <UiButton
          :variant="reviewEvent === 'request_changes' ? 'warning' : 'primary'"
          :disabled="reviewBusy || (reviewEvent === 'request_changes' && !reviewBody.trim())"
          @click="confirmReview"
        >
          {{ t("common.confirm") }}
        </UiButton>
      </template>
    </UiModal>
  </section>
</template>
