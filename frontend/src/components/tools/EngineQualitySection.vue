<script setup lang="ts">
import { confirmAction } from "../../composables/useConfirm";
import { ref } from "vue";
import { Activity, Box, FileWarning, Globe, Search, Server } from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import type {
  MeshLintResult,
  AnimRigLintResult,
  AudioInspectResult,
  LocalizationAuditResult,
  NativePluginsAbiResult,
  MissingMaterialScanResult,
  LodStreamingAuditResult,
  PhysicsCollisionAuditResult,
  MaterialInstanceOverrideResult,
  DCCLockWatchResult,
  ShaderInspectResult,
  SymbolsDefinesAuditResult,
  DataTableAuditResult,
  AssetCherryPickResult,
  UnrealAssetDiffResult,
  SceneStreamAuditResult,
  SceneStreamResolveResult,
} from "../../types";

const props = defineProps<{
  currentRepoPath: string;
}>();

const emit = defineEmits<{
  (e: "notify", msg: string): void;
}>();

function notify(msg: string) {
  emit("notify", msg);
}

const meshPath = ref("");
const meshPreset = ref("mobile_character");
const meshResult = ref<MeshLintResult | null>(null);
const isLintingMesh = ref(false);

const rigPath = ref("");
const rigPreset = ref("Mobile");
const rigResult = ref<AnimRigLintResult | null>(null);
const isLintingRig = ref(false);

const audioPath = ref("");
const audioResult = ref<AudioInspectResult | null>(null);
const isInspectingAudio = ref(false);

const fontPath = ref("");
const localizationResult = ref<LocalizationAuditResult | null>(null);
const isAuditingLocalization = ref(false);

const pluginResult = ref<NativePluginsAbiResult | null>(null);
const isAuditingPlugins = ref(false);

const materialTargetDir = ref("");
const materialResult = ref<MissingMaterialScanResult | null>(null);
const isScanningMaterials = ref(false);

const auditScopeDir = ref("");
const lodResult = ref<LodStreamingAuditResult | null>(null);
const isAuditingLod = ref(false);
const physicsResult = ref<PhysicsCollisionAuditResult | null>(null);
const isAuditingPhysics = ref(false);
const materialOverrideResult = ref<MaterialInstanceOverrideResult | null>(null);
const isAuditingMaterialOverrides = ref(false);
const dccResult = ref<DCCLockWatchResult | null>(null);
const isDetectingDcc = ref(false);
const dccAssetPath = ref("");
const isAutoLockingDcc = ref(false);

const shaderPath = ref("");
const shaderResult = ref<ShaderInspectResult | null>(null);
const isInspectingShader = ref(false);

const symbolsBase = ref("HEAD~1");
const symbolsTarget = ref("HEAD");
const symbolsResult = ref<SymbolsDefinesAuditResult | null>(null);
const isAuditingSymbols = ref(false);

const dataTablePath = ref("");
const dataTableBase = ref("HEAD");
const dataTableTarget = ref("WORKTREE");
const dataTableResult = ref<DataTableAuditResult | null>(null);
const isAuditingTables = ref(false);

const assetPickTargetRef = ref("");
const assetPickPaths = ref("");
const assetPickResolveDeps = ref(true);
const assetPickDryRun = ref(true);
const assetPickResult = ref<AssetCherryPickResult | null>(null);
const isPickingAssets = ref(false);

const unrealAssetPath = ref("");
const unrealBaseRef = ref("HEAD");
const unrealTargetRef = ref("WORKTREE");
const unrealDiffResult = ref<UnrealAssetDiffResult | null>(null);
const isDiffingUnreal = ref(false);

const sceneTargetBranch = ref("main");
const sceneAuditPath = ref("");
const sceneAuditResult = ref<SceneStreamAuditResult | null>(null);
const isAuditingScene = ref(false);

const resolveScenePath = ref("");
const resolveSceneGuid = ref("");
const resolveSceneAction = ref("AUTO_REGENERATE_GUID");
const resolveSceneResult = ref<SceneStreamResolveResult | null>(null);
const isResolvingScene = ref(false);

async function handleAuditSceneStream() {
  if (!props.currentRepoPath || isAuditingScene.value) return;
  isAuditingScene.value = true;
  try {
    sceneAuditResult.value = await gitService.auditSceneStreamMerge(
      props.currentRepoPath,
      sceneTargetBranch.value.trim() || "main",
      sceneAuditPath.value.trim()
    );
    if (sceneAuditResult.value) notify(sceneAuditResult.value.summaryMessage);
  } catch (err: any) {
    notify(`场景流式合并审计失败: ${err?.message || err}`);
  } finally {
    isAuditingScene.value = false;
  }
}

async function handleResolveSceneConflict() {
  if (
    !props.currentRepoPath ||
    !resolveScenePath.value.trim() ||
    !resolveSceneGuid.value.trim() ||
    isResolvingScene.value
  )
    return;
  if (
    !(await confirmAction(
      `确定对场景 ${resolveScenePath.value.trim()} 应用 ${resolveSceneAction.value} 吗？\n\n该操作会直接修改关卡文件，请确认已有备份或提交。`
    ))
  )
    return;
  isResolvingScene.value = true;
  try {
    resolveSceneResult.value = await gitService.resolveSceneStreamConflict(
      props.currentRepoPath,
      resolveScenePath.value.trim(),
      resolveSceneAction.value,
      resolveSceneGuid.value.trim()
    );
    if (resolveSceneResult.value) notify(resolveSceneResult.value.message);
  } catch (err: any) {
    notify(`场景冲突修复失败: ${err?.message || err}`);
  } finally {
    isResolvingScene.value = false;
  }
}

async function handlePickAssets() {
  const paths = assetPickPaths.value
    .split(/[\n,]/)
    .map((path) => path.trim())
    .filter(Boolean);
  if (!props.currentRepoPath || !assetPickTargetRef.value.trim() || paths.length === 0) return;
  if (isPickingAssets.value) return;
  if (
    !assetPickDryRun.value &&
    !(await confirmAction(
      `确定把 ${paths.length} 个资产从 ${assetPickTargetRef.value.trim()} 拣选到当前分支吗？${assetPickResolveDeps.value ? "将自动包含依赖。" : ""}`
    ))
  )
    return;
  isPickingAssets.value = true;
  try {
    assetPickResult.value = await gitService.cherryPickAssetBundle(
      props.currentRepoPath,
      assetPickTargetRef.value.trim(),
      paths,
      assetPickResolveDeps.value,
      assetPickDryRun.value
    );
    if (assetPickResult.value) notify(assetPickResult.value.message || assetPickResult.value.summary);
  } catch (err: any) {
    notify(`资产拣选失败: ${err?.message || err}`);
  } finally {
    isPickingAssets.value = false;
  }
}

async function handleDiffUnrealAsset() {
  if (!props.currentRepoPath || !unrealAssetPath.value.trim() || isDiffingUnreal.value) return;
  isDiffingUnreal.value = true;
  try {
    unrealDiffResult.value = await gitService.diffUnrealAssetNodes(
      props.currentRepoPath,
      unrealAssetPath.value.trim(),
      unrealBaseRef.value.trim() || "HEAD",
      unrealTargetRef.value.trim() || "WORKTREE"
    );
    if (unrealDiffResult.value) notify(unrealDiffResult.value.message);
  } catch (err: any) {
    notify(`Unreal 资产差异失败: ${err?.message || err}`);
  } finally {
    isDiffingUnreal.value = false;
  }
}

async function handleAuditDataTables() {
  if (!props.currentRepoPath || isAuditingTables.value) return;
  isAuditingTables.value = true;
  try {
    dataTableResult.value = await gitService.auditDataTableDiff(
      props.currentRepoPath,
      dataTablePath.value.trim(),
      dataTableBase.value.trim() || "HEAD",
      dataTableTarget.value.trim() || "WORKTREE"
    );
    if (dataTableResult.value) {
      notify(
        dataTableResult.value.analysisKnown
          ? `数据表审计完成：${dataTableResult.value.totalTables} 张表，${dataTableResult.value.criticalCount} 项严重 / ${dataTableResult.value.warningCount} 项警告`
          : "数据表审计：未发现可审计的数据表"
      );
    }
  } catch (err: any) {
    notify(`数据表差异审计失败: ${err?.message || err}`);
  } finally {
    isAuditingTables.value = false;
  }
}

async function handleInspectShader() {
  if (!props.currentRepoPath || !shaderPath.value.trim() || isInspectingShader.value) return;
  isInspectingShader.value = true;
  try {
    shaderResult.value = await gitService.inspectShaderVariants(
      props.currentRepoPath,
      shaderPath.value.trim()
    );
    if (shaderResult.value) {
      notify(
        `${shaderResult.value.targetShader}: ${shaderResult.value.totalVariants} 变体（${shaderResult.value.severity}）`
      );
    }
  } catch (err: any) {
    notify(`Shader 变体审计失败: ${err?.message || err}`);
  } finally {
    isInspectingShader.value = false;
  }
}

async function handleAuditSymbols() {
  if (!props.currentRepoPath || isAuditingSymbols.value) return;
  isAuditingSymbols.value = true;
  try {
    symbolsResult.value = await gitService.auditSymbolsAndDefines(
      props.currentRepoPath,
      symbolsBase.value.trim(),
      symbolsTarget.value.trim()
    );
    if (symbolsResult.value) {
      notify(
        symbolsResult.value.analysisKnown
          ? `宏定义审计完成：${symbolsResult.value.totalDefines} 个宏，危险调试宏 ${symbolsResult.value.dangerousDebugDefines.length} 个`
          : "宏定义审计：未发现可解析的宏定义来源"
      );
    }
  } catch (err: any) {
    notify(`宏定义审计失败: ${err?.message || err}`);
  } finally {
    isAuditingSymbols.value = false;
  }
}

async function handleAutoLockDcc() {
  if (!props.currentRepoPath || !dccAssetPath.value.trim() || isAutoLockingDcc.value) return;
  isAutoLockingDcc.value = true;
  try {
    const res = await gitService.autoLockDccAsset(
      props.currentRepoPath,
      dccAssetPath.value.trim()
    );
    if (res?.success) {
      notify(res.message || `已锁定 ${dccAssetPath.value.trim()}`);
      dccAssetPath.value = "";
    } else {
      notify(res?.message || "DCC 资产上锁失败");
    }
  } catch (err: any) {
    notify(`DCC 资产上锁失败: ${err?.message || err}`);
  } finally {
    isAutoLockingDcc.value = false;
  }
}

async function handleAuditLod() {
  if (!props.currentRepoPath || isAuditingLod.value) return;
  isAuditingLod.value = true;
  try {
    lodResult.value = await gitService.auditLodStreaming(
      props.currentRepoPath,
      auditScopeDir.value.trim()
    );
    if (lodResult.value) {
      notify(
        lodResult.value.analysisKnown
          ? `LOD / 纹理串流审计完成：健康度 ${lodResult.value.overallHealthScore}，缺失 LOD ${lodResult.value.lodMissingCount}，缺失串流 ${lodResult.value.streamingMissingCount}`
          : "LOD / 纹理串流审计：未发现可审计资产，未作评分"
      );
    }
  } catch (err: any) {
    notify(`LOD 串流审计失败: ${err?.message || err}`);
  } finally {
    isAuditingLod.value = false;
  }
}

async function handleAuditPhysics() {
  if (!props.currentRepoPath || isAuditingPhysics.value) return;
  isAuditingPhysics.value = true;
  try {
    physicsResult.value = await gitService.auditPhysicsMeshCollision(
      props.currentRepoPath,
      auditScopeDir.value.trim()
    );
    if (physicsResult.value) {
      notify(
        physicsResult.value.analysisKnown
          ? `物理碰撞体检完成：${physicsResult.value.totalColliders} 个碰撞体，健康度 ${physicsResult.value.healthScore}`
          : "物理碰撞体检：未发现可审计的碰撞体，未作评分"
      );
    }
  } catch (err: any) {
    notify(`物理碰撞体检失败: ${err?.message || err}`);
  } finally {
    isAuditingPhysics.value = false;
  }
}

async function handleAuditMaterialOverrides() {
  if (!props.currentRepoPath || isAuditingMaterialOverrides.value) return;
  isAuditingMaterialOverrides.value = true;
  try {
    materialOverrideResult.value = await gitService.auditMaterialInstanceOverrides(
      props.currentRepoPath,
      auditScopeDir.value.trim()
    );
    if (materialOverrideResult.value) {
      notify(
        `材质参数审计完成：${materialOverrideResult.value.totalInstances} 个材质，${materialOverrideResult.value.deadParametersCount} 个失效参数`
      );
    }
  } catch (err: any) {
    notify(`材质参数审计失败: ${err?.message || err}`);
  } finally {
    isAuditingMaterialOverrides.value = false;
  }
}

async function handleDetectDcc() {
  if (!props.currentRepoPath || isDetectingDcc.value) return;
  isDetectingDcc.value = true;
  try {
    dccResult.value = await gitService.detectDccLockActivity(props.currentRepoPath);
    if (dccResult.value) {
      notify(
        `DCC 锁巡检完成：${dccResult.value.activeLocksCount} 个活动锁，${dccResult.value.collisionCount} 个冲突`
      );
    }
  } catch (err: any) {
    notify(`DCC 锁巡检失败: ${err?.message || err}`);
  } finally {
    isDetectingDcc.value = false;
  }
}

function severityClass(severity: string) {
  if (severity === "OK") return "bg-success/15 text-success";
  if (severity === "WARNING") return "bg-warning/15 text-warning";
  if (severity === "UNKNOWN") return "bg-raised text-ink-muted";
  return "bg-danger/15 text-danger";
}

async function handleLintMesh() {
  if (!props.currentRepoPath || !meshPath.value.trim() || isLintingMesh.value) return;
  isLintingMesh.value = true;
  try {
    meshResult.value = await gitService.lintModelMesh(
      props.currentRepoPath,
      meshPath.value.trim(),
      meshPreset.value
    );
    if (meshResult.value) {
      notify(
        `${meshResult.value.path}: ${meshResult.value.triangleCountKnown ? `${meshResult.value.triangleCount} 面` : "面数未知"} / 预算 ${meshResult.value.budgetLimit}（${meshResult.value.severity}）`
      );
    }
  } catch (err: any) {
    notify(`模型面数体检失败: ${err?.message || err}`);
  } finally {
    isLintingMesh.value = false;
  }
}

async function handleLintRig() {
  if (!props.currentRepoPath || !rigPath.value.trim() || isLintingRig.value) return;
  isLintingRig.value = true;
  try {
    rigResult.value = await gitService.lintAnimationRig(
      props.currentRepoPath,
      rigPath.value.trim(),
      rigPreset.value
    );
    if (rigResult.value) {
      notify(
        `${rigResult.value.assetPath}: ${rigResult.value.boneCount} 骨骼 / ${rigResult.value.performanceRating}`
      );
    }
  } catch (err: any) {
    notify(`骨骼动画体检失败: ${err?.message || err}`);
  } finally {
    isLintingRig.value = false;
  }
}

async function handleInspectAudio() {
  if (!props.currentRepoPath || !audioPath.value.trim() || isInspectingAudio.value) return;
  isInspectingAudio.value = true;
  try {
    audioResult.value = await gitService.inspectAudioFile(
      props.currentRepoPath,
      audioPath.value.trim()
    );
    if (audioResult.value) notify(audioResult.value.message);
  } catch (err: any) {
    notify(`音频资产体检失败: ${err?.message || err}`);
  } finally {
    isInspectingAudio.value = false;
  }
}

async function handleAuditLocalization() {
  if (!props.currentRepoPath || isAuditingLocalization.value) return;
  isAuditingLocalization.value = true;
  try {
    localizationResult.value = await gitService.auditLocalizationParity(
      props.currentRepoPath,
      fontPath.value.trim()
    );
    if (localizationResult.value) {
      notify(
        localizationResult.value.analysisKnown
          ? `本地化体检完成：${localizationResult.value.languagesFound.length} 种语言，对齐度 ${localizationResult.value.overallParityScore}%`
          : "本地化体检：未发现可审计的本地化文本"
      );
    }
  } catch (err: any) {
    notify(`本地化对齐体检失败: ${err?.message || err}`);
  } finally {
    isAuditingLocalization.value = false;
  }
}

async function handleAuditPlugins() {
  if (!props.currentRepoPath || isAuditingPlugins.value) return;
  isAuditingPlugins.value = true;
  try {
    pluginResult.value = await gitService.auditNativePluginsAbi(props.currentRepoPath);
    if (pluginResult.value) {
      notify(
        `原生插件审计完成：${pluginResult.value.totalPluginsCount} 个插件，${pluginResult.value.defectCount} 项缺陷`
      );
    }
  } catch (err: any) {
    notify(`原生插件 ABI 审计失败: ${err?.message || err}`);
  } finally {
    isAuditingPlugins.value = false;
  }
}

async function handleScanMaterials() {
  if (!props.currentRepoPath || isScanningMaterials.value) return;
  isScanningMaterials.value = true;
  try {
    materialResult.value = await gitService.scanMissingMaterials(
      props.currentRepoPath,
      materialTargetDir.value.trim()
    );
    if (materialResult.value) notify(materialResult.value.message);
  } catch (err: any) {
    notify(`缺失材质扫描失败: ${err?.message || err}`);
  } finally {
    isScanningMaterials.value = false;
  }
}
</script>

<template>
  <div class="space-y-3 pt-4">
    <div class="pb-2 border-b border-line">
      <h3 class="text-sm font-bold text-ink flex items-center gap-2">
        <Box class="w-4 h-4 text-accent" />
        <span>资产质检工具</span>
      </h3>
      <p class="text-xs text-ink-muted mt-0.5">
        模型面数 / 骨骼动画 / 音频规范 / 本地化对齐 / 原生插件 ABI / 缺失材质。
      </p>
    </div>

    <div class="grid grid-cols-1 lg:grid-cols-2 gap-3">
      <!-- Mesh -->
      <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-3">
        <div class="flex items-center gap-2">
          <Box class="w-4 h-4 text-purple" />
          <span class="text-xs font-bold text-ink">模型面数体检</span>
        </div>
        <div class="flex items-center gap-2">
          <input
            v-model="meshPath"
            type="text"
            placeholder="模型路径，例如 Assets/Characters/hero.fbx"
            class="flex-1 px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
          />
          <select
            v-model="meshPreset"
            aria-label="模型面数预算预设"
            class="px-2 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink focus:outline-none focus:border-accent cursor-pointer"
          >
            <option value="mobile_prop">移动端道具 5k</option>
            <option value="mobile_character">移动端角色 30k</option>
            <option value="pc_character">PC 角色 100k</option>
            <option value="pc_environment">PC 场景 250k</option>
          </select>
          <button
            @click="handleLintMesh"
            :disabled="!meshPath.trim() || isLintingMesh"
            class="px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-white text-xs font-semibold transition cursor-pointer disabled:opacity-40 shrink-0"
          >
            {{ isLintingMesh ? "体检中..." : "体检" }}
          </button>
        </div>
        <template v-if="meshResult">
          <div class="flex items-center gap-2">
            <span class="px-1.5 py-0.5 rounded text-[9px] font-bold" :class="severityClass(meshResult.severity)">
              {{ meshResult.severity }}
            </span>
            <span class="text-[11px] text-ink-muted font-mono truncate">{{ meshResult.path }}</span>
          </div>
          <div class="grid grid-cols-4 gap-2">
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">三角面</div>
              <div class="text-sm font-bold font-mono" :class="meshResult.isOverBudget ? 'text-danger' : 'text-ink'">
                {{ meshResult.triangleCountKnown ? meshResult.triangleCount : "未知" }}
              </div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">顶点</div>
              <div class="text-sm font-bold font-mono text-ink">{{ meshResult.triangleCountKnown ? meshResult.vertexCount : "未知" }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">材质</div>
              <div class="text-sm font-bold font-mono text-purple">{{ meshResult.triangleCountKnown ? meshResult.materialCount : "—" }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">预算</div>
              <div class="text-sm font-bold font-mono text-ink-muted">{{ meshResult.budgetLimit }}</div>
            </div>
          </div>
          <p v-if="meshResult.advice" class="text-[10px] text-ink-muted">{{ meshResult.advice }}</p>
          <div v-if="(meshResult.warnings || []).length > 0" class="space-y-1">
            <div v-for="(warn, i) in meshResult.warnings || []" :key="i" class="text-[10px] text-warning">{{ warn }}</div>
          </div>
        </template>
      </div>

      <!-- Rig -->
      <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-3">
        <div class="flex items-center gap-2">
          <Activity class="w-4 h-4 text-warning" />
          <span class="text-xs font-bold text-ink">骨骼动画体检</span>
        </div>
        <div class="flex items-center gap-2">
          <input
            v-model="rigPath"
            type="text"
            placeholder="动画路径，例如 Assets/Anim/hero_idle.fbx"
            class="flex-1 px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
          />
          <select
            v-model="rigPreset"
            aria-label="骨骼动画预算档位"
            class="px-2 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink focus:outline-none focus:border-accent cursor-pointer"
          >
            <option value="Mobile">Mobile</option>
            <option value="Console">Console</option>
            <option value="Cinematic">Cinematic</option>
          </select>
          <button
            @click="handleLintRig"
            :disabled="!rigPath.trim() || isLintingRig"
            class="px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-white text-xs font-semibold transition cursor-pointer disabled:opacity-40 shrink-0"
          >
            {{ isLintingRig ? "体检中..." : "体检" }}
          </button>
        </div>
        <template v-if="rigResult">
          <div class="flex items-center gap-2">
            <span class="px-1.5 py-0.5 rounded text-[9px] font-bold" :class="severityClass(rigResult.severity)">
              {{ rigResult.severity }}
            </span>
            <span class="text-[11px] text-ink-muted">{{ rigResult.performanceRating }}</span>
            <span class="text-[11px] text-ink-muted font-mono truncate ml-auto">{{ rigResult.assetPath }}</span>
          </div>
          <div class="grid grid-cols-4 gap-2">
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">骨骼</div>
              <div class="text-sm font-bold font-mono text-ink">{{ rigResult.analysisKnown ? rigResult.boneCount : "—" }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">影响/顶点</div>
              <div class="text-sm font-bold font-mono text-ink">{{ rigResult.maxInfluencesPerVertex > 0 ? rigResult.maxInfluencesPerVertex : "—" }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">采样率</div>
              <div class="text-sm font-bold font-mono text-ink">{{ rigResult.overallSampleRate > 0 ? rigResult.overallSampleRate : "—" }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">轨道</div>
              <div class="text-sm font-bold font-mono text-purple">{{ rigResult.analysisKnown ? rigResult.totalTrackCount : "—" }}</div>
            </div>
          </div>
          <p v-if="rigResult.analysisKnown && rigResult.maxInfluencesPerVertex === 0" class="text-[10px] text-ink-muted">
            蒙皮权重影响数未解析，未纳入预算判定。
          </p>
          <div v-if="(rigResult.issues || []).length > 0" class="space-y-1">
            <div v-for="(issue, i) in rigResult.issues || []" :key="i" class="text-[10px] text-warning">{{ issue }}</div>
          </div>
          <p v-else class="text-[10px] text-success">未发现骨骼 / 动画超预算问题</p>
        </template>
      </div>

      <!-- Audio -->
      <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-3">
        <div class="flex items-center gap-2">
          <Activity class="w-4 h-4 text-cyan" />
          <span class="text-xs font-bold text-ink">音频规范体检</span>
        </div>
        <div class="flex items-center gap-2">
          <input
            v-model="audioPath"
            type="text"
            placeholder="音频路径，例如 Assets/Audio/ui_click.wav"
            class="flex-1 px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
          />
          <button
            @click="handleInspectAudio"
            :disabled="!audioPath.trim() || isInspectingAudio"
            class="px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-white text-xs font-semibold transition cursor-pointer disabled:opacity-40 shrink-0"
          >
            {{ isInspectingAudio ? "读取中..." : "体检" }}
          </button>
        </div>
        <template v-if="audioResult">
          <div class="grid grid-cols-4 gap-2">
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">格式</div>
              <div class="text-sm font-bold text-ink">{{ audioResult.format }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">采样率</div>
              <div
                class="text-sm font-bold font-mono"
                :class="!audioResult.metadataKnown ? 'text-ink-muted' : audioResult.isStandard48k ? 'text-success' : 'text-warning'"
              >
                {{ audioResult.metadataKnown ? audioResult.sampleRate : "未知" }}
              </div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">声道</div>
              <div class="text-xs font-bold text-ink truncate">{{ audioResult.channelStr }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">时长</div>
              <div class="text-xs font-bold text-ink truncate">{{ audioResult.durationStr }}</div>
            </div>
          </div>
          <audio v-if="audioResult.dataUri" :src="audioResult.dataUri" controls class="w-full h-8"></audio>
          <div v-if="(audioResult.warnings || []).length > 0" class="space-y-1">
            <div v-for="(warn, i) in audioResult.warnings || []" :key="i" class="text-[10px] text-warning">{{ warn }}</div>
          </div>
          <p v-else class="text-[10px]" :class="audioResult.metadataKnown ? 'text-success' : 'text-ink-muted'">{{ audioResult.message }}</p>
        </template>
      </div>

      <!-- Localization -->
      <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-3">
        <div class="flex items-center gap-2">
          <Globe class="w-4 h-4 text-success" />
          <span class="text-xs font-bold text-ink">本地化对齐体检</span>
        </div>
        <div class="flex items-center gap-2">
          <input
            v-model="fontPath"
            type="text"
            placeholder="可选：字体路径（检查缺字）"
            class="flex-1 px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
          />
          <button
            @click="handleAuditLocalization"
            :disabled="isAuditingLocalization"
            class="px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-white text-xs font-semibold transition cursor-pointer disabled:opacity-40 shrink-0"
          >
            {{ isAuditingLocalization ? "体检中..." : "开始体检" }}
          </button>
        </div>
        <template v-if="localizationResult">
          <div class="flex items-center gap-2 text-[11px]">
            <span class="px-1.5 py-0.5 rounded text-[9px] font-bold" :class="severityClass(localizationResult.severity)">
              {{ localizationResult.severity }}
            </span>
            <span class="text-ink">{{ localizationResult.languagesFound.length }} 种语言</span>
            <span class="text-ink-muted">{{ localizationResult.totalUniqueKeys }} 个键</span>
            <span class="ml-auto font-mono font-bold text-accent">{{ localizationResult.analysisKnown ? `${localizationResult.overallParityScore}%` : "—" }}</span>
          </div>
          <div class="space-y-1 max-h-28 overflow-y-auto">
            <div
              v-for="stat in localizationResult.languageStats || []"
              :key="stat.language"
              class="flex items-center gap-2 text-[10px] font-mono"
            >
              <span class="text-ink w-14 shrink-0">{{ stat.language }}</span>
              <div class="flex-1 h-1.5 rounded bg-canvas overflow-hidden">
                <div class="h-full rounded bg-accent/70" :style="{ width: `${stat.parityPercent}%` }"></div>
              </div>
              <span class="text-ink-muted w-20 text-right shrink-0">{{ stat.translatedKeys }}/{{ stat.totalKeys }}</span>
            </div>
          </div>
          <p v-if="(localizationResult.untranslatedList || []).length > 0" class="text-[10px] text-warning">
            缺失翻译 {{ (localizationResult.untranslatedList || []).length }} 项，例如
            {{ (localizationResult.untranslatedList || []).slice(0, 3).map((entry) => entry.key).join("、") }}
          </p>
          <div v-if="localizationResult.fontAudit" class="rounded-lg border border-line bg-canvas/40 p-2 space-y-1 text-[10px] font-mono">
            <div class="flex flex-wrap items-center gap-2">
              <span class="text-ink font-semibold">字体 {{ localizationResult.fontAudit.fontFormat }}</span>
              <span class="text-ink-muted">{{ localizationResult.fontAudit.totalFontGlyphs }} 字形 / 检查 {{ localizationResult.fontAudit.totalCharsChecked }} 字符</span>
              <span :class="localizationResult.fontAudit.missingGlyphsCount > 0 ? 'text-danger' : 'text-success'">
                缺字 {{ localizationResult.fontAudit.missingGlyphsCount }} · 覆盖 {{ localizationResult.fontAudit.coveragePercent }}%
              </span>
            </div>
            <div v-if="localizationResult.fontAudit.missingGlyphsCount > 0" class="text-warning break-all">
              缺失字形:
              {{ localizationResult.fontAudit.missingGlyphs.slice(0, 12).map((glyph) => `${glyph.char}(${glyph.unicodeHex})`).join(" ") }}
            </div>
          </div>
          <ul v-if="(localizationResult.advice || []).length > 0" class="text-[10px] text-warning space-y-0.5">
            <li v-for="(tip, index) in localizationResult.advice" :key="index">{{ tip }}</li>
          </ul>
          <p v-else class="text-[10px] text-success">所有语言键值完整对齐</p>
        </template>
      </div>

      <!-- Native plugins -->
      <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-3">
        <div class="flex items-center justify-between gap-2">
          <div class="flex items-center gap-2">
            <Server class="w-4 h-4 text-purple" />
            <span class="text-xs font-bold text-ink">原生插件 ABI 审计</span>
          </div>
          <button
            @click="handleAuditPlugins"
            :disabled="isAuditingPlugins"
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-50"
          >
            <Search class="w-3.5 h-3.5" :class="{ 'animate-spin': isAuditingPlugins }" />
            <span>{{ isAuditingPlugins ? "扫描中..." : "扫描插件" }}</span>
          </button>
        </div>
        <template v-if="pluginResult">
          <div class="grid grid-cols-3 gap-2">
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">插件</div>
              <div class="text-sm font-bold font-mono text-ink">{{ pluginResult.totalPluginsCount }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">合规</div>
              <div class="text-sm font-bold font-mono text-success">{{ pluginResult.compliantCount }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">缺陷</div>
              <div class="text-sm font-bold font-mono text-danger">{{ pluginResult.defectCount }}</div>
            </div>
          </div>
          <div class="space-y-1 max-h-40 overflow-y-auto">
            <div
              v-for="plugin in pluginResult.plugins || []"
              :key="plugin.relativePath"
              class="px-2 py-1 rounded bg-canvas/40 border border-line text-[10px] font-mono"
            >
              <div class="flex items-center gap-2">
                <span class="text-ink truncate flex-1" :title="plugin.relativePath">{{ plugin.pluginName || plugin.relativePath }}</span>
                <span class="text-ink-muted shrink-0">{{ plugin.targetPlatform }}</span>
                <span class="text-ink-muted shrink-0">{{ (plugin.architectures || []).join("/") }}</span>
                <span v-if="plugin.hasIssue" class="text-danger shrink-0">缺陷</span>
              </div>
              <div v-if="(plugin.issues || []).length > 0" class="text-danger truncate" :title="plugin.issues.join('；')">
                {{ (plugin.issues || []).join("；") }}
              </div>
            </div>
            <div v-if="(pluginResult.plugins || []).length === 0" class="text-[11px] text-success">未发现原生插件二进制</div>
          </div>
        </template>
      </div>

      <!-- Missing materials -->
      <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-3">
        <div class="flex items-center gap-2">
          <FileWarning class="w-4 h-4 text-danger" />
          <span class="text-xs font-bold text-ink">缺失材质 / 贴图引用</span>
        </div>
        <div class="flex items-center gap-2">
          <input
            v-model="materialTargetDir"
            type="text"
            placeholder="可选扫描目录，例如 Assets/Scenes"
            class="flex-1 px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
          />
          <button
            @click="handleScanMaterials"
            :disabled="isScanningMaterials"
            class="px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-white text-xs font-semibold transition cursor-pointer disabled:opacity-40 shrink-0"
          >
            {{ isScanningMaterials ? "扫描中..." : "开始扫描" }}
          </button>
        </div>
        <template v-if="materialResult">
          <div class="grid grid-cols-4 gap-2">
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">材质</div>
              <div class="text-sm font-bold font-mono text-ink">{{ materialResult.totalMaterialsScanned }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">损坏</div>
              <div class="text-sm font-bold font-mono text-danger">{{ materialResult.brokenMaterialsCount }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">粉红 Shader</div>
              <div class="text-sm font-bold font-mono text-warning">{{ materialResult.pinkShaderCount }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">健康度</div>
              <div class="text-sm font-bold font-mono" :class="materialResult.healthScore >= 85 ? 'text-success' : materialResult.healthScore >= 60 ? 'text-warning' : 'text-danger'">
                {{ materialResult.healthScore }}
              </div>
            </div>
          </div>
          <div class="space-y-1 max-h-40 overflow-y-auto">
            <div
              v-for="material in materialResult.brokenMaterials || []"
              :key="material.materialPath"
              class="px-2 py-1 rounded bg-canvas/40 border border-line text-[10px] font-mono"
            >
              <div class="flex items-center gap-2">
                <span class="text-ink truncate flex-1" :title="material.materialPath">{{ material.materialName || material.materialPath }}</span>
                <span class="text-danger shrink-0">{{ material.issueType }}</span>
                <span v-if="material.isPinkShader" class="text-warning shrink-0">粉红</span>
              </div>
              <div v-if="(material.missingTextures || []).length > 0" class="text-ink-muted truncate">
                缺失贴图: {{ (material.missingTextures || []).map((tex) => tex.propertySlot).join("、") }}
              </div>
            </div>
            <div v-if="(materialResult.brokenMaterials || []).length === 0" class="text-[11px] text-success">未发现损坏材质引用</div>
          </div>
          <p class="text-[10px] text-ink-muted">{{ materialResult.summary }}</p>
        </template>
      </div>

      <!-- Shader variants -->
      <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-3">
        <div class="flex items-center gap-2">
          <Activity class="w-4 h-4 text-cyan" />
          <span class="text-xs font-bold text-ink">Shader 变体审计</span>
        </div>
        <div class="flex items-center gap-2">
          <input
            v-model="shaderPath"
            type="text"
            placeholder="Shader 路径，例如 Assets/Shaders/Character.shader"
            class="flex-1 px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
          />
          <button
            @click="handleInspectShader"
            :disabled="!shaderPath.trim() || isInspectingShader"
            class="px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-white text-xs font-semibold transition cursor-pointer disabled:opacity-40 shrink-0"
          >
            {{ isInspectingShader ? "审计中..." : "审计变体" }}
          </button>
        </div>
        <template v-if="shaderResult">
          <div class="flex items-center gap-2">
            <span class="px-1.5 py-0.5 rounded text-[9px] font-bold" :class="severityClass(shaderResult.severity)">
              {{ shaderResult.severity }}
            </span>
            <span class="text-[11px] text-ink-muted font-mono truncate">{{ shaderResult.targetShader }}</span>
          </div>
          <div class="grid grid-cols-4 gap-2">
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">实际变体</div>
              <div class="text-sm font-bold font-mono" :class="shaderResult.isExplosion ? 'text-danger' : 'text-ink'">{{ shaderResult.totalVariants }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">理论上限</div>
              <div class="text-sm font-bold font-mono text-ink-muted">{{ shaderResult.theoreticalMax }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">Pass</div>
              <div class="text-sm font-bold font-mono text-purple">{{ shaderResult.passCount }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">关键字</div>
              <div class="text-sm font-bold font-mono text-ink">{{ shaderResult.totalKeywords }}</div>
            </div>
          </div>
          <div class="space-y-0.5 max-h-32 overflow-y-auto">
            <div
              v-for="offender in (shaderResult.offenderList || []).slice(0, 5)"
              :key="offender.path"
              class="flex items-center gap-2 text-[10px] font-mono"
            >
              <span class="text-ink truncate flex-1" :title="offender.path">{{ offender.path }}</span>
              <span :class="offender.severity === 'CRITICAL' ? 'text-danger' : offender.severity === 'WARNING' ? 'text-warning' : 'text-success'" class="shrink-0">
                {{ offender.variantCount }}
              </span>
            </div>
          </div>
          <p v-if="(shaderResult.strippingAdvice || []).length > 0" class="text-[10px] text-ink-muted">
            {{ shaderResult.strippingAdvice[0] }}
          </p>
        </template>
      </div>

      <!-- Project-wide audits -->
      <div class="lg:col-span-2 rounded-xl border border-line bg-surface/30 p-3 space-y-3">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <div>
            <div class="text-xs font-bold text-ink">工程级审计 (LOD / 物理 / 材质 / DCC)</div>
            <div class="text-[10px] text-ink-muted">可选限定扫描目录，留空则扫描整个仓库。</div>
          </div>
          <input
            v-model="auditScopeDir"
            type="text"
            placeholder="扫描目录（可选），例如 Assets/Characters"
            class="w-72 px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
          />
        </div>

        <div class="grid grid-cols-1 lg:grid-cols-2 gap-3">
          <!-- LOD streaming -->
          <div class="rounded-lg border border-line bg-canvas/30 p-3 space-y-2">
            <div class="flex items-center justify-between gap-2">
              <span class="text-xs font-bold text-ink">LOD 与纹理串流</span>
              <button
                @click="handleAuditLod"
                :disabled="isAuditingLod"
                class="px-2.5 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-[11px] font-semibold text-ink transition cursor-pointer disabled:opacity-50"
              >
                {{ isAuditingLod ? "审计中..." : "开始审计" }}
              </button>
            </div>
            <template v-if="lodResult">
              <div class="grid grid-cols-4 gap-1.5">
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">网格</div>
                  <div class="text-xs font-bold font-mono text-ink">{{ lodResult.totalMeshesAudited }}</div>
                </div>
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">缺 LOD</div>
                  <div class="text-xs font-bold font-mono text-warning">{{ lodResult.lodMissingCount }}</div>
                </div>
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">缺串流</div>
                  <div class="text-xs font-bold font-mono text-warning">{{ lodResult.streamingMissingCount }}</div>
                </div>
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">健康度</div>
                  <div
                    class="text-xs font-bold font-mono"
                    :class="!lodResult.analysisKnown ? 'text-ink-muted' : lodResult.overallHealthScore >= 85 ? 'text-success' : 'text-warning'"
                  >
                    {{ lodResult.analysisKnown ? lodResult.overallHealthScore : "—" }}
                  </div>
                </div>
              </div>
              <p class="text-[10px] text-ink-muted">预计显存节省 {{ lodResult.potentialSavingsMb }} MB ({{ lodResult.savingsPercentage }}%)</p>
              <p class="text-[10px] text-ink-muted">{{ lodResult.summary }}</p>
              <p
                v-if="(lodResult.lodUnknownCount || 0) + (lodResult.streamingUnknownCount || 0) > 0"
                class="text-[10px] text-warning"
              >
                {{ lodResult.lodUnknownCount }} 项网格面数未知 / {{ lodResult.streamingUnknownCount }} 项纹理尺寸未知，未计入评分
              </p>
              <div v-if="(lodResult.meshes || []).some((mesh) => mesh.severity !== 'OK')" class="space-y-0.5 max-h-24 overflow-y-auto">
                <div
                  v-for="mesh in (lodResult.meshes || []).filter((item) => item.severity !== 'OK').slice(0, 4)"
                  :key="mesh.path"
                  class="text-[10px] font-mono text-warning truncate"
                  :title="mesh.path"
                >
                  {{ mesh.path }} · {{ mesh.severity }}
                </div>
              </div>
              <p
                v-else-if="lodResult.analysisKnown && lodResult.lodMissingCount === 0 && lodResult.streamingMissingCount === 0 && (lodResult.lodUnknownCount || 0) + (lodResult.streamingUnknownCount || 0) === 0"
                class="text-[10px] text-success"
              >LOD 与串流配置完整</p>
            </template>
          </div>

          <!-- Physics collision -->
          <div class="rounded-lg border border-line bg-canvas/30 p-3 space-y-2">
            <div class="flex items-center justify-between gap-2">
              <span class="text-xs font-bold text-ink">物理碰撞体</span>
              <button
                @click="handleAuditPhysics"
                :disabled="isAuditingPhysics"
                class="px-2.5 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-[11px] font-semibold text-ink transition cursor-pointer disabled:opacity-50"
              >
                {{ isAuditingPhysics ? "审计中..." : "开始审计" }}
              </button>
            </div>
            <template v-if="physicsResult">
              <div class="grid grid-cols-4 gap-1.5">
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">碰撞体</div>
                  <div class="text-xs font-bold font-mono text-ink">{{ physicsResult.totalColliders }}</div>
                </div>
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">凸包</div>
                  <div class="text-xs font-bold font-mono text-success">{{ physicsResult.convexHullsCount }}</div>
                </div>
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">凹网格</div>
                  <div class="text-xs font-bold font-mono text-warning">{{ physicsResult.concaveMeshCount }}</div>
                </div>
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">健康度</div>
                  <div
                    class="text-xs font-bold font-mono"
                    :class="!physicsResult.analysisKnown ? 'text-ink-muted' : physicsResult.healthScore >= 85 ? 'text-success' : 'text-warning'"
                  >
                    {{ physicsResult.analysisKnown ? physicsResult.healthScore : "—" }}
                  </div>
                </div>
              </div>
              <div v-if="(physicsResult.overbudgetHulls || []).length > 0" class="space-y-0.5 max-h-20 overflow-y-auto">
                <div
                  v-for="hull in (physicsResult.overbudgetHulls || []).slice(0, 4)"
                  :key="hull.meshPath"
                  class="text-[10px] font-mono text-danger truncate"
                  :title="hull.meshPath"
                >
                  {{ hull.meshPath }} · {{ hull.vertexCount }} 顶点
                </div>
              </div>
              <p v-else class="text-[10px] text-success">未发现超预算凸包</p>
              <p v-if="(physicsResult.nonConvexDynamicColliders || []).length > 0" class="text-[10px] text-warning">
                动态刚体非凸碰撞 {{ physicsResult.nonConvexDynamicColliders.length }} 处
              </p>
              <p v-if="(physicsResult.missingPhysicMaterials || []).length > 0" class="text-[10px] text-warning">
                缺失物理材质 {{ physicsResult.missingPhysicMaterials.length }} 处
              </p>
              <p class="text-[10px] text-ink-muted">{{ physicsResult.summary }}</p>
            </template>
          </div>

          <!-- Material overrides -->
          <div class="rounded-lg border border-line bg-canvas/30 p-3 space-y-2">
            <div class="flex items-center justify-between gap-2">
              <span class="text-xs font-bold text-ink">材质参数覆盖</span>
              <button
                @click="handleAuditMaterialOverrides"
                :disabled="isAuditingMaterialOverrides"
                class="px-2.5 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-[11px] font-semibold text-ink transition cursor-pointer disabled:opacity-50"
              >
                {{ isAuditingMaterialOverrides ? "审计中..." : "开始审计" }}
              </button>
            </div>
            <template v-if="materialOverrideResult">
              <div class="grid grid-cols-3 gap-1.5">
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">材质</div>
                  <div class="text-xs font-bold font-mono text-ink">{{ materialOverrideResult.totalInstances }}</div>
                </div>
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">覆盖参数</div>
                  <div class="text-xs font-bold font-mono text-purple">{{ materialOverrideResult.overriddenParametersCount }}</div>
                </div>
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">失效参数</div>
                  <div class="text-xs font-bold font-mono text-danger">{{ materialOverrideResult.deadParametersCount }}</div>
                </div>
              </div>
              <div class="space-y-0.5 max-h-24 overflow-y-auto">
                <div
                  v-for="instance in (materialOverrideResult.instances || []).slice(0, 5)"
                  :key="instance.instancePath"
                  class="flex items-center gap-2 text-[10px] font-mono"
                >
                  <span class="text-ink truncate flex-1" :title="instance.instancePath">{{ instance.instancePath }}</span>
                  <span :class="instance.deadCount > 0 ? 'text-danger' : 'text-success'" class="shrink-0">
                    {{ instance.deadCount > 0 ? `${instance.deadCount} 失效` : "正常" }}
                  </span>
                </div>
                <div v-if="(materialOverrideResult.instances || []).length === 0" class="text-[10px] text-success">未发现可解析的 Unity 材质</div>
              </div>
              <p class="text-[10px] text-ink-muted">{{ materialOverrideResult.summary }}</p>
            </template>
          </div>

          <!-- DCC locks -->
          <div class="rounded-lg border border-line bg-canvas/30 p-3 space-y-2">
            <div class="flex items-center justify-between gap-2">
              <span class="text-xs font-bold text-ink">DCC 临时锁巡检</span>
              <button
                @click="handleDetectDcc"
                :disabled="isDetectingDcc"
                class="px-2.5 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-[11px] font-semibold text-ink transition cursor-pointer disabled:opacity-50"
              >
                {{ isDetectingDcc ? "巡检中..." : "开始巡检" }}
              </button>
            </div>
            <div class="flex items-center gap-1.5">
              <input
                v-model="dccAssetPath"
                type="text"
                placeholder="自动上锁资产路径，例如 Assets/hero.psd"
                @keydown.enter="handleAutoLockDcc"
                class="flex-1 px-2 py-1 rounded bg-canvas border border-line text-[10px] text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
              />
              <button
                @click="handleAutoLockDcc"
                :disabled="!dccAssetPath.trim() || isAutoLockingDcc"
                class="px-2 py-1 rounded bg-surface hover:bg-raised border border-line text-[10px] font-semibold text-warning transition cursor-pointer disabled:opacity-40 shrink-0"
              >
                {{ isAutoLockingDcc ? "上锁中..." : "自动上锁" }}
              </button>
            </div>
            <template v-if="dccResult">
              <div class="grid grid-cols-4 gap-1.5">
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">DCC 文件</div>
                  <div class="text-xs font-bold font-mono text-ink">{{ dccResult.totalDccFiles }}</div>
                </div>
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">活动锁</div>
                  <div class="text-xs font-bold font-mono text-warning">{{ dccResult.activeLocksCount }}</div>
                </div>
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">未保护</div>
                  <div class="text-xs font-bold font-mono text-ink-muted">{{ dccResult.unprotectedCount }}</div>
                </div>
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">冲突</div>
                  <div class="text-xs font-bold font-mono text-danger">{{ dccResult.collisionCount }}</div>
                </div>
              </div>
              <p v-if="(dccResult.runningProcesses || []).length > 0" class="text-[10px] text-ink-muted truncate">
                运行中: {{ (dccResult.runningProcesses || []).map((proc) => proc.dccTool || proc.processName).join("、") }}
              </p>
              <div class="space-y-0.5 max-h-24 overflow-y-auto">
                <div
                  v-for="item in (dccResult.items || []).slice(0, 5)"
                  :key="item.lockFilePath"
                  class="flex items-center gap-2 text-[10px] font-mono"
                >
                  <span class="text-ink truncate flex-1" :title="item.lockFilePath">{{ item.lockFilePath }}</span>
                  <span class="text-ink-muted shrink-0">{{ item.dccTool }}</span>
                  <span v-if="item.collisionRisk" class="text-danger shrink-0">冲突</span>
                </div>
                <div v-if="(dccResult.items || []).length === 0" class="text-[10px] text-success">未发现 DCC 临时锁文件</div>
              </div>
              <p class="text-[10px] text-ink-muted">{{ dccResult.summary }}</p>
            </template>
          </div>

          <!-- Symbols & defines -->
          <div class="rounded-lg border border-line bg-canvas/30 p-3 space-y-2">
            <div class="flex items-center justify-between gap-2">
              <span class="text-xs font-bold text-ink">宏定义与调试开关</span>
              <button
                @click="handleAuditSymbols"
                :disabled="isAuditingSymbols"
                class="px-2.5 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-[11px] font-semibold text-ink transition cursor-pointer disabled:opacity-50"
              >
                {{ isAuditingSymbols ? "审计中..." : "开始审计" }}
              </button>
            </div>
            <div class="flex items-center gap-1.5">
              <input
                v-model="symbolsBase"
                type="text"
                placeholder="基线引用 HEAD~1"
                class="w-24 px-2 py-1 rounded bg-canvas border border-line text-[10px] text-ink font-mono focus:outline-none focus:border-accent"
              />
              <span class="text-[10px] text-ink-muted">→</span>
              <input
                v-model="symbolsTarget"
                type="text"
                placeholder="目标引用 HEAD"
                class="w-24 px-2 py-1 rounded bg-canvas border border-line text-[10px] text-ink font-mono focus:outline-none focus:border-accent"
              />
            </div>
            <template v-if="symbolsResult">
              <div class="flex items-center gap-2 text-[10px]">
                <span class="px-1.5 py-0.5 rounded text-[9px] font-bold" :class="severityClass(symbolsResult.severity)">
                  {{ symbolsResult.severity }}
                </span>
                <span class="text-ink-muted">共 {{ symbolsResult.totalDefines }} 个宏</span>
                <span class="text-danger ml-auto">{{ symbolsResult.dangerousDebugDefines.length }} 个危险调试宏</span>
              </div>
              <div v-if="symbolsResult.dangerousDebugDefines.length > 0" class="space-y-0.5">
                <div
                  v-for="define in symbolsResult.dangerousDebugDefines"
                  :key="define"
                  class="text-[10px] text-danger font-mono truncate"
                  :title="define"
                >
                  {{ define }}
                </div>
              </div>
              <p v-else-if="symbolsResult.analysisKnown" class="text-[10px] text-success">未发现发布分支危险调试宏</p>
              <p v-else class="text-[10px] text-ink-muted">{{ symbolsResult.summary }}</p>
              <div class="flex flex-wrap gap-1 text-[9px] font-mono">
                <span
                  v-for="define in (symbolsResult.addedDefines || []).slice(0, 4)"
                  :key="`add-${define}`"
                  class="px-1 py-0.2 rounded bg-success/10 text-success"
                >
                  +{{ define }}
                </span>
                <span
                  v-for="define in (symbolsResult.removedDefines || []).slice(0, 4)"
                  :key="`del-${define}`"
                  class="px-1 py-0.2 rounded bg-danger/10 text-danger"
                >
                  -{{ define }}
                </span>
              </div>
            </template>
          </div>

          <!-- Data table semantic diff -->
          <div class="rounded-lg border border-line bg-canvas/30 p-3 space-y-2">
            <div class="flex items-center justify-between gap-2">
              <span class="text-xs font-bold text-ink">数据表语义差异</span>
              <button
                @click="handleAuditDataTables"
                :disabled="isAuditingTables"
                class="px-2.5 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-[11px] font-semibold text-ink transition cursor-pointer disabled:opacity-50"
              >
                {{ isAuditingTables ? "审计中..." : "开始审计" }}
              </button>
            </div>
            <input
              v-model="dataTablePath"
              type="text"
              placeholder="数据表路径（可选，留空扫描变更表）"
              class="w-full px-2 py-1 rounded bg-canvas border border-line text-[10px] text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
            />
            <div class="flex items-center gap-1.5">
              <input
                v-model="dataTableBase"
                type="text"
                placeholder="基线 HEAD"
                class="w-24 px-2 py-1 rounded bg-canvas border border-line text-[10px] text-ink font-mono focus:outline-none focus:border-accent"
              />
              <span class="text-[10px] text-ink-muted">→</span>
              <input
                v-model="dataTableTarget"
                type="text"
                placeholder="目标 WORKTREE"
                class="w-28 px-2 py-1 rounded bg-canvas border border-line text-[10px] text-ink font-mono focus:outline-none focus:border-accent"
              />
            </div>
            <template v-if="dataTableResult">
              <div class="grid grid-cols-3 gap-1.5">
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">数据表</div>
                  <div class="text-xs font-bold font-mono text-ink">{{ dataTableResult.totalTables }}</div>
                </div>
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">严重</div>
                  <div class="text-xs font-bold font-mono text-danger">{{ dataTableResult.criticalCount }}</div>
                </div>
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">警告</div>
                  <div class="text-xs font-bold font-mono text-warning">{{ dataTableResult.warningCount }}</div>
                </div>
              </div>
              <div class="space-y-1 max-h-32 overflow-y-auto">
                <div
                  v-for="report in dataTableResult.auditedTables || []"
                  :key="report.filePath"
                  class="px-2 py-1 rounded bg-surface/50 border border-line text-[10px] font-mono"
                >
                  <div class="flex items-center gap-2">
                    <span class="text-ink truncate flex-1" :title="report.filePath">{{ report.tableName || report.filePath }}</span>
                    <span class="text-success shrink-0">+{{ report.addedRowsCount }}</span>
                    <span class="text-warning shrink-0">~{{ report.modifiedRowsCount }}</span>
                    <span class="text-danger shrink-0">-{{ report.deletedRowsCount }}</span>
                  </div>
                  <div class="text-ink-muted truncate" :title="report.semanticSummary">
                    {{ (report.schemaChanges || []).length }} 项结构变更 · {{ (report.anomalies || []).length }} 项异常
                  </div>
                </div>
                <div v-if="(dataTableResult.auditedTables || []).length === 0" class="text-[10px] text-ink-muted">未发现可审计的数据表</div>
              </div>
              <p class="text-[10px] text-ink-muted">{{ dataTableResult.overallSummary }}</p>
            </template>
          </div>

          <!-- Cross-branch asset cherry-pick -->
          <div class="rounded-lg border border-line bg-canvas/30 p-3 space-y-2">
            <div class="flex items-center justify-between gap-2">
              <span class="text-xs font-bold text-ink">跨分支资产拣选</span>
              <span class="text-[9px] text-ink-muted font-mono">含 .meta / 依赖</span>
            </div>
            <input
              v-model="assetPickTargetRef"
              type="text"
              placeholder="来源引用，例如 feature/art 或 v1.2.0"
              class="w-full px-2 py-1 rounded bg-canvas border border-line text-[10px] text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
            />
            <textarea
              v-model="assetPickPaths"
              rows="2"
              placeholder="资产路径，逗号或换行分隔，例如 Assets/Hero.fbx, Assets/Hero.png"
              class="w-full px-2 py-1 rounded bg-canvas border border-line text-[10px] text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent resize-y"
            ></textarea>
            <div class="flex flex-wrap items-center gap-3 text-[10px] text-ink-muted">
              <label class="flex items-center gap-1 cursor-pointer">
                <input v-model="assetPickResolveDeps" type="checkbox" class="rounded accent-accent scale-90" />
                <span>包含依赖</span>
              </label>
              <label class="flex items-center gap-1 cursor-pointer">
                <input v-model="assetPickDryRun" type="checkbox" class="rounded accent-accent scale-90" />
                <span>仅预览</span>
              </label>
              <button
                @click="handlePickAssets"
                :disabled="isPickingAssets || !assetPickTargetRef.trim() || !assetPickPaths.trim()"
                class="ml-auto px-2.5 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-[11px] font-semibold text-accent transition cursor-pointer disabled:opacity-40"
              >
                {{ isPickingAssets ? "处理中..." : assetPickDryRun ? "预览拣选" : "执行拣选" }}
              </button>
            </div>
            <template v-if="assetPickResult">
              <div class="grid grid-cols-4 gap-1.5">
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">请求</div>
                  <div class="text-xs font-bold font-mono text-ink">{{ assetPickResult.totalRequested }}</div>
                </div>
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">依赖</div>
                  <div class="text-xs font-bold font-mono text-purple">{{ assetPickResult.totalDependencies }}</div>
                </div>
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">总计</div>
                  <div class="text-xs font-bold font-mono text-ink">{{ assetPickResult.totalAssetsCount }}</div>
                </div>
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">体积</div>
                  <div class="text-xs font-bold font-mono text-ink">{{ assetPickResult.totalBytesStr }}</div>
                </div>
              </div>
              <div class="space-y-0.5 max-h-28 overflow-y-auto">
                <div
                  v-for="asset in (assetPickResult.assets || []).slice(0, 8)"
                  :key="asset.path"
                  class="flex items-center gap-2 text-[10px] font-mono"
                >
                  <span class="text-ink truncate flex-1" :title="asset.path">{{ asset.path }}</span>
                  <span v-if="asset.isDependency" class="text-purple shrink-0">依赖</span>
                  <span v-if="asset.isLfs" class="text-warning shrink-0">LFS</span>
                  <span class="text-ink-muted shrink-0">{{ asset.status }}</span>
                </div>
              </div>
              <p class="text-[10px] text-ink-muted">{{ assetPickResult.summary }}</p>
            </template>
          </div>

          <!-- Unreal asset node diff -->
          <div class="rounded-lg border border-line bg-canvas/30 p-3 space-y-2">
            <div class="flex items-center justify-between gap-2">
              <span class="text-xs font-bold text-ink">Unreal 资产节点差异</span>
              <span class="text-[9px] text-ink-muted font-mono">UAsset / Blueprint</span>
            </div>
            <input
              v-model="unrealAssetPath"
              type="text"
              placeholder="资产路径，例如 Content/BP_Hero.uasset"
              class="w-full px-2 py-1 rounded bg-canvas border border-line text-[10px] text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
            />
            <div class="flex items-center gap-1.5">
              <input
                v-model="unrealBaseRef"
                type="text"
                placeholder="基线 HEAD"
                class="w-24 px-2 py-1 rounded bg-canvas border border-line text-[10px] text-ink font-mono focus:outline-none focus:border-accent"
              />
              <span class="text-[10px] text-ink-muted">→</span>
              <input
                v-model="unrealTargetRef"
                type="text"
                placeholder="目标 WORKTREE"
                class="w-28 px-2 py-1 rounded bg-canvas border border-line text-[10px] text-ink font-mono focus:outline-none focus:border-accent"
              />
              <button
                @click="handleDiffUnrealAsset"
                :disabled="!unrealAssetPath.trim() || isDiffingUnreal"
                class="ml-auto px-2.5 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-[11px] font-semibold text-accent transition cursor-pointer disabled:opacity-40"
              >
                {{ isDiffingUnreal ? "对比中..." : "对比节点" }}
              </button>
            </div>
            <template v-if="unrealDiffResult">
              <div class="grid grid-cols-4 gap-1.5">
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-success">{{ unrealDiffResult.analysisKnown ? "新增" : "候选新增" }}</div>
                  <div class="text-xs font-bold font-mono text-success">{{ (unrealDiffResult.addedNodes || []).length }}</div>
                </div>
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-danger">{{ unrealDiffResult.analysisKnown ? "删除" : "候选删除" }}</div>
                  <div class="text-xs font-bold font-mono text-danger">{{ (unrealDiffResult.removedNodes || []).length }}</div>
                </div>
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-warning">{{ unrealDiffResult.analysisKnown ? "变更" : "候选变更" }}</div>
                  <div class="text-xs font-bold font-mono text-warning">{{ (unrealDiffResult.modifiedNodes || []).length }}</div>
                </div>
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">{{ unrealDiffResult.analysisKnown ? "未变" : "候选未变" }}</div>
                  <div class="text-xs font-bold font-mono text-ink-muted">{{ unrealDiffResult.unchangedCount }}</div>
                </div>
              </div>
              <div class="flex flex-wrap items-center gap-x-2 gap-y-0.5 text-[10px] font-mono text-ink-muted">
                <span>格式 {{ unrealDiffResult.assetFormat }}</span>
                <span v-if="unrealDiffResult.targetHeader">引擎 {{ unrealDiffResult.targetHeader.engineVersion }}</span>
                <span v-if="unrealDiffResult.targetHeader">Magic {{ unrealDiffResult.targetHeader.magic }}</span>
                <span>节点 {{ unrealDiffResult.totalBaseNodes }} → {{ unrealDiffResult.totalTargetNodes }}</span>
              </div>
              <p class="text-[10px] text-ink-muted">{{ unrealDiffResult.visualSummary }}</p>
              <p v-if="!unrealDiffResult.analysisKnown" class="text-[10px] text-warning">{{ unrealDiffResult.analysisNote }}</p>
              <div class="space-y-0.5 max-h-24 overflow-y-auto">
                <div
                  v-for="node in (unrealDiffResult.modifiedNodes || []).slice(0, 5)"
                  :key="node.nodeId"
                  class="text-[10px] font-mono text-warning truncate"
                  :title="node.summary"
                >
                  {{ node.nodeName }} · {{ node.summary }}
                </div>
              </div>
            </template>
          </div>

          <!-- Scene stream merge audit -->
          <div class="rounded-lg border border-line bg-canvas/30 p-3 space-y-2">
            <div class="flex items-center justify-between gap-2">
              <span class="text-xs font-bold text-ink">场景流式合并审计</span>
              <button
                @click="handleAuditSceneStream"
                :disabled="isAuditingScene"
                class="px-2.5 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-[11px] font-semibold text-ink transition cursor-pointer disabled:opacity-50"
              >
                {{ isAuditingScene ? "审计中..." : "开始审计" }}
              </button>
            </div>
            <div class="flex items-center gap-1.5">
              <input
                v-model="sceneTargetBranch"
                type="text"
                placeholder="目标分支 main"
                class="w-28 px-2 py-1 rounded bg-canvas border border-line text-[10px] text-ink font-mono focus:outline-none focus:border-accent"
              />
              <input
                v-model="sceneAuditPath"
                type="text"
                placeholder="关卡路径（可选）"
                class="flex-1 px-2 py-1 rounded bg-canvas border border-line text-[10px] text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
              />
            </div>
            <template v-if="sceneAuditResult">
              <div class="grid grid-cols-4 gap-1.5">
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">安全分</div>
                  <div
                    class="text-xs font-bold font-mono"
                    :class="!sceneAuditResult.analysisKnown ? 'text-ink-muted' : sceneAuditResult.safetyScore >= 85 ? 'text-success' : sceneAuditResult.safetyScore >= 60 ? 'text-warning' : 'text-danger'"
                  >
                    {{ sceneAuditResult.analysisKnown ? sceneAuditResult.safetyScore : "—" }}
                  </div>
                </div>
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">GUID 冲突</div>
                  <div class="text-xs font-bold font-mono text-danger">{{ (sceneAuditResult.guidCollisions || []).length }}</div>
                </div>
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">空间重叠</div>
                  <div class="text-xs font-bold font-mono text-warning">{{ (sceneAuditResult.spatialOverlaps || []).length }}</div>
                </div>
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">变换冲突</div>
                  <div class="text-xs font-bold font-mono text-warning">{{ (sceneAuditResult.transformConflicts || []).length }}</div>
                </div>
              </div>
              <p
                class="text-[10px] font-mono"
                :class="!sceneAuditResult.analysisKnown ? 'text-ink-muted' : sceneAuditResult.safetyLevel === 'SAFE' ? 'text-success' : 'text-warning'"
              >
                {{ sceneAuditResult.safetyLevel }} · {{ sceneAuditResult.mergeVerdict }}
              </p>
              <p class="text-[10px] text-ink-muted">{{ sceneAuditResult.summaryMessage }}</p>
              <div class="space-y-0.5 max-h-24 overflow-y-auto">
                <div
                  v-for="item in (sceneAuditResult.recommendedPlan || []).slice(0, 4)"
                  :key="`${item.category}:${item.targetId}`"
                  class="text-[10px] font-mono truncate"
                  :class="item.automated ? 'text-success' : 'text-ink-muted'"
                  :title="item.description"
                >
                  [{{ item.category }}] {{ item.action }} · {{ item.description }}
                </div>
              </div>
            </template>
          </div>

          <!-- Scene conflict resolve -->
          <div class="rounded-lg border border-line bg-canvas/30 p-3 space-y-2">
            <div class="flex items-center justify-between gap-2">
              <span class="text-xs font-bold text-ink">场景 Actor GUID 修复</span>
              <span class="text-[9px] text-warning font-bold">会修改关卡文件</span>
            </div>
            <input
              v-model="resolveScenePath"
              type="text"
              placeholder="关卡路径，例如 Assets/Scenes/Main.unity"
              class="w-full px-2 py-1 rounded bg-canvas border border-line text-[10px] text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
            />
            <div class="flex items-center gap-1.5">
              <input
                v-model="resolveSceneGuid"
                type="text"
                placeholder="冲突 Actor GUID"
                class="flex-1 px-2 py-1 rounded bg-canvas border border-line text-[10px] text-ink font-mono focus:outline-none focus:border-accent"
              />
              <select
                v-model="resolveSceneAction"
                aria-label="场景冲突处理方式"
                class="px-2 py-1 rounded bg-canvas border border-line text-[10px] text-ink focus:outline-none focus:border-accent cursor-pointer"
              >
                <option value="AUTO_REGENERATE_GUID">重新生成 GUID</option>
                <option value="SPATIAL_OFFSET">空间偏移（Unity）</option>
              </select>
              <button
                @click="handleResolveSceneConflict"
                :disabled="!resolveScenePath.trim() || !resolveSceneGuid.trim() || isResolvingScene"
                class="px-2.5 py-1 rounded-lg bg-danger-strong hover:opacity-90 text-white text-[11px] font-semibold transition cursor-pointer disabled:opacity-40 shrink-0"
              >
                {{ isResolvingScene ? "修复中..." : "应用修复" }}
              </button>
            </div>
            <p v-if="resolveSceneResult" class="text-[10px] text-ink-muted font-mono">
              {{ resolveSceneResult.message }}
            </p>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
