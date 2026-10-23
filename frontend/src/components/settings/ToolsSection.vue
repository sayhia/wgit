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

async function loadExternalTools() {
  isLoadingTools.value = true;
  try {
    const repo = props.currentRepoPath;
    const [tools, cfg, web] = await Promise.all([
      gitService.detectExternalTools().catch(() => []),
      repo ? gitService.getExternalToolConfig(repo).catch(() => null) : null,
      repo ? gitService.getRemoteWebInfo(repo).catch(() => null) : null,
    ]);
    detectedTools.value = tools || [];
    if (cfg) {
      selectedDiffTool.value = cfg.diffTool || "cursor";
      selectedMergeTool.value = cfg.mergeTool || "cursor";
      customDiffCmd.value = cfg.customDiffCmd || "";
      customMergeCmd.value = cfg.customMergeCmd || "";
      isGlobalTools.value = cfg.isGlobal;
    }
    if (web) remoteWebInfo.value = web;
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
    notify("外部对比与合并工具配置已保存！");
  } catch (err) {
    notify("保存配置失败: " + (err as Error).message);
  } finally {
    isSavingTools.value = false;
  }
}

async function openWebHomepage() {
  if (!props.currentRepoPath) return;
  try {
    await gitService.openRepoInWeb(props.currentRepoPath);
  } catch (err) {
    notify("打开网页失败: " + (err as Error).message);
  }
}

watch(() => props.currentRepoPath, loadExternalTools, { immediate: true });
</script>

<template>
      <!-- 5. External Diff / Merge Tools & Web Forge -->
      <section  class="space-y-6">
        <div>
          <h2 class="text-sm font-bold text-ink">外部对比与合并工具 (External Diff & Merge Tools)</h2>
          <p class="text-xs text-ink-secondary mt-1">调用本地安装的专业编辑器或比对工具进行双栏差异复查与三方冲突合并。</p>
        </div>

        <!-- Detected tools grid -->
        <div class="p-5 rounded-2xl bg-surface border border-line space-y-4">
          <div class="flex items-center justify-between">
            <span class="text-xs font-bold text-ink">本机已检测到的外部工具</span>
            <button
              @click="loadExternalTools"
              class="flex items-center gap-1 text-[11px] text-accent hover:underline cursor-pointer"
            >
              <RefreshCw class="w-3 h-3" :class="{ 'animate-spin': isLoadingTools }" /> 重新扫描
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
                  {{ tool.isAvailable ? '已检测到' : '未安装' }}
                </span>
              </div>

              <p class="text-[10px] text-ink-muted leading-tight line-clamp-2">
                {{ tool.description }}
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
          <span class="text-xs font-bold text-ink">默认外部工具设定</span>

          <div class="grid grid-cols-2 gap-4">
            <div class="flex flex-col gap-1.5">
              <label class="text-xs text-ink font-medium">默认差异对比工具 (diff.tool)</label>
              <select
                v-model="selectedDiffTool"
                class="w-full px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink focus:outline-none focus:border-accent"
              >
                <option v-for="t in detectedTools.filter(x => x.supportsDiff)" :key="t.id" :value="t.id">
                  {{ t.name }} {{ t.isAvailable ? '(已安装)' : '(未安装)' }}
                </option>
                <option value="custom">自定义命令模板</option>
              </select>
            </div>

            <div class="flex flex-col gap-1.5">
              <label class="text-xs text-ink font-medium">默认冲突合并工具 (merge.tool)</label>
              <select
                v-model="selectedMergeTool"
                class="w-full px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink focus:outline-none focus:border-accent"
              >
                <option v-for="t in detectedTools.filter(x => x.supportsMerge)" :key="t.id" :value="t.id">
                  {{ t.name }} {{ t.isAvailable ? '(已安装)' : '(未安装)' }}
                </option>
                <option value="custom">自定义命令模板</option>
              </select>
            </div>
          </div>

          <div v-if="selectedDiffTool === 'custom'" class="flex flex-col gap-1.5 pt-2 border-t border-line">
            <label class="text-xs text-ink font-medium">自定义比对命令模板</label>
            <input
              v-model="customDiffCmd"
              type="text"
              placeholder="bcomp {left} {right}"
              class="w-full px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink font-mono"
            />
            <span class="text-[10px] text-ink-muted">支持变量: <code class="text-accent">{left}</code> (旧版本), <code class="text-accent">{right}</code> (新版本)</span>
          </div>

          <div v-if="selectedMergeTool === 'custom'" class="flex flex-col gap-1.5 pt-2 border-t border-line">
            <label class="text-xs text-ink font-medium">自定义合并命令模板</label>
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
              <span>应用于全局 Git 配置 (--global)</span>
            </label>

            <button
              @click="saveExternalTools"
              class="flex items-center gap-1.5 px-4 py-1.5 rounded-lg bg-accent-strong text-white font-semibold text-xs cursor-pointer shadow-sm hover:bg-accent-strong-hover"
              :disabled="isSavingTools"
            >
              <RefreshCw v-if="isSavingTools" class="w-3.5 h-3.5 animate-spin" />
              <span>保存工具配置</span>
            </button>
          </div>
        </div>

        <!-- Remote Web Forge Card -->
        <div v-if="remoteWebInfo" class="p-5 rounded-2xl bg-surface border border-line space-y-3">
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2">
              <Globe class="w-4 h-4 text-accent" />
              <span class="text-xs font-bold text-ink">远程代码托管平台 (Remote Web Forge)</span>
            </div>
            <span class="px-2 py-0.5 rounded text-[10px] font-bold uppercase bg-accent/15 text-accent border border-accent/20">
              {{ remoteWebInfo.provider }}
            </span>
          </div>

          <div class="p-3 rounded-xl bg-canvas border border-line flex items-center justify-between gap-4">
            <div class="flex flex-col min-w-0">
              <span class="text-[10px] text-ink-muted">远程仓库 URL ({{ remoteWebInfo.remoteName }})</span>
              <span class="text-xs font-mono text-ink truncate">{{ remoteWebInfo.remoteUrl }}</span>
            </div>

            <button
              v-if="remoteWebInfo.hasRemote"
              @click="openWebHomepage"
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-hover border border-line text-xs font-semibold text-ink transition cursor-pointer shrink-0"
            >
              <ExternalLink class="w-3.5 h-3.5" />
              <span>在网页中打开</span>
            </button>
          </div>
        </div>
      </section>

</template>
