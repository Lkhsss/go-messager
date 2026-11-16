import http, { type ApiResponse } from "./http";

// 可清理分类的 key，与后端 cleanupKey* 对应。
export type CleanupCategoryKey =
  | "deleted_posts"
  | "deleted_comments"
  | "orphan_comments"
  | "orphan_reactions"
  | "orphan_post_tags"
  | "orphan_daily_progress";

export interface CleanupCategory {
  key: CleanupCategoryKey;
  label: string;
  description: string;
  // 可清理条目数。
  count: number;
  // 估算占用字节数。
  bytes: number;
}

export interface CleanupDatabaseInfo {
  page_size: number;
  page_count: number;
  file_size_bytes: number;
  freelist_count: number;
  free_bytes: number;
}

export interface CleanupStats {
  categories: CleanupCategory[];
  total_count: number;
  total_bytes: number;
  database: CleanupDatabaseInfo;
}

export interface CleanupResult {
  key: CleanupCategoryKey;
  deleted: number;
  freed_bytes: number;
}

export interface VacuumResult {
  before_bytes: number;
  after_bytes: number;
  freed_bytes: number;
  free_bytes: number;
}

// 清理统计。
export async function fetchCleanupStats(): Promise<CleanupStats> {
  const { data } = await http.get<ApiResponse<CleanupStats>>("/admin/cleanup");
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "获取清理统计失败" };
  }
  return data.data;
}

// 清理单个分类。
export async function cleanupCategory(key: CleanupCategoryKey): Promise<CleanupResult> {
  const { data } = await http.delete<ApiResponse<CleanupResult>>(`/admin/cleanup/${key}`);
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "清理失败" };
  }
  return data.data;
}

// 一键清理所有分类。
export async function cleanupAll(): Promise<CleanupResult[]> {
  const { data } = await http.delete<ApiResponse<CleanupResult[]>>("/admin/cleanup");
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "清理失败" };
  }
  return data.data;
}

// 压缩数据库（VACUUM）。
export async function vacuumDatabase(): Promise<VacuumResult> {
  const { data } = await http.post<ApiResponse<VacuumResult>>("/admin/cleanup/vacuum");
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "压缩数据库失败" };
  }
  return data.data;
}
