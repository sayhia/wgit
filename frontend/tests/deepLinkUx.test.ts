import { describe, test, expect } from "bun:test";
import { readFileSync } from "fs";
import { resolve } from "path";
import {
  parseDeepLink,
  generateDeepLink,
  VALID_TABS,
  type DeepLinkPayload,
} from "../src/utils/deepLink";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

describe("S4: Deep Link Protocol & URL Scheme Suite", () => {
  describe("parseDeepLink URL Parsing", () => {
    test("returns null for empty or invalid inputs", () => {
      expect(parseDeepLink("")).toBeNull();
      expect(parseDeepLink("   ")).toBeNull();
      expect(parseDeepLink("not-a-url")).toBeNull();
      expect(parseDeepLink("ftp://random.site")).toBeNull();
    });

    test("parses standard wgit://open URL with repo, tab, and ref", () => {
      const url = "wgit://open?repo=%2FUsers%2Fstudio%2Fmy-project&tab=history&ref=a1b2c3d";
      const payload = parseDeepLink(url);
      expect(payload).not.toBeNull();
      expect(payload?.action).toBe("open");
      expect(payload?.repo).toBe("/Users/studio/my-project");
      expect(payload?.tab).toBe("history");
      expect(payload?.ref).toBe("a1b2c3d");
    });

    test("parses wgit://commit convenience format and infers history tab", () => {
      const url = "wgit://commit?repo=%2FUsers%2Fstudio%2Fmy-project&hash=feedbeef99";
      const payload = parseDeepLink(url);
      expect(payload).not.toBeNull();
      expect(payload?.action).toBe("commit");
      expect(payload?.repo).toBe("/Users/studio/my-project");
      expect(payload?.hash).toBe("feedbeef99");
      expect(payload?.tab).toBe("history");
    });

    test("parses wgit://diff convenience format with file and line", () => {
      const url = "wgit://diff?repo=%2FUsers%2Fstudio%2Fmy-project&file=src%2Fmain.ts&line=120";
      const payload = parseDeepLink(url);
      expect(payload).not.toBeNull();
      expect(payload?.action).toBe("diff");
      expect(payload?.repo).toBe("/Users/studio/my-project");
      expect(payload?.file).toBe("src/main.ts");
      expect(payload?.line).toBe(120);
      expect(payload?.tab).toBe("workspace");
    });

    test("parses file string containing colon line number (e.g. file:42)", () => {
      const url = "wgit://open?repo=%2Fproject&file=src%2Fapp.vue:42";
      const payload = parseDeepLink(url);
      expect(payload).not.toBeNull();
      expect(payload?.file).toBe("src/app.vue");
      expect(payload?.line).toBe(42);
      expect(payload?.tab).toBe("workspace");
    });

    test("parses wgit://clone format with remote URL", () => {
      const url = "wgit://clone?url=https%3A%2F%2Fgithub.com%2Fstudio%2Fengine.git";
      const payload = parseDeepLink(url);
      expect(payload).not.toBeNull();
      expect(payload?.action).toBe("clone");
      expect(payload?.url).toBe("https://github.com/studio/engine.git");
    });

    test("unwraps web query wrapper (?wgit_open=...)", () => {
      const inner = "wgit://open?repo=%2Fdata%2Frepo&tab=tools";
      const webUrl = `http://localhost:8080/?wgit_open=${encodeURIComponent(inner)}`;
      const payload = parseDeepLink(webUrl);
      expect(payload).not.toBeNull();
      expect(payload?.repo).toBe("/data/repo");
      expect(payload?.tab).toBe("tools");
    });

    test("parses HTTP query parameters in web/server mode", () => {
      const webUrl = "http://localhost:8080/?repo=%2Fvar%2Frepo&tab=branches";
      const payload = parseDeepLink(webUrl);
      expect(payload).not.toBeNull();
      expect(payload?.action).toBe("open");
      expect(payload?.repo).toBe("/var/repo");
      expect(payload?.tab).toBe("branches");
    });

    test("validates tab against VALID_TABS set", () => {
      const invalidTabUrl = "wgit://open?repo=%2Frepo&tab=hacked_tab";
      const payload = parseDeepLink(invalidTabUrl);
      expect(payload).not.toBeNull();
      expect(payload?.tab).toBeUndefined();

      for (const tab of VALID_TABS) {
        const p = parseDeepLink(`wgit://open?repo=%2Frepo&tab=${tab}`);
        expect(p?.tab).toBe(tab as any);
      }
    });
  });

  describe("generateDeepLink Builder", () => {
    test("generates formatted wgit://open URL", () => {
      const result = generateDeepLink({
        repo: "/Users/test/my-repo",
        tab: "history",
        ref: "commit123",
      });
      expect(result).toBe("wgit://open?repo=%2FUsers%2Ftest%2Fmy-repo&tab=history&ref=commit123");
    });

    test("generates wgit://commit URL when hash is provided without action", () => {
      const result = generateDeepLink({
        repo: "/my/repo",
        hash: "deadbeef",
      });
      expect(result).toContain("wgit://commit?");
      expect(result).toContain("hash=deadbeef");
    });

    test("generates wgit://clone URL when url is provided", () => {
      const result = generateDeepLink({
        url: "https://git.internal/repo.git",
      });
      expect(result).toBe("wgit://clone?url=https%3A%2F%2Fgit.internal%2Frepo.git");
    });
  });

  describe("App.vue and View Integration Contract", () => {
    const appVueContent = readFileSync(resolve(__dirname, "../src/App.vue"), "utf-8");
    const commitInspectorContent = readFileSync(
      resolve(__dirname, "../src/components/history/CommitInspector.vue"),
      "utf-8"
    );

    test("App.vue imports deepLink handler and listens for wgit:open-url and wgit:deep-link", () => {
      expect(appVueContent).toContain("parseDeepLink");
      expect(appVueContent).toContain("handleDeepLink");
      expect(appVueContent).toContain('addEventListener("wgit:open-url"');
      expect(appVueContent).toContain('addEventListener("wgit:deep-link"');
      expect(appVueContent).toContain('removeEventListener("wgit:open-url"');
      expect(appVueContent).toContain('removeEventListener("wgit:deep-link"');
    });

    test("App.vue queries backend for initial deep link on startup", () => {
      expect(appVueContent).toContain("getInitialDeepLink");
    });

    test("CommitInspector.vue supports copying deep link for the current commit", () => {
      expect(commitInspectorContent).toContain("generateDeepLink");
      expect(commitInspectorContent).toContain("copyDeepLink");
      expect(commitInspectorContent).toContain("history.inspector.deeplink");
    });
  });

  describe("i18n Completeness for S4", () => {
    test("zh-CN contains history.inspector.deeplink keys", () => {
      expect((zhCN as Record<string, string>)["history.inspector.deeplink"]).toBe("深层链接");
      expect((zhCN as Record<string, string>)["history.inspector.deeplink.hint"]).toBeDefined();
    });

    test("en-US contains history.inspector.deeplink keys", () => {
      expect((enUS as Record<string, string>)["history.inspector.deeplink"]).toBe("Deep Link");
      expect((enUS as Record<string, string>)["history.inspector.deeplink.hint"]).toBeDefined();
    });
  });
});
