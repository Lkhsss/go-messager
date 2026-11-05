<script setup lang="ts">
import type { TagItem } from '@/api/tag'
import type { ReactionSummary, ReactionType } from '@/api/reaction'
import { tagBadgeStyle } from '@/utils/tagColor'
import UserAvatar from '@/components/UserAvatar.vue'
import ReactionBar from '@/components/ReactionBar.vue'

defineProps<{
  postId: number
  user: string
  title: string
  subtitle: string
  body: string
  userGravatar?: string
  tags?: TagItem[]
  reactions?: ReactionSummary
  myReactions?: ReactionType[]
  reacting?: boolean
}>()

defineEmits<{ (e: 'react', type: ReactionType): void }>()
</script>

<template>
  <article class="card bg-base-100 shadow-sm transition hover:-translate-y-0.5 hover:shadow-md">
    <div class="card-body gap-3">
      <header class="flex items-center gap-3">
        <UserAvatar :name="user" :hash="userGravatar" :size="40" />
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

      <ReactionBar :reactions="reactions" :my-reactions="myReactions" :busy="reacting"
        @react="$emit('react', $event)" />
    </div>
  </article>
</template>
