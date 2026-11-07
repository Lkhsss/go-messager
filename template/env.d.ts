/// <reference types="vite/client" />

declare module "markdown-it-emoji" {
  import type MarkdownIt from "markdown-it";

  type EmojiPlugin = (md: MarkdownIt, options?: {
    defs?: Record<string, string>;
    shortcuts?: Record<string, string>;
    enabled?: string[];
  }) => void;

  export const full: EmojiPlugin;
  export const light: EmojiPlugin;
  export const bare: EmojiPlugin;
}

declare module "markdown-it-emoji/lib/data/full.mjs" {
  const emojis: Record<string, string>;
  export default emojis;
}
