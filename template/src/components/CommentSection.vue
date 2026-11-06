<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { createComment, fetchComments, type CommentItem, type PostItem } from '@/api/post'
import { isApiError } from '@/api/http'
import CommentNode from '@/components/CommentNode.vue'

const props = defineProps<{ postId: number }>()

const comments = ref<CommentItem[]>([])
const flatComments = ref<PostItem[]>([])
const loading = ref(false)
const errorMsg = ref('')
const body = ref('')
const submitting = ref(false)

function buildTree(list: PostItem[]): CommentItem[] {
  const map = new Map<number, CommentItem>()
  for (const item of list) {
    map.set(item.ID, { ...item, children: [] })
  }

  const roots: CommentItem[] = []
  for (const node of map.values()) {
    const parent = node.parent_id ? map.get(node.parent_id) : undefined
    if (parent) {
      parent.children.push(node)
    } else {
      roots.push(node)
    }
  }

  return roots
}

async function loadComments() {
  loading.value = true
  errorMsg.value = ''
  try {
    const list = await fetchComments(props.postId)
    flatComments.value = list
    comments.value = buildTree(list)
  } catch (err) {
    errorMsg.value = isApiError(err) ? err.message : '获取评论失败，请稍后再试'
  } finally {
    loading.value = false
  }
}

async function submitComment() {
  if (submitting.value) return

  const content = body.value.trim()
  if (!content) {
    errorMsg.value = '请输入评论内容'
    return
  }

  submitting.value = true
  errorMsg.value = ''
  try {
    await createComment(props.postId, content)
    body.value = ''
    await loadComments()
  } catch (err) {
    errorMsg.value = isApiError(err) ? err.message : '发表评论失败，请稍后再试'
  } finally {
    submitting.value = false
  }
}

onMounted(() => {
  void loadComments()
})
</script>

<template>
  <section class="card mt-6 bg-base-100 shadow-sm">
    <div class="card-body gap-4">
      <div class="flex flex-wrap items-end justify-between gap-2">
        <h2 class="card-title text-lg">评论区</h2>
        <span class="text-xs text-base-content/45">{{ flatComments.length }} 条评论</span>
      </div>

      <form class="space-y-2" @submit.prevent="submitComment">
        <textarea v-model="body" rows="4" class="textarea textarea-bordered w-full text-sm"
          placeholder="写下你的评论，支持 Markdown…" :disabled="submitting"></textarea>
        <div class="flex items-center justify-between gap-2">
          <span class="text-xs text-base-content/45">评论也可以收到点赞和表情反应</span>
          <button type="submit" class="btn btn-primary btn-sm" :disabled="submitting">
            <span v-if="submitting" class="loading loading-spinner loading-sm"></span>
            发表评论
          </button>
        </div>
      </form>

      <div v-if="errorMsg" role="alert" class="alert alert-error alert-soft py-2 text-sm">
        <span>{{ errorMsg }}</span>
      </div>

      <div v-if="loading && flatComments.length === 0" class="flex justify-center py-8">
        <span class="loading loading-spinner loading-lg"></span>
      </div>

      <div v-else-if="comments.length === 0" class="py-8 text-center text-sm text-base-content/45">
        还没有评论，来写第一条吧
      </div>

      <div v-else class="space-y-3" :class="{ 'opacity-60': loading }">
        <CommentNode v-for="comment in comments" :key="comment.ID" :comment="comment" :post-id="postId"
          @created="loadComments" />
      </div>
    </div>
  </section>
</template>
