import { Call } from "@wailsio/runtime";
import { t } from "../i18n";

export interface AiConflictAdvice {
  oursSummary: string;
  theirsSummary: string;
  recommendedResolution: string;
  explanation: string;
  source: "ai" | "heuristic";
}

export interface ParsedConflictBlock {
  ours: string;
  theirs: string;
  base?: string;
}

function parseConflictBlocksFromText(
  text: string,
  stripDiffPrefix: boolean
): ParsedConflictBlock[] {
  if (!text) return [];
  const lines = text.replace(/\r\n/g, "\n").split("\n");
  const blocks: ParsedConflictBlock[] = [];
  let inOurs = false;
  let inBase = false;
  let inTheirs = false;
  let oursLines: string[] = [];
  let baseLines: string[] = [];
  let theirsLines: string[] = [];

  for (let i = 0; i < lines.length; i++) {
    const rawLine = lines[i];
    if (rawLine === undefined) continue;
    const line =
      stripDiffPrefix && (rawLine.startsWith("+") || rawLine.startsWith(" "))
        ? rawLine.slice(1)
        : rawLine;

    if (line.startsWith("<<<<<<<")) {
      inOurs = true;
      inBase = false;
      inTheirs = false;
      oursLines = [];
      baseLines = [];
      theirsLines = [];
    } else if (inOurs && line.startsWith("|||||||")) {
      inOurs = false;
      inBase = true;
    } else if ((inOurs || inBase) && line.startsWith("=======")) {
      inOurs = false;
      inBase = false;
      inTheirs = true;
    } else if (inTheirs && line.startsWith(">>>>>>>")) {
      inTheirs = false;
      blocks.push({
        ours: oursLines.join("\n"),
        theirs: theirsLines.join("\n"),
        base: baseLines.length > 0 ? baseLines.join("\n") : undefined,
      });
    } else if (inOurs) {
      oursLines.push(line);
    } else if (inBase) {
      baseLines.push(line);
    } else if (inTheirs) {
      theirsLines.push(line);
    }
  }
  return blocks;
}

/** Extract conflict blocks from unified diff text (strips one diff prefix). */
export function extractConflictBlocks(text: string): ParsedConflictBlock[] {
  return parseConflictBlocksFromText(text, true);
}

/** Extract conflict blocks from a real conflicted file (preserves all text). */
export function parseConflictFileBlocks(text: string): ParsedConflictBlock[] {
  return parseConflictBlocksFromText(text, false);
}

export function extractFirstConflictBlock(
  text: string
): ParsedConflictBlock | null {
  const blocks = extractConflictBlocks(text);
  if (blocks.length > 0 && blocks[0]) {
    return blocks[0];
  }

  // Fallback for standard unified diff text when conflict markers aren't explicit
  const lines = text ? text.split("\n") : [];
  const oursLines: string[] = [];
  const theirsLines: string[] = [];
  for (const l of lines) {
    if (!l) continue;
    if (l.startsWith("-") && !l.startsWith("---")) {
      oursLines.push(l.slice(1));
    } else if (l.startsWith("+") && !l.startsWith("+++")) {
      theirsLines.push(l.slice(1));
    }
  }
  if (oursLines.length > 0 || theirsLines.length > 0) {
    return {
      ours: oursLines.join("\n"),
      theirs: theirsLines.join("\n"),
    };
  }
  return null;
}

const LEGACY_CONFIG_KEY = "wgit.localai.config.v1";

export interface LegacyLocalAiConfig {
  endpoint: string;
  model: string;
  apiKey: string;
}

/** Reads the plaintext key a pre-backend build left in localStorage. */
export function readLegacyLocalAiConfig(): LegacyLocalAiConfig | null {
  try {
    if (typeof localStorage === "undefined") return null;
    const raw = localStorage.getItem(LEGACY_CONFIG_KEY);
    if (!raw) return null;
    const parsed = JSON.parse(raw);
    const apiKey = typeof parsed?.apiKey === "string" ? parsed.apiKey.trim() : "";
    if (!apiKey) return null;
    const endpointRaw = typeof parsed?.endpoint === "string" ? parsed.endpoint : parsed?.baseUrl;
    return {
      endpoint: typeof endpointRaw === "string" ? endpointRaw.trim() : "",
      model: typeof parsed?.model === "string" ? parsed.model.trim() : "",
      apiKey,
    };
  } catch {
    return null;
  }
}

/** Removes only the legacy plaintext key, leaving unrelated legacy fields. */
export function clearLegacyLocalAiKey(): void {
  try {
    if (typeof localStorage === "undefined") return;
    const raw = localStorage.getItem(LEGACY_CONFIG_KEY);
    if (!raw) return;
    let parsed: Record<string, unknown>;
    try {
      parsed = JSON.parse(raw);
    } catch {
      localStorage.removeItem(LEGACY_CONFIG_KEY);
      return;
    }
    if (!parsed || typeof parsed !== "object") {
      localStorage.removeItem(LEGACY_CONFIG_KEY);
      return;
    }
    delete parsed.apiKey;
    if (Object.keys(parsed).length === 0) {
      localStorage.removeItem(LEGACY_CONFIG_KEY);
    } else {
      localStorage.setItem(LEGACY_CONFIG_KEY, JSON.stringify(parsed));
    }
  } catch {
    /* storage unavailable */
  }
}

/**
 * Analyzes Git merge conflict blocks and generates an intelligent resolution.
 *
 * AI requests go through `AiService.GenerateConflictAdvice`
 * (git_ai_service.go), gated by the aiConflictBatch feature flag, redacted and
 * audited server-side, with the plaintext key held in the OS secret store.
 *
 * The deterministic synthesizer is never mixed in silently: the advisor shows
 * an explicit error and lets the user choose the heuristic fallback, which is
 * labeled as such in the UI.
 */

/** Shape of the backend RPC: ours and theirs are mandatory, base optional. */
type ConflictAdviceRpc = (ours: string, theirs: string, base: string) => Promise<string>;

/** Backend conflict-advice RPC, addressed by its full AiService name. */
function defaultConflictAdviceRpc(ours: string, theirs: string, base: string): Promise<string> {
  return Call.ByName(
    "main.AiService.GenerateConflictAdvice",
    ours,
    theirs,
    base
  ) as Promise<string>;
}

// Seam so headless tests can simulate the backend (house pattern:
// resetFeatureFlagsForTest in composables/useFeatureFlags.ts).
let conflictAdviceRpc: ConflictAdviceRpc = defaultConflictAdviceRpc;

/** Test-only seam: swap the backend RPC; `null` restores the real one. */
export function setConflictAdviceBackendForTest(rpc: ConflictAdviceRpc | null): void {
  conflictAdviceRpc = rpc ?? defaultConflictAdviceRpc;
}

/**
 * Parses the backend's raw suggestion text into the pieces the advisor modal
 * renders: the contract (Go aiConflictAdvicePromptTemplate) is exactly one
 * fenced code block with the merged result followed by ≤5 plain risk lines.
 * A completion that ignores the contract degrades to "everything is the
 * resolution" rather than returning empty code.
 */
export function parseConflictAdvice(
  raw: string
): { recommendedResolution: string; explanation: string } {
  const text = (raw || "").trim();
  const fence = text.match(/```[^\n]*\n([\s\S]*?)```/);
  if (fence && fence[1] !== undefined) {
    const before = text.slice(0, fence.index ?? 0);
    const after = text.slice((fence.index ?? 0) + fence[0].length);
    const explanation = (before + "\n" + after)
      .split("\n")
      .map((line) => line.trim())
      .filter(Boolean)
      .join("\n");
    return {
      recommendedResolution: fence[1].replace(/\n+$/, ""),
      explanation,
    };
  }
  return {
    recommendedResolution: text,
    explanation: "",
  };
}

/**
 * Deterministic fallback synthesizer: merges unique non-empty lines from both
 * sides. It is intentionally explicit — callers must label it as a heuristic,
 * never present it as an AI result.
 */
export function heuristicConflictAdvice(params: {
  ours: string;
  theirs: string;
}): AiConflictAdvice {
  const oursLines = params.ours.split("\n");
  const theirsLines = params.theirs.split("\n");

  const combined: string[] = [];
  const seen = new Set<string>();

  for (const line of oursLines) {
    combined.push(line);
    if (line.trim()) seen.add(line.trim());
  }
  for (const line of theirsLines) {
    if (!seen.has(line.trim())) {
      combined.push(line);
    }
  }

  const synthesized = combined.join("\n");

  return {
    ...branchSummaries(params.ours, params.theirs),
    recommendedResolution: synthesized,
    explanation: "",
    source: "heuristic",
  };
}

/** Line-count branch summaries are local statistics, not model output. */
function branchSummaries(ours: string, theirs: string) {
  return {
    oursSummary: t("workspace.conflict.ai.ours.stat", { count: ours.split("\n").length }),
    theirsSummary: t("workspace.conflict.ai.theirs.stat", { count: theirs.split("\n").length }),
  };
}

/** Local line-count stats shown beside both sides of a conflict suggestion. */
export function conflictBranchSummaries(ours: string, theirs: string) {
  return branchSummaries(ours, theirs);
}

/**
 * Requests one backend AI suggestion. Failures are surfaced to the caller so
 * the UI can show an explicit error / heuristic choice instead of silently
 * presenting deterministic text as an AI result.
 */
export async function requestConflictAdvice(params: {
  ours: string;
  theirs: string;
  base?: string;
  filePath?: string;
}): Promise<AiConflictAdvice> {
  const ours = params.ours ?? "";
  const theirs = params.theirs ?? "";
  const base = params.base ?? "";
  if (ours.trim() === "" || theirs.trim() === "") {
    throw new Error(t("workspace.conflict.ai.empty.block"));
  }

  const raw = await conflictAdviceRpc(ours, theirs, base);
  const parsed = parseConflictAdvice(raw);
  if (parsed.recommendedResolution.trim() === "") {
    throw new Error(t("workspace.conflict.ai.empty.result"));
  }
  return {
    ...branchSummaries(ours, theirs),
    recommendedResolution: parsed.recommendedResolution,
    explanation: parsed.explanation,
    source: "ai",
  };
}
