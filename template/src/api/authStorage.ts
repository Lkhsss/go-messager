// 登录态存储，供 axios 和 Pinia 共用。
import type { ReactionSummary } from "./reaction";

export type Role = "admin" | "user" | "none";

export interface AuthUser {
  name: string;
  role: Role;
  // 头像版本号（换头像后会变化），前端据此拼 /api/avatar 地址
  avatar_version?: string;
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

// token 体系使用带版本的新 key，旧版只存用户名的登录态不再兼容
const STORAGE_KEY = "go-messager:auth:v2";

// 清理旧版只存用户名的 key。
const LEGACY_STORAGE_KEY = "go-messager:user";

try {
  localStorage.removeItem(LEGACY_STORAGE_KEY);
} catch {
  // localStorage 不可用时忽略
}

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
    const avatarVersion = typeof storedUser?.avatar_version === "string" ? storedUser.avatar_version : undefined;
    const reactionTotals = storedUser?.reaction_totals;

    return {
      token: parsed.token,
      expiresAt,
      user: {
        name,
        role: isRole(roleCandidate) ? roleCandidate : "user",
        ...(avatarVersion ? { avatar_version: avatarVersion } : {}),
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
