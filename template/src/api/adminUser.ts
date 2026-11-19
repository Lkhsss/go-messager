import http, { type ApiResponse, type PaginationMeta } from "./http";
import type { MyStats } from "./stats";

export type AdminUserRole = "admin" | "user" | "none";

// 对应 Go 端 AdminUserListItem。
export interface AdminUserItem {
  id: number;
  name: string;
  role: AdminUserRole;
  trusted: boolean;
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

// 用户列表。
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

export async function fetchAdminUserDetail(id: number): Promise<AdminUserDetail> {
  const { data } = await http.get<ApiResponse<AdminUserDetail>>(`/admin/users/${id}`);
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "获取用户详情失败" };
  }
  return data.data;
}

// 封禁/解封。
export async function setAdminUserDisabled(id: number, disabled: boolean): Promise<AdminUserItem> {
  const { data } = await http.patch<ApiResponse<AdminUserItem>>(`/admin/users/${id}/status`, {
    disabled,
  });
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "更新用户状态失败" };
  }
  return data.data;
}

// 标记/取消可信用户。
export async function setAdminUserTrusted(id: number, trusted: boolean): Promise<AdminUserItem> {
  const { data } = await http.patch<ApiResponse<AdminUserItem>>(`/admin/users/${id}/trusted`, {
    trusted,
  });
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "更新可信状态失败" };
  }
  return data.data;
}

// 修改密码。
export async function changeAdminUserPassword(id: number, password: string): Promise<void> {
  const { data } = await http.put<ApiResponse<{ id: number }>>(`/admin/users/${id}/password`, {
    password,
  });
  if (!data.success) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "修改密码失败" };
  }
}

// 软删除账号，文章和评论保留。
export async function deleteAdminUser(id: number): Promise<void> {
  const { data } = await http.delete<ApiResponse<{ id: number }>>(`/admin/users/${id}`);
  if (!data.success) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "删除用户失败" };
  }
}

export interface DeleteUntrustedUsersResult {
  // 删除的用户数。
  deleted: number;
  // 随之删除的文章数。
  posts: number;
  // 随之删除的评论与回复数。
  comments: number;
}

// 硬删除所有未标记可信的普通用户及其内容。
export async function deleteUntrustedUsers(): Promise<DeleteUntrustedUsersResult> {
  const { data } =
    await http.delete<ApiResponse<DeleteUntrustedUsersResult>>("/admin/users/untrusted");
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "清理未可信用户失败" };
  }
  return data.data;
}

export interface ClearAdminUserPostsResult {
  // 删除的文章数。
  deleted: number;
  // 随之删除的评论与回复数。
  comments: number;
}

// 清空指定用户的全部文章及其下的评论。
export async function clearAdminUserPosts(id: number): Promise<ClearAdminUserPostsResult> {
  const { data } = await http.delete<ApiResponse<ClearAdminUserPostsResult>>(
    `/admin/users/${id}/posts`,
  );
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "清空用户文章失败" };
  }
  return data.data;
}
