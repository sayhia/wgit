<script setup lang="ts">
import { ref, watch, computed } from "vue";
import {
  FileText,
  Copy,
  Check,
  RefreshCw,
  Sparkles,
  AlertTriangle,
  GitCommit,
  Users,
} from "lucide-vue-next";
import UiModal from "../ui/UiModal.vue";
import UiButton from "../ui/UiButton.vue";
import { gitService } from "../../services/gitService";
import type { ReleaseChangelogResult } from "../../types";
import { t } from "../../i18n";

const props = defineProps<{
  open: boolean;
  repoPath: string;
  initialFromRef?: string;
  initialToRef?: string;
}>();

const emit = defineEmits<{
  (e: "update:open", value: boolean): void;
  (e: "notify", msg: string): void;
}>();

const fromRef = ref(props.initialFromRef || "");
const toRef = ref(props.initialToRef || "HEAD");
const isLoading = ref(false);
const result = ref<ReleaseChangelogResult | null>(null);
const copied = ref(false);

watch(
  () => props.open,
  (open) => {
    if (open) {
      if (props.initialFromRef) fromRef.value = props.initialFromRef;
      if (props.initialToRef) toRef.value = props.initialToRef;
      if (!result.value) {
        void handleGenerate();
      }
    }
  }
);

async function handleGenerate() {
  if (!props.repoPath) return;
  isLoading.value = true;
  try {
    const res = await gitService.generateReleaseChangelog(
      props.repoPath,
      fromRef.value.trim(),
      toRef.value.trim() || "HEAD"
    );
    result.value = res;
    if (res.fromRef && !fromRef.value) {
      fromRef.value = res.fromRef;
    }
  } catch (err: any) {
    emit("notify", t("release.changelog.toast.failed", { error: err?.message || err }));
  } finally {
    isLoading.value = false;
  }
}

async function handleCopy() {
  if (!result.value?.markdown) return;
  try {
    await navigator.clipboard.writeText(result.value.markdown);
    copied.value = true;
    setTimeout(() => {
      copied.value = false;
    }, 2000);
    emit("notify", t("release.changelog.toast.copied"));
  } catch {
    /* ignore */
  }
}

const categoryCounts = computed(() => {
  if (!result.value?.categories) return [];
  const entries = Object.entries(result.value.categories);
  return entries
    .map(([key, list]) => ({ key, count: list.length }))
    .filter((c) => c.count > 0);
});
</script>

<template>
  <UiModal
    :open="open"
    :title="t('release.changelog.modal.title')"
    width="max-w-[720px]"
    @close="emit('update:open', false)"
  >
    <div class="space-y-4">
      <!-- Range Selector Controls -->
      <div class="p-3.5 rounded-xl bg-surface border border-line space-y-3">
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="text-xs font-medium text-ink block mb-1">
              {{ t("release.changelog.from.label") }}
            </label>
            <input
              v-model="fromRef"
              type="text"
              :placeholder="t('release.changelog.from.placeholder')"
              class="w-full px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs font-mono text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
            />
          </div>
          <div>
            <label class="text-xs font-medium text-ink block mb-1">
              {{ t("release.changelog.to.label") }}
            </label>
            <input
              v-model="toRef"
              type="text"
              placeholder="HEAD"
              class="w-full px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs font-mono text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
            />
          </div>
        </div>

        <div class="flex items-center justify-between pt-1">
          <span class="text-[11px] text-ink-muted">
            {{ t("release.changelog.range.desc") }}
          </span>
          <UiButton
            variant="primary"
            size="sm"
            :disabled="isLoading"
            @click="handleGenerate"
            class="flex items-center gap-1.5"
          >
            <RefreshCw v-if="isLoading" class="w-3.5 h-3.5 animate-spin" aria-hidden="true" />
            <Sparkles v-else class="w-3.5 h-3.5" aria-hidden="true" />
            <span>{{ t("release.changelog.btn.generate") }}</span>
          </UiButton>
        </div>
      </div>

      <!-- Result View -->
      <div v-if="result" class="space-y-3">
        <!-- Stats Summary -->
        <div class="grid grid-cols-3 gap-2 text-center">
          <div class="p-2.5 rounded-lg bg-surface border border-line">
            <span class="text-[10px] text-ink-muted block uppercase">{{ t("release.changelog.stat.commits") }}</span>
            <span class="text-base font-bold font-mono text-ink">{{ result.totalCommits }}</span>
          </div>
          <div class="p-2.5 rounded-lg bg-surface border border-line">
            <span class="text-[10px] text-ink-muted block uppercase">{{ t("release.changelog.stat.breaking") }}</span>
            <span
              class="text-base font-bold font-mono"
              :class="result.breakingChanges.length > 0 ? 'text-danger' : 'text-success'"
            >
              {{ result.breakingChanges.length }}
            </span>
          </div>
          <div class="p-2.5 rounded-lg bg-surface border border-line">
            <span class="text-[10px] text-ink-muted block uppercase">{{ t("release.changelog.stat.contributors") }}</span>
            <span class="text-base font-bold font-mono text-purple">{{ result.contributors.length }}</span>
          </div>
        </div>

        <!-- Category Badges -->
        <div v-if="categoryCounts.length > 0" class="flex flex-wrap items-center gap-1.5">
          <span
            v-for="cat in categoryCounts"
            :key="cat.key"
            class="px-2 py-0.5 rounded text-[10px] font-mono font-semibold bg-raised border border-line text-ink-secondary"
          >
            {{ cat.key }}: {{ cat.count }}
          </span>
        </div>

        <!-- Breaking Changes Alert -->
        <div
          v-if="result.breakingChanges.length > 0"
          class="p-2.5 rounded-lg bg-danger-bg border border-danger/30 text-xs text-danger flex items-start gap-2"
        >
          <AlertTriangle class="w-4 h-4 shrink-0 mt-0.5" />
          <div>
            <span class="font-bold block">{{ t("release.changelog.breaking.alert") }}</span>
            <ul class="list-disc list-inside mt-1 space-y-0.5 text-[11px]">
              <li v-for="b in result.breakingChanges" :key="b.hash">
                <span class="font-semibold">{{ b.scope ? `${b.scope}: ` : "" }}</span>
                <span>{{ b.subject }}</span>
              </li>
            </ul>
          </div>
        </div>

        <!-- Markdown Document Area -->
        <div class="space-y-1.5">
          <div class="flex items-center justify-between">
            <label class="text-xs font-semibold text-ink flex items-center gap-1.5">
              <FileText class="w-3.5 h-3.5 text-accent" aria-hidden="true" />
              <span>{{ t("release.changelog.preview.label") }}</span>
            </label>
            <UiButton
              variant="secondary"
              size="xs"
              @click="handleCopy"
              class="flex items-center gap-1"
            >
              <Check v-if="copied" class="w-3 h-3 text-success" aria-hidden="true" />
              <Copy v-else class="w-3 h-3" aria-hidden="true" />
              <span>{{ copied ? t("ws.copy.done") : t("ws.copy") }}</span>
            </UiButton>
          </div>
          <textarea
            :value="result.markdown"
            readonly
            rows="10"
            class="w-full p-3 rounded-lg bg-canvas border border-line font-mono text-xs text-ink leading-relaxed focus:outline-none resize-none select-all"
          />
        </div>
      </div>
    </div>

    <template #footer>
      <div class="flex items-center justify-end">
        <UiButton variant="secondary" size="sm" @click="emit('update:open', false)">
          {{ t("common.close") }}
        </UiButton>
      </div>
    </template>
  </UiModal>
</template>
