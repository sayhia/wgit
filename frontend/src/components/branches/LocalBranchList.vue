<script setup lang="ts">
import { computed, ref } from "vue";
import {
  Layers,
  RefreshCw,
  Trash2,
  Sparkles,
  AlertTriangle,
  GitBranch,
  ArrowRight,
  GitCommitHorizontal,
  CheckCircle2,
  Link2,
  GitMerge,
  GitPullRequest,
  Edit2,
  ExternalLink,
  ArrowDownCircle,
  Copy,
  ClipboardCheck,
} from "lucide-vue-next";
import { useContextMenu, type ContextMenuItem } from "../../composables/useContextMenu";
import { useRovingList } from "../../composables/useRovingList";
import UiContextMenu from "../ui/UiContextMenu.vue";
import type { BranchInfo, BranchInsight, BranchInsightSummary } from "../../types";
import { t } from "../../i18n";

interface LocalBranchRow {
  branch: BranchInfo;
  insight: BranchInsight | null;
  meta: { label: string; cls: string; desc: string } | null;
  cleanupCandidate: boolean;
}

const props = defineProps<{
  summary: BranchInsightSummary | null;
  loading: boolean;
  insightsLoading: boolean;
  error: string;
  cleanupDisabled: boolean;
  cleaningMerged: boolean;
  cleanableCount: number;
  filteredCount: number;
  rows: LocalBranchRow[];
  moreCount: number;
}>();

const emit = defineEmits<{
  (e: "retry-insights"): void;
  (e: "cleanup-merged"): void;
  (e: "set-upstream", branch: BranchInfo): void;
  (e: "checkout", name: string): void;
  (e: "merge", name: string): void;
  (e: "rebase", name: string): void;
  (e: "rename", name: string): void;
  (e: "open-web", name: string): void;
  (e: "cleanup", branch: BranchInfo): void;
  (e: "delete", name: string, force?: boolean): void;
  (e: "show-more"): void;
}>();

const { setItemRef, tabindexFor, onKeydown: onRowKeydown, onFocus: onRowFocus } = useRovingList(
  () => props.rows,
  (row) => row.branch.name
);

const {
  isOpen: branchMenuOpen,
  position: branchMenuPos,
  items: branchMenuList,
  open: openMenuAt,
  close: closeBranchMenu,
} = useContextMenu();
const menuBranch = ref<BranchInfo | null>(null);

function branchMenuItems(branch: BranchInfo): ContextMenuItem[] {
  const items: ContextMenuItem[] = [];
  if (!branch.isCurrent) {
    items.push({ id: "checkout", label: t("branches.local.menu.checkout"), icon: GitBranch });
    items.push({ id: "merge", label: t("branches.local.menu.merge"), icon: GitMerge });
    items.push({ id: "rebase", label: t("branches.local.menu.rebase"), icon: GitPullRequest });
  } else {
    items.push({
      id: "checkout",
      label: t("branches.local.menu.checkout.disabled"),
      icon: GitBranch,
      disabled: true,
    });
  }
  items.push({ id: "rename", label: t("branches.local.menu.rename"), icon: Edit2, separatorBefore: true });
  if (!branch.isCurrent) {
    items.push({ id: "set-upstream", label: t("branches.local.menu.upstream"), icon: Link2 });
  }
  items.push({ id: "copy", label: t("branches.local.menu.copy"), icon: Copy, hint: branch.name });
  items.push({ id: "open-web", label: t("branches.local.menu.open.web"), icon: ExternalLink });
  items.push({
    id: "cleanup",
    label: t("branches.local.menu.cleanup"),
    icon: ClipboardCheck,
    danger: true,
    separatorBefore: true,
    disabled: branch.isCurrent,
  });
  items.push({
    id: "delete",
    label: t("branches.local.menu.delete"),
    icon: Trash2,
    danger: true,
    disabled: branch.isCurrent,
  });
  return items;
}

function openBranchMenu(branch: BranchInfo, event: MouseEvent) {
  menuBranch.value = branch;
  openMenuAt(event, branchMenuItems(branch));
}

async function handleBranchMenuSelect(id: string) {
  const branch = menuBranch.value;
  if (!branch) return;
  switch (id) {
    case "checkout":
      emit("checkout", branch.name);
      break;
    case "merge":
      emit("merge", branch.name);
      break;
    case "rebase":
      emit("rebase", branch.name);
      break;
    case "rename":
      emit("rename", branch.name);
      break;
    case "set-upstream":
      emit("set-upstream", branch);
      break;
    case "copy":
      await navigator.clipboard.writeText(branch.name);
      break;
    case "open-web":
      emit("open-web", branch.name);
      break;
    case "cleanup":
      emit("cleanup", branch);
      break;
    case "delete":
      emit("delete", branch.name, true);
      break;
  }
}

/**
 * The backend also returns a rendered `summary.hint`, but it is generated in
 * Chinese. Rebuild the sentence from the counters so it follows the UI locale.
 */
const insightHint = computed(() => {
  const s = props.summary;
  if (!s) return "";
  const base = s.baseBranch || t("branches.local.insight.auto");
  if (s.merged > 0) return t("branches.local.insight.hint.merged", { count: s.merged, base });
  if (s.unpushed > 0) return t("branches.local.insight.hint.unpushed", { count: s.unpushed });
  if (s.stale > 0) return t("branches.local.insight.hint.stale", { count: s.stale, days: s.staleDays });
  return t("branches.local.insight.hint.healthy");
});

function formatInsightDay(ins?: BranchInsight): string {
  if (!ins?.lastDate) return "";
  const day = ins.lastDate.slice(0, 10);
  return /^\d{4}-\d{2}-\d{2}$/.test(day) ? day : ins.lastDate;
}

function formatDaysAgo(days?: number | null): string {
  if (days === undefined || days === null || Number.isNaN(days)) return "";
  if (days <= 0) return t("branches.local.days.today");
  if (days === 1) return t("branches.local.days.one");
  if (days < 30) return t("branches.local.days.n", { count: days });
  if (days < 365) return t("branches.local.months.n", { count: Math.floor(days / 30) });
  return t("branches.local.years.n", { count: Math.floor(days / 365) });
}
</script>

<template>
        <!-- Branch insights summary strip -->
        <div class="rounded-xl border border-line bg-surface/50 p-3 flex flex-col gap-2">
          <div class="flex items-center justify-between gap-3 flex-wrap">
            <div class="flex items-center gap-2 flex-wrap min-w-0">
              <span class="flex items-center gap-1.5 text-xs font-bold text-ink shrink-0">
                <Layers class="w-3.5 h-3.5 text-accent" />
                <span>{{ t("branches.local.insight.title") }}</span>
              </span>
              <RefreshCw v-if="insightsLoading && summary" class="w-3 h-3 text-ink-muted animate-spin shrink-0" :title="t('branches.local.insight.refreshing')" />

              <template v-if="summary">
                <span class="px-2 py-0.5 rounded-md bg-raised border border-line text-[11px] text-ink-secondary">
                  {{ t("branches.local.insight.local") }} <b class="text-ink">{{ summary.total }}</b>
                </span>
                <span v-if="summary.protected > 0" class="px-2 py-0.5 rounded-md bg-raised border border-line text-[11px] text-ink-secondary">
                  {{ t("branches.local.insight.protected") }} <b class="text-ink">{{ summary.protected }}</b>
                </span>
                <span class="px-2 py-0.5 rounded-md bg-success/10 border border-success/25 text-[11px] text-success">
                  {{ t("branches.local.insight.merged") }} <b>{{ summary.merged }}</b>
                </span>
                <span class="px-2 py-0.5 rounded-md bg-warning/10 border border-warning/25 text-[11px] text-warning">
                  {{ t("branches.local.insight.unpushed") }} <b>{{ summary.unpushed }}</b>
                </span>
                <span class="px-2 py-0.5 rounded-md bg-cyan/10 border border-cyan/25 text-[11px] text-cyan">
                  {{ t("branches.local.insight.stale") }} <b>{{ summary.stale }}</b>
                </span>
                <span class="px-2 py-0.5 rounded-md bg-raised border border-line text-[11px] text-ink-secondary">
                  {{ t("branches.local.insight.base") }}
                  <b class="font-mono text-ink">{{ summary.baseBranch || t("branches.local.insight.auto") }}</b>
                </span>
              </template>

              <span v-else-if="loading || insightsLoading" class="flex items-center gap-1.5 text-[11px] text-ink-muted">
                <RefreshCw class="w-3 h-3 animate-spin" />
                {{ t("branches.local.insight.analyzing") }}
              </span>
              <span v-else class="text-[11px] text-ink-muted">{{ error || t("branches.local.insight.empty") }}</span>
            </div>

            <div class="flex items-center gap-2 shrink-0">
              <button
                v-if="!summary && !insightsLoading"
                @click="emit('retry-insights')"
                class="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-raised hover:bg-hover text-ink-secondary hover:text-ink text-[11px] font-medium transition cursor-pointer border border-line"
:title="t('branches.local.insight.retry.hint')"
              >
                <RefreshCw class="w-3 h-3" />
                <span>{{ t("branches.local.insight.retry") }}</span>
              </button>
              <button
                @click="emit('cleanup-merged')"
                :disabled="cleanupDisabled"
                class="flex items-center gap-1.5 px-3 py-1 rounded-lg text-[11px] font-semibold transition border"
                :class="cleanupDisabled
                  ? 'bg-raised border-line text-ink-muted cursor-not-allowed'
                  : 'bg-danger/10 border-danger/30 text-danger hover:bg-danger/20 cursor-pointer'"
                :title="cleanupDisabled ? t('branches.local.cleanup.disabled') : t('branches.local.cleanup.hint')"
              >
                <RefreshCw v-if="cleaningMerged" class="w-3 h-3 animate-spin" />
                <Trash2 v-else class="w-3 h-3" />
                <span>{{ t("branches.local.cleanup") }}</span>
                <span v-if="cleanableCount > 0" class="font-mono">({{ cleanableCount }})</span>
              </button>
            </div>
          </div>

          <div v-if="insightHint" class="flex items-start gap-1.5 text-[11px] text-ink-muted">
            <Sparkles class="w-3 h-3 text-accent shrink-0 mt-0.5" />
            <span>{{ insightHint }}</span>
          </div>
          <div v-if="error && summary" class="flex items-start gap-1.5 text-[11px] text-ink-muted">
            <AlertTriangle class="w-3 h-3 shrink-0 mt-0.5" />
            <span>{{ t("branches.local.error.with.list", { error }) }}</span>
          </div>
        </div>

        <div v-if="!loading && filteredCount === 0" class="py-12 text-center text-xs text-ink-muted border border-dashed border-line rounded-2xl">
          {{ t("branches.local.empty") }}
        </div>

        <div
          v-for="(row, index) in rows"
          :key="row.branch.name"
          :ref="(el) => setItemRef(row, el)"
          :tabindex="tabindexFor(row, index)"
          :aria-label="
            row.branch.isCurrent
              ? t('branches.local.row.aria.current', { name: row.branch.name })
              : t('branches.local.row.aria', { name: row.branch.name })
          "
          class="flex items-center justify-between gap-2 p-3 rounded-xl bg-surface/60 border border-line hover:border-line-strong transition duration-150 group focus:outline-none focus-visible:ring-2 focus-visible:ring-accent/45"
          :class="{ 'border-accent/40 bg-accent/5': row.branch.isCurrent }"
          :title="row.meta?.desc || undefined"
          @focus="onRowFocus(row)"
          @keydown="onRowKeydown($event, index)"
          @keydown.enter.self="!row.branch.isCurrent && emit('checkout', row.branch.name)"
          @contextmenu="openBranchMenu(row.branch, $event)"
        >
          <div class="flex items-center gap-3 min-w-0">
            <div
              class="w-8 h-8 rounded-lg flex items-center justify-center shrink-0 shadow-inner"
              :class="row.branch.isCurrent ? 'bg-accent/20 text-accent ring-1 ring-accent/30' : 'bg-surface text-ink-muted'"
            >
              <GitBranch class="w-4 h-4" />
            </div>

            <div class="flex flex-col min-w-0">
              <div class="flex items-center gap-2 flex-wrap">
                <span class="font-mono text-xs font-bold text-ink">{{ row.branch.name }}</span>
                <span v-if="row.branch.isCurrent" class="px-2 py-0.5 rounded-full bg-accent/20 text-accent text-[10px] font-bold border border-accent/30">
                  {{ t("branches.local.current") }}
                </span>
                <span
                  v-if="row.meta && !row.branch.isCurrent"
                  class="px-1.5 py-0.5 rounded-full border text-[10px] font-bold"
                  :class="row.meta.cls"
                  :title="row.meta.desc"
                >
                  {{ row.meta.label }}
                </span>
                <span v-if="row.insight?.hasUpstream && row.insight?.upstream" class="text-[10px] text-ink-muted font-mono flex items-center gap-1">
                  <ArrowRight class="w-2.5 h-2.5 opacity-60" /> {{ row.insight.upstream }}
                </span>
                <span v-else-if="row.branch.upstream" class="text-[10px] text-ink-muted font-mono flex items-center gap-1">
                  <ArrowRight class="w-2.5 h-2.5 opacity-60" /> {{ row.branch.upstream }}
                </span>
              </div>

              <!-- Last commit metadata from branch insights -->
              <div v-if="row.insight" class="flex items-center gap-1.5 text-[11px] text-ink-secondary mt-0.5 min-w-0 max-w-lg">
                <GitCommitHorizontal class="w-3 h-3 shrink-0 text-ink-muted" />
                <span class="font-mono text-[10px] text-ink-muted shrink-0">{{ row.insight.shortHash }}</span>
                <span class="truncate">{{ row.insight.lastSubject || t("branches.local.no.subject") }}</span>
                <span class="shrink-0 text-ink-muted">{{ formatInsightDay(row.insight) }}</span>
                <span
                  v-if="formatDaysAgo(row.insight.daysSinceCommit)"
                  class="shrink-0 px-1.5 py-0.5 rounded bg-raised border border-line text-[10px] text-ink-muted font-mono"
                >
                  {{ formatDaysAgo(row.insight.daysSinceCommit) }}
                </span>
              </div>
              <div v-else-if="row.branch.commitHash" class="flex items-center gap-1 text-[11px] text-ink-muted mt-0.5 font-mono">
                <GitCommitHorizontal class="w-3 h-3 shrink-0" />
                <span>{{ row.branch.commitHash.substring(0, 8) }}</span>
              </div>
            </div>
          </div>

          <!-- Actions -->
          <div class="flex items-center gap-1.5 shrink-0">
            <span
              v-if="row.branch.ahead"
              class="px-1.5 py-0.5 rounded border border-accent/30 bg-accent/10 text-[10px] text-accent font-bold font-mono"
              :title="t('branches.local.ahead.hint', { count: row.branch.ahead })"
            >↑{{ row.branch.ahead }}</span>
            <span
              v-if="row.branch.behind"
              class="px-1.5 py-0.5 rounded border border-cyan/30 bg-cyan/10 text-[10px] text-cyan font-bold font-mono"
              :title="t('branches.local.behind.hint', { count: row.branch.behind })"
            >↓{{ row.branch.behind }}</span>
            <span
              v-if="row.insight?.hasUpstream && !row.branch.ahead && !row.branch.behind"
              class="flex items-center gap-1 px-1.5 py-0.5 rounded border border-line bg-raised text-[10px] text-ink-muted font-medium"
              :title="t('branches.local.synced.hint')"
            >
              <CheckCircle2 class="w-3 h-3" /> {{ t("branches.local.synced") }}
            </span>

            <!-- Set upstream for untracked local branches -->
            <button :aria-label="t('branches.local.upstream.hint')"
              v-if="!row.branch.upstream"
              @click="emit('set-upstream', row.branch)"
              class="p-1.5 rounded-lg hover:bg-hover text-ink-muted hover:text-accent transition cursor-pointer"
              :title="t('branches.local.upstream.hint')"
            >
              <Link2 class="w-3.5 h-3.5" />
            </button>

            <template v-if="!row.branch.isCurrent">
              <button
                @click="emit('checkout', row.branch.name)"
                class="px-2.5 py-1 rounded-lg bg-raised hover:bg-hover text-ink text-xs font-medium transition cursor-pointer border border-line"
              >
                {{ t("branches.local.checkout") }}
              </button>

              <button
                @click="emit('merge', row.branch.name)"
                class="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-raised hover:bg-hover text-accent text-xs font-medium transition cursor-pointer border border-line"
                :title="t('branches.local.merge.hint')"
              >
                <GitMerge class="w-3 h-3" />
                <span>{{ t("branches.local.merge") }}</span>
              </button>

              <button
                @click="emit('rebase', row.branch.name)"
                class="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-raised hover:bg-hover text-ink-secondary hover:text-ink text-xs font-medium transition cursor-pointer border border-line"
                :title="t('branches.local.rebase.hint')"
              >
                <GitPullRequest class="w-3 h-3" />
                <span>{{ t("branches.local.rebase") }}</span>
              </button>

              <button :aria-label="t('branches.local.rename.hint')"
                @click="emit('rename', row.branch.name)"
                class="p-1.5 rounded-lg hover:bg-hover text-ink-muted hover:text-ink transition cursor-pointer"
                :title="t('branches.local.rename.hint')"
              >
                <Edit2 class="w-3.5 h-3.5" />
              </button>

              <button :aria-label="t('branches.local.open.web.hint')"
                @click="emit('open-web', row.branch.name)"
                class="p-1.5 rounded-lg hover:bg-hover text-ink-muted hover:text-ink transition cursor-pointer"
                :title="t('branches.local.open.web.hint')"
              >
                <ExternalLink class="w-3.5 h-3.5 text-accent" />
              </button>

              <!-- Stale / merged branches get a category-aware cleanup action -->
              <button
                v-if="row.cleanupCandidate"
                @click="emit('cleanup', row.branch)"
                class="flex items-center gap-1 px-2 py-1 rounded-lg bg-danger/10 hover:bg-danger/20 text-danger border border-danger/30 text-[11px] font-semibold transition cursor-pointer"
                :title="t('branches.local.cleanup.row.hint', { category: row.meta?.label ?? '' })"
              >
                <Trash2 class="w-3 h-3" />
                <span>{{ t("branches.local.cleanup.row") }}</span>
              </button>
              <button :aria-label="t('branches.local.delete.hint')"
                v-else
                @click="emit('delete', row.branch.name, false)"
                class="p-1.5 rounded-lg hover:bg-danger/20 text-ink-muted hover:text-danger transition cursor-pointer"
                :title="t('branches.local.delete.hint')"
              >
                <Trash2 class="w-3.5 h-3.5" />
              </button>
            </template>
            <span v-else class="text-[11px] font-mono text-accent font-medium px-2 py-0.5">
              {{ t("branches.local.checked.out") }}
            </span>
          </div>
        </div>

        <button
          v-if="moreCount > 0"
          @click="emit('show-more')"
          class="w-full py-2 rounded-xl border border-dashed border-line text-xs text-ink-secondary hover:text-ink hover:border-line-strong transition cursor-pointer flex items-center justify-center gap-1.5"
        >
          <ArrowDownCircle class="w-3.5 h-3.5" />
          <span>{{ t("branches.local.show.more", { count: moreCount }) }}</span>
        </button>

    <UiContextMenu
      :open="branchMenuOpen"
      :x="branchMenuPos.x"
      :y="branchMenuPos.y"
      :items="branchMenuList"
      @select="handleBranchMenuSelect"
      @close="closeBranchMenu"
    />
</template>
