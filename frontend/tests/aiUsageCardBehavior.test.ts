import { beforeEach, describe, expect, mock, test } from "bun:test";
import { flushPromises, mount } from "@vue/test-utils";
import "./domSetup";
import type { AIUsageSummary } from "../src/services/aiService";

const SUMMARY: AIUsageSummary = {
  days: 30,
  since: "2026-10-01T00:00:00Z",
  calls: 3,
  success: 3,
  failed: 0,
  cancelled: 0,
  tokensIn: 300,
  tokensOut: 150,
  usageKnownCalls: 3,
  costKnownCalls: 3,
  costUnknownCalls: 0,
  estimatedCostUsd: 0.6,
  monthCostUsd: 1.2,
  monthlyBudgetUsd: 10,
  budgetPercent: 12,
  overBudget: false,
  inputUsdPerMTok: 1,
  outputUsdPerMTok: 2,
  byKind: [],
  byModel: [],
  byRepo: [
    {
      key: "/repo/alpha",
      calls: 2,
      success: 2,
      failed: 0,
      cancelled: 0,
      tokensIn: 100,
      tokensOut: 50,
      usageKnownCalls: 2,
      costKnownCalls: 2,
      estimatedCostUsd: 0.4,
    },
    {
      key: "/repo/beta",
      calls: 1,
      success: 1,
      failed: 0,
      cancelled: 0,
      tokensIn: 200,
      tokensOut: 100,
      usageKnownCalls: 1,
      costKnownCalls: 1,
      estimatedCostUsd: 0.2,
    },
  ],
  byDay: [],
};

let summary: AIUsageSummary = SUMMARY;

mock.module("@wailsio/runtime", () => ({
  Call: {
    ByName: (name: string) => {
      if (name === "main.AiService.GetAIUsageSummary") return Promise.resolve(summary);
      return Promise.resolve(null);
    },
    ByID: () => Promise.resolve(null),
  },
  Events: { On: () => () => {}, Emit: () => {}, Off: () => {} },
  Window: { Name: () => "test" },
  Screens: {},
}));

const AiUsageCard = (await import("../src/components/settings/AiUsageCard.vue")).default;

beforeEach(() => {
  summary = SUMMARY;
});

describe("AiUsageCard repository scope", () => {
  test("filters calls, tokens and cost by repository while the month stays global", async () => {
    const wrapper = mount(AiUsageCard);
    await flushPromises();

    const cards = () => wrapper.findAll("[data-testid='ai-usage-card'] .text-sm.font-bold");
    expect(cards()[0]!.text()).toBe("3");
    expect(cards()[1]!.text()).toBe("450");
    expect(cards()[2]!.text()).toBe("$0.6000");
    expect(cards()[3]!.text()).toBe("$1.2000");

    const filter = wrapper.find("[data-testid='ai-usage-repo-filter']");
    expect(filter.exists()).toBe(true);
    await filter.setValue("/repo/alpha");

    expect(cards()[0]!.text()).toBe("2");
    expect(cards()[1]!.text()).toBe("150");
    expect(cards()[2]!.text()).toBe("$0.4000");
    expect(cards()[3]!.text()).toBe("$1.2000");

    await filter.setValue("");
    expect(cards()[0]!.text()).toBe("3");
    expect(cards()[2]!.text()).toBe("$0.6000");
  });

  test("keeps the all-repositories option distinct from calls without a repo", async () => {
    summary = {
      ...SUMMARY,
      calls: 4,
      tokensIn: 310,
      tokensOut: 155,
      usageKnownCalls: 4,
      costKnownCalls: 4,
      estimatedCostUsd: 0.65,
      byRepo: [
        ...(SUMMARY.byRepo || []),
        {
          key: "",
          calls: 1,
          success: 1,
          failed: 0,
          cancelled: 0,
          tokensIn: 10,
          tokensOut: 5,
          usageKnownCalls: 1,
          costKnownCalls: 1,
          estimatedCostUsd: 0.05,
        },
      ],
    };
    const wrapper = mount(AiUsageCard);
    await flushPromises();
    const cards = () => wrapper.findAll("[data-testid='ai-usage-card'] .text-sm.font-bold");
    expect(cards()[0]!.text()).toBe("4");

    const filter = wrapper.find("[data-testid='ai-usage-repo-filter']");
    await filter.setValue("__none__");
    expect(cards()[0]!.text()).toBe("1");
    expect(cards()[1]!.text()).toBe("15");
    expect(cards()[2]!.text()).toBe("$0.0500");

    await filter.setValue("__all__");
    expect(cards()[0]!.text()).toBe("4");
    expect(cards()[2]!.text()).toBe("$0.6500");
  });

  test("hides the filter when every call belongs to one bucket", async () => {
    summary = { ...SUMMARY, byRepo: [SUMMARY.byRepo![0]!] };
    const wrapper = mount(AiUsageCard);
    await flushPromises();
    expect(wrapper.find("[data-testid='ai-usage-repo-filter']").exists()).toBe(false);
    expect(wrapper.text()).toContain("$0.6000");
  });
});
