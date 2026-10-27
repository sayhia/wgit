import { ref } from "vue";
import { gitService } from "../services/gitService";
import type { RepoCachePrewarmResult } from "../types";

export interface WorkspaceSessionState {
  lastRepoPath: string;
  activeView: string;
  selectedFilePath?: string;
  lastUpdated: number;
}

const SESSION_STORAGE_KEY = "wgit:session:state";

export function useSessionRestore() {
  const isPrewarming = ref(false);
  const lastPrewarmResult = ref<RepoCachePrewarmResult | null>(null);

  function loadSessionState(): WorkspaceSessionState | null {
    try {
      if (typeof localStorage === "undefined") return null;
      const raw = localStorage.getItem(SESSION_STORAGE_KEY);
      if (!raw) return null;
      return JSON.parse(raw) as WorkspaceSessionState;
    } catch {
      return null;
    }
  }

  function saveSessionState(state: Partial<WorkspaceSessionState>) {
    try {
      if (typeof localStorage === "undefined") return;
      const current = loadSessionState() || {
        lastRepoPath: "",
        activeView: "workspace",
        lastUpdated: Date.now(),
      };
      const updated: WorkspaceSessionState = {
        ...current,
        ...state,
        lastUpdated: Date.now(),
      };
      localStorage.setItem(SESSION_STORAGE_KEY, JSON.stringify(updated));
    } catch (err) {
      console.warn("Failed to persist session state:", err);
    }
  }

  async function prewarmRepo(repoPath: string): Promise<RepoCachePrewarmResult | null> {
    if (!repoPath) return null;
    isPrewarming.value = true;
    try {
      const res = await gitService.prewarmRepoCache(repoPath);
      lastPrewarmResult.value = res;
      return res;
    } catch (err) {
      console.debug("Repo cache prewarming skipped or failed:", err);
      return null;
    } finally {
      isPrewarming.value = false;
    }
  }

  function clearSession() {
    try {
      if (typeof localStorage !== "undefined") {
        localStorage.removeItem(SESSION_STORAGE_KEY);
      }
    } catch {}
  }

  return {
    loadSessionState,
    saveSessionState,
    prewarmRepo,
    clearSession,
    isPrewarming,
    lastPrewarmResult,
  };
}
