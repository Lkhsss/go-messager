import emojiDefinitions from "markdown-it-emoji/lib/data/full.mjs";

const EMOJI_SHORTCODE_RE = /:([a-z0-9_+-]+):/g;

// 只替换 emoji，不启用 Markdown。
export function renderEmojiText(source: string): string {
  if (!source || !source.includes(":")) return source;

  return source.replace(EMOJI_SHORTCODE_RE, (match, name: string) => {
    return emojiDefinitions[name.toLowerCase()] ?? match;
  });
}
