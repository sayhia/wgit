<script setup lang="ts">
import { computed } from "vue";
import { GitBranch } from "lucide-vue-next";
import UiModal from "../../ui/UiModal.vue";
import UiButton from "../../ui/UiButton.vue";
import { t } from "../../../i18n";

const props = defineProps<{
  open: boolean;
  name: string;
  currentBranch: string;
  /** Candidate start points; the current branch is always offered first. */
  branches: string[];
  baseBranch: string;
}>();

const emit = defineEmits<{
  (e: "update:open", value: boolean): void;
  (e: "update:name", value: string): void;
  (e: "update:baseBranch", value: string): void;
  (e: "confirm"): void;
}>();

const nameModel = computed({
  get: () => props.name,
  set: (value: string) => emit("update:name", value),
});

const baseModel = computed({
  get: () => props.baseBranch || props.currentBranch,
  set: (value: string) => emit("update:baseBranch", value),
});

const baseOptions = computed(() => {
  const options = [props.currentBranch, ...props.branches].filter(Boolean);
  return [...new Set(options)];
});
</script>

<template>
  <UiModal
    :open="open"
    :label="t('branches.create.modal.title')"
    width="max-w-sm"
    @update:open="emit('update:open', $event)"
  >
    <template #header>
      <h3 class="text-sm font-bold text-ink flex items-center gap-2">
        <GitBranch class="w-4 h-4 text-accent" />
        <span>{{ t("branches.create.modal.title") }}</span>
      </h3>
    </template>

    <div class="space-y-3">
      <p class="text-xs text-ink-muted">
        {{ t("branches.create.modal.intro", { branch: baseModel }) }}
      </p>
      <label class="block space-y-1">
        <span class="text-xs font-semibold text-ink-secondary">{{ t("branches.create.modal.base") }}</span>
        <select
          v-model="baseModel"
          :aria-label="t('branches.create.modal.base')"
          class="w-full px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
        >
          <option v-for="option in baseOptions" :key="option" :value="option">{{ option }}</option>
        </select>
      </label>
      <label class="block space-y-1">
        <span class="text-xs font-semibold text-ink-secondary">{{ t("branches.create.modal.name") }}</span>
        <input
          v-model="nameModel"
          type="text"
          :aria-label="t('branches.create.modal.name')"
          :placeholder="t('branches.create.modal.name.placeholder')"
          @keyup.enter="emit('confirm')"
          class="w-full px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent font-mono"
        />
      </label>
    </div>

    <template #footer>
      <UiButton variant="ghost" @click="emit('update:open', false)">{{ t("branches.create.modal.cancel") }}</UiButton>
      <UiButton variant="primary" @click="emit('confirm')">{{ t("branches.create.modal.confirm") }}</UiButton>
    </template>
  </UiModal>
</template>
