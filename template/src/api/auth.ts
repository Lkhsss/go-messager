import http, { type ApiResponse } from "./http";

export interface Credentials {
  name: string;
  password: string;
}

export interface AuthResult {
  name: string;
}

// 登录 POST /api/authorize
// 参数以 query params 传递；成功 200，失败 400/401
export async function authorize(credentials: Credentials): Promise<AuthResult> {
  const { data } = await http.post<ApiResponse<AuthResult>>("/authorize", null, {
    params: credentials,
  });
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "登录失败" };
  }
  return data.data;
}

// 注册 POST /api/register
// 参数以 query params 传递；成功 201，失败 400/409
export async function register(credentials: Credentials): Promise<AuthResult> {
  const { data } = await http.post<ApiResponse<AuthResult>>("/register", null, {
    params: credentials,
  });
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "注册失败" };
  }
  return data.data;
}
