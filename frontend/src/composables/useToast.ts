import { onScopeDispose, ref } from "vue";

/** Shared transient toast state; auto-dismisses after `duration` ms. */
export function useToast(duration = 2400) {
  const toastText = ref("");
  const showToast = ref(false);
  let timer: ReturnType<typeof setTimeout> | null = null;

  function notify(message: string) {
    toastText.value = message;
    showToast.value = true;
    if (timer) clearTimeout(timer);
    timer = setTimeout(() => {
      showToast.value = false;
      timer = null;
    }, duration);
  }

  onScopeDispose(() => {
    if (timer) clearTimeout(timer);
    timer = null;
  });

  return { toastText, showToast, notify };
}
