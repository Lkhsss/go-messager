<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Post from '@/components/Post.vue'
import CommentSection from '@/components/CommentSection.vue'
import { fetchPost, type PostItem } from '@/api/post'
import { toggleReaction, type ReactionType } from '@/api/reaction'
import { isApiError } from '@/api/http'

const route = useRoute()
const router = useRouter()

const post = ref<PostItem | null>(null)
const loading = ref(false)
const errorMsg = ref('')
const reacting = ref(false)
function formatDate(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString('zh-CN')
}

async function loadPost() {
  const raw = Array.isArray(route.params.id) ? route.params.id[0] : route.params.id
  const id = Number(raw)
  if (!Number.isInteger(id) || id <= 0) {
    errorMsg.value = '文章 ID 无效'
    return
  }

  loading.value = true
  errorMsg.value = ''
  try {
    post.value = await fetchPost(id)
  } catch (err) {
    errorMsg.value = isApiError(err) ? err.message : '加载文章失败，请稍后再试'
  } finally {
    loading.value = false
  }
}

async function onReact(type: ReactionType) {
  if (!post.value || reacting.value) return

  reacting.value = true
  try {
    const result = await toggleReaction(post.value.ID, type)
    post.value.reactions = result.reactions
    post.value.my_reactions = result.my_reactions
  } catch (err) {
    errorMsg.value = isApiError(err) ? err.message : '操作失败，请稍后再试'
  } finally {
    reacting.value = false
  }
}

onMounted(() => {
  void loadPost()
})
</script>

<template>
  <main class="mx-auto w-full max-w-3xl p-4 sm:p-6 lg:p-8">
    <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
      <button type="button" class="btn btn-ghost btn-sm" @click="router.back()">返回</button>
      <span v-if="post" class="text-right text-xs text-base-content/40">
        {{ formatDate(post.CreatedAt) }}
      </span>
    </div>

    <div v-if="loading" class="flex justify-center py-16">
      <span class="loading loading-spinner loading-lg"></span>
    </div>

    <div v-else-if="errorMsg" role="alert" class="alert alert-error alert-soft">
      <span>{{ errorMsg }}</span>
      <button type="button" class="btn btn-sm" @click="loadPost">重试</button>
      <router-link to="/" class="btn btn-ghost btn-sm">回首页</router-link>
    </div>

    <template v-else-if="post">
      <Post :post-id="post.ID" :user="post.user" :title="post.title" :subtitle="post.subtitle" :body="post.body"
        :user-avatar-version="post.user_avatar_version" :tags="post.tags" :reactions="post.reactions"
        :my-reactions="post.my_reactions" :is-private="post.private" :reacting="reacting" :detail="true"
        @react="onReact" />

      <CommentSection :post-id="post.ID" />
    </template>
  </main>
</template>
