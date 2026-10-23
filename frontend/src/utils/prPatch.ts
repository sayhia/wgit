/**
 * Unified-diff parsing for the PR file viewer.
 *
 * Kept as a pure function (no Vue, no DOM) so the tricky parts — hunk headers,
 * line counters, "\ No newline at end of file" markers — are unit-testable.
 */

export type PrPatchLineKind = "add" | "del" | "context" | "meta";

export interface PrPatchLine {
  kind: PrPatchLineKind;
  text: string;
  oldLine: number | null;
  newLine: number | null;
}

export interface PrPatchModel {
  lines: PrPatchLine[];
  additions: number;
  deletions: number;
}

const HUNK_RE = /^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@/;

export function parseUnifiedPatch(patch: string): PrPatchModel {
  const lines: PrPatchLine[] = [];
  let additions = 0;
  let deletions = 0;
  if (!patch) return { lines, additions, deletions };

  let oldNo = 0;
  let newNo = 0;
  let inHunk = false;
  const rawLines = patch.split("\n");
  // A trailing newline produces one empty tail element; drop it so the viewer
  // does not render a phantom row.
  if (rawLines.length > 0 && rawLines[rawLines.length - 1] === "") rawLines.pop();

  for (const raw of rawLines) {
    const hunk = HUNK_RE.exec(raw);
    if (hunk) {
      oldNo = Number(hunk[1]);
      newNo = Number(hunk[2]);
      inHunk = true;
      lines.push({ kind: "meta", text: raw, oldLine: null, newLine: null });
      continue;
    }
    // File headers ("diff --git", "index", "---", "+++") live outside hunks.
    // Classifying "+++" as an addition is the classic off-by-one in every
    // hand-rolled diff viewer, so gate on the hunk state.
    if (!inHunk || raw.startsWith("diff --git")) {
      inHunk = false;
      lines.push({ kind: "meta", text: raw, oldLine: null, newLine: null });
      continue;
    }
    if (raw.startsWith("+")) {
      lines.push({ kind: "add", text: raw.slice(1), oldLine: null, newLine: newNo++ });
      additions++;
      continue;
    }
    if (raw.startsWith("-")) {
      lines.push({ kind: "del", text: raw.slice(1), oldLine: oldNo++, newLine: null });
      deletions++;
      continue;
    }
    if (raw.startsWith("\\")) {
      lines.push({ kind: "meta", text: raw, oldLine: null, newLine: null });
      continue;
    }
    if (raw.startsWith(" ")) {
      lines.push({ kind: "context", text: raw.slice(1), oldLine: oldNo++, newLine: newNo++ });
      continue;
    }
    // "diff --git", "index", "---", "+++" and anything else outside a hunk.
    lines.push({ kind: "meta", text: raw, oldLine: null, newLine: null });
  }

  return { lines, additions, deletions };
}
