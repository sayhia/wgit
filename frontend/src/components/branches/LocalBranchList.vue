<script setup lang="ts">
import { ref } from "vue";
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
import UiContextMenu from "../ui/UiContextMenu.vue";
import type { BranchInfo, BranchInsight, BranchInsightSummary } from "../../types";

interface LocalBranchRow {
  branch: BranchInfo;
  insight: BranchInsight | null;
  meta: { label: string; cls: string; desc: string } | null;
  cleanupCandidate: boolean;
}

defineProps<{
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
    items.push({ id: "checkout", label: "切换到此分支", icon: GitBranch });
    items.push({ id: "merge", label: "合并到当前分支", icon: GitMerge });
    items.push({ id: "rebase", label: "变基到当前分支", icon: GitPullRequest });
  } else {
    items.push({ id: "checkout", label: "当前分支（不可切换）", icon: GitBranch, disabled: true });
  }
  items.push({ id: "rename", label: "重命名分支…", icon: Edit2, separatorBefore: true });
  if (!branch.isCurrent) {
    items.push({ id: "set-upstream", label: "设置上游跟踪分支…", icon: Link2 });
  }
  items.push({ id: "copy", label: "复制分支名", icon: Copy, hint: branch.name });
  items.push({ id: "open-web", label: "在网页端查看", icon: ExternalLink });
  items.push({
    id: "cleanup",
    label: "删除已合并分支…",
    icon: ClipboardCheck,
    danger: true,
    separatorBefore: true,
    disabled: branch.isCurrent,
  });
  items.push({ id: "delete", label: "强制删除分支…", icon: Trash2, danger: true, disabled: branch.isCurrent });
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

function formatInsightDay(ins?: BranchInsight): string {
  if (!ins?.lastDate) return "";
  const day = ins.lastDate.slice(0, 10);
  return /^\d{4}-\d{2}-\d{2}$/.test(day) ? day : ins.lastDate;
}

function formatDaysAgo(days?: number | null): string {
  if (days === undefined || days === null || Number.isNaN(days)) return "";
  if (days <= 0) return "今天";
  if (days === 1) return "1 天前";
  if (days < 30) return days + " 天前";
  if (days < 365) return Math.floor(days / 30) + " 个月前";
  return Math.floor(days / 365) + " 年前";
}
</script>

<template>
        <!-- Branch insights summary strip -->
        <div class="rounded-xl border border-line bg-surface/50 p-3 flex flex-col gap-2">
          <div class="flex items-center justify-between gap-3 flex-wrap">
            <div class="flex items-center gap-2 flex-wrap min-w-0">
              <span class="flex items-center gap-1.5 text-xs font-bold text-ink shrink-0">
                <Layers class="w-3.5 h-3.5 text-accent" />
                <span>分支洞察</span>
              </span>
              <RefreshCw v-if="insightsLoading && summary" class="w-3 h-3 text-ink-muted animate-spin shrink-0" title="正在刷新分支洞察" />

              <template v-if="summary">
                <span class="px-2 py-0.5 rounded-md bg-raised border border-line text-[11px] text-ink-secondary">
                  本地分支 <b class="text-ink">{{ summary.total }}</b>
                </span>
                <span v-if="summary.protected > 0" class="px-2 py-0.5 rounded-md bg-raised border border-line text-[11px] text-ink-secondary">
                  长期分支 <b class="text-ink">{{ summary.protected }}</b>
                </span>
                <span class="px-2 py-0.5 rounded-md bg-success/10 border border-success/25 text-[11px] text-success">
                  已合并 <b>{{ summary.merged }}</b>
                </span>
                <span class="px-2 py-0.5 rounded-md bg-warning/10 border border-warning/25 text-[11px] text-warning">
                  未推送 <b>{{ summary.unpushed }}</b>
                </span>
                <span class="px-2 py-0.5 rounded-md bg-cyan/10 border border-cyan/25 text-[11px] text-cyan">
                  长期停滞 <b>{{ summary.stale }}</b>
                </span>
                <span class="px-2 py-0.5 rounded-md bg-raised border border-line text-[11px] text-ink-secondary">
                  基准 <b class="font-mono text-ink">{{ summary.baseBranch || "自动检测" }}</b>
                </span>
              </template>

              <span v-else-if="loading || insightsLoading" class="flex items-center gap-1.5 text-[11px] text-ink-muted">
                <RefreshCw class="w-3 h-3 animate-spin" />
                正在分析分支健康度...
              </span>
              <span v-else class="text-[11px] text-ink-muted">{{ error || "暂无分支洞察数据" }}</span>
            </div>

            <div class="flex items-center gap-2 shrink-0">
              <button
                v-if="!summary && !insightsLoading"
                @click="emit('retry-insights')"
                class="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-raised hover:bg-hover text-ink-secondary hover:text-ink text-[11px] font-medium transition cursor-pointer border border-line"
                title="重新分析分支"
              >
                <RefreshCw class="w-3 h-3" />
                <span>重试</span>
              </button>
              <button
                @click="emit('cleanup-merged')"
                :disabled="cleanupDisabled"
                class="flex items-center gap-1.5 px-3 py-1 rounded-lg text-[11px] font-semibold transition border"
                :class="cleanupDisabled
                  ? 'bg-raised border-line text-ink-muted cursor-not-allowed'
                  : 'bg-danger/10 border-danger/30 text-danger hover:bg-danger/20 cursor-pointer'"
                :title="cleanupDisabled ? '没有可清理的已合并分支' : '安全删除已完全合并到基准分支的本地分支'"
              >
                <RefreshCw v-if="cleaningMerged" class="w-3 h-3 animate-spin" />
                <Trash2 v-else class="w-3 h-3" />
                <span>清理已合并分支</span>
                <span v-if="cleanableCount > 0" class="font-mono">({{ cleanableCount }})</span>
              </button>
            </div>
          </div>

          <div v-if="summary?.hint" class="flex items-start gap-1.5 text-[11px] text-ink-muted">
            <Sparkles class="w-3 h-3 text-accent shrink-0 mt-0.5" />
            <span>{{ summary.hint }}</span>
          </div>
          <div v-if="error && summary" class="flex items-start gap-1.5 text-[11px] text-ink-muted">
            <AlertTriangle class="w-3 h-3 shrink-0 mt-0.5" />
            <span>{{ error }}（分支列表仍可正常使用）</span>
          </div>
        </div>

        <div v-if="!loading && filteredCount === 0" class="py-12 text-center text-xs text-ink-muted border border-dashed border-line rounded-2xl">
          没有匹配的本地分支。
        </div>

        <div
          v-for="row in rows"
          :key="row.branch.name"
          :aria-label="`分支 ${row.branch.name}${row.branch.isCurrent ? '（当前）' : ''}`"
          class="flex items-center justify-between gap-2 p-3 rounded-xl bg-surface/60 border border-line hover:border-line-strong transition duration-150 group"
          :class="{ 'border-accent/40 bg-accent/5': row.branch.isCurrent }"
          :title="row.insight?.suggestion || undefined"
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
                  当前 HEAD
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
                <span class="truncate">{{ row.insight.lastSubject || "(无提交信息)" }}</span>
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
              :title="`领先上游 ${row.branch.ahead} 个提交`"
            >↑{{ row.branch.ahead }}</span>
            <span
              v-if="row.branch.behind"
              class="px-1.5 py-0.5 rounded border border-cyan/30 bg-cyan/10 text-[10px] text-cyan font-bold font-mono"
              :title="`落后上游 ${row.branch.behind} 个提交`"
            >↓{{ row.branch.behind }}</span>
            <span
              v-if="row.insight?.hasUpstream && !row.branch.ahead && !row.branch.behind"
              class="flex items-center gap-1 px-1.5 py-0.5 rounded border border-line bg-raised text-[10px] text-ink-muted font-medium"
              title="与上游分支同步"
            >
              <CheckCircle2 class="w-3 h-3" /> 已同步
            </span>

            <!-- Set upstream for untracked local branches -->
            <button aria-label="设置上游跟踪分支"
              v-if="!row.branch.upstream"
              @click="emit('set-upstream', row.branch)"
              class="p-1.5 rounded-lg hover:bg-hover text-ink-muted hover:text-accent transition cursor-pointer"
              title="设置上游跟踪分支"
            >
              <Link2 class="w-3.5 h-3.5" />
            </button>

            <template v-if="!row.branch.isCurrent">
              <button
                @click="emit('checkout', row.branch.name)"
                class="px-2.5 py-1 rounded-lg bg-raised hover:bg-hover text-ink text-xs font-medium transition cursor-pointer border border-line"
              >
                检出
              </button>

              <button
                @click="emit('merge', row.branch.name)"
                class="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-raised hover:bg-hover text-accent text-xs font-medium transition cursor-pointer border border-line"
                title="合并此分支到当前 HEAD"
              >
                <GitMerge class="w-3 h-3" />
                <span>合并</span>
              </button>

              <button
                @click="emit('rebase', row.branch.name)"
                class="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-raised hover:bg-hover text-ink-secondary hover:text-ink text-xs font-medium transition cursor-pointer border border-line"
                title="变基当前分支到此分支"
              >
                <GitPullRequest class="w-3 h-3" />
                <span>变基</span>
              </button>

              <button aria-label="重命名分支"
                @click="emit('rename', row.branch.name)"
                class="p-1.5 rounded-lg hover:bg-hover text-ink-muted hover:text-ink transition cursor-pointer"
                title="重命名分支"
              >
                <Edit2 class="w-3.5 h-3.5" />
              </button>

              <button aria-label="在 GitHub / GitLab 网页端查看分支"
                @click="emit('open-web', row.branch.name)"
                class="p-1.5 rounded-lg hover:bg-hover text-ink-muted hover:text-ink transition cursor-pointer"
                title="在 GitHub / GitLab 网页端查看分支"
              >
                <ExternalLink class="w-3.5 h-3.5 text-accent" />
              </button>

              <!-- Stale / merged branches get a category-aware cleanup action -->
              <button
                v-if="row.cleanupCandidate"
                @click="emit('cleanup', row.branch)"
                class="flex items-center gap-1 px-2 py-1 rounded-lg bg-danger/10 hover:bg-danger/20 text-danger border border-danger/30 text-[11px] font-semibold transition cursor-pointer"
                :title="`清理${row.meta?.label ?? ''}分支`"
              >
                <Trash2 class="w-3 h-3" />
                <span>清理</span>
              </button>
              <button aria-label="删除分支"
                v-else
                @click="emit('delete', row.branch.name, false)"
                class="p-1.5 rounded-lg hover:bg-danger/20 text-ink-muted hover:text-danger transition cursor-pointer"
                title="删除分支"
              >
                <Trash2 class="w-3.5 h-3.5" />
              </button>
            </template>
            <span v-else class="text-[11px] font-mono text-accent font-medium px-2 py-0.5">
              已检出
            </span>
          </div>
        </div>

        <button
          v-if="moreCount > 0"
          @click="emit('show-more')"
          class="w-full py-2 rounded-xl border border-dashed border-line text-xs text-ink-secondary hover:text-ink hover:border-line-strong transition cursor-pointer flex items-center justify-center gap-1.5"
        >
          <ArrowDownCircle class="w-3.5 h-3.5" />
          <span>显示更多 (+{{ moreCount }})</span>
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
