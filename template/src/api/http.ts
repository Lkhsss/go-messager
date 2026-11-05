import axios, { isAxiosError } from "axios";
import { AUTH_EXPIRED_EVENT, clearAuth, getToken } from "./authStorage";

// 后端统一响应格式（见 Go 端 response.go）
export interface ApiError {
  code: string;
  message: string;
}

// 分页元信息（见 Go 端 response.go 的 Meta）
export interface PaginationMeta {
  page: number;
  per_page: number;
  total: number;
  total_pages: number;
}

export interface ApiResponse<T> {
  success: boolean;
  data?: T;
  error?: ApiError;
  meta?: PaginationMeta;
}

// 开发环境走 vite 代理，生产环境与后端同源
const http = axios.create({
  baseURL: "/api",
  timeout: 15000,
});

// 每个请求自动带上登录令牌（authStorage 是独立模块，不会和 store 形成循环依赖）
http.interceptors.request.use((config) => {
  const token = getToken();
  if (token) {
    config.headers.set("Authorization", `Bearer ${token}`);
  }
  return config;
});

const NETWORK_ERROR_CODE = "NETWORK_ERROR";

// 这些错误码代表登录态已不可用，需要清掉本地登录态重新登录。
// 登录接口密码错误也是 401，但 code 是 INVALID_CREDENTIALS，不能触发全局登出。
// FORBIDDEN（需要管理员权限）不会导致登出，所以不在这里。
const AUTH_ERROR_CODES = new Set([
  "UNAUTHORIZED",
  "INVALID_TOKEN",
  "TOKEN_EXPIRED",
  "ACCOUNT_DISABLED",
]);

function isAuthErrorCode(code: string | undefined): boolean {
  return code !== undefined && AUTH_ERROR_CODES.has(code);
}

// 把 axios 异常统一转换成后端错误格式，调用方只需处理 { code, message }
http.interceptors.response.use(
  (response) => response,
  (error: unknown) => {
    if (isAxiosError<ApiResponse<unknown>>(error)) {
      const payload = error.response?.data;

      // 登录态失效（token 过期/伪造，或账号被停用）：清本地登录态并通知应用跳回登录页
      if (
        (error.response?.status === 401 || error.response?.status === 403) &&
        isAuthErrorCode(payload?.error?.code) &&
        getToken()
      ) {
        clearAuth();
        window.dispatchEvent(new Event(AUTH_EXPIRED_EVENT));
      }

      if (payload?.error) {
        return Promise.reject(payload.error);
      }
      if (!error.response) {
        return Promise.reject<ApiError>({
          code: NETWORK_ERROR_CODE,
          message: "无法连接服务器，请检查后端服务是否已启动",
        });
      }
      return Promise.reject<ApiError>({
        code: `HTTP_${error.response.status}`,
        message: error.message,
      });
    }
    return Promise.reject<ApiError>({
      code: NETWORK_ERROR_CODE,
      message: error instanceof Error ? error.message : "未知错误",
    });
  },
);

export function isApiError(value: unknown): value is ApiError {
  return (
    typeof value === "object" &&
    value !== null &&
    typeof (value as ApiError).code === "string" &&
    typeof (value as ApiError).message === "string"
  );
}

export default http;
