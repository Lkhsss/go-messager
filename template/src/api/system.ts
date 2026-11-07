import http, { type ApiResponse } from "./http";

// 系统状态 GET /api/status（公开接口）
export interface SystemStatus {
  // false 表示数据库里还没有用户，前端应引导创建第一个管理员账号
  initialized: boolean;
}

export async function getStatus(): Promise<SystemStatus> {
  const { data } = await http.get<ApiResponse<SystemStatus>>("/status");
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "获取系统状态失败" };
  }
  return data.data;
}

// 运行时长 GET /api/uptime（公开接口）
export interface UptimeInfo {
  uptime_seconds: number;
  // 后端进程启动时刻；老版本后端可能不返回，前端会用 uptime_seconds 兜底
  started_at?: string;
}

export async function getUptime(): Promise<UptimeInfo> {
  const { data } = await http.get<ApiResponse<UptimeInfo>>("/uptime");
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "获取运行时长失败" };
  }
  return data.data;
}
