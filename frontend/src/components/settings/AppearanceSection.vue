<script setup lang="ts">
import { computed, type Component } from "vue";
import { Monitor, Moon, Sun, Check, Languages, Contrast, Layers } from "lucide-vue-next";
import { useTheme, type ThemeMode } from "../../composables/useTheme";
import { useAccent, type AccentId } from "../../composables/useAccent";
import { t, locale, setLocale, LOCALES, type Locale } from "../../i18n";

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

function selectLocale(next: Locale, label: string) {
  if (locale.value === next) return;
  setLocale(next);
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
  <section class="space-y-6">
    <div>
      <h2 class="text-sm font-bold text-ink">{{ t("settings.appearance.title") }}</h2>
      <p class="text-xs text-ink-secondary mt-1">{{ t("settings.appearance.subtitle") }}</p>
    </div>

    <div class="grid grid-cols-3 gap-3">
      <button
        v-for="option in themeOptions"
        :key="option.value"
        @click="selectMode(option.value)"
        class="p-4 rounded-2xl border text-left transition cursor-pointer"
        :class="
          mode === option.value
            ? 'border-accent bg-accent-bg text-accent shadow-xs'
            : 'border-line bg-surface text-ink hover:bg-raised'
        "
      >
        <component :is="option.icon" class="w-4 h-4 mb-2.5" />
        <div class="text-xs font-semibold">{{ option.label }}</div>
        <div class="text-[10px] mt-0.5" :class="mode === option.value ? 'text-accent' : 'text-ink-muted'">
          {{ option.description }}
        </div>
      </button>
    </div>

    <div class="space-y-2">
      <div class="text-xs font-semibold text-ink">{{ t("settings.accent.title") }}</div>
      <p class="text-[11px] text-ink-secondary">
        {{ t("settings.appearance.accent.hint") }}
      </p>
      <div class="grid grid-cols-2 sm:grid-cols-3 gap-2">
        <button
          v-for="option in options"
          :key="option.id"
          @click="selectAccent(option.id, t(option.labelKey))"
          class="flex items-center gap-2.5 px-3 py-2 rounded-xl border text-left transition cursor-pointer"
          :class="
            accent === option.id
              ? 'border-accent bg-accent-bg'
              : 'border-line bg-surface hover:bg-raised'
          "
        >
          <span
            class="w-5 h-5 rounded-full shrink-0 ring-1 ring-inset ring-black/10"
            :style="{ backgroundColor: option.swatch }"
          />
          <span class="min-w-0 flex-1">
            <span class="block text-[11px] font-semibold text-ink">{{ t(option.labelKey) }}</span>
            <span class="block text-[10px] text-ink-muted truncate">{{ t(option.hintKey) }}</span>
          </span>
          <Check v-if="accent === option.id" class="w-3.5 h-3.5 text-accent shrink-0" />
        </button>
      </div>
    </div>

    <div class="space-y-2">
      <div class="text-xs font-semibold text-ink flex items-center gap-1.5">
        <Languages class="w-3.5 h-3.5 text-accent" />
        <span>{{ t("settings.language.title") }}</span>
      </div>
      <p class="text-[11px] text-ink-secondary">{{ t("settings.language.subtitle") }}</p>
      <div class="flex flex-wrap gap-2">
        <button
          v-for="loc in LOCALES"
          :key="loc.id"
          @click="selectLocale(loc.id, loc.label)"
          class="px-3 py-1.5 rounded-lg border text-xs font-semibold transition cursor-pointer"
          :class="
            locale === loc.id
              ? 'border-accent bg-accent-bg text-accent'
              : 'border-line bg-surface text-ink hover:bg-raised'
          "
          :aria-pressed="locale === loc.id"
        >
          {{ loc.label }}
        </button>
      </div>
    </div>

    <!-- 视觉无障碍与质感优化 (Accessibility & Glass Effects) -->
    <div class="space-y-3 pt-1">
      <div>
        <div class="text-xs font-semibold text-ink flex items-center gap-1.5">
          <Contrast class="w-3.5 h-3.5 text-accent" />
          <span>{{ t("settings.appearance.accessibility.title") }}</span>
        </div>
        <p class="text-[11px] text-ink-secondary mt-0.5">
          {{ t("settings.appearance.accessibility.desc") }}
        </p>
      </div>

      <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
        <!-- 高对比度模式 -->
        <label
          class="flex items-center justify-between p-3.5 rounded-xl border border-line bg-surface hover:bg-raised transition cursor-pointer select-none"
        >
          <div class="flex items-center gap-2.5 min-w-0 pr-2">
            <div class="w-7 h-7 rounded-lg bg-raised flex items-center justify-center shrink-0 text-accent">
              <Contrast class="w-4 h-4" />
            </div>
            <div class="min-w-0">
              <div class="text-xs font-medium text-ink">{{ t("settings.appearance.highcontrast.title") }}</div>
              <div class="text-[10px] text-ink-muted mt-0.5 truncate">{{ t("settings.appearance.highcontrast.desc") }}</div>
            </div>
          </div>
          <input
            type="checkbox"
            :checked="highContrast"
            @change="toggleHighContrast"
            class="w-4 h-4 rounded border-line bg-canvas text-accent focus:ring-0 cursor-pointer shrink-0"
          />
        </label>

        <!-- 减少透明度与模糊 -->
        <label
          class="flex items-center justify-between p-3.5 rounded-xl border border-line bg-surface hover:bg-raised transition cursor-pointer select-none"
        >
          <div class="flex items-center gap-2.5 min-w-0 pr-2">
            <div class="w-7 h-7 rounded-lg bg-raised flex items-center justify-center shrink-0 text-accent">
              <Layers class="w-4 h-4" />
            </div>
            <div class="min-w-0">
              <div class="text-xs font-medium text-ink">{{ t("settings.appearance.transparency.title") }}</div>
              <div class="text-[10px] text-ink-muted mt-0.5 truncate">{{ t("settings.appearance.transparency.desc") }}</div>
            </div>
          </div>
          <input
            type="checkbox"
            :checked="reduceTransparency"
            @change="toggleReduceTransparency"
            class="w-4 h-4 rounded border-line bg-canvas text-accent focus:ring-0 cursor-pointer shrink-0"
          />
        </label>
      </div>
    </div>

    <div class="p-4 rounded-xl border border-line bg-surface/40 flex items-center justify-between gap-4">
      <div>
        <div class="text-xs font-medium text-ink">{{ t("settings.appearance.semantic.title") }}</div>
        <div class="text-[10px] text-ink-muted mt-0.5">{{ t("settings.appearance.semantic.desc") }}</div>
      </div>
      <div class="flex items-center gap-1.5 shrink-0">
        <span class="w-3.5 h-3.5 rounded-full bg-accent-strong" title="accent" />
        <span class="w-3.5 h-3.5 rounded-full bg-success-strong" title="success" />
        <span class="w-3.5 h-3.5 rounded-full bg-warning-strong" title="warning" />
        <span class="w-3.5 h-3.5 rounded-full bg-danger-strong" title="danger" />
      </div>
    </div>
  </section>
</template>
