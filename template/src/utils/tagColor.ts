import type { TagItem } from "@/api/tag";

// 默认标签色。
export const DEFAULT_TAG_COLOR = "#6b7280";

export function tagColor(tag: Pick<TagItem, "color">): string {
  return tag.color || DEFAULT_TAG_COLOR;
}

export function tagColorStyle(tag: Pick<TagItem, "color">): Record<string, string> {
  return { "--tag-color": tagColor(tag) };
}

// 标签徽标样式。
export function tagBadgeStyle(tag: Pick<TagItem, "color">): Record<string, string> {
  const color = tagColor(tag);
  return {
    color,
    backgroundColor: `color-mix(in srgb, ${color} 14%, transparent)`,
    borderColor: `color-mix(in srgb, ${color} 35%, transparent)`,
  };
}
