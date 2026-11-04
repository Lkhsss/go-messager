import axios, { isAxiosError } from "axios";

// 后端统一响应格式（见 Go 端 response.go）
export interface ApiError {
  code: string;
  message: string;
}

export interface ApiResponse<T> {
  success: boolean;
  data?: T;
  error?: ApiError;
}

// 开发环境走 vite 代理，生产环境与后端同源
const http = axios.create({
  baseURL: "/api",
  timeout: 15000,
});

const NETWORK_ERROR_CODE = "NETWORK_ERROR";

// 把 axios 异常统一转换成后端错误格式，调用方只需处理 { code, message }
http.interceptors.response.use(
  (response) => response,
  (error: unknown) => {
    if (isAxiosError<ApiResponse<unknown>>(error)) {
      const payload = error.response?.data;
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
