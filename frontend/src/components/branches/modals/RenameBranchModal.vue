<script setup lang="ts">
import { computed } from "vue";
import { Edit2 } from "lucide-vue-next";
import UiModal from "../../ui/UiModal.vue";
import UiButton from "../../ui/UiButton.vue";

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
    label="重命名分支"
    width="max-w-sm"
    @update:open="emit('update:open', $event)"
  >
    <template #header>
      <h3 class="text-sm font-bold text-ink flex items-center gap-2">
        <Edit2 class="w-4 h-4 text-accent" />
        <span>重命名分支</span>
      </h3>
    </template>

    <div class="space-y-3">
      <p class="text-xs text-ink-muted">
        将分支 <span class="font-mono text-ink font-semibold">{{ target }}</span> 重命名为:
      </p>
      <input
        v-model="nameModel"
        type="text"
        aria-label="新分支名称"
        placeholder="新分支名称"
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
        <span>同时重命名远端分支（push 新分支名并删除旧远端分支）</span>
      </label>
    </div>

    <template #footer>
      <UiButton variant="ghost" @click="emit('update:open', false)">取消</UiButton>
      <UiButton variant="primary" @click="emit('confirm')">确认重命名</UiButton>
    </template>
  </UiModal>
</template>
