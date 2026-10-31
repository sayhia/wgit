<script setup lang="ts">
import { computed, onMounted } from "vue";
import {
  ensureFlagsLoaded,
  isFeatureEnabled,
  type FeatureFlagKey,
} from "../../composables/useFeatureFlags";

const props = defineProps<{ flag: string }>();

// App.vue must stay untouched: the gate lazily bootstraps the flag set on its
// first render (ensureFlagsLoaded is an idempotent singleton), and the reactive
// flag store re-renders this component once the backend answer lands.
onMounted(() => {
  void ensureFlagsLoaded();
});

const enabled = computed(() => isFeatureEnabled(props.flag as FeatureFlagKey));
</script>

<template>
  <!-- Enabled: render the gated content. Disabled: render the optional
       fallback slot, otherwise nothing at all (unknown flags never throw). -->
  <slot v-if="enabled" />
  <slot v-else name="fallback" />
</template>
