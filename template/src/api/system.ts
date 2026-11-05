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
