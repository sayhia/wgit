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
  GitCompare,
} from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import { t } from "../../i18n";
import type { FileTimelineEntry, CodeOwnershipReport, DiffComparisonResult, FileLineageReport } from "../../types";
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

const activeSubTab = ref<"player" | "ownership" | "range">("player");
const timeline = ref<FileTimelineEntry[]>([]);
const currentIndex = ref(0);
const isPlaying = ref(false);
const playbackSpeed = ref<1 | 2 | 5>(1);
const ownershipReport = ref<CodeOwnershipReport | null>(null);
const lineageReport = ref<FileLineageReport | null>(null);
const isLoadingTimeline = ref(false);
const isLoadingOwnership = ref(false);
let playTimer: ReturnType<typeof setInterval> | null = null;

// Dual Checkpoint Range Diff State
const baseCommitSha = ref("");
const targetCommitSha = ref("");
const rangeDiff = ref<DiffComparisonResult | null>(null);
const isLoadingRangeDiff = ref(false);

async function loadRangeDiff() {
  if (!props.currentRepoPath || !props.filePath || !baseCommitSha.value || !targetCommitSha.value) return;
  isLoadingRangeDiff.value = true;
  try {
    const res = await gitService.getFileCommitRangeDiff(props.currentRepoPath, props.filePath, baseCommitSha.value, targetCommitSha.value);
    rangeDiff.value = res;
  } catch (err: any) {
    emit("notify", String(err?.message || err));
  } finally {
    isLoadingRangeDiff.value = false;
  }
}

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
    if (list && list.length > 0) {
      baseCommitSha.value = list[0]?.commitHash || "";
      targetCommitSha.value = list[list.length - 1]?.commitHash || "";
    }
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

  try {
    lineageReport.value = await gitService.traceFileRenamesAcrossHistory(props.currentRepoPath, props.filePath);
  } catch (err: any) {
    // lineage tracing is non-fatal
    console.debug("Failed to trace file lineage:", err);
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
  },
  { immediate: true }
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
          <button
            data-testid="timelapse-tab-range"
            @click="activeSubTab = 'range'"
            class="px-2.5 py-1 rounded text-xs font-medium transition cursor-pointer"
            :class="activeSubTab === 'range' ? 'bg-raised text-accent shadow-xs' : 'text-ink-secondary hover:text-ink'"
          >
            {{ t("timelapse.tab.range") }}
          </button>
        </div>
      </div>

      <!-- File Lineage Forensic Breadcrumb (Phase 51) -->
      <div
        v-if="lineageReport && lineageReport.renames && lineageReport.renames.length > 0"
        data-testid="file-lineage-breadcrumb"
        class="px-3 py-1.5 rounded-lg bg-accent-bg border border-accent/25 text-xs flex items-center gap-2 flex-wrap"
      >
        <span class="text-[10px] font-bold text-accent uppercase tracking-wider">{{ t("timelapse.lineage.title") }}:</span>
        <span class="text-[11px] font-mono text-ink-muted">
          {{ t("timelapse.lineage.origin") }}: <span class="text-ink font-semibold">{{ lineageReport.originPath }}</span>
        </span>
        <span class="text-ink-muted text-[10px]">→</span>
        <div class="flex items-center gap-1 flex-wrap font-mono text-[11px]">
          <span
            v-for="(r, idx) in lineageReport.renames"
            :key="idx"
            class="px-1.5 py-0.5 rounded bg-surface border border-line text-[10px] text-ink"
            :title="`${r.similarity}% at ${r.commitHash.slice(0, 7)}: ${r.oldPath} -> ${r.newPath}`"
          >
            {{ r.oldPath }} ➔ {{ r.newPath }} ({{ r.commitHash.slice(0, 7) }})
          </span>
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
          <div v-if="currentEntry" class="p-3 rounded-xl border border-line bg-surface flex flex-wrap items-center justify-between gap-2">
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
                  <span>{{ (currentEntry.authorDate || "").replace("T", " ").substring(0, 16) }}</span>
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
              <Pause v-if="isPlaying" class="w-4 h-4" aria-hidden="true" />
              <Play v-else class="w-4 h-4" aria-hidden="true" />
            </button>

            <button
              @click="stepPrev"
              :disabled="currentIndex === 0"
              class="p-1.5 rounded-md bg-canvas hover:bg-raised border border-line text-ink transition cursor-pointer disabled:opacity-30"
              :title="t('timelapse.step.prev')"
            >
              <SkipBack class="w-3.5 h-3.5" aria-hidden="true" />
            </button>

            <button
              @click="stepNext"
              :disabled="currentIndex === timeline.length - 1"
              class="p-1.5 rounded-md bg-canvas hover:bg-raised border border-line text-ink transition cursor-pointer disabled:opacity-30"
              :title="t('timelapse.step.next')"
            >
              <SkipForward class="w-3.5 h-3.5" aria-hidden="true" />
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
          <div class="rounded-xl border border-line bg-canvas max-h-[420px] overflow-auto font-mono text-xs">
            <div
              v-for="(line, lineIdx) in numberedLines"
              :key="lineIdx"
              class="flex items-start hover:bg-raised px-2 py-0.5 leading-relaxed"
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
                class="p-3 rounded-xl border border-line bg-surface space-y-2"
              >
                <div class="flex items-center justify-between">
                  <div class="flex items-center gap-2">
                    <span class="font-bold text-xs text-ink">{{ owner.author }}</span>
                    <span class="text-[10px] text-ink-muted font-mono">{{ owner.email }}</span>
                    <span
                      class="px-1.5 py-0.5 rounded text-[10px] font-medium"
                      :class="owner.role === 'primary_owner'
                        ? 'bg-purple-bg text-purple border border-purple/30'
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

      <!-- Range Diff Subtab (Dual Checkpoints) -->
      <div v-else-if="activeSubTab === 'range'" class="space-y-4" data-testid="timelapse-range-diff">
          <div class="p-3.5 rounded-xl border border-line bg-surface space-y-3">
            <div class="flex items-center justify-between">
              <span class="text-xs font-bold text-ink flex items-center gap-1.5">
                <GitCompare class="w-4 h-4 text-accent" />
                <span>{{ t("timelapse.range.title") }}</span>
              </span>
              <button
                type="button"
                @click="loadRangeDiff"
                :disabled="isLoadingRangeDiff || !baseCommitSha || !targetCommitSha"
                class="flex items-center gap-1 px-3 py-1.5 rounded-lg bg-accent text-accent-contrast text-xs font-semibold hover:opacity-90 transition cursor-pointer disabled:opacity-40"
              >
                <GitCompare class="w-3.5 h-3.5" :class="{ 'animate-spin': isLoadingRangeDiff }" aria-hidden="true" />
                <span>{{ isLoadingRangeDiff ? t("timelapse.range.comparing") : t("timelapse.range.compare") }}</span>
              </button>
            </div>

            <div class="grid grid-cols-2 gap-3">
              <div class="flex flex-col gap-1">
                <label class="text-[10px] text-ink-muted font-semibold">{{ t("timelapse.range.base") }}</label>
                <select
                  v-model="baseCommitSha"
                  class="px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs font-mono text-ink focus:outline-none focus:border-accent"
                >
                  <option v-for="c in timeline" :key="`base-${c.commitHash}`" :value="c.commitHash">
                    {{ c.commitHash.substring(0, 7) }} - {{ c.subject.slice(0, 28) }}
                  </option>
                </select>
              </div>

              <div class="flex flex-col gap-1">
                <label class="text-[10px] text-ink-muted font-semibold">{{ t("timelapse.range.target") }}</label>
                <select
                  v-model="targetCommitSha"
                  class="px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs font-mono text-ink focus:outline-none focus:border-accent"
                >
                  <option v-for="c in timeline" :key="`target-${c.commitHash}`" :value="c.commitHash">
                    {{ c.commitHash.substring(0, 7) }} - {{ c.subject.slice(0, 28) }}
                  </option>
                </select>
              </div>
            </div>
          </div>

          <!-- Range Diff Results -->
          <div v-if="rangeDiff" class="space-y-2">
            <div class="flex items-center justify-between text-xs font-mono px-1">
              <span class="text-ink-muted">{{ rangeDiff.refA.substring(0, 7) }}...{{ rangeDiff.refB.substring(0, 7) }}</span>
              <div class="flex items-center gap-2">
                <span class="text-success font-semibold">+{{ rangeDiff.additions }}</span>
                <span class="text-danger font-semibold">-{{ rangeDiff.deletions }}</span>
              </div>
            </div>

            <div class="border border-line rounded-lg bg-canvas overflow-hidden max-h-96 overflow-y-auto">
              <div
                v-for="(line, idx) in rangeDiff.diff.split('\n')"
                :key="idx"
                class="px-3 py-0.5 text-xs font-mono leading-5 whitespace-pre"
                :class="line.startsWith('+') ? 'bg-success-bg text-success' : line.startsWith('-') ? 'bg-danger-bg text-danger' : line.startsWith('@@') ? 'bg-purple-bg text-purple font-semibold' : 'text-ink-secondary'"
              >
                {{ line || ' ' }}
              </div>
            </div>
          </div>
          <div v-else class="text-center text-xs text-ink-muted py-8 border border-dashed border-line rounded-xl">
            {{ t("timelapse.range.empty") }}
          </div>
        </div>
      </div>
  </UiModal>
</template>
