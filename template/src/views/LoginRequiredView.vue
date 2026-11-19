<script setup lang="ts">
import { computed } from "vue";
import { useRoute, useRouter } from "vue-router";
import { redirectToLogin } from "@/utils/authRedirect";

const route = useRoute();
const router = useRouter();

// 只接受站内路径，避免开放重定向。
const redirectPath = computed(() => {
  const query = route.query.redirect;
  if (typeof query === "string" && query.startsWith("/") && !query.startsWith("//")) {
    return query;
  }
  return "/";
});

function goLogin() {
  redirectToLogin(router, redirectPath.value);
}

// 直接打开 401 页时没有历史记录，退化为回首页。
function goBack() {
  if (window.history.length > 1) {
    router.back();
    return;
  }
  void router.push({ name: "home" });
}
</script>

<template>
  <main
    class="mx-auto flex w-full max-w-3xl flex-1 flex-col items-center justify-center px-4 py-16 text-center sm:py-24"
  >
    <span
      class="text-rotate font-mono text-7xl font-black tracking-tight text-primary duration-5000 sm:text-8xl"
    >
      <span class="justify-items-center">
        <span>401</span>
        <span>需要登录</span>
      </span>
    </span>

    <p class="mt-6 max-w-md text-sm text-base-content/60">
      这个页面需要登录后才能访问，登录成功后会回到刚才的位置。
    </p>
    <code
      class="mt-4 max-w-full truncate rounded-box bg-base-200 px-3 py-1.5 font-mono text-xs text-base-content/60"
    >
      {{ redirectPath }}
    </code>

    <div class="mt-8 flex flex-wrap justify-center gap-2">
      <button type="button" class="btn btn-primary btn-sm" @click="goLogin">去登录</button>
      <button type="button" class="btn btn-ghost btn-sm" @click="goBack">返回上一页</button>
    </div>
  </main>
</template>
