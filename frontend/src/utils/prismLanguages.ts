import Prism from "prismjs";

// Common language components in dependency order
import "prismjs/components/prism-clike";
import "prismjs/components/prism-c";
import "prismjs/components/prism-cpp";
import "prismjs/components/prism-csharp";
import "prismjs/components/prism-java";
import "prismjs/components/prism-kotlin";
import "prismjs/components/prism-swift";
import "prismjs/components/prism-go";
import "prismjs/components/prism-python";
import "prismjs/components/prism-rust";
import "prismjs/components/prism-javascript";
import "prismjs/components/prism-typescript";
import "prismjs/components/prism-jsx";
import "prismjs/components/prism-tsx";
import "prismjs/components/prism-bash";
import "prismjs/components/prism-json";
import "prismjs/components/prism-yaml";
import "prismjs/components/prism-toml";
import "prismjs/components/prism-markdown";
import "prismjs/components/prism-markup";
import "prismjs/components/prism-css";
import "prismjs/components/prism-scss";
import "prismjs/components/prism-sql";
import "prismjs/components/prism-lua";
import "prismjs/components/prism-glsl";
import "prismjs/components/prism-hlsl";
import "prismjs/components/prism-protobuf";
import "prismjs/components/prism-graphql";
import "prismjs/components/prism-docker";
import "prismjs/components/prism-diff";
import "prismjs/components/prism-ini";
import "prismjs/components/prism-markup-templating";
import "prismjs/components/prism-php";
import "prismjs/components/prism-ruby";
import "prismjs/components/prism-dart";
import "prismjs/components/prism-zig";

export function getPrismLanguageId(languageOrExt?: string): string {
  if (!languageOrExt) return "plaintext";
  const lang = languageOrExt.toLowerCase().trim();

  // Mapping from display language or extension to Prism grammar name
  const map: Record<string, string> = {
    go: "go",
    typescript: "typescript",
    ts: "typescript",
    mts: "typescript",
    cts: "typescript",
    "typescript react": "tsx",
    tsx: "tsx",
    javascript: "javascript",
    js: "javascript",
    mjs: "javascript",
    cjs: "javascript",
    "javascript react": "jsx",
    jsx: "jsx",
    vue: "markup",
    svelte: "markup",
    html: "markup",
    htm: "markup",
    xml: "markup",
    svg: "markup",
    python: "python",
    py: "python",
    rust: "rust",
    rs: "rust",
    c: "c",
    h: "c",
    "c++": "cpp",
    cpp: "cpp",
    hpp: "cpp",
    cc: "cpp",
    cxx: "cpp",
    "c#": "csharp",
    cs: "csharp",
    csharp: "csharp",
    java: "java",
    kotlin: "kotlin",
    kt: "kotlin",
    kts: "kotlin",
    swift: "swift",
    php: "php",
    ruby: "ruby",
    rb: "ruby",
    dart: "dart",
    zig: "zig",
    shader: "hlsl",
    hlsl: "hlsl",
    glsl: "glsl",
    frag: "glsl",
    vert: "glsl",
    comp: "glsl",
    usf: "hlsl",
    ush: "hlsl",
    css: "css",
    scss: "scss",
    sass: "scss",
    less: "css",
    json: "json",
    json5: "json",
    yaml: "yaml",
    yml: "yaml",
    toml: "toml",
    markdown: "markdown",
    md: "markdown",
    mdx: "markdown",
    shell: "bash",
    sh: "bash",
    bash: "bash",
    zsh: "bash",
    powershell: "bash",
    ps1: "bash",
    bat: "bash",
    cmd: "bash",
    lua: "lua",
    sql: "sql",
    protobuf: "protobuf",
    proto: "protobuf",
    graphql: "graphql",
    gql: "graphql",
    dockerfile: "docker",
    docker: "docker",
    diff: "diff",
    patch: "diff",
    config: "ini",
    ini: "ini",
    git: "ini",
    properties: "ini",
  };

  return map[lang] || (Prism.languages[lang] ? lang : "plaintext");
}

export function highlightCode(code: string, languageOrExt?: string): string {
  if (!code) return "";
  const langId = getPrismLanguageId(languageOrExt);
  const grammar = Prism.languages[langId];

  if (!grammar) {
    return code
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;");
  }

  try {
    return Prism.highlight(code, grammar, langId);
  } catch (err) {
    // best-effort: fall back to the plain-text tokenizer.
    console.debug("[Prism] Highlighting error:", err);
    return code
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;");
  }
}

export default Prism;
