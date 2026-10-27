import type {
  RepoStatus,
  CommitItem,
  CommitDetail,
  CommitSigningResult,
  CommitSigningStatus,
  GitVersionInfo,
  SecretFinding,
  BranchInfo,
  ConflictBlockResolveResult,
  MetaConflictResolutionResult,
  LfsLockNotificationTemplate,
  RepoInfo,
  ProjectSourceFileItem,
  SourceFileDetail,
  FullParityStatusSummary,
  PinnacleParityStatusSummary,
  TagInfo,
  StashInfo,
  StashFileEntry,
  GitHookInfo,
  GitHookRunResult,
  GitHooksFrameworkInfo,
  GitUserConfig,
  GitProxyConfig,
  ReflogItem,
  RemoteDetail,
  WorktreeInfo,
  WorktreeStatus,
  AddWorktreeAdvancedOptions,
  LfsLockInfo,
  LfsTrackPattern,
  SubmoduleInfo,
  DiskBreakdownResult,
  LargeFileInfo,
  LfsConvertResult,
  LfsBulkUnlockResult,
  RangeDiffResult,
  FixupCommitResult,
  ObjectStoreOptimizeResult,
  MergeOverlapResult,
  HealthFixResult,
  FixCenterResult,
  AuditFixResult,
  SecretRemediationResult,
  LanServerInfo,
  ComplianceRules,
  CommitPolicyResult,
  RepoComplianceReport,
  MaterialDiffResult,
  TextureMetadataResult,
  LanLfsPeerStatus,
  LanLfsTransferResult,
  LanLfsShareConfig,
  ReleaseChangelogResult,
  SparsePreset,
  SparseApplyResult,
  SceneHydrateResult,
  BranchRiverData,
  StaleBranchesReport,
  BranchArchiveResult,
  CodeOwnershipReport,
  FileTimelineEntry,
  LocalCiResult,
  WebhookDispatchResult,
  RepoCachePrewarmResult,
  AssetDiffResult,
  GitBundleResult,
  CommitCIStatus,
  PartialCloneStatus,
  HydrateBlobsResult,
  MergeConflictPrediction,
  EngineProcessStatus,
  ExternalDiffToolResult,
  LfsMigratePreview,
  LfsMigrateOptions,
  LfsMigrateResult,
  TagVerifyResult,
  AssetImpactResult,
  EngineBootstrapOptions,
  EngineBootstrapResult,
  OperationLogResult,
  UndoResult,
  TerminalStatus,
  RepoMaintenanceStats,
  LfsPruneResult,
  GitFsckResult,
  LfsCacheStats,
  LfsFilterConfig,
  IgnoreSanitizerResult,
  IgnoreSanitizeActionResult,
  CheckIgnoreResult,
  FileRevertResult,
  FileTimelineResult,
  PickaxeResult,
  TagDetail,
  SignatureDetails,
  TeamActivityResult,
  PrePushChecklistResult,
  CommitSafetyFilterResult,
  GitCommandLog,
  GameEngineInfo,
  EngineWorkflowInfo,
  AssetMetadata,
  AssetReferenceResult,
  UnityMetaMismatch,
  UnityMergeStatus,
  TextureLintResult,
  SubtreeListResult,
  SubtreeItem,
  SubtreeActionResult,
  SubtreeSplitResult,
  SubtreeUpdateParams,
  CustomActionConfig,
  CustomActionResult,
  CommitTemplateInfo,
  RepoOverviewStats,
  CommitTopologyStats,
  ProjectDetailItem,
  PrivacyComplianceResult,
  AuthDiagnosticResult,
  LfsTuningConfig,
  LfsTuningResult,
  LfsFileRevision,
  LfsLockActivityResult,
  LockableSyncResult,
  PrePushVerificationResult,
  FileStatus,
  TAPDWorkItemListResult,
  TAPDBindResult,
  TAPDCommitReferenceResult,
  StudioCIResult,
  StudioCIActionResult,
  CredentialVaultResult,
  StoreSecretResult,
  StudioWebhookResult,
  StudioDDCResult,
  OfflineQueueResult,
  StudioSwitchboardResult,
  WorkspaceStats,
  StorageBreakdownItem,
  DiffComparisonResult,
  RepoHealthReport,
  HistoryLargeBlob,
  ConfigEntry,
  MeshLintResult,
  AnimRigLintResult,
  AudioInspectResult,
  LocalizationAuditResult,
  NativePluginsAbiResult,
  MissingMaterialScanResult,
  LodStreamingAuditResult,
  PhysicsCollisionAuditResult,
  MaterialInstanceOverrideResult,
  DCCLockWatchResult,
  DCCAutoLockResult,
  PackageBudgetResult,
  CommitLintResult,
  CrashStackDemangleResult,
  HotfixManifestResult,
  SymbolsDefinesAuditResult,
  ShaderInspectResult,
  DataTableAuditResult,
  UnrealCacheCleanResult,
  AssetCherryPickResult,
  UnrealAssetDiffResult,
  UnrealProjectInfo,
  SceneStreamAuditResult,
  SceneStreamResolveResult,
  FontLicenseAuditResult,
  LightmapBakeAuditResult,
  TextureChannelPackingResult,
  AssetCycleDetectResult,
  HotfixBackportResult,
  BundleDiffResult,
  AudioBankDiffResult,
  WorldDataLayersDiffResult,
  OfflinePatchBundleResult,
  ShaderSelfHealingResult,
  ArtistPipelineResult,
  BlameLine,
  BlameOptions,
  BulkLockResult,
  BranchCompareResult,
  BranchInsightsResult,
  CommitPaginationResult,
  CloneOptions,
  GrepMatchItem,
  MultiRemoteStatusResult,
  MultiRemoteSyncResult,
  RemoteTestResult,
  RebaseTodoItem,
  BatchCherryPickResult,
  BisectStatus,
  BisectRunResult,
  WorktreeOpResult,
  GitHookContent,
  ChangeReview,
  ReviewSessionSummary,
  ReleasePlan,
  ReleaseCreateResult,
  StudioChangelogResult,
  ExternalToolInfo,
  ExternalToolConfig,
  RemoteWebInfo,
  CommitNoteItem,
  CommitNotesResult,
  PullAdvancedOptions,
  PullResult,
  PushAdvancedOptions,
  PushResult,
  MergePreviewResult,
  MergeAdvancedOptions,
  MergeResult,
  RebaseAdvancedOptions,
  RebaseResult,
  RebaseProgressStatus,
  PrProviderInfo,
  PrListResult,
  PrDetail,
  PrActionResult,
} from "../types";

import { Call } from "@wailsio/runtime";
import { LruCache } from "../utils/lruCache";

export const commitDetailCache = new LruCache<string, CommitDetail>(150);

/** Result row for large-repo quick-open file search (backend: SearchProjectFiles). */
export interface ProjectFileMatch {
  path: string;
  name: string;
  dir: string;
  isBinary?: boolean;
}

/**
 * Deterministic dev-preview fixtures so the Vite-only preview (no Wails runtime)
 * can exercise large-project views: 2302 commits, hundreds of branches and a
 * realistic file tree. These never ship in the desktop app path.
 */
function mockCommitList(): CommitItem[] {
  const subjects = [
    "feat(studio): 大型项目工作区支持",
    "fix(history): 分页加载不再重复",
    "perf(sparse): 稀疏检出加速",
    "feat(lfs): 批量锁定资产",
    "refactor(branches): 分支洞察与清理",
    "chore(deps): 升级依赖",
    "feat(search): 代码检索与快速打开",
    "fix(diff): 超大差异截断保护",
  ];
  const authors = ["sunmking", "alice", "bob", "carol"];
  const commits: CommitItem[] = [];
  for (let i = 0; i < 2302; i++) {
    const hash = ("a1b2c3d4" + i.toString(16).padStart(32, "0")).slice(0, 40);
    const lane = i % 3;
    commits.push({
      hash,
      shortHash: hash.slice(0, 7),
      parents: i + 1 < 2302 ? [("a1b2c3d4" + (i + 1).toString(16).padStart(32, "0")).slice(0, 40)] : [],
      authorName: authors[i % authors.length]!,
      authorEmail: `${authors[i % authors.length]}@studio.dev`,
      date: new Date(Date.now() - i * 3600_000).toISOString(),
      relativeDate: i === 0 ? "刚刚" : `${i} 小时前`,
      subject: `${subjects[i % subjects.length]} (#${2302 - i})`,
      body: i % 5 === 0 ? "这是用于开发预览的提交正文，展示大型仓库历史记录的分页与筛选能力。" : "",
      refs: i === 0 ? ["HEAD -> main", "origin/main"] : i === 120 ? ["tag: v1.0.0"] : [],
      isHead: i === 0,
      lane,
      lines: i + 1 < 2302 ? [{ fromLane: lane, toLane: lane, color: "#3d63dd" }] : [],
      signatureStatus: i % 7 === 0 ? "G" : "N",
      signer: i % 7 === 0 ? "sunmking (GPG)" : "",
    } as CommitItem);
  }
  return commits;
}

function mockProjectFiles(): ProjectFileMatch[] {
  const paths = [
    "frontend/src/App.vue",
    "frontend/src/components/HistoryView.vue",
    "frontend/src/components/BranchesView.vue",
    "frontend/src/components/WorkspaceView.vue",
    "frontend/src/components/ToolsView.vue",
    "frontend/src/components/QuickOpen.vue",
    "frontend/src/services/gitService.ts",
    "frontend/src/services/mock.ts",
    "git_service.go",
    "git_large_repo.go",
    "git_parser.go",
    "git_branch_insights.go",
    "main.go",
    "README.md",
    "docs/大型项目使用指南.md",
    "docs/架构说明.md",
    "assets/hero_albedo.png",
    "assets/中文配置表.xlsx",
    "deploy/prod/cluster.yaml",
    "scripts/benchmark_large_repo.sh",
  ];
  return paths.map((p) => {
    const idx = p.lastIndexOf("/");
    return {
      path: p,
      name: idx === -1 ? p : p.slice(idx + 1),
      dir: idx === -1 ? "" : p.slice(0, idx),
      isBinary: /\.(png|xlsx|zip|bin)$/i.test(p),
    };
  });
}

/**
 * Dev-preview state for the git bisect panel (mock backend only).
 */
let mockBisectActive = false;
let mockBisectJudged = 0;
const MOCK_BISECT_HASHES = ["a1b2c3d4e5f6", "b2c3d4e5f6a7", "c3d4e5f6a7b8", "d4e5f6a7b8c9", "e5f6a7b8c9d0"];

function mockBisectStatus(): BisectStatus {
  const hash = MOCK_BISECT_HASHES[Math.min(mockBisectJudged, MOCK_BISECT_HASHES.length - 1)]!;
  return {
    isActive: mockBisectActive,
    currentHash: mockBisectActive ? hash : "",
    currentStep: mockBisectActive ? `已完成 ${mockBisectJudged} 次 good/bad 判定` : "",
    badRef: mockBisectActive ? "a1b2c3d" : "",
    goodRefs: mockBisectActive ? ["0000001"] : [],
    log: mockBisectActive
      ? "git bisect start\ngit bisect bad a1b2c3d\ngit bisect good 0000001\n"
      : "",
  };
}

/**
 * Dev-preview state for change-review sessions (mock backend only).
 */
function makeMockReview(base = "origin/main", head = "HEAD"): ChangeReview {
  const files = [
    { path: "frontend/src/components/HistoryView.vue", status: "approved", changeType: "M", note: "分页与筛选逻辑已复核", additions: 128, deletions: 12, category: "Code", isBinary: false, updatedAt: "2026-10-21 00:30", commentCount: 1, openCommentCount: 0 },
    { path: "git_large_repo.go", status: "changes", changeType: "A", note: "搜索排序建议增加长度权重测试", additions: 420, deletions: 0, category: "Code", isBinary: false, updatedAt: "2026-10-21 00:31", commentCount: 2, openCommentCount: 1 },
    { path: "docs/screenshots/15-rebase-plan.png", status: "pending", changeType: "A", note: "", additions: 0, deletions: 0, category: "Asset", isBinary: true, updatedAt: "", commentCount: 0, openCommentCount: 0 },
  ];
  return {
    base,
    head,
    baseLabel: base,
    headLabel: head,
    key: `${base}...${head}`,
    createdAt: "2026-10-21 00:20",
    updatedAt: "2026-10-21 00:31",
    summary: "大型项目功能二轮评审",
    ready: false,
    files,
    comments: [
      { id: "c1", path: "frontend/src/components/HistoryView.vue", line: 120, side: "new", body: "这段虚拟滚动窗口计算很关键，已验证 3000 条不卡顿。", author: "sunmking", createdAt: "2026-10-21 00:26", resolved: true },
      { id: "c2", path: "git_large_repo.go", line: 88, side: "new", body: "建议为 fuzzyScore 增加单元测试覆盖中文路径。", author: "alice", createdAt: "2026-10-21 00:28", resolved: false },
    ],
    commits: [
      { hash: "a1b2c3d4e5f6", shortHash: "a1b2c3d", author: "sunmking", date: "2026-10-21 00:22", subject: "feat(history): 分页 + 筛选 + 虚拟滚动" },
      { hash: "b2c3d4e5f6a7", shortHash: "b2c3d4e", author: "sunmking", date: "2026-10-21 00:25", subject: "feat(large-repo): 服务端模糊文件搜索" },
    ],
    stats: {
      total: 3,
      approved: 1,
      changes: 1,
      pending: 1,
      comments: 2,
      openComments: 1,
      additions: 548,
      deletions: 12,
      progress: 67,
      verdict: "changes_requested",
      verdictDetail: "仍有 1 个文件待评审、1 条未解决评论",
    },
  };
}

let mockReview: ChangeReview = makeMockReview();

/**
 * Errors raised by the Wails runtime for a real backend failure (as opposed to a
 * missing backend). RuntimeError = method returned an error, ReferenceError =
 * unknown method, TypeError = invalid arguments.
 */
const BACKEND_ERROR_NAMES = new Set(["RuntimeError", "ReferenceError", "TypeError"]);

/**
 * Invokes a backend Go method registered via Wails v3 application.NewService(&GitService{})
 */
export async function callBackend<T = any>(methodName: string, ...args: any[]): Promise<T> {
  const fqn = methodName.startsWith("main.GitService.") ? methodName : `main.GitService.${methodName}`;

  try {
    return await Call.ByName(fqn, ...args);
  } catch (err) {
    const errName = (err as { name?: string })?.name ?? "";
    // Real backend/git failures must reach the caller. The old blanket fallback
    // masked them (e.g. stash clear reported success while doing nothing).
    if (!import.meta.env.DEV || BACKEND_ERROR_NAMES.has(errName)) {
      throw err instanceof Error
        ? err
        : new Error((err as { message?: string })?.message ?? String(err));
    }
    // Standalone `bun run dev` preview has no Wails backend; the runtime rejects
    // with a plain transport error, so serve deterministic fixtures instead.
    console.debug(`[Wails v3] Backend unavailable, using preview fixtures for ${methodName}:`, err);
    return getDevMockResponse<T>(methodName, args);
  }
}

// Stateful preview fixture so hook enable/disable/bulk actions can be exercised
// in the standalone `bun run dev` preview without a desktop backend.
let devMockHookState: Array<{ name: string; installed: boolean; enabled: boolean }> | null = null;

function ensureDevMockHookState(): Array<{ name: string; installed: boolean; enabled: boolean }> {
  if (!devMockHookState) {
    const fixture: Array<[string, boolean, boolean]> = [
      ["pre-commit", true, true],
      ["prepare-commit-msg", true, false],
      ["commit-msg", true, true],
      ["post-commit", false, false],
      ["pre-rebase", true, false],
      ["post-checkout", false, false],
      ["post-merge", true, true],
      ["pre-push", true, false],
    ];
    devMockHookState = fixture.map(([name, installed, enabled]) => ({
      name,
      installed,
      enabled: installed && enabled,
    }));
  }
  return devMockHookState;
}

let devMockGitignorePatterns: string[] | null = null;

function ensureDevMockGitignore(): string[] {
  if (!devMockGitignorePatterns) {
    devMockGitignorePatterns = [
      "# managed by wgit preview",
      "/node_modules/",
      "/dist/",
      "/bin/wgit",
      "*.log",
    ];
  }
  return devMockGitignorePatterns;
}

function devMockHooks(base: string): Array<{ name: string; installed: boolean; enabled: boolean; path: string }> {
  return ensureDevMockHookState().map((h) => ({ ...h, path: `${base}/.git/hooks/${h.name}` }));
}

function getDevMockResponse<T>(methodName: string, args: any[]): T {
  const method = methodName.replace("main.GitService.", "");
  const currentDir = typeof window !== "undefined" ? window.location.pathname : "/Users/sunmking/workplace/wgit";

  switch (method) {
    case "GetInitialDeepLink":
      return "" as unknown as T;
    case "GetGitignorePatterns":
      return [...ensureDevMockGitignore()] as unknown as T;
    case "AddGitignorePattern": {
      const pattern = String(args[1] || "").trim();
      if (pattern) {
        const list = ensureDevMockGitignore();
        if (!list.includes(pattern)) list.push(pattern);
      }
      return undefined as unknown as T;
    }
    case "RemoveGitignorePattern": {
      const pattern = String(args[1] || "").trim();
      if (pattern) {
        const list = ensureDevMockGitignore();
        const idx = list.indexOf(pattern);
        if (idx >= 0) list.splice(idx, 1);
      }
      return undefined as unknown as T;
    }
    case "UpdateGitignorePattern": {
      const oldPattern = String(args[1] || "").trim();
      const newPattern = String(args[2] || "").trim();
      const list = ensureDevMockGitignore();
      const index = list.indexOf(oldPattern);
      if (index < 0) throw new Error(`gitignore rule not found: ${oldPattern}`);
      if (newPattern && newPattern !== oldPattern && list.includes(newPattern)) {
        throw new Error(`gitignore rule already exists: ${newPattern}`);
      }
      if (newPattern) list[index] = newPattern;
      return undefined as unknown as T;
    }
    case "SwapGitignorePatterns": {
      const firstPattern = String(args[1] || "").trim();
      const secondPattern = String(args[2] || "").trim();
      const list = ensureDevMockGitignore();
      const firstIdx = list.indexOf(firstPattern);
      const secondIdx = list.indexOf(secondPattern);
      if (firstIdx < 0) throw new Error(`gitignore rule not found: ${firstPattern}`);
      if (secondIdx < 0) throw new Error(`gitignore rule not found: ${secondPattern}`);
      list[firstIdx] = secondPattern;
      list[secondIdx] = firstPattern;
      return undefined as unknown as T;
    }
    case "GetGitHooks":
      return devMockHooks(String(args[0] || currentDir)) as unknown as T;
    case "DetectHooksFramework": {
      const repo = String(args[0] || currentDir);
      return {
        type: "native",
        configFile: "",
        hooksPath: `${repo}/.git/hooks`,
      } as unknown as T;
    }
    case "RunGitHook": {
      const hookName = String(args[1] || "hook");
      return {
        exitCode: 0,
        stdout: `Hook ${hookName} executed successfully (mock)`,
        stderr: "",
        durationMs: 15,
        success: true,
      } as unknown as T;
    }
    case "InstallSampleHook":
      return undefined as unknown as T;
    case "CommitAdvanced":
      return undefined as unknown as T;
    case "ToggleGitHook": {
      const hookName = String(args[1] || "");
      const enable = !!args[2];
      const hook = ensureDevMockHookState().find((h) => h.name === hookName);
      if (hook) {
        if (hook.installed) hook.enabled = enable;
        else if (enable) {
          hook.installed = true;
          hook.enabled = true;
        }
      }
      return undefined as unknown as T;
    }
    case "GetRecentRepos":
      return ["/Users/sunmking/workplace/wgit", "/Users/sunmking/workplace/project-alpha"] as unknown as T;
    case "OpenRepo":
      return {
        path: args[0] || "/Users/sunmking/workplace/wgit",
        name: "wgit",
        currentBranch: "main",
        isClean: false,
      } as unknown as T;
    case "OpenRepoDialog":
      return {
        path: "/Users/sunmking/workplace/wgit",
        name: "wgit",
        currentBranch: "main",
        isClean: false,
      } as unknown as T;
    case "GetRemoteDetails":
      return [
        { name: "origin", url: "git@github.com:wumitech-com/cancer.git", type: "fetch" },
        { name: "upstream", url: "git@github.com:upstream/cancer.git", type: "fetch" },
      ] as unknown as T;
    case "GetWorktrees":
      return [
        {
          path: "/Users/sunmking/workplace/wgit",
          branch: "main",
          head: "a1b2c3d4e5f60718293a4b5c6d7e8f9012345678",
          isBare: false,
          isLocked: false,
          lockReason: "",
          isMain: true,
          isPrunable: false,
          pruneReason: "",
          isDetached: false,
          dirtyCount: 0,
        },
        {
          path: "/Users/sunmking/workplace/wgit-hotfix",
          branch: "hotfix/v1.0.1",
          head: "f9e8d7c6b5a43210fedcba9876543210abcdef01",
          isBare: false,
          isLocked: false,
          lockReason: "",
          isMain: false,
          isPrunable: false,
          pruneReason: "",
          isDetached: false,
          dirtyCount: 1,
        },
      ] as unknown as T;
    case "AddWorktreeAdvanced":
      return undefined as unknown as T;
    case "MoveWorktree":
      return undefined as unknown as T;
    case "GetWorktreeStatus":
      return {
        path: args[1] || args[0] || "",
        isDirty: false,
        dirtyCount: 0,
        summary: "工作区整洁，无未提交的变更",
      } as unknown as T;
    case "PruneSingleWorktree":
      return undefined as unknown as T;
    case "GetRepoStatus":
      return {
        currentBranch: "main",
        trackingBranch: "origin/main",
        ahead: 1,
        behind: 0,
        isClean: false,
        headCommit: "a1b2c3d4e5f60718293a4b5c6d7e8f9012345678",
        isInitialCommit: false,
        totalCommits: 42,
        stagedFiles: [
          {
            path: "src/App.vue",
            stagedStatus: "M",
            unstagedStatus: "",
            isStaged: true,
            isLfs: false,
            additions: 42,
            deletions: 12,
            size: "3.2 KB",
          },
        ],
        unstagedFiles: [
          {
            path: "vite.config.ts",
            stagedStatus: "",
            unstagedStatus: "M",
            isStaged: false,
            isLfs: false,
            additions: 5,
            deletions: 1,
            size: "380 B",
          },
          {
            path: "src/components/WorkspaceView.vue",
            stagedStatus: "",
            unstagedStatus: "?",
            isStaged: false,
            isLfs: false,
            additions: 120,
            deletions: 0,
            size: "4.8 KB",
          },
        ],
        untrackedFiles: [],
        conflictedFiles: [],
        isMerging: false,
        isRebasing: false,
        isCherryPicking: false,
      } as unknown as T;
    case "GetProjectSourceFiles":
      return [
        { path: "src/App.vue", name: "App.vue", size: 3200, status: "staged", isBinary: false },
        { path: "src/main.ts", name: "main.ts", size: 850, status: "clean", isBinary: false },
        { path: "src/style.css", name: "style.css", size: 1200, status: "clean", isBinary: false },
        { path: "vite.config.ts", name: "vite.config.ts", size: 380, status: "modified", isBinary: false },
        { path: "main.go", name: "main.go", size: 1100, status: "clean", isBinary: false },
        { path: "git_service.go", name: "git_service.go", size: 145000, status: "clean", isBinary: false },
        { path: "git_parser.go", name: "git_parser.go", size: 98000, status: "clean", isBinary: false },
        { path: "draft-notes.md", name: "draft-notes.md", size: 640, status: "untracked", isBinary: false },
      ] as unknown as T;
    case "GetSourceFileDetail":
      return {
        path: args[1] || "src/App.vue",
        name: "App.vue",
        size: 3200,
        sizeFormatted: "3.2 KB",
        lineCount: 45,
        language: "vue",
        content: `<template>\n  <div class="h-screen flex flex-col bg-canvas">\n    <HeaderBar />\n  </div>\n</template>`,
        isBinary: false,
        lastCommitHash: "c0ffee42",
        lastAuthor: "sunmking",
        lastDate: "2026-10-19 16:00",
        lastSubject: "feat: modern Vue 3 studio layout",
      } as unknown as T;
    case "GetWorkingDiff":
      return `--- a/${args[1] || "file.ts"}\n+++ b/${args[1] || "file.ts"}\n@@ -1,5 +1,8 @@\n import { ref } from "vue";\n+import { LucideIcon } from "lucide-vue-next";\n \n-export const count = 0;\n+export const count = ref(1);\n+// Ultra-modern reactive Git studio\n` as unknown as T;
    case "GetCommits":
      return mockCommitList().slice(0, Number(args[1] ?? 150) || 150) as unknown as T;
    case "GetCommitCount":
      return 2302 as unknown as T;
    case "GetCommitsFiltered": {
      const skip = Number(args[6] ?? args[1] ?? 0) || 0;
      const limit = Number(args[7] ?? args[2] ?? 50) || 50;
      const base = mockCommitList();
      return {
        commits: base.slice(skip, skip + limit),
        totalCount: base.length,
        hasMore: skip + limit < base.length,
      } as unknown as T;
    }
    case "GetCommitDetail": {
      const list = mockCommitList();
      const target = list.find((c) => c.hash === args[1]) || list[0]!;
      return {
        commit: target,
        files: [
          { path: "frontend/src/components/HistoryView.vue", stagedStatus: "M", unstagedStatus: "", isStaged: true, isLfs: false, additions: 128, deletions: 12, size: "18 KB" },
          { path: "git_large_repo.go", stagedStatus: "A", unstagedStatus: "", isStaged: true, isLfs: false, additions: 420, deletions: 0, size: "12 KB" },
        ],
        additions: 548,
        deletions: 12,
        diff: "diff --git a/example b/example\n+示例差异\n",
        diffTruncated: false,
      } as unknown as T;
    }
    // Health-report preview: same shape as the Go payload (text + key + params).
    case "RunRepoHealthCheck":
      return {
        score: 82,
        grade: "B",
        passCount: 4,
        warnCount: 2,
        failCount: 0,
        items: [
          {
            key: "identity",
            title: "提交身份",
            titleKey: "health.dim.identity.title",
            icon: "🪪",
            status: "pass",
            score: 100,
            detail: "sunmking <sunmking@studio.dev>，提交作者信息完整",
            detailKey: "health.identity.detail.ok",
            params: { name: "sunmking", email: "sunmking@studio.dev" },
            action: "settings",
          },
          {
            key: "objects",
            title: "对象数据库",
            titleKey: "health.dim.objects.title",
            icon: "🧬",
            status: "pass",
            score: 100,
            detail: "对象完整性与连通性检查通过",
            detailKey: "health.objects.detail.ok",
            action: "fsck",
          },
          {
            key: "ignore",
            title: "忽略规则",
            titleKey: "health.dim.ignore.title",
            icon: "🚫",
            status: "warn",
            score: 78,
            detail: "Unity · 健康度 78 分，建议补齐 2 条规则",
            detailKey: "health.ignore.detail.base",
            detailSuffixKeys: ["health.ignore.detail.suffix.missing"],
            params: { engine: "Unity", score: "78", count: "2" },
            action: "ignore",
          },
          {
            key: "largefiles",
            title: "大文件与 LFS",
            titleKey: "health.dim.largefiles.title",
            icon: "💾",
            status: "pass",
            score: 100,
            detail: "未发现超过 10MB 的未托管大文件（LFS 已接管 12 个资产）",
            detailKey: "health.largefiles.detail.ok",
            params: { count: "12" },
            action: "lfs",
          },
          {
            key: "signing",
            title: "提交签名",
            titleKey: "health.dim.signing.title",
            icon: "✍️",
            status: "warn",
            score: 65,
            detail: "未启用提交签名，无法校验提交来源真实性",
            detailKey: "health.signing.detail.disabled",
            action: "signing",
          },
          {
            key: "workspace",
            title: "工作区状态",
            titleKey: "health.dim.workspace.title",
            icon: "🧯",
            status: "pass",
            score: 85,
            detail: "无冲突；工作区有 3 个待处理改动",
            detailKey: "health.workspace.detail.dirty",
            params: { count: "3" },
            action: "conflict",
          },
        ],
        summary: "六项体检完成：4 项通过、2 项待优化、0 项异常，综合健康度 82 分。",
        summaryKey: "health.summary.mixed",
        summaryParams: { pass: "4", warn: "2", fail: "0", score: "82" },
        checkedAt: "2026-10-24 19:40:00",
        repoName: "wgit",
      } as unknown as T;
    // Fix-center preview: mirrors the Go payload (text + key + params) so the
    // browser build exercises the same localization path as the desktop app.
    case "ScanFixCenter":
      return {
        items: [
          {
            id: "ignore-rules",
            title: "忽略规则缺口",
            titleKey: "fix.item.ignore.title",
            severity: "warn",
            count: 3,
            summary: "Unity · 健康度 72：缺 2 条规则，1 个构建产物被误跟踪。",
            summaryKey: "fix.item.ignore.summary",
            params: { engine: "Unity", score: "72", missing: "2", tracked: "1" },
            fixable: true,
            fixLabel: "补全规则并取消跟踪",
            fixLabelKey: "fix.item.ignore.fix",
            targets: [],
            needsReview: false,
          },
          {
            id: "staged-large-lfs",
            title: "暂存区未走 LFS 的大文件",
            titleKey: "fix.item.large.title",
            severity: "warn",
            count: 2,
            summary: "2 个文件超过 10MB 且未由 LFS 跟踪，合计 84.2 MB。",
            summaryKey: "fix.item.large.summary",
            params: { count: "2", size: "84.2 MB" },
            fixable: true,
            fixLabel: "全部转为 LFS",
            fixLabelKey: "fix.item.large.fix",
            targets: ["assets/city.fbx", "assets/hero.psd"],
            needsReview: false,
          },
        ],
        totalFixes: 2,
        scannedAt: "2026-10-24 10:00",
        message: "发现 2 类可修复问题。改写 .gitignore/.gitattributes 的修复会先备份到 .git/wgit-fix-backup/。",
        messageKey: "fix.message.found",
        messageParams: { count: "2" },
        analysisComplete: true,
        warnings: [],
      } as unknown as T;
    case "GetBranchInsights": {
      const items = [
        { name: "main", isCurrent: true, isRemote: false, shortHash: "a1b2c3d", lastAuthor: "sunmking", lastDate: "2026-10-20", lastSubject: "feat: 大型项目支持", ahead: 2, behind: 0, hasUpstream: true, upstream: "origin/main", merged: false, daysSinceCommit: 0, category: "current", suggestion: "当前分支" },
        { name: "feature/large-repo", isCurrent: false, isRemote: false, shortHash: "b2c3d4e", lastAuthor: "sunmking", lastDate: "2026-10-19", lastSubject: "feat: monorepo tools", ahead: 5, behind: 1, hasUpstream: true, upstream: "origin/feature/large-repo", merged: false, daysSinceCommit: 1, category: "active", suggestion: "活跃开发中" },
        { name: "feature/old-hud", isCurrent: false, isRemote: false, shortHash: "c3d4e5f", lastAuthor: "alice", lastDate: "2026-08-01", lastSubject: "feat: old hud", ahead: 0, behind: 42, hasUpstream: false, upstream: "", merged: true, daysSinceCommit: 81, category: "merged", suggestion: "已合入基准，可安全删除" },
        { name: "bugfix/stale-branch", isCurrent: false, isRemote: false, shortHash: "d4e5f60", lastAuthor: "bob", lastDate: "2026-07-01", lastSubject: "fix: stale", ahead: 3, behind: 120, hasUpstream: false, upstream: "", merged: false, daysSinceCommit: 112, category: "stale", suggestion: "长期未活动" },
      ];
      return {
        base: "main",
        items,
        summary: { total: items.length, current: "main", merged: 1, unpushed: 1, stale: 1, cleanable: 1, staleDays: 90, baseBranch: "main", hint: "已合入且长期未动的分支建议清理" },
      } as unknown as T;
    }
    case "GetMergedBranches":
      return ["feature/old-hud", "feature/legacy-ui"] as unknown as T;
    case "DeleteMergedBranches":
      return ((args[1] as string[] | undefined) || []) as unknown as T;
    case "SetUpstream":
    case "UnsetUpstream":
    case "DeleteRemoteBranch":
      return null as unknown as T;
    case "SearchProjectFiles": {
      const query = String(args[1] ?? "").toLowerCase();
      const files = mockProjectFiles();
      const matched = query
        ? files.filter((f) => f.path.toLowerCase().includes(query) || f.name.toLowerCase().includes(query))
        : files;
      const limit = Number(args[2] ?? 80) || 80;
      return matched.slice(0, limit) as unknown as T;
    }
    case "SearchRepoCode": {
      const query = String(args[1] ?? "");
      if (!query) return [] as unknown as T;
      return [
        { path: "frontend/src/components/WorkspaceView.vue", lineNumber: 512, lineContent: `const searchQuery = ref("${query}");` },
        { path: "frontend/src/components/HistoryView.vue", lineNumber: 66, lineContent: `const historyQuery = "${query}";` },
        { path: "git_large_repo.go", lineNumber: 240, lineContent: `// SearchProjectFiles fuzzy search for ${query}` },
      ].slice(0, Number(args[3] ?? 200) || 200) as unknown as T;
    }
    case "CloneRepoAdvanced":
      return {
        path: (args[0] as any)?.targetPath || "/Users/sunmking/workplace/cloned-repo",
        name: "cloned-repo",
        currentBranch: "main",
        isClean: true,
        remotes: ["origin"],
      } as unknown as T;
    case "PickFolderDialog":
      return "/Users/sunmking/workplace" as unknown as T;
    case "InitRepo":
      return {
        path: args[0] || "/Users/sunmking/workplace/new-repo",
        name: "new-repo",
        currentBranch: "main",
        isClean: true,
        remotes: [],
      } as unknown as T;
    case "PruneRemote":
      return `已清理远程 [${args[1] || "origin"}] 的失效分支引用（prune 完成）` as unknown as T;
    case "TestRemoteConnection":
      return { remote: args[1] || "origin", success: true, latency: "42ms", heads: ["main", "dev", "release"] } as unknown as T;
    case "GetMultiRemoteStatus":
      return {
        currentBranch: "main",
        remotes: [
          { name: "origin", fetchUrl: "git@github.com:org/mono.git", pushUrl: "git@github.com:org/mono.git", branch: "main", ahead: 2, behind: 0 },
          { name: "upstream", fetchUrl: "git@github.com:upstream/mono.git", pushUrl: "git@github.com:upstream/mono.git", branch: "main", ahead: 0, behind: 15 },
        ],
      } as unknown as T;
    case "SyncMultiRemotes":
      return { action: args[1] || "fetch-all", success: true, messages: ["已成功从所有远端 (Fetch All) 获取最新分支与标签索引。"] } as unknown as T;
    case "InteractiveRebasePlan": {
      const upstream = String(args[1] ?? "origin/main");
      const commits = mockCommitList().slice(0, 4).reverse();
      return commits.map((c, i) => ({
        action: i === 1 ? "reword" : i === 3 ? "squash" : "pick",
        hash: c.hash,
        shortHash: c.shortHash,
        subject: c.subject,
        author: c.authorName,
      })) as unknown as T & { upstream?: string };
    }
    case "InteractiveRebaseExecute":
      return "Successfully rebased and updated refs/heads/main.（mock）" as unknown as T;
    case "CherryPickMultiple": {
      const hashes = (args[1] as string[] | undefined) || [];
      return {
        successCount: hashes.length,
        totalCount: hashes.length,
        failedCommitHash: "",
        errorMessage: "",
        hasConflict: false,
        appliedHashes: hashes,
      } as unknown as T;
    }
    case "PullAdvanced":
      return { success: true, message: "Already up to date.", updated: false, alreadyUp: true, fastForward: false, hasConflicts: false } as unknown as T;
    case "GetAheadCommits":
      return mockCommitList().slice(0, 3) as unknown as T;
    case "GetBehindCommits":
      return mockCommitList().slice(3, 5) as unknown as T;
    case "PushAdvanced":
      return { success: true, message: "Everything up-to-date" } as unknown as T;
    case "PreviewMerge":
      return {
        canMergeCleanly: true,
        hasConflicts: false,
        conflicts: [],
        changedFiles: ["frontend/src/App.vue", "git_service.go"],
        aheadCount: 2,
        behindCount: 0,
        baseCommit: "a1b2c3d",
        targetCommit: "d4e5f6a",
        details: "可以干净合并，无任何代码冲突。",
      } as unknown as T;
    case "MergeAdvanced":
      return { success: true, message: "Merge completed", hasConflicts: false } as unknown as T;
    case "RebaseAdvanced":
      return { success: true, message: "Rebase completed", hasConflicts: false } as unknown as T;
    case "GetRebaseProgress":
      return {
        isRebasing: false,
        headName: "feature",
        onto: "main",
        currentStep: 0,
        totalSteps: 0,
        currentCommit: "",
        hasConflicts: false,
        conflictedFiles: [],
      } as unknown as T;
    case "AbortConflictOperation":
      return "操作已放弃，工作区已恢复。" as unknown as T;
    case "ContinueConflictOperation":
      return "操作已继续执行。" as unknown as T;
    case "SkipConflictOperation":
      return "已跳过该提交。" as unknown as T;
    case "CherryPickRange":
      return "已成功 Cherry-Pick 所选区间提交（mock）" as unknown as T;
    case "BisectStatus":
      return mockBisectStatus() as unknown as T;
    case "BisectStart":
      mockBisectActive = true;
      mockBisectJudged = 0;
      return mockBisectStatus() as unknown as T;
    case "BisectGood":
    case "BisectBad":
      if (mockBisectActive) mockBisectJudged = Math.min(mockBisectJudged + 1, MOCK_BISECT_HASHES.length - 1);
      return mockBisectStatus() as unknown as T;
    case "BisectReset":
      mockBisectActive = false;
      mockBisectJudged = 0;
      return null as unknown as T;
    case "BisectRun": {
      mockBisectActive = true;
      mockBisectJudged = MOCK_BISECT_HASHES.length - 1;
      const st = mockBisectStatus();
      return {
        success: true,
        firstBadCommit: MOCK_BISECT_HASHES[MOCK_BISECT_HASHES.length - 1],
        log: "git bisect run automated test succeeded\n" + st.log,
        exitCode: 0,
        message: "Bisect 自动测试完成",
        status: st,
      } as unknown as T;
    }
    case "GetChangeReview":
      mockReview = makeMockReview(String(args[1] ?? "origin/main"), String(args[2] ?? "HEAD"));
      return mockReview as unknown as T;
    case "GetSafeDirectories":
      return ["/Users/sunmking/workplace/wgit", "/tmp/shared-repo"] as unknown as T;
    case "AddSafeDirectory":
    case "RemoveSafeDirectory":
      return null as unknown as T;
    case "CheckSafeDirectory":
      return true as unknown as T;
    case "ListChangeReviews":
      return [
        { key: mockReview.key, base: mockReview.base, head: mockReview.head, updatedAt: mockReview.updatedAt, total: 3, approved: 1, changes: 1, pending: 1, comments: 2, openComments: 1, additions: 548, deletions: 12, ready: false, verdict: mockReview.stats.verdict, verdictDetail: mockReview.stats.verdictDetail },
        { key: "v1.0.0...HEAD", base: "v1.0.0", head: "HEAD", updatedAt: "2026-10-20 22:10", total: 5, approved: 5, changes: 0, pending: 0, comments: 3, openComments: 0, additions: 1024, deletions: 88, ready: true, verdict: "ready", verdictDetail: "全部文件已通过，可合并" },
      ] as unknown as T;
    case "SetReviewFileState": {
      const p = String(args[3] ?? "");
      const status = String(args[4] ?? "pending");
      const note = String(args[5] ?? "");
      mockReview.files = mockReview.files.map((f) => (f.path === p ? { ...f, status, note, updatedAt: "2026-10-21 00:40" } : f));
      return mockReview as unknown as T;
    }
    case "AddReviewComment": {
      const c = {
        id: "c" + (mockReview.comments.length + 1),
        path: String(args[3] ?? ""),
        line: Number(args[4] ?? 1),
        side: String(args[5] ?? "new"),
        body: String(args[6] ?? ""),
        author: "sunmking",
        createdAt: "2026-10-21 00:41",
        resolved: false,
      };
      mockReview.comments = [...mockReview.comments, c];
      return mockReview as unknown as T;
    }
    case "ResolveReviewComment": {
      const id = String(args[3] ?? "");
      const resolved = Boolean(args[4]);
      mockReview.comments = mockReview.comments.map((c) => (c.id === id ? { ...c, resolved } : c));
      return mockReview as unknown as T;
    }
    case "ClearChangeReview":
      return "已清除该评审会话（mock）" as unknown as T;
    case "ExportChangeReviewMarkdown":
      return "# 变更评审报告\n\n- 基准: origin/main\n- HEAD\n\n## 文件结论\n\n| 文件 | 结论 | 备注 |\n| --- | --- | --- |\n| HistoryView.vue | 通过 | 分页与筛选逻辑已复核 |\n| git_large_repo.go | 需修改 | 补充中文路径测试 |\n" as unknown as T;
    case "GetReviewFileDiff":
      return "diff --git a/frontend/src/components/HistoryView.vue b/frontend/src/components/HistoryView.vue\n@@ -1,3 +1,5 @@\n <script setup lang=\"ts\">\n+import { nextTick } from \"vue\";\n+// 评审 diff 预览（mock）\n" as unknown as T;
    case "GetReleasePlan":
      return {
        branch: "main",
        lastTag: "v2.3.1",
        commitCount: 18,
        aheadOfRemote: 3,
        behindRemote: 0,
        suggestedVersion: "v2.4.0",
        nextMinor: true,
        // Mirrors the Go payload: rendered text plus catalog keys + params, so
        // the browser preview exercises the same localization path.
        checks: [
          {
            key: "commits",
            title: "待发布提交",
            titleKey: "release.check.commits.title",
            status: "pass",
            detail: "自 v2.3.1 以来共 18 个提交",
            detailKey: "release.check.commits.detail.ok",
            params: { base: "v2.3.1", count: "18" },
            blocking: false,
          },
          {
            key: "workspace",
            title: "工作区",
            titleKey: "release.check.workspace.title",
            status: "warn",
            detail: "有 2 个未提交改动，打出的标签不会包含这些内容",
            detailKey: "release.check.workspace.detail.dirty",
            params: { count: "2" },
            blocking: true,
          },
          {
            key: "branch",
            title: "发布分支",
            titleKey: "release.check.branch.title",
            status: "pass",
            detail: "当前位于发布线路 main",
            detailKey: "release.check.branch.detail.ok",
            params: { branch: "main" },
            blocking: false,
          },
          {
            key: "remote",
            title: "远端同步",
            titleKey: "release.check.remote.title",
            status: "pass",
            detail: "本地领先 origin/main 3 个提交，发版后记得推送代码",
            detailKey: "release.check.remote.detail.ahead",
            params: { upstream: "origin/main", count: "3" },
            blocking: false,
          },
          {
            key: "signing",
            title: "提交签名",
            titleKey: "release.check.signing.title",
            status: "warn",
            detail: "未启用提交签名，发布来源不可校验",
            detailKey: "release.check.signing.detail.disabled",
            blocking: false,
          },
        ],
        changelog: "## v2.4.0\n\n### Features\n- 大型项目历史分页与筛选\n- 交互式变基与批量 Cherry-Pick\n\n### Fixes\n- 修复中文路径解析\n",
        ready: false,
        summary: "暂不可发布：有 1 项阻塞检查未通过。",
        summaryKey: "release.summary.blocked",
        summaryParams: { count: "1" },
      } as unknown as T;
    case "CreateRelease":
      return {
        success: true,
        tag: String(args[1] ?? "v2.4.0"),
        pushed: Boolean(args[3]),
        message: "发布标签创建成功（mock）",
        pushOutput: "",
      } as unknown as T;
    case "GenerateStudioChangelog": {
      const items = mockCommitList().slice(0, 9);
      return {
        fromRef: String(args[1] ?? "v2.3.1"),
        toRef: String(args[2] ?? "HEAD"),
        totalCount: 18,
        categories: [
          { category: "Features", icon: "✨", commits: items.slice(0, 5) },
          { category: "Fixes", icon: "🐛", commits: items.slice(5, 8) },
          { category: "Chores", icon: "🧹", commits: items.slice(8, 9) },
        ],
        markdownDoc: "## v2.4.0\n\n### Features\n- 大型项目历史分页与筛选\n- 交互式变基与批量 Cherry-Pick\n\n### Fixes\n- 修复中文路径解析\n\n### Chores\n- README 更新\n",
      } as unknown as T;
    }
    case "GetBranches":
      return [
        { name: "main", isCurrent: true, isRemote: false, upstream: "origin/main", ahead: 1, behind: 0, lastCommitHash: "a1b2c3d", lastCommitSubject: "feat: Vue 3 + Wails v3" },
        { name: "feature/studio-ui", isCurrent: false, isRemote: false, upstream: "", ahead: 0, behind: 0, lastCommitHash: "7890abc", lastCommitSubject: "wip: studio visualizer" },
        { name: "origin/main", isCurrent: false, isRemote: true, upstream: "", ahead: 0, behind: 0, lastCommitHash: "f9e8d7c", lastCommitSubject: "feat(core): 140 轮全能力" },
      ] as unknown as T;
    case "GetTags":
      return [
        { name: "v1.0.0", isAnnotated: true, commitHash: "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0", message: "Release v1.0.0" },
        { name: "v2.3.1", isAnnotated: false, commitHash: "f9e8d7c6b5a4938271605f4e3d2c1b0a9f8e7d6c", message: "hotfix: preview tag" },
      ] as unknown as T;
    case "GetDiffBetween":
      return {
        refA: args[1] || "main",
        refB: args[2] || "HEAD",
        additions: 128,
        deletions: 32,
        files: [
          { path: "src/App.vue", additions: 80, deletions: 12, isLFS: false },
          { path: "docs/screenshots/README.md", additions: 48, deletions: 20, isLFS: false },
        ],
        diff: "--- a/src/App.vue\n+++ b/src/App.vue\n@@ -1,4 +1,4 @@\n-const version = 1;\n+const version = 2;\n // preview compare patch\n",
      } as unknown as T;
    case "GetMergeBaseDiff":
      return {
        refA: args[1] || "main",
        refB: args[2] || "HEAD",
        additions: 96,
        deletions: 18,
        files: [
          { path: "src/features/compare.ts", additions: 96, deletions: 18, isLFS: false },
        ],
        diff: "--- /dev/null\n+++ b/src/features/compare.ts\n@@ -0,0 +1,2 @@\n+export const threeDot = true;\n+// merge-base comparison preview\n",
      } as unknown as T;
    case "GetFileDiffBetween":
      return `--- a/${args[3] || "file.ts"}\n+++ b/${args[3] || "file.ts"}\n@@ -1,3 +1,5 @@\n import { ref } from "vue";\n+// ref-compare preview\n+export const compareMode = ${JSON.stringify(String(args[4] || false))};\n const count = ref(0);\n` as unknown as T;
    case "GetSubmodules":
      return [
        {
          name: "engine-core",
          path: "libs/engine-core",
          url: "https://github.com/example/engine-core.git",
          branch: "main",
          commit: "7e8a9b0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f6a",
          status: "synced",
        },
        {
          name: "shader-lib",
          path: "shaders/common",
          url: "https://github.com/example/shader-lib.git",
          branch: "release/v2",
          commit: "f1e2d3c4b5a60718293a4b5c6d7e8f9012345678",
          status: "modified",
        },
      ] as unknown as T;
    case "GetSparseCheckoutFolders":
      return ["src/components", "frontend/src", "docs"] as unknown as T;
    case "GetRepositoryDiskBreakdown":
      return {
        totalSize: 524288000,
        totalSizeStr: "500.00 MB",
        workingTreeSize: 314572800,
        workingTreeStr: "300.00 MB",
        gitInternalSize: 157286400,
        gitInternalStr: "150.00 MB",
        lfsStoreSize: 52428800,
        lfsStoreStr: "50.00 MB",
        engineCacheSize: 0,
        engineCacheStr: "0 B",
        topFolders: [
          { path: "frontend", size: 209715200, sizeStr: "200.00 MB" },
          { path: "bin", size: 62914560, sizeStr: "60.00 MB" },
          { path: ".git", size: 157286400, sizeStr: "150.00 MB" },
        ],
        assetCategories: [
          { name: "Source Code", size: 209715200, sizeStr: "200 MB", fileCount: 150 },
          { name: "Binaries", size: 62914560, sizeStr: "60 MB", fileCount: 4 },
        ],
        recommendations: [
          "运行 Git GC 深度垃圾回收可收缩 .git 仓库松散对象体积",
          "大型二进制资源建议纳入 Git LFS 跟踪，防止主分支体积膨胀",
        ],
      } as unknown as T;
    case "ScanLargeFiles":
      return [
        { path: "bin/wgit", sizeBytes: 52428800, sizeHuman: "50.0 MB", isLfs: false },
        { path: "assets/textures/hero_albedo.png", sizeBytes: 15728640, sizeHuman: "15.0 MB", isLfs: true },
      ] as unknown as T;
    case "GetMaintenanceStats":
    case "RunMaintenance":
      return {
        gitDirSize: "150.00 MB",
        packFilesCount: 3,
        looseObjects: 42,
      } as unknown as T;
    case "LfsPrune":
      return {
        dryRun: args[1] ?? true,
        output: "Pruning 12 local LFS objects (180.5 MB reclaimed)",
        prunedFiles: 12,
        reclaimedStr: "180.5 MB",
        success: true,
      } as unknown as T;
    case "VerifyRepoHealth":
      return {
        isClean: true,
        danglingBlob: 8,
        danglingTree: 2,
        danglingCommit: 1,
        rawErrors: [],
      } as unknown as T;
    case "GetFileBlame":
    case "GetFileBlameWithOptions":
      return [
        {
          lineNumber: 1,
          commitHash: "a1b2c3d4e5f60718293a4b5c6d7e8f9012345678",
          shortHash: "a1b2c3d",
          author: "sunmking",
          authorEmail: "sunmking@example.com",
          authorTimestamp: 1792725636,
          authorTz: "+0800",
          date: "2026-10-19",
          summary: "feat: modern Vue 3 studio layout",
          content: "// wgit modern repository",
          previousCommit: "9597860bf4ff5d8698d8c1f2efcc909b0ce466d1",
          previousFilename: "main.go",
          isBoundary: false,
        },
      ] as unknown as T;
    case "CompareBranches":
      return {
        baseBranch: args[1] || "main",
        targetBranch: args[2] || "feature",
        aheadCount: 2,
        behindCount: 1,
        aheadCommits: [
          {
            hash: "c1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0",
            shortHash: "c1a2b3c",
            parents: [],
            authorName: "sunmking",
            authorEmail: "sunmking@example.com",
            date: "2026-10-20T10:00:00Z",
            relativeDate: "2 小时前",
            subject: "feat(studio): 新增分支双向比对引擎",
            body: "",
            refs: [],
            isHead: false,
            lane: 0,
            lines: [],
          },
        ],
        behindCommits: [
          {
            hash: "b9a8c7d6e5f4a3b2c1d0e9f8a7b6c5d4e3f2a1b0",
            shortHash: "b9a8c7d",
            parents: [],
            authorName: "collaborator",
            authorEmail: "collab@example.com",
            date: "2026-10-19T18:00:00Z",
            relativeDate: "昨天",
            subject: "chore: 主干依赖版本升级",
            body: "",
            refs: [],
            isHead: false,
            lane: 0,
            lines: [],
          },
        ],
      } as unknown as T;
    case "LockAllModifiedLfs":
      return {
        lockedFiles: ["textures/hero_albedo.png"],
        skippedFiles: [],
        failedFiles: [],
        totalCount: 1,
        message: "成功锁定 1 个改动的 LFS 资产文件",
      } as unknown as T;
    case "DetectExternalTools":
      return [
        {
          id: "cursor",
          name: "Cursor",
          executable: "/Applications/Cursor.app/Contents/Resources/app/bin/cursor",
          isAvailable: true,
          supportsDiff: true,
          supportsMerge: true,
          description: "智能 AI 代码编辑器 (Cursor)，支持左右分栏与三方合并",
        },
        {
          id: "vscode",
          name: "Visual Studio Code",
          executable: "/Applications/Visual Studio Code.app/Contents/Resources/app/bin/code",
          isAvailable: true,
          supportsDiff: true,
          supportsMerge: true,
          description: "主流轻量编辑器，支持原生分屏 Diff 与合并裁决",
        },
        {
          id: "beyondcompare",
          name: "Beyond Compare",
          executable: "/Applications/Beyond Compare.app/Contents/MacOS/bcomp",
          isAvailable: false,
          supportsDiff: true,
          supportsMerge: true,
          description: "工业级专业文件与目录对比合并工具，对游戏重度资产极其友好",
        },
        {
          id: "idea",
          name: "IntelliJ IDEA / JetBrains",
          executable: "idea",
          isAvailable: true,
          supportsDiff: true,
          supportsMerge: true,
          description: "JetBrains 全功能 IDE 差异对比与冲突合并器",
        },
        {
          id: "opendiff",
          name: "FileMerge (opendiff)",
          executable: "/usr/bin/opendiff",
          isAvailable: true,
          supportsDiff: true,
          supportsMerge: true,
          description: "macOS / Xcode 原生内置双栏/三方对比工具",
        },
      ] as unknown as T;
    case "GetCommitSigningStatus":
      return {
        gpgSignEnabled: true,
        signingKey: "ABC123",
        gpgFormat: "ssh",
        availableSshKeys: ["id_ed25519"],
        hasGpgBinary: true,
        recentCommits: [],
        compliantCount: 0,
        totalChecked: 0,
        complianceRate: "0%",
      } as unknown as T;
    case "GetExternalToolConfig":
      return {
        diffTool: "cursor",
        mergeTool: "cursor",
        customDiffCmd: "",
        customMergeCmd: "",
        isGlobal: false,
      } as unknown as T;
    case "GetRemoteWebInfo":
      return {
        hasRemote: true,
        remoteName: "origin",
        remoteUrl: "git@github.com:sunmking/wgit.git",
        provider: "github",
        webBaseUrl: "https://github.com/sunmking/wgit",
        commitUrlTpl: "https://github.com/sunmking/wgit/commit/{hash}",
        branchUrlTpl: "https://github.com/sunmking/wgit/tree/{branch}",
        fileUrlTpl: "https://github.com/sunmking/wgit/blob/{branch}/{path}",
        compareUrlTpl: "https://github.com/sunmking/wgit/compare/{base}...{head}",
      } as unknown as T;
    case "GetNotedCommitHashes":
      return ["c0ffee42", "a1b2c3d4"] as unknown as T;
    case "GetCommitNote":
      return (args[1] === "c0ffee42" ? "QA-PASS: 工业级大型项目全量验收通过，无内存泄漏" : "") as unknown as T;
    case "SaveCommitNote":
      return "已成功保存批注" as unknown as T;
    case "DeleteCommitNote":
      return "已成功删除批注" as unknown as T;
    case "PushCommitNotes":
      return "已将提交批注推送至远端" as unknown as T;
    case "GetGitVersion":
      return { version: "git version 2.47.1 (Apple Git-143)", system: "darwin/arm64" } as unknown as T;
    case "LfsPull":
    case "LfsPush":
      return "" as unknown as T;
    case "ExportCommitReport":
      return "# wgit 提交发布报告\n\n| Commit | 作者 | 日期 | 说明 |\n|---|---|---|---|\n| `a1b2c3d` | sunmking | 2026-10-21 | feat(studio): 大型项目工作区支持 |\n" as unknown as T;
    case "ExportPatchRange":
      return undefined as unknown as T;
    case "GeneratePatchRange":
      return `diff --git a/preview.txt b/preview.txt\n--- a/preview.txt\n+++ b/preview.txt\n@@ -1 +1 @@\n-old\n+new\n// range patch preview (${args[1]}..${args[2]})\n` as unknown as T;
    case "ScanStagedSecrets":
      return [
        {
          filePath: "src/config.ts",
          lineNumber: 12,
          ruleName: "AWS Access Key",
          severity: "CRITICAL",
          redactedSnippet: "AKIA...****",
        },
      ] as unknown as T;
    case "ListSubtrees":
      return {
        subtrees: [
          {
            prefix: "packages/shared-ui",
            remoteOrUrl: "https://github.com/example/shared-ui.git",
            branch: "main",
            lastCommit: "a1b2c3d4",
            lastSquash: "e5f6a7b8c9d01234567890abcdef1234567890ab",
          },
          {
            prefix: "libs/core-utils",
            remoteOrUrl: "https://github.com/example/core-utils.git",
            branch: "main",
            lastCommit: "f8e7d6c5",
            lastSquash: "",
          },
        ],
      } as unknown as T;
    case "AddSubtree":
      return {
        success: true,
        message: `成功将子树 ${args[2] || ""} 集成至目录 ${args[1] || ""}`,
        output: "git subtree add successful (mock)",
      } as unknown as T;
    case "PullSubtree":
      return {
        success: true,
        message: `成功从远端同步子树 ${args[1] || ""} 最新更新`,
        output: "git subtree pull successful (mock)",
      } as unknown as T;
    case "PushSubtree":
      return {
        success: true,
        message: `成功将子树 ${args[1] || ""} 变更推送回上游`,
        output: "git subtree push successful (mock)",
      } as unknown as T;
    case "SplitSubtree":
      return {
        success: true,
        prefix: String(args[1] || ""),
        newCommit: "118d691d63e4348edd60c86452803d929409c5c4",
        branchName: String(args[2] || ""),
        message: `成功将子树 ${args[1] || ""} 拆分为新分支 ${args[2] || "split-branch"} (118d691d)`,
        output: "118d691d63e4348edd60c86452803d929409c5c4",
      } as unknown as T;
    case "UpdateSubtreeMetadata":
      return undefined as unknown as T;
    case "UnlinkSubtree":
      return undefined as unknown as T;
    default:
      return null as unknown as T;
  }
}

export const gitService = {
  // Repository Management
  openRepo: (path: string) => callBackend<RepoInfo>("OpenRepo", path),
  openRepoDialog: () => callBackend<RepoInfo>("OpenRepoDialog"),
  getRecentRepos: () => callBackend<string[]>("GetRecentRepos"),
  initRepo: (path: string) => callBackend<RepoInfo>("InitRepo", path),
  pickFolderDialog: () => callBackend<string>("PickFolderDialog"),
  /** Large-repo clone: full / blobless (--filter=blob:none) / shallow (--depth), branch + submodules. */
  cloneRepoAdvanced: (opts: CloneOptions) => callBackend<RepoInfo>("CloneRepoAdvanced", opts),

  // Status & Workspace
  getRepoStatus: (repoPath: string) => callBackend<RepoStatus>("GetRepoStatus", repoPath),
  getProjectSourceFiles: (repoPath: string) => callBackend<ProjectSourceFileItem[]>("GetProjectSourceFiles", repoPath),
  getSourceFileDetail: (repoPath: string, filePath: string, revision = "") => callBackend<SourceFileDetail>("GetSourceFileDetail", repoPath, filePath, revision),
  getSourceFileDiff: (repoPath: string, filePath: string, revision = "") => callBackend<string>("GetSourceFileDiff", repoPath, filePath, revision),
  getFileContentAtCommit: (repoPath: string, commitHash: string, filePath: string) =>
    callBackend<string>("GetFileContentAtCommit", repoPath, commitHash, filePath),
  getWorkingDiff: (repoPath: string, filePath: string, staged: boolean) => callBackend<string>("GetWorkingDiff", repoPath, filePath, staged),
  getWorkingDiffAdvanced: (repoPath: string, filePath: string, staged: boolean, ignoreWhitespace: boolean) =>
    callBackend<string>("GetWorkingDiffAdvanced", repoPath, filePath, staged, ignoreWhitespace),
  getWorkingWordDiff: (repoPath: string, filePath: string, staged: boolean) =>
    callBackend<string>("GetWorkingWordDiff", repoPath, filePath, staged),
  revertFileToRevision: (repoPath: string, filePath: string, commitHash: string) =>
    callBackend<FileRevertResult | null>("RevertFileToRevision", repoPath, filePath, commitHash),
  getFileRevisionTimeline: (repoPath: string, filePath: string) =>
    callBackend<FileTimelineResult | null>("GetFileRevisionTimeline", repoPath, filePath),
  pickaxeSearch: (
    repoPath: string,
    query: string,
    mode: "content" | "regex" = "content",
    author = "",
    pathFilter = "",
    limit = 20,
    ignoreCase = false
  ) =>
    callBackend<PickaxeResult | null>(
      "PickaxeSearch",
      repoPath,
      query,
      mode,
      author,
      pathFilter,
      limit,
      ignoreCase
    ),

  // Large-repo navigation & search
  /** Fuzzy file search backed by `git ls-files` (server-side, scales to monorepos). */
  searchProjectFiles: (repoPath: string, query: string, limit = 80) =>
    callBackend<ProjectFileMatch[]>("SearchProjectFiles", repoPath, query, limit),
  /** Repo-wide content search backed by `git grep`. */
  searchRepoCode: (repoPath: string, query: string, ignoreCase = true, maxResults = 200) =>
    callBackend<GrepMatchItem[]>("SearchRepoCode", repoPath, query, ignoreCase, maxResults),

  // Stage & Discard
  stageFile: (repoPath: string, filePath: string) => callBackend<void>("StageFile", repoPath, filePath),
  unstageFile: (repoPath: string, filePath: string) => callBackend<void>("UnstageFile", repoPath, filePath),
  stageAll: (repoPath: string) => callBackend<void>("StageAll", repoPath),
  unstageAll: (repoPath: string) => callBackend<void>("UnstageAll", repoPath),
  discardFile: (repoPath: string, filePath: string) => callBackend<void>("DiscardFile", repoPath, filePath),
  // ---- Working tree file management (rm / mv / untrack / .gitignore) ----
  deleteRepoFile: (repoPath: string, filePath: string) =>
    callBackend<void>("DeleteRepoFile", repoPath, filePath),
  renameRepoFile: (repoPath: string, oldPath: string, newPath: string) =>
    callBackend<void>("RenameRepoFile", repoPath, oldPath, newPath),
  untrackRepoFile: (repoPath: string, filePath: string) =>
    callBackend<void>("UntrackRepoFile", repoPath, filePath),
  addGitignorePattern: (repoPath: string, pattern: string) =>
    callBackend<void>("AddGitignorePattern", repoPath, pattern),
  getGitignorePatterns: (repoPath: string) =>
    callBackend<string[]>("GetGitignorePatterns", repoPath),
  stageRepoPath: (repoPath: string, path: string) =>
    callBackend<void>("StageRepoPath", repoPath, path),
  unstageRepoPath: (repoPath: string, path: string) =>
    callBackend<void>("UnstageRepoPath", repoPath, path),
  removeGitignorePattern: (repoPath: string, pattern: string) =>
    callBackend<void>("RemoveGitignorePattern", repoPath, pattern),
  updateGitignorePattern: (repoPath: string, oldPattern: string, newPattern: string) =>
    callBackend<void>("UpdateGitignorePattern", repoPath, oldPattern, newPattern),
  swapGitignorePatterns: (repoPath: string, firstPattern: string, secondPattern: string) =>
    callBackend<void>("SwapGitignorePatterns", repoPath, firstPattern, secondPattern),
  discardUntrackedFile: (repoPath: string, filePath: string) =>
    callBackend<void>("DiscardUntrackedFile", repoPath, filePath),
  stageHunk: (repoPath: string, patch: string) => callBackend<void>("StageHunk", repoPath, patch),
  discardHunk: (repoPath: string, patch: string) => callBackend<void>("DiscardHunk", repoPath, patch),
  stageLines: (repoPath: string, patch: string) => callBackend<void>("StageLines", repoPath, patch),
  discardLines: (repoPath: string, patch: string) => callBackend<void>("DiscardLines", repoPath, patch),

  // Commits & History
  /** @deprecated Kept for backwards compatibility. Use commitAdvanced or fastCommit. */
  commit: (repoPath: string, message: string, amend = false) => callBackend<void>("Commit", repoPath, message, amend),
  commitAdvanced: (repoPath: string, message: string, amend = false, noVerify = false) =>
    callBackend<void>("CommitAdvanced", repoPath, message, amend, noVerify),
  fastCommit: (repoPath: string, message: string) => callBackend<any>("FastCommit", repoPath, message),
  getCommits: (repoPath: string, limit = 100, branch = "") => callBackend<CommitItem[]>("GetCommits", repoPath, limit, branch),
  /** Total commit count for the given branch scope ("" / "HEAD" => --all). */
  getCommitCount: (repoPath: string, branch = "") => callBackend<number>("GetCommitCount", repoPath, branch),
  /** Skip/limit pagination (server-side), for filtered/infinite commit lists. */
  getCommitsFiltered: (
    repoPath: string,
    branch = "",
    author = "",
    since = "",
    until = "",
    keyword = "",
    skip = 0,
    limit = 100
  ) => callBackend<CommitPaginationResult>("GetCommitsFiltered", repoPath, branch, author, since, until, keyword, skip, limit),
  getCommitDetail: async (repoPath: string, commitHash: string): Promise<CommitDetail> => {
    const key = `${repoPath}:${commitHash}`;
    const cached = commitDetailCache.get(key);
    if (cached) return cached;
    const res = await callBackend<CommitDetail>("GetCommitDetail", repoPath, commitHash);
    if (res) commitDetailCache.set(key, res);
    return res;
  },
  clearCommitDetailCache: (): void => {
    commitDetailCache.clear();
  },
  cherryPick: (repoPath: string, commitHash: string) => callBackend<void>("CherryPick", repoPath, commitHash),
  revertCommit: (repoPath: string, commitHash: string) => callBackend<void>("RevertCommit", repoPath, commitHash),
  resetBranch: (repoPath: string, commitHash: string, mode: "soft" | "mixed" | "hard" = "mixed") => callBackend<void>("ResetBranch", repoPath, commitHash, mode),
  /** Batch backport: apply several commits (oldest first) in one operation. */
  cherryPickMultiple: (repoPath: string, commitHashes: string[]) =>
    callBackend<BatchCherryPickResult | null>("CherryPickMultiple", repoPath, commitHashes),
  cherryPickRange: (repoPath: string, startHash: string, endHash: string) =>
    callBackend<string>("CherryPickRange", repoPath, startHash, endHash),

  // Interactive rebase (multi-commit rewrite / squash / drop)
  getRebaseCommits: (repoPath: string, upstream: string) =>
    callBackend<CommitItem[]>("GetRebaseCommits", repoPath, upstream),
  interactiveRebasePlan: (repoPath: string, upstream: string) =>
    callBackend<RebaseTodoItem[]>("InteractiveRebasePlan", repoPath, upstream),
  interactiveRebaseExecute: (repoPath: string, upstream: string, todos: RebaseTodoItem[]) =>
    callBackend<string>("InteractiveRebaseExecute", repoPath, upstream, todos),

  // Git bisect (regression hunting on large histories)
  bisectStatus: (repoPath: string) => callBackend<BisectStatus | null>("BisectStatus", repoPath),
  bisectStart: (repoPath: string, badRef = "HEAD", goodRef = "") =>
    callBackend<BisectStatus | null>("BisectStart", repoPath, badRef, goodRef),
  bisectGood: (repoPath: string) => callBackend<BisectStatus | null>("BisectGood", repoPath),
  bisectBad: (repoPath: string) => callBackend<BisectStatus | null>("BisectBad", repoPath),
  bisectReset: (repoPath: string) => callBackend<void>("BisectReset", repoPath),
  bisectRun: (repoPath: string, scriptCmd: string) =>
    callBackend<BisectRunResult>("BisectRun", repoPath, scriptCmd),

  // Branches & Merging
  getBranches: (repoPath: string) => callBackend<BranchInfo[]>("GetBranches", repoPath),
  checkoutBranch: (repoPath: string, branchName: string, createNew = false) => callBackend<void>("CheckoutBranch", repoPath, branchName, createNew),
  createBranch: (repoPath: string, branchName: string, startPoint = "") => callBackend<void>("CreateBranch", repoPath, branchName, startPoint),
  deleteBranch: (repoPath: string, branchName: string, force = false) => callBackend<void>("DeleteBranch", repoPath, branchName, force),
  deleteRemoteBranch: (repoPath: string, remoteOrBranch: string, branchName?: string) => {
    let remote = remoteOrBranch;
    let branch = branchName ?? "";
    if (!branch) {
      const slashIdx = remoteOrBranch.indexOf("/");
      if (slashIdx > 0) {
        remote = remoteOrBranch.slice(0, slashIdx);
        branch = remoteOrBranch.slice(slashIdx + 1);
      } else {
        remote = "origin";
        branch = remoteOrBranch;
      }
    }
    return callBackend<void>("DeleteRemoteBranch", repoPath, remote, branch);
  },
  resolveConflict: (repoPath: string, filePath: string, resolution: "ours" | "theirs" | "backup_and_theirs" | "mark_resolved" | "mergetool") =>
    callBackend<void>("ResolveConflict", repoPath, filePath, resolution),
  autoResolveMetaConflicts: (
    repoPath: string,
    strategy: "theirs" | "ours" | "backup_theirs" = "theirs"
  ) =>
    callBackend<MetaConflictResolutionResult>(
      "AutoResolveMetaConflicts",
      repoPath,
      strategy
    ),
  generateLfsLockNotification: (
    repoPath: string,
    filePath: string,
    note = ""
  ) =>
    callBackend<LfsLockNotificationTemplate>(
      "GenerateLfsLockNotification",
      repoPath,
      filePath,
      note
    ),
  resolveConflictBlock: (
    repoPath: string,
    filePath: string,
    blockIndex: number,
    choice: "ours" | "theirs" | "both"
  ) =>
    callBackend<ConflictBlockResolveResult>(
      "ResolveConflictBlock",
      repoPath,
      filePath,
      blockIndex,
      choice
    ),
  resolveConflictWithCustomContent: (
    repoPath: string,
    filePath: string,
    content: string
  ) =>
    callBackend<ConflictBlockResolveResult>(
      "ResolveConflictWithCustomContent",
      repoPath,
      filePath,
      content
    ),
  getConflictFileContent: (repoPath: string, filePath: string) => callBackend<string>("GetConflictFileContent", repoPath, filePath),
  renameBranch: (repoPath: string, oldName: string, newName: string) => callBackend<void>("RenameBranch", repoPath, oldName, newName),
  renameBranchWithRemote: (repoPath: string, oldName: string, newName: string, updateRemote: boolean) =>
    callBackend<void>("RenameBranchWithRemote", repoPath, oldName, newName, updateRemote),
  squashCommits: (repoPath: string, commitCount: number, message: string) =>
    callBackend<string>("SquashCommits", repoPath, commitCount, message),
  revertNoCommit: (repoPath: string, commitHash: string) =>
    callBackend<string>("RevertNoCommit", repoPath, commitHash),
  checkoutCommitDetached: (repoPath: string, commitHash: string) =>
    callBackend<void>("CheckoutCommitDetached", repoPath, commitHash),
  syncSubmodule: (repoPath: string, submodulePath: string) =>
    callBackend<string>("SyncSubmodule", repoPath, submodulePath),
  clearGitProxy: (repoPath: string, isGlobal = false) =>
    callBackend<void>("ClearGitProxy", repoPath, isGlobal),
  stashStagedOnly: (repoPath: string, message = "") =>
    callBackend<string>("StashStagedOnly", repoPath, message),
  discardAll: (repoPath: string) => callBackend<void>("DiscardAll", repoPath),
  autoLockDccAsset: (repoPath: string, assetPath: string) =>
    callBackend<DCCAutoLockResult | null>("AutoLockDCCAsset", repoPath, assetPath),
  getCommitDetailsBatch: (repoPath: string, commitHashes: string[]) =>
    callBackend<CommitItem[]>("GetCommitDetailsBatch", repoPath, commitHashes),
  lintCommitMessage: (message: string) =>
    callBackend<CommitLintResult | null>("LintCommitMessage", message),
  demangleCrashStack: (repoPath: string, rawCrashDump: string, targetCommit = "") =>
    callBackend<CrashStackDemangleResult | null>(
      "DemangleCrashStack",
      repoPath,
      rawCrashDump,
      targetCommit
    ),
  generateHotfixManifest: (repoPath: string, baseRef: string, targetRef: string, appVersion: string) =>
    callBackend<HotfixManifestResult | null>(
      "GenerateHotfixManifest",
      repoPath,
      baseRef,
      targetRef,
      appVersion
    ),
  auditSymbolsAndDefines: (repoPath: string, baseBranch: string, targetBranch: string) =>
    callBackend<SymbolsDefinesAuditResult | null>(
      "AuditSymbolsAndDefines",
      repoPath,
      baseBranch,
      targetBranch
    ),
  inspectShaderVariants: (repoPath: string, targetPath: string) =>
    callBackend<ShaderInspectResult | null>("InspectShaderVariants", repoPath, targetPath),
  auditDataTableDiff: (repoPath: string, targetTable: string, baseRef: string, targetRef: string) =>
    callBackend<DataTableAuditResult | null>(
      "AuditDataTableDiff",
      repoPath,
      targetTable,
      baseRef,
      targetRef
    ),
  cleanGameEngineCaches: (repoPath: string, engineType: string) =>
    callBackend<UnrealCacheCleanResult | null>("CleanGameEngineCaches", repoPath, engineType),
  cleanUnrealEngineCache: (repoPath: string) =>
    callBackend<UnrealCacheCleanResult | null>("CleanUnrealEngineCache", repoPath),
  launchGameEngine: (repoPath: string, engineType: string) =>
    callBackend<string>("LaunchGameEngine", repoPath, engineType),
  cherryPickAssetBundle: (
    repoPath: string,
    targetRef: string,
    assetPaths: string[],
    resolveDependencies = true,
    dryRun = true
  ) =>
    callBackend<AssetCherryPickResult | null>(
      "CherryPickAssetBundle",
      repoPath,
      targetRef,
      assetPaths,
      resolveDependencies,
      dryRun
    ),
  diffUnrealAssetNodes: (repoPath: string, assetPath: string, baseRef: string, targetRef: string) =>
    callBackend<UnrealAssetDiffResult | null>(
      "DiffUnrealAssetNodes",
      repoPath,
      assetPath,
      baseRef,
      targetRef
    ),
  detectUnrealProject: (repoPath: string) =>
    callBackend<UnrealProjectInfo | null>("DetectUnrealProject", repoPath),
  auditSceneStreamMerge: (repoPath: string, targetBranch: string, scenePath = "") =>
    callBackend<SceneStreamAuditResult | null>("AuditSceneStreamMerge", repoPath, targetBranch, scenePath),
  resolveSceneStreamConflict: (
    repoPath: string,
    scenePath: string,
    action: string,
    actorGuid: string
  ) =>
    callBackend<SceneStreamResolveResult | null>(
      "ResolveSceneStreamConflict",
      repoPath,
      scenePath,
      action,
      actorGuid
    ),
  auditFontLicenses: (repoPath: string) =>
    callBackend<FontLicenseAuditResult | null>("AuditFontLicenses", repoPath),
  auditLightmapBakeSettings: (repoPath: string, targetLevel = "") =>
    callBackend<LightmapBakeAuditResult | null>("AuditLightmapBakeSettings", repoPath, targetLevel),
  auditTextureChannelPacking: (repoPath: string, targetDir = "") =>
    callBackend<TextureChannelPackingResult | null>(
      "AuditTextureChannelPacking",
      repoPath,
      targetDir
    ),
  detectAssetCyclicDependencies: (repoPath: string, targetDir = "") =>
    callBackend<AssetCycleDetectResult | null>("DetectAssetCyclicDependencies", repoPath, targetDir),
  traceHotfixBackportStatus: (repoPath: string, hotfixBranch: string, targetBranch: string) =>
    callBackend<HotfixBackportResult | null>(
      "TraceHotfixBackportStatus",
      repoPath,
      hotfixBranch,
      targetBranch
    ),
  analyzeBundleDiff: (repoPath: string, baseManifest: string, targetManifest: string) =>
    callBackend<BundleDiffResult | null>(
      "AnalyzeBundleDiff",
      repoPath,
      baseManifest,
      targetManifest
    ),
  auditAudioBankDiff: (
    repoPath: string,
    targetPath: string,
    baseRef: string,
    targetRef: string
  ) =>
    callBackend<AudioBankDiffResult | null>(
      "AuditAudioBankDiff",
      repoPath,
      targetPath,
      baseRef,
      targetRef
    ),
  diffWorldDataLayers: (repoPath: string, baseRef: string, targetRef: string) =>
    callBackend<WorldDataLayersDiffResult | null>(
      "DiffWorldDataLayers",
      repoPath,
      baseRef,
      targetRef
    ),
  generateOfflinePatchBundle: (
    repoPath: string,
    baseRef: string,
    targetRef: string,
    outputPath: string
  ) =>
    callBackend<OfflinePatchBundleResult | null>(
      "GenerateOfflinePatchBundle",
      repoPath,
      baseRef,
      targetRef,
      outputPath
    ),
  shaderSelfHealingWizard: (repoPath: string, action: string) =>
    callBackend<ShaderSelfHealingResult | null>("ShaderSelfHealingWizard", repoPath, action),
  artistOneClickPipeline: (repoPath: string, commitMessage: string, targetBranch: string) =>
    callBackend<ArtistPipelineResult | null>(
      "ArtistOneClickPipeline",
      repoPath,
      commitMessage,
      targetBranch
    ),
  setUpstream: (repoPath: string, branch: string, upstream: string) => callBackend<void>("SetUpstream", repoPath, branch, upstream),
  unsetUpstream: (repoPath: string, branchName: string) => callBackend<void>("UnsetUpstream", repoPath, branchName),
  /** Branch lifecycle insights: merged / unpushed / stale / active categories vs a base branch. */
  getBranchInsights: (repoPath: string, base = "") => callBackend<BranchInsightsResult | null>("GetBranchInsights", repoPath, base),
  getMergedBranches: (repoPath: string, targetBranch = "") => callBackend<string[]>("GetMergedBranches", repoPath, targetBranch),
  deleteMergedBranches: (repoPath: string, branches: string[]) => callBackend<string[]>("DeleteMergedBranches", repoPath, branches),
  stashBranch: (repoPath: string, index: number, branchName: string) => callBackend<void>("StashBranch", repoPath, index, branchName),

  // Tags
  getTags: (repoPath: string) => callBackend<TagInfo[]>("GetTags", repoPath),
  createTag: (repoPath: string, tagName: string, commitHash = "", message = "") => callBackend<void>("CreateTag", repoPath, tagName, commitHash, message),
  createAnnotatedTag: (repoPath: string, name: string, message: string, sign: boolean) =>
    callBackend<void>("CreateAnnotatedTag", repoPath, name, message, sign),
  getTagDetail: (repoPath: string, tagName: string) =>
    callBackend<TagDetail | null>("GetTagDetail", repoPath, tagName),
  pushTag: (repoPath: string, remote: string, tagName: string) =>
    callBackend<string>("PushTag", repoPath, remote, tagName),
  deleteTag: (repoPath: string, tagName: string) => callBackend<void>("DeleteTag", repoPath, tagName),
  deleteBatchTags: (repoPath: string, tags: string[], deleteRemote = false, remote = "origin") =>
    callBackend<string[]>("DeleteBatchTags", repoPath, tags, deleteRemote, remote),
  deleteRemoteTag: (repoPath: string, remote: string, tagName: string) =>
    callBackend<string>("DeleteRemoteTag", repoPath, remote, tagName),

  // Stashes
  getStashes: (repoPath: string) => callBackend<StashInfo[]>("GetStashes", repoPath),
  getStashDiff: (repoPath: string, index = 0) => callBackend<string>("GetStashDiff", repoPath, index),
  getStashFiles: (repoPath: string, index = 0) => callBackend<StashFileEntry[]>("GetStashFiles", repoPath, String(index)),
  applyFileFromStash: (repoPath: string, index = 0, filePath: string) =>
    callBackend<void>("ApplyFileFromStash", repoPath, `stash@{${index}}`, filePath),
  /** @deprecated Kept for backwards compatibility. Use stashSaveAdvanced. */
  stashSave: (repoPath: string, message = "", keepIndex = false, includeUntracked = false) => callBackend<void>("StashSave", repoPath, message, keepIndex, includeUntracked),
  stashSaveAdvanced: (repoPath: string, message = "", keepIndex = false, includeUntracked = false, includeAll = false) =>
    callBackend<void>("StashSaveAdvanced", repoPath, message, keepIndex, includeUntracked, includeAll),
  saveStashWithFiles: (repoPath: string, message: string, files: string[]) =>
    callBackend<void>("SaveStashWithFiles", repoPath, message, files),
  stashApply: (repoPath: string, index = 0) => callBackend<void>("StashApply", repoPath, index),
  stashPop: (repoPath: string, index = 0) => callBackend<void>("StashPop", repoPath, index),
  stashDrop: (repoPath: string, index = 0) => callBackend<void>("StashDrop", repoPath, index),
  stashClear: (repoPath: string) => callBackend<void>("StashClear", repoPath),

  // Reflog & Recovery
  getReflog: (repoPath: string, limit = 50) => callBackend<ReflogItem[]>("GetReflog", repoPath, limit),
  checkoutReflog: (repoPath: string, selector: string) => callBackend<void>("CheckoutReflog", repoPath, selector),
  createBranchFromReflog: (repoPath: string, branchName: string, commitHash: string) => callBackend<void>("CreateBranchFromReflog", repoPath, branchName, commitHash),

  // Workspace Cleaning
  cleanWorkingTree: (repoPath: string, dryRun = true) => callBackend<string[]>("CleanWorkingTree", repoPath, dryRun),

  // Worktrees
  getWorktrees: (repoPath: string) => callBackend<WorktreeInfo[]>("GetWorktrees", repoPath),
  /** @deprecated Kept for backwards compatibility. Use addWorktreeAdvanced. */
  addWorktree: (repoPath: string, path: string, branch: string, newBranch = false) => callBackend<void>("AddWorktree", repoPath, path, branch, newBranch),
  addWorktreeAdvanced: (repoPath: string, opts: AddWorktreeAdvancedOptions) => callBackend<void>("AddWorktreeAdvanced", repoPath, opts),
  moveWorktree: (repoPath: string, oldPath: string, newPath: string) => callBackend<void>("MoveWorktree", repoPath, oldPath, newPath),
  getWorktreeStatus: (repoPath: string, worktreePath: string) => callBackend<WorktreeStatus>("GetWorktreeStatus", repoPath, worktreePath),
  pruneSingleWorktree: (repoPath: string, worktreePath: string) => callBackend<void>("PruneSingleWorktree", repoPath, worktreePath),
  removeWorktree: (repoPath: string, path: string, force = false) => callBackend<void>("RemoveWorktree", repoPath, path, force),
  lockWorktree: (repoPath: string, worktreePath: string, reason = "") => callBackend<void>("LockWorktree", repoPath, worktreePath, reason),
  unlockWorktree: (repoPath: string, worktreePath: string) => callBackend<void>("UnlockWorktree", repoPath, worktreePath),

  // Git Notes
  getCommitNote: (repoPath: string, commitHash: string) => callBackend<string>("GetCommitNote", repoPath, commitHash),
  saveCommitNote: (repoPath: string, commitHash: string, content: string) => callBackend<string>("SaveCommitNote", repoPath, commitHash, content),
  deleteCommitNote: (repoPath: string, commitHash: string) => callBackend<string>("DeleteCommitNote", repoPath, commitHash),
  getCommitNotes: (repoPath: string, limit = 50) => callBackend<CommitNotesResult | null>("GetCommitNotes", repoPath, limit),
  getCommitSignatureDetails: (repoPath: string, commitHash: string) =>
    callBackend<SignatureDetails | null>("GetCommitSignatureDetails", repoPath, commitHash),
  getNotedCommitHashes: (repoPath: string) => callBackend<string[]>("GetNotedCommitHashes", repoPath),
  pushCommitNotes: (repoPath: string, remote = "") => callBackend<string>("PushCommitNotes", repoPath, remote),

  // Remotes
  getRemoteDetails: (repoPath: string) => callBackend<RemoteDetail[]>("GetRemoteDetails", repoPath),
  addRemote: (repoPath: string, name: string, url: string) => callBackend<void>("AddRemote", repoPath, name, url),
  renameRemote: (repoPath: string, oldName: string, newName: string) =>
    callBackend<void>("RenameRemote", repoPath, oldName, newName),
  setRemoteUrl: (repoPath: string, name: string, newUrl: string) =>
    callBackend<void>("SetRemoteURL", repoPath, name, newUrl),
  removeRemote: (repoPath: string, name: string) => callBackend<void>("RemoveRemote", repoPath, name),
  /** Fetch a specific remote with --prune (large projects usually have several remotes/forks). */
  pruneRemote: (repoPath: string, remoteName = "origin") => callBackend<string>("PruneRemote", repoPath, remoteName),
  testRemoteConnection: (repoPath: string, remote: string) =>
    callBackend<RemoteTestResult | null>("TestRemoteConnection", repoPath, remote),
  getMultiRemoteStatus: (repoPath: string) => callBackend<MultiRemoteStatusResult | null>("GetMultiRemoteStatus", repoPath),
  syncMultiRemotes: (repoPath: string, action: "fetch-all" | "push-all") =>
    callBackend<MultiRemoteSyncResult | null>("SyncMultiRemotes", repoPath, action),

  // Git LFS
  getLfsLocks: (repoPath: string) => callBackend<LfsLockInfo[]>("GetLfsLocks", repoPath),
  getLfsTrackedPatterns: (repoPath: string) => callBackend<LfsTrackPattern[]>("GetLfsTrackedPatterns", repoPath),
  trackLfsPattern: (repoPath: string, pattern: string) => callBackend<void>("TrackLfsPattern", repoPath, pattern),
  untrackLfsPattern: (repoPath: string, pattern: string) => callBackend<void>("UntrackLfsPattern", repoPath, pattern),

  // Remotes & Sync
  fetch: (repoPath: string, remote = "") => callBackend<void>("Fetch", repoPath, remote),
  pull: (repoPath: string, remote = "", branch = "") => callBackend<string>("Pull", repoPath, remote, branch),
  push: (repoPath: string, remote = "", branch = "", force = false) =>
    callBackend<string>("Push", repoPath, remote, branch, force),

  // OS Integration
  openInTerminal: (repoPath: string) => callBackend<void>("OpenInTerminal", repoPath),
  openInFinder: (repoPath: string) => callBackend<void>("OpenInFinder", repoPath),

  // Large-scale Repositories & Submodules
  getSubmodules: (repoPath: string) => callBackend<SubmoduleInfo[]>("GetSubmodules", repoPath),
  updateSubmodules: (repoPath: string, init = true, recursive = true) => callBackend<string>("UpdateSubmodules", repoPath, init, recursive),
  updateSubmodulesRecursive: (repoPath: string) => callBackend<string>("UpdateSubmodulesRecursive", repoPath),
  listSubtrees: (repoPath: string) => callBackend<SubtreeListResult | null>("ListSubtrees", repoPath),
  addSubtree: (repoPath: string, prefix: string, remoteOrUrl: string, branch: string, squash: boolean) =>
    callBackend<SubtreeActionResult | null>("AddSubtree", repoPath, prefix, remoteOrUrl, branch, squash),
  pullSubtree: (repoPath: string, prefix: string, remoteOrUrl: string, branch: string, squash: boolean) =>
    callBackend<SubtreeActionResult | null>("PullSubtree", repoPath, prefix, remoteOrUrl, branch, squash),
  pushSubtree: (repoPath: string, prefix: string, remoteOrUrl: string, branch: string) =>
    callBackend<SubtreeActionResult | null>("PushSubtree", repoPath, prefix, remoteOrUrl, branch),
  splitSubtree: (repoPath: string, prefix: string, branchName = "", squash = false, rejoin = false) =>
    callBackend<SubtreeSplitResult | null>("SplitSubtree", repoPath, prefix, branchName, squash, rejoin),
  updateSubtreeMetadata: (repoPath: string, prefix: string, remoteOrUrl: string, branch: string, squash: boolean) =>
    callBackend<void>("UpdateSubtreeMetadata", repoPath, prefix, remoteOrUrl, branch, squash),
  unlinkSubtree: (repoPath: string, prefix: string) =>
    callBackend<void>("UnlinkSubtree", repoPath, prefix),
  updateSubmoduleRemote: (repoPath: string, submodulePath = "") => callBackend<string>("UpdateSubmoduleRemote", repoPath, submodulePath),
  addSubmodule: (repoPath: string, url: string, path: string) => callBackend<void>("AddSubmodule", repoPath, url, path),
  removeSubmodule: (repoPath: string, path: string) => callBackend<void>("RemoveSubmodule", repoPath, path),
  setSubmoduleBranch: (repoPath: string, submodulePath: string, branch: string) => callBackend<void>("SetSubmoduleBranch", repoPath, submodulePath, branch),

  // Sparse Checkout (Monorepo)
  getSparseCheckoutFolders: (repoPath: string) => callBackend<string[]>("GetSparseCheckoutFolders", repoPath),
  setSparseCheckoutFolders: (repoPath: string, folders: string[]) => callBackend<void>("SetSparseCheckoutFolders", repoPath, folders),
  getSparsePresets: (repoPath: string) => callBackend<SparsePreset[]>("GetSparsePresets", repoPath),
  applyRoleSparsePreset: (repoPath: string, presetId: string, extraPatterns: string[] = []) =>
    callBackend<SparseApplyResult>("ApplyRoleSparsePreset", repoPath, presetId, extraPatterns),
  hydrateSceneLfsAssets: (repoPath: string, scenePath: string) =>
    callBackend<SceneHydrateResult>("HydrateSceneLfsAssets", repoPath, scenePath),

  // Branch River & Stale Archiver
  getBranchRiverData: (repoPath: string, baseBranch: string = "") =>
    callBackend<BranchRiverData>("GetBranchRiverData", repoPath, baseBranch),
  detectStaleBranches: (repoPath: string, daysThreshold: number = 30) =>
    callBackend<StaleBranchesReport>("DetectStaleBranches", repoPath, daysThreshold),
  archiveAndDeleteBranches: (repoPath: string, branches: string[], bundlePath: string = "") =>
    callBackend<BranchArchiveResult>("ArchiveAndDeleteBranches", repoPath, branches, bundlePath),

  // Code Ownership & Evolution Timeline
  getCodeOwnership: (repoPath: string, targetPath: string = "") =>
    callBackend<CodeOwnershipReport>("GetCodeOwnership", repoPath, targetPath),
  getFileCommitTimeline: (repoPath: string, filePath: string, maxCommits: number = 20) =>
    callBackend<FileTimelineEntry[]>("GetFileCommitTimeline", repoPath, filePath, maxCommits),

  // Local CI Sandbox Runner & Team Webhook Hub
  runLocalCiSandbox: (repoPath: string, commands: string[], timeoutSec: number = 60) =>
    callBackend<LocalCiResult>("RunLocalCiSandbox", repoPath, commands, timeoutSec),
  dispatchTeamWebhook: (webhookUrl: string, eventType: string = "", payload: string = "", secret: string = "") =>
    callBackend<WebhookDispatchResult>("DispatchTeamWebhook", webhookUrl, eventType, payload, secret),
  prewarmRepoCache: (repoPath: string) =>
    callBackend<RepoCachePrewarmResult>("PrewarmRepoCache", repoPath),

  // Storage & Large Files Scanner
  getRepositoryDiskBreakdown: (repoPath: string) => callBackend<DiskBreakdownResult | null>("GetRepositoryDiskBreakdown", repoPath),
  scanLargeFiles: (repoPath: string, minSizeMB = 5) => callBackend<LargeFileInfo[]>("ScanLargeFiles", repoPath, minSizeMB),
  scanStagedSecrets: (repoPath: string) => callBackend<SecretFinding[]>("ScanStagedSecrets", repoPath),
  checkStagedNonLfsLargeFiles: (repoPath: string, thresholdMB = 10) => callBackend<LargeFileInfo[]>("CheckStagedNonLfsLargeFiles", repoPath, thresholdMB),

  // .gitignore audit & repair
  auditIgnoreRules: (repoPath: string) => callBackend<IgnoreSanitizerResult | null>("AuditIgnoreRules", repoPath),
  applyIgnoreSanitizer: (repoPath: string, engineType: string, untrackExisting: boolean) =>
    callBackend<IgnoreSanitizeActionResult | null>("ApplyIgnoreSanitizer", repoPath, engineType, untrackExisting),
  addToGitignore: (repoPath: string, pattern: string) => callBackend<void>("AddToGitignore", repoPath, pattern),
  generateGitignoreTemplate: (repoPath: string, templateType: string) =>
    callBackend<void>("GenerateGitignoreTemplate", repoPath, templateType),
  checkIgnoreRule: (repoPath: string, filePath: string) =>
    callBackend<CheckIgnoreResult | null>("CheckIgnoreRule", repoPath, filePath),

  // Repository Maintenance & Health
  getMaintenanceStats: (repoPath: string) => callBackend<RepoMaintenanceStats | null>("GetMaintenanceStats", repoPath),
  getGitHooks: (repoPath: string) => callBackend<GitHookInfo[]>("GetGitHooks", repoPath),
  detectHooksFramework: (repoPath: string) =>
    callBackend<GitHooksFrameworkInfo>("DetectHooksFramework", repoPath),
  runGitHook: (repoPath: string, hookName: string, args: string[] = []) =>
    callBackend<GitHookRunResult>("RunGitHook", repoPath, hookName, args),
  installSampleHook: (repoPath: string, hookName: string) =>
    callBackend<void>("InstallSampleHook", repoPath, hookName),
  toggleGitHook: (repoPath: string, hookName: string, enable: boolean, scriptContent = "") =>
    callBackend<void>("ToggleGitHook", repoPath, hookName, enable, scriptContent),
  getProjectAllFiles: (repoPath: string) => callBackend<string[]>("GetProjectAllFiles", repoPath),
  getGameEngineInfo: (repoPath: string) => callBackend<GameEngineInfo | null>("DetectGameEngineProject", repoPath),
  getGameEngineWorkflows: (repoPath: string) =>
    callBackend<EngineWorkflowInfo[]>("DetectGameEngineWorkflows", repoPath),
  inspectAssetMetadata: (repoPath: string, filePath: string) =>
    callBackend<AssetMetadata | null>("InspectAssetMetadata", repoPath, filePath),
  inspectAssetReferences: (repoPath: string, filePath: string) =>
    callBackend<AssetReferenceResult | null>("InspectAssetReferences", repoPath, filePath),
  checkUnityMetaSync: (repoPath: string) =>
    callBackend<UnityMetaMismatch[]>("CheckUnityMetaSync", repoPath),
  autoFixUnityMeta: (repoPath: string) => callBackend<string[]>("AutoFixUnityMeta", repoPath),
  detectUnityYamlMerge: (repoPath: string) =>
    callBackend<UnityMergeStatus | null>("DetectUnityYamlMerge", repoPath),
  configureUnityYamlMerge: (repoPath: string, customPath: string, applyGitattributes: boolean) =>
    callBackend<void>("ConfigureUnityYamlMerge", repoPath, customPath, applyGitattributes),
  getGameEngineGitAttributesPreset: (engine: string) =>
    callBackend<string>("GetGameEngineGitAttributesPreset", engine),
  lintGameTextures: (repoPath: string, checkAll = true) =>
    callBackend<TextureLintResult | null>("LintGameTextures", repoPath, checkAll),
  exportArchive: (repoPath: string, ref: string, outputPath: string) =>
    callBackend<void>("ExportArchive", repoPath, ref, outputPath),
  exportPatch: (repoPath: string, commitHash: string, outputPath: string) =>
    callBackend<void>("ExportPatch", repoPath, commitHash, outputPath),
  exportPatchRange: (repoPath: string, startHash: string, endHash: string, outputPath: string) =>
    callBackend<void>("ExportPatchRange", repoPath, startHash, endHash, outputPath),
  generatePatchRange: (repoPath: string, startHash: string, endHash: string) =>
    callBackend<string>("GeneratePatchRange", repoPath, startHash, endHash),
  createGitBundle: (repoPath: string, outputPath: string, refRange = "--all") =>
    callBackend<string>("CreateGitBundle", repoPath, outputPath, refRange),
  createGitBundleAdvanced: (repoPath: string, revSpec = "HEAD", outputPath = "") =>
    callBackend<GitBundleResult>("CreateGitBundleAdvanced", repoPath, revSpec, outputPath),
  verifyGitBundle: (repoPath: string, bundlePath: string) =>
    callBackend<string>("VerifyGitBundle", repoPath, bundlePath),
  getCommitCIStatus: (repoPath: string, commitHash: string) =>
    callBackend<CommitCIStatus>("GetCommitCIStatus", repoPath, commitHash),
  getPartialCloneStatus: (repoPath: string) =>
    callBackend<PartialCloneStatus>("GetPartialCloneStatus", repoPath),
  hydrateBlobs: (repoPath: string, paths: string[]) =>
    callBackend<HydrateBlobsResult>("HydrateBlobs", repoPath, paths),
  predictMergeConflicts: (repoPath: string, targetBranch: string) =>
    callBackend<MergeConflictPrediction>("PredictMergeConflicts", repoPath, targetBranch),
  detectRunningGameEngines: (repoPath: string) =>
    callBackend<EngineProcessStatus | null>("DetectRunningGameEngines", repoPath),
  launchExternalDiffTool: (
    repoPath: string,
    toolId: string,
    oldRef: string,
    newRef: string,
    filePath: string
  ) =>
    callBackend<ExternalDiffToolResult | null>(
      "LaunchExternalDiffTool",
      repoPath,
      toolId,
      oldRef,
      newRef,
      filePath
    ),
  applyPatch: (repoPath: string, patchPath: string) =>
    callBackend<void>("ApplyPatch", repoPath, patchPath),
  applyPatchContent: (repoPath: string, patchContent: string, checkOnly = false) =>
    callBackend<string>("ApplyPatchContent", repoPath, patchContent, checkOnly),
  getCommitTemplate: (repoPath: string) =>
    callBackend<CommitTemplateInfo | null>("GetCommitTemplate", repoPath),
  saveCommitTemplate: (repoPath: string, content: string, setGitConfig = true) =>
    callBackend<void>("SaveCommitTemplate", repoPath, content, setGitConfig),
  getRepoOverview: (repoPath: string) =>
    callBackend<RepoOverviewStats | null>("GetRepoOverview", repoPath),
  getCommitTopologyStats: (repoPath: string) =>
    callBackend<CommitTopologyStats | null>("GetCommitTopologyStats", repoPath),
  getTeamActivity: (repoPath: string, days = 182) =>
    callBackend<TeamActivityResult | null>("GetTeamActivity", repoPath, days),
  auditPrePushChecklist: (repoPath: string, targetBranch = "") =>
    callBackend<PrePushChecklistResult | null>("AuditPrePushChecklist", repoPath, targetBranch),
  auditCommitContentSafety: (repoPath: string, recentCount = 20) =>
    callBackend<CommitSafetyFilterResult | null>("AuditCommitContentSafety", repoPath, recentCount),
  getActivityLogs: () => callBackend<GitCommandLog[]>("GetActivityLogs"),
  clearActivityLogs: () => callBackend<void>("ClearActivityLogs"),
  scanLocalProjects: (searchRoot = "") =>
    callBackend<string[]>("ScanLocalProjects", searchRoot),
  getProjectsDetails: (paths: string[], activePath = "") =>
    callBackend<ProjectDetailItem[]>("GetProjectsDetails", paths, activePath),
  openWebURL: (targetUrl: string) => callBackend<void>("OpenWebURL", targetUrl),
  auditPrivacyAndCompliance: (repoPath: string) =>
    callBackend<PrivacyComplianceResult | null>("AuditPrivacyAndCompliance", repoPath),
  diagnoseGitAuth: (repoPath: string, host = "github.com") =>
    callBackend<AuthDiagnosticResult | null>("DiagnoseGitAuth", repoPath, host),
  getCustomActions: (repoPath: string) =>
    callBackend<CustomActionConfig[]>("GetCustomActions", repoPath),
  saveCustomActions: (repoPath: string, actions: CustomActionConfig[]) =>
    callBackend<void>("SaveCustomActions", repoPath, actions),
  executeCustomAction: (repoPath: string, actionId: string) =>
    callBackend<CustomActionResult | null>("ExecuteCustomAction", repoPath, actionId),
  revealFileInFinder: (repoPath: string, filePath: string) =>
    callBackend<void>("RevealFileInFinder", repoPath, filePath),
  openFileWithDefaultApp: (repoPath: string, filePath: string) =>
    callBackend<void>("OpenFileWithDefaultApp", repoPath, filePath),
  getGitAttributes: (repoPath: string) => callBackend<string>("GetGitAttributes", repoPath),
  saveGitAttributes: (repoPath: string, content: string) =>
    callBackend<void>("SaveGitAttributes", repoPath, content),
  runMaintenance: (repoPath: string) => callBackend<RepoMaintenanceStats | null>("RunMaintenance", repoPath),
  lfsPrune: (repoPath: string, dryRun = true, verbose = false) => callBackend<LfsPruneResult | null>("LfsPrune", repoPath, dryRun, verbose),
  verifyLfsPointers: (repoPath: string) => callBackend<GitFsckResult>("VerifyLfsPointers", repoPath),
  lfsPull: (repoPath: string) => callBackend<string>("LfsPull", repoPath),
  lfsPush: (repoPath: string) => callBackend<string>("LfsPush", repoPath),
  getLfsCacheStats: (repoPath: string) => callBackend<LfsCacheStats | null>("InspectLfsCacheStats", repoPath),
  getLfsFilterConfig: (repoPath: string) => callBackend<LfsFilterConfig | null>("GetLfsFilterConfig", repoPath),
  setLfsFilterConfig: (repoPath: string, include: string[], exclude: string[]) =>
    callBackend<void>("SetLfsFilterConfig", repoPath, include, exclude),
  getLfsTuningConfig: (repoPath: string) =>
    callBackend<LfsTuningConfig | null>("GetLfsTuningConfig", repoPath),
  applyLfsTuningOptimization: (repoPath: string, preset: string) =>
    callBackend<LfsTuningResult | null>("ApplyLfsTuningOptimization", repoPath, preset),
  inspectLfsFileHistory: (repoPath: string, filePath: string) =>
    callBackend<LfsFileRevision[]>("InspectLfsFileHistory", repoPath, filePath),
  getLfsLockActivity: (repoPath: string) =>
    callBackend<LfsLockActivityResult | null>("GetLfsLockActivity", repoPath),
  getLfsDiskUsage: (repoPath: string) => callBackend<string>("GetLfsDiskUsage", repoPath),
  executeLfsSelectivePull: (repoPath: string) =>
    callBackend<string>("ExecuteLfsSelectivePull", repoPath),
  trackExtensionsAsLfs: (repoPath: string, extensions: string[]) =>
    callBackend<void>("TrackExtensionsAsLfs", repoPath, extensions),
  checkFileLockStatus: (repoPath: string, filePath: string) =>
    callBackend<string>("CheckFileLockStatus", repoPath, filePath),
  checkRemoteLocksConflict: (repoPath: string) =>
    callBackend<FileStatus[]>("CheckRemoteLocksConflict", repoPath),
  exportLfsLocksReport: (repoPath: string, format: string) =>
    callBackend<string>("ExportLfsLocksReport", repoPath, format),
  verifyPrePushLfs: (repoPath: string, remote: string, branch: string) =>
    callBackend<PrePushVerificationResult | null>("VerifyPrePushLfs", repoPath, remote, branch),
  syncLockableFilePermissions: (repoPath: string) =>
    callBackend<LockableSyncResult | null>("SyncLockableFilePermissions", repoPath),
  toggleLockableReadOnlyProtection: (repoPath: string, enable: boolean) =>
    callBackend<LockableSyncResult | null>("ToggleLockableReadOnlyProtection", repoPath, enable),
  verifyRepoHealth: (repoPath: string) => callBackend<GitFsckResult | null>("VerifyRepoHealth", repoPath),

  // Source Blame
  /** @deprecated Kept for backwards compatibility. Use getFileBlameWithOptions. */
  getFileBlame: (repoPath: string, filePath: string) => callBackend<BlameLine[]>("GetFileBlame", repoPath, filePath),
  getFileBlameWithOptions: (repoPath: string, opts: BlameOptions) =>
    callBackend<BlameLine[]>("GetFileBlameWithOptions", repoPath, opts),

  // File History & Revisions
  getWorkspaceStats: (repoPath: string) =>
    callBackend<WorkspaceStats | null>("GetWorkspaceStats", repoPath),
  getLastCommitMessage: (repoPath: string) =>
    callBackend<string>("GetLastCommitMessage", repoPath),
  getRepoStorageBreakdown: (repoPath: string) =>
    callBackend<StorageBreakdownItem[]>("GetRepoStorageBreakdown", repoPath),
  getDiffBetween: (repoPath: string, refA: string, refB: string) =>
    callBackend<DiffComparisonResult | null>("GetDiffBetween", repoPath, refA, refB),
  /** Three-dot comparison: diff refA...refB (against the merge-base). */
  getMergeBaseDiff: (repoPath: string, refA: string, refB: string) =>
    callBackend<DiffComparisonResult | null>("GetMergeBaseDiff", repoPath, refA, refB),
  getFileDiffBetween: (repoPath: string, refA: string, refB: string, filePath: string, mergeBase = false) =>
    callBackend<string>("GetFileDiffBetween", repoPath, refA, refB, filePath, mergeBase),
  runRepoHealthCheck: (repoPath: string) =>
    callBackend<RepoHealthReport | null>("RunRepoHealthCheck", repoPath),
  scanHistoryLargeBlobs: (repoPath: string, limit = 25) =>
    callBackend<HistoryLargeBlob[]>("ScanHistoryLargeBlobs", repoPath, limit),
  scanEmptyDirectories: (repoPath: string) =>
    callBackend<string[]>("ScanEmptyDirectories", repoPath),
  generateGitkeeps: (repoPath: string, directories: string[]) =>
    callBackend<number>("GenerateGitkeeps", repoPath, directories),
  getDetailedGitConfig: (repoPath: string) =>
    callBackend<ConfigEntry[]>("GetDetailedGitConfig", repoPath),
  setCustomConfigKey: (repoPath: string, key: string, value: string, isGlobal = false) =>
    callBackend<void>("SetCustomConfigKey", repoPath, key, value, isGlobal),
  getSafeDirectories: () =>
    callBackend<string[]>("GetSafeDirectories"),
  addSafeDirectory: (repoPath: string) =>
    callBackend<void>("AddSafeDirectory", repoPath),
  checkSafeDirectory: (repoPath: string) =>
    callBackend<boolean>("CheckSafeDirectory", repoPath),
  removeSafeDirectory: (dirPath: string) =>
    callBackend<void>("RemoveSafeDirectory", dirPath),
  lintModelMesh: (repoPath: string, targetPath: string, budgetPreset: string) =>
    callBackend<MeshLintResult | null>("LintModelMesh", repoPath, targetPath, budgetPreset),
  lintAnimationRig: (repoPath: string, assetPath: string, preset: string) =>
    callBackend<AnimRigLintResult | null>("LintAnimationRig", repoPath, assetPath, preset),
  inspectAudioFile: (repoPath: string, targetPath: string) =>
    callBackend<AudioInspectResult | null>("InspectAudioFile", repoPath, targetPath),
  auditLocalizationParity: (repoPath: string, fontPath = "") =>
    callBackend<LocalizationAuditResult | null>("AuditLocalizationParity", repoPath, fontPath),
  auditNativePluginsAbi: (repoPath: string) =>
    callBackend<NativePluginsAbiResult | null>("AuditNativePluginsAbi", repoPath),
  scanMissingMaterials: (repoPath: string, targetDir = "") =>
    callBackend<MissingMaterialScanResult | null>("ScanMissingMaterials", repoPath, targetDir),
  getImageBase64: (repoPath: string, commitHash: string, filePath: string) =>
    callBackend<string>("GetImageBase64", repoPath, commitHash, filePath),
  auditLodStreaming: (repoPath: string, targetDir = "") =>
    callBackend<LodStreamingAuditResult | null>("AuditLodStreaming", repoPath, targetDir),
  auditPhysicsMeshCollision: (repoPath: string, targetDir = "") =>
    callBackend<PhysicsCollisionAuditResult | null>("AuditPhysicsMeshCollision", repoPath, targetDir),
  auditMaterialInstanceOverrides: (repoPath: string, targetDir = "") =>
    callBackend<MaterialInstanceOverrideResult | null>(
      "AuditMaterialInstanceOverrides",
      repoPath,
      targetDir
    ),
  detectDccLockActivity: (repoPath: string) =>
    callBackend<DCCLockWatchResult | null>("DetectDCCLockActivity", repoPath),
  analyzePackageSizeBudget: (repoPath: string, targetPlatform = "") =>
    callBackend<PackageBudgetResult | null>("AnalyzePackageSizeBudget", repoPath, targetPlatform),

  // Branch Comparison
  compareBranches: (repoPath: string, baseBranch: string, targetBranch: string) =>
    callBackend<BranchCompareResult | null>("CompareBranches", repoPath, baseBranch, targetBranch),

  // Batch Operations
  batchStageFiles: (repoPath: string, filePaths: string[]) => callBackend<void>("BatchStageFiles", repoPath, filePaths),
  batchUnstageFiles: (repoPath: string, filePaths: string[]) => callBackend<void>("BatchUnstageFiles", repoPath, filePaths),
  batchDiscardFiles: (repoPath: string, filePaths: string[]) => callBackend<void>("BatchDiscardFiles", repoPath, filePaths),

  // LFS File Locking
  lockFile: (repoPath: string, filePath: string) => callBackend<void>("LockFile", repoPath, filePath),
  unlockFile: (repoPath: string, filePath: string, force = false) => callBackend<void>("UnlockFile", repoPath, filePath, force),
  lockAllModifiedLfs: (repoPath: string) => callBackend<BulkLockResult | null>("LockAllModifiedLfs", repoPath),

  // Studio & Parity Hubs
  getPinnacleParityStatusSummary: (repoPath: string) => callBackend<PinnacleParityStatusSummary>("GetPinnacleParityStatusSummary", repoPath),
  getFullParityStatusSummary: (repoPath: string) => callBackend<FullParityStatusSummary>("GetFullParityStatusSummary", repoPath),
  fetchTAPDWorkItems: (repoPath: string) =>
    callBackend<TAPDWorkItemListResult | null>("FetchTAPDWorkItems", repoPath),
  bindCommitToTAPD: (repoPath: string, commitHash: string, tapdId: string, newStatus: string) =>
    callBackend<TAPDBindResult | null>("BindCommitToTAPD", repoPath, commitHash, tapdId, newStatus),
  parseTAPDCommitReferences: (commitMessage: string) =>
    callBackend<TAPDCommitReferenceResult | null>("ParseTAPDCommitReferences", commitMessage),
  getStudioCIPipelines: (repoPath: string) =>
    callBackend<StudioCIResult | null>("GetStudioCIPipelines", repoPath),
  triggerStudioCIPipeline: (repoPath: string, pipelineName: string) =>
    callBackend<StudioCIActionResult | null>("TriggerStudioCIPipeline", repoPath, pipelineName),
  inspectCredentialVault: (repoPath: string) =>
    callBackend<CredentialVaultResult | null>("InspectCredentialVault", repoPath),
  storeCredentialSecret: (
    repoPath: string,
    secretType: string,
    identifier: string,
    secretValue: string,
    scope: string
  ) =>
    callBackend<StoreSecretResult | null>(
      "StoreCredentialSecret",
      repoPath,
      secretType,
      identifier,
      secretValue,
      scope
    ),
  manageStudioWebhooks: (
    repoPath: string,
    action: string,
    webhookType: string,
    webhookUrl: string,
    eventRule: string
  ) =>
    callBackend<StudioWebhookResult | null>(
      "ManageStudioWebhooks",
      repoPath,
      action,
      webhookType,
      webhookUrl,
      eventRule
    ),
  manageStudioDDC: (repoPath: string, action: string) =>
    callBackend<StudioDDCResult | null>("ManageStudioDDC", repoPath, action),
  manageOfflineSubmitQueue: (repoPath: string, action: string, itemId: string, payload: string) =>
    callBackend<OfflineQueueResult | null>(
      "ManageOfflineSubmitQueue",
      repoPath,
      action,
      itemId,
      payload
    ),
  manageStudioSwitchboard: (
    repoPath: string,
    action: string,
    targetPath: string,
    targetBranch: string,
    extraParam: string
  ) =>
    callBackend<StudioSwitchboardResult | null>(
      "ManageStudioSwitchboard",
      repoPath,
      action,
      targetPath,
      targetBranch,
      extraParam
    ),

  // Configuration & Settings
  getGitConfig: (repoPath: string) => callBackend<GitUserConfig>("GetGitConfig", repoPath),
  getGitVersion: () => callBackend<GitVersionInfo | null>("GetGitVersion"),
  setGitConfig: (repoPath: string, name: string, email: string, global = false) => callBackend<void>("SetGitConfig", repoPath, name, email, global),
  getGitProxy: (repoPath: string) => callBackend<GitProxyConfig>("GetGitProxy", repoPath),
  setGitProxy: (repoPath: string, proxyUrl: string, isGlobal = false) => callBackend<void>("SetGitProxy", repoPath, proxyUrl, isGlobal),
  getCommitSigningStatus: (repoPath: string) =>
    callBackend<CommitSigningStatus | null>("GetCommitSigningStatus", repoPath),
  configureCommitSigning: (repoPath: string, enable: boolean, keyPath = "", format = "openpgp") =>
    callBackend<CommitSigningResult>("ConfigureCommitSigning", repoPath, enable, keyPath, format),

  // Change review sessions (base...head review with per-file verdicts & comments)
  getChangeReview: (repoPath: string, base: string, head: string) =>
    callBackend<ChangeReview | null>("GetChangeReview", repoPath, base, head),
  listChangeReviews: (repoPath: string) =>
    callBackend<ReviewSessionSummary[]>("ListChangeReviews", repoPath),
  setReviewFileState: (repoPath: string, base: string, head: string, path: string, status: string, note: string) =>
    callBackend<ChangeReview | null>("SetReviewFileState", repoPath, base, head, path, status, note),
  addReviewComment: (repoPath: string, base: string, head: string, path: string, line: number, side: string, body: string) =>
    callBackend<ChangeReview | null>("AddReviewComment", repoPath, base, head, path, line, side, body),
  resolveReviewComment: (repoPath: string, base: string, head: string, id: string, resolved: boolean) =>
    callBackend<ChangeReview | null>("ResolveReviewComment", repoPath, base, head, id, resolved),
  deleteReviewComment: (repoPath: string, base: string, head: string, id: string) =>
    callBackend<ChangeReview | null>("DeleteReviewComment", repoPath, base, head, id),
  clearChangeReview: (repoPath: string, base: string, head: string) =>
    callBackend<string>("ClearChangeReview", repoPath, base, head),
  exportChangeReviewMarkdown: (repoPath: string, base: string, head: string) =>
    callBackend<string>("ExportChangeReviewMarkdown", repoPath, base, head),
  exportCommitReport: (repoPath: string, since = "", until = "", format = "markdown") =>
    callBackend<string>("ExportCommitReport", repoPath, since, until, format),
  getReviewFileDiff: (repoPath: string, base: string, head: string, path: string) =>
    callBackend<string>("GetReviewFileDiff", repoPath, base, head, path),

  // Release wizard & studio changelog
  getReleasePlan: (repoPath: string) => callBackend<ReleasePlan | null>("GetReleasePlan", repoPath),
  createRelease: (repoPath: string, version: string, message: string, push: boolean, remote: string, sign: boolean) =>
    callBackend<ReleaseCreateResult | null>("CreateRelease", repoPath, version, message, push, remote, sign),
  generateStudioChangelog: (repoPath: string, fromRef: string, toRef: string) =>
    callBackend<StudioChangelogResult | null>("GenerateStudioChangelog", repoPath, fromRef, toRef),

  // External Diff / Merge Tools & Web Forge
  detectExternalTools: () => callBackend<ExternalToolInfo[]>("DetectExternalTools"),
  getExternalToolConfig: (repoPath: string) => callBackend<ExternalToolConfig>("GetExternalToolConfig", repoPath),
  setExternalToolConfig: (repoPath: string, diffTool: string, mergeTool: string, customDiffCmd = "", customMergeCmd = "", isGlobal = false) =>
    callBackend<void>("SetExternalToolConfig", repoPath, diffTool, mergeTool, customDiffCmd, customMergeCmd, isGlobal),
  openExternalDiff: (repoPath: string, filePath: string, isStaged = false, toolId = "") =>
    callBackend<void>("OpenExternalDiff", repoPath, filePath, isStaged, toolId),
  openExternalMerge: (repoPath: string, filePath: string, toolId = "") =>
    callBackend<void>("OpenExternalMerge", repoPath, filePath, toolId),
  getRemoteWebInfo: (repoPath: string) => callBackend<RemoteWebInfo>("GetRemoteWebInfo", repoPath),
  openCommitInWeb: (repoPath: string, commitHash: string) => callBackend<void>("OpenCommitInWeb", repoPath, commitHash),
  openFileInWeb: (repoPath: string, filePath: string, branch = "") => callBackend<void>("OpenFileInWeb", repoPath, filePath, branch),
  openBranchInWeb: (repoPath: string, branch = "") => callBackend<void>("OpenBranchInWeb", repoPath, branch),
  openCompareInWeb: (repoPath: string, baseBranch = "", headBranch = "") => callBackend<void>("OpenCompareInWeb", repoPath, baseBranch, headBranch),
  openRepoInWeb: (repoPath: string) => callBackend<void>("OpenRepoInWeb", repoPath),

  // Pull, Push, Rebase, Merge Advanced Ops
  pullAdvanced: (repoPath: string, opts: PullAdvancedOptions) =>
    callBackend<PullResult>("PullAdvanced", repoPath, opts),
  getAheadCommits: (repoPath: string, remote = "", branch = "") =>
    callBackend<CommitItem[]>("GetAheadCommits", repoPath, remote, branch),
  getBehindCommits: (repoPath: string, remote = "", branch = "") =>
    callBackend<CommitItem[]>("GetBehindCommits", repoPath, remote, branch),
  pushAdvanced: (repoPath: string, opts: PushAdvancedOptions) =>
    callBackend<PushResult>("PushAdvanced", repoPath, opts),
  previewMerge: (repoPath: string, targetBranch: string) =>
    callBackend<MergePreviewResult>("PreviewMerge", repoPath, targetBranch),
  mergeAdvanced: (repoPath: string, opts: MergeAdvancedOptions) =>
    callBackend<MergeResult>("MergeAdvanced", repoPath, opts),
  rebaseAdvanced: (repoPath: string, opts: RebaseAdvancedOptions) =>
    callBackend<RebaseResult>("RebaseAdvanced", repoPath, opts),
  getRebaseProgress: (repoPath: string) =>
    callBackend<RebaseProgressStatus>("GetRebaseProgress", repoPath),
  abortConflictOperation: (repoPath: string) =>
    callBackend<string>("AbortConflictOperation", repoPath),
  continueConflictOperation: (repoPath: string) =>
    callBackend<string>("ContinueConflictOperation", repoPath),
  skipConflictOperation: (repoPath: string) =>
    callBackend<string>("SkipConflictOperation", repoPath),

  // ---- 闭环补齐：诊断 → 修复 -------------------------------------------
  trackStagedLargeFilesAsLfs: (repoPath: string, paths: string[], lockable = true) =>
    callBackend<LfsConvertResult>("TrackStagedLargeFilesAsLfs", repoPath, paths, lockable),
  unlockStaleLfsLocks: (repoPath: string, paths: string[], force = true) =>
    callBackend<LfsBulkUnlockResult>("UnlockStaleLfsLocks", repoPath, paths, force),
  getRangeDiff: (repoPath: string, oldBase: string, oldHead: string, newBase: string, newHead: string) =>
    callBackend<RangeDiffResult>("GetRangeDiff", repoPath, oldBase, oldHead, newBase, newHead),
  createFixupCommit: (repoPath: string, targetHash: string, mode: "fixup" | "squash" | "amend" = "fixup") =>
    callBackend<FixupCommitResult>("CreateFixupCommit", repoPath, targetHash, mode),
  optimizeObjectStore: (repoPath: string, tasks: string[] = []) =>
    callBackend<ObjectStoreOptimizeResult>("OptimizeObjectStore", repoPath, tasks),
  getMergeOverlapWarning: (repoPath: string, targetBranch: string, days = 7) =>
    callBackend<MergeOverlapResult>("GetMergeOverlapWarning", repoPath, targetBranch, days),
  runHealthFix: (repoPath: string, key: string) =>
    callBackend<HealthFixResult>("RunHealthFix", repoPath, key),

  // ---- 修复中心：聚合诊断 → 一键修复 -----------------------------------
  scanFixCenter: (repoPath: string) =>
    callBackend<FixCenterResult>("ScanFixCenter", repoPath),
  applyAuditFix: (repoPath: string, fixId: string, targets: string[] = []) =>
    callBackend<AuditFixResult>("ApplyAuditFix", repoPath, fixId, targets),
  remediateStagedSecrets: (repoPath: string, paths: string[], writeGitignore = true, allowlist = false) =>
    callBackend<SecretRemediationResult>("RemediateStagedSecrets", repoPath, paths, writeGitignore, allowlist),

  // ---- 二进制资产差异 ------------------------------------------------
  getAssetDiff: (repoPath: string, path: string, oldRef: string, newRef: string) =>
    callBackend<AssetDiffResult>("GetAssetDiff", repoPath, path, oldRef, newRef),

  // ---- LFS 历史迁移向导 ----------------------------------------------
  previewLfsMigration: (repoPath: string, patterns: string[], everything = true, revRange = "") =>
    callBackend<LfsMigratePreview>("PreviewLfsMigration", repoPath, patterns, everything, revRange),
  executeLfsMigration: (repoPath: string, opts: LfsMigrateOptions) =>
    callBackend<LfsMigrateResult>("ExecuteLfsMigration", repoPath, opts),

  // ---- 发布标签验签 --------------------------------------------------
  verifyTagSignature: (repoPath: string, tagName: string) =>
    callBackend<TagVerifyResult>("VerifyTagSignature", repoPath, tagName),

  // ---- 资产影响分析 --------------------------------------------------
  analyzeAssetImpact: (repoPath: string, targetPath: string, depth = 2) =>
    callBackend<AssetImpactResult>("AnalyzeAssetImpact", repoPath, targetPath, depth),

  // ---- 引擎工程 Bootstrap --------------------------------------------
  runEngineBootstrap: (repoPath: string, opts: EngineBootstrapOptions) =>
    callBackend<EngineBootstrapResult>("RunEngineBootstrap", repoPath, opts),

  // ---- 操作日志与撤销 ------------------------------------------------
  bisectSkip: (repoPath: string) => callBackend<BisectStatus>("BisectSkip", repoPath),

  // ---- 工作树修复 / 钩子脚本 -----------------------------------------
  repairWorktrees: (repoPath: string, paths: string[] = []) =>
    callBackend<WorktreeOpResult>("RepairWorktrees", repoPath, paths),
  pruneWorktrees: (repoPath: string) =>
    callBackend<WorktreeOpResult>("PruneWorktrees", repoPath),
  readGitHook: (repoPath: string, name: string) =>
    callBackend<GitHookContent>("ReadGitHook", repoPath, name),
  saveGitHook: (repoPath: string, name: string, content: string, enable = true) =>
    callBackend<void>("SaveGitHook", repoPath, name, content, enable),

  getOperationLog: (repoPath: string, limit = 40) =>
    callBackend<OperationLogResult>("GetOperationLog", repoPath, limit),
  undoLastOperation: (repoPath: string) =>
    callBackend<UndoResult>("UndoLastOperation", repoPath),

  // ---- 内置终端 ------------------------------------------------------
  startTerminal: (repoPath: string, cols = 100, rows = 30) =>
    callBackend<TerminalStatus>("StartTerminal", repoPath, cols, rows),
  writeTerminal: (data: string) => callBackend<void>("WriteTerminal", data),
  resizeTerminal: (cols: number, rows: number) => callBackend<void>("ResizeTerminal", cols, rows),
  stopTerminal: () => callBackend<void>("StopTerminal"),
  getTerminalStatus: () => callBackend<TerminalStatus>("GetTerminalStatus"),

  // ---- PR / MR 就地评审 ----------------------------------------------
  getPullRequestProvider: (repoPath: string) =>
    callBackend<PrProviderInfo>("GetPullRequestProvider", repoPath),
  savePullRequestToken: (repoPath: string, token: string) =>
    callBackend<PrProviderInfo>("SavePullRequestToken", repoPath, token),
  clearPullRequestToken: (repoPath: string) =>
    callBackend<PrProviderInfo>("ClearPullRequestToken", repoPath),
  listPullRequests: (repoPath: string, state = "open", limit = 30, page = 1) =>
    callBackend<PrListResult>("ListPullRequests", repoPath, state, limit, page),
  getPullRequestDetail: (repoPath: string, number: number) =>
    callBackend<PrDetail>("GetPullRequestDetail", repoPath, number),
  createPullRequestComment: (
    repoPath: string,
    number: number,
    body: string,
    filePath = "",
    line = 0,
    side = "new"
  ) =>
    callBackend<PrActionResult>(
      "CreatePullRequestComment",
      repoPath,
      number,
      body,
      filePath,
      line,
      side
    ),
  submitPullRequestReview: (repoPath: string, number: number, event: string, body = "") =>
    callBackend<PrActionResult>("SubmitPullRequestReview", repoPath, number, event, body),
  mergePullRequest: (repoPath: string, number: number, method = "merge") =>
    callBackend<PrActionResult>("MergePullRequest", repoPath, number, method),

  // Deep Link protocol handler support
  getInitialDeepLink: () => callBackend<string>("GetInitialDeepLink"),

  // LAN Peer Review Server & Enterprise Compliance
  getLanReviewServerStatus: () =>
    callBackend<LanServerInfo>("GetLanReviewServerStatus"),
  startLanReviewServer: (repoPath: string, port = 8080) =>
    callBackend<LanServerInfo>("StartLanReviewServer", repoPath, port),
  stopLanReviewServer: () =>
    callBackend<LanServerInfo>("StopLanReviewServer"),
  broadcastLanReviewEvent: (eventType: string, payload: string) =>
    callBackend<void>("BroadcastLanReviewEvent", eventType, payload),
  detectLanLfsPeers: (repoPath: string) =>
    callBackend<LanLfsPeerStatus>("DetectLanLfsPeers", repoPath),
  syncLanLfsBlob: (repoPath: string, peerUrl: string, oid: string) =>
    callBackend<LanLfsTransferResult>("SyncLanLfsBlob", repoPath, peerUrl, oid),
  configureLanLfsSharing: (repoPath: string, enable: boolean, port: number) =>
    callBackend<LanLfsShareConfig>("ConfigureLanLfsSharing", repoPath, enable, port),
  generateReleaseChangelog: (repoPath: string, fromRef = "", toRef = "HEAD") =>
    callBackend<ReleaseChangelogResult>("GenerateReleaseChangelog", repoPath, fromRef, toRef),
  saveRepoComplianceRules: (repoPath: string, rules: ComplianceRules) =>
    callBackend<void>("SaveRepoComplianceRules", repoPath, rules),
  inspectRepoCompliance: (repoPath: string) =>
    callBackend<RepoComplianceReport>("InspectRepoCompliance", repoPath),
  validateCommitPolicy: (
    repoPath: string,
    commitMessage: string,
    authorEmail: string,
    branchName: string
  ) =>
    callBackend<CommitPolicyResult>(
      "ValidateCommitPolicy",
      repoPath,
      commitMessage,
      authorEmail,
      branchName
    ),

  // Material & Texture Inspection (Phase F1)
  compareMaterialProperties: (
    repoPath: string,
    path: string,
    oldRef: string,
    newRef: string
  ) =>
    callBackend<MaterialDiffResult>(
      "CompareMaterialProperties",
      repoPath,
      path,
      oldRef,
      newRef
    ),
  inspectTextureMetadata: (repoPath: string, path: string) =>
    callBackend<TextureMetadataResult>(
      "InspectTextureMetadata",
      repoPath,
      path
    ),

  // Generic studio dispatcher for any of the 140 round backend methods
  invoke: <T = any>(methodName: string, ...args: any[]) => callBackend<T>(methodName, ...args),
};

export const exportPatchRange = gitService.exportPatchRange;
export const generatePatchRange = gitService.generatePatchRange;
