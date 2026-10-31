import { gitService } from "./gitService";
import type {
  ChangeReview,
  PrActionResult,
  PrDetail,
  PrProviderInfo,
  PrSummary,
  ReviewComment,
  ReviewFileState,
  ReviewSessionSummary,
  TAPDWorkItem,
  TAPDWorkItemListResult,
} from "../types";

/**
 * Review source adapter.
 *
 * Three review sources exist today:
 *   · local  — offline `base..head` sessions stored in `.git/wgit-review`
 *   · remote — GitHub / GitLab PR/MR objects behind a token
 *   · tapd   — TAPD agile work items, read-only snapshot from the backend RPC
 *
 * They share the same interaction (per-file diff, line comments, a verdict) but
 * disagree on almost everything else: local sessions have per-file verdicts and
 * a progress counter, remote PRs have CI checks, review events and a merge
 * button, TAPD items carry none of these. Rather than teaching every view about
 * each of them, all implement the `ReviewProvider` contract below and declare
 * what they support through `capabilities`. A view asks "can you do this?"
 * instead of "which provider are you?".
 *
 * Adding Gitee / Bitbucket / a self-hosted review server means writing one more
 * adapter — no view changes, as long as the new adapter honours the contract.
 */

export type ReviewSourceKind = "local" | "remote" | "tapd";

/** What a source can actually do. Views must gate UI on these flags. */
export interface ReviewCapabilities {
  /** Verdict per changed file (local sessions). */
  perFileVerdict: boolean;
  /** One verdict for the whole review (PR review events). */
  overallVerdict: boolean;
  inlineComments: boolean;
  /** Comments can be marked resolved / reopened. */
  resolveComment: boolean;
  deleteComment: boolean;
  /** CI checks / pipelines. */
  checks: boolean;
  merge: boolean;
  exportMarkdown: boolean;
  /** Requires an access token managed by the source. */
  token: boolean;
  /** List endpoint supports paging. */
  paging: boolean;
}

/** Unified review outcome. `pending` also means "comment only, no verdict". */
export type ReviewVerdict = "approved" | "changes" | "pending";

/**
 * One row in the review list.
 *
 * Extends `PrSummary` so the remote view keeps rendering unchanged, and adds
 * the two fields every source must provide regardless of platform.
 */
export interface ReviewItem extends PrSummary {
  /** Stable id inside the source: local `"<base>..<head>"`, remote the PR number. */
  id: string;
  verdict: ReviewVerdict;
  /** 0-100; local sessions compute it from per-file verdicts, remote stays 0. */
  progress: number;
}

export interface ReviewFile extends PrFileLike {
  verdict: ReviewVerdict;
  note: string;
}

/** Structural copy of `PrFile` — kept local so the adapter does not depend on it. */
interface PrFileLike {
  path: string;
  oldPath: string;
  status: string;
  additions: number;
  deletions: number;
  patch: string;
  truncated: boolean;
  binary: boolean;
}

export interface ReviewThreadComment {
  id: string;
  author: string;
  body: string;
  path: string;
  line: number;
  side: string;
  createdAt: string;
  /** `review` = anchored to a line, `issue` = plain thread. */
  kind: "review" | "issue";
  replyTo: string;
  resolved: boolean;
}

export interface ReviewCheckView {
  name: string;
  status: string;
  conclusion: string;
  url: string;
  detail: string;
}

export interface ReviewDetail {
  summary: ReviewItem;
  body: string;
  files: ReviewFile[];
  comments: ReviewThreadComment[];
  checks: ReviewCheckView[];
  commitCount: number;
  baseSha: string;
  headSha: string;
  startSha: string;
  filesTruncated: boolean;
  message: string;
  /**
   * Source-native payload (local: the full `ChangeReview` with stats/commits).
   * Views that want to stay provider-agnostic must not read this; it exists so
   * a source can render affordances the unified model does not cover yet.
   */
  native?: unknown;
}

export interface ReviewListQuery {
  state?: string;
  limit?: number;
  page?: number;
}

export interface ReviewListResult<TNativeRow = unknown> {
  items: ReviewItem[];
  total: number;
  hasMore: boolean;
  message: string;
  webBaseUrl: string;
  /**
   * Source-native rows, index-aligned with `items`.
   *
   * The unified item is enough for a provider-agnostic list, but the local
   * workspace renders per-file verdict counts that have no remote equivalent.
   * Rather than flattening the model until both look the same, a source may
   * hand its own row shape to its own list component.
   */
  nativeItems?: TNativeRow[];
}

export interface ReviewCommentInput {
  body: string;
  path?: string;
  line?: number;
  side?: string;
}

/**
 * The minimal read contract every review provider implements: declare a kind,
 * advertise `capabilities`, and serve the list/detail views. Write affordances
 * (comments, verdicts, merge) live on `ReviewSource`; an external adapter may
 * start read-only and grow into the full contract without touching consumers.
 */
export interface ReviewProvider {
  readonly kind: ReviewSourceKind;
  readonly capabilities: ReviewCapabilities;
  list(query?: ReviewListQuery): Promise<ReviewListResult>;
  detail(id: string): Promise<ReviewDetail>;
}

/** The full contract every review source implements. */
export interface ReviewSource extends ReviewProvider {
  /** Unified diff text for one file (local fetches it lazily, remote reuses the patch). */
  diff(id: string, path: string): Promise<string>;
  addComment(id: string, input: ReviewCommentInput): Promise<ReviewDetail | null>;
  setFileVerdict?(
    id: string,
    path: string,
    verdict: ReviewVerdict,
    note?: string
  ): Promise<ReviewDetail | null>;
  resolveComment?(id: string, commentId: string, resolved: boolean): Promise<ReviewDetail | null>;
  deleteComment?(id: string, commentId: string): Promise<ReviewDetail | null>;
  /** Whole-review verdict. `pending` submits a comment-only review. */
  submitVerdict?(
    id: string,
    verdict: ReviewVerdict,
    body?: string
  ): Promise<PrActionResult>;
  merge?(id: string, method: string): Promise<PrActionResult>;
  clear?(id: string): Promise<string>;
  exportMarkdown?(id: string): Promise<string>;
}

/** Local sessions always support per-file verdicts and comment threads. */
export interface LocalReviewSource extends ReviewSource {
  readonly kind: "local";
  list(): Promise<ReviewListResult<ReviewSessionSummary>>;
  setFileVerdict(
    id: string,
    path: string,
    verdict: ReviewVerdict,
    note?: string
  ): Promise<ReviewDetail | null>;
  resolveComment(id: string, commentId: string, resolved: boolean): Promise<ReviewDetail | null>;
  deleteComment(id: string, commentId: string): Promise<ReviewDetail | null>;
  clear(id: string): Promise<string>;
  exportMarkdown(id: string): Promise<string>;
}

/** Remote sources additionally own the token lifecycle. */
export interface RemoteReviewSource extends ReviewSource {
  readonly kind: "remote";
  submitVerdict(id: string, verdict: ReviewVerdict, body?: string): Promise<PrActionResult>;
  merge(id: string, method: string): Promise<PrActionResult>;
  provider(): Promise<PrProviderInfo>;
  saveToken(token: string): Promise<PrProviderInfo>;
  clearToken(): Promise<PrProviderInfo>;
}

// ------------------------------------------------------------------ errors --

/**
 * Source-neutral failure codes.
 *
 * A provider adapter translates its own transport errors into these, so views
 * can show the same localized message for "GitHub returned 404" and "the local
 * session file is gone" instead of leaking provider wording into the UI.
 */
export type ReviewErrorCode = "not_found" | "no_token" | "unsupported" | "invalid_id";

export class ReviewSourceError extends Error {
  constructor(
    readonly code: ReviewErrorCode,
    message?: string
  ) {
    super(message ?? code);
    this.name = "ReviewSourceError";
  }
}

// ------------------------------------------------------------- shared bits --

/** git refs cannot contain `..`, so it is a safe id separator. */
const LOCAL_ID_SEPARATOR = "..";

export function localReviewId(base: string, head: string): string {
  return `${base}${LOCAL_ID_SEPARATOR}${head}`;
}

export function parseLocalReviewId(id: string): { base: string; head: string } {
  const at = id.indexOf(LOCAL_ID_SEPARATOR);
  if (at <= 0) throw new ReviewSourceError("invalid_id", `invalid local review id: ${id}`);
  return { base: id.slice(0, at), head: id.slice(at + LOCAL_ID_SEPARATOR.length) };
}

/** Maps a stored verdict string from the backend onto the unified scale. */
export function verdictFromStatus(status: string): ReviewVerdict {
  if (status === "approved") return "approved";
  if (status === "changes") return "changes";
  return "pending";
}

export function verdictFromCounts(counts: {
  total: number;
  approved: number;
  changes: number;
  pending: number;
}): ReviewVerdict {
  if (counts.changes > 0) return "changes";
  if (counts.total > 0 && counts.pending === 0 && counts.approved > 0) return "approved";
  return "pending";
}

export function progressFromCounts(counts: {
  total: number;
  approved: number;
  changes: number;
}): number {
  if (counts.total <= 0) return 0;
  const done = counts.approved + counts.changes;
  return Math.max(0, Math.min(100, Math.round((done / counts.total) * 100)));
}

/** Maps a unified verdict onto GitHub / GitLab review events. */
export function reviewEventOf(verdict: ReviewVerdict): string {
  if (verdict === "approved") return "approve";
  if (verdict === "changes") return "request_changes";
  return "comment";
}

/** Reads the native payload back out of a local detail. */
export function localNative(detail: ReviewDetail | null): ChangeReview | null {
  return (detail?.native as ChangeReview | undefined) ?? null;
}

// ------------------------------------------------------------ local source --

export type LocalReviewBackend = Pick<
  typeof gitService,
  | "listChangeReviews"
  | "getChangeReview"
  | "getReviewFileDiff"
  | "setReviewFileState"
  | "addReviewComment"
  | "resolveReviewComment"
  | "deleteReviewComment"
  | "clearChangeReview"
  | "exportChangeReviewMarkdown"
>;

export const LOCAL_REVIEW_CAPABILITIES: ReviewCapabilities = {
  perFileVerdict: true,
  overallVerdict: false,
  inlineComments: true,
  resolveComment: true,
  deleteComment: true,
  checks: false,
  merge: false,
  exportMarkdown: true,
  token: false,
  paging: false,
};

function mapLocalSession(session: ReviewSessionSummary): ReviewItem {
  const counts = {
    total: session.total,
    approved: session.approved,
    changes: session.changes,
    pending: session.pending,
  };
  return {
    id: localReviewId(session.base, session.head),
    number: 0,
    title: `${session.base} → ${session.head}`,
    state: "open",
    isDraft: false,
    author: "",
    sourceBranch: session.head,
    targetBranch: session.base,
    createdAt: "",
    updatedAt: session.updatedAt,
    commentCount: session.comments,
    additions: session.additions,
    deletions: session.deletions,
    changedFiles: session.total,
    webUrl: "",
    labels: [],
    checksState: "",
    mergeable: false,
    mergeableState: session.verdictDetail || session.verdict,
    verdict: verdictFromCounts(counts),
    progress: progressFromCounts(counts),
  };
}

function mapLocalFile(file: ReviewFileState): ReviewFile {
  return {
    path: file.path,
    oldPath: "",
    status: file.changeType,
    additions: file.additions,
    deletions: file.deletions,
    patch: "",
    truncated: false,
    binary: file.isBinary,
    verdict: verdictFromStatus(file.status),
    note: file.note ?? "",
  };
}

function mapLocalComment(comment: ReviewComment): ReviewThreadComment {
  return {
    id: comment.id,
    author: comment.author,
    body: comment.body,
    path: comment.path,
    line: comment.line,
    side: comment.side,
    createdAt: comment.createdAt,
    kind: "review",
    replyTo: "",
    resolved: comment.resolved,
  };
}

function mapLocalReview(review: ChangeReview): ReviewDetail {
  const counts = {
    total: review.stats.total,
    approved: review.stats.approved,
    changes: review.stats.changes,
    pending: review.stats.pending,
  };
  const summary: ReviewItem = {
    id: localReviewId(review.base, review.head),
    number: 0,
    title: `${review.baseLabel || review.base} → ${review.headLabel || review.head}`,
    state: "open",
    isDraft: false,
    author: "",
    sourceBranch: review.head,
    targetBranch: review.base,
    createdAt: review.createdAt,
    updatedAt: review.updatedAt,
    commentCount: review.stats.comments,
    additions: review.stats.additions,
    deletions: review.stats.deletions,
    changedFiles: review.stats.total,
    webUrl: "",
    labels: [],
    checksState: "",
    mergeable: false,
    mergeableState: review.stats.verdictDetail || review.stats.verdict,
    verdict: verdictFromCounts(counts),
    progress: review.stats.progress || progressFromCounts(counts),
  };
  return {
    summary,
    body: review.summary,
    files: (review.files ?? []).map(mapLocalFile),
    comments: (review.comments ?? []).map(mapLocalComment),
    checks: [],
    commitCount: (review.commits ?? []).length,
    baseSha: review.base,
    headSha: review.head,
    startSha: review.base,
    filesTruncated: false,
    message: "",
    native: review,
  };
}

/**
 * Offline `base..head` review sessions stored in `.git/wgit-review`.
 * `backend` is injectable so the mapping can be tested without a live repo.
 */
export function localReviewSource(
  repoPath: string,
  backend: LocalReviewBackend = gitService
): LocalReviewSource {
  const refsOf = (id: string) => parseLocalReviewId(id);

  return {
    kind: "local",
    capabilities: LOCAL_REVIEW_CAPABILITIES,

    async list() {
      const sessions = await backend.listChangeReviews(repoPath);
      const rows = sessions ?? [];
      return {
        items: rows.map(mapLocalSession),
        total: rows.length,
        hasMore: false,
        message: "",
        webBaseUrl: "",
        nativeItems: rows,
      };
    },

    async detail(id) {
      const { base, head } = refsOf(id);
      const review = await backend.getChangeReview(repoPath, base, head);
      if (!review) throw new ReviewSourceError("not_found", "review session not found");
      return mapLocalReview(review);
    },

    async diff(id, path) {
      const { base, head } = refsOf(id);
      const raw = await backend.getReviewFileDiff(repoPath, base, head, path);
      return typeof raw === "string" ? raw : "";
    },

    async addComment(id, input) {
      const { base, head } = refsOf(id);
      const review = await backend.addReviewComment(
        repoPath,
        base,
        head,
        input.path ?? "",
        input.line ?? 0,
        input.side ?? "new",
        input.body
      );
      return review ? mapLocalReview(review) : null;
    },

    async setFileVerdict(id, path, verdict, note = "") {
      const { base, head } = refsOf(id);
      const review = await backend.setReviewFileState(repoPath, base, head, path, verdict, note);
      return review ? mapLocalReview(review) : null;
    },

    async resolveComment(id, commentId, resolved) {
      const { base, head } = refsOf(id);
      const review = await backend.resolveReviewComment(repoPath, base, head, commentId, resolved);
      return review ? mapLocalReview(review) : null;
    },

    async deleteComment(id, commentId) {
      const { base, head } = refsOf(id);
      const review = await backend.deleteReviewComment(repoPath, base, head, commentId);
      return review ? mapLocalReview(review) : null;
    },

    async clear(id) {
      const { base, head } = refsOf(id);
      return backend.clearChangeReview(repoPath, base, head);
    },

    async exportMarkdown(id) {
      const { base, head } = refsOf(id);
      const markdown = await backend.exportChangeReviewMarkdown(repoPath, base, head);
      return typeof markdown === "string" ? markdown : "";
    },
  };
}

// ----------------------------------------------------------- remote source --

export type RemoteReviewBackend = Pick<
  typeof gitService,
  | "getPullRequestProvider"
  | "savePullRequestToken"
  | "clearPullRequestToken"
  | "listPullRequests"
  | "getPullRequestDetail"
  | "createPullRequestComment"
  | "submitPullRequestReview"
  | "mergePullRequest"
>;

export const REMOTE_REVIEW_CAPABILITIES: ReviewCapabilities = {
  perFileVerdict: false,
  overallVerdict: true,
  inlineComments: true,
  resolveComment: false,
  deleteComment: false,
  checks: true,
  merge: true,
  exportMarkdown: false,
  token: true,
  paging: true,
};

function mapRemoteSummary(summary: PrSummary): ReviewItem {
  return {
    ...summary,
    id: String(summary.number),
    // A merged PR is by definition accepted; anything else has no verdict yet.
    verdict: summary.state === "merged" ? "approved" : "pending",
    progress: 0,
  };
}

function mapRemoteDetail(detail: PrDetail): ReviewDetail {
  return {
    ...detail,
    summary: mapRemoteSummary(detail.summary),
    files: (detail.files ?? []).map((file) => ({
      ...file,
      verdict: "pending" as ReviewVerdict,
      note: "",
    })),
    comments: (detail.comments ?? []).map((comment) => ({
      ...comment,
      resolved: false,
    })),
  };
}

/**
 * GitHub / GitLab PR/MR via the backend RPCs.
 *
 * The adapter caches the last detail so `diff()` can serve a file's patch
 * without a second network round-trip.
 */
export function remoteReviewSource(
  repoPath: string,
  backend: RemoteReviewBackend = gitService
): RemoteReviewSource {
  let cache: { id: string; detail: ReviewDetail } | null = null;

  const number = (id: string) => {
    const parsed = Number(id);
    if (!Number.isFinite(parsed) || parsed <= 0) {
      throw new ReviewSourceError("invalid_id", `invalid PR id: ${id}`);
    }
    return parsed;
  };

  return {
    kind: "remote",
    capabilities: REMOTE_REVIEW_CAPABILITIES,

    async list(query = {}) {
      const result = await backend.listPullRequests(
        repoPath,
        query.state ?? "open",
        query.limit ?? 50,
        query.page ?? 1
      );
      return {
        items: (result.items ?? []).map(mapRemoteSummary),
        total: result.total ?? 0,
        hasMore: !!result.hasMore,
        message: result.message ?? "",
        webBaseUrl: result.webBaseUrl ?? "",
      };
    },

    async detail(id) {
      const detail = mapRemoteDetail(await backend.getPullRequestDetail(repoPath, number(id)));
      cache = { id, detail };
      return detail;
    },

    async diff(id, path) {
      const cached = cache?.id === id ? cache.detail : await this.detail(id);
      const file = cached.files.find((entry) => entry.path === path);
      return file?.patch ?? "";
    },

    async addComment(id, input) {
      await backend.createPullRequestComment(
        repoPath,
        number(id),
        input.body,
        input.path ?? "",
        input.line ?? 0,
        input.side ?? "new"
      );
      return this.detail(id);
    },

    async submitVerdict(id, verdict, body = "") {
      return backend.submitPullRequestReview(repoPath, number(id), reviewEventOf(verdict), body);
    },

    async merge(id, method) {
      return backend.mergePullRequest(repoPath, number(id), method);
    },

    provider() {
      return backend.getPullRequestProvider(repoPath);
    },

    saveToken(token) {
      return backend.savePullRequestToken(repoPath, token);
    },

    clearToken() {
      return backend.clearPullRequestToken(repoPath);
    },
  };
}

// ------------------------------------------------------------ tapd source --

/**
 * TAPD agile work items, served by the backend `FetchTAPDWorkItems` RPC.
 *
 * Deliberately read-only and minimal: a work item has no diff, no line
 * comments and no merge button in the review sense, so the adapter starts at
 * the bare `ReviewProvider` contract and can grow into the full `ReviewSource`
 * only if TAPD-side affordances are ever added. Nothing here fabricates remote
 * data — every mapped field is either a real value from the RPC or an explicit
 * zero/empty.
 */

export type TAPDReviewBackend = Pick<typeof gitService, "fetchTAPDWorkItems">;

/**
 * Honest capability statement for TAPD work items.
 *
 * Everything is false: the RPC returns a flat work-item snapshot (id, title,
 * status, description, web url) with no diffs, comments, checks or write
 * endpoints, and the TAPD connection is configured backend-side (env), so the
 * source manages no token. `paging` is false because `FetchTAPDWorkItems`
 * returns the whole snapshot in one call with no page argument.
 */
export const TAPD_REVIEW_CAPABILITIES: ReviewCapabilities = {
  perFileVerdict: false,
  overallVerdict: false,
  inlineComments: false,
  resolveComment: false,
  deleteComment: false,
  checks: false,
  merge: false,
  exportMarkdown: false,
  token: false,
  paging: false,
};

/**
 * Conservative TAPD status → unified verdict mapping.
 *
 * TAPD item statuses (see `TAPDWorkItem.status`): "open" | "in_progress" |
 * "resolved" | "verified" | "closed". The mapping never claims a verdict the
 * ticket does not carry:
 *   · "verified" → "approved"   the only state with an explicit acceptance
 *                               signal (已验证);
 *   · anything else → "pending" "open"/"in_progress" are simply not done yet,
 *                               and "resolved" is dev-side completion without
 *                               verification — no approval evidence either;
 *   · "changes" is never produced: TAPD carries no "needs changes" signal, so
 *     asserting one would fabricate review data. Unknown/empty statuses fall
 *     back to "pending" for the same reason.
 */
function tapdVerdictOf(status: string): ReviewVerdict {
  return status === "verified" ? "approved" : "pending";
}

/**
 * Which tickets count as "open" (i.e. reviewable) work. "open" here takes the
 * PR-list sense — not finished — so every status except "closed" is listed;
 * a closed ticket has left the review flow entirely.
 */
function isOpenTAPDItem(item: TAPDWorkItem): boolean {
  return item.status !== "closed";
}

function mapTAPDSummary(item: TAPDWorkItem): ReviewItem {
  const numericId = Number(item.id);
  return {
    id: item.id,
    number: Number.isFinite(numericId) && numericId > 0 ? numericId : 0,
    title: item.title,
    state: item.status === "closed" ? "closed" : "open",
    isDraft: false,
    author: item.owner,
    // Work items have no branches, diffs or comment counts in this snapshot —
    // left empty/zero rather than guessed. `webUrl` feeds the existing
    // open-web channel (gitService.openWebURL) for the detail jump to TAPD.
    sourceBranch: "",
    targetBranch: "",
    createdAt: item.createdTime,
    updatedAt: item.updatedTime,
    commentCount: 0,
    additions: 0,
    deletions: 0,
    changedFiles: 0,
    webUrl: item.url,
    labels: [],
    checksState: "",
    mergeable: false,
    mergeableState: "",
    verdict: tapdVerdictOf(item.status),
    progress: 0,
  };
}

function mapTAPDDetail(item: TAPDWorkItem): ReviewDetail {
  return {
    summary: mapTAPDSummary(item),
    body: item.description ?? "",
    files: [],
    comments: [],
    checks: [],
    commitCount: 0,
    baseSha: "",
    headSha: "",
    startSha: "",
    filesTruncated: false,
    message: "",
    // The raw TAPD row (statusLabel, priority, iteration, boundCommits, url)
    // rides along for a future TAPD-aware view, same pattern as the local
    // source's native `ChangeReview`.
    native: item,
  };
}

/** Read-only TAPD source: registry contract without write affordances. */
export interface TAPDReviewSource extends ReviewProvider {
  readonly kind: "tapd";
}

/**
 * TAPD work items for one repo.
 *
 * `backend` is injectable so the mapping can be tested without a live RPC.
 * `list()` never throws: a failed or not-connected RPC yields the honest empty
 * state. The not-connected notice travels in `ReviewListResult.message`,
 * passed through verbatim from the backend — the adapter authors no
 * user-facing copy of its own, so no i18n key is needed. `total` counts
 * exactly the listed rows so `hasMore: false` stays truthful.
 */
export function createTAPDReviewSource(
  repoPath: string,
  backend: TAPDReviewBackend = gitService
): TAPDReviewSource {
  const fetchSnapshot = async (): Promise<TAPDWorkItemListResult | null> => {
    try {
      return await backend.fetchTAPDWorkItems(repoPath);
    } catch {
      // Transport failure degrades to the same honest empty state as an
      // unconnected workspace — a dead RPC must not look like an error page.
      return null;
    }
  };

  return {
    kind: "tapd",
    capabilities: TAPD_REVIEW_CAPABILITIES,

    async list() {
      const result = await fetchSnapshot();
      const rows = result?.isConnected ? (result.items ?? []).filter(isOpenTAPDItem) : [];
      return {
        items: rows.map(mapTAPDSummary),
        total: rows.length,
        hasMore: false,
        message: result?.message ?? "",
        webBaseUrl: "",
        nativeItems: rows,
      };
    },

    async detail(id) {
      const result = await fetchSnapshot();
      const item = (result?.items ?? []).find((row) => row.id === id);
      if (!item) {
        // Unconnected or unknown id are the same fact to the caller: no such
        // item exists right now. Throwing `not_found` matches local source.
        throw new ReviewSourceError("not_found", `TAPD work item not found: ${id}`);
      }
      return mapTAPDDetail(item);
    },
  };
}

// --------------------------------------------------------------- registry --

/**
 * Providers are repo-scoped, so the registry stores factories rather than
 * instances: `getReviewProvider(kind)` hands back the registered factory and
 * the caller binds it to a repository. The built-ins register themselves
 * below; a future adapter (GitLab / Gitee / ...) registers its own kind the
 * same way and becomes creatable without touching any view or factory.
 */
export type ReviewProviderFactory = (repoPath: string) => ReviewProvider;

const reviewProviderRegistry = new Map<ReviewSourceKind, ReviewProviderFactory>();

/** Registers (or replaces) the provider factory for `kind`. */
export function registerReviewProvider(
  kind: ReviewSourceKind,
  factory: ReviewProviderFactory
): void {
  reviewProviderRegistry.set(kind, factory);
}

/** Looks up the provider factory for `kind`; unknown kinds yield `null`, never a throw. */
export function getReviewProvider(kind: ReviewSourceKind): ReviewProviderFactory | null {
  return reviewProviderRegistry.get(kind) ?? null;
}

registerReviewProvider("local", (repoPath) => localReviewSource(repoPath));
registerReviewProvider("remote", (repoPath) => remoteReviewSource(repoPath));
registerReviewProvider("tapd", (repoPath) => createTAPDReviewSource(repoPath));

// ---------------------------------------------------------------- factory --

export function createReviewSource(kind: "local", repoPath: string): LocalReviewSource;
export function createReviewSource(kind: "remote", repoPath: string): RemoteReviewSource;
export function createReviewSource(kind: "tapd", repoPath: string): TAPDReviewSource;
export function createReviewSource(
  kind: ReviewSourceKind,
  repoPath: string
): ReviewSource | TAPDReviewSource;
export function createReviewSource(
  kind: ReviewSourceKind,
  repoPath: string
): ReviewSource | TAPDReviewSource {
  if (kind === "local") return localReviewSource(repoPath);
  if (kind === "remote") return remoteReviewSource(repoPath);
  return createTAPDReviewSource(repoPath);
}
