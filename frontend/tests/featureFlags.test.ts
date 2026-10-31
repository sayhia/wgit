import { beforeEach, describe, expect, spyOn, test } from "bun:test";
import { mount } from "@vue/test-utils";
import FeatureGate from "../src/components/ui/FeatureGate.vue";
import {
  ensureFlagsLoaded,
  featureFlags,
  isFeatureEnabled,
  loadFlags,
  resetFeatureFlagsForTest,
  type FeatureFlagKey,
} from "../src/composables/useFeatureFlags";
import { featureFlagsApi } from "../src/services/gitService";
import type { FeatureFlags } from "../src/types";
import "./domSetup";

const ALL_KEYS: FeatureFlagKey[] = ["aiCommitMsg", "aiSummary", "aiConflictBatch", "reviewFlow", "assetBoard"];

const allDisabled = (): FeatureFlags => ({
  aiCommitMsg: false,
  aiSummary: false,
  aiConflictBatch: false,
  reviewFlow: false,
  assetBoard: false,
});

/** Lets onMounted → ensureFlagsLoaded → reactive assign → re-render propagate. */
async function flushAsync(): Promise<void> {
  await new Promise((resolve) => setTimeout(resolve, 0));
  await Promise.resolve();
  await new Promise((resolve) => setTimeout(resolve, 0));
  await Promise.resolve();
}

beforeEach(() => {
  resetFeatureFlagsForTest();
});

describe("useFeatureFlags: defaults & pure helpers", () => {
  test("every flag reads false before any backend load", () => {
    for (const key of ALL_KEYS) {
      expect(isFeatureEnabled(key)).toBe(false);
      expect(featureFlags[key]).toBe(false);
    }
  });

  test("unknown flag keys never read as enabled", () => {
    expect(isFeatureEnabled("nonexistentFlag" as FeatureFlagKey)).toBe(false);
  });
});

describe("useFeatureFlags: loadFlags idempotency", () => {
  test("backend is called exactly once across repeated loadFlags/ensureFlagsLoaded", async () => {
    const spy = spyOn(featureFlagsApi, "getFeatureFlags").mockResolvedValue({
      ...allDisabled(),
      aiCommitMsg: true,
    });

    await loadFlags();
    await loadFlags();
    await ensureFlagsLoaded();

    // Singleton promise: the backend wrapper (callBackend layer) counted 1 call.
    expect(spy).toHaveBeenCalledTimes(1);
    expect(featureFlags.aiCommitMsg).toBe(true);
    for (const key of ALL_KEYS) {
      if (key !== "aiCommitMsg") {
        expect(featureFlags[key]).toBe(false);
      }
    }

    spy.mockRestore();
  });

  test("ensureFlagsLoaded returns the same singleton promise", async () => {
    const spy = spyOn(featureFlagsApi, "getFeatureFlags").mockResolvedValue(allDisabled());

    const p1 = ensureFlagsLoaded();
    const p2 = ensureFlagsLoaded();
    expect(p1).toBe(p2);
    await Promise.all([p1, p2]);

    expect(spy).toHaveBeenCalledTimes(1);
    spy.mockRestore();
  });

  test("a failed load keeps defaults and allows a retry", async () => {
    const spy = spyOn(featureFlagsApi, "getFeatureFlags")
      .mockRejectedValueOnce(new Error("backend down"))
      .mockResolvedValueOnce({ ...allDisabled(), reviewFlow: true });

    await loadFlags(); // fails silently, defaults kept
    expect(isFeatureEnabled("reviewFlow")).toBe(false);

    await loadFlags(); // retry succeeds
    expect(isFeatureEnabled("reviewFlow")).toBe(true);
    expect(spy).toHaveBeenCalledTimes(2);

    spy.mockRestore();
  });
});

describe("FeatureGate: rendering on first render bootstrap", () => {
  test("hides gated content while disabled, shows it once flags load", async () => {
    const spy = spyOn(featureFlagsApi, "getFeatureFlags").mockResolvedValue({
      ...allDisabled(),
      reviewFlow: true,
    });

    const wrapper = mount(FeatureGate, {
      props: { flag: "reviewFlow" },
      slots: { default: "REVIEW_FLOW_PANEL" },
    });

    // First render happens before the backend answer lands: dark.
    expect(wrapper.text()).not.toContain("REVIEW_FLOW_PANEL");

    await flushAsync();
    await wrapper.vm.$nextTick();

    // Lazy bootstrap on first render pulled the flags and re-rendered.
    expect(wrapper.text()).toContain("REVIEW_FLOW_PANEL");
    expect(spy).toHaveBeenCalledTimes(1);

    spy.mockRestore();
  });

  test("renders nothing (without error) when the flag stays disabled", async () => {
    const spy = spyOn(featureFlagsApi, "getFeatureFlags").mockResolvedValue(allDisabled());

    const wrapper = mount(FeatureGate, {
      props: { flag: "assetBoard" },
      slots: { default: "ASSET_BOARD_PANEL" },
    });

    await flushAsync();
    await wrapper.vm.$nextTick();

    expect(wrapper.text()).not.toContain("ASSET_BOARD_PANEL");
    expect(wrapper.text()).toBe("");

    spy.mockRestore();
  });

  test("renders the optional fallback slot when disabled", async () => {
    const spy = spyOn(featureFlagsApi, "getFeatureFlags").mockResolvedValue(allDisabled());

    const wrapper = mount(FeatureGate, {
      props: { flag: "assetBoard" },
      slots: {
        default: "ASSET_BOARD_PANEL",
        fallback: "COMING_SOON_PLACEHOLDER",
      },
    });

    await flushAsync();
    await wrapper.vm.$nextTick();

    expect(wrapper.text()).toContain("COMING_SOON_PLACEHOLDER");
    expect(wrapper.text()).not.toContain("ASSET_BOARD_PANEL");

    spy.mockRestore();
  });

  test("an unknown flag key renders nothing instead of throwing", async () => {
    const spy = spyOn(featureFlagsApi, "getFeatureFlags").mockResolvedValue(allDisabled());

    const wrapper = mount(FeatureGate, {
      props: { flag: "notARealFlag" },
      slots: { default: "SHOULD_NOT_RENDER" },
    });

    await flushAsync();
    await wrapper.vm.$nextTick();

    expect(wrapper.text()).not.toContain("SHOULD_NOT_RENDER");

    spy.mockRestore();
  });
});
