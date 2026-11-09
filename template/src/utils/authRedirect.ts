import type { Router } from "vue-router";

// 未登录时跳转登录页，并带上回跳地址。
export function redirectToLogin(router: Router, redirect: string): void {
  void router.push({ name: "login", query: { redirect } });
}