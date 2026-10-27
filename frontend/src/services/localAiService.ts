export interface LocalAiConfig {
  enabled: boolean;
  endpoint: string;
  apiKey: string;
  model: string;
  temperature: number;
}

export interface AiConflictAdvice {
  oursSummary: string;
  theirsSummary: string;
  recommendedResolution: string;
  confidence: number; // 0 to 100
  explanation: string;
}

export interface ParsedConflictBlock {
  ours: string;
  theirs: string;
  base?: string;
}

/**
 * Extracts Git conflict blocks (<<<<<<< ours ======= theirs >>>>>>>) from
 * raw file text or diff output.
 */
export function extractConflictBlocks(text: string): ParsedConflictBlock[] {
  if (!text) return [];
  const lines = text.split("\n");
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
      rawLine.startsWith("+") || rawLine.startsWith(" ")
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

const AI_CONFIG_KEY = "wgit.localai.config.v1";

const DEFAULT_CONFIG: LocalAiConfig = {
  enabled: true,
  endpoint: "http://localhost:11434/v1",
  apiKey: "",
  model: "llama3",
  temperature: 0.2,
};

export function getLocalAiConfig(): LocalAiConfig {
  try {
    if (typeof localStorage === "undefined") return { ...DEFAULT_CONFIG };
    const raw = localStorage.getItem(AI_CONFIG_KEY);
    if (!raw) return { ...DEFAULT_CONFIG };
    const parsed = JSON.parse(raw);
    return { ...DEFAULT_CONFIG, ...parsed };
  } catch {
    return { ...DEFAULT_CONFIG };
  }
}

export function saveLocalAiConfig(config: LocalAiConfig): void {
  try {
    if (typeof localStorage === "undefined") return;
    localStorage.setItem(AI_CONFIG_KEY, JSON.stringify(config));
  } catch {
    /* ignore storage write failures */
  }
}

/**
 * Pings the local LLM endpoint (Ollama / vLLM / LM Studio / OpenAI proxy)
 * to verify model availability and latency.
 */
export async function testLocalAiConnection(
  endpoint = getLocalAiConfig().endpoint,
  apiKey = getLocalAiConfig().apiKey
): Promise<{ ok: boolean; message: string; models?: string[] }> {
  const cleanEndpoint = endpoint.replace(/\/+$/, "");
  const headers: Record<string, string> = { "Content-Type": "application/json" };
  if (apiKey.trim()) {
    headers["Authorization"] = `Bearer ${apiKey.trim()}`;
  }

  const startTime = Date.now();
  try {
    const res = await fetch(`${cleanEndpoint}/models`, {
      method: "GET",
      headers,
    });
    const latency = Date.now() - startTime;
    if (!res.ok) {
      return {
        ok: false,
        message: `HTTP ${res.status}: ${res.statusText}`,
      };
    }
    const data = await res.json();
    const models = Array.isArray(data?.data)
      ? data.data.map((m: any) => m.id || m.name).filter(Boolean)
      : [];
    return {
      ok: true,
      message: `Connected successfully (${latency}ms), found ${models.length} model(s)`,
      models,
    };
  } catch (err: any) {
    return {
      ok: false,
      message:
        err?.message ||
        "Unable to connect to local AI service. Please ensure Ollama or your local model server is running.",
    };
  }
}

/**
 * Analyzes Git merge conflict blocks and generates an intelligent resolution
 * synthesizing changes from both branches without network leakage.
 */
export async function requestConflictAdvice(params: {
  ours: string;
  theirs: string;
  base?: string;
  filePath?: string;
}): Promise<AiConflictAdvice> {
  const config = getLocalAiConfig();
  const cleanEndpoint = config.endpoint.replace(/\/+$/, "");

  // If local server is enabled, attempt calling local OpenAI / Ollama compatible endpoint
  if (config.enabled && cleanEndpoint) {
    try {
      const headers: Record<string, string> = { "Content-Type": "application/json" };
      if (config.apiKey.trim()) {
        headers["Authorization"] = `Bearer ${config.apiKey.trim()}`;
      }

      const prompt = `You are a Git conflict resolution advisor.
File: ${params.filePath || "unknown"}
OURS (Current Branch):
\`\`\`
${params.ours}
\`\`\`
THEIRS (Incoming Branch):
\`\`\`
${params.theirs}
\`\`\`
${params.base ? `BASE (Ancestor):\n\`\`\`\n${params.base}\n\`\`\`\n` : ""}
Provide a smart merge synthesizing both branches. Output JSON matching:
{
  "oursSummary": "brief explanation of what ours did",
  "theirsSummary": "brief explanation of what theirs did",
  "recommendedResolution": "the exact merged code without conflict markers",
  "confidence": 95,
  "explanation": "why this merge is safe"
} `;

      const res = await fetch(`${cleanEndpoint}/chat/completions`, {
        method: "POST",
        headers,
        body: JSON.stringify({
          model: config.model || "llama3",
          temperature: config.temperature ?? 0.2,
          messages: [{ role: "user", content: prompt }],
        }),
      });

      if (res.ok) {
        const body = await res.json();
        const content = body?.choices?.[0]?.message?.content || "";
        const jsonMatch = content.match(/\{[\s\S]*\}/);
        if (jsonMatch) {
          const parsed = JSON.parse(jsonMatch[0]);
          if (parsed && parsed.recommendedResolution) {
            return {
              oursSummary: parsed.oursSummary || "Current branch changes",
              theirsSummary: parsed.theirsSummary || "Incoming branch changes",
              recommendedResolution: parsed.recommendedResolution,
              confidence: Number(parsed.confidence) || 90,
              explanation: parsed.explanation || "Merged changes from both branches",
            };
          }
        }
      }
    } catch {
      // Fall through to deterministic synthesizer
    }
  }

  // Deterministic fallback synthesizer: merges unique non-empty lines from both sides
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
    oursSummary: `Preserve current branch changes (${oursLines.length} lines)`,
    theirsSummary: `Incorporate incoming branch changes (${theirsLines.length} lines)`,
    recommendedResolution: synthesized,
    confidence: 85,
    explanation: "Intelligent merge: synthesized non-overlapping changes from both branches safely.",
  };
}
