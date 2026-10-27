<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import {
  ExternalLink,
  GitPullRequest,
  Inbox,
  KeyRound,
  Loader2,
  RefreshCw,
  Search,
} from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import {
  createReviewSource,
  type RemoteReviewSource,
  type ReviewVerdict,
} from "../../services/reviewSource";
import type { PrAnchor, PrDetail, PrProviderInfo, PrSummary } from "../../types";
import UiButton from "../ui/UiButton.vue";
import Toast from "../ui/Toast.vue";
import { useToast } from "../../composables/useToast";
import { confirmAction } from "../../composables/useConfirm";
import { useRovingList } from "../../composables/useRovingList";
import PrDetailPanel from "./PrDetailPanel.vue";
import PrTokenModal from "./PrTokenModal.vue";
import { checksDot, checksLabel, formatRelativeTime, prStateChip, prStateLabel } from "./meta";
import { t } from "../../i18n";

/**
 * In-app PR / MR review (C2).
 *
 * The backend owns every network call and the token; this view is a thin
 * shell around `ListPullRequests` / `GetPullRequestDetail` plus the four
 * write actions (comment, approve, request changes, merge).
 */
const props = defineProps<{ currentRepoPath: string }>();

const emit = defineEmits<{
  (e: "switch-mode", mode: "local" | "remote"): void;
}>();

type StateFilter = "open" | "closed" | "merged" | "all";

const { toastText, showToast, notify } = useToast(3600);

/**
 * Every PR call goes through the ReviewSource adapter, so this view only knows
 * the unified contract. A Gitee / Bitbucket backend needs a new adapter, not a
 * new view.
 */
let reviewSource: RemoteReviewSource | null = null;
let reviewSourceRepo = "";
function source(): RemoteReviewSource {
  if (!reviewSource || reviewSourceRepo !== props.currentRepoPath) {
    reviewSource = createReviewSource("remote", props.currentRepoPath);
    reviewSourceRepo = props.currentRepoPath;
  }
  return reviewSource;
}

/** UI review buttons use provider events; the adapter speaks verdicts. */
function verdictOfEvent(event: string): ReviewVerdict {
  if (event === "approve") return "approved";
  if (event === "request_changes") return "changes";
  return "pending";
}

const provider = ref<PrProviderInfo | null>(null);
const items = ref<PrSummary[]>([]);
const listMessage = ref("");
const stateFilter = ref<StateFilter>("open");
const search = ref("");
const loadingProvider = ref(false);
const loadingList = ref(false);
const loadingDetail = ref(false);
const detail = ref<PrDetail | null>(null);
const selectedNumber = ref<number | null>(null);
const anchor = ref<PrAnchor | null>(null);
const commentBusy = ref(false);
const reviewBusy = ref(false);
const mergeBusy = ref(false);
const tokenModalOpen = ref(false);
const tokenBusy = ref(false);
const commentEpoch = ref(0);
const page = ref(1);
const hasMore = ref(false);

const supported = computed(() => !!provider.value?.supported);
const hasToken = computed(() => !!provider.value?.hasToken);
const providerLabel = computed(() => {
  const id = provider.value?.provider ?? "";
  if (id === "github") return "GitHub";
  if (id === "gitlab") return "GitLab";
  return id ? id.toUpperCase() : "";
});

const filtered = computed(() => {
  const q = search.value.trim().toLowerCase();
  if (!q) return items.value;
  return items.value.filter((item) =>
    [item.title, item.author, item.sourceBranch, item.targetBranch].some((value) =>
      (value || "").toLowerCase().includes(q)
    )
  );
});

const { setItemRef, tabindexFor, onKeydown: onPrRowKeydown, onFocus: onPrRowFocus } = useRovingList(
  () => filtered.value,
  (item) => String(item.number)
);

const stateFilters = computed(() => [
  { id: "open" as StateFilter, label: t("pr.state.open") },
  { id: "closed" as StateFilter, label: t("pr.state.closed") },
  { id: "merged" as StateFilter, label: t("pr.state.merged") },
  { id: "all" as StateFilter, label: t("pr.state.all") },
]);

function errText(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}

async function loadProvider() {
  if (!props.currentRepoPath) return;
  const repoPath = props.currentRepoPath;
  loadingProvider.value = true;
  try {
    const info = await source().provider();
    if (repoPath !== props.currentRepoPath) return;
    provider.value = info;
  } catch (err) {
    if (repoPath !== props.currentRepoPath) return;
    provider.value = null;
    notify(errText(err));
  } finally {
    loadingProvider.value = false;
  }
}

async function loadList(reset = true) {
  if (!props.currentRepoPath || !supported.value || !hasToken.value) {
    items.value = [];
    hasMore.value = false;
    return;
  }
  if (reset) page.value = 1;
  const repoPath = props.currentRepoPath;
  loadingList.value = true;
  try {
    const result = await source().list({
      state: stateFilter.value,
      limit: 50,
      page: page.value,
    });
    if (repoPath !== props.currentRepoPath) return;
    const batch = result.items ?? [];
    items.value = reset ? batch : [...items.value, ...batch];
    hasMore.value = !!result.hasMore;
    listMessage.value = result.message || "";
  } catch (err) {
    if (repoPath !== props.currentRepoPath) return;
    if (reset) {
      items.value = [];
      hasMore.value = false;
    } else {
      // A failed page fetch must not advance the cursor, or the retry skips it.
      page.value = Math.max(1, page.value - 1);
    }
    listMessage.value = errText(err);
  } finally {
    loadingList.value = false;
  }
}

async function loadMore() {
  if (!hasMore.value || loadingList.value) return;
  page.value += 1;
  await loadList(false);
}

async function refreshAll() {
  await loadProvider();
  if (supported.value && hasToken.value) {
    await loadList();
  }
}

async function selectPr(item: PrSummary) {
  if (selectedNumber.value === item.number) return;
  selectedNumber.value = item.number;
  anchor.value = null;
  detail.value = null;
  await loadDetail();
}

async function loadDetail() {
  if (!props.currentRepoPath || selectedNumber.value == null) return;
  const repoPath = props.currentRepoPath;
  const number = selectedNumber.value;
  loadingDetail.value = true;
  try {
    const d = await source().detail(String(number));
    if (repoPath !== props.currentRepoPath || number !== selectedNumber.value) return;
    detail.value = d;
  } catch (err) {
    if (repoPath !== props.currentRepoPath || number !== selectedNumber.value) return;
    detail.value = null;
    notify(errText(err));
  } finally {
    loadingDetail.value = false;
  }
}

async function submitComment(payload: { body: string; path: string; line: number; side: string }) {
  const repoPath = props.currentRepoPath;
  const number = selectedNumber.value;
  if (!repoPath || number == null) return;
  commentBusy.value = true;
  try {
    const result = await source().addComment(String(number), {
      body: payload.body,
      path: payload.path,
      line: payload.line,
      side: payload.side,
    });
    notify(result?.message || t("pr.comment.submit"));
    if (repoPath === props.currentRepoPath && number === selectedNumber.value) {
      anchor.value = null;
      commentEpoch.value += 1;
      await loadDetail();
    }
  } catch (err) {
    notify(errText(err));
  } finally {
    commentBusy.value = false;
  }
}

async function submitReview(payload: { event: string; body: string }) {
  const repoPath = props.currentRepoPath;
  const number = selectedNumber.value;
  if (!repoPath || number == null) return;
  reviewBusy.value = true;
  try {
    const result = await source().submitVerdict(
      String(number),
      verdictOfEvent(payload.event),
      payload.body
    );
    notify(result.message || t("common.confirm"));
    if (repoPath === props.currentRepoPath && number === selectedNumber.value) {
      await Promise.all([loadDetail(), loadList()]);
    }
  } catch (err) {
    notify(errText(err));
  } finally {
    reviewBusy.value = false;
  }
}

async function mergePr(payload: { method: string }) {
  const repoPath = props.currentRepoPath;
  const number = selectedNumber.value;
  if (!repoPath || number == null || !detail.value) return;
  const confirmed = await confirmAction({
    title: t("pr.merge.confirm.title"),
    message: t("pr.merge.confirm.message", { number }),
    detail: `${t("pr.merge.title")}: ${payload.method}`,
    tone: "warning",
    confirmLabel: t("pr.merge.action"),
  });
  if (!confirmed) return;
  if (repoPath !== props.currentRepoPath || number !== selectedNumber.value) {
    notify(t("pr.error.repo.switched"));
    return;
  }

  mergeBusy.value = true;
  try {
    const result = await source().merge(String(number), payload.method);
    notify(result.message || t("pr.merge.action"));
    await Promise.all([loadDetail(), loadList()]);
  } catch (err) {
    notify(errText(err));
  } finally {
    mergeBusy.value = false;
  }
}

async function saveToken(token: string) {
  const repoPath = props.currentRepoPath;
  tokenBusy.value = true;
  try {
    const info = await source().saveToken(token);
    if (repoPath !== props.currentRepoPath) return;
    provider.value = info;
    notify(info.message || t("pr.token.setup"));
    tokenModalOpen.value = false;
    await loadList();
  } catch (err) {
    notify(errText(err));
  } finally {
    tokenBusy.value = false;
  }
}

async function clearToken() {
  const repoPath = props.currentRepoPath;
  const host = provider.value?.host;
  const confirmed = await confirmAction({
    message: t("pr.token.clear"),
    detail: host ? `${host} · .git/wgit_vault.json` : undefined,
    tone: "danger",
    confirmLabel: t("pr.token.clear"),
  });
  if (!confirmed) return;
  if (repoPath !== props.currentRepoPath) {
    notify(t("pr.error.repo.switched.token"));
    return;
  }

  tokenBusy.value = true;
  try {
    const info = await source().clearToken();
    if (repoPath !== props.currentRepoPath) return;
    provider.value = info;
    items.value = [];
    detail.value = null;
    selectedNumber.value = null;
    tokenModalOpen.value = false;
    notify(info.message || t("pr.token.clear"));
  } catch (err) {
    notify(errText(err));
  } finally {
    tokenBusy.value = false;
  }
}

async function openExternal(url: string) {
  if (!url) return;
  try {
    await gitService.openWebURL(url);
  } catch (err) {
    notify(errText(err));
  }
}

onMounted(refreshAll);

watch(
  () => props.currentRepoPath,
  () => {
    provider.value = null;
    page.value = 1;
    hasMore.value = false;
    items.value = [];
    detail.value = null;
    selectedNumber.value = null;
    anchor.value = null;
    listMessage.value = "";
    void refreshAll();
  }
);

watch(stateFilter, () => {
  void loadList(true);
});
</script>

<template>
  <div class="flex-1 flex flex-col bg-canvas text-ink overflow-hidden select-none font-sans min-h-0">
    <!-- Header -->
    <header class="h-12 shrink-0 border-b border-line px-4 flex items-center gap-3 bg-surface/40 backdrop-blur-sm">
      <div class="flex items-center gap-2 shrink-0">
        <div class="w-7 h-7 rounded-lg bg-accent/15 text-accent flex items-center justify-center">
          <GitPullRequest class="w-4 h-4" />
        </div>
        <span class="text-xs font-semibold text-ink whitespace-nowrap">{{ t("pr.title") }}</span>
      </div>

      <!-- Source switch: remote PR/MR vs local base...head sessions -->
      <div class="flex items-center rounded-lg border border-line bg-raised/60 p-0.5 shrink-0" role="group" :aria-label="t('review.mode.label')">
        <button
          type="button"
          class="px-2 py-0.5 rounded-md text-[10px] font-semibold text-ink-muted hover:text-ink transition cursor-pointer"
          :aria-pressed="false"
          @click="emit('switch-mode', 'local')"
        >
          {{ t("review.mode.local") }}
        </button>
        <button
          type="button"
          class="px-2 py-0.5 rounded-md text-[10px] font-semibold bg-surface text-accent shadow-sm cursor-default"
          :aria-pressed="true"
        >
          {{ t("review.mode.remote") }}
        </button>
      </div>

      <span
        v-if="providerLabel"
        class="text-[10px] font-bold px-1.5 py-0.5 rounded bg-raised border border-line text-ink-muted shrink-0"
      >
        {{ providerLabel }}
      </span>
      <span v-if="provider?.projectPath" class="text-[10px] font-mono text-ink-muted truncate min-w-0" :title="provider.remoteUrl">
        {{ provider.projectPath }}
      </span>

      <span class="flex-1"></span>

      <button
        v-if="provider"
        type="button"
        class="flex items-center gap-1 text-[10px] px-2 py-1 rounded-lg border transition cursor-pointer"
        :class="
          provider.hasToken
            ? 'border-success/25 bg-success/10 text-success hover:bg-success/15'
            : 'border-warning/25 bg-warning/10 text-warning hover:bg-warning/15'
        "
        :title="provider.tokenHint"
        @click="tokenModalOpen = true"
      >
        <KeyRound class="w-3 h-3" />
        <span class="truncate max-w-[220px]">
          {{ provider.hasToken ? provider.tokenSource || t("pr.token.setup") : t("pr.token.missing") }}
        </span>
        <span v-if="provider.account" class="font-mono">· {{ provider.account }}</span>
      </button>

      <UiButton variant="ghost" size="xs" :disabled="loadingProvider || loadingList" @click="refreshAll">
        <RefreshCw class="w-3 h-3" :class="{ 'animate-spin': loadingProvider || loadingList }" />
        {{ t("pr.refresh") }}
      </UiButton>
      <UiButton
        v-if="provider?.webBaseUrl"
        variant="ghost"
        size="xs"
        @click="openExternal(provider.webBaseUrl)"
      >
        <ExternalLink class="w-3 h-3" />
        {{ t("pr.open.browser") }}
      </UiButton>
    </header>

    <!-- Unsupported host -->
    <div v-if="provider && !provider.supported" class="flex-1 flex items-center justify-center p-6">
      <div class="max-w-md text-center space-y-2">
        <Inbox class="w-8 h-8 mx-auto text-ink-muted" />
        <p class="text-xs font-semibold text-ink">{{ t("pr.empty.unsupported") }}</p>
        <p class="text-[11px] text-ink-muted leading-relaxed">{{ provider.message }}</p>
        <UiButton v-if="provider.webBaseUrl" variant="secondary" size="sm" @click="openExternal(provider.webBaseUrl)">
          <ExternalLink class="w-3 h-3" />
          {{ t("pr.open.browser") }}
        </UiButton>
      </div>
    </div>

    <div v-else class="flex-1 flex min-h-0">
      <!-- List column -->
      <aside class="w-[320px] shrink-0 border-r border-line flex flex-col min-h-0 bg-surface/20">
        <div class="p-2 space-y-2 border-b border-line">
          <div class="flex items-center gap-1">
            <button
              v-for="filter in stateFilters"
              :key="filter.id"
              type="button"
              class="flex-1 px-1.5 py-1 rounded-lg text-[10px] font-semibold transition cursor-pointer"
              :class="
                stateFilter === filter.id
                  ? 'bg-raised text-accent shadow-sm'
                  : 'text-ink-muted hover:text-ink hover:bg-raised/60'
              "
              @click="stateFilter = filter.id"
            >
              {{ filter.label }}
            </button>
          </div>
          <div class="relative">
            <Search class="w-3 h-3 absolute left-2 top-1/2 -translate-y-1/2 text-ink-muted pointer-events-none" />
            <input
              v-model="search"
              type="search"
              :placeholder="t('pr.filter.search')"
              class="w-full pl-7 pr-2 py-1.5 rounded-lg bg-canvas border border-line text-[11px] text-ink focus:outline-none focus:border-accent-strong"
            />
          </div>
        </div>

        <div class="flex-1 overflow-y-auto min-h-0">
          <div v-if="loadingList" class="p-4 flex items-center gap-2 text-[11px] text-ink-muted">
            <Loader2 class="w-3.5 h-3.5 animate-spin" />
            {{ t("pr.loading") }}
          </div>

          <div v-else-if="!provider?.hasToken" class="p-3 space-y-2">
            <p class="text-[11px] text-ink-muted leading-relaxed">{{ provider?.message }}</p>
            <UiButton variant="primary" size="xs" @click="tokenModalOpen = true">
              <KeyRound class="w-3 h-3" />
              {{ t("pr.token.setup") }}
            </UiButton>
          </div>

          <div v-else-if="!filtered.length" class="p-4 space-y-1">
            <p class="text-[11px] text-ink-muted">{{ listMessage || t("pr.empty.no.items") }}</p>
          </div>

          <button
            v-for="(item, index) in filtered"
            :key="item.number"
            :ref="(el) => setItemRef(item, el)"
            :tabindex="tabindexFor(item, index)"
            type="button"
            class="w-full text-left px-3 py-2 border-b border-line/60 transition cursor-pointer focus:outline-none focus-visible:ring-2 focus-visible:ring-accent/45"
            :class="selectedNumber === item.number ? 'bg-accent/8' : 'hover:bg-raised/60'"
            @focus="onPrRowFocus(item)"
            @keydown="onPrRowKeydown($event, index)"
            @click="selectPr(item)"
          >
            <div class="flex items-center gap-1.5">
              <span class="text-[10px] font-mono text-ink-muted shrink-0">#{{ item.number }}</span>
              <span
                class="text-[9px] font-bold px-1 py-0.5 rounded shrink-0"
                :class="prStateChip(item.state, item.isDraft)"
              >
                {{ prStateLabel(item.state, item.isDraft) }}
              </span>
              <span class="w-1.5 h-1.5 rounded-full shrink-0" :class="checksDot(item.checksState)" :title="checksLabel(item.checksState)"></span>
              <span class="flex-1"></span>
              <span class="text-[9px] text-ink-muted shrink-0">{{ formatRelativeTime(item.updatedAt) }}</span>
            </div>
            <p class="text-[11px] font-semibold text-ink leading-snug line-clamp-2 mt-0.5">{{ item.title }}</p>
            <div class="flex items-center gap-1.5 mt-1 text-[9px] text-ink-muted">
              <span class="truncate max-w-[80px]">{{ item.author || "—" }}</span>
              <span class="font-mono truncate max-w-[110px]" :title="item.sourceBranch">{{ item.sourceBranch }}</span>
              <span>→</span>
              <span class="font-mono truncate max-w-[80px]" :title="item.targetBranch">{{ item.targetBranch }}</span>
              <span class="flex-1"></span>
              <span class="font-mono text-success">+{{ item.additions }}</span>
              <span class="font-mono text-danger">-{{ item.deletions }}</span>
            </div>
            <div v-if="item.labels?.length" class="flex flex-wrap gap-1 mt-1">
              <span v-for="label in (item.labels ?? []).slice(0, 3)" :key="label" class="text-[9px] px-1 py-0.5 rounded bg-accent/12 text-accent">
                {{ label }}
              </span>
            </div>
          </button>

          <button
            v-if="hasMore && provider?.hasToken"
            type="button"
            class="w-full py-2.5 text-[11px] font-semibold text-accent hover:bg-accent/10 transition cursor-pointer disabled:opacity-50"
            :disabled="loadingList"
            @click="loadMore"
          >
            {{ loadingList ? t("pr.loading") : t("pr.load.more") }}
          </button>
        </div>
      </aside>

      <!-- Detail column -->
      <PrDetailPanel
        :detail="detail"
        :loading="loadingDetail"
        :comment-busy="commentBusy"
        :review-busy="reviewBusy"
        :merge-busy="mergeBusy"
        :anchor="anchor"
        :comment-epoch="commentEpoch"
        @comment="submitComment"
        @review="submitReview"
        @merge="mergePr"
        @anchor="anchor = $event"
        @open-external="openExternal"
        @refresh="loadDetail"
      />
    </div>

    <PrTokenModal
      :open="tokenModalOpen"
      :provider="provider"
      :busy="tokenBusy"
      @save="saveToken"
      @clear="clearToken"
      @update:open="tokenModalOpen = $event"
    />

  </div>
</template>
