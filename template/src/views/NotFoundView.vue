<script setup lang="ts">
import { useRoute, useRouter } from 'vue-router'

const route = useRoute()
const router = useRouter()

// 直接打开 404 页时没有历史记录，退化为回首页。
function goBack() {
  if (window.history.length > 1) {
    router.back()
    return
  }
  void router.push({ name: 'home' })
}
</script>

<template>
  <main class="mx-auto flex w-full max-w-3xl flex-1 flex-col items-center justify-center px-4 py-16 text-center sm:py-24">
    <span class="text-rotate font-mono text-7xl font-black tracking-tight text-primary duration-5000 sm:text-8xl">
      <span class="justify-items-center">
        <span>404</span>
        <span>页面不存在</span>
      </span>
    </span>
    <!-- <h1 class="mt-4 text-2xl font-semibold">页面不存在</h1> -->
    <p class="mt-6 max-w-md text-sm text-base-content/60">
      你访问的页面可能已被删除、改名，或者链接拼写有误。
    </p>
    <code class="mt-4 max-w-full truncate rounded-box bg-base-200 px-3 py-1.5 font-mono text-xs text-base-content/60">
      {{ route.fullPath }}
    </code>

    <div class="mt-8 flex flex-wrap justify-center gap-2">
      <router-link :to="{ name: 'home' }" class="btn btn-primary btn-sm">回到首页</router-link>
      <button type="button" class="btn btn-ghost btn-sm" @click="goBack">返回上一页</button>
    </div>
  </main>
</template>
