import type { CommitItem } from "../types";

export interface ParsedHistoryQuery {
  author?: string;
  path?: string;
  grep?: string;
  since?: string;
  until?: string;
  hash?: string;
  text?: string;
}

/**
 * Parses user input search string into structured search criteria.
 * Example: `author:alice grep:"fix bug" since:2026-01-01 main.go`
 */
export function parseHistoryQuery(raw: string): ParsedHistoryQuery {
  const query: ParsedHistoryQuery = {};
  if (!raw || !raw.trim()) return query;

  // Regex to extract `key:value` or `key:"quoted value"`
  const tokenRegex = /(\b(?:author|committer|path|file|grep|msg|since|until|after|before|hash):(?:"([^"]+)"|'([^']+)'|(\S+)))/gi;
  let remaining = raw;
  let match: RegExpExecArray | null;

  while ((match = tokenRegex.exec(raw)) !== null) {
    const fullMatch = match[0]!;
    const colonIdx = fullMatch.indexOf(":");
    const key = fullMatch.slice(0, colonIdx).toLowerCase();
    const val = match[2] ?? match[3] ?? match[4] ?? "";

    switch (key) {
      case "author":
      case "committer":
        query.author = val.toLowerCase();
        break;
      case "path":
      case "file":
        query.path = val.toLowerCase();
        break;
      case "grep":
      case "msg":
        query.grep = val.toLowerCase();
        break;
      case "since":
      case "after":
        query.since = val;
        break;
      case "until":
      case "before":
        query.until = val;
        break;
      case "hash":
        query.hash = val.toLowerCase();
        break;
    }

    remaining = remaining.replace(fullMatch, " ");
  }

  const freeText = remaining.replace(/\s+/g, " ").trim().toLowerCase();
  if (freeText) {
    query.text = freeText;
  }

  return query;
}

/**
 * Evaluates whether a commit item matches the parsed search query.
 */
export function matchesHistoryQuery(commit: CommitItem, query: ParsedHistoryQuery): boolean {
  if (query.author) {
    const authorMatch = (commit.authorName && commit.authorName.toLowerCase().includes(query.author)) ||
      (commit.authorEmail && commit.authorEmail.toLowerCase().includes(query.author));
    if (!authorMatch) return false;
  }

  if (query.hash) {
    if (!commit.hash.toLowerCase().startsWith(query.hash)) return false;
  }

  if (query.grep) {
    const subjectMatch = commit.subject && commit.subject.toLowerCase().includes(query.grep);
    const bodyMatch = commit.body && commit.body.toLowerCase().includes(query.grep);
    if (!subjectMatch && !bodyMatch) return false;
  }

  if (query.since && commit.date) {
    if (commit.date < query.since) return false;
  }

  if (query.until && commit.date) {
    if (commit.date > query.until) return false;
  }

  if (query.text) {
    const inSubject = commit.subject && commit.subject.toLowerCase().includes(query.text);
    const inAuthor = commit.authorName && commit.authorName.toLowerCase().includes(query.text);
    const inHash = commit.hash && commit.hash.toLowerCase().startsWith(query.text);
    if (!inSubject && !inAuthor && !inHash) return false;
  }

  return true;
}

/**
 * Filters a list of commits using the query string.
 */
export function filterCommitsByQuery(commits: CommitItem[], rawQuery: string): CommitItem[] {
  if (!rawQuery || !rawQuery.trim()) return commits;
  const parsed = parseHistoryQuery(rawQuery);
  return commits.filter((c) => matchesHistoryQuery(c, parsed));
}
