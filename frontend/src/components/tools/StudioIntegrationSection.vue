<script setup lang="ts">
import { confirmAction } from "../../composables/useConfirm";
import { ref, watch } from "vue";
import { Activity, CheckCircle2, ChevronDown, ChevronRight, Play, RefreshCw, Rocket } from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import type {
  TAPDWorkItemListResult,
  TAPDCommitReferenceResult,
  StudioCIResult,
  FullParityStatusSummary,
  PinnacleParityStatusSummary,
} from "../../types";
import { t } from "../../i18n";

const props = defineProps<{
  currentRepoPath: string;
}>();

const emit = defineEmits<{
  (e: "notify", msg: string): void;
}>();

function notify(msg: string) {
  emit("notify", msg);
}

const expanded = ref(false);
const tapd = ref<TAPDWorkItemListResult | null>(null);
const isLoadingTAPD = ref(false);
const bindCommitHash = ref("");
const bindTapdID = ref("");
const bindStatus = ref("resolved");
const isBinding = ref(false);
const parseMessage = ref("");
const parseResult = ref<TAPDCommitReferenceResult | null>(null);
const isParsing = ref(false);

const ci = ref<StudioCIResult | null>(null);
const isLoadingCI = ref(false);
const isTriggeringCI = ref(false);

const parityKind = ref<"" | "full" | "pinnacle">("");
const parity = ref<FullParityStatusSummary | PinnacleParityStatusSummary | null>(null);
const isLoadingParity = ref("");

async function loadTAPD() {
  if (!props.currentRepoPath || isLoadingTAPD.value) return;
  isLoadingTAPD.value = true;
  try {
    const [list, commits] = await Promise.all([
      gitService.fetchTAPDWorkItems(props.currentRepoPath),
      // 头提交仅用于预填绑定框：空仓/读取失败时留空让用户手填，属诚实降级。
      gitService.getCommits(props.currentRepoPath, 1).catch(() => []),
    ]);
    tapd.value = list;
    const head = commits[0];
    if (!bindCommitHash.value && head) {
      bindCommitHash.value = head.shortHash || head.hash;
    }
  } catch (err: any) {
    notify(t("studio.toast.tapd.load.failed", { error: err?.message || err }));
  } finally {
    isLoadingTAPD.value = false;
  }
}

async function loadCI() {
  if (!props.currentRepoPath || isLoadingCI.value) return;
  isLoadingCI.value = true;
  try {
    ci.value = await gitService.getStudioCIPipelines(props.currentRepoPath);
  } catch (err: any) {
    notify(t("studio.toast.ci.load.failed", { error: err?.message || err }));
  } finally {
    isLoadingCI.value = false;
  }
}

function ensureLoaded() {
  void loadTAPD();
  void loadCI();
}

function toggleExpanded() {
  expanded.value = !expanded.value;
  if (expanded.value && !tapd.value && !ci.value) ensureLoaded();
}

async function handleBind() {
  if (!props.currentRepoPath || isBinding.value) return;
  if (!bindCommitHash.value.trim() || !bindTapdID.value.trim()) return;
  if (
    !(await confirmAction(
      t("studio.confirm.bind", {
        commit: bindCommitHash.value.trim(),
        id: bindTapdID.value.trim(),
        status: bindStatus.value,
      })
    ))
  )
    return;
  isBinding.value = true;
  try {
    const res = await gitService.bindCommitToTAPD(
      props.currentRepoPath,
      bindCommitHash.value.trim(),
      bindTapdID.value.trim(),
      bindStatus.value
    );
    notify(res?.message || t("studio.toast.bind.done"));
    await loadTAPD();
  } catch (err: any) {
    notify(t("studio.toast.bind.failed", { error: err?.message || err }));
  } finally {
    isBinding.value = false;
  }
}

async function handleParse() {
  const msg = parseMessage.value.trim();
  if (!msg || isParsing.value) return;
  isParsing.value = true;
  try {
    parseResult.value = await gitService.parseTAPDCommitReferences(msg);
    const count = parseResult.value?.foundTickets.length ?? 0;
    notify(count > 0 ? t("studio.toast.parse.found", { count }) : t("studio.toast.parse.none"));
  } catch (err: any) {
    notify(t("studio.toast.parse.failed", { error: err?.message || err }));
  } finally {
    isParsing.value = false;
  }
}

async function handleTriggerCI() {
  if (!props.currentRepoPath || isTriggeringCI.value) return;
  if (!(await confirmAction(t("studio.confirm.trigger.ci")))) return;
  isTriggeringCI.value = true;
  try {
    const res = await gitService.triggerStudioCIPipeline(
      props.currentRepoPath,
      ci.value?.latestRun?.name || "Game-Client-Daily-Build"
    );
    notify(res?.message || t("studio.toast.ci.triggered"));
    await loadCI();
  } catch (err: any) {
    notify(t("studio.toast.ci.trigger.failed", { error: err?.message || err }));
  } finally {
    isTriggeringCI.value = false;
  }
}

async function handleLoadParity(kind: "full" | "pinnacle") {
  if (!props.currentRepoPath || isLoadingParity.value) return;
  isLoadingParity.value = kind;
  try {
    parity.value =
      kind === "full"
        ? await gitService.getFullParityStatusSummary(props.currentRepoPath)
        : await gitService.getPinnacleParityStatusSummary(props.currentRepoPath);
    parityKind.value = kind;
  } catch (err: any) {
    notify(t("studio.toast.parity.load.failed", { error: err?.message || err }));
  } finally {
    isLoadingParity.value = "";
  }
}

watch(
  () => props.currentRepoPath,
  () => {
    tapd.value = null;
    ci.value = null;
    parity.value = null;
    parityKind.value = "";
    if (expanded.value) ensureLoaded();
  }
);
</script>

<template>
  <div class="space-y-3 pt-4">
    <div class="flex items-center justify-between pb-2 border-b border-line">
      <div>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <Rocket class="w-4 h-4 text-purple" />
          <span>{{ t("studio.title") }}</span>
        </h3>
        <p class="text-xs text-ink-muted mt-0.5">
          {{ t("studio.intro") }}
        </p>
      </div>
      <button
        @click="toggleExpanded"
        class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer"
      >
        <component :is="expanded ? ChevronDown : ChevronRight" class="w-3.5 h-3.5" />
        <span>{{ expanded ? t("studio.collapse") : t("studio.expand") }}</span>
      </button>
    </div>

    <div v-if="expanded" class="space-y-4">
      <!-- TAPD -->
      <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-3">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <div class="flex items-center gap-2">
            <span class="text-xs font-bold text-ink">{{ t("studio.tapd.title") }}</span>
            <span v-if="tapd" class="px-1.5 py-0.2 rounded bg-surface border border-line text-[10px] font-mono text-ink-muted">
              {{ tapd.workspaceId }}
            </span>
          </div>
          <button
            @click="loadTAPD"
            :disabled="isLoadingTAPD"
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-50"
          >
            <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isLoadingTAPD }" aria-hidden="true" />
            <span>{{ isLoadingTAPD ? t("studio.loading") : t("studio.tapd.load") }}</span>
          </button>
        </div>

        <template v-if="tapd">
          <p v-if="tapd.items.length === 0" class="text-[11px] text-ink-muted">{{ tapd.message }}</p>
          <template v-else>
          <div class="grid grid-cols-4 gap-2">
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">{{ t("studio.tapd.total") }}</div>
              <div class="text-sm font-bold font-mono text-ink">{{ tapd.totalCount }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">{{ t("studio.tapd.stories") }}</div>
              <div class="text-sm font-bold font-mono text-accent">{{ tapd.storiesCount }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">{{ t("studio.tapd.tasks") }}</div>
              <div class="text-sm font-bold font-mono text-purple">{{ tapd.tasksCount }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">{{ t("studio.tapd.bugs") }}</div>
              <div class="text-sm font-bold font-mono text-danger">{{ tapd.bugsCount }}</div>
            </div>
          </div>

          <div class="rounded-lg border border-line bg-canvas/30 divide-y divide-line/40 max-h-64 overflow-y-auto">
            <div v-for="item in tapd.items" :key="item.id" class="p-2.5 space-y-1">
              <div class="flex items-center gap-2">
                <span
                  class="px-1 py-0.2 rounded text-[9px] font-bold shrink-0"
                  :class="item.type === 'bug' ? 'bg-danger/15 text-danger' : item.type === 'story' ? 'bg-accent/15 text-accent' : 'bg-purple/15 text-purple'"
                >
                  {{ item.type === "bug" ? t("studio.tapd.bug") : item.type === "story" ? t("studio.tapd.story") : t("studio.tapd.task") }}
                </span>
                <span class="text-xs text-ink truncate flex-1" :title="item.title">{{ item.title }}</span>
                <span class="text-[10px] text-ink-muted shrink-0">{{ item.statusLabel }}</span>
              </div>
              <div class="flex items-center gap-2 text-[10px] text-ink-muted font-mono">
                <span>#{{ item.id }}</span>
                <span>{{ item.owner }}</span>
                <span class="truncate">{{ item.iteration }}</span>
                <span v-if="item.boundCommits.length > 0" class="text-success">{{ item.boundCommits.join(" · ") }}</span>
                <button
                  @click="bindTapdID = item.id"
                  class="ml-auto px-2 py-0.5 rounded bg-surface hover:bg-raised border border-line text-[10px] text-accent transition cursor-pointer shrink-0"
                >
                  {{ t("studio.tapd.fill.id") }}
                </button>
              </div>
            </div>
          </div>

          </template>

          <div class="grid grid-cols-1 md:grid-cols-4 gap-2">
            <input
              v-model="bindCommitHash"
              type="text"
              :placeholder="t('studio.tapd.commit.placeholder')"
              class="px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
            />
            <input
              v-model="bindTapdID"
              type="text"
              :placeholder="t('studio.tapd.id.placeholder')"
              class="px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
            />
            <select
              v-model="bindStatus"
              class="px-2 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink focus:outline-none focus:border-accent cursor-pointer"
            >
              <option value="in_progress">{{ t("studio.tapd.status.in.progress") }}</option>
              <option value="resolved">{{ t("studio.tapd.status.resolved") }}</option>
              <option value="verified">{{ t("studio.tapd.status.verified") }}</option>
              <option value="closed">{{ t("studio.tapd.status.closed") }}</option>
            </select>
            <button
              @click="handleBind"
              :disabled="isBinding || !bindCommitHash.trim() || !bindTapdID.trim()"
              class="px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-xs font-semibold transition cursor-pointer disabled:opacity-40"
            >
              {{ isBinding ? t("studio.tapd.binding") : t("studio.tapd.bind") }}
            </button>
          </div>
          <p class="text-[10px] text-ink-muted">{{ t("studio.tapd.bind.hint") }}</p>
        </template>

        <div class="flex items-center gap-2">
          <input
            v-model="parseMessage"
            type="text"
            :placeholder="t('studio.tapd.parse.placeholder')"
            @keydown.enter="handleParse"
            class="flex-1 px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
          />
          <button
            @click="handleParse"
            :disabled="!parseMessage.trim() || isParsing"
            class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-accent transition cursor-pointer disabled:opacity-40 shrink-0"
          >
            {{ isParsing ? t("studio.tapd.parsing") : t("studio.tapd.parse") }}
          </button>
        </div>
        <div v-if="parseResult" class="flex flex-wrap items-center gap-1.5">
          <span v-if="parseResult.hasTapdTag" class="px-1.5 py-0.5 rounded bg-success/15 text-success text-[10px] font-mono">
            {{ parseResult.formattedTag }}
          </span>
          <span
            v-for="ticket in parseResult.foundTickets"
            :key="ticket.ticketId"
            class="px-1.5 py-0.5 rounded bg-surface border border-line text-[10px] font-mono text-ink-secondary"
          >
            #{{ ticket.ticketId }}
          </span>
          <span v-if="!parseResult.hasTapdTag" class="text-[11px] text-ink-muted">{{ t("studio.tapd.parse.none") }}</span>
        </div>
      </div>

      <!-- CI -->
      <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-3">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <div class="flex items-center gap-2">
            <Activity class="w-4 h-4 text-accent" />
            <span class="text-xs font-bold text-ink">{{ t("studio.ci.title") }}</span>
            <span v-if="ci" class="text-[10px] text-ink-muted font-mono">{{ ci.provider }} · {{ ci.activeBranch }}</span>
          </div>
          <div class="flex items-center gap-2">
            <button
              @click="loadCI"
              :disabled="isLoadingCI"
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-50"
            >
              <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isLoadingCI }" aria-hidden="true" />
              <span>{{ t("studio.ci.refresh") }}</span>
            </button>
            <button
              @click="handleTriggerCI"
              :disabled="isTriggeringCI || !ci?.latestRun"
              :title="ci?.latestRun ? t('studio.ci.trigger.hint') : t('studio.ci.trigger.disabled')"
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-xs font-semibold transition cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
            >
              <Play class="w-3.5 h-3.5" aria-hidden="true" />
              <span>{{ isTriggeringCI ? t("studio.ci.triggering") : t("studio.ci.trigger") }}</span>
            </button>
          </div>
        </div>

        <template v-if="ci?.latestRun">
          <div class="flex flex-wrap items-center gap-2 text-[11px]">
            <span
              class="px-1.5 py-0.5 rounded font-bold"
              :class="ci.latestRun.status === 'SUCCESS' ? 'bg-success/15 text-success' : ci.latestRun.status === 'FAILED' ? 'bg-danger/15 text-danger' : 'bg-warning/15 text-warning'"
            >
              {{ ci.latestRun.status }}
            </span>
            <span class="font-mono text-ink">{{ ci.latestRun.id }}</span>
            <span class="text-ink-muted font-mono">{{ ci.latestRun.shortHash }}</span>
            <span class="text-ink-muted">{{ ci.latestRun.author }}</span>
            <span class="text-ink-muted ml-auto">{{ ci.latestRun.duration }}</span>
          </div>
          <div class="space-y-1">
            <div
              v-for="stage in ci.latestRun.stages"
              :key="stage.name"
              class="flex items-center gap-2 px-2 py-1 rounded bg-canvas/40 border border-line text-[11px]"
            >
              <CheckCircle2 class="w-3 h-3 shrink-0" :class="stage.status === 'SUCCESS' ? 'text-success' : 'text-warning'" />
              <span class="text-ink truncate flex-1">{{ stage.name }}</span>
              <span class="text-ink-muted font-mono shrink-0">{{ stage.duration }}</span>
            </div>
          </div>
          <p class="text-[10px] text-ink-muted">{{ ci.message }}</p>
        </template>
        <p v-else-if="!isLoadingCI" class="text-[11px] text-ink-muted">{{ ci?.message || t("studio.ci.empty") }}</p>
      </div>

      <!-- Backend capability inventory -->
      <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-3">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <div>
            <div class="text-xs font-bold text-ink">{{ t("studio.parity.title") }}</div>
            <div class="text-[10px] text-ink-muted">{{ t("studio.parity.subtitle") }}</div>
          </div>
          <div class="flex items-center gap-2">
            <button
              @click="handleLoadParity('full')"
              :disabled="!!isLoadingParity"
              class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-50"
            >
              {{ isLoadingParity === "full" ? t("studio.loading") : t("studio.parity.full") }}
            </button>
            <button
              @click="handleLoadParity('pinnacle')"
              :disabled="!!isLoadingParity"
              class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-50"
            >
              {{ isLoadingParity === "pinnacle" ? t("studio.loading") : t("studio.parity.pinnacle") }}
            </button>
          </div>
        </div>

        <div v-if="parity" class="space-y-2">
          <div class="flex flex-wrap items-center gap-2 text-[11px]">
            <span class="px-1.5 py-0.5 rounded bg-accent/15 text-accent font-bold">
              {{ parity.completedRounds }}/{{ parity.totalRounds }} · {{ parity.parityPercentage }}%
            </span>
            <span class="text-ink-muted font-mono truncate">{{ parity.systemReadiness }}</span>
            <span class="text-ink-muted ml-auto font-mono">{{ parity.clientPlatform }}</span>
          </div>
          <div class="space-y-1">
            <div
              v-for="domain in parity.domains"
              :key="domain.domainId"
              class="flex items-center gap-2 px-2 py-1 rounded bg-canvas/40 border border-line text-[11px]"
            >
              <span class="text-ink truncate flex-1">{{ domain.domainName }}</span>
              <span class="text-ink-muted font-mono shrink-0">{{ domain.completedCount }}/{{ domain.totalModules }}</span>
              <span class="text-success font-mono shrink-0">{{ domain.completionRate }}%</span>
            </div>
          </div>
          <p class="text-[10px] text-ink-muted">{{
            t("studio.parity.footnote", { kind: parityKind === "pinnacle" ? "Pinnacle" : "Full" })
          }}</p>
        </div>
      </div>
    </div>
  </div>
</template>
