import http, { type ApiResponse } from "./http";

// 经验变化，与 Go 端 XPGain 对应。
export interface XPGain {
  task: "login" | "article" | "comment" | "interaction";
  task_label: string;
  gained: number;
  exp: number;
  level: number;
  leveled_up: boolean;
}

export interface DailyTaskState {
  key: "login" | "article" | "comment" | "interaction";
  label: string;
  description: string;
  reward: number;
  progress: number;
  target: number;
  completed: boolean;
}

export interface ExperienceStatus {
  date: string;
  exp: number;
  level: number;
  level_start_exp: number;
  next_level_exp: number;
  level_progress_exp: number;
  level_need_exp: number;
  tasks: DailyTaskState[];
}

// 经验条与每日任务状态。
export async function fetchExperience(): Promise<ExperienceStatus> {
  const { data } = await http.get<ApiResponse<ExperienceStatus>>("/me/experience");
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "获取经验信息失败" };
  }
  return data.data;
}
