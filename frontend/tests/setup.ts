import "./domSetup";
import { plugin } from "bun";
import { parse, compileScript } from "@vue/compiler-sfc";
import { registerLocaleMessages } from "../src/i18n";
import enUS from "../src/i18n/messages/en-US";

// Register Bun SFC compiler plugin so `.vue` files can be imported and mounted directly
plugin({
  name: "vue-sfc-loader",
  setup(build) {
    build.onLoad({ filter: /\.vue$/ }, async ({ path }) => {
      const source = await Bun.file(path).text();
      const { descriptor } = parse(source, { filename: path });
      const id = "sfc-" + Math.random().toString(36).slice(2);
      if (descriptor.script || descriptor.scriptSetup) {
        const compiled = compileScript(descriptor, { id, inlineTemplate: true });
        return { contents: compiled.content, loader: "ts" };
      }
      return { contents: "export default {}", loader: "ts" };
    });
  },
});

// Pre-seed English locale catalog for synchronous unit testing safety
registerLocaleMessages("en-US", enUS as unknown as Record<string, string>);
