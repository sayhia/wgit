<script setup lang="ts">
defineProps<{
  text: string;
  visible: boolean;
}>();
</script>

<template>
  <transition
    enter-active-class="transition duration-200 ease-out"
    enter-from-class="transform translate-y-4 opacity-0"
    enter-to-class="transform translate-y-0 opacity-100"
    leave-active-class="transition duration-150 ease-in"
    leave-from-class="transform translate-y-0 opacity-100"
    leave-to-class="transform translate-y-4 opacity-0"
  >
    <div
      v-if="visible"
      role="status"
      aria-live="polite"
      aria-atomic="true"
      class="fixed bottom-6 right-6 z-[60] max-w-md px-4 py-2 rounded-xl bg-surface border border-line shadow-toast text-xs text-ink flex items-center gap-2 select-none pointer-events-none"
    >
      <span aria-hidden="true" class="w-1.5 h-1.5 rounded-full bg-accent-strong shrink-0"></span>
      <span class="break-words leading-relaxed text-left">{{ text }}</span>
    </div>
  </transition>
</template>
