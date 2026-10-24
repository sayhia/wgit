<script setup lang="ts">
import { ref, watch } from "vue";
import {
  Globe,
  RefreshCw,
  ExternalLink,
  Code,
  CheckCircle2,
  XCircle,
} from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import type { ExternalToolInfo, RemoteWebInfo } from "../../types";
import { backendText, t } from "../../i18n";

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
      <!-- 5. External Diff / Merge Tools & Web Forge -->
      <section  class="space-y-6">
        <div>
          <h2 class="text-sm font-bold text-ink">{{ t("settings.tools.title") }}</h2>
          <p class="text-xs text-ink-secondary mt-1">{{ t("settings.tools.intro") }}</p>
        </div>

        <!-- Detected tools grid -->
        <div class="p-5 rounded-2xl bg-surface border border-line space-y-4">
          <div class="flex items-center justify-between">
            <span class="text-xs font-bold text-ink">{{ t("settings.tools.detected") }}</span>
            <button
              @click="loadExternalTools"
              class="flex items-center gap-1 text-[11px] text-accent hover:underline cursor-pointer"
            >
              <RefreshCw class="w-3 h-3" :class="{ 'animate-spin': isLoadingTools }" />
              {{ t("settings.tools.rescan") }}
            </button>
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div
              v-for="tool in detectedTools"
              :key="tool.id"
              class="p-3 rounded-xl border transition-all flex flex-col justify-between gap-2"
              :class="tool.isAvailable ? 'bg-canvas border-line/80' : 'bg-surface/40 border-line/30 opacity-60'"
            >
              <div class="flex items-start justify-between">
                <div class="flex items-center gap-2">
                  <div
                    class="w-7 h-7 rounded-lg flex items-center justify-center font-bold text-xs"
                    :class="tool.isAvailable ? 'bg-accent/15 text-accent' : 'bg-canvas text-ink-muted'"
                  >
                    <Code class="w-3.5 h-3.5" />
                  </div>
                  <div>
                    <h4 class="text-xs font-bold text-ink">{{ tool.name }}</h4>
                    <span class="text-[10px] text-ink-muted font-mono block truncate max-w-[170px]">
                      {{ tool.id }}
                    </span>
                  </div>
                </div>

                <span
                  class="px-2 py-0.5 rounded text-[10px] font-semibold flex items-center gap-1"
                  :class="tool.isAvailable ? 'bg-success/15 text-success' : 'bg-canvas text-ink-muted'"
                >
                  <CheckCircle2 v-if="tool.isAvailable" class="w-2.5 h-2.5" />
                  <XCircle v-else class="w-2.5 h-2.5" />
                  {{ tool.isAvailable ? t("settings.tools.available") : t("settings.tools.missing") }}
                </span>
              </div>

              <p class="text-[10px] text-ink-muted leading-tight line-clamp-2">
                {{ toolDescription(tool) }}
              </p>

              <div class="flex items-center gap-1 text-[9px] font-mono text-ink-muted truncate">
                <span v-if="tool.supportsDiff" class="px-1.5 py-0.5 rounded bg-surface border border-line">Diff</span>
                <span v-if="tool.supportsMerge" class="px-1.5 py-0.5 rounded bg-surface border border-line">Merge</span>
                <span class="truncate ml-1">{{ tool.executable || '—' }}</span>
              </div>
            </div>
          </div>
        </div>

        <!-- Tool Configuration Card -->
        <div class="p-5 rounded-2xl bg-surface border border-line space-y-4">
          <span class="text-xs font-bold text-ink">{{ t("settings.tools.defaults") }}</span>

          <div class="grid grid-cols-2 gap-4">
            <div class="flex flex-col gap-1.5">
              <label class="text-xs text-ink font-medium">{{ t("settings.tools.diff.tool") }}</label>
              <select
                v-model="selectedDiffTool"
                class="w-full px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink focus:outline-none focus:border-accent"
              >
                <option v-for="tool in detectedTools.filter(x => x.supportsDiff)" :key="tool.id" :value="tool.id">
                  {{ tool.name }}
                  {{ tool.isAvailable ? t("settings.tools.installed") : t("settings.tools.not.installed") }}
                </option>
                <option value="custom">{{ t("settings.tools.custom") }}</option>
              </select>
            </div>

            <div class="flex flex-col gap-1.5">
              <label class="text-xs text-ink font-medium">{{ t("settings.tools.merge.tool") }}</label>
              <select
                v-model="selectedMergeTool"
                class="w-full px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink focus:outline-none focus:border-accent"
              >
                <option v-for="tool in detectedTools.filter(x => x.supportsMerge)" :key="tool.id" :value="tool.id">
                  {{ tool.name }}
                  {{ tool.isAvailable ? t("settings.tools.installed") : t("settings.tools.not.installed") }}
                </option>
                <option value="custom">{{ t("settings.tools.custom") }}</option>
              </select>
            </div>
          </div>

          <div v-if="selectedDiffTool === 'custom'" class="flex flex-col gap-1.5 pt-2 border-t border-line">
            <label class="text-xs text-ink font-medium">{{ t("settings.tools.custom.diff") }}</label>
            <input
              v-model="customDiffCmd"
              type="text"
              placeholder="bcomp {left} {right}"
              class="w-full px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink font-mono"
            />
            <span class="text-[10px] text-ink-muted">
              {{ t("settings.tools.vars") }} <code class="text-accent">{left}</code>
              {{ t("settings.tools.vars.left") }}, <code class="text-accent">{right}</code>
              {{ t("settings.tools.vars.right") }}
            </span>
          </div>

          <div v-if="selectedMergeTool === 'custom'" class="flex flex-col gap-1.5 pt-2 border-t border-line">
            <label class="text-xs text-ink font-medium">{{ t("settings.tools.custom.merge") }}</label>
            <input
              v-model="customMergeCmd"
              type="text"
              placeholder="code --wait --merge {theirs} {ours} {base} {result}"
              class="w-full px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink font-mono"
            />
          </div>

          <div class="flex items-center justify-between pt-3 border-t border-line">
            <label class="flex items-center gap-2 text-xs text-ink-secondary cursor-pointer">
              <input
                v-model="isGlobalTools"
                type="checkbox"
                class="rounded border-line bg-canvas text-accent focus:ring-0"
              />
              <span>{{ t("settings.tools.global") }}</span>
            </label>

            <button
              @click="saveExternalTools"
              class="flex items-center gap-1.5 px-4 py-1.5 rounded-lg bg-accent-strong text-accent-contrast font-semibold text-xs cursor-pointer shadow-sm hover:bg-accent-strong-hover"
              :disabled="isSavingTools"
            >
              <RefreshCw v-if="isSavingTools" class="w-3.5 h-3.5 animate-spin" />
              <span>{{ t("settings.tools.save") }}</span>
            </button>
          </div>
        </div>

        <!-- Remote Web Forge Card -->
        <div v-if="remoteWebInfo" class="p-5 rounded-2xl bg-surface border border-line space-y-3">
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2">
              <Globe class="w-4 h-4 text-accent" />
              <span class="text-xs font-bold text-ink">{{ t("settings.tools.remote.title") }}</span>
            </div>
            <span class="px-2 py-0.5 rounded text-[10px] font-bold uppercase bg-accent/15 text-accent border border-accent/20">
              {{ remoteWebInfo.provider }}
            </span>
          </div>

          <div class="p-3 rounded-xl bg-canvas border border-line flex items-center justify-between gap-4">
            <div class="flex flex-col min-w-0">
              <span class="text-[10px] text-ink-muted">{{
                t("settings.tools.remote.url", { name: remoteWebInfo.remoteName })
              }}</span>
              <span class="text-xs font-mono text-ink truncate">{{ remoteWebInfo.remoteUrl }}</span>
            </div>

            <button
              v-if="remoteWebInfo.hasRemote"
              @click="openWebHomepage"
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-hover border border-line text-xs font-semibold text-ink transition cursor-pointer shrink-0"
            >
              <ExternalLink class="w-3.5 h-3.5" />
              <span>{{ t("settings.tools.remote.open") }}</span>
            </button>
          </div>
        </div>
      </section>

</template>
