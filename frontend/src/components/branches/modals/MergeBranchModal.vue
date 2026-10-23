<script setup lang="ts">
import { computed } from "vue";
import { useModalA11y } from "../../../composables/useModalA11y";
import { GitMerge, ArrowRight, RefreshCw, CheckCircle2, AlertTriangle, Users } from "lucide-vue-next";
import type { MergePreviewResult, MergeOverlapResult } from "../../../types";

const props = defineProps<{
  open: boolean;
  source: string;
  currentBranch: string;
  preview: MergePreviewResult | null;
  loadingPreview: boolean;
  overlap: MergeOverlapResult | null;
  loadingOverlap: boolean;
  mode: "default" | "no-ff" | "squash" | "ff-only";
  strategy: "" | "ours" | "theirs";
  autostash: boolean;
  message: string;
  merging: boolean;
}>();

const emit = defineEmits<{
  (e: "update:open", value: boolean): void;
  (e: "update:mode", value: "default" | "no-ff" | "squash" | "ff-only"): void;
  (e: "update:strategy", value: "" | "ours" | "theirs"): void;
  (e: "update:autostash", value: boolean): void;
  (e: "update:message", value: string): void;
  (e: "confirm"): void;
}>();

const { containerRef: dialogRef } = useModalA11y(() => props.open, () => emit("update:open", false));

const modeModel = computed({
  get: () => props.mode,
  set: (value: "default" | "no-ff" | "squash" | "ff-only") => emit("update:mode", value),
});
const strategyModel = computed({
  get: () => props.strategy,
  set: (value: "" | "ours" | "theirs") => emit("update:strategy", value),
});
const autostashModel = computed({
  get: () => props.autostash,
  set: (value: boolean) => emit("update:autostash", value),
});
const messageModel = computed({
  get: () => props.message,
  set: (value: string) => emit("update:message", value),
});
</script>

<template>
    <!-- Merge Branch Modal Dialog -->
    <div
      v-if="open"
      ref="dialogRef"
      role="dialog"
      aria-modal="true"
      aria-label="合并分支"
      class="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center z-50 p-4"
    >
      <div class="w-[500px] max-h-[90vh] overflow-y-auto rounded-2xl bg-surface border border-line p-5 shadow-2xl flex flex-col gap-4">
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <GitMerge class="w-4 h-4 text-accent" />
          <span>高级分支合并 (Merge)</span>
        </h3>

        <div class="p-3 rounded-xl bg-canvas border border-line flex items-center justify-between">
          <div class="flex flex-col">
            <span class="text-[10px] text-ink-muted">源分支</span>
            <span class="font-mono text-xs font-bold text-accent">{{ source }}</span>
          </div>
          <ArrowRight class="w-4 h-4 text-ink-muted" />
          <div class="flex flex-col items-end">
            <span class="text-[10px] text-ink-muted">合并到当前分支</span>
            <span class="font-mono text-xs font-bold text-ink">{{ currentBranch }}</span>
          </div>
        </div>

        <!-- Pre-merge Conflict Preview Card -->
        <div class="rounded-xl border p-3 flex flex-col gap-2 text-xs" :class="preview?.canMergeCleanly ? 'bg-success/10 border-success/30' : preview?.hasConflicts ? 'bg-danger/10 border-danger/30' : 'bg-canvas border-line'">
          <div class="flex items-center justify-between">
            <span class="font-bold flex items-center gap-1.5" :class="preview?.canMergeCleanly ? 'text-success' : preview?.hasConflicts ? 'text-danger' : 'text-ink-secondary'">
              <span v-if="loadingPreview" class="flex items-center gap-1">
                <RefreshCw class="w-3.5 h-3.5 animate-spin" />
                正在预检合并冲突...
              </span>
              <span v-else-if="preview?.canMergeCleanly" class="flex items-center gap-1">
                <CheckCircle2 class="w-4 h-4" />
                预检通过：可安全自动合并（无代码冲突）
              </span>
              <span v-else-if="preview?.hasConflicts" class="flex items-center gap-1">
                <AlertTriangle class="w-4 h-4" />
                预检警告：检测到 {{ preview.conflicts.length }} 个文件存在冲突
              </span>
            </span>

            <span v-if="preview" class="text-[10px] text-ink-muted">
              变动 {{ preview.changedFiles.length }} 个文件
            </span>
          </div>

          <!-- Conflicted files list preview -->
          <div v-if="preview?.hasConflicts && preview.conflicts.length > 0" class="max-h-24 overflow-y-auto space-y-1 font-mono text-[10px] text-danger bg-danger/10 p-2 rounded-lg">
            <div v-for="cf in preview.conflicts" :key="cf" class="truncate">• {{ cf }}</div>
          </div>
        </div>

        <!-- Team overlap: merge-tree says nothing about WHO is editing the same files -->
        <div
          v-if="loadingOverlap || (overlap && overlap.overlapFiles.length > 0)"
          class="rounded-xl border p-3 flex flex-col gap-2 text-xs"
          :class="overlap?.riskLevel === 'high' ? 'bg-danger/10 border-danger/30' : overlap?.riskLevel === 'medium' ? 'bg-warning/10 border-warning/30' : 'bg-canvas border-line'"
        >
          <div class="flex items-center justify-between gap-2">
            <span class="font-bold flex items-center gap-1.5 text-ink">
              <Users class="w-3.5 h-3.5" />
              <span v-if="loadingOverlap" class="flex items-center gap-1">
                <RefreshCw class="w-3 h-3 animate-spin" />
                正在检查队友改动重叠...
              </span>
              <span v-else>{{ overlap?.overlapFiles.length }} 个文件双方都在改</span>
            </span>
            <span v-if="overlap" class="text-[10px] text-ink-muted shrink-0">
              我 {{ overlap.myFileCount }} · 对方 {{ overlap.teamFileCount }}
            </span>
          </div>

          <p v-if="overlap" class="text-[11px] text-ink-secondary leading-relaxed">{{ overlap.message }}</p>

          <div v-if="overlap && overlap.overlapFiles.length > 0" class="max-h-24 overflow-y-auto space-y-1">
            <div
              v-for="f in overlap.overlapFiles"
              :key="f.path"
              class="flex items-center gap-2 font-mono text-[10px] text-ink-muted"
            >
              <span class="truncate flex-1" :title="f.path">{{ f.path }}</span>
              <span class="shrink-0">{{ f.authors.join(" / ") }}</span>
            </div>
          </div>

          <p v-if="overlap && overlap.authors.length > 0" class="text-[10px] text-ink-muted">
            建议先与 {{ overlap.authors.join("、") }} 对齐这几处改动，再执行合并。
          </p>
        </div>

        <div class="flex flex-col gap-1.5">
          <label class="text-[11px] text-ink-secondary font-medium">合并策略</label>
          <div class="grid grid-cols-2 gap-2">
            <button
              @click="modeModel = 'no-ff'"
              class="p-2 rounded-lg border text-left text-xs transition cursor-pointer"
              :class="modeModel === 'no-ff' ? 'border-accent bg-accent/10 text-accent font-semibold' : 'border-line bg-canvas text-ink-secondary'"
            >
              <div>创建合并提交 (--no-ff)</div>
              <div class="text-[10px] text-ink-muted font-normal mt-0.5">完整保留分支历史记录</div>
            </button>
            <button
              @click="modeModel = 'squash'"
              class="p-2 rounded-lg border text-left text-xs transition cursor-pointer"
              :class="modeModel === 'squash' ? 'border-accent bg-accent/10 text-accent font-semibold' : 'border-line bg-canvas text-ink-secondary'"
            >
              <div>压缩提交 (--squash)</div>
              <div class="text-[10px] text-ink-muted font-normal mt-0.5">将所有改动压为一个新提交</div>
            </button>
            <button
              @click="modeModel = 'ff-only'"
              class="p-2 rounded-lg border text-left text-xs transition cursor-pointer"
              :class="modeModel === 'ff-only' ? 'border-accent bg-accent/10 text-accent font-semibold' : 'border-line bg-canvas text-ink-secondary'"
            >
              <div>仅快进 (--ff-only)</div>
              <div class="text-[10px] text-ink-muted font-normal mt-0.5">无法快进时中止合并</div>
            </button>
            <button
              @click="modeModel = 'default'"
              class="p-2 rounded-lg border text-left text-xs transition cursor-pointer"
              :class="modeModel === 'default' ? 'border-accent bg-accent/10 text-accent font-semibold' : 'border-line bg-canvas text-ink-secondary'"
            >
              <div>默认快速合并</div>
              <div class="text-[10px] text-ink-muted font-normal mt-0.5">按 Git 默认规则执行</div>
            </button>
          </div>
        </div>

        <!-- Strategy Options (-X ours / theirs) & Autostash -->
        <div class="grid grid-cols-2 gap-2">
          <div class="flex flex-col gap-1">
            <label class="text-[11px] text-ink-secondary font-medium">冲突自动偏好 (-X)</label>
            <select
              v-model="strategyModel"
              class="px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink focus:outline-none focus:border-accent"
            >
              <option value="">默认（冲突时标记并由人工裁决）</option>
              <option value="ours">优先保留当前分支内容 (-X ours)</option>
              <option value="theirs">优先采用源分支内容 (-X theirs)</option>
            </select>
          </div>

          <div class="flex flex-col justify-end">
            <label class="flex items-center gap-2 text-xs text-ink cursor-pointer select-none py-1">
              <input
                v-model="autostashModel"
                type="checkbox"
                class="rounded border-line bg-canvas text-accent focus:ring-0"
              />
              <span>自动暂存改动 (--autostash)</span>
            </label>
          </div>
        </div>

        <div class="flex flex-col gap-1">
          <label class="text-[11px] text-ink-secondary font-medium">提交说明</label>
          <input
            v-model="messageModel"
            type="text"
            class="w-full px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent font-sans"
          />
        </div>

        <div class="flex items-center justify-end gap-2 pt-2 border-t border-line/60">
          <button
            @click="emit('update:open', false)"
            class="px-3 py-1.5 rounded-lg text-xs text-ink-secondary hover:text-ink cursor-pointer"
            :disabled="merging"
          >
            取消
          </button>
          <button
            @click="emit('confirm')"
            class="flex items-center gap-1.5 px-4 py-1.5 rounded-lg bg-accent-strong text-white font-semibold text-xs cursor-pointer shadow-sm hover:bg-accent-strong-hover"
            :disabled="merging"
          >
            <RefreshCw v-if="merging" class="w-3.5 h-3.5 animate-spin" />
            <span>{{ merging ? '正在合并...' : '执行合并' }}</span>
          </button>
        </div>
      </div>
    </div>

</template>
