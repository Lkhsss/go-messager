import "highlight.js/styles/github-dark.css";
import MarkdownIt, { type MarkdownIt as MarkdownItInstance } from "markdown-it";
import DOMPurify from "dompurify";
import { full as fullEmoji } from "markdown-it-emoji";
import hljs from "highlight.js/lib/core";
import type { TagItem } from "@/api/tag";
import { DEFAULT_TAG_COLOR, hexToRgba, tagColor } from "@/utils/tagColor";

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

// 标签字符与长度，需与 Go 端 tags_auto.go 保持一致。
const TAG_CHAR = /[\p{L}\p{N}_-]/u;
// # 前一个字符若是单词或 URL 字符，则不视为标签（避免 a#b、http://x/#a）。
const TAG_BLOCKED_PREV = /[\p{L}\p{N}_#/&?=.:-]/u;
const MAX_TAG_RUNES = 30;

// 去掉末尾 -_，并要求至少含一个字母或数字。
function normalizeTagName(raw: string): string {
  const name = raw.replace(/[-_]+$/u, "");
  if (!name || Array.from(name).length > MAX_TAG_RUNES) return "";
  return /[\p{L}\p{N}]/u.test(name) ? name : "";
}

// 把 #标签 解析为自定义 token，渲染成行内药丸。
function tagPillPlugin(md: MarkdownItInstance) {
  md.inline.ruler.before("emphasis", "tag_pill", (state, silent) => {
    const start = state.pos;
    if (state.src.charCodeAt(start) !== 0x23) return false; // '#'
    if (start > 0) {
      const prev = String.fromCodePoint(state.src.codePointAt(start - 1) ?? 0);
      if (TAG_BLOCKED_PREV.test(prev)) return false;
    }

    const segments: { name: string; end: number }[] = [];
    let pos = start;

    while (pos < state.posMax && state.src.charCodeAt(pos) === 0x23) {
      let cursor = pos + 1;
      let count = 0;
      while (cursor < state.posMax && count < MAX_TAG_RUNES) {
        const code = state.src.codePointAt(cursor);
        if (code === undefined) break;
        const ch = String.fromCodePoint(code);
        if (!TAG_CHAR.test(ch)) break;
        cursor += ch.length;
        count++;
      }

      const name = normalizeTagName(state.src.slice(pos + 1, cursor));
      if (!name) break;

      segments.push({ name, end: cursor });
      pos = cursor;

      // 下一个字符不是 #，或 # 后不是标签字符，则链结束。
      if (state.src.charCodeAt(pos) !== 0x23) break;
      const next = state.src.codePointAt(pos + 1);
      if (next === undefined || !TAG_CHAR.test(String.fromCodePoint(next))) break;
    }

    if (segments.length === 0) return false;

    if (!silent) {
      for (const segment of segments) {
        const token = state.push("tag_pill", "", 0);
        token.content = segment.name;
        token.markup = "#";
      }
    }

    const last = segments[segments.length - 1];
    state.pos = last ? last.end : start;
    return true;
  });

  md.renderer.rules.tag_pill = (tokens, idx, _options, env) => {
    const name = tokens[idx]?.content ?? "";
    const colors = (env as { tagColors?: Record<string, string> } | undefined)?.tagColors;
    const color = colors?.[name] ?? DEFAULT_TAG_COLOR;
    // 传入了审核映射但没命中 → 待审核标签，悬浮提示；未传映射（评论）不加提示。
    const title = colors !== undefined && colors[name] === undefined ? ' title="标签待审核"' : "";

    return (
      `<span class="tag-pill"${title} style="color:${color};` +
      `background-color:${hexToRgba(color, 0.14)};border-color:${hexToRgba(color, 0.35)}">` +
      `#${md.utils.escapeHtml(name)}</span>`
    );
  };
}

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

// 正文 #标签 高亮。
md.use(tagPillPlugin);

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
    .map(
      (line, index) =>
        `<pre data-prefix="${index + 1}"><code${langClass}>${line || " "}</code></pre>`,
    )
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

export function renderMarkdown(source: string, tags?: Pick<TagItem, "name" | "color">[]): string {
  const env = tags
    ? { tagColors: Object.fromEntries(tags.map((tag) => [tag.name, tagColor(tag)])) }
    : {};
  const html = md.render(source ?? "", env);
  // 兜底消毒。
  return DOMPurify.sanitize(html, {
    // 正文药丸依赖内联样式与悬浮提示。
    ADD_ATTR: ["target", "rel", "loading", "style", "title"],
    ALLOW_DATA_ATTR: true, // 保留 mockup-code 的 data-prefix
  });
}

// 提取正文中的 #标签，顺序与正文一致且已去重。
// 直接复用 markdown-it 的 tag_pill token，保证与正文高亮完全一致。
export function extractBodyTags(source: string): string[] {
  if (!source) return [];

  const tokens = md.parse(source, {});
  const names: string[] = [];
  const seen = new Set<string>();

  for (const token of tokens) {
    if (token.type !== "inline" || !token.children) continue;
    for (const child of token.children) {
      if (child.type !== "tag_pill" || seen.has(child.content)) continue;
      seen.add(child.content);
      names.push(child.content);
    }
  }

  return names;
}
