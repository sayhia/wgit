<script setup lang="ts">
import { ref, watch, nextTick } from "vue";
import {
  ExternalLink,
  History,
  FileText,
  Image,
  Box,
  FileSpreadsheet,
  Code2,
  RotateCcw,
  FolderOpen,
  Copy,
  Clock,
  ChevronLeft,
  ChevronRight,
  X,
  User,
  Calendar,
} from "lucide-vue-next";
import type { BlameLine, FileRevisionItem, SourceFileDetail, TimeMachineSnapshot } from "../../types";
import { relativeTime } from "../../utils/relativeTime";
import { t } from "../../i18n";

const props = defineProps<{
  sourceFileDetail: SourceFileDetail | null;
  highlightedSourceLines: string[];
  highlightedSourceLine: number | null;
  showBlame: boolean;
  blameLines: BlameLine[];
  showFileHistory: boolean;
  fileHistoryList: FileRevisionItem[];
  selectedFileRevisionHash: string;
  isLoadingFileHistory: boolean;
  imageZoom: number;
  imageBgMode: "checkered" | "dark" | "white";
  imagePixelated: boolean;
  svgViewMode: "image" | "code";
  imageWidth: number;
  imageHeight: number;
  isCodeWrap: boolean;
  timeMachineActive?: boolean;
  timeMachineCurrent?: TimeMachineSnapshot | null;
  canTimeMachineBack?: boolean;
  canTimeMachineForward?: boolean;
}>();

const emit = defineEmits<{
  (e: "toggle-blame"): void;
  (e: "open-web", path: string): void;
  (e: "reveal-file", path: string): void;
  (e: "open-default", path: string): void;
  (e: "copy-hash", hash: string): void;
  (e: "copy-image-base64"): void;
  (e: "copy-revision-content"): void;
  (e: "update:showFileHistory", value: boolean): void;
  (e: "view-revision", hash: string): void;
  (e: "diff-revision", hash: string): void;
  (e: "restore-revision"): void;
  (e: "revert-revision", hash: string): void;
  (e: "update:imageBgMode", value: "checkered" | "dark" | "white"): void;
  (e: "update:imagePixelated", value: boolean): void;
  (e: "update:svgViewMode", value: "image" | "code"): void;
  (e: "zoom-in"): void;
  (e: "zoom-out"): void;
  (e: "reset-zoom"): void;
  (e: "image-loaded", size: { width: number; height: number }): void;
  (e: "blame-prior", line: BlameLine): void;
  (e: "inspect-history", hash: string): void;
  (e: "time-machine-back"): void;
  (e: "time-machine-forward"): void;
  (e: "time-machine-exit"): void;
}>();

/** Non-null accessor for the template, guarded there by `blameLines.length > idx`. */
function blameForLine(idx: number): BlameLine {
  return props.blameLines[idx] as BlameLine;
}

function formatBlameRelativeTime(line: BlameLine): string {
  const iso = line.authorTimestamp ? new Date(line.authorTimestamp * 1000).toISOString() : line.date;
  return relativeTime(iso, line.date);
}

function getBlameAgeDays(line: BlameLine): number {
  const ts = line.authorTimestamp ? line.authorTimestamp * 1000 : Date.parse(line.date);
  if (isNaN(ts)) return 999;
  return Math.max(0, (Date.now() - ts) / (1000 * 60 * 60 * 24));
}

function getBlameHeatmapClass(line: BlameLine): string {
  const age = getBlameAgeDays(line);
  if (age <= 7) return "border-l-2 border-l-cyan-400 bg-cyan-500/10 text-cyan-200";
  if (age <= 30) return "border-l-2 border-l-emerald-400 bg-emerald-500/5 text-emerald-200";
  if (age <= 90) return "border-l-2 border-l-blue-400/80 bg-blue-500/5 text-blue-200";
  if (age <= 180) return "border-l-2 border-l-indigo-400/70 text-indigo-200";
  if (age <= 365) return "border-l-2 border-l-amber-500/60 text-amber-200";
  return "border-l-2 border-l-line/40 text-ink-muted";
}

interface BlamePopoverState {
  line: BlameLine;
  x: number;
  y: number;
  pinned: boolean;
}

const activePopover = ref<BlamePopoverState | null>(null);
let hoverTimeout: ReturnType<typeof setTimeout> | null = null;
let isHoveringPopover = false;

function onBlameCellClick(line: BlameLine, ev: MouseEvent) {
  if (hoverTimeout) clearTimeout(hoverTimeout);
  if (activePopover.value?.line.commitHash === line.commitHash && activePopover.value.pinned) {
    activePopover.value = null;
    return;
  }
  const rect = (ev.currentTarget as HTMLElement).getBoundingClientRect();
  activePopover.value = {
    line,
    x: Math.min(window.innerWidth - 380, Math.max(16, rect.left)),
    y: Math.min(window.innerHeight - 260, rect.bottom + 4),
    pinned: true,
  };
}

function onBlameCellMouseEnter(line: BlameLine, ev: MouseEvent) {
  if (hoverTimeout) clearTimeout(hoverTimeout);
  if (activePopover.value?.pinned) return;
  const rect = (ev.currentTarget as HTMLElement).getBoundingClientRect();
  activePopover.value = {
    line,
    x: Math.min(window.innerWidth - 380, Math.max(16, rect.left)),
    y: Math.min(window.innerHeight - 260, rect.bottom + 4),
    pinned: false,
  };
}

function onBlameCellMouseLeave() {
  if (activePopover.value?.pinned) return;
  hoverTimeout = setTimeout(() => {
    if (!isHoveringPopover && !activePopover.value?.pinned) {
      activePopover.value = null;
    }
  }, 180);
}

function onPopoverMouseEnter() {
  isHoveringPopover = true;
  if (hoverTimeout) clearTimeout(hoverTimeout);
}

function onPopoverMouseLeave() {
  isHoveringPopover = false;
  if (!activePopover.value?.pinned) {
    activePopover.value = null;
  }
}

function closePopover() {
  activePopover.value = null;
  isHoveringPopover = false;
}

function getFileIcon(pathOrExt?: string | null) {
  if (!pathOrExt) return FileText;
  const parts = pathOrExt.split(".");
  const ext = (parts.length > 1 ? parts.pop()! : pathOrExt).toLowerCase();
  if (["png", "jpg", "jpeg", "tga", "dds", "psd", "webp", "gif", "ico", "bmp", "svg", "avif", "tif", "tiff"].includes(ext)) return Image;
  if (["uasset", "umap", "prefab", "fbx", "obj", "blend", "dae", "gltf", "glb"].includes(ext)) return Box;
  if (["csv", "xlsx", "xls", "json", "tsv"].includes(ext)) return FileSpreadsheet;
  if (["go", "ts", "mts", "cts", "tsx", "js", "mjs", "cjs", "jsx", "vue", "svelte", "py", "rs", "cpp", "h", "c", "hpp", "cs", "hlsl", "shader", "usf", "ush", "glsl", "frag", "vert", "html", "css", "scss", "sass", "less", "php", "rb", "dart", "zig", "lua", "sql", "sh", "bash", "zsh", "xml", "yaml", "yml", "toml"].includes(ext)) return Code2;
  return FileText;
}


const sourceCodeScrollRef = ref<HTMLElement | null>(null);
const SOURCE_LINE_HEIGHT = 24; // code line height: leading-6 => 24px

function onImageLoaded(e: Event) {
  const img = e.target as HTMLImageElement;
  if (img) emit("image-loaded", { width: img.naturalWidth, height: img.naturalHeight });
}

watch(
  () => props.highlightedSourceLine,
  async (line) => {
    if (!line || line <= 0) return;
    await nextTick();
    requestAnimationFrame(() => {
      const container = sourceCodeScrollRef.value;
      if (!container) return;
      const lineEl = container.querySelector<HTMLElement>(`[data-source-line="${line}"]`);
      if (lineEl) {
        const containerRect = container.getBoundingClientRect();
        const lineRect = lineEl.getBoundingClientRect();
        container.scrollTop += lineRect.top - containerRect.top - container.clientHeight / 2 + lineRect.height / 2;
      } else {
        container.scrollTop = Math.max(0, (line - 1) * SOURCE_LINE_HEIGHT);
      }
      if (container.scrollTop < 0) container.scrollTop = 0;
    });
  }
);
</script>

<template>
      <div class="flex-1 flex flex-col overflow-hidden bg-canvas">
        <div v-if="sourceFileDetail" class="flex-1 flex flex-col overflow-hidden">
          <!-- Source Code Header Bar -->
          <div class="h-10 border-b border-line px-4 flex items-center justify-between bg-surface/30 backdrop-blur-sm shrink-0">
            <div class="flex items-center gap-2 font-mono text-xs text-ink truncate">
              <component :is="getFileIcon(sourceFileDetail.path)" class="w-4 h-4 text-accent shrink-0" />
              <span class="font-bold truncate">{{ sourceFileDetail.path }}</span>
            </div>

            <div class="flex items-center gap-3 text-xs text-ink-muted font-mono shrink-0">
              <span v-if="!sourceFileDetail.isImage || svgViewMode === 'code'">{{
                t("source.panorama.lines", { count: sourceFileDetail.lineCount })
              }}</span>
              <span v-if="sourceFileDetail.isImage && imageWidth && imageHeight">{{ imageWidth }} × {{ imageHeight }} px</span>
              <span>{{ sourceFileDetail.sizeFormatted }}</span>
              <span class="px-2 py-0.5 rounded bg-surface border border-line text-[10px] text-accent">
                {{ sourceFileDetail.language }}
              </span>
              <button
                @click="emit('open-web', sourceFileDetail.path)"
                class="flex items-center gap-1 px-2 py-0.5 rounded bg-surface hover:bg-hover text-ink-muted hover:text-ink text-[10px] font-mono border border-line transition cursor-pointer"
                :title="t('source.panorama.open.web.hint')"
              >
                <ExternalLink class="w-3 h-3 text-accent" />
                <span>{{ t("source.panorama.open.web") }}</span>
              </button>
              <button
                @click="emit('reveal-file', sourceFileDetail.path)"
                class="flex items-center gap-1 px-2 py-0.5 rounded bg-surface hover:bg-hover text-ink-muted hover:text-ink text-[10px] font-mono border border-line transition cursor-pointer"
                :title="t('source.panorama.reveal.hint')"
              >
                <FolderOpen class="w-3 h-3 text-warning" />
                <span>{{ t("source.panorama.reveal") }}</span>
              </button>
              <button
                @click="emit('open-default', sourceFileDetail.path)"
                class="flex items-center gap-1 px-2 py-0.5 rounded bg-surface hover:bg-hover text-ink-muted hover:text-ink text-[10px] font-mono border border-line transition cursor-pointer"
                :title="t('source.panorama.open.default.hint')"
              >
                <FileText class="w-3 h-3 text-purple" />
                <span>{{ t("source.panorama.open.default") }}</span>
              </button>
              <button
                v-if="sourceFileDetail.isImage"
                @click="emit('copy-image-base64')"
                class="flex items-center gap-1 px-2 py-0.5 rounded bg-surface hover:bg-hover text-ink-muted hover:text-ink text-[10px] font-mono border border-line transition cursor-pointer"
                :title="t('source.panorama.copy.base64.hint')"
              >
                <Code2 class="w-3 h-3 text-cyan" />
                <span>{{ t("source.panorama.copy.base64") }}</span>
              </button>
            </div>

          </div>

          <!-- Main Content and File History Drawer Container -->
          <div class="flex-1 flex overflow-hidden">
            <!-- 1. IMAGE PREVIEW CANVAS -->
          <div
            v-if="sourceFileDetail.isImage && svgViewMode === 'image' && sourceFileDetail.imageData"
            class="flex-1 overflow-auto flex flex-col items-center justify-center p-6 relative select-none"
            :class="imageBgMode === 'checkered' ? 'bg-checkered' : imageBgMode === 'white' ? 'bg-white' : 'bg-canvas'"
          >
            <!-- Image viewport with zoom & render style -->
            <div class="max-w-full max-h-full flex items-center justify-center p-4">
              <img
                :src="sourceFileDetail.imageData"
                :alt="sourceFileDetail.name"
                @load="onImageLoaded"
                :style="{
                  transform: `scale(${imageZoom / 100})`,
                  transformOrigin: 'center center',
                  imageRendering: imagePixelated ? 'pixelated' : 'auto'
                }"
                class="max-w-[85vw] max-h-[70vh] object-contain shadow-2xl rounded-sm transition-transform duration-100 ease-out border border-line/40"
              />
            </div>

            <!-- Image Info Overlay Footer -->
            <div class="absolute bottom-3 left-1/2 -translate-x-1/2 px-4 py-1.5 rounded-full bg-surface/90 backdrop-blur-md border border-line shadow-lg flex items-center gap-4 text-xs font-mono text-ink shrink-0">
              <span v-if="imageWidth && imageHeight" class="flex items-center gap-1 text-ink-muted">
                {{ t("source.panorama.image.size") }}
                <strong class="text-ink">{{ imageWidth }} × {{ imageHeight }}</strong> px
              </span>
              <span class="flex items-center gap-1 text-ink-muted">
                {{ t("source.panorama.image.bytes") }}
                <strong class="text-ink">{{ sourceFileDetail.sizeFormatted }}</strong>
              </span>
              <span class="flex items-center gap-1 text-ink-muted">
                {{ t("source.panorama.image.zoom") }}
                <strong class="text-accent">{{ imageZoom }}%</strong>
              </span>
            </div>

          </div>

          <!-- 2. BINARY NON-IMAGE NOTICE -->
          <div
            v-else-if="sourceFileDetail.isBinary && (!sourceFileDetail.isImage || !sourceFileDetail.imageData)"
            class="flex-1 flex flex-col items-center justify-center gap-3 text-ink-muted p-8 text-center"
          >
            <component :is="getFileIcon(sourceFileDetail.path)" class="w-12 h-12 text-ink-muted" />
            <div class="text-sm font-semibold text-ink">{{ t("source.panorama.binary.title") }}</div>
            <div class="text-xs max-w-md text-ink-muted leading-relaxed">
              {{ t("source.panorama.binary.body", { size: sourceFileDetail.sizeFormatted }) }}
            </div>

          </div>

          <!-- 3. CODE VIEWER CANVAS WITH SYNTAX HIGHLIGHTING & TIME MACHINE -->
          <div
            v-else
            class="flex-1 flex flex-col overflow-hidden min-w-0"
          >
            <!-- TIME MACHINE NAVIGATION BANNER -->
            <div
              v-if="timeMachineActive"
              class="px-4 py-2 bg-gradient-to-r from-purple/20 via-accent/10 to-transparent border-b border-purple/30 flex items-center justify-between shrink-0 select-none z-10 animate-in slide-in-from-top-1 duration-150"
            >
              <div class="flex items-center gap-2.5 text-xs truncate mr-4">
                <span class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[11px] font-semibold bg-purple/20 text-purple border border-purple/40 shrink-0">
                  <Clock class="w-3.5 h-3.5 text-purple" />
                  {{ t("timemachine.banner.mode") }}
                </span>
                <span class="text-ink-muted text-xs shrink-0">{{ t("timemachine.banner.current") }}:</span>
                <span class="font-mono font-bold text-accent shrink-0">{{ timeMachineCurrent?.shortHash }}</span>
                <span class="text-ink truncate text-xs" :title="timeMachineCurrent?.subject">
                  ({{ timeMachineCurrent?.subject || t("timemachine.banner.no.subject") }})
                </span>
              </div>

              <div class="flex items-center gap-2 shrink-0">
                <button
                  :disabled="!canTimeMachineBack"
                  @click="emit('time-machine-back')"
                  class="flex items-center gap-1 px-2.5 py-1 rounded text-xs font-medium border border-line bg-surface hover:bg-hover text-ink transition disabled:opacity-40 disabled:cursor-not-allowed cursor-pointer"
                  :title="t('timemachine.banner.back')"
                >
                  <ChevronLeft class="w-3.5 h-3.5" />
                  <span>{{ t("timemachine.banner.back") }}</span>
                </button>
                <button
                  :disabled="!canTimeMachineForward"
                  @click="emit('time-machine-forward')"
                  class="flex items-center gap-1 px-2.5 py-1 rounded text-xs font-medium border border-line bg-surface hover:bg-hover text-ink transition disabled:opacity-40 disabled:cursor-not-allowed cursor-pointer"
                  :title="t('timemachine.banner.forward')"
                >
                  <span>{{ t("timemachine.banner.forward") }}</span>
                  <ChevronRight class="w-3.5 h-3.5" />
                </button>
                <button
                  @click="emit('time-machine-exit')"
                  class="flex items-center gap-1 px-2.5 py-1 rounded text-xs font-bold border border-purple/40 bg-purple/20 hover:bg-purple/30 text-purple transition cursor-pointer"
                  :title="t('timemachine.banner.exit')"
                >
                  <X class="w-3.5 h-3.5" />
                  <span>{{ t("timemachine.banner.exit") }}</span>
                </button>
              </div>
            </div>

            <!-- Code Viewer Scroll Canvas -->
            <div
              ref="sourceCodeScrollRef"
              class="flex-1 overflow-auto p-2 font-mono text-[12px] leading-6 select-text"
            >
              <div
                v-for="(lineHtml, idx) in highlightedSourceLines"
                :key="idx"
                :data-source-line="idx + 1"
                class="flex items-start hover:bg-surface/50 px-2 rounded-xs transition-colors group"
                :class="highlightedSourceLine === idx + 1 ? 'bg-accent/15 ring-1 ring-inset ring-accent/30' : ''"
              >
                <!-- Git Blame Gutter Column -->
                <div
                  v-if="showBlame && blameLines.length > idx"
                  class="flex items-center gap-2 pr-2.5 mr-2 border-r border-line/60 select-none text-[11px] font-mono shrink-0 w-80 md:w-96 overflow-hidden cursor-pointer transition-colors relative"
                  :class="getBlameHeatmapClass(blameForLine(idx))"
                  @click="onBlameCellClick(blameForLine(idx), $event)"
                  @mouseenter="onBlameCellMouseEnter(blameForLine(idx), $event)"
                  @mouseleave="onBlameCellMouseLeave"
                  :title="
                    t('source.panorama.blame.tooltip', {
                      hash: blameForLine(idx).commitHash,
                      author: blameForLine(idx).author,
                      date: blameForLine(idx).date,
                      summary: blameForLine(idx).summary,
                    })
                  "
                >
                  <!-- Short Hash -->
                  <span
                    @click.stop="emit('copy-hash', blameForLine(idx).commitHash)"
                    class="text-accent hover:underline cursor-pointer font-bold shrink-0 font-mono pl-1"
                    :title="t('blame.action.copy.hint')"
                  >
                    {{ blameForLine(idx).shortHash || blameForLine(idx).commitHash.substring(0, 7) }}
                  </span>

                  <!-- Author -->
                  <span class="truncate max-w-[72px] font-sans text-ink-secondary" :title="blameForLine(idx).author">
                    {{ blameForLine(idx).author }}
                  </span>

                  <!-- Relative Time -->
                  <span class="text-[10px] text-ink-muted shrink-0 font-mono" :title="blameForLine(idx).date">
                    {{ formatBlameRelativeTime(blameForLine(idx)) }}
                  </span>

                  <!-- Commit Summary -->
                  <span class="truncate flex-1 text-ink-muted text-[10px]" :title="blameForLine(idx).summary">
                    {{ blameForLine(idx).summary }}
                  </span>

                  <!-- Quick Actions on Hover -->
                  <div class="hidden group-hover:flex items-center gap-1 pl-1 shrink-0 bg-surface/80 rounded px-1">
                    <button
                      @click.stop="emit('blame-prior', blameForLine(idx))"
                      class="p-0.5 text-ink-muted hover:text-purple transition"
                      :title="t('blame.action.prior')"
                    >
                      <RotateCcw class="w-3 h-3" />
                    </button>
                    <button
                      @click.stop="emit('inspect-history', blameForLine(idx).commitHash)"
                      class="p-0.5 text-ink-muted hover:text-accent transition"
                      :title="t('blame.action.inspect')"
                    >
                      <History class="w-3 h-3" />
                    </button>
                    <button
                      @click.stop="emit('copy-hash', blameForLine(idx).commitHash)"
                      class="p-0.5 text-ink-muted hover:text-ink transition"
                      :title="t('blame.action.copy')"
                    >
                      <Copy class="w-3 h-3" />
                    </button>
                  </div>
                </div>

                <!-- Line Number -->
                <span class="w-12 text-right pr-4 text-[10px] text-ink-muted select-none shrink-0 font-mono">
                  {{ idx + 1 }}
                </span>
                <!-- Highlighted Code Line -->
                <span
                  class="flex-1 font-mono"
                  :class="isCodeWrap ? 'whitespace-pre-wrap break-all' : 'whitespace-pre'"
                  v-html="lineHtml || ' '"
                ></span>
              </div>
            </div>
          </div>

          <!-- 4. FILE HISTORY DRAWER -->
          <div
            v-if="showFileHistory"
            class="w-80 border-l border-line bg-surface/90 backdrop-blur-md flex flex-col shrink-0 animate-in slide-in-from-right duration-150"
          >
            <div class="p-3 border-b border-line flex items-center justify-between">
              <div class="flex items-center gap-2">
                <History class="w-4 h-4 text-purple" />
                <span class="text-xs font-bold text-ink">{{ t("source.panorama.history.title") }}</span>
              </div>
              <button
                @click="emit('update:showFileHistory', false)"
                class="p-1 rounded hover:bg-surface text-ink-muted hover:text-ink cursor-pointer"
              >
                ×
              </button>
            </div>

            <!-- Active revision banner if looking at historical revision -->
            <div
              v-if="selectedFileRevisionHash"
              class="p-2.5 bg-purple/15 border-b border-purple/30 text-xs flex items-center justify-between"
            >
              <span class="text-purple font-medium truncate max-w-[190px]">
                {{ t("source.panorama.history.browsing", { hash: selectedFileRevisionHash.substring(0, 8) }) }}
              </span>
              <div class="flex items-center gap-2 shrink-0">
                <button
                  @click="emit('copy-revision-content')"
                  class="text-[10px] font-semibold text-accent hover:underline"
                  :title="t('source.panorama.history.copy.hint')"
                >
                  {{ t("source.panorama.history.copy") }}
                </button>
                <button
                  @click="emit('restore-revision')"
                  class="text-[11px] font-bold text-purple hover:underline"
                >
                  {{ t("source.panorama.history.latest") }}
                </button>
              </div>
            </div>

            <div class="flex-1 overflow-y-auto divide-y divide-line/40 text-xs">
              <div
                v-for="c in fileHistoryList"
                :key="c.commitHash"
                @click="emit('view-revision', c.commitHash)"
                class="p-3 hover:bg-surface transition cursor-pointer flex flex-col gap-1.5"
                :class="selectedFileRevisionHash === c.commitHash ? 'bg-purple/10 border-l-2 border-purple' : ''"
              >
                <div class="flex items-center justify-between">
                  <span
                    @click.stop="emit('copy-hash', c.commitHash)"
                    class="font-mono text-accent hover:underline text-[11px] font-bold"
                    :title="t('source.panorama.history.copy.hash')"
                  >
                    {{ c.shortHash }}
                  </span>
                  <span class="text-[10px] text-ink-muted">{{ c.date }}</span>
                </div>
                <div class="font-medium text-ink text-xs leading-snug line-clamp-2">
                  {{ c.subject }}
                </div>
                <div class="text-[10px] text-ink-muted flex items-center justify-between gap-2">
                  <span class="truncate">{{ c.authorName }}</span>
                  <span class="font-mono shrink-0">{{ c.sizeStr }}</span>
                </div>
                <div class="flex items-center gap-3">
                  <button
                    @click.stop="emit('diff-revision', c.commitHash)"
                    class="flex items-center gap-1 text-[10px] text-accent hover:text-ink transition cursor-pointer"
                    :title="t('source.panorama.history.diff.hint')"
                  >
                    <Code2 class="w-2.5 h-2.5" />
                    <span>{{ t("source.panorama.history.diff") }}</span>
                  </button>
                  <button
                    @click.stop="emit('revert-revision', c.commitHash)"
                    class="flex items-center gap-1 text-[10px] text-warning hover:text-ink transition cursor-pointer"
                    :title="t('source.panorama.history.restore.hint')"
                  >
                    <RotateCcw class="w-2.5 h-2.5" />
                    <span>{{ t("source.panorama.history.restore") }}</span>
                  </button>
                </div>
              </div>

              <div v-if="fileHistoryList.length === 0" class="p-8 text-center text-xs text-ink-muted">
                {{ isLoadingFileHistory ? t("source.panorama.history.loading") : t("source.panorama.history.empty") }}
              </div>
            </div>
          </div>
        </div>
      </div>

      <div v-else class="flex-1 flex items-center justify-center text-xs text-ink-muted">
        {{ t("source.panorama.empty") }}
      </div>
    </div>

    <!-- FLOATING BLAME COMMIT DETAIL POPOVER CARD -->
    <div
      v-if="activePopover"
      class="fixed z-50 w-80 md:w-96 rounded-lg bg-surface/95 backdrop-blur-md border border-line shadow-2xl p-3.5 flex flex-col gap-2.5 animate-in fade-in zoom-in-95 duration-100 select-text"
      :style="{ top: `${activePopover.y}px`, left: `${activePopover.x}px` }"
      @mouseenter="onPopoverMouseEnter"
      @mouseleave="onPopoverMouseLeave"
    >
      <!-- Header: Short Hash, Boundary badge, Close button -->
      <div class="flex items-center justify-between pb-2 border-b border-line/60">
        <div class="flex items-center gap-2">
          <span class="font-mono font-bold text-accent text-xs px-1.5 py-0.5 rounded bg-accent/15 border border-accent/30">
            {{ activePopover.line.shortHash || activePopover.line.commitHash.substring(0, 7) }}
          </span>
          <span
            v-if="activePopover.line.isBoundary"
            class="text-[10px] font-semibold text-warning px-1.5 py-0.5 rounded bg-warning/15 border border-warning/30"
          >
            {{ t("blame.gutter.root") }}
          </span>
        </div>
        <button
          @click="closePopover"
          class="text-ink-muted hover:text-ink p-1 rounded hover:bg-hover transition cursor-pointer"
          :title="t('blame.popover.close')"
        >
          <X class="w-3.5 h-3.5" />
        </button>
      </div>

      <!-- Commit Summary -->
      <div class="text-xs font-semibold text-ink leading-snug line-clamp-2" :title="activePopover.line.summary">
        {{ activePopover.line.summary || t("timemachine.banner.no.subject") }}
      </div>

      <!-- Author & Email & Date -->
      <div class="flex flex-col gap-1 text-[11px] text-ink-secondary">
        <div class="flex items-center gap-1.5 truncate">
          <User class="w-3.5 h-3.5 text-ink-muted shrink-0" />
          <span class="font-medium text-ink">{{ activePopover.line.author }}</span>
          <span v-if="activePopover.line.authorEmail" class="text-ink-muted truncate">
            &lt;{{ activePopover.line.authorEmail }}&gt;
          </span>
        </div>
        <div class="flex items-center gap-1.5">
          <Calendar class="w-3.5 h-3.5 text-ink-muted shrink-0" />
          <span>{{ activePopover.line.date }}</span>
          <span class="text-ink-muted font-mono">({{ formatBlameRelativeTime(activePopover.line) }})</span>
        </div>
      </div>

      <!-- Actions -->
      <div class="flex items-center gap-1.5 pt-2 border-t border-line/60">
        <!-- Inspect in History -->
        <button
          @click="emit('inspect-history', activePopover.line.commitHash); closePopover()"
          class="flex-1 flex items-center justify-center gap-1 px-2 py-1 rounded bg-surface hover:bg-hover border border-line text-[11px] font-medium text-ink transition cursor-pointer"
          :title="t('blame.action.inspect.hint')"
        >
          <History class="w-3.5 h-3.5 text-purple" />
          <span>{{ t("blame.action.inspect") }}</span>
        </button>

        <!-- Blame Prior Commit -->
        <button
          :disabled="activePopover.line.isBoundary"
          @click="emit('blame-prior', activePopover.line); closePopover()"
          class="flex-1 flex items-center justify-center gap-1 px-2 py-1 rounded bg-purple/15 hover:bg-purple/25 border border-purple/30 text-[11px] font-medium text-purple transition cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
          :title="activePopover.line.isBoundary ? t('timemachine.toast.boundary') : t('blame.action.prior.hint')"
        >
          <RotateCcw class="w-3.5 h-3.5" />
          <span>{{ t("blame.action.prior") }}</span>
        </button>

        <!-- Copy Hash -->
        <button
          @click="emit('copy-hash', activePopover.line.commitHash); closePopover()"
          class="flex items-center justify-center p-1.5 rounded bg-surface hover:bg-hover border border-line text-ink-muted hover:text-ink transition cursor-pointer"
          :title="t('blame.action.copy.hint')"
        >
          <Copy class="w-3.5 h-3.5" />
        </button>
      </div>
    </div>
</template>
