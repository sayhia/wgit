<script setup lang="ts">
import { ref, watch, onMounted } from "vue";
import {
  Disc,
  RotateCcw,
  ShieldCheck,
  AlertCircle,
  Trash2,
  Plus,
  RefreshCw,
  CheckCircle2,
  FileCode,
} from "lucide-vue-next";
import UiModal from "../ui/UiModal.vue";
import UiButton from "../ui/UiButton.vue";
import { gitService } from "../../services/gitService";
import { t } from "../../i18n";
import type { FlightSnapshotInfo, SnapshotRestoreResult } from "../../types";

const props = defineProps<{
  repoPath: string;
}>();

const emit = defineEmits<{
  (e: "restored", result: SnapshotRestoreResult): void;
}>();

const isOpen = ref(false);
const snapshots = ref<FlightSnapshotInfo[]>([]);
const isLoading = ref(false);
const isTakingSnapshot = ref(false);
const isRestoring = ref(false);
const error = ref<string | null>(null);
const successMessage = ref<string | null>(null);

async function loadSnapshots() {
  if (!props.repoPath) return;
  isLoading.value = true;
  error.value = null;
  try {
    snapshots.value = await gitService.listFlightSnapshots(props.repoPath);
  } catch (err: unknown) {
    error.value = err instanceof Error ? err.message : String(err);
  } finally {
    isLoading.value = false;
  }
}

async function takeSnapshot() {
  if (!props.repoPath) return;
  isTakingSnapshot.value = true;
  error.value = null;
  successMessage.value = null;
  try {
    const snap = await gitService.createFlightRecorderSnapshot(
      props.repoPath,
      "manual",
      ""
    );
    snapshots.value = [snap, ...snapshots.value];
    successMessage.value = t("flight_recorder.snapshot.success");
    setTimeout(() => {
      successMessage.value = null;
    }, 2500);
  } catch (err: unknown) {
    error.value = err instanceof Error ? err.message : String(err);
  } finally {
    isTakingSnapshot.value = false;
  }
}

async function restoreSnapshot(snapshotId: string) {
  if (!props.repoPath) return;
  isRestoring.value = true;
  error.value = null;
  successMessage.value = null;
  try {
    const res = await gitService.restoreFlightSnapshot(props.repoPath, snapshotId);
    if (res.success) {
      successMessage.value = t("flight_recorder.restore.success", { count: res.restoredFiles });
      emit("restored", res);
      setTimeout(() => {
        isOpen.value = false;
      }, 1500);
    } else {
      error.value = res.message;
    }
  } catch (err: unknown) {
    error.value = t("flight_recorder.restore.failed", {
      error: err instanceof Error ? err.message : String(err),
    });
  } finally {
    isRestoring.value = false;
  }
}

async function purgeOld() {
  if (!props.repoPath) return;
  isLoading.value = true;
  try {
    const purged = await gitService.purgeFlightSnapshots(props.repoPath, 10);
    successMessage.value = t("flight_recorder.purge.success", { count: purged });
    void loadSnapshots();
    setTimeout(() => {
      successMessage.value = null;
    }, 2500);
  } catch (err: unknown) {
    error.value = err instanceof Error ? err.message : String(err);
  } finally {
    isLoading.value = false;
  }
}

watch(
  () => isOpen.value,
  (val) => {
    if (val) {
      void loadSnapshots();
    }
  }
);

onMounted(() => {
  if (props.repoPath) {
    void loadSnapshots();
  }
});
</script>

<template>
  <div class="inline-flex items-center">
    <!-- Trigger Pill -->
    <button
      type="button"
      data-testid="flight-recorder-trigger-btn"
      class="inline-flex items-center gap-1.5 px-2 py-1 rounded-md text-[11px] font-medium border transition-colors cursor-pointer"
      :class="
        snapshots.length > 0
          ? 'bg-indigo-950/40 border-indigo-500/40 text-indigo-300 hover:bg-indigo-900/50'
          : 'bg-neutral-800/60 border-neutral-700/60 text-neutral-400 hover:bg-neutral-700/60'
      "
      :title="t('flight_recorder.title')"
      @click="isOpen = true"
    >
      <Disc class="w-3.5 h-3.5" :class="{ 'text-indigo-400 animate-spin': isTakingSnapshot }" />
      <span>{{ t('flight_recorder.badge') }}</span>
      <span
        v-if="snapshots.length > 0"
        class="bg-indigo-900/80 text-indigo-200 px-1 rounded-full text-[10px] font-mono"
      >
        {{ snapshots.length }}
      </span>
    </button>

    <!-- Modal -->
    <UiModal
      :open="isOpen"
      :title="t('flight_recorder.title')"
      width="max-w-2xl"
      @update:open="isOpen = $event"
    >
      <div class="space-y-4 text-xs text-neutral-300">
        <!-- Subtitle & Controls -->
        <div class="flex items-center justify-between pb-2 border-b border-neutral-700/60">
          <p class="text-neutral-400">
            {{ t('flight_recorder.subtitle') }}
          </p>
          <div class="flex items-center gap-2">
            <UiButton
              size="sm"
              variant="secondary"
              :disabled="isLoading || snapshots.length <= 5"
              class="flex items-center gap-1"
              @click="purgeOld"
            >
              <Trash2 class="w-3.5 h-3.5" />
              <span>{{ t('flight_recorder.btn.purge') }}</span>
            </UiButton>
            <UiButton
              size="sm"
              variant="primary"
              :disabled="isTakingSnapshot"
              data-testid="take-flight-snapshot-btn"
              class="flex items-center gap-1.5"
              @click="takeSnapshot"
            >
              <Plus class="w-3.5 h-3.5" />
              <span>{{ t('flight_recorder.btn.snapshot') }}</span>
            </UiButton>
          </div>
        </div>

        <!-- Banner Feedback -->
        <div
          v-if="successMessage"
          class="p-2.5 bg-emerald-950/40 border border-emerald-500/30 rounded-lg text-emerald-300 flex items-center gap-2"
        >
          <CheckCircle2 class="w-4 h-4 text-emerald-400 shrink-0" />
          <span>{{ successMessage }}</span>
        </div>

        <div
          v-if="error"
          class="p-2.5 bg-red-950/40 border border-red-500/30 rounded-lg text-red-300 flex items-center gap-2"
        >
          <AlertCircle class="w-4 h-4 text-red-400 shrink-0" />
          <span>{{ error }}</span>
        </div>

        <!-- Snapshot List -->
        <div v-if="isLoading" class="py-10 flex flex-col items-center justify-center gap-2 text-neutral-400">
          <RefreshCw class="w-6 h-6 animate-spin text-indigo-400" />
          <span>{{ t('common.loading') }}</span>
        </div>

        <div
          v-else-if="snapshots.length === 0"
          class="py-10 text-center text-neutral-500 bg-neutral-900/30 rounded-lg border border-neutral-800"
        >
          <Disc class="w-8 h-8 text-neutral-600 mx-auto mb-2" />
          <p>{{ t('flight_recorder.list.empty') }}</p>
        </div>

        <div v-else class="space-y-2 max-h-[380px] overflow-y-auto pr-1">
          <div
            v-for="snap in snapshots"
            :key="snap.id"
            class="p-3 bg-neutral-800/60 hover:bg-neutral-800/90 border border-neutral-700/60 rounded-lg transition-colors flex items-center justify-between gap-3"
          >
            <!-- Left Info -->
            <div class="flex items-center gap-3 min-w-0 flex-1">
              <Disc class="w-4 h-4 text-indigo-400 shrink-0" />
              <div class="min-w-0 flex-1">
                <div class="flex items-center gap-2">
                  <span class="font-medium text-neutral-200 truncate">
                    {{ snap.message }}
                  </span>
                  <span
                    class="px-1.5 py-0.5 rounded text-[10px] font-mono shrink-0"
                    :class="
                      snap.triggerAction.startsWith('pre-')
                        ? 'bg-amber-950/60 text-amber-300 border border-amber-500/30'
                        : 'bg-indigo-950/60 text-indigo-300 border border-indigo-500/30'
                    "
                  >
                    {{ snap.triggerAction }}
                  </span>
                </div>
                <div class="text-[11px] text-neutral-400 flex items-center gap-2 mt-1">
                  <span>{{ snap.timestamp }}</span>
                  <span>·</span>
                  <span class="font-mono text-indigo-300">
                    {{ snap.treeHash ? snap.treeHash.slice(0, 7) : snap.id }}
                  </span>
                  <span>·</span>
                  <span class="text-neutral-400">
                    {{ t('flight_recorder.file_count', { count: snap.fileCount }) }}
                  </span>
                </div>
              </div>
            </div>

            <!-- Restore Action Button -->
            <UiButton
              size="sm"
              variant="secondary"
              data-testid="restore-flight-snapshot-btn"
              :disabled="isRestoring"
              class="flex items-center gap-1.5 shrink-0 hover:border-emerald-500/50 hover:text-emerald-300"
              @click="restoreSnapshot(snap.id)"
            >
              <RotateCcw class="w-3.5 h-3.5" :class="{ 'animate-spin': isRestoring }" />
              <span>{{ t('flight_recorder.btn.restore') }}</span>
            </UiButton>
          </div>
        </div>

        <!-- Footer -->
        <div class="flex justify-end pt-3 border-t border-neutral-800">
          <UiButton
            size="sm"
            variant="secondary"
            @click="isOpen = false"
          >
            {{ t('common.cancel') }}
          </UiButton>
        </div>
      </div>
    </UiModal>
  </div>
</template>
