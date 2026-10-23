import { nextTick, onBeforeUnmount, ref, watch } from "vue";

const FOCUSABLE =
  'a[href], area[href], button:not([disabled]), input:not([disabled]):not([type="hidden"]), select:not([disabled]), textarea:not([disabled]), iframe, object, embed, [contenteditable], [tabindex]:not([tabindex="-1"])';

export function useModalA11y(isOpen: () => boolean, onClose: () => void) {
  const containerRef = ref<HTMLElement | null>(null);
  let previousFocus: HTMLElement | null = null;
  let listening = false;

  function focusables(): HTMLElement[] {
    const root = containerRef.value;
    if (!root) return [];
    return [...root.querySelectorAll<HTMLElement>(FOCUSABLE)].filter(
      (el) => el.offsetParent !== null && el.getAttribute("aria-hidden") !== "true"
    );
  }

  function onKeydown(event: KeyboardEvent) {
    if (!isOpen()) return;
    if (event.key === "Escape") {
      event.preventDefault();
      event.stopPropagation();
      onClose();
      return;
    }
    if (event.key !== "Tab") return;
    const items = focusables();
    if (items.length === 0) return;
    const first = items[0]!;
    const last = items[items.length - 1]!;
    const active = document.activeElement as HTMLElement | null;
    if (!active || !containerRef.value?.contains(active)) {
      event.preventDefault();
      first.focus();
      return;
    }
    if (event.shiftKey && active === first) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && active === last) {
      event.preventDefault();
      first.focus();
    }
  }

  function start() {
    if (listening) return;
    listening = true;
    previousFocus = document.activeElement as HTMLElement | null;
    document.addEventListener("keydown", onKeydown, true);
    nextTick(() => focusables()[0]?.focus());
  }

  function stop() {
    if (!listening) return;
    listening = false;
    document.removeEventListener("keydown", onKeydown, true);
    if (previousFocus && document.contains(previousFocus)) previousFocus.focus();
    previousFocus = null;
  }

  watch(
    isOpen,
    (open) => {
      if (open) start();
      else stop();
    },
    { immediate: true }
  );
  onBeforeUnmount(stop);

  return { containerRef };
}
