<script setup lang="ts">
import { computed } from "vue";
import { GitPullRequest, ArrowRight, ShieldAlert, RefreshCw } from "lucide-vue-next";
import UiModal from "../../ui/UiModal.vue";
import UiButton from "../../ui/UiButton.vue";

const props = defineProps<{
  open: boolean;
  currentBranch: string;
  upstream: string;
  onto: string;
  autostash: boolean;
  autosquash: boolean;
  rebasing: boolean;
}>();

const emit = defineEmits<{
  (e: "update:open", value: boolean): void;
  (e: "update:onto", value: string): void;
  (e: "update:autostash", value: boolean): void;
  (e: "update:autosquash", value: boolean): void;
  (e: "confirm"): void;
}>();

const ontoModel = computed({
  get: () => props.onto,
  set: (value: string) => emit("update:onto", value),
});
const autostashModel = computed({
  get: () => props.autostash,
  set: (value: boolean) => emit("update:autostash", value),
});
const autosquashModel = computed({
  get: () => props.autosquash,
  set: (value: boolean) => emit("update:autosquash", value),
});
</script>

<template>
  <UiModal
    :open="open"
    label="变基分支"
    width="max-w-lg"
    @update:open="emit('update:open', $event)"
  >
    <template #header>
      <h3 class="text-sm font-bold text-ink flex items-center gap-2">
        <GitPullRequest class="w-4 h-4 text-accent" />
        <span>变基 (Rebase) 进阶控制</span>
      </h3>
    </template>

    <div class="space-y-3">
      <div class="p-3 rounded-xl bg-canvas border border-line flex items-center justify-between">
        <div class="flex flex-col">
          <span class="text-[10px] text-ink-muted">当前所在分支</span>
          <span class="font-mono text-xs font-bold text-ink">{{ currentBranch }}</span>
        </div>
        <ArrowRight class="w-4 h-4 text-ink-muted" />
        <div class="flex flex-col items-end">
          <span class="text-[10px] text-ink-muted">变基基线 (Upstream)</span>
          <span class="font-mono text-xs font-bold text-accent">{{ upstream }}</span>
        </div>
      </div>

      <div class="flex flex-col gap-1">
        <label class="text-[11px] text-ink-secondary font-medium" for="wgit-rebase-onto">
          指定移植新基准 (可选 --onto)
        </label>
        <input
          id="wgit-rebase-onto"
          v-model="ontoModel"
          type="text"
          placeholder="留空则直接变基到基线上；或填入新基准 Commit / 分支"
          class="w-full px-3 py-1.5 rounded-lg bg-canvas border border-line text-xs font-mono text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
        />
      </div>

      <div class="space-y-2 p-3 rounded-xl bg-canvas border border-line/60">
        <label class="flex items-start gap-2 text-xs text-ink cursor-pointer select-none">
          <input v-model="autostashModel" type="checkbox" class="mt-0.5 rounded accent-[var(--ug-accent-strong)]" />
          <span>自动储藏未提交修改 (--autostash)，变基完成后自动恢复</span>
        </label>
        <label class="flex items-start gap-2 text-xs text-ink cursor-pointer select-none">
          <input v-model="autosquashModel" type="checkbox" class="mt-0.5 rounded accent-[var(--ug-accent-strong)]" />
          <span>自动处理 fixup! 和 squash! 提交 (--autosquash)</span>
        </label>
      </div>

      <div class="p-3 rounded-xl bg-warning/10 border border-warning/20 text-xs text-warning flex items-start gap-2">
        <ShieldAlert class="w-4 h-4 shrink-0 mt-0.5" />
        <span>变基会重写当前分支历史并生成新 Commit Hash。若产生冲突，工作区横幅将提供「继续/跳过/放弃」操作。</span>
      </div>
    </div>

    <template #footer>
      <UiButton variant="ghost" :disabled="rebasing" @click="emit('update:open', false)">取消</UiButton>
      <UiButton variant="primary" :disabled="rebasing" @click="emit('confirm')">
        <RefreshCw v-if="rebasing" class="w-3.5 h-3.5 animate-spin" />
        <span>{{ rebasing ? "正在变基..." : "开始变基" }}</span>
      </UiButton>
    </template>
  </UiModal>
</template>
