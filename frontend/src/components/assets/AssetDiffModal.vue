<script setup lang="ts">
import { computed, nextTick, ref, watch } from "vue";
import {
  X,
  RefreshCw,
  Columns2,
  Blend,
  MoveHorizontal,
  ScanEye,
  ImageOff,
  FileBox,
  Music,
  Network,
  UserRound,
} from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import { useModalA11y } from "../../composables/useModalA11y";
import { checkDiffEligibility, pixelDelta, isChangedPixel, dimmedGray, DIFF_HIGHLIGHT } from "../../utils/assetDiff";
import type { AssetDiffResult, AssetImpactResult } from "../../types";

const props = defineProps<{
  open: boolean;
  repoPath: string;
  path: string;
  oldRef: string;
  newRef: string;
}>();

const emit = defineEmits<{ (e: "update:open", value: boolean): void }>();

type Mode = "side" | "overlay" | "swipe" | "diff";

const mode = ref<Mode>("side");
const data = ref<AssetDiffResult | null>(null);
const loading = ref(false);
const error = ref("");
const opacity = ref(50);
const swipePct = ref(50);
const diffUrl = ref("");
const diffComputing = ref(false);
const diffReason = ref("");

// Impact analysis: what breaks if this asset is renamed or deleted.
const showImpact = ref(false);
const impact = ref<AssetImpactResult | null>(null);
const isLoadingImpact = ref(false);
const impactError = ref("");

async function loadImpact() {
  if (!props.repoPath || !props.path || isLoadingImpact.value) return;
  isLoadingImpact.value = true;
  impactError.value = "";
  try {
    impact.value = await gitService.analyzeAssetImpact(props.repoPath, props.path, 2);
  } catch (err: any) {
    impactError.value = err?.message || String(err);
  } finally {
    isLoadingImpact.value = false;
  }
}

function toggleImpact() {
  showImpact.value = !showImpact.value;
  if (showImpact.value && !impact.value) void loadImpact();
}

function impactRiskMeta(level: string) {
  switch (level) {
    case "high":
      return { label: "高影响", cls: "bg-danger/15 text-danger" };
    case "medium":
      return { label: "中等", cls: "bg-warning/15 text-warning" };
    case "low":
      return { label: "较小", cls: "bg-success/15 text-success" };
    case "unknown":
      return { label: "分析不完整", cls: "bg-warning/15 text-warning" };
    default:
      return { label: "无引用", cls: "bg-surface text-ink-muted border border-line" };
  }
}

const { containerRef: dialogRef } = useModalA11y(
  () => props.open,
  () => emit("update:open", false)
);

const bothPreviewable = computed(
  () => Boolean(data.value?.old.previewable) && Boolean(data.value?.new.previewable)
);

const modes: { id: Mode; label: string; icon: any; needBoth: boolean }[] = [
  { id: "side", label: "并排", icon: Columns2, needBoth: true },
  { id: "overlay", label: "叠加", icon: Blend, needBoth: true },
  { id: "swipe", label: "滑动", icon: MoveHorizontal, needBoth: true },
  { id: "diff", label: "差值", icon: ScanEye, needBoth: true },
];

function selectMode(next: Mode) {
  mode.value = next;
  if (next === "diff" && !diffUrl.value) void computeDiff();
}

function loadImage(src: string): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const img = new Image();
    img.onload = () => resolve(img);
    img.onerror = () => reject(new Error("图片解码失败"));
    img.src = src;
  });
}

// Pixel diff is only meaningful when both sides share a canvas, and only
// affordable below a few megapixels (the loop is O(w*h) in JS).
async function computeDiff() {
  const d = data.value;
  if (!d?.old.dataUrl || !d?.new.dataUrl || diffComputing.value) return;

  const eligibility = checkDiffEligibility(d.old.width, d.old.height, d.new.width, d.new.height);
  if (!eligibility.ok) {
    diffReason.value = eligibility.reason;
    diffUrl.value = "";
    return;
  }

  diffComputing.value = true;
  diffReason.value = "";
  try {
    const [a, b] = await Promise.all([loadImage(d.old.dataUrl), loadImage(d.new.dataUrl)]);
    const w = a.naturalWidth;
    const h = a.naturalHeight;
    const canvasA = document.createElement("canvas");
    canvasA.width = w;
    canvasA.height = h;
    const ctxA = canvasA.getContext("2d")!;
    ctxA.drawImage(a, 0, 0);
    const da = ctxA.getImageData(0, 0, w, h).data;

    const canvasB = document.createElement("canvas");
    canvasB.width = w;
    canvasB.height = h;
    const ctxB = canvasB.getContext("2d")!;
    ctxB.drawImage(b, 0, 0);
    const db = ctxB.getImageData(0, 0, w, h).data;

    const out = ctxA.createImageData(w, h);
    for (let i = 0; i < da.length; i += 4) {
      const r0 = da[i]!;
      const g0 = da[i + 1]!;
      const b0 = da[i + 2]!;
      const r1 = db[i]!;
      const g1 = db[i + 1]!;
      const b1 = db[i + 2]!;
      if (isChangedPixel(pixelDelta(r0, g0, b0, r1, g1, b1))) {
        out.data[i] = DIFF_HIGHLIGHT.r;
        out.data[i + 1] = DIFF_HIGHLIGHT.g;
        out.data[i + 2] = DIFF_HIGHLIGHT.b;
        out.data[i + 3] = 255;
      } else {
        // Dimmed grayscale base so the changed pixels pop.
        const g = dimmedGray(r0, g0, b0);
        out.data[i] = g;
        out.data[i + 1] = g;
        out.data[i + 2] = g;
        out.data[i + 3] = 255;
      }
    }
    ctxA.putImageData(out, 0, 0);
    diffUrl.value = canvasA.toDataURL("image/png");
  } catch (err: any) {
    diffReason.value = err?.message || "差值计算失败";
  } finally {
    diffComputing.value = false;
  }
}

async function load() {
  if (!props.repoPath || !props.path) return;
  loading.value = true;
  error.value = "";
  data.value = null;
  diffUrl.value = "";
  diffReason.value = "";
  mode.value = "side";
  impact.value = null;
  showImpact.value = false;
  try {
    data.value = await gitService.getAssetDiff(
      props.repoPath,
      props.path,
      props.oldRef || "HEAD",
      props.newRef || "WORKTREE"
    );
    if (data.value?.old.previewable && data.value?.new.previewable) {
      await nextTick();
    }
  } catch (err: any) {
    error.value = err?.message || String(err);
  } finally {
    loading.value = false;
  }
}

watch(
  () => [props.open, props.path, props.oldRef, props.newRef],
  () => {
    if (props.open) void load();
  },
  { immediate: true }
);

const kindMeta = computed(() => {
  switch (data.value?.kind) {
    case "image":
      return { label: "图片资产", icon: ScanEye, cls: "bg-accent/15 text-accent" };
    case "audio":
      return { label: "音频资产", icon: Music, cls: "bg-warning/15 text-warning" };
    case "model":
      return { label: "模型资产", icon: FileBox, cls: "bg-purple/15 text-purple" };
    default:
      return { label: "二进制文件", icon: ImageOff, cls: "bg-surface text-ink-muted border border-line" };
  }
});

function deltaClass(delta: number) {
  if (delta > 0) return "text-warning";
  if (delta < 0) return "text-success";
  return "text-ink-muted";
}
</script>

<template>
  <div
    v-if="open"
    ref="dialogRef"
    role="dialog"
    aria-modal="true"
    aria-label="资产差异对比"
    class="fixed inset-0 bg-black/65 backdrop-blur-sm flex items-center justify-center z-50 p-4"
  >
    <div class="w-[980px] max-w-full max-h-[92vh] overflow-hidden rounded-2xl bg-surface border border-line shadow-2xl flex flex-col">
      <!-- Header -->
      <div class="flex items-start justify-between gap-3 px-5 py-3.5 border-b border-line shrink-0">
        <div class="min-w-0">
          <div class="flex items-center gap-2 flex-wrap">
            <span class="px-1.5 py-0.5 rounded text-[9px] font-bold shrink-0" :class="kindMeta.cls">
              {{ kindMeta.label }}
            </span>
            <span class="font-mono text-xs font-bold text-ink truncate" :title="path">{{ path }}</span>
          </div>
          <p class="text-[11px] text-ink-muted mt-1">
            <span class="font-mono">{{ oldRef || "HEAD" }}</span>
            <span class="mx-1">→</span>
            <span class="font-mono">{{ newRef || "WORKTREE" }}</span>
          </p>
        </div>
        <div class="flex items-center gap-2 shrink-0">
          <button aria-label="重新加载"
            @click="load"
            :disabled="loading"
            class="p-1.5 rounded-lg border border-line bg-canvas hover:bg-raised text-ink-muted hover:text-accent transition cursor-pointer disabled:opacity-40"
            title="重新加载"
          >
            <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': loading }" />
          </button>
          <button aria-label="关闭"
            @click="emit('update:open', false)"
            class="p-1.5 rounded-lg border border-line bg-canvas hover:bg-raised text-ink-muted hover:text-danger transition cursor-pointer"
            title="关闭"
          >
            <X class="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      <!-- Body -->
      <div class="flex-1 overflow-y-auto px-5 py-4 space-y-4">
        <div v-if="loading" class="py-16 flex flex-col items-center gap-3 text-ink-muted">
          <RefreshCw class="w-5 h-5 animate-spin" />
          <span class="text-xs">正在读取两个版本…</span>
        </div>

        <p v-else-if="error" class="py-10 text-center text-xs text-danger">{{ error }}</p>

        <template v-else-if="data">
          <!-- Metadata -->
          <div class="grid grid-cols-2 gap-3">
            <div class="p-3 rounded-xl border border-line bg-canvas/50 space-y-1">
              <div class="text-[10px] font-semibold text-ink-muted">旧版本</div>
              <div class="text-xs text-ink">
                <span v-if="data.old.exists">
                  {{ data.old.width > 0 ? `${data.old.width}×${data.old.height} · ` : "" }}{{ data.old.sizeHuman }}
                </span>
                <span v-else class="text-ink-muted">不存在</span>
              </div>
            </div>
            <div class="p-3 rounded-xl border border-line bg-canvas/50 space-y-1">
              <div class="text-[10px] font-semibold text-ink-muted">新版本</div>
              <div class="text-xs text-ink">
                <span v-if="data.new.exists">
                  {{ data.new.width > 0 ? `${data.new.width}×${data.new.height} · ` : "" }}{{ data.new.sizeHuman }}
                </span>
                <span v-else class="text-ink-muted">已删除</span>
              </div>
            </div>
          </div>

          <div class="flex flex-wrap items-center gap-3 text-[11px]">
            <span class="text-ink-secondary">{{ data.message }}</span>
            <span v-if="data.sizeDelta !== 0" class="font-mono" :class="deltaClass(data.sizeDelta)">
              体积 {{ data.sizeDelta > 0 ? "+" : "" }}{{ (data.sizeDelta / 1024).toFixed(1) }} KB
            </span>
          </div>

          <!-- Image comparison -->
          <template v-if="data.kind === 'image' && bothPreviewable">
            <div class="flex items-center gap-1 p-0.5 rounded-lg border border-line bg-canvas w-fit">
              <button
                v-for="m in modes"
                :key="m.id"
                @click="selectMode(m.id)"
                class="flex items-center gap-1.5 px-2.5 py-1 rounded text-[11px] font-medium transition cursor-pointer"
                :class="mode === m.id ? 'bg-raised text-accent font-semibold shadow-xs' : 'text-ink-muted hover:text-ink'"
              >
                <component :is="m.icon" class="w-3.5 h-3.5" />
                <span>{{ m.label }}</span>
              </button>
            </div>

            <!-- side by side -->
            <div v-if="mode === 'side'" class="grid grid-cols-2 gap-3">
              <div class="space-y-1.5 min-w-0">
                <div class="text-[10px] text-ink-muted text-center">旧</div>
                <div class="rounded-lg border border-line bg-[repeating-conic-gradient(#0000_0%_25%,#00000010_0%_50%)] bg-[length:16px_16px] p-2 flex items-center justify-center h-[340px]">
                  <img :src="data.old.dataUrl" alt="旧版本" class="max-w-full max-h-full object-contain" />
                </div>
              </div>
              <div class="space-y-1.5 min-w-0">
                <div class="text-[10px] text-ink-muted text-center">新</div>
                <div class="rounded-lg border border-line bg-[repeating-conic-gradient(#0000_0%_25%,#00000010_0%_50%)] bg-[length:16px_16px] p-2 flex items-center justify-center h-[340px]">
                  <img :src="data.new.dataUrl" alt="新版本" class="max-w-full max-h-full object-contain" />
                </div>
              </div>
            </div>

            <!-- overlay -->
            <div v-else-if="mode === 'overlay'" class="space-y-2">
              <div class="relative rounded-lg border border-line overflow-hidden bg-[repeating-conic-gradient(#0000_0%_25%,#00000010_0%_50%)] bg-[length:16px_16px] flex items-center justify-center h-[420px]">
                <img :src="data.old.dataUrl" alt="旧版本" class="max-w-full max-h-full object-contain" />
                <img
                  :src="data.new.dataUrl"
                  alt="新版本"
                  class="absolute max-w-full max-h-full object-contain"
                  :style="{ opacity: opacity / 100 }"
                />
              </div>
              <div class="flex items-center gap-3">
                <span class="text-[10px] text-ink-muted shrink-0">旧</span>
                <input v-model.number="opacity" type="range" min="0" max="100" class="flex-1 accent-[var(--ug-accent-strong)]" />
                <span class="text-[10px] text-ink-muted shrink-0">新</span>
                <span class="text-[10px] font-mono text-ink w-10 text-right">{{ opacity }}%</span>
              </div>
            </div>

            <!-- swipe -->
            <div v-else-if="mode === 'swipe'" class="space-y-2">
              <div class="relative rounded-lg border border-line overflow-hidden bg-[repeating-conic-gradient(#0000_0%_25%,#00000010_0%_50%)] bg-[length:16px_16px] flex items-center justify-center h-[420px]">
                <img :src="data.old.dataUrl" alt="旧版本" class="max-w-full max-h-full object-contain select-none" />
                <img
                  :src="data.new.dataUrl"
                  alt="新版本"
                  class="absolute max-w-full max-h-full object-contain select-none"
                  :style="{ clipPath: `inset(0 ${100 - swipePct}% 0 0)` }"
                />
                <div class="absolute top-0 bottom-0 w-px bg-accent shadow-[0_0_8px_var(--ug-accent)]" :style="{ left: swipePct + '%' }" />
              </div>
              <div class="flex items-center gap-3">
                <span class="text-[10px] text-ink-muted shrink-0">旧</span>
                <input v-model.number="swipePct" type="range" min="0" max="100" class="flex-1 accent-[var(--ug-accent-strong)]" />
                <span class="text-[10px] text-ink-muted shrink-0">新</span>
                <span class="text-[10px] font-mono text-ink w-10 text-right">{{ swipePct }}%</span>
              </div>
            </div>

            <!-- pixel diff -->
            <div v-else class="space-y-2">
              <div v-if="diffComputing" class="py-16 flex flex-col items-center gap-2 text-ink-muted">
                <RefreshCw class="w-4 h-4 animate-spin" />
                <span class="text-[11px]">正在逐像素比对…</span>
              </div>
              <div
                v-else-if="diffUrl"
                class="rounded-lg border border-line bg-[repeating-conic-gradient(#0000_0%_25%,#00000010_0%_50%)] bg-[length:16px_16px] p-2 flex items-center justify-center h-[420px]"
              >
                <img :src="diffUrl" alt="差值图" class="max-w-full max-h-full object-contain" />
              </div>
              <p v-else class="py-10 text-center text-[11px] text-ink-muted">{{ diffReason || "暂无差值图" }}</p>
              <p v-if="diffUrl" class="text-[10px] text-ink-muted text-center">
                洋红像素为差异区域（阈值 ΔRGB &gt; 24），其余以低亮度灰底呈现便于定位。
              </p>
            </div>
          </template>

          <!-- Non-previewable -->
          <div v-else class="py-12 flex flex-col items-center gap-2 text-ink-muted rounded-xl border border-line bg-canvas/40">
            <component :is="kindMeta.icon" class="w-8 h-8 opacity-50" />
            <p class="text-xs font-semibold text-ink">该格式无法在客户端内并排对比</p>
            <p class="text-[11px] max-w-md text-center leading-relaxed">
              {{ data.old.format || data.new.format }} 属于二进制资产，上面的尺寸与体积变化已可判断改动规模；
              逐像素对比请用 DCC 工具或外部差异工具打开。
            </p>
          </div>

          <!-- Impact analysis -->
          <div class="rounded-xl border border-line bg-canvas/40">
            <button
              @click="toggleImpact"
              class="w-full flex items-center justify-between gap-2 px-3 py-2 text-left cursor-pointer"
            >
              <span class="flex items-center gap-2 text-[11px] font-bold text-ink min-w-0">
                <Network class="w-3.5 h-3.5 text-accent shrink-0" />
                <span class="shrink-0">影响面分析</span>
                <span class="font-normal text-ink-muted truncate">改名前确认会波及哪些预制体 / 场景 / 脚本</span>
              </span>
              <span class="text-[10px] text-ink-muted shrink-0">{{ showImpact ? "收起" : "展开" }}</span>
            </button>

            <div v-if="showImpact" class="px-3 pb-3 space-y-2 border-t border-line/60 pt-2">
              <div v-if="isLoadingImpact" class="py-4 text-center text-[11px] text-ink-muted">
                扫描引用中…（会遍历仓库内的文本资产）
              </div>
              <p v-else-if="impactError" class="text-[11px] text-danger">{{ impactError }}</p>
              <template v-else-if="impact">
                <div class="flex items-center gap-2 flex-wrap">
                  <span class="px-1.5 py-0.5 rounded text-[9px] font-bold shrink-0" :class="impactRiskMeta(impact.riskLevel).cls">
                    {{ impactRiskMeta(impact.riskLevel).label }}
                  </span>
                  <span class="text-[11px] text-ink">{{ impact.message }}</span>
                </div>
                <div v-if="impact.owners.length" class="flex items-center gap-1.5 text-[10px] text-ink-muted">
                  <UserRound class="w-3 h-3 shrink-0" />
                  <span class="truncate">负责人：{{ impact.owners.join("、") }}</span>
                </div>
                <ul v-if="impact.warnings?.length" class="space-y-0.5 text-[10px] text-warning">
                  <li v-for="warning in impact.warnings" :key="warning">• {{ warning }}</li>
                </ul>
                <div v-if="impact.references.length" class="max-h-52 overflow-y-auto space-y-0.5">
                  <div
                    v-for="ref in impact.references"
                    :key="ref.path"
                    class="flex items-center gap-2 text-[10px] font-mono"
                  >
                    <span
                      class="shrink-0 px-1 rounded"
                      :class="ref.depth === 1 ? 'bg-accent/15 text-accent' : 'bg-surface border border-line text-ink-muted'"
                    >{{ ref.depth === 1 ? "直接" : "间接" }}</span>
                    <span class="text-ink truncate flex-1" :title="ref.path">{{ ref.path }}</span>
                    <span class="text-ink-muted shrink-0">{{ ref.lastAuthor }}</span>
                    <span class="text-ink-muted shrink-0 w-8 text-right">{{ ref.matchCount }}×</span>
                  </div>
                </div>
                <p v-else class="text-[11px] text-ink-muted">
                  {{ impact.analysisComplete ? "没有任何文件引用该资产。" : "未发现引用，但扫描不完整，不能据此删除。" }}
                </p>
              </template>
            </div>
          </div>
        </template>
      </div>
    </div>
  </div>
</template>
