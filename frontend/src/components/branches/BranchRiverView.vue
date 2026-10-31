<script setup lang="ts">
import { ref, computed, onMounted, watch } from "vue";
import {
  GitFork,
  RefreshCw,
  Archive,
  AlertTriangle,
  CheckCircle2,
  Clock,
  ArrowRight,
  GitBranch,
  ExternalLink,
  ShieldAlert,
  Loader2,
  Trash2,
  Search,
  GitCompare,
} from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import { t } from "../../i18n";
import type { BranchRiverData, BranchRiverNode, StaleBranchesReport, CommitItem } from "../../types";
import UiModal from "../ui/UiModal.vue";
import CompareCommitsModal from "../history/CompareCommitsModal.vue";

const props = defineProps<{
  currentRepoPath: string;
}>();

const emit = defineEmits<{
  (e: "switch-branch", branchName: string): void;
  (e: "compare-branch", branchName: string): void;
  (e: "notify", msg: string): void;
}>();

const riverData = ref<BranchRiverData | null>(null);
const isLoading = ref(false);

// Stale Branch Archiver State
const showArchiverModal = ref(false);
const staleReport = ref<StaleBranchesReport | null>(null);
const isLoadingStale = ref(false);
const isArchiving = ref(false);
const selectedStaleBranches = ref<string[]>([]);

// Incremental Base Compare State
const showCompareModal = ref(false);
const compareBaseCommit = ref<CommitItem | null>(null);
const compareTargetCommit = ref<CommitItem | null>(null);

function makePseudoCommit(refName: string, subject: string = refName): CommitItem {
  return {
    hash: refName,
    shortHash: refName.slice(0, 10),
    parents: [],
    authorName: "",
    authorEmail: "",
    date: "",
    relativeDate: "",
    subject,
    body: "",
    refs: [refName],
    isHead: false,
    lane: 0,
    lines: [],
    signatureStatus: "N",
    signer: "",
  };
}

function handleOpenCompareWithBase(branchName: string, subject?: string) {
  if (!riverData.value) return;
  const base = riverData.value.baseBranch || "main";
  compareBaseCommit.value = makePseudoCommit(base, `Base: ${base}`);
  compareTargetCommit.value = makePseudoCommit(branchName, subject || branchName);
  showCompareModal.value = true;
}

function handleSwapCompare(base: CommitItem | null, target: CommitItem | null) {
  compareBaseCommit.value = base;
  compareTargetCommit.value = target;
}

// Branch search/filter
const searchQuery = ref("");
const selectedPrefix = ref<string>("all");
const PREFIX_OPTIONS = ["all", "feature", "fix", "release", "hotfix", "chore"] as const;

const filteredNodes = computed(() => {
  if (!riverData.value) return [];
  const q = searchQuery.value.trim().toLowerCase();
  let nodes = riverData.value.nodes;
  if (selectedPrefix.value !== "all") {
    nodes = nodes.filter((n) => n.name.startsWith(`${selectedPrefix.value}/`));
  }
  if (!q) return nodes;
  return nodes.filter(
    (n) =>
      n.name.toLowerCase().includes(q) ||
      (n.lastCommitAuthor && n.lastCommitAuthor.toLowerCase().includes(q))
  );
});

function notify(msg: string) {
  emit("notify", msg);
}

async function loadRiverData() {
  if (!props.currentRepoPath) return;
  isLoading.value = true;
  try {
    riverData.value = await gitService.getBranchRiverData(props.currentRepoPath);
  } catch (err: any) {
    notify(err?.message || String(err));
  } finally {
    isLoading.value = false;
  }
}

async function openArchiver() {
  if (!props.currentRepoPath) return;
  showArchiverModal.value = true;
  isLoadingStale.value = true;
  try {
    const rep = await gitService.detectStaleBranches(props.currentRepoPath, 30);
    staleReport.value = rep;
    selectedStaleBranches.value = rep.branches
      .filter((b) => b.isMerged || b.remoteGone)
      .map((b) => b.name);
  } catch (err: any) {
    notify(err?.message || String(err));
  } finally {
    isLoadingStale.value = false;
  }
}

function toggleSelectAllStale() {
  if (!staleReport.value) return;
  if (selectedStaleBranches.value.length === staleReport.value.branches.length) {
    selectedStaleBranches.value = [];
  } else {
    selectedStaleBranches.value = staleReport.value.branches.map((b) => b.name);
  }
}

async function handleArchiveAndDelete() {
  if (!props.currentRepoPath || selectedStaleBranches.value.length === 0 || isArchiving.value) return;
  isArchiving.value = true;
  try {
    const res = await gitService.archiveAndDeleteBranches(props.currentRepoPath, selectedStaleBranches.value);
    if (res.success) {
      notify(t("river.archiver.success", { count: res.deletedBranches.length, bundle: res.bundlePath.split("/").pop() || "bundle" }));
      showArchiverModal.value = false;
      await loadRiverData();
    } else {
      notify(res.message);
    }
  } catch (err: any) {
    notify(err?.message || String(err));
  } finally {
    isArchiving.value = false;
  }
}

function getRiskLevelClass(level: string) {
  switch (level) {
    case "critical":
      return "bg-rose/15 text-rose border-rose/30";
    case "high":
      return "bg-amber/15 text-amber border-amber/30";
    case "medium":
      return "bg-blue/15 text-blue border-blue/30";
    default:
      return "bg-emerald/15 text-emerald border-emerald/30";
  }
}

function getRiskLabel(level: string) {
  switch (level) {
    case "critical":
      return t("river.risk.critical");
    case "high":
      return t("river.risk.high");
    case "medium":
      return t("river.risk.medium");
    default:
      return t("river.risk.low");
  }
}

onMounted(() => {
  void loadRiverData();
});

watch(
  () => props.currentRepoPath,
  () => {
    void loadRiverData();
  }
);
</script>

<template>
  <div class="space-y-4" data-testid="branch-river-view">
    <!-- River Topology Header -->
    <div class="p-4 rounded-xl border border-line bg-surface flex flex-wrap items-center justify-between gap-3">
      <div class="space-y-1">
        <div class="flex items-center gap-2">
          <GitFork class="w-4 h-4 text-accent" />
          <h2 class="text-sm font-bold text-ink">{{ t("river.title") }}</h2>
          <span v-if="riverData" class="px-2 py-0.5 rounded text-[10px] font-mono bg-accent-bg text-accent border border-accent/30">
            {{ t("river.base", { base: riverData.baseBranch }) }}
          </span>
        </div>
        <p class="text-xs text-ink-muted">{{ t("river.subtitle") }}</p>
      </div>

      <div class="flex items-center gap-2">
        <button
          @click="openArchiver"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer shadow-2xs"
          data-testid="open-archiver-btn"
        >
          <Archive class="w-3.5 h-3.5 text-amber" aria-hidden="true" />
          <span>{{ t("river.archiver.btn") }}</span>
        </button>

        <button
          @click="loadRiverData"
          :disabled="isLoading"
          class="p-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-ink transition cursor-pointer"
          :title="t('river.refresh')"
        >
          <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isLoading }" aria-hidden="true" />
        </button>
      </div>
    </div>

    <!-- Quick Stat Pills -->
    <div v-if="riverData" class="flex flex-wrap items-center gap-2 text-xs">
      <span class="px-2.5 py-1 rounded-lg bg-surface border border-line font-medium text-ink">
        {{ t("river.total", { count: riverData.totalBranches }) }}
      </span>
      <span class="px-2.5 py-1 rounded-lg bg-emerald/10 border border-emerald/20 text-emerald font-medium">
        {{ t("river.active", { count: riverData.activeBranches }) }}
      </span>
      <span class="px-2.5 py-1 rounded-lg bg-purple-bg border border-purple/20 text-purple font-medium">
        {{ t("river.merged", { count: riverData.mergedBranches }) }}
      </span>
      <span class="px-2.5 py-1 rounded-lg bg-amber/10 border border-amber/20 text-amber font-medium">
        {{ t("river.stale", { count: riverData.staleBranches }) }}
      </span>
      <span class="ml-auto text-[11px] font-mono text-ink-muted">
        {{ riverData.scanDurationMs }}ms
      </span>
    </div>

    <!-- Branch Search Filter & Prefix Grouping -->
    <div v-if="riverData" class="flex flex-col gap-2">
      <div class="flex items-center gap-2">
        <div class="relative flex-1 max-w-sm">
          <Search class="absolute left-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-ink-muted pointer-events-none" />
          <input
            v-model="searchQuery"
            type="text"
            :placeholder="t('river.search.placeholder')"
            class="w-full pl-8 pr-3 py-1.5 rounded-lg bg-surface border border-line text-xs text-ink placeholder:text-ink-muted focus:outline-none focus:border-accent/60 transition"
            data-testid="river-search-input"
          />
        </div>
        <span v-if="searchQuery.trim()" class="text-[11px] text-ink-muted">
          {{ t("river.search.result", { count: filteredNodes.length, total: riverData.nodes.length }) }}
        </span>
      </div>

      <div class="flex items-center gap-1.5 flex-wrap" data-testid="river-prefix-filter">
        <button
          v-for="p in PREFIX_OPTIONS"
          :key="p"
          type="button"
          @click="selectedPrefix = p"
          class="px-2 py-0.5 rounded-full text-[11px] font-medium transition cursor-pointer"
          :class="selectedPrefix === p ? 'bg-accent text-accent-contrast shadow-xs' : 'bg-surface hover:bg-raised text-ink-muted border border-line'"
        >
          {{ p === 'all' ? t('common.all') : `${p}/*` }}
        </button>
      </div>
    </div>

    <!-- Branch River Flow Cards -->
    <div v-if="riverData" class="space-y-2.5">
      <div
        v-for="node in filteredNodes"
        :key="node.name"
        class="p-3.5 rounded-xl border transition flex flex-col md:flex-row md:items-center justify-between gap-3"
        :class="node.isHead ? 'border-accent bg-accent-bg' : 'border-line bg-surface hover:bg-raised'"
        :data-branch="node.name"
      >
        <!-- Left info: Branch name, ahead/behind, status -->
        <div class="space-y-1.5 min-w-0">
          <div class="flex items-center gap-2 flex-wrap">
            <span class="font-bold text-xs font-mono text-ink flex items-center gap-1.5">
              <GitBranch class="w-3.5 h-3.5 text-accent" />
              <span>{{ node.name }}</span>
            </span>

            <span v-if="node.isHead" class="px-1.5 py-0.5 rounded text-[10px] font-bold bg-accent text-accent-contrast">
              HEAD
            </span>

            <span
              v-if="node.status === 'merged'"
              class="px-1.5 py-0.5 rounded text-[10px] font-medium bg-purple-bg text-purple border border-purple/30"
            >
              {{ t("river.merged", { count: 1 }).replace(" 1", "") }}
            </span>
            <span
              v-else-if="node.status === 'stale'"
              class="px-1.5 py-0.5 rounded text-[10px] font-medium bg-amber/15 text-amber border border-amber/30"
              data-testid="stale-branch-badge"
            >
              {{ t("river.stale", { count: 1 }).replace(" 1", "") }}
            </span>

            <!-- Ahead / Behind Badges -->
            <div class="flex items-center gap-1 text-[11px] font-mono">
              <span v-if="node.aheadCount > 0" class="px-1.5 py-0.2 rounded bg-emerald/15 text-emerald font-semibold">
                +{{ node.aheadCount }}
              </span>
              <span v-if="node.behindCount > 0" class="px-1.5 py-0.2 rounded bg-amber/15 text-amber font-semibold">
                -{{ node.behindCount }}
              </span>
            </div>

            <!-- Divergence commit -->
            <span v-if="node.divergencePoint" class="text-[10px] font-mono text-ink-muted">
              {{ t("river.divergence", { hash: node.divergencePoint }) }}
            </span>
          </div>

          <!-- Commit Subject & Author -->
          <div class="text-xs text-ink-secondary truncate flex items-center gap-2">
            <span class="text-ink-muted font-medium">{{ node.lastCommitAuthor }}</span>
            <span>·</span>
            <span class="truncate">{{ node.lastCommitSubject || "—" }}</span>
          </div>
        </div>

        <!-- Right info: Conflict Risk & Actions -->
        <div class="flex items-center gap-3 shrink-0">
          <!-- Risk meter -->
          <div v-if="!node.isMerged && node.name !== riverData.baseBranch" class="flex flex-col items-end gap-1">
            <div class="flex items-center gap-1.5">
              <span class="text-[10px] font-medium text-ink-muted">
                {{ t("river.risk.score", { score: node.conflictRiskScore }) }}
              </span>
              <span class="px-1.5 py-0.5 rounded text-[10px] font-semibold border" :class="getRiskLevelClass(node.conflictRiskLevel)">
                {{ getRiskLabel(node.conflictRiskLevel) }}
              </span>
            </div>
            <!-- Progress bar -->
            <div class="w-24 h-1.5 bg-canvas rounded-full overflow-hidden border border-line">
              <div
                class="h-full rounded-full transition-all duration-300"
                :class="node.conflictRiskScore > 75 ? 'bg-rose' : node.conflictRiskScore > 40 ? 'bg-amber' : 'bg-emerald'"
                :style="{ width: `${node.conflictRiskScore}%` }"
              />
            </div>
          </div>

          <!-- Actions -->
          <div class="flex items-center gap-1.5">
            <button
              v-if="!node.isHead"
              @click="emit('switch-branch', node.name)"
              class="px-2.5 py-1 rounded-md bg-surface hover:bg-raised border border-line text-xs font-medium text-ink transition cursor-pointer"
            >
              {{ t("river.action.checkout") }}
            </button>
            <button
              v-if="node.name !== riverData.baseBranch"
              @click="handleOpenCompareWithBase(node.name, node.lastCommitSubject)"
              data-testid="river-compare-base-btn"
              class="px-2.5 py-1 rounded-md bg-surface hover:bg-raised border border-line text-xs font-medium text-ink transition cursor-pointer flex items-center gap-1"
              :title="t('branch.river.compare.base.hint')"
            >
              <GitCompare class="w-3.5 h-3.5 text-accent" aria-hidden="true" />
              <span>{{ t("branch.river.compare.base") }}</span>
            </button>
            <button
              v-if="node.name !== riverData.baseBranch"
              @click="emit('compare-branch', node.name)"
              class="px-2.5 py-1 rounded-md bg-surface hover:bg-raised border border-line text-xs font-medium text-ink-secondary hover:text-ink transition cursor-pointer"
            >
              {{ t("river.action.compare") }}
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Lifecycle Archiver Modal -->
    <UiModal
      v-model:open="showArchiverModal"
      :title="t('river.archiver.title')"
      width="max-w-2xl"
    >
      <div class="p-4 space-y-4">
        <p class="text-xs text-ink-muted leading-relaxed">
          {{ t("river.archiver.desc") }}
        </p>

        <div v-if="isLoadingStale" class="py-8 flex flex-col items-center justify-center gap-2 text-xs text-ink-muted">
          <Loader2 class="w-5 h-5 animate-spin text-accent" />
          <span>{{ t("river.archiver.scanning") }}</span>
        </div>

        <div v-else-if="staleReport && staleReport.branches.length > 0" class="space-y-3">
          <div class="flex items-center justify-between text-xs pb-2 border-b border-line">
            <button
              @click="toggleSelectAllStale"
              class="font-medium text-accent hover:underline cursor-pointer"
            >
              {{ selectedStaleBranches.length === staleReport.branches.length ? "Deselect All" : "Select All" }}
            </button>
            <span class="text-ink-muted">
              {{ t("river.archiver.selected", { count: selectedStaleBranches.length }) }}
            </span>
          </div>

          <div class="max-h-60 overflow-y-auto space-y-1.5">
            <label
              v-for="b in staleReport.branches"
              :key="b.name"
              class="flex items-center gap-2.5 p-2 rounded-lg border border-line bg-surface hover:bg-raised cursor-pointer text-xs"
            >
              <input
                type="checkbox"
                v-model="selectedStaleBranches"
                :value="b.name"
                class="rounded border-line text-accent focus:ring-accent"
              />
              <span class="font-mono font-bold text-ink">{{ b.name }}</span>
              <span
                v-if="b.isMerged"
                class="px-1.5 py-0.2 rounded text-[10px] bg-purple-bg text-purple font-medium"
              >
                Merged
              </span>
              <span
                v-if="b.remoteGone"
                class="px-1.5 py-0.2 rounded text-[10px] bg-rose/15 text-rose font-medium"
              >
                Gone
              </span>
              <span v-if="b.daysInactive > 0" class="text-[11px] text-ink-muted ml-auto font-mono">
                {{ t("river.archiver.days", { days: b.daysInactive }) }}
              </span>
            </label>
          </div>

          <div class="flex justify-end pt-2">
            <button
              @click="handleArchiveAndDelete"
              :disabled="selectedStaleBranches.length === 0 || isArchiving"
              class="flex items-center gap-1.5 px-4 py-2 rounded-lg bg-danger text-white text-xs font-semibold hover:bg-danger-hover transition cursor-pointer disabled:opacity-50"
            >
              <Loader2 v-if="isArchiving" class="w-3.5 h-3.5 animate-spin" aria-hidden="true" />
              <Trash2 v-else class="w-3.5 h-3.5" aria-hidden="true" />
              <span>{{ t("river.archiver.confirm") }}</span>
            </button>
          </div>
        </div>

        <div v-else class="py-8 text-center text-xs text-ink-muted">
          {{ t("river.archiver.empty") }}
        </div>
      </div>
    </UiModal>

    <!-- Base Branch Compare Modal -->
    <CompareCommitsModal
      :is-open="showCompareModal"
      :repo-path="currentRepoPath"
      :base-commit="compareBaseCommit"
      :target-commit="compareTargetCommit"
      @swap="handleSwapCompare"
      @close="showCompareModal = false"
    />
  </div>
</template>
