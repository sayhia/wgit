<script setup lang="ts">
import { ref, watch, nextTick } from "vue";
import { ExternalLink, History, FileText, Image, Box, FileSpreadsheet, Code2, RotateCcw, FolderOpen } from "lucide-vue-next";
import type { BlameLine, FileRevisionItem, SourceFileDetail } from "../../types";
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
}>();

/** Non-null accessor for the template, guarded there by `blameLines.length > idx`. */
function blameForLine(idx: number): BlameLine {
  return props.blameLines[idx] as BlameLine;
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

          <!-- 3. CODE VIEWER CANVAS WITH SYNTAX HIGHLIGHTING -->
          <div
            v-else
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
                class="flex items-center gap-2 pr-3 mr-2 border-r border-line/60 select-none text-[11px] font-mono text-ink-muted shrink-0 w-72 overflow-hidden"
                :title="
                  t('source.panorama.blame.tooltip', {
                    hash: blameForLine(idx).commitHash,
                    author: blameForLine(idx).author,
                    date: blameForLine(idx).date,
                    summary: blameForLine(idx).summary,
                  })
                "
              >
                <span
                  @click="emit('copy-hash', blameForLine(idx).commitHash)"
                  class="text-accent hover:underline cursor-pointer font-bold shrink-0 font-mono"
                  :title="t('source.panorama.blame.copy')"
                >
                  {{ blameForLine(idx).shortHash || blameForLine(idx).commitHash.substring(0, 7) }}
                </span>
                <span class="truncate max-w-[80px] text-ink-secondary" :title="blameForLine(idx).author">
                  {{ blameForLine(idx).author }}
                </span>
                <span class="text-[10px] text-ink-muted shrink-0">
                  {{ blameForLine(idx).date }}
                </span>
                <span class="truncate flex-1 text-ink-muted text-[10px] hidden group-hover:inline">
                  {{ blameForLine(idx).summary }}
                </span>
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
</template>
