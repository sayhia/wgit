/**
 * Thin wrapper for the backend asset board (Go: git_asset_service.go,
 * AssetService — 方案 §3.7 / F7 最小闭环).
 *
 * The service merges the tracked-asset scan with DCC (LFS) lock state into one
 * board entry per asset and persists a minimal review state machine in
 * .wgit/assets.json. Four states only: tracked / locked / review / approved —
 * locked is forced by the backend while a DCC lock exists, so the UI treats
 * tracked and locked as system-owned and only lets the user flip
 * review <-> approved.
 *
 * Routing note: AssetService RPCs are addressed by their full name
 * (main.AssetService.*) via Call.ByName — gitService.callBackend hardcodes the
 * main.GitService. prefix, which would miss the AssetService surface entirely
 * (same rationale as the AiService routes in aiService.ts). The generated
 * bindings in bindings/wgit/assetservice.ts address calls by numeric Call.ByID
 * instead; this wrapper goes through names so the routing is readable and the
 * mirror types below stay the single type source for the UI.
 */
import { Call } from "@wailsio/runtime";

/** Four legal states of the on-disk asset state machine (Go: assetState* constants). */
export const ASSET_STATES = ["tracked", "locked", "review", "approved"] as const;

export type AssetState = (typeof ASSET_STATES)[number];

/** States the user may set directly; tracked/locked are system-judged. */
export const USER_SETTABLE_ASSET_STATES: readonly AssetState[] = ["review", "approved"];

/** One board row (mirror of Go AssetItem / bindings AssetItem). */
export interface AssetItem {
  path: string;
  state: string;
  owner: string;
  sizeBytes: number;
  budgetRatio: number;
  updatedAt: string;
}

/** One board generation (mirror of Go AssetBoard / bindings AssetBoard). */
export interface AssetBoard {
  items: AssetItem[] | null;
  /** Final-state counts over ALL assets (not truncated by limit). */
  summary: Record<string, number> | null;
  generatedAt: string;
  warnings: string[] | null;
}

/**
 * Generate the asset board: tracked-asset scan -> on-disk states -> DCC locks
 * force state=locked. A non-positive limit falls back to the backend default
 * (100); values above 500 are clamped server-side.
 */
export function getAssetBoard(repoPath: string, limit: number): Promise<AssetBoard | null> {
  return Call.ByName("main.AssetService.GetAssetBoard", repoPath, limit) as Promise<AssetBoard | null>;
}

/**
 * Persist one asset's review state into .wgit/assets.json. The backend rejects
 * illegal states, paths outside the current scan, and — by the forced-lock
 * rule — writes that collide with an active DCC lock.
 */
export function setAssetState(repoPath: string, path: string, state: AssetState | string): Promise<void> {
  return Call.ByName("main.AssetService.SetAssetState", repoPath, path, state) as Promise<void>;
}

/**
 * Read the persisted state machine (path -> state). Missing or corrupted files
 * come back as an empty map; the next setAssetState rebuilds the file.
 */
export function getAssetStates(repoPath: string): Promise<Record<string, string>> {
  return Call.ByName("main.AssetService.GetAssetStates", repoPath) as Promise<Record<string, string>>;
}
