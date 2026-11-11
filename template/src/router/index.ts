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
    // 无需登录即可访问（如 401 提示页）。
    allowAnonymous?: boolean;
    // 维护模式下仍然允许访问（登录、初始化、维护页）。
    skipMaintenance?: boolean;
  }
}

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: "/setup",
      name: "setup",
      component: () => import("@/views/SetupView.vue"),
      meta: { layout: "blank", setupOnly: true, skipMaintenance: true },
    },
    {
      path: "/login",
      name: "login",
      component: () => import("@/views/LoginView.vue"),
      meta: { layout: "blank", guestOnly: true, skipMaintenance: true },
    },
    {
      path: "/maintenance",
      name: "maintenance",
      component: () => import("@/views/MaintenanceView.vue"),
      // 维护页本身不能被维护判断再次拦截。
      meta: { layout: "blank", skipMaintenance: true },
    },
    {
      path: "/",
      name: "home",
      component: () => import("@/views/HomeView.vue"),
      // 公开模式下游客可浏览；隐私模式由全局守卫统一拦截。
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
      // 公开模式下游客可浏览；隐私模式由全局守卫统一拦截。
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
      // 公开模式下游客可浏览；隐私模式由全局守卫统一拦截。
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
      path: "/me",
      name: "me",
      component: () => import("@/views/MeView.vue"),
      meta: { requiresAuth: true },
    },
    // 设置页暂时下线，等待后续调整后恢复。
    // {
    //   path: "/setting",
    //   name: "setting",
    //   component: () => import("@/views/SettingView.vue"),
    //   meta: { requiresAuth: true },
    // },
    {
      path: "/401",
      name: "login-required",
      component: () => import("@/views/LoginRequiredView.vue"),
      // 隐私模式下也不能被守卫再次拦截。
      meta: { allowAnonymous: true },
    },
    {
      // 未知路径统一显示 404 界面（必须放在最后）。
      path: "/:pathMatch(.*)*",
      name: "not-found",
      component: () => import("@/views/NotFoundView.vue"),
      // 公开模式下游客可浏览；隐私模式由全局守卫统一拦截。
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

  // 维护模式：普通用户只能停留在维护页，管理员照常使用。
  if (to.name === "maintenance") {
    return system.isMaintenance || auth.isAdmin ? true : { name: "home" };
  }
  if (system.isMaintenance && !auth.isAdmin && !to.meta.skipMaintenance) {
    return { name: "maintenance" };
  }

  // 已登录跳过登录页。
  if (to.meta.guestOnly && auth.isLoggedIn) {
    return { name: "home" };
  }

  // 401 提示页无需登录；已登录用户不需要看它。
  if (to.meta.allowAnonymous) {
    return auth.isLoggedIn ? { name: "home" } : true;
  }

  // 隐私模式下除登录/初始化页外都要求登录；公开模式只拦截需要登录的页面。
  const needsAuth = to.meta.requiresAuth || (!system.isPublic && !to.meta.guestOnly);
  if (needsAuth && !auth.isLoggedIn) {
    // 公开模式展示 401 页面，隐私模式保持直接跳登录页。
    if (system.isPublic) {
      return { name: "login-required", query: { redirect: to.fullPath } };
    }
    return { name: "login", query: { redirect: to.fullPath } };
  }

  // 非管理员不能进管理页。
  if (to.meta.requiresAdmin && !auth.isAdmin) {
    return { name: "home" };
  }

  return true;
});

export default router;
