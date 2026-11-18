import axios, { isAxiosError, type InternalAxiosRequestConfig } from "axios";
import {
  AUTH_EXPIRED_EVENT,
  AUTH_REFRESHED_EVENT,
  clearAuth,
  getToken,
  updateAuthToken,
} from "./authStorage";

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

// 开发走 vite 代理，生产同源。
const http = axios.create({
  baseURL: "/api",
  timeout: 15000,
});

// 滑动续期响应头（见 Go 端 middleware.go）。
const AUTH_TOKEN_HEADER = "x-auth-token";
const AUTH_EXPIRES_HEADER = "x-auth-expires-at";

// 自动带上登录令牌。
http.interceptors.request.use((config) => {
  const token = getToken();
  if (token) {
    config.headers.set("Authorization", `Bearer ${token}`);
  }
  return config;
});

const NETWORK_ERROR_CODE = "NETWORK_ERROR";

// 维护模式拦截时派发的事件，由 main.ts 监听后切到维护页。
export const MAINTENANCE_EVENT = "go-messager:maintenance";

// 这些错误码需要清登录态。密码错误的 401 不在此列。
const AUTH_ERROR_CODES = new Set([
  "UNAUTHORIZED",
  "INVALID_TOKEN",
  "TOKEN_EXPIRED",
  "ACCOUNT_DISABLED",
]);

function isAuthErrorCode(code: string | undefined): boolean {
  return code !== undefined && AUTH_ERROR_CODES.has(code);
}

// 取出请求实际携带的 Bearer token，用于判断登录态失效时是否该清理。
function requestToken(config: InternalAxiosRequestConfig | undefined): string | undefined {
  const raw = config?.headers?.get("Authorization");
  if (typeof raw !== "string") return undefined;
  const match = /^bearer\s+(.+)$/i.exec(raw.trim());
  return match?.[1];
}

// 统一转换成后端错误格式；成功响应顺带处理滑动续期下发的 token。
http.interceptors.response.use(
  (response) => {
    // 服务端在剩余有效期不足时换了新 token，这里直接落库。
    const token = response.headers[AUTH_TOKEN_HEADER];
    const expiresAt = response.headers[AUTH_EXPIRES_HEADER];
    if (typeof token === "string" && token && typeof expiresAt === "string" && expiresAt) {
      updateAuthToken(token, expiresAt);
      window.dispatchEvent(new Event(AUTH_REFRESHED_EVENT));
    }
    return response;
  },
  (error: unknown) => {
    if (isAxiosError<ApiResponse<unknown>>(error)) {
      const payload = error.response?.data;
      const currentToken = getToken();

      // 登录态失效：清本地状态并通知应用。
      if (
        (error.response?.status === 401 || error.response?.status === 403) &&
        isAuthErrorCode(payload?.error?.code) &&
        currentToken
      ) {
        // 多标签/续期竞态：只有失败请求用的就是当前 token 才清态，
        // 否则可能把其他标签刚续上的新 token 一起清掉。
        const failedToken = requestToken(error.config);
        if (failedToken === undefined || failedToken === currentToken) {
          clearAuth();
          window.dispatchEvent(new Event(AUTH_EXPIRED_EVENT));
        }
      }

      // 维护模式：通知应用切到维护页，不清登录态，恢复后可继续使用。
      if (error.response?.status === 503 && payload?.error?.code === "MAINTENANCE") {
        window.dispatchEvent(new Event(MAINTENANCE_EVENT));
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
