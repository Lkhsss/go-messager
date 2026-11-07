<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import Pagination from '@/components/Pagination.vue'
import { deletePost, fetchMyPosts, setPostPrivate, type PostItem } from '@/api/post'
import { isApiError, type PaginationMeta } from '@/api/http'
import { tagBadgeStyle } from '@/utils/tagColor'
import { renderEmojiText } from '@/utils/markdown'
import { useAuthStore } from '@/stores/auth'

const PER_PAGE = 10

const router = useRouter()
const auth = useAuthStore()
const posts = ref<PostItem[]>([])
const meta = ref<PaginationMeta | null>(null)
const page = ref(1)
const loading = ref(false)
const errorMsg = ref('')
const successMsg = ref('')

const deleteDialog = ref<HTMLDialogElement | null>(null)
const deletingPost = ref<PostItem | null>(null)
const deleting = ref(false)
const deleteError = ref('')
const managingPostId = ref<number | null>(null)

function formatDate(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString('zh-CN')
}

async function onTogglePrivate(post: PostItem) {
  if (managingPostId.value !== null) return

  managingPostId.value = post.ID
  errorMsg.value = ''
  successMsg.value = ''
  try {
    const updated = await setPostPrivate(post.ID, !post.private)
    post.private = updated.private
    const title = renderEmojiText(post.title)
    successMsg.value = updated.private ? `文章「${title}」已隐藏` : `文章「${title}」已取消隐藏`
  } catch (err) {
    errorMsg.value = isApiError(err) ? err.message : '更新可见性失败，请稍后再试'
  } finally {
    managingPostId.value = null
  }
}

function editPost(post: PostItem) {
  void router.push({ name: 'post-edit', params: { id: post.ID } })
}

async function loadPosts(next = page.value) {
  if (loading.value) return

  loading.value = true
  errorMsg.value = ''
  try {
    const result = await fetchMyPosts({ page: next, perPage: PER_PAGE })
    posts.value = result.posts
    meta.value = result.meta
    page.value = result.meta.page > 0 ? result.meta.page : next
  } catch (err) {
    errorMsg.value = isApiError(err) ? err.message : '获取我的文章失败，请稍后再试'
  } finally {
    loading.value = false
  }
}

function goTo(next: number) {
  const totalPages = meta.value?.total_pages ?? 0
  if (loading.value || next < 1 || next > totalPages || next === page.value) return
  void loadPosts(next)
}

function openDelete(post: PostItem) {
  deletingPost.value = post
  deleteError.value = ''
  errorMsg.value = ''
  successMsg.value = ''
  if (!deleteDialog.value?.open) {
    deleteDialog.value?.showModal()
  }
}

function closeDelete() {
  if (deleting.value) return
  deleteDialog.value?.close()
}

function onDeleteClosed() {
  deletingPost.value = null
  deleteError.value = ''
}

function onDeleteCancel(event: Event) {
  if (deleting.value) {
    event.preventDefault()
  }
}

async function confirmDelete() {
  if (deleting.value) return

  const target = deletingPost.value
  if (!target) return

  deleting.value = true
  deleteError.value = ''
  try {
    await deletePost(target.ID)
    successMsg.value = `文章「${renderEmojiText(target.title)}」已删除`
    deleting.value = false
    deleteDialog.value?.close()

    const nextPage = posts.value.length === 1 && page.value > 1 ? page.value - 1 : page.value
    await loadPosts(nextPage)
  } catch (err) {
    deleteError.value = isApiError(err) ? err.message : '删除失败，请稍后再试'
  } finally {
    deleting.value = false
  }
}

onMounted(() => {
  void loadPosts(1)
})
</script>

<template>
  <main class="mx-auto w-full max-w-3xl p-4 sm:p-6 lg:p-8">
    <div class="mb-4 flex flex-wrap items-end justify-between gap-3">
      <div>
        <h1 class="text-2xl font-semibold lg:text-3xl">我的文章</h1>
        <p v-if="meta" class="mt-1 text-sm text-base-content/60">共 {{ meta.total }} 篇</p>
      </div>
      <router-link to="/post/new" class="btn btn-primary btn-sm">
        写文章
        <kbd class="kbd kbd-xs">N</kbd>
      </router-link>
    </div>

    <div v-if="successMsg" role="status" class="alert alert-success alert-soft mb-4 py-2 text-sm">
      <span>{{ successMsg }}</span>
    </div>

    <div v-if="loading && posts.length === 0" class="flex justify-center py-16">
      <span class="loading loading-spinner loading-lg"></span>
    </div>

    <div v-else-if="errorMsg" role="alert" class="alert alert-error alert-soft">
      <span>{{ errorMsg }}</span>
      <button type="button" class="btn btn-sm" :disabled="loading" @click="loadPosts(page)">重试</button>
    </div>

    <div v-else-if="posts.length === 0" class="py-16 text-center text-base-content/50">
      <p>还没有发布文章</p>
      <router-link to="/post/new" class="btn btn-primary btn-sm mt-4">
        写第一篇文章
        <kbd class="kbd kbd-xs">N</kbd>
      </router-link>
    </div>

    <template v-else>
      <div class="space-y-3" :class="{ 'opacity-60': loading }">
        <article v-for="post in posts" :key="post.ID" class="card bg-base-100 shadow-sm">
          <div class="card-body gap-3">
            <div class="flex flex-wrap items-start justify-between gap-3">
              <div class="min-w-0">
                <div class="flex flex-wrap items-center gap-2">
                  <h2 class="card-title break-words text-lg"><router-link :to="'/post/' + post.ID" class="hover:underline">{{ renderEmojiText(post.title) }}</router-link></h2>
                  <span v-if="post.private" class="badge badge-warning badge-sm">Private</span>
                </div>
                <p v-if="post.subtitle" class="break-words text-sm text-base-content/60">{{ renderEmojiText(post.subtitle) }}</p>
                <p class="mt-1 text-xs text-base-content/40">{{ formatDate(post.CreatedAt) }}</p>
              </div>
              <div class="flex shrink-0 items-center gap-1">
                <button v-if="auth.isAdmin" type="button" class="btn btn-ghost btn-sm"
                  :disabled="deleting || managingPostId !== null" @click="onTogglePrivate(post)">
                  {{ post.private ? '取消隐藏' : '隐藏' }}
                </button>
                <button type="button" class="btn btn-ghost btn-sm" :disabled="deleting" @click="editPost(post)">
                  编辑
                </button>
                <button type="button" class="btn btn-ghost btn-sm text-error" :disabled="deleting" @click="openDelete(post)">
                  删除
                </button>
              </div>
            </div>

            <div v-if="post.tags?.length" class="flex flex-wrap gap-1">
              <span v-for="tag in post.tags" :key="tag.ID" class="badge badge-sm" :style="tagBadgeStyle(tag)">
                #{{ tag.name }}
              </span>
            </div>
          </div>
        </article>
      </div>

      <div v-if="meta" class="mt-6 flex justify-center">
        <Pagination :page="page" :total-pages="meta.total_pages" :disabled="loading" @change="goTo" />
      </div>
    </template>

    <!-- 删除文章确认弹窗 -->
    <dialog ref="deleteDialog" class="modal" @cancel="onDeleteCancel" @close="onDeleteClosed">
      <div class="modal-box">
        <h3 class="text-lg font-semibold">删除文章</h3>

        <div class="mt-3 space-y-3 text-sm">
          <p>确定删除「{{ renderEmojiText(deletingPost?.title ?? '') }}」吗？</p>
          <p class="text-base-content/60">删除后文章将不再显示，标签使用次数会自动更新。</p>

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
  </main>
</template>





