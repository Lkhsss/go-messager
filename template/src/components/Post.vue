<script setup lang="ts">
import type { TagItem } from '@/api/tag'
import { tagBadgeStyle } from '@/utils/tagColor'

defineProps<{
  user: string
  title: string
  subtitle: string
  body: string
  tags?: TagItem[]
}>()
</script>

<template>
  <article class="card bg-base-100 shadow-sm transition hover:-translate-y-0.5 hover:shadow-md">
    <div class="card-body gap-3">
      <header class="flex items-center gap-3">
        <div class="avatar avatar-placeholder">
          <div class="w-10 rounded-full bg-primary text-primary-content">
            <span class="text-lg font-semibold">{{ user.slice(0, 1).toUpperCase() }}</span>
          </div>
        </div>
        <div class="min-w-0">
          <p class="truncate font-medium">{{ user }}</p>
        </div>
      </header>

      <div class="space-y-1">
        <h2 class="card-title break-words text-lg">{{ title }}</h2>
        <p v-if="subtitle" class="break-words text-sm text-base-content/60">{{ subtitle }}</p>
      </div>

      <p class="whitespace-pre-line break-words text-sm leading-relaxed text-base-content/80">{{ body }}</p>

      <div v-if="tags && tags.length" class="flex flex-wrap gap-1">
        <span v-for="tag in tags" :key="tag.ID" class="badge badge-sm" :style="tagBadgeStyle(tag)"
          :title="`被 ${tag.usage_count} 篇文章使用`">
          #{{ tag.name }}
        </span>
      </div>
    </div>
  </article>
</template>
