<script setup lang="ts">
import { computed } from "vue";
import { GitBranch } from "lucide-vue-next";
import UiModal from "../../ui/UiModal.vue";
import UiButton from "../../ui/UiButton.vue";

const props = defineProps<{
  open: boolean;
  name: string;
  currentBranch: string;
}>();

const emit = defineEmits<{
  (e: "update:open", value: boolean): void;
  (e: "update:name", value: string): void;
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
    label="新建分支"
    width="max-w-sm"
    @update:open="emit('update:open', $event)"
  >
    <template #header>
      <h3 class="text-sm font-bold text-ink flex items-center gap-2">
        <GitBranch class="w-4 h-4 text-accent" />
        <span>新建分支</span>
      </h3>
    </template>

    <div class="space-y-3">
      <p class="text-xs text-ink-muted">
        从当前分支 <span class="font-mono text-ink font-semibold">{{ currentBranch }}</span> 检出新分支
      </p>
      <input
        v-model="nameModel"
        type="text"
        aria-label="新分支名称"
        placeholder="例如: feature/awesome-ui"
        @keyup.enter="emit('confirm')"
        class="w-full px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent font-mono"
      />
    </div>

    <template #footer>
      <UiButton variant="ghost" @click="emit('update:open', false)">取消</UiButton>
      <UiButton variant="primary" @click="emit('confirm')">创建分支</UiButton>
    </template>
  </UiModal>
</template>
