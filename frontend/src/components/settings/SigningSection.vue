<script setup lang="ts">
import { ref, watch } from "vue";
import { User, RefreshCw } from "lucide-vue-next";
import { gitService } from "../../services/gitService";
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

async function loadSigningStatus() {
  if (!props.currentRepoPath) return;
  try {
    const status = await gitService.getCommitSigningStatus(props.currentRepoPath);
    if (!status) return;
    enableSigning.value = status.gpgSignEnabled;
    signingKeyPath.value = status.signingKey || "";
    signingFormat.value = status.gpgFormat === "ssh" ? "ssh" : "openpgp";
    availableSshKeys.value = status.availableSshKeys || [];
    hasGpgBinary.value = status.hasGpgBinary;
  } catch (err) {
    console.debug("Failed to load commit signing status:", err);
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
              <p v-if="availableSshKeys.length > 0" class="text-[10px] text-ink-muted font-mono">
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
      </section>

</template>
