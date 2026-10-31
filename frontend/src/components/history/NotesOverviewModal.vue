<script setup lang="ts">
import { t } from "../../i18n";
import { StickyNote } from "lucide-vue-next";
import UiModal from "../ui/UiModal.vue";
import type { CommitNotesResult, CommitNoteItem } from "../../types";

defineProps<{
  open: boolean;
  notesOverview: CommitNotesResult | null;
  isLoading: boolean;
}>();

const emit = defineEmits<{
  (e: "update:open", val: boolean): void;
  (e: "select-note", note: CommitNoteItem): void;
}>();
</script>

<template>
  <UiModal
    :open="open"
    :title="t('history.notes.title')"
    width="max-w-2xl"
    @update:open="emit('update:open', $event)"
  >
    <template #header>
      <div class="flex items-center gap-2 min-w-0">
        <StickyNote class="w-4 h-4 text-warning shrink-0" />
        <h3 class="text-sm font-bold text-ink whitespace-nowrap">{{ t("history.notes.title") }}</h3>
        <span v-if="notesOverview" class="text-[10px] text-ink-muted font-mono truncate">
          {{ t("history.notes.summary", { count: notesOverview.total, ref: notesOverview.ref }) }}
        </span>
      </div>
    </template>

    <div class="space-y-2">
      <div v-if="isLoading" class="py-10 text-center text-xs text-ink-muted">{{ t("history.notes.loading") }}</div>
      <div v-else-if="notesOverview && notesOverview.notes.length === 0" class="py-10 text-center text-xs text-ink-muted">
        {{ t("history.notes.empty") }}
      </div>
      <div
        v-for="note in notesOverview?.notes || []"
        :key="`${note.commitHash}:${note.date}`"
        role="button"
        tabindex="0"
        class="p-3 rounded-lg border border-line bg-surface hover:bg-raised transition cursor-pointer space-y-1.5 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/50"
        @click="emit('select-note', note)"
        @keydown.enter.prevent="emit('select-note', note)"
        @keydown.space.prevent="emit('select-note', note)"
      >
        <div class="flex items-center gap-2 min-w-0">
          <span class="font-mono text-accent text-[11px] font-bold shrink-0">{{ note.shortHash }}</span>
          <span class="text-xs text-ink font-medium truncate">{{ note.subject }}</span>
          <span class="ml-auto text-[10px] text-ink-muted shrink-0">{{ note.date }}</span>
        </div>
        <div class="text-[10px] text-ink-muted">{{ note.authorName }}</div>
        <div class="px-2 py-1 rounded bg-canvas border border-line/60 text-[11px] text-ink-secondary line-clamp-2">
          {{ note.preview || note.content }}
        </div>
      </div>
    </div>
  </UiModal>
</template>
