<script setup lang="ts">
import { confirmAction } from "../../composables/useConfirm";
import { computed, ref } from "vue";
import { AlertTriangle, CheckCircle2, ChevronDown, ChevronRight, Play } from "lucide-vue-next";
import { gitService } from "../../services/gitService";

const props = defineProps<{
  currentRepoPath: string;
}>();

const emit = defineEmits<{
  (e: "notify", msg: string): void;
}>();

function notify(msg: string) {
  emit("notify", msg);
}

interface DemoField {
  key: string;
  label: string;
  placeholder: string;
  defaultValue?: string;
}

interface DemoTool {
  id: string;
  name: string;
  description: string;
  fields: DemoField[];
  real?: boolean;
  run: (args: Record<string, string>) => Promise<unknown>;
}

const tools: DemoTool[] = [
  {
    id: "fonts",
    name: "字体版权审计",
    description: "解析字体 name 表，检查版权与商用许可。",
    real: true,
    fields: [],
    run: () => gitService.auditFontLicenses(props.currentRepoPath),
  },
  {
    id: "lightmap",
    name: "光照烘焙审计",
    description: "解析 Unity 场景 / LightingSettings 的 Bake Resolution、Atlas 与压缩设置。",
    real: true,
    fields: [{ key: "level", label: "关卡路径（可选）", placeholder: "Assets/Levels/MainCity" }],
    run: (args) => gitService.auditLightmapBakeSettings(props.currentRepoPath, args.level || ""),
  },
  {
    id: "texturePacking",
    name: "PBR 贴图通道打包",
    description: "解析 PNG/JPEG/TGA 通道与 sRGB 标记，检查 ORM 打包、法线与显存浪费。",
    real: true,
    fields: [{ key: "dir", label: "扫描目录（可选）", placeholder: "Assets/Textures" }],
    run: (args) => gitService.auditTextureChannelPacking(props.currentRepoPath, args.dir || ""),
  },
  {
    id: "cycles",
    name: "资产循环依赖检测",
    description: "基于 Unity .meta GUID 扫描 Prefab / Material / Asset 的闭环依赖。",
    real: true,
    fields: [{ key: "dir", label: "扫描目录（可选）", placeholder: "Assets" }],
    run: (args) => gitService.detectAssetCyclicDependencies(props.currentRepoPath, args.dir || ""),
  },
  {
    id: "hotfixBackport",
    name: "热修复反向合流追踪",
    description: "检查 hotfix 分支提交是否完整回到主干。",
    real: true,
    fields: [
      { key: "hotfix", label: "Hotfix 分支", placeholder: "hotfix/v1.0.1", defaultValue: "hotfix/v1.0.1" },
      { key: "target", label: "目标分支", placeholder: "main", defaultValue: "main" },
    ],
    run: (args) =>
      gitService.traceHotfixBackportStatus(
        props.currentRepoPath,
        args.hotfix || "hotfix/v1.0.1",
        args.target || "main"
      ),
  },
  {
    id: "bundleDiff",
    name: "AssetBundle 差异",
    description: "解析 JSON / Unity .manifest 清单，对比 Bundle 增删改、体积与重复资产。",
    real: true,
    fields: [
      { key: "base", label: "Base Manifest", placeholder: "base_manifest.json", defaultValue: "base_manifest.json" },
      { key: "target", label: "Target Manifest", placeholder: "target_manifest.json", defaultValue: "target_manifest.json" },
    ],
    run: (args) =>
      gitService.analyzeBundleDiff(
        props.currentRepoPath,
        args.base || "base_manifest.json",
        args.target || "target_manifest.json"
      ),
  },
  {
    id: "audioBank",
    name: "Wwise / FMOD Bank 差分",
    description: "解析 Wwise BKHD / FMOD FSB5 头，对比 HIRC 对象、体积变化并扫描超大音频。",
    real: true,
    fields: [
      { key: "path", label: "Bank 路径", placeholder: "Audio/Main.bnk", defaultValue: "Audio/Main.bnk" },
      { key: "base", label: "基线引用", placeholder: "HEAD~1", defaultValue: "HEAD~1" },
      { key: "target", label: "目标引用", placeholder: "HEAD", defaultValue: "HEAD" },
    ],
    run: (args) =>
      gitService.auditAudioBankDiff(
        props.currentRepoPath,
        args.path || "Audio/Main.bnk",
        args.base || "HEAD~1",
        args.target || "HEAD"
      ),
  },
  {
    id: "worldLayers",
    name: "World Partition 数据层差异",
    description: "按 Git 引用比对 Unreal Data Layer 资产增删改；Actor 归属需 UE Editor 导出。",
    real: true,
    fields: [
      { key: "base", label: "基线引用", placeholder: "HEAD~1", defaultValue: "HEAD~1" },
      { key: "target", label: "目标引用", placeholder: "HEAD", defaultValue: "HEAD" },
    ],
    run: (args) =>
      gitService.diffWorldDataLayers(
        props.currentRepoPath,
        args.base || "HEAD~1",
        args.target || "HEAD"
      ),
  },
  {
    id: "offlinePatch",
    name: "离线补丁包生成",
    description: "生成离线补丁包元数据与 LFS 清单。",
    real: true,
    fields: [
      { key: "base", label: "基线引用", placeholder: "HEAD~1", defaultValue: "HEAD~1" },
      { key: "target", label: "目标引用", placeholder: "HEAD", defaultValue: "HEAD" },
      { key: "output", label: "输出路径（可选）", placeholder: "offline_patch.wgitbundle" },
    ],
    run: (args) =>
      gitService.generateOfflinePatchBundle(
        props.currentRepoPath,
        args.base || "HEAD~1",
        args.target || "HEAD",
        args.output || ""
      ),
  },
  {
    id: "shaderHeal",
    name: "Shader 自愈向导",
    description: "检查 shader include 与材质 shader GUID，可清理本地缓存。",
    real: true,
    fields: [
      { key: "action", label: "动作", placeholder: "HEAL_ALL / CLEAR_SHADER_CACHE", defaultValue: "HEAL_ALL" },
    ],
    run: (args) => gitService.shaderSelfHealingWizard(props.currentRepoPath, args.action || "HEAL_ALL"),
  },
];

const hasDemoTools = computed(() => tools.some((tool) => !tool.real));
const expanded = ref(false);
const selectedId = ref(tools[0]?.id || "");
const values = ref<Record<string, string>>({});
const isRunning = ref(false);
const resultJson = ref("");
const lastToolName = ref("");

const selectedTool = computed(() => tools.find((tool) => tool.id === selectedId.value) || tools[0]);

function fieldValue(field: DemoField) {
  const key = `${selectedId.value}.${field.key}`;
  return values.value[key] ?? field.defaultValue ?? "";
}

function setFieldValue(field: DemoField, value: string) {
  values.value[`${selectedId.value}.${field.key}`] = value;
}

async function handleRun() {
  const tool = selectedTool.value;
  if (!tool || !props.currentRepoPath || isRunning.value) return;
  if (tool.id === "shaderHeal") {
    const actionField = tool.fields[0];
    const action = (actionField ? fieldValue(actionField).trim() : "HEAL_ALL").toUpperCase() || "HEAL_ALL";
    if (
      (action === "HEAL_ALL" || action === "CLEAR_SHADER_CACHE") &&
      !(await confirmAction("将清理本地 Shader 缓存目录，确定继续吗？"))
    ) {
      return;
    }
  }
  isRunning.value = true;
  resultJson.value = "";
  lastToolName.value = tool.name;
  try {
    const args: Record<string, string> = {};
    tool.fields.forEach((field) => {
      args[field.key] = fieldValue(field).trim();
    });
    const result = await tool.run(args);
    resultJson.value = JSON.stringify(result ?? null, null, 2);
    notify(`${tool.name} 已执行${tool.real ? "" : "（演示数据）"}`);
  } catch (err: any) {
    resultJson.value = JSON.stringify({ error: err?.message || String(err) }, null, 2);
    notify(`${tool.name} 失败: ${err?.message || err}`);
  } finally {
    isRunning.value = false;
  }
}
</script>

<template>
  <div class="space-y-3 pt-4">
    <div class="flex items-center justify-between pb-2 border-b border-line">
      <div>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <AlertTriangle v-if="hasDemoTools" class="w-4 h-4 text-warning" />
          <CheckCircle2 v-else class="w-4 h-4 text-success" />
          <span>引擎高级诊断与预览接口</span>
          <span
            v-if="hasDemoTools"
            class="px-1.5 py-0.2 rounded bg-warning/15 text-warning text-[9px] font-bold"
          >含演示接口</span>
          <span v-else class="px-1.5 py-0.2 rounded bg-success/15 text-success text-[9px] font-bold">全部真实执行</span>
        </h3>
        <p class="text-xs text-ink-muted mt-0.5">
          <template v-if="hasDemoTools">标注「演示」的接口返回后端样例，不代表真实工程状态；标注「真实」的执行本地扫描或生成。</template>
          <template v-else>所有接口均读取本地文件或 Git 数据，返回真实结果与限制说明，不使用固定样例。</template>
        </p>
      </div>
      <button
        @click="expanded = !expanded"
        class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer shrink-0"
      >
        <component :is="expanded ? ChevronDown : ChevronRight" class="w-3.5 h-3.5" />
        <span>{{ expanded ? "收起" : "展开诊断接口" }}</span>
      </button>
    </div>

    <div
      v-if="expanded"
      class="rounded-xl border p-3 space-y-3"
      :class="hasDemoTools ? 'border-warning/25 bg-warning/5' : 'border-line bg-surface/40'"
    >
      <div class="grid grid-cols-1 md:grid-cols-3 gap-2">
        <select
          v-model="selectedId"
          class="px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink focus:outline-none focus:border-accent cursor-pointer"
        >
          <option v-for="tool in tools" :key="tool.id" :value="tool.id">{{ tool.name }}</option>
        </select>
        <div class="md:col-span-2 flex items-center gap-2 text-[11px] text-ink-muted self-center">
          <span
            class="px-1.5 py-0.2 rounded text-[9px] font-bold shrink-0"
            :class="selectedTool?.real ? 'bg-success/15 text-success' : 'bg-warning/15 text-warning'"
          >
            {{ selectedTool?.real ? "真实执行" : "演示数据" }}
          </span>
          <span>{{ selectedTool?.description }}</span>
        </div>
      </div>

      <div v-if="selectedTool && selectedTool.fields.length > 0" class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-2">
        <label v-for="field in selectedTool.fields" :key="field.key" class="flex flex-col gap-1">
          <span class="text-[10px] text-ink-muted">{{ field.label }}</span>
          <input
            :value="fieldValue(field)"
            @input="setFieldValue(field, ($event.target as HTMLInputElement).value)"
            type="text"
            :placeholder="field.placeholder"
            class="px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
          />
        </label>
      </div>

      <div class="flex items-center gap-2">
        <button
          @click="handleRun"
          :disabled="isRunning || !currentRepoPath"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-warning-strong hover:opacity-90 text-warning-contrast text-xs font-semibold transition cursor-pointer disabled:opacity-40"
        >
          <Play class="w-3.5 h-3.5" />
          <span>{{ isRunning ? "执行中..." : selectedTool?.real ? "执行真实接口" : "运行演示接口" }}</span>
        </button>
        <span class="text-[10px] text-ink-muted">结果以只读 JSON 展示，便于核对字段。</span>
      </div>

      <div v-if="resultJson" class="space-y-1">
        <div class="text-[10px] text-ink-muted">返回值 · {{ lastToolName }}</div>
        <pre class="max-h-80 overflow-auto p-3 rounded-lg bg-canvas border border-line text-[10px] leading-relaxed font-mono whitespace-pre-wrap break-all">{{ resultJson }}</pre>
      </div>
    </div>
  </div>
</template>
