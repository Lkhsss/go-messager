<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { createComment, deletePost, fetchComments, type CommentItem, type PostItem } from '@/api/post'
import { isApiError } from '@/api/http'
import CommentNode from '@/components/CommentNode.vue'

const props = defineProps<{ postId: number }>()

const comments = ref<CommentItem[]>([])
const flatComments = ref<PostItem[]>([])
const loading = ref(false)
const errorMsg = ref('')
const body = ref('')
const submitting = ref(false)

// 删除评论
const deleteDialog = ref<HTMLDialogElement | null>(null)
const deletingComment = ref<CommentItem | null>(null)
const deleting = ref(false)
const deleteError = ref('')

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

function descendantCount(comment: CommentItem): number {
  let total = 0
  for (const child of comment.children) {
    total += 1 + descendantCount(child)
  }
  return total
}

function openDelete(comment: CommentItem) {
  deletingComment.value = comment
  deleteError.value = ''
  if (!deleteDialog.value?.open) {
    deleteDialog.value?.showModal()
  }
}

function closeDelete() {
  if (deleting.value) return
  deleteDialog.value?.close()
}

function onDeleteClosed() {
  deletingComment.value = null
  deleteError.value = ''
}

function onDeleteCancel(event: Event) {
  if (deleting.value) {
    event.preventDefault()
  }
}

async function confirmDelete() {
  if (deleting.value) return

  const target = deletingComment.value
  if (!target) return

  deleting.value = true
  deleteError.value = ''
  try {
    await deletePost(target.ID)
    deleting.value = false
    deleteDialog.value?.close()
    await loadComments()
  } catch (err) {
    deleteError.value = isApiError(err) ? err.message : '删除评论失败，请稍后再试'
  } finally {
    deleting.value = false
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
        <div class="flex justify-end gap-2">

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
          @created="loadComments" @delete="openDelete" />
      </div>
    </div>
    <!-- 删除评论确认弹窗 -->
    <dialog ref="deleteDialog" class="modal" @cancel="onDeleteCancel" @close="onDeleteClosed">
      <div class="modal-box">
        <h3 class="text-lg font-semibold">删除评论</h3>

        <div class="mt-3 space-y-3 text-sm">
          <p>确定删除这条评论吗？</p>
          <p v-if="deletingComment && descendantCount(deletingComment) > 0" class="text-warning">
            这条评论下还有 {{ descendantCount(deletingComment) }} 条回复，删除后会一并删除。
          </p>
          <p class="text-base-content/60">删除后不可恢复。</p>

          <div v-if="deleteError" role="alert" class="alert alert-error alert-soft py-2 text-sm">
            <span>{{ deleteError }}</span>
          </div>
        </div>

        <div class="modal-action">
          <button type="button" class="btn btn-ghost" :disabled="deleting" @click="closeDelete">取消</button>
          <button type="button" class="btn btn-error" :disabled="deleting" @click="confirmDelete">
            <span v-if="deleting" class="loading loading-spinner loading-sm"></span>
            删除
          </button>
        </div>
      </div>
      <form method="dialog" class="modal-backdrop" :class="{ 'pointer-events-none': deleting }"><button>close</button></form>
    </dialog>
  </section>
</template>




