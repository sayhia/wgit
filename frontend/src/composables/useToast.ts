import { computed, ref } from "vue";
import { recordCrash } from "../utils/crashReporter";

export type ToastTone = "info" | "success" | "warning" | "danger";

export interface ToastAction {
  label: string;
  run: () => void;
}

export interface ToastOptions {
  tone?: ToastTone;
  /** Milliseconds before auto-dismiss. `0` keeps the toast until dismissed. */
  duration?: number;
  action?: ToastAction;
}

export interface ToastItem {
  id: number;
  text: string;
  tone: ToastTone;
  action?: ToastAction;
  duration: number;
}

/**
 * Toasts live in a module-level store.
 *
 * They used to be per-composable refs, so every view kept its own `toastText`
 * and mounted its own `<Toast>`. Two views firing at once stacked two identical
 * panels in the same corner, and a toast raised by a component that forgot to
 * mount the component simply vanished. One store, one mount.
 */
export const toasts = ref<ToastItem[]>([]);
const timers = new Map<number, ReturnType<typeof setTimeout>>();
let nextId = 1;

/** Oldest toasts drop off first so a burst cannot cover the workspace. */
const MAX_VISIBLE = 3;

function clearTimer(id: number) {
  const timer = timers.get(id);
  if (timer) {
    clearTimeout(timer);
    timers.delete(id);
  }
}

export function dismissToast(id: number) {
  clearTimer(id);
  toasts.value = toasts.value.filter((toast) => toast.id !== id);
}

export function pushToast(text: string, options: ToastOptions = {}, fallbackDuration = 2400): number {
  const message = text.trim();
  if (!message) return 0;

  const duration = options.duration ?? fallbackDuration;
  const tone = options.tone ?? "info";

  // Repeating the same message (e.g. mashing a shortcut) refreshes the existing
  // entry instead of growing the stack with duplicates.
  const duplicate = toasts.value.find((toast) => toast.text === message && toast.tone === tone);
  if (duplicate) {
    clearTimer(duplicate.id);
    duplicate.duration = duration;
    if (duration > 0) {
      timers.set(duplicate.id, setTimeout(() => dismissToast(duplicate.id), duration));
    }
    return duplicate.id;
  }

  const item: ToastItem = { id: nextId++, text: message, tone, action: options.action, duration };
  toasts.value = [...toasts.value, item].slice(-MAX_VISIBLE);
  if (duration > 0) {
    timers.set(item.id, setTimeout(() => dismissToast(item.id), duration));
  }
  return item.id;
}

export function runToastAction(id: number) {
  const item = toasts.value.find((toast) => toast.id === id);
  if (!item?.action) return;
  const { run } = item.action;
  dismissToast(id);
  try {
    run();
  } catch (err) {
    reportError("Undo", err);
  }
}

/**
 * Report a failed operation: keep the technical detail in the console, show the
 * message to the user.
 *
 * Git's own wording is deliberately preserved — `fatal: unable to read tree …`
 * tells a developer far more than "操作失败".
 */
export function reportError(scope: string, err: unknown): void {
  const message = err instanceof Error ? err.message : String(err ?? "");
  console.error(`${scope}:`, err);
  try {
    recordCrash(err, scope);
  } catch {
    // best-effort
  }
  pushToast(message ? `${scope}: ${message}` : scope, { tone: "danger", duration: 6000 });
}

/** Test seam: clear every toast and pending timer. */
export function resetToasts() {
  for (const timer of timers.values()) clearTimeout(timer);
  timers.clear();
  toasts.value = [];
}

export function useToast(duration = 2400) {
  const toastText = computed(() => toasts.value[toasts.value.length - 1]?.text ?? "");
  const showToast = computed(() => toasts.value.length > 0);

  function notify(message: string, options: ToastOptions = {}) {
    return pushToast(message, options, duration);
  }

  return { toastText, showToast, toasts, notify, dismiss: dismissToast };
}
