<script setup lang="ts">
import { computed } from "vue";
import { badgeClasses, type BadgeTone, type BadgeSize } from "../../utils/uiVariants";

const props = withDefaults(
  defineProps<{
    tone?: BadgeTone;
    size?: BadgeSize;
  }>(),
  { tone: "muted", size: "xs" }
);

const classes = computed(() => badgeClasses(props.tone, props.size));
</script>

<template>
  <span :class="classes">
    <slot />
  </span>
</template>
