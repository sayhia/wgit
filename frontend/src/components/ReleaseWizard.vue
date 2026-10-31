<script setup lang="ts">
import { backendText, t } from "../i18n";
import { ref, computed, watch, onMounted } from "vue";
import {
  Rocket,
  Tag,
  GitCommitHorizontal,
  ArrowUpCircle,
  ArrowDownCircle,
  RefreshCw,
  Copy,
  Check,
  AlertTriangle,
  CheckCircle2,
  AlertCircle,
  ClipboardCheck,
  Send,
  FileText,
  ShieldCheck,
  Sparkles,
  History,
  Archive,
  FolderDown,
} from "lucide-vue-next";
import ReleaseChangelogModal from "./tools/ReleaseChangelogModal.vue";
import { gitService } from "../services/gitService";
import type {
  ReleasePlan,
  ReleaseCreateResult,
  StudioChangelogResult,
  StudioChangelogCategory,
  ReleaseArtifactsResult,
  ChecksumVerifyResult,
} from "../types";

const props = defineProps<{
  repoPath: string;
}>();

const emit = defineEmits<{
  (e: "refresh"): void;
}>();

// ---------------- release plan ----------------
const plan = ref<ReleasePlan | null>(null);
const isLoading = ref(false);
const loadError = ref("");

// ---------------- tag form ----------------
const versionInput = ref("");
const messageInput = ref("");
const pushAfterCreate = ref(true);
const remoteInput = ref("");
const signTag = ref(false);
const isCreating = ref(false);
const createError = ref("");
const createResult = ref<ReleaseCreateResult | null>(null);

// ---------------- studio changelog ----------------
const changelog = ref<StudioChangelogResult | null>(null);
const isLoadingChangelog = ref(false);
const changelogError = ref("");
const fromRefInput = ref("");
const toRefInput = ref("HEAD");
const copiedDoc = ref(false);
const showChangelogModal = ref(false);
let copiedTimer: any = null;

// ---------------- release artifacts studio ----------------
const isGeneratingArtifacts = ref(false);
const artifactsResult = ref<ReleaseArtifactsResult | null>(null);

async function generateReleaseArtifacts() {
  const targetTag = versionInput.value.trim() || plan.value?.lastTag || "HEAD";
  if (!props.repoPath || !targetTag) return;
  isGeneratingArtifacts.value = true;
  try {
    const res = await gitService.createReleaseArtifacts(props.repoPath, targetTag, true);
    artifactsResult.value = res;
    notify(t("release.artifacts.success", { count: res.archiveZip ? 2 : 1 }));
  } catch (err: any) {
    notify(t("release.artifacts.failed", { error: String(err?.message || err) }));
  } finally {
    isGeneratingArtifacts.value = false;
  }
}

async function copyChecksum(sha: string) {
  try {
    await navigator.clipboard.writeText(sha);
    notify(t("release.artifacts.copied_hash"));
  } catch {
    // ignore
  }
}

// Artifact Checksum Self-Check State
const verifyPathInput = ref("");
const verifyHashInput = ref("");
const isVerifyingChecksum = ref(false);
const checksumResult = ref<ChecksumVerifyResult | null>(null);

async function handleVerifyChecksum() {
  if (!verifyPathInput.value.trim() || isVerifyingChecksum.value) return;
  isVerifyingChecksum.value = true;
  try {
    const res = await gitService.verifyArtifactChecksum(verifyPathInput.value.trim(), verifyHashInput.value.trim());
    checksumResult.value = res;
    notify(res.message);
  } catch (err: any) {
    notify(String(err?.message || err));
  } finally {
    isVerifyingChecksum.value = false;
  }
}

// ---------------- toast ----------------
const toastMessage = ref("");
const showToast = ref(false);
let toastTimer: any = null;

function notify(msg: string) {
  toastMessage.value = msg;
  showToast.value = true;
  if (toastTimer) clearTimeout(toastTimer);
  toastTimer = setTimeout(() => {
    showToast.value = false;
  }, 2600);
}

const checks = computed(() => plan.value?.checks ?? []);
const passCount = computed(() => checks.value.filter((c) => c.status === "pass").length);
const warnCount = computed(() => checks.value.filter((c) => c.status === "warn").length);
const failCount = computed(() => checks.value.filter((c) => c.status === "fail").length);
const blockingChecks = computed(() => checks.value.filter((c) => c.blocking && c.status === "fail"));

/**
 * Renders a backend-supplied pre-flight message.
 *
 * The backend sends both rendered text and a catalog key; prefer the key so the
 * sentence follows the UI language, and fall back to the text when the key is
 * unknown. An empty `base` means "no previous tag" — localize that placeholder
 * instead of showing a dangling "since ".
 */
function planText(
  key: string | undefined,
  fallback: string,
  params?: Record<string, string>
): string {
  const resolved = params ? { ...params } : undefined;
  if (resolved && "base" in resolved && !resolved.base) {
    resolved.base = t("release.base.initial");
  }
  return backendText(key, fallback, resolved);
}

const summaryText = computed(() =>
  planText(plan.value?.summaryKey, plan.value?.summary ?? "", plan.value?.summaryParams)
);

const versionLooksValid = computed(() => /^[A-Za-z0-9][A-Za-z0-9._+-]*$/.test(versionInput.value.trim()));

const canCreate = computed(
  () =>
    !!plan.value &&
    !!versionInput.value.trim() &&
    versionLooksValid.value &&
    blockingChecks.value.length === 0 &&
    !isCreating.value
);

const createDisabledReason = computed(() => {
  if (!plan.value) return t("release.error.no.plan");
  if (!versionInput.value.trim()) return t("release.error.version.required");
  if (!versionLooksValid.value) return t("release.error.version.invalid");
  if (blockingChecks.value.length > 0) {
    return t("release.error.blocking", { count: blockingChecks.value.length });
  }
  return "";
});

const releaseSucceeded = computed(
  () => !!createResult.value?.success && (!pushAfterCreate.value || !!createResult.value?.pushed)
);

const changelogCategories = computed(() => changelog.value?.categories ?? []);

function statusLabel(status: string): string {
  if (status === "pass") return t("release.status.pass");
  if (status === "warn") return t("release.status.warn");
  return t("release.status.fail");
}

function statusClass(status: string): string {
  if (status === "pass") return "bg-success-bg text-success border-success/30";
  if (status === "warn") return "bg-warning-bg text-warning border-warning/30";
  return "bg-danger-bg text-danger border-danger/30";
}

/** Short Chinese category name without the English suffix, e.g. "游戏玩法与系统逻辑". */
function shortCategory(cat: StudioChangelogCategory): string {
  const loose = cat as StudioChangelogCategory & { name?: string };
  const label = loose.category || loose.name || t("release.category.other");
  return label.split(" (")[0] ?? label;
}

/** Real backend uses commits[]; tolerate dev-preview mocks that send count. */
function categoryCount(cat: StudioChangelogCategory): number {
  const loose = cat as StudioChangelogCategory & { count?: number };
  if (Array.isArray(loose.commits)) return loose.commits.length;
  return typeof loose.count === "number" ? loose.count : 0;
}

function categoryIcon(cat: StudioChangelogCategory): string {
  return cat.icon || "•";
}

function resetChangelogInputs() {
  fromRefInput.value = "";
  toRefInput.value = "HEAD";
  changelog.value = null;
  changelogError.value = "";
}

function applySuggestedVersion(ver: string) {
  versionInput.value = ver;
}

async function generateChangelog() {
  if (!props.repoPath) return;
  isLoadingChangelog.value = true;
  changelogError.value = "";
  try {
    const res = await gitService.generateStudioChangelog(
      props.repoPath,
      fromRefInput.value.trim(),
      toRefInput.value.trim() || "HEAD"
    );
    changelog.value = res;
    if (res) {
      // Reflect the range actually used (backend auto-detects an empty fromRef).
      if (res.fromRef) fromRefInput.value = res.fromRef;
      if (res.toRef) toRefInput.value = res.toRef;
    }
  } catch (err: any) {
    changelog.value = null;
    changelogError.value = t("release.error.changelog", { error: err?.message || err });
  } finally {
    isLoadingChangelog.value = false;
  }
}

async function loadPlan(preserveResult = true) {
  if (!props.repoPath) {
    plan.value = null;
    return;
  }
  isLoading.value = true;
  loadError.value = "";
  if (!preserveResult) createResult.value = null;
  createError.value = "";
  try {
    const res = await gitService.getReleasePlan(props.repoPath);
    plan.value = res ?? null;
    if (res) {
      versionInput.value = res.suggestedVersion || "";
      messageInput.value = res.suggestedVersion ? `Release ${res.suggestedVersion}` : "";
      fromRefInput.value = res.lastTag || "";
      toRefInput.value = "HEAD";
      await generateChangelog();
    } else {
      resetChangelogInputs();
    }
  } catch (err: any) {
    plan.value = null;
    loadError.value = t("release.error.plan", { error: err?.message || err });
    resetChangelogInputs();
  } finally {
    isLoading.value = false;
  }
}

async function handleCreateRelease() {
  if (!props.repoPath || !canCreate.value) return;
  isCreating.value = true;
  createError.value = "";
  createResult.value = null;
  try {
    const res = await gitService.createRelease(
      props.repoPath,
      versionInput.value.trim(),
      messageInput.value.trim(),
      pushAfterCreate.value,
      remoteInput.value.trim(),
      signTag.value
    );
    createResult.value = res;
    if (res?.success) {
      notify(res.pushed ? t("release.toast.tag.pushed", { tag: res.tag }) : t("release.toast.tag.created", { tag: res.tag }));
      // Emitting refresh lets the parent reload the plan (and the rest of the app)
      // without clearing the result banner shown below.
      emit("refresh");
    } else if (res) {
      createError.value = res.message || t("release.error.create.short");
    } else {
      createError.value = t("release.error.create.no.result");
    }
  } catch (err: any) {
    createError.value = t("release.error.create", { error: err?.message || err });
  } finally {
    isCreating.value = false;
  }
}

async function copyChangelog() {
  const doc = changelog.value?.markdownDoc ?? "";
  if (!doc) return;
  try {
    await navigator.clipboard.writeText(doc);
    notify(t("release.toast.changelog.copied"));
  } catch {
    notify(t("release.toast.copy.failed"));
  }
  copiedDoc.value = true;
  if (copiedTimer) clearTimeout(copiedTimer);
  copiedTimer = setTimeout(() => {
    copiedDoc.value = false;
  }, 1600);
}

// Keep the default tag message in sync while the user has not customized it.
watch(versionInput, (value, previous) => {
  const autoPrevious = previous ? `Release ${previous}` : "";
  if (!messageInput.value.trim() || messageInput.value === autoPrevious) {
    messageInput.value = value ? `Release ${value}` : "";
  }
});

watch(
  () => props.repoPath,
  () => {
    plan.value = null;
    createResult.value = null;
    createError.value = "";
    loadError.value = "";
    resetChangelogInputs();
    void loadPlan();
  }
);

onMounted(() => {
  void loadPlan();
});

defineExpose({ loadPlan });
</script>

<template>
  <div class="space-y-5 relative">
    <!-- Header -->
    <div class="flex items-start justify-between gap-3 pb-3 border-b border-line">
      <div>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <Rocket class="w-4 h-4 text-accent" />
          <span>{{ t("release.title") }}</span>
          <span class="text-xs text-ink-muted font-normal">{{ t("release.subtitle") }}</span>
        </h3>
        <p class="text-xs text-ink-muted mt-1">
          {{ t("release.desc") }}
        </p>
      </div>

      <button
        @click="loadPlan(false)"
        :disabled="isLoading || !repoPath"
        class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-medium text-ink transition cursor-pointer shrink-0 disabled:opacity-50 disabled:cursor-not-allowed"
      >
        <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isLoading }" />
        <span>{{ t("release.refresh") }}</span>
      </button>
    </div>

    <!-- No repo -->
    <div
      v-if="!repoPath"
      class="py-16 text-center text-xs text-ink-muted border border-dashed border-line rounded-2xl"
    >
      {{ t("release.no.repo") }}
    </div>

    <!-- Loading (first load) -->
    <div
      v-else-if="isLoading && !plan"
      class="py-16 flex flex-col items-center gap-3 text-xs text-ink-muted"
    >
      <RefreshCw class="w-5 h-5 animate-spin text-accent" />
      <span>{{ t("release.loading") }}</span>
    </div>

    <!-- Load error -->
    <div
      v-else-if="loadError"
      class="p-3 rounded-xl bg-danger-bg border border-danger/30 text-xs text-danger flex items-start gap-2"
    >
      <AlertTriangle class="w-4 h-4 shrink-0 mt-0.5" />
      <span class="flex-1 leading-relaxed">{{ loadError }}</span>
      <button
        @click="loadPlan(false)"
        class="shrink-0 px-2.5 py-1 rounded-md bg-surface hover:bg-raised border border-line text-[11px] font-semibold text-ink transition cursor-pointer"
      >
        {{ t("release.retry") }}
      </button>
    </div>

    <!-- Empty plan (empty repo / no history) -->
    <div
      v-else-if="!plan"
      class="py-16 text-center text-xs text-ink-muted border border-dashed border-line rounded-2xl flex flex-col items-center gap-2"
    >
      <History class="w-6 h-6 text-ink-muted" />
      <span>{{ t("release.no.history") }}</span>
    </div>

    <!-- ============================= -->
    <!-- MAIN: checks (left) + form / changelog (right) -->
    <!-- ============================= -->
    <div v-else class="grid grid-cols-1 lg:grid-cols-5 gap-5">
      <!-- LEFT: plan overview + pre-flight checks -->
      <div class="lg:col-span-2 space-y-4">
        <!-- Overview -->
        <div class="p-4 rounded-xl border border-line bg-surface flex flex-col gap-3">
          <div class="flex items-center justify-between gap-2">
            <span class="text-xs font-bold text-ink flex items-center gap-1.5">
              <GitCommitHorizontal class="w-3.5 h-3.5 text-accent" />
              <span>{{ t("release.overview") }}</span>
            </span>
            <span
              class="px-2 py-0.5 rounded-full border text-[10px] font-bold"
              :class="plan.ready
                ? 'bg-success-bg text-success border-success/30'
                : 'bg-danger-bg text-danger border-danger/30'"
            >
              {{ plan.ready ? t("release.ready") : t("release.blocked") }}
            </span>
          </div>

          <div class="grid grid-cols-2 gap-2 text-xs">
            <div class="p-2.5 rounded-lg bg-surface border border-line flex flex-col gap-0.5">
              <span class="text-[10px] text-ink-muted">{{ t("release.current.branch") }}</span>
              <span class="font-mono font-bold text-ink truncate" :title="plan.branch">{{ plan.branch }}</span>
            </div>
            <div class="p-2.5 rounded-lg bg-surface border border-line flex flex-col gap-0.5">
              <span class="text-[10px] text-ink-muted">{{ t("release.last.tag") }}</span>
              <span class="font-mono font-bold text-ink truncate">{{ plan.lastTag || t("release.first.release") }}</span>
            </div>
            <div class="p-2.5 rounded-lg bg-surface border border-line flex flex-col gap-0.5">
              <span class="text-[10px] text-ink-muted">{{ t("release.pending.commits") }}</span>
              <span class="font-mono font-bold text-accent">{{ t("release.commits.count", { count: plan.commitCount }) }}</span>
            </div>
            <div class="p-2.5 rounded-lg bg-surface border border-line flex flex-col gap-0.5">
              <span class="text-[10px] text-ink-muted">{{ t("release.suggested.version") }}</span>
              <span class="font-mono font-bold text-success">
                {{ plan.suggestedVersion }}
                <span class="text-[10px] font-sans font-medium text-ink-muted">({{ plan.nextMinor ? "Minor" : "Patch" }})</span>
              </span>
            </div>
          </div>

          <div class="flex items-center gap-2 flex-wrap text-[11px]">
            <span
              v-if="plan.aheadOfRemote > 0"
              class="flex items-center gap-1 px-2 py-0.5 rounded-full bg-accent-bg border border-accent/30 text-accent font-mono font-bold"
              :title="t('release.ahead.title')"
            >
              <ArrowUpCircle class="w-3 h-3" /> ↑{{ plan.aheadOfRemote }}
            </span>
            <span
              v-if="plan.behindRemote > 0"
              class="flex items-center gap-1 px-2 py-0.5 rounded-full bg-cyan-bg border border-cyan/30 text-cyan font-mono font-bold"
              :title="t('release.behind.title')"
            >
              <ArrowDownCircle class="w-3 h-3" /> ↓{{ plan.behindRemote }}
            </span>
            <span
              v-if="!plan.aheadOfRemote && !plan.behindRemote"
              class="flex items-center gap-1 px-2 py-0.5 rounded-full bg-surface border border-line text-ink-muted"
            >
              <CheckCircle2 class="w-3 h-3 text-success" /> {{ t("release.in.sync") }}
            </span>
            <span class="px-2 py-0.5 rounded-full bg-surface border border-line text-ink-muted">
              {{ t("release.counts", { fail: failCount, warn: warnCount }) }}
            </span>
          </div>

          <p class="text-[11px] text-ink-muted leading-relaxed">{{ summaryText }}</p>
        </div>

        <!-- Pre-flight checks -->
        <div class="rounded-xl border border-line bg-surface overflow-hidden">
          <div class="px-4 py-2.5 border-b border-line flex items-center justify-between gap-2">
            <span class="text-xs font-bold text-ink flex items-center gap-1.5">
              <ClipboardCheck class="w-3.5 h-3.5 text-accent" />
              <span>{{ t("release.checks.title") }}</span>
            </span>
            <span class="text-[10px] text-ink-muted font-mono">
              {{ t("release.checks.summary", { pass: passCount, warn: warnCount, fail: failCount }) }}
            </span>
          </div>

          <div class="divide-y divide-line/40">
            <div v-for="check in checks" :key="check.key" class="p-3 flex items-start gap-2.5">
              <CheckCircle2 v-if="check.status === 'pass'" class="w-4 h-4 text-success shrink-0 mt-0.5" />
              <AlertTriangle v-else-if="check.status === 'warn'" class="w-4 h-4 text-warning shrink-0 mt-0.5" />
              <AlertCircle v-else class="w-4 h-4 text-danger shrink-0 mt-0.5" />

              <div class="flex flex-col gap-0.5 min-w-0">
                <div class="flex items-center gap-1.5 flex-wrap">
                  <span class="text-xs font-semibold text-ink">{{
                    planText(check.titleKey, check.title)
                  }}</span>
                  <span class="px-1.5 py-0.2 rounded text-[9px] font-bold border" :class="statusClass(check.status)">
                    {{ statusLabel(check.status) }}
                  </span>
                  <span
                    v-if="check.blocking"
                    class="px-1.5 py-0.2 rounded text-[9px] font-bold bg-danger-bg text-danger border border-danger/30"
                    :title="t('release.blocking.title')"
                  >
                    {{ t("release.blocking") }}
                  </span>
                </div>
                <span class="text-[11px] text-ink-muted leading-relaxed">{{
                  planText(check.detailKey, check.detail, check.params)
                }}</span>
              </div>
            </div>

            <div v-if="checks.length === 0" class="py-8 text-center text-xs text-ink-muted">
              {{ t("release.checks.empty") }}
            </div>
          </div>
        </div>
      </div>

      <!-- RIGHT: version form + changelog -->
      <div class="lg:col-span-3 space-y-4">
        <!-- Version & tag form -->
        <div class="p-4 rounded-xl border border-line bg-surface space-y-3">
          <div class="flex items-center justify-between gap-2 pb-2 border-b border-line">
            <span class="text-sm font-bold text-ink flex items-center gap-1.5">
              <Tag class="w-4 h-4 text-accent" />
              <span>{{ t("release.version.title") }}</span>
            </span>
            <span class="text-[10px] text-ink-muted font-mono">git tag -a &lt;version&gt; -m &lt;message&gt;</span>
          </div>

          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div class="flex flex-col gap-1">
              <div class="flex items-center justify-between">
                <label class="text-xs text-ink-secondary font-medium">{{ t("release.version.label") }}</label>
                <!-- SemVer Quick Preset Chips -->
                <div v-if="plan" class="flex items-center gap-1">
                  <button
                    v-if="plan.patchVersion"
                    type="button"
                    @click="applySuggestedVersion(plan.patchVersion)"
                    class="px-1.5 py-0.5 rounded text-[10px] font-mono transition cursor-pointer border border-line"
                    :class="versionInput === plan.patchVersion ? 'bg-accent-bg text-accent border-accent/40 font-bold' : 'bg-surface hover:bg-raised text-ink-secondary'"
                    :title="t('release.semver.patch.hint', { ver: plan.patchVersion })"
                  >
                    Patch {{ plan.patchVersion }}
                  </button>
                  <button
                    v-if="plan.minorVersion"
                    type="button"
                    @click="applySuggestedVersion(plan.minorVersion)"
                    class="px-1.5 py-0.5 rounded text-[10px] font-mono transition cursor-pointer border border-line"
                    :class="versionInput === plan.minorVersion ? 'bg-accent-bg text-accent border-accent/40 font-bold' : 'bg-surface hover:bg-raised text-ink-secondary'"
                    :title="t('release.semver.minor.hint', { ver: plan.minorVersion })"
                  >
                    Minor {{ plan.minorVersion }}
                  </button>
                  <button
                    v-if="plan.majorVersion"
                    type="button"
                    @click="applySuggestedVersion(plan.majorVersion)"
                    class="px-1.5 py-0.5 rounded text-[10px] font-mono transition cursor-pointer border border-line"
                    :class="versionInput === plan.majorVersion ? 'bg-accent-bg text-accent border-accent/40 font-bold' : 'bg-surface hover:bg-raised text-ink-secondary'"
                    :title="t('release.semver.major.hint', { ver: plan.majorVersion })"
                  >
                    Major {{ plan.majorVersion }}
                  </button>
                </div>
              </div>
              <input
                v-model="versionInput"
                type="text"
                :aria-label="t('release.version.label')"
                placeholder="v2.5.0"
                class="px-3 py-2 rounded-lg bg-canvas border text-xs text-ink placeholder-ink-muted focus:outline-none font-mono"
                :class="versionLooksValid ? 'border-line focus:border-accent' : 'border-danger/60 focus:border-danger'"
              />
              <span class="text-[10px]" :class="versionLooksValid ? 'text-ink-muted' : 'text-danger'">
                {{ t("release.version.suggested", { version: plan.suggestedVersion, hint: plan.nextMinor ? t("release.version.hint.minor") : t("release.version.hint.patch") }) }}
              </span>
            </div>

            <div class="flex flex-col gap-1">
              <label class="text-xs text-ink-secondary font-medium">{{ t("release.remote.label") }}</label>
              <input
                v-model="remoteInput"
                type="text"
                :aria-label="t('release.remote.placeholder')"
                :placeholder="t('release.remote.placeholder')"
                :disabled="!pushAfterCreate"
                class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent font-mono disabled:opacity-50"
              />
              <span class="text-[10px] text-ink-muted">{{ t("release.remote.hint") }}</span>
            </div>
          </div>

          <div class="flex flex-col gap-1">
            <label class="text-xs text-ink-secondary font-medium">{{ t("release.message.label") }}</label>
            <textarea
              v-model="messageInput"
              rows="2"
              placeholder="Release v2.5.0"
              class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent font-sans resize-y"
            />
          </div>

          <div class="flex items-center gap-4 flex-wrap">
            <label class="flex items-center gap-1.5 text-xs text-ink cursor-pointer select-none">
              <input v-model="pushAfterCreate" type="checkbox" class="rounded accent-accent" />
              <span class="flex items-center gap-1"><Send class="w-3 h-3 text-accent" />{{ t("release.push.after") }}</span>
            </label>
            <label class="flex items-center gap-1.5 text-xs text-ink cursor-pointer select-none">
              <input v-model="signTag" type="checkbox" class="rounded accent-accent" />
              <span class="flex items-center gap-1"><ShieldCheck class="w-3 h-3 text-purple" />{{ t("release.sign.label") }}</span>
            </label>
          </div>

          <div
            v-if="signTag"
            class="p-2.5 rounded-lg bg-purple-bg border border-purple/25 text-[11px] text-purple flex items-start gap-2"
          >
            <ShieldCheck class="w-3.5 h-3.5 shrink-0 mt-0.5" />
            <span>{{ t("release.sign.hint") }}</span>
          </div>

          <!-- Create error -->
          <div
            v-if="createError"
            class="p-3 rounded-xl bg-danger-bg border border-danger/30 text-xs text-danger flex items-start gap-2"
          >
            <AlertTriangle class="w-4 h-4 shrink-0 mt-0.5" />
            <span class="flex-1 leading-relaxed">{{ createError }}</span>
          </div>

          <!-- Create result banner -->
          <div
            v-if="createResult"
            class="p-3 rounded-xl border flex flex-col gap-2"
            :class="releaseSucceeded ? 'bg-success-bg border-success/30' : 'bg-warning-bg border-warning/30'"
          >
            <div
              class="flex items-center gap-2 text-xs font-bold"
              :class="releaseSucceeded ? 'text-success' : 'text-warning'"
            >
              <CheckCircle2 v-if="releaseSucceeded" class="w-4 h-4 shrink-0" />
              <AlertTriangle v-else class="w-4 h-4 shrink-0" />
              <span>{{ createResult.message || t("release.created") }}</span>
            </div>
            <div class="flex items-center gap-2 flex-wrap text-[11px]">
              <span class="px-2 py-0.5 rounded bg-canvas border border-line font-mono text-ink">
                tag: {{ createResult.tag }}
              </span>
              <span
                class="px-2 py-0.5 rounded border font-medium"
                :class="createResult.pushed
                  ? 'bg-success-bg text-success border-success/30'
                  : 'bg-surface text-ink-muted border-line'"
              >
                {{ createResult.pushed ? t("release.pushed") : t("release.local.only") }}
              </span>
            </div>
            <pre
              v-if="createResult.pushOutput"
              class="max-h-32 overflow-auto rounded-lg bg-canvas border border-line p-2 font-mono text-[10px] leading-4 text-ink-secondary whitespace-pre-wrap"
            >{{ createResult.pushOutput }}</pre>
          </div>

          <!-- Create button -->
          <button
            @click="handleCreateRelease"
            :disabled="!canCreate"
            class="w-full flex items-center justify-center gap-2 py-2.5 rounded-xl bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-xs font-semibold transition cursor-pointer shadow-sm disabled:opacity-50 disabled:cursor-not-allowed"
          >
            <RefreshCw v-if="isCreating" class="w-4 h-4 animate-spin" />
            <Rocket v-else class="w-4 h-4" />
            <span>{{ isCreating ? t("release.creating") : t("release.create.button", { version: versionInput.trim() }) }}</span>
          </button>

          <p
            v-if="createDisabledReason"
            class="text-[11px] flex items-center gap-1"
            :class="blockingChecks.length > 0 || !versionLooksValid ? 'text-danger' : 'text-ink-muted'"
          >
            <AlertTriangle class="w-3 h-3 shrink-0" />
            <span>{{ createDisabledReason }}</span>
          </p>
        </div>

        <!-- Studio changelog -->
        <div class="p-4 rounded-xl border border-line bg-surface space-y-3">
          <div class="flex items-center justify-between gap-2 pb-2 border-b border-line flex-wrap">
            <span class="text-sm font-bold text-ink flex items-center gap-1.5">
              <FileText class="w-4 h-4 text-purple" />
              <span>{{ t("release.changelog.title") }}</span>
              <span class="text-[11px] text-ink-muted font-normal">(Studio Changelog)</span>
            </span>
            <div class="flex items-center gap-2">
              <span
                v-if="changelog"
                class="px-1.5 py-0.5 rounded-full bg-purple-bg text-purple border border-purple/30 text-[10px] font-bold"
              >
                {{ t("release.changelog.commits", { count: changelog.totalCount }) }}
              </span>
              <button
                @click="copyChangelog"
                :disabled="!changelog?.markdownDoc"
                class="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-[11px] font-medium text-ink transition cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed"
              >
                <Check v-if="copiedDoc" class="w-3 h-3 text-success" />
                <Copy v-else class="w-3 h-3" />
                <span>{{ copiedDoc ? t("release.copied") : t("release.copy.markdown") }}</span>
              </button>
              <button
                type="button"
                @click="showChangelogModal = true"
                class="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-accent-bg hover:bg-accent-bg-strong border border-accent/30 text-[11px] font-semibold text-accent transition cursor-pointer"
              >
                <Sparkles class="w-3 h-3 text-accent" />
                <span>{{ t("release.changelog.modal.btn") }}</span>
              </button>
            </div>
          </div>

          <!-- Ref range controls -->
          <div class="flex items-end gap-2 flex-wrap">
            <div class="flex flex-col gap-1 flex-1 min-w-[150px]">
              <label class="text-[10px] text-ink-muted">{{ t("release.changelog.from") }}</label>
              <input
                v-model="fromRefInput"
                type="text"
                :aria-label="t('release.changelog.from.placeholder')"
                :placeholder="t('release.changelog.from.placeholder')"
                class="px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-[11px] text-ink placeholder-ink-muted focus:outline-none focus:border-purple font-mono"
              />
            </div>
            <div class="flex flex-col gap-1 flex-1 min-w-[130px]">
              <label class="text-[10px] text-ink-muted">{{ t("release.changelog.to") }}</label>
              <input
                v-model="toRefInput"
                type="text"
                :aria-label="t('release.changelog.to')"
                placeholder="HEAD"
                class="px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-[11px] text-ink placeholder-ink-muted focus:outline-none focus:border-purple font-mono"
              />
            </div>
            <button
              @click="generateChangelog"
              :disabled="isLoadingChangelog || !repoPath"
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-purple-strong hover:bg-purple-dark text-white text-[11px] font-semibold transition cursor-pointer shadow-xs disabled:opacity-50 disabled:cursor-not-allowed"
            >
              <RefreshCw class="w-3 h-3" :class="{ 'animate-spin': isLoadingChangelog }" />
              <span>{{ isLoadingChangelog ? t("release.changelog.loading") : t("release.changelog.regenerate") }}</span>
            </button>
          </div>

          <!-- Category summary chips -->
          <div v-if="changelogCategories.length > 0" class="flex items-center gap-1.5 flex-wrap">
            <span
              v-for="(cat, idx) in changelogCategories"
              :key="`${shortCategory(cat)}-${idx}`"
              class="px-2 py-0.5 rounded-full border border-line bg-surface text-[10px] text-ink-secondary"
            >
              <span class="mr-0.5">{{ categoryIcon(cat) }}</span>{{ shortCategory(cat) }} · {{ categoryCount(cat) }}
            </span>
          </div>

          <!-- Markdown preview -->
          <div
            v-if="isLoadingChangelog"
            class="py-12 flex flex-col items-center gap-2 text-xs text-ink-muted"
          >
            <RefreshCw class="w-4 h-4 animate-spin text-purple" />
            <span>{{ t("release.changelog.generating") }}</span>
          </div>

          <div
            v-else-if="changelogError"
            class="p-3 rounded-lg bg-danger-bg border border-danger/30 text-xs text-danger flex items-start gap-2"
          >
            <AlertTriangle class="w-4 h-4 shrink-0 mt-0.5" />
            <span class="flex-1 leading-relaxed">{{ changelogError }}</span>
          </div>

          <pre
            v-else-if="changelog?.markdownDoc"
            class="max-h-96 overflow-auto rounded-lg bg-canvas border border-line p-3 font-mono text-[11px] leading-5 text-ink-secondary whitespace-pre-wrap"
          >{{ changelog.markdownDoc }}</pre>

          <div
            v-else
            class="py-10 text-center text-xs text-ink-muted border border-dashed border-line rounded-lg flex flex-col items-center gap-2"
          >
            <FileText class="w-5 h-5 text-ink-muted" />
            <span>{{ t("release.changelog.empty") }}</span>
          </div>
        </div>

        <!-- Release Artifacts Studio -->
        <div class="p-4 rounded-xl border border-line bg-surface space-y-3" data-testid="release-artifacts-panel">
          <div class="flex items-center justify-between gap-2 pb-2 border-b border-line flex-wrap">
            <span class="text-sm font-bold text-ink flex items-center gap-1.5">
              <Archive class="w-4 h-4 text-accent" />
              <span>{{ t("release.artifacts.title") }}</span>
              <span class="text-[11px] text-ink-muted font-normal">(Release Artifacts Studio)</span>
            </span>

            <button
              @click="generateReleaseArtifacts"
              :disabled="isGeneratingArtifacts || !repoPath"
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-accent text-accent-contrast text-xs font-semibold hover:opacity-90 transition cursor-pointer shadow-xs disabled:opacity-50"
            >
              <RefreshCw v-if="isGeneratingArtifacts" class="w-3.5 h-3.5 animate-spin" />
              <FolderDown v-else class="w-3.5 h-3.5" />
              <span>{{ isGeneratingArtifacts ? t("release.artifacts.generating") : t("release.artifacts.build") }}</span>
            </button>
          </div>

          <p class="text-xs text-ink-muted leading-relaxed">
            {{ t("release.artifacts.desc") }}
          </p>

          <!-- Artifacts Results -->
          <div v-if="artifactsResult" class="space-y-2">
            <div class="text-[11px] font-mono text-ink-muted flex items-center justify-between">
              <span>{{ t("release.artifacts.tag") }}: <span class="font-bold text-ink">{{ artifactsResult.tagName }}</span></span>
              <button
                v-if="artifactsResult.sha256Digest"
                type="button"
                @click="copyChecksum(artifactsResult.sha256Digest)"
                class="text-[10px] text-accent hover:underline flex items-center gap-1 cursor-pointer shrink-0"
              >
                <Copy class="w-3 h-3" />
                <span>{{ t("common.copy") }} SHA256</span>
              </button>
            </div>

            <div class="space-y-1.5">
              <div
                v-if="artifactsResult.archiveTarGz"
                class="p-2.5 rounded-lg bg-canvas border border-line space-y-1.5 text-xs font-mono"
              >
                <div class="flex items-center justify-between">
                  <div class="flex items-center gap-2">
                    <span class="px-1.5 py-0.5 rounded bg-raised text-[10px] uppercase font-bold text-purple">TAR.GZ</span>
                    <span class="font-semibold text-ink">{{ artifactsResult.archiveTarGz }}</span>
                  </div>
                </div>
              </div>

              <div
                v-if="artifactsResult.archiveZip"
                class="p-2.5 rounded-lg bg-canvas border border-line space-y-1.5 text-xs font-mono"
              >
                <div class="flex items-center justify-between">
                  <div class="flex items-center gap-2">
                    <span class="px-1.5 py-0.5 rounded bg-raised text-[10px] uppercase font-bold text-purple">ZIP</span>
                    <span class="font-semibold text-ink">{{ artifactsResult.archiveZip }}</span>
                  </div>
                </div>
              </div>
            </div>

            <div v-if="artifactsResult.checksumFile" class="text-[10px] text-ink-muted font-mono pt-1">
              {{ t("release.artifacts.checksum_file") }}: {{ artifactsResult.checksumFile }}
            </div>
          </div>

          <!-- Checksum Integrity Self-Check Section -->
          <div class="pt-3 border-t border-line/60 space-y-2">
            <span class="text-xs font-bold text-ink flex items-center gap-1.5">
              <ShieldCheck class="w-3.5 h-3.5 text-accent" />
              <span>{{ t("release.checksum.verify.title") }}</span>
            </span>

            <div class="grid grid-cols-1 md:grid-cols-2 gap-2">
              <input
                v-model="verifyPathInput"
                type="text"
                :aria-label="t('release.checksum.verify.path.placeholder')"
                :placeholder="t('release.checksum.verify.path.placeholder')"
                class="px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs font-mono text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
              />
              <input
                v-model="verifyHashInput"
                type="text"
                :aria-label="t('release.checksum.verify.hash.placeholder')"
                :placeholder="t('release.checksum.verify.hash.placeholder')"
                class="px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs font-mono text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
              />
            </div>

            <div class="flex items-center justify-between gap-2">
              <div v-if="checksumResult" class="text-xs font-mono flex items-center gap-1.5">
                <span
                  class="px-2 py-0.5 rounded font-semibold"
                  :class="checksumResult.matches ? 'bg-success-bg text-success' : 'bg-danger-bg text-danger'"
                >
                  {{ checksumResult.matches ? t("release.checksum.matches") : t("release.checksum.mismatch") }}
                </span>
                <span class="text-[10px] text-ink-muted truncate max-w-[280px]" :title="checksumResult.computedHash">
                  {{ checksumResult.computedHash }}
                </span>
              </div>
              <div v-else />

              <button
                @click="handleVerifyChecksum"
                :disabled="!verifyPathInput.trim() || isVerifyingChecksum"
                class="flex items-center gap-1.5 px-3 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-accent transition cursor-pointer disabled:opacity-50 ml-auto"
                data-testid="verify-checksum-btn"
              >
                <RefreshCw v-if="isVerifyingChecksum" class="w-3 h-3 animate-spin" />
                <ShieldCheck v-else class="w-3 h-3" />
                <span>{{ isVerifyingChecksum ? t("release.checksum.verifying") : t("release.checksum.verify.btn") }}</span>
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Toast -->
    <transition
      enter-active-class="transition duration-200 ease-out"
      enter-from-class="transform translate-y-4 opacity-0"
      enter-to-class="transform translate-y-0 opacity-100"
      leave-active-class="transition duration-150 ease-in"
      leave-from-class="transform translate-y-0 opacity-100"
      leave-to-class="transform translate-y-4 opacity-0"
    >
      <div
        v-if="showToast"
        class="fixed bottom-6 left-1/2 -translate-x-1/2 bg-ink text-canvas text-xs px-4 py-2 rounded-full shadow-2xl flex items-center gap-2 z-50 font-medium select-none pointer-events-none"
      >
        <Sparkles class="w-3.5 h-3.5 text-accent" />
        <span>{{ toastMessage }}</span>
      </div>
    </transition>
    <!-- Semantic Changelog Generator Modal -->
    <ReleaseChangelogModal
      v-model:open="showChangelogModal"
      :repo-path="repoPath"
      :initial-from-ref="fromRefInput"
      :initial-to-ref="toRefInput"
      @notify="notify"
    />
  </div>
</template>
