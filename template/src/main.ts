import "./styles/main.css";

import { createApp } from "vue";
import { createPinia } from "pinia";

import App from "./App.vue";
import router from "./router";
import { AUTH_EXPIRED_EVENT } from "@/api/authStorage";
import { useAuthStore } from "@/stores/auth";
import { useSystemStore } from "@/stores/system";

const app = createApp(App);

app.use(createPinia());
app.use(router);

// token 失效：清登录态并回登录页。
window.addEventListener(AUTH_EXPIRED_EVENT, () => {
  const auth = useAuthStore();
  const system = useSystemStore();
  auth.logout();

  const current = router.currentRoute.value;
  if (current.name === "login" || current.name === "setup") return;

  // 公开模式下当前页面可匿名访问时留在原页，避免游客被弹到登录页。
  if (system.isPublic && !current.meta.requiresAuth) return;

  void router.replace({ name: "login", query: { redirect: current.fullPath } });
});

app.mount("#app");
