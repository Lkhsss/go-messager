import http, { type ApiResponse } from "./http";
import type { MaintenanceInfo } from "./system";

// 保存维护模式配置的入参。
export interface MaintenanceInput {
  enabled: boolean;
  title: string;
  body: string;
}

// 读取维护模式配置（管理员）。
export async function fetchMaintenance(): Promise<MaintenanceInfo> {
  const { data } = await http.get<ApiResponse<MaintenanceInfo>>("/admin/maintenance");
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "获取维护模式配置失败" };
  }
  return data.data;
}

// 保存维护模式配置（管理员）。
export async function saveMaintenance(input: MaintenanceInput): Promise<MaintenanceInfo> {
  const { data } = await http.put<ApiResponse<MaintenanceInfo>>("/admin/maintenance", input);
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "保存维护模式配置失败" };
  }
  return data.data;
}
