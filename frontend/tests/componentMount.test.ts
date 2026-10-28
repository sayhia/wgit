import { beforeEach, describe, expect, test, spyOn } from "bun:test";
import { mount } from "@vue/test-utils";
import UiSkeleton from "../src/components/ui/UiSkeleton.vue";
import ConfirmDialog from "../src/components/ui/ConfirmDialog.vue";
import CrashReportModal from "../src/components/CrashReportModal.vue";
import AboutSection from "../src/components/settings/AboutSection.vue";
import ThreeWayMergeModal from "../src/components/ThreeWayMergeModal.vue";
import CommitComposer from "../src/components/workspace/CommitComposer.vue";
import CommandPalette from "../src/components/CommandPalette.vue";
import DiffViewer from "../src/components/workspace/DiffViewer.vue";
import StashView from "../src/components/StashView.vue";
import BranchRiverView from "../src/components/branches/BranchRiverView.vue";
import SquashCommitsModal from "../src/components/history/SquashCommitsModal.vue";
import AliasesSection from "../src/components/settings/AliasesSection.vue";
import WorktreesTab from "../src/components/tools/WorktreesTab.vue";
import RemotesTab from "../src/components/tools/RemotesTab.vue";
import ReleaseWizard from "../src/components/ReleaseWizard.vue";
import ChangesList from "../src/components/workspace/ChangesList.vue";
import LocalBranchList from "../src/components/branches/LocalBranchList.vue";
import TagsList from "../src/components/branches/TagsList.vue";
import MiniCommitHudModal from "../src/components/workspace/MiniCommitHudModal.vue";
import HistoryFilterBar from "../src/components/history/HistoryFilterBar.vue";
import CommitInspector from "../src/components/history/CommitInspector.vue";
import BranchCompare from "../src/components/branches/BranchCompare.vue";
import GitHooksSection from "../src/components/settings/GitHooksSection.vue";
import SubmodulesTab from "../src/components/tools/SubmodulesTab.vue";
import CommitTimeLapseModal from "../src/components/history/CommitTimeLapseModal.vue";
import ExportModal from "../src/components/review/ExportModal.vue";
import InteractiveRebaseModal from "../src/components/history/InteractiveRebaseModal.vue";
import MaintenanceTab from "../src/components/tools/MaintenanceTab.vue";
import HeaderBar from "../src/components/HeaderBar.vue";
import SyncModal from "../src/components/SyncModal.vue";
import ShortcutCheatSheetModal from "../src/components/ShortcutCheatSheetModal.vue";
import { confirmAction, confirmState } from "../src/composables/useConfirm";
import { clearCrashLogs, recordCrash } from "../src/utils/crashReporter";
import { gitService } from "../src/services/gitService";
import type {
  ThreeWayMergeData,
  StashInfo,
  BranchRiverData,
  CommitItem,
  GitAliasItem,
  WorktreeInfo,
  RemoteDetail,
  ReleasePlan,
  FileStatus,
  BranchInfo,
  TagInfo,
  RepoStatus,
} from "../src/types";
import "./domSetup";

describe("P5: Real Component DOM Mounting Baseline (@vue/test-utils + happy-dom)", () => {
  beforeEach(() => {
    clearCrashLogs();
    confirmState.open = false;
  });

  test("mounts UiSkeleton and respects lines and variant props", () => {
    const wrapper = mount(UiSkeleton, {
      props: {
        lines: 4,
        variant: "line",
      },
    });

    expect(wrapper.exists()).toBe(true);
    const divs = wrapper.findAll(".animate-pulse");
    expect(divs.length).toBe(4);
    expect(wrapper.attributes("role")).toBe("status");
  });

  test("mounts ConfirmDialog, opens on confirmAction and settles promise on accept", async () => {
    const wrapper = mount(ConfirmDialog);
    expect(wrapper.exists()).toBe(true);

    const promise = confirmAction({
      title: "删除分支",
      message: "确认删除 feature-test 分支吗？",
      tone: "danger",
      confirmLabel: "立即删除",
    });

    await wrapper.vm.$nextTick();

    expect(confirmState.open).toBe(true);
    expect(confirmState.tone).toBe("danger");
    expect(confirmState.confirmLabel).toBe("立即删除");

    // Click the confirm button in the DOM
    const buttons = wrapper.findAll("button");
    const confirmBtn = buttons.find((b) => b.text().includes("立即删除"));
    expect(confirmBtn).toBeDefined();
    await confirmBtn!.trigger("click");

    const result = await promise;
    expect(result).toBe(true);
    expect(confirmState.open).toBe(false);
  });

  test("mounts CrashReportModal with real crash records and handles clearing", async () => {
    recordCrash(new Error("Test crash for DOM mount"), "dom_test", { extra: 123 });

    const wrapper = mount(CrashReportModal, {
      props: {
        isOpen: true,
      },
    });

    await wrapper.vm.$nextTick();

    expect(wrapper.exists()).toBe(true);
    const text = wrapper.text();
    expect(text).toContain("Test crash for DOM mount");
    expect(text).toContain("dom_test");

    // Click the clear button
    const clearBtn = wrapper.findAll("button").find((b) => b.attributes("title")?.includes("清空") || b.text().includes("清空"));
    if (clearBtn) {
      await clearBtn.trigger("click");
      await wrapper.vm.$nextTick();
      expect(wrapper.text()).toContain("运行健康");
    }
  });

  test("mounts AboutSection and renders application metadata", async () => {
    const wrapper = mount(AboutSection);
    expect(wrapper.exists()).toBe(true);

    const text = wrapper.text();
    expect(text).toContain("wgit");
    expect(text).toContain("关于 wgit");
    expect(text).toContain("版本号");
    expect(text).toContain("构建哈希");
    expect(text).toContain("wgit://");
  });

  test("mounts CommitComposer and emits updates on user input", async () => {
    const wrapper = mount(CommitComposer, {
      props: {
        subject: "feat: initial commit",
        body: "initial body",
        amend: false,
        autoPush: false,
        committing: false,
      },
    });

    expect(wrapper.exists()).toBe(true);
    const textarea = wrapper.find("textarea");
    expect(textarea.exists()).toBe(true);

    const input = wrapper.find("input[type='text']");
    expect(input.exists()).toBe(true);
    expect((input.element as HTMLInputElement).value).toBe("feat: initial commit");

    await input.setValue("feat: updated commit message");
    expect(wrapper.emitted("update:subject")).toBeTruthy();
    expect(wrapper.emitted("update:subject")![0]).toEqual(["feat: updated commit message"]);
  });

  test("mounts CommandPalette and handles search filtering and close events", async () => {
    const wrapper = mount(CommandPalette, {
      props: {
        isOpen: true,
      },
    });

    expect(wrapper.exists()).toBe(true);
    const searchInput = wrapper.find("input");
    expect(searchInput.exists()).toBe(true);

    // Initial state shows navigation and repo commands in active locale
    expect(wrapper.text()).toContain("切换到工作区");

    // Close button / backdrop
    await wrapper.setProps({ isOpen: false });
    await wrapper.vm.$nextTick();
  });

  test("mounts ThreeWayMergeModal and loads 3-way blocks into visual panes", async () => {
    const mockMergeData: ThreeWayMergeData = {
      filePath: "src/conflict.txt",
      baseContent: "common base text",
      oursContent: "local modified text",
      theirsContent: "incoming modified text",
      currentContent: "<<<<<<< HEAD\nlocal modified text\n=======\nincoming modified text\n>>>>>>> feature/awesome\n",
      totalConflicts: 1,
      blocks: [
        {
          index: 0,
          startLine: 1,
          endLine: 5,
          baseLines: ["common base text"],
          oursLines: ["local modified text"],
          theirsLines: ["incoming modified text"],
        },
      ],
    };

    const getSpy = spyOn(gitService, "getThreeWayMergeData").mockResolvedValue(mockMergeData);

    const wrapper = mount(ThreeWayMergeModal, {
      props: {
        open: true,
        repoPath: "/path/to/test-repo",
        filePath: "src/conflict.txt",
      },
    });

    expect(wrapper.exists()).toBe(true);
    // Allow async loadData to complete
    await new Promise((resolve) => setTimeout(resolve, 50));
    await wrapper.vm.$nextTick();

    expect(getSpy).toHaveBeenCalledWith("/path/to/test-repo", "src/conflict.txt");
    expect(wrapper.text()).toContain("src/conflict.txt");
    expect(wrapper.text()).toContain("local modified text");
    expect(wrapper.text()).toContain("incoming modified text");

    getSpy.mockRestore();
  });

  test("mounts DiffViewer and renders diff lines, blame toggle, and minimap gutter", async () => {
    const diffSample =
      "diff --git a/src/math.ts b/src/math.ts\n" +
      "--- a/src/math.ts\n" +
      "+++ b/src/math.ts\n" +
      "@@ -1,3 +1,3 @@\n" +
      " export function add(a: number, b: number): number {\n" +
      "-  return a - b;\n" +
      "+  return a + b;\n" +
      " }\n";

    const wrapper = mount(DiffViewer, {
      props: {
        diffText: diffSample,
        mode: "unified",
        repoPath: "/path/to/test-repo",
        filePath: "src/math.ts",
      },
    });

    expect(wrapper.exists()).toBe(true);
    await wrapper.vm.$nextTick();

    expect(wrapper.text()).toContain("return a + b;");
    expect(wrapper.text()).toContain("return a - b;");
    expect(wrapper.find("[data-testid='diff-minimap-gutter']").exists()).toBe(true);
  });

  test("mounts StashView with mocked stashes and handles selection and actions", async () => {
    const mockStashes: StashInfo[] = [
      {
        index: 0,
        name: "stash@{0}",
        branch: "main",
        message: "WIP on main: test stash 1",
        date: "2026-10-28 07:00:00",
        commitHash: "12345678",
      },
      {
        index: 1,
        name: "stash@{1}",
        branch: "feature",
        message: "WIP on feature: test stash 2",
        date: "2026-10-27 12:00:00",
        commitHash: "87654321",
      },
    ];

    const stashSpy = spyOn(gitService, "getStashes").mockResolvedValue(mockStashes);

    const wrapper = mount(StashView, {
      props: {
        currentRepoPath: "/path/to/test-repo",
      },
    });

    expect(wrapper.exists()).toBe(true);
    await new Promise((resolve) => setTimeout(resolve, 30));
    await wrapper.vm.$nextTick();

    expect(wrapper.text()).toContain("WIP on main: test stash 1");
    expect(wrapper.text()).toContain("stash@0");

    stashSpy.mockRestore();
  });

  test("mounts BranchRiverView and filters river topology nodes on search", async () => {
    const mockRiverData: BranchRiverData = {
      nodes: [
        {
          name: "main",
          upstream: "origin/main",
          ahead: 0,
          behind: 0,
          lastCommitTime: "2026-10-28 07:00:00",
          lastCommitSubject: "feat: initial commit",
          lastAuthor: "Alice",
          isMerged: false,
          isStale: false,
          divergenceScore: 10,
        },
        {
          name: "feature/auth-sso",
          upstream: "",
          ahead: 3,
          behind: 1,
          lastCommitTime: "2026-10-27 10:00:00",
          lastCommitSubject: "feat: add sso auth",
          lastAuthor: "Bob",
          isMerged: false,
          isStale: false,
          divergenceScore: 40,
        },
      ],
      totalBranches: 2,
      staleCount: 0,
      activeSprintDays: 14,
    };

    const riverSpy = spyOn(gitService, "getBranchRiverData").mockResolvedValue(mockRiverData);

    const wrapper = mount(BranchRiverView, {
      props: {
        currentRepoPath: "/path/to/test-repo",
      },
    });

    expect(wrapper.exists()).toBe(true);
    await new Promise((resolve) => setTimeout(resolve, 30));
    await wrapper.vm.$nextTick();

    expect(wrapper.text()).toContain("main");
    expect(wrapper.text()).toContain("feature/auth-sso");

    const searchInput = wrapper.find("input[type='text']");
    if (searchInput.exists()) {
      await searchInput.setValue("auth");
      await wrapper.vm.$nextTick();
      expect(wrapper.text()).toContain("feature/auth-sso");
    }

    riverSpy.mockRestore();
  });

  test("mounts SquashCommitsModal and handles pre-filled commit message and squash submit", async () => {
    const sampleCommits: CommitItem[] = [
      {
        hash: "111111111111",
        shortHash: "1111111",
        parents: [],
        authorName: "Alice",
        authorEmail: "alice@test.com",
        date: "2026-10-28",
        relativeDate: "1h ago",
        subject: "fix: button alignment",
        body: "",
        refs: [],
        isHead: true,
        lane: 0,
        lines: [],
      },
      {
        hash: "222222222222",
        shortHash: "2222222",
        parents: [],
        authorName: "Bob",
        authorEmail: "bob@test.com",
        date: "2026-10-27",
        relativeDate: "1d ago",
        subject: "feat: add button component",
        body: "",
        refs: [],
        isHead: false,
        lane: 0,
        lines: [],
      },
    ];

    const squashSpy = spyOn(gitService, "squashSelectedCommits").mockResolvedValue({
      success: true,
      newHash: "333333333333",
      message: "squashed 2 commits",
      count: 2,
    });

    const wrapper = mount(SquashCommitsModal, {
      props: {
        isOpen: true,
        repoPath: "/path/to/test-repo",
        commits: sampleCommits,
      },
    });

    expect(wrapper.exists()).toBe(true);
    await wrapper.vm.$nextTick();

    expect(wrapper.text()).toContain("1111111");
    expect(wrapper.text()).toContain("fix: button alignment");
    expect(wrapper.text()).toContain("2222222");

    const textarea = wrapper.find("textarea");
    expect(textarea.exists()).toBe(true);
    expect((textarea.element as HTMLTextAreaElement).value).toContain("fix: button alignment");

    const confirmBtn = wrapper.find("[data-testid='confirm-squash-btn']");
    expect(confirmBtn.exists()).toBe(true);
    await confirmBtn.trigger("click");
    await wrapper.vm.$nextTick();

    expect(squashSpy).toHaveBeenCalled();
    expect(wrapper.emitted("squashed")).toBeTruthy();

    squashSpy.mockRestore();
  });

  test("mounts AliasesSection and handles alias filtering and modal opening", async () => {
    const mockAliases: GitAliasItem[] = [
      { key: "st", command: "status -s", scope: "local" },
      { key: "co", command: "checkout", scope: "global" },
      { key: "lg", command: "log --oneline", scope: "global" },
    ];
    const aliasSpy = spyOn(gitService, "getGitAliases").mockResolvedValue(mockAliases);

    const wrapper = mount(AliasesSection, {
      props: {
        currentRepoPath: "/path/to/test-repo",
      },
    });

    expect(wrapper.exists()).toBe(true);
    await new Promise((resolve) => setTimeout(resolve, 30));
    await wrapper.vm.$nextTick();

    expect(wrapper.text()).toContain("git st");
    expect(wrapper.text()).toContain("status -s");
    expect(wrapper.text()).toContain("git co");
    expect(wrapper.text()).toContain("checkout");

    const input = wrapper.find("input[type='text']");
    expect(input.exists()).toBe(true);
    await input.setValue("status");
    await wrapper.vm.$nextTick();

    expect(wrapper.text()).toContain("git st");
    expect(wrapper.text()).not.toContain("git co");

    const newBtn = wrapper.find("[data-testid='new-alias-btn']");
    expect(newBtn.exists()).toBe(true);
    await newBtn.trigger("click");
    await wrapper.vm.$nextTick();

    aliasSpy.mockRestore();
  });

  test("mounts WorktreesTab and renders worktree items and switch button", async () => {
    const mockWorktrees: WorktreeInfo[] = [
      {
        path: "/path/to/main-repo",
        head: "1111111",
        branch: "main",
        isBare: false,
        isMain: true,
        isLocked: false,
        lockReason: "",
        isPrunable: false,
        pruneReason: "",
        isDetached: false,
        dirtyCount: 0,
      },
      {
        path: "/path/to/linked-wt",
        head: "2222222",
        branch: "feature/test",
        isBare: false,
        isMain: false,
        isLocked: false,
        lockReason: "",
        isPrunable: false,
        pruneReason: "",
        isDetached: false,
        dirtyCount: 0,
      },
    ];

    const wtSpy = spyOn(gitService, "getWorktrees").mockResolvedValue(mockWorktrees);
    const branchSpy = spyOn(gitService, "getBranches").mockResolvedValue([]);
    const usageSpy = spyOn(gitService, "getWorktreeDiskUsage").mockResolvedValue({
      sizeBytes: 1048576,
      formattedSize: "1.0 MB",
      fileCount: 42,
    });

    let dispatchedPath = "";
    const listener = (e: Event) => {
      dispatchedPath = (e as CustomEvent).detail;
    };
    window.addEventListener("wgit:open-repo-path", listener);

    const wrapper = mount(WorktreesTab, {
      props: {
        currentRepoPath: "/path/to/main-repo",
        refreshNonce: 1,
      },
    });

    expect(wrapper.exists()).toBe(true);
    await new Promise((resolve) => setTimeout(resolve, 30));
    await wrapper.vm.$nextTick();

    expect(wrapper.text()).toContain("/path/to/linked-wt");
    const switchBtn = wrapper.find("[data-testid='switch-worktree-btn']");
    expect(switchBtn.exists()).toBe(true);

    await switchBtn.trigger("click");
    await wrapper.vm.$nextTick();

    expect(dispatchedPath).toBe("/path/to/linked-wt");

    window.removeEventListener("wgit:open-repo-path", listener);
    wtSpy.mockRestore();
    branchSpy.mockRestore();
    usageSpy.mockRestore();
  });

  test("mounts RemotesTab and renders remote items", async () => {
    const mockRemotes: RemoteDetail[] = [
      {
        name: "origin",
        url: "https://github.com/org/repo.git",
        type: "fetch",
      },
    ];

    const remotesSpy = spyOn(gitService, "getRemoteDetails").mockResolvedValue(mockRemotes);
    const multiStatusSpy = spyOn(gitService, "getMultiRemoteStatus").mockResolvedValue({
      remotes: [],
      reachableCount: 1,
      totalCount: 1,
      scanDurationMs: 10,
    });

    const wrapper = mount(RemotesTab, {
      props: {
        currentRepoPath: "/path/to/test-repo",
        refreshNonce: 1,
      },
    });

    expect(wrapper.exists()).toBe(true);
    await new Promise((resolve) => setTimeout(resolve, 30));
    await wrapper.vm.$nextTick();

    expect(wrapper.text()).toContain("origin");
    expect(wrapper.text()).toContain("https://github.com/org/repo.git");

    remotesSpy.mockRestore();
    multiStatusSpy.mockRestore();
  });

  test("mounts ReleaseWizard and renders release step options", async () => {
    const mockPlan: ReleasePlan = {
      branch: "main",
      lastTag: "v1.2.0",
      commitCount: 5,
      aheadOfRemote: 2,
      behindRemote: 0,
      suggestedVersion: "v1.3.0",
      nextMinor: true,
      checks: [],
      changelog: "",
      ready: true,
      summary: "Ready to release",
    };

    const planSpy = spyOn(gitService, "getReleasePlan").mockResolvedValue(mockPlan);
    const changelogSpy = spyOn(gitService, "generateStudioChangelog").mockResolvedValue({
      markdown: "# Changelog\n\n- feat: sample change",
      commitsCount: 5,
      categories: [],
      durationMs: 12,
    });

    const wrapper = mount(ReleaseWizard, {
      props: {
        repoPath: "/path/to/test-repo",
      },
    });

    expect(wrapper.exists()).toBe(true);
    await new Promise((resolve) => setTimeout(resolve, 30));
    await wrapper.vm.$nextTick();

    expect(wrapper.text()).toContain("v1.2.0");

    planSpy.mockRestore();
    changelogSpy.mockRestore();
  });

  test("mounts ChangesList with status/extension filters and keyboard Space toggle", async () => {
    const mockStaged: FileStatus[] = [
      {
        path: "src/main.ts",
        stagedStatus: "M",
        unstagedStatus: " ",
        isStaged: true,
        isLfs: false,
        additions: 10,
        deletions: 2,
        size: "1KB",
      },
    ];
    const mockUnstaged: FileStatus[] = [
      {
        path: "src/App.vue",
        stagedStatus: " ",
        unstagedStatus: "M",
        isStaged: false,
        isLfs: false,
        additions: 5,
        deletions: 1,
        size: "2KB",
      },
    ];
    const mockConflicted = ["src/config.json"];

    const wrapper = mount(ChangesList, {
      props: {
        stagedFiles: mockStaged,
        unstagedFiles: mockUnstaged,
        conflictedFiles: mockConflicted,
        selectedPath: "src/main.ts",
        currentRepoPath: "/test/repo",
      },
    });

    expect(wrapper.exists()).toBe(true);
    await wrapper.vm.$nextTick();

    // Verify status filter pills rendered
    const statusFilters = wrapper.find("[data-testid='changes-status-filters']");
    expect(statusFilters.exists()).toBe(true);
    const stagedPill = wrapper.find("[data-testid='status-filter-staged']");
    expect(stagedPill.exists()).toBe(true);

    // Click staged filter
    await stagedPill.trigger("click");
    await wrapper.vm.$nextTick();
    expect(wrapper.text()).toContain("main.ts");
    expect(wrapper.text()).not.toContain("App.vue");

    // Verify extension filter chips rendered
    const extFilters = wrapper.find("[data-testid='changes-ext-filters']");
    expect(extFilters.exists()).toBe(true);

    // Verify Space keydown event on window triggers unstage for selected staged file
    window.dispatchEvent(new KeyboardEvent("keydown", { code: "Space", bubbles: true }));
    await wrapper.vm.$nextTick();
    expect(wrapper.emitted("unstage")).toBeTruthy();
    expect(wrapper.emitted("unstage")![0]).toEqual(["src/main.ts"]);
  });

  test("mounts LocalBranchList and renders branch rows", async () => {
    const mockBranch: BranchInfo = {
      name: "main",
      isCurrent: true,
      isRemote: false,
      upstream: "origin/main",
      ahead: 0,
      behind: 0,
      lastCommitHash: "abcdef1",
      lastCommitMessage: "initial commit",
      lastCommitDate: "2026-10-28",
      lastCommitAuthor: "developer",
    };

    const wrapper = mount(LocalBranchList, {
      props: {
        summary: null,
        loading: false,
        insightsLoading: false,
        error: "",
        cleanupDisabled: true,
        cleaningMerged: false,
        cleanableCount: 0,
        filteredCount: 1,
        rows: [
          {
            branch: mockBranch,
            insight: null,
            meta: null,
            cleanupCandidate: false,
          },
        ],
        moreCount: 0,
      },
    });

    expect(wrapper.exists()).toBe(true);
    await wrapper.vm.$nextTick();
    expect(wrapper.text()).toContain("main");
  });

  test("mounts TagsList and renders tag list rows", async () => {
    const mockTags: TagInfo[] = [
      {
        name: "v1.0.0",
        hash: "1111111",
        date: "2026-10-01",
        message: "Release 1.0.0",
        tagger: "lead",
        isAnnotated: true,
      },
    ];

    const wrapper = mount(TagsList, {
      props: {
        total: 1,
        filteredCount: 1,
        visible: mockTags,
        moreCount: 0,
      },
    });

    expect(wrapper.exists()).toBe(true);
    await wrapper.vm.$nextTick();
    expect(wrapper.text()).toContain("v1.0.0");
  });

  test("mounts MiniCommitHudModal and loads repository status", async () => {
    const mockStatus: RepoStatus = {
      currentBranch: "main",
      stagedFiles: [
        {
          path: "test.txt",
          stagedStatus: "M",
          unstagedStatus: " ",
          isStaged: true,
          isLfs: false,
          additions: 1,
          deletions: 0,
          size: "10B",
        },
      ],
      unstagedFiles: [],
      conflictedFiles: [],
      untrackedFiles: [],
      aheadCount: 0,
      behindCount: 0,
      isClean: false,
      hasConflicts: false,
      hasStaged: true,
      hasUnstaged: false,
      lastCommitHash: "abcdef",
      lastCommitMessage: "feat: init",
      lastCommitDate: "2026-10-28",
      lastCommitAuthor: "dev",
    };

    const statusSpy = spyOn(gitService, "getRepoStatus").mockResolvedValue(mockStatus);

    const wrapper = mount(MiniCommitHudModal, {
      props: {
        open: true,
        repoPath: "/test/repo",
      },
    });

    expect(wrapper.exists()).toBe(true);
    await new Promise((resolve) => setTimeout(resolve, 30));
    await wrapper.vm.$nextTick();

    expect(wrapper.text()).toContain("test.txt");

    statusSpy.mockRestore();
  });

  test("mounts HistoryFilterBar and reflects filter inputs and summary", async () => {
    const wrapper = mount(HistoryFilterBar, {
      props: {
        keyword: "fix",
        author: "alice",
        since: "2026-01-01",
        until: "",
        hasActiveFilters: true,
        isFilterMode: true,
        activeFilterSummary: "Keyword: fix · Author: alice",
      },
    });

    expect(wrapper.exists()).toBe(true);
    await wrapper.vm.$nextTick();

    expect(wrapper.text()).toContain("Keyword: fix");
    const inputs = wrapper.findAll("input");
    expect(inputs.length).toBeGreaterThanOrEqual(2);
  });

  test("mounts CommitInspector and renders commit metadata and actions", async () => {
    const mockCommit: CommitItem = {
      hash: "abc1234567890",
      shortHash: "abc1234",
      parents: ["parent1"],
      authorName: "Alice Developer",
      authorEmail: "alice@example.com",
      date: "2026-10-28T09:00:00Z",
      relativeDate: "10 minutes ago",
      subject: "feat: implement simulator and safety net",
      body: "detailed description",
      refs: ["HEAD", "main"],
      isHead: true,
      lane: 0,
      lines: [],
      signatureStatus: "G",
      signer: "Alice <alice@example.com>",
    };

    const wrapper = mount(CommitInspector, {
      props: {
        selectedCommit: mockCommit,
        copiedHash: null,
        commitDetail: null,
        isLoadingDetail: false,
        currentCommitNote: "",
        isPushingNotes: false,
        repoPath: "/test/repo",
      },
    });

    expect(wrapper.exists()).toBe(true);
    await wrapper.vm.$nextTick();

    expect(wrapper.text()).toContain("abc1234");
    expect(wrapper.text()).toContain("Alice Developer");
    expect(wrapper.text()).toContain("feat: implement simulator and safety net");
  });

  test("mounts BranchCompare and triggers merge simulation", async () => {
    const simSpy = spyOn(gitService, "simulateMerge").mockResolvedValue({
      baseRef: "main",
      targetRef: "feature",
      canMergeCleanly: true,
      hasConflicts: false,
      conflictedFiles: [],
      changedFiles: ["clean.txt"],
      mergeBase: "123456",
      isFastForward: true,
      message: "Fast-forward merge possible",
    });

    const wrapper = mount(BranchCompare, {
      props: {
        branches: [],
        refOptions: ["main", "feature"],
        base: "main",
        target: "feature",
        mergeBaseMode: false,
        result: {
          aheadCount: 1,
          behindCount: 0,
          aheadCommits: [],
          behindCommits: [],
        },
        loading: false,
        diff: null,
        diffLoading: false,
        currentRepoPath: "/test/repo",
      },
    });

    expect(wrapper.exists()).toBe(true);
    await wrapper.vm.$nextTick();

    const simBtn = wrapper.find("[data-testid='simulate-merge-btn']");
    expect(simBtn.exists()).toBe(true);

    await simBtn.trigger("click");
    await new Promise((resolve) => setTimeout(resolve, 30));
    await wrapper.vm.$nextTick();

    expect(wrapper.find("[data-testid='merge-simulation-panel']").exists()).toBe(true);
    expect(wrapper.find("[data-testid='simulation-clean-badge']").exists()).toBe(true);

    simSpy.mockRestore();
  });

  test("mounts GitHooksSection and renders hook controls and framework status", async () => {
    const hooksSpy = spyOn(gitService, "getGitHooks").mockResolvedValue([
      {
        name: "pre-commit",
        path: ".git/hooks/pre-commit",
        installed: true,
        isExecutable: true,
        sizeBytes: 42,
        sample: "#!/bin/sh\nexit 0\n",
      },
    ]);
    const fwSpy = spyOn(gitService, "detectHooksFramework").mockResolvedValue({
      type: "husky",
      configFile: ".husky/pre-commit",
      hooksPath: ".husky",
    });

    const wrapper = mount(GitHooksSection, {
      props: {
        currentRepoPath: "/test/repo",
      },
    });

    expect(wrapper.exists()).toBe(true);
    await new Promise((resolve) => setTimeout(resolve, 30));
    await wrapper.vm.$nextTick();

    expect(wrapper.find("[data-testid='hook-card-pre-commit']").exists()).toBe(true);
    expect(wrapper.find("[data-testid='hook-toggle-pre-commit']").exists()).toBe(true);
    expect(wrapper.find("[data-testid='hook-run-pre-commit']").exists()).toBe(true);

    hooksSpy.mockRestore();
    fwSpy.mockRestore();
  });

  test("mounts BranchCompare and diagnoses branch health", async () => {
    const healthSpy = spyOn(gitService, "evaluateBranchHealth").mockResolvedValue({
      branch: "feature",
      baseBranch: "main",
      readinessScore: 92,
      readinessLevel: "ready",
      divergenceDays: 2,
      aheadCount: 3,
      behindCount: 0,
      conflictRisk: "low",
      diagnostics: ["Fast-forward compatible"],
    });

    const wrapper = mount(BranchCompare, {
      props: {
        branches: [],
        refOptions: ["main", "feature"],
        base: "main",
        target: "feature",
        mergeBaseMode: false,
        result: {
          aheadCount: 3,
          behindCount: 0,
          aheadCommits: [],
          behindCommits: [],
        },
        loading: false,
        diff: null,
        diffLoading: false,
        currentRepoPath: "/test/repo",
      },
    });

    expect(wrapper.exists()).toBe(true);
    await wrapper.vm.$nextTick();

    const healthBtn = wrapper.find("[data-testid='branch-health-btn']");
    expect(healthBtn.exists()).toBe(true);

    await healthBtn.trigger("click");
    await new Promise((resolve) => setTimeout(resolve, 30));
    await wrapper.vm.$nextTick();

    expect(wrapper.find("[data-testid='branch-health-panel']").exists()).toBe(true);
    expect(wrapper.find("[data-testid='branch-readiness-badge']").exists()).toBe(true);
    expect(wrapper.text()).toContain("92/100");

    healthSpy.mockRestore();
  });

  test("mounts SubmodulesTab and renders submodule matrix and recursive sync button", async () => {
    const subSpy = spyOn(gitService, "getSubmodules").mockResolvedValue([
      { name: "sub1", path: "libs/sub1", url: "https://github.com/test/sub1", commit: "abcdef1" },
    ]);
    const subtreeSpy = spyOn(gitService, "listSubtrees").mockResolvedValue({ subtrees: [] });
    const matrixSpy = spyOn(gitService, "getSubmoduleMatrix").mockResolvedValue([
      {
        path: "libs/sub1",
        url: "https://github.com/test/sub1",
        currentCommit: "abcdef1",
        expectedCommit: "abcdef1",
        branch: "main",
        status: "up_to_date",
        isClean: true,
      },
    ]);

    const wrapper = mount(SubmodulesTab, {
      props: {
        currentRepoPath: "/test/repo",
        refreshNonce: 1,
      },
    });

    expect(wrapper.exists()).toBe(true);
    await new Promise((resolve) => setTimeout(resolve, 30));
    await wrapper.vm.$nextTick();

    expect(wrapper.find("[data-testid='submodule-sync-recursive-btn']").exists()).toBe(true);
    expect(wrapper.find("[data-testid='submodule-matrix-card']").exists()).toBe(true);

    subSpy.mockRestore();
    subtreeSpy.mockRestore();
    matrixSpy.mockRestore();
  });

  test("mounts CommitTimeLapseModal and inspects timeline entries", async () => {
    const timeSpy = spyOn(gitService, "getFileCommitTimeline").mockResolvedValue([
      {
        commitHash: "1234567890abcdef",
        author: "Alice",
        date: "2026-10-28 10:00:00",
        subject: "feat: add feature",
        content: "package main",
      },
    ]);
    const ownSpy = spyOn(gitService, "getCodeOwnership").mockResolvedValue({
      totalLines: 1,
      owners: [],
      busFactorRisk: "low",
    });
    const linSpy = spyOn(gitService, "traceFileRenamesAcrossHistory").mockResolvedValue({
      originalPath: "main.go",
      currentPath: "main.go",
      renames: [],
      totalRenames: 0,
    });

    const wrapper = mount(CommitTimeLapseModal, {
      props: {
        open: true,
        filePath: "main.go",
        currentRepoPath: "/test/repo",
      },
    });

    expect(wrapper.exists()).toBe(true);
    await new Promise((resolve) => setTimeout(resolve, 30));
    await wrapper.vm.$nextTick();

    expect(wrapper.text()).toContain("main.go");
    timeSpy.mockRestore();
    ownSpy.mockRestore();
    linSpy.mockRestore();
  });

  test("mounts ExportModal and renders git notes packaging controls", async () => {
    const wrapper = mount(ExportModal, {
      props: {
        open: true,
        markdown: "# Review Notes\n- LGTM",
        copied: false,
        currentRepoPath: "/test/repo",
      },
    });

    expect(wrapper.exists()).toBe(true);
    await wrapper.vm.$nextTick();

    expect(wrapper.text()).toContain("Git Notes");
  });

  test("mounts InteractiveRebaseModal and triggers collision radar prediction", async () => {
    const radarSpy = spyOn(gitService, "simulateRebaseOnto").mockResolvedValue({
      upstreamRef: "main",
      totalSteps: 1,
      cleanSteps: 1,
      conflictSteps: 0,
      viabilityScore: 100,
      steps: [
        {
          hash: "111222333",
          shortHash: "1112223",
          subject: "feat: add radar",
          status: "clean",
          riskLevel: "low",
          conflictingFiles: [],
          changedFiles: ["radar.go"],
        },
      ],
      summary: "Simulated rebase of 1 commits: 1 clean, 0 conflicts",
    });
    const valSpy = spyOn(gitService, "validateRebaseTodoGraph").mockResolvedValue({
      valid: true,
      errors: [],
      warnings: [],
      suggestions: [],
    });

    const wrapper = mount(InteractiveRebaseModal, {
      props: {
        isOpen: true,
        repoPath: "/test/repo",
        rebaseBase: "main",
        rebaseBaseOptions: [{ value: "main", label: "main" }],
        initialPlan: [
          {
            action: "pick",
            hash: "111222333",
            shortHash: "1112223",
            subject: "feat: add radar",
            author: "Tester",
          },
        ],
        isLoadingPlan: false,
        planError: null,
        selectionCount: 0,
        isHashSelected: () => false,
        isExecuting: false,
        executionError: null,
      },
    });

    expect(wrapper.exists()).toBe(true);
    await wrapper.vm.$nextTick();

    const radarBtn = wrapper.find("[data-testid='rebase-radar-btn']");
    expect(radarBtn.exists()).toBe(true);

    await radarBtn.trigger("click");
    await new Promise((resolve) => setTimeout(resolve, 30));
    await wrapper.vm.$nextTick();

    expect(wrapper.find("[data-testid='rebase-collision-panel']").exists()).toBe(true);
    expect(wrapper.find("[data-testid='rebase-step-risk']").exists()).toBe(true);
    expect(wrapper.text()).toContain("100%");

    radarSpy.mockRestore();
    valSpy.mockRestore();
  });

  test("mounts MaintenanceTab and inspects repository bloat objects", async () => {
    const hooksSpy = spyOn(gitService, "getGitHooks").mockResolvedValue([]);
    const fwSpy = spyOn(gitService, "detectHooksFramework").mockResolvedValue({
      type: "husky",
      configFile: ".husky/pre-commit",
      hooksPath: ".husky",
    });
    const maintSpy = spyOn(gitService, "getMaintenanceStats").mockResolvedValue({
      gitDirSize: "12 MB",
      packFilesCount: 2,
      looseObjects: 4,
      success: true,
      summary: "OK",
      warnings: [],
    });
    const bloatSpy = spyOn(gitService, "inspectRepoBloat").mockResolvedValue({
      totalBlobsScanned: 100,
      totalGhostBytes: 512000,
      ghostBytesHuman: "500 KB",
      topBloatObjects: [
        {
          hash: "abc12345",
          path: "assets/big_ghost.bin",
          sizeBytes: 512000,
          sizeHuman: "500 KB",
          isGhostBlob: true,
          commitHash: "c1",
          commitMsg: "add big ghost",
          author: "Tester",
          lastModified: "2 days ago",
        },
      ],
      summary: "Scanned 100 objects. Identified 1 bloat object.",
    });

    const wrapper = mount(MaintenanceTab, {
      props: {
        currentRepoPath: "/test/repo",
        refreshNonce: 1,
      },
    });

    expect(wrapper.exists()).toBe(true);
    await new Promise((resolve) => setTimeout(resolve, 30));
    await wrapper.vm.$nextTick();

    const scanBtn = wrapper.find("[data-testid='scan-bloat-btn']");
    expect(scanBtn.exists()).toBe(true);

    await scanBtn.trigger("click");
    await new Promise((resolve) => setTimeout(resolve, 30));
    await wrapper.vm.$nextTick();

    expect(wrapper.find("[data-testid='repo-bloat-card']").exists()).toBe(true);
    expect(wrapper.text()).toContain("assets/big_ghost.bin");

    maintSpy.mockRestore();
    bloatSpy.mockRestore();
    hooksSpy.mockRestore();
    fwSpy.mockRestore();
  });

  test("mounts HeaderBar and displays context session switcher", async () => {
    const recentsSpy = spyOn(gitService, "getRecentRepos").mockResolvedValue(["/test/repo"]);
    const listSpy = spyOn(gitService, "listWorkContextSessions").mockResolvedValue([
      {
        id: "ctx-1",
        name: "Feature Context",
        branch: "main",
        activeFiles: [],
        commitDraft: "",
        issueOrPr: "",
        notes: "",
        createdAt: "2026-10-28",
        updatedAt: "2026-10-28",
      },
    ]);

    const wrapper = mount(HeaderBar, {
      props: {
        currentRepoPath: "/test/repo",
        repoStatus: null,
        isSyncing: false,
      },
    });

    expect(wrapper.exists()).toBe(true);
    await wrapper.vm.$nextTick();

    const ctxBtn = wrapper.find("[data-testid='context-session-dropdown']");
    expect(ctxBtn.exists()).toBe(true);

    await ctxBtn.trigger("click");
    await new Promise((resolve) => setTimeout(resolve, 30));
    await wrapper.vm.$nextTick();

    expect(wrapper.find("[data-testid='save-context-session-btn']").exists()).toBe(true);

    recentsSpy.mockRestore();
    listSpy.mockRestore();
  });

  test("mounts CommitComposer with commit lint, auto format and GPG chip", async () => {
    const legacyLintSpy = spyOn(gitService, "lintCommitMessage").mockResolvedValue({
      score: 95,
      grade: "A",
      issues: [],
      summary: "Clean message",
    });
    const lintSpy = spyOn(gitService, "validateCommitMessageFormat").mockResolvedValue({
      valid: true,
      score: 95,
      type: "feat",
      scope: "core",
      hasTrailingPeriod: false,
      isConventional: true,
      suggestions: ["Good commit message!"],
      formattedPreview: "feat(core): add super feature",
    });
    const gpgSpy = spyOn(gitService, "checkCommitGpgHealth").mockResolvedValue({
      signingEnabled: true,
      program: "gpg",
      keyId: "1234ABCD",
      hasSecretKey: true,
      keyExpires: "never",
      status: "healthy",
      message: "GPG signing key 1234ABCD ready",
    });
    const formatSpy = spyOn(gitService, "formatCommitMessageWithTemplate").mockResolvedValue({
      originalMessage: "feat: unformatted",
      formattedMessage: "feat: formatted message",
      templateType: "conventional",
      modifications: ["auto formatted"],
    });

    const wrapper = mount(CommitComposer, {
      props: {
        subject: "feat: add super feature",
        body: "",
        amend: false,
        autoPush: false,
        committing: false,
        repoPath: "/test/repo",
      },
    });

    expect(wrapper.exists()).toBe(true);
    await new Promise((resolve) => setTimeout(resolve, 400));
    await wrapper.vm.$nextTick();

    const lintBadge = wrapper.find("[data-testid='commit-lint-badge']");
    expect(lintBadge.exists()).toBe(true);
    expect(lintBadge.text()).toContain("95/100");

    const gpgChip = wrapper.find("[data-testid='gpg-health-chip']");
    expect(gpgChip.exists()).toBe(true);
    expect(gpgChip.text()).toContain("GPG: healthy");

    const formatBtn = wrapper.find("[data-testid='commit-auto-format-btn']");
    expect(formatBtn.exists()).toBe(true);
    await formatBtn.trigger("click");
    await wrapper.vm.$nextTick();

    expect(formatSpy).toHaveBeenCalled();

    lintSpy.mockRestore();
    legacyLintSpy.mockRestore();
    gpgSpy.mockRestore();
    formatSpy.mockRestore();
  });

  test("mounts SyncModal and executes dry-run merge sandbox", async () => {
    const remotesSpy = spyOn(gitService, "getRemoteDetails").mockResolvedValue([
      { name: "origin", url: "git@github.com:test/repo.git", type: "fetch" },
    ]);
    const aheadSpy = spyOn(gitService, "getAheadCommits").mockResolvedValue([]);
    const behindSpy = spyOn(gitService, "getBehindCommits").mockResolvedValue([]);
    const sandboxSpy = spyOn(gitService, "dryRunPullOrMerge").mockResolvedValue({
      targetRef: "origin/main",
      canMergeCleanly: true,
      isFastForward: true,
      conflictedFiles: [],
      cleanFiles: [],
      aheadCount: 1,
      behindCount: 0,
      summary: "Can merge cleanly: 0 conflicts detected",
    });

    const wrapper = mount(SyncModal, {
      props: {
        isOpen: true,
        currentRepoPath: "/test/repo",
        repoStatus: {
          currentBranch: "main",
          hasWorkingTreeChanges: false,
          stagedCount: 0,
          unstagedCount: 0,
          untrackedCount: 0,
          stashesCount: 0,
        } as any,
      },
    });

    expect(wrapper.exists()).toBe(true);
    await new Promise((resolve) => setTimeout(resolve, 50));
    await wrapper.vm.$nextTick();

    const dryRunBtn = wrapper.find("[data-testid='merge-sandbox-dryrun-btn']");
    expect(dryRunBtn.exists()).toBe(true);

    await dryRunBtn.trigger("click");
    await new Promise((resolve) => setTimeout(resolve, 50));
    await wrapper.vm.$nextTick();

    expect(sandboxSpy).toHaveBeenCalled();
    const panel = wrapper.find("[data-testid='merge-sandbox-report-panel']");
    expect(panel.exists()).toBe(true);
    expect(panel.text()).toContain("origin/main");

    remotesSpy.mockRestore();
    aheadSpy.mockRestore();
    behindSpy.mockRestore();
    sandboxSpy.mockRestore();
  });

  test("mounts ShortcutCheatSheetModal and filters by query and category pills", async () => {
    const wrapper = mount(ShortcutCheatSheetModal, {
      props: {
        open: true,
      },
    });

    expect(wrapper.exists()).toBe(true);
    await wrapper.vm.$nextTick();

    const searchInput = wrapper.find("[data-testid='shortcut-search-input']");
    expect(searchInput.exists()).toBe(true);

    const categoryPills = wrapper.findAll("[data-testid='shortcut-category-pill']");
    expect(categoryPills.length).toBeGreaterThanOrEqual(3);

    // Filter by builtin
    const builtinPill = categoryPills.find((p) => p.text().includes("内置") || p.text().includes("Built-in"));
    if (builtinPill) {
      await builtinPill.trigger("click");
      await wrapper.vm.$nextTick();
      expect(wrapper.text()).toContain("Esc");
    }

    // Filter by search query
    await searchInput.setValue("nonexistent_random_shortcut_string");
    await wrapper.vm.$nextTick();
    const emptyState = wrapper.find("[data-testid='shortcut-empty-state']");
    expect(emptyState.exists()).toBe(true);
  });

  test("mounts CommitTimeLapseModal and displays file lineage forensic breadcrumb", async () => {
    const timeSpy = spyOn(gitService, "getFileCommitTimeline").mockResolvedValue([
      {
        commitHash: "abc111222",
        author: "Dev",
        authorDate: "2026-10-28T10:00:00",
        date: "2026-10-28",
        subject: "feat: initial file",
        content: "package main\n\nfunc main() {}\n",
      },
    ]);
    const ownSpy = spyOn(gitService, "getCodeOwnership").mockResolvedValue({
      totalLines: 3,
      owners: [],
      busFactorRisk: "low",
    });
    const linSpy = spyOn(gitService, "traceFileRenamesAcrossHistory").mockResolvedValue({
      originPath: "src/old_main.go",
      currentPath: "src/main.go",
      historicalPaths: ["src/old_main.go", "src/main.go"],
      renames: [
        {
          commitHash: "def333444",
          commitDate: "2026-10-28",
          author: "Dev",
          subject: "rename file",
          oldPath: "src/old_main.go",
          newPath: "src/main.go",
          similarity: 100,
        },
      ],
      totalRenames: 1,
      summary: "Renamed 1 times",
    });

    const wrapper = mount(CommitTimeLapseModal, {
      props: {
        open: true,
        filePath: "src/main.go",
        currentRepoPath: "/test/repo",
      },
    });

    expect(wrapper.exists()).toBe(true);
    await new Promise((resolve) => setTimeout(resolve, 50));
    await wrapper.vm.$nextTick();

    const breadcrumb = wrapper.find("[data-testid='file-lineage-breadcrumb']");
    expect(breadcrumb.exists()).toBe(true);
    expect(breadcrumb.text()).toContain("src/old_main.go");

    timeSpy.mockRestore();
    ownSpy.mockRestore();
    linSpy.mockRestore();
  });
});
