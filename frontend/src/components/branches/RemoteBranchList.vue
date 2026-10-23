<script setup lang="ts">
import { GitFork, GitBranch, ArrowDownCircle } from "lucide-vue-next";
import type { BranchInfo } from "../../types";

defineProps<{
  total: number;
  visible: BranchInfo[];
  moreCount: number;
}>();

const emit = defineEmits<{
  (e: "checkout", name: string): void;
  (e: "show-more"): void;
}>();
</script>

<template>
        <div v-if="total === 0" class="py-12 text-center text-xs text-ink-muted border border-dashed border-line rounded-2xl">
          没有匹配的远程跟踪分支。
        </div>

        <div v-else class="flex items-center justify-between px-1 text-[11px] text-ink-muted">
          <span>共 {{ total }} 个远程跟踪分支</span>
          <span v-if="moreCount > 0">当前显示前 {{ visible.length }} 个</span>
        </div>

        <div
          v-for="b in visible"
          :key="b.name"
          class="flex items-center justify-between gap-2 p-3 rounded-xl bg-surface/60 border border-line hover:border-line-strong transition"
        >
          <div class="flex items-center gap-3 min-w-0">
            <div class="w-8 h-8 rounded-lg bg-cyan/10 text-cyan flex items-center justify-center shrink-0">
              <GitFork class="w-4 h-4" />
            </div>
            <div class="flex flex-col min-w-0">
              <div class="flex items-center gap-2 flex-wrap">
                <span class="font-mono text-xs font-bold text-ink">{{ b.name }}</span>
                <span
                  v-if="b.commitHash"
                  class="font-mono text-[10px] text-ink-muted bg-raised px-1.5 py-0.5 rounded border border-line"
                  title="最新提交"
                >
                  {{ b.commitHash.substring(0, 8) }}
                </span>
                <span
                  v-if="b.ahead"
                  class="px-1.5 py-0.5 rounded border border-accent/30 bg-accent/10 text-[10px] text-accent font-bold font-mono"
                  :title="`领先上游 ${b.ahead} 个提交`"
                >↑{{ b.ahead }}</span>
                <span
                  v-if="b.behind"
                  class="px-1.5 py-0.5 rounded border border-cyan/30 bg-cyan/10 text-[10px] text-cyan font-bold font-mono"
                  :title="`落后上游 ${b.behind} 个提交`"
                >↓{{ b.behind }}</span>
              </div>
              <span class="text-[11px] text-ink-muted mt-0.5">远程跟踪分支 · 可检出为本地分支</span>
            </div>
          </div>
          <button
            @click="emit('checkout', b.name)"
            class="flex items-center gap-1 px-3 py-1 rounded-lg bg-raised hover:bg-hover text-cyan text-xs font-medium transition cursor-pointer border border-line shrink-0"
          >
            <GitBranch class="w-3 h-3" />
            <span>跟踪检出至本地</span>
          </button>
        </div>

        <button
          v-if="moreCount > 0"
          @click="emit('show-more')"
          class="w-full py-2 rounded-xl border border-dashed border-line text-xs text-ink-secondary hover:text-ink hover:border-line-strong transition cursor-pointer flex items-center justify-center gap-1.5"
        >
          <ArrowDownCircle class="w-3.5 h-3.5" />
          <span>显示更多 (+{{ moreCount }})</span>
        </button>
</template>
