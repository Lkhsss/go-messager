<script setup lang="ts">
import type { MyStats } from '@/api/stats'

defineProps<{ stats: MyStats }>()
</script>

<template>
  <div class="grid grid-cols-2 gap-3 sm:grid-cols-4 lg:grid-cols-7">
    <div class="rounded-box border border-base-200 bg-base-100 p-3 shadow-sm">
      <p class="text-xs text-base-content/50">文章数量</p>
      <p class="mt-1 font-mono text-2xl font-semibold">{{ stats.post_count }}</p>
    </div>
    <div class="rounded-box border border-base-200 bg-base-100 p-3 shadow-sm">
      <p class="text-xs text-base-content/50">字数</p>
      <p class="mt-1 font-mono text-2xl font-semibold">{{ stats.word_count }}</p>
    </div>
    <div class="rounded-box border border-base-200 bg-base-100 p-3 shadow-sm">
      <p class="text-xs text-base-content/50">点赞</p>
      <p class="mt-1 font-mono text-2xl font-semibold">{{ stats.like }}</p>
    </div>
    <div class="rounded-box border border-base-200 bg-base-100 p-3 shadow-sm">
      <p class="text-xs text-base-content/50">踩</p>
      <p class="mt-1 font-mono text-2xl font-semibold">{{ stats.dislike }}</p>
    </div>
    <div class="rounded-box border border-base-200 bg-base-100 p-3 shadow-sm">
      <p class="text-xs text-base-content/50">疑问</p>
      <p class="mt-1 font-mono text-2xl font-semibold">{{ stats.question }}</p>
    </div>
    <div class="rounded-box border border-base-200 bg-base-100 p-3 shadow-sm">
      <p class="text-xs text-base-content/50">发出的评论</p>
      <p class="mt-1 font-mono text-2xl font-semibold">{{ stats.comment_count }}</p>
    </div>
    <div class="rounded-box border border-base-200 bg-base-100 p-3 shadow-sm">
      <p class="text-xs text-base-content/50">文章收到评论</p>
      <p class="mt-1 font-mono text-2xl font-semibold">{{ stats.received_comment_count }}</p>
    </div>
  </div>
</template>
