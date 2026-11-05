// 登录态存储：独立模块，供 axios 拦截器和 Pinia store 共用，避免循环依赖。
import type { ReactionSummary } from "./reaction";

export type Role = "admin" | "user" | "none";

export interface AuthUser {
  name: string;
  role: Role;
  // 仅自己可见，用于生成 Gravatar 头像
  email?: string;
  // md5(email)，前端用它拼 gravatar.com 地址
  gravatar_hash?: string;
  // 该用户文章收到的反应总和
  reaction_totals?: ReactionSummary;
}

export interface StoredAuth {
  token: string;
  expiresAt: string;
  user: AuthUser;
}

// token 失效时派发的事件，由 main.ts 监听后跳转登录页
export const AUTH_EXPIRED_EVENT = "go-messager:auth-expired";

// 旧版本只存了用户名（go-messager:user），token 体系是新 key，老用户重新登录一次即可
const STORAGE_KEY = "go-messager:auth:v2";

function isRole(value: unknown): value is Role {
  return value === "admin" || value === "user" || value === "none";
}

export function readAuth(): StoredAuth | null {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return null;

    const parsed = JSON.parse(raw) as Partial<StoredAuth> | null;
    const storedUser = parsed?.user;
    const name = storedUser?.name;
    if (!parsed?.token || typeof name !== "string" || !name) {
      localStorage.removeItem(STORAGE_KEY);
      return null;
    }

    const expiresAt = typeof parsed.expiresAt === "string" ? parsed.expiresAt : "";
    // 本地就能判断过期时直接丢弃，避免带着废 token 发请求
    if (expiresAt && Date.parse(expiresAt) <= Date.now()) {
      localStorage.removeItem(STORAGE_KEY);
      return null;
    }

    const roleCandidate = storedUser?.role;
    const email = typeof storedUser?.email === "string" ? storedUser.email : undefined;
    const gravatarHash = typeof storedUser?.gravatar_hash === "string" ? storedUser.gravatar_hash : undefined;
    const reactionTotals = storedUser?.reaction_totals;

    return {
      token: parsed.token,
      expiresAt,
      user: {
        name,
        role: isRole(roleCandidate) ? roleCandidate : "user",
        ...(email ? { email } : {}),
        ...(gravatarHash ? { gravatar_hash: gravatarHash } : {}),
        ...(reactionTotals ? { reaction_totals: reactionTotals } : {}),
      },
    };
  } catch {
    return null;
  }
}

export function writeAuth(auth: StoredAuth): void {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(auth));
  } catch {
    // localStorage 不可用时忽略，仅保留内存中的登录态
  }
}

export function clearAuth(): void {
  try {
    localStorage.removeItem(STORAGE_KEY);
  } catch {
    // 忽略
  }
}

export function getToken(): string | null {
  return readAuth()?.token ?? null;
}
