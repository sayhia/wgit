<script setup lang="ts">
import { computed } from "vue";
import { Tag } from "lucide-vue-next";
import UiModal from "../../ui/UiModal.vue";
import UiButton from "../../ui/UiButton.vue";
import { t } from "../../../i18n";

const props = defineProps<{
  open: boolean;
  name: string;
  message: string;
  sign: boolean;
}>();

const emit = defineEmits<{
  (e: "update:open", value: boolean): void;
  (e: "update:name", value: string): void;
  (e: "update:message", value: string): void;
  (e: "update:sign", value: boolean): void;
  (e: "confirm"): void;
}>();

const nameModel = computed({
  get: () => props.name,
  set: (value: string) => emit("update:name", value),
});
const messageModel = computed({
  get: () => props.message,
  set: (value: string) => emit("update:message", value),
});
const signModel = computed({
  get: () => props.sign,
  set: (value: boolean) => emit("update:sign", value),
});
</script>

<template>
  <UiModal
    :open="open"
    :label="t('branches.tag.create.title')"
    width="max-w-sm"
    @update:open="emit('update:open', $event)"
  >
    <template #header>
      <h3 class="text-sm font-bold text-ink flex items-center gap-2">
        <Tag class="w-4 h-4 text-purple" />
        <span>{{ t("branches.tag.create.heading") }}</span>
      </h3>
    </template>

    <div class="space-y-3">
      <div class="flex flex-col gap-1">
        <label class="text-[11px] text-ink-secondary font-medium" for="wgit-tag-name">
          {{ t("branches.tag.create.name") }}
        </label>
        <input
          id="wgit-tag-name"
          v-model="nameModel"
          type="text"
          placeholder="v1.0.0"
          class="w-full px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent font-mono"
        />
      </div>
      <div class="flex flex-col gap-1">
        <label class="text-[11px] text-ink-secondary font-medium" for="wgit-tag-message">
          {{ t("branches.tag.create.annotation") }}
        </label>
        <input
          id="wgit-tag-message"
          v-model="messageModel"
          type="text"
          :placeholder="t('branches.tag.create.annotation.placeholder')"
          @keyup.enter="emit('confirm')"
          class="w-full px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent font-sans"
        />
      </div>
      <label class="flex items-start gap-2 text-[11px] text-ink-secondary cursor-pointer select-none">
        <input type="checkbox" v-model="signModel" class="mt-0.5 rounded accent-[var(--ug-purple-strong)]" />
        <span>{{ t("branches.tag.create.sign") }}</span>
      </label>
    </div>

    <template #footer>
      <UiButton variant="ghost" @click="emit('update:open', false)">{{ t("branches.tag.create.cancel") }}</UiButton>
      <UiButton variant="purple" @click="emit('confirm')">{{ t("branches.tag.create.confirm") }}</UiButton>
    </template>
  </UiModal>
</template>
