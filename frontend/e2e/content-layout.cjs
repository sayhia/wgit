#!/usr/bin/env node
/**
 * Data-bearing history, branch, review and stash layouts at desktop minimum sizes.
 * Runs against the Vite dev preview: fixtures replace only methods on that
 * page's imported service object and never reach a real repository or host.
 */
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const { chromium } = require("playwright");

const BASE = process.env.BASE_URL || "http://127.0.0.1:9245/";
const OUT = path.join(__dirname, "out", "content-layout");
const STASH_MESSAGE = "WIP on feature/long-branch: snapshot with a much longer descriptive message";
const ACCOUNT = "studio-developer-with-a-long-account-name";
const LONG_BRANCH = `feature/${"long-subsystem-name-".repeat(7)}`;
const LONG_REMOTE = `origin/${"remote-feature-name-".repeat(7)}`;
const LONG_SUBJECT = `feat(layout): ${"Keep repository history readable / 保持提交历史可读 ".repeat(8)}`;
const CASES = [
  { width: 960, height: 640 },
  { width: 1280, height: 800 },
];

async function installFixtures(page) {
  await page.evaluate(async ({ stashMessage, account, longBranch, longRemote, longSubject }) => {
    const { gitService } = await import("/src/services/gitService.ts");
    const originalCommits = gitService.getCommits;
    gitService.getCommits = async (...args) => (await originalCommits(...args)).map((commit, index) =>
      index < 2 ? { ...commit, subject: `${longSubject} #${index}` } : commit
    );
    gitService.getBranches = async () => [
      { name: "main", isCurrent: true, isRemote: false, ahead: 1, behind: 0, upstream: "origin/main", commitHash: "a1b2c3d" },
      { name: longBranch, isCurrent: false, isRemote: false, ahead: 3, behind: 1, upstream: longRemote, commitHash: "b2c3d4e" },
      { name: longRemote, isCurrent: false, isRemote: true, ahead: 0, behind: 0, commitHash: "b2c3d4e" },
    ];
    gitService.getStashes = async () => [{ index: 0, message: stashMessage, date: "2026-10-31 15:40" }];
    gitService.getStashDiff = async () => "diff --git a/a.ts b/a.ts\n@@ -1 +1 @@\n-a\n+b";
    gitService.getStashFiles = async () => [{ path: "a.ts", status: "M", additions: 1, deletions: 1 }];
    gitService.getPullRequestProvider = async () => ({
      supported: true, provider: "github", hasToken: true, account,
      tokenSource: "Repository token", tokenHint: "Browser layout fixture",
      projectPath: "studio/large-project-main-repository",
      remoteUrl: "https://github.com/studio/large-project-main-repository",
      webBaseUrl: "https://github.com/studio/large-project-main-repository",
    });
    const summary = {
      number: 17, title: "Keep large repository history navigable in narrow windows",
      state: "open", isDraft: false, author: account,
      sourceBranch: "feature/long-branch-name", targetBranch: "main",
      createdAt: "2026-10-30T00:00:00Z", updatedAt: "2026-10-31T00:00:00Z",
      commentCount: 1, additions: 12, deletions: 3, changedFiles: 1,
      webUrl: "https://github.com/studio/large-project-main-repository/pull/17",
      labels: [], checksState: "success", mergeable: true, mergeableState: "clean",
    };
    gitService.listPullRequests = async () => ({ items: [summary], hasMore: false, total: 1 });
    gitService.getPullRequestDetail = async () => ({
      summary, body: "Description\n".repeat(150), files: [], comments: [], checks: [], commitCount: 1,
    });
  }, { stashMessage: STASH_MESSAGE, account: ACCOUNT, longBranch: LONG_BRANCH, longRemote: LONG_REMOTE, longSubject: LONG_SUBJECT });
}

(async () => {
  fs.mkdirSync(OUT, { recursive: true });
  const browser = await chromium.launch({ channel: "chrome" });
  try {
    for (const viewport of CASES) {
      for (const locale of ["zh-CN", "en-US"]) {
        const page = await browser.newPage({ viewport });
        const label = `${viewport.width}-${locale}`;
        const english = locale === "en-US";
        await page.addInitScript((value) => localStorage.setItem("wgit.locale", value), locale);
        await page.goto(BASE, { waitUntil: "networkidle" });
        await installFixtures(page);

        await page.getByLabel(english ? "Commit Graph & History" : "提交图谱 & 历史", { exact: true }).click();
        const commitRows = page.locator('[role="listbox"] [role="option"]');
        await commitRows.first().waitFor({ state: "visible" });
        const density = await commitRows.first().evaluate((element) => {
          const graph = element.querySelector("svg[height]");
          const subject = element.querySelector("[title^='feat(layout):']");
          return { height: element.getBoundingClientRect().height, graphHeight: Number(graph.getAttribute("height")), subjectHeight: subject.getBoundingClientRect().height };
        });
        assert.equal(density.height, 40, `${label}: history rows lost their compact single-line measure`);
        assert.equal(density.graphHeight, density.height, `${label}: DAG geometry disagrees with the row`);
        assert.ok(density.subjectHeight <= 20, `${label}: long multilingual subject wraps inside a virtual row`);
        await commitRows.nth(0).getByRole("checkbox").click({ force: true });
        await commitRows.nth(1).getByRole("checkbox").click({ force: true });
        const batch = page.getByTestId("history-batch-actions");
        await batch.waitFor({ state: "visible" });
        const batchLayout = await batch.evaluate((element) => {
          const bounds = element.getBoundingClientRect();
          return [...element.querySelectorAll("button")].map((button) => {
            const box = button.getBoundingClientRect();
            return { text: button.textContent.trim(), height: box.height, inside: box.left >= bounds.left && box.right <= bounds.right + 1 && box.bottom <= bounds.bottom + 1 };
          });
        });
        assert.equal(batchLayout.length, 7, `${label}: a batch action disappeared`);
        assert.ok(batchLayout.every((button) => button.inside && button.height <= 34), `${label}: batch labels wrap or actions are clipped: ${JSON.stringify(batchLayout)}`);
        await page.screenshot({ path: path.join(OUT, `history-selection-${label}.png`) });
        await batch.getByRole("button", { name: english ? "Clear selection" : "清空选择", exact: true }).click();
        assert.equal(await batch.count(), 0, `${label}: clear selection did not clear the batch state`);
        // Keyboard navigation must still scroll selected commits into view
        // after the row/DAG measure changes; do not scroll the locator for it.
        for (let index = 0; index < 45; index++) await page.keyboard.press("j");
        const selection = await page.locator('[role="listbox"] [role="option"][aria-selected="true"]').evaluate((element) => {
          const viewport = element.parentElement.parentElement.getBoundingClientRect();
          const row = element.getBoundingClientRect();
          const pad = element.parentElement.firstElementChild;
          return { visible: row.top >= viewport.top - 1 && row.bottom <= viewport.bottom + 1, rowHeight: row.height, padHeight: pad.getBoundingClientRect().height, offset: element.offsetTop };
        });
        assert.ok(selection.visible, `${label}: keyboard selection fell outside the history viewport`);
        assert.equal(selection.rowHeight, 40);
        assert.ok(selection.padHeight > 0, `${label}: navigation did not exercise a virtualized scroll window`);
        assert.equal(selection.padHeight % 40, 0, `${label}: virtual padding lost alignment`);
        assert.equal(selection.offset % 40, 0, `${label}: selected row lost virtual alignment`);

        await page.getByLabel(english ? "Branches & Tags" : "分支 & 标签", { exact: true }).click();
        const localName = page.getByTitle(LONG_BRANCH, { exact: true });
        await localName.waitFor({ state: "visible" });
        const localLayout = await localName.evaluate((element) => ({
          nameHeight: element.getBoundingClientRect().height,
          rowHeight: element.closest("[tabindex]").getBoundingClientRect().height,
          isTruncated: element.scrollWidth > element.clientWidth,
        }));
        assert.ok(localLayout.isTruncated && localLayout.nameHeight <= 20 && localLayout.rowHeight <= 96, `${label}: a long branch expands into a paragraph`);
        await page.screenshot({ path: path.join(OUT, `long-branch-${label}.png`) });
        await page.getByRole("button", { name: english ? "Remote-tracking (1)" : "远程跟踪 (1)", exact: true }).click();
        const remoteName = page.getByTitle(LONG_REMOTE, { exact: true });
        await remoteName.waitFor({ state: "visible" });
        const remoteLayout = await remoteName.evaluate((element) => ({
          nameHeight: element.getBoundingClientRect().height,
          rowHeight: element.closest("[tabindex]").getBoundingClientRect().height,
          isTruncated: element.scrollWidth > element.clientWidth,
        }));
        assert.ok(remoteLayout.isTruncated && remoteLayout.nameHeight <= 20 && remoteLayout.rowHeight <= 96, `${label}: a long remote branch expands into a paragraph`);
        await page.screenshot({ path: path.join(OUT, `long-remote-${label}.png`) });

        await page.getByLabel(english ? "Change Review" : "评审", { exact: true }).click();
        const pane = page.locator("main main");
        const note = pane.locator("textarea").first();
        await note.waitFor({ state: "visible" });
        const review = await pane.evaluate((element) => {
          const boxes = [...element.querySelectorAll("textarea")].map((input) => input.getBoundingClientRect());
          const diff = element.querySelector(".overflow-auto");
          const overflow = [...element.querySelectorAll("div")].filter((row) =>
            row.classList.contains("flex") && !row.classList.contains("overflow-auto") &&
            row.clientWidth > 0 && row.scrollWidth > row.clientWidth + 2
          ).map((row) => ({ text: row.textContent.trim().slice(0, 80), width: row.clientWidth, scrollWidth: row.scrollWidth }));
          return { noteWidth: boxes[0].width, commentWidth: boxes[1].width, diffHeight: diff.clientHeight, overflow };
        });
        assert.ok(review.noteWidth >= 200, `${label}: note squeezed to ${review.noteWidth}px`);
        assert.ok(review.commentWidth >= 150, `${label}: comment squeezed to ${review.commentWidth}px`);
        assert.ok(review.diffHeight >= 100, `${label}: diff starved to ${review.diffHeight}px`);
        assert.deepEqual(review.overflow, [], `${label}: review metadata overflows`);
        await page.screenshot({ path: path.join(OUT, `review-${label}.png`) });

        await page.getByLabel(english ? "Stashes & Snapshots" : "储藏箱 & 快照", { exact: true }).click();
        const title = page.getByText(STASH_MESSAGE, { exact: true }).last();
        await title.waitFor({ state: "visible" });
        const stash = await title.evaluate((element) => {
          const header = element.parentElement.parentElement;
          const rect = header.getBoundingClientRect();
          return {
            titleWidth: element.getBoundingClientRect().width,
            overflow: header.scrollWidth > header.clientWidth + 1,
            clippedActions: [...header.querySelectorAll("button")].filter((button) => {
              const box = button.getBoundingClientRect();
              return box.left < rect.left - 1 || box.right > rect.right + 1 || box.bottom > rect.bottom + 1;
            }).map((button) => button.textContent.trim()),
          };
        });
        assert.ok(stash.titleWidth >= 100, `${label}: selected stash identity disappeared`);
        assert.equal(stash.overflow, false, `${label}: stash toolbar overflows`);
        assert.deepEqual(stash.clippedActions, [], `${label}: stash actions clipped`);
        await page.screenshot({ path: path.join(OUT, `stash-${label}.png`) });

        await page.getByLabel(english ? "Change Review" : "评审", { exact: true }).click();
        await page.getByRole("button", { name: english ? "Remote PR" : "远端 PR", exact: true }).click();
        const account = page.locator(`[title="${ACCOUNT}"]`);
        await account.waitFor({ state: "visible" });
        const remote = await account.evaluate((element) => {
          const button = element.closest("button");
          const header = button.closest("header");
          const tokenBox = button.getBoundingClientRect();
          const headerBox = header.getBoundingClientRect();
          return {
            tokenHeight: tokenBox.height,
            tokenWithinHeader: tokenBox.bottom <= headerBox.bottom + 1,
            headerOverflow: header.scrollWidth > header.clientWidth + 1,
          };
        });
        assert.ok(remote.tokenHeight <= 36, `${label}: long account wraps token button to ${remote.tokenHeight}px`);
        assert.equal(remote.tokenWithinHeader, true, `${label}: token button exceeds header`);
        assert.equal(remote.headerOverflow, false, `${label}: remote header overflows`);
        await page.getByText("Keep large repository history navigable in narrow windows", { exact: true }).click();
        const tabs = page.getByRole("tablist");
        await tabs.waitFor({ state: "visible" });
        const remoteDetail = await tabs.evaluate((element) => {
          const section = element.closest("section");
          const content = section.querySelector(".overflow-y-auto");
          return { bottom: section.getBoundingClientRect().bottom, contentHeight: content.clientHeight, contentScrollHeight: content.scrollHeight };
        });
        assert.ok(remoteDetail.bottom <= viewport.height + 1, `${label}: remote detail extends below window`);
        assert.ok(remoteDetail.contentHeight > 0 && remoteDetail.contentScrollHeight > remoteDetail.contentHeight, `${label}: remote detail must scroll within the window`);
        await page.screenshot({ path: path.join(OUT, `remote-${label}.png`) });
        console.log(`  ✓ ${label}: 40px history and keyboard scrolling; batch actions fit; long refs fit; note ${Math.round(review.noteWidth)}px; stash/PR actions fit`);
        await page.close();
      }
    }
  } finally {
    await browser.close();
  }
  console.log("content layout audit passed: history, long branch refs, local reviews, stashes and remote PRs fit 960/1280 windows");
})().catch((error) => {
  console.error(error);
  process.exit(1);
});
