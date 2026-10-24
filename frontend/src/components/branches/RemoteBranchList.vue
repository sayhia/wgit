<script setup lang="ts">
import { GitFork, GitBranch, ArrowDownCircle } from "lucide-vue-next";
import type { BranchInfo } from "../../types";
import { useRovingList } from "../../composables/useRovingList";
import { t } from "../../i18n";

const props = defineProps<{
  total: number;
  visible: BranchInfo[];
  moreCount: number;
}>();

const emit = defineEmits<{
  (e: "checkout", name: string): void;
  (e: "show-more"): void;
}>();

const { setItemRef, tabindexFor, onKeydown: onRowKeydown, onFocus: onRowFocus } = useRovingList(
  () => props.visible,
  (branch) => branch.name
);
</script>

<template>
        <div v-if="total === 0" class="py-12 text-center text-xs text-ink-muted border border-dashed border-line rounded-2xl">
          {{ t("branches.remote.empty") }}
        </div>

        <div v-else class="flex items-center justify-between px-1 text-[11px] text-ink-muted">
          <span>{{ t("branches.remote.total", { count: total }) }}</span>
          <span v-if="moreCount > 0">{{ t("branches.remote.showing", { count: visible.length }) }}</span>
        </div>

        <div
          v-for="(b, index) in visible"
          :key="b.name"
          :ref="(el) => setItemRef(b, el)"
          :tabindex="tabindexFor(b, index)"
          :aria-label="t('branches.remote.row.aria', { name: b.name })"
          class="flex items-center justify-between gap-2 p-3 rounded-xl bg-surface/60 border border-line hover:border-line-strong transition focus:outline-none focus-visible:ring-2 focus-visible:ring-accent/45"
          @focus="onRowFocus(b)"
          @keydown="onRowKeydown($event, index)"
          @keydown.enter.self="emit('checkout', b.name)"
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
                  :title="t('branches.remote.latest.hint')"
                >
                  {{ b.commitHash.substring(0, 8) }}
                </span>
                <span
                  v-if="b.ahead"
                  class="px-1.5 py-0.5 rounded border border-accent/30 bg-accent/10 text-[10px] text-accent font-bold font-mono"
                  :title="t('branches.local.ahead.hint', { count: b.ahead })"
                >↑{{ b.ahead }}</span>
                <span
                  v-if="b.behind"
                  class="px-1.5 py-0.5 rounded border border-cyan/30 bg-cyan/10 text-[10px] text-cyan font-bold font-mono"
                  :title="t('branches.local.behind.hint', { count: b.behind })"
                >↓{{ b.behind }}</span>
              </div>
              <span class="text-[11px] text-ink-muted mt-0.5">{{ t("branches.remote.kind") }}</span>
            </div>
          </div>
          <button
            @click="emit('checkout', b.name)"
            class="flex items-center gap-1 px-3 py-1 rounded-lg bg-raised hover:bg-hover text-cyan text-xs font-medium transition cursor-pointer border border-line shrink-0"
          >
            <GitBranch class="w-3 h-3" />
            <span>{{ t("branches.remote.checkout") }}</span>
          </button>
        </div>

        <button
          v-if="moreCount > 0"
          @click="emit('show-more')"
          class="w-full py-2 rounded-xl border border-dashed border-line text-xs text-ink-secondary hover:text-ink hover:border-line-strong transition cursor-pointer flex items-center justify-center gap-1.5"
        >
          <ArrowDownCircle class="w-3.5 h-3.5" />
          <span>{{ t("branches.remote.show.more", { count: moreCount }) }}</span>
        </button>
</template>
