<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import { ShieldCheck, Play, Edit3, RefreshCw, CheckCircle2, AlertTriangle, FileCode2, Terminal, Check } from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import { t } from "../../i18n";
import UiModal from "../ui/UiModal.vue";
import UiButton from "../ui/UiButton.vue";
import type { GitHookInfo, GitHooksFrameworkInfo, GitHookRunResult } from "../../types";

const props = defineProps<{
  currentRepoPath: string;
}>();

const emit = defineEmits<{
  (e: "notify", msg: string): void;
}>();

const hooks = ref<GitHookInfo[]>([]);
const framework = ref<GitHooksFrameworkInfo | null>(null);
const isLoading = ref(false);
const searchQuery = ref("");

// Editor state
const showEditModal = ref(false);
const editingHookName = ref("");
const editingScript = ref("");
const isSaving = ref(false);

// Test Run state
const showRunModal = ref(false);
const runningHookName = ref("");
const isRunning = ref(false);
const runResult = ref<GitHookRunResult | null>(null);

const KNOWN_HOOKS = [
  { name: "pre-commit", descKey: "settings.hooks.desc.pre_commit" },
  { name: "commit-msg", descKey: "settings.hooks.desc.commit_msg" },
  { name: "pre-push", descKey: "settings.hooks.desc.pre_push" },
  { name: "prepare-commit-msg", descKey: "settings.hooks.desc.prepare_commit_msg" },
  { name: "post-commit", descKey: "settings.hooks.desc.post_commit" },
  { name: "post-checkout", descKey: "settings.hooks.desc.post_checkout" },
  { name: "post-merge", descKey: "settings.hooks.desc.post_merge" },
  { name: "pre-rebase", descKey: "settings.hooks.desc.pre_rebase" },
];

const filteredHooks = computed(() => {
  const q = searchQuery.value.trim().toLowerCase();
  if (!q) return KNOWN_HOOKS;
  return KNOWN_HOOKS.filter((h) => h.name.toLowerCase().includes(q) || t(h.descKey).toLowerCase().includes(q));
});

function getHookInfo(name: string): GitHookInfo | undefined {
  return hooks.value.find((h) => h.name === name);
}

async function loadHooks() {
  if (!props.currentRepoPath) return;
  isLoading.value = true;
  try {
    const [hList, fw] = await Promise.all([
      gitService.getGitHooks(props.currentRepoPath),
      gitService.detectHooksFramework(props.currentRepoPath),
    ]);
    hooks.value = hList;
    framework.value = fw;
  } catch (err: any) {
    emit("notify", String(err?.message || err));
  } finally {
    isLoading.value = false;
  }
}

async function toggleHook(name: string) {
  if (!props.currentRepoPath) return;
  const existing = getHookInfo(name);
  const willEnable = !existing?.installed;
  try {
    const content = existing?.installed ? "" : "#!/bin/sh\n\n# wgit auto-generated hook\nexit 0\n";
    await gitService.toggleGitHook(props.currentRepoPath, name, willEnable, content);
    emit("notify", willEnable ? t("settings.hooks.toast.enabled", { name }) : t("settings.hooks.toast.disabled", { name }));
    await loadHooks();
  } catch (err: any) {
    emit("notify", String(err?.message || err));
  }
}

async function openEditor(name: string) {
  if (!props.currentRepoPath) return;
  editingHookName.value = name;
  try {
    const res = await gitService.readGitHook(props.currentRepoPath, name);
    editingScript.value = res.content || "#!/bin/sh\n\nexit 0\n";
  } catch {
    editingScript.value = "#!/bin/sh\n\nexit 0\n";
  }
  showEditModal.value = true;
}

async function saveEditor() {
  if (!props.currentRepoPath || !editingHookName.value || isSaving.value) return;
  isSaving.value = true;
  try {
    await gitService.saveGitHook(props.currentRepoPath, editingHookName.value, editingScript.value, true);
    emit("notify", t("settings.hooks.toast.saved", { name: editingHookName.value }));
    showEditModal.value = false;
    await loadHooks();
  } catch (err: any) {
    emit("notify", String(err?.message || err));
  } finally {
    isSaving.value = false;
  }
}

async function runHookDryRun(name: string) {
  if (!props.currentRepoPath || isRunning.value) return;
  runningHookName.value = name;
  isRunning.value = true;
  runResult.value = null;
  showRunModal.value = true;
  try {
    const res = await gitService.runGitHook(props.currentRepoPath, name, []);
    runResult.value = res;
  } catch (err: any) {
    runResult.value = {
      exitCode: 1,
      success: false,
      stdout: "",
      stderr: String(err?.message || err),
      durationMs: 0,
    };
  } finally {
    isRunning.value = false;
  }
}

onMounted(() => {
  void loadHooks();
});
</script>

<template>
  <div class="space-y-4">
    <!-- Header -->
    <div class="flex flex-wrap items-center justify-between gap-3 pb-3 border-b border-line">
      <div class="flex flex-col">
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <ShieldCheck class="w-4 h-4 text-accent" />
          <span>{{ t("settings.hooks.title") }}</span>
        </h3>
        <p class="text-xs text-ink-muted mt-0.5">
          {{ t("settings.hooks.subtitle") }}
        </p>
      </div>

      <div class="flex items-center gap-2">
        <span
          v-if="framework && framework.type !== 'none'"
          class="px-2 py-0.5 rounded text-[11px] font-mono font-medium bg-purple/15 text-purple border border-purple/30"
          :title="framework.configFile"
        >
          {{ framework.type }}
        </span>
        <button
          @click="loadHooks"
          :disabled="isLoading"
          class="flex items-center gap-1.5 px-3 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-medium text-ink transition cursor-pointer"
        >
          <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isLoading }" aria-hidden="true" />
          <span>{{ t("settings.aliases.refresh") }}</span>
        </button>
      </div>
    </div>

    <!-- Search Input -->
    <div class="relative">
      <input
        v-model="searchQuery"
        type="text"
        :aria-label="t('settings.hooks.search.placeholder')"
        :placeholder="t('settings.hooks.search.placeholder')"
        class="w-full px-3 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
      />
    </div>

    <!-- Hook Cards Grid -->
    <div class="grid grid-cols-1 md:grid-cols-2 gap-3" data-testid="hooks-grid">
      <div
        v-for="hook in filteredHooks"
        :key="hook.name"
        :data-testid="`hook-card-${hook.name}`"
        class="p-3.5 rounded-xl border border-line bg-surface/40 hover:bg-surface/70 transition flex flex-col justify-between gap-3"
      >
        <div class="flex items-start justify-between gap-2">
          <div class="flex flex-col min-w-0">
            <div class="flex items-center gap-2">
              <span class="font-mono text-xs font-bold text-ink">{{ hook.name }}</span>
              <span
                class="px-1.5 py-0.2 rounded text-[9px] font-semibold uppercase"
                :class="getHookInfo(hook.name)?.installed ? 'bg-success/20 text-success' : 'bg-surface border border-line text-ink-muted'"
              >
                {{ getHookInfo(hook.name)?.installed ? t("settings.hooks.status.enabled") : t("settings.hooks.status.disabled") }}
              </span>
            </div>
            <p class="text-[11px] text-ink-muted mt-1 leading-relaxed">
              {{ t(hook.descKey) }}
            </p>
          </div>
        </div>

        <!-- Action Bar -->
        <div class="flex items-center justify-between pt-2 border-t border-line/50">
          <button
            type="button"
            :data-testid="`hook-toggle-${hook.name}`"
            @click="toggleHook(hook.name)"
            class="text-[11px] font-medium transition cursor-pointer hover:underline"
            :class="getHookInfo(hook.name)?.installed ? 'text-danger' : 'text-success'"
          >
            {{ getHookInfo(hook.name)?.installed ? t("settings.hooks.action.disable") : t("settings.hooks.action.enable") }}
          </button>

          <div class="flex items-center gap-1.5">
            <button
              type="button"
              :data-testid="`hook-run-${hook.name}`"
              @click="runHookDryRun(hook.name)"
              :disabled="!getHookInfo(hook.name)?.installed"
              class="flex items-center gap-1 px-2 py-1 rounded bg-canvas hover:bg-raised border border-line text-[11px] font-medium text-ink transition cursor-pointer disabled:opacity-40"
              :title="t('settings.hooks.action.run.hint')"
            >
              <Play class="w-3 h-3 text-accent" aria-hidden="true" />
              <span>{{ t("settings.hooks.action.run") }}</span>
            </button>
            <button
              type="button"
              :data-testid="`hook-edit-${hook.name}`"
              @click="openEditor(hook.name)"
              class="flex items-center gap-1 px-2 py-1 rounded bg-canvas hover:bg-raised border border-line text-[11px] font-medium text-ink transition cursor-pointer"
              :title="t('settings.hooks.action.edit')"
            >
              <Edit3 class="w-3 h-3 text-ink-muted" aria-hidden="true" />
              <span>{{ t("settings.hooks.action.edit") }}</span>
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Script Editor Modal -->
    <UiModal
      :open="showEditModal"
      :title="t('settings.hooks.modal.edit.title', { name: editingHookName })"
      width="max-w-xl"
      @update:open="showEditModal = $event"
    >
      <template #header>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <FileCode2 class="w-4 h-4 text-accent" />
          <span>{{ t("settings.hooks.modal.edit.title", { name: editingHookName }) }}</span>
        </h3>
      </template>

      <div class="space-y-2">
        <textarea
          v-model="editingScript"
          spellcheck="false"
          rows="12"
          class="w-full p-2.5 rounded-lg bg-canvas border border-line text-xs font-mono text-ink placeholder-ink-muted focus:outline-none focus:border-accent resize-y"
        />
      </div>

      <template #footer>
        <UiButton variant="secondary" @click="showEditModal = false">{{ t("common.cancel") }}</UiButton>
        <UiButton variant="primary" :disabled="isSaving" @click="saveEditor">
          {{ isSaving ? t("common.saving") : t("common.save") }}
        </UiButton>
      </template>
    </UiModal>

    <!-- Dry-Run Execution Output Modal -->
    <UiModal
      :open="showRunModal"
      :title="t('settings.hooks.modal.run.title', { name: runningHookName })"
      width="max-w-lg"
      @update:open="showRunModal = $event"
    >
      <template #header>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <Terminal class="w-4 h-4 text-accent" />
          <span>{{ t("settings.hooks.modal.run.title", { name: runningHookName }) }}</span>
        </h3>
      </template>

      <div class="space-y-3">
        <div v-if="isRunning" class="py-6 flex flex-col items-center justify-center gap-2 text-xs text-ink-muted">
          <RefreshCw class="w-4 h-4 text-accent animate-spin" />
          <span>{{ t("settings.hooks.running") }}</span>
        </div>

        <div
          v-else-if="runResult"
          class="space-y-2"
          :role="runResult.success ? 'status' : 'alert'"
          :aria-live="runResult.success ? 'polite' : 'assertive'"
        >
          <div class="flex items-center justify-between text-xs font-mono pb-2 border-b border-line">
            <span class="flex items-center gap-1.5 font-bold" :class="runResult.success ? 'text-success' : 'text-danger'">
              <CheckCircle2 v-if="runResult.success" class="w-3.5 h-3.5" />
              <AlertTriangle v-else class="w-3.5 h-3.5" />
              <span>Exit Code: {{ runResult.exitCode }}</span>
            </span>
            <span class="text-ink-muted">{{ runResult.durationMs }}ms</span>
          </div>

          <div class="p-2.5 rounded-lg bg-canvas border border-line text-xs font-mono text-ink max-h-56 overflow-y-auto whitespace-pre-wrap">
            {{ runResult.stdout || runResult.stderr || "(no output)" }}
          </div>
        </div>
      </div>

      <template #footer>
        <UiButton variant="secondary" @click="showRunModal = false">{{ t("common.close") }}</UiButton>
      </template>
    </UiModal>
  </div>
</template>
