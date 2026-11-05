import http, { type ApiResponse } from "./http";
import type { AuthUser } from "./authStorage";

export interface Credentials {
  name: string;
  password: string;
}

// 注册请求：邮箱可选，仅用于 Gravatar 头像
export interface RegisterInput extends Credentials {
  email?: string;
}

// 与 Go 端 AuthResult 对齐
export interface AuthResult {
  token: string;
  expires_at: string;
  user: AuthUser;
}

// 登录 POST /api/authorize
// 参数放 JSON body，避免密码出现在 URL / 访问日志里
export async function authorize(credentials: Credentials): Promise<AuthResult> {
  const { data } = await http.post<ApiResponse<AuthResult>>("/authorize", credentials);
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "登录失败" };
  }
  return data.data;
}

// 注册 POST /api/register
// 库里没有用户时，第一个注册的账号是管理员；成功即登录
export async function register(input: RegisterInput): Promise<AuthResult> {
  const payload: RegisterInput = { name: input.name, password: input.password };
  const email = input.email?.trim();
  if (email) {
    payload.email = email;
  }

  const { data } = await http.post<ApiResponse<AuthResult>>("/register", payload);
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "注册失败" };
  }
  return data.data;
}

// 当前用户 GET /api/me（需要 token）
// 用于校验本地 token 是否仍然有效，并刷新角色（角色以服务端为准）
export async function fetchMe(): Promise<AuthUser> {
  const { data } = await http.get<ApiResponse<AuthUser>>("/me");
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "获取用户信息失败" };
  }
  return data.data;
}

// 修改当前用户资料 PATCH /api/me（目前只有 Gravatar 邮箱）
export async function updateMe(email: string): Promise<AuthUser> {
  const { data } = await http.patch<ApiResponse<AuthUser>>("/me", { email });
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "保存失败" };
  }
  return data.data;
}

