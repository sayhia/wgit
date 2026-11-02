<script setup lang="ts">
import { computed, ref } from "vue";
import { FolderOpen, FolderGit2, CheckSquare, Square } from "lucide-vue-next";
import { t } from "../../i18n";
import { gitService } from "../../services/gitService";
import type { ProjectDetailItem } from "../../types";

/**
 * Add local repositories by scanning a folder.
 *
 * The root is chosen with the native picker rather than typed: a hand-typed
 * path is wrong more often than not (trailing slash, `~` that never expands,
 * a folder that moved), and every wrong guess costs a full directory walk
 * before it fails.
 *
 * Results are multi-select so scanning a workspace can add everything at once.
 * Adding only writes the recent list — it does not switch the current
 * workspace, because opening five repositories in a row is not what "add"
 * means. Clicking a row still opens that one repository.
 */
defineProps<{ highlighted?: boolean }>();
const emit = defineEmits<{
  (e: "opened", path: string): void;
  (e: "failed", message: string): void;
  (e: "started"): void;
  (e: "added", count: number): void;
}>();

const scanRoot = ref("");
const rootButtonRef = ref<HTMLButtonElement | null>(null);
const scannedProjects = ref<ProjectDetailItem[]>([]);
const isScanning = ref(false);
const isAdding = ref(false);
const selected = ref<Set<string>>(new Set());

const allSelected = computed(
  () => scannedProjects.value.length > 0 && selected.value.size === scannedProjects.value.length
);

async function pickRoot() {
  try {
    const folder = await gitService.pickFolderDialog();
    if (folder) {
      scanRoot.value = folder;
      // A new root invalidates both the old results and any selection in them.
      scannedProjects.value = [];
      selected.value = new Set();
    }
  } catch {
    // the user dismissed the native dialog; nothing to report
  }
}

async function runScan() {
  if (isScanning.value || !scanRoot.value) return;
  emit("started");
  isScanning.value = true;
  selected.value = new Set();
  try {
    const paths = await gitService.scanLocalProjects(scanRoot.value);
    scannedProjects.value = (await gitService.getProjectsDetails(paths || [], "")) || [];
    if (scannedProjects.value.length === 0) {
      emit("failed", t("onboard.error.scan.none", { root: scanRoot.value }));
    }
  } catch (err: any) {
    emit("failed", t("onboard.error.scan.failed", { error: err?.message || err }));
  } finally {
    isScanning.value = false;
  }
}

function toggle(path: string) {
  const next = new Set(selected.value);
  if (next.has(path)) next.delete(path);
  else next.add(path);
  selected.value = next;
}

function toggleAll() {
  selected.value = allSelected.value
    ? new Set()
    : new Set(scannedProjects.value.map((project) => project.path));
}

async function addSelected() {
  if (isAdding.value) return;
  if (selected.value.size === 0) {
    emit("failed", t("onboard.scan.add.none"));
    return;
  }
  const paths = [...selected.value];
  isAdding.value = true;
  emit("started");
  try {
    const updated = await gitService.addReposToRecent(paths);
    if (updated) {
      emit("added", updated.length);
    }
    selected.value = new Set();
  } catch (err: any) {
    emit("failed", t("onboard.scan.add.failed", { error: err?.message || err }));
  } finally {
    isAdding.value = false;
  }
}

/** Called by the parent when a shortcut jumps straight to this card. */
async function focusInput() {
  rootButtonRef.value?.scrollIntoView({ block: "center", behavior: "smooth" });
  rootButtonRef.value?.focus();
}

defineExpose({ focusInput });
</script>

<template>
  <div
    class="rounded-2xl bg-canvas border overflow-hidden transition"
    :class="highlighted ? 'border-accent/60 ring-1 ring-accent/30' : 'border-line'"
  >
    <div class="px-3.5 py-2 border-b border-line/70 flex items-center gap-1.5">
      <FolderOpen class="w-3.5 h-3.5 text-ink-muted" />
      <span class="text-[11px] font-semibold text-ink-muted uppercase tracking-wider">{{ t("onboard.scan") }}</span>
    </div>

    <div class="p-2 space-y-2">
      <!-- Root picker + scan -->
      <div class="flex items-center gap-1.5">
        <button
          ref="rootButtonRef"
          type="button"
          :aria-label="t('onboard.scan.placeholder')"
          class="flex-1 min-w-0 px-2.5 py-1.5 rounded-lg bg-surface border border-line text-xs text-left font-mono transition cursor-pointer hover:border-accent focus:outline-none focus:border-accent"
          :class="scanRoot ? 'text-ink truncate' : 'text-ink-muted'"
          :title="scanRoot || t('onboard.scan.pick')"
          @click="pickRoot"
        >
          {{ scanRoot || t("onboard.scan.pick") }}
        </button>
        <button
          type="button"
          @click="runScan"
          :disabled="!scanRoot || isScanning"
          class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-accent transition cursor-pointer disabled:opacity-40 shrink-0"
        >
          {{ isScanning ? t("onboard.scanning") : t("onboard.scan.run") }}
        </button>
      </div>

      <!-- Results -->
      <div v-if="scannedProjects.length > 0" class="space-y-1">
        <div class="flex items-center justify-between px-1 pt-0.5">
          <button
            type="button"
            class="flex items-center gap-1 text-[10px] font-semibold text-ink-muted hover:text-accent transition cursor-pointer"
            @click="toggleAll"
          >
            <CheckSquare v-if="allSelected" class="w-3 h-3" />
            <Square v-else class="w-3 h-3" />
            <span>{{ allSelected ? t("onboard.scan.select.none") : t("onboard.scan.select.all") }}</span>
          </button>
          <span class="text-[10px] text-ink-muted">{{ t("onboard.scan.found", { count: scannedProjects.length }) }}</span>
        </div>

        <div class="max-h-56 overflow-y-auto space-y-0.5">
          <div
            v-for="project in scannedProjects"
            :key="project.path"
            class="flex items-center gap-2 px-2 py-1.5 rounded-lg hover:bg-raised transition"
          >
            <input
              type="checkbox"
              :checked="selected.has(project.path)"
              :aria-label="project.name"
              class="w-3 h-3 rounded border-line accent-accent cursor-pointer shrink-0"
              @change="toggle(project.path)"
            />
            <button
              type="button"
              class="flex items-center gap-2 min-w-0 flex-1 text-left cursor-pointer"
              :title="project.path"
              @click="emit('opened', project.path)"
            >
              <FolderGit2 class="w-3.5 h-3.5 text-ink-muted shrink-0" />
              <span class="text-xs text-ink font-semibold truncate">{{ project.name }}</span>
              <span class="px-1.5 py-0.2 rounded bg-accent-bg text-accent text-[9px] font-bold shrink-0">
                {{ project.engineType }}
              </span>
              <span class="text-[10px] text-ink-muted font-mono truncate">{{ project.currentBranch }}</span>
              <span v-if="project.isDirty" class="text-[10px] text-warning shrink-0">
                {{ t("onboard.scan.dirty", { count: project.dirtyCount }) }}
              </span>
              <span
                v-if="project.statusWarning"
                class="text-[10px] font-bold text-warning shrink-0 cursor-help"
                role="status"
                :title="project.statusWarning"
                :aria-label="project.statusWarning"
              >!</span>
            </button>
          </div>
        </div>

        <div class="flex items-center justify-between gap-2 pt-0.5">
          <span class="text-[10px] text-ink-muted font-mono truncate">{{ scanRoot }}</span>
          <button
            type="button"
            @click="addSelected"
            :disabled="isAdding || selected.size === 0"
            class="px-2.5 py-1 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-[11px] font-semibold transition cursor-pointer disabled:opacity-40 shrink-0"
          >
            {{ t("onboard.scan.add.selected", { count: selected.size }) }}
          </button>
        </div>
      </div>

      <div v-else-if="!isScanning" class="py-2 text-center text-[11px] text-ink-muted">
        {{ t("onboard.scan.hint") }}
      </div>
    </div>
  </div>
</template>
