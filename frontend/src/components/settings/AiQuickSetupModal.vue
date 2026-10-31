<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { Sparkles } from "lucide-vue-next";
import UiModal from "../ui/UiModal.vue";
import UiButton from "../ui/UiButton.vue";
import AiQuickSetup from "./AiQuickSetup.vue";
import { useAiSetup } from "../../composables/useAiSetup";
import { t } from "../../i18n";

const { aiSetupOpen, aiSetupReason, closeAiSetup } = useAiSetup();
const savedOnce = ref(false);
watch(
  () => aiSetupOpen.value,
  (open) => {
    if (open) savedOnce.value = false;
  }
);
const open = computed({
  get: () => aiSetupOpen.value,
  set: (value: boolean) => {
    if (!value) closeAiSetup();
  },
});
</script>

<template>
  <UiModal :open="open" :title="t('ai.setup.title')" width="max-w-4xl" @update:open="open = $event">
    <div class="space-y-3" data-testid="ai-quick-setup-modal">
      <div v-if="aiSetupReason" class="flex items-start gap-2 p-2.5 rounded-xl border border-warning/30 bg-warning/10 text-[11px] text-warning">
        <Sparkles class="w-4 h-4 shrink-0 mt-0.5" />
        <span class="break-words">{{ aiSetupReason }}</span>
      </div>
      <p class="text-[11px] text-ink-muted leading-relaxed">{{ t("ai.setup.desc") }}</p>
      <div class="max-h-[68vh] overflow-y-auto pr-1 pb-20">
        <AiQuickSetup v-if="aiSetupOpen" compact @saved="savedOnce = true" />
      </div>
    </div>
    <template #footer>
      <span v-if="savedOnce" class="text-[11px] text-success mr-auto">{{ t("ai.setup.savedhint") }}</span>
      <UiButton variant="secondary" data-testid="ai-quick-close" @click="closeAiSetup">{{ t("common.close") }}</UiButton>
    </template>
  </UiModal>
</template>
