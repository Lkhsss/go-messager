<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { getUptime } from '@/api/system'

interface Elapsed {
  years: number
  months: number
  days: number
  hours: number
  minutes: number
  seconds: number
}

const startedAtMs = ref(0)
const nowMs = ref(Date.now())
const loadFailed = ref(false)

let tickTimer: number | undefined
let syncTimer: number | undefined

const loaded = computed(() => startedAtMs.value > 0)
const elapsed = computed<Elapsed>(() => {
  if (!loaded.value) return { years: 0, months: 0, days: 0, hours: 0, minutes: 0, seconds: 0 }
  return elapsedYMDHMS(new Date(startedAtMs.value), new Date(nowMs.value))
})

function addYears(date: Date, years: number): Date {
  const result = new Date(date)
  result.setFullYear(result.getFullYear() + years)
  return result
}

function addMonths(date: Date, months: number): Date {
  const result = new Date(date)
  const day = result.getDate()
  result.setDate(1)
  result.setMonth(result.getMonth() + months)
  const lastDay = new Date(result.getFullYear(), result.getMonth() + 1, 0).getDate()
  result.setDate(Math.min(day, lastDay))
  return result
}

// 精确计算年月日时分秒，避免按 365/30 估算。
function elapsedYMDHMS(start: Date, end: Date): Elapsed {
  if (Number.isNaN(start.getTime()) || end <= start) {
    return { years: 0, months: 0, days: 0, hours: 0, minutes: 0, seconds: 0 }
  }

  let years = end.getFullYear() - start.getFullYear()
  let cursor = addYears(start, years)
  while (cursor > end) {
    years -= 1
    cursor = addYears(start, years)
  }

  let months = 0
  while (months < 11) {
    const next = addMonths(cursor, months + 1)
    if (next > end) break
    months += 1
  }
  cursor = addMonths(cursor, months)

  const dayMs = 86_400_000
  const hourMs = 3_600_000
  const minuteMs = 60_000
  const remaining = Math.max(0, end.getTime() - cursor.getTime())
  const days = Math.floor(remaining / dayMs)
  const hours = Math.floor((remaining % dayMs) / hourMs)
  const minutes = Math.floor((remaining % hourMs) / minuteMs)
  const seconds = Math.floor((remaining % minuteMs) / 1000)

  return { years: Math.max(0, years), months, days, hours, minutes, seconds }
}

// daisyUI countdown 需要 --value 和 --digits。
function countdownStyle(value: number): Record<string, string> {
  return {
    '--value': String(value),
    '--digits': String(Math.max(1, String(value).length)),
  }
}

async function loadUptime() {
  try {
    const data = await getUptime()
    const parsed = data.started_at ? Date.parse(data.started_at) : Number.NaN
    const fallback = Date.now() - Math.max(0, data.uptime_seconds) * 1000
    startedAtMs.value = Number.isNaN(parsed) || parsed <= 0 ? fallback : parsed
    nowMs.value = Date.now()
    loadFailed.value = false
  } catch {
    // 失败时保留上一次成功的数据；首次加载失败则显示占位符。
    loadFailed.value = true
  }
}

onMounted(() => {
  void loadUptime()
  tickTimer = window.setInterval(() => {
    nowMs.value = Date.now()
  }, 1000)
  // 定期校准，避免后端重启后前端一直使用旧的启动时刻。
  syncTimer = window.setInterval(() => {
    void loadUptime()
  }, 60_000)
})

onUnmounted(() => {
  if (tickTimer !== undefined) window.clearInterval(tickTimer)
  if (syncTimer !== undefined) window.clearInterval(syncTimer)
})
</script>

<template>
  <footer class="footer footer-center border-t border-base-300 bg-base-200 px-4 py-3 text-xs text-base-content/55">
    <div v-if="loaded" class="flex flex-wrap items-center justify-center gap-x-1" :title="loadFailed ? '暂时无法刷新运行时长' : undefined">
      <span class="sr-only">
        本站已运行：{{ elapsed.years }}年/{{ elapsed.months }}月/{{ elapsed.days }}日/{{ elapsed.hours }}时/{{ elapsed.minutes }}分/{{ elapsed.seconds }}秒
      </span>

      <span aria-hidden="true">本站已运行：</span>
      <span aria-hidden="true" class="countdown font-mono text-xs">
        <span :style="countdownStyle(elapsed.years)">{{ elapsed.years }}</span>
      </span>
      <span aria-hidden="true">年</span>
      <span aria-hidden="true">/</span>
      <span aria-hidden="true" class="countdown font-mono text-xs">
        <span :style="countdownStyle(elapsed.months)">{{ elapsed.months }}</span>
      </span>
      <span aria-hidden="true">月</span>
      <span aria-hidden="true">/</span>
      <span aria-hidden="true" class="countdown font-mono text-xs">
        <span :style="countdownStyle(elapsed.days)">{{ elapsed.days }}</span>
      </span>
      <span aria-hidden="true">日</span>
      <span aria-hidden="true">/</span>
      <span aria-hidden="true" class="countdown font-mono text-xs">
        <span :style="countdownStyle(elapsed.hours)">{{ elapsed.hours }}</span>
      </span>
      <span aria-hidden="true">时</span>
      <span aria-hidden="true">/</span>
      <span aria-hidden="true" class="countdown font-mono text-xs">
        <span :style="countdownStyle(elapsed.minutes)">{{ elapsed.minutes }}</span>
      </span>
      <span aria-hidden="true">分</span>
      <span aria-hidden="true">/</span>
      <span aria-hidden="true" class="countdown font-mono text-xs">
        <span :style="countdownStyle(elapsed.seconds)">{{ elapsed.seconds }}</span>
      </span>
      <span aria-hidden="true">秒</span>
    </div>
    <div v-else class="text-base-content/40">
      本站已运行：--年/--月/--日/--时/--分/--秒
    </div>

    <!-- 备案号位置：备案通过后，将下面注释替换为实际备案链接。 -->
    <!--
      <a class="link link-hover" href="https://beian.miit.gov.cn/" target="_blank" rel="noopener noreferrer">
        京ICP备XXXXXXXX号
      </a>
    -->
  </footer>
</template>
