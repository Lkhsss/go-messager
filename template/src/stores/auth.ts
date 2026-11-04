import { computed, ref } from "vue";
import { defineStore } from "pinia";
import { authorize, register, type Credentials } from "@/api/auth";

// 后端目前没有下发 token，登录态只保存在前端，刷新页面后需重新登录
const STORAGE_KEY = "go-messager:user";

function readStoredUser(): string | null {
  try {
    return localStorage.getItem(STORAGE_KEY);
  } catch {
    return null;
  }
}

export const useAuthStore = defineStore("auth", () => {
  const user = ref<string | null>(readStoredUser());
  const isLoggedIn = computed(() => user.value !== null);

  function setUser(name: string) {
    user.value = name;
    try {
      localStorage.setItem(STORAGE_KEY, name);
    } catch {
      // localStorage 不可用时忽略，仅保留内存中的登录态
    }
  }

  async function login(credentials: Credentials) {
    const result = await authorize(credentials);
    setUser(result.name);
    return result;
  }

  async function signup(credentials: Credentials) {
    const result = await register(credentials);
    setUser(result.name);
    return result;
  }

  function logout() {
    user.value = null;
    try {
      localStorage.removeItem(STORAGE_KEY);
    } catch {
      // 忽略
    }
  }

  // 从 localStorage 重新读取登录态（其它标签页登录/退出后同步过来）
  function sync() {
    user.value = readStoredUser();
  }

  return { user, isLoggedIn, login, signup, logout, sync };
});
