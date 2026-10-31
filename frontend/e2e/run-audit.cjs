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
 * Exit code is non-zero when any check finds something, so CI can gate on it.
 */
const { spawnSync } = require("node:child_process");
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

if (failed) {
  process.stdout.write(`\n${failed} of ${STEPS.length} audits failed against ${base}\n`);
  process.exit(1);
}
process.stdout.write(`\nAll ${STEPS.length} audits passed against ${base}\n`);
