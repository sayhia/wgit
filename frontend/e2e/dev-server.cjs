/**
 * Dev-server helper for the browser audits and the visual baseline.
 *
 * Both scripts assume something is answering on BASE_URL. CI boots a Vite
 * server in its own step; locally nothing does, so `bun run audit` and
 * `bun run shots:check` both died with ECONNREFUSED before testing anything.
 * This module owns the "probe, maybe start, always clean up" logic so the two
 * entry points cannot drift apart.
 */
const { spawn } = require("node:child_process");
const http = require("node:http");
const path = require("node:path");

/** True when something already answers at this URL. */
function probe(base) {
  return new Promise((resolve) => {
    const req = http.get(base, (res) => {
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

function waitFor(base, deadlineMs) {
  const started = Date.now();
  return new Promise((resolve, reject) => {
    const attempt = () => {
      const req = http.get(base, (res) => {
        res.resume();
        resolve();
      });
      req.on("error", () => {
        if (Date.now() - started > deadlineMs) {
          reject(new Error(`dev server did not answer within ${deadlineMs / 1000}s`));
          return;
        }
        setTimeout(attempt, 500);
      });
      req.setTimeout(2000, () => req.destroy(new Error("probe timeout")));
    };
    attempt();
  });
}

let child = null;

/**
 * Make sure something answers at `base`. Returns true when this call started
 * the server (and therefore owns shutting it down).
 */
async function ensureServer(base) {
  if (await probe(base)) return false;

  const { port } = new URL(base);
  process.stdout.write(`nothing at ${base} — starting the Vite dev server\n`);
  child = spawn("bun", ["run", "dev", "--port", port], {
    cwd: path.join(__dirname, ".."),
    stdio: "ignore",
    detached: true,
  });
  try {
    await waitFor(base, 60000);
  } catch (err) {
    stopServer();
    throw err;
  }
  process.stdout.write("dev server ready\n");
  return true;
}

function stopServer() {
  if (!child) return;
  // detached + negative pid kills the group; vite spawns workers that outlive
  // a plain SIGTERM to the parent and keep holding the port.
  try {
    process.kill(-child.pid, "SIGTERM");
  } catch {
    child.kill("SIGTERM");
  }
  child = null;
  process.stdout.write("dev server stopped\n");
}

process.on("exit", stopServer);
process.on("SIGINT", () => {
  stopServer();
  process.exit(130);
});

module.exports = { ensureServer, stopServer };
