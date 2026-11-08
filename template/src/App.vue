<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { RouterLink, RouterView, useRoute, useRouter } from 'vue-router'
import SidebartoggleOpen from './components/icons/SidebarToggleOpen.vue';
import SidebarToggleClose from './components/icons/SidebarToggleClose.vue';
import HomeIcon from './components/icons/HomeIcon.vue';
import SettingIcon from './components/icons/SettingIcon.vue';
import UserAvatar from './components/UserAvatar.vue';
import SiteFooter from './components/SiteFooter.vue';
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

function isTypingTarget(target: EventTarget | null): boolean {
  return target instanceof HTMLElement &&
    Boolean(target.closest('input, textarea, select, [contenteditable="true"]'))
}

function focusSearch(): boolean {
  const input = document.querySelector<HTMLInputElement>('input[type="search"]')
  if (!input) return false
  input.focus()
  input.select()
  return true
}

function onShortcutKeydown(event: KeyboardEvent) {
  if (event.defaultPrevented || event.repeat || event.ctrlKey || event.metaKey || event.altKey) return
  if (route.meta.layout === 'blank' || isTypingTarget(event.target)) return
  if (document.querySelector('dialog[open]')) return

  const key = event.key.toLowerCase()
  if (key === 's') {
    if (focusSearch()) event.preventDefault()
    return
  }

  if (key === 'n') {
    event.preventDefault()
    if (route.name !== 'post-new') {
      void router.push({ name: 'post-new' })
    }
  }
}

onMounted(() => window.addEventListener('keydown', onShortcutKeydown))
onUnmounted(() => window.removeEventListener('keydown', onShortcutKeydown))
</script>

<template>
  <!-- 登录页等独立布局的路由不显示侧边栏 -->
  <RouterView v-if="route.meta.layout === 'blank'" />
  <div v-else class="drawer lg:drawer-open">
    <input id="my-drawer-4" v-model="sidebarOpen" type="checkbox" class="drawer-toggle inline" />
    <div class="drawer-content flex min-h-screen flex-col">
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
        <div class="flex-none flex items-center gap-2">
          <UserAvatar v-if="auth.user" :name="auth.user.name" :version="auth.user.avatar_version" :size="32" />
          <span v-if="auth.isAdmin" class="badge badge-primary badge-sm">管理员</span>
          <span v-if="auth.user" class="hidden text-sm text-base-content/70 sm:inline">{{ auth.user.name }}</span>
          <button type="button" class="btn btn-ghost btn-sm" @click="logout">退出登录</button>
        </div>
      </nav>
      <div class="flex flex-1 flex-col">
        <RouterView />
      </div>
      <SiteFooter />
    </div>

    <div class="drawer-side is-drawer-close:overflow-visible">
      <label for="my-drawer-4" aria-label="close sidebar" class="drawer-overlay"></label>
      <div class="flex min-h-full flex-col items-start bg-base-200 is-drawer-close:w-14 is-drawer-open:w-64">
        <ul class="menu w-full grow">
          <li>

            <router-link to="/" class="is-drawer-close:tooltip is-drawer-close:tooltip-right" data-tip="Homepage">
              <HomeIcon />
              <span class="is-drawer-close:hidden">Homepage</span>
            </router-link>
          </li>

          <li>
            <router-link to="/post/new" class="is-drawer-close:tooltip is-drawer-close:tooltip-right"
              data-tip="写文章">
              <svg aria-label="写文章" xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none"
                viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                  d="M11 5h6a2 2 0 012 2v11a2 2 0 01-2 2H6a2 2 0 01-2-2v-6m12-8L8 15l-4 1 1-4 8-8z" />
              </svg>
              <span class="is-drawer-close:hidden">写文章</span>
            </router-link>
          </li>

          <!-- 我的文章 -->
          <li>
            <router-link to="/my-posts" class="is-drawer-close:tooltip is-drawer-close:tooltip-right"
              data-tip="我的文章">
              <svg aria-label="我的文章" xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none"
                viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                  d="M7 4h7l4 4v12H7z" />
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14 4v4h4" />
              </svg>
              <span class="is-drawer-close:hidden">我的文章</span>
            </router-link>
          </li>
          <!-- 排行榜 -->
          <li>
            <router-link to="/leaderboard" class="is-drawer-close:tooltip is-drawer-close:tooltip-right"
              data-tip="排行榜">
              <svg aria-label="排行榜" xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none"
                viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                  d="M8 21h8m-4-4v4m-6-9a3 3 0 01-3-3V5h4v4a3 3 0 01-3 3zm12 0a3 3 0 01-3-3V5h4v4a3 3 0 01-3 3zM6 5h12v4a6 6 0 01-12 0V5z" />
              </svg>
              <span class="is-drawer-close:hidden">排行榜</span>
            </router-link>
          </li>
          <!-- 数据 -->
          <li>
            <router-link to="/data" class="is-drawer-close:tooltip is-drawer-close:tooltip-right"
              data-tip="数据">
              <svg aria-label="数据" xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none"
                viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                  d="M4 19V5m0 14h16M8 16l3-5 3 3 4-6" />
              </svg>
              <span class="is-drawer-close:hidden">数据</span>
            </router-link>
          </li>
          <!-- 仅管理员可见：管理 -->
          <li v-if="auth.isAdmin">
            <router-link to="/admin" class="is-drawer-close:tooltip is-drawer-close:tooltip-right"
              data-tip="管理">
              <svg aria-label="管理" xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none"
                viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                  d="M12 3l7 4v5c0 4.5-3 7.5-7 9-4-1.5-7-4.5-7-9V7l7-4z" />
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4" />
              </svg>
              <span class="is-drawer-close:hidden">管理</span>
            </router-link>
          </li>

          <li>
            <router-link to="/setting" class="is-drawer-close:tooltip is-drawer-close:tooltip-right"
              data-tip="Settings">
              <SettingIcon />
              <span class="is-drawer-close:hidden">Settings</span>
            </router-link>
          </li>
        </ul>
      </div>
    </div>
  </div>
</template>



