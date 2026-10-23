<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from "vue";
import { useModalA11y } from "../composables/useModalA11y";
import {
  FolderGit2,
  FolderOpen,
  Download,
  FolderPlus,
  GitBranch,
  ArrowRight,
  Check,
  X,
  Loader2,
  ChevronDown,
  HardDrive,
  Sparkles,
  AlertTriangle,
  Clock,
} from "lucide-vue-next";
import { gitService } from "../services/gitService";
import type { ProjectDetailItem } from "../types";

const props = defineProps<{
  mode?: "welcome" | "clone" | "init" | "scan";
  canClose?: boolean;
}>();

const emit = defineEmits<{
  (e: "opened", path: string): void;
  (e: "close"): void;
}>();

const { containerRef: dialogRef } = useModalA11y(() => true, () => { if (props.canClose) emit("close"); });

const activeMode = ref<"welcome" | "clone" | "init" | "scan">(props.mode || "welcome");
const recentRepos = ref<string[]>([]);
const isLoadingRecents = ref(false);
const errorMessage = ref("");

// ---- Clone form ----
const cloneURL = ref("");
const cloneTarget = ref("");
const cloneType = ref<"full" | "blobless" | "shallow">("blobless");
const cloneDepth = ref(1);
const cloneBranch = ref("");
const cloneSubmodules = ref(false);
const showAdvancedClone = ref(false);
const isCloning = ref(false);
const cloneProgressHint = ref("");

// ---- Init form ----
const initPath = ref("");
const isIniting = ref(false);

const canSubmitClone = computed(() => cloneURL.value.trim() !== "" && cloneTarget.value.trim() !== "");
const canSubmitInit = computed(() => initPath.value.trim() !== "");

const cloneTypeLabel = computed(() => {
  switch (cloneType.value) {
    case "blobless":
      return "Blobless 部分克隆（推荐大型项目）";
    case "shallow":
      return "浅克隆（仅最近历史）";
    default:
      return "完整克隆（拉取全部历史）";
  }
});

async function loadRecents() {
  isLoadingRecents.value = true;
  try {
    const list = await gitService.getRecentRepos();
    if (list) recentRepos.value = list;
  } catch (err) {
    console.debug("Failed to load recent repos:", err);
  } finally {
    isLoadingRecents.value = false;
  }
}

async function handleOpenDialog() {
  errorMessage.value = "";
  try {
    const info = await gitService.openRepoDialog();
    if (info?.path) {
      emit("opened", info.path);
    }
  } catch (err: any) {
    const msg = err?.message || String(err);
    if (msg.includes("not a valid git repository") || msg.includes("不是一个有效的 Git 仓库") || msg.includes("不是有效的 Git 仓库")) {
      const match = msg.match(/['"「]([^'"」]+)['"」]/);
      const folder = match ? match[1] : "";
      if (folder) {
        initPath.value = folder;
        activeMode.value = "init";
        errorMessage.value = `所选目录「${folder}」尚未初始化为 Git 仓库。已为您切换至初始化页面，点击下方“初始化”即可创建仓库并打开。`;
        return;
      }
    }
    errorMessage.value = `打开仓库失败：${msg}`;
  }
}

const scanRoot = ref("");
// "扫描本地工程" entry point (command palette / repo rail): jump straight to the
// scan form and focus its input instead of making the user find the card.
const scanInputRef = ref<HTMLInputElement | null>(null);
const isScanMode = computed(() => activeMode.value === "scan");

async function focusScanInput() {
  activeMode.value = "scan";
  await nextTick();
  const el = scanInputRef.value;
  if (el) {
    el.scrollIntoView({ block: "center", behavior: "smooth" });
    el.focus();
  }
}
const scannedProjects = ref<ProjectDetailItem[]>([]);
const isScanningProjects = ref(false);

async function handleScanProjects() {
  const root = scanRoot.value.trim();
  if (!root || isScanningProjects.value) return;
  errorMessage.value = "";
  isScanningProjects.value = true;
  try {
    const paths = await gitService.scanLocalProjects(root);
    scannedProjects.value = (await gitService.getProjectsDetails(paths || [], "")) || [];
    if (scannedProjects.value.length === 0) {
      errorMessage.value = `在 ${root} 下未发现 Git 工程（扫描深度约 3 层）`;
    }
  } catch (err: any) {
    errorMessage.value = `扫描本地工程失败：${err?.message || err}`;
  } finally {
    isScanningProjects.value = false;
  }
}

async function handleOpenRecent(path: string) {
  errorMessage.value = "";
  try {
    const info = await gitService.openRepo(path);
    if (info?.path) {
      emit("opened", info.path);
    }
  } catch (err: any) {
    errorMessage.value = `打开仓库失败：${err?.message || err}`;
  }
}

async function browseCloneTarget() {
  try {
    const folder = await gitService.pickFolderDialog();
    if (folder) {
      cloneTarget.value = folder;
    }
  } catch (err) {
    console.debug("Pick folder cancelled/failed:", err);
  }
}

async function handleClone() {
  if (!canSubmitClone.value || isCloning.value) return;
  errorMessage.value = "";
  isCloning.value = true;
  cloneProgressHint.value =
    cloneType.value === "blobless"
      ? "正在执行 Blobless 部分克隆，大仓库首次拉取仅下载目录树与最近需要的对象…"
      : cloneType.value === "shallow"
      ? `正在执行 Shallow 克隆（depth=${cloneDepth.value}）…`
      : "正在执行完整克隆，请耐心等待大型仓库下载完成…";
  try {
    const info = await gitService.cloneRepoAdvanced({
      url: cloneURL.value.trim(),
      targetPath: cloneTarget.value.trim(),
      cloneType: cloneType.value,
      depth: cloneDepth.value,
      branch: cloneBranch.value.trim(),
      recurseSubmodules: cloneSubmodules.value,
    });
    if (info?.path) {
      emit("opened", info.path);
    } else {
      errorMessage.value = "克隆完成但未能打开仓库，请检查目标目录。";
    }
  } catch (err: any) {
    errorMessage.value = `克隆失败：${err?.message || err}`;
  } finally {
    isCloning.value = false;
    cloneProgressHint.value = "";
  }
}

async function browseInitPath() {
  try {
    const folder = await gitService.pickFolderDialog();
    if (folder) {
      initPath.value = folder;
    }
  } catch (err) {
    console.debug("Pick folder cancelled/failed:", err);
  }
}

async function handleInit() {
  if (!canSubmitInit.value || isIniting.value) return;
  errorMessage.value = "";
  isIniting.value = true;
  try {
    const info = await gitService.initRepo(initPath.value.trim());
    if (info?.path) {
      emit("opened", info.path);
    }
  } catch (err: any) {
    errorMessage.value = `初始化仓库失败：${err?.message || err}`;
  } finally {
    isIniting.value = false;
  }
}

onMounted(() => {
  loadRecents();
  if (props.mode === "scan") void focusScanInput();
});

watch(
  () => props.mode,
  (mode) => {
    if (mode === "scan") void focusScanInput();
  }
);
</script>

<template>
  <div
    ref="dialogRef"
    role="dialog"
    aria-modal="true"
    aria-label="打开或克隆仓库"
    class="fixed inset-0 z-40 bg-canvas/95 backdrop-blur-2xl flex items-center justify-center p-6 select-none overflow-y-auto">
    <div class="w-full max-w-3xl rounded-3xl bg-surface border border-line shadow-2xl overflow-hidden my-auto">
      <!-- Hero header -->
      <div class="relative px-8 pt-4 pb-3.5 border-b border-line bg-gradient-to-br from-accent/10 via-surface to-surface">
        <button aria-label="关闭"
          v-if="props.canClose"
          @click="emit('close')"
          class="absolute right-4 top-4 p-1.5 rounded-lg text-ink-muted hover:text-ink hover:bg-raised transition cursor-pointer"
          title="关闭"
        >
          <X class="w-4 h-4" />
        </button>
        <div class="flex items-center gap-3">
          <div class="w-11 h-11 rounded-2xl bg-accent/15 border border-accent/30 flex items-center justify-center">
            <GitBranch class="w-6 h-6 text-accent" />
          </div>
          <div class="flex flex-col">
            <h1 class="text-lg font-bold text-ink tracking-tight">wgit · 大型项目 Git 工作台</h1>
            <p class="text-xs text-ink-muted">打开、克隆或初始化一个仓库，开始处理分支、历史与海量资产。</p>
          </div>
        </div>
      </div>

      <!-- Error banner -->
      <div
        v-if="errorMessage"
        class="mx-8 mt-5 px-3 py-2.5 rounded-xl bg-danger/10 border border-danger/30 text-danger text-xs flex items-start gap-2"
      >
        <AlertTriangle class="w-4 h-4 shrink-0 mt-0.5" />
        <span class="break-all">{{ errorMessage }}</span>
      </div>

      <div class="grid grid-cols-1 md:grid-cols-2 gap-4 p-6">
        <!-- Left: actions -->
        <div class="flex flex-col gap-3">
          <button
            @click="handleOpenDialog"
            class="group flex items-center gap-3 p-3.5 rounded-2xl bg-canvas border border-line hover:border-accent/50 hover:bg-raised/60 transition text-left cursor-pointer"
          >
            <div class="w-9 h-9 rounded-xl bg-accent/10 text-accent flex items-center justify-center shrink-0">
              <FolderOpen class="w-4.5 h-4.5" />
            </div>
            <div class="flex-1 min-w-0">
              <div class="text-sm font-semibold text-ink">打开本地仓库</div>
              <div class="text-[11px] text-ink-muted truncate">选择本机上已有的 Git 工作区</div>
            </div>
            <ArrowRight class="w-4 h-4 text-ink-muted group-hover:text-accent transition shrink-0" />
          </button>

          <button
            @click="activeMode = 'clone'"
            class="group flex items-center gap-3 p-3.5 rounded-2xl border text-left transition cursor-pointer"
            :class="activeMode === 'clone' ? 'bg-accent/10 border-accent/50' : 'bg-canvas border-line hover:border-accent/50 hover:bg-raised/60'"
          >
            <div class="w-9 h-9 rounded-xl bg-cyan/10 text-cyan flex items-center justify-center shrink-0">
              <Download class="w-4.5 h-4.5" />
            </div>
            <div class="flex-1 min-w-0">
              <div class="text-sm font-semibold text-ink">克隆远程仓库</div>
              <div class="text-[11px] text-ink-muted truncate">支持 Blobless / 浅克隆，专为超大仓库加速</div>
            </div>
            <ArrowRight class="w-4 h-4 text-ink-muted group-hover:text-cyan transition shrink-0" />
          </button>

          <button
            @click="activeMode = 'init'"
            class="group flex items-center gap-3 p-3.5 rounded-2xl border text-left transition cursor-pointer"
            :class="activeMode === 'init' ? 'bg-success/10 border-success/50' : 'bg-canvas border-line hover:border-success/50 hover:bg-raised/60'"
          >
            <div class="w-9 h-9 rounded-xl bg-success/10 text-success flex items-center justify-center shrink-0">
              <FolderPlus class="w-4.5 h-4.5" />
            </div>
            <div class="flex-1 min-w-0">
              <div class="text-sm font-semibold text-ink">初始化新仓库</div>
              <div class="text-[11px] text-ink-muted truncate">在空目录中执行 git init</div>
            </div>
            <ArrowRight class="w-4 h-4 text-ink-muted group-hover:text-success transition shrink-0" />
          </button>

          <!-- Recent repos -->
          <div class="mt-1 rounded-2xl bg-canvas border border-line overflow-hidden">
            <div class="px-3.5 py-2 border-b border-line/70 flex items-center gap-1.5">
              <Clock class="w-3.5 h-3.5 text-ink-muted" />
              <span class="text-[11px] font-semibold text-ink-muted uppercase tracking-wider">最近打开</span>
            </div>
            <div class="max-h-40 overflow-y-auto p-1.5 space-y-0.5">
              <button
                v-for="repo in recentRepos"
                :key="repo"
                @click="handleOpenRecent(repo)"
                class="w-full flex items-center gap-2 px-2.5 py-1.5 rounded-lg hover:bg-raised/80 transition text-left cursor-pointer"
              >
                <FolderGit2 class="w-3.5 h-3.5 text-ink-muted shrink-0" />
                <span class="text-xs text-ink truncate flex-1">{{ repo.split('/').pop() || repo }}</span>
                <span class="text-[10px] text-ink-muted font-mono truncate max-w-[140px]">{{ repo }}</span>
              </button>
              <div v-if="!isLoadingRecents && recentRepos.length === 0" class="py-3 text-center text-[11px] text-ink-muted">
                暂无最近项目
              </div>
            </div>
          </div>

          <!-- Local project scan -->
          <div
            class="mt-1 rounded-2xl bg-canvas border overflow-hidden transition"
            :class="isScanMode ? 'border-accent/60 ring-1 ring-accent/30' : 'border-line'"
          >
            <div class="px-3.5 py-2 border-b border-line/70 flex items-center gap-1.5">
              <FolderOpen class="w-3.5 h-3.5 text-ink-muted" />
              <span class="text-[11px] font-semibold text-ink-muted uppercase tracking-wider">扫描本地工程</span>
            </div>
            <div class="p-2 space-y-2">
              <div class="flex items-center gap-1.5">
                <input
                  ref="scanInputRef"
                  v-model="scanRoot"
                  type="text"
                  placeholder="输入扫描根目录，例如 /Users/me/projects"
                  class="flex-1 min-w-0 px-2.5 py-1.5 rounded-lg bg-surface border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
                />
                <button
                  @click="handleScanProjects"
                  :disabled="!scanRoot.trim() || isScanningProjects"
                  class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-accent transition cursor-pointer disabled:opacity-40 shrink-0"
                >
                  {{ isScanningProjects ? "扫描中..." : "扫描" }}
                </button>
              </div>
              <div v-if="scannedProjects.length > 0" class="max-h-56 overflow-y-auto space-y-1">
                <button
                  v-for="project in scannedProjects"
                  :key="project.path"
                  @click="handleOpenRecent(project.path)"
                  class="w-full flex items-center gap-2 px-2.5 py-1.5 rounded-lg hover:bg-raised/80 transition text-left cursor-pointer"
                  :title="project.path"
                >
                  <FolderGit2 class="w-3.5 h-3.5 text-ink-muted shrink-0" />
                  <span class="text-xs text-ink font-semibold truncate">{{ project.name }}</span>
                  <span class="px-1.5 py-0.2 rounded bg-accent/15 text-accent text-[9px] font-bold shrink-0">
                    {{ project.engineType }}
                  </span>
                  <span class="text-[10px] text-ink-muted font-mono truncate">{{ project.currentBranch }}</span>
                  <span v-if="project.isDirty" class="text-[10px] text-warning shrink-0">{{ project.dirtyCount }} changes</span>
                  <span class="ml-auto text-[10px] text-ink-muted font-mono truncate max-w-[140px]">{{ project.path }}</span>
                </button>
              </div>
              <div v-else-if="!isScanningProjects" class="py-2 text-center text-[11px] text-ink-muted">
                输入目录后扫描其中的 Git 工程（约 3 层深度）
              </div>
            </div>
          </div>
        </div>

        <!-- Right: selected action form -->
        <div class="rounded-2xl bg-canvas border border-line p-5 flex flex-col">
          <!-- Clone form -->
          <template v-if="activeMode === 'clone'">
            <div class="flex items-center gap-2 mb-4">
              <Download class="w-4 h-4 text-cyan" />
              <h2 class="text-sm font-bold text-ink">克隆远程仓库</h2>
            </div>

            <div class="flex flex-col gap-3">
              <label class="flex flex-col gap-1">
                <span class="text-[11px] font-medium text-ink-secondary">仓库 URL</span>
                <input
                  v-model="cloneURL"
                  type="text"
                  placeholder="git@github.com:org/big-monorepo.git"
                  class="w-full px-3 py-2 rounded-lg bg-surface border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent font-mono"
                />
              </label>

              <label class="flex flex-col gap-1">
                <span class="text-[11px] font-medium text-ink-secondary">目标目录</span>
                <div class="flex items-center gap-1.5">
                  <input
                    v-model="cloneTarget"
                    type="text"
                    placeholder="/Users/you/repos/big-monorepo"
                    class="flex-1 min-w-0 px-3 py-2 rounded-lg bg-surface border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent font-mono"
                  />
                  <button aria-label="选择目录"
                    @click="browseCloneTarget"
                    class="px-2.5 py-2 rounded-lg bg-surface hover:bg-raised border border-line text-xs text-ink transition cursor-pointer shrink-0"
                    title="选择目录"
                  >
                    <FolderOpen class="w-3.5 h-3.5" />
                  </button>
                </div>
              </label>

              <label class="flex flex-col gap-1">
                <span class="text-[11px] font-medium text-ink-secondary">克隆模式</span>
                <div class="relative">
                  <select
                    v-model="cloneType"
                    class="w-full appearance-none px-3 py-2 pr-8 rounded-lg bg-surface border border-line text-xs text-ink focus:outline-none focus:border-accent cursor-pointer"
                  >
                    <option value="blobless">Blobless 部分克隆（推荐大型项目）</option>
                    <option value="shallow">浅克隆（仅最近历史）</option>
                    <option value="full">完整克隆（全部历史）</option>
                  </select>
                  <ChevronDown class="w-3.5 h-3.5 text-ink-muted absolute right-2.5 top-2.5 pointer-events-none" />
                </div>
                <span class="text-[10px] text-ink-muted leading-relaxed">
                  Blobless 仅按需下载文件内容（`--filter=blob:none`），体积可降至 10%–30%；浅克隆只保留最近 N 条历史。
                </span>
              </label>

              <div v-if="cloneType === 'shallow'" class="flex flex-col gap-1">
                <span class="text-[11px] font-medium text-ink-secondary">浅克隆深度（提交数）</span>
                <input
                  v-model.number="cloneDepth"
                  type="number"
                  min="1"
                  class="w-full px-3 py-2 rounded-lg bg-surface border border-line text-xs text-ink focus:outline-none focus:border-accent font-mono"
                />
              </div>

              <button
                @click="showAdvancedClone = !showAdvancedClone"
                class="flex items-center gap-1 text-[11px] text-accent hover:text-accent-strong transition cursor-pointer self-start"
              >
                <ChevronDown class="w-3 h-3 transition-transform" :class="{ 'rotate-180': showAdvancedClone }" />
                <span>高级选项</span>
              </button>

              <div v-if="showAdvancedClone" class="flex flex-col gap-3 p-3 rounded-xl bg-surface/60 border border-line/70">
                <label class="flex flex-col gap-1">
                  <span class="text-[11px] font-medium text-ink-secondary">检出分支（可选）</span>
                  <input
                    v-model="cloneBranch"
                    type="text"
                    placeholder="main / release/v2"
                    class="w-full px-3 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent font-mono"
                  />
                </label>
                <label class="flex items-center gap-2 cursor-pointer">
                  <input v-model="cloneSubmodules" type="checkbox" class="accent-accent" />
                  <span class="text-[11px] text-ink-secondary">递归克隆子模块（--recurse-submodules）</span>
                </label>
              </div>

              <div
                v-if="cloneProgressHint"
                class="flex items-start gap-2 px-3 py-2 rounded-lg bg-cyan/10 border border-cyan/30 text-[11px] text-cyan"
              >
                <Loader2 class="w-3.5 h-3.5 animate-spin shrink-0 mt-0.5" />
                <span>{{ cloneProgressHint }}</span>
              </div>

              <button
                @click="handleClone"
                :disabled="!canSubmitClone || isCloning"
                class="flex items-center justify-center gap-2 px-4 py-2.5 rounded-xl bg-cyan-strong text-white text-xs font-semibold transition cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed"
              >
                <Loader2 v-if="isCloning" class="w-4 h-4 animate-spin" />
                <Download v-else class="w-4 h-4" />
                <span>{{ isCloning ? "正在克隆…" : "开始克隆" }}</span>
              </button>
            </div>
          </template>

          <!-- Init form -->
          <template v-else-if="activeMode === 'init'">
            <div class="flex items-center gap-2 mb-4">
              <FolderPlus class="w-4 h-4 text-success" />
              <h2 class="text-sm font-bold text-ink">初始化本地仓库</h2>
            </div>
            <div class="flex flex-col gap-3">
              <label class="flex flex-col gap-1">
                <span class="text-[11px] font-medium text-ink-secondary">工作区目录</span>
                <div class="flex items-center gap-1.5">
                  <input
                    v-model="initPath"
                    type="text"
                    placeholder="/Users/you/projects/new-project"
                    class="flex-1 min-w-0 px-3 py-2 rounded-lg bg-surface border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent font-mono"
                  />
                  <button aria-label="选择目录"
                    @click="browseInitPath"
                    class="px-2.5 py-2 rounded-lg bg-surface hover:bg-raised border border-line text-xs text-ink transition cursor-pointer shrink-0"
                    title="选择目录"
                  >
                    <FolderOpen class="w-3.5 h-3.5" />
                  </button>
                </div>
              </label>
              <p class="text-[11px] text-ink-muted leading-relaxed">
                在该目录执行 <code class="font-mono text-ink-secondary">git init</code>，随后可作为普通仓库使用分支、历史与大型项目工具。
              </p>
              <button
                @click="handleInit"
                :disabled="!canSubmitInit || isIniting"
                class="flex items-center justify-center gap-2 px-4 py-2.5 rounded-xl bg-success-strong text-white text-xs font-semibold transition cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed"
              >
                <Loader2 v-if="isIniting" class="w-4 h-4 animate-spin" />
                <Check v-else class="w-4 h-4" />
                <span>{{ isIniting ? "正在初始化…" : "初始化仓库" }}</span>
              </button>
            </div>
          </template>

          <!-- Welcome hints -->
          <template v-else>
            <div class="flex items-center gap-2 mb-4">
              <Sparkles class="w-4 h-4 text-accent" />
              <h2 class="text-sm font-bold text-ink">大型项目能力</h2>
            </div>
            <ul class="flex flex-col gap-2.5 text-[11px] text-ink-secondary">
              <li class="flex items-start gap-2">
                <HardDrive class="w-3.5 h-3.5 text-cyan shrink-0 mt-0.5" />
                <span><strong class="text-ink">Blobless / 浅克隆</strong>：克隆阶段即压缩 70%+ 下载体积。</span>
              </li>
              <li class="flex items-start gap-2">
                <GitBranch class="w-3.5 h-3.5 text-accent shrink-0 mt-0.5" />
                <span><strong class="text-ink">分支洞察与清理</strong>：几百个分支也能一键按活跃度排序、清理已合并分支。</span>
              </li>
              <li class="flex items-start gap-2">
                <Sparkles class="w-3.5 h-3.5 text-purple shrink-0 mt-0.5" />
                <span><strong class="text-ink">⌘P / ⌘⇧F</strong>：服务端快速打开文件与全仓代码搜索，无需遍历全部文件。</span>
              </li>
              <li class="flex items-start gap-2">
                <FolderGit2 class="w-3.5 h-3.5 text-success shrink-0 mt-0.5" />
                <span><strong class="text-ink">子模块 / 工作树 / 稀疏检出 / LFS</strong>：在效能工具箱中集中管理。</span>
              </li>
            </ul>
            <div class="mt-auto pt-4 text-[10px] text-ink-muted leading-relaxed">
              提示：已安装 Git 客户端与系统凭据即可使用；克隆大型仓库建议优先选择 Blobless 模式。
            </div>
          </template>
        </div>
      </div>

      <div v-if="activeMode !== 'welcome'" class="px-8 pb-6">
        <button
          @click="activeMode = 'welcome'"
          class="text-[11px] text-ink-muted hover:text-ink transition cursor-pointer"
        >
          ← 返回全部入口
        </button>
      </div>
    </div>
  </div>
</template>
