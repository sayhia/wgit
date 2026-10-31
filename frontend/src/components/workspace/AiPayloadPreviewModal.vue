<script setup lang="ts">
import { Copy } from "lucide-vue-next";
import UiModal from "../ui/UiModal.vue";
import UiButton from "../ui/UiButton.vue";
import { t } from "../../i18n";
import { pushToast } from "../../composables/useToast";

const props = defineProps<{ isOpen: boolean; text: string }>();
const emit = defineEmits<{ (e: "close"): void }>();

async function copy(): Promise<void> {
  try {
    await navigator.clipboard.writeText(props.text);
    pushToast(t("ws.copy.done"), { tone: "success", duration: 1500 });
  } catch {
    /* clipboard unavailable */
  }
}
</script>

<template>
  <UiModal
    :open="isOpen"
    :title="t('ai.preview.title')"
    width="max-w-3xl"
    @update:open="emit('close')"
  >
    <div class="space-y-2">
      <p class="text-[11px] text-ink-muted leading-relaxed">{{ t("ai.preview.hint") }}</p>
      <pre
        data-testid="ai-preview-content"
        class="max-h-[60vh] overflow-auto p-3 rounded-xl bg-canvas border border-line text-[11px] font-mono text-ink whitespace-pre-wrap select-text"
      >{{ text || t("ai.preview.empty") }}</pre>
    </div>
    <template #footer>
      <UiButton variant="secondary" @click="copy">
        <Copy class="w-3.5 h-3.5" />
        {{ t("ws.copy") }}
      </UiButton>
      <UiButton variant="primary" @click="emit('close')">{{ t("common.close") }}</UiButton>
    </template>
  </UiModal>
</template>
