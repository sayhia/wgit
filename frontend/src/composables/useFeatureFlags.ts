import { reactive } from "vue";
import { featureFlagsApi } from "../services/gitService";
import type { FeatureFlags } from "../types";

/**
 * Advisor-plan §3.2/§3.3 feature-flag foundation.
 *
 * Flags live module-level (singleton): every consumer sees the same reactive
 * state. Until the backend answers, everything is false, so gated surfaces
 * stay dark by default — including in the browser preview, where the flag RPC
 * has no dev fixture and resolves null.
 */

/** Union of known flag keys; unknown keys always read as disabled. */
export type FeatureFlagKey = keyof FeatureFlags;

const DEFAULT_FLAGS: FeatureFlags = {
  aiCommitMsg: false,
  aiSummary: false,
  aiConflictBatch: false,
  reviewFlow: false,
  assetBoard: false,
};

/** Module-level reactive flag set shared by all consumers. */
export const featureFlags = reactive<FeatureFlags>({ ...DEFAULT_FLAGS });

let loadPromise: Promise<void> | null = null;

/**
 * Fetch the flag set from the backend exactly once; later calls reuse the
 * in-flight/settled promise (singleton). Failures keep the defaults and allow
 * a retry on the next call.
 */
export function loadFlags(): Promise<void> {
  if (!loadPromise) {
    loadPromise = featureFlagsApi
      .getFeatureFlags()
      .then((flags) => {
        Object.assign(featureFlags, DEFAULT_FLAGS, flags ?? {});
      })
      .catch((err: unknown) => {
        console.warn("[useFeatureFlags] flag load failed, keeping defaults:", err);
        loadPromise = null;
      });
  }
  return loadPromise;
}

/**
 * Idempotent bootstrap for consumers that must not hook App.vue
 * (FeatureGate triggers this on its first render).
 */
export function ensureFlagsLoaded(): Promise<void> {
  return loadFlags();
}

/** True only when the named flag is explicitly enabled by the backend. */
export function isFeatureEnabled(key: FeatureFlagKey): boolean {
  return featureFlags[key] === true;
}

/** @internal test-only: reset singleton state between tests. */
export function resetFeatureFlagsForTest(): void {
  Object.assign(featureFlags, DEFAULT_FLAGS);
  loadPromise = null;
}
