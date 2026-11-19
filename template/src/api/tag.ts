import http, { type ApiResponse } from "./http";

// 对应 Go 端 Tag，gorm 字段首字母大写。
export interface TagItem {
  ID: number;
  CreatedAt: string;
  UpdatedAt: string;
  DeletedAt: string | null;
  name: string;
  // #rrggbb，空值用默认色。
  color: string;
  // 是否已审核：未审核不进入首页筛选与文章下方标签区。
  reviewed: boolean;
  // 使用次数，后端维护。
  usage_count: number;
}

export interface TagQuery {
  q?: string;
  sort?: "name" | "hot";
  // 不传返回全部。
  limit?: number;
}

// 标签列表。
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

// 创建标签。
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

// 编辑标签。
export async function updateTag(id: number, name: string, color = ""): Promise<TagItem> {
  const { data } = await http.put<ApiResponse<TagItem>>(`/tag/${id}`, { name, color });
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "更新标签失败" };
  }
  return data.data;
}

// 删除标签，文章保留。
export async function deleteTag(id: number): Promise<void> {
  const { data } = await http.delete<ApiResponse<{ id: number }>>(`/tag/${id}`);
  if (!data.success) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "删除标签失败" };
  }
}

// 管理端标签列表，包含待审核标签。
export async function fetchAdminTags(): Promise<TagItem[]> {
  const { data } = await http.get<ApiResponse<TagItem[]>>("/admin/tags");
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "获取标签失败" };
  }
  return data.data;
}

// 审核 / 取消审核标签。
export async function reviewTag(id: number, reviewed: boolean): Promise<TagItem> {
  const { data } = await http.patch<ApiResponse<TagItem>>(`/admin/tags/${id}/review`, { reviewed });
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: reviewed ? "审核失败" : "取消审核失败" };
  }
  return data.data;
}
