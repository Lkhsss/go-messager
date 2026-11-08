import "highlight.js/styles/github-dark.css";
import MarkdownIt from "markdown-it";
import DOMPurify from "dompurify";
import { full as fullEmoji } from "markdown-it-emoji";
import hljs from "highlight.js/lib/core";

import bash from "highlight.js/lib/languages/bash";
import cpp from "highlight.js/lib/languages/cpp";
import css from "highlight.js/lib/languages/css";
import go from "highlight.js/lib/languages/go";
import java from "highlight.js/lib/languages/java";
import javascript from "highlight.js/lib/languages/javascript";
import json from "highlight.js/lib/languages/json";
import markdown from "highlight.js/lib/languages/markdown";
import python from "highlight.js/lib/languages/python";
import rust from "highlight.js/lib/languages/rust";
import sql from "highlight.js/lib/languages/sql";
import typescript from "highlight.js/lib/languages/typescript";
import xml from "highlight.js/lib/languages/xml";
import yaml from "highlight.js/lib/languages/yaml";

// 只注册常用语言。
hljs.registerLanguage("bash", bash);
hljs.registerLanguage("cpp", cpp);
hljs.registerLanguage("css", css);
hljs.registerLanguage("go", go);
hljs.registerLanguage("java", java);
hljs.registerLanguage("javascript", javascript);
hljs.registerLanguage("json", json);
hljs.registerLanguage("markdown", markdown);
hljs.registerLanguage("python", python);
hljs.registerLanguage("rust", rust);
hljs.registerLanguage("sql", sql);
hljs.registerLanguage("typescript", typescript);
hljs.registerLanguage("xml", xml);
hljs.registerLanguage("yaml", yaml);

// 围栏语言别名。
const LANG_ALIASES: Record<string, string> = {
  js: "javascript",
  jsx: "javascript",
  ts: "typescript",
  tsx: "typescript",
  sh: "bash",
  shell: "bash",
  zsh: "bash",
  py: "python",
  yml: "yaml",
  html: "xml",
  vue: "xml",
  golang: "go",
  "c++": "cpp",
  csharp: "cpp",
};

// 超长代码跳过语法高亮。
const MAX_HIGHLIGHT_CHARS = 20_000;

const md = new MarkdownIt({
  // 禁用原始 HTML。
  html: false,
  linkify: true,
  // 单个换行渲染为 <br>。
  breaks: true,
});

// emoji 短代码。
md.use(fullEmoji);

// 链接新窗口打开，防止 opener 漏洞。
md.renderer.rules.link_open = (tokens, idx, options, env, self) => {
  const token = tokens[idx];
  if (!token) return "";
  token.attrSet("target", "_blank");
  token.attrSet("rel", "noopener noreferrer");
  return self.renderToken(tokens, idx, options);
};

md.renderer.rules.image = (tokens, idx, options, env, self) => {
  const token = tokens[idx];
  if (!token) return "";
  token.attrSet("loading", "lazy");
  return self.renderToken(tokens, idx, options);
};

// 按行拆分高亮 HTML，保持 span 配对。
function splitHighlightedLines(html: string): string[] {
  const lines: string[] = [];
  let current = "";
  const openTags: string[] = [];
  const tokens = html.match(/<[^>]+>|[^<]+/g) ?? [];

  for (const token of tokens) {
    if (token.startsWith("</")) {
      current += token;
      openTags.pop();
      continue;
    }
    if (token.startsWith("<")) {
      current += token;
      openTags.push(token);
      continue;
    }

    const parts = token.split("\n");
    for (let i = 0; i < parts.length; i++) {
      if (i > 0) {
        // 换行时闭合并重新打开 span。
        current += openTags
          .slice()
          .reverse()
          .map((tag) => `</${tagName(tag)}>`)
          .join("");
        lines.push(current);
        current = openTags.join("");
      }
      current += parts[i] ?? "";
    }
  }

  lines.push(current);
  return lines;
}

function tagName(openTag: string): string {
  const match = /^<([a-zA-Z0-9-]+)/.exec(openTag);
  return match?.[1] ?? "span";
}

// 返回逐行 HTML，未高亮时退回纯文本。
function codeToLines(code: string, rawLang: string): string[] {
  const lang = LANG_ALIASES[rawLang] ?? rawLang;

  if (!lang || code.length > MAX_HIGHLIGHT_CHARS || !hljs.getLanguage(lang)) {
    return md.utils.escapeHtml(code).split("\n");
  }

  const html = hljs.highlight(code, { language: lang, ignoreIllegals: true }).value;
  const lines = splitHighlightedLines(html);

  // 去掉末尾空行。
  if (lines.length > 1 && lines[lines.length - 1] === "") {
    lines.pop();
  }

  return lines;
}

// 代码块使用 daisyUI mockup-code。
function renderMockupCode(content: string, rawLang: string): string {
  const lang = /^[a-zA-Z0-9_+#.-]+$/.test(rawLang) ? rawLang : "";
  const code = content.replace(/\n$/, "");
  const lines = codeToLines(code, lang);
  const langClass = lang ? ` class="hljs language-${lang}"` : ` class="hljs"`;

  const badge = lang ? `<span class="code-lang">${md.utils.escapeHtml(lang)}</span>` : "";
  const rows = lines
    .map((line, index) => `<pre data-prefix="${index + 1}"><code${langClass}>${line || " "}</code></pre>`)
    .join("");

  return `<div class="mockup-code my-3">${badge}${rows}</div>\n`;
}

md.renderer.rules.fence = (tokens, idx) => {
  const token = tokens[idx];
  if (!token) return "";

  const rawLang = (token.info || "").trim().split(/\s+/)[0] ?? "";
  return renderMockupCode(token.content, rawLang);
};

// 缩进代码块同样使用 mockup-code。
md.renderer.rules.code_block = (tokens, idx) => {
  const token = tokens[idx];
  if (!token) return "";

  return renderMockupCode(token.content, "");
};

export function renderMarkdown(source: string): string {
  const html = md.render(source ?? "");
  // 兜底消毒。
  return DOMPurify.sanitize(html, {
    ADD_ATTR: ["target", "rel", "loading"],
    ALLOW_DATA_ATTR: true, // 保留 mockup-code 的 data-prefix
  });
}

