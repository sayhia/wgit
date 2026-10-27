import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const audioDiffViewerFile = fileURLToPath(
  new URL("../src/components/assets/AudioDiffViewer.vue", import.meta.url)
);
const model3dDiffViewerFile = fileURLToPath(
  new URL("../src/components/assets/Model3dDiffViewer.vue", import.meta.url)
);
const assetDiffModalFile = fileURLToPath(
  new URL("../src/components/assets/AssetDiffModal.vue", import.meta.url)
);

describe("Phase M1: Multi-Modal Asset Diff (Audio & 3D)", () => {
  describe("AudioDiffViewer", () => {
    const src = readFileSync(audioDiffViewerFile, "utf8");

    test("contains Web Audio API decoding and canvas waveforms", () => {
      expect(src).toContain("decodeAudioData");
      expect(src).toContain("drawWaveform");
      expect(src).toContain("drawOverlayWaveform");
      expect(src).toContain("extractPeaks");
    });

    test("supports A/B track switching and split/overlay mode toggle", () => {
      expect(src).toContain("switchTrack");
      expect(src).toContain("compareMode");
      expect(src).toContain("assets.audio.mode.split");
      expect(src).toContain("assets.audio.mode.overlay");
    });

    test("supports playback, seeking, volume and mute controls", () => {
      expect(src).toContain("togglePlay");
      expect(src).toContain("handleSeek");
      expect(src).toContain("toggleMute");
      expect(src).toContain("volume");
    });
  });

  describe("Model3dDiffViewer", () => {
    const src = readFileSync(model3dDiffViewerFile, "utf8");

    test("contains 3D canvas viewport with perspective rotation and zoom", () => {
      expect(src).toContain("render3D");
      expect(src).toContain("handleMouseDown");
      expect(src).toContain("handleMouseMove");
      expect(src).toContain("handleWheel");
      expect(src).toContain("rotX");
      expect(src).toContain("rotY");
      expect(src).toContain("zoom");
    });

    test("supports OBJ and GLB topology parsing and stats display", () => {
      expect(src).toContain("parseModelHeader");
      expect(src).toContain("vertexCount");
      expect(src).toContain("faceCount");
      expect(src).toContain("dimensions");
      expect(src).toContain("assets.model.vertices");
      expect(src).toContain("assets.model.faces");
      expect(src).toContain("assets.model.bounds");
    });

    test("supports wireframe toggling and view reset", () => {
      expect(src).toContain("isWireframe");
      expect(src).toContain("resetView");
      expect(src).toContain("assets.model.wireframe");
      expect(src).toContain("assets.model.reset");
    });
  });

  describe("AssetDiffModal Integration", () => {
    const src = readFileSync(assetDiffModalFile, "utf8");

    test("imports and conditionally renders AudioDiffViewer and Model3dDiffViewer", () => {
      expect(src).toContain('import AudioDiffViewer from "./AudioDiffViewer.vue"');
      expect(src).toContain('import Model3dDiffViewer from "./Model3dDiffViewer.vue"');
      expect(src).toContain("data.kind === 'audio'");
      expect(src).toContain("<AudioDiffViewer");
      expect(src).toContain("data.kind === 'model'");
      expect(src).toContain("<Model3dDiffViewer");
    });
  });

  describe("Multi-Modal i18n Completeness", () => {
    const requiredKeys = [
      "assets.audio.mode.split",
      "assets.audio.mode.overlay",
      "assets.model.wireframe",
      "assets.model.interaction.hint",
      "assets.model.reset",
      "assets.model.vertices",
      "assets.model.faces",
      "assets.model.bounds",
    ];

    test("all audio and 3d keys exist in zh-CN and en-US", () => {
      for (const key of requiredKeys) {
        expect((zhCN as Record<string, string>)[key]).toBeDefined();
        expect((enUS as Record<string, string>)[key]).toBeDefined();
      }
    });
  });
});
