// 写文章草稿：按用户隔离存在 localStorage，发布成功后清除。
import type { TagItem } from "@/api/tag";

const STORAGE_KEY_PREFIX = "go-messager:post-draft:v1:";

export interface PostDraft {
  title: string;
  subtitle: string;
  body: string;
  // 标签快照：即使标签之后被删除，恢复草稿时也能回显名字和颜色。
  tags: TagItem[];
  // 最近一次自动保存时间（毫秒时间戳）。
  savedAt: number;
}

// 草稿内容是否值得保留。
export function hasPostDraftContent(
  draft: Pick<PostDraft, "title" | "subtitle" | "body" | "tags">,
): boolean {
  return (
    draft.title.trim() !== "" ||
    draft.subtitle.trim() !== "" ||
    draft.body.trim() !== "" ||
    draft.tags.length > 0
  );
}

export function readPostDraft(userName: string): PostDraft | null {
  if (!userName) return null;

  try {
    const raw = localStorage.getItem(storageKey(userName));
    if (!raw) return null;

    const parsed = JSON.parse(raw) as Partial<PostDraft> | null;
    if (!parsed) return null;

    const draft: PostDraft = {
      title: typeof parsed.title === "string" ? parsed.title : "",
      subtitle: typeof parsed.subtitle === "string" ? parsed.subtitle : "",
      body: typeof parsed.body === "string" ? parsed.body : "",
      tags: normalizeTags(parsed.tags),
      savedAt: typeof parsed.savedAt === "number" ? parsed.savedAt : 0,
    };

    // 空草稿没有恢复价值，顺手清掉。
    if (!hasPostDraftContent(draft)) {
      localStorage.removeItem(storageKey(userName));
      return null;
    }
    return draft;
  } catch {
    // 解析失败或 localStorage 不可用时按没有草稿处理。
    return null;
  }
}

export function savePostDraft(
  userName: string,
  draft: Pick<PostDraft, "title" | "subtitle" | "body" | "tags">,
): boolean {
  if (!userName) return false;

  try {
    const payload: PostDraft = { ...draft, savedAt: Date.now() };
    localStorage.setItem(storageKey(userName), JSON.stringify(payload));
    return true;
  } catch {
    // 超配额或被禁用时静默失败，不打断写作。
    return false;
  }
}

export function clearPostDraft(userName: string): void {
  if (!userName) return;
  try {
    localStorage.removeItem(storageKey(userName));
  } catch {
    // 忽略
  }
}

function storageKey(userName: string): string {
  return `${STORAGE_KEY_PREFIX}${userName}`;
}

// 只保留展示和提交需要的字段，避免旧数据缺字段时崩溃。
function normalizeTags(value: unknown): TagItem[] {
  if (!Array.isArray(value)) return [];

  const tags: TagItem[] = [];
  for (const item of value) {
    if (typeof item !== "object" || item === null) continue;

    const candidate = item as Partial<TagItem>;
    if (typeof candidate.ID !== "number" || !Number.isFinite(candidate.ID)) continue;
    if (typeof candidate.name !== "string" || !candidate.name) continue;

    tags.push({
      ID: candidate.ID,
      CreatedAt: typeof candidate.CreatedAt === "string" ? candidate.CreatedAt : "",
      UpdatedAt: typeof candidate.UpdatedAt === "string" ? candidate.UpdatedAt : "",
      DeletedAt: typeof candidate.DeletedAt === "string" ? candidate.DeletedAt : null,
      name: candidate.name,
      color: typeof candidate.color === "string" ? candidate.color : "",
      reviewed: candidate.reviewed === true,
      usage_count: typeof candidate.usage_count === "number" ? candidate.usage_count : 0,
    });
  }
  return tags;
}
