<script setup lang="ts">
import { AlertTriangle, CheckCircle2, RotateCcw } from "lucide-vue-next";

defineProps<{
  isMerging: boolean;
  isRebasing: boolean;
  isCherryPicking: boolean;
  conflictedFiles: string[];
  mergeHeadBranch: string;
  rebaseHeadName: string;
  rebaseStep: number;
  rebaseTotal: number;
}>();

const emit = defineEmits<{
  (e: "continue"): void;
  (e: "skip"): void;
  (e: "abort"): void;
}>();
</script>

<template>
  <div
    class="px-4 py-2.5 bg-danger/10 border-b border-danger/30 text-xs flex flex-wrap items-center justify-between gap-2 shrink-0 select-none"
  >
    <div class="flex items-center gap-2 text-danger font-medium">
      <AlertTriangle class="w-4 h-4 shrink-0 animate-pulse" />
      <span>
        <strong class="font-bold">
          {{ isMerging ? `正在进行合并 (${mergeHeadBranch || '传入分支'})` : isRebasing ? `正在进行变基 (${rebaseHeadName || '目标分支'} - 步骤 ${rebaseStep || 0}/${rebaseTotal || 0})` : isCherryPicking ? '正在进行 Cherry-Pick 操作' : '检测到未决代码冲突 (Conflict)' }}
        </strong>:
        <span v-if="conflictedFiles.length > 0">
          共 {{ conflictedFiles.length }} 个文件存在冲突，请快速裁决并暂存。
        </span>
        <span v-else class="text-success font-semibold">
          ✓ 所有冲突已解决！请点击「继续操作」完成当前提交。
        </span>
      </span>
    </div>

    <div class="flex items-center gap-2">
      <button
        @click="emit('continue')"
        :disabled="conflictedFiles.length > 0"
        class="flex items-center gap-1 px-3 py-1 rounded-lg text-xs font-bold transition cursor-pointer shadow-xs disabled:opacity-40 disabled:cursor-not-allowed"
        :class="conflictedFiles.length === 0 ? 'bg-success-strong text-white hover:bg-success-strong-hover' : 'bg-surface border border-line text-ink-muted'"
        title="继续执行下一步 (git rebase --continue / git commit)"
      >
        <CheckCircle2 class="w-3.5 h-3.5" />
        <span>{{ isMerging ? '完成合并提交' : '继续变基 (Continue)' }}</span>
      </button>

      <button
        v-if="isRebasing || isCherryPicking"
        @click="emit('skip')"
        class="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer"
        title="跳过当前提交 (git rebase --skip)"
      >
        <span>跳过此提交 (Skip)</span>
      </button>

      <button
        @click="emit('abort')"
        class="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-danger/20 hover:bg-danger/30 text-danger font-semibold text-xs transition cursor-pointer border border-danger/40"
        title="放弃操作并回退至先前状态"
      >
        <RotateCcw class="w-3.5 h-3.5" />
        <span>放弃 (Abort)</span>
      </button>
    </div>
  </div>
</template>
