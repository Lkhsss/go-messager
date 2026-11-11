import http, { type ApiResponse } from "./http";

// 验证码题目，image 为 data:image/png;base64，答案只在服务端保存。
export interface CaptchaChallenge {
  id: string;
  image: string;
  // 有效期（秒）。
  expires_in: number;
}

// 人机验证通过后的一次性凭证。
export interface CaptchaTicket {
  ticket: string;
  expires_in: number;
}

export async function fetchCaptcha(): Promise<CaptchaChallenge> {
  const { data } = await http.get<ApiResponse<CaptchaChallenge>>("/captcha");
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "获取验证码失败" };
  }
  return data.data;
}

export async function verifyCaptcha(id: string, answer: string): Promise<CaptchaTicket> {
  const { data } = await http.post<ApiResponse<CaptchaTicket>>("/captcha/verify", { id, answer });
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "验证失败" };
  }
  return data.data;
}
