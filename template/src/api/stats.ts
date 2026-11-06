import http, { type ApiResponse } from "./http";

export type StatsUnit = "hour" | "day" | "week" | "month";

export interface MyStats {
  post_count: number;
  word_count: number;
  like: number;
  dislike: number;
  question: number;
  comment_count: number;
  received_comment_count: number;
}

export interface ReactionStats {
  like: number;
  dislike: number;
  question: number;
  total: number;
}

export interface TimePoint {
  label: string;
  count: number;
}

export interface TagUsage {
  name: string;
  count: number;
}

export interface HeatmapPoint {
  date: string;
  count: number;
}

export interface AdminStats {
  post_count: number;
  user_count: number;
  comment_count: number;
  login_count: number;
  registration_total: number;
  registration_recent: number;
  tag_count: number;
  reactions: ReactionStats;
  unit: StatsUnit;
  article_series: TimePoint[];
  tag_usage: TagUsage[];
  heatmap: HeatmapPoint[];
}

export async function fetchMyStats(): Promise<MyStats> {
  const { data } = await http.get<ApiResponse<MyStats>>("/me/stats");
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "获取个人统计失败" };
  }
  return data.data;
}

export async function fetchAdminStats(unit: StatsUnit): Promise<AdminStats> {
  const { data } = await http.get<ApiResponse<AdminStats>>("/admin/stats", {
    params: { unit },
  });
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "获取全站统计失败" };
  }
  return data.data;
}
