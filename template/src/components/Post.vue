<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { TagItem } from '@/api/tag'
import type { ReactionSummary, ReactionType } from '@/api/reaction'
import { tagBadgeStyle } from '@/utils/tagColor'
import { renderMarkdown } from '@/utils/markdown'
import UserAvatar from '@/components/UserAvatar.vue'
import ReactionBar from '@/components/ReactionBar.vue'

const props = defineProps<{
  postId: number
  user: string
  title: string
  subtitle: string
  body: string
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

const bodyEl = ref<HTMLElement | null>(null)
const expanded = ref(false)
const overflowing = ref(false)
let resizeTimer: ReturnType<typeof setTimeout> | undefined

// Markdown 渲染结果已经过 DOMPurify 消毒，可以安全用于 v-html
const renderedBody = computed(() => renderMarkdown(props.body))

function onContentLoad() {
  // 图片/字体等内容加载完成后重新测量折叠状态
  if (!expanded.value) {
    void measure()
  }
}

// 折叠状态下比较 scrollHeight 与 clientHeight：被截断才显示「展开全文」
async function measure() {
  await nextTick()

  const el = bodyEl.value
  if (!el || expanded.value) {
    return
  }

  overflowing.value = el.scrollHeight - el.clientHeight > 2
}

function toggleExpanded() {
  expanded.value = !expanded.value
  if (!expanded.value) {
    void measure()
  }
}

function onResize() {
  if (resizeTimer) clearTimeout(resizeTimer)
  resizeTimer = setTimeout(() => {
    if (!expanded.value) {
      void measure()
    }
  }, 150)
}

onMounted(() => {
  void measure()
  // 字体加载完成后高度可能变化，再量一次
  if (document.fonts) {
    void document.fonts.ready.then(() => {
      if (!expanded.value) {
        void measure()
      }
    })
  }
  window.addEventListener('resize', onResize)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', onResize)
  if (resizeTimer) clearTimeout(resizeTimer)
})

// 文章内容变化（例如列表刷新）时恢复折叠并重新测量
watch(
  () => [props.body, props.title],
  () => {
    expanded.value = false
    void measure()
  },
)
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
          <router-link v-if="!detail" :to="`/post/${postId}`" class="hover:underline">{{ title }}</router-link>
          <span v-else>{{ title }}</span>
        </h2>
        <p v-if="subtitle" class="break-words text-sm text-base-content/60">{{ subtitle }}</p>
      </div>

      <div class="relative" @load.capture="onContentLoad">
        <div ref="bodyEl" class="post-body markdown-body text-sm text-base-content/80"
          :class="{ 'post-body--clamped': !expanded && !detail }" v-html="renderedBody"></div>

        <!-- 折叠时底部渐隐，提示还有内容 -->
        <div v-if="!expanded && overflowing" class="pointer-events-none absolute inset-x-0 bottom-0 h-8"
          style="background-image: linear-gradient(to top, var(--color-base-100, #ffffff), transparent)">
        </div>
      </div>

      <button v-if="!detail && (overflowing || expanded)" type="button" class="btn btn-ghost btn-xs self-start px-1 text-primary"
        :aria-expanded="expanded" @click="toggleExpanded">
        {{ expanded ? '收起' : '展开全文' }}
        <svg aria-hidden="true" xmlns="http://www.w3.org/2000/svg" class="h-3 w-3 transition-transform"
          :class="{ 'rotate-180': expanded }" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
        </svg>
      </button>

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

<style scoped>
/* 折叠时最多显示 6 行（.markdown-body 的行高为 1.7） */
.post-body--clamped {
  max-height: calc(6 * 1.7em);
  overflow: hidden;
}
</style>


