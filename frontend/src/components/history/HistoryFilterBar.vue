<script setup lang="ts">
import { computed } from "vue";
import { Search, User, Calendar, X, Download, StickyNote } from "lucide-vue-next";
import { t } from "../../i18n";

const props = defineProps<{
  keyword: string;
  author: string;
  since: string;
  until: string;
  hasActiveFilters: boolean;
  isFilterMode: boolean;
  activeFilterSummary: string;
}>();

const emit = defineEmits<{
  (e: "update:keyword", value: string): void;
  (e: "update:author", value: string): void;
  (e: "update:since", value: string): void;
  (e: "update:until", value: string): void;
  (e: "clear"): void;
  (e: "export-report"): void;
  (e: "open-pickaxe"): void;
  (e: "open-notes"): void;
}>();

const keywordModel = computed({ get: () => props.keyword, set: (v: string) => emit("update:keyword", v) });
const authorModel = computed({ get: () => props.author, set: (v: string) => emit("update:author", v) });
const sinceModel = computed({ get: () => props.since, set: (v: string) => emit("update:since", v) });
const untilModel = computed({ get: () => props.until, set: (v: string) => emit("update:until", v) });
</script>

<template>
      <!-- Server-side filter bar -->
      <div class="border-b border-line/60 px-4 py-1.5 flex items-center gap-2 bg-surface/20 shrink-0 flex-wrap">
        <div class="relative flex-1 min-w-[160px] max-w-[300px]">
          <Search class="w-3.5 h-3.5 text-ink-muted absolute left-2 top-1.5 pointer-events-none" />
          <input
            v-model="keywordModel"
            type="text"
            :aria-label="t('history.filter.message.aria')"
            :placeholder="t('history.filter.message.placeholder')"
            class="w-full pl-7 pr-2 py-1 rounded-md bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent font-sans transition"
          />
        </div>

        <div class="relative w-[150px]">
          <User class="w-3.5 h-3.5 text-ink-muted absolute left-2 top-1.5 pointer-events-none" />
          <input
            v-model="authorModel"
            type="text"
            :aria-label="t('history.filter.author.aria')"
            :placeholder="t('history.filter.author.placeholder')"
            class="w-full pl-7 pr-2 py-1 rounded-md bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent font-sans transition"
          />
        </div>

        <div class="flex items-center gap-1 text-[10px] text-ink-muted font-mono">
          <Calendar class="w-3.5 h-3.5 shrink-0" />
          <input
            v-model="sinceModel"
            type="date"
            class="px-1.5 py-1 rounded-md bg-canvas border border-line text-[11px] text-ink font-mono focus:outline-none focus:border-accent cursor-pointer"
            :title="t('history.filter.since.hint')"
          />
          <span class="text-ink-muted">–</span>
          <input
            v-model="untilModel"
            type="date"
            class="px-1.5 py-1 rounded-md bg-canvas border border-line text-[11px] text-ink font-mono focus:outline-none focus:border-accent cursor-pointer"
            :title="t('history.filter.until.hint')"
          />
        </div>

        <button
          v-if="hasActiveFilters"
          @click="emit('clear')"
          class="flex items-center gap-1 px-2 py-1 rounded-md bg-raised hover:bg-hover-strong text-[10px] text-ink-secondary hover:text-ink transition cursor-pointer border border-line"
          :title="t('history.filter.clear.hint')"
        >
          <X class="w-3 h-3" />
          <span>{{ t("history.filter.clear") }}</span>
        </button>

        <button
          @click="emit('open-notes')"
          class="flex items-center gap-1 px-2 py-1 rounded-md bg-raised hover:bg-hover-strong text-[10px] text-ink-secondary hover:text-ink transition cursor-pointer border border-line"
          :title="t('history.filter.notes.hint')"
        >
          <StickyNote class="w-3 h-3" />
          <span>{{ t("history.filter.notes") }}</span>
        </button>

        <button
          @click="emit('open-pickaxe')"
          class="flex items-center gap-1 px-2 py-1 rounded-md bg-raised hover:bg-hover-strong text-[10px] text-ink-secondary hover:text-ink transition cursor-pointer border border-line"
          :title="t('history.filter.archaeology.hint')"
        >
          <Search class="w-3 h-3" />
          <span>{{ t("history.filter.archaeology") }}</span>
        </button>

        <button
          @click="emit('export-report')"
          class="flex items-center gap-1 px-2 py-1 rounded-md bg-raised hover:bg-hover-strong text-[10px] text-ink-secondary hover:text-ink transition cursor-pointer border border-line"
          :title="t('history.filter.export.hint')"
        >
          <Download class="w-3 h-3" />
          <span>{{ t("history.filter.export") }}</span>
        </button>

        <span v-if="isFilterMode" class="text-[10px] text-ink-muted truncate max-w-[280px]">
          {{ activeFilterSummary }}
        </span>
      </div>

</template>
