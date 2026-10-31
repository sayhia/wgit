<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import {
  AlertCircle,
  ExternalLink,
  GitPullRequest,
  Loader2,
  RefreshCw,
} from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import {
  createTAPDReviewSource,
  ReviewSourceError,
  type ReviewDetail,
  type TAPDReviewSource,
} from "../../services/reviewSource";
import type { TAPDWorkItem } from "../../types";
import { t } from "../../i18n";

const props = defineProps<{
  currentRepoPath: string;
  /** Test seam; production uses the local TAPD binding adapter. */
  source?: TAPDReviewSource;
}>();

const emit = defineEmits<{
  (e: "switch-mode", mode: "local" | "remote" | "tapd"): void;
}>();

const items = ref<TAPDWorkItem[]>([]);
const message = ref("");
const errorMessage = ref("");
const isLoading = ref(false);
const isLoadingDetail = ref(false);
const selectedId = ref("");
const detail = ref<ReviewDetail | null>(null);
const detailNative = ref<TAPDWorkItem | null>(null);

const source = computed<TAPDReviewSource>(
  () => props.source ?? createTAPDReviewSource(props.currentRepoPath)
);
const counts = computed(() => ({
  total: items.value.length,
  stories: items.value.filter((item) => item.type === "story").length,
  tasks: items.value.filter((item) => item.type === "task").length,
  bugs: items.value.filter((item) => item.type === "bug").length,
}));

function typeLabel(type: string): string {
  if (type === "bug") return t("review.tapd.type.bug");
  if (type === "task") return t("review.tapd.type.task");
  return t("review.tapd.type.story");
}

function typeClass(type: string): string {
  if (type === "bug") return "bg-danger/10 text-danger border-danger/30";
  if (type === "task") return "bg-accent/10 text-accent border-accent/30";
  return "bg-purple/10 text-purple border-purple/30";
}

function shortTime(value: string): string {
  return value ? value.replace("T", " ").slice(0, 16) : "—";
}

async function loadDetail(id: string): Promise<void> {
  if (!id) return;
  selectedId.value = id;
  isLoadingDetail.value = true;
  detail.value = null;
  detailNative.value = null;
  try {
    const next = await source.value.detail(id);
    detail.value = next;
    detailNative.value = (next.native as TAPDWorkItem | undefined) ?? null;
  } catch (err) {
    if (err instanceof ReviewSourceError && err.code === "not_found") {
      errorMessage.value = t("review.tapd.detail.notfound");
    } else {
      errorMessage.value = t("review.tapd.detail.failed", {
        error: err instanceof Error ? err.message : String(err),
      });
    }
  } finally {
    isLoadingDetail.value = false;
  }
}

async function load(): Promise<void> {
  isLoading.value = true;
  errorMessage.value = "";
  try {
    const result = await source.value.list();
    message.value = result.message;
    items.value = (result.nativeItems as TAPDWorkItem[]) ?? [];
    const keep = items.value.some((item) => item.id === selectedId.value);
    if (!keep) {
      selectedId.value = items.value[0]?.id ?? "";
      detail.value = null;
      detailNative.value = null;
    }
    if (selectedId.value) await loadDetail(selectedId.value);
  } catch (err) {
    errorMessage.value = t("review.tapd.load.failed", {
      error: err instanceof Error ? err.message : String(err),
    });
  } finally {
    isLoading.value = false;
  }
}

async function openInTAPD(url: string): Promise<void> {
  if (!url) return;
  try {
    await gitService.openWebURL(url);
  } catch (err) {
    errorMessage.value = t("review.tapd.open.failed", {
      error: err instanceof Error ? err.message : String(err),
    });
  }
}

watch(
  () => props.currentRepoPath,
  () => {
    selectedId.value = "";
    void load();
  }
);

onMounted(load);
</script>

<template>
  <section class="flex-1 flex flex-col min-h-0" data-testid="tapd-review-view">
    <header class="min-h-12 shrink-0 border-b border-line px-4 py-1.5 flex flex-wrap items-center gap-x-3 gap-y-1.5 bg-surface/40 backdrop-blur-sm">
      <div class="flex items-center gap-2 shrink-0">
        <div class="w-7 h-7 rounded-lg bg-purple/15 text-purple flex items-center justify-center">
          <GitPullRequest class="w-4 h-4" />
        </div>
        <span class="text-xs font-semibold text-ink whitespace-nowrap">{{ t("review.tapd.title") }}</span>
      </div>

      <div class="flex items-center rounded-lg border border-line bg-raised/60 p-0.5 shrink-0" role="group" :aria-label="t('review.mode.label')">
        <button
          type="button"
          class="px-2 py-0.5 rounded-md text-[10px] font-semibold text-ink-muted hover:text-ink transition cursor-pointer"
          :aria-pressed="false"
          @click="emit('switch-mode', 'local')"
        >
          {{ t("review.mode.local") }}
        </button>
        <button
          type="button"
          class="px-2 py-0.5 rounded-md text-[10px] font-semibold text-ink-muted hover:text-ink transition cursor-pointer"
          :aria-pressed="false"
          @click="emit('switch-mode', 'remote')"
        >
          {{ t("review.mode.remote") }}
        </button>
        <button
          type="button"
          class="px-2 py-0.5 rounded-md text-[10px] font-semibold bg-surface text-accent shadow-sm cursor-default"
          :aria-pressed="true"
        >
          {{ t("review.mode.tapd") }}
        </button>
      </div>

      <span class="text-[10px] font-mono text-ink-muted">{{ t("review.tapd.readonly") }}</span>
      <div class="flex items-center gap-2 text-[10px] text-ink-muted">
        <span>{{ t("review.tapd.count", { count: counts.total }) }}</span>
        <span class="text-purple">{{ counts.stories }} S</span>
        <span class="text-accent">{{ counts.tasks }} T</span>
        <span class="text-danger">{{ counts.bugs }} B</span>
      </div>
      <button
        type="button"
        class="ml-auto flex items-center gap-1 px-2 py-1 rounded-lg bg-surface border border-line text-[11px] text-ink-muted hover:text-ink transition cursor-pointer disabled:opacity-40"
        :disabled="isLoading"
        :aria-label="t('review.tapd.refresh')"
        data-testid="tapd-review-refresh"
        @click="load"
      >
        <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isLoading }" aria-hidden="true" />
        {{ t("review.tapd.refresh") }}
      </button>
    </header>

    <div v-if="errorMessage" class="m-3 p-2.5 rounded-xl border border-danger/30 bg-danger/10 text-danger text-[11px] flex items-start gap-2" role="alert">
      <AlertCircle class="w-4 h-4 shrink-0 mt-0.5" />
      <span class="min-w-0 break-words">{{ errorMessage }}</span>
    </div>

    <div v-if="isLoading && items.length === 0" class="flex-1 flex items-center justify-center gap-2 text-xs text-ink-muted">
      <Loader2 class="w-4 h-4 animate-spin text-purple" />
      {{ t("review.tapd.loading") }}
    </div>
    <div v-else-if="items.length === 0" class="flex-1 flex flex-col items-center justify-center text-center gap-2 p-6" data-testid="tapd-review-empty">
      <GitPullRequest class="w-8 h-8 text-ink-muted" />
      <p class="text-xs font-semibold text-ink">{{ t("review.tapd.empty") }}</p>
      <p v-if="message" class="text-[11px] text-ink-muted leading-relaxed break-words">{{ message }}</p>
      <p v-else class="text-[11px] text-ink-muted">{{ t("review.tapd.empty.hint") }}</p>
    </div>
    <div v-else class="flex-1 min-h-0 flex flex-col lg:flex-row">
      <div class="lg:w-[360px] lg:shrink-0 border-b lg:border-b-0 lg:border-r border-line flex flex-col min-h-0">
        <div class="flex-1 overflow-y-auto p-2 space-y-1">
          <button
            v-for="item in items"
            :key="item.id"
            type="button"
            class="w-full text-left p-2.5 rounded-xl border transition cursor-pointer space-y-1"
            :class="item.id === selectedId ? 'bg-raised border-accent/40' : 'bg-surface border-line hover:bg-raised/60'"
            :data-testid="`tapd-item-${item.id}`"
            @click="loadDetail(item.id)"
          >
            <div class="flex items-center gap-1.5 min-w-0">
              <span class="px-1.5 py-0.5 rounded border text-[9px] font-bold shrink-0" :class="typeClass(item.type)">
                {{ typeLabel(item.type) }}
              </span>
              <span class="text-[10px] font-mono text-ink-muted shrink-0">#{{ item.id }}</span>
              <span class="ml-auto text-[9px] text-ink-muted shrink-0">{{ item.statusLabel || item.status }}</span>
            </div>
            <p class="text-[11px] font-semibold text-ink truncate">{{ item.title }}</p>
            <p class="text-[10px] text-ink-muted truncate">
              {{ item.owner || "—" }} · {{ shortTime(item.updatedTime) }}
            </p>
          </button>
        </div>
      </div>

      <div class="flex-1 min-h-0 overflow-y-auto p-4">
        <div v-if="isLoadingDetail" class="h-full flex items-center justify-center gap-2 text-xs text-ink-muted">
          <Loader2 class="w-4 h-4 animate-spin text-purple" />
          {{ t("review.tapd.loading") }}
        </div>
        <div v-else-if="!detail" class="h-full flex flex-col items-center justify-center text-center gap-2 text-ink-muted">
          <GitPullRequest class="w-8 h-8 opacity-40" />
          <p class="text-xs">{{ t("review.tapd.select") }}</p>
        </div>
        <article v-else class="space-y-4" data-testid="tapd-detail">
          <div class="space-y-1">
            <div class="flex items-center gap-2 flex-wrap">
              <span class="px-1.5 py-0.5 rounded border text-[9px] font-bold" :class="typeClass(detailNative?.type ?? 'story')">
                {{ typeLabel(detailNative?.type ?? "story") }}
              </span>
              <span class="text-[10px] font-mono text-ink-muted">#{{ detail.summary.id }}</span>
              <span class="text-[10px] px-1.5 py-0.5 rounded bg-canvas border border-line text-ink-muted">
                {{ detailNative?.statusLabel || detail.summary.state }}
              </span>
            </div>
            <h2 class="text-sm font-bold text-ink leading-snug">{{ detail.summary.title }}</h2>
          </div>

          <dl class="grid grid-cols-1 sm:grid-cols-2 gap-2 text-[11px]">
            <div class="p-2 rounded-lg bg-canvas border border-line">
              <dt class="text-[10px] text-ink-muted">{{ t("review.tapd.owner") }}</dt>
              <dd class="text-ink truncate">{{ detailNative?.owner || "—" }}</dd>
            </div>
            <div class="p-2 rounded-lg bg-canvas border border-line">
              <dt class="text-[10px] text-ink-muted">{{ t("review.tapd.creator") }}</dt>
              <dd class="text-ink truncate">{{ detailNative?.creator || "—" }}</dd>
            </div>
            <div class="p-2 rounded-lg bg-canvas border border-line">
              <dt class="text-[10px] text-ink-muted">{{ t("review.tapd.priority") }}</dt>
              <dd class="text-ink truncate">{{ detailNative?.priorityLabel || detailNative?.priority || "—" }}</dd>
            </div>
            <div class="p-2 rounded-lg bg-canvas border border-line">
              <dt class="text-[10px] text-ink-muted">{{ t("review.tapd.iteration") }}</dt>
              <dd class="text-ink truncate">{{ detailNative?.iteration || "—" }}</dd>
            </div>
            <div class="p-2 rounded-lg bg-canvas border border-line sm:col-span-2">
              <dt class="text-[10px] text-ink-muted">{{ t("review.tapd.updated") }}</dt>
              <dd class="text-ink font-mono">{{ shortTime(detailNative?.updatedTime ?? detail.summary.updatedAt) }}</dd>
            </div>
          </dl>

          <div class="space-y-1.5">
            <h3 class="text-[11px] font-bold text-ink">{{ t("review.tapd.description") }}</h3>
            <pre v-if="detail.body" class="p-3 rounded-xl bg-canvas border border-line text-[11px] text-ink-secondary whitespace-pre-wrap select-text font-sans">{{ detail.body }}</pre>
            <p v-else class="text-[11px] text-ink-muted">{{ t("review.tapd.no.description") }}</p>
          </div>

          <div v-if="detailNative?.boundCommits?.length" class="space-y-1.5">
            <h3 class="text-[11px] font-bold text-ink">{{ t("review.tapd.bound") }}</h3>
            <ul class="space-y-1">
              <li v-for="commit in detailNative.boundCommits" :key="commit" class="text-[10px] font-mono text-ink-muted px-2 py-1 rounded bg-canvas border border-line">
                {{ commit }}
              </li>
            </ul>
          </div>

          <button
            v-if="detail.summary.webUrl"
            type="button"
            class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface border border-line text-[11px] text-ink-muted hover:text-ink transition cursor-pointer"
            data-testid="tapd-open"
            @click="openInTAPD(detail.summary.webUrl)"
          >
            <ExternalLink class="w-3.5 h-3.5" aria-hidden="true" />
            {{ t("review.tapd.open") }}
          </button>
        </article>
      </div>
    </div>
  </section>
</template>
