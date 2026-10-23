<script setup lang="ts">
import { computed, type Component } from "vue";
import { Monitor, Moon, Sun, Check, Languages } from "lucide-vue-next";
import { useTheme, type ThemeMode } from "../../composables/useTheme";
import { useAccent, type AccentId } from "../../composables/useAccent";
import { t, locale, setLocale, LOCALES, type Locale } from "../../i18n";

const emit = defineEmits<{ (e: "notify", msg: string): void }>();
const { mode, setThemeMode } = useTheme();
const { accent, setAccent, options } = useAccent();

const themeOptions = computed<{ value: ThemeMode; label: string; description: string; icon: Component }[]>(() => [
  { value: "dark", label: t("settings.theme.dark"), description: t("settings.theme.dark.desc"), icon: Moon },
  { value: "light", label: t("settings.theme.light"), description: t("settings.theme.light.desc"), icon: Sun },
  { value: "system", label: t("settings.theme.system"), description: t("settings.theme.system.desc"), icon: Monitor },
]);

function selectMode(next: ThemeMode) {
  if (mode.value === next) return;
  setThemeMode(next);
  emit("notify", next === "system" ? "已切换为跟随系统主题" : `已切换为${next === "dark" ? "深色" : "浅色"}主题`);
}

function selectLocale(next: Locale, label: string) {
  if (locale.value === next) return;
  setLocale(next);
  emit("notify", `界面语言已切换为 ${label}`);
}

function selectAccent(next: AccentId, label: string) {
  if (accent.value === next) return;
  setAccent(next);
  emit("notify", `强调色已切换为「${label}」`);
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
        只替换强调色族（按钮、选中态、链接、焦点环），表面与正文色保持原有对比度。
      </p>
      <div class="grid grid-cols-2 sm:grid-cols-3 gap-2">
        <button
          v-for="option in options"
          :key="option.id"
          @click="selectAccent(option.id, option.label)"
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
            <span class="block text-[11px] font-semibold text-ink">{{ option.label }}</span>
            <span class="block text-[10px] text-ink-muted truncate">{{ option.hint }}</span>
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

    <div class="p-4 rounded-xl border border-line bg-surface/40 flex items-center justify-between gap-4">
      <div>
        <div class="text-xs font-medium text-ink">语义色板</div>
        <div class="text-[10px] text-ink-muted mt-0.5">成功 / 警告 / 危险色固定不随强调色变化，避免语义被稀释。</div>
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
