<script setup lang="ts">
import { computed } from "vue";
import { buttonClasses, type ButtonVariant, type ButtonSize } from "../../utils/uiVariants";

const props = withDefaults(
  defineProps<{
    variant?: ButtonVariant;
    size?: ButtonSize;
    disabled?: boolean;
    type?: "button" | "submit" | "reset";
  }>(),
  { variant: "secondary", size: "sm", disabled: false, type: "button" }
);

const classes = computed(() => buttonClasses(props.variant, props.size));
</script>

<template>
  <button :type="type" :disabled="disabled" :class="classes">
    <slot />
  </button>
</template>
