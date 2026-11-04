<script setup lang="ts">
import { ref } from 'vue'
import { RouterLink, RouterView } from 'vue-router'
import SidebartoggleOpen from './components/icons/SidebarToggleOpen.vue';
import SidebarToggleClose from './components/icons/SidebarToggleClose.vue';
import HomeIcon from './components/icons/HomeIcon.vue';
import SettingIcon from './components/icons/SettingIcon.vue';

const sidebarOpen = ref(false)
function toggleSidebar() {
  sidebarOpen.value = !sidebarOpen.value
}
</script>

<template>
  <div class="drawer lg:drawer-open">
    <input id="my-drawer-4" v-model="sidebarOpen" type="checkbox" class="drawer-toggle inline" />
    <div class="drawer-content">
      <!-- Navbar -->
      <nav class="navbar w-full bg-base-300">
        <button type="button" :aria-label="sidebarOpen ? 'close sidebar' : 'open sidebar'" aria-controls="my-drawer-4"
          :aria-expanded="sidebarOpen" class="btn btn-square btn-ghost drawer-button" @click="toggleSidebar">
          <SidebartoggleOpen v-if="!sidebarOpen" />
          <SidebarToggleClose v-else />
        </button>
        <div class="navbar-center">
          <a class="btn btn-ghost text-xl">Go Messager</a>
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
            <router-link to="/setting" class="is-drawer-close:tooltip is-drawer-close:tooltip-right" data-tip="Settings">
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
