<script setup lang="ts">
import { X } from "lucide-vue-next";
import { t } from "../../i18n";
import {
  dismissToast,
  runToastAction,
  toasts,
  type ToastTone,
} from "../../composables/useToast";

/**
 * The only `<Toast>` mount in the app.
 *
 * State lives in `useToast`, so a toast raised anywhere — including deep inside
 * a tools tab — lands here. Older builds mounted one per view, which is how two
 * identical panels ended up stacked in the same corner.
 */
const TONE_DOT: Record<ToastTone, string> = {
  info: "bg-accent-strong",
  success: "bg-success-strong",
  warning: "bg-warning-strong",
  danger: "bg-danger-strong",
};

const TONE_RING: Record<ToastTone, string> = {
  info: "border-line",
  success: "border-success/40",
  warning: "border-warning/40",
  danger: "border-danger/40",
};
</script>

<template>
  <div
    class="fixed bottom-6 right-6 z-[90] flex flex-col items-end gap-2 pointer-events-none select-none"
  >
    <transition-group
      enter-active-class="transition duration-200 ease-out"
      enter-from-class="transform translate-y-4 opacity-0"
      enter-to-class="transform translate-y-0 opacity-100"
      leave-active-class="transition duration-150 ease-in"
      leave-from-class="transform translate-y-0 opacity-100"
      leave-to-class="transform translate-y-4 opacity-0"
      move-class="transition duration-150 ease-out"
    >
      <div
        v-for="toast in toasts"
        :key="toast.id"
        :role="toast.tone === 'danger' ? 'alert' : 'status'"
        :aria-live="toast.tone === 'danger' ? 'assertive' : 'polite'"
        aria-atomic="true"
        data-glass
        data-testid="toast-item"
        class="pointer-events-auto max-w-md px-3.5 py-2 rounded-xl glass glass-panel border text-xs text-ink flex items-center gap-2"
        :class="TONE_RING[toast.tone]"
      >
        <span aria-hidden="true" class="w-1.5 h-1.5 rounded-full shrink-0" :class="TONE_DOT[toast.tone]"></span>
        <span class="break-words leading-relaxed text-left">{{ toast.text }}</span>
        <button
          v-if="toast.action"
          type="button"
          data-testid="toast-undo-btn"
          class="ml-1 px-1.5 py-0.5 rounded-md bg-raised/80 hover:bg-raised text-[11px] font-semibold text-accent hover:text-accent-strong transition cursor-pointer shrink-0"
          @click="runToastAction(toast.id)"
        >
          {{ toast.action.label }}
        </button>
        <button
          type="button"
          class="p-0.5 rounded text-ink-muted hover:text-ink transition cursor-pointer shrink-0"
          :aria-label="t('common.close')"
          @click="dismissToast(toast.id)"
        >
          <X class="w-3 h-3" aria-hidden="true" />
        </button>
      </div>
    </transition-group>
  </div>
</template>
