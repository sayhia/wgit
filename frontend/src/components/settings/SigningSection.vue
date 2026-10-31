<script setup lang="ts">
import { ref, watch } from "vue";
import { RefreshCw, Check } from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import type { SignedCommitItem, ComplianceRules } from "../../types";
import { t } from "../../i18n";
import SettingsPageHeader from "./SettingsPageHeader.vue";
import SettingsCard from "./SettingsCard.vue";

const props = defineProps<{ currentRepoPath: string }>();
const emit = defineEmits<{ (e: "notify", msg: string): void }>();

function notify(msg: string) {
  emit("notify", msg);
}

const enableSigning = ref(false);
const signingFormat = ref<"openpgp" | "ssh">("openpgp");
const signingKeyPath = ref("");
const isSavingSigning = ref(false);
const availableSshKeys = ref<string[]>([]);
const hasGpgBinary = ref(false);
const complianceRate = ref("");
const compliantCount = ref(0);
const totalChecked = ref(0);
const recentCommits = ref<SignedCommitItem[]>([]);

// Enterprise Compliance Rules State
const hasRulesFile = ref(false);
const isSavingRules = ref(false);
const complianceRules = ref<ComplianceRules>({
  enforceSignedCommits: false,
  requireCommitMessagePattern: `^(feat|fix|docs|style|refactor|perf|test|chore)(\\(.*\\))?!?: .+`,
  disallowDirectCommitBranches: ["main", "master", "release"],
  maxFileSizeMb: 50,
  requireEmailDomain: "",
});
const directCommitBranchesInput = ref("main, master, release");

async function loadSigningStatus() {
  if (!props.currentRepoPath) return;
  try {
    const status = await gitService.getCommitSigningStatus(props.currentRepoPath);
    if (status) {
      enableSigning.value = status.gpgSignEnabled;
      signingKeyPath.value = status.signingKey || "";
      signingFormat.value = status.gpgFormat === "ssh" ? "ssh" : "openpgp";
      availableSshKeys.value = status.availableSshKeys || [];
      hasGpgBinary.value = status.hasGpgBinary;
      complianceRate.value = status.complianceRate || "";
      compliantCount.value = status.compliantCount || 0;
      totalChecked.value = status.totalChecked || 0;
      recentCommits.value = status.recentCommits || [];
    }

    const report = await gitService.inspectRepoCompliance(props.currentRepoPath);
    if (report) {
      hasRulesFile.value = report.hasRulesFile;
      complianceRules.value = report.rules;
      if (report.rules.disallowDirectCommitBranches) {
        directCommitBranchesInput.value = report.rules.disallowDirectCommitBranches.join(", ");
      }
    }
  } catch (err) {
    // best-effort: the section renders its defaults.
    console.debug("Failed to load commit signing and compliance status:", err);
  }
}

watch(() => props.currentRepoPath, loadSigningStatus, { immediate: true });

async function saveSigning() {
  if (!props.currentRepoPath) return;
  isSavingSigning.value = true;
  try {
    const res = await gitService.configureCommitSigning(
      props.currentRepoPath,
      enableSigning.value,
      signingKeyPath.value,
      signingFormat.value
    );
    notify(res?.message || t("settings.signing.saved"));
  } catch (err: any) {
    notify(t("settings.signing.save.failed", { error: err?.message || err }));
  } finally {
    isSavingSigning.value = false;
  }
}

async function saveComplianceRules() {
  if (!props.currentRepoPath) return;
  isSavingRules.value = true;
  try {
    const branches = directCommitBranchesInput.value
      .split(",")
      .map((s) => s.trim())
      .filter(Boolean);

    const payload: ComplianceRules = {
      ...complianceRules.value,
      disallowDirectCommitBranches: branches,
    };

    await gitService.saveRepoComplianceRules(props.currentRepoPath, payload);
    hasRulesFile.value = true;
    notify(t("settings.compliance.saved"));
  } catch (err: any) {
    notify(t("settings.compliance.save.failed", { error: err?.message || err }));
  } finally {
    isSavingRules.value = false;
  }
}
</script>

<template>
  <section class="@container space-y-4">
    <SettingsPageHeader :title="t('settings.signing.title')" :description="t('settings.signing.intro')" />

    <SettingsCard>
      <form class="space-y-4" @submit.prevent="saveSigning">
        <label class="flex items-start justify-between gap-4 cursor-pointer">
          <span class="min-w-0">
            <span class="block text-xs font-medium text-ink">{{ t("settings.signing.enable") }}</span>
            <span class="mt-1 block text-[11px] leading-relaxed text-ink-muted">{{ t("settings.signing.enable.desc") }}</span>
          </span>
          <input v-model="enableSigning" type="checkbox" class="mt-0.5 w-4 h-4 shrink-0 rounded border-line bg-canvas text-accent focus:ring-0 cursor-pointer" />
        </label>

        <template v-if="enableSigning">
          <fieldset class="space-y-2 border-t border-line/60 pt-4">
            <legend class="text-xs font-medium text-ink">{{ t("settings.signing.format") }}</legend>
            <div class="flex flex-wrap items-center gap-x-5 gap-y-2">
              <label class="flex items-center gap-2 text-xs text-ink cursor-pointer">
                <input v-model="signingFormat" type="radio" value="openpgp" class="text-accent focus:ring-0" />
                <span>{{ t("settings.signing.format.openpgp") }}</span>
              </label>
              <label class="flex items-center gap-2 text-xs text-ink cursor-pointer">
                <input v-model="signingFormat" type="radio" value="ssh" class="text-accent focus:ring-0" />
                <span>{{ t("settings.signing.format.ssh") }}</span>
              </label>
            </div>
          </fieldset>

          <div class="space-y-2">
            <label class="flex min-w-0 flex-col gap-1.5">
              <span class="text-xs font-medium text-ink">{{ t("settings.signing.key") }}</span>
              <input
                v-model="signingKeyPath"
                type="text"
                :aria-label="t('settings.signing.key')"
                :placeholder="t('settings.signing.key.placeholder')"
                spellcheck="false"
                class="w-full min-w-0 px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink focus:outline-none focus:border-accent font-mono"
              />
            </label>
            <div v-if="availableSshKeys.length > 0 && signingFormat === 'ssh'" class="space-y-2">
              <p class="text-[11px] text-ink-muted">{{ t("settings.signing.ssh.detected.title") }}</p>
              <div class="flex flex-wrap gap-1.5">
                <button
                  v-for="key in availableSshKeys"
                  :key="key"
                  type="button"
                  @click="signingKeyPath = key"
                  :aria-pressed="signingKeyPath === key"
                  :title="key"
                  class="flex max-w-full items-center gap-1.5 px-2 py-1 rounded-lg bg-canvas hover:bg-raised border border-line text-[11px] font-mono text-ink-secondary transition cursor-pointer"
                  :class="{ 'border-accent text-accent font-semibold': signingKeyPath === key }"
                >
                  <span class="min-w-0 truncate">{{ key }}</span>
                  <Check v-if="signingKeyPath === key" class="w-3 h-3 shrink-0 text-accent" aria-hidden="true" />
                </button>
              </div>
            </div>
            <p v-else-if="availableSshKeys.length > 0" class="text-[11px] leading-relaxed text-ink-muted font-mono break-all">
              {{ t("settings.signing.ssh.detected", { keys: availableSshKeys.join(" / ") }) }}
            </p>
            <p v-else-if="!hasGpgBinary && signingFormat === 'openpgp'" class="text-[11px] leading-relaxed text-warning">
              {{ t("settings.signing.gpg.missing") }}
            </p>
          </div>
        </template>

        <div class="flex justify-end border-t border-line/60 pt-4">
          <button
            type="submit"
            class="flex shrink-0 items-center gap-1.5 px-4 py-2 rounded-lg bg-accent-strong text-accent-contrast font-semibold text-xs cursor-pointer hover:bg-accent-strong-hover disabled:opacity-50 disabled:cursor-not-allowed"
            :disabled="isSavingSigning"
          >
            <RefreshCw v-if="isSavingSigning" class="w-3.5 h-3.5 animate-spin" aria-hidden="true" />
            {{ t("settings.signing.save") }}
          </button>
        </div>
      </form>
    </SettingsCard>

    <SettingsCard v-if="totalChecked > 0">
      <template #header>
        <h3 class="text-xs font-semibold text-ink">{{ t("settings.signing.audit.title") }}</h3>
      </template>
      <template #actions>
        <span
          class="rounded-md px-2 py-0.5 font-mono text-[11px] font-medium"
          :class="compliantCount === totalChecked ? 'bg-success-bg text-success' : 'bg-warning-bg text-warning'"
        >{{ complianceRate }}</span>
      </template>
      <p class="mb-3 text-xs leading-relaxed text-ink-secondary">
        {{ t("settings.signing.audit.desc", { count: totalChecked, compliant: compliantCount, rate: complianceRate }) }}
      </p>
      <div class="space-y-1 max-h-48 overflow-y-auto">
            <div
              v-for="commit in recentCommits.slice(0, 6)"
              :key="commit.commitHash"
  class="flex items-center justify-between gap-3 py-1.5 text-[11px] font-mono"
            >
          <div class="flex items-center gap-2 min-w-0 flex-1">
            <span
              class="shrink-0 rounded px-1.5 py-0.5 text-[10px] font-medium"
              :class="commit.signatureStatus === 'G' || commit.signatureStatus === 'U' ? 'bg-success-bg text-success' : 'bg-danger-bg text-danger'"
            >
                  {{ commit.signatureStatus === 'G' || commit.signatureStatus === 'U' ? t("settings.signing.badge.valid") : t("settings.signing.badge.unsigned") }}
            </span>
            <span class="text-ink-muted shrink-0">{{ commit.shortHash }}</span>
            <span class="text-ink truncate" :title="commit.subject">{{ commit.subject }}</span>
          </div>
          <span class="max-w-28 truncate text-ink-muted" :title="commit.authorName">{{ commit.authorName }}</span>
        </div>
      </div>
    </SettingsCard>

    <SettingsCard>
      <template #header>
        <h3 class="text-xs font-semibold text-ink">{{ t("settings.compliance.title") }}</h3>
      </template>
      <template #actions>
                  <span
  class="rounded-md px-2 py-0.5 text-[10px] font-medium"
          :class="hasRulesFile ? 'bg-accent-bg text-accent' : 'bg-raised text-ink-muted'"
        >{{ hasRulesFile ? t("settings.compliance.file.active") : t("settings.compliance.file.default") }}</span>
      </template>
      <form class="space-y-4" @submit.prevent="saveComplianceRules">
        <p class="text-xs leading-relaxed text-ink-secondary">{{ t("settings.compliance.desc") }}</p>
        <label class="flex items-start justify-between gap-4 cursor-pointer">
          <span class="min-w-0">
            <span class="block text-xs font-medium text-ink">{{ t("settings.compliance.enforce.sign") }}</span>
            <span class="mt-1 block text-[11px] leading-relaxed text-ink-muted">{{ t("settings.compliance.enforce.sign.desc") }}</span>
          </span>
          <input v-model="complianceRules.enforceSignedCommits" type="checkbox" class="mt-0.5 w-4 h-4 shrink-0 rounded border-line bg-canvas text-accent focus:ring-0 cursor-pointer" />
        </label>
        <label class="flex min-w-0 flex-col gap-1.5">
          <span class="text-xs font-medium text-ink">{{ t("settings.compliance.msg.pattern") }}</span>
          <input
            v-model="complianceRules.requireCommitMessagePattern"
            type="text"
            spellcheck="false"
            :placeholder="t('settings.compliance.msg.pattern.placeholder')"
            class="w-full min-w-0 px-3 py-2 rounded-lg bg-canvas border border-line text-xs font-mono text-ink focus:outline-none focus:border-accent"
          />
        </label>
        <div class="grid grid-cols-1 @min-[560px]:grid-cols-2 gap-4">
          <label class="flex min-w-0 flex-col gap-1.5">
            <span class="text-xs font-medium text-ink">{{ t("settings.compliance.protected.branches") }}</span>
            <input v-model="directCommitBranchesInput" type="text" spellcheck="false" placeholder="main, master, release" class="w-full min-w-0 px-3 py-2 rounded-lg bg-canvas border border-line text-xs font-mono text-ink focus:outline-none focus:border-accent" />
            <span class="text-[11px] leading-relaxed text-ink-muted">{{ t("settings.compliance.protected.branches.hint") }}</span>
          </label>
          <label class="flex min-w-0 flex-col gap-1.5">
            <span class="text-xs font-medium text-ink">{{ t("settings.compliance.email.domain") }}</span>
            <input v-model="complianceRules.requireEmailDomain" type="text" spellcheck="false" placeholder="@company.com" class="w-full min-w-0 px-3 py-2 rounded-lg bg-canvas border border-line text-xs font-mono text-ink focus:outline-none focus:border-accent" />
          </label>
        </div>
        <div class="flex justify-end border-t border-line/60 pt-4">
          <button
            type="submit"
            class="flex shrink-0 items-center gap-1.5 px-4 py-2 rounded-lg bg-accent-strong text-accent-contrast font-semibold text-xs cursor-pointer hover:bg-accent-strong-hover disabled:opacity-50 disabled:cursor-not-allowed"
            :disabled="isSavingRules"
          >
            <RefreshCw v-if="isSavingRules" class="w-3.5 h-3.5 animate-spin" aria-hidden="true" />
            <Check v-else class="w-3.5 h-3.5" aria-hidden="true" />
            {{ t("settings.compliance.save.btn") }}
          </button>
        </div>
      </form>
    </SettingsCard>
  </section>
</template>
