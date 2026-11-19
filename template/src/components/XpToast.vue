<script setup lang="ts">
import { useToastStore } from "@/stores/toast";

const toast = useToastStore();
</script>

<template>
  <div class="toast toast-top toast-end z-50">
    <div
      v-for="item in toast.items"
      :key="item.id"
      role="status"
      aria-live="polite"
      class="alert shadow-lg"
      :class="item.levelUp ? 'alert-warning' : 'alert-success'"
    >
      <div>
        <p class="font-semibold">{{ item.title }}</p>
        <p class="text-xs opacity-80">{{ item.detail }}</p>
      </div>
      <button type="button" class="btn btn-ghost btn-xs" @click="toast.dismiss(item.id)">
        知道了
      </button>
    </div>
  </div>
</template>
