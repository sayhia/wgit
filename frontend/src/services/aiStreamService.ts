import { Call, Events } from "@wailsio/runtime";
import { t } from "../i18n";

/** One streaming generation request; the backend picks the task by `kind`. */
export type AIStreamKind =
  | "commit"
  | "summary"
  | "conflict"
  | "explain"
  | "chat"
  | "review"
  | "release";

export interface AIChatTurn {
  role: "user" | "assistant";
  content: string;
}

export interface AIStreamInput {
  kind: AIStreamKind;
  repoPath?: string;
  filePath?: string;
  diff?: string;
  commitHash?: string;
  ours?: string;
  theirs?: string;
  base?: string;
  question?: string;
  scope?: "diff" | "repo";
  history?: AIChatTurn[];
  /** Attach the currently selected file to a repository-scope chat (opt-in). */
  includeFile?: boolean;
  from?: string;
  to?: string;
  maxCommits?: number;
}

export type AIStreamOutcome =
  | { kind: "done"; text: string }
  | { kind: "error"; message: string }
  | { kind: "cancelled" };

export interface AIStreamHandlers {
  onDelta?: (delta: string) => void;
  /** Provider thinking deltas and read-only tool-call notices. */
  onReasoning?: (delta: string) => void;
}

export interface AIStreamSession {
  requestId: string;
  completion: Promise<AIStreamOutcome>;
  cancel: () => Promise<boolean>;
  dispose: () => void;
}

const DELTA_EVENT = "ai:stream:delta";
const DONE_EVENT = "ai:stream:done";
const ERROR_EVENT = "ai:stream:error";

function newRequestId(): string {
  const cryptoApi = typeof crypto !== "undefined" ? crypto : undefined;
  if (cryptoApi && typeof cryptoApi.randomUUID === "function") {
    return cryptoApi.randomUUID();
  }
  return `ai-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 12)}`;
}

function streamPayload(event: unknown): Record<string, unknown> | null {
  const raw =
    event && typeof event === "object" && "data" in event
      ? (event as { data: unknown }).data
      : event;
  if (!raw) return null;
  if (typeof raw === "string") {
    try {
      return JSON.parse(raw) as Record<string, unknown>;
    } catch {
      return null;
    }
  }
  return typeof raw === "object" ? (raw as Record<string, unknown>) : null;
}

/**
 * Starts one backend stream. Listeners are attached before the Start RPC so
 * even an immediately-fast model cannot emit a delta before we are listening.
 */
export async function startAIStream(
  input: AIStreamInput,
  handlers: AIStreamHandlers = {}
): Promise<AIStreamSession> {
  const requestId = newRequestId();
  let settled = false;
  let resolveCompletion!: (outcome: AIStreamOutcome) => void;
  const completion = new Promise<AIStreamOutcome>((resolve) => {
    resolveCompletion = resolve;
  });

  const disposers: Array<() => void> = [];
  const cleanup = () => {
    while (disposers.length > 0) {
      const off = disposers.pop();
      try {
        off?.();
      } catch {
        /* listener already detached */
      }
    }
  };
  const finish = (outcome: AIStreamOutcome) => {
    if (settled) return;
    settled = true;
    cleanup();
    resolveCompletion(outcome);
  };

  disposers.push(
    Events.On(DELTA_EVENT, (event: unknown) => {
      const data = streamPayload(event);
      if (!data || data.requestId !== requestId) return;
      const delta = typeof data.delta === "string" ? data.delta : "";
      const reasoning = typeof data.reasoning === "string" ? data.reasoning : "";
      if (delta) handlers.onDelta?.(delta);
      if (reasoning) handlers.onReasoning?.(reasoning);
    })
  );
  disposers.push(
    Events.On(DONE_EVENT, (event: unknown) => {
      const data = streamPayload(event);
      if (!data || data.requestId !== requestId) return;
      finish({ kind: "done", text: typeof data.text === "string" ? data.text : "" });
    })
  );
  disposers.push(
    Events.On(ERROR_EVENT, (event: unknown) => {
      const data = streamPayload(event);
      if (!data || data.requestId !== requestId) return;
      if (data.cancelled === true) {
        finish({ kind: "cancelled" });
      } else {
        finish({
          kind: "error",
          message: typeof data.message === "string" ? data.message : t("ai.stream.failed"),
        });
      }
    })
  );

  try {
    await Call.ByName("main.AiService.StartAIStream", {
      requestId,
      kind: input.kind,
      repoPath: input.repoPath ?? "",
      filePath: input.filePath ?? "",
      diff: input.diff ?? "",
      commitHash: input.commitHash ?? "",
      ours: input.ours ?? "",
      theirs: input.theirs ?? "",
      base: input.base ?? "",
      question: input.question ?? "",
      scope: input.scope ?? "",
      history: input.history ?? [],
      includeFile: input.includeFile === true,
      from: input.from ?? "",
      to: input.to ?? "",
      maxCommits: input.maxCommits ?? 0,
    });
  } catch (err) {
    cleanup();
    throw err;
  }

  return {
    requestId,
    completion,
    async cancel(): Promise<boolean> {
      let active = false;
      try {
        active = Boolean(
          (await Call.ByName("main.AiService.CancelAIRequest", requestId)) as boolean
        );
      } catch {
        active = false;
      }
      if (active) {
        finish({ kind: "cancelled" });
      }
      return active;
    },
    dispose: cleanup,
  };
}
