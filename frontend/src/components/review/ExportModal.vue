<script setup lang="ts">
import { FileText, Check, Copy } from "lucide-vue-next";
import UiModal from "../ui/UiModal.vue";
import UiButton from "../ui/UiButton.vue";
import { t } from "../../i18n";

const props = defineProps<{
  open: boolean;
  markdown: string;
  copied: boolean;
}>();

const emit = defineEmits<{
  (e: "update:open", value: boolean): void;
  (e: "copy"): void;
}>();

</script>

<template>
  <UiModal
    :open="open"
    :title="t('review.export.title')"
    width="max-w-[720px]"
    body-class="-mx-5 -my-4 p-0"
    @update:open="emit('update:open', $event)"
  >
    <template #header>
      <div class="flex items-center gap-2 min-w-0">
        <FileText class="w-4 h-4 text-accent shrink-0" />
        <span class="text-xs font-semibold text-ink whitespace-nowrap">{{ t("review.export.title") }}</span>
        <span v-if="copied" class="text-[10px] text-success flex items-center gap-1 whitespace-nowrap">
          <Check class="w-3 h-3" />{{ t("review.export.copied") }}
        </span>
        <UiButton variant="secondary" size="xs" class="ml-2" @click="emit('copy')">
          <Copy class="w-3 h-3" />
          {{ t("review.export.copy") }}
        </UiButton>
      </div>
    </template>

    <pre class="p-4 text-[11px] leading-5 font-mono text-ink-secondary whitespace-pre-wrap break-words">{{ markdown }}</pre>
  </UiModal>
</template>
