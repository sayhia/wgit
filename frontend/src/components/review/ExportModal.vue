<script setup lang="ts">
import { FileText, Check, Copy, X } from "lucide-vue-next";
import { useModalA11y } from "../../composables/useModalA11y";

const props = defineProps<{
  open: boolean;
  markdown: string;
  copied: boolean;
}>();

const emit = defineEmits<{
  (e: "update:open", value: boolean): void;
  (e: "copy"): void;
}>();

const { containerRef: dialogRef } = useModalA11y(() => props.open, () => emit("update:open", false));
</script>

<template>
    <!-- Export markdown preview modal -->
    <div
      v-if="open"
      ref="dialogRef"
      role="dialog"
      aria-modal="true"
      aria-label="导出评审报告"
      @click.self="emit('update:open', false)"
      class="fixed inset-0 z-50 bg-black/60 backdrop-blur-md flex items-center justify-center p-8"
    >
      <div class="w-[720px] max-w-[calc(100vw-4rem)] max-h-[80vh] rounded-2xl bg-surface/95 border border-line shadow-2xl backdrop-blur-2xl flex flex-col overflow-hidden">
        <div class="h-11 shrink-0 px-4 border-b border-line flex items-center gap-2">
          <FileText class="w-4 h-4 text-accent" />
          <span class="text-xs font-semibold text-ink">评审报告 Markdown 预览</span>
          <span v-if="copied" class="text-[10px] text-success flex items-center gap-1">
            <Check class="w-3 h-3" />已复制到剪贴板
          </span>
          <div class="ml-auto flex items-center gap-1.5">
            <button
              @click="emit('copy')"
              class="flex items-center gap-1 px-2 py-1 rounded-lg bg-surface border border-line text-[11px] text-ink-secondary hover:text-ink hover:bg-raised transition cursor-pointer"
            >
              <Copy class="w-3.5 h-3.5" />
              复制
            </button>
            <button
              @click="emit('update:open', false)"
              class="flex items-center gap-1 px-2 py-1 rounded-lg bg-surface border border-line text-[11px] text-ink-secondary hover:text-ink hover:bg-raised transition cursor-pointer"
            >
              <X class="w-3.5 h-3.5" />
              关闭
            </button>
          </div>
        </div>
        <pre class="flex-1 overflow-auto p-4 text-[11px] leading-5 font-mono text-ink-secondary whitespace-pre-wrap break-words">{{ markdown }}</pre>
      </div>
    </div>
</template>
