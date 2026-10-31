<script setup lang="ts">
import { computed, nextTick, onUnmounted, ref, watch } from "vue";
import { Bot, Eye, Send, Sparkles, Square, Trash2, User } from "lucide-vue-next";
import UiModal from "../ui/UiModal.vue";
import UiButton from "../ui/UiButton.vue";
import AiReasoningPanel from "../ui/AiReasoningPanel.vue";
import { previewAIRequest } from "../../services/aiService";
import { t } from "../../i18n";
import {
  startAIStream,
  type AIChatTurn,
  type AIStreamInput,
  type AIStreamSession,
} from "../../services/aiStreamService";

interface ChatMessage extends AIChatTurn {
  reasoning?: string;
  error?: string;
  streaming?: boolean;
}

const props = defineProps<{
  isOpen: boolean;
  repoPath?: string;
  filePath?: string;
  diff?: string;
  scope?: "diff" | "repo";
}>();

const emit = defineEmits<{ (e: "close"): void }>();

const messages = ref<ChatMessage[]>([]);
const draft = ref("");
const includeFile = ref(false);
const previewText = ref("");
const previewError = ref("");
const previewBusy = ref(false);
const isStreaming = ref(false);
const scrollArea = ref<HTMLElement | null>(null);
let session: AIStreamSession | null = null;
let targetKey = "";
let runSeq = 0;

const scope = computed<"diff" | "repo">(() => {
  if (props.scope) return props.scope;
  return (props.diff || "").trim() ? "diff" : "repo";
});
const canSend = computed(() => !isStreaming.value && draft.value.trim() !== "");

function scrollToBottom(): void {
  void nextTick(() => {
    const el = scrollArea.value;
    if (el) el.scrollTop = el.scrollHeight;
  });
}

function stopStream(): void {
  runSeq++;
  const active = session;
  session = null;
  if (active) void active.cancel().finally(() => active.dispose());
  isStreaming.value = false;
  const last = messages.value[messages.value.length - 1];
  if (last?.streaming) last.streaming = false;
}

function resetConversation(): void {
  stopStream();
  messages.value = [];
  draft.value = "";
}

function chatInput(question: string): AIStreamInput {
  const history = messages.value
    .filter((item) => !item.error && item.content.trim() !== "")
    .map((item) => ({ role: item.role, content: item.content }) as AIChatTurn);
  return {
    kind: "chat",
    repoPath: props.repoPath || "",
    filePath: props.filePath || "",
    diff: props.diff || "",
    scope: scope.value,
    question,
    history,
    includeFile: scope.value === "repo" && includeFile.value,
  };
}

async function previewOutbound(): Promise<void> {
  const question = draft.value.trim();
  if (!question || previewBusy.value) return;
  previewBusy.value = true;
  previewError.value = "";
  try {
    previewText.value = await previewAIRequest(chatInput(question));
  } catch (err) {
    previewText.value = "";
    previewError.value = err instanceof Error ? err.message : String(err);
  } finally {
    previewBusy.value = false;
  }
}

async function send(): Promise<void> {
  const question = draft.value.trim();
  if (!question || isStreaming.value) return;
  const input = chatInput(question);
  const userMessage: ChatMessage = { role: "user", content: question };
  const assistant: ChatMessage = { role: "assistant", content: "", reasoning: "", streaming: true };
  messages.value.push(userMessage, assistant);
  draft.value = "";
  previewText.value = "";
  previewError.value = "";
  isStreaming.value = true;
  scrollToBottom();
  const seq = ++runSeq;
  try {
  const next = await startAIStream(
  input,
      {
      onDelta: (delta) => {
        if (seq !== runSeq) return;
        assistant.content += delta;
        scrollToBottom();
      },
      onReasoning: (delta) => {
        if (seq !== runSeq) return;
        assistant.reasoning = (assistant.reasoning || "") + delta;
      },
      }
    );
    session = next;
    const outcome = await next.completion;
    session = null;
    if (seq !== runSeq) return;
    assistant.streaming = false;
    if (outcome.kind === "done") {
      assistant.content = outcome.text;
    } else if (outcome.kind === "cancelled") {
      if (!assistant.content.trim()) assistant.content = t("ai.stream.cancelled");
    } else {
      assistant.error = outcome.message;
    }
  } catch (err) {
    assistant.streaming = false;
    assistant.error = err instanceof Error ? err.message : String(err);
  } finally {
    if (seq === runSeq) isStreaming.value = false;
    scrollToBottom();
  }
}

function onKeydown(event: KeyboardEvent): void {
  if (event.key === "Enter" && !event.shiftKey) {
    event.preventDefault();
    void send();
  }
}


watch(
  () => [props.isOpen, props.repoPath, props.filePath, props.scope] as const,
  ([open]) => {
    if (!open) {
      stopStream();
      return;
    }
    const key = `${props.repoPath}|${props.filePath}|${props.scope}|${(props.diff || "").length}`;
    if (key !== targetKey) {
      targetKey = key;
      includeFile.value = false;
      previewText.value = "";
      previewError.value = "";
      resetConversation();
    }
  },
  { immediate: true }
);

watch(includeFile, (enabled) => {
  if (!enabled) {
    previewText.value = "";
    previewError.value = "";
  }
});

onUnmounted(stopStream);
</script>

<template>
  <UiModal
    :open="isOpen"
    :title="t('ai.chat.title')"
    width="max-w-3xl"
    @update:open="emit('close')"
  >
    <div class="space-y-3" data-testid="ai-chat-modal">
      <div class="flex flex-wrap items-center gap-2 text-[10px]">
        <span class="px-2 py-1 rounded-lg bg-accent-bg border border-accent/20 text-accent font-semibold flex items-center gap-1">
          <Sparkles class="w-3 h-3" />
          {{ t(`ai.chat.scope.${scope}`) }}
        </span>
        <span class="px-2 py-1 rounded-lg bg-canvas border border-line text-ink-muted">{{ t("ai.chat.readonly") }}</span>
        <span v-if="filePath" class="px-2 py-1 rounded-lg bg-canvas border border-line text-ink-muted font-mono truncate max-w-[260px]">{{ filePath }}</span>
        <span class="text-ink-muted">{{ t("ai.chat.desc") }}</span>
      </div>

      <label
        v-if="scope === 'repo' && filePath"
        class="flex items-start gap-2 p-2 rounded-xl border border-line bg-canvas text-[11px] cursor-pointer"
        data-testid="ai-chat-include-file"
      >
        <input
  v-model="includeFile"
  type="checkbox"
  class="mt-0.5 accent-[var(--ug-accent)]"
          :aria-describedby="'ai-chat-include-file-hint'"
        />
        <span class="min-w-0 flex-1">
          <span class="font-semibold text-ink">{{ t("ai.chat.include.file") }}</span>
          <span id="ai-chat-include-file-hint" class="block text-[10px] text-ink-muted mt-0.5 leading-relaxed">
            {{ t("ai.chat.include.file.hint") }}
          </span>
          <button
            type="button"
            class="mt-1 inline-flex items-center gap-1 text-[10px] font-semibold text-accent hover:text-accent-strong cursor-pointer disabled:opacity-40"
            :disabled="!draft.trim() || previewBusy"
            data-testid="ai-chat-preview"
            @click.prevent="previewOutbound"
          >
            <Eye class="w-3 h-3" />
            {{ previewBusy ? t("common.loading") : t("ai.chat.preview") }}
          </button>
        </span>
      </label>
      <div
  v-if="previewText || previewError"
  class="rounded-xl border border-line bg-canvas p-2 space-y-1"
  data-testid="ai-chat-preview-result"
      >
        <p v-if="previewError" class="text-[10px] text-danger" role="alert">
          {{ t("ai.chat.preview.failed", { error: previewError }) }}
        </p>
        <pre
  v-else
  class="max-h-40 overflow-y-auto text-[10px] font-mono text-ink-muted whitespace-pre-wrap select-text"
        >{{ previewText }}</pre>
      </div>

      <div
        ref="scrollArea"
        class="max-h-[52vh] min-h-[180px] overflow-y-auto space-y-3 p-1"
        data-testid="ai-chat-messages"
      >
        <p v-if="!messages.length" class="py-12 text-center text-xs text-ink-muted">{{ t("ai.chat.empty") }}</p>
        <div
          v-for="(message, index) in messages"
          :key="index"
          class="flex gap-2"
          :class="message.role === 'user' ? 'justify-end' : 'justify-start'"
        >
          <div
            class="max-w-[92%] rounded-2xl border p-2.5 space-y-1.5"
            :class="message.role === 'user'
              ? 'bg-accent-bg border-accent/25'
              : 'bg-canvas border-line'"
          >
            <div class="flex items-center gap-1.5 text-[10px] font-bold text-ink-muted">
              <User v-if="message.role === 'user'" class="w-3 h-3" />
              <Bot v-else class="w-3 h-3 text-accent" />
              <span>{{ message.role === "user" ? t("ai.chat.you") : t("ai.chat.assistant") }}</span>
              <span v-if="message.streaming" class="text-accent">{{ t("ai.summarizing") }}</span>
            </div>

            <AiReasoningPanel :text="message.reasoning" />

            <p v-if="message.content" class="text-xs text-ink leading-relaxed whitespace-pre-wrap break-words select-text">{{ message.content }}</p>
            <p v-if="message.error" class="text-[11px] text-danger whitespace-pre-wrap break-words" role="alert">
              {{ t("ai.chat.failed", { message: message.error }) }}
            </p>
          </div>
        </div>
      </div>

      <div class="space-y-2">
        <textarea
          v-model="draft"
          rows="2"
          :placeholder="t('ai.chat.placeholder')"
          data-testid="ai-chat-input"
          class="w-full px-3 py-2 rounded-xl bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent resize-y"
          @keydown="onKeydown"
        ></textarea>
        <div class="flex flex-wrap items-center justify-between gap-2">
          <UiButton variant="ghost" :disabled="!messages.length" @click="resetConversation">
            <Trash2 class="w-3.5 h-3.5" />
            {{ t("ai.chat.clear") }}
          </UiButton>
          <div class="flex items-center gap-1.5">
            <UiButton v-if="isStreaming" variant="secondary" @click="stopStream">
              <Square class="w-3.5 h-3.5" />
              {{ t("ai.chat.stop") }}
            </UiButton>
            <UiButton v-else variant="primary" :disabled="!canSend" data-testid="ai-chat-send" @click="send">
              <Send class="w-3.5 h-3.5" />
              {{ t("ai.chat.send") }}
            </UiButton>
          </div>
        </div>
      </div>
    </div>
  </UiModal>
</template>
