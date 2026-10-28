<script setup lang="ts">
import { computed, ref } from "vue";
import { GitCompare, RefreshCw, ExternalLink, ArrowLeftRight, Eye, Layers, Play } from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import type { BranchInfo, BranchCompareResult, DiffComparisonResult, RangeDiffResult, MergeSimulationResult } from "../../types";
import { t } from "../../i18n";
import { relativeTime } from "../../utils/relativeTime";

const props = defineProps<{
  branches: BranchInfo[];
  /** Branch + tag names offered as datalist suggestions; arbitrary refs/hashes are allowed. */
  refOptions: string[];
  base: string;
  target: string;
  /** true renders three-dot (merge-base) comparison semantics. */
  mergeBaseMode: boolean;
  result: BranchCompareResult | null;
  loading: boolean;
  diff: DiffComparisonResult | null;
  diffLoading: boolean;
  currentRepoPath: string;
}>();

const emit = defineEmits<{
  (e: "update:base", value: string): void;
  (e: "update:target", value: string): void;
  (e: "run"): void;
  (e: "show-diff"): void;
  (e: "open-web"): void;
  (e: "update:merge-base-mode", value: boolean): void;
  (e: "open-file", path: string): void;
  (e: "show-file-diff", path: string): void;
  (e: "rebase", target: string): void;
  (e: "merge", target: string): void;
}>();

const baseModel = computed({
  get: () => props.base,
  set: (value: string) => emit("update:base", value),
});
const targetModel = computed({
  get: () => props.target,
  set: (value: string) => emit("update:target", value),
});

function swapRefs() {
  const base = props.base;
  emit("update:base", props.target);
  emit("update:target", base);
  emit("run");
}

// ---- range-diff: patch-level comparison of two commit ranges -------------
// The canonical use case is a force-pushed PR: hashes change, but
// `git range-diff` matches commits by patch content, so it can tell
// "rewritten" from "really new".
const showRangeDiff = ref(false);
const rdOldBase = ref("");
const rdOldHead = ref("");
const rdNewBase = ref("");
const rdNewHead = ref("");
const rdResult = ref<RangeDiffResult | null>(null);
const rdLoading = ref(false);
const rdError = ref("");

// Merge Dry-Run Simulator
const simulation = ref<MergeSimulationResult | null>(null);
const isSimulating = ref(false);

async function runMergeSimulation() {
  if (!props.currentRepoPath || !props.base || !props.target || isSimulating.value) return;
  isSimulating.value = true;
  try {
    simulation.value = await gitService.simulateMerge(props.currentRepoPath, props.base, props.target);
  } catch {
    simulation.value = null;
  } finally {
    isSimulating.value = false;
  }
}

function toggleRangeDiff() {
  showRangeDiff.value = !showRangeDiff.value;
  if (showRangeDiff.value && !rdOldBase.value) {
    rdOldBase.value = props.base;
    rdOldHead.value = props.target;
    rdNewBase.value = props.base;
    rdNewHead.value = "HEAD";
  }
}

async function runRangeDiff() {
  if (!props.currentRepoPath || rdLoading.value) return;
  const missing = [rdOldBase, rdOldHead, rdNewBase, rdNewHead].some((r) => !r.value.trim());
  if (missing) {
    rdError.value = t("branches.compare.range.error.incomplete");
    return;
  }
  rdLoading.value = true;
  rdError.value = "";
  try {
    rdResult.value = await gitService.getRangeDiff(
      props.currentRepoPath,
      rdOldBase.value.trim(),
      rdOldHead.value.trim(),
      rdNewBase.value.trim(),
      rdNewHead.value.trim()
    );
  } catch (err: any) {
    rdError.value = err?.message || String(err);
    rdResult.value = null;
  } finally {
    rdLoading.value = false;
  }
}

/** Module-scope maps hold i18n keys — resolve through `t()` in the template. */
const RD_KIND_META: Record<string, { labelKey: string; cls: string }> = {
  "=": { labelKey: "branches.compare.range.kind.equal", cls: "bg-surface border border-line text-ink-muted" },
  "!": { labelKey: "branches.compare.range.kind.changed", cls: "bg-warning/15 text-warning" },
  ">": { labelKey: "branches.compare.range.kind.added", cls: "bg-success/15 text-success" },
  "<": { labelKey: "branches.compare.range.kind.removed", cls: "bg-danger/15 text-danger" },
};
</script>

<template>
        <div class="space-y-4">
          <!-- Selection & Overview Header -->
          <div class="p-4 rounded-xl border border-line bg-surface/50 shadow-xs flex flex-col gap-4">
            <div class="flex items-center justify-between pb-3 border-b border-line">
              <div class="flex flex-wrap items-center gap-2">
                <GitCompare class="w-4 h-4 text-accent" />
                <span class="text-sm font-bold text-ink">{{ t("branches.compare.title") }}</span>
                <span class="text-xs text-ink-muted">{{ t("branches.compare.subtitle") }}</span>
              </div>
              <button
                @click="emit('run')"
                :disabled="loading"
                class="flex items-center gap-1.5 px-3 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer"
              >
                <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': loading }" />
                <span>{{ t("branches.compare.recompute") }}</span>
              </button>
            </div>

            <!-- Ref Selectors (branch / tag / commit hash) -->
            <datalist id="wgit-compare-ref-options">
              <option v-for="ref in refOptions" :key="ref" :value="ref" />
            </datalist>
            <div class="flex flex-wrap items-end gap-3">
              <div class="flex-1 min-w-[180px] flex flex-col gap-1.5">
                <label class="text-xs text-ink-secondary font-medium">{{ t("branches.compare.base.label") }}</label>
                <input
                  v-model="baseModel"
                  list="wgit-compare-ref-options"
                  type="text"
                  spellcheck="false"
                  :placeholder="t('branches.compare.ref.placeholder')"
                  class="px-3 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
                  @keydown.enter="emit('run')"
                />
              </div>

              <button :aria-label="t('branches.compare.swap')"
                @click="swapRefs"
                class="mb-0.5 p-1.5 rounded-lg border border-line bg-surface hover:bg-raised text-ink-muted hover:text-accent transition cursor-pointer shrink-0"
                :title="t('branches.compare.swap')"
              >
                <ArrowLeftRight class="w-3.5 h-3.5" />
              </button>

              <div class="flex-1 min-w-[180px] flex flex-col gap-1.5">
                <label class="text-xs text-ink-secondary font-medium">{{ t("branches.compare.target.label") }}</label>
                <input
                  v-model="targetModel"
                  list="wgit-compare-ref-options"
                  type="text"
                  spellcheck="false"
                  :placeholder="t('branches.compare.ref.placeholder')"
                  class="px-3 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
                  @keydown.enter="emit('run')"
                />
              </div>
            </div>

            <!-- Comparison mode toggle -->
            <div class="flex flex-wrap items-center gap-2 text-[11px]">
              <span class="text-ink-muted shrink-0">{{ t("branches.compare.mode") }}</span>
              <div class="flex items-center bg-canvas p-0.5 rounded-lg border border-line shrink-0">
                <button
                  @click="emit('update:merge-base-mode', false)"
                  class="px-2 py-0.5 rounded text-[10px] font-medium transition cursor-pointer whitespace-nowrap"
                  :class="!mergeBaseMode ? 'bg-raised text-accent font-semibold shadow-xs' : 'text-ink-muted hover:text-ink'"
                  :title="t('branches.compare.mode.two.hint')"
                >
                  {{ t("branches.compare.mode.two") }}
                </button>
                <button
                  @click="emit('update:merge-base-mode', true)"
                  class="px-2 py-0.5 rounded text-[10px] font-medium transition cursor-pointer whitespace-nowrap"
                  :class="mergeBaseMode ? 'bg-raised text-accent font-semibold shadow-xs' : 'text-ink-muted hover:text-ink'"
                  :title="t('branches.compare.mode.three.hint')"
                >
                  {{ t("branches.compare.mode.three") }}
                </button>
              </div>
              <span class="text-[10px] text-ink-muted truncate">
                {{ mergeBaseMode ? t("branches.compare.mode.three.desc") : t("branches.compare.mode.two.desc") }}
              </span>
            </div>

            <!-- Stats Bar -->
            <div v-if="result" class="flex flex-wrap items-center justify-between gap-2 pt-3 border-t border-line/60">
              <div class="flex flex-wrap items-center gap-2 shrink-0">
                <div class="flex items-center gap-1.5 px-3 py-1 rounded-lg bg-success/15 border border-success/30 text-success text-xs font-bold whitespace-nowrap shrink-0">
                  <span>{{ t("branches.compare.ahead", { count: result.aheadCount }) }}</span>
                </div>
                <div class="flex items-center gap-1.5 px-3 py-1 rounded-lg bg-warning/15 border border-warning/30 text-warning text-xs font-bold whitespace-nowrap shrink-0">
                  <span>{{ t("branches.compare.behind", { count: result.behindCount }) }}</span>
                </div>
              </div>

              <!-- Quick merge/rebase trigger -->
              <div class="flex flex-wrap items-center gap-2">
                <button
                  @click="emit('show-diff')"
                  :disabled="diffLoading"
                  class="flex items-center gap-1.5 shrink-0 whitespace-nowrap px-3 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-50"
                >
                  <GitCompare class="w-3.5 h-3.5 text-purple" />
                  <span>{{ diffLoading ? t("branches.compare.diff.loading") : t("branches.compare.diff.show") }}</span>
                </button>
                <button
                  @click="runMergeSimulation"
                  :disabled="isSimulating"
                  class="flex items-center gap-1.5 shrink-0 whitespace-nowrap px-3 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-accent transition cursor-pointer disabled:opacity-50"
                  data-testid="simulate-merge-btn"
                >
                  <RefreshCw v-if="isSimulating" class="w-3.5 h-3.5 animate-spin" />
                  <Layers v-else class="w-3.5 h-3.5" />
                  <span>{{ isSimulating ? t("branches.compare.simulate.loading") : t("branches.compare.simulate.btn") }}</span>
                </button>
                <button
                  @click="emit('open-web')"
                  class="flex items-center gap-1.5 shrink-0 whitespace-nowrap px-3 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer"
                  :title="t('branches.compare.open.web.hint')"
                >
                  <ExternalLink class="w-3.5 h-3.5 text-accent" />
                  <span>{{ t("branches.compare.open.web") }}</span>
                </button>
                <button
                  v-if="result.behindCount > 0"
                  @click="emit('rebase', target)"
                  class="px-3 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-warning transition cursor-pointer shrink-0 whitespace-nowrap"
                >
                  {{ t("branches.compare.rebase.onto", { target }) }}
                </button>
                <button
                  v-if="result.aheadCount > 0"
                  @click="emit('merge', target)"
                  class="px-3 py-1 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-xs font-semibold transition cursor-pointer shrink-0 whitespace-nowrap"
                >
                  {{ t("branches.compare.merge") }}
                </button>
              </div>
            </div>

            <!-- Merge Simulator Dry-Run Status Banner -->
            <div
              v-if="simulation"
              class="p-2.5 rounded-lg border flex flex-col gap-1.5 text-xs transition select-none"
              :class="simulation.canMergeCleanly ? 'bg-success/10 border-success/30 text-success' : 'bg-danger/10 border-danger/30 text-danger'"
              data-testid="merge-simulation-panel"
            >
              <div class="flex items-center justify-between font-semibold">
                <div class="flex items-center gap-1.5">
                  <span v-if="simulation.canMergeCleanly" data-testid="simulation-clean-badge">{{ t("branches.compare.simulate.clean") }}</span>
                  <span v-else data-testid="simulation-conflicts-badge">{{ t("branches.compare.simulate.conflicts", { count: simulation.conflictedFiles.length }) }}</span>
                  <span class="text-[10px] px-1.5 py-0.2 rounded font-mono" :class="simulation.canMergeCleanly ? 'bg-success/20 text-success' : 'bg-danger/20 text-danger'">
                    {{ simulation.isFastForward ? t("branches.compare.simulate.ff") : t("branches.compare.simulate.three_way") }}
                  </span>
                </div>
                <span v-if="simulation.changedFiles.length > 0" class="text-[10px] text-ink-muted font-normal font-mono">
                  {{ t("branches.compare.simulate.changed.files", { count: simulation.changedFiles.length }) }}
                </span>
              </div>
              <div v-if="simulation.hasConflicts && simulation.conflictedFiles.length > 0" class="text-[11px] font-mono text-danger/80 space-y-0.5">
                <div v-for="cf in simulation.conflictedFiles" :key="cf" class="truncate">• {{ cf }}</div>
              </div>
            </div>
          </div>

          <!-- Two Column Lists for Ahead & Behind Commits -->
          <div v-if="result" class="grid grid-cols-2 gap-4">
            <!-- Ahead Commits Column -->
            <div class="space-y-2">
              <div class="flex items-center justify-between pb-1 border-b border-line text-xs font-bold text-success">
                <span>{{ t("branches.compare.ahead.title", { ref: target, count: result.aheadCount }) }}</span>
              </div>
              <div class="space-y-1.5">
                <div
                  v-for="c in result.aheadCommits || []"
                  :key="c.hash"
                  class="p-2.5 rounded-lg border border-line bg-surface/30 hover:bg-surface/60 transition flex flex-col gap-1 text-xs"
                >
                  <div class="flex items-center justify-between">
                    <span class="font-mono text-accent font-bold">{{ c.shortHash }}</span>
                    <span class="text-[10px] text-ink-muted">{{ relativeTime(c.date, c.relativeDate || c.date.split("T")[0]) }}</span>
                  </div>
                  <div class="font-medium text-ink truncate">{{ c.subject }}</div>
                  <div class="text-[10px] text-ink-muted">{{ c.authorName }}</div>
                </div>
                <div v-if="(result.aheadCommits || []).length === 0" class="py-6 text-center text-xs text-ink-muted">
                  {{ t("branches.compare.ahead.empty") }}
                </div>
              </div>
            </div>

            <!-- Behind Commits Column -->
            <div class="space-y-2">
              <div class="flex items-center justify-between pb-1 border-b border-line text-xs font-bold text-warning">
                <span>{{ t("branches.compare.behind.title", { ref: base, count: result.behindCount }) }}</span>
              </div>
              <div class="space-y-1.5">
                <div
                  v-for="c in result.behindCommits || []"
                  :key="c.hash"
                  class="p-2.5 rounded-lg border border-line bg-surface/30 hover:bg-surface/60 transition flex flex-col gap-1 text-xs"
                >
                  <div class="flex items-center justify-between">
                    <span class="font-mono text-warning font-bold">{{ c.shortHash }}</span>
                    <span class="text-[10px] text-ink-muted">{{ relativeTime(c.date, c.relativeDate || c.date.split("T")[0]) }}</span>
                  </div>
                  <div class="font-medium text-ink truncate">{{ c.subject }}</div>
                  <div class="text-[10px] text-ink-muted">{{ c.authorName }}</div>
                </div>
                <div v-if="(result.behindCommits || []).length === 0" class="py-6 text-center text-xs text-ink-muted">
                  {{ t("branches.compare.behind.empty") }}
                </div>
              </div>
            </div>
          </div>

          <div v-if="diff" class="p-4 rounded-xl border border-line bg-surface/40 space-y-3">
            <div class="flex flex-wrap items-center justify-between gap-2">
              <div class="text-xs font-bold text-ink">
                {{ t("branches.compare.diff.title") }}
                <span class="font-mono text-ink-muted font-normal">{{ diff.refA }}{{ mergeBaseMode ? '...' : '..' }}{{ diff.refB }}</span>
              </div>
              <div class="text-[11px] font-mono">
                <span class="text-success">+{{ diff.additions }}</span>
                <span class="text-danger ml-1">-{{ diff.deletions }}</span>
                <span class="text-ink-muted ml-2">{{ t("branches.compare.diff.files", { count: diff.files.length }) }}</span>
              </div>
            </div>
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-1 max-h-40 overflow-y-auto">
              <div
                v-for="file in diff.files"
                :key="file.path"
                @click="emit('show-file-diff', file.path)"
                class="group/file px-2 py-1 rounded bg-canvas/40 border border-line flex items-center gap-2 text-[11px] font-mono cursor-pointer hover:bg-raised hover:border-accent/40 transition"
                :title="t('branches.compare.diff.file.hint')"
              >
                <span class="text-ink truncate flex-1" :title="file.path">{{ file.path }}</span>
                <span class="text-success shrink-0">+{{ file.additions }}</span>
                <span class="text-danger shrink-0">-{{ file.deletions }}</span>
                <button :aria-label="t('branches.compare.open.file.hint')"
                  @click.stop="emit('open-file', file.path)"
                  class="shrink-0 p-0.5 rounded text-ink-muted/0 group-hover/file:text-ink-muted hover:text-accent hover:bg-accent/10 transition"
                  :title="t('branches.compare.open.file.hint')"
                >
                  <Eye class="w-3 h-3" />
                </button>
              </div>
              <div v-if="diff.files.length === 0" class="col-span-2 py-3 text-center text-[11px] text-ink-muted">
                {{ t("branches.compare.diff.empty") }}
              </div>
            </div>
            <div v-if="diff.diff" class="space-y-1">
              <div class="text-[10px] text-ink-muted font-semibold">{{ t("branches.compare.patch.preview") }}</div>
              <pre class="max-h-96 overflow-auto p-3 rounded-lg bg-canvas border border-line text-[11px] leading-relaxed font-mono whitespace-pre-wrap break-all">{{ diff.diff }}</pre>
            </div>
          </div>

          <!-- range-diff: force-push 前后，按补丁内容而非 hash 对照 -->
          <div class="rounded-xl border border-line bg-surface/40">
            <button
              @click="toggleRangeDiff"
              class="w-full flex items-center justify-between gap-2 px-4 py-2.5 text-left cursor-pointer"
            >
              <span class="flex items-center gap-2 text-xs font-bold text-ink">
                <Layers class="w-3.5 h-3.5 text-purple" />
                <span>{{ t("branches.compare.range.title") }}</span>
                <span class="text-[10px] font-normal text-ink-muted">{{ t("branches.compare.range.subtitle") }}</span>
              </span>
              <span class="text-[10px] text-ink-muted shrink-0">{{
                showRangeDiff ? t("branches.compare.collapse") : t("branches.compare.expand")
              }}</span>
            </button>

            <div v-if="showRangeDiff" class="px-4 pb-4 space-y-3 border-t border-line/60 pt-3">
              <div class="grid grid-cols-2 gap-3">
                <div class="space-y-2">
                  <div class="text-[10px] font-semibold text-ink-secondary">{{ t("branches.compare.range.old") }}</div>
                  <div class="flex items-center gap-1.5">
                    <input
                      v-model="rdOldBase"
                      list="wgit-compare-ref-options"
                      spellcheck="false"
                      :placeholder="t('branches.compare.range.old.base')"
                      class="flex-1 min-w-0 px-2 py-1 rounded-lg bg-canvas border border-line text-[11px] font-mono text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
                    />
                    <span class="text-ink-muted text-[11px]">..</span>
                    <input
                      v-model="rdOldHead"
                      list="wgit-compare-ref-options"
                      spellcheck="false"
                      :placeholder="t('branches.compare.range.old.head')"
                      class="flex-1 min-w-0 px-2 py-1 rounded-lg bg-canvas border border-line text-[11px] font-mono text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
                    />
                  </div>
                </div>
                <div class="space-y-2">
                  <div class="text-[10px] font-semibold text-ink-secondary">{{ t("branches.compare.range.new") }}</div>
                  <div class="flex items-center gap-1.5">
                    <input
                      v-model="rdNewBase"
                      list="wgit-compare-ref-options"
                      spellcheck="false"
                      :placeholder="t('branches.compare.range.new.base')"
                      class="flex-1 min-w-0 px-2 py-1 rounded-lg bg-canvas border border-line text-[11px] font-mono text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
                    />
                    <span class="text-ink-muted text-[11px]">..</span>
                    <input
                      v-model="rdNewHead"
                      list="wgit-compare-ref-options"
                      spellcheck="false"
                      :placeholder="t('branches.compare.range.new.head')"
                      class="flex-1 min-w-0 px-2 py-1 rounded-lg bg-canvas border border-line text-[11px] font-mono text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
                    />
                  </div>
                </div>
              </div>

              <div class="flex items-center gap-2">
                <button
                  @click="runRangeDiff"
                  :disabled="rdLoading"
                  class="flex items-center gap-1.5 px-3 py-1 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-xs font-semibold transition cursor-pointer disabled:opacity-40"
                >
                  <Play class="w-3 h-3" />
                  <span>{{ rdLoading ? t("branches.compare.range.running") : t("branches.compare.range.run") }}</span>
                </button>
                <span v-if="rdResult" class="text-[10px] text-ink-muted">
                  {{
                    t("branches.compare.range.summary", {
                      equal: rdResult.equal,
                      changed: rdResult.changed,
                      added: rdResult.added,
                      removed: rdResult.removed,
                    })
                  }}
                </span>
              </div>

              <p v-if="rdError" class="text-[11px] text-danger">{{ rdError }}</p>

              <div v-if="rdResult" class="space-y-1">
                <p class="text-[11px] text-ink-secondary">{{ rdResult.message }}</p>
                <div class="max-h-64 overflow-y-auto space-y-1">
                  <div
                    v-for="(entry, idx) in rdResult.entries"
                    :key="`${entry.oldHash}-${entry.newHash}-${idx}`"
                    class="flex items-center gap-2 px-2 py-1 rounded-lg bg-canvas/50 border border-line text-[11px]"
                  >
                    <span
                      class="shrink-0 px-1.5 py-0.5 rounded text-[9px] font-bold"
                      :class="RD_KIND_META[entry.kind]?.cls || 'bg-surface text-ink-muted'"
                    >{{ RD_KIND_META[entry.kind] ? t(RD_KIND_META[entry.kind]!.labelKey) : entry.kind }}</span>
                    <span class="font-mono text-ink-muted shrink-0 w-16 text-right">
                      {{ entry.oldHash || "-------" }}
                    </span>
                    <span class="text-ink-muted shrink-0">→</span>
                    <span class="font-mono text-ink shrink-0 w-16">
                      {{ entry.newHash || "-------" }}
                    </span>
                    <span class="text-ink truncate flex-1" :title="entry.subject">{{
                      entry.subject || t("branches.compare.range.no.subject")
                    }}</span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
</template>
