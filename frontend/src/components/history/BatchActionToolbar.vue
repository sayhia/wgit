<script setup lang="ts">
import { GitCompare, GitBranch, X, Copy } from "lucide-vue-next";

defineProps<{ selectionCount: number }>();

const emit = defineEmits<{
  (e: "batch-cherry-pick"): void;
  (e: "copy-details"): void;
  (e: "rebase"): void;
  (e: "clear"): void;
}>();
</script>

<template>
      <!-- Multi-select batch actions toolbar -->
      <div
        v-if="selectionCount > 0"
        class="border-b border-line/60 px-4 py-1.5 flex items-center gap-2 bg-accent/5 shrink-0 flex-wrap"
      >
        <span class="text-[11px] font-semibold text-accent whitespace-nowrap">
          已选择 {{ selectionCount }} 条提交
        </span>
        <span class="text-[10px] text-ink-muted font-mono whitespace-nowrap">
          (Ctrl/⌘+点击、Shift+点击或复选框多选)
        </span>
        <div class="flex items-center gap-1.5 ml-auto">
          <button
            @click="emit('batch-cherry-pick')"
            class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-canvas hover:bg-raised text-[11px] font-medium text-ink transition cursor-pointer border border-line/60"
            title="按从旧到新的顺序批量 Cherry-Pick 所选提交"
          >
            <GitCompare class="w-3.5 h-3.5 text-accent" />
            <span>批量 Cherry-Pick</span>
          </button>
          <button
            @click="emit('copy-details')"
            class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-canvas hover:bg-raised text-[11px] font-medium text-ink transition cursor-pointer border border-line/60"
            title="复制所选提交的完整详情（Hash / 作者 / 时间 / 标题 / 正文）"
          >
            <Copy class="w-3.5 h-3.5 text-purple" />
            <span>复制详情</span>
          </button>
          <button
            @click="emit('rebase')"
            class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-canvas hover:bg-raised text-[11px] font-medium text-ink transition cursor-pointer border border-line/60"
            title="交互式变基：pick / reword / squash / drop 等"
          >
            <GitBranch class="w-3.5 h-3.5 text-cyan" />
            <span>交互式变基</span>
          </button>
          <button
            @click="emit('clear')"
            class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-raised hover:bg-hover-strong text-[11px] text-ink-secondary hover:text-ink transition cursor-pointer border border-line"
            title="清空多选 (Esc)"
          >
            <X class="w-3 h-3" />
            <span>清空选择</span>
          </button>
        </div>
      </div>

</template>
