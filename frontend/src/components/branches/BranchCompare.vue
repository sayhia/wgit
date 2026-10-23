<script setup lang="ts">
import { computed, ref } from "vue";
import { GitCompare, RefreshCw, ExternalLink, ArrowLeftRight, Eye, Layers, Play } from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import type { BranchInfo, BranchCompareResult, DiffComparisonResult, RangeDiffResult } from "../../types";

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
    rdError.value = "四个引用都要填写完整。";
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

const RD_KIND_META: Record<string, { label: string; cls: string }> = {
  "=": { label: "等价", cls: "bg-surface border border-line text-ink-muted" },
  "!": { label: "已改写", cls: "bg-warning/15 text-warning" },
  ">": { label: "新增", cls: "bg-success/15 text-success" },
  "<": { label: "已移除", cls: "bg-danger/15 text-danger" },
};
</script>

<template>
        <div class="space-y-4">
          <!-- Selection & Overview Header -->
          <div class="p-4 rounded-xl border border-line bg-surface/50 shadow-xs flex flex-col gap-4">
            <div class="flex items-center justify-between pb-3 border-b border-line">
              <div class="flex flex-wrap items-center gap-2">
                <GitCompare class="w-4 h-4 text-accent" />
                <span class="text-sm font-bold text-ink">版本对比 (Refs Compare)</span>
                <span class="text-xs text-ink-muted">支持分支 / 标签 / Commit Hash，分析超前 (Ahead) 与落后 (Behind) 差异</span>
              </div>
              <button
                @click="emit('run')"
                :disabled="loading"
                class="flex items-center gap-1.5 px-3 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer"
              >
                <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': loading }" />
                <span>重新计算</span>
              </button>
            </div>

            <!-- Ref Selectors (branch / tag / commit hash) -->
            <datalist id="wgit-compare-ref-options">
              <option v-for="ref in refOptions" :key="ref" :value="ref" />
            </datalist>
            <div class="flex flex-wrap items-end gap-3">
              <div class="flex-1 min-w-[180px] flex flex-col gap-1.5">
                <label class="text-xs text-ink-secondary font-medium">基准版本 (Base Ref):</label>
                <input
                  v-model="baseModel"
                  list="wgit-compare-ref-options"
                  type="text"
                  spellcheck="false"
                  placeholder="分支 / 标签 / Commit Hash"
                  class="px-3 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
                  @keydown.enter="emit('run')"
                />
              </div>

              <button aria-label="交换基准与比对版本"
                @click="swapRefs"
                class="mb-0.5 p-1.5 rounded-lg border border-line bg-surface hover:bg-raised text-ink-muted hover:text-accent transition cursor-pointer shrink-0"
                title="交换基准与比对版本"
              >
                <ArrowLeftRight class="w-3.5 h-3.5" />
              </button>

              <div class="flex-1 min-w-[180px] flex flex-col gap-1.5">
                <label class="text-xs text-ink-secondary font-medium">比对版本 (Target Ref):</label>
                <input
                  v-model="targetModel"
                  list="wgit-compare-ref-options"
                  type="text"
                  spellcheck="false"
                  placeholder="分支 / 标签 / Commit Hash"
                  class="px-3 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
                  @keydown.enter="emit('run')"
                />
              </div>
            </div>

            <!-- Comparison mode toggle -->
            <div class="flex flex-wrap items-center gap-2 text-[11px]">
              <span class="text-ink-muted shrink-0">对比方式:</span>
              <div class="flex items-center bg-canvas p-0.5 rounded-lg border border-line shrink-0">
                <button
                  @click="emit('update:merge-base-mode', false)"
                  class="px-2 py-0.5 rounded text-[10px] font-medium transition cursor-pointer whitespace-nowrap"
                  :class="!mergeBaseMode ? 'bg-raised text-accent font-semibold shadow-xs' : 'text-ink-muted hover:text-ink'"
                  title="两点对比 A..B：直接比较两个版本的树（含两侧各自独有的改动）"
                >
                  两点 A..B
                </button>
                <button
                  @click="emit('update:merge-base-mode', true)"
                  class="px-2 py-0.5 rounded text-[10px] font-medium transition cursor-pointer whitespace-nowrap"
                  :class="mergeBaseMode ? 'bg-raised text-accent font-semibold shadow-xs' : 'text-ink-muted hover:text-ink'"
                  title="三点对比 A...B：以 merge-base 为基准，只比较目标版本分叉后引入的改动"
                >
                  三点 A...B (merge-base)
                </button>
              </div>
              <span class="text-[10px] text-ink-muted/80 truncate">
                {{ mergeBaseMode ? "仅比较分叉后目标版本引入的改动，与托管平台 PR/MR 视角一致" : "直接比较两个版本的完整树差异" }}
              </span>
            </div>

            <!-- Stats Bar -->
            <div v-if="result" class="flex flex-wrap items-center justify-between gap-2 pt-3 border-t border-line/60">
              <div class="flex flex-wrap items-center gap-2 shrink-0">
                <div class="flex items-center gap-1.5 px-3 py-1 rounded-lg bg-success/15 border border-success/30 text-success text-xs font-bold whitespace-nowrap shrink-0">
                  <span>领先 {{ result.aheadCount }} 个提交</span>
                </div>
                <div class="flex items-center gap-1.5 px-3 py-1 rounded-lg bg-warning/15 border border-warning/30 text-warning text-xs font-bold whitespace-nowrap shrink-0">
                  <span>落后 {{ result.behindCount }} 个提交</span>
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
                  <span>{{ diffLoading ? "加载差异中..." : "查看差异详情" }}</span>
                </button>
                <button
                  @click="emit('open-web')"
                  class="flex items-center gap-1.5 shrink-0 whitespace-nowrap px-3 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer"
                  title="在 GitHub / GitLab 网页端发起 Pull Request / Merge Request"
                >
                  <ExternalLink class="w-3.5 h-3.5 text-accent" />
                  <span>发起 Web PR / MR</span>
                </button>
                <button
                  v-if="result.behindCount > 0"
                  @click="emit('rebase', target)"
                  class="px-3 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-warning transition cursor-pointer shrink-0 whitespace-nowrap"
                >
                  变基拉平 (Rebase onto {{ target }})
                </button>
                <button
                  v-if="result.aheadCount > 0"
                  @click="emit('merge', target)"
                  class="px-3 py-1 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-white text-xs font-semibold transition cursor-pointer shrink-0 whitespace-nowrap"
                >
                  合入改动 (Merge)
                </button>
              </div>
            </div>
          </div>

          <!-- Two Column Lists for Ahead & Behind Commits -->
          <div v-if="result" class="grid grid-cols-2 gap-4">
            <!-- Ahead Commits Column -->
            <div class="space-y-2">
              <div class="flex items-center justify-between pb-1 border-b border-line text-xs font-bold text-success">
                <span>{{ target }} 独有的提交 ({{ result.aheadCount }})</span>
              </div>
              <div class="space-y-1.5">
                <div
                  v-for="c in result.aheadCommits || []"
                  :key="c.hash"
                  class="p-2.5 rounded-lg border border-line bg-surface/30 hover:bg-surface/60 transition flex flex-col gap-1 text-xs"
                >
                  <div class="flex items-center justify-between">
                    <span class="font-mono text-accent font-bold">{{ c.shortHash }}</span>
                    <span class="text-[10px] text-ink-muted">{{ c.relativeDate || c.date.split('T')[0] }}</span>
                  </div>
                  <div class="font-medium text-ink truncate">{{ c.subject }}</div>
                  <div class="text-[10px] text-ink-muted">{{ c.authorName }}</div>
                </div>
                <div v-if="(result.aheadCommits || []).length === 0" class="py-6 text-center text-xs text-ink-muted">
                  无领先提交
                </div>
              </div>
            </div>

            <!-- Behind Commits Column -->
            <div class="space-y-2">
              <div class="flex items-center justify-between pb-1 border-b border-line text-xs font-bold text-warning">
                <span>{{ base }} 独有的提交 ({{ result.behindCount }})</span>
              </div>
              <div class="space-y-1.5">
                <div
                  v-for="c in result.behindCommits || []"
                  :key="c.hash"
                  class="p-2.5 rounded-lg border border-line bg-surface/30 hover:bg-surface/60 transition flex flex-col gap-1 text-xs"
                >
                  <div class="flex items-center justify-between">
                    <span class="font-mono text-warning font-bold">{{ c.shortHash }}</span>
                    <span class="text-[10px] text-ink-muted">{{ c.relativeDate || c.date.split('T')[0] }}</span>
                  </div>
                  <div class="font-medium text-ink truncate">{{ c.subject }}</div>
                  <div class="text-[10px] text-ink-muted">{{ c.authorName }}</div>
                </div>
                <div v-if="(result.behindCommits || []).length === 0" class="py-6 text-center text-xs text-ink-muted">
                  无落后提交
                </div>
              </div>
            </div>
          </div>

          <div v-if="diff" class="p-4 rounded-xl border border-line bg-surface/40 space-y-3">
            <div class="flex flex-wrap items-center justify-between gap-2">
              <div class="text-xs font-bold text-ink">
                差异详情
                <span class="font-mono text-ink-muted font-normal">{{ diff.refA }}{{ mergeBaseMode ? '...' : '..' }}{{ diff.refB }}</span>
              </div>
              <div class="text-[11px] font-mono">
                <span class="text-success">+{{ diff.additions }}</span>
                <span class="text-danger ml-1">-{{ diff.deletions }}</span>
                <span class="text-ink-muted ml-2">{{ diff.files.length }} 个文件</span>
              </div>
            </div>
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-1 max-h-40 overflow-y-auto">
              <div
                v-for="file in diff.files"
                :key="file.path"
                @click="emit('show-file-diff', file.path)"
                class="group/file px-2 py-1 rounded bg-canvas/40 border border-line flex items-center gap-2 text-[11px] font-mono cursor-pointer hover:bg-raised hover:border-accent/40 transition"
                title="查看该文件在两个版本间的差异"
              >
                <span class="text-ink truncate flex-1" :title="file.path">{{ file.path }}</span>
                <span class="text-success shrink-0">+{{ file.additions }}</span>
                <span class="text-danger shrink-0">-{{ file.deletions }}</span>
                <button aria-label="在工作区打开该文件"
                  @click.stop="emit('open-file', file.path)"
                  class="shrink-0 p-0.5 rounded text-ink-muted/0 group-hover/file:text-ink-muted hover:text-accent hover:bg-accent/10 transition"
                  title="在工作区打开该文件"
                >
                  <Eye class="w-3 h-3" />
                </button>
              </div>
              <div v-if="diff.files.length === 0" class="col-span-2 py-3 text-center text-[11px] text-ink-muted">
                两个引用之间没有文件差异
              </div>
            </div>
            <div v-if="diff.diff" class="space-y-1">
              <div class="text-[10px] text-ink-muted font-semibold">补丁预览</div>
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
                <span>range-diff 补丁对照</span>
                <span class="text-[10px] font-normal text-ink-muted">force-push 后判断哪些提交被改写、哪些是新增</span>
              </span>
              <span class="text-[10px] text-ink-muted shrink-0">{{ showRangeDiff ? "收起" : "展开" }}</span>
            </button>

            <div v-if="showRangeDiff" class="px-4 pb-4 space-y-3 border-t border-line/60 pt-3">
              <div class="grid grid-cols-2 gap-3">
                <div class="space-y-2">
                  <div class="text-[10px] font-semibold text-ink-secondary">旧范围（force-push 前）</div>
                  <div class="flex items-center gap-1.5">
                    <input
                      v-model="rdOldBase"
                      list="wgit-compare-ref-options"
                      spellcheck="false"
                      placeholder="旧 base"
                      class="flex-1 min-w-0 px-2 py-1 rounded-lg bg-canvas border border-line text-[11px] font-mono text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
                    />
                    <span class="text-ink-muted text-[11px]">..</span>
                    <input
                      v-model="rdOldHead"
                      list="wgit-compare-ref-options"
                      spellcheck="false"
                      placeholder="旧 head"
                      class="flex-1 min-w-0 px-2 py-1 rounded-lg bg-canvas border border-line text-[11px] font-mono text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
                    />
                  </div>
                </div>
                <div class="space-y-2">
                  <div class="text-[10px] font-semibold text-ink-secondary">新范围</div>
                  <div class="flex items-center gap-1.5">
                    <input
                      v-model="rdNewBase"
                      list="wgit-compare-ref-options"
                      spellcheck="false"
                      placeholder="新 base"
                      class="flex-1 min-w-0 px-2 py-1 rounded-lg bg-canvas border border-line text-[11px] font-mono text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
                    />
                    <span class="text-ink-muted text-[11px]">..</span>
                    <input
                      v-model="rdNewHead"
                      list="wgit-compare-ref-options"
                      spellcheck="false"
                      placeholder="新 head"
                      class="flex-1 min-w-0 px-2 py-1 rounded-lg bg-canvas border border-line text-[11px] font-mono text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
                    />
                  </div>
                </div>
              </div>

              <div class="flex items-center gap-2">
                <button
                  @click="runRangeDiff"
                  :disabled="rdLoading"
                  class="flex items-center gap-1.5 px-3 py-1 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-white text-xs font-semibold transition cursor-pointer disabled:opacity-40"
                >
                  <Play class="w-3 h-3" />
                  <span>{{ rdLoading ? "对照中..." : "运行对照" }}</span>
                </button>
                <span v-if="rdResult" class="text-[10px] text-ink-muted">
                  等价 {{ rdResult.equal }} · 改写 {{ rdResult.changed }} · 新增 {{ rdResult.added }} · 移除 {{ rdResult.removed }}
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
                    >{{ RD_KIND_META[entry.kind]?.label || entry.kind }}</span>
                    <span class="font-mono text-ink-muted shrink-0 w-16 text-right">
                      {{ entry.oldHash || "-------" }}
                    </span>
                    <span class="text-ink-muted/50 shrink-0">→</span>
                    <span class="font-mono text-ink shrink-0 w-16">
                      {{ entry.newHash || "-------" }}
                    </span>
                    <span class="text-ink truncate flex-1" :title="entry.subject">{{ entry.subject || "(无提交信息)" }}</span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
</template>
