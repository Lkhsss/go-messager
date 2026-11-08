import { createRouter, createWebHistory } from "vue-router";
import { useAuthStore } from "@/stores/auth";
import { useSystemStore } from "@/stores/system";

// 路由 meta 字段。
declare module "vue-router" {
  interface RouteMeta {
    // 使用空白布局。
    layout?: "blank";
    // 需要登录。
    requiresAuth?: boolean;
    // 仅游客。
    guestOnly?: boolean;
    // 仅未初始化时。
    setupOnly?: boolean;
    // 仅管理员。
    requiresAdmin?: boolean;
  }
}

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: "/setup",
      name: "setup",
      component: () => import("@/views/SetupView.vue"),
      meta: { layout: "blank", setupOnly: true },
    },
    {
      path: "/login",
      name: "login",
      component: () => import("@/views/LoginView.vue"),
      meta: { layout: "blank", guestOnly: true },
    },
    {
      path: "/",
      name: "home",
      component: () => import("@/views/HomeView.vue"),
      meta: { requiresAuth: true },
    },
    {
      path: "/post/new",
      name: "post-new",
      component: () => import("@/views/PostCreateView.vue"),
      meta: { requiresAuth: true },
    },
    {
      path: "/post/:id",
      name: "post-detail",
      component: () => import("@/views/PostDetailView.vue"),
      meta: { requiresAuth: true },
    },
    {
      path: "/my-posts",
      name: "my-posts",
      component: () => import("@/views/MyPostsView.vue"),
      meta: { requiresAuth: true },
    },
    {
      path: "/post/:id/edit",
      name: "post-edit",
      component: () => import("@/views/PostCreateView.vue"),
      meta: { requiresAuth: true },
    },
    {
      path: "/leaderboard",
      name: "leaderboard",
      component: () => import("@/views/LeaderboardView.vue"),
      meta: { requiresAuth: true },
    },
    {
      path: "/data",
      name: "data",
      component: () => import("@/views/DataView.vue"),
      meta: { requiresAuth: true },
    },
    {
      path: "/admin",
      name: "admin",
      component: () => import("@/views/AdminView.vue"),
      meta: { requiresAuth: true, requiresAdmin: true },
    },
    {
      path: "/setting",
      name: "setting",
      component: () => import("@/views/SettingView.vue"),
      meta: { requiresAuth: true },
    },
  ],
});

router.beforeEach(async (to) => {
  const system = useSystemStore();

  // 未初始化时只允许 setup。
  const initialized = await system.ensureInitialized();
  if (!initialized) {
    return to.name === "setup" ? true : { name: "setup" };
  }
  if (to.name === "setup") {
    return { name: "login" };
  }

  // 校验 token 并同步登录态。
  const auth = useAuthStore();
  await auth.ensureSession();

  // 已登录跳过登录页。
  if (to.meta.guestOnly && auth.isLoggedIn) {
    return { name: "home" };
  }

  if (to.meta.requiresAuth && !auth.isLoggedIn) {
    return { name: "login", query: { redirect: to.fullPath } };
  }

  // 非管理员不能进管理页。
  if (to.meta.requiresAdmin && !auth.isAdmin) {
    return { name: "home" };
  }

  return true;
});

export default router;
