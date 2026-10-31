<script setup lang="ts">
import { ref } from "vue";
import { t } from "../../i18n";
import { Search } from "lucide-vue-next";
import UiModal from "../ui/UiModal.vue";
import { gitService } from "../../services/gitService";
import { writeClipboardText } from "../../utils/clipboard";
import type { PickaxeResult } from "../../types";

const props = defineProps<{
  open: boolean;
  repoPath: string;
}>();

const emit = defineEmits<{
  (e: "update:open", val: boolean): void;
  (e: "select-commit", hash: string): void;
  (e: "notify", msg: string): void;
}>();

const pickaxeQuery = ref("");
const pickaxeMode = ref<"content" | "regex">("content");
const pickaxeAuthor = ref("");
const pickaxePathFilter = ref("");
const pickaxeIgnoreCase = ref(false);
const pickaxeResult = ref<PickaxeResult | null>(null);
const isPickaxeSearching = ref(false);

async function handlePickaxeSearch() {
  if (!pickaxeQuery.value.trim() || !props.repoPath || isPickaxeSearching.value) return;
  isPickaxeSearching.value = true;
  try {
    const res = await gitService.pickaxeSearch(
      props.repoPath,
      pickaxeQuery.value.trim(),
      pickaxeMode.value,
      pickaxeAuthor.value.trim(),
      pickaxePathFilter.value.trim(),
      50,
      pickaxeIgnoreCase.value
    );
    pickaxeResult.value = res;
    if (res && res.totalHits === 0) {
      emit("notify", t("history.toast.pickaxe.none"));
    }
  } catch (err: any) {
    emit("notify", t("history.toast.pickaxe.failed", { error: err?.message || err }));
  } finally {
    isPickaxeSearching.value = false;
  }
}

async function copyPickaxeHash(hash: string) {
  const error = await writeClipboardText(hash);
  emit(
    "notify",
  error
      ? t("common.copy.failed", { error })
      : t("history.toast.copied", { text: hash.slice(0, 8) })
  );
  emit("select-commit", hash);
}

function pickaxeChangeClass(change: string) {
  switch (change) {
    case "added":
      return "bg-success-bg text-success";
    case "removed":
      return "bg-danger-bg text-danger";
    case "modified":
      return "bg-warning-bg text-warning";
    default:
      return "bg-surface text-ink-muted";
  }
}

function pickaxeChangeLabel(change: string) {
  switch (change) {
    case "added":
      return t("history.pickaxe.change.added");
    case "removed":
      return t("history.pickaxe.change.removed");
    case "modified":
      return t("history.pickaxe.change.changed");
    default:
      return change;
  }
}
</script>

<template>
  <UiModal
    :open="open"
    :title="t('history.pickaxe.title')"
    width="max-w-3xl"
    @update:open="emit('update:open', $event)"
  >
    <template #header>
      <h3 class="text-sm font-bold text-ink flex items-center gap-2">
        <Search class="w-4 h-4 text-purple" />
        {{ t("history.pickaxe.title") }}
      </h3>
    </template>

    <div class="space-y-3">
      <div class="flex flex-wrap items-center gap-2">
        <input
          v-model="pickaxeQuery"
          type="text"
          :placeholder="t('history.pickaxe.query.placeholder')"
          @keydown.enter="handlePickaxeSearch"
          class="flex-1 min-w-[220px] px-3 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink placeholder-ink-muted font-mono focus:outline-none focus:border-accent"
        />
        <select
          v-model="pickaxeMode"
          class="px-2 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink focus:outline-none focus:border-accent"
          title="content = git log -S; regex = git log -G"
        >
          <option value="content">{{ t("history.pickaxe.mode.content") }}</option>
          <option value="regex">{{ t("history.pickaxe.mode.regex") }}</option>
        </select>
        <input
          v-model="pickaxeAuthor"
          type="text"
          :placeholder="t('history.pickaxe.author.placeholder')"
          class="w-32 px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
        />
        <input
          v-model="pickaxePathFilter"
          type="text"
          :placeholder="t('history.pickaxe.path.placeholder')"
          class="w-40 px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink placeholder-ink-muted font-mono focus:outline-none focus:border-accent"
        />
        <label class="flex items-center gap-1.5 text-[11px] text-ink-muted cursor-pointer select-none">
          <input v-model="pickaxeIgnoreCase" type="checkbox" class="rounded accent-purple" />
          <span>{{ t("history.pickaxe.ignore.case") }}</span>
        </label>
        <button
          @click="handlePickaxeSearch"
          :disabled="!pickaxeQuery.trim() || isPickaxeSearching"
          class="px-3 py-1.5 rounded-lg bg-purple-strong text-white text-xs font-semibold transition cursor-pointer disabled:opacity-40 shrink-0"
        >
          {{ isPickaxeSearching ? t("history.pickaxe.searching") : t("history.pickaxe.search") }}
        </button>
      </div>

      <div v-if="pickaxeResult" class="flex flex-wrap items-center gap-3 px-3 py-2 rounded-lg bg-canvas border border-line text-[11px] text-ink-muted font-mono">
        <span>{{ t("history.pickaxe.hits", { hits: pickaxeResult.totalHits, analyzed: pickaxeResult.analyzedHits }) }}</span>
        <span class="text-success">+{{ pickaxeResult.addedLines }}</span>
        <span class="text-danger">-{{ pickaxeResult.removedLines }}</span>
        <span>{{ t("history.pickaxe.files", { count: pickaxeResult.fileCount }) }}</span>
        <span v-if="pickaxeResult.firstSeen">{{ t("history.pickaxe.first", { date: pickaxeResult.firstSeen }) }}</span>
        <span v-if="pickaxeResult.lastSeen">{{ t("history.pickaxe.last", { date: pickaxeResult.lastSeen }) }}</span>
        <span v-if="pickaxeResult.truncated" class="text-warning">{{ t("history.pickaxe.truncated") }}</span>
      </div>

      <div v-if="pickaxeResult && pickaxeResult.hits.length > 0" class="space-y-2">
        <div
          v-for="hit in pickaxeResult.hits"
          :key="`${hit.hash}:${hit.change}`"
          class="p-2.5 rounded-lg border border-line bg-surface space-y-1.5"
        >
          <div class="flex items-center gap-2 min-w-0">
            <button
              @click="copyPickaxeHash(hit.hash)"
              class="font-mono text-accent hover:underline text-[11px] font-bold shrink-0"
              :title="t('history.pickaxe.copy.hash')"
            >
              {{ hit.shortHash }}
            </button>
            <span
              class="px-1.5 py-0.2 rounded text-[9px] font-bold shrink-0"
              :class="pickaxeChangeClass(hit.change)"
            >
              {{ pickaxeChangeLabel(hit.change) }}
            </span>
            <span class="text-xs text-ink font-medium truncate">{{ hit.subject }}</span>
            <span class="ml-auto text-[10px] text-ink-muted font-mono shrink-0">{{ hit.date }}</span>
          </div>
          <div class="flex items-center gap-2 text-[10px] text-ink-muted">
            <span>{{ hit.author }}</span>
            <span class="font-mono text-success">+{{ hit.added }}</span>
            <span class="font-mono text-danger">-{{ hit.removed }}</span>
          </div>
          <div v-if="hit.snippet" class="px-2 py-1 rounded bg-canvas border border-line/60 font-mono text-[10px] text-ink-secondary break-all">
            {{ hit.snippet }}
          </div>
          <div v-if="hit.files.length > 0" class="text-[10px] text-ink-muted font-mono truncate" :title="hit.files.join(', ')">
            {{ hit.files.join(" · ") }}
          </div>
        </div>
      </div>
      <div v-else-if="pickaxeResult && !isPickaxeSearching" class="py-10 text-center text-xs text-ink-muted">
        {{ t("history.pickaxe.none") }}
      </div>
    </div>
  </UiModal>
</template>
