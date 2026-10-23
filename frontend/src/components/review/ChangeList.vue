<script setup lang="ts">
import { computed } from "vue";
import { Loader2, FileText, MessageSquare, GitCommitHorizontal, ChevronDown, ChevronRight } from "lucide-vue-next";
import type { ChangeReview, ReviewFileState, ReviewCommitItem } from "../../types";
import { changeTypeMeta, fileStatus } from "./meta";

const props = defineProps<{
  review: ChangeReview | null;
  files: ReviewFileState[];
  commits: ReviewCommitItem[];
  selectedPath: string;
  commitsOpen: boolean;
  isLoadingReview: boolean;
}>();

const emit = defineEmits<{
  (e: "select-file", file: ReviewFileState): void;
  (e: "update:commitsOpen", value: boolean): void;
}>();

const commitsOpenModel = computed({
  get: () => props.commitsOpen,
  set: (value: boolean) => emit("update:commitsOpen", value),
});
</script>

<template>
      <!-- Column 2: changed files + commits -->
      <section class="w-56 xl:w-72 shrink-0 border-r border-line flex flex-col bg-surface/20 min-h-0">
        <div class="h-9 shrink-0 px-3 border-b border-line flex items-center gap-1.5">
          <FileText class="w-3.5 h-3.5 text-ink-muted" />
          <span class="text-[11px] font-semibold text-ink">变更文件</span>
          <span class="text-[10px] font-mono text-ink-muted">{{ files.length }}</span>
          <span v-if="review" class="ml-auto flex items-center gap-1.5 text-[9px] font-mono">
            <span class="text-success">+{{ review.stats.additions }}</span>
            <span class="text-danger">-{{ review.stats.deletions }}</span>
          </span>
        </div>

        <div class="flex-1 overflow-y-auto p-1.5 space-y-1 min-h-0">
          <div v-if="isLoadingReview && !review" class="py-8 text-center text-[11px] text-ink-muted">
            <Loader2 class="w-4 h-4 animate-spin mx-auto mb-1.5" />
            正在加载评审会话…
          </div>
          <div v-else-if="!review" class="py-8 px-3 text-center text-[11px] text-ink-muted">
            尚未加载评审会话
            <div class="mt-1 text-[10px]">输入 base / head 后点击「开始 / 加载评审」</div>
          </div>
          <div v-else-if="files.length === 0" class="py-8 text-center text-[11px] text-ink-muted">
            所选范围没有文件变更
          </div>
          <button
            v-for="file in files"
            :key="file.path"
            @click="emit('select-file', file)"
            class="w-full text-left px-2 py-1.5 rounded-lg border transition cursor-pointer min-w-0"
            :class="
              selectedPath === file.path
                ? 'bg-raised border-accent/40 shadow-xs'
                : 'bg-surface/40 border-line hover:bg-raised/60'
            "
            :title="file.path"
          >
            <div class="flex items-center gap-1.5 min-w-0">
              <span
                class="shrink-0 px-1 py-0.5 rounded border text-[9px] font-mono"
                :class="changeTypeMeta(file.changeType).chip"
              >
                {{ changeTypeMeta(file.changeType).label }}
              </span>
              <span class="text-[11px] text-ink truncate">{{ file.path }}</span>
            </div>
            <div class="mt-0.5 flex items-center gap-2 text-[9px] font-mono text-ink-muted">
              <span class="text-success shrink-0">+{{ file.additions }}</span>
              <span class="text-danger shrink-0">-{{ file.deletions }}</span>
              <span class="flex items-center gap-0.5 shrink-0" :title="`${file.commentCount} 条评论`">
                <MessageSquare class="w-2.5 h-2.5" />{{ file.commentCount }}
              </span>
              <span v-if="file.openCommentCount > 0" class="text-warning shrink-0">
                {{ file.openCommentCount }} 待处理
              </span>
              <span
                class="ml-auto shrink-0 px-1 py-0.5 rounded border text-[9px]"
                :class="fileStatus(file.status).chip"
              >
                {{ fileStatus(file.status).label }}
              </span>
            </div>
          </button>
        </div>

        <!-- Commits of the review range -->
        <div v-if="review" class="shrink-0 border-t border-line max-h-48 flex flex-col min-h-0">
          <button
            @click="commitsOpenModel = !commitsOpenModel"
            class="h-8 px-3 shrink-0 flex items-center justify-between text-[10px] text-ink-muted hover:text-ink transition cursor-pointer"
          >
            <span class="flex items-center gap-1.5">
              <GitCommitHorizontal class="w-3.5 h-3.5" />
              提交范围 ({{ commits.length }})
            </span>
            <component :is="commitsOpenModel ? ChevronDown : ChevronRight" class="w-3.5 h-3.5" />
          </button>
          <div v-if="commitsOpenModel" class="overflow-y-auto px-1.5 pb-1.5 space-y-0.5 min-h-0">
            <div v-if="commits.length === 0" class="py-2 text-center text-[10px] text-ink-muted">
              该范围没有提交记录
            </div>
            <div v-for="commit in commits" :key="commit.hash" class="px-2 py-1 rounded-lg hover:bg-raised/60 min-w-0">
              <div class="flex items-center gap-1.5 text-[9px] text-ink-muted min-w-0">
                <span class="font-mono text-accent shrink-0">{{ commit.shortHash || commit.hash.slice(0, 7) }}</span>
                <span class="truncate">{{ commit.author }}</span>
                <span class="ml-auto shrink-0">{{ commit.date }}</span>
              </div>
              <div class="text-[10px] text-ink-secondary truncate">{{ commit.subject }}</div>
            </div>
          </div>
        </div>
      </section>

</template>
