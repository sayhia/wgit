<script setup lang="ts">
import { ref, watch } from "vue";
import { User, RefreshCw, ShieldCheck, FileCheck, Lock, Check } from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import type { SignedCommitItem, ComplianceRules } from "../../types";
import { t } from "../../i18n";

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
      <!-- 2. Signing -->
      <section  class="space-y-6">
        <div>
          <h2 class="text-sm font-bold text-ink">{{ t("settings.signing.title") }}</h2>
          <p class="text-xs text-ink-secondary mt-1">{{ t("settings.signing.intro") }}</p>
        </div>

        <div class="p-5 rounded-2xl bg-surface border border-line space-y-4">
          <div class="flex items-center justify-between">
            <div class="flex flex-col">
              <span class="text-xs font-bold text-ink">{{ t("settings.signing.enable") }}</span>
              <span class="text-[11px] text-ink-muted mt-0.5">{{ t("settings.signing.enable.desc") }}</span>
            </div>
            <input
              v-model="enableSigning"
              type="checkbox"
              class="w-4 h-4 rounded border-line bg-canvas text-accent focus:ring-0 cursor-pointer"
            />
          </div>

          <template v-if="enableSigning">
            <div class="flex flex-col gap-1.5 pt-2">
              <label class="text-xs text-ink font-medium">{{ t("settings.signing.format") }}</label>
              <div class="flex items-center gap-4">
                <label class="flex items-center gap-1.5 text-xs text-ink cursor-pointer">
                  <input
                    v-model="signingFormat"
                    type="radio"
                    value="openpgp"
                    class="text-accent focus:ring-0"
                  />
                  <span>{{ t("settings.signing.format.openpgp") }}</span>
                </label>
                <label class="flex items-center gap-1.5 text-xs text-ink cursor-pointer">
                  <input
                    v-model="signingFormat"
                    type="radio"
                    value="ssh"
                    class="text-accent focus:ring-0"
                  />
                  <span>SSH Key (id_ed25519.pub)</span>
                </label>
              </div>
            </div>

            <div class="flex flex-col gap-1.5">
              <label class="text-xs text-ink font-medium">{{ t("settings.signing.key") }}</label>
              <input
                v-model="signingKeyPath"
                type="text"
                :placeholder="t('settings.signing.key.placeholder')"
                class="w-full px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink focus:outline-none focus:border-accent font-mono"
              />
              <div v-if="availableSshKeys.length > 0 && signingFormat === 'ssh'" class="space-y-1">
                <span class="text-[10px] text-ink-muted">{{ t("settings.signing.ssh.detected.title") }}:</span>
                <div class="flex flex-wrap gap-1.5">
                  <button
                    v-for="key in availableSshKeys"
                    :key="key"
                    type="button"
                    @click="signingKeyPath = key"
                    class="px-2 py-0.5 rounded bg-surface hover:bg-raised border border-line text-[10px] font-mono text-ink-secondary transition cursor-pointer flex items-center gap-1"
                    :class="{ 'border-accent text-accent font-semibold': signingKeyPath === key }"
                  >
                    <span>{{ key }}</span>
                    <span v-if="signingKeyPath === key" class="text-accent">✓</span>
                  </button>
                </div>
              </div>
              <p v-else-if="availableSshKeys.length > 0" class="text-[10px] text-ink-muted font-mono">
                {{ t("settings.signing.ssh.detected", { keys: availableSshKeys.join(" / ") }) }}
              </p>
              <p v-else-if="!hasGpgBinary && signingFormat === 'openpgp'" class="text-[10px] text-warning">
                {{ t("settings.signing.gpg.missing") }}
              </p>
            </div>
          </template>

          <div class="flex items-center justify-end pt-2 border-t border-line">
            <button
              @click="saveSigning"
              class="flex items-center gap-1.5 px-4 py-1.5 rounded-lg bg-accent-strong text-accent-contrast font-semibold text-xs cursor-pointer shadow-sm hover:bg-accent-strong-hover"
              :disabled="isSavingSigning"
            >
              <RefreshCw v-if="isSavingSigning" class="w-3.5 h-3.5 animate-spin" />
              <span>{{ t("settings.signing.save") }}</span>
            </button>
          </div>
        </div>

        <!-- Signing compliance audit -->
        <div v-if="totalChecked > 0" class="p-5 rounded-2xl bg-surface border border-line space-y-3">
          <div class="flex items-center justify-between">
            <div>
              <span class="text-xs font-bold text-ink flex items-center gap-1.5">
                <ShieldCheck class="w-4 h-4 text-accent" />
                <span>{{ t("settings.signing.audit.title") }}</span>
              </span>
              <p class="text-[11px] text-ink-muted mt-0.5">
                {{ t("settings.signing.audit.desc", { count: totalChecked, compliant: compliantCount, rate: complianceRate }) }}
              </p>
            </div>
            <span
              class="px-2.5 py-0.5 rounded-full font-mono text-xs font-bold"
              :class="compliantCount === totalChecked ? 'bg-success/15 text-success' : 'bg-warning/15 text-warning'"
            >
              {{ complianceRate }}
            </span>
          </div>

          <div class="space-y-1 max-h-48 overflow-y-auto">
            <div
              v-for="commit in recentCommits.slice(0, 6)"
              :key="commit.commitHash"
              class="flex items-center justify-between gap-2 p-1.5 rounded hover:bg-surface/40 text-[10px] font-mono"
            >
              <div class="flex items-center gap-1.5 min-w-0 flex-1">
                <span
                  class="px-1 py-0.2 rounded text-[9px] font-bold shrink-0"
                  :class="commit.signatureStatus === 'G' || commit.signatureStatus === 'U' ? 'bg-success/15 text-success' : 'bg-danger/15 text-danger'"
                >
                  {{ commit.signatureStatus === 'G' || commit.signatureStatus === 'U' ? t("settings.signing.badge.valid") : t("settings.signing.badge.unsigned") }}
                </span>
                <span class="text-ink-muted shrink-0">{{ commit.shortHash }}</span>
                <span class="text-ink truncate" :title="commit.subject">{{ commit.subject }}</span>
              </div>
              <span class="text-ink-muted shrink-0">{{ commit.authorName }}</span>
            </div>
          </div>
        </div>

        <!-- Enterprise Compliance & Rules Policy (.wgit-rules.json) -->
        <div class="p-5 rounded-2xl bg-surface border border-line space-y-4">
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2">
              <div class="w-8 h-8 rounded-lg bg-accent/15 text-accent flex items-center justify-center">
                <FileCheck class="w-4 h-4" />
              </div>
              <div>
                <div class="flex items-center gap-2">
                  <span class="text-xs font-bold text-ink">{{ t("settings.compliance.title") }}</span>
                  <span
                    class="px-2 py-0.5 rounded-full text-[10px] font-mono font-semibold"
                    :class="hasRulesFile ? 'bg-accent/15 text-accent' : 'bg-surface text-ink-muted border border-line'"
                  >
                    {{ hasRulesFile ? t("settings.compliance.file.active") : t("settings.compliance.file.default") }}
                  </span>
                </div>
                <p class="text-[11px] text-ink-muted mt-0.5">
                  {{ t("settings.compliance.desc") }}
                </p>
              </div>
            </div>
          </div>

          <div class="space-y-3 pt-2 border-t border-line">
            <!-- 1. Enforce signed commits checkbox -->
            <div class="flex items-center justify-between">
              <div class="flex flex-col">
                <span class="text-xs font-medium text-ink">{{ t("settings.compliance.enforce.sign") }}</span>
                <span class="text-[10px] text-ink-muted">{{ t("settings.compliance.enforce.sign.desc") }}</span>
              </div>
              <input
                v-model="complianceRules.enforceSignedCommits"
                type="checkbox"
                class="w-4 h-4 rounded border-line bg-canvas text-accent focus:ring-0 cursor-pointer"
              />
            </div>

            <!-- 2. Commit message regex -->
            <div class="flex flex-col gap-1">
              <label class="text-xs text-ink font-medium">{{ t("settings.compliance.msg.pattern") }}</label>
              <input
                v-model="complianceRules.requireCommitMessagePattern"
                type="text"
                spellcheck="false"
                class="w-full px-3 py-1.5 rounded-lg bg-canvas border border-line text-xs font-mono text-ink focus:outline-none focus:border-accent"
                :placeholder="t('settings.compliance.msg.pattern.placeholder')"
              />
            </div>

            <!-- 3. Protected branches -->
            <div class="flex flex-col gap-1">
              <label class="text-xs text-ink font-medium">{{ t("settings.compliance.protected.branches") }}</label>
              <input
                v-model="directCommitBranchesInput"
                type="text"
                spellcheck="false"
                class="w-full px-3 py-1.5 rounded-lg bg-canvas border border-line text-xs font-mono text-ink focus:outline-none focus:border-accent"
                placeholder="main, master, release"
              />
              <span class="text-[10px] text-ink-muted">{{ t("settings.compliance.protected.branches.hint") }}</span>
            </div>

            <!-- 4. Required email domain -->
            <div class="flex flex-col gap-1">
              <label class="text-xs text-ink font-medium">{{ t("settings.compliance.email.domain") }}</label>
              <input
                v-model="complianceRules.requireEmailDomain"
                type="text"
                spellcheck="false"
                class="w-full px-3 py-1.5 rounded-lg bg-canvas border border-line text-xs font-mono text-ink focus:outline-none focus:border-accent"
                placeholder="@company.com"
              />
            </div>

            <div class="flex items-center justify-end pt-2 border-t border-line">
              <button
                @click="saveComplianceRules"
                class="flex items-center gap-1.5 px-4 py-1.5 rounded-lg bg-accent-strong text-accent-contrast font-semibold text-xs cursor-pointer shadow-sm hover:bg-accent-strong-hover disabled:opacity-50"
                :disabled="isSavingRules"
              >
                <RefreshCw v-if="isSavingRules" class="w-3.5 h-3.5 animate-spin" />
                <Check v-else class="w-3.5 h-3.5" />
                <span>{{ t("settings.compliance.save.btn") }}</span>
              </button>
            </div>
          </div>
        </div>
      </section>

</template>
