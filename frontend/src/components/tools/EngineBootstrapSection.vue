<script setup lang="ts">
import { confirmAction } from "../../composables/useConfirm";
import { ref, computed, watch } from "vue";
import { Sparkles, RefreshCw, CheckCircle2, AlertTriangle, MinusCircle, Rocket } from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import type { EngineBootstrapResult, EngineBootstrapStep } from "../../types";

const props = defineProps<{
  currentRepoPath: string;
  refreshNonce: number;
  /** Detected engine display name, used only for the default hint. */
  engineType?: string;
  readiness: number;
}>();

const emit = defineEmits<{
  (e: "refresh"): void;
  (e: "notify", msg: string): void;
}>();

const writeGitattributes = ref(true);
const writeGitignore = ref(true);
const untrackArtifacts = ref(true);
const configureUnityMerge = ref(true);
const lfsPatterns = ref("*.psd,*.fbx,*.uasset,*.umap,*.tga,*.exr,*.wav");
const isRunning = ref(false);
const result = ref<EngineBootstrapResult | null>(null);

const isUnity = computed(() => (props.engineType || "").toLowerCase().includes("unity"));

function parsePatterns(input: string): string[] {
  // TrackExtensionsAsLfs expects bare extensions ("psd") or globs ("*.psd").
  return input
    .split(/[,\n]/)
    .map((p) => p.trim())
    .filter(Boolean);
}

async function runBootstrap() {
  if (!props.currentRepoPath || isRunning.value) return;
  const patterns = parsePatterns(lfsPatterns.value);
  const confirmed = await confirmAction(
    { message: `将一次性配置工程基线：\n\n` +
      `${writeGitattributes.value ? "· 写入 .gitattributes 引擎预设（只补缺失行，不覆盖自定义）\n" : ""}` +
      `${writeGitignore.value ? "· 补全 .gitignore 规则" + (untrackArtifacts.value ? "并取消跟踪已提交的构建产物" : "") + "\n" : ""}` +
      `${isUnity.value && configureUnityMerge.value ? "· 配置 UnityYAMLMerge\n" : ""}` +
      `${patterns.length ? `· 跟踪 LFS 扩展名：${patterns.join("、")}\n` : ""}`, tone: "danger" }
  );
  if (!confirmed) return;

  isRunning.value = true;
  try {
    const res = await gitService.runEngineBootstrap(props.currentRepoPath, {
      engine: "",
      writeGitattributes: writeGitattributes.value,
      writeGitignore: writeGitignore.value,
      untrackArtifacts: untrackArtifacts.value,
      configureUnityMerge: isUnity.value && configureUnityMerge.value,
      lfsExtensions: patterns,
    });
    result.value = res;
    emit("notify", res.message);
    emit("refresh");
  } catch (err: any) {
    emit("notify", `工程配置失败: ${err?.message || err}`);
  } finally {
    isRunning.value = false;
  }
}

function stepMeta(status: EngineBootstrapStep["status"]) {
  switch (status) {
    case "applied":
      return { icon: CheckCircle2, cls: "text-success", badge: "已应用" };
    case "failed":
      return { icon: AlertTriangle, cls: "text-danger", badge: "失败" };
    default:
      return { icon: MinusCircle, cls: "text-ink-muted", badge: "跳过" };
  }
}

watch(
  () => [props.currentRepoPath, props.refreshNonce],
  () => {
    result.value = null;
  }
);
</script>

<template>
  <div class="space-y-3" data-tool-section="engine-bootstrap">
    <div class="pb-2 border-b border-line flex items-start justify-between gap-3">
      <div>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <Sparkles class="w-4 h-4 text-accent" />
          <span>工程基线一键配置 (Bootstrap)</span>
        </h3>
        <p class="text-xs text-ink-muted mt-0.5">
          新建或接手工程时该配的固定几件事：`.gitattributes` 预设、`.gitignore`、UnityYAMLMerge、LFS 跟踪范围。
          只补缺失项，不会覆盖你手写的规则。
        </p>
      </div>
      <div class="flex items-center gap-2 shrink-0">
        <div class="text-right">
          <div class="text-[10px] text-ink-muted">就绪度</div>
          <div
            class="text-sm font-bold font-mono"
            :class="readiness >= 85 ? 'text-success' : readiness >= 50 ? 'text-warning' : 'text-danger'"
          >{{ readiness }}%</div>
        </div>
      </div>
    </div>

    <div class="grid grid-cols-1 sm:grid-cols-2 gap-1.5">
      <label class="flex items-start gap-2 px-2.5 py-1.5 rounded-lg border border-line bg-surface/30 cursor-pointer hover:bg-raised/50 transition">
        <input v-model="writeGitattributes" type="checkbox" class="mt-0.5 accent-[var(--ug-accent-strong)]" />
        <span class="min-w-0">
          <span class="block text-[11px] font-semibold text-ink">.gitattributes 引擎预设</span>
          <span class="block text-[10px] text-ink-muted">LFS 过滤器、Unity 合并驱动、换行规范</span>
        </span>
      </label>

      <label class="flex items-start gap-2 px-2.5 py-1.5 rounded-lg border border-line bg-surface/30 cursor-pointer hover:bg-raised/50 transition">
        <input v-model="writeGitignore" type="checkbox" class="mt-0.5 accent-[var(--ug-accent-strong)]" />
        <span class="min-w-0">
          <span class="block text-[11px] font-semibold text-ink">.gitignore 规则</span>
          <span class="block text-[10px] text-ink-muted">补齐 Library / Intermediate / obj 等缓存目录</span>
        </span>
      </label>

      <label
        v-if="writeGitignore"
        class="flex items-start gap-2 px-2.5 py-1.5 rounded-lg border border-line bg-surface/30 cursor-pointer hover:bg-raised/50 transition"
      >
        <input v-model="untrackArtifacts" type="checkbox" class="mt-0.5 accent-[var(--ug-accent-strong)]" />
        <span class="min-w-0">
          <span class="block text-[11px] font-semibold text-ink">取消跟踪已提交的构建产物</span>
          <span class="block text-[10px] text-ink-muted">把历史误提交的缓存移出索引（保留磁盘文件）</span>
        </span>
      </label>

      <label
        v-if="isUnity"
        class="flex items-start gap-2 px-2.5 py-1.5 rounded-lg border border-line bg-surface/30 cursor-pointer hover:bg-raised/50 transition"
      >
        <input v-model="configureUnityMerge" type="checkbox" class="mt-0.5 accent-[var(--ug-accent-strong)]" />
        <span class="min-w-0">
          <span class="block text-[11px] font-semibold text-ink">配置 UnityYAMLMerge</span>
          <span class="block text-[10px] text-ink-muted">让 .unity / .prefab 按语义合并，而不是逐行文本</span>
        </span>
      </label>
    </div>

    <div class="space-y-1.5">
      <label class="text-[11px] font-semibold text-ink-secondary">LFS 跟踪扩展名（逗号分隔，留空跳过）</label>
      <input
        v-model="lfsPatterns"
        spellcheck="false"
        placeholder="*.psd,*.fbx,*.uasset"
        class="w-full px-3 py-1.5 rounded-lg bg-canvas border border-line text-xs font-mono text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
      />
    </div>

    <button
      @click="runBootstrap"
      :disabled="isRunning"
      class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-white text-xs font-semibold transition cursor-pointer disabled:opacity-40"
    >
      <Rocket class="w-3.5 h-3.5" :class="{ 'animate-pulse': isRunning }" />
      <span>{{ isRunning ? "配置中..." : "一键配置工程基线" }}</span>
    </button>

    <div v-if="result" class="space-y-1.5">
      <div
        class="rounded-lg border px-2.5 py-1.5 text-[11px]"
        :class="result.failed > 0 ? 'border-warning/30 bg-warning/10 text-warning' : 'border-success/30 bg-success/10 text-success'"
      >
        {{ result.message }}（引擎：{{ result.engine }}）
      </div>
      <div
        v-for="step in result.steps"
        :key="step.id"
        class="flex items-start gap-2 px-2.5 py-1 rounded-lg border border-line bg-surface/30 text-[11px]"
      >
        <component :is="stepMeta(step.status).icon" class="w-3.5 h-3.5 shrink-0 mt-0.5" :class="stepMeta(step.status).cls" />
        <span class="font-semibold text-ink shrink-0">{{ step.label }}</span>
        <span class="text-ink-muted flex-1">{{ step.detail }}</span>
      </div>
    </div>
  </div>
</template>
