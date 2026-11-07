<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import Post from '@/components/Post.vue'
import Pagination from '@/components/Pagination.vue'
import TagPicker from '@/components/TagPicker.vue'
import { deletePost, fetchPosts, fetchRecentPostCount, setPostPrivate, type PostItem } from '@/api/post'
import type { TagItem } from '@/api/tag'
import { toggleReaction, type ReactionType } from '@/api/reaction'
import { isApiError, type PaginationMeta } from '@/api/http'
import { tagBadgeStyle } from '@/utils/tagColor'
import { renderEmojiText } from '@/utils/markdown'
import { useAuthStore } from '@/stores/auth'

const POSTS_PER_PAGE = 10

const posts = ref<PostItem[]>([])
const meta = ref<PaginationMeta | null>(null)
const recentPostCount = ref<number | null>(null)
const page = ref(1)
const loading = ref(false)
const errorMsg = ref('')
const reactingPostId = ref<number | null>(null)

// 管理员操作
const auth = useAuthStore()
const managingPostId = ref<number | null>(null)
const deleteDialog = ref<HTMLDialogElement | null>(null)
const deletingPost = ref<PostItem | null>(null)
const deleting = ref(false)
const deleteError = ref('')

// 三态筛选：includeIds 必须包含，excludeIds 必须不包含
const includeIds = ref<number[]>([])
const excludeIds = ref<number[]>([])
// 记录参与筛选的标签对象：折叠/搜索后也能显示筛选摘要
const filterTags = ref<Record<number, TagItem>>({})

const activeFilters = computed(() => {
  const result: { tag: TagItem; kind: 'include' | 'exclude' }[] = []

  for (const id of includeIds.value) {
    const tag = filterTags.value[id]
    if (tag) result.push({ tag, kind: 'include' })
  }
  for (const id of excludeIds.value) {
    const tag = filterTags.value[id]
    if (tag) result.push({ tag, kind: 'exclude' })
  }

  return result
})

const hasFilters = computed(() => includeIds.value.length > 0 || excludeIds.value.length > 0)

function toggleTag(tag: TagItem) {
  filterTags.value = { ...filterTags.value, [tag.ID]: tag }

  const included = includeIds.value.includes(tag.ID)
  const excluded = excludeIds.value.includes(tag.ID)

  // 先清掉旧状态，再按 默认 → 包含 → 排除 → 默认 循环
  includeIds.value = includeIds.value.filter((id) => id !== tag.ID)
  excludeIds.value = excludeIds.value.filter((id) => id !== tag.ID)

  if (!included && !excluded) {
    includeIds.value = [...includeIds.value, tag.ID]
  } else if (included) {
    excludeIds.value = [...excludeIds.value, tag.ID]
  }

  void loadPosts(1)
}

function clearFilters() {
  includeIds.value = []
  excludeIds.value = []
  void loadPosts(1)
}

// 切换反应：用接口返回值局部更新这一条，不整页刷新
async function onReact(post: PostItem, type: ReactionType) {
  if (reactingPostId.value !== null) return

  reactingPostId.value = post.ID
  try {
    const result = await toggleReaction(post.ID, type)
    const target = posts.value.find((item) => item.ID === post.ID)
    if (target) {
      target.reactions = result.reactions
      target.my_reactions = result.my_reactions
    }
  } catch (err) {
    errorMsg.value = isApiError(err) ? err.message : '操作失败，请稍后再试'
  } finally {
    reactingPostId.value = null
  }
}

// 管理员隐藏/取消隐藏
async function onTogglePrivate(post: PostItem) {
  if (managingPostId.value !== null) return

  managingPostId.value = post.ID
  errorMsg.value = ''
  try {
    const updated = await setPostPrivate(post.ID, !post.private)
    const target = posts.value.find((item) => item.ID === post.ID)
    if (target) {
      target.private = updated.private
    }
  } catch (err) {
    errorMsg.value = isApiError(err) ? err.message : '更新可见性失败，请稍后再试'
  } finally {
    managingPostId.value = null
  }
}

function openDelete(post: PostItem) {
  deletingPost.value = post
  deleteError.value = ''
  errorMsg.value = ''
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
    const nextPage = posts.value.length === 1 && page.value > 1 ? page.value - 1 : page.value
    deleting.value = false
    deleteDialog.value?.close()
    await loadPosts(nextPage)
    void loadRecentPostCount()
  } catch (err) {
    deleteError.value = isApiError(err) ? err.message : '删除失败，请稍后再试'
  } finally {
    deleting.value = false
  }
}
async function loadPosts(next = page.value) {
  if (loading.value) return

  loading.value = true
  errorMsg.value = ''
  try {
    const result = await fetchPosts({
      page: next,
      perPage: POSTS_PER_PAGE,
      includeTagIds: includeIds.value,
      excludeTagIds: excludeIds.value,
    })
    posts.value = result.posts
    meta.value = result.meta
    page.value = result.meta.page > 0 ? result.meta.page : next
  } catch (err) {
    errorMsg.value = isApiError(err) ? err.message : '获取帖子失败，请稍后再试'
  } finally {
    loading.value = false
  }
}

async function loadRecentPostCount() {
  try {
    recentPostCount.value = await fetchRecentPostCount()
  } catch {
    // 保留上一次成功获取的数字；首次失败时标题显示 --。
  }
}

onMounted(() => {
  void loadPosts(1)
  void loadRecentPostCount()
})

function goTo(next: number) {
  const totalPages = meta.value?.total_pages ?? 0
  if (loading.value || next < 1 || next > totalPages || next === page.value) return
  void loadPosts(next)
}
</script>


<template>
  <main class="mx-auto w-full max-w-3xl p-4 sm:p-6 lg:max-w-5xl lg:p-8 2xl:max-w-6xl">
    <div class="mb-4 flex flex-wrap items-end justify-between gap-3">
      <div>
        <h1 class="text-xl font-semibold sm:text-2xl lg:text-3xl">
          <span class="text-rotate duration-6000">
            <span>
              <span>帖子</span>
              <span>最新: {{ recentPostCount ?? '--' }}</span>
            </span>
          </span>
        </h1>
        <p v-if="meta" class="mt-1 text-sm text-base-content/60">共 {{ meta.total }} 条</p>
      </div>
      <router-link to="/post/new" class="btn btn-primary btn-sm">
        写文章
        <kbd class="kbd kbd-xs">N</kbd>
      </router-link>
    </div>

    <!-- 标签筛选：折叠时展示一排热门标签，支持搜索；点击三态循环 -->
    <section class="mb-6 space-y-2">
      <TagPicker mode="filter" :include-ids="includeIds" :exclude-ids="excludeIds" @toggle="toggleTag" />

      <div v-if="hasFilters" class="flex flex-wrap items-center gap-2 text-xs">
        <span class="text-base-content/60">当前筛选：</span>
        <span v-for="item in activeFilters" :key="`${item.kind}-${item.tag.ID}`" class="badge badge-sm"
          :style="tagBadgeStyle(item.tag)">
          {{ item.kind === 'include' ? '必须' : '排除' }} #{{ item.tag.name }}
        </span>
        <button type="button" class="btn btn-ghost btn-xs" @click="clearFilters">清除筛选</button>
      </div>
    </section>

    <div v-if="loading && posts.length === 0" class="flex justify-center py-16">
      <span class="loading loading-spinner loading-lg"></span>
    </div>

    <div v-else-if="errorMsg" role="alert" class="alert alert-error alert-soft">
      <span>{{ errorMsg }}</span>
      <button type="button" class="btn btn-sm" :disabled="loading" @click="loadPosts(page)">
        重试
      </button>
    </div>

    <div v-else-if="posts.length === 0" class="py-16 text-center text-base-content/50">
      {{ hasFilters ? '没有符合筛选条件的帖子' : '还没有帖子' }}
    </div>

    <template v-else>
      <div class="grid grid-cols-1 gap-4 xl:grid-cols-2" :class="{ 'opacity-60': loading }">
                <Post v-for="post in posts" :key="post.ID" :post-id="post.ID" :user="post.user" :title="post.title"
          :subtitle="post.subtitle" :excerpt="post.excerpt" :user-avatar-version="post.user_avatar_version" :tags="post.tags"
          :reactions="post.reactions" :my-reactions="post.my_reactions" :reacting="reactingPostId === post.ID"
          :is-private="post.private" :can-manage="auth.isAdmin" :managing="managingPostId === post.ID"
          @react="onReact(post, $event)" @toggle-private="onTogglePrivate(post)" @delete="openDelete(post)" />
      </div>

      <div v-if="meta" class="mt-6 flex justify-center">
        <Pagination :page="page" :total-pages="meta.total_pages" :disabled="loading" @change="goTo" />
      </div>
    </template>
    <!-- 管理员删除文章确认弹窗 -->
    <dialog ref="deleteDialog" class="modal" @cancel="onDeleteCancel" @close="onDeleteClosed">
      <div class="modal-box">
        <h3 class="text-lg font-semibold">删除文章</h3>

        <div class="mt-3 space-y-3 text-sm">
          <p>确定删除「{{ renderEmojiText(deletingPost?.title ?? '') }}」吗？</p>
          <p class="text-base-content/60">删除后文章将从所有列表和详情中消失，数据保留为已删除状态。</p>

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





