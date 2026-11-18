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

// #rgb / #rrggbb 转 rgba，供正文药丸的内联样式使用（DOMPurify 只保留标准 CSS 属性）。
export function hexToRgba(color: string, alpha: number): string {
  const raw = (color || DEFAULT_TAG_COLOR).replace("#", "");
  const hex = raw.length === 3 ? raw.split("").map((ch) => ch + ch).join("") : raw;
  if (!/^[0-9a-fA-F]{6}$/.test(hex)) {
    return `rgba(107, 114, 128, ${alpha})`;
  }

  const r = Number.parseInt(hex.slice(0, 2), 16);
  const g = Number.parseInt(hex.slice(2, 4), 16);
  const b = Number.parseInt(hex.slice(4, 6), 16);
  return `rgba(${r}, ${g}, ${b}, ${alpha})`;
}
