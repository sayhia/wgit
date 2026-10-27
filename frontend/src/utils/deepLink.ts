/**
 * Deep Link / URL Scheme Protocol Handler for wgit.
 *
 * Supported Schemes:
 *   - wgit://open?repo=<path>&tab=<tab>&ref=<hash>&file=<path>&line=<line>
 *   - wgit://commit?repo=<path>&hash=<hash>
 *   - wgit://diff?repo=<path>&file=<path>&line=<line>
 *   - wgit://clone?url=<remote_url>
 *   - Web Query: ?wgit_open=wgit%3A%2F%2F... or ?repo=...&tab=...&ref=...
 */

export type ValidTab =
  | "workspace"
  | "history"
  | "branches"
  | "stashes"
  | "review"
  | "pr"
  | "tools"
  | "settings";

export const VALID_TABS: ReadonlySet<string> = new Set([
  "workspace",
  "history",
  "branches",
  "stashes",
  "review",
  "pr",
  "tools",
  "settings",
]);

export interface DeepLinkPayload {
  action: "open" | "commit" | "diff" | "clone";
  repo?: string;
  tab?: ValidTab;
  ref?: string;
  hash?: string;
  file?: string;
  line?: number;
  url?: string;
  rawUrl: string;
}

export interface GenerateDeepLinkOptions {
  action?: "open" | "commit" | "diff" | "clone";
  repo?: string;
  tab?: ValidTab | string;
  ref?: string;
  hash?: string;
  file?: string;
  line?: number;
  url?: string;
}

/**
 * Parses a deep link URL into a structured DeepLinkPayload.
 * Returns null if the URL cannot be recognized as a valid wgit deep link.
 */
export function parseDeepLink(rawInput: string): DeepLinkPayload | null {
  if (!rawInput || typeof rawInput !== "string") return null;
  const input = rawInput.trim();
  if (!input) return null;

  // 1. Unwrap web query wrapper (?wgit_open=...)
  if (input.includes("wgit_open=")) {
    try {
      const parsedUrl = new URL(input, "http://localhost");
      const wrapped = parsedUrl.searchParams.get("wgit_open");
      if (wrapped) {
        return parseDeepLink(decodeURIComponent(wrapped));
      }
    } catch {
      // Fallback regex
      const m = input.match(/[?&]wgit_open=([^&]+)/);
      if (m && m[1]) {
        return parseDeepLink(decodeURIComponent(m[1]));
      }
    }
  }

  // 2. HTTP/HTTPS direct query parameters (e.g. ?repo=/foo&tab=history)
  if (input.startsWith("http://") || input.startsWith("https://")) {
    try {
      const httpUrl = new URL(input);
      const repo = httpUrl.searchParams.get("repo") || httpUrl.searchParams.get("repository") || httpUrl.searchParams.get("path") || undefined;
      const tabRaw = httpUrl.searchParams.get("tab") || httpUrl.searchParams.get("view") || undefined;
      const ref = httpUrl.searchParams.get("ref") || httpUrl.searchParams.get("branch") || undefined;
      const hash = httpUrl.searchParams.get("hash") || httpUrl.searchParams.get("commit") || ref;
      const fileRaw = httpUrl.searchParams.get("file") || httpUrl.searchParams.get("filepath") || undefined;
      const lineStr = httpUrl.searchParams.get("line") || httpUrl.searchParams.get("l") || undefined;

      if (repo || tabRaw || hash || fileRaw) {
        let file = fileRaw;
        let line = lineStr ? parseInt(lineStr, 10) : undefined;
        if (file && file.includes(":") && !line) {
          const parts = file.split(":");
          file = parts[0];
          line = parseInt(parts[1] || "", 10) || undefined;
        }

        const tab = (tabRaw && VALID_TABS.has(tabRaw) ? (tabRaw as ValidTab) : undefined);
        return {
          action: "open",
          repo: repo ? decodeURIComponent(repo).trim() : undefined,
          tab: tab || (file ? "workspace" : hash ? "history" : undefined),
          ref: ref ? decodeURIComponent(ref).trim() : undefined,
          hash: hash ? decodeURIComponent(hash).trim() : undefined,
          file: file ? decodeURIComponent(file).trim() : undefined,
          line: Number.isFinite(line) ? line : undefined,
          rawUrl: input,
        };
      }
    } catch {
      /* ignore invalid http URLs */
    }
  }

  // 3. Native wgit:// scheme
  if (!input.startsWith("wgit://")) {
    return null;
  }

  // Synthesize standard URL for reliable parsing
  // Replace "wgit://" with "http://wgit.local/" to leverage standard URL parser safely
  const synthetic = "http://wgit.local/" + input.slice("wgit://".length);
  let parsed: URL;
  try {
    parsed = new URL(synthetic);
  } catch {
    return null;
  }

  // Action is either the first path segment (or host if formatted as wgit://open?...)
  let actionStr = "";
  const host = parsed.host.toLowerCase();
  const pathParts = parsed.pathname.split("/").filter(Boolean);

  if (host !== "wgit.local" && host !== "") {
    actionStr = host;
  } else if (pathParts.length > 0) {
    actionStr = pathParts[0]?.toLowerCase() || "";
  }

  if (!actionStr) {
    actionStr = "open";
  }

  let action: "open" | "commit" | "diff" | "clone" = "open";
  if (actionStr === "commit") action = "commit";
  else if (actionStr === "diff") action = "diff";
  else if (actionStr === "clone") action = "clone";
  else action = "open";

  const params = parsed.searchParams;
  const repoRaw = params.get("repo") || params.get("repository") || params.get("path") || undefined;
  const tabRaw = params.get("tab") || params.get("view") || undefined;
  const refRaw = params.get("ref") || params.get("branch") || undefined;
  const hashRaw = params.get("hash") || params.get("commit") || refRaw;
  const fileRaw = params.get("file") || params.get("filepath") || undefined;
  const lineRaw = params.get("line") || params.get("l") || undefined;
  const urlRaw = params.get("url") || params.get("remote") || params.get("cloneUrl") || undefined;

  let file = fileRaw ? decodeURIComponent(fileRaw).trim() : undefined;
  let line = lineRaw ? parseInt(lineRaw, 10) : undefined;
  if (file && file.includes(":") && !line) {
    const parts = file.split(":");
    file = parts[0];
    line = parseInt(parts[1] || "", 10) || undefined;
  }

  let tab = (tabRaw && VALID_TABS.has(tabRaw) ? (tabRaw as ValidTab) : undefined);
  if (!tab) {
    if (action === "commit") tab = "history";
    else if (action === "diff" || file) tab = "workspace";
  }

  return {
    action,
    repo: repoRaw ? decodeURIComponent(repoRaw).trim() : undefined,
    tab,
    ref: refRaw ? decodeURIComponent(refRaw).trim() : undefined,
    hash: hashRaw ? decodeURIComponent(hashRaw).trim() : undefined,
    file,
    line: Number.isFinite(line) ? line : undefined,
    url: urlRaw ? decodeURIComponent(urlRaw).trim() : undefined,
    rawUrl: input,
  };
}

/**
 * Builds a standardized wgit:// deep link.
 */
export function generateDeepLink(options: GenerateDeepLinkOptions): string {
  const action = options.action || (options.url ? "clone" : options.hash ? "commit" : "open");
  const params = new URLSearchParams();

  if (options.repo) {
    params.set("repo", options.repo);
  }

  if (options.tab && VALID_TABS.has(options.tab)) {
    params.set("tab", options.tab);
  }

  if (options.ref) {
    params.set("ref", options.ref);
  }

  if (options.hash && options.hash !== options.ref) {
    params.set("hash", options.hash);
  }

  if (options.file) {
    params.set("file", options.file);
  }

  if (Number.isFinite(options.line)) {
    params.set("line", String(options.line));
  }

  if (options.url) {
    params.set("url", options.url);
  }

  const query = params.toString();
  return query ? `wgit://${action}?${query}` : `wgit://${action}`;
}
