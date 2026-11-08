import emojiDefinitions from "markdown-it-emoji/lib/data/full.mjs";

const EMOJI_SHORTCODE_RE = /:([a-z0-9_+-]+):/g;

// 标题、副标题、摘要只做 emoji 替换，不启用 Markdown，避免改变原有纯文本语义。
// 独立成模块：首页/列表页只需要这张短代码表，不必加载 markdown-it 和 highlight.js。
export function renderEmojiText(source: string): string {
  if (!source || !source.includes(":")) return source;

  return source.replace(EMOJI_SHORTCODE_RE, (match, name: string) => {
    return emojiDefinitions[name.toLowerCase()] ?? match;
  });
}
