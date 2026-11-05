import type { TagItem } from "@/api/tag";

// 未指定颜色时的默认「高级灰」
export const DEFAULT_TAG_COLOR = "#6b7280";

export function tagColor(tag: Pick<TagItem, "color">): string {
  return tag.color || DEFAULT_TAG_COLOR;
}

// 提供给 :style 绑定，配合 CSS 中的 var(--tag-color) 使用
export function tagColorStyle(tag: Pick<TagItem, "color">): Record<string, string> {
  return { "--tag-color": tagColor(tag) };
}

// 徽标（只读展示）的内联样式：文字用标签色，底色是同色的低透明度
export function tagBadgeStyle(tag: Pick<TagItem, "color">): Record<string, string> {
  const color = tagColor(tag);
  return {
    color,
    backgroundColor: `color-mix(in srgb, ${color} 14%, transparent)`,
    borderColor: `color-mix(in srgb, ${color} 35%, transparent)`,
  };
}
