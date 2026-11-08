<script setup lang="ts">
import { computed, defineAsyncComponent } from 'vue'
import type { TagItem } from '@/api/tag'
import type { ReactionSummary, ReactionType } from '@/api/reaction'
import { tagBadgeStyle } from '@/utils/tagColor'
import { renderEmojiText } from '@/utils/emoji'
import UserAvatar from '@/components/UserAvatar.vue'
import ReactionBar from '@/components/ReactionBar.vue'

// Markdown 和代码高亮异步加载。
const MarkdownContent = defineAsyncComponent(() => import('@/components/MarkdownContent.vue'))

const props = defineProps<{
  postId: number
  user: string
  title: string
  subtitle: string
  body?: string
  excerpt?: string
  userAvatarVersion?: string
  tags?: TagItem[]
  reactions?: ReactionSummary
  myReactions?: ReactionType[]
  reacting?: boolean
  isPrivate?: boolean
  canManage?: boolean
  managing?: boolean
  detail?: boolean
}>()

defineEmits<{
  (e: 'react', type: ReactionType): void
  (e: 'toggle-private'): void
  (e: 'delete'): void
}>()

const renderedTitle = computed(() => renderEmojiText(props.title))
const renderedSubtitle = computed(() => renderEmojiText(props.subtitle))
const renderedExcerpt = computed(() => renderEmojiText(props.excerpt ?? ''))
</script>

<template>
  <article class="card bg-base-100 shadow-sm transition hover:-translate-y-0.5 hover:shadow-md">
    <div class="card-body gap-3">
      <header class="flex flex-wrap items-center gap-3">
        <UserAvatar :name="user" :version="userAvatarVersion" :size="40" />
        <div class="min-w-0">
          <p class="truncate font-medium">{{ user }}</p>
        </div>
        <span v-if="isPrivate" class="badge badge-warning badge-sm">Private</span>
        <div v-if="canManage" class="ml-auto flex items-center gap-1">
          <button type="button" class="btn btn-ghost btn-xs" :disabled="managing" @click="$emit('toggle-private')">
            {{ isPrivate ? '取消隐藏' : '隐藏' }}
          </button>
          <button type="button" class="btn btn-ghost btn-xs text-error" :disabled="managing" @click="$emit('delete')">
            删除
          </button>
        </div>
      </header>

      <div class="space-y-1">
        <h2 class="card-title break-words text-lg">
          <router-link v-if="!detail" :to="`/post/${postId}`" class="hover:underline">{{ renderedTitle }}</router-link>
          <span v-else>{{ renderedTitle }}</span>
        </h2>
        <p v-if="subtitle" class="break-words text-sm text-base-content/60">{{ renderedSubtitle }}</p>
      </div>

      <MarkdownContent v-if="detail" :source="body ?? ''" class="text-sm text-base-content/80" />
      <p v-else-if="excerpt" class="whitespace-pre-line text-sm leading-6 text-base-content/70">{{ renderedExcerpt }}</p>

      <div v-if="!detail" class="flex justify-end">
        <router-link :to="`/post/${postId}`" class="btn btn-ghost btn-sm px-2 text-primary">
          查看详情
          <svg aria-hidden="true" xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none"
            viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
          </svg>
        </router-link>
      </div>

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
