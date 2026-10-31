import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

const read = (path: string) => readFileSync(fileURLToPath(new URL(path, import.meta.url)), "utf8");

describe("settings and release form controls have accessible names", () => {
  test("identity, proxy and signing inputs reuse their visible label keys", () => {
    const identity = read("../src/components/settings/IdentitySection.vue");
    expect(identity).toContain(':aria-label="t(\'settings.identity.name\')"');
    expect(identity).toContain(':aria-label="t(\'settings.identity.email\')"');

    const proxy = read("../src/components/settings/ProxySection.vue");
    expect(proxy).toContain(':aria-label="t(\'settings.proxy.url\')"');

    const signing = read("../src/components/settings/SigningSection.vue");
    expect(signing).toContain(':aria-label="t(\'settings.signing.key\')"');
  });

  test("release wizard version and target ref are named", () => {
    const release = read("../src/components/ReleaseWizard.vue");
    expect(release).toContain(':aria-label="t(\'release.version.label\')"');
    expect(release).toContain(':aria-label="t(\'release.changelog.to\')"');
  });

  test("alias, tool, model and advanced-config inputs are named", () => {
    const aliases = read("../src/components/settings/AliasesSection.vue");
    expect(aliases).toContain(':aria-label="t(\'settings.aliases.field.key\')"');
    expect(aliases).toContain(':aria-label="t(\'settings.aliases.field.command\')"');

    const tools = read("../src/components/settings/ToolsSection.vue");
    expect(tools).toContain(':aria-label="t(\'settings.tools.custom.diff\')"');
    expect(tools).toContain(':aria-label="t(\'settings.tools.custom.merge\')"');

    const connection = read("../src/components/settings/AiConnectionForm.vue");
    expect(connection).toContain('for="ai-quick-model-input"');
    expect(connection).toContain('id="ai-quick-model-input"');
    expect(connection).toContain('t("ai.quick.model")');
    // The timeout uses a wrapping label, so its accessible name follows the
    // visible field text even without an extra aria-label.
    const timeoutIndex = connection.indexOf('v-model.number="form.timeoutSeconds"');
    expect(timeoutIndex).toBeGreaterThan(-1);
    const labelStart = connection.lastIndexOf("<label", timeoutIndex);
    const labelEnd = connection.indexOf("</label>", timeoutIndex);
    expect(labelStart).toBeGreaterThan(-1);
    expect(labelEnd).toBeGreaterThan(timeoutIndex);
    expect(connection.slice(labelStart, labelEnd)).toContain('t("settings.localai.timeout.label")');

    const config = read("../src/components/settings/AdvancedConfigSection.vue");
    expect(config).toContain(':aria-label="t(\'settings.config.write.key\')"');
    expect(config).toContain(':aria-label="t(\'settings.config.write.value\')"');
  });

  test("studio ops and sparse inputs are named", () => {
    const ops = read("../src/components/tools/StudioOpsSection.vue");
    expect(ops).toContain(':aria-label="t(\'ops.secret.id.placeholder\')"');
    expect(ops).toContain(':aria-label="t(\'ops.secret.value.placeholder\')"');
    expect(ops).toContain(':aria-label="t(\'ops.webhook.url.placeholder\')"');
    expect(ops).toContain(':aria-label="t(\'ops.queue.target.placeholder\')"');

    const sparse = read("../src/components/tools/SparseTab.vue");
    expect(sparse).toContain(':aria-label="t(\'sparse.partial.hydrate\')"');
    expect(sparse).toContain(':aria-label="t(\'sparse.placeholder\')"');
  });

  test("reflog branch name and bisect bad ref are named", () => {
    const reflog = read("../src/components/tools/ReflogTab.vue");
    expect(reflog).toContain(':aria-label="t(\'reflog.modal.name\')"');

    const bisect = read("../src/components/BisectPanel.vue");
    expect(bisect).toContain(':aria-label="t(\'bisect.bad.label\')"');
  });
});
