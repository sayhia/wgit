<script setup lang="ts">
import { t } from "../../i18n";
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
  Copy,
} from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import FeatureGate from "../ui/FeatureGate.vue";
import AssetBoard from "./AssetBoard.vue";
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
      t("engine.confirm.artist"),
    ))
  )
    return;
  isArtistDelivering.value = true;
  try {
    artistResult.value = await gitService.artistOneClickPipeline(
      props.currentRepoPath,
      artistMessage.value.trim() || t("engine.artist.default.message"),
      artistTargetBranch.value.trim()
    );
    if (artistResult.value) notify(artistResult.value.message);
  } catch (err: any) {
    notify(t("engine.toast.artist.failed", { error: err?.message || err }));
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
    notify(t("engine.toast.launch.no.project"));
    return;
  }
  if (!(await confirmAction(t("engine.confirm.launch", { engine: engineInfo.value?.engineType ?? "" })))) return;
  isLaunchingEngine.value = true;
  try {
    const message = await gitService.launchGameEngine(props.currentRepoPath, engineKey.value);
    notify(message || t("engine.toast.launch.sent"));
  } catch (err: any) {
    notify(t("engine.toast.launch.failed", { error: err?.message || err }));
  } finally {
    isLaunchingEngine.value = false;
  }
}

async function handleCleanCaches() {
  if (!props.currentRepoPath || isCleaningCaches.value) return;
  const key = engineKey.value;
  if (!key) {
    notify(t("engine.toast.cache.no.project"));
    return;
  }
  if (
    !(await confirmAction(
      { message: t("engine.confirm.cache.clean", { engine: engineInfo.value?.engineType ?? "" }), tone: "danger" }
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
        ? t("engine.toast.cache.cleaned", { count: cacheCleanResult.value.cleanedFolders.length, size: cacheCleanResult.value.freedSizeStr })
        : t("engine.toast.cache.cleaned.short")
    );
    await loadEngineData();
  } catch (err: any) {
    notify(t("engine.toast.cache.failed", { error: err?.message || err }));
  } finally {
    isCleaningCaches.value = false;
  }
}

async function loadEngineData() {
  if (!props.currentRepoPath) return;
  isLoadingEngine.value = true;
  try {
    const [infoRes, flowsRes, mergeRes, unrealRes] = await Promise.allSettled([
      gitService.getGameEngineInfo(props.currentRepoPath),
      gitService.getGameEngineWorkflows(props.currentRepoPath),
      gitService.detectUnityYamlMerge(props.currentRepoPath),
      gitService.detectUnrealProject(props.currentRepoPath),
    ]);
    const errors: string[] = [];
    if (infoRes.status === "fulfilled") {
      engineInfo.value = infoRes.value;
    } else {
      errors.push(String((infoRes.reason as any)?.message || infoRes.reason));
    }
    if (flowsRes.status === "fulfilled") {
      workflows.value = flowsRes.value || [];
    } else {
      errors.push(String((flowsRes.reason as any)?.message || flowsRes.reason));
    }
    if (mergeRes.status === "fulfilled") {
      unityMerge.value = mergeRes.value;
      if (mergeRes.value?.detectedToolPath && !mergeToolPath.value) {
        mergeToolPath.value = mergeRes.value.detectedToolPath;
      }
    } else {
      errors.push(String((mergeRes.reason as any)?.message || mergeRes.reason));
    }
    if (unrealRes.status === "fulfilled") {
      unrealInfo.value = unrealRes.value?.isUnrealProject ? unrealRes.value : null;
    } else {
      errors.push(String((unrealRes.reason as any)?.message || unrealRes.reason));
    }
    if (errors.length > 0) {
      notify(t("engine.toast.load.failed", { error: errors.join("; ") }));
    }
  } finally {
    isLoadingEngine.value = false;
  }
}

async function inspectAsset() {
  const path = assetPath.value.trim();
  if (!props.currentRepoPath || !path || isInspectingAsset.value) return;
  isInspectingAsset.value = true;
  try {
    const [metaRes, refsRes] = await Promise.allSettled([
      gitService.inspectAssetMetadata(props.currentRepoPath, path),
      gitService.inspectAssetReferences(props.currentRepoPath, path),
    ]);
    const errors: string[] = [];
    // Clear stale results for the previous asset whenever a half fails.
    assetMeta.value = metaRes.status === "fulfilled" ? metaRes.value : null;
    assetRefs.value = refsRes.status === "fulfilled" ? refsRes.value : null;
    if (metaRes.status === "rejected") {
      errors.push(String((metaRes.reason as any)?.message || metaRes.reason));
    }
    if (refsRes.status === "rejected") {
      errors.push(String((refsRes.reason as any)?.message || refsRes.reason));
    }
    if (errors.length > 0) {
      // Partial or total failure: surface what went wrong instead of a success toast.
      notify(t("engine.toast.asset.partial", { error: errors.join("; ") }));
      return;
    }
    const meta = assetMeta.value;
    const refs = assetRefs.value;
    notify(
      refs
        ? t("engine.toast.asset.checked", { size: meta?.sizeHuman || t("engine.unknown.size"), refs: refs.referenceCount })
        : t("engine.toast.asset.meta.read")
    );
  } catch (err: any) {
    notify(t("engine.toast.asset.failed", { error: err?.message || err }));
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
        ? t("engine.toast.meta.found", { count: unityMismatches.value.length })
        : t("engine.toast.meta.ok")
    );
  } catch (err: any) {
    notify(t("engine.toast.meta.failed", { error: err?.message || err }));
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
        ? t("engine.toast.meta.fixed", { count: fixed.length })
        : t("engine.toast.meta.nothing")
    );
    await checkUnityMeta();
    emit("refresh");
  } catch (err: any) {
    notify(t("engine.toast.meta.fix.failed", { error: err?.message || err }));
    // A partial fix may still have staged some files; reload the real state.
    await checkUnityMeta();
    emit("refresh");
  } finally {
    isFixingMeta.value = false;
  }
}

async function copyPath(path: string) {
  if (!path) return;
  try {
    await navigator.clipboard.writeText(path);
    notify(t("engine.toast.copied", { path }));
  } catch {
    notify(t("engine.toast.copy.failed"));
  }
}

async function handleConfigureUnityMerge() {
  if (!props.currentRepoPath || isConfiguringMerge.value) return;
  const path = mergeToolPath.value.trim() || unityMerge.value?.detectedToolPath || "";
  if (!path) {
    notify(t("engine.toast.yamlmere.not.found"));
    return;
  }
  isConfiguringMerge.value = true;
  try {
    await gitService.configureUnityYamlMerge(
      props.currentRepoPath,
      path,
      applyMergeAttributes.value
    );
    notify(t("engine.toast.yamlmere.configured"));
    await loadEngineData();
  } catch (err: any) {
    notify(t("engine.toast.yamlmere.failed", { error: err?.message || err }));
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
    notify(t("engine.toast.preset.no.engine"));
    return;
  }
  isApplyingPreset.value = true;
  try {
    const preset = await gitService.getGameEngineGitAttributesPreset(engine);
    if (!preset?.trim()) {
      notify(t("engine.toast.preset.none"));
      return;
    }
    if (!(await confirmAction({ message: t("engine.confirm.preset", { engine }), tone: "danger" }))) {
      return;
    }
    await gitService.saveGitAttributes(props.currentRepoPath, preset);
    notify(t("engine.toast.preset.applied", { engine }));
    await loadEngineData();
    emit("refresh");
  } catch (err: any) {
    notify(t("engine.toast.preset.failed", { error: err?.message || err }));
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
        ? t("engine.toast.texture.issues", { count: textureResult.value.totalIssues })
        : t("engine.toast.texture.ok")
    );
  } catch (err: any) {
    notify(t("engine.toast.texture.failed", { error: err?.message || err }));
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
  if (issueType === "orphan_meta") return "bg-danger-bg text-danger";
  if (issueType === "missing_meta") return "bg-warning-bg text-warning";
  return "bg-accent-bg text-accent";
}
</script>

<template>
  <div class="space-y-6">
    <!-- Section A0: Asset board (assetBoard flag; F7 minimal loop). Mounted
         above the engine sections — gating lives here, not inside the child. -->
    <FeatureGate flag="assetBoard">
      <AssetBoard
        :current-repo-path="currentRepoPath"
        :refresh-nonce="refreshNonce"
      />
    </FeatureGate>

    <!-- Section A: Engine detection -->
    <div class="space-y-3" data-tool-section="engine-detect">
      <div class="flex items-center justify-between pb-2 border-b border-line">
        <div>
          <h3 class="text-sm font-bold text-ink flex items-center gap-2">
            <Box class="w-4 h-4 text-accent" />
            <span>{{ t("engine.title") }}</span>
          </h3>
          <p class="text-xs text-ink-muted mt-0.5">
            {{ t("engine.desc") }}
          </p>
        </div>
        <div class="flex items-center gap-2">
          <button
            @click="handleApplyEnginePreset"
            :disabled="!engineInfo || isApplyingPreset"
            class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-accent transition cursor-pointer disabled:opacity-40"
            :title="t('engine.preset.title')"
          >
            {{ isApplyingPreset ? t("engine.applying") : t("engine.preset.apply") }}
          </button>
          <button
            @click="loadEngineData"
            :disabled="isLoadingEngine"
            class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-40"
          >
            {{ isLoadingEngine ? t("engine.detecting") : t("engine.redetect") }}
          </button>
        </div>
      </div>

      <div v-if="engineInfo" class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-2">
        <div class="p-3 rounded-xl border border-line bg-surface">
          <div class="text-[10px] text-ink-muted">{{ t("engine.type") }}</div>
          <div class="text-sm font-bold text-ink truncate">{{ engineInfo.engineType }}</div>
        </div>
        <div class="p-3 rounded-xl border border-line bg-surface">
          <div class="text-[10px] text-ink-muted">{{ t("engine.project") }}</div>
          <div class="text-sm font-bold text-ink truncate" :title="engineInfo.projectName">{{ engineInfo.projectName }}</div>
        </div>
        <div class="p-3 rounded-xl border border-line bg-surface">
          <div class="text-[10px] text-ink-muted">{{ t("engine.version") }}</div>
          <div class="text-sm font-bold font-mono text-ink">{{ engineInfo.engineVersion || "—" }}</div>
        </div>
        <div class="p-3 rounded-xl border border-line bg-surface">
          <div class="text-[10px] text-ink-muted">.gitattributes</div>
          <div class="text-sm font-bold" :class="engineInfo.hasGitAttributes ? 'text-success' : 'text-ink-muted'">
            {{ engineInfo.hasGitAttributes ? t("engine.configured") : t("engine.missing") }}
          </div>
        </div>
        <div class="p-3 rounded-xl border border-line bg-surface">
          <div class="text-[10px] text-ink-muted">{{ t("engine.lfs.rules") }}</div>
          <div class="text-sm font-bold" :class="engineInfo.hasLfsRules ? 'text-success' : 'text-warning'">
            {{ engineInfo.hasLfsRules ? t("engine.enabled") : t("engine.disabled") }}
          </div>
        </div>
      </div>

      <div v-if="unrealInfo" class="rounded-xl border border-line bg-surface p-3 grid grid-cols-2 lg:grid-cols-4 gap-2">
        <div class="min-w-0">
          <div class="text-[10px] text-ink-muted">{{ t("engine.unreal.project") }}</div>
          <div class="text-xs font-bold text-ink truncate" :title="unrealInfo.projectName">{{ unrealInfo.projectName }}</div>
        </div>
        <div class="min-w-0">
          <div class="text-[10px] text-ink-muted">{{ t("engine.version") }}</div>
          <div class="text-xs font-bold font-mono text-ink">{{ unrealInfo.engineVersion || "—" }}</div>
        </div>
        <div class="min-w-0">
          <div class="text-[10px] text-ink-muted">{{ t("engine.cache.dirs") }}</div>
          <div class="text-xs font-bold text-ink truncate" :title="(unrealInfo.cacheFoldersFound || []).join(t('common.list.comma'))">
            {{ t("engine.cache.summary", { count: (unrealInfo.cacheFoldersFound || []).length, size: unrealInfo.cacheSizeStr }) }}
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
          class="p-3 rounded-xl border border-line bg-surface space-y-1.5"
        >
          <div class="flex items-center gap-2">
            <Sparkles class="w-3.5 h-3.5 text-purple" />
            <span class="text-xs font-bold text-ink uppercase">{{ w.engineType }}</span>
            <span class="text-[10px] text-ink-muted truncate">{{ w.projectName }}</span>
            <span v-if="w.engineVersion" class="ml-auto text-[10px] font-mono text-ink-muted">{{ w.engineVersion }}</span>
          </div>
          <div class="text-[10px] text-ink-muted font-mono">
            {{ t("engine.cache.line", { size: w.cacheSizeStr || "0 B", count: (w.cacheFolders || []).length }) }}
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
            <span>{{ t("engine.asset.title") }}</span>
          </h3>
          <p class="text-xs text-ink-muted mt-0.5">
            {{ t("engine.asset.desc") }}
          </p>
        </div>
      </div>

      <div class="flex items-center gap-2">
        <input
          v-model="assetPath"
          type="text"
          :placeholder="t('engine.asset.placeholder')"
          @keydown.enter="inspectAsset"
          class="flex-1 px-3 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
        />
        <button
          @click="inspectAsset"
          :disabled="!assetPath.trim() || isInspectingAsset"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-xs font-semibold transition cursor-pointer disabled:opacity-40 shrink-0"
        >
          <Search class="w-3.5 h-3.5" aria-hidden="true" />
          {{ isInspectingAsset ? t("engine.checking") : t("engine.asset.check") }}
        </button>
      </div>

      <div v-if="assetMeta || assetRefs" class="grid grid-cols-1 lg:grid-cols-2 gap-3">
        <div v-if="assetMeta" class="rounded-xl border border-line bg-surface p-3 space-y-2">
          <div class="text-xs font-bold text-ink">{{ t("engine.asset.meta") }}</div>
          <div class="grid grid-cols-2 gap-2 text-[11px]">
            <div class="p-2 rounded-lg bg-canvas border border-line">
              <div class="text-[10px] text-ink-muted">{{ t("engine.asset.size") }}</div>
              <div class="font-mono text-ink">{{ assetMeta.sizeHuman }}</div>
            </div>
            <div class="p-2 rounded-lg bg-canvas border border-line">
              <div class="text-[10px] text-ink-muted">{{ t("engine.asset.type") }}</div>
              <div class="font-mono text-ink truncate">{{ assetMeta.mimeType || assetMeta.extension }}</div>
            </div>
            <div v-if="assetMeta.dimensions" class="p-2 rounded-lg bg-canvas border border-line">
              <div class="text-[10px] text-ink-muted">{{ t("engine.asset.dimensions") }}</div>
              <div class="font-mono text-ink">{{ assetMeta.dimensions }}</div>
            </div>
            <div v-if="assetMeta.duration" class="p-2 rounded-lg bg-canvas border border-line">
              <div class="text-[10px] text-ink-muted">{{ t("engine.asset.duration") }}</div>
              <div class="font-mono text-ink">{{ assetMeta.duration }}</div>
            </div>
            <div class="p-2 rounded-lg bg-canvas border border-line">
              <div class="text-[10px] text-ink-muted">LFS</div>
              <div class="font-mono" :class="assetMeta.isLfs ? 'text-success' : 'text-ink-muted'">
                {{ assetMeta.isLfs ? t("engine.yes") : t("engine.no") }}
              </div>
            </div>
            <div class="p-2 rounded-lg bg-canvas border border-line">
              <div class="text-[10px] text-ink-muted">{{ t("engine.asset.modified") }}</div>
              <div class="font-mono text-ink text-[10px]">{{ assetMeta.lastModTime }}</div>
            </div>
          </div>
          <div
            v-if="assetMeta.lfsPointerError"
            class="px-2 py-1 rounded-lg bg-warning-bg border border-warning/30 text-[10px] text-warning"
          >
            {{ t("engine.asset.lfs.pointer.failed", { error: assetMeta.lfsPointerError }) }}
          </div>
        </div>

        <div v-if="assetRefs" class="rounded-xl border border-line bg-surface p-3 space-y-2">
          <div class="flex items-center gap-2">
            <div class="text-xs font-bold text-ink">{{ t("engine.asset.refs") }}</div>
            <span
              class="px-1.5 py-0.5 rounded text-[10px] font-bold"
              :class="!assetRefs.analysisComplete
                ? 'bg-warning-bg text-warning'
                : assetRefs.safeToDelete ? 'bg-success-bg text-success' : 'bg-danger-bg text-danger'"
            >
              {{ !assetRefs.analysisComplete ? t("engine.refs.incomplete") : assetRefs.safeToDelete ? t("engine.refs.safe") : t("engine.refs.used") }}
            </span>
            <span class="ml-auto text-[10px] text-ink-muted font-mono">{{ t("engine.refs.count", { count: assetRefs.referenceCount }) }}</span>
          </div>
          <div class="text-[10px] text-ink-muted">{{ assetRefs.message }}</div>
          <ul v-if="assetRefs.warnings?.length" class="space-y-0.5 text-[10px] text-warning">
            <li v-for="warning in assetRefs.warnings" :key="warning">• {{ warning }}</li>
          </ul>
          <div class="space-y-1 max-h-48 overflow-y-auto">
            <div
              v-for="ref in assetRefs.references"
              :key="`${ref.filePath}:${ref.matchCount}`"
              class="px-2 py-1.5 rounded-lg bg-canvas border border-line text-[10px]"
            >
              <div class="flex items-center gap-2">
                <span class="px-1 py-0.2 rounded bg-accent-bg text-accent font-bold shrink-0">{{ ref.fileType }}</span>
                <span class="font-mono text-ink truncate">{{ ref.filePath }}</span>
                <span class="ml-auto text-ink-muted shrink-0">×{{ ref.matchCount }}</span>
              </div>
              <div v-if="ref.previewSnippet" class="font-mono text-ink-muted truncate mt-0.5">{{ ref.previewSnippet }}</div>
            </div>
            <div v-if="(assetRefs.references || []).length === 0" class="py-4 text-center text-ink-muted">
              {{ assetRefs.analysisComplete ? t("engine.refs.none") : t("engine.refs.none.incomplete") }}
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Section C: Unity meta sync -->
    <div class="space-y-3" data-tool-section="engine-unity-meta">
      <!-- Actions keep nowrap labels; the row wraps when the English
           description cannot share the line with both buttons. -->
      <div class="flex flex-wrap items-center justify-between gap-x-3 gap-y-2 pb-2 border-b border-line">
        <div class="min-w-0 flex-1">
          <h3 class="text-sm font-bold text-ink flex items-center gap-2">
            <ShieldCheck class="w-4 h-4 text-success" />
            <span>{{ t("engine.meta.title") }}</span>
          </h3>
          <p class="text-xs text-ink-muted mt-0.5">
            {{ t("engine.meta.desc") }}
          </p>
        </div>
        <div class="ml-auto flex flex-wrap items-center gap-2 shrink-0">
          <button
            @click="handleAutoFixMeta"
            :disabled="isFixingMeta || unityMismatches.length === 0"
            class="whitespace-nowrap px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-warning transition cursor-pointer disabled:opacity-40"
            :title="t('engine.meta.fix.title')"
          >
            {{ isFixingMeta ? t("engine.fixing") : t("engine.meta.fix") }}
          </button>
          <button
            @click="checkUnityMeta"
            :disabled="isCheckingMeta"
            class="whitespace-nowrap px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-40"
          >
            {{ isCheckingMeta ? t("engine.checking") : t("engine.recheck") }}
          </button>
        </div>
      </div>

      <div v-if="unityMismatches.length > 0" class="space-y-1.5">
        <div
          v-for="m in unityMismatches"
          :key="`${m.issueType}:${m.metaPath}`"
          class="flex items-center justify-between gap-2 px-2.5 py-1.5 rounded-lg bg-warning-bg border border-warning/20 text-[11px] group"
        >
          <div class="flex items-start gap-2 min-w-0 flex-1">
            <AlertTriangle class="w-3.5 h-3.5 text-warning shrink-0 mt-0.5" />
            <span class="px-1.5 py-0.5 rounded text-[9px] font-bold shrink-0" :class="issueClass(m.issueType)">
              {{ m.issueType }}
            </span>
            <div class="min-w-0">
              <div class="font-mono text-ink truncate" :title="m.assetPath">{{ m.assetPath }}</div>
              <div class="text-[10px] text-ink-muted">{{ m.message }}</div>
            </div>
          </div>
          <div class="flex items-center gap-1 shrink-0">
            <button
              @click="copyPath(m.assetPath)"
              class="p-1 text-ink-muted hover:text-ink transition cursor-pointer"
              :title="t('engine.copy.asset.path')"
            >
              <Copy class="w-3 h-3" aria-hidden="true" />
            </button>
            <button
              @click="copyPath(m.metaPath)"
              class="p-1 text-ink-muted hover:text-ink transition cursor-pointer"
              :title="t('engine.copy.meta.path')"
            >
              <FileCode class="w-3 h-3" aria-hidden="true" />
            </button>
          </div>
        </div>
      </div>
      <div v-else-if="metaChecked" class="py-3 text-center text-xs text-success">
        {{ t("engine.meta.ok") }}
      </div>
    </div>

    <!-- Section D: Unity YAML merge -->
    <div class="space-y-3" data-tool-section="engine-unity-merge">
      <div class="flex items-center justify-between pb-2 border-b border-line">
        <div>
          <h3 class="text-sm font-bold text-ink flex items-center gap-2">
            <Wrench class="w-4 h-4 text-purple" />
            <span>{{ t("engine.yamlmerge.title") }}</span>
          </h3>
          <p class="text-xs text-ink-muted mt-0.5">
            {{ t("engine.yamlmerge.desc") }}
          </p>
        </div>
        <span
          class="px-2 py-0.5 rounded text-[10px] font-bold"
          :class="unityMerge?.isConfigured ? 'bg-success-bg text-success' : 'bg-warning-bg text-warning'"
        >
          {{ unityMerge?.isConfigured ? t("engine.configured") : t("engine.not.configured") }}
        </span>
      </div>

      <div v-if="unityMerge" class="grid grid-cols-2 lg:grid-cols-4 gap-2">
        <div class="p-2.5 rounded-lg border border-line bg-surface">
          <div class="text-[10px] text-ink-muted">{{ t("engine.yamlmerge.detected") }}</div>
          <div class="font-mono text-[10px] text-ink truncate" :title="unityMerge.detectedToolPath">
            {{ unityMerge.detectedToolPath || t("engine.not.detected") }}
          </div>
        </div>
        <div class="p-2.5 rounded-lg border border-line bg-surface">
          <div class="text-[10px] text-ink-muted">{{ t("engine.yamlmerge.path") }}</div>
          <div class="font-mono text-[10px] text-ink truncate" :title="unityMerge.configuredToolPath">
            {{ unityMerge.configuredToolPath || "—" }}
          </div>
        </div>
        <div class="p-2.5 rounded-lg border border-line bg-surface">
          <div class="text-[10px] text-ink-muted">{{ t("engine.yamlmerge.installed") }}</div>
          <div class="text-xs text-ink">{{ t("engine.yamlmerge.versions", { count: (unityMerge.installedVersions || []).length }) }}</div>
        </div>
        <div class="p-2.5 rounded-lg border border-line bg-surface">
          <div class="text-[10px] text-ink-muted">{{ t("engine.yamlmerge.rules") }}</div>
          <div class="text-xs" :class="unityMerge.hasGitattributes ? 'text-success' : 'text-warning'">
            {{ unityMerge.hasGitattributes ? t("engine.configured") : t("engine.missing") }}
          </div>
        </div>
      </div>

      <div class="flex flex-wrap items-center gap-2">
        <input
          v-model="mergeToolPath"
          type="text"
          :placeholder="t('engine.yamlmerge.placeholder')"
          class="flex-1 min-w-[260px] px-3 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
        />
        <label class="flex items-center gap-1.5 text-[11px] text-ink-muted cursor-pointer select-none">
          <input v-model="applyMergeAttributes" type="checkbox" class="rounded accent-purple" />
          <span>{{ t("engine.yamlmerge.write.rules") }}</span>
        </label>
        <button
          @click="handleConfigureUnityMerge"
          :disabled="isConfiguringMerge"
          class="px-3 py-1.5 rounded-lg bg-purple-strong text-white text-xs font-semibold transition cursor-pointer disabled:opacity-40 shrink-0"
        >
          {{ isConfiguringMerge ? t("engine.configuring") : t("engine.yamlmerge.configure") }}
        </button>
      </div>
    </div>

    <!-- Section E: Texture lint -->
    <div class="space-y-3" data-tool-section="engine-texture">
      <div class="flex items-center justify-between pb-2 border-b border-line">
        <div>
          <h3 class="text-sm font-bold text-ink flex items-center gap-2">
            <Image class="w-4 h-4 text-cyan" />
            <span>{{ t("engine.texture.title") }}</span>
          </h3>
          <p class="text-xs text-ink-muted mt-0.5">
            {{ t("engine.texture.desc") }}
          </p>
        </div>
        <button
          @click="handleLintTextures"
          :disabled="isLintingTextures"
          class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-40"
        >
          {{ isLintingTextures ? t("engine.auditing") : t("engine.texture.run") }}
        </button>
      </div>

      <div v-if="textureResult" class="space-y-2">
        <div class="flex items-center gap-3 text-[11px] text-ink-muted font-mono">
          <span>{{ t("engine.texture.checked", { count: textureResult.totalChecked }) }}</span>
          <span :class="textureResult.totalIssues > 0 ? 'text-warning' : 'text-success'">
            {{ t("engine.texture.issues", { count: textureResult.totalIssues }) }}
          </span>
        </div>
        <div v-if="(textureResult.items || []).length > 0" class="space-y-1 max-h-56 overflow-y-auto">
          <div
            v-for="item in textureResult.items || []"
            :key="item.path"
            class="flex items-center gap-2 px-2.5 py-1.5 rounded-lg border text-[11px]"
            :class="item.severity === 'ERROR' ? 'bg-danger-bg border-danger/20' : item.severity === 'WARNING' ? 'bg-warning-bg border-warning/20' : 'bg-surface border-line'"
          >
            <span
              class="px-1.5 py-0.5 rounded text-[9px] font-bold shrink-0"
              :class="item.severity === 'ERROR' ? 'bg-danger-bg text-danger' : item.severity === 'WARNING' ? 'bg-warning-bg text-warning' : 'bg-success-bg text-success'"
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
          {{ t("engine.texture.ok") }}
        </div>
      </div>
    </div>

    <!-- Section F: Engine runtime & cache -->
    <div class="space-y-3 pt-4" data-tool-section="engine-runtime">
      <div class="pb-2 border-b border-line">
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <Play class="w-4 h-4 text-success" />
          <span>{{ t("engine.launch.title") }}</span>
        </h3>
        <p class="text-xs text-ink-muted mt-0.5">
          {{ t("engine.launch.desc") }}
        </p>
      </div>
      <div class="flex flex-wrap items-center gap-2 rounded-xl border border-line bg-surface p-3">
        <span class="text-[11px] text-ink-muted">
          {{ t("engine.launch.current") }}
          <span class="font-mono text-ink">{{ engineInfo?.engineType || t("engine.not.detected") }}</span>
        </span>
        <button
          @click="handleLaunchEngine"
          :disabled="!engineKey || isLaunchingEngine"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-xs font-semibold transition cursor-pointer disabled:opacity-40"
        >
          <Play class="w-3.5 h-3.5" aria-hidden="true" />
          <span>{{ isLaunchingEngine ? t("engine.launching") : t("engine.launch.run") }}</span>
        </button>
        <button
          @click="handleCleanCaches"
          :disabled="!engineKey || isCleaningCaches"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-warning/30 text-xs font-semibold text-warning transition cursor-pointer disabled:opacity-40"
        >
          <Trash2 class="w-3.5 h-3.5" aria-hidden="true" />
          <span>{{ isCleaningCaches ? t("engine.cleaning") : t("engine.launch.clean") }}</span>
        </button>
        <span v-if="cacheCleanResult" class="text-[10px] text-ink-muted font-mono">
          {{ t("engine.cache.cleaned", { count: cacheCleanResult.cleanedFolders.length, size: cacheCleanResult.freedSizeStr }) }}
        </span>
      </div>

      <div class="rounded-xl border border-line bg-surface p-3 space-y-2">
        <div class="text-xs font-bold text-ink">{{ t("engine.artist.title") }}</div>
        <div class="flex flex-wrap items-center gap-2">
          <input
            v-model="artistMessage"
            type="text"
            :placeholder="t('engine.artist.message.placeholder')"
            class="flex-1 min-w-[220px] px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
          />
          <input
            v-model="artistTargetBranch"
            type="text"
            :placeholder="t('engine.artist.branch.placeholder')"
            class="w-44 px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
          />
          <button
            @click="handleArtistDelivery"
            :disabled="isArtistDelivering"
            class="px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-xs font-semibold transition cursor-pointer disabled:opacity-40"
          >
            {{ isArtistDelivering ? t("engine.artist.running") : t("engine.artist.run") }}
          </button>
        </div>
        <p class="text-[10px] text-ink-muted">
          {{ t("engine.artist.hint") }}
        </p>
        <template v-if="artistResult">
          <div class="grid grid-cols-4 gap-2">
            <div class="p-2 rounded-lg border border-line bg-canvas">
              <div class="text-[10px] text-ink-muted">{{ t("engine.artist.files") }}</div>
              <div class="text-sm font-bold font-mono text-ink">{{ artistResult.stagedAssetsCount }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas">
              <div class="text-[10px] text-ink-muted">{{ t("engine.artist.textures") }}</div>
              <div class="text-sm font-bold font-mono text-purple">{{ artistResult.texturesChecked }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas">
              <div class="text-[10px] text-ink-muted">{{ t("engine.artist.meshes") }}</div>
              <div class="text-sm font-bold font-mono text-cyan">{{ artistResult.meshesChecked }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas">
              <div class="text-[10px] text-ink-muted">{{ t("engine.artist.pushed") }}</div>
              <div class="text-sm font-bold" :class="artistResult.pushed ? 'text-success' : 'text-warning'">
                {{ artistResult.pushed ? t("engine.yes") : t("engine.no") }}
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
