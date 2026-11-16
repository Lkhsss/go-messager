import http, { type ApiResponse } from "./http";

// 站点访问模式。
export type AccessMode = "private" | "public";

// 维护模式状态。
export interface MaintenanceInfo {
  enabled: boolean;
  title: string;
  body: string;
}

// 系统状态。
export interface SystemStatus {
  // 没有用户时为 false，需要初始化。
  initialized: boolean;
  // private：所有页面必须登录；public：游客可浏览文章。
  mode: AccessMode;
  // 维护模式，旧版后端不返回。
  maintenance?: MaintenanceInfo;
  // 是否允许注册，旧版后端不返回时按允许处理。
  register_enabled?: boolean;
}

export async function getStatus(): Promise<SystemStatus> {
  const { data } = await http.get<ApiResponse<SystemStatus>>("/status");
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "获取系统状态失败" };
  }
  return data.data;
}

// 运行时长。
export interface UptimeInfo {
  uptime_seconds: number;
  // 后端启动时刻，可能缺失。
  started_at?: string;
}

export async function getUptime(): Promise<UptimeInfo> {
  const { data } = await http.get<ApiResponse<UptimeInfo>>("/uptime");
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "获取运行时长失败" };
  }
  return data.data;
}
