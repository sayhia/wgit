<script setup lang="ts">
import { computed } from "vue";
import type { PrFile, PrComment } from "../../types";
import { parseUnifiedPatch, type PrPatchLine } from "../../utils/prPatch";
import { fileStatusChip, fileStatusLabel } from "./meta";
import { t } from "../../i18n";

const props = defineProps<{
  file: PrFile;
  comments: PrComment[];
  expanded: boolean;
}>();

const emit = defineEmits<{
  (e: "toggle"): void;
  (e: "anchor", payload: { path: string; line: number; side: string }): void;
}>();

const model = computed(() => parseUnifiedPatch(props.file.patch));

function rowClass(line: PrPatchLine): string {
  switch (line.kind) {
    case "add":
      return "bg-success/10";
    case "del":
      return "bg-danger/10";
    case "meta":
      return "bg-raised/60 text-ink-muted";
    default:
      return "";
  }
}

function lineNo(line: PrPatchLine): string {
  return String(line.newLine ?? line.oldLine ?? "");
}

function anchorOf(line: PrPatchLine): void {
  if (line.kind === "meta") return;
  if (line.newLine != null) {
    emit("anchor", { path: props.file.path, line: line.newLine, side: "new" });
  } else if (line.oldLine != null) {
    emit("anchor", { path: props.file.path, line: line.oldLine, side: "old" });
  }
}
</script>

<template>
  <div class="border border-line rounded-xl overflow-hidden bg-canvas/40">
    <button
      type="button"
      class="w-full flex items-center gap-2 px-3 py-2 text-left hover:bg-raised/60 transition cursor-pointer"
      :aria-expanded="expanded"
      @click="emit('toggle')"
    >
      <span class="text-[10px] font-bold px-1.5 py-0.5 rounded" :class="fileStatusChip(file.status)">
        {{ fileStatusLabel(file.status) }}
      </span>
      <span class="flex-1 min-w-0 text-[11px] font-mono text-ink truncate" :title="file.path">
        {{ file.path }}
      </span>
      <span v-if="file.oldPath && file.oldPath !== file.path" class="text-[10px] text-ink-muted truncate max-w-[140px]" :title="file.oldPath">
        ← {{ file.oldPath }}
      </span>
      <span class="text-[10px] font-mono text-success shrink-0">+{{ file.additions }}</span>
      <span class="text-[10px] font-mono text-danger shrink-0">-{{ file.deletions }}</span>
      <span
        v-if="comments.length"
        class="text-[10px] px-1.5 py-0.5 rounded bg-accent/12 text-accent shrink-0"
      >
        {{ comments.length }}
      </span>
      <span class="text-[10px] text-ink-muted shrink-0">{{ expanded ? "▾" : "▸" }}</span>
    </button>

    <div v-if="expanded" class="border-t border-line">
      <div v-if="file.binary" class="px-3 py-3 text-[11px] text-ink-muted">
        {{ t("pr.files.binary") }}
      </div>
      <div v-else class="max-h-[460px] overflow-auto font-mono text-[11px] leading-5">
        <div
          v-for="(line, index) in model.lines"
          :key="index"
          class="flex items-start"
          :class="rowClass(line)"
        >
          <button
            v-if="line.kind !== 'meta'"
            type="button"
            class="w-11 shrink-0 text-right pr-1.5 text-ink-muted/60 hover:text-accent hover:bg-accent/10 select-none cursor-pointer"
            :title="t('pr.comment.anchor', { path: file.path, line: lineNo(line) })"
            @click="anchorOf(line)"
          >
            {{ lineNo(line) }}
          </button>
          <span v-else class="w-11 shrink-0"></span>
          <span class="w-3 shrink-0 text-ink-muted select-none">
            {{ line.kind === "add" ? "+" : line.kind === "del" ? "-" : " " }}
          </span>
          <pre class="flex-1 min-w-0 whitespace-pre-wrap break-all px-1">{{ line.text }}</pre>
        </div>
      </div>
      <p v-if="file.truncated" class="px-3 py-2 text-[10px] text-warning border-t border-line">
        补丁过大，已截断显示
      </p>
      <div v-if="comments.length" class="border-t border-line divide-y divide-line">
        <div v-for="comment in comments" :key="comment.id" class="px-3 py-2 space-y-1">
          <div class="flex items-center gap-2 text-[10px] text-ink-muted">
            <span class="font-semibold text-ink">{{ comment.author }}</span>
            <span v-if="comment.line" class="font-mono">:{{ comment.line }}</span>
            <span>{{ comment.createdAt.slice(0, 10) }}</span>
          </div>
          <p class="text-[11px] text-ink whitespace-pre-wrap break-words">{{ comment.body }}</p>
        </div>
      </div>
    </div>
  </div>
</template>
