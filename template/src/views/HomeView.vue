<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import Post from '@/components/Post.vue'
import Pagination from '@/components/Pagination.vue'
import TagPicker from '@/components/TagPicker.vue'
import { fetchPosts, type PostItem } from '@/api/post'
import type { TagItem } from '@/api/tag'
import { isApiError, type PaginationMeta } from '@/api/http'
import { tagBadgeStyle } from '@/utils/tagColor'

const POSTS_PER_PAGE = 10

const posts = ref<PostItem[]>([])
const meta = ref<PaginationMeta | null>(null)
const page = ref(1)
const loading = ref(false)
const errorMsg = ref('')

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

onMounted(() => {
  void loadPosts(1)
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
        <h1 class="text-2xl font-semibold lg:text-3xl">帖子</h1>
        <p v-if="meta" class="mt-1 text-sm text-base-content/60">共 {{ meta.total }} 条</p>
      </div>
      <router-link to="/post/new" class="btn btn-primary btn-sm">写文章</router-link>
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
        <Post v-for="post in posts" :key="post.ID" :user="post.user" :title="post.title" :subtitle="post.subtitle"
          :body="post.body" :user-gravatar="post.user_gravatar" :tags="post.tags" />
      </div>

      <div v-if="meta" class="mt-6 flex justify-center">
        <Pagination :page="page" :total-pages="meta.total_pages" :disabled="loading" @change="goTo" />
      </div>
    </template>
  </main>
</template>
