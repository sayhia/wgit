#!/usr/bin/env node
/**
 * AI end-to-end smoke: real wgit-server + mock AI + fake CLI + Playwright.
 *
 * This is the only test that exercises the Wails event channel, the real
 * frontend settings/composer, and the Go AI transports together. It is kept out
 * of `bun run audit` because it needs the built server binary and starts a
 * temporary app process:
 *
 *   wails3 task build:server
 *   bun run audit:ai
 *
 * Everything runs in a temp config dir / temp repo; the real Keychain and the
 * user's config are never touched.
 */
const { chromium } = require("playwright");
const { PNG } = require("pngjs");
const http = require("node:http");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const { execFileSync, spawn } = require("node:child_process");
const net = require("node:net");

const REPO_ROOT = path.resolve(__dirname, "../..");
const BINARY = path.join(REPO_ROOT, "bin/wgit-server");
const VISUAL_BASELINES = path.join(__dirname, "baselines");
const VISUAL_OUT = path.join(__dirname, "out");
const VISUAL_UPDATE = process.env.AI_VISUAL_UPDATE === "1";
// Font antialiasing differs slightly across machines; a layout/palette change
// moves far more pixels than this.
const VISUAL_TOLERANCE_RATIO = 0.03;

function differingPixels(a, b) {
  const left = PNG.sync.read(a);
  const right = PNG.sync.read(b);
  if (left.width !== right.width || left.height !== right.height) return -1;
  let count = 0;
  for (let i = 0; i < left.data.length; i += 4) {
    if (
      left.data[i] !== right.data[i] ||
      left.data[i + 1] !== right.data[i + 1] ||
      left.data[i + 2] !== right.data[i + 2]
    ) {
      count++;
    }
  }
  return count;
}

function freePort() {
  return new Promise((resolve, reject) => {
    const server = net.createServer();
    server.once("error", reject);
    server.listen(0, "127.0.0.1", () => {
      const port = server.address().port;
      server.close(() => resolve(port));
    });
  });
}

function startMockAI() {
  return new Promise((resolve) => {
    const events = [];
    const server = http.createServer((req, res) => {
      const url = new URL(req.url, "http://127.0.0.1");
      if (req.method === "GET" && url.pathname.endsWith("/models")) {
        res.writeHead(200, { "Content-Type": "application/json" });
        res.end(JSON.stringify({ data: [{ id: "z-model" }, { id: "a-model" }] }));
        return;
      }
      if (req.method === "POST" && url.pathname.endsWith("/chat/completions")) {
        if (url.pathname.includes("/hang/")) {
          events.push("hang");
          return;
        }
        if (url.pathname.includes("/unauthorized/")) {
          res.writeHead(401, { "Content-Type": "application/json" });
          res.end(JSON.stringify({ error: { message: "unauthorized" } }));
          return;
        }
        if (url.pathname.includes("/flaky/")) {
          events.push("flaky");
          if (events.filter((item) => item === "flaky").length === 1) {
            res.writeHead(500, { "Content-Type": "application/json" });
            res.end(JSON.stringify({ error: { message: "flaky failure" } }));
            return;
          }
        }
        if (url.pathname.includes("/fail/")) {
          res.writeHead(500, { "Content-Type": "application/json" });
          res.end(JSON.stringify({ error: { message: "mock failure" } }));
          return;
        }
        res.writeHead(200, { "Content-Type": "text/event-stream" });
        const write = (payload) => {
          res.write(`data: ${JSON.stringify(payload)}\n\n`);
        };
        write({ choices: [{ delta: { reasoning_content: "checking context", content: "feat: e2e-" } }] });
        setTimeout(() => {
          write({ choices: [{ delta: { content: "api" } }] });
          write({ choices: [], usage: { prompt_tokens: 120, completion_tokens: 40 } });
          res.write("data: [DONE]\n\n");
          res.end();
        }, 350);
        events.push("api-stream");
        return;
      }
      res.writeHead(404);
      res.end();
    });
    server.listen(0, "127.0.0.1", () => {
      resolve({ server, port: server.address().port, events });
    });
  });
}

async function waitFor(url, timeoutMs = 15000) {
  const started = Date.now();
  while (Date.now() - started < timeoutMs) {
    try {
      const res = await fetch(url);
      if (res.ok) return;
    } catch {
      /* not ready */
    }
    await new Promise((resolve) => setTimeout(resolve, 150));
  }
  throw new Error(`timed out waiting for ${url}`);
}

async function main() {
  if (!fs.existsSync(BINARY)) {
    throw new Error(`missing ${BINARY}; run: wails3 task build:server`);
  }
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "wgit-ai-e2e-"));
  const repo = path.join(root, "repo");
  const configDir = path.join(root, "config");
  fs.mkdirSync(repo, { recursive: true });
  fs.mkdirSync(configDir, { recursive: true });

  const git = (...args) =>
    execFileSync("git", args, { cwd: repo, stdio: "pipe" }).toString();
  git("init", "-q");
  git("config", "user.name", "AI E2E");
  git("config", "user.email", "ai-e2e@example.com");
  fs.writeFileSync(path.join(repo, "app.txt"), "hello\n");
  git("add", "app.txt");
  git("commit", "-qm", "chore: base");
  fs.appendFileSync(path.join(repo, "app.txt"), "world\n");
  git("add", "app.txt");

  const mock = await startMockAI();
  const serverPort = await freePort();
  const cliPath = path.join(root, "fake-cli.sh");
  fs.writeFileSync(
    cliPath,
    "#!/bin/sh\nprintf 'feat: e2e-'\nsleep 0.8\nprintf 'cli\\n'\n",
    { mode: 0o755 }
  );
  fs.writeFileSync(
    path.join(configDir, "config.json"),
    JSON.stringify({
      recentRepos: [repo],
      lastRepo: repo,
      theme: "dark",
      flags: {
        aiCommitMsg: true,
        aiSummary: true,
        aiConflictBatch: true,
        reviewFlow: false,
        assetBoard: false,
      },
      schemaVersion: 2,
    })
  );

  const server = spawn(BINARY, [], {
    env: {
      ...process.env,
      WGIT_CONFIG_DIR: configDir,
      WAILS_SERVER_PORT: String(serverPort),
    },
    stdio: ["ignore", "pipe", "pipe"],
  });
  let serverLog = "";
  server.stdout.on("data", (chunk) => (serverLog += chunk));
  server.stderr.on("data", (chunk) => (serverLog += chunk));
  const base = `http://127.0.0.1:${serverPort}/`;
  let browser;
  let callId = 0;
  const visualDiffs = [];

  async function captureVisual(page, name, locator, options = {}) {
    fs.mkdirSync(VISUAL_OUT, { recursive: true });
    const target = locator || page;
    const buffer = await target.screenshot({ animations: "disabled", maskColor: "#0b0e14", ...options });
    fs.writeFileSync(path.join(VISUAL_OUT, `ai-${name}.png`), buffer);
    const baseline = path.join(VISUAL_BASELINES, `ai-${name}.png`);
    if (VISUAL_UPDATE) {
      fs.mkdirSync(VISUAL_BASELINES, { recursive: true });
      fs.writeFileSync(baseline, buffer);
      return;
    }
    if (!fs.existsSync(baseline)) {
      console.log(`visual baseline missing for ai-${name}.png`);
      return;
    }
    const changed = differingPixels(fs.readFileSync(baseline), buffer);
    if (changed < 0) {
      visualDiffs.push(`ai-${name}.png: size changed`);
      return;
    }
    const pixels = PNG.sync.read(buffer).width * PNG.sync.read(buffer).height;
    if (changed / pixels > VISUAL_TOLERANCE_RATIO) {
      visualDiffs.push(`ai-${name}.png: ${changed} px differ (${((changed / pixels) * 100).toFixed(2)}%)`);
    }
  }

  async function rpc(method, args = []) {
    const res = await fetch(new URL("wails/runtime", base), {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "x-wails-client-id": "ai-e2e",
      },
      body: JSON.stringify({
        object: 0,
        method: 0,
        args: {
          "call-id": String(++callId),
          methodName: method,
          args,
        },
      }),
    });
    const text = await res.text();
    let json;
    try {
      json = JSON.parse(text);
    } catch {
      // Wails returns string results as raw text, not a JSON-encoded string.
      if (!res.ok) throw new Error(`${method}: ${text.slice(0, 300)}`);
      return text;
    }
    if (json.error) throw new Error(`${method}: ${JSON.stringify(json.error)}`);
    return Object.prototype.hasOwnProperty.call(json, "result") ? json.result : json;
  }

  const saveAPIWithTimeout = (baseUrl, timeoutSeconds) =>
    rpc("main.AiService.SaveAISettings", [
      {
        mode: "api",
        baseUrl,
        model: "a-model",
        timeoutSeconds,
        protocol: "openai",
        cliCommand: "",
        cliArgs: [],
        cliInput: "stdin",
        apiKey: "",
        clearApiKey: false,
      },
    ]);
  const saveAPI = (baseUrl) => saveAPIWithTimeout(baseUrl, 10);
  const saveCLI = () =>
    rpc("main.AiService.SaveAISettings", [
      {
        mode: "cli",
        baseUrl: "",
        model: "",
        timeoutSeconds: 10,
        cliCommand: cliPath,
        cliArgs: [],
        cliInput: "stdin",
        apiKey: "",
        clearApiKey: false,
      },
    ]);

  try {
    await waitFor(base);

    browser = await chromium.launch({ channel: "chrome" });
    const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });
    const errors = [];
    page.on("console", (msg) => {
      const text = msg.text();
      if (msg.type() === "error" && !/Failed to load resource|wails\/(runtime|custom)/.test(text)) {
        errors.push(text);
      }
    });
    page.on("pageerror", (err) => errors.push(`pageerror: ${err.message}`));
    await page.addInitScript(() => localStorage.setItem("wgit.locale", "zh-CN"));
    await page.goto(base, { waitUntil: "networkidle" });
    await page.waitForTimeout(700);
    await page.evaluate(
      (repoPath) =>
        window.dispatchEvent(
          new CustomEvent("wgit:deep-link", {
            detail: `wgit://open?repo=${repoPath}&tab=workspace`,
          })
        ),
      repo
    );
    await page.waitForTimeout(2200);

    const subject = page.locator('input[aria-label="提交说明"]').first();
    const commitBody = page.locator('textarea[aria-label="提交正文（可选）"]').first();

    // First-run runtime wizard: an unconfigured AI action opens the three-step
    // quick setup instead of sending the user to the full settings page.
    await subject.fill("");
    await page.locator('[data-testid="smart-generate-btn"]').click();
    await page.locator('[data-testid="ai-quick-setup-modal"]').waitFor({ state: "visible", timeout: 8000 });
    await page.locator('[data-testid="ai-quick-preset-custom"]').click();
    if ((await page.locator('[data-testid="ai-quick-preset-custom"]').getAttribute("aria-pressed")) !== "true") {
      throw new Error("selected provider is not exposed via aria-pressed");
    }
    if ((await page.locator('label[for="ai-quick-model-input"]').count()) === 0) {
      throw new Error("model input label is not associated");
    }
    await page.locator('[data-testid="ai-quick-endpoint"]').fill(`http://127.0.0.1:${mock.port}/v1`);
    await page.locator('[data-testid="ai-quick-model"]').fill("gpt-4.1-mini");
    await page.waitForFunction(
      () => (document.querySelector('[data-testid="ai-quick-setup-modal"]')?.innerText || "").includes("每百万 token"),
      { timeout: 5000 }
    );
    await page.locator('[data-testid="ai-quick-save"]').click();
    await page.locator('[data-testid="ai-quick-test-result"]').waitFor({ state: "visible", timeout: 10000 });
    const quickResult = await page.locator('[data-testid="ai-quick-test-result"]').innerText();
    if (!quickResult.includes("连接正常")) {
      throw new Error(`quick setup test failed: ${quickResult}`);
    }
    const activeProfile = page.locator('[data-testid^="ai-quick-profile-"]').first();
    if ((await activeProfile.getAttribute("aria-pressed")) !== "true") {
      throw new Error("active profile is not exposed via aria-pressed");
    }
    await captureVisual(page, "quick-setup-modal", page.locator('[data-testid="ai-quick-setup-modal"]'), {
      mask: [page.locator('[data-testid="ai-quick-endpoint"]')],
    });
    await page.locator('[data-testid="ai-quick-close"]').click();
    await page.locator('[data-testid="ai-quick-setup-modal"]').waitFor({ state: "hidden", timeout: 5000 });
    const wizardConfig = await rpc("main.AiService.GetAIConfig");
    if (wizardConfig.mode !== "api" || wizardConfig.model !== "gpt-4.1-mini") {
      throw new Error(`wizard saved unexpected config: ${JSON.stringify(wizardConfig)}`);
    }
    await page.waitForTimeout(400);

    await subject.fill("");
    await page.locator('[data-testid="smart-generate-btn"]').click();
    await page.waitForTimeout(300);
    const wizardConfirm = page.getByRole("button", { name: "确认", exact: true }).first();
    if (await wizardConfirm.isVisible().catch(() => false)) await wizardConfirm.click();
    await page.locator('[data-testid="ai-stream-preview"]').waitFor({ state: "visible", timeout: 8000 });
    await page.waitForFunction(
      () => (document.querySelector('input[aria-label="提交说明"]')?.value || "") === "feat: e2e-api",
      { timeout: 8000 }
    );
    // Explain diff uses the same stream transport and is read-only.
    const fileRow = page.locator("span:visible", { hasText: "app.txt" }).first();
    if (await fileRow.count()) {
      await fileRow.click();
      await page.waitForTimeout(700);
      const explain = page.locator('[data-testid="explain-diff-btn"]').first();
      if (await explain.count()) {
        await explain.click();
        await page.waitForTimeout(900);
        if (!(await page.locator("body").innerText()).includes("AI 解释改动")) {
          throw new Error("explain-diff modal did not open");
        }
        await page.keyboard.press("Escape");
        await page.waitForTimeout(300);
      }
    }

    await subject.fill("keep-me");
    await page.locator('[data-testid="smart-generate-btn"]').click();
    await page.waitForTimeout(250);
    await page.getByRole("button", { name: "确认", exact: true }).first().click();
    await page.locator('[data-testid="ai-stream-cancel"]').waitFor({ state: "visible", timeout: 8000 });
    await page.locator('[data-testid="ai-stream-cancel"]').click();
    await page.waitForTimeout(600);
    if ((await subject.inputValue()) !== "keep-me") {
      throw new Error("cancel overwrote the draft");
    }
    await saveCLI();
    if (await commitBody.count()) await commitBody.fill("");
    await subject.fill("");
    await page.locator('[data-testid="smart-generate-btn"]').click();
    await page.waitForFunction(
      () => (document.querySelector('input[aria-label="提交说明"]')?.value || "") === "feat: e2e-cli",
      { timeout: 10000 }
    );
    // Quick setup: model discovery, save, and one-click profile switching.
    await page.evaluate(
      (repoPath) =>
        window.dispatchEvent(
          new CustomEvent("wgit:deep-link", {
            detail: `wgit://open?repo=${repoPath}&tab=settings`,
          })
        ),
      repo
    );
    await page.waitForTimeout(1200);
    await page.getByText("端侧 AI / 本地模型", { exact: true }).click();
    await page.waitForTimeout(900);
    await page.locator('[data-testid="ai-quick-mode-api"]').click();
    await page.locator('[data-testid="ai-quick-preset-custom"]').click();
    await page.locator('[data-testid="ai-quick-endpoint"]').fill(`http://127.0.0.1:${mock.port}/v1`);
    await page.locator('[data-testid="ai-quick-model-fetch"]').click();
    await page.waitForFunction(
      () => (document.querySelector('[data-testid="ai-quick-model"]')?.value || "") === "a-model",
      { timeout: 8000 }
    );
    await page.locator('[data-testid="ai-quick-save-only"]').click();
    await page.waitForTimeout(800);
    const saved = await rpc("main.AiService.GetAIConfig");
    if (saved.model !== "a-model") {
      throw new Error(`saved model = ${saved.model}`);
    }

    const profiles = await rpc("main.AiService.ListAIProfiles");
    const gptProfile = profiles.find((profile) => profile.model === "gpt-4.1-mini");
    const apiProfile = profiles.find((profile) => profile.model === "a-model");
    if (!gptProfile || !apiProfile) {
      throw new Error(`profiles missing: ${JSON.stringify(profiles)}`);
    }
    await page.locator(`[data-testid="ai-quick-profile-${gptProfile.id}"]`).click();
    await page.waitForTimeout(1400);
    if ((await rpc("main.AiService.GetAIConfig")).model !== "gpt-4.1-mini") {
      throw new Error("switching to the gpt profile did not restore its model");
    }
    await page.locator(`[data-testid="ai-quick-profile-${apiProfile.id}"]`).click();
    // Let the success toast disappear so the visual baseline stays stable.
    await page.waitForTimeout(2800);

    const quickPanel = page.locator('[data-testid="ai-quick-setup"]').first();
    await quickPanel.scrollIntoViewIfNeeded();
    await page.waitForTimeout(400);
    await captureVisual(page, "settings-quick", page, {
      mask: [
        page.locator('[data-testid="ai-usage-card"]'),
        page.locator('[data-testid="ai-quick-endpoint"]'),
      ],
    });
    // Control plane: routing precedence (repo task > repo model > global task),
    // budgets and the read-only assistant prompts.
    await rpc("main.AiService.SaveAIAdvancedConfig", [
      {
        taskModels: { chat: "chat-model", review: "review-model", release: "release-model" },
        inputUsdPerMTok: 0,
        outputUsdPerMTok: 0,
        monthlyBudgetUsd: 0.0001,
        retryCount: 0,
        rateLimitPerMinute: 0,
        fallbackBaseUrl: "",
        fallbackModel: "",
        fallbackProtocol: "openai",
      },
    ]);
    const advanced = await rpc("main.AiService.GetAIAdvancedConfig");
    if (advanced.taskModels.chat !== "chat-model" || advanced.monthlyBudgetUsd !== 0.0001) {
      throw new Error(`advanced config roundtrip = ${JSON.stringify(advanced)}`);
    }
    const chatPreview = await rpc("main.AiService.PreviewAIRequest", [
      {
        requestId: "", kind: "chat", repoPath: repo, filePath: "app.txt",
        diff: "+world", scope: "diff", question: "what changed", history: [],
      },
    ]);
    if (!chatPreview.includes("what changed") || !chatPreview.includes("+world")) {
      throw new Error("diff chat preview did not include the question and diff");
    }
    const repoChatPreview = await rpc("main.AiService.PreviewAIRequest", [
      { requestId: "", kind: "chat", repoPath: repo, scope: "repo", question: "what is this", history: [] },
    ]);
    if (!repoChatPreview.includes("Tracked files")) {
      throw new Error("repo chat preview did not include the repository context");
    }
    const head = git("rev-parse", "HEAD").trim();
    const reviewPreview = await rpc("main.AiService.PreviewAIRequest", [
      { requestId: "", kind: "review", repoPath: repo, commitHash: head },
    ]);
    if (!reviewPreview.includes("## Verdict") || !reviewPreview.includes("chore: base")) {
      throw new Error("review preview did not include the commit context");
    }
    const releasePreview = await rpc("main.AiService.PreviewAIRequest", [
      { requestId: "", kind: "release", repoPath: repo, from: "", to: "", maxCommits: 5 },
    ]);
    if (!releasePreview.includes("## Highlights") || !releasePreview.includes("chore: base")) {
      throw new Error("release preview did not include the commit list");
    }
    // Read-only chat over the current diff.
    await page.evaluate(
      (repoPath) =>
        window.dispatchEvent(
          new CustomEvent("wgit:deep-link", {
            detail: `wgit://open?repo=${repoPath}&tab=workspace`,
          })
        ),
      repo
    );
    await page.waitForTimeout(1800);
    const chatFileRow = page.locator("span:visible", { hasText: "app.txt" }).first();
    if (await chatFileRow.count()) {
      await chatFileRow.click();
      await page.waitForTimeout(600);
    }

    // Repository-scoped AI settings live next to the repository, not in the
    // global settings page.
    const repoAiButton = page.locator('[data-testid="repo-ai-btn"]').first();
    if (!(await repoAiButton.count())) throw new Error("repository AI button missing");
    await repoAiButton.click();
    await page.locator('[data-testid="ai-repo-modal"]').waitFor({ state: "visible", timeout: 5000 });
    await page.waitForFunction(
      () => !document.querySelector('[data-testid="ai-repo-model"]')?.disabled,
      { timeout: 5000 }
    );
    await page.locator('[data-testid="ai-repo-model"]').fill("repo-model");
    await page.locator('[data-testid="ai-repo-tasks"] summary').click();
    await page.locator('[data-testid="ai-repo-task-release"]').fill("repo-release-model");
    await page.locator('[data-testid="ai-repo-save"]').click();
    await page.locator('[data-testid="ai-repo-modal"]').waitFor({ state: "hidden", timeout: 5000 });
    const repoOverride = await rpc("main.AiService.GetAIRepoOverride", [repo]);
    if (repoOverride.model !== "repo-model" || repoOverride.taskModels?.release !== "repo-release-model") {
      throw new Error(`repo override = ${JSON.stringify(repoOverride)}`);
    }
    const chatButton = page.locator('[data-testid="ai-chat-btn"]').first();
    if (!(await chatButton.count())) throw new Error("diff chat button missing");
    await chatButton.click();
    await page.locator('[data-testid="ai-chat-input"]').fill("what changed?");
    await page.locator('[data-testid="ai-chat-send"]').click();
    await page.waitForFunction(
      () => (document.querySelector('[data-testid="ai-chat-messages"]')?.innerText || "").includes("feat: e2e-api"),
      { timeout: 8000 }
    );
    const reasoningToggle = page.getByText("模型思考", { exact: true }).first();
    if (await reasoningToggle.count()) {
      await reasoningToggle.click();
      await page.waitForTimeout(250);
    }
    await captureVisual(page, "chat-modal", page.locator('[data-testid="ai-chat-modal"]'));
    await page.keyboard.press("Escape");
    await page.waitForTimeout(300);
    const reviewButton = page.locator('[data-testid="ai-review-btn"]').first();
    if (!(await reviewButton.count())) throw new Error("diff review button missing");
    await reviewButton.click();
    await page.locator('[data-testid="ai-review-modal"]').waitFor({ state: "visible", timeout: 5000 });
    await page.waitForFunction(
      () => (document.querySelector('[data-testid="ai-review-modal"]')?.innerText || "").includes("feat: e2e-api"),
      { timeout: 8000 }
    );
    await captureVisual(page, "review-modal", page.locator('[data-testid="ai-review-modal"]'));
    await page.keyboard.press("Escape");
    await page.waitForTimeout(300);
    // Release notes run against the real commit log.
    await page.evaluate(
      (repoPath) =>
        window.dispatchEvent(
          new CustomEvent("wgit:deep-link", {
            detail: `wgit://open?repo=${repoPath}&tab=history`,
          })
        ),
      repo
    );
    await page.waitForTimeout(2000);
    const releaseButton = page.locator('[data-testid="ai-release-notes-btn"]').first();
    if (!(await releaseButton.count())) throw new Error("release notes button missing");
    await releaseButton.click();
    await page.locator('[data-testid="ai-release-modal"]').waitFor({ state: "visible", timeout: 5000 });
    await page.locator('[data-testid="ai-release-run"]').click();
    await page.waitForFunction(
      () => (document.querySelector('[data-testid="ai-release-modal"]')?.innerText || "").includes("feat: e2e-api"),
      { timeout: 8000 }
    );
    await captureVisual(page, "release-modal", page.locator('[data-testid="ai-release-modal"]'));
    await page.keyboard.press("Escape");
    await page.waitForTimeout(300);
    // Durable usage reflects the real models selected per task/repo.
    const durable = await rpc("main.AiService.GetAICallLogs", [100]);
    const byKind = (kind) => durable.find((entry) => entry.kind === kind);
    if (byKind("chat")?.model !== "repo-model") {
      throw new Error(`chat log model = ${byKind("chat")?.model}`);
    }
    if (byKind("review")?.model !== "repo-model") {
      throw new Error(`review log model = ${byKind("review")?.model}`);
    }
    if (byKind("release")?.model !== "repo-release-model") {
      throw new Error(`release log model = ${byKind("release")?.model}`);
    }
    if (!byKind("chat")?.usageKnown || byKind("chat").tokensIn + byKind("chat").tokensOut <= 0) {
      throw new Error("chat usage was not recorded");
    }
    const usage = await rpc("main.AiService.GetAIUsageSummary", [30]);
    if (usage.calls < 3 || !usage.overBudget) {
      throw new Error(`usage summary = ${JSON.stringify(usage)}`);
    }
    if (usage.costKnownCalls < 1 || usage.costUnknownCalls < 1) {
      throw new Error(`catalog cost accounting = ${JSON.stringify(usage)}`);
    }
    const exportPath = await rpc("main.AiService.ExportAICallLogs", ["csv"]);
    const csv = fs.readFileSync(exportPath, "utf8");
    if (!csv.includes("repo-model") || !csv.includes("cost_known") || csv.includes("checking context") || csv.includes("+world")) {
      throw new Error("usage export leaked payload or missed models");
    }
    // Open the folded advanced settings for the deep test path.
    await page.evaluate(
      (repoPath) =>
        window.dispatchEvent(
          new CustomEvent("wgit:deep-link", {
            detail: `wgit://open?repo=${repoPath}&tab=settings`,
          })
        ),
      repo
    );
    await page.waitForTimeout(1200);
    await page.getByText("端侧 AI / 本地模型", { exact: true }).click();
    await page.waitForTimeout(900);
    await page.locator('[data-testid="ai-usage-card"]').scrollIntoViewIfNeeded();
    await page.locator('[data-testid="ai-usage-budget-input"]').fill("0.0002");
    await page.locator('[data-testid="ai-usage-budget-save"]').click();
    await page.waitForTimeout(800);
    const budgetConfig = await rpc("main.AiService.GetAIAdvancedConfig");
    if (budgetConfig.monthlyBudgetUsd !== 0.0002) {
      throw new Error(`budget-only save = ${JSON.stringify(budgetConfig)}`);
    }
    const advancedDetails = page.locator('[data-testid="ai-advanced-settings"]');
    if ((await advancedDetails.getAttribute("open")) === null) {
      await advancedDetails.locator("summary").first().click();
    }
    await page.waitForTimeout(500);
    // Local-only diagnostics and profile rename are reachable from advanced settings.
    await page.locator('[data-testid="ai-diagnostics-card"]').waitFor({ state: "visible", timeout: 8000 });
    for (const id of ["endpoint", "key", "model", "fallback"]) {
      if ((await page.locator(`[data-testid="ai-diagnostic-${id}"]`).count()) === 0) {
        throw new Error(`missing diagnostic row: ${id}`);
      }
    }
    if ((await page.locator('[data-testid="ai-profile-rename"]').count()) === 0) {
      throw new Error("profile rename affordance is missing");
    }
    for (const id of ["ai-config-export", "ai-config-import"]) {
      if ((await page.locator(`[data-testid="${id}"]`).count()) === 0) {
        throw new Error(`missing config transfer control: ${id}`);
      }
    }
    // Deep test must surface a real provider failure, not a green reachability probe.
    await saveAPI(`http://127.0.0.1:${mock.port}/fail/v1`);
    await page.locator('[data-testid="localai-endpoint-input"]').fill(
      `http://127.0.0.1:${mock.port}/fail/v1`
    );
    await page.locator('[data-testid="localai-deep-test-btn"]').click();
    await page.waitForTimeout(800);
    if (!(await page.locator("body").innerText()).includes("HTTP 500")) {
      throw new Error("deep test did not show the provider HTTP failure");
    }

    // 401 deep test through the real backend result (the UI path is already
    // covered by the 500 case above).
    await saveAPI(`http://127.0.0.1:${mock.port}/unauthorized/v1`);
    const unauthorized = await rpc("main.AiService.TestAIDeepResult");
    if (unauthorized.ok || !JSON.stringify(unauthorized).includes("401")) {
      throw new Error(`unauthorized deep test = ${JSON.stringify(unauthorized)}`);
    }

    await saveAPIWithTimeout(`http://127.0.0.1:${mock.port}/hang/v1`, 1);
    const timedOut = await rpc("main.AiService.TestAIDeepResult");
    if (timedOut.ok || !JSON.stringify(timedOut).includes("超时")) {
      throw new Error(`timeout deep test = ${JSON.stringify(timedOut)}`);
    }
    // Retry: first request fails, the user clicks generate again, second succeeds.
    await saveAPI(`http://127.0.0.1:${mock.port}/flaky/v1`);
    await page.evaluate(
      (repoPath) =>
        window.dispatchEvent(
          new CustomEvent("wgit:deep-link", {
            detail: `wgit://open?repo=${repoPath}&tab=workspace`,
          })
        ),
      repo
    );
    await page.waitForTimeout(1200);
    const retrySubject = page.locator('input[aria-label="提交说明"]').first();
    const retryBody = page.locator('textarea[aria-label="提交正文（可选）"]').first();
    if (await retryBody.count()) await retryBody.fill("");
    await retrySubject.fill("");
    await page.locator('[data-testid="smart-generate-btn"]').click();
    await page.waitForTimeout(900);
    if ((await retrySubject.inputValue()) !== "") {
      throw new Error("failed AI request wrote a draft");
    }
    await page.locator('[data-testid="smart-generate-btn"]').click();
    await page.locator('[data-testid="ai-stream-preview"]').waitFor({ state: "visible", timeout: 8000 });
    await page.waitForFunction(
      () => (document.querySelector('input[aria-label="提交说明"]')?.value || "") === "feat: e2e-api",
      { timeout: 8000 }
    );
    // Flag off must use local rules and must not start an AI stream.
    await rpc("main.GitService.SaveAppFlags", [
      { aiCommitMsg: false, aiSummary: true, aiConflictBatch: true, reviewFlow: false, assetBoard: false },
    ]);
    await page.reload({ waitUntil: "networkidle" });
    await page.waitForTimeout(700);
    await page.evaluate(
      (repoPath) =>
        window.dispatchEvent(
          new CustomEvent("wgit:deep-link", {
            detail: `wgit://open?repo=${repoPath}&tab=workspace`,
          })
        ),
      repo
    );
    await page.waitForTimeout(1800);
    const offSubject = page.locator('input[aria-label="提交说明"]').first();
    await offSubject.fill("");
    await page.locator('[data-testid="smart-generate-btn"]').click();
    await page.waitForFunction(
      () => (document.querySelector('input[aria-label="提交说明"]')?.value || "").length > 0,
      { timeout: 5000 }
    );
    if ((await offSubject.inputValue()) === "feat: e2e-api") {
      throw new Error("flag-off generate called the AI endpoint");
    }
    if (await page.locator('[data-testid="ai-stream-preview"]').isVisible().catch(() => false)) {
      throw new Error("flag-off generate started an AI stream");
    }
    // Phone: the new actions and the chat modal must fit without horizontal
    // page overflow (the wider visual baselines run at 1440px).
    await page.setViewportSize({ width: 390, height: 844 });
    await page.reload({ waitUntil: "networkidle" });
    await page.waitForTimeout(600);
    await page.evaluate(
      (repoPath) =>
        window.dispatchEvent(
          new CustomEvent("wgit:deep-link", {
            detail: `wgit://open?repo=${repoPath}&tab=workspace`,
          })
        ),
      repo
    );
    await page.waitForTimeout(1600);
    const workspaceOverflow = await page.evaluate(
      () => document.documentElement.scrollWidth - window.innerWidth
    );
    if (workspaceOverflow > 2) throw new Error(`workspace phone overflow: ${workspaceOverflow}px`);
    const phoneChat = page.locator('[data-testid="repo-chat-btn"]').first();
    if (!(await phoneChat.count())) throw new Error("repo chat button missing on phone");
    await phoneChat.click();
    await page.locator('[data-testid="ai-chat-modal"]').waitFor({ state: "visible", timeout: 5000 });
    const chatBox = await page.locator('[data-testid="ai-chat-modal"]').boundingBox();
    if (!chatBox || chatBox.x < 0 || chatBox.x + chatBox.width > 391) {
      throw new Error(`chat modal exceeds phone viewport: ${JSON.stringify(chatBox)}`);
    }
    fs.mkdirSync(VISUAL_OUT, { recursive: true });
    await page.screenshot({ path: path.join(VISUAL_OUT, "ai-chat-phone.png") });
    await page.keyboard.press("Escape");
    await page.waitForTimeout(250);

    await page.evaluate(
      (repoPath) =>
        window.dispatchEvent(
          new CustomEvent("wgit:deep-link", {
            detail: `wgit://open?repo=${repoPath}&tab=settings`,
          })
        ),
      repo
    );
    await page.waitForTimeout(1200);
    await page.getByText("端侧 AI / 本地模型", { exact: true }).click();
    await page.waitForTimeout(800);
    const settingsOverflow = await page.evaluate(
      () => document.documentElement.scrollWidth - window.innerWidth
    );
    if (settingsOverflow > 2) throw new Error(`settings phone overflow: ${settingsOverflow}px`);

    if (errors.length) throw new Error(`browser errors: ${errors.join(" | ")}`);
    if (visualDiffs.length) throw new Error(`AI visual changes: ${visualDiffs.join(" | ")}`);
    console.log(
      "AI E2E passed: API/CLI stream, cancel, model picker, chat, review, release notes, routing, usage, visual baselines"
    );
  } catch (err) {
    if (serverLog.trim()) console.error(serverLog.slice(-4000));
    throw err;
  } finally {
    if (browser) await browser.close();
    server.kill("SIGTERM");
    mock.server.close();
    if (mock.server.closeAllConnections) mock.server.closeAllConnections();
    fs.rmSync(root, { recursive: true, force: true });
  }
}

main().catch((err) => {
  console.error(err.message || err);
  process.exit(1);
});
