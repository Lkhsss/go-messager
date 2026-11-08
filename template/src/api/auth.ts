import http, { type ApiResponse } from "./http";
import type { AuthUser } from "./authStorage";

export interface Credentials {
  name: string;
  password: string;
}

export interface AuthResult {
  token: string;
  expires_at: string;
  user: AuthUser;
}

// 登录。密码放 body，避免出现在 URL。
export async function authorize(credentials: Credentials): Promise<AuthResult> {
  const { data } = await http.post<ApiResponse<AuthResult>>("/authorize", credentials);
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "登录失败" };
  }
  return data.data;
}

// 注册，首个用户是管理员。
export async function register(credentials: Credentials): Promise<AuthResult> {
  const { data } = await http.post<ApiResponse<AuthResult>>("/register", credentials);
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "注册失败" };
  }
  return data.data;
}

// 当前用户。
export async function fetchMe(): Promise<AuthUser> {
  const { data } = await http.get<ApiResponse<AuthUser>>("/me");
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "获取用户信息失败" };
  }
  return data.data;
}

// 上传/替换头像。
export async function updateAvatar(avatar: string): Promise<AuthUser> {
  const { data } = await http.put<ApiResponse<AuthUser>>("/me/avatar", { avatar });
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "头像保存失败" };
  }
  return data.data;
}

// 清除头像。
export async function removeAvatar(): Promise<AuthUser> {
  const { data } = await http.delete<ApiResponse<AuthUser>>("/me/avatar");
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "头像移除失败" };
  }
  return data.data;
}

