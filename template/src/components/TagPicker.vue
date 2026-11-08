<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { fetchTags, type TagItem } from '@/api/tag'
import { tagColorStyle } from '@/utils/tagColor'

// 折叠时展示的热门标签数量
const HOT_LIMIT = 12
// 搜索一次最多返回多少条
const SEARCH_LIMIT = 100
const SEARCH_DEBOUNCE_MS = 300

const props = withDefaults(
  defineProps<{
    // filter：三态筛选（默认 → 包含 → 排除）；select：多选
    mode?: 'filter' | 'select'
    includeIds?: number[]
    excludeIds?: number[]
    selectedIds?: number[]
    maxSelect?: number
    // 编辑文章时从文章带进来的标签，保证不在热门列表里也能回显
    knownTags?: TagItem[]
  }>(),
  {
    mode: 'filter',
    includeIds: () => [],
    excludeIds: () => [],
    selectedIds: () => [],
    maxSelect: 5,
    knownTags: () => [],
  },
)

const emit = defineEmits<{ (e: 'toggle', tag: TagItem): void }>()

const hotTags = ref<TagItem[]>([])
const listTags = ref<TagItem[]>([])
const expanded = ref(false)
const keyword = ref('')
const loading = ref(false)
const errorMsg = ref('')

// 记住见过的标签：折叠或搜索后，已选中的标签仍要显示
const tagCache = new Map<number, TagItem>()
let searchTimer: ReturnType<typeof setTimeout> | undefined

function remember(tags: TagItem[]) {
  for (const tag of tags) {
    tagCache.set(tag.ID, tag)
  }
}

watch(
  () => props.knownTags,
  (tags) => remember(tags),
  { immediate: true, deep: true },
)

async function loadHot() {
  loading.value = true
  errorMsg.value = ''
  try {
    const tags = await fetchTags({ sort: 'hot', limit: HOT_LIMIT + 1 })
    remember(tags)
    hotTags.value = tags
  } catch {
    errorMsg.value = '标签加载失败'
    hotTags.value = []
  } finally {
    loading.value = false
  }
}

async function loadList() {
  loading.value = true
  errorMsg.value = ''
  try {
    const term = keyword.value.trim()
    const tags = term
      ? await fetchTags({ q: term, sort: 'name', limit: SEARCH_LIMIT })
      : await fetchTags({ sort: 'name' })
    remember(tags)
    listTags.value = tags
  } catch {
    errorMsg.value = '标签加载失败'
    listTags.value = []
  } finally {
    loading.value = false
  }
}

const hotOnly = computed(() => hotTags.value.slice(0, HOT_LIMIT))
const hasMore = computed(() => hotTags.value.length > HOT_LIMIT)
const sourceTags = computed(() => (expanded.value || keyword.value.trim() ? listTags.value : hotOnly.value))

// 已选标签即使不在热门列表也显示。
const activeIds = computed(() => [...props.includeIds, ...props.excludeIds, ...props.selectedIds])

const displayTags = computed(() => {
  const shown = new Set(sourceTags.value.map((tag) => tag.ID))
  const extras = activeIds.value
    .filter((id) => !shown.has(id))
    .map((id) => tagCache.get(id))
    .filter((tag): tag is TagItem => Boolean(tag))

  return [...sourceTags.value, ...extras]
})

const maxReached = computed(() => props.mode === 'select' && props.selectedIds.length >= props.maxSelect)
const isOpen = computed(() => expanded.value || Boolean(keyword.value.trim()))

type ChipState = 'neutral' | 'include' | 'exclude' | 'selected'

function stateOf(tag: TagItem): ChipState {
  if (props.mode === 'select') {
    return props.selectedIds.includes(tag.ID) ? 'selected' : 'neutral'
  }
  if (props.includeIds.includes(tag.ID)) {
    return 'include'
  }
  if (props.excludeIds.includes(tag.ID)) {
    return 'exclude'
  }
  return 'neutral'
}

function isDisabled(tag: TagItem): boolean {
  return maxReached.value && stateOf(tag) !== 'selected'
}

function stateLabel(tag: TagItem): string {
  const state = stateOf(tag)
  if (state === 'include') return '必须包含'
  if (state === 'selected') return '已选择'
  if (state === 'exclude') return '必须排除'
  return '未筛选'
}

function onToggle(tag: TagItem) {
  if (isDisabled(tag)) return
  emit('toggle', tag)
}

function onKeywordInput() {
  expanded.value = true
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(() => {
    void loadList()
  }, SEARCH_DEBOUNCE_MS)
}

function clearKeyword() {
  keyword.value = ''
  if (searchTimer) clearTimeout(searchTimer)
  if (expanded.value) {
    void loadList()
  } else {
    void loadHot()
  }
}

function toggleExpanded() {
  if (isOpen.value) {
    // 收起时一并清空搜索，回到热门一排
    keyword.value = ''
    expanded.value = false
    if (searchTimer) clearTimeout(searchTimer)
    void loadHot()
    return
  }

  expanded.value = true
  void loadList()
}

onMounted(() => {
  void loadHot()
})

onBeforeUnmount(() => {
  if (searchTimer) clearTimeout(searchTimer)
})
</script>

<template>
  <div class="space-y-2">
    <div class="flex flex-wrap items-center gap-2">
      <label class="input input-sm input-bordered flex w-full items-center gap-2 sm:w-56">
        <svg aria-hidden="true" xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 opacity-60" fill="none"
          viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
            d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
        </svg>
        <input v-model="keyword" type="search" class="grow" placeholder="搜索标签"
          @input="onKeywordInput" @keydown.enter.prevent />
        <kbd class="kbd kbd-xs shrink-0">S</kbd>
        <button v-if="keyword" type="button" class="btn btn-ghost btn-xs" aria-label="清空搜索" @click="clearKeyword">
          ✕
        </button>
      </label>

      <button v-if="hasMore || isOpen" type="button" class="btn btn-ghost btn-xs" @click="toggleExpanded">
        {{ isOpen ? '收起' : '展开全部' }}
      </button>

      <span class="text-xs text-base-content/50">
        {{ mode === 'select' ? `已选 ${selectedIds.length}/${maxSelect}` : '点击=包含，再点=排除，再点=默认' }}
      </span>
    </div>

    <div v-if="errorMsg" class="text-xs text-error">{{ errorMsg }}</div>

    <div v-else-if="loading && displayTags.length === 0" class="flex items-center gap-2 text-xs text-base-content/60">
      <span class="loading loading-spinner loading-xs"></span>
      正在加载标签…
    </div>

    <div v-else-if="displayTags.length === 0" class="text-xs text-base-content/60">
      {{ keyword ? '没有匹配的标签' : '还没有标签' }}
    </div>

    <!-- 折叠时单行横向滚动，展开/搜索时换行 -->
    <div v-else class="tag-strip" :class="{ 'tag-strip--wrap': isOpen }">
      <button v-for="tag in displayTags" :key="tag.ID" type="button" class="tag-chip"
        :class="[`tag-chip--${stateOf(tag)}`, { 'tag-chip--disabled': isDisabled(tag) }]" :style="tagColorStyle(tag)"
        :disabled="isDisabled(tag)" :aria-label="`标签 ${tag.name}：${stateLabel(tag)}`"
        :title="`${tag.name} · 被 ${tag.usage_count} 篇文章使用`" @click="onToggle(tag)">
        #{{ tag.name }}
        <span v-if="isOpen" class="tag-chip__count">{{ tag.usage_count }}</span>
      </button>
    </div>
  </div>
</template>

<style scoped>
.tag-strip {
  display: flex;
  gap: 0.5rem;
  overflow-x: auto;
  padding-bottom: 0.25rem;
}

.tag-strip--wrap {
  flex-wrap: wrap;
  overflow-x: visible;
}

.tag-strip::-webkit-scrollbar {
  height: 4px;
}

.tag-strip::-webkit-scrollbar-thumb {
  background: color-mix(in srgb, currentColor 20%, transparent);
  border-radius: 9999px;
}

.tag-chip {
  flex: none;
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  padding: 0.125rem 0.5rem;
  border-radius: 0.5rem;
  border: 1px solid color-mix(in srgb, var(--tag-color) 35%, transparent);
  background: color-mix(in srgb, var(--tag-color) 12%, transparent);
  color: var(--tag-color);
  font-size: 0.75rem;
  line-height: 1.5rem;
  opacity: 0.8;
  transition: opacity 0.15s, background-color 0.15s, box-shadow 0.15s;
}

.tag-chip:hover {
  opacity: 1;
}

/* 激活（包含 / 已选择）：变亮 + 加粗 + 同色描边 */
.tag-chip--include,
.tag-chip--selected {
  opacity: 1;
  font-weight: 700;
  background: color-mix(in srgb, var(--tag-color) 26%, transparent);
  border-color: var(--tag-color);
  box-shadow: 0 0 0 1px var(--tag-color);
}

/* 否定（排除）：删除线 + 加粗 + 去饱和颜色 + 虚线边框 */
.tag-chip--exclude {
  opacity: 1;
  font-weight: 700;
  text-decoration: line-through;
  color: color-mix(in srgb, var(--tag-color) 45%, #9ca3af);
  background: transparent;
  border-style: dashed;
  border-color: color-mix(in srgb, var(--tag-color) 40%, transparent);
}

.tag-chip--disabled {
  opacity: 0.35;
  cursor: not-allowed;
}

.tag-chip__count {
  font-size: 0.625rem;
  opacity: 0.7;
}
</style>



