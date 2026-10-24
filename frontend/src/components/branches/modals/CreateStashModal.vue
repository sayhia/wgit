<script setup lang="ts">
import { computed } from "vue";
import { Archive } from "lucide-vue-next";
import UiModal from "../../ui/UiModal.vue";
import UiButton from "../../ui/UiButton.vue";
import { t } from "../../../i18n";

const props = defineProps<{
  open: boolean;
  message: string;
  untracked: boolean;
  keep: boolean;
}>();

const emit = defineEmits<{
  (e: "update:open", value: boolean): void;
  (e: "update:message", value: string): void;
  (e: "update:untracked", value: boolean): void;
  (e: "update:keep", value: boolean): void;
  (e: "confirm"): void;
}>();

const messageModel = computed({
  get: () => props.message,
  set: (value: string) => emit("update:message", value),
});
const untrackedModel = computed({
  get: () => props.untracked,
  set: (value: boolean) => emit("update:untracked", value),
});
const keepModel = computed({
  get: () => props.keep,
  set: (value: boolean) => emit("update:keep", value),
});
</script>

<template>
  <UiModal
    :open="open"
    :label="t('branches.stash.create.title')"
    width="max-w-sm"
    @update:open="emit('update:open', $event)"
  >
    <template #header>
      <h3 class="text-sm font-bold text-ink flex items-center gap-2">
        <Archive class="w-4 h-4 text-warning" />
        <span>{{ t("branches.stash.create.heading") }}</span>
      </h3>
    </template>

    <div class="space-y-3">
      <div class="flex flex-col gap-1">
        <label class="text-[11px] text-ink-secondary font-medium" for="wgit-stash-message">{{ t("branches.stash.create.message") }}</label>
        <input
          id="wgit-stash-message"
          v-model="messageModel"
          type="text"
          :placeholder="t('branches.stash.create.message.placeholder')"
          @keyup.enter="emit('confirm')"
          class="w-full px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-warning font-sans"
        />
      </div>

      <div class="flex flex-col gap-2 pt-1">
        <label class="flex items-center gap-2 text-xs text-ink cursor-pointer select-none">
          <input
            v-model="untrackedModel"
            type="checkbox"
            class="rounded accent-[var(--ug-warning-strong)]"
          />
          <span>{{ t("branches.stash.create.untracked") }}</span>
        </label>
        <label class="flex items-center gap-2 text-xs text-ink cursor-pointer select-none">
          <input
            v-model="keepModel"
            type="checkbox"
            class="rounded accent-[var(--ug-warning-strong)]"
          />
          <span>{{ t("branches.stash.create.keep.index") }}</span>
        </label>
      </div>
    </div>

    <template #footer>
      <UiButton variant="ghost" @click="emit('update:open', false)">{{ t("branches.stash.create.cancel") }}</UiButton>
      <UiButton variant="warning" @click="emit('confirm')">{{ t("branches.stash.create.confirm") }}</UiButton>
    </template>
  </UiModal>
</template>
