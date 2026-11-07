<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import UserAvatar from '@/components/UserAvatar.vue'
import { fetchLeaderboard, type LeaderboardData, type LeaderboardEntry, type LeaderboardPeriod } from '@/api/leaderboard'
import { isApiError } from '@/api/http'
import { tagBadgeStyle } from '@/utils/tagColor'

type LeaderboardType = 'like' | 'dislike' | 'question'

const periods: { value: LeaderboardPeriod; label: string }[] = [
  { value: 'day', label: '一天' },
  { value: 'week', label: '一周' },
  { value: 'month', label: '一个月' },
  { value: 'year', label: '一年' },
]

const types: { value: LeaderboardType; label: string; emoji: string }[] = [
  { value: 'like', label: '点赞', emoji: '👍' },
  { value: 'dislike', label: '点踩', emoji: '👎' },
  { value: 'question', label: '问号', emoji: '❓' },
]

const period = ref<LeaderboardPeriod>('week')
const activeType = ref<LeaderboardType>('like')
const data = ref<LeaderboardData | null>(null)
const loading = ref(false)
const errorMsg = ref('')

const entries = computed<LeaderboardEntry[]>(() => data.value?.[activeType.value] ?? [])
const activeMeta = computed(() => types.find((item) => item.value === activeType.value) ?? types[0]!)

function formatDate(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString('zh-CN')
}

async function loadLeaderboard() {
  loading.value = true
  errorMsg.value = ''
  try {
    data.value = await fetchLeaderboard(period.value)
  } catch (err) {
    errorMsg.value = isApiError(err) ? err.message : '获取排行榜失败，请稍后再试'
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  void loadLeaderboard()
})

watch(period, () => {
  void loadLeaderboard()
})

function rankClass(index: number): string {
  if (index === 0) return 'text-warning'
  if (index === 1) return 'text-base-content/70'
  if (index === 2) return 'text-amber-700'
  return 'text-base-content/35'
}
</script>

<template>
  <main class="mx-auto w-full max-w-3xl p-4 sm:p-6 lg:p-8">
    <header class="mb-5 flex flex-wrap items-end justify-between gap-3">
      <div>

        <h1 class="mt-1 text-2xl font-semibold lg:text-3xl">排行榜</h1>

      </div>
      <button type="button" class="btn btn-ghost btn-sm" :disabled="loading" @click="loadLeaderboard">刷新</button>
    </header>

    <div class="mb-5 flex flex-wrap items-center justify-between gap-3">
      <div class="join">
        <button v-for="item in periods" :key="item.value" type="button" class="btn btn-sm join-item"
          :class="{ 'btn-active': period === item.value }" :disabled="loading" @click="period = item.value">
          {{ item.label }}
        </button>
      </div>

      <div role="tablist" class="tabs tabs-box">
        <button v-for="item in types" :key="item.value" type="button" role="tab" class="tab" :aria-label="item.label" :title="item.label"
          :class="{ 'tab-active': activeType === item.value }" :aria-selected="activeType === item.value"
          @click="activeType = item.value">
          {{ item.emoji }}
        </button>
      </div>
    </div>

    <div v-if="loading && !data" class="flex justify-center py-16">
      <span class="loading loading-spinner loading-lg"></span>
    </div>

    <div v-else-if="errorMsg" role="alert" class="alert alert-error alert-soft">
      <span>{{ errorMsg }}</span>
      <button type="button" class="btn btn-sm" @click="loadLeaderboard">重试</button>
    </div>

    <template v-else-if="data">
      <p class="mb-3 text-xs text-base-content/45">
        统计范围：{{ formatDate(data.since) }} 至今 · {{ activeMeta.emoji }} Top 10
      </p>

      <ol v-if="entries.length" class="space-y-2" :class="{ 'opacity-60': loading }">
        <li v-for="(entry, index) in entries" :key="entry.post.ID">
          <router-link :to="`/post/${entry.post.ID}`"
            class="flex items-center gap-3 rounded-box border border-base-200 bg-base-100 px-3 py-3 shadow-sm transition hover:-translate-y-0.5 hover:shadow-md">
            <span class="w-7 shrink-0 text-center font-mono text-lg font-semibold" :class="rankClass(index)">
              {{ index + 1 }}
            </span>
            <UserAvatar :name="entry.post.user" :version="entry.post.user_avatar_version" :size="36" />
            <div class="min-w-0 flex-1">
              <p class="truncate font-medium">{{ entry.post.title }}</p>
              <div class="mt-1 flex flex-wrap items-center gap-2 text-xs text-base-content/45">
                <span class="truncate">{{ entry.post.user }}</span>
                <span v-if="entry.post.private" class="badge badge-warning badge-sm">Private</span>
                <span v-for="tag in entry.post.tags" :key="tag.ID" class="badge badge-sm" :style="tagBadgeStyle(tag)">
                  #{{ tag.name }}
                </span>
              </div>
            </div>
            <div class="shrink-0 text-right">
              <p class="font-mono text-lg font-semibold">{{ entry.count }}</p>
              <p class="text-[0.65rem] text-base-content/45">{{ activeMeta.emoji }}</p>
            </div>
          </router-link>
        </li>
      </ol>

      <div v-else class="py-16 text-center text-base-content/50">
        该时间段暂无数据
      </div>
    </template>
  </main>
</template>


