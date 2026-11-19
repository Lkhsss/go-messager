<script setup lang="ts">
import { computed, onMounted, onUnmounted } from "vue";
import { useRouter } from "vue-router";
import { useAuthStore } from "@/stores/auth";
import { useSystemStore } from "@/stores/system";

// 普通用户停留在维护页时定时确认站点是否已恢复。
const POLL_INTERVAL = 15000;

const router = useRouter();
const auth = useAuthStore();
const system = useSystemStore();

const title = computed(() => system.maintenance.title || "站点维护中");
const body = computed(() => (system.maintenance.body ?? "").trim());
const hasBody = computed(() => body.value.length > 0);

let timer: number | undefined;

async function poll() {
  await system.refreshStatus();
  if (!system.isMaintenance) {
    await router.replace({ name: "home" });
  }
}

// 已登录的普通用户可以退出，换成管理员账号登录。
function signOut() {
  auth.logout();
}

onMounted(() => {
  timer = window.setInterval(() => {
    void poll();
  }, POLL_INTERVAL);
});

onUnmounted(() => {
  if (timer !== undefined) window.clearInterval(timer);
});
</script>

<template>
  <div
    class="flex min-h-screen flex-col items-center justify-center bg-base-200 px-6 py-16 text-center"
  >
    <!-- 没有正文时只显示放大的标题；标题与「前方禁行」交替滚动 -->
    <template v-if="hasBody">
      <h1 class="max-w-3xl text-2xl font-semibold sm:text-3xl">
        <span class="text-rotate leading-tight duration-4000">
          <span class="justify-items-center">
            <span>{{ title }}</span>
            <span>前方禁行</span>
          </span>
        </span>
      </h1>
      <p class="mt-6 max-w-2xl whitespace-pre-line text-base leading-relaxed text-base-content/70">
        {{ body }}
      </p>
    </template>
    <h1 v-else class="max-w-4xl text-4xl font-bold tracking-tight sm:text-6xl">
      <span class="text-rotate leading-tight duration-4000">
        <span class="justify-items-center">
          <span>{{ title }}</span>
          <span>前方禁行</span>
        </span>
      </span>
    </h1>

    <div class="mt-10 flex flex-wrap items-center justify-center gap-3 text-sm">
      <router-link v-if="auth.isAdmin" to="/admin" class="btn btn-primary btn-sm"
        >进入管理</router-link
      >
      <template v-else-if="auth.isLoggedIn">
        <span class="text-base-content/50">站点恢复后会自动刷新</span>
        <button type="button" class="btn btn-ghost btn-sm" @click="signOut">退出登录</button>
      </template>
      <router-link v-else to="/login" class="btn btn-ghost btn-sm">管理员登录</router-link>
    </div>
  </div>
</template>
