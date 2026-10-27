<script setup lang="ts">
import { computed } from "vue";
import { skeletonClasses } from "../../utils/uiVariants";
import { t } from "../../i18n";

const props = withDefaults(
  defineProps<{
    variant?: "line" | "block" | "card" | "rows";
    /** Number of stacked lines when variant="line". */
    lines?: number;
    /** Number of placeholder rows when variant="rows". */
    rows?: number;
    /** Row height in px; match the list this stands in for (commit rows are 56). */
    rowHeight?: number;
    /** Optional left gutter (graph lane, avatar) width in px. */
    gutter?: number;
  }>(),
  { variant: "line", lines: 3, rows: 8, rowHeight: 56, gutter: 0 }
);

// `rows` is a layout mode, not a size; it falls back to the line class.
const classes = computed(() => skeletonClasses(props.variant === "rows" ? "line" : props.variant));
const lineWidths = ["w-full", "w-11/12", "w-4/5", "w-2/3", "w-1/2"];
</script>

<template>
  <div v-if="variant === 'line'" class="space-y-1.5" role="status" :aria-label="t('ui.loading')">
    <div
      v-for="i in lines"
      :key="i"
      :class="[classes, lineWidths[(i - 1) % lineWidths.length]]"
    />
  </div>
  <div v-else-if="variant === 'rows'" class="flex flex-col gap-1.5 py-1" role="status" :aria-label="t('ui.loading')">
    <!--
      Row placeholders mirror the real list geometry, so the content does not
      jump when it arrives. A spinner cannot promise that.
    -->
    <div
      v-for="row in rows"
      :key="row"
      class="flex items-center gap-3 px-3 rounded-lg bg-surface/40 animate-pulse"
      :style="{ height: `${Math.max(16, rowHeight - 6)}px`, animationDelay: `${row * 40}ms` }"
    >
      <div v-if="gutter" class="shrink-0 h-2 rounded-full bg-raised" :style="{ width: `${gutter}px` }"></div>
      <div class="flex flex-col gap-1.5 flex-1 min-w-0">
        <div class="h-2 rounded-full bg-raised" :style="{ width: `${45 + ((row * 17) % 40)}%` }"></div>
        <div class="h-1.5 rounded-full bg-raised/70" :style="{ width: `${25 + ((row * 11) % 30)}%` }"></div>
      </div>
    </div>
  </div>
  <div v-else :class="classes" role="status" :aria-label="t('ui.loading')" />
</template>
