<script setup lang="ts">
import { computed, type Component } from "vue";
import { Monitor, Moon, Sun, Check, Contrast, Layers } from "lucide-vue-next";
import { useTheme, type ThemeMode } from "../../composables/useTheme";
import { useAccent, type AccentId } from "../../composables/useAccent";
import { t, locale, setLocale, loadLocale, LOCALES, type Locale } from "../../i18n";

import SettingsPageHeader from "./SettingsPageHeader.vue";
import SettingsCard from "./SettingsCard.vue";

const emit = defineEmits<{ (e: "notify", msg: string): void }>();
const {
  mode,
  setThemeMode,
  highContrast,
  setHighContrast,
  reduceTransparency,
  setReduceTransparency,
} = useTheme();
const { accent, setAccent, options } = useAccent();

const themeOptions = computed<{ value: ThemeMode; label: string; description: string; icon: Component }[]>(() => [
  { value: "dark", label: t("settings.theme.dark"), description: t("settings.theme.dark.desc"), icon: Moon },
  { value: "light", label: t("settings.theme.light"), description: t("settings.theme.light.desc"), icon: Sun },
  { value: "system", label: t("settings.theme.system"), description: t("settings.theme.system.desc"), icon: Monitor },
]);

function selectMode(next: ThemeMode) {
  if (mode.value === next) return;
  setThemeMode(next);
  emit(
    "notify",
    next === "system"
      ? t("settings.appearance.toast.system")
      : t("settings.appearance.toast.theme", {
          theme: next === "dark" ? t("settings.theme.dark") : t("settings.theme.light"),
        })
  );
}

async function selectLocale(next: Locale, label: string) {
  if (locale.value === next) return;
  setLocale(next);
  // The confirmation belongs to the *new* language, but the catalog may still
  // be in flight — without this await the toast falls back to Chinese and the
  // user sees "界面语言已切换为 English" right after choosing English.
  await loadLocale(next);
  emit("notify", t("settings.appearance.toast.locale", { label }));
}

function selectAccent(next: AccentId, label: string) {
  if (accent.value === next) return;
  setAccent(next);
  emit("notify", t("settings.appearance.toast.accent", { label }));
}

function toggleHighContrast() {
  const next = !highContrast.value;
  setHighContrast(next);
  emit(
    "notify",
    next
      ? t("settings.appearance.highcontrast.toast.enabled")
      : t("settings.appearance.highcontrast.toast.disabled")
  );
}

function toggleReduceTransparency() {
  const next = !reduceTransparency.value;
  setReduceTransparency(next);
  emit(
    "notify",
    next
      ? t("settings.appearance.transparency.toast.enabled")
      : t("settings.appearance.transparency.toast.disabled")
  );
}
</script>

<template>
  <section class="@container space-y-4">
    <SettingsPageHeader :title="t('settings.appearance.title')" :description="t('settings.appearance.subtitle')" />

    <SettingsCard>
      <template #header>
        <h3 class="text-xs font-semibold text-ink">{{ t("settings.appearance.theme.title") }}</h3>
      </template>
      <div class="grid grid-cols-1 @min-[420px]:grid-cols-3 gap-2.5" role="group" :aria-label="t('settings.appearance.theme.title')">
        <button
          v-for="option in themeOptions"
          :key="option.value"
          type="button"
          @click="selectMode(option.value)"
          :aria-pressed="mode === option.value"
          class="flex min-w-0 items-start gap-3 rounded-xl border p-3 text-left transition cursor-pointer"
          :class="mode === option.value ? 'border-accent bg-accent-bg text-accent' : 'border-line bg-canvas text-ink hover:bg-raised'"
        >
          <component :is="option.icon" class="w-4 h-4 mt-0.5 shrink-0" aria-hidden="true" />
          <span class="min-w-0">
            <span class="block text-xs font-medium">{{ option.label }}</span>
            <span class="mt-1 block text-[11px] leading-relaxed" :class="mode === option.value ? 'text-accent' : 'text-ink-muted'">{{ option.description }}</span>
          </span>
        </button>
      </div>
    </SettingsCard>

    <SettingsCard>
      <template #header>
        <h3 class="text-xs font-semibold text-ink">{{ t("settings.accent.title") }}</h3>
      </template>
      <p class="mb-3 text-xs leading-relaxed text-ink-secondary">{{ t("settings.appearance.accent.hint") }}</p>
      <div class="grid grid-cols-1 @min-[300px]:grid-cols-2 @min-[560px]:grid-cols-3 gap-2" role="group" :aria-label="t('settings.accent.title')">
        <button
          v-for="option in options"
          :key="option.id"
          type="button"
          @click="selectAccent(option.id, t(option.labelKey))"
          :aria-pressed="accent === option.id"
          class="flex min-w-0 items-center gap-2.5 px-3 py-2.5 rounded-lg border text-left transition cursor-pointer"
          :class="accent === option.id ? 'border-accent bg-accent-bg' : 'border-line bg-canvas hover:bg-raised'"
        >
          <span class="w-5 h-5 rounded-full shrink-0 ring-1 ring-inset ring-black/10" :style="{ backgroundColor: option.swatch }" />
          <span class="min-w-0 flex-1">
            <span class="block text-xs font-medium text-ink">{{ t(option.labelKey) }}</span>
            <span class="mt-0.5 block text-[11px] text-ink-muted truncate">{{ t(option.hintKey) }}</span>
          </span>
          <Check v-if="accent === option.id" class="w-3.5 h-3.5 text-accent shrink-0" aria-hidden="true" />
        </button>
      </div>
      <div class="mt-4 flex flex-wrap items-center justify-between gap-3 border-t border-line/60 pt-3">
        <div class="min-w-0 flex-1 basis-56">
          <h4 class="text-xs font-medium text-ink">{{ t("settings.appearance.semantic.title") }}</h4>
          <p class="mt-1 text-[11px] leading-relaxed text-ink-muted">{{ t("settings.appearance.semantic.desc") }}</p>
        </div>
        <div class="flex items-center gap-1.5 shrink-0">
          <span class="w-3.5 h-3.5 rounded-full bg-accent-strong" title="accent" />
          <span class="w-3.5 h-3.5 rounded-full bg-success-strong" title="success" />
          <span class="w-3.5 h-3.5 rounded-full bg-warning-strong" title="warning" />
          <span class="w-3.5 h-3.5 rounded-full bg-danger-strong" title="danger" />
        </div>
      </div>
    </SettingsCard>

    <SettingsCard>
      <template #header>
        <h3 class="text-xs font-semibold text-ink">{{ t("settings.language.title") }}</h3>
      </template>
      <div class="flex flex-wrap items-center justify-between gap-3">
        <p class="min-w-0 flex-1 basis-56 text-xs leading-relaxed text-ink-secondary">{{ t("settings.language.subtitle") }}</p>
        <div class="flex flex-wrap gap-2" role="group" :aria-label="t('settings.language.title')">
          <button
            v-for="loc in LOCALES"
            :key="loc.id"
            type="button"
            @click="selectLocale(loc.id, loc.label)"
            class="px-3 py-1.5 rounded-lg border text-xs font-medium transition cursor-pointer"
            :class="locale === loc.id ? 'border-accent bg-accent-bg text-accent' : 'border-line bg-canvas text-ink hover:bg-raised'"
            :aria-pressed="locale === loc.id"
          >
            {{ loc.label }}
          </button>
        </div>
      </div>
    </SettingsCard>

    <SettingsCard>
      <template #header>
        <h3 class="text-xs font-semibold text-ink">{{ t("settings.appearance.accessibility.title") }}</h3>
      </template>
      <p class="mb-3 text-xs leading-relaxed text-ink-secondary">{{ t("settings.appearance.accessibility.desc") }}</p>
      <div class="divide-y divide-line/60">
        <label class="flex items-center justify-between gap-4 pb-3 cursor-pointer select-none">
          <span class="flex min-w-0 items-start gap-3">
            <Contrast class="w-4 h-4 mt-0.5 shrink-0 text-ink-muted" aria-hidden="true" />
            <span class="min-w-0">
              <span class="block text-xs font-medium text-ink">{{ t("settings.appearance.highcontrast.title") }}</span>
              <span class="mt-1 block text-[11px] leading-relaxed text-ink-muted line-clamp-2">{{ t("settings.appearance.highcontrast.desc") }}</span>
            </span>
          </span>
          <input
            type="checkbox"
            :checked="highContrast"
            @change="toggleHighContrast"
            class="w-4 h-4 rounded border-line bg-canvas text-accent focus:ring-0 cursor-pointer shrink-0"
          />
        </label>
        <label class="flex items-center justify-between gap-4 pt-3 cursor-pointer select-none">
          <span class="flex min-w-0 items-start gap-3">
            <Layers class="w-4 h-4 mt-0.5 shrink-0 text-ink-muted" aria-hidden="true" />
            <span class="min-w-0">
              <span class="block text-xs font-medium text-ink">{{ t("settings.appearance.transparency.title") }}</span>
              <span class="mt-1 block text-[11px] leading-relaxed text-ink-muted line-clamp-2">{{ t("settings.appearance.transparency.desc") }}</span>
            </span>
          </span>
          <input
            type="checkbox"
            :checked="reduceTransparency"
            @change="toggleReduceTransparency"
            class="w-4 h-4 rounded border-line bg-canvas text-accent focus:ring-0 cursor-pointer shrink-0"
          />
        </label>
      </div>
    </SettingsCard>
  </section>
</template>
