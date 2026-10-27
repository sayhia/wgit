import { reactive } from "vue";

/**
 * Promise-based confirmation dialog.
 *
 * `window.confirm()` is synchronous, unstyled, blocking, and cannot express
 * "this one is destructive". This module keeps a single dialog instance at the
 * app root and returns a Promise, so call sites read almost the same:
 *
 *   if (!(await confirmAction({ message: "确定删除？", tone: "danger" }))) return;
 *
 * Only one dialog can be open at a time: opening a second one resolves the
 * first as `false`, which is the safe default for a cancelled question.
 */

export type ConfirmTone = "default" | "warning" | "danger";

export interface ConfirmOptions {
  title?: string;
  message: string;
  /** Secondary paragraph, e.g. what exactly will be lost. */
  detail?: string;
  confirmLabel?: string;
  cancelLabel?: string;
  tone?: ConfirmTone;
  /**
   * When set, the user must type this exact string before confirming. Used for
   * the few operations that cannot be undone (history rewrite, force push).
   */
  requireText?: string;
  requireTextHint?: string;
  /** Scope summary, e.g. number of files or commits affected by a destructive action */
  affectedSummary?: string;
}

export interface ConfirmState extends ConfirmOptions {
  open: boolean;
  requireTextValue: string;
  busy: boolean;
}

export const confirmState = reactive<ConfirmState>({
  open: false,
  message: "",
  requireTextValue: "",
  busy: false,
});

let resolver: ((ok: boolean) => void) | null = null;

export function confirmAction(options: ConfirmOptions | string): Promise<boolean> {
  if (resolver) {
    const pending = resolver;
    resolver = null;
    pending(false);
  }

  const normalized = typeof options === "string" ? { message: options } : options;
  Object.assign(confirmState, {
    title: undefined,
    detail: undefined,
    confirmLabel: undefined,
    cancelLabel: undefined,
    requireText: undefined,
    requireTextHint: undefined,
    affectedSummary: undefined,
    tone: "default",
    ...normalized,
    open: true,
    requireTextValue: "",
    busy: false,
  });

  return new Promise<boolean>((resolve) => {
    resolver = resolve;
  });
}

/** Called by the dialog host. Exported so tests can drive the state machine. */
export function settleConfirm(ok: boolean): void {
  const pending = resolver;
  resolver = null;
  confirmState.open = false;
  confirmState.busy = false;
  if (pending) pending(ok);
}

export function canConfirm(state: ConfirmState): boolean {
  if (!state.open || state.busy) return false;
  if (!state.requireText) return true;
  return state.requireTextValue.trim() === state.requireText;
}

/** Test helper: drop any pending promise without leaking between cases. */
export function resetConfirm(): void {
  resolver = null;
  confirmState.open = false;
  confirmState.busy = false;
  confirmState.requireText = undefined;
  confirmState.requireTextValue = "";
}
