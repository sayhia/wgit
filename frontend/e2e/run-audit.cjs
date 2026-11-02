#!/usr/bin/env node
/**
 * Browser audit entry point.
 *
 * Runs the checks that need a real renderer — the ones a unit test cannot make:
 * a translated UI with no console errors, and WCAG AA contrast across every
 * theme × accent preset. Both drive a running dev server; point `BASE_URL` at
 * something else to audit a preview build.
 *
 *   bun run audit
 *   BASE_URL=http://127.0.0.1:4173/ bun run audit
 *
 * If nothing is listening on the base URL, a Vite dev server is started on the
 * port and torn down afterwards. CI boots its own server and passes BASE_URL,
 * so this path never triggers there — but locally `bun run audit` no longer
 * fails with a connection refused before the first check even runs.
 *
 * Exit code is non-zero when any check finds something, so CI can gate on it.
 */
const { spawn, spawnSync } = require("node:child_process");
const http = require("node:http");
const path = require("node:path");

const STEPS = [
  { name: "smoke (i18n + console errors)", file: "smoke.cjs" },
  { name: "contrast (WCAG AA)", file: "contrast.cjs" },
  { name: "theme surfaces (neutral materials + accessibility fallbacks)", file: "theme-surfaces.cjs" },
  { name: "responsive (390/768/960/1280 widths + pane sizing)", file: "responsive.cjs" },
  { name: "settings and tools (scrolling + narrow toolbars)", file: "settings-layout.cjs" },
  { name: "popovers (reachable + dismissible)", file: "popovers.cjs" },
  { name: "modal layout (long refs + short windows)", file: "modal-layout.cjs" },
];

const base = process.env.BASE_URL || "http://127.0.0.1:9245/";
const { hostname, port } = new URL(base);

/** Resolve once the URL answers, or reject after the deadline. */
function waitForServer(url, deadlineMs) {
  const started = Date.now();
  return new Promise((resolve, reject) => {
    const probe = () => {
      const req = http.get(url, (res) => {
        res.resume();
        resolve();
      });
      req.on("error", () => {
        if (Date.now() - started > deadlineMs) {
          reject(new Error(`dev server did not answer within ${deadlineMs / 1000}s`));
          return;
        }
        setTimeout(probe, 500);
      });
      req.setTimeout(2000, () => req.destroy(new Error("probe timeout")));
    };
    probe();
  });
}

/** True when something already listens, so we must not start a second server. */
function probeNow(url) {
  return new Promise((resolve) => {
    const req = http.get(url, (res) => {
      res.resume();
      resolve(true);
    });
    req.on("error", () => resolve(false));
    req.setTimeout(1500, () => {
      req.destroy();
      resolve(false);
    });
  });
}

let devServer = null;

async function ensureServer() {
  if (await probeNow(base)) return;
  process.stdout.write(`nothing at ${base} — starting the Vite dev server\n`);
  devServer = spawn("bun", ["run", "dev", "--port", port], {
    cwd: path.join(__dirname, ".."),
    stdio: "ignore",
    detached: true,
  });
  try {
    await waitForServer(base, 60000);
  } catch (err) {
    devServer.kill("SIGTERM");
    devServer = null;
    throw err;
  }
  process.stdout.write("dev server ready\n");
}

function stopServer() {
  if (!devServer) return;
  // detached + negative pid kills the whole process group; vite spawns workers
  // that survive a plain SIGTERM to the parent.
  try {
    process.kill(-devServer.pid, "SIGTERM");
  } catch {
    devServer.kill("SIGTERM");
  }
  devServer = null;
}

(async () => {
  try {
    await ensureServer();
  } catch (err) {
    process.stderr.write(`${err.message}\n`);
    process.exit(1);
  }

  let failed = 0;
  for (const step of STEPS) {
    process.stdout.write(`\n▶ ${step.name}\n`);
    const result = spawnSync(process.execPath, [path.join(__dirname, step.file)], {
      stdio: "inherit",
      env: { ...process.env, BASE_URL: base },
    });
    if (result.status !== 0) {
      failed++;
      process.stdout.write(`✗ ${step.name} reported findings\n`);
    } else {
      process.stdout.write(`✓ ${step.name}\n`);
    }
  }

  if (devServer) {
    stopServer();
    process.stdout.write("dev server stopped\n");
  }

  if (failed) {
    process.stdout.write(`\n${failed} of ${STEPS.length} audits failed against ${base}\n`);
    process.exit(1);
  }
  process.stdout.write(`\nAll ${STEPS.length} audits passed against ${base}\n`);
})();
