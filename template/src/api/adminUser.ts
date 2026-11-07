import http, { type ApiResponse, type PaginationMeta } from "./http";
import type { MyStats } from "./stats";

export type AdminUserRole = "admin" | "user" | "none";

// 与 Go 端 handler_admin_user.go 的 AdminUserListItem 对齐
export interface AdminUserItem {
  id: number;
  name: string;
  role: AdminUserRole;
  avatar_version?: string;
  created_at: string;
  last_login_at?: string | null;
  last_active_at?: string | null;
  login_count: number;
  post_count: number;
  comment_count: number;
  word_count: number;
  like: number;
  dislike: number;
  question: number;
}

export interface AdminUserDetail {
  user: AdminUserItem;
  // 与 /api/me/stats 完全相同的统计口径
  stats: MyStats;
}

export interface AdminUserQuery {
  page?: number;
  perPage?: number;
  q?: string;
}

export interface AdminUserListResult {
  users: AdminUserItem[];
  meta: PaginationMeta;
}

// 用户列表 GET /api/admin/users?page=1&per_page=20&q=name
export async function fetchAdminUsers({
  page = 1,
  perPage = 10,
  q = "",
}: AdminUserQuery = {}): Promise<AdminUserListResult> {
  const params: Record<string, string | number> = { page, per_page: perPage };
  const keyword = q.trim();
  if (keyword) {
    params.q = keyword;
  }

  const { data } = await http.get<ApiResponse<AdminUserItem[]>>("/admin/users", { params });
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "获取用户列表失败" };
  }

  const users = data.data;
  return {
    users,
    meta: data.meta ?? {
      page,
      per_page: perPage,
      total: users.length,
      total_pages: users.length > 0 ? 1 : 0,
    },
  };
}

// 用户详情 GET /api/admin/users/:id
export async function fetchAdminUserDetail(id: number): Promise<AdminUserDetail> {
  const { data } = await http.get<ApiResponse<AdminUserDetail>>(`/admin/users/${id}`);
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "获取用户详情失败" };
  }
  return data.data;
}

// 封禁/解封 PATCH /api/admin/users/:id/status
export async function setAdminUserDisabled(id: number, disabled: boolean): Promise<AdminUserItem> {
  const { data } = await http.patch<ApiResponse<AdminUserItem>>(`/admin/users/${id}/status`, {
    disabled,
  });
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "更新用户状态失败" };
  }
  return data.data;
}

// 修改密码 PUT /api/admin/users/:id/password
export async function changeAdminUserPassword(id: number, password: string): Promise<void> {
  const { data } = await http.put<ApiResponse<{ id: number }>>(`/admin/users/${id}/password`, {
    password,
  });
  if (!data.success) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "修改密码失败" };
  }
}

// 删除账号 DELETE /api/admin/users/:id
// 后端只软删除账号，文章和评论会保留。
export async function deleteAdminUser(id: number): Promise<void> {
  const { data } = await http.delete<ApiResponse<{ id: number }>>(`/admin/users/${id}`);
  if (!data.success) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "删除用户失败" };
  }
}
