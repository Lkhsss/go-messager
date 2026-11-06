<script setup lang="ts">
import { computed, ref } from 'vue'
import type { CommentItem } from '@/api/post'
import { createComment } from '@/api/post'
import { toggleReaction, type ReactionType } from '@/api/reaction'
import { isApiError } from '@/api/http'
import { renderMarkdown } from '@/utils/markdown'
import UserAvatar from '@/components/UserAvatar.vue'
import ReactionBar from '@/components/ReactionBar.vue'
import { useAuthStore } from '@/stores/auth'

defineOptions({ name: 'CommentNode' })

const props = withDefaults(
  defineProps<{
    comment: CommentItem
    postId: number
    depth?: number
  }>(),
  { depth: 0 },
)

const emit = defineEmits<{
  (e: 'created'): void
  (e: 'delete', comment: CommentItem): void
}>()

const auth = useAuthStore()
const canDelete = computed(() => auth.isAdmin || auth.user?.name === props.comment.user)

const replying = ref(false)
const replyBody = ref('')
const submitting = ref(false)
const reacting = ref(false)
const errorMsg = ref('')

const renderedBody = computed(() => renderMarkdown(props.comment.body ?? ''))

function formatDate(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString('zh-CN')
}

async function submitReply() {
  if (submitting.value) return

  const body = replyBody.value.trim()
  if (!body) {
    errorMsg.value = '请输入回复内容'
    return
  }

  submitting.value = true
  errorMsg.value = ''
  try {
    await createComment(props.postId, body, props.comment.ID)
    replyBody.value = ''
    replying.value = false
    emit('created')
  } catch (err) {
    errorMsg.value = isApiError(err) ? err.message : '回复失败，请稍后再试'
  } finally {
    submitting.value = false
  }
}

async function onReact(type: ReactionType) {
  if (reacting.value) return

  reacting.value = true
  errorMsg.value = ''
  try {
    const result = await toggleReaction(props.comment.ID, type)
    props.comment.reactions = result.reactions
    props.comment.my_reactions = result.my_reactions
  } catch (err) {
    errorMsg.value = isApiError(err) ? err.message : '操作失败，请稍后再试'
  } finally {
    reacting.value = false
  }
}
</script>

<template>
  <article class="rounded-box border border-base-200 p-3 sm:p-4"
    :class="{ 'ml-3 border-l-4 border-l-base-300 sm:ml-6': depth > 0 }">
    <header class="flex items-center gap-2">
      <UserAvatar :name="comment.user" :version="comment.user_avatar_version" :size="32" />
      <div class="min-w-0">
        <p class="truncate text-sm font-medium">{{ comment.user }}</p>
        <p class="text-[0.65rem] text-base-content/40">{{ formatDate(comment.CreatedAt) }}</p>
      </div>
    </header>

    <div class="markdown-body mt-2 text-sm text-base-content/80" v-html="renderedBody"></div>

    <div class="mt-2 flex flex-wrap items-center gap-2">
      <ReactionBar :reactions="comment.reactions" :my-reactions="comment.my_reactions" :busy="reacting"
        @react="onReact" />
      <button type="button" class="btn btn-ghost btn-xs" @click="replying = !replying">回复</button>
      <button v-if="canDelete" type="button" class="btn btn-ghost btn-xs text-error" @click="$emit('delete', comment)">删除</button>
    </div>

    <div v-if="replying" class="mt-2 space-y-2">
      <textarea v-model="replyBody" rows="3" class="textarea textarea-bordered w-full text-sm"
        placeholder="写下回复…" :disabled="submitting"></textarea>
      <div class="flex justify-end gap-2">
        <button type="button" class="btn btn-ghost btn-xs" :disabled="submitting"
          @click="replying = false; replyBody = ''; errorMsg = ''">
          取消
        </button>
        <button type="button" class="btn btn-primary btn-xs" :disabled="submitting" @click="submitReply">
          <span v-if="submitting" class="loading loading-spinner loading-xs"></span>
          回复
        </button>
      </div>
    </div>

    <div v-if="errorMsg" class="mt-1 text-xs text-error">{{ errorMsg }}</div>

    <div v-if="comment.children.length" class="mt-3 space-y-3">
      <CommentNode v-for="child in comment.children" :key="child.ID" :comment="child" :post-id="postId"
        :depth="depth + 1" @created="$emit('created')" @delete="$emit('delete', $event)" />
    </div>
  </article>
</template>


