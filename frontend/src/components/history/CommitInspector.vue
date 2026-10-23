<script setup lang="ts">
import { computed, ref, watch } from "vue";
import {
  Check,
  Copy,
  GitPullRequest,
  GitMerge,
  GitCompare,
  RotateCcw,
  GitBranch,
  Tag,
  AlertTriangle,
  ExternalLink,
  StickyNote,
  Send,
  FileText,
  RefreshCw,
  ChevronDown,
  ShieldCheck,
  Wand2,
} from "lucide-vue-next";
import type { CommitItem, CommitDetail } from "../../types";

type CommitDetailWithPatch = CommitDetail & { diffTruncated?: boolean };

const props = defineProps<{
  selectedCommit: CommitItem | null;
  copiedHash: string | null;
  commitDetail: CommitDetailWithPatch | null;
  isLoadingDetail: boolean;
  currentCommitNote: string;
  isPushingNotes: boolean;
}>();

const emit = defineEmits<{
  (e: "copy-hash", hash: string): void;
  (e: "rebase-onto"): void;
  (e: "rebase-dialog", hash: string): void;
  (e: "cherry-pick"): void;
  (e: "create-fixup"): void;
  (e: "revert"): void;
  (e: "revert-no-commit"): void;
  (e: "checkout-detached"): void;
  (e: "new-branch"): void;
  (e: "new-tag"): void;
  (e: "reset"): void;
  (e: "open-web", hash: string): void;
  (e: "signature-details", hash: string): void;
  (e: "edit-note"): void;
  (e: "delete-note"): void;
  (e: "push-notes"): void;
}>();

const PATCH_RENDER_LIMIT = 4000;
const showPatch = ref(false);
const patchWrap = ref(false);
const patchShowAll = ref(false);

watch(
  () => props.selectedCommit?.hash,
  () => {
    showPatch.value = false;
    patchWrap.value = false;
    patchShowAll.value = false;
  }
);

type PatchLineKind = "add" | "del" | "hunk" | "meta" | "context";

interface PatchLine {
  text: string;
  kind: PatchLineKind;
}

const PATCH_META_PREFIXES = [
  "diff --git",
  "index ",
  "new file mode",
  "deleted file mode",
  "old mode",
  "new mode",
  "similarity index",
  "dissimilarity index",
  "rename from",
  "rename to",
  "copy from",
  "copy to",
  "Binary files ",
  "\\ No newline",
];

function classifyPatchLine(text: string): PatchLineKind {
  if (text.startsWith("@@")) return "hunk";
  if (text.startsWith("+++") || text.startsWith("---")) return "meta";
  if (PATCH_META_PREFIXES.some((prefix) => text.startsWith(prefix))) return "meta";
  if (text.startsWith("+")) return "add";
  if (text.startsWith("-")) return "del";
  return "context";
}

const patchLines = computed<PatchLine[]>(() => {
  const diff = props.commitDetail?.diff ?? "";
  if (!diff) return [];
  const raw = diff.split("\n");
  if (raw.length > 0 && raw[raw.length - 1] === "") raw.pop();
  return raw.map((text) => ({ text, kind: classifyPatchLine(text) }));
});

const patchLineCount = computed(() => patchLines.value.length);
const patchOverflow = computed(
  () => !patchShowAll.value && patchLineCount.value > PATCH_RENDER_LIMIT
);
const visiblePatchLines = computed(() =>
  patchOverflow.value ? patchLines.value.slice(0, PATCH_RENDER_LIMIT) : patchLines.value
);
const patchTruncatedByServer = computed(() => Boolean(props.commitDetail?.diffTruncated));
const patchIsBinary = computed(() => {
  const diff = props.commitDetail?.diff ?? "";
  return /^Binary files /m.test(diff) || diff.includes("GIT binary patch");
});

function patchLineClass(kind: PatchLineKind): string {
  switch (kind) {
    case "add":
      return "bg-success/10 text-success";
    case "del":
      return "bg-danger/10 text-danger";
    case "hunk":
      return "bg-accent/10 text-accent border-y border-accent/20";
    case "meta":
      return "text-ink-muted/70";
    default:
      return "text-ink-secondary";
  }
}

</script>

<template>
    <!-- Right: Commit Inspector Drawer -->
    <aside class="w-80 xl:w-96 bg-surface/50 backdrop-blur-md flex flex-col shrink-0 overflow-y-auto select-none border-l border-line/40">
      <div v-if="selectedCommit" class="p-4 flex flex-col gap-4">
        <!-- Hash & Copy & Quick Actions -->
        <div class="flex items-center justify-between pb-3 border-b border-line">
          <div class="flex items-center gap-2">
            <span class="font-mono text-xs font-semibold text-accent">{{ selectedCommit.shortHash }}</span>
            <button aria-label="复制完整 Commit Hash"
              @click="emit('copy-hash', selectedCommit.hash)"
              class="p-1 rounded hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer"
              title="复制完整 Commit Hash"
            >
              <Check v-if="copiedHash === selectedCommit.hash" class="w-3.5 h-3.5 text-success" />
              <Copy v-else class="w-3.5 h-3.5" />
            </button>
          </div>
          <span class="text-[10px] text-ink-muted">{{ selectedCommit.relativeDate }}</span>
        </div>

        <!-- Subject & Body -->
        <div class="flex flex-col gap-1.5">
          <h2 class="text-sm font-bold text-ink leading-snug">{{ selectedCommit.subject }}</h2>
          <p v-if="selectedCommit.body" class="text-xs text-ink-secondary whitespace-pre-wrap font-sans leading-relaxed bg-canvas/40 p-2.5 rounded-lg border border-line/60">
            {{ selectedCommit.body }}
          </p>
        </div>

        <!-- Quick Operation Buttons (Cherry-pick, Revert, Tag, Branch, Reset, Rebase) -->
        <div class="flex flex-wrap gap-1.5 p-2 rounded-xl bg-surface border border-line">
          <button
            @click="emit('rebase-onto')"
            class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-warning/15 hover:bg-warning/25 text-xs font-semibold text-warning transition cursor-pointer border border-warning/30"
            title="将当前分支变基到此提交 (Rebase current branch onto this commit)"
          >
            <GitPullRequest class="w-3.5 h-3.5" />
            <span>变基到此</span>
          </button>

          <button
            @click="emit('rebase-dialog', selectedCommit.hash)"
            class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-cyan/15 hover:bg-cyan/25 text-xs font-semibold text-cyan transition cursor-pointer border border-cyan/30"
            title="以此提交为起点发起交互式变基 (git rebase -i)"
          >
            <GitMerge class="w-3.5 h-3.5" />
            <span>交互式变基</span>
          </button>

          <button
            @click="emit('cherry-pick')"
            class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-canvas hover:bg-raised text-xs font-medium text-ink transition cursor-pointer border border-line/60"
            title="应用此提交到当前检出分支"
          >
            <GitCompare class="w-3.5 h-3.5 text-accent" />
            <span>Cherry-Pick</span>
          </button>

          <button
            @click="emit('create-fixup')"
            class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-canvas hover:bg-raised text-xs font-medium text-ink transition cursor-pointer border border-line/60"
            title="把当前暂存区改动整理成 fixup! 提交，变基时勾选 autosquash 会自动折叠进此提交"
          >
            <Wand2 class="w-3.5 h-3.5 text-purple" />
            <span>整理为 Fixup</span>
          </button>

          <button
            @click="emit('revert')"
            class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-canvas hover:bg-raised text-xs font-medium text-ink transition cursor-pointer border border-line/60"
            title="生成反向提交回滚此更改"
          >
            <RotateCcw class="w-3.5 h-3.5 text-warning" />
            <span>Revert</span>
          </button>

          <button
            @click="emit('revert-no-commit')"
            class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-canvas hover:bg-raised text-xs font-medium text-ink transition cursor-pointer border border-line/60"
            title="回滚此提交到工作区，但不自动创建提交"
          >
            <RotateCcw class="w-3.5 h-3.5 text-warning" />
            <span>Revert 不提交</span>
          </button>

          <button
            @click="emit('checkout-detached')"
            class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-canvas hover:bg-raised text-xs font-medium text-ink transition cursor-pointer border border-line/60"
            title="分离 HEAD 检出此提交（不创建分支）"
          >
            <GitBranch class="w-3.5 h-3.5 text-cyan" />
            <span>分离检出</span>
          </button>

          <button
            @click="emit('new-branch')"
            class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-canvas hover:bg-raised text-xs font-medium text-ink transition cursor-pointer border border-line/60"
            title="以此提交为起点创建分支"
          >
            <GitBranch class="w-3.5 h-3.5 text-cyan" />
            <span>分支</span>
          </button>

          <button
            @click="emit('new-tag')"
            class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-canvas hover:bg-raised text-xs font-medium text-ink transition cursor-pointer border border-line/60"
            title="为此提交创建标签"
          >
            <Tag class="w-3.5 h-3.5 text-purple" />
            <span>标签</span>
          </button>

          <button
            @click="emit('reset')"
            class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-canvas hover:bg-danger/20 text-xs font-medium text-danger transition cursor-pointer border border-danger/30"
            title="重置当前分支到此提交"
          >
            <AlertTriangle class="w-3.5 h-3.5" />
            <span>重置</span>
          </button>

          <button
            @click="emit('signature-details', selectedCommit.hash)"
            class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-canvas hover:bg-raised text-xs font-medium text-ink transition cursor-pointer border border-line/60"
            title="查看 GPG / SSH 签名校验详情"
          >
            <ShieldCheck class="w-3.5 h-3.5 text-success" />
            <span>签名</span>
          </button>

          <button
            @click="emit('open-web', selectedCommit.hash)"
            class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-canvas hover:bg-raised text-xs font-medium text-ink transition cursor-pointer border border-line/60"
            title="在 GitHub / GitLab 网页端查看此提交"
          >
            <ExternalLink class="w-3.5 h-3.5 text-accent" />
            <span>网页查看</span>
          </button>
        </div>

        <!-- Author Meta -->
        <div class="grid grid-cols-2 gap-2 text-xs bg-canvas/30 p-2.5 rounded-lg border border-line/60">
          <div class="flex flex-col gap-0.5">
            <span class="text-[10px] text-ink-muted uppercase tracking-wider">作者</span>
            <span class="font-medium text-ink truncate">{{ selectedCommit.authorName }}</span>
          </div>
          <div class="flex flex-col gap-0.5">
            <span class="text-[10px] text-ink-muted uppercase tracking-wider">邮箱</span>
            <span class="text-ink-secondary font-mono text-[10px] truncate">{{ selectedCommit.authorEmail }}</span>
          </div>
        </div>

        <!-- Git Notes Card -->
        <div class="p-3 rounded-xl border border-warning/30 bg-warning/5 flex flex-col gap-2">
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-1.5 text-xs font-bold text-ink">
              <StickyNote class="w-3.5 h-3.5 text-warning" />
              <span>提交批注 (Git Notes)</span>
            </div>
            <div class="flex items-center gap-1.5">
              <button
                v-if="currentCommitNote"
                @click="emit('edit-note')"
                class="px-2 py-0.5 rounded hover:bg-warning/20 text-ink text-[11px] font-medium transition cursor-pointer"
              >
                编辑
              </button>
              <button
                v-if="currentCommitNote"
                @click="emit('delete-note')"
                class="px-2 py-0.5 rounded hover:bg-danger/20 text-danger text-[11px] font-medium transition cursor-pointer"
              >
                删除
              </button>
              <button
                v-if="currentCommitNote"
                @click="emit('push-notes')"
                :disabled="isPushingNotes"
                class="flex items-center gap-1 px-2 py-0.5 rounded bg-surface hover:bg-raised text-[11px] text-ink font-mono border border-line transition cursor-pointer"
                title="将批注推送到远端仓库 (git push &lt;remote&gt; refs/notes/commits)"
              >
                <Send class="w-2.5 h-2.5 text-accent" />
                <span>{{ isPushingNotes ? '推送中...' : '推送远端' }}</span>
              </button>
              <button
                v-else
                @click="emit('edit-note')"
                class="flex items-center gap-1 px-2 py-0.5 rounded bg-warning/20 hover:bg-warning/30 text-warning text-xs font-semibold transition cursor-pointer"
              >
                + 添加批注
              </button>
            </div>
          </div>

          <div v-if="currentCommitNote" class="text-xs text-ink whitespace-pre-wrap font-sans bg-canvas/60 p-2.5 rounded-lg border border-line/40 leading-relaxed select-text">
            {{ currentCommitNote }}
          </div>
          <div v-else class="text-[11px] text-ink-muted italic">
            暂无批注。Git Notes 可在不篡改提交 Hash 的前提下为提交附加审核、测试或上线备注。
          </div>
        </div>

        <!-- Changed Files in Commit -->
        <div class="flex flex-col gap-2">
          <div class="flex items-center justify-between">
            <span class="text-[11px] font-semibold text-ink-muted uppercase tracking-wider">本次变更文件</span>
            <span v-if="commitDetail" class="text-[10px] text-ink-muted font-mono">
              +{{ commitDetail.additions }} / -{{ commitDetail.deletions }}
            </span>
          </div>

          <div v-if="isLoadingDetail" class="py-4 text-center text-xs text-ink-muted">
            <RefreshCw class="w-4 h-4 animate-spin mx-auto mb-1" /> 加载文件清单...
          </div>

          <div v-else-if="commitDetail" class="space-y-1 max-h-72 overflow-y-auto">
            <div
              v-for="f in commitDetail.files"
              :key="f.path"
              class="flex items-center justify-between p-2 rounded-lg bg-surface/70 border border-line/60 text-xs font-mono"
            >
              <div class="flex items-center gap-1.5 truncate">
                <FileText class="w-3.5 h-3.5 text-ink-muted shrink-0" />
                <span class="truncate text-ink-secondary">{{ f.path }}</span>
              </div>
              <div class="flex items-center gap-1 shrink-0 text-[10px]">
                <span v-if="f.additions" class="text-success font-bold">+{{ f.additions }}</span>
                <span v-if="f.deletions" class="text-danger font-bold">-{{ f.deletions }}</span>
              </div>
            </div>
          </div>
        </div>

        <!-- Commit Patch Preview (collapsible, collapsed by default) -->
        <div v-if="commitDetail" class="flex flex-col gap-2">
          <button
            @click="showPatch = !showPatch"
            class="flex items-center justify-between w-full px-2.5 py-1.5 rounded-lg bg-surface border border-line hover:bg-raised/60 transition cursor-pointer"
            :title="showPatch ? '收起统一补丁' : '展开统一补丁'"
          >
            <span class="flex items-center gap-1.5 text-[11px] font-semibold text-ink-secondary uppercase tracking-wider">
              <ChevronDown
                class="w-3.5 h-3.5 text-ink-muted transition-transform"
                :class="showPatch ? 'rotate-0' : '-rotate-90'"
              />
              变更补丁 (Patch)
            </span>
            <span class="flex items-center gap-2 text-[10px] font-mono">
              <span class="text-success font-bold">+{{ commitDetail.additions }}</span>
              <span class="text-danger font-bold">-{{ commitDetail.deletions }}</span>
              <span v-if="patchLineCount > 0" class="text-ink-muted">{{ patchLineCount }} 行</span>
            </span>
          </button>

          <template v-if="showPatch">
            <div class="flex items-center justify-between gap-2">
              <span
                v-if="patchTruncatedByServer"
                class="flex items-center gap-1 text-[10px] text-warning leading-tight"
                title="后端为大型仓库限制了补丁负载大小"
              >
                <AlertTriangle class="w-3 h-3 shrink-0" />
                补丁过大已截断，仅显示前 {{ patchLineCount }} 行
              </span>
              <span v-else class="text-[10px] text-ink-muted font-mono truncate">统一 diff</span>
              <button
                @click="patchWrap = !patchWrap"
                class="px-2 py-0.5 rounded-md bg-canvas hover:bg-raised border border-line/60 text-[10px] text-ink-secondary hover:text-ink transition cursor-pointer shrink-0"
                :title="patchWrap ? '关闭自动换行' : '开启自动换行'"
              >
                {{ patchWrap ? "不换行" : "自动换行" }}
              </button>
            </div>

            <div
              v-if="!commitDetail.diff"
              class="py-6 text-center text-[11px] text-ink-muted rounded-lg border border-line/60 bg-canvas/40"
            >
              该提交没有可显示的文本补丁（二进制文件 / 空变更 / 合并提交）
            </div>

            <div
              v-else-if="patchIsBinary"
              class="py-6 text-center text-[11px] text-ink-muted rounded-lg border border-line/60 bg-canvas/40"
            >
              二进制文件无法以文本方式预览（可查看上方变更文件列表）
            </div>

            <div
              v-else
              class="max-h-[420px] overflow-auto rounded-lg border border-line/60 bg-canvas/60 font-mono text-[10px] leading-relaxed"
            >
              <div
                v-for="(line, lineIndex) in visiblePatchLines"
                :key="lineIndex"
                class="flex items-start px-2 py-px"
                :class="patchLineClass(line.kind)"
              >
                <span class="w-8 shrink-0 select-none text-right pr-2 text-ink-muted/50">
                  {{ lineIndex + 1 }}
                </span>
                <span
                  class="flex-1 min-w-0"
                  :class="patchWrap ? 'whitespace-pre-wrap break-all' : 'whitespace-pre'"
                >{{ line.text || " " }}</span>
              </div>

              <div v-if="patchOverflow" class="p-2 text-center border-t border-line/40 bg-surface/40">
                <button
                  @click="patchShowAll = true"
                  class="px-2.5 py-1 rounded-md bg-raised hover:bg-hover-strong border border-line text-[10px] text-ink-secondary hover:text-ink transition cursor-pointer"
                >
                  补丁较大：已显示前 {{ PATCH_RENDER_LIMIT }} 行，点击显示全部（{{ patchLineCount }} 行）
                </button>
              </div>
              <div
                v-else-if="patchShowAll && patchLineCount > PATCH_RENDER_LIMIT"
                class="p-1.5 text-center text-[10px] text-ink-muted"
              >
                已显示全部 {{ patchLineCount }} 行
              </div>
            </div>
          </template>
        </div>
      </div>

      <div v-else class="flex-1 flex items-center justify-center text-xs text-ink-muted">
        请在左侧选择一条提交以查看详情
      </div>
    </aside>
</template>
