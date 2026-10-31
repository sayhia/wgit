# 设置页整页改版 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将设置页重构为“4 组 13 叶子”的分组导航 + 满宽内容，并把 AI 设置拆成连接/行为/用量与诊断三个独立分区，能力零回退。

**Architecture:** 新增纯数据的设置结构模块与共享外壳组件；先在不改动现有 UI 的前提下构建并测试三个 AI 分区，最后一次性切换 `SettingsView` 到新导航与满宽布局，再统一 10 个非 AI 分区的外壳样式。所有 RPC、config schema、数据库保持不变。

**Tech Stack:** Vue 3.5 + TypeScript + Tailwind v4 + Bun test + @vue/test-utils + Playwright。

**Spec:** `docs/superpowers/specs/2026-10-31-settings-full-redesign.md`

## Global Constraints

- 不改后端 RPC、`AIConfig` 字段、SQLite schema、Keychain 行为。
- 保留原 11 类设置能力与 AI 11 项能力；不删除旧配置迁移、多源密钥、导入导出、5 个灰度开关。
- 原 10 个叶子 id 保持不变；`localai` 仅作为 `ai-connection` 的兼容别名。
- 右侧内容满宽，移除 `max-w-3xl`；1440/1024/768/390 四档不得横向溢出。
- 所有新增界面文案必须同时写入 `zh-CN.ts` 与 `en-US.ts`。
- 不 commit / 不 push，除非用户明确确认；确认后只暂存本计划涉及的文件。
- 工作区可能含并行改动；不得 reset、不得整文件 checkout，只按路径/hunk 操作。
- 每个任务结束运行对应测试；TDD 顺序不可跳过。

---

## File Structure

**新增**

- `frontend/src/utils/settingsStructure.ts` — 分组、叶子、别名归一化（纯数据）。
- `frontend/src/components/settings/SettingsCard.vue` — 统一卡片表面。
- `frontend/src/components/settings/SettingsPageHeader.vue` — 叶子标题/说明。
- `frontend/src/components/settings/SettingsNav.vue` — 分组导航 + 搜索 + 历史。
- `frontend/src/components/settings/SettingsSearch.vue` — 搜索输入/结果/高亮。
- `frontend/src/components/settings/AiConnectionForm.vue` — 合并后的连接编辑器。
- `frontend/src/components/settings/AiProfilesCard.vue` — 档案切换/重命名/删除。
- `frontend/src/components/settings/AiKeysCard.vue` — 多源密钥。
- `frontend/src/components/settings/AiConnectionSection.vue` — 连接叶子。
- `frontend/src/components/settings/AiBehaviorSection.vue` — 行为叶子。
- `frontend/src/components/settings/AiInsightsSection.vue` — 用量与诊断叶子。
- `frontend/src/components/settings/AiTaskRoutingCard.vue` — 任务模型覆盖。
- `frontend/src/components/settings/AiReliabilityCard.vue` — 重试/限流/降级。
- `frontend/src/components/settings/AiDiagnosticsCard.vue` — 只读诊断。
- `frontend/src/components/settings/AiTransferActions.vue` — 配置导入/导出。

**修改**

- `frontend/src/components/SettingsView.vue`
- `frontend/src/utils/settingsIndex.ts`
- `frontend/src/commands/registry.ts`
- `frontend/src/i18n/messages/zh-CN.ts`、`en-US.ts`
- `frontend/src/components/settings/AiQuickSetupModal.vue`
- 10 个非 AI `*Section.vue`（仅头部/卡片样式）
- 相关测试与 `e2e/shots.cjs` 基线

**删除（最后一批，测试迁移后）**

- `frontend/src/components/settings/AiControlPlaneSection.vue`
- `frontend/src/components/settings/AiQuickSetup.vue`
- `frontend/src/components/settings/LocalAiSection.vue`

---

### Task 1: 设置分组与叶子模型

**Files:**
- Create: `frontend/src/utils/settingsStructure.ts`
- Test: `frontend/tests/settingsStructure.test.ts`

**Interfaces:**
- Produces: `SettingsLeaf`、`SettingsGroupId`、`SETTINGS_GROUPS`、`SETTINGS_LEAVES`、`SETTINGS_LEGACY_ALIASES`、`normalizeSettingsLeaf(value: string | null | undefined): SettingsLeaf`

- [ ] **Step 1: 写失败测试**

```ts
import { describe, expect, test } from "bun:test";
import {
  SETTINGS_GROUPS,
  SETTINGS_LEAVES,
  normalizeSettingsLeaf,
} from "../src/utils/settingsStructure";

describe("settings structure", () => {
  test("exposes four non-empty groups covering thirteen unique leaves", () => {
    expect(SETTINGS_GROUPS.map((group) => group.id)).toEqual([
      "general",
      "git",
      "ai",
      "system",
    ]);
    expect(SETTINGS_GROUPS.every((group) => group.leaves.length > 0)).toBe(true);
    expect(new Set(SETTINGS_LEAVES).size).toBe(13);
  });

  test("normalizes the legacy localai leaf and rejects unknown ids", () => {
    expect(normalizeSettingsLeaf("localai")).toBe("ai-connection");
    expect(normalizeSettingsLeaf("ai-behavior")).toBe("ai-behavior");
    expect(normalizeSettingsLeaf("nope")).toBe("appearance");
    expect(normalizeSettingsLeaf(undefined)).toBe("appearance");
  });
});
```

- [ ] **Step 2: 运行并确认失败**

Run: `cd frontend && bun test tests/settingsStructure.test.ts`
Expected: FAIL — module not found.

- [ ] **Step 3: 实现**

```ts
export type SettingsGroupId = "general" | "git" | "ai" | "system";

export type SettingsLeaf =
  | "appearance"
  | "identity"
  | "shortcuts"
  | "signing"
  | "tools"
  | "aliases"
  | "hooks"
  | "ai-connection"
  | "ai-behavior"
  | "ai-insights"
  | "proxy"
  | "config"
  | "about";

export interface SettingsGroup {
  id: SettingsGroupId;
  labelKey: string;
  leaves: readonly SettingsLeaf[];
}

export const SETTINGS_GROUPS: readonly SettingsGroup[] = [
  { id: "general", labelKey: "settings.group.general", leaves: ["appearance", "identity", "shortcuts"] },
  { id: "git", labelKey: "settings.group.git", leaves: ["signing", "tools", "aliases", "hooks"] },
  { id: "ai", labelKey: "settings.group.ai", leaves: ["ai-connection", "ai-behavior", "ai-insights"] },
  { id: "system", labelKey: "settings.group.system", leaves: ["proxy", "config", "about"] },
];

export const SETTINGS_LEAVES = SETTINGS_GROUPS.flatMap((group) => group.leaves);

export const SETTINGS_LEGACY_ALIASES: Readonly<Record<string, SettingsLeaf>> = {
  localai: "ai-connection",
};

export function normalizeSettingsLeaf(value: string | null | undefined): SettingsLeaf {
  const raw = (value ?? "").trim();
  const mapped = SETTINGS_LEGACY_ALIASES[raw] ?? raw;
  return (SETTINGS_LEAVES as readonly string[]).includes(mapped)
    ? (mapped as SettingsLeaf)
    : "appearance";
}
```

- [ ] **Step 4: 运行并确认通过**

Run: `cd frontend && bun test tests/settingsStructure.test.ts`
Expected: PASS（2 tests）。

- [ ] **Step 5: 状态检查（不提交）**

Run: `git status --short -- frontend/src/utils/settingsStructure.ts frontend/tests/settingsStructure.test.ts`
Expected: 仅这两个文件变化。

---

### Task 2: 共享卡片与标题组件

**Files:**
- Create: `frontend/src/components/settings/SettingsCard.vue`
- Create: `frontend/src/components/settings/SettingsPageHeader.vue`
- Test: `frontend/tests/settingsPrimitives.test.ts`

**Interfaces:**
- Produces: `SettingsCard`（默认 slot、`header` slot、`actions` slot）；`SettingsPageHeader`（`title`、`description` props，`actions` slot）。

- [ ] **Step 1: 写失败测试**

```ts
import { describe, expect, test } from "bun:test";
import { mount } from "@vue/test-utils";
import SettingsCard from "../src/components/settings/SettingsCard.vue";
import SettingsPageHeader from "../src/components/settings/SettingsPageHeader.vue";
import "./domSetup";

test("SettingsCard renders header/actions/default slots", () => {
  const wrapper = mount(SettingsCard, {
    slots: { header: "<span>Header</span>", actions: "<button>Act</button>", default: "<p>Body</p>" },
  });
  expect(wrapper.text()).toContain("Header");
  expect(wrapper.text()).toContain("Act");
  expect(wrapper.text()).toContain("Body");
  expect(wrapper.find("section").exists()).toBe(true);
});

test("SettingsPageHeader renders one heading and description", () => {
  const wrapper = mount(SettingsPageHeader, {
    props: { title: "AI 连接", description: "配置 API 或 CLI" },
  });
  expect(wrapper.find("h2").text()).toBe("AI 连接");
  expect(wrapper.text()).toContain("配置 API 或 CLI");
});
```

- [ ] **Step 2: 运行并确认失败**

Run: `cd frontend && bun test tests/settingsPrimitives.test.ts`
Expected: FAIL — components not found.

- [ ] **Step 3: 实现**

`SettingsCard.vue`：

```vue
<script setup lang="ts">
defineProps<{ padded?: boolean }>();
</script>

<template>
  <section class="rounded-2xl border border-line bg-surface/60">
    <div v-if="$slots.header || $slots.actions" class="flex flex-wrap items-start justify-between gap-3 border-b border-line/60 px-4 py-3">
      <div class="min-w-0"><slot name="header" /></div>
      <div v-if="$slots.actions" class="flex flex-wrap items-center gap-2"><slot name="actions" /></div>
    </div>
    <div :class="padded === false ? '' : 'p-4'"><slot /></div>
  </section>
</template>
```

`SettingsPageHeader.vue`：

```vue
<script setup lang="ts">
defineProps<{ title: string; description?: string }>();
</script>

<template>
  <header class="mb-4">
    <h2 class="text-sm font-bold text-ink">{{ title }}</h2>
    <p v-if="description" class="mt-1 text-xs text-ink-secondary leading-relaxed">{{ description }}</p>
    <slot name="actions" />
  </header>
</template>
```

- [ ] **Step 4: 运行并确认通过**

Run: `cd frontend && bun test tests/settingsPrimitives.test.ts`
Expected: PASS（2 tests）。

- [ ] **Step 5: 状态检查**

Run: `git status --short -- frontend/src/components/settings/SettingsCard.vue frontend/src/components/settings/SettingsPageHeader.vue frontend/tests/settingsPrimitives.test.ts`

---

### Task 3: 连接编辑器基础组件（从 AiQuickSetup 迁移）

**Files:**
- Create: `frontend/src/components/settings/AiConnectionForm.vue`（以现有 `AiQuickSetup.vue` 为底）
- Test: `frontend/tests/aiConnectionForm.test.ts`（迁移并裁剪 `aiQuickSetupBehavior.test.ts`）

**Interfaces:**
- Props: `refreshKey?: number`, `compact?: boolean`
- Emits: `saved: []`
- 复用现有 `getAIProviderCatalog/getAIConfig/listAIKeys/listAIModels/saveAISettings/testAIConnectionResult/detectAICLIs`。

- [ ] **Step 1: 复制组件并改名为 `AiConnectionForm.vue`**

操作：复制 `AiQuickSetup.vue` 全文到 `AiConnectionForm.vue`，把内部组件注释/标题改为“连接”；`defineProps`/`defineEmits` 保持。

- [ ] **Step 2: 写失败测试（迁移现有挂载测试）**

把 `tests/aiQuickSetupBehavior.test.ts` 复制为 `tests/aiConnectionForm.test.ts`，保留其 `mock.module("@wailsio/runtime")`、CATALOG/CONFIG 与 `flushPromises` 基座，并补充：

```ts
import { spyOn } from "bun:test";
import * as aiService from "../src/services/aiService";
const AiConnectionForm = (await import("../src/components/settings/AiConnectionForm.vue")).default;
```

并保留以下断言（去掉已删除的 profile 条）：

```ts
test("renders preset, model and key fields", async () => {
  const wrapper = mount(AiConnectionForm);
  await flushPromises();
  expect(wrapper.find("[data-testid='ai-quick-preset-openai']").exists()).toBe(true);
  expect(wrapper.find("#ai-quick-model-input").exists()).toBe(true);
  expect(wrapper.find("#ai-quick-key-input").exists()).toBe(true);
  expect(wrapper.find("[data-testid='ai-quick-save']").exists()).toBe(true);
});

test("compact mode keeps the preset, model and key fields", async () => {
  const wrapper = mount(AiConnectionForm, { props: { compact: true } });
  await flushPromises();
  expect(wrapper.find("#ai-quick-model-input").exists()).toBe(true);
  expect(wrapper.find("#ai-quick-key-input").exists()).toBe(true);
});
```

- [ ] **Step 3: 运行并确认失败**

Run: `cd frontend && bun test tests/aiConnectionForm.test.ts`
Expected: FAIL — file/module or renamed test ids missing.

- [ ] **Step 4: 让组件通过测试**

确认 `AiConnectionForm.vue` 保留 `data-testid="ai-quick-*"` 与现有保存/测试逻辑；若复制时改动了 testid，改回原值。运行：

Run: `cd frontend && bun test tests/aiConnectionForm.test.ts`
Expected: PASS。

- [ ] **Step 5: 状态检查**

Run: `git status --short -- frontend/src/components/settings/AiConnectionForm.vue frontend/tests/aiConnectionForm.test.ts`

---

### Task 4: 连接编辑器增加“高级连接”

**Files:**
- Modify: `frontend/src/components/settings/AiConnectionForm.vue`
- Modify: `frontend/tests/aiConnectionForm.test.ts`

**Interfaces:**
- `savePayload()` 必须包含 `protocol`、`timeoutSeconds`、`extraHeaders`、`cliArgs`、`cliInput`，并继续调用 `saveAISettings`。
- `compact === true` 时高级区不渲染。

- [ ] **Step 1: 写失败测试**

```ts
test("advanced connection fields are editable and saved", async () => {
  const saveSpy = spyOn(aiService, "saveAISettings").mockResolvedValue();
  const wrapper = mount(AiConnectionForm);
  await flushPromises();
  await wrapper.find("[data-testid='ai-connection-advanced'] summary").trigger("click");
  await wrapper.find("[data-testid='ai-connection-protocol']").setValue("anthropic");
  await wrapper.find("[data-testid='ai-connection-timeout']").setValue("90");
  await wrapper.find("[data-testid='ai-connection-headers']").setValue("X-Test: 1");
  await wrapper.find("[data-testid='ai-quick-save-only']").trigger("click");
  await flushPromises();
  expect(saveSpy).toHaveBeenCalled();
  const payload = saveSpy.mock.calls[0]![0] as Record<string, unknown>;
  expect(payload.protocol).toBe("anthropic");
  expect(payload.timeoutSeconds).toBe(90);
  expect(payload.extraHeaders).toEqual({ "X-Test": "1" });
  saveSpy.mockRestore();
});

test("compact mode hides the advanced connection disclosure", async () => {
  const wrapper = mount(AiConnectionForm, { props: { compact: true } });
  await flushPromises();
  expect(wrapper.find("[data-testid='ai-connection-advanced']").exists()).toBe(false);
});
```

- [ ] **Step 2: 运行并确认失败**

Run: `cd frontend && bun test tests/aiConnectionForm.test.ts -t "advanced connection"`
Expected: FAIL — selectors missing.

- [ ] **Step 3: 实现**

在 `AiConnectionForm.vue` 内新增：

```vue
<details v-if="!compact" data-testid="ai-connection-advanced" class="rounded-xl border border-line bg-canvas/40 px-3 py-2">
  <summary class="cursor-pointer text-[11px] font-semibold text-ink">高级连接</summary>
  <div class="mt-2 grid grid-cols-1 gap-3 lg:grid-cols-2">
    <label class="space-y-1">
      <span class="ai-field-label">协议</span>
      <select v-model="form.protocol" data-testid="ai-connection-protocol" class="ai-input">
        <option value="openai">OpenAI 兼容</option>
        <option value="anthropic">Anthropic</option>
        <option value="gemini">Gemini</option>
        <option value="azure">Azure OpenAI</option>
      </select>
    </label>
    <label class="space-y-1">
      <span class="ai-field-label">超时（秒）</span>
      <input v-model.number="form.timeoutSeconds" type="number" min="0" max="600" data-testid="ai-connection-timeout" class="ai-input" />
    </label>
  </div>
  <label class="mt-3 block space-y-1">
    <span class="ai-field-label">自定义请求头（每行 Key: Value）</span>
    <textarea v-model="headersText" data-testid="ai-connection-headers" class="ai-input min-h-16" />
  </label>
  <div class="mt-3 grid grid-cols-1 gap-3 lg:grid-cols-2">
    <label class="space-y-1">
      <span class="ai-field-label">CLI 参数（空格分隔）</span>
      <input v-model="cliArgsText" data-testid="ai-connection-cli-args" class="ai-input" />
    </label>
    <label class="space-y-1">
      <span class="ai-field-label">CLI 输入方式</span>
      <select v-model="form.cliInput" data-testid="ai-connection-cli-input" class="ai-input">
        <option value="stdin">stdin</option>
        <option value="arg">arg</option>
      </select>
    </label>
  </div>
</details>
```

先扩展现有响应式表单（保留已有字段）：

```ts
const form = reactive({
  // ...existing fields
  protocol: "openai",
  timeoutSeconds: 60,
  cliInput: "stdin" as "stdin" | "arg",
});
const headersText = ref("");
const cliArgsText = ref("");
```

在 `savePayload()` 中加入：

```ts
protocol: form.protocol,
timeoutSeconds: form.timeoutSeconds || 0,
extraHeaders: parseHeaders(headersText.value),
cliArgs: cliArgsText.value.split(/\s+/).filter(Boolean),
cliInput: form.cliInput,
```

`parseHeaders` 实现（只接受 `K: V` 行，忽略空行）：

```ts
function parseHeaders(text: string): Record<string, string> {
  const out: Record<string, string> = {};
  for (const line of text.split("\n")) {
    const index = line.indexOf(":");
    if (index <= 0) continue;
    const key = line.slice(0, index).trim();
    const value = line.slice(index + 1).trim();
    if (key && value) out[key] = value;
  }
  return out;
}
```

非法请求头继续走现有 `settings.localai.headers.invalid` 警告，不提交。`compact` 时强制基础模式并把 `headersText`/`cliArgsText` 保留为空。

- [ ] **Step 4: 运行并确认通过**

Run: `cd frontend && bun test tests/aiConnectionForm.test.ts`
Expected: PASS（含原有迁移用例）。

- [ ] **Step 5: 状态检查**

Run: `git status --short -- frontend/src/components/settings/AiConnectionForm.vue frontend/tests/aiConnectionForm.test.ts`

---

### Task 5: 档案卡与多源密钥卡

**Files:**
- Create: `frontend/src/components/settings/AiProfilesCard.vue`
- Create: `frontend/src/components/settings/AiKeysCard.vue`
- Test: `frontend/tests/aiProfilesKeys.test.ts`

**Interfaces:**
- `AiProfilesCard` emits `changed`；内部调用 `listAIProfiles/activateAIProfile/deleteAIProfile/getAIAdvancedConfig/saveAIAdvancedConfig`。
- `AiKeysCard` emits `changed`；内部调用 `listAIKeys/deleteAIKeyForOrigin`。

- [ ] **Step 1: 写失败测试**

```ts
import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

const read = (p: string) => readFileSync(fileURLToPath(new URL(p, import.meta.url)), "utf8");

test("profile card owns rename and delete controls", () => {
  const src = read("../src/components/settings/AiProfilesCard.vue");
  expect(src).toContain("ai-profile-rename");
  expect(src).toContain("activateAIProfile");
  expect(src).toContain("deleteAIProfile");
});

test("key card owns multi-origin deletion", () => {
  const src = read("../src/components/settings/AiKeysCard.vue");
  expect(src).toContain("ai-keys-card");
  expect(src).toContain("deleteAIKeyForOrigin");
});
```

- [ ] **Step 2: 运行并确认失败**

Run: `cd frontend && bun test tests/aiProfilesKeys.test.ts`
Expected: FAIL — files not found.

- [ ] **Step 3: 实现**

把 `AiControlPlaneSection.vue` 中：
- `profiles` 状态、`switchProfile/startRenameProfile/cancelRenameProfile/saveProfileName/removeProfile` 与 profiles 卡模板（约 668–746 行）迁到 `AiProfilesCard.vue`；
- `keys` 状态、`removeKey` 与 keys 卡模板（约 748 行起）迁到 `AiKeysCard.vue`。
两个组件都用 `SettingsCard` 包裹，加载失败显示 `role="alert"` 横幅，保存/删除成功 `pushToast` 并 `emit("changed")`。
不要复制 `isLoading` 全局状态；每个卡自己的 `isLoading`/`error`。

- [ ] **Step 4: 运行并确认通过**

Run: `cd frontend && bun test tests/aiProfilesKeys.test.ts`
Expected: PASS（2 tests）。

- [ ] **Step 5: 状态检查**

Run: `git status --short -- frontend/src/components/settings/AiProfilesCard.vue frontend/src/components/settings/AiKeysCard.vue frontend/tests/aiProfilesKeys.test.ts`

---

### Task 6: 连接叶子（表单 + 档案 + 密钥 + 旧迁移）

**Files:**
- Create: `frontend/src/components/settings/AiConnectionSection.vue`
- Test: `frontend/tests/aiConnectionSection.test.ts`

**Interfaces:**
- Uses `AiConnectionForm`, `AiProfilesCard`, `AiKeysCard`, `readLegacyLocalAiConfig`, `clearLegacyLocalAiKey`。
- Emits `changed`；`refreshKey` prop 透传给子卡。

- [ ] **Step 1: 写失败测试**

```ts
import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

const src = readFileSync(
  fileURLToPath(new URL("../src/components/settings/AiConnectionSection.vue", import.meta.url)),
  "utf8"
);
test("connection section composes form, profiles, keys and legacy migration", () => {
  for (const text of [
    "AiConnectionForm",
    "AiProfilesCard",
    "AiKeysCard",
    "localai-legacy-banner",
    "readLegacyLocalAiConfig",
  ]) expect(src).toContain(text);
  expect(src).not.toContain("localai-backend-status");
});
```

- [ ] **Step 2: 运行并确认失败**

Run: `cd frontend && bun test tests/aiConnectionSection.test.ts`
Expected: FAIL — file not found.

- [ ] **Step 3: 实现**

从 `LocalAiSection.vue` 搬移：
- `legacyConfig`、`legacyBusy`、`readLegacyLocalAiConfig`、`importLegacyKey`、`discardLegacyKey` 与 legacy banner 模板；
- 页面结构：

```vue
<template>
  <div class="space-y-4">
    <AiConnectionForm :refresh-key="refreshKey" @saved="emit('changed')" />
    <AiProfilesCard :refresh-key="refreshKey" @changed="emit('changed')" />
    <AiKeysCard :refresh-key="refreshKey" @changed="emit('changed')" />
    <div v-if="legacyConfig" data-testid="localai-legacy-banner" class="...">...</div>
  </div>
</template>
```

不保留手动连接表单、模式切换、状态卡；这些已由 `AiConnectionForm` 提供。

- [ ] **Step 4: 运行并确认通过**

Run: `cd frontend && bun test tests/aiConnectionSection.test.ts`
Expected: PASS。

- [ ] **Step 5: 状态检查**

Run: `git status --short -- frontend/src/components/settings/AiConnectionSection.vue frontend/tests/aiConnectionSection.test.ts`

---

### Task 7: 行为叶子（提交偏好 + 路由 + 可靠性 + 灰度）

**Files:**
- Create: `frontend/src/components/settings/AiTaskRoutingCard.vue`
- Create: `frontend/src/components/settings/AiReliabilityCard.vue`
- Create: `frontend/src/components/settings/AiBehaviorSection.vue`
- Test: `frontend/tests/aiBehaviorSection.test.ts`

**Interfaces:**
- `AiBehaviorSection` 加载 `getAIAdvancedConfig` 与 `featureFlagsApi.getFeatureFlags`；保存调用 `saveAIAdvancedConfig` / `saveAppFlags`。
- 保留 `reviewFlow` 灰度键与原有 5 个开关。

- [ ] **Step 1: 写失败测试**

```ts
test("behavior section keeps commit preferences, routing, reliability and five flags", () => {
  const src = read("../src/components/settings/AiBehaviorSection.vue");
  for (const text of [
    "getAIAdvancedConfig",
    "saveAIAdvancedConfig",
    "aiCommitMsg",
    "aiSummary",
    "aiConflictBatch",
    "reviewFlow",
    "assetBoard",
    "AiTaskRoutingCard",
    "AiReliabilityCard",
  ]) expect(src).toContain(text);
});
test("routing card exposes every task kind", () => {
  const src = read("../src/components/settings/AiTaskRoutingCard.vue");
  for (const kind of ["commit", "summary", "conflict", "explain", "chat", "review", "release"]) {
    expect(src).toContain(kind);
  }
});
```

- [ ] **Step 2: 运行并确认失败**

Run: `cd frontend && bun test tests/aiBehaviorSection.test.ts`
Expected: FAIL — files not found.

- [ ] **Step 3: 实现**

- 从 `AiControlPlaneSection.vue` 搬移 routing 卡（约 584–606 行）与 reliability/fallback 卡（约 607–667 行）到两个子组件。
- 从 `LocalAiSection.vue` 搬移 `FLAG_DEFS`、`flagState`、`toggleFlag` 与模板循环到 `AiBehaviorSection`。
- 提交偏好字段从 `LocalAiSection` 手动表单搬到行为区顶部（语言、含正文、指令），与 advanced config 一起保存。
- 行为区保存按钮统一为 `data-testid="ai-behavior-save"`；灰度开关仍即时保存。

- [ ] **Step 4: 运行并确认通过**

Run: `cd frontend && bun test tests/aiBehaviorSection.test.ts`
Expected: PASS。

- [ ] **Step 5: 状态检查**

---

### Task 8: 用量与诊断叶子

**Files:**
- Create: `frontend/src/components/settings/AiDiagnosticsCard.vue`
- Create: `frontend/src/components/settings/AiTransferActions.vue`
- Create: `frontend/src/components/settings/AiInsightsSection.vue`
- Test: `frontend/tests/aiInsightsSection.test.ts`

**Interfaces:**
- `AiInsightsSection` 组合 `AiUsageCard`、`AiDiagnosticsCard`、`AiTransferActions`。
- 诊断只读；导入/导出沿用现有 RPC 与密钥拒绝逻辑。

- [ ] **Step 1: 写失败测试**

```ts
test("insights section keeps usage, diagnostics and transfer", () => {
  const src = read("../src/components/settings/AiInsightsSection.vue");
  for (const text of ["AiUsageCard", "AiDiagnosticsCard", "AiTransferActions"]) {
    expect(src).toContain(text);
  }
});
test("diagnostics card never calls a network send", () => {
  const src = read("../src/components/settings/AiDiagnosticsCard.vue");
  expect(src).toContain("ai-diagnostics-card");
  expect(src).not.toContain("StartAIStream");
  expect(src).not.toContain("testAIConnection");
});
```

- [ ] **Step 2: 运行并确认失败**

Run: `cd frontend && bun test tests/aiInsightsSection.test.ts`
Expected: FAIL。

- [ ] **Step 3: 实现**

- 从 `AiControlPlaneSection.vue` 搬移 diagnostics 卡（约 556–583 行）、导入/导出按钮与 `exportConfig/pickImportFile/handleImportFile`。
- `AiTransferActions` 保留 `data-testid="ai-config-export"/"ai-config-import"`。
- `AiInsightsSection` 顺序：用量、诊断、导入/导出；不再包含 profiles/keys/routing/reliability。

- [ ] **Step 4: 运行并确认通过**

Run: `cd frontend && bun test tests/aiInsightsSection.test.ts`
Expected: PASS。

- [ ] **Step 5: 状态检查**

---

### Task 9: 设置页外壳切换（分组导航 + 满宽 + 13 叶子）

**Files:**
- Create: `frontend/src/components/settings/SettingsNav.vue`
- Create: `frontend/src/components/settings/SettingsSearch.vue`
- Modify: `frontend/src/components/SettingsView.vue`
- Modify: `frontend/src/utils/settingsIndex.ts`
- Modify: `frontend/src/i18n/messages/zh-CN.ts`、`en-US.ts`
- Modify: `frontend/src/commands/registry.ts`
- Modify: `frontend/src/components/settings/AiQuickSetupModal.vue`
- Delete: `AiControlPlaneSection.vue`、`AiQuickSetup.vue`、`LocalAiSection.vue`
- Test: `frontend/tests/settingsShell.test.ts`

**Interfaces:**
- `SettingsNav` props: `activeLeaf: SettingsLeaf`；emits `select-leaf: [SettingsLeaf]`；内部用 `SETTINGS_GROUPS`、`SETTINGS_CATEGORY_LABEL_KEYS`、`SETTINGS_ITEM_INDEX`、`t()`。
- `SettingsView` 使用 `normalizeSettingsLeaf`；`activeLeaf` 类型 `SettingsLeaf`；内容分发 13 个叶子。

- [ ] **Step 1: 写失败测试**

```ts
import { describe, expect, test } from "bun:test";
import { mount } from "@vue/test-utils";
import SettingsNav from "../src/components/settings/SettingsNav.vue";
import SettingsView from "../src/components/SettingsView.vue";
import "./domSetup";

test("nav renders four groups and thirteen leaves", () => {
  const wrapper = mount(SettingsNav, { props: { activeLeaf: "ai-connection" } });
  expect(wrapper.findAll("[data-settings-leaf]").length).toBe(13);
  expect(wrapper.text()).toContain("AI");
  expect(wrapper.find("[data-settings-leaf='ai-connection']").attributes("aria-current")).toBe("page");
});

test("settings view uses the full-width column", () => {
  const src = readFileSync(fileURLToPath(new URL("../src/components/SettingsView.vue", import.meta.url)), "utf8");
  expect(src).not.toContain("max-w-3xl");
  expect(src).toContain("normalizeSettingsLeaf");
});
```

- [ ] **Step 2: 运行并确认失败**

Run: `cd frontend && bun test tests/settingsShell.test.ts`
Expected: FAIL — nav missing / max-w-3xl still present.

- [ ] **Step 3: 实现**

- 从 `SettingsView.vue` 抽出搜索状态/历史/高亮/键盘逻辑到 `SettingsSearch.vue`，`SettingsNav` 组合搜索与分组。
- 导航结构：

```vue
<nav :aria-label="t('settings.nav.label')" class="...">
  <SettingsSearch ... />
  <div v-for="group in SETTINGS_GROUPS" :key="group.id" role="group" :aria-labelledby="`settings-group-${group.id}`">
    <h3 :id="`settings-group-${group.id}`" class="...">{{ t(group.labelKey) }}</h3>
    <button
      v-for="leaf in group.leaves"
      :key="leaf"
      :data-settings-leaf="leaf"
      :aria-current="activeLeaf === leaf ? 'page' : undefined"
      @click="emit('select-leaf', leaf)"
    >{{ t(SETTINGS_CATEGORY_LABEL_KEYS[leaf]) }}</button>
  </div>
</nav>
```

- `SettingsView` 内容分发：AI 三叶子渲染 `AiConnectionSection`/`AiBehaviorSection`/`AiInsightsSection`；10 个旧叶子原样。`localai` 深链经 `normalizeSettingsLeaf` 到 `ai-connection`。
- 移除右侧 `max-w-3xl mx-auto`，改用 `w-full`。
- `settingsIndex.ts` 把原 `localai` 条目拆成 3 条 AI 条目；新增 `settings.category.ai-connection/ai-behavior/ai-insights` 与 `settings.group.*` 双语 key。
- `commands/registry.ts` 追加 3 条 AI 设置命令，保留旧命令。
- `AiQuickSetupModal.vue` 改用 `AiConnectionForm`。
- 同步更新引用旧组件的测试：`tests/aiQuickSetupBehavior.test.ts`（删除或改为新组件）、`tests/aiControlPlane.test.ts`（profiles/keys/transfer 断言迁移到新组件）、`tests/settingsAiFlags.test.ts`（LocalAiSection 源断言改为 SettingsView/新 AI 区）、`tests/aiConfigDedup.test.ts`（改为新组件结构）。
- 全局搜索 `LocalAiSection|AiQuickSetup|AiControlPlaneSection`，确认无运行时代码引用后再删除三个旧组件。

- [ ] **Step 4: 运行并确认通过**

Run: `cd frontend && bun test tests/settingsStructure.test.ts tests/settingsShell.test.ts tests/aiConfigDedup.test.ts tests/aiConnectionForm.test.ts tests/aiBehaviorSection.test.ts tests/aiInsightsSection.test.ts`
Expected: PASS。

Run: `cd frontend && bun run typecheck`
Expected: PASS。

- [ ] **Step 5: 状态检查**

Run: `git status --short -- frontend/src/components/SettingsView.vue frontend/src/components/settings frontend/src/utils/settingsIndex.ts frontend/src/commands/registry.ts frontend/src/i18n/messages`

---

### Task 10: 非 AI 分区样式统一与验收

**Files:**
- Modify: 10 个 `frontend/src/components/settings/*Section.vue`
- Modify: `e2e/shots.cjs`（仅必要基线）
- Test: `frontend/tests/settingsShell.test.ts`（补充）

**Interfaces:**
- 每个分区移除自带 `<h2>` + intro（由 `SettingsPageHeader` 统一），保留内部卡片业务逻辑。
- 顶层卡片统一 `SettingsCard`；间距统一 `space-y-4/5`。

- [ ] **Step 1: 写失败守卫**

```ts
test("legacy sections no longer duplicate the page heading", () => {
  for (const file of ["AppearanceSection", "IdentitySection", "SigningSection", "ProxySection", "ToolsSection", "AliasesSection", "GitHooksSection", "AdvancedConfigSection", "ShortcutsSection", "AboutSection"]) {
    const src = read(`../src/components/settings/${file}.vue`);
    expect(src).not.toMatch(/<h2[^>]*>\{\{\s*t\("settings\.[^"]+\.title"\)/);
  }
});
```

- [ ] **Step 2: 运行并确认失败**

Run: `cd frontend && bun test tests/settingsShell.test.ts -t "duplicate"`
Expected: FAIL（至少一个分区仍有主标题）。

- [ ] **Step 3: 实现**

逐个分区删除顶部：

```vue
<div>
  <h2 class="text-sm font-bold text-ink">{{ t("settings.<name>.title") }}</h2>
  <p class="text-xs text-ink-secondary mt-1">{{ t("settings.<name>.intro") }}</p>
</div>
```

由 `SettingsView` 的 `SettingsPageHeader` 渲染。为 `SETTINGS_CATEGORY_META` 增加 `descriptionKey`（复用各分区现有 `settings.*.intro` key；没有 intro 的分区补双语 key），并用它渲染描述。仅改外壳，不碰表单逻辑/RPC。

- [ ] **Step 4: 全量门禁**

```bash
cd frontend
bun test tests
bun run typecheck
bun run build
bun run audit
bun run shots:check
```

Expected: 全部通过；若视觉基线仅因设置页布局变化而失败，人工核对后更新设置页相关基线，不更新无关截图。

- [ ] **Step 5: 手工四档检查**

- 1440：左栏 240px、右侧满宽、无 `max-w-3xl` 死区。
- 1024：分组导航可用，内容无横向滚动。
- 768/390：顶部横向导航可滚动，内容不溢出；AI 三叶子可切换。
- 旧深链 `wgit:settings-section` 的 `localai` 与 `shortcuts` 均落到正确叶子。

---

## Self-Review

- **Spec coverage:** 分组/13 叶子（Task 1/9）、满宽（Task 9）、共享外壳（Task 2/10）、AI 连接/行为/用量（Task 3–8）、档案/密钥/迁移/灰度（Task 5–7）、搜索/深链/命令（Task 9）、验收（Task 10）。
- **Placeholder scan:** 未使用 TBD/TODO；迁移步骤均给出源组件、目标组件、选择器与测试。
- **Type consistency:** `SettingsLeaf`、`SETTINGS_GROUPS`、`normalizeSettingsLeaf` 在 Task 1 定义，Task 9 使用；`AiConnectionForm` 的 props/emit 在 Task 3 定义，Task 6/9 使用；`AiProfilesCard`/`AiKeysCard` 的 `changed` 事件在 Task 5 定义，Task 6 消费。
- **Known sequencing:** 旧 AI UI 在 Task 9 前保持可用；Task 3–8 是未接线的增量组件，Task 9 一次性切换并删除旧文件。

## Execution Handoff

计划保存于 `docs/superpowers/plans/2026-10-31-settings-full-redesign.md`。可选：

1. **Subagent-Driven（推荐）**：每个 Task 派新 subagent，完成后两阶段 review。
2. **Inline Execution**：在当前会话按 Task 批量执行，检查点 review。

未获确认前不开始改代码；不 commit/push。
