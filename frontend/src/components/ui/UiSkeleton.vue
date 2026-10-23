<script setup lang="ts">
import { computed } from "vue";
import { skeletonClasses } from "../../utils/uiVariants";

const props = withDefaults(
  defineProps<{
    variant?: "line" | "block" | "card";
    /** Number of stacked lines when variant="line". */
    lines?: number;
  }>(),
  { variant: "line", lines: 3 }
);

const classes = computed(() => skeletonClasses(props.variant));
const lineWidths = ["w-full", "w-11/12", "w-4/5", "w-2/3", "w-1/2"];
</script>

<template>
  <div v-if="variant === 'line'" class="space-y-1.5" role="status" aria-label="加载中">
    <div
      v-for="i in lines"
      :key="i"
      :class="[classes, lineWidths[(i - 1) % lineWidths.length]]"
    />
  </div>
  <div v-else :class="classes" role="status" aria-label="加载中" />
</template>
