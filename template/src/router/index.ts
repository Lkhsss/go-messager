import { createRouter, createWebHistory } from "vue-router";
import HomeView from "../views/HomeView.vue";
import SettingView from "@/views/SettingView.vue";
import LoginView from "@/views/LoginView.vue";
import { useAuthStore } from "@/stores/auth";

// 路由 meta 字段的类型声明（vue-router 默认未声明这些字段）
declare module "vue-router" {
  interface RouteMeta {
    // 不套用带侧边栏的主布局
    layout?: "blank";
    // 未登录不可访问
    requiresAuth?: boolean;
    // 已登录时不可访问（如登录页）
    guestOnly?: boolean;
  }
}

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: "/login",
      name: "login",
      component: LoginView,
      meta: { layout: "blank", guestOnly: true },
    },
    {
      path: "/",
      name: "home",
      component: HomeView,
      meta: { requiresAuth: true },
    },
    {
      path: "/setting",
      name: "setting",
      // route level code-splitting
      // this generates a separate chunk (About.[hash].js) for this route
      // which is lazy-loaded when the route is visited.
      component: SettingView,
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

router.beforeEach((to) => {
  const auth = useAuthStore();

  // 登录态存在 localStorage 中（多标签页共享），导航前先同步，避免其他标签页退出后当前页面仍是旧状态
  auth.sync();

  // 已登录用户不需要再看登录页
  if (to.meta.guestOnly && auth.isLoggedIn) {
    return { name: "home" };
  }

  if (to.meta.requiresAuth && !auth.isLoggedIn) {
    return { name: "login", query: { redirect: to.fullPath } };
  }

  return true;
});

export default router;

