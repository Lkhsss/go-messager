import http, { type ApiResponse } from "./http";
import type { PostItem } from "./post";

export type LeaderboardPeriod = "day" | "week" | "month" | "year";

export interface LeaderboardEntry {
  post: PostItem;
  count: number;
}

export interface LeaderboardData {
  period: LeaderboardPeriod;
  since: string;
  like: LeaderboardEntry[];
  dislike: LeaderboardEntry[];
  question: LeaderboardEntry[];
}

export async function fetchLeaderboard(period: LeaderboardPeriod): Promise<LeaderboardData> {
  const { data } = await http.get<ApiResponse<LeaderboardData>>("/leaderboard", { params: { period } });
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "获取排行榜失败" };
  }
  return data.data;
}
