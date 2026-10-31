<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { X } from "lucide-vue-next";
import { t } from "../../i18n";
import {
  dismissToast,
  pauseToast,
  resumeToast,
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

const stackRef = ref<HTMLElement | null>(null);
let resizeObserver: ResizeObserver | null = null;
let newestVisibleId = 0;

/** Shared with UiModal: reserve the painted stack and an 8px separation. */
function measureStack(): void {
  const stack = stackRef.value;
  if (!stack) return;
  const box = stack.getBoundingClientRect();
  const rootStyle = document.documentElement.style;
  if (box.height <= 0) {
    rootStyle.removeProperty("--ug-toast-reserve");
    return;
  }
  rootStyle.setProperty("--ug-toast-reserve", `${Math.ceil(window.innerHeight - box.top + 8)}px`);
}

async function refreshStack(): Promise<void> {
  await nextTick();
  const stack = stackRef.value;
  const newest = toasts.value[toasts.value.length - 1];
  if (stack && newest && newest.id !== newestVisibleId) {
    // Show the start of a new long error, including its action/close controls.
    const item = stack.querySelector<HTMLElement>(`[data-toast-id="${newest.id}"]`);
    if (item) stack.scrollTop = item.offsetTop;
  }
  newestVisibleId = newest?.id ?? 0;
  measureStack();
}

function finishEntering(element: Element): void {
  const newest = toasts.value[toasts.value.length - 1];
  if (newest && element.getAttribute("data-toast-id") === String(newest.id) && stackRef.value) {
    // A leaving notice may have occupied layout space during the first
    // measurement. Anchor again once that transition has finished.
    stackRef.value.scrollTop = (element as HTMLElement).offsetTop;
  }
  measureStack();
}

watch(toasts, refreshStack, { flush: "post" });
onMounted(() => {
  if (typeof ResizeObserver !== "undefined" && stackRef.value) {
    resizeObserver = new ResizeObserver(measureStack);
    resizeObserver.observe(stackRef.value);
  }
  window.addEventListener("resize", measureStack);
  void refreshStack();
});
onBeforeUnmount(() => {
  resizeObserver?.disconnect();
  window.removeEventListener("resize", measureStack);
  document.documentElement.style.removeProperty("--ug-toast-reserve");
});
</script>

<template>
  <div
    ref="stackRef"
    data-testid="toast-stack"
    class="fixed bottom-4 right-4 left-4 sm:left-auto sm:bottom-5 sm:right-5 z-[90] flex max-h-[30dvh] flex-col items-end gap-2 overflow-y-auto overscroll-contain select-none"
    :class="toasts.length ? 'pointer-events-auto' : 'pointer-events-none'"
  >
    <transition-group
      enter-active-class="transition duration-200 ease-out"
      enter-from-class="transform translate-y-4 opacity-0"
      enter-to-class="transform translate-y-0 opacity-100"
      leave-active-class="transition duration-150 ease-in"
      leave-from-class="transform translate-y-0 opacity-100"
      leave-to-class="transform translate-y-4 opacity-0"
      move-class="transition duration-150 ease-out"
      @after-enter="finishEntering"
    >
      <div
        v-for="toast in toasts"
        :key="toast.id"
        :data-toast-id="toast.id"
        :role="toast.tone === 'danger' ? 'alert' : 'status'"
        :aria-live="toast.tone === 'danger' ? 'assertive' : 'polite'"
        aria-atomic="true"
        @mouseenter="pauseToast(toast.id)"
        @mouseleave="resumeToast(toast.id)"
        @focusin="pauseToast(toast.id)"
        @focusout="resumeToast(toast.id)"
        data-glass
        data-testid="toast-item"
        class="pointer-events-auto w-full sm:w-auto max-w-md min-w-0 shrink-0 px-3.5 py-3 rounded-[var(--ug-radius-lg)] glass glass-panel glass-popover border border-line shadow-[var(--ug-shadow-toast)] text-xs text-ink flex items-start gap-2.5"
      >
        <span aria-hidden="true" class="w-1.5 h-1.5 mt-1.5 rounded-full shrink-0" :class="TONE_DOT[toast.tone]"></span>
        <span class="min-w-0 flex-1 break-words [overflow-wrap:anywhere] leading-relaxed text-left">{{ toast.text }}</span>
        <button
          v-if="toast.action"
          type="button"
          data-testid="toast-undo-btn"
          class="ml-1 max-w-[40%] px-1.5 py-0.5 rounded-md hover:bg-raised text-[11px] font-medium text-accent whitespace-normal break-words transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent cursor-pointer shrink-0"
          @click="runToastAction(toast.id)"
        >
          {{ toast.action.label }}
        </button>
        <button
          type="button"
          class="flex h-5 w-5 items-center justify-center rounded text-ink-muted hover:text-ink hover:bg-raised transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent cursor-pointer shrink-0"
          :aria-label="t('common.close')"
          @click="dismissToast(toast.id)"
        >
          <X class="w-3 h-3" aria-hidden="true" />
        </button>
      </div>
    </transition-group>
  </div>
</template>
