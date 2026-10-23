<script setup lang="ts">
import { confirmAction } from "../../composables/useConfirm";
import { ref, computed, watch, onMounted, watchEffect } from "vue";
import {
  Box,
  Sparkles,
  FileCode,
  ShieldCheck,
  AlertTriangle,
  Search,
  Wrench,
  Image,
  CheckCircle2,
  Play,
  Trash2,
} from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import EngineQualitySection from "./EngineQualitySection.vue";
import EngineBootstrapSection from "./EngineBootstrapSection.vue";
import EngineDemoSection from "./EngineDemoSection.vue";
import { toolsStatus } from "../../composables/toolsStatus";
import type {
  GameEngineInfo,
  EngineWorkflowInfo,
  AssetMetadata,
  AssetReferenceResult,
  UnityMetaMismatch,
  UnityMergeStatus,
  TextureLintResult,
  UnrealCacheCleanResult,
  UnrealProjectInfo,
  ArtistPipelineResult,
} from "../../types";

const props = defineProps<{
  currentRepoPath: string;
  refreshNonce: number;
}>();

const emit = defineEmits<{
  (e: "refresh"): void;
  (e: "notify", msg: string): void;
}>();

function notify(msg: string) {
  emit("notify", msg);
}

const engineInfo = ref<GameEngineInfo | null>(null);
const workflows = ref<EngineWorkflowInfo[]>([]);
const isLoadingEngine = ref(false);

/** Four independent 25-point checks, mirroring the backend readiness score. */
const engineReadiness = computed(() => {
  const info = engineInfo.value;
  if (!info || !info.engineType || info.engineType === "None") return 0;
  let score = 20; // engine identified
  if (info.hasGitAttributes) score += 30;
  if (info.hasLfsRules) score += 30;
  const isUnity = (info.engineType || "").toLowerCase().includes("unity");
  if (!isUnity || unityMerge.value?.isConfigured) score += 20;
  return score;
});

const assetPath = ref("");
const assetMeta = ref<AssetMetadata | null>(null);
const assetRefs = ref<AssetReferenceResult | null>(null);
const isInspectingAsset = ref(false);

const unityMismatches = ref<UnityMetaMismatch[]>([]);
const isCheckingMeta = ref(false);
const metaChecked = ref(false);
const isFixingMeta = ref(false);

const unityMerge = ref<UnityMergeStatus | null>(null);
const mergeToolPath = ref("");
const applyMergeAttributes = ref(true);
const isConfiguringMerge = ref(false);

const textureResult = ref<TextureLintResult | null>(null);
const isLintingTextures = ref(false);
const isApplyingPreset = ref(false);
const isLaunchingEngine = ref(false);
const isCleaningCaches = ref(false);
const cacheCleanResult = ref<UnrealCacheCleanResult | null>(null);
const unrealInfo = ref<UnrealProjectInfo | null>(null);
const artistMessage = ref("");
const artistTargetBranch = ref("");
const artistResult = ref<ArtistPipelineResult | null>(null);
const isArtistDelivering = ref(false);

async function handleArtistDelivery() {
  if (!props.currentRepoPath || isArtistDelivering.value) return;
  if (
    !(await confirmAction(
      "美术一键交付将：\n1. 暂存全部改动\n2. 生成 [Art] 提交\n3. 推送到目标分支\n\n确定继续吗？"
    ))
  )
    return;
  isArtistDelivering.value = true;
  try {
    artistResult.value = await gitService.artistOneClickPipeline(
      props.currentRepoPath,
      artistMessage.value.trim() || "更新美术资产",
      artistTargetBranch.value.trim()
    );
    if (artistResult.value) notify(artistResult.value.message);
  } catch (err: any) {
    notify(`美术一键交付失败: ${err?.message || err}`);
  } finally {
    isArtistDelivering.value = false;
  }
}

const engineKey = computed(() => {
  const type = (engineInfo.value?.engineType || "").toLowerCase();
  if (type.includes("unreal")) return "unreal";
  if (type.includes("unity")) return "unity";
  if (type.includes("godot")) return "godot";
  if (type.includes("cocos")) return "cocos";
  return "";
});

async function handleLaunchEngine() {
  if (!props.currentRepoPath || isLaunchingEngine.value) return;
  if (!engineKey.value) {
    notify("未检测到可启动的游戏引擎工程");
    return;
  }
  if (!(await confirmAction(`确定启动 ${engineInfo.value?.engineType} 编辑器打开当前工程吗？`))) return;
  isLaunchingEngine.value = true;
  try {
    const message = await gitService.launchGameEngine(props.currentRepoPath, engineKey.value);
    notify(message || "已发送引擎启动指令");
  } catch (err: any) {
    notify(`启动引擎失败: ${err?.message || err}`);
  } finally {
    isLaunchingEngine.value = false;
  }
}

async function handleCleanCaches() {
  if (!props.currentRepoPath || isCleaningCaches.value) return;
  const key = engineKey.value;
  if (!key) {
    notify("未检测到可清理缓存的游戏引擎工程");
    return;
  }
  if (
    !(await confirmAction(
      { message: `确定清理 ${engineInfo.value?.engineType} 的本地缓存目录吗？\n\n该操作会删除 Library / Temp / Intermediate / Saved 等可重建缓存，无法撤销。`, tone: "danger" }
    ))
  )
    return;
  isCleaningCaches.value = true;
  try {
    cacheCleanResult.value =
      key === "unreal"
        ? await gitService.cleanUnrealEngineCache(props.currentRepoPath)
        : await gitService.cleanGameEngineCaches(props.currentRepoPath, key);
    notify(
      cacheCleanResult.value
        ? `缓存清理完成：${cacheCleanResult.value.cleanedFolders.length} 个目录，释放 ${cacheCleanResult.value.freedSizeStr}`
        : "缓存清理完成"
    );
    await loadEngineData();
  } catch (err: any) {
    notify(`缓存清理失败: ${err?.message || err}`);
  } finally {
    isCleaningCaches.value = false;
  }
}

async function loadEngineData() {
  if (!props.currentRepoPath) return;
  isLoadingEngine.value = true;
  try {
    const [info, flows, merge, unreal] = await Promise.all([
      gitService.getGameEngineInfo(props.currentRepoPath).catch(() => null),
      gitService.getGameEngineWorkflows(props.currentRepoPath).catch(() => []),
      gitService.detectUnityYamlMerge(props.currentRepoPath).catch(() => null),
      gitService.detectUnrealProject(props.currentRepoPath).catch(() => null),
    ]);
    engineInfo.value = info;
    workflows.value = flows || [];
    unityMerge.value = merge;
    unrealInfo.value = unreal?.isUnrealProject ? unreal : null;
    if (merge?.detectedToolPath && !mergeToolPath.value) {
      mergeToolPath.value = merge.detectedToolPath;
    }
  } catch (err) {
    console.error("Failed to load engine info:", err);
  } finally {
    isLoadingEngine.value = false;
  }
}

async function inspectAsset() {
  const path = assetPath.value.trim();
  if (!props.currentRepoPath || !path || isInspectingAsset.value) return;
  isInspectingAsset.value = true;
  try {
    const [meta, refs] = await Promise.all([
      gitService.inspectAssetMetadata(props.currentRepoPath, path).catch(() => null),
      gitService.inspectAssetReferences(props.currentRepoPath, path).catch(() => null),
    ]);
    assetMeta.value = meta;
    assetRefs.value = refs;
    notify(
      refs
        ? `资产检查完成：${meta?.sizeHuman || "未知体积"}，${refs.referenceCount} 处引用`
        : "资产元数据已读取"
    );
  } catch (err: any) {
    notify(`资产检查失败: ${err?.message || err}`);
  } finally {
    isInspectingAsset.value = false;
  }
}

async function checkUnityMeta() {
  if (!props.currentRepoPath || isCheckingMeta.value) return;
  isCheckingMeta.value = true;
  try {
    unityMismatches.value =
      (await gitService.checkUnityMetaSync(props.currentRepoPath)) || [];
    metaChecked.value = true;
    notify(
      unityMismatches.value.length > 0
        ? `发现 ${unityMismatches.value.length} 处 Unity meta 配对问题`
        : "Unity meta 配对检查通过"
    );
  } catch (err: any) {
    notify(`Unity meta 检查失败: ${err?.message || err}`);
  } finally {
    isCheckingMeta.value = false;
  }
}

async function handleAutoFixMeta() {
  if (!props.currentRepoPath || isFixingMeta.value) return;
  isFixingMeta.value = true;
  try {
    const fixed = await gitService.autoFixUnityMeta(props.currentRepoPath);
    notify(
      fixed && fixed.length > 0
        ? `已自动修复 ${fixed.length} 个 meta / 资产配对`
        : "没有可自动修复的配对问题"
    );
    await checkUnityMeta();
    emit("refresh");
  } catch (err: any) {
    notify(`自动修复 meta 失败: ${err?.message || err}`);
    // A partial fix may still have staged some files; reload the real state.
    await checkUnityMeta();
    emit("refresh");
  } finally {
    isFixingMeta.value = false;
  }
}

async function handleConfigureUnityMerge() {
  if (!props.currentRepoPath || isConfiguringMerge.value) return;
  const path = mergeToolPath.value.trim() || unityMerge.value?.detectedToolPath || "";
  if (!path) {
    notify("未检测到 UnityYAMLMerge，请先填写可执行文件路径");
    return;
  }
  isConfiguringMerge.value = true;
  try {
    await gitService.configureUnityYamlMerge(
      props.currentRepoPath,
      path,
      applyMergeAttributes.value
    );
    notify("UnityYAMLMerge 已配置到仓库 Git 设置");
    await loadEngineData();
  } catch (err: any) {
    notify(`配置 UnityYAMLMerge 失败: ${err?.message || err}`);
  } finally {
    isConfiguringMerge.value = false;
  }
}

function presetEngineKey() {
  const type = (engineInfo.value?.engineType || "").toLowerCase();
  if (type.includes("unity")) return "unity";
  if (type.includes("unreal")) return "unreal";
  if (type.includes("godot")) return "godot";
  if (type.includes("cocos")) return "cocos";
  return "";
}

async function handleApplyEnginePreset() {
  if (!props.currentRepoPath || isApplyingPreset.value) return;
  const engine = presetEngineKey();
  if (!engine) {
    notify("未能识别支持的引擎类型，无法应用预设");
    return;
  }
  isApplyingPreset.value = true;
  try {
    const preset = await gitService.getGameEngineGitAttributesPreset(engine);
    if (!preset?.trim()) {
      notify("该引擎暂无可用的 .gitattributes 预设");
      return;
    }
    if (!(await confirmAction({ message: `确定用「${engine}」预设覆盖仓库根目录 .gitattributes 吗？现有内容将被替换。`, tone: "danger" }))) {
      return;
    }
    await gitService.saveGitAttributes(props.currentRepoPath, preset);
    notify(`已应用 ${engine} 引擎 .gitattributes 预设`);
    await loadEngineData();
    emit("refresh");
  } catch (err: any) {
    notify(`应用引擎预设失败: ${err?.message || err}`);
  } finally {
    isApplyingPreset.value = false;
  }
}

async function handleLintTextures() {
  if (!props.currentRepoPath || isLintingTextures.value) return;
  isLintingTextures.value = true;
  try {
    textureResult.value = await gitService.lintGameTextures(props.currentRepoPath, true);
    notify(
      textureResult.value && textureResult.value.totalIssues > 0
        ? `纹理审计发现 ${textureResult.value.totalIssues} 项规范问题`
        : "纹理规范审计通过"
    );
  } catch (err: any) {
    notify(`纹理审计失败: ${err?.message || err}`);
  } finally {
    isLintingTextures.value = false;
  }
}

watchEffect(() => {
  toolsStatus.loading.engine =
    isLoadingEngine.value ||
    isInspectingAsset.value ||
    isCheckingMeta.value ||
    isFixingMeta.value ||
    isConfiguringMerge.value ||
    isLintingTextures.value ||
    isApplyingPreset.value ||
    isLaunchingEngine.value ||
    isCleaningCaches.value ||
    isArtistDelivering.value;
});

onMounted(() => {
  void loadEngineData();
  void checkUnityMeta();
});

watch(
  () => props.refreshNonce,
  () => {
    void loadEngineData();
    void checkUnityMeta();
  }
);

function issueClass(issueType: string) {
  if (issueType === "orphan_meta") return "bg-danger/15 text-danger";
  if (issueType === "missing_meta") return "bg-warning/15 text-warning";
  return "bg-accent/15 text-accent";
}
</script>

<template>
  <div class="space-y-6">
    <!-- Section A: Engine detection -->
    <div class="space-y-3" data-tool-section="engine-detect">
      <div class="flex items-center justify-between pb-2 border-b border-line">
        <div>
          <h3 class="text-sm font-bold text-ink flex items-center gap-2">
            <Box class="w-4 h-4 text-accent" />
            <span>游戏引擎检测与工程概览</span>
          </h3>
          <p class="text-xs text-ink-muted mt-0.5">
            自动识别 Unity / Unreal / Godot / Cocos 工程，检查 `.gitattributes` 与 LFS 规则。
          </p>
        </div>
        <div class="flex items-center gap-2">
          <button
            @click="handleApplyEnginePreset"
            :disabled="!engineInfo || isApplyingPreset"
            class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-accent transition cursor-pointer disabled:opacity-40"
            title="用当前引擎推荐的 .gitattributes 预设覆盖仓库配置"
          >
            {{ isApplyingPreset ? "应用中..." : "应用引擎预设" }}
          </button>
          <button
            @click="loadEngineData"
            :disabled="isLoadingEngine"
            class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-40"
          >
            {{ isLoadingEngine ? "检测中..." : "重新检测" }}
          </button>
        </div>
      </div>

      <div v-if="engineInfo" class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-2">
        <div class="p-3 rounded-xl border border-line bg-surface/30">
          <div class="text-[10px] text-ink-muted">引擎类型</div>
          <div class="text-sm font-bold text-ink truncate">{{ engineInfo.engineType }}</div>
        </div>
        <div class="p-3 rounded-xl border border-line bg-surface/30">
          <div class="text-[10px] text-ink-muted">工程名</div>
          <div class="text-sm font-bold text-ink truncate" :title="engineInfo.projectName">{{ engineInfo.projectName }}</div>
        </div>
        <div class="p-3 rounded-xl border border-line bg-surface/30">
          <div class="text-[10px] text-ink-muted">引擎版本</div>
          <div class="text-sm font-bold font-mono text-ink">{{ engineInfo.engineVersion || "—" }}</div>
        </div>
        <div class="p-3 rounded-xl border border-line bg-surface/30">
          <div class="text-[10px] text-ink-muted">.gitattributes</div>
          <div class="text-sm font-bold" :class="engineInfo.hasGitAttributes ? 'text-success' : 'text-ink-muted'">
            {{ engineInfo.hasGitAttributes ? "已配置" : "缺失" }}
          </div>
        </div>
        <div class="p-3 rounded-xl border border-line bg-surface/30">
          <div class="text-[10px] text-ink-muted">LFS 规则</div>
          <div class="text-sm font-bold" :class="engineInfo.hasLfsRules ? 'text-success' : 'text-warning'">
            {{ engineInfo.hasLfsRules ? "已启用" : "未启用" }}
          </div>
        </div>
      </div>

      <div v-if="unrealInfo" class="rounded-xl border border-line bg-surface/30 p-3 grid grid-cols-2 lg:grid-cols-4 gap-2">
        <div class="min-w-0">
          <div class="text-[10px] text-ink-muted">Unreal 工程</div>
          <div class="text-xs font-bold text-ink truncate" :title="unrealInfo.projectName">{{ unrealInfo.projectName }}</div>
        </div>
        <div class="min-w-0">
          <div class="text-[10px] text-ink-muted">引擎版本</div>
          <div class="text-xs font-bold font-mono text-ink">{{ unrealInfo.engineVersion || "—" }}</div>
        </div>
        <div class="min-w-0">
          <div class="text-[10px] text-ink-muted">缓存目录</div>
          <div class="text-xs font-bold text-ink truncate" :title="(unrealInfo.cacheFoldersFound || []).join('、')">
            {{ (unrealInfo.cacheFoldersFound || []).length }} 个 · {{ unrealInfo.cacheSizeStr }}
          </div>
        </div>
        <div class="min-w-0">
          <div class="text-[10px] text-ink-muted">.uproject</div>
          <div class="text-[10px] font-mono text-ink-muted truncate" :title="unrealInfo.uprojectPath">{{ unrealInfo.uprojectPath }}</div>
        </div>
      </div>

      <div v-if="workflows.length > 0" class="grid grid-cols-1 lg:grid-cols-2 gap-2">
        <div
          v-for="w in workflows"
          :key="`${w.engineType}:${w.projectPath}`"
          class="p-3 rounded-xl border border-line bg-surface/30 space-y-1.5"
        >
          <div class="flex items-center gap-2">
            <Sparkles class="w-3.5 h-3.5 text-purple" />
            <span class="text-xs font-bold text-ink uppercase">{{ w.engineType }}</span>
            <span class="text-[10px] text-ink-muted truncate">{{ w.projectName }}</span>
            <span v-if="w.engineVersion" class="ml-auto text-[10px] font-mono text-ink-muted">{{ w.engineVersion }}</span>
          </div>
          <div class="text-[10px] text-ink-muted font-mono">
            缓存 {{ w.cacheSizeStr || "0 B" }} · {{ (w.cacheFolders || []).length }} 个缓存目录
          </div>
          <ul class="space-y-0.5">
            <li
              v-for="action in w.recommendedActions"
              :key="action"
              class="text-[10px] text-ink-secondary flex items-start gap-1"
            >
              <span class="text-accent shrink-0">·</span>
              <span>{{ action }}</span>
            </li>
          </ul>
        </div>
      </div>
    </div>

    <!-- Section A2: One-click project baseline -->
    <EngineBootstrapSection
      :current-repo-path="currentRepoPath"
      :refresh-nonce="refreshNonce"
      :engine-type="engineInfo?.engineType || ''"
      :readiness="engineReadiness"
      @refresh="emit('refresh')"
      @notify="notify"
    />

    <!-- Section B: Asset inspector -->
    <div class="space-y-3" data-tool-section="engine-asset">
      <div class="flex items-center justify-between pb-2 border-b border-line">
        <div>
          <h3 class="text-sm font-bold text-ink flex items-center gap-2">
            <FileCode class="w-4 h-4 text-cyan" />
            <span>资产元数据与引用检查</span>
          </h3>
          <p class="text-xs text-ink-muted mt-0.5">
            查看资产体积 / 类型 / LFS 状态，并扫描工程内引用，判断是否可安全删除。
          </p>
        </div>
      </div>

      <div class="flex items-center gap-2">
        <input
          v-model="assetPath"
          type="text"
          placeholder="资产相对路径，例如 Assets/Characters/hero.png"
          @keydown.enter="inspectAsset"
          class="flex-1 px-3 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
        />
        <button
          @click="inspectAsset"
          :disabled="!assetPath.trim() || isInspectingAsset"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-white text-xs font-semibold transition cursor-pointer disabled:opacity-40 shrink-0"
        >
          <Search class="w-3.5 h-3.5" />
          {{ isInspectingAsset ? "检查中..." : "检查资产" }}
        </button>
      </div>

      <div v-if="assetMeta || assetRefs" class="grid grid-cols-1 lg:grid-cols-2 gap-3">
        <div v-if="assetMeta" class="rounded-xl border border-line bg-surface/30 p-3 space-y-2">
          <div class="text-xs font-bold text-ink">元数据</div>
          <div class="grid grid-cols-2 gap-2 text-[11px]">
            <div class="p-2 rounded-lg bg-canvas/40 border border-line">
              <div class="text-[10px] text-ink-muted">体积</div>
              <div class="font-mono text-ink">{{ assetMeta.sizeHuman }}</div>
            </div>
            <div class="p-2 rounded-lg bg-canvas/40 border border-line">
              <div class="text-[10px] text-ink-muted">类型</div>
              <div class="font-mono text-ink truncate">{{ assetMeta.mimeType || assetMeta.extension }}</div>
            </div>
            <div v-if="assetMeta.dimensions" class="p-2 rounded-lg bg-canvas/40 border border-line">
              <div class="text-[10px] text-ink-muted">尺寸</div>
              <div class="font-mono text-ink">{{ assetMeta.dimensions }}</div>
            </div>
            <div v-if="assetMeta.duration" class="p-2 rounded-lg bg-canvas/40 border border-line">
              <div class="text-[10px] text-ink-muted">时长</div>
              <div class="font-mono text-ink">{{ assetMeta.duration }}</div>
            </div>
            <div class="p-2 rounded-lg bg-canvas/40 border border-line">
              <div class="text-[10px] text-ink-muted">LFS</div>
              <div class="font-mono" :class="assetMeta.isLfs ? 'text-success' : 'text-ink-muted'">
                {{ assetMeta.isLfs ? "是" : "否" }}
              </div>
            </div>
            <div class="p-2 rounded-lg bg-canvas/40 border border-line">
              <div class="text-[10px] text-ink-muted">修改时间</div>
              <div class="font-mono text-ink text-[10px]">{{ assetMeta.lastModTime }}</div>
            </div>
          </div>
        </div>

        <div v-if="assetRefs" class="rounded-xl border border-line bg-surface/30 p-3 space-y-2">
          <div class="flex items-center gap-2">
            <div class="text-xs font-bold text-ink">引用扫描</div>
            <span
              class="px-1.5 py-0.5 rounded text-[10px] font-bold"
              :class="!assetRefs.analysisComplete
                ? 'bg-warning/15 text-warning'
                : assetRefs.safeToDelete ? 'bg-success/15 text-success' : 'bg-danger/15 text-danger'"
            >
              {{ !assetRefs.analysisComplete ? "扫描不完整" : assetRefs.safeToDelete ? "可安全删除" : "仍被引用" }}
            </span>
            <span class="ml-auto text-[10px] text-ink-muted font-mono">{{ assetRefs.referenceCount }} 处</span>
          </div>
          <div class="text-[10px] text-ink-muted">{{ assetRefs.message }}</div>
          <ul v-if="assetRefs.warnings?.length" class="space-y-0.5 text-[10px] text-warning">
            <li v-for="warning in assetRefs.warnings" :key="warning">• {{ warning }}</li>
          </ul>
          <div class="space-y-1 max-h-48 overflow-y-auto">
            <div
              v-for="ref in assetRefs.references"
              :key="`${ref.filePath}:${ref.matchCount}`"
              class="px-2 py-1.5 rounded-lg bg-canvas/40 border border-line text-[10px]"
            >
              <div class="flex items-center gap-2">
                <span class="px-1 py-0.2 rounded bg-accent/15 text-accent font-bold shrink-0">{{ ref.fileType }}</span>
                <span class="font-mono text-ink truncate">{{ ref.filePath }}</span>
                <span class="ml-auto text-ink-muted shrink-0">×{{ ref.matchCount }}</span>
              </div>
              <div v-if="ref.previewSnippet" class="font-mono text-ink-muted truncate mt-0.5">{{ ref.previewSnippet }}</div>
            </div>
            <div v-if="(assetRefs.references || []).length === 0" class="py-4 text-center text-ink-muted">
              {{ assetRefs.analysisComplete ? "未发现引用" : "未发现引用，但扫描不完整" }}
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Section C: Unity meta sync -->
    <div class="space-y-3" data-tool-section="engine-unity-meta">
      <div class="flex items-center justify-between pb-2 border-b border-line">
        <div>
          <h3 class="text-sm font-bold text-ink flex items-center gap-2">
            <ShieldCheck class="w-4 h-4 text-success" />
            <span>Unity Meta 配对检查</span>
          </h3>
          <p class="text-xs text-ink-muted mt-0.5">
            检查变更 / 未跟踪资产与 `.meta` 的配对关系，发现缺失、孤立或暂存不一致。
          </p>
        </div>
        <div class="flex items-center gap-2">
          <button
            @click="handleAutoFixMeta"
            :disabled="isFixingMeta || unityMismatches.length === 0"
            class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-warning transition cursor-pointer disabled:opacity-40"
            title="自动暂存未配对的 .meta / 资产"
          >
            {{ isFixingMeta ? "修复中..." : "一键修复配对" }}
          </button>
          <button
            @click="checkUnityMeta"
            :disabled="isCheckingMeta"
            class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-40"
          >
            {{ isCheckingMeta ? "检查中..." : "重新检查" }}
          </button>
        </div>
      </div>

      <div v-if="unityMismatches.length > 0" class="space-y-1.5">
        <div
          v-for="m in unityMismatches"
          :key="`${m.issueType}:${m.metaPath}`"
          class="flex items-start gap-2 px-2.5 py-1.5 rounded-lg bg-warning/5 border border-warning/20 text-[11px]"
        >
          <AlertTriangle class="w-3.5 h-3.5 text-warning shrink-0 mt-0.5" />
          <span class="px-1.5 py-0.5 rounded text-[9px] font-bold shrink-0" :class="issueClass(m.issueType)">
            {{ m.issueType }}
          </span>
          <div class="min-w-0">
            <div class="font-mono text-ink truncate">{{ m.assetPath }}</div>
            <div class="text-[10px] text-ink-muted">{{ m.message }}</div>
          </div>
        </div>
      </div>
      <div v-else-if="metaChecked" class="py-3 text-center text-xs text-success">
        未发现 Unity meta 配对问题
      </div>
    </div>

    <!-- Section D: Unity YAML merge -->
    <div class="space-y-3" data-tool-section="engine-unity-merge">
      <div class="flex items-center justify-between pb-2 border-b border-line">
        <div>
          <h3 class="text-sm font-bold text-ink flex items-center gap-2">
            <Wrench class="w-4 h-4 text-purple" />
            <span>Unity 智能合并 (UnityYAMLMerge)</span>
          </h3>
          <p class="text-xs text-ink-muted mt-0.5">
            配置 `merge=unityyamlmerge` 驱动，让 Prefab / Scene / Material 冲突按语义合并。
          </p>
        </div>
        <span
          class="px-2 py-0.5 rounded text-[10px] font-bold"
          :class="unityMerge?.isConfigured ? 'bg-success/15 text-success' : 'bg-warning/15 text-warning'"
        >
          {{ unityMerge?.isConfigured ? "已配置" : "未配置" }}
        </span>
      </div>

      <div v-if="unityMerge" class="grid grid-cols-2 lg:grid-cols-4 gap-2">
        <div class="p-2.5 rounded-lg border border-line bg-surface/30">
          <div class="text-[10px] text-ink-muted">检测到的工具</div>
          <div class="font-mono text-[10px] text-ink truncate" :title="unityMerge.detectedToolPath">
            {{ unityMerge.detectedToolPath || "未检测到" }}
          </div>
        </div>
        <div class="p-2.5 rounded-lg border border-line bg-surface/30">
          <div class="text-[10px] text-ink-muted">已配置路径</div>
          <div class="font-mono text-[10px] text-ink truncate" :title="unityMerge.configuredToolPath">
            {{ unityMerge.configuredToolPath || "—" }}
          </div>
        </div>
        <div class="p-2.5 rounded-lg border border-line bg-surface/30">
          <div class="text-[10px] text-ink-muted">已安装版本</div>
          <div class="text-xs text-ink">{{ (unityMerge.installedVersions || []).length }} 个</div>
        </div>
        <div class="p-2.5 rounded-lg border border-line bg-surface/30">
          <div class="text-[10px] text-ink-muted">.gitattributes 合并规则</div>
          <div class="text-xs" :class="unityMerge.hasGitattributes ? 'text-success' : 'text-warning'">
            {{ unityMerge.hasGitattributes ? "已配置" : "缺失" }}
          </div>
        </div>
      </div>

      <div class="flex flex-wrap items-center gap-2">
        <input
          v-model="mergeToolPath"
          type="text"
          placeholder="UnityYAMLMerge 可执行文件路径（留空使用自动检测）"
          class="flex-1 min-w-[260px] px-3 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
        />
        <label class="flex items-center gap-1.5 text-[11px] text-ink-muted cursor-pointer select-none">
          <input v-model="applyMergeAttributes" type="checkbox" class="rounded accent-purple" />
          <span>同时写入 .gitattributes 合并规则</span>
        </label>
        <button
          @click="handleConfigureUnityMerge"
          :disabled="isConfiguringMerge"
          class="px-3 py-1.5 rounded-lg bg-purple-strong text-white text-xs font-semibold transition cursor-pointer disabled:opacity-40 shrink-0"
        >
          {{ isConfiguringMerge ? "配置中..." : "配置 UnityYAMLMerge" }}
        </button>
      </div>
    </div>

    <!-- Section E: Texture lint -->
    <div class="space-y-3" data-tool-section="engine-texture">
      <div class="flex items-center justify-between pb-2 border-b border-line">
        <div>
          <h3 class="text-sm font-bold text-ink flex items-center gap-2">
            <Image class="w-4 h-4 text-cyan" />
            <span>纹理规范审计 (Texture Lint)</span>
          </h3>
          <p class="text-xs text-ink-muted mt-0.5">
            检查项目内纹理的 2 的幂尺寸、超大分辨率与整体规范，给出优化建议。
          </p>
        </div>
        <button
          @click="handleLintTextures"
          :disabled="isLintingTextures"
          class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-40"
        >
          {{ isLintingTextures ? "审计中..." : "开始纹理审计" }}
        </button>
      </div>

      <div v-if="textureResult" class="space-y-2">
        <div class="flex items-center gap-3 text-[11px] text-ink-muted font-mono">
          <span>检查 {{ textureResult.totalChecked }} 张纹理</span>
          <span :class="textureResult.totalIssues > 0 ? 'text-warning' : 'text-success'">
            {{ textureResult.totalIssues }} 项问题
          </span>
        </div>
        <div v-if="(textureResult.items || []).length > 0" class="space-y-1 max-h-56 overflow-y-auto">
          <div
            v-for="item in textureResult.items || []"
            :key="item.path"
            class="flex items-center gap-2 px-2.5 py-1.5 rounded-lg border text-[11px]"
            :class="item.severity === 'ERROR' ? 'bg-danger/5 border-danger/20' : item.severity === 'WARNING' ? 'bg-warning/5 border-warning/20' : 'bg-surface/30 border-line'"
          >
            <span
              class="px-1.5 py-0.5 rounded text-[9px] font-bold shrink-0"
              :class="item.severity === 'ERROR' ? 'bg-danger/15 text-danger' : item.severity === 'WARNING' ? 'bg-warning/15 text-warning' : 'bg-success/15 text-success'"
            >
              {{ item.severity }}
            </span>
            <span class="font-mono text-ink truncate flex-1" :title="item.path">{{ item.path }}</span>
            <span class="font-mono text-ink-muted shrink-0">{{ item.dimensions }}</span>
            <span class="font-mono text-ink-muted shrink-0">{{ item.sizeStr }}</span>
            <span class="text-ink-muted truncate max-w-[220px]" :title="item.advice">{{ item.issue || item.advice }}</span>
          </div>
        </div>
        <div v-else class="py-3 text-center text-xs text-success flex items-center justify-center gap-1.5">
          <CheckCircle2 class="w-4 h-4" />
          未发现纹理规范问题
        </div>
      </div>
    </div>

    <!-- Section F: Engine runtime & cache -->
    <div class="space-y-3 pt-4" data-tool-section="engine-runtime">
      <div class="pb-2 border-b border-line">
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <Play class="w-4 h-4 text-success" />
          <span>引擎启动与缓存清理</span>
        </h3>
        <p class="text-xs text-ink-muted mt-0.5">
          启动当前工程对应的引擎编辑器，或清理可重建的本地缓存目录。
        </p>
      </div>
      <div class="flex flex-wrap items-center gap-2 rounded-xl border border-line bg-surface/30 p-3">
        <span class="text-[11px] text-ink-muted">
          当前引擎：
          <span class="font-mono text-ink">{{ engineInfo?.engineType || "未检测到" }}</span>
        </span>
        <button
          @click="handleLaunchEngine"
          :disabled="!engineKey || isLaunchingEngine"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-white text-xs font-semibold transition cursor-pointer disabled:opacity-40"
        >
          <Play class="w-3.5 h-3.5" />
          <span>{{ isLaunchingEngine ? "启动中..." : "启动引擎编辑器" }}</span>
        </button>
        <button
          @click="handleCleanCaches"
          :disabled="!engineKey || isCleaningCaches"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-warning/30 text-xs font-semibold text-warning transition cursor-pointer disabled:opacity-40"
        >
          <Trash2 class="w-3.5 h-3.5" />
          <span>{{ isCleaningCaches ? "清理中..." : "清理本地缓存" }}</span>
        </button>
        <span v-if="cacheCleanResult" class="text-[10px] text-ink-muted font-mono">
          已清理 {{ cacheCleanResult.cleanedFolders.length }} 个目录 · 释放 {{ cacheCleanResult.freedSizeStr }}
        </span>
      </div>

      <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-2">
        <div class="text-xs font-bold text-ink">美术一键交付</div>
        <div class="flex flex-wrap items-center gap-2">
          <input
            v-model="artistMessage"
            type="text"
            placeholder="提交说明（自动加 [Art] 前缀），例如 更新角色贴图"
            class="flex-1 min-w-[220px] px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
          />
          <input
            v-model="artistTargetBranch"
            type="text"
            placeholder="目标分支（默认当前分支）"
            class="w-44 px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
          />
          <button
            @click="handleArtistDelivery"
            :disabled="isArtistDelivering"
            class="px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-white text-xs font-semibold transition cursor-pointer disabled:opacity-40"
          >
            {{ isArtistDelivering ? "交付中..." : "一键交付资产" }}
          </button>
        </div>
        <p class="text-[10px] text-ink-muted">
          暂存全部改动 → 生成 [Art] 提交 → 推送到 origin 目标分支；推送失败时保留本地提交。
        </p>
        <template v-if="artistResult">
          <div class="grid grid-cols-4 gap-2">
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">提交文件</div>
              <div class="text-sm font-bold font-mono text-ink">{{ artistResult.stagedAssetsCount }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">贴图</div>
              <div class="text-sm font-bold font-mono text-purple">{{ artistResult.texturesChecked }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">网格</div>
              <div class="text-sm font-bold font-mono text-cyan">{{ artistResult.meshesChecked }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">已推送</div>
              <div class="text-sm font-bold" :class="artistResult.pushed ? 'text-success' : 'text-warning'">
                {{ artistResult.pushed ? "是" : "否" }}
              </div>
            </div>
          </div>
          <div class="space-y-0.5 max-h-32 overflow-y-auto">
            <div
              v-for="(log, index) in artistResult.pipelineLogs || []"
              :key="index"
              class="text-[10px] font-mono text-ink-muted"
            >
              {{ log }}
            </div>
          </div>
        </template>
      </div>
    </div>

    <EngineQualitySection :current-repo-path="currentRepoPath" @notify="notify" />
    <EngineDemoSection :current-repo-path="currentRepoPath" @notify="notify" />
  </div>
</template>
