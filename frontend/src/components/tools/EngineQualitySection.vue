<script setup lang="ts">
import { t } from "../../i18n";
import { confirmAction } from "../../composables/useConfirm";
import { ref } from "vue";
import { Activity, Box, Copy, FileWarning, Globe, Search, Server } from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import { buildAuditPlan } from "../../utils/auditPlan";
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
    notify(t("eq.toast.scene.audit.failed", { error: err?.message || err }));
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
      t("eq.confirm.scene.fix", { scene: resolveScenePath.value.trim(), action: resolveSceneAction.value })
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
    notify(t("eq.toast.scene.fix.failed", { error: err?.message || err }));
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
      t("eq.confirm.asset.pick", { count: paths.length, from: assetPickTargetRef.value.trim(), deps: assetPickResolveDeps.value ? t("eq.asset.pick.deps.suffix") : "" })
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
    notify(t("eq.toast.asset.pick.failed", { error: err?.message || err }));
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
    notify(t("eq.toast.unreal.diff.failed", { error: err?.message || err }));
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
          ? t("eq.toast.datatable.done", { tables: dataTableResult.value.totalTables, critical: dataTableResult.value.criticalCount, warn: dataTableResult.value.warningCount })
          : t("eq.toast.datatable.none")
      );
    }
  } catch (err: any) {
    notify(t("eq.toast.datatable.failed", { error: err?.message || err }));
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
        t("eq.toast.shader.done", { shader: shaderResult.value.targetShader, variants: shaderResult.value.totalVariants, severity: shaderResult.value.severity })
      );
    }
  } catch (err: any) {
    notify(t("eq.toast.shader.failed", { error: err?.message || err }));
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
          ? t("eq.toast.symbols.done", { total: symbolsResult.value.totalDefines, dangerous: symbolsResult.value.dangerousDebugDefines.length })
          : t("eq.toast.symbols.none")
      );
    }
  } catch (err: any) {
    notify(t("eq.toast.symbols.failed", { error: err?.message || err }));
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
      notify(res.message || t("eq.toast.dcc.locked", { path: dccAssetPath.value.trim() }));
      dccAssetPath.value = "";
    } else {
      notify(res?.message || t("eq.toast.dcc.lock.failed.short"));
    }
  } catch (err: any) {
    notify(t("eq.toast.dcc.lock.failed", { error: err?.message || err }));
  } finally {
    isAutoLockingDcc.value = false;
  }
}

async function handleQuickLockDcc(path: string) {
  dccAssetPath.value = path;
  await handleAutoLockDcc();
}

async function copyDccPath(path: string) {
  if (!path) return;
  try {
    await navigator.clipboard.writeText(path);
    notify(t("eq.toast.dcc.path.copied"));
  } catch {
    notify(t("eq.toast.copy.failed"));
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
          ? t("eq.toast.lod.done", { score: lodResult.value.overallHealthScore, lod: lodResult.value.lodMissingCount, streaming: lodResult.value.streamingMissingCount })
          : t("eq.toast.lod.none")
      );
    }
  } catch (err: any) {
    notify(t("eq.toast.lod.failed", { error: err?.message || err }));
  } finally {
    isAuditingLod.value = false;
  }
}

/**
 * Exports this audit as a Markdown brief.
 *
 * The five engine-asset audits cannot fix what they find — they scan by
 * extension and size without parsing .meta, so there is no defensible target
 * value to write. Handing the user a description they can act on outside the
 * app is the useful half of the job.
 */
async function exportLodPlan() {
  const r = lodResult.value;
  if (!r) return;
  const markdown = buildAuditPlan({
    title: t("eq.lod.title"),
    summary: r.summary || "",
    advice: r.recommendations || [],
    counts: {
      [t("eq.lod.meshes")]: r.totalMeshesAudited,
      [t("eq.lod.missing")]: r.lodMissingCount,
      [t("eq.lod.streaming")]: r.streamingMissingCount,
    },
    raw: r,
    scope: auditScopeDir.value.trim() || ".",
  });
  try {
    await navigator.clipboard.writeText(markdown);
    notify(t("eq.audit.plan.copied"));
  } catch (err: any) {
    notify(t("eq.audit.plan.copy.failed", { error: err?.message || err }));
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
          ? t("eq.toast.physics.done", { colliders: physicsResult.value.totalColliders, score: physicsResult.value.healthScore })
          : t("eq.toast.physics.none")
      );
    }
  } catch (err: any) {
    notify(t("eq.toast.physics.failed", { error: err?.message || err }));
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
        t("eq.toast.material.done", { instances: materialOverrideResult.value.totalInstances, dead: materialOverrideResult.value.deadParametersCount })
      );
    }
  } catch (err: any) {
    notify(t("eq.toast.material.failed", { error: err?.message || err }));
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
        t("eq.toast.dcc.audit.done", { locks: dccResult.value.activeLocksCount, conflicts: dccResult.value.collisionCount })
      );
    }
  } catch (err: any) {
    notify(t("eq.toast.dcc.audit.failed", { error: err?.message || err }));
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
        t("eq.toast.mesh.done", { path: meshResult.value.path, triangles: meshResult.value.triangleCountKnown ? t("eq.mesh.triangles", { count: meshResult.value.triangleCount }) : t("eq.mesh.triangles.unknown") })
      );
    }
  } catch (err: any) {
    notify(t("eq.toast.mesh.failed", { error: err?.message || err }));
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
        t("eq.toast.rig.done", { path: rigResult.value.assetPath, bones: rigResult.value.boneCount, rating: rigResult.value.performanceRating })
      );
    }
  } catch (err: any) {
    notify(t("eq.toast.rig.failed", { error: err?.message || err }));
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
    notify(t("eq.toast.audio.failed", { error: err?.message || err }));
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
          ? t("eq.toast.l10n.done", { languages: localizationResult.value.languagesFound.length, score: localizationResult.value.overallParityScore })
          : t("eq.toast.l10n.none")
      );
    }
  } catch (err: any) {
    notify(t("eq.toast.l10n.failed", { error: err?.message || err }));
  } finally {
    isAuditingLocalization.value = false;
  }
}

/** Renders the missing keys as a paste-ready snippet and copies it. */
async function handleCopyLocalizationPatch() {
  if (!props.currentRepoPath || isCopyingPatch.value) return;
  isCopyingPatch.value = true;
  try {
    // Empty target language: the backend picks the first one that has gaps,
    // which beats asking the user to recall an exact tag before seeing output.
    const patch = await gitService.generateLocalizationKeyPatch(props.currentRepoPath, "", "json");
    if (!patch.content) {
      notify(patch.message || t("eq.l10n.patch.none"));
      return;
    }
    await navigator.clipboard.writeText(patch.content);
    notify(t("eq.l10n.patch.copied", { count: patch.missingCount, lang: patch.targetLanguage }));
  } catch (err: any) {
    notify(t("eq.l10n.patch.failed", { error: err?.message || err }));
  } finally {
    isCopyingPatch.value = false;
  }
}

const isCopyingPatch = ref(false);

async function handleAuditPlugins() {
  if (!props.currentRepoPath || isAuditingPlugins.value) return;
  isAuditingPlugins.value = true;
  try {
    pluginResult.value = await gitService.auditNativePluginsAbi(props.currentRepoPath);
    if (pluginResult.value) {
      notify(
        t("eq.toast.plugin.done", { total: pluginResult.value.totalPluginsCount, defects: pluginResult.value.defectCount })
      );
    }
  } catch (err: any) {
    notify(t("eq.toast.plugin.failed", { error: err?.message || err }));
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
    notify(t("eq.toast.material.scan.failed", { error: err?.message || err }));
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
        <span>{{ t("eq.title") }}</span>
      </h3>
      <p class="text-xs text-ink-muted mt-0.5">
        {{ t("eq.desc") }}
      </p>
    </div>

    <div class="grid grid-cols-1 lg:grid-cols-2 gap-3">
      <!-- Mesh -->
      <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-3">
        <div class="flex items-center gap-2">
          <Box class="w-4 h-4 text-purple" />
          <span class="text-xs font-bold text-ink">{{ t("eq.mesh.title") }}</span>
        </div>
        <div class="flex items-center gap-2">
          <input
            v-model="meshPath"
            type="text"
            :placeholder="t('eq.mesh.placeholder')"
            class="flex-1 px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
          />
          <select
            v-model="meshPreset"
            :aria-label="t('eq.mesh.preset')"
            class="px-2 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink focus:outline-none focus:border-accent cursor-pointer"
          >
            <option value="mobile_prop">{{ t("eq.mesh.preset.mobile.prop") }}</option>
            <option value="mobile_character">{{ t("eq.mesh.preset.mobile.character") }}</option>
            <option value="pc_character">{{ t("eq.mesh.preset.pc.character") }}</option>
            <option value="pc_environment">{{ t("eq.mesh.preset.pc.env") }}</option>
          </select>
          <button
            @click="handleLintMesh"
            :disabled="!meshPath.trim() || isLintingMesh"
            class="px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-xs font-semibold transition cursor-pointer disabled:opacity-40 shrink-0"
          >
            {{ isLintingMesh ? t("eq.checking") : t("eq.check") }}
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
              <div class="text-[10px] text-ink-muted">{{ t("eq.mesh.triangles.label") }}</div>
              <div class="text-sm font-bold font-mono" :class="meshResult.isOverBudget ? 'text-danger' : 'text-ink'">
                {{ meshResult.triangleCountKnown ? meshResult.triangleCount : t("eq.unknown") }}
              </div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">{{ t("eq.mesh.vertices") }}</div>
              <div class="text-sm font-bold font-mono text-ink">{{ meshResult.triangleCountKnown ? meshResult.vertexCount : t("eq.unknown") }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">{{ t("eq.mesh.materials") }}</div>
              <div class="text-sm font-bold font-mono text-purple">{{ meshResult.triangleCountKnown ? meshResult.materialCount : "—" }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">{{ t("eq.mesh.budget") }}</div>
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
          <span class="text-xs font-bold text-ink">{{ t("eq.rig.title") }}</span>
        </div>
        <div class="flex items-center gap-2">
          <input
            v-model="rigPath"
            type="text"
            :placeholder="t('eq.rig.placeholder')"
            class="flex-1 px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
          />
          <select
            v-model="rigPreset"
            :aria-label="t('eq.rig.preset')"
            class="px-2 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink focus:outline-none focus:border-accent cursor-pointer"
          >
            <option value="Mobile">Mobile</option>
            <option value="Console">Console</option>
            <option value="Cinematic">Cinematic</option>
          </select>
          <button
            @click="handleLintRig"
            :disabled="!rigPath.trim() || isLintingRig"
            class="px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-xs font-semibold transition cursor-pointer disabled:opacity-40 shrink-0"
          >
            {{ isLintingRig ? t("eq.checking") : t("eq.check") }}
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
              <div class="text-[10px] text-ink-muted">{{ t("eq.rig.bones") }}</div>
              <div class="text-sm font-bold font-mono text-ink">{{ rigResult.analysisKnown ? rigResult.boneCount : "—" }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">{{ t("eq.rig.influences") }}</div>
              <div class="text-sm font-bold font-mono text-ink">{{ rigResult.maxInfluencesPerVertex > 0 ? rigResult.maxInfluencesPerVertex : "—" }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">{{ t("eq.rig.sample.rate") }}</div>
              <div class="text-sm font-bold font-mono text-ink">{{ rigResult.overallSampleRate > 0 ? rigResult.overallSampleRate : "—" }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">{{ t("eq.rig.tracks") }}</div>
              <div class="text-sm font-bold font-mono text-purple">{{ rigResult.analysisKnown ? rigResult.totalTrackCount : "—" }}</div>
            </div>
          </div>
          <p v-if="rigResult.analysisKnown && rigResult.maxInfluencesPerVertex === 0" class="text-[10px] text-ink-muted">
            {{ t("eq.rig.influences.unknown") }}
          </p>
          <div v-if="(rigResult.issues || []).length > 0" class="space-y-1">
            <div v-for="(issue, i) in rigResult.issues || []" :key="i" class="text-[10px] text-warning">{{ issue }}</div>
          </div>
          <p v-else class="text-[10px] text-success">{{ t("eq.rig.ok") }}</p>
        </template>
      </div>

      <!-- Audio -->
      <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-3">
        <div class="flex items-center gap-2">
          <Activity class="w-4 h-4 text-cyan" />
          <span class="text-xs font-bold text-ink">{{ t("eq.audio.title") }}</span>
        </div>
        <div class="flex items-center gap-2">
          <input
            v-model="audioPath"
            type="text"
            :placeholder="t('eq.audio.placeholder')"
            class="flex-1 px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
          />
          <button
            @click="handleInspectAudio"
            :disabled="!audioPath.trim() || isInspectingAudio"
            class="px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-xs font-semibold transition cursor-pointer disabled:opacity-40 shrink-0"
          >
            {{ isInspectingAudio ? t("eq.reading") : t("eq.check") }}
          </button>
        </div>
        <template v-if="audioResult">
          <div class="grid grid-cols-4 gap-2">
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">{{ t("eq.audio.format") }}</div>
              <div class="text-sm font-bold text-ink">{{ audioResult.format }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">{{ t("eq.audio.sample.rate") }}</div>
              <div
                class="text-sm font-bold font-mono"
                :class="!audioResult.metadataKnown ? 'text-ink-muted' : audioResult.isStandard48k ? 'text-success' : 'text-warning'"
              >
                {{ audioResult.metadataKnown ? audioResult.sampleRate : t("eq.unknown") }}
              </div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">{{ t("eq.audio.channels") }}</div>
              <div class="text-xs font-bold text-ink truncate">{{ audioResult.channelStr }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">{{ t("eq.audio.duration") }}</div>
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
          <span class="text-xs font-bold text-ink">{{ t("eq.l10n.title") }}</span>
        </div>
        <div class="flex items-center gap-2">
          <input
            v-model="fontPath"
            type="text"
            :placeholder="t('eq.l10n.placeholder')"
            class="flex-1 px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
          />
          <button
            @click="handleAuditLocalization"
            :disabled="isAuditingLocalization"
            class="px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-xs font-semibold transition cursor-pointer disabled:opacity-40 shrink-0"
          >
            {{ isAuditingLocalization ? t("eq.checking") : t("eq.l10n.run") }}
          </button>
          <button
            @click="handleCopyLocalizationPatch"
            :disabled="isCopyingPatch"
            class="px-2.5 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-40 shrink-0"
            :title="t('eq.l10n.patch.btn')"
          >
            {{ t("eq.l10n.patch.btn") }}
          </button>
        </div>
        <template v-if="localizationResult">
          <div class="flex items-center gap-2 text-[11px]">
            <span class="px-1.5 py-0.5 rounded text-[9px] font-bold" :class="severityClass(localizationResult.severity)">
              {{ localizationResult.severity }}
            </span>
            <span class="text-ink">{{ t("eq.l10n.languages", { count: localizationResult.languagesFound.length }) }}</span>
            <span class="text-ink-muted">{{ t("eq.l10n.keys", { count: localizationResult.totalUniqueKeys }) }}</span>
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
            {{ t("eq.l10n.missing", { count: (localizationResult.untranslatedList || []).length }) }}
            {{ (localizationResult.untranslatedList || []).slice(0, 3).map((entry) => entry.key)
              .join(t("common.list.comma")) }}
          </p>
          <div v-if="localizationResult.fontAudit" class="rounded-lg border border-line bg-canvas/40 p-2 space-y-1 text-[10px] font-mono">
            <div class="flex flex-wrap items-center gap-2">
              <span class="text-ink font-semibold">{{ t("eq.l10n.font", { format: localizationResult.fontAudit.fontFormat }) }}</span>
              <span class="text-ink-muted">{{ t("eq.l10n.glyphs", { total: localizationResult.fontAudit.totalFontGlyphs, checked: localizationResult.fontAudit.totalCharsChecked }) }}</span>
              <span :class="localizationResult.fontAudit.missingGlyphsCount > 0 ? 'text-danger' : 'text-success'">
                {{ t("eq.l10n.coverage", { missing: localizationResult.fontAudit.missingGlyphsCount, percent: localizationResult.fontAudit.coveragePercent }) }}
              </span>
            </div>
            <div v-if="localizationResult.fontAudit.missingGlyphsCount > 0" class="text-warning break-all">
              {{ t("eq.l10n.missing.glyphs") }}
              {{ localizationResult.fontAudit.missingGlyphs.slice(0, 12).map((glyph) => `${glyph.char}(${glyph.unicodeHex})`).join(" ") }}
            </div>
          </div>
          <ul v-if="(localizationResult.advice || []).length > 0" class="text-[10px] text-warning space-y-0.5">
            <li v-for="(tip, index) in localizationResult.advice" :key="index">{{ tip }}</li>
          </ul>
          <p v-else class="text-[10px] text-success">{{ t("eq.l10n.ok") }}</p>
        </template>
      </div>

      <!-- Native plugins -->
      <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-3">
        <div class="flex items-center justify-between gap-2">
          <div class="flex items-center gap-2">
            <Server class="w-4 h-4 text-purple" />
            <span class="text-xs font-bold text-ink">{{ t("eq.plugin.title") }}</span>
          </div>
          <button
            @click="handleAuditPlugins"
            :disabled="isAuditingPlugins"
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-50"
          >
            <Search class="w-3.5 h-3.5" :class="{ 'animate-spin': isAuditingPlugins }" />
            <span>{{ isAuditingPlugins ? t("eq.scanning") : t("eq.plugin.scan") }}</span>
          </button>
        </div>
        <template v-if="pluginResult">
          <div class="grid grid-cols-3 gap-2">
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">{{ t("eq.plugin.count") }}</div>
              <div class="text-sm font-bold font-mono text-ink">{{ pluginResult.totalPluginsCount }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">{{ t("eq.plugin.compliance") }}</div>
              <div class="text-sm font-bold font-mono text-success">{{ pluginResult.compliantCount }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">{{ t("eq.plugin.defects") }}</div>
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
                <span v-if="plugin.hasIssue" class="text-danger shrink-0">{{ t("eq.plugin.defect") }}</span>
              </div>
              <div v-if="(plugin.issues || []).length > 0" class="text-danger truncate" :title="plugin.issues.join(t('common.list.semicolon'))">
                {{ (plugin.issues || []).join(t("common.list.semicolon")) }}
              </div>
            </div>
            <div v-if="(pluginResult.plugins || []).length === 0" class="text-[11px] text-success">{{ t("eq.plugin.none") }}</div>
          </div>
        </template>
      </div>

      <!-- Missing materials -->
      <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-3">
        <div class="flex items-center gap-2">
          <FileWarning class="w-4 h-4 text-danger" />
          <span class="text-xs font-bold text-ink">{{ t("eq.material.title") }}</span>
        </div>
        <div class="flex items-center gap-2">
          <input
            v-model="materialTargetDir"
            type="text"
            :placeholder="t('eq.material.placeholder')"
            class="flex-1 px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
          />
          <button
            @click="handleScanMaterials"
            :disabled="isScanningMaterials"
            class="px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-xs font-semibold transition cursor-pointer disabled:opacity-40 shrink-0"
          >
            {{ isScanningMaterials ? t("eq.scanning") : t("eq.material.scan") }}
          </button>
        </div>
        <template v-if="materialResult">
          <div class="grid grid-cols-4 gap-2">
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">{{ t("eq.material.count") }}</div>
              <div class="text-sm font-bold font-mono text-ink">{{ materialResult.totalMaterialsScanned }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">{{ t("eq.material.broken") }}</div>
              <div class="text-sm font-bold font-mono text-danger">{{ materialResult.brokenMaterialsCount }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">{{ t("eq.material.pink") }}</div>
              <div class="text-sm font-bold font-mono text-warning">{{ materialResult.pinkShaderCount }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">{{ t("eq.material.health") }}</div>
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
                <span v-if="material.isPinkShader" class="text-warning shrink-0">{{ t("eq.material.pink.short") }}</span>
              </div>
              <div v-if="(material.missingTextures || []).length > 0" class="text-ink-muted truncate">
                {{ t("eq.material.missing.textures", { list: (material.missingTextures || []).map((tex) => tex.propertySlot).join(", ") }) }}
              </div>
            </div>
            <div v-if="(materialResult.brokenMaterials || []).length === 0" class="text-[11px] text-success">{{ t("eq.material.none") }}</div>
          </div>
          <p class="text-[10px] text-ink-muted">{{ materialResult.summary }}</p>
        </template>
      </div>

      <!-- Shader variants -->
      <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-3">
        <div class="flex items-center gap-2">
          <Activity class="w-4 h-4 text-cyan" />
          <span class="text-xs font-bold text-ink">{{ t("eq.shader.title") }}</span>
        </div>
        <div class="flex items-center gap-2">
          <input
            v-model="shaderPath"
            type="text"
            :placeholder="t('eq.shader.placeholder')"
            class="flex-1 px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
          />
          <button
            @click="handleInspectShader"
            :disabled="!shaderPath.trim() || isInspectingShader"
            class="px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-xs font-semibold transition cursor-pointer disabled:opacity-40 shrink-0"
          >
            {{ isInspectingShader ? t("eq.auditing") : t("eq.shader.run") }}
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
              <div class="text-[10px] text-ink-muted">{{ t("eq.shader.actual") }}</div>
              <div class="text-sm font-bold font-mono" :class="shaderResult.isExplosion ? 'text-danger' : 'text-ink'">{{ shaderResult.totalVariants }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">{{ t("eq.shader.limit") }}</div>
              <div class="text-sm font-bold font-mono text-ink-muted">{{ shaderResult.theoreticalMax }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">Pass</div>
              <div class="text-sm font-bold font-mono text-purple">{{ shaderResult.passCount }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">{{ t("eq.shader.keywords") }}</div>
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
            <div class="text-xs font-bold text-ink">{{ t("eq.project.title") }}</div>
            <div class="text-[10px] text-ink-muted">{{ t("eq.project.desc") }}</div>
          </div>
          <input
            v-model="auditScopeDir"
            type="text"
            :placeholder="t('eq.project.placeholder')"
            class="w-72 px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
          />
        </div>

        <div class="grid grid-cols-1 lg:grid-cols-2 gap-3">
          <!-- LOD streaming -->
          <div class="rounded-lg border border-line bg-canvas/30 p-3 space-y-2">
            <div class="flex items-center justify-between gap-2">
              <span class="text-xs font-bold text-ink">{{ t("eq.lod.title") }}</span>
              <button
                @click="handleAuditLod"
                :disabled="isAuditingLod"
                class="px-2.5 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-[11px] font-semibold text-ink transition cursor-pointer disabled:opacity-50"
              >
                {{ isAuditingLod ? t("eq.auditing") : t("eq.run.audit") }}
              </button>
            </div>
            <template v-if="lodResult">
              <div class="grid grid-cols-4 gap-1.5">
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">{{ t("eq.lod.meshes") }}</div>
                  <div class="text-xs font-bold font-mono text-ink">{{ lodResult.totalMeshesAudited }}</div>
                </div>
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">{{ t("eq.lod.missing") }}</div>
                  <div class="text-xs font-bold font-mono text-warning">{{ lodResult.lodMissingCount }}</div>
                </div>
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">{{ t("eq.lod.streaming") }}</div>
                  <div class="text-xs font-bold font-mono text-warning">{{ lodResult.streamingMissingCount }}</div>
                </div>
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">{{ t("eq.lod.health") }}</div>
                  <div
                    class="text-xs font-bold font-mono"
                    :class="!lodResult.analysisKnown ? 'text-ink-muted' : lodResult.overallHealthScore >= 85 ? 'text-success' : 'text-warning'"
                  >
                    {{ lodResult.analysisKnown ? lodResult.overallHealthScore : "—" }}
                  </div>
                </div>
              </div>
              <p class="text-[10px] text-ink-muted">{{ t("eq.lod.savings", { mb: lodResult.potentialSavingsMb, percent: lodResult.savingsPercentage }) }}</p>
              <p class="text-[10px] text-ink-muted">{{ lodResult.summary }}</p>
              <button
                type="button"
                class="self-start px-2 py-0.5 rounded-md border border-line bg-canvas hover:bg-raised text-[10px] font-semibold text-ink-muted hover:text-accent transition cursor-pointer"
                :title="t('eq.audit.plan.export')"
                @click="exportLodPlan"
              >
                {{ t("eq.audit.plan.export") }}
              </button>
              <p
                v-if="(lodResult.lodUnknownCount || 0) + (lodResult.streamingUnknownCount || 0) > 0"
                class="text-[10px] text-warning"
              >
                {{ t("eq.lod.unknown", { meshes: lodResult.lodUnknownCount, textures: lodResult.streamingUnknownCount }) }}
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
              >{{ t("eq.lod.ok") }}</p>
            </template>
          </div>

          <!-- Physics collision -->
          <div class="rounded-lg border border-line bg-canvas/30 p-3 space-y-2">
            <div class="flex items-center justify-between gap-2">
              <span class="text-xs font-bold text-ink">{{ t("eq.physics.title") }}</span>
              <button
                @click="handleAuditPhysics"
                :disabled="isAuditingPhysics"
                class="px-2.5 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-[11px] font-semibold text-ink transition cursor-pointer disabled:opacity-50"
              >
                {{ isAuditingPhysics ? t("eq.auditing") : t("eq.run.audit") }}
              </button>
            </div>
            <template v-if="physicsResult">
              <div class="grid grid-cols-4 gap-1.5">
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">{{ t("eq.physics.colliders") }}</div>
                  <div class="text-xs font-bold font-mono text-ink">{{ physicsResult.totalColliders }}</div>
                </div>
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">{{ t("eq.physics.hulls") }}</div>
                  <div class="text-xs font-bold font-mono text-success">{{ physicsResult.convexHullsCount }}</div>
                </div>
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">{{ t("eq.physics.concave") }}</div>
                  <div class="text-xs font-bold font-mono text-warning">{{ physicsResult.concaveMeshCount }}</div>
                </div>
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">{{ t("eq.physics.health") }}</div>
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
                  {{ t("eq.physics.hull.line", { path: hull.meshPath, vertices: hull.vertexCount }) }}
                </div>
              </div>
              <p v-else class="text-[10px] text-success">{{ t("eq.physics.ok") }}</p>
              <p v-if="(physicsResult.nonConvexDynamicColliders || []).length > 0" class="text-[10px] text-warning">
                {{ t("eq.physics.nonconvex", { count: physicsResult.nonConvexDynamicColliders.length }) }}
              </p>
              <p v-if="(physicsResult.missingPhysicMaterials || []).length > 0" class="text-[10px] text-warning">
                {{ t("eq.physics.no.material", { count: physicsResult.missingPhysicMaterials.length }) }}
              </p>
              <p class="text-[10px] text-ink-muted">{{ physicsResult.summary }}</p>
            </template>
          </div>

          <!-- Material overrides -->
          <div class="rounded-lg border border-line bg-canvas/30 p-3 space-y-2">
            <div class="flex items-center justify-between gap-2">
              <span class="text-xs font-bold text-ink">{{ t("eq.override.title") }}</span>
              <button
                @click="handleAuditMaterialOverrides"
                :disabled="isAuditingMaterialOverrides"
                class="px-2.5 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-[11px] font-semibold text-ink transition cursor-pointer disabled:opacity-50"
              >
                {{ isAuditingMaterialOverrides ? t("eq.auditing") : t("eq.run.audit") }}
              </button>
            </div>
            <template v-if="materialOverrideResult">
              <div class="grid grid-cols-3 gap-1.5">
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">{{ t("eq.override.materials") }}</div>
                  <div class="text-xs font-bold font-mono text-ink">{{ materialOverrideResult.totalInstances }}</div>
                </div>
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">{{ t("eq.override.params") }}</div>
                  <div class="text-xs font-bold font-mono text-purple">{{ materialOverrideResult.overriddenParametersCount }}</div>
                </div>
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">{{ t("eq.override.dead") }}</div>
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
                    {{ instance.deadCount > 0 ? t("eq.override.dead.count", { count: instance.deadCount }) : t("eq.ok") }}
                  </span>
                </div>
                <div v-if="(materialOverrideResult.instances || []).length === 0" class="text-[10px] text-success">{{ t("eq.override.none") }}</div>
              </div>
              <p class="text-[10px] text-ink-muted">{{ materialOverrideResult.summary }}</p>
            </template>
          </div>

          <!-- DCC locks -->
          <div class="rounded-lg border border-line bg-canvas/30 p-3 space-y-2">
            <div class="flex items-center justify-between gap-2">
              <span class="text-xs font-bold text-ink">{{ t("eq.dcc.title") }}</span>
              <button
                @click="handleDetectDcc"
                :disabled="isDetectingDcc"
                class="px-2.5 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-[11px] font-semibold text-ink transition cursor-pointer disabled:opacity-50"
              >
                {{ isDetectingDcc ? t("eq.auditing") : t("eq.dcc.run") }}
              </button>
            </div>
            <div class="flex items-center gap-1.5">
              <input
                v-model="dccAssetPath"
                type="text"
                :placeholder="t('eq.dcc.placeholder')"
                @keydown.enter="handleAutoLockDcc"
                class="flex-1 px-2 py-1 rounded bg-canvas border border-line text-[10px] text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
              />
              <button
                @click="handleAutoLockDcc"
                :disabled="!dccAssetPath.trim() || isAutoLockingDcc"
                class="px-2 py-1 rounded bg-surface hover:bg-raised border border-line text-[10px] font-semibold text-warning transition cursor-pointer disabled:opacity-40 shrink-0"
              >
                {{ isAutoLockingDcc ? t("eq.dcc.locking") : t("eq.dcc.lock") }}
              </button>
            </div>
            <template v-if="dccResult">
              <div class="grid grid-cols-4 gap-1.5">
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">{{ t("eq.dcc.files") }}</div>
                  <div class="text-xs font-bold font-mono text-ink">{{ dccResult.totalDccFiles }}</div>
                </div>
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">{{ t("eq.dcc.active.locks") }}</div>
                  <div class="text-xs font-bold font-mono text-warning">{{ dccResult.activeLocksCount }}</div>
                </div>
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">{{ t("eq.dcc.unprotected") }}</div>
                  <div class="text-xs font-bold font-mono text-ink-muted">{{ dccResult.unprotectedCount }}</div>
                </div>
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">{{ t("eq.dcc.conflicts") }}</div>
                  <div class="text-xs font-bold font-mono text-danger">{{ dccResult.collisionCount }}</div>
                </div>
              </div>
              <p v-if="(dccResult.runningProcesses || []).length > 0" class="text-[10px] text-ink-muted truncate">
                {{ t("eq.dcc.running", { list: (dccResult.runningProcesses || []).map((proc) => proc.dccTool || proc.processName).join(", ") }) }}
              </p>
              <div class="space-y-1 max-h-32 overflow-y-auto">
                <div
                  v-for="item in (dccResult.items || []).slice(0, 8)"
                  :key="item.lockFilePath"
                  class="flex items-center justify-between gap-2 p-1 rounded hover:bg-surface/40 text-[10px] font-mono group"
                >
                  <div class="flex items-center gap-1.5 min-w-0 flex-1">
                    <span class="text-ink truncate" :title="item.sourceAssetPath || item.lockFilePath">
                      {{ item.sourceAssetPath || item.lockFilePath }}
                    </span>
                    <span class="px-1 py-0.2 rounded bg-surface text-[9px] text-ink-muted shrink-0">{{ item.dccTool }}</span>
                    <span v-if="item.lockOwner" class="text-[9px] text-purple shrink-0">@{{ item.lockOwner }}</span>
                    <span v-if="item.collisionRisk" class="px-1 rounded bg-danger/15 text-danger text-[9px] font-semibold shrink-0">{{ t("eq.dcc.conflict") }}</span>
                  </div>
                  <div class="flex items-center gap-1 shrink-0">
                    <button
                      v-if="item.sourceAssetPath"
                      @click="handleQuickLockDcc(item.sourceAssetPath)"
                      :disabled="isAutoLockingDcc"
                      class="px-1.5 py-0.5 rounded bg-surface hover:bg-raised border border-line text-[9px] text-warning transition cursor-pointer disabled:opacity-40"
                      :title="t('eq.dcc.item.lock.title')"
                    >
                      {{ t("eq.dcc.item.lock") }}
                    </button>
                    <button
                      @click="copyDccPath(item.sourceAssetPath || item.lockFilePath)"
                      class="p-0.5 text-ink-muted hover:text-ink transition cursor-pointer"
                      :title="t('eq.dcc.copy.path')"
                    >
                      <Copy class="w-3 h-3" />
                    </button>
                  </div>
                </div>
                <div v-if="(dccResult.items || []).length === 0" class="text-[10px] text-success">{{ t("eq.dcc.none") }}</div>
              </div>
              <p class="text-[10px] text-ink-muted">{{ dccResult.summary }}</p>
            </template>
          </div>

          <!-- Symbols & defines -->
          <div class="rounded-lg border border-line bg-canvas/30 p-3 space-y-2">
            <div class="flex items-center justify-between gap-2">
              <span class="text-xs font-bold text-ink">{{ t("eq.symbols.title") }}</span>
              <button
                @click="handleAuditSymbols"
                :disabled="isAuditingSymbols"
                class="px-2.5 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-[11px] font-semibold text-ink transition cursor-pointer disabled:opacity-50"
              >
                {{ isAuditingSymbols ? t("eq.auditing") : t("eq.run.audit") }}
              </button>
            </div>
            <div class="flex items-center gap-1.5">
              <input
                v-model="symbolsBase"
                type="text"
                :placeholder="t('eq.symbols.base')"
                class="w-24 px-2 py-1 rounded bg-canvas border border-line text-[10px] text-ink font-mono focus:outline-none focus:border-accent"
              />
              <span class="text-[10px] text-ink-muted">→</span>
              <input
                v-model="symbolsTarget"
                type="text"
                :placeholder="t('eq.symbols.target')"
                class="w-24 px-2 py-1 rounded bg-canvas border border-line text-[10px] text-ink font-mono focus:outline-none focus:border-accent"
              />
            </div>
            <template v-if="symbolsResult">
              <div class="flex items-center gap-2 text-[10px]">
                <span class="px-1.5 py-0.5 rounded text-[9px] font-bold" :class="severityClass(symbolsResult.severity)">
                  {{ symbolsResult.severity }}
                </span>
                <span class="text-ink-muted">{{ t("eq.symbols.total", { count: symbolsResult.totalDefines }) }}</span>
                <span class="text-danger ml-auto">{{ t("eq.symbols.dangerous", { count: symbolsResult.dangerousDebugDefines.length }) }}</span>
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
              <p v-else-if="symbolsResult.analysisKnown" class="text-[10px] text-success">{{ t("eq.symbols.ok") }}</p>
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
              <span class="text-xs font-bold text-ink">{{ t("eq.table.title") }}</span>
              <button
                @click="handleAuditDataTables"
                :disabled="isAuditingTables"
                class="px-2.5 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-[11px] font-semibold text-ink transition cursor-pointer disabled:opacity-50"
              >
                {{ isAuditingTables ? t("eq.auditing") : t("eq.run.audit") }}
              </button>
            </div>
            <input
              v-model="dataTablePath"
              type="text"
              :placeholder="t('eq.table.placeholder')"
              class="w-full px-2 py-1 rounded bg-canvas border border-line text-[10px] text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
            />
            <div class="flex items-center gap-1.5">
              <input
                v-model="dataTableBase"
                type="text"
                :placeholder="t('eq.table.base')"
                class="w-24 px-2 py-1 rounded bg-canvas border border-line text-[10px] text-ink font-mono focus:outline-none focus:border-accent"
              />
              <span class="text-[10px] text-ink-muted">→</span>
              <input
                v-model="dataTableTarget"
                type="text"
                :placeholder="t('eq.table.target')"
                class="w-28 px-2 py-1 rounded bg-canvas border border-line text-[10px] text-ink font-mono focus:outline-none focus:border-accent"
              />
            </div>
            <template v-if="dataTableResult">
              <div class="grid grid-cols-3 gap-1.5">
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">{{ t("eq.table.tables") }}</div>
                  <div class="text-xs font-bold font-mono text-ink">{{ dataTableResult.totalTables }}</div>
                </div>
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">{{ t("eq.table.critical") }}</div>
                  <div class="text-xs font-bold font-mono text-danger">{{ dataTableResult.criticalCount }}</div>
                </div>
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">{{ t("eq.table.warn") }}</div>
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
                    {{ t("eq.table.changes", { schema: (report.schemaChanges || []).length, anomalies: (report.anomalies || []).length }) }}
                  </div>
                </div>
                <div v-if="(dataTableResult.auditedTables || []).length === 0" class="text-[10px] text-ink-muted">{{ t("eq.table.none") }}</div>
              </div>
              <p class="text-[10px] text-ink-muted">{{ dataTableResult.overallSummary }}</p>
            </template>
          </div>

          <!-- Cross-branch asset cherry-pick -->
          <div class="rounded-lg border border-line bg-canvas/30 p-3 space-y-2">
            <div class="flex items-center justify-between gap-2">
              <span class="text-xs font-bold text-ink">{{ t("eq.pick.title") }}</span>
              <span class="text-[9px] text-ink-muted font-mono">{{ t("eq.pick.hint") }}</span>
            </div>
            <input
              v-model="assetPickTargetRef"
              type="text"
              :placeholder="t('eq.pick.from')"
              class="w-full px-2 py-1 rounded bg-canvas border border-line text-[10px] text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
            />
            <textarea
              v-model="assetPickPaths"
              rows="2"
              :placeholder="t('eq.pick.paths')"
              class="w-full px-2 py-1 rounded bg-canvas border border-line text-[10px] text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent resize-y"
            ></textarea>
            <div class="flex flex-wrap items-center gap-3 text-[10px] text-ink-muted">
              <label class="flex items-center gap-1 cursor-pointer">
                <input v-model="assetPickResolveDeps" type="checkbox" class="rounded accent-accent scale-90" />
                <span>{{ t("eq.pick.deps") }}</span>
              </label>
              <label class="flex items-center gap-1 cursor-pointer">
                <input v-model="assetPickDryRun" type="checkbox" class="rounded accent-accent scale-90" />
                <span>{{ t("eq.pick.dryrun") }}</span>
              </label>
              <button
                @click="handlePickAssets"
                :disabled="isPickingAssets || !assetPickTargetRef.trim() || !assetPickPaths.trim()"
                class="ml-auto px-2.5 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-[11px] font-semibold text-accent transition cursor-pointer disabled:opacity-40"
              >
                {{ isPickingAssets ? t("eq.processing") : assetPickDryRun ? t("eq.pick.preview") : t("eq.pick.run") }}
              </button>
            </div>
            <template v-if="assetPickResult">
              <div class="grid grid-cols-4 gap-1.5">
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">{{ t("eq.pick.requested") }}</div>
                  <div class="text-xs font-bold font-mono text-ink">{{ assetPickResult.totalRequested }}</div>
                </div>
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">{{ t("eq.pick.dependencies") }}</div>
                  <div class="text-xs font-bold font-mono text-purple">{{ assetPickResult.totalDependencies }}</div>
                </div>
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">{{ t("eq.pick.total") }}</div>
                  <div class="text-xs font-bold font-mono text-ink">{{ assetPickResult.totalAssetsCount }}</div>
                </div>
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">{{ t("eq.pick.size") }}</div>
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
                  <span v-if="asset.isDependency" class="text-purple shrink-0">{{ t("eq.pick.dependency") }}</span>
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
              <span class="text-xs font-bold text-ink">{{ t("eq.unreal.title") }}</span>
              <span class="text-[9px] text-ink-muted font-mono">UAsset / Blueprint</span>
            </div>
            <input
              v-model="unrealAssetPath"
              type="text"
              :placeholder="t('eq.unreal.placeholder')"
              class="w-full px-2 py-1 rounded bg-canvas border border-line text-[10px] text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
            />
            <div class="flex items-center gap-1.5">
              <input
                v-model="unrealBaseRef"
                type="text"
                :placeholder="t('eq.table.base')"
                class="w-24 px-2 py-1 rounded bg-canvas border border-line text-[10px] text-ink font-mono focus:outline-none focus:border-accent"
              />
              <span class="text-[10px] text-ink-muted">→</span>
              <input
                v-model="unrealTargetRef"
                type="text"
                :placeholder="t('eq.table.target')"
                class="w-28 px-2 py-1 rounded bg-canvas border border-line text-[10px] text-ink font-mono focus:outline-none focus:border-accent"
              />
              <button
                @click="handleDiffUnrealAsset"
                :disabled="!unrealAssetPath.trim() || isDiffingUnreal"
                class="ml-auto px-2.5 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-[11px] font-semibold text-accent transition cursor-pointer disabled:opacity-40"
              >
                {{ isDiffingUnreal ? t("eq.diffing") : t("eq.unreal.run") }}
              </button>
            </div>
            <template v-if="unrealDiffResult">
              <div class="grid grid-cols-4 gap-1.5">
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-success">{{ unrealDiffResult.analysisKnown ? t("eq.unreal.added") : t("eq.unreal.candidate.added") }}</div>
                  <div class="text-xs font-bold font-mono text-success">{{ (unrealDiffResult.addedNodes || []).length }}</div>
                </div>
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-danger">{{ unrealDiffResult.analysisKnown ? t("eq.unreal.removed") : t("eq.unreal.candidate.removed") }}</div>
                  <div class="text-xs font-bold font-mono text-danger">{{ (unrealDiffResult.removedNodes || []).length }}</div>
                </div>
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-warning">{{ unrealDiffResult.analysisKnown ? t("eq.unreal.changed") : t("eq.unreal.candidate.changed") }}</div>
                  <div class="text-xs font-bold font-mono text-warning">{{ (unrealDiffResult.modifiedNodes || []).length }}</div>
                </div>
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">{{ unrealDiffResult.analysisKnown ? t("eq.unreal.unchanged") : t("eq.unreal.candidate.unchanged") }}</div>
                  <div class="text-xs font-bold font-mono text-ink-muted">{{ unrealDiffResult.unchangedCount }}</div>
                </div>
              </div>
              <div class="flex flex-wrap items-center gap-x-2 gap-y-0.5 text-[10px] font-mono text-ink-muted">
                <span>{{ t("eq.unreal.format", { format: unrealDiffResult.assetFormat }) }}</span>
                <span v-if="unrealDiffResult.targetHeader">{{ t("eq.unreal.engine", { version: unrealDiffResult.targetHeader.engineVersion }) }}</span>
                <span v-if="unrealDiffResult.targetHeader">Magic {{ unrealDiffResult.targetHeader.magic }}</span>
                <span>{{ t("eq.unreal.nodes", { base: unrealDiffResult.totalBaseNodes, target: unrealDiffResult.totalTargetNodes }) }}</span>
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
              <span class="text-xs font-bold text-ink">{{ t("eq.scene.title") }}</span>
              <button
                @click="handleAuditSceneStream"
                :disabled="isAuditingScene"
                class="px-2.5 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-[11px] font-semibold text-ink transition cursor-pointer disabled:opacity-50"
              >
                {{ isAuditingScene ? t("eq.auditing") : t("eq.run.audit") }}
              </button>
            </div>
            <div class="flex items-center gap-1.5">
              <input
                v-model="sceneTargetBranch"
                type="text"
                :placeholder="t('eq.scene.target')"
                class="w-28 px-2 py-1 rounded bg-canvas border border-line text-[10px] text-ink font-mono focus:outline-none focus:border-accent"
              />
              <input
                v-model="sceneAuditPath"
                type="text"
                :placeholder="t('eq.scene.level')"
                class="flex-1 px-2 py-1 rounded bg-canvas border border-line text-[10px] text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
              />
            </div>
            <template v-if="sceneAuditResult">
              <div class="grid grid-cols-4 gap-1.5">
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">{{ t("eq.scene.safety") }}</div>
                  <div
                    class="text-xs font-bold font-mono"
                    :class="!sceneAuditResult.analysisKnown ? 'text-ink-muted' : sceneAuditResult.safetyScore >= 85 ? 'text-success' : sceneAuditResult.safetyScore >= 60 ? 'text-warning' : 'text-danger'"
                  >
                    {{ sceneAuditResult.analysisKnown ? sceneAuditResult.safetyScore : "—" }}
                  </div>
                </div>
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">{{ t("eq.scene.guid") }}</div>
                  <div class="text-xs font-bold font-mono text-danger">{{ (sceneAuditResult.guidCollisions || []).length }}</div>
                </div>
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">{{ t("eq.scene.overlap") }}</div>
                  <div class="text-xs font-bold font-mono text-warning">{{ (sceneAuditResult.spatialOverlaps || []).length }}</div>
                </div>
                <div class="p-1.5 rounded bg-surface/60 text-center">
                  <div class="text-[9px] text-ink-muted">{{ t("eq.scene.transform") }}</div>
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
              <span class="text-xs font-bold text-ink">{{ t("eq.scene.fix.title") }}</span>
              <span class="text-[9px] text-warning font-bold">{{ t("eq.scene.fix.warn") }}</span>
            </div>
            <input
              v-model="resolveScenePath"
              type="text"
              :placeholder="t('eq.scene.fix.placeholder')"
              class="w-full px-2 py-1 rounded bg-canvas border border-line text-[10px] text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
            />
            <div class="flex items-center gap-1.5">
              <input
                v-model="resolveSceneGuid"
                type="text"
                :placeholder="t('eq.scene.fix.guid')"
                class="flex-1 px-2 py-1 rounded bg-canvas border border-line text-[10px] text-ink font-mono focus:outline-none focus:border-accent"
              />
              <select
                v-model="resolveSceneAction"
                :aria-label="t('eq.scene.fix.action')"
                class="px-2 py-1 rounded bg-canvas border border-line text-[10px] text-ink focus:outline-none focus:border-accent cursor-pointer"
              >
                <option value="AUTO_REGENERATE_GUID">{{ t("eq.scene.fix.regenerate") }}</option>
                <option value="SPATIAL_OFFSET">{{ t("eq.scene.fix.offset") }}</option>
              </select>
              <button
                @click="handleResolveSceneConflict"
                :disabled="!resolveScenePath.trim() || !resolveSceneGuid.trim() || isResolvingScene"
                class="px-2.5 py-1 rounded-lg bg-danger-strong hover:opacity-90 text-white text-[11px] font-semibold transition cursor-pointer disabled:opacity-40 shrink-0"
              >
                {{ isResolvingScene ? t("eq.scene.fix.running") : t("eq.scene.fix.apply") }}
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
