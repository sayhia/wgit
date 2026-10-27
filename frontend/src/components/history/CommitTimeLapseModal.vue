<script setup lang="ts">
import { reportError } from "../../composables/useToast";
import { ref, computed, watch, onUnmounted } from "vue";
import {
  Play,
  Pause,
  SkipBack,
  SkipForward,
  FastForward,
  Clock,
  Users,
  ShieldAlert,
  ShieldCheck,
  AlertTriangle,
  FileCode,
  Loader2,
  Calendar,
  User,
  GitCommit,
} from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import { t } from "../../i18n";
import type { FileTimelineEntry, CodeOwnershipReport } from "../../types";
import UiModal from "../ui/UiModal.vue";

const props = defineProps<{
  open: boolean;
  filePath: string;
  currentRepoPath: string;
}>();

const emit = defineEmits<{
  (e: "update:open", value: boolean): void;
  (e: "notify", msg: string): void;
}>();

const activeSubTab = ref<"player" | "ownership">("player");
const timeline = ref<FileTimelineEntry[]>([]);
const currentIndex = ref(0);
const isPlaying = ref(false);
const playbackSpeed = ref<1 | 2 | 5>(1);
const ownershipReport = ref<CodeOwnershipReport | null>(null);
const isLoadingTimeline = ref(false);
const isLoadingOwnership = ref(false);
let playTimer: ReturnType<typeof setInterval> | null = null;

const currentEntry = computed<FileTimelineEntry | null>(() => {
  if (timeline.value.length === 0) return null;
  const idx = Math.max(0, Math.min(currentIndex.value, timeline.value.length - 1));
  return timeline.value[idx] || null;
});

const numberedLines = computed(() => {
  if (!currentEntry.value) return [];
  return currentEntry.value.content.split("\n");
});

function stopPlayback() {
  if (playTimer) {
    clearInterval(playTimer);
    playTimer = null;
  }
  isPlaying.value = false;
}

function startPlayback() {
  stopPlayback();
  isPlaying.value = true;
  const intervalMs = Math.round(1200 / playbackSpeed.value);
  playTimer = setInterval(() => {
    if (currentIndex.value < timeline.value.length - 1) {
      currentIndex.value++;
    } else {
      stopPlayback();
    }
  }, intervalMs);
}

function togglePlay() {
  if (isPlaying.value) {
    stopPlayback();
  } else {
    if (currentIndex.value >= timeline.value.length - 1) {
      currentIndex.value = 0;
    }
    startPlayback();
  }
}

function cycleSpeed() {
  if (playbackSpeed.value === 1) playbackSpeed.value = 2;
  else if (playbackSpeed.value === 2) playbackSpeed.value = 5;
  else playbackSpeed.value = 1;

  if (isPlaying.value) {
    startPlayback();
  }
}

function stepPrev() {
  stopPlayback();
  if (currentIndex.value > 0) currentIndex.value--;
}

function stepNext() {
  stopPlayback();
  if (currentIndex.value < timeline.value.length - 1) currentIndex.value++;
}

async function loadData() {
  if (!props.open || !props.currentRepoPath || !props.filePath) return;
  stopPlayback();
  isLoadingTimeline.value = true;
  isLoadingOwnership.value = true;

  try {
    const list = await gitService.getFileCommitTimeline(props.currentRepoPath, props.filePath, 30);
    timeline.value = list || [];
    currentIndex.value = Math.max(0, (list || []).length - 1);
  } catch (err: any) {
    emit("notify", err?.message || String(err));
  } finally {
    isLoadingTimeline.value = false;
  }

  try {
    ownershipReport.value = await gitService.getCodeOwnership(props.currentRepoPath, props.filePath);
  } catch (err: any) {
    reportError("Failed to load ownership:", err);
  } finally {
    isLoadingOwnership.value = false;
  }
}

watch(
  () => props.open,
  (val) => {
    if (val) {
      void loadData();
    } else {
      stopPlayback();
    }
  }
);

onUnmounted(() => {
  stopPlayback();
});
</script>

<template>
  <UiModal
    :open="open"
    @update:open="emit('update:open', $event)"
    :title="t('timelapse.title')"
    width="max-w-4xl"
  >
    <div class="p-4 space-y-4" data-testid="time-lapse-modal">
      <!-- Header Info & Sub-Tabs -->
      <div class="flex flex-wrap items-center justify-between gap-3 pb-3 border-b border-line">
        <div class="min-w-0">
          <div class="flex items-center gap-2">
            <FileCode class="w-4 h-4 text-accent shrink-0" />
            <span class="font-mono text-xs font-bold text-ink truncate">{{ filePath }}</span>
          </div>
          <p class="text-[11px] text-ink-muted mt-0.5">{{ t("timelapse.subtitle") }}</p>
        </div>

        <div class="flex items-center gap-1 bg-surface p-1 rounded-lg border border-line">
          <button
            @click="activeSubTab = 'player'"
            class="px-2.5 py-1 rounded text-xs font-medium transition cursor-pointer"
            :class="activeSubTab === 'player' ? 'bg-raised text-accent shadow-xs' : 'text-ink-secondary hover:text-ink'"
          >
            {{ t("timelapse.tab.player") }}
          </button>
          <button
            @click="activeSubTab = 'ownership'"
            class="px-2.5 py-1 rounded text-xs font-medium transition cursor-pointer"
            :class="activeSubTab === 'ownership' ? 'bg-raised text-accent shadow-xs' : 'text-ink-secondary hover:text-ink'"
          >
            {{ t("timelapse.tab.ownership") }}
          </button>
        </div>
      </div>

      <!-- TAB 1: PLAYER -->
      <div v-if="activeSubTab === 'player'" class="space-y-3">
        <div v-if="isLoadingTimeline" class="py-16 flex flex-col items-center justify-center gap-2 text-xs text-ink-muted">
          <Loader2 class="w-6 h-6 animate-spin text-accent" />
          <span>{{ t("timelapse.play") }}...</span>
        </div>

        <div v-else-if="timeline.length === 0" class="py-16 text-center text-xs text-ink-muted">
          {{ t("timelapse.empty") }}
        </div>

        <div v-else class="space-y-3">
          <!-- Commit Banner -->
          <div v-if="currentEntry" class="p-3 rounded-xl border border-line bg-surface/50 flex flex-wrap items-center justify-between gap-2">
            <div class="space-y-1 min-w-0">
              <div class="flex items-center gap-2 flex-wrap">
                <span class="px-2 py-0.5 rounded text-[10px] font-mono bg-canvas border border-line font-bold text-ink">
                  {{ currentEntry.commitHash.substring(0, 7) }}
                </span>
                <span class="text-xs font-bold text-ink truncate">
                  {{ currentEntry.subject }}
                </span>
                <span class="px-1.5 py-0.5 rounded text-[10px] font-mono font-medium bg-emerald/15 text-emerald">
                  {{ currentEntry.diffSummary }}
                </span>
              </div>
              <div class="flex items-center gap-3 text-[11px] text-ink-muted">
                <span class="flex items-center gap-1">
                  <User class="w-3 h-3" />
                  <span>{{ currentEntry.author }}</span>
                </span>
                <span class="flex items-center gap-1">
                  <Calendar class="w-3 h-3" />
                  <span>{{ currentEntry.authorDate.replace("T", " ").substring(0, 16) }}</span>
                </span>
              </div>
            </div>

            <div class="text-xs font-mono font-bold text-accent">
              {{ currentIndex + 1 }} / {{ timeline.length }}
            </div>
          </div>

          <!-- Controls Bar -->
          <div class="p-2.5 rounded-xl border border-line bg-surface flex items-center gap-3">
            <button
              @click="togglePlay"
              class="p-2 rounded-lg bg-accent text-accent-contrast hover:bg-accent-strong transition cursor-pointer shadow-xs"
              :title="isPlaying ? t('timelapse.pause') : t('timelapse.play')"
              data-testid="play-btn"
            >
              <Pause v-if="isPlaying" class="w-4 h-4" />
              <Play v-else class="w-4 h-4" />
            </button>

            <button
              @click="stepPrev"
              :disabled="currentIndex === 0"
              class="p-1.5 rounded-md bg-canvas hover:bg-raised border border-line text-ink transition cursor-pointer disabled:opacity-30"
              :title="t('timelapse.step.prev')"
            >
              <SkipBack class="w-3.5 h-3.5" />
            </button>

            <button
              @click="stepNext"
              :disabled="currentIndex === timeline.length - 1"
              class="p-1.5 rounded-md bg-canvas hover:bg-raised border border-line text-ink transition cursor-pointer disabled:opacity-30"
              :title="t('timelapse.step.next')"
            >
              <SkipForward class="w-3.5 h-3.5" />
            </button>

            <!-- Scrubber Slider -->
            <input
              type="range"
              min="0"
              :max="timeline.length - 1"
              v-model.number="currentIndex"
              class="flex-1 cursor-pointer accent-accent h-1.5 bg-canvas rounded-lg"
              data-testid="timeline-slider"
            />

            <!-- Speed toggle -->
            <button
              @click="cycleSpeed"
              class="px-2 py-1 rounded-md bg-canvas hover:bg-raised border border-line text-xs font-mono font-semibold text-ink transition cursor-pointer"
            >
              {{ t("timelapse.speed", { speed: playbackSpeed }) }}
            </button>
          </div>

          <!-- Historical File Content Viewer -->
          <div class="rounded-xl border border-line bg-canvas/80 max-h-[420px] overflow-auto font-mono text-xs">
            <div
              v-for="(line, lineIdx) in numberedLines"
              :key="lineIdx"
              class="flex items-start hover:bg-raised/30 px-2 py-0.5 leading-relaxed"
            >
              <span class="w-10 text-right pr-3 select-none text-ink-muted text-[10px] shrink-0">
                {{ lineIdx + 1 }}
              </span>
              <pre class="flex-1 whitespace-pre-wrap break-all text-ink font-mono">{{ line || " " }}</pre>
            </div>
          </div>
        </div>
      </div>

      <!-- TAB 2: CODE OWNERSHIP & BUS FACTOR -->
      <div v-else-if="activeSubTab === 'ownership'" class="space-y-4" data-testid="ownership-panel">
        <div v-if="isLoadingOwnership" class="py-16 flex flex-col items-center justify-center gap-2 text-xs text-ink-muted">
          <Loader2 class="w-6 h-6 animate-spin text-accent" />
          <span>{{ t("ownership.title") }}...</span>
        </div>

        <div v-else-if="ownershipReport" class="space-y-4">
          <!-- Bus Factor Status Banner -->
          <div
            class="p-3.5 rounded-xl border flex items-start gap-3"
            :class="ownershipReport.busFactorAlert === 'critical'
              ? 'border-rose/30 bg-rose/10'
              : ownershipReport.busFactorAlert === 'warning'
              ? 'border-amber/30 bg-amber/10'
              : 'border-emerald/30 bg-emerald/10'"
          >
            <ShieldAlert v-if="ownershipReport.busFactorAlert === 'critical'" class="w-5 h-5 text-rose shrink-0 mt-0.5" />
            <AlertTriangle v-else-if="ownershipReport.busFactorAlert === 'warning'" class="w-5 h-5 text-amber shrink-0 mt-0.5" />
            <ShieldCheck v-else class="w-5 h-5 text-emerald shrink-0 mt-0.5" />

            <div class="space-y-1 text-xs">
              <div class="flex items-center gap-2">
                <span class="font-bold text-sm text-ink">
                  {{ t("ownership.busfactor", { count: ownershipReport.busFactor }) }}
                </span>
                <span
                  class="px-2 py-0.5 rounded text-[10px] font-semibold"
                  :class="ownershipReport.busFactorAlert === 'critical'
                    ? 'bg-rose text-white'
                    : ownershipReport.busFactorAlert === 'warning'
                    ? 'bg-amber text-black'
                    : 'bg-emerald text-white'"
                >
                  {{ ownershipReport.busFactorAlert === 'critical'
                    ? t("ownership.alert.critical")
                    : ownershipReport.busFactorAlert === 'warning'
                    ? t("ownership.alert.warning")
                    : t("ownership.alert.healthy") }}
                </span>
              </div>
              <p class="text-ink-secondary leading-relaxed">
                {{ ownershipReport.recommendations[0] || t("ownership.desc") }}
              </p>
            </div>
          </div>

          <!-- Overall Stats -->
          <div class="flex flex-wrap items-center gap-3 text-xs">
            <span class="px-3 py-1.5 rounded-lg bg-surface border border-line font-medium text-ink">
              {{ t("ownership.total.commits", { count: ownershipReport.totalCommits }) }}
            </span>
            <span class="px-3 py-1.5 rounded-lg bg-surface border border-line font-medium text-ink">
              {{ t("ownership.total.lines", { count: ownershipReport.totalLines }) }}
            </span>
          </div>

          <!-- Owners List -->
          <div class="space-y-2">
            <h4 class="text-xs font-bold text-ink flex items-center gap-1.5">
              <Users class="w-3.5 h-3.5 text-accent" />
              <span>{{ t("ownership.title") }}</span>
            </h4>

            <div class="space-y-2">
              <div
                v-for="owner in ownershipReport.owners"
                :key="owner.author"
                class="p-3 rounded-xl border border-line bg-surface/60 space-y-2"
              >
                <div class="flex items-center justify-between">
                  <div class="flex items-center gap-2">
                    <span class="font-bold text-xs text-ink">{{ owner.author }}</span>
                    <span class="text-[10px] text-ink-muted font-mono">{{ owner.email }}</span>
                    <span
                      class="px-1.5 py-0.5 rounded text-[10px] font-medium"
                      :class="owner.role === 'primary_owner'
                        ? 'bg-purple/15 text-purple border border-purple/30'
                        : owner.role === 'major_contributor'
                        ? 'bg-blue/15 text-blue border border-blue/30'
                        : 'bg-canvas text-ink-muted border border-line'"
                    >
                      {{ owner.role === 'primary_owner'
                        ? t("ownership.role.primary")
                        : owner.role === 'major_contributor'
                        ? t("ownership.role.major")
                        : t("ownership.role.minor") }}
                    </span>
                  </div>

                  <span class="text-xs font-bold font-mono text-ink">
                    {{ owner.percentage.toFixed(1) }}%
                  </span>
                </div>

                <!-- Progress bar -->
                <div class="w-full h-1.5 bg-canvas rounded-full overflow-hidden border border-line">
                  <div
                    class="h-full rounded-full bg-accent transition-all duration-300"
                    :style="{ width: `${owner.percentage}%` }"
                  />
                </div>

                <div class="flex items-center justify-between text-[11px] text-ink-muted font-mono">
                  <span>{{ owner.linesAuthored }} lines ({{ owner.commitCount }} commits)</span>
                  <span v-if="owner.lastActiveDate">
                    {{ t("ownership.last.active", { date: owner.lastActiveDate.substring(0, 10) }) }}
                  </span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </UiModal>
</template>
