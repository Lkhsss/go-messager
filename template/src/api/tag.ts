import http, { type ApiResponse } from "./http";

// 与 Go 端 Tag 对齐：gorm.Model 字段首字母大写，业务字段是 name / color / usage_count
export interface TagItem {
  ID: number;
  CreatedAt: string;
  UpdatedAt: string;
  DeletedAt: string | null;
  name: string;
  // #rrggbb；空字符串表示使用默认高级灰
  color: string;
  // 被多少篇文章使用（后端维护）
  usage_count: number;
}

export interface TagQuery {
  // 按名称模糊搜索
  q?: string;
  // name：按名称；hot：按使用次数倒序
  sort?: "name" | "hot";
  // 不传表示全部；折叠时传 HOT_LIMIT + 1 用于判断是否还有更多
  limit?: number;
}

// 标签列表 GET /api/tag?q=go&sort=hot&limit=13
export async function fetchTags({ q, sort, limit }: TagQuery = {}): Promise<TagItem[]> {
  const { data } = await http.get<ApiResponse<TagItem[]>>("/tag", {
    params: {
      ...(q ? { q } : {}),
      ...(sort ? { sort } : {}),
      ...(limit ? { limit } : {}),
    },
  });

  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "获取标签失败" };
  }
  return data.data;
}

// 创建标签 POST /api/tag（仅管理员，普通用户会收到 403 FORBIDDEN）
// color 可选：#RGB / #RRGGBB，留空表示默认灰
export async function createTag(name: string, color = ""): Promise<TagItem> {
  const payload: { name: string; color?: string } = { name };
  if (color) {
    payload.color = color;
  }

  const { data } = await http.post<ApiResponse<TagItem>>("/tag", payload);
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "创建标签失败" };
  }
  return data.data;
}

// 编辑标签 PUT /api/tag/:id（仅管理员）
// color 传空字符串表示恢复默认高级灰
export async function updateTag(id: number, name: string, color = ""): Promise<TagItem> {
  const { data } = await http.put<ApiResponse<TagItem>>(`/tag/${id}`, { name, color });
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "更新标签失败" };
  }
  return data.data;
}

// 删除标签 DELETE /api/tag/:id（仅管理员）
// 后端会同时清理文章与标签的关联，文章本身保留
export async function deleteTag(id: number): Promise<void> {
  const { data } = await http.delete<ApiResponse<{ id: number }>>(`/tag/${id}`);
  if (!data.success) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "删除标签失败" };
  }
}

