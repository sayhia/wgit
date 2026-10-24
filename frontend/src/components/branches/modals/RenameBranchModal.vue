<script setup lang="ts">
import { computed } from "vue";
import { Edit2 } from "lucide-vue-next";
import UiModal from "../../ui/UiModal.vue";
import UiButton from "../../ui/UiButton.vue";
import { t } from "../../../i18n";

const props = defineProps<{
  open: boolean;
  target: string;
  name: string;
  updateRemote: boolean;
}>();

const emit = defineEmits<{
  (e: "update:open", value: boolean): void;
  (e: "update:name", value: string): void;
  (e: "update:updateRemote", value: boolean): void;
  (e: "confirm"): void;
}>();

const nameModel = computed({
  get: () => props.name,
  set: (value: string) => emit("update:name", value),
});
</script>

<template>
  <UiModal
    :open="open"
    :label="t('branches.rename.modal.title')"
    width="max-w-sm"
    @update:open="emit('update:open', $event)"
  >
    <template #header>
      <h3 class="text-sm font-bold text-ink flex items-center gap-2">
        <Edit2 class="w-4 h-4 text-accent" />
        <span>{{ t("branches.rename.modal.title") }}</span>
      </h3>
    </template>

    <div class="space-y-3">
      <p class="text-xs text-ink-muted">
        {{ t("branches.rename.modal.intro", { name: target }) }}
      </p>
      <input
        v-model="nameModel"
        type="text"
        :aria-label="t('branches.rename.modal.name')"
        :placeholder="t('branches.rename.modal.name')"
        @keyup.enter="emit('confirm')"
        class="w-full px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent font-mono"
      />
      <label class="flex items-start gap-2 text-[11px] text-ink-muted cursor-pointer select-none">
        <input
          :checked="updateRemote"
          @change="emit('update:updateRemote', ($event.target as HTMLInputElement).checked)"
          type="checkbox"
          class="mt-0.5 rounded accent-[var(--ug-accent-strong)]"
        />
        <span>{{ t("branches.rename.modal.remote") }}</span>
      </label>
    </div>

    <template #footer>
      <UiButton variant="ghost" @click="emit('update:open', false)">{{ t("branches.rename.modal.cancel") }}</UiButton>
      <UiButton variant="primary" @click="emit('confirm')">{{ t("branches.rename.modal.confirm") }}</UiButton>
    </template>
  </UiModal>
</template>
