import { reactive } from "vue";

/** Cross-tab status consumed by the Tools header (spinner + tab badges). */
export const toolsStatus = reactive({
  loading: {} as Record<string, boolean>,
  counts: {} as Record<string, number>,
});
