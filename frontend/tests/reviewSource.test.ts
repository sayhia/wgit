import { describe, expect, test } from "bun:test";
import {
  createReviewSource,
  getReviewProvider,
  registerReviewProvider,
  ReviewSourceError,
  localNative,
  localReviewId,
  localReviewSource,
  LOCAL_REVIEW_CAPABILITIES,
  parseLocalReviewId,
  progressFromCounts,
  remoteReviewSource,
  REMOTE_REVIEW_CAPABILITIES,
  reviewEventOf,
  verdictFromCounts,
  verdictFromStatus,
  type LocalReviewBackend,
  type RemoteReviewBackend,
  type ReviewProvider,
  type ReviewSourceKind,
} from "../src/services/reviewSource";
import type { ChangeReview, PrDetail, ReviewSessionSummary } from "../src/types";

// --------------------------------------------------------------- fixtures --

function session(overrides: Partial<ReviewSessionSummary> = {}): ReviewSessionSummary {
  return {
    key: "origin/main..HEAD",
    base: "origin/main",
    head: "HEAD",
    updatedAt: "2026-10-23T09:00:00Z",
    total: 4,
    approved: 2,
    changes: 1,
    pending: 1,
    comments: 3,
    openComments: 1,
    additions: 40,
    deletions: 7,
    ready: false,
    verdict: "需修改",
    verdictDetail: "有 1 个文件需要修改",
    ...overrides,
  } as ReviewSessionSummary;
}

function changeReview(overrides: Partial<ChangeReview> = {}): ChangeReview {
  return {
    base: "origin/main",
    head: "HEAD",
    baseLabel: "main",
    headLabel: "feature/x",
    key: "origin/main..HEAD",
    createdAt: "2026-10-23T08:00:00Z",
    updatedAt: "2026-10-23T09:00:00Z",
    summary: "range summary",
    ready: false,
    files: [
      {
        path: "src/a.ts",
        status: "approved",
        changeType: "modified",
        note: "looks fine",
        additions: 10,
        deletions: 2,
        category: "code",
        isBinary: false,
        updatedAt: "2026-10-23T09:00:00Z",
        commentCount: 1,
        openCommentCount: 0,
      },
    ],
    comments: [
      {
        id: "c1",
        path: "src/a.ts",
        line: 12,
        side: "new",
        body: "nit",
        author: "me",
        createdAt: "2026-10-23T09:00:00Z",
        resolved: false,
      },
    ],
    commits: [],
    stats: {
      total: 1,
      approved: 1,
      changes: 0,
      pending: 0,
      comments: 1,
      openComments: 0,
      additions: 10,
      deletions: 2,
      progress: 100,
      verdict: "可以合并",
      verdictDetail: "全部通过",
    },
    ...overrides,
  } as ChangeReview;
}

function prDetail(overrides: Partial<PrDetail> = {}): PrDetail {
  return {
    summary: {
      number: 7,
      title: "Add adapter",
      state: "open",
      isDraft: false,
      author: "alice",
      sourceBranch: "feat",
      targetBranch: "main",
      createdAt: "2026-10-20T00:00:00Z",
      updatedAt: "2026-10-23T00:00:00Z",
      commentCount: 2,
      additions: 30,
      deletions: 4,
      changedFiles: 2,
      webUrl: "https://github.com/acme/demo/pull/7",
      labels: [],
      checksState: "success",
      mergeable: true,
      mergeableState: "clean",
    },
    body: "body",
    files: [
      {
        path: "src/b.ts",
        oldPath: "",
        status: "modified",
        additions: 5,
        deletions: 1,
        patch: "@@ -1 +1 @@",
        truncated: false,
        binary: false,
      },
    ],
    comments: [
      {
        id: "r1",
        author: "bob",
        body: "why",
        path: "src/b.ts",
        line: 3,
        side: "new",
        createdAt: "2026-10-23T00:00:00Z",
        kind: "review",
        replyTo: "",
      },
    ],
    checks: [],
    commitCount: 1,
    baseSha: "aaa",
    headSha: "bbb",
    startSha: "aaa",
    filesTruncated: false,
    message: "",
    ...overrides,
  } as PrDetail;
}

// ------------------------------------------------------------------ pure --

describe("local review ids", () => {
  test("round-trips base and head", () => {
    const id = localReviewId("origin/main", "HEAD");
    expect(id).toBe("origin/main..HEAD");
    expect(parseLocalReviewId(id)).toEqual({ base: "origin/main", head: "HEAD" });
  });

  test("keeps slashes and tags intact", () => {
    expect(parseLocalReviewId(localReviewId("refs/tags/v1.0", "feature/a-b"))).toEqual({
      base: "refs/tags/v1.0",
      head: "feature/a-b",
    });
  });

  test("rejects ids without a separator", () => {
    expect(() => parseLocalReviewId("HEAD")).toThrow();
    expect(() => parseLocalReviewId("..HEAD")).toThrow();
  });
});

describe("verdict mapping", () => {
  test("maps stored statuses", () => {
    expect(verdictFromStatus("approved")).toBe("approved");
    expect(verdictFromStatus("changes")).toBe("changes");
    expect(verdictFromStatus("pending")).toBe("pending");
    expect(verdictFromStatus("")).toBe("pending");
  });

  test("a single needs-changes file outranks approvals", () => {
    expect(verdictFromCounts({ total: 3, approved: 2, changes: 1, pending: 0 })).toBe("changes");
  });

  test("needs every file settled before approving", () => {
    expect(verdictFromCounts({ total: 3, approved: 2, changes: 0, pending: 1 })).toBe("pending");
    expect(verdictFromCounts({ total: 2, approved: 2, changes: 0, pending: 0 })).toBe("approved");
  });

  test("an empty review is pending, not approved", () => {
    expect(verdictFromCounts({ total: 0, approved: 0, changes: 0, pending: 0 })).toBe("pending");
  });

  test("progress ignores pending files and clamps", () => {
    expect(progressFromCounts({ total: 0, approved: 0, changes: 0 })).toBe(0);
    expect(progressFromCounts({ total: 4, approved: 1, changes: 1 })).toBe(50);
    expect(progressFromCounts({ total: 2, approved: 2, changes: 0 })).toBe(100);
  });

  test("maps verdicts onto provider review events", () => {
    expect(reviewEventOf("approved")).toBe("approve");
    expect(reviewEventOf("changes")).toBe("request_changes");
    expect(reviewEventOf("pending")).toBe("comment");
  });
});

// ----------------------------------------------------------------- local --

describe("local review source", () => {
  const calls: { method: string; args: unknown[] }[] = [];
  const backend = {
    listChangeReviews: async () => [session(), session({ base: "main", head: "dev", total: 1 })],
    getChangeReview: async (_repo: string, base: string, head: string) => {
      calls.push({ method: "get", args: [base, head] });
      return changeReview({ base, head });
    },
    getReviewFileDiff: async () => "@@ diff @@",
    setReviewFileState: async (_r: string, base: string, head: string, path: string, status: string) => {
      calls.push({ method: "set", args: [base, head, path, status] });
      return changeReview({ base, head });
    },
    addReviewComment: async () => changeReview(),
    resolveReviewComment: async () => changeReview(),
    deleteReviewComment: async () => changeReview(),
    clearChangeReview: async () => "cleared",
    exportChangeReviewMarkdown: async () => "# report",
  } as unknown as LocalReviewBackend;

  test("lists sessions with unified ids and native rows", async () => {
    const source = localReviewSource("/repo", backend);
    const result = await source.list();
    expect(result.items).toHaveLength(2);
    expect(result.items[0]!.id).toBe("origin/main..HEAD");
    expect(result.items[0]!.verdict).toBe("changes");
    expect(result.items[0]!.progress).toBe(75);
    expect(result.nativeItems).toHaveLength(2);
    expect(result.nativeItems![1]!.base).toBe("main");
  });

  test("detail maps files and comments onto the shared model", async () => {
    const source = localReviewSource("/repo", backend);
    const detail = await source.detail("origin/main..HEAD");
    expect(detail.summary.id).toBe("origin/main..HEAD");
    expect(detail.files[0]!.verdict).toBe("approved");
    expect(detail.files[0]!.note).toBe("looks fine");
    expect(detail.comments[0]!.resolved).toBe(false);
    expect(detail.checks).toEqual([]);
    expect(localNative(detail)?.base).toBe("origin/main");
  });

  test("declares per-file verdicts and no merge", () => {
    const source = localReviewSource("/repo", backend);
    expect(source.capabilities.perFileVerdict).toBe(true);
    expect(source.capabilities.overallVerdict).toBe(false);
    expect(source.capabilities.merge).toBe(false);
    expect(source.capabilities.token).toBe(false);
    expect(source.merge).toBeUndefined();
  });

  test("passes the parsed range through to the backend", async () => {
    calls.length = 0;
    const source = localReviewSource("/repo", backend);
    await source.setFileVerdict("main..dev", "src/a.ts", "changes", "nope");
    expect(calls).toEqual([
      { method: "set", args: ["main", "dev", "src/a.ts", "changes"] },
    ]);
  });

  test("reports a missing session with a source-neutral code", async () => {
    const empty = {
      ...backend,
      getChangeReview: async () => null,
    } as unknown as LocalReviewBackend;
    const source = localReviewSource("/repo", empty);
    const failure = source.detail("origin/main..HEAD");
    await expect(failure).rejects.toBeInstanceOf(ReviewSourceError);
    await expect(failure).rejects.toMatchObject({ code: "not_found" });
  });

  test("diff and export go through the adapter", async () => {
    const source = localReviewSource("/repo", backend);
    expect(await source.diff("origin/main..HEAD", "src/a.ts")).toBe("@@ diff @@");
    expect(await source.exportMarkdown("origin/main..HEAD")).toBe("# report");
    expect(await source.clear("origin/main..HEAD")).toBe("cleared");
  });
});

// ---------------------------------------------------------------- remote --

describe("remote review source", () => {
  let detailCalls = 0;
  const backend = {
    getPullRequestProvider: async () => ({ supported: true, hasToken: true }),
    savePullRequestToken: async () => ({ supported: true, hasToken: true }),
    clearPullRequestToken: async () => ({ supported: true, hasToken: false }),
    listPullRequests: async () => ({
      provider: "github",
      items: [
        {
          number: 7,
          title: "Add adapter",
          state: "open",
          isDraft: false,
          author: "alice",
          sourceBranch: "feat",
          targetBranch: "main",
          createdAt: "",
          updatedAt: "",
          commentCount: 0,
          additions: 1,
          deletions: 0,
          changedFiles: 1,
          webUrl: "",
          labels: [],
          checksState: "",
          mergeable: true,
          mergeableState: "clean",
        },
        {
          number: 8,
          title: "Old",
          state: "merged",
          isDraft: false,
          author: "bob",
          sourceBranch: "x",
          targetBranch: "main",
          createdAt: "",
          updatedAt: "",
          commentCount: 0,
          additions: 0,
          deletions: 0,
          changedFiles: 0,
          webUrl: "",
          labels: [],
          checksState: "",
          mergeable: false,
          mergeableState: "",
        },
      ],
      total: 2,
      hasMore: false,
      fetchedAt: "",
      webBaseUrl: "https://github.com/acme/demo",
      message: "",
    }),
    getPullRequestDetail: async () => {
      detailCalls += 1;
      return prDetail();
    },
    createPullRequestComment: async () => ({ success: true, action: "comment", message: "ok", webUrl: "", newState: "" }),
    submitPullRequestReview: async (_r: string, _n: number, event: string) => ({
      success: true,
      action: event,
      message: "ok",
      webUrl: "",
      newState: "",
    }),
    mergePullRequest: async () => ({ success: true, action: "merge", message: "merged", webUrl: "", newState: "merged" }),
  } as unknown as RemoteReviewBackend;

  test("uses the PR number as id and marks merged as approved", async () => {
    const source = remoteReviewSource("/repo", backend);
    const result = await source.list();
    expect(result.items.map((item) => item.id)).toEqual(["7", "8"]);
    expect(result.items[1]!.verdict).toBe("approved");
    expect(result.items[0]!.verdict).toBe("pending");
  });

  test("declares overall verdicts and merge, no per-file verdicts", () => {
    const source = remoteReviewSource("/repo", backend);
    expect(source.capabilities.overallVerdict).toBe(true);
    expect(source.capabilities.merge).toBe(true);
    expect(source.capabilities.perFileVerdict).toBe(false);
    expect(source.capabilities.token).toBe(true);
    expect(source.setFileVerdict).toBeUndefined();
  });

  test("diff reuses the cached detail instead of refetching", async () => {
    detailCalls = 0;
    const source = remoteReviewSource("/repo", backend);
    await source.detail("7");
    const patch = await source.diff("7", "src/b.ts");
    expect(patch).toBe("@@ -1 +1 @@");
    expect(detailCalls).toBe(1);
  });

  test("maps verdicts onto review events", async () => {
    const source = remoteReviewSource("/repo", backend);
    const approved = await source.submitVerdict("7", "approved");
    const changes = await source.submitVerdict("7", "changes", "please fix");
    const commentOnly = await source.submitVerdict("7", "pending");
    expect(approved.action).toBe("approve");
    expect(changes.action).toBe("request_changes");
    expect(commentOnly.action).toBe("comment");
  });

  test("rejects non-numeric ids before hitting the backend", async () => {
    const source = remoteReviewSource("/repo", backend);
    await expect(source.detail("abc")).rejects.toThrow();
  });
});

// --------------------------------------------------------------- factory --

describe("createReviewSource", () => {
  test("returns the adapter matching the requested kind", () => {
    expect(createReviewSource("local", "/repo").kind).toBe("local");
    expect(createReviewSource("remote", "/repo").kind).toBe("remote");
  });

  test("both kinds expose the same read surface", () => {
    for (const kind of ["local", "remote"] as const) {
      const source = createReviewSource(kind, "/repo");
      expect(typeof source.list).toBe("function");
      expect(typeof source.detail).toBe("function");
      expect(typeof source.diff).toBe("function");
      expect(typeof source.addComment).toBe("function");
    }
  });
});

// --------------------------------------------------------------- provider --

describe("review provider contract", () => {
  test("unknown kinds resolve to null instead of throwing", () => {
    expect(() => getReviewProvider("gitee" as ReviewSourceKind)).not.toThrow();
    expect(getReviewProvider("gitee" as ReviewSourceKind)).toBeNull();
  });

  test("local is a built-in provider satisfying the read contract", () => {
    const factory = getReviewProvider("local");
    expect(factory).not.toBeNull();
    const provider = factory!("/repo");
    // Compile-time lock: a local source is assignable to the provider contract.
    const asProvider: ReviewProvider = provider;
    expect(asProvider.kind).toBe("local");
    expect(asProvider.capabilities).toBe(LOCAL_REVIEW_CAPABILITIES);
    expect(typeof asProvider.list).toBe("function");
    expect(typeof asProvider.detail).toBe("function");
  });

  test("the registry hands back the same registration on every call", () => {
    expect(getReviewProvider("local")).toBe(getReviewProvider("local"));
    expect(getReviewProvider("remote")).toBe(getReviewProvider("remote"));
  });

  test("createReviewSource and the registry agree on the local provider", () => {
    const viaRegistry = getReviewProvider("local")!("/repo");
    const viaFactory = createReviewSource("local", "/repo");
    expect(viaRegistry.kind).toBe(viaFactory.kind);
    expect(viaRegistry.capabilities).toBe(viaFactory.capabilities);
  });

  test("a new adapter can register its kind and be served by the registry", () => {
    const tapd: ReviewProvider = {
      kind: "remote",
      capabilities: REMOTE_REVIEW_CAPABILITIES,
      list: async () => ({ items: [], total: 0, hasMore: false, message: "", webBaseUrl: "" }),
      detail: async () => {
        throw new ReviewSourceError("not_found", "tapd adapter not wired yet");
      },
    };
    registerReviewProvider("tapd" as ReviewSourceKind, () => tapd);
    const factory = getReviewProvider("tapd" as ReviewSourceKind);
    expect(factory).not.toBeNull();
    expect(factory!("/repo")).toBe(tapd);
  });
});
