<script setup lang="ts">
import { ref } from 'vue'
import { RouterLink, RouterView, useRoute, useRouter } from 'vue-router'
import SidebartoggleOpen from './components/icons/SidebarToggleOpen.vue';
import SidebarToggleClose from './components/icons/SidebarToggleClose.vue';
import HomeIcon from './components/icons/HomeIcon.vue';
import SettingIcon from './components/icons/SettingIcon.vue';
import { useAuthStore } from './stores/auth';

const sidebarOpen = ref(false)
function toggleSidebar() {
  sidebarOpen.value = !sidebarOpen.value
}

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

async function logout() {
  auth.logout()
  await router.replace({ name: 'login' })
}
</script>

<template>
  <!-- 登录页等独立布局的路由不显示侧边栏 -->
  <RouterView v-if="route.meta.layout === 'blank'" />
  <div v-else class="drawer lg:drawer-open">
    <input id="my-drawer-4" v-model="sidebarOpen" type="checkbox" class="drawer-toggle inline" />
    <div class="drawer-content">
      <!-- Navbar -->
      <nav class="navbar w-full bg-base-300">
        <button type="button" :aria-label="sidebarOpen ? 'close sidebar' : 'open sidebar'" aria-controls="my-drawer-4"
          :aria-expanded="sidebarOpen" class="btn btn-square btn-ghost drawer-button" @click="toggleSidebar">
          <SidebartoggleOpen v-if="!sidebarOpen" />
          <SidebarToggleClose v-else />
        </button>
        <div class="flex-1">
          <div class="tooltip tooltip-right">
            <div class="tooltip-content">
              <div class="animate-bounce text-orange-250 -rotate-10 text-2xl font-black">From LKH!</div>
            </div><a class="btn btn-ghost text-xl">Go Messager</a>
          </div>

        </div>
        <div class="flex-none">
          <span v-if="auth.user" class="hidden text-sm text-base-content/70 sm:inline">{{ auth.user }}</span>
          <button type="button" class="btn btn-ghost btn-sm" @click="logout">退出登录</button>
        </div>
      </nav>
      <!-- Page content here -->
      <RouterView />
    </div>

    <div class="drawer-side is-drawer-close:overflow-visible">
      <label for="my-drawer-4" aria-label="close sidebar" class="drawer-overlay"></label>
      <div class="flex min-h-full flex-col items-start bg-base-200 is-drawer-close:w-14 is-drawer-open:w-64">
        <!-- Sidebar content here -->
        <ul class="menu w-full grow">
          <!-- List item -->
          <li>

            <router-link to="/" class="is-drawer-close:tooltip is-drawer-close:tooltip-right" data-tip="Homepage">
              <!-- Home icon -->
              <HomeIcon />
              <span class="is-drawer-close:hidden">Homepage</span>
            </router-link>
          </li>

          <!-- List item -->
          <li>
            <router-link to="/setting" class="is-drawer-close:tooltip is-drawer-close:tooltip-right"
              data-tip="Settings">
              <!-- Settings icon -->
              <SettingIcon />
              <span class="is-drawer-close:hidden">Settings</span>
            </router-link>
          </li>
        </ul>
      </div>
    </div>
  </div>
</template>
