<script setup lang="ts">
import { ref, watch } from "vue";
import {
  RefreshCw,
  ExternalLink,
  Code,
  CheckCircle2,
  XCircle,
} from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import type { ExternalToolInfo, RemoteWebInfo } from "../../types";
import { backendText, t } from "../../i18n";
import SettingsPageHeader from "./SettingsPageHeader.vue";
import SettingsCard from "./SettingsCard.vue";
import TeamWebhookSection from "./TeamWebhookSection.vue";

const props = defineProps<{ currentRepoPath: string }>();
const emit = defineEmits<{ (e: "notify", msg: string): void }>();

function notify(msg: string) {
  emit("notify", msg);
}

const detectedTools = ref<ExternalToolInfo[]>([]);
const selectedDiffTool = ref("cursor");
const selectedMergeTool = ref("cursor");
const customDiffCmd = ref("");
const customMergeCmd = ref("");
const isGlobalTools = ref(false);
const isSavingTools = ref(false);
const isLoadingTools = ref(false);
const remoteWebInfo = ref<RemoteWebInfo | null>(null);

const unityMergeCustomPath = ref("");
const unityApplyGitattributes = ref(true);
const isConfiguringUnityMerge = ref(false);

async function configureUnityMerge() {
  if (!props.currentRepoPath || isConfiguringUnityMerge.value) return;
  isConfiguringUnityMerge.value = true;
  try {
    await gitService.configureUnityYamlMerge(
      props.currentRepoPath,
      unityMergeCustomPath.value.trim(),
      unityApplyGitattributes.value
    );
    notify(t("settings.tools.unity.yaml.success"));
  } catch (err: any) {
    notify(t("settings.tools.unity.yaml.failed", { error: err?.message || err }));
  } finally {
    isConfiguringUnityMerge.value = false;
  }
}

/**
 * Product blurbs live in the catalog; the backend description is the fallback
 * for tools the catalog does not know yet (new entries, user-added tools).
 */
const TOOL_DESCRIPTION_KEYS: Record<string, string> = {
  cursor: "settings.tools.desc.cursor",
  vscode: "settings.tools.desc.vscode",
  beyondcompare: "settings.tools.desc.beyondcompare",
  idea: "settings.tools.desc.idea",
  sublimemerge: "settings.tools.desc.sublimemerge",
  opendiff: "settings.tools.desc.opendiff",
  meld: "settings.tools.desc.meld",
};

function toolDescription(tool: ExternalToolInfo): string {
  return backendText(TOOL_DESCRIPTION_KEYS[tool.id], tool.description);
}

async function loadExternalTools() {
  isLoadingTools.value = true;
  try {
    const repo = props.currentRepoPath;
    const [toolsRes, cfgRes, webRes] = await Promise.allSettled([
      gitService.detectExternalTools(),
      repo ? gitService.getExternalToolConfig(repo) : Promise.resolve(null),
      repo ? gitService.getRemoteWebInfo(repo) : Promise.resolve(null),
    ]);
    const errors: string[] = [];
    if (toolsRes.status === "fulfilled") {
      detectedTools.value = toolsRes.value || [];
    } else {
      errors.push(String((toolsRes.reason as any)?.message || toolsRes.reason));
    }
    if (cfgRes.status === "fulfilled") {
      const cfg = cfgRes.value;
      if (cfg) {
        selectedDiffTool.value = cfg.diffTool || "cursor";
        selectedMergeTool.value = cfg.mergeTool || "cursor";
        customDiffCmd.value = cfg.customDiffCmd || "";
        customMergeCmd.value = cfg.customMergeCmd || "";
        isGlobalTools.value = cfg.isGlobal;
      }
    } else {
      errors.push(String((cfgRes.reason as any)?.message || cfgRes.reason));
    }
    if (webRes.status === "fulfilled") {
      if (webRes.value) remoteWebInfo.value = webRes.value;
    } else {
      errors.push(String((webRes.reason as any)?.message || webRes.reason));
    }
    if (errors.length > 0) {
      notify(t("settings.tools.load.failed", { error: errors.join("; ") }));
    }
  } finally {
    isLoadingTools.value = false;
  }
}

async function saveExternalTools() {
  if (!props.currentRepoPath) return;
  isSavingTools.value = true;
  try {
    await gitService.setExternalToolConfig(
      props.currentRepoPath,
      selectedDiffTool.value,
      selectedMergeTool.value,
      customDiffCmd.value.trim(),
      customMergeCmd.value.trim(),
      isGlobalTools.value
    );
    notify(t("settings.tools.saved"));
  } catch (err) {
    notify(t("settings.tools.save.failed", { error: (err as Error).message }));
  } finally {
    isSavingTools.value = false;
  }
}

async function openWebHomepage() {
  if (!props.currentRepoPath) return;
  try {
    await gitService.openRepoInWeb(props.currentRepoPath);
  } catch (err) {
    notify(t("settings.tools.open.web.failed", { error: (err as Error).message }));
  }
}

watch(() => props.currentRepoPath, loadExternalTools, { immediate: true });
</script>

<template>
  <section class="space-y-4 min-w-0">
    <SettingsPageHeader :title="t('settings.tools.title')" :description="t('settings.tools.intro')" />

    <SettingsCard class="@container">
      <template #header>
        <h3 class="text-xs font-semibold text-ink">{{ t("settings.tools.detected") }}</h3>
      </template>
      <template #actions>
        <button
          type="button"
          @click="loadExternalTools"
          :disabled="isLoadingTools"
          class="flex items-center gap-1.5 text-xs text-accent hover:underline cursor-pointer disabled:opacity-50"
        >
          <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isLoadingTools }" aria-hidden="true" />
          {{ t("settings.tools.rescan") }}
        </button>
      </template>
      <div class="grid grid-cols-1 @min-[40rem]:grid-cols-2 gap-3">
        <div
          v-for="tool in detectedTools"
          :key="tool.id"
          class="min-w-0 p-3 rounded-xl border flex flex-col gap-2"
          :class="tool.isAvailable ? 'bg-canvas border-line/80' : 'bg-canvas border-line/50'"
        >
          <div class="flex flex-wrap items-start justify-between gap-2">
            <div class="min-w-0 flex items-center gap-2">
              <Code class="w-4 h-4 text-accent shrink-0" aria-hidden="true" />
              <div class="min-w-0">
                <h4 class="text-xs font-semibold text-ink break-words">{{ tool.name }}</h4>
                <span class="text-[10px] text-ink-muted font-mono break-all">{{ tool.id }}</span>
              </div>
            </div>
            <span class="px-2 py-0.5 rounded text-[10px] font-medium flex items-center gap-1 shrink-0"
              :class="tool.isAvailable ? 'bg-success-bg text-success' : 'bg-surface text-ink-muted'">
              <CheckCircle2 v-if="tool.isAvailable" class="w-3 h-3" aria-hidden="true" />
              <XCircle v-else class="w-3 h-3" aria-hidden="true" />
              {{ tool.isAvailable ? t("settings.tools.available") : t("settings.tools.missing") }}
            </span>
          </div>
          <p class="text-[11px] text-ink-secondary leading-relaxed">{{ toolDescription(tool) }}</p>
          <div class="flex flex-wrap items-center gap-1 text-[10px] font-mono text-ink-muted mt-auto">
            <span v-if="tool.supportsDiff" class="px-1.5 py-0.5 rounded bg-surface border border-line">Diff</span>
            <span v-if="tool.supportsMerge" class="px-1.5 py-0.5 rounded bg-surface border border-line">Merge</span>
            <span class="min-w-0 break-all">{{ tool.executable || '—' }}</span>
          </div>
        </div>
      </div>
    </SettingsCard>

    <SettingsCard class="@container">
      <template #header><h3 class="text-xs font-semibold text-ink">{{ t("settings.tools.defaults") }}</h3></template>
      <div class="space-y-4">
        <div class="grid grid-cols-1 @min-[36rem]:grid-cols-2 gap-4">
          <div class="min-w-0 flex flex-col gap-1.5">
            <label for="settings-diff-tool" class="text-xs text-ink font-medium">{{ t("settings.tools.diff.tool") }}</label>
            <select id="settings-diff-tool" v-model="selectedDiffTool" class="w-full min-w-0 px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink focus:outline-none focus:border-accent">
                <option v-for="tool in detectedTools.filter(x => x.supportsDiff)" :key="tool.id" :value="tool.id">
                {{ tool.name }} {{ tool.isAvailable ? t("settings.tools.installed") : t("settings.tools.not.installed") }}
              </option>
              <option value="custom">{{ t("settings.tools.custom") }}</option>
            </select>
          </div>
          <div class="min-w-0 flex flex-col gap-1.5">
            <label for="settings-merge-tool" class="text-xs text-ink font-medium">{{ t("settings.tools.merge.tool") }}</label>
            <select id="settings-merge-tool" v-model="selectedMergeTool" class="w-full min-w-0 px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink focus:outline-none focus:border-accent">
                <option v-for="tool in detectedTools.filter(x => x.supportsMerge)" :key="tool.id" :value="tool.id">
                {{ tool.name }} {{ tool.isAvailable ? t("settings.tools.installed") : t("settings.tools.not.installed") }}
              </option>
              <option value="custom">{{ t("settings.tools.custom") }}</option>
            </select>
          </div>
        </div>
        <div v-if="selectedDiffTool === 'custom'" class="flex flex-col gap-1.5">
          <label for="settings-custom-diff" class="text-xs text-ink font-medium">{{ t("settings.tools.custom.diff") }}</label>
          <input id="settings-custom-diff" v-model="customDiffCmd" type="text" :aria-label="t('settings.tools.custom.diff')" placeholder="bcomp {left} {right}" class="w-full px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent" />
          <span class="text-[11px] text-ink-muted leading-relaxed">{{ t("settings.tools.vars") }} <code class="text-accent">{left}</code> {{ t("settings.tools.vars.left") }}, <code class="text-accent">{right}</code> {{ t("settings.tools.vars.right") }}</span>
        </div>
        <div v-if="selectedMergeTool === 'custom'" class="flex flex-col gap-1.5">
          <label for="settings-custom-merge" class="text-xs text-ink font-medium">{{ t("settings.tools.custom.merge") }}</label>
          <input id="settings-custom-merge" v-model="customMergeCmd" type="text" :aria-label="t('settings.tools.custom.merge')" placeholder="code --wait --merge {theirs} {ours} {base} {result}" class="w-full px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent" />
        </div>
        <div class="flex flex-wrap items-center justify-between gap-3 pt-3 border-t border-line/60">
          <label class="flex items-start gap-2 text-xs text-ink-secondary cursor-pointer">
            <input v-model="isGlobalTools" type="checkbox" class="mt-0.5 shrink-0 rounded border-line bg-canvas text-accent focus:ring-0" />
            <span>{{ t("settings.tools.global") }}</span>
          </label>
          <button type="button" @click="saveExternalTools" :disabled="isSavingTools" class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-accent-strong text-accent-contrast font-semibold text-xs cursor-pointer hover:bg-accent-strong-hover disabled:opacity-50 shrink-0">
            <RefreshCw v-if="isSavingTools" class="w-3.5 h-3.5 animate-spin" aria-hidden="true" />
            <span>{{ t("settings.tools.save") }}</span>
          </button>
        </div>
      </div>
    </SettingsCard>

    <SettingsCard v-if="remoteWebInfo">
      <template #header><h3 class="text-xs font-semibold text-ink">{{ t("settings.tools.remote.title") }}</h3></template>
      <template #actions><span class="px-2 py-0.5 rounded text-[10px] font-medium bg-accent-bg text-accent">{{ remoteWebInfo.provider }}</span></template>
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div class="min-w-0 flex-1 basis-48">
          <div class="text-[11px] text-ink-muted">{{ t("settings.tools.remote.url", { name: remoteWebInfo.remoteName }) }}</div>
          <div class="mt-1 text-xs font-mono text-ink break-all">{{ remoteWebInfo.remoteUrl }}</div>
        </div>
        <button v-if="remoteWebInfo.hasRemote" type="button" @click="openWebHomepage" class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-hover border border-line text-xs font-medium text-ink cursor-pointer shrink-0">
          <ExternalLink class="w-3.5 h-3.5" aria-hidden="true" /><span>{{ t("settings.tools.remote.open") }}</span>
        </button>
      </div>
    </SettingsCard>

    <SettingsCard>
      <template #header><h3 class="text-xs font-semibold text-ink">{{ t("settings.tools.unity.yaml.title") }}</h3></template>
      <template #actions><span class="px-2 py-0.5 rounded text-[10px] font-medium bg-purple-bg text-purple">Unity</span></template>
      <div class="space-y-3">
        <p class="text-xs text-ink-secondary leading-relaxed">{{ t("settings.tools.unity.yaml.desc") }}</p>
        <input v-model="unityMergeCustomPath" type="text" :aria-label="t('settings.tools.unity.yaml.placeholder')" :placeholder="t('settings.tools.unity.yaml.placeholder')" class="w-full px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent" />
        <div class="flex flex-wrap items-center justify-between gap-3">
          <label class="flex items-start gap-2 text-xs text-ink-secondary cursor-pointer">
            <input v-model="unityApplyGitattributes" type="checkbox" class="mt-0.5 shrink-0 rounded border-line bg-canvas text-accent focus:ring-0" />
            <span>{{ t("settings.tools.unity.yaml.attributes") }}</span>
          </label>
          <button type="button" @click="configureUnityMerge" :disabled="isConfiguringUnityMerge || !currentRepoPath" class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-accent-strong text-accent-contrast hover:bg-accent-strong-hover font-semibold text-xs cursor-pointer transition disabled:opacity-50 shrink-0">
            <RefreshCw v-if="isConfiguringUnityMerge" class="w-3.5 h-3.5 animate-spin" aria-hidden="true" />
            <span>{{ isConfiguringUnityMerge ? t("settings.tools.unity.yaml.configuring") : t("settings.tools.unity.yaml.configure") }}</span>
          </button>
        </div>
      </div>
    </SettingsCard>
    <TeamWebhookSection @notify="notify" />
  </section>
</template>
