import { ref } from "vue";

/**
 * One global quick-setup surface. Any AI entry point that finds no usable
 * configuration can open it without detouring through the full settings page.
 */
const aiSetupOpen = ref(false);
const aiSetupReason = ref("");

export function openAiSetup(reason = ""): void {
  aiSetupReason.value = reason.trim();
  aiSetupOpen.value = true;
}

export function closeAiSetup(): void {
  aiSetupOpen.value = false;
  aiSetupReason.value = "";
}

export function useAiSetup() {
  return { aiSetupOpen, aiSetupReason, openAiSetup, closeAiSetup };
}
