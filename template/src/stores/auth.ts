import { computed, ref } from "vue";
import { defineStore } from "pinia";
import {
  authorize,
  fetchMe,
  register,
  removeAvatar as removeAvatarApi,
  updateAvatar as updateAvatarApi,
  type Credentials,
  type RegisterCredentials,
} from "@/api/auth";
import { clearAuth, readAuth, writeAuth, type AuthUser, type StoredAuth } from "@/api/authStorage";
import { isApiError } from "@/api/http";

// 这些错误码需要清登录态。
const AUTH_ERROR_CODES = ["UNAUTHORIZED", "INVALID_TOKEN", "TOKEN_EXPIRED", "ACCOUNT_DISABLED"];

export const useAuthStore = defineStore("auth", () => {
  const initial = readAuth();

  const token = ref<string | null>(initial?.token ?? null);
  const user = ref<AuthUser | null>(initial?.user ?? null);
  const expiresAt = ref(initial?.expiresAt ?? "");

  // 已校验的 token；初始为 null，刷新后先校验一次。
  let validatedToken: string | null = null;
  let inflight: Promise<boolean> | null = null;

  const isLoggedIn = computed(() => token.value !== null && user.value !== null);
  const isAdmin = computed(() => user.value?.role === "admin");

  function setAuth(next: StoredAuth) {
    token.value = next.token;
    user.value = next.user;
    expiresAt.value = next.expiresAt;
    validatedToken = next.token;
    writeAuth(next);
  }

  function applyStored(next: StoredAuth | null) {
    token.value = next?.token ?? null;
    user.value = next?.user ?? null;
    expiresAt.value = next?.expiresAt ?? "";
    // 退出、换号或滑动续期换发 token 后都需要重新校验。
    if (next === null || next.token !== validatedToken) {
      validatedToken = null;
    }
  }

  async function login(credentials: Credentials) {
    const result = await authorize(credentials);
    setAuth({ token: result.token, expiresAt: result.expires_at, user: result.user });
    return result;
  }

  async function signup(credentials: RegisterCredentials) {
    const result = await register(credentials);
    setAuth({ token: result.token, expiresAt: result.expires_at, user: result.user });
    return result;
  }

  // 只刷新用户资料。
  async function updateAvatar(dataUrl: string) {
    const nextUser = await updateAvatarApi(dataUrl);
    if (token.value) {
      setAuth({ token: token.value, expiresAt: expiresAt.value, user: nextUser });
    }
    return nextUser;
  }

  // 清除头像。
  async function removeAvatar() {
    const nextUser = await removeAvatarApi();
    if (token.value) {
      setAuth({ token: token.value, expiresAt: expiresAt.value, user: nextUser });
    }
    return nextUser;
  }

  // 同步经验/等级，药丸和经验条即时更新。
  function applyExperience(exp: number, level: number) {
    if (!user.value || !token.value) return;
    user.value = { ...user.value, exp, level };
    writeAuth({ token: token.value, expiresAt: expiresAt.value, user: user.value });
  }

  function logout() {
    applyStored(null);
    clearAuth();
  }

  // 从 localStorage 同步登录态。
  function sync() {
    applyStored(readAuth());
  }

  // 校验 token 并从服务端刷新角色。
  async function ensureSession(): Promise<boolean> {
    sync();

    if (!token.value) {
      validatedToken = null;
      return false;
    }
    if (validatedToken === token.value) {
      return true;
    }
    if (inflight) {
      return inflight;
    }

    const requestedToken = token.value;
    inflight = fetchMe()
      .then((me) => {
        // 期间可能已退出或换号。
        if (token.value !== requestedToken) return false;
        setAuth({ token: requestedToken, expiresAt: expiresAt.value, user: me });
        return true;
      })
      .catch((err) => {
        // 仅服务端否认 token 时登出。
        if (isApiError(err) && AUTH_ERROR_CODES.includes(err.code)) {
          logout();
        }
        return false;
      })
      .finally(() => {
        inflight = null;
      });

    return inflight;
  }

  return {
    token,
    user,
    expiresAt,
    isLoggedIn,
    isAdmin,
    login,
    signup,
    updateAvatar,
    removeAvatar,
    applyExperience,
    logout,
    sync,
    ensureSession,
  };
});
