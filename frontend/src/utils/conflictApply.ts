import { gitService } from "../services/gitService";
import { parseConflictFileBlocks } from "../services/localAiService";
import type { ConflictBlockResolveResult } from "../types";

export interface ConflictSuggestionPayload {
  content: string;
  expectedOurs: string;
  expectedTheirs: string;
  expectedBase: string;
}

/**
 * Applies a suggestion by locating the matching block in the *current* file,
 * not a captured index. Batch suggestions can be applied after earlier blocks
 * have shifted, while still failing closed if the block changed.
 */
export async function applyConflictSuggestionByContent(
  repoPath: string,
  filePath: string,
  payload: ConflictSuggestionPayload
): Promise<ConflictBlockResolveResult | null> {
  const content = await gitService.getConflictFileContent(repoPath, filePath);
  const blocks = parseConflictFileBlocks(content);
  const index = blocks.findIndex(
    (block) =>
      block.ours === payload.expectedOurs &&
      block.theirs === payload.expectedTheirs &&
      (block.base ?? "") === payload.expectedBase
  );
  if (index < 0) return null;
  return gitService.resolveConflictBlockWithCustomContent(
    repoPath,
    filePath,
    index,
    payload.content,
    payload.expectedOurs,
    payload.expectedTheirs,
    payload.expectedBase
  );
}
