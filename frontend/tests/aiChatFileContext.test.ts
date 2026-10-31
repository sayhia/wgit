import { beforeEach, describe, expect, mock, test } from "bun:test";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import "./domSetup";
import type { AIStreamInput } from "../src/services/aiStreamService";

let captured: AIStreamInput | null = null;
let previewCaptured: unknown = null;

// Keep every real export: Bun module mocks are process-wide, and replacing the
// whole aiService module would break later test files that need its other RPCs.
const actualStream = await import("../src/services/aiStreamService");
const actualAI = await import("../src/services/aiService");

mock.module("../src/services/aiService", () => ({
  ...actualAI,
  previewAIRequest: async (input: unknown) => {
    previewCaptured = input;
    return "preview payload";
  },
}));

mock.module("../src/services/aiStreamService", () => ({
  ...actualStream,
  startAIStream: async (input: AIStreamInput) => {
    captured = input;
    return {
      requestId: "test-request",
      completion: Promise.resolve({ kind: "done", text: "answer" }),
      cancel: async () => true,
      dispose: () => {},
    };
  },
}));

const AiChatModal = (await import("../src/components/workspace/AiChatModal.vue")).default;

beforeEach(() => {
  captured = null;
  previewCaptured = null;
});

async function send(wrapper: VueWrapper): Promise<void> {
  await wrapper.find("[data-testid='ai-chat-input']").setValue("解释这个函数");
  await wrapper.find("[data-testid='ai-chat-send']").trigger("click");
  await flushPromises();
}

describe("AI chat current-file attachment", () => {
  test("is off by default and only sends the file after explicit opt-in", async () => {
    const wrapper = mount(AiChatModal, {
      props: {
        isOpen: true,
        repoPath: "/tmp/repo",
        filePath: "src/answer.ts",
        scope: "repo",
      },
    });
    const checkbox = wrapper.find("[data-testid='ai-chat-include-file'] input");
    expect(checkbox.exists()).toBe(true);
    expect((checkbox.element as HTMLInputElement).checked).toBe(false);

    await send(wrapper);
    expect(captured?.includeFile).toBe(false);
    expect(captured?.filePath).toBe("src/answer.ts");

    await checkbox.setValue(true);
    await wrapper.find("[data-testid='ai-chat-input']").setValue("解释这个函数");
    await wrapper.find("[data-testid='ai-chat-preview']").trigger("click");
    await flushPromises();
    expect((previewCaptured as AIStreamInput)?.includeFile).toBe(true);
    expect(wrapper.find("[data-testid='ai-chat-preview-result']").text()).toContain("preview payload");

    await send(wrapper);
    expect(captured?.includeFile).toBe(true);
  });

  test("is hidden when the scope is a diff or no file is selected", async () => {
    const diffWrapper = mount(AiChatModal, {
      props: { isOpen: true, repoPath: "/tmp/repo", filePath: "src/answer.ts", scope: "diff" },
    });
    expect(diffWrapper.find("[data-testid='ai-chat-include-file']").exists()).toBe(false);
    await send(diffWrapper);
    expect(captured?.includeFile).toBe(false);

    const repoWrapper = mount(AiChatModal, {
      props: { isOpen: true, repoPath: "/tmp/repo", scope: "repo" },
    });
    expect(repoWrapper.find("[data-testid='ai-chat-include-file']").exists()).toBe(false);
  });
});
