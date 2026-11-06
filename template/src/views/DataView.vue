<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { fetchAdminStats, fetchMyStats, type AdminStats, type MyStats, type StatsUnit } from '@/api/stats'
import { isApiError } from '@/api/http'
import LineChart from '@/components/charts/LineChart.vue'
import PieChart from '@/components/charts/PieChart.vue'
import ContributionHeatmap from '@/components/charts/ContributionHeatmap.vue'

const auth = useAuthStore()

const myStats = ref<MyStats | null>(null)
const adminStats = ref<AdminStats | null>(null)
const unit = ref<StatsUnit>('day')
const loadingMy = ref(false)
const loadingAdmin = ref(false)
const errorMy = ref('')
const errorAdmin = ref('')

const units: { value: StatsUnit; label: string }[] = [
  { value: 'hour', label: '时' },
  { value: 'day', label: '天' },
  { value: 'week', label: '周' },
  { value: 'month', label: '月' },
]

async function loadMyStats() {
  loadingMy.value = true
  errorMy.value = ''
  try {
    myStats.value = await fetchMyStats()
  } catch (err) {
    errorMy.value = isApiError(err) ? err.message : '获取个人统计失败，请稍后再试'
  } finally {
    loadingMy.value = false
  }
}

async function loadAdminStats() {
  if (!auth.isAdmin) return

  loadingAdmin.value = true
  errorAdmin.value = ''
  try {
    adminStats.value = await fetchAdminStats(unit.value)
  } catch (err) {
    errorAdmin.value = isApiError(err) ? err.message : '获取全站统计失败，请稍后再试'
  } finally {
    loadingAdmin.value = false
  }
}

onMounted(() => {
  void loadMyStats()
  if (auth.isAdmin) {
    void loadAdminStats()
  }
})

watch(unit, () => {
  void loadAdminStats()
})
</script>

<template>
  <main class="mx-auto w-full max-w-5xl p-4 sm:p-6 lg:p-8">
    <header class="mb-6">
      <p class="text-[0.65rem] font-medium uppercase tracking-[0.3em] text-base-content/40">Insights</p>
      <h1 class="mt-1 text-2xl font-semibold lg:text-3xl">数据</h1>
      <p class="mt-1 text-sm text-base-content/55">文章、互动和社区活跃度统计</p>
    </header>

    <section class="space-y-4">
      <div class="flex items-end justify-between gap-3">
        <h2 class="text-lg font-semibold">我的统计</h2>
        <button type="button" class="btn btn-ghost btn-xs" :disabled="loadingMy" @click="loadMyStats">刷新</button>
      </div>

      <div v-if="loadingMy && !myStats" class="flex justify-center py-10">
        <span class="loading loading-spinner loading-lg"></span>
      </div>
      <div v-else-if="errorMy" role="alert" class="alert alert-error alert-soft">
        <span>{{ errorMy }}</span>
        <button type="button" class="btn btn-sm" @click="loadMyStats">重试</button>
      </div>
      <div v-else-if="myStats" class="grid grid-cols-2 gap-3 sm:grid-cols-4 lg:grid-cols-7">
        <div class="rounded-box border border-base-200 bg-base-100 p-3 shadow-sm">
          <p class="text-xs text-base-content/50">文章数量</p>
          <p class="mt-1 font-mono text-2xl font-semibold">{{ myStats.post_count }}</p>
        </div>
        <div class="rounded-box border border-base-200 bg-base-100 p-3 shadow-sm">
          <p class="text-xs text-base-content/50">字数</p>
          <p class="mt-1 font-mono text-2xl font-semibold">{{ myStats.word_count }}</p>
        </div>
        <div class="rounded-box border border-base-200 bg-base-100 p-3 shadow-sm">
          <p class="text-xs text-base-content/50">点赞</p>
          <p class="mt-1 font-mono text-2xl font-semibold">{{ myStats.like }}</p>
        </div>
        <div class="rounded-box border border-base-200 bg-base-100 p-3 shadow-sm">
          <p class="text-xs text-base-content/50">踩</p>
          <p class="mt-1 font-mono text-2xl font-semibold">{{ myStats.dislike }}</p>
        </div>
        <div class="rounded-box border border-base-200 bg-base-100 p-3 shadow-sm">
          <p class="text-xs text-base-content/50">疑问</p>
          <p class="mt-1 font-mono text-2xl font-semibold">{{ myStats.question }}</p>
        </div>
        <div class="rounded-box border border-base-200 bg-base-100 p-3 shadow-sm">
          <p class="text-xs text-base-content/50">我发表的评论</p>
          <p class="mt-1 font-mono text-2xl font-semibold">{{ myStats.comment_count }}</p>
        </div>
        <div class="rounded-box border border-base-200 bg-base-100 p-3 shadow-sm">
          <p class="text-xs text-base-content/50">文章收到评论</p>
          <p class="mt-1 font-mono text-2xl font-semibold">{{ myStats.received_comment_count }}</p>
        </div>
      </div>
    </section>

    <template v-if="auth.isAdmin">
      <section class="mt-8 space-y-4">
        <div class="flex flex-wrap items-end justify-between gap-3">
          <div>
            <h2 class="text-lg font-semibold">全站统计</h2>
            <p class="mt-1 text-xs text-base-content/45">时间按服务器本地时区统计</p>
          </div>
          <div class="join">
            <button v-for="item in units" :key="item.value" type="button" class="btn btn-sm join-item"
              :class="{ 'btn-active': unit === item.value }" :disabled="loadingAdmin" @click="unit = item.value">
              {{ item.label }}
            </button>
          </div>
        </div>

        <div v-if="loadingAdmin && !adminStats" class="flex justify-center py-10">
          <span class="loading loading-spinner loading-lg"></span>
        </div>
        <div v-else-if="errorAdmin" role="alert" class="alert alert-error alert-soft">
          <span>{{ errorAdmin }}</span>
          <button type="button" class="btn btn-sm" @click="loadAdminStats">重试</button>
        </div>
        <template v-else-if="adminStats">
          <div class="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
            <div class="rounded-box border border-base-200 bg-base-100 p-3 shadow-sm">
              <p class="text-xs text-base-content/50">文章数</p>
              <p class="mt-1 font-mono text-2xl font-semibold">{{ adminStats.post_count }}</p>
            </div>
            <div class="rounded-box border border-base-200 bg-base-100 p-3 shadow-sm">
              <p class="text-xs text-base-content/50">用户数</p>
              <p class="mt-1 font-mono text-2xl font-semibold">{{ adminStats.user_count }}</p>
            </div>
            <div class="rounded-box border border-base-200 bg-base-100 p-3 shadow-sm">
              <p class="text-xs text-base-content/50">评论总数</p>
              <p class="mt-1 font-mono text-2xl font-semibold">{{ adminStats.comment_count }}</p>
            </div>
            <div class="rounded-box border border-base-200 bg-base-100 p-3 shadow-sm">
              <p class="text-xs text-base-content/50">登录次数</p>
              <p class="mt-1 font-mono text-2xl font-semibold">{{ adminStats.login_count }}</p>
            </div>
            <div class="rounded-box border border-base-200 bg-base-100 p-3 shadow-sm">
              <p class="text-xs text-base-content/50">注册人数</p>
              <p class="mt-1 font-mono text-2xl font-semibold">{{ adminStats.registration_total }}</p>
              <p class="mt-1 text-[0.65rem] text-base-content/45">近 30 天 +{{ adminStats.registration_recent }}</p>
            </div>
            <div class="rounded-box border border-base-200 bg-base-100 p-3 shadow-sm">
              <p class="text-xs text-base-content/50">标签数</p>
              <p class="mt-1 font-mono text-2xl font-semibold">{{ adminStats.tag_count }}</p>
            </div>
            <div class="rounded-box border border-base-200 bg-base-100 p-3 shadow-sm">
              <p class="text-xs text-base-content/50">反应总数</p>
              <p class="mt-1 font-mono text-2xl font-semibold">{{ adminStats.reactions.total }}</p>
            </div>
            <div class="rounded-box border border-base-200 bg-base-100 p-3 shadow-sm">
              <p class="text-xs text-base-content/50">赞 / 踩 / 疑问</p>
              <p class="mt-1 font-mono text-lg font-semibold">
                {{ adminStats.reactions.like }} / {{ adminStats.reactions.dislike }} / {{ adminStats.reactions.question }}
              </p>
            </div>
          </div>

          <div class="grid gap-4 lg:grid-cols-2">
            <div class="card bg-base-100 shadow-sm">
              <div class="card-body gap-2">
                <h3 class="text-sm font-medium">文章发送趋势</h3>
                <LineChart :data="adminStats.article_series" :height="280" />
              </div>
            </div>
            <div class="card bg-base-100 shadow-sm">
              <div class="card-body gap-2">
                <h3 class="text-sm font-medium">标签使用分布</h3>
                <PieChart :data="adminStats.tag_usage" />
              </div>
            </div>
          </div>

          <div class="card bg-base-100 shadow-sm">
            <div class="card-body gap-2">
              <div class="flex flex-wrap items-center justify-between gap-2">
                <h3 class="text-sm font-medium">互动热力图</h3>
                <span class="text-xs text-base-content/45">文章 + 评论 + 反应</span>
              </div>
              <ContributionHeatmap :data="adminStats.heatmap" />
            </div>
          </div>
        </template>
      </section>
    </template>
  </main>
</template>

