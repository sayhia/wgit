<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from "vue";
import { Play, Pause, Volume2, VolumeX, Music, Repeat } from "lucide-vue-next";
import { t } from "../../i18n";

const props = defineProps<{
  oldUrl?: string;
  newUrl?: string;
  oldFormat?: string;
  newFormat?: string;
  oldSize?: string;
  newSize?: string;
}>();

const isPlaying = ref(false);
const activeTrack = ref<"old" | "new">("new");
const currentProgress = ref(0);
const duration = ref(0);
const volume = ref(80);
const isMuted = ref(false);
const compareMode = ref<"split" | "overlay">("split");

const oldAudioRef = ref<HTMLAudioElement | null>(null);
const newAudioRef = ref<HTMLAudioElement | null>(null);
const oldCanvasRef = ref<HTMLCanvasElement | null>(null);
const newCanvasRef = ref<HTMLCanvasElement | null>(null);
const overlayCanvasRef = ref<HTMLCanvasElement | null>(null);

let audioContext: AudioContext | null = null;
let oldAudioBuffer: AudioBuffer | null = null;
let newAudioBuffer: AudioBuffer | null = null;
let animFrameId: number | null = null;

const oldAudioStats = ref<{ duration: string; sampleRate: string; channels: string } | null>(null);
const newAudioStats = ref<{ duration: string; sampleRate: string; channels: string } | null>(null);

function formatDuration(sec: number): string {
  if (!Number.isFinite(sec) || sec <= 0) return "0:00";
  const m = Math.floor(sec / 60);
  const s = Math.floor(sec % 60);
  return `${m}:${s.toString().padStart(2, "0")}`;
}

async function decodeAudioBuffer(dataUrl: string): Promise<AudioBuffer | null> {
  try {
    const res = await fetch(dataUrl);
    const arrayBuf = await res.arrayBuffer();
    if (!audioContext) {
      const AudioCtx = window.AudioContext || (window as any).webkitAudioContext;
      if (AudioCtx) audioContext = new AudioCtx();
    }
    if (!audioContext) return null;
    return await audioContext.decodeAudioData(arrayBuf);
  } catch {
    return null;
  }
}

function extractPeaks(buffer: AudioBuffer, numPeaks = 120): number[] {
  const channelData = buffer.getChannelData(0);
  const step = Math.ceil(channelData.length / numPeaks);
  const peaks: number[] = [];
  for (let i = 0; i < numPeaks; i++) {
    const start = i * step;
    let max = 0;
    for (let j = 0; j < step && start + j < channelData.length; j++) {
      const val = Math.abs(channelData[start + j]!);
      if (val > max) max = val;
    }
    peaks.push(max);
  }
  return peaks;
}

function drawWaveform(canvas: HTMLCanvasElement, peaks: number[], color: string, progress = 0) {
  const ctx = canvas.getContext("2d");
  if (!ctx) return;
  const w = canvas.width;
  const h = canvas.height;
  ctx.clearRect(0, 0, w, h);

  const mid = h / 2;
  const barWidth = Math.max(1, w / peaks.length - 1);

  peaks.forEach((peak, i) => {
    const x = (i / peaks.length) * w;
    const barHeight = Math.max(2, peak * (mid - 4));
    const isPast = (i / peaks.length) <= progress;

    ctx.fillStyle = isPast ? color : "rgba(150, 150, 150, 0.35)";
    ctx.fillRect(x, mid - barHeight, barWidth, barHeight * 2);
  });
}

function drawOverlayWaveform(
  canvas: HTMLCanvasElement,
  peaksOld: number[],
  peaksNew: number[],
  progress = 0
) {
  const ctx = canvas.getContext("2d");
  if (!ctx) return;
  const w = canvas.width;
  const h = canvas.height;
  ctx.clearRect(0, 0, w, h);

  const mid = h / 2;
  const len = Math.max(peaksOld.length, peaksNew.length);
  const barWidth = Math.max(1, w / len - 1);

  for (let i = 0; i < len; i++) {
    const x = (i / len) * w;
    const oldVal = peaksOld[i] || 0;
    const newVal = peaksNew[i] || 0;

    // Draw old (background / cyan)
    const oldHeight = Math.max(2, oldVal * (mid - 4));
    ctx.fillStyle = "rgba(56, 189, 248, 0.4)";
    ctx.fillRect(x, mid - oldHeight, barWidth, oldHeight * 2);

    // Draw new (accent / green)
    const newHeight = Math.max(2, newVal * (mid - 4));
    ctx.fillStyle = "rgba(74, 222, 128, 0.75)";
    ctx.fillRect(x, mid - newHeight, barWidth, newHeight * 2);
  }
}

async function initAudio() {
  if (props.oldUrl) {
    oldAudioBuffer = await decodeAudioBuffer(props.oldUrl);
    if (oldAudioBuffer) {
      oldAudioStats.value = {
        duration: formatDuration(oldAudioBuffer.duration),
        sampleRate: `${(oldAudioBuffer.sampleRate / 1000).toFixed(1)} kHz`,
        channels: oldAudioBuffer.numberOfChannels === 2 ? "Stereo" : "Mono",
      };
      duration.value = Math.max(duration.value, oldAudioBuffer.duration);
    }
  }

  if (props.newUrl) {
    newAudioBuffer = await decodeAudioBuffer(props.newUrl);
    if (newAudioBuffer) {
      newAudioStats.value = {
        duration: formatDuration(newAudioBuffer.duration),
        sampleRate: `${(newAudioBuffer.sampleRate / 1000).toFixed(1)} kHz`,
        channels: newAudioBuffer.numberOfChannels === 2 ? "Stereo" : "Mono",
      };
      duration.value = Math.max(duration.value, newAudioBuffer.duration);
    }
  }

  renderWaveforms();
}

function renderWaveforms() {
  const pOld = oldAudioBuffer ? extractPeaks(oldAudioBuffer) : [];
  const pNew = newAudioBuffer ? extractPeaks(newAudioBuffer) : [];

  if (oldCanvasRef.value && pOld.length > 0) {
    drawWaveform(oldCanvasRef.value, pOld, "#38bdf8", currentProgress.value);
  }
  if (newCanvasRef.value && pNew.length > 0) {
    drawWaveform(newCanvasRef.value, pNew, "#4ade80", currentProgress.value);
  }
  if (overlayCanvasRef.value && (pOld.length > 0 || pNew.length > 0)) {
    drawOverlayWaveform(overlayCanvasRef.value, pOld, pNew, currentProgress.value);
  }
}

function getActiveAudioEl(): HTMLAudioElement | null {
  return activeTrack.value === "old" ? oldAudioRef.value : newAudioRef.value;
}

function togglePlay() {
  const el = getActiveAudioEl();
  if (!el) return;
  if (isPlaying.value) {
    el.pause();
    isPlaying.value = false;
  } else {
    el.play().then(() => {
      isPlaying.value = true;
      trackProgress();
    }).catch(() => {
      isPlaying.value = false;
    });
  }
}

function switchTrack(track: "old" | "new") {
  if (activeTrack.value === track) return;
  const currentEl = getActiveAudioEl();
  const currentTime = currentEl ? currentEl.currentTime : 0;
  const wasPlaying = isPlaying.value;

  if (currentEl) currentEl.pause();

  activeTrack.value = track;
  const nextEl = getActiveAudioEl();
  if (nextEl) {
    nextEl.currentTime = currentTime;
    if (wasPlaying) {
      nextEl.play().catch(() => {});
    }
  }
}

function trackProgress() {
  const el = getActiveAudioEl();
  if (!el || !isPlaying.value) return;

  if (el.duration > 0) {
    currentProgress.value = el.currentTime / el.duration;
  }
  renderWaveforms();

  if (el.ended) {
    isPlaying.value = false;
    currentProgress.value = 0;
    renderWaveforms();
    return;
  }
  animFrameId = requestAnimationFrame(trackProgress);
}

function handleSeek(e: MouseEvent, targetCanvas: HTMLCanvasElement | null) {
  if (!targetCanvas) return;
  const rect = targetCanvas.getBoundingClientRect();
  const ratio = Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width));
  currentProgress.value = ratio;

  const el = getActiveAudioEl();
  if (el && el.duration) {
    el.currentTime = ratio * el.duration;
  }
  renderWaveforms();
}

function toggleMute() {
  isMuted.value = !isMuted.value;
  if (oldAudioRef.value) oldAudioRef.value.muted = isMuted.value;
  if (newAudioRef.value) newAudioRef.value.muted = isMuted.value;
}

watch(volume, (vol) => {
  const v = vol / 100;
  if (oldAudioRef.value) oldAudioRef.value.volume = v;
  if (newAudioRef.value) newAudioRef.value.volume = v;
});

watch(() => [props.oldUrl, props.newUrl], () => {
  void initAudio();
});

onMounted(() => {
  void initAudio();
});

onUnmounted(() => {
  if (animFrameId) cancelAnimationFrame(animFrameId);
  if (oldAudioRef.value) oldAudioRef.value.pause();
  if (newAudioRef.value) newAudioRef.value.pause();
  if (audioContext && audioContext.state !== "closed") {
    try {
      void audioContext.close();
    } catch {
      /* ignore */
    }
  }
});
</script>

<template>
  <div class="audio-diff-viewer flex flex-col gap-3">
    <!-- Hidden native audio players -->
    <audio v-if="oldUrl" ref="oldAudioRef" :src="oldUrl" preload="metadata" />
    <audio v-if="newUrl" ref="newAudioRef" :src="newUrl" preload="metadata" />

    <!-- Top Track Switcher & Mode Toolbar -->
    <div class="flex items-center justify-between p-2 rounded-xl bg-surface/50 border border-line">
      <div class="flex items-center gap-1.5">
        <button
          @click="switchTrack('old')"
          :disabled="!oldUrl"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold transition cursor-pointer border"
          :class="activeTrack === 'old'
            ? 'bg-cyan/15 text-cyan border-cyan/40 shadow-xs'
            : 'bg-canvas text-ink-muted hover:text-ink border-line disabled:opacity-40'"
        >
          <span class="w-2 h-2 rounded-full" :class="activeTrack === 'old' ? 'bg-cyan' : 'bg-line'"></span>
          <span>{{ t("assets.diff.short.old") }} (A)</span>
        </button>

        <button
          @click="switchTrack('new')"
          :disabled="!newUrl"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold transition cursor-pointer border"
          :class="activeTrack === 'new'
            ? 'bg-success/15 text-success border-success/40 shadow-xs'
            : 'bg-canvas text-ink-muted hover:text-ink border-line disabled:opacity-40'"
        >
          <span class="w-2 h-2 rounded-full" :class="activeTrack === 'new' ? 'bg-success' : 'bg-line'"></span>
          <span>{{ t("assets.diff.short.new") }} (B)</span>
        </button>

        <button
          @click="compareMode = compareMode === 'split' ? 'overlay' : 'split'"
          class="ml-2 flex items-center gap-1 px-2.5 py-1.5 rounded-lg bg-canvas hover:bg-raised text-ink-secondary hover:text-ink text-xs font-medium border border-line transition cursor-pointer"
          :title="compareMode === 'split' ? t('assets.audio.mode.overlay') : t('assets.audio.mode.split')"
        >
          <Repeat class="w-3.5 h-3.5 text-accent" />
          <span>{{ compareMode === 'split' ? t('assets.audio.mode.overlay') : t('assets.audio.mode.split') }}</span>
        </button>
      </div>

      <!-- Playback and Volume controls -->
      <div class="flex items-center gap-3">
        <button
          @click="togglePlay"
          class="flex items-center justify-center w-8 h-8 rounded-full bg-accent text-accent-contrast hover:bg-accent-strong transition cursor-pointer shadow-sm"
        >
          <Pause v-if="isPlaying" class="w-4 h-4 fill-current" />
          <Play v-else class="w-4 h-4 fill-current ml-0.5" />
        </button>

        <span class="text-xs font-mono text-ink-secondary min-w-[36px]">
          {{ formatDuration((getActiveAudioEl()?.currentTime || 0)) }}
        </span>

        <div class="flex items-center gap-1.5 pl-2 border-l border-line/60">
          <button @click="toggleMute" class="p-1 rounded text-ink-muted hover:text-ink cursor-pointer">
            <VolumeX v-if="isMuted" class="w-3.5 h-3.5 text-danger" />
            <Volume2 v-else class="w-3.5 h-3.5" />
          </button>
          <input
            v-model.number="volume"
            type="range"
            min="0"
            max="100"
            class="w-16 h-1 accent-accent cursor-pointer"
          />
        </div>
      </div>
    </div>

    <!-- Waveform Viewport (Split or Overlay) -->
    <div v-if="compareMode === 'split'" class="grid grid-cols-1 md:grid-cols-2 gap-3">
      <!-- Old Track Waveform -->
      <div class="p-3 rounded-xl border border-line bg-canvas/60 flex flex-col gap-2">
        <div class="flex items-center justify-between text-xs text-ink-muted">
          <span class="font-semibold text-cyan">{{ t("assets.diff.old") }}</span>
          <span v-if="oldAudioStats" class="font-mono text-[11px]">
            {{ oldAudioStats.duration }} · {{ oldAudioStats.sampleRate }} · {{ oldAudioStats.channels }}
          </span>
        </div>
        <canvas
          ref="oldCanvasRef"
          width="400"
          height="90"
          @click="handleSeek($event, oldCanvasRef)"
          class="w-full h-[90px] rounded bg-surface/40 border border-line/40 cursor-pointer"
        ></canvas>
      </div>

      <!-- New Track Waveform -->
      <div class="p-3 rounded-xl border border-line bg-canvas/60 flex flex-col gap-2">
        <div class="flex items-center justify-between text-xs text-ink-muted">
          <span class="font-semibold text-success">{{ t("assets.diff.new") }}</span>
          <span v-if="newAudioStats" class="font-mono text-[11px]">
            {{ newAudioStats.duration }} · {{ newAudioStats.sampleRate }} · {{ newAudioStats.channels }}
          </span>
        </div>
        <canvas
          ref="newCanvasRef"
          width="400"
          height="90"
          @click="handleSeek($event, newCanvasRef)"
          class="w-full h-[90px] rounded bg-surface/40 border border-line/40 cursor-pointer"
        ></canvas>
      </div>
    </div>

    <!-- Overlay Mode Waveform -->
    <div v-else class="p-3 rounded-xl border border-line bg-canvas/60 flex flex-col gap-2">
      <div class="flex items-center justify-between text-xs text-ink-muted">
        <div class="flex items-center gap-3">
          <span class="flex items-center gap-1.5 text-cyan">
            <span class="w-2.5 h-2.5 rounded-full bg-cyan/60"></span>
            <span>{{ t("assets.diff.old") }}</span>
          </span>
          <span class="flex items-center gap-1.5 text-success">
            <span class="w-2.5 h-2.5 rounded-full bg-success"></span>
            <span>{{ t("assets.diff.new") }}</span>
          </span>
        </div>
        <span class="text-[11px] font-mono text-ink-secondary">
          {{ formatDuration(duration) }}
        </span>
      </div>
      <canvas
        ref="overlayCanvasRef"
        width="800"
        height="120"
        @click="handleSeek($event, overlayCanvasRef)"
        class="w-full h-[120px] rounded bg-surface/40 border border-line/40 cursor-pointer"
      ></canvas>
    </div>
  </div>
</template>
