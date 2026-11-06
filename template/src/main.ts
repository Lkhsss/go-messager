import "highlight.js/styles/github-dark.css";
import "./styles/main.css";

import { createApp } from "vue";
import { createPinia } from "pinia";

import App from "./App.vue";
import router from "./router";
import { AUTH_EXPIRED_EVENT } from "@/api/authStorage";
import { useAuthStore } from "@/stores/auth";

const app = createApp(App);

app.use(createPinia());
app.use(router);

// token 失效（见 api/http.ts）时清登录态并回到登录页，保留当前路径方便登录后跳回
window.addEventListener(AUTH_EXPIRED_EVENT, () => {
  const auth = useAuthStore();
  auth.logout();

  const current = router.currentRoute.value;
  if (current.name === "login" || current.name === "setup") return;

  void router.replace({ name: "login", query: { redirect: current.fullPath } });
});

app.mount("#app");
