import { describe, expect, test } from "bun:test";
import { mount } from "@vue/test-utils";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import AiReasoningPanel from "../src/components/ui/AiReasoningPanel.vue";
import "./domSetup";

const source = (path: string) => readFileSync(fileURLToPath(new URL(path, import.meta.url)), "utf8");

describe("AiReasoningPanel", () => {
  test("renders nothing until the provider emits reasoning", () => {
    const wrapper = mount(AiReasoningPanel, { props: { text: "" } });
    expect(wrapper.find("[data-testid='ai-reasoning']").exists()).toBe(false);
  });

  test("starts collapsed, exposes pressed state and reveals the text on click", async () => {
    const wrapper = mount(AiReasoningPanel, { props: { text: "step one" } });
    const toggle = wrapper.find("[data-testid='ai-reasoning-toggle']");
    expect(toggle.exists()).toBe(true);
    expect(toggle.attributes("aria-expanded")).toBe("false");
    expect(wrapper.find("[data-testid='ai-reasoning-text']").exists()).toBe(false);

    await toggle.trigger("click");
    expect(toggle.attributes("aria-expanded")).toBe("true");
    expect(wrapper.find("[data-testid='ai-reasoning-text']").text()).toContain("step one");

    await toggle.trigger("click");
    expect(toggle.attributes("aria-expanded")).toBe("false");
    expect(wrapper.find("[data-testid='ai-reasoning-text']").exists()).toBe(false);
  });
});

describe("reasoning is wired into every streaming surface", () => {
  const surfaces = [
    "../src/components/workspace/AiChatModal.vue",
    "../src/components/workspace/AiReviewModal.vue",
    "../src/components/workspace/AiReleaseNotesModal.vue",
    "../src/components/workspace/AiExplainDiffModal.vue",
    "../src/components/workspace/AiConflictAdvisor.vue",
    "../src/components/workspace/AiConflictBatchModal.vue",
    "../src/components/workspace/CommitComposer.vue",
    "../src/components/history/CommitInspector.vue",
  ];

  test("each surface renders the shared panel and forwards reasoning deltas", () => {
    for (const path of surfaces) {
      const content = source(path);
      expect(content).toContain('import AiReasoningPanel from "../ui/AiReasoningPanel.vue"');
      expect(content).toContain("<AiReasoningPanel");
      if (path !== "../src/components/workspace/AiChatModal.vue") {
        // Chat keeps reasoning per assistant message; the rest forward a delta.
        expect(content).toContain("onReasoning:");
      }
    }
  });

  test("reasoning is not mixed into the final answer text", () => {
    for (const path of surfaces) {
      const content = source(path);
      expect(content).not.toContain("content += `\\n\\n${delta}`");
    }
  });
});
