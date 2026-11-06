import { computed, ref } from "vue";
import { defineStore } from "pinia";
import {
  authorize,
  fetchMe,
  register,
  removeAvatar as removeAvatarApi,
  updateAvatar as updateAvatarApi,
  type Credentials,
} from "@/api/auth";
import { clearAuth, readAuth, writeAuth, type AuthUser, type StoredAuth } from "@/api/authStorage";
import { isApiError } from "@/api/http";

// 这些错误码说明登录态已不可用，需要清掉登录态
const AUTH_ERROR_CODES = ["UNAUTHORIZED", "INVALID_TOKEN", "TOKEN_EXPIRED", "ACCOUNT_DISABLED"];

export const useAuthStore = defineStore("auth", () => {
  const initial = readAuth();

  const token = ref<string | null>(initial?.token ?? null);
  const user = ref<AuthUser | null>(initial?.user ?? null);
  const expiresAt = ref(initial?.expiresAt ?? "");

  // 已经通过 /api/me 校验过的 token，避免每次路由跳转都发请求。
  // 初始为 null：刷新页面后先校验一次，避免信任 localStorage 里的旧角色/已封禁账号。
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
    if (next === null) {
      validatedToken = null;
    }
  }

  async function login(credentials: Credentials) {
    const result = await authorize(credentials);
    setAuth({ token: result.token, expiresAt: result.expires_at, user: result.user });
    return result;
  }

  async function signup(credentials: Credentials) {
    const result = await register(credentials);
    setAuth({ token: result.token, expiresAt: result.expires_at, user: result.user });
    return result;
  }

  // 上传头像：token 不变，只刷新用户资料
  async function updateAvatar(dataUrl: string) {
    const nextUser = await updateAvatarApi(dataUrl);
    if (token.value) {
      setAuth({ token: token.value, expiresAt: expiresAt.value, user: nextUser });
    }
    return nextUser;
  }

  // 清除头像
  async function removeAvatar() {
    const nextUser = await removeAvatarApi();
    if (token.value) {
      setAuth({ token: token.value, expiresAt: expiresAt.value, user: nextUser });
    }
    return nextUser;
  }

  function logout() {
    applyStored(null);
    clearAuth();
  }

  // 从 localStorage 重新读取登录态（其它标签页登录/退出后同步过来）
  function sync() {
    applyStored(readAuth());
  }

  // 校验本地 token 是否仍然有效，并从服务端刷新角色。
  // 角色以数据库为准：管理员在后台把某人改成 none 后，前端下次校验就会失效。
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
        // 请求期间用户可能已经退出或换了账号
        if (token.value !== requestedToken) return false;
        setAuth({ token: requestedToken, expiresAt: expiresAt.value, user: me });
        return true;
      })
      .catch((err) => {
        // 只有服务端明确否认 token 才登出；网络故障时保留登录态，避免断网掉线
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
    logout,
    sync,
    ensureSession,
  };
});

