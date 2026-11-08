import { createRouter, createWebHistory } from "vue-router";
import { useAuthStore } from "@/stores/auth";
import { useSystemStore } from "@/stores/system";

// 路由 meta 字段的类型声明（vue-router 默认未声明这些字段）
declare module "vue-router" {
  interface RouteMeta {
    // 不套用带侧边栏的主布局
    layout?: "blank";
    // 未登录不可访问
    requiresAuth?: boolean;
    // 已登录时不可访问（如登录页）
    guestOnly?: boolean;
    // 只在系统未初始化（还没有第一个用户）时可访问
    setupOnly?: boolean;
    // 仅管理员可访问
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
    // {
    //   path: '/about',
    //   name: 'about',
    //   // route level code-splitting
    //   // this generates a separate chunk (About.[hash].js) for this route
    //   // which is lazy-loaded when the route is visited.
    //   component: () => import('../views/AboutView.vue'),
    // },
  ],
});

router.beforeEach(async (to) => {
  const system = useSystemStore();

  // 1. 系统还没创建第一个用户：只允许访问初始化页
  const initialized = await system.ensureInitialized();
  if (!initialized) {
    return to.name === "setup" ? true : { name: "setup" };
  }
  if (to.name === "setup") {
    return { name: "login" };
  }

  // 2. 有 token 时校验一次（角色以服务端为准），同时同步多标签页登录态
  const auth = useAuthStore();
  await auth.ensureSession();

  // 已登录用户不需要再看登录页
  if (to.meta.guestOnly && auth.isLoggedIn) {
    return { name: "home" };
  }

  if (to.meta.requiresAuth && !auth.isLoggedIn) {
    return { name: "login", query: { redirect: to.fullPath } };
  }

  // 非管理员不能进管理页（未登录的情况上面已重定向到登录页）
  if (to.meta.requiresAdmin && !auth.isAdmin) {
    return { name: "home" };
  }

  return true;
});

export default router;





