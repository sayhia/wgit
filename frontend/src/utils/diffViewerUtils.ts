import { t } from "../i18n";

export interface FileModeChangeInfo {
  oldMode: string;
  newMode: string;
  desc: string;
  text: string;
}

/**
 * Detect file mode changes from git diff headers:
 * - old mode <mode> and new mode <mode>
 * - new file mode <mode>
 * - deleted file mode <mode>
 */
export function parseFileModeChange(diffText: string): FileModeChangeInfo | null {
  if (!diffText) return null;
  const lines = diffText.split("\n");

  let oldMode: string | null = null;
  let newMode: string | null = null;

  for (const line of lines) {
    if (line.startsWith("@@")) break;
    const oldMatch = line.match(/^old mode (\d+)/);
    if (oldMatch) oldMode = oldMatch[1]!;
    const newMatch = line.match(/^new mode (\d+)/);
    if (newMatch) newMode = newMatch[1]!;
    const newFileMatch = line.match(/^new file mode (\d+)/);
    if (newFileMatch) {
      oldMode = "—";
      newMode = newFileMatch[1]!;
    }
    const delFileMatch = line.match(/^deleted file mode (\d+)/);
    if (delFileMatch) {
      oldMode = delFileMatch[1]!;
      newMode = "—";
    }
  }

  if (!oldMode && !newMode) return null;

  const from = oldMode || "—";
  const to = newMode || "—";
  const targetMode = to !== "—" ? to : from;

  let descKey = "";
  if (targetMode === "100755") descKey = "workspace.diff.file.mode.exec";
  else if (targetMode === "100644") descKey = "workspace.diff.file.mode.regular";
  else if (targetMode === "120000") descKey = "workspace.diff.file.mode.symlink";
  else if (targetMode === "160000") descKey = "workspace.diff.file.mode.submodule";

  const desc = descKey ? ` (${t(descKey)})` : "";
  return {
    oldMode: from,
    newMode: to,
    desc,
    text: t("workspace.diff.file.mode.changed", { from, to, desc }),
  };
}

/**
 * Detect whether diffText represents a binary file difference.
 */
export function isBinaryDiffText(diffText: string): boolean {
  if (!diffText) return false;
  return /Binary files .* differ/.test(diffText);
}

/**
 * Extract the hunk diff lines starting from startLineIdx up to the next hunk header.
 */
export function extractHunkText(diffLines: string[], startLineIdx: number): string {
  if (!diffLines || startLineIdx < 0 || startLineIdx >= diffLines.length) return "";
  const result: string[] = [];
  for (let i = startLineIdx; i < diffLines.length; i++) {
    const l = diffLines[i]!;
    if (i > startLineIdx && l.startsWith("@@")) break;
    result.push(l);
  }
  return result.join("\n");
}

/**
 * Clean prefix character (+ / -) from a selected diff line.
 */
export function cleanSelectedLineText(text: string): string {
  if (text.startsWith("+") || text.startsWith("-")) {
    return text.slice(1);
  }
  return text;
}
