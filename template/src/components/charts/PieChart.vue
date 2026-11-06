<script setup lang="ts">
import { computed } from 'vue'
import type { TagUsage } from '@/api/stats'

const props = defineProps<{ data: TagUsage[] }>()

const colors = ['#3b82f6', '#8b5cf6', '#ec4899', '#f59e0b', '#10b981', '#06b6d4', '#ef4444', '#84cc16', '#64748b']

const displayData = computed<TagUsage[]>(() => {
  const sorted = [...props.data].filter((item) => item.count > 0).sort((a, b) => b.count - a.count)
  if (sorted.length <= 8) return sorted

  const top = sorted.slice(0, 8)
  const other = sorted.slice(8).reduce((sum, item) => sum + item.count, 0)
  if (other > 0) top.push({ name: '其他', count: other })
  return top
})

const total = computed(() => displayData.value.reduce((sum, item) => sum + item.count, 0))

const segments = computed(() => {
  let offset = 0
  return displayData.value.map((item, index) => {
    const percent = total.value > 0 ? (item.count / total.value) * 100 : 0
    const segment = {
      ...item,
      color: colors[index % colors.length],
      percent,
      offset,
    }
    offset += percent
    return segment
  })
})
</script>

<template>
  <div v-if="displayData.length" class="flex flex-col items-center gap-5 sm:flex-row">
    <svg viewBox="0 0 42 42" class="h-44 w-44 shrink-0 -rotate-90" role="img" aria-label="标签使用饼图">
      <circle cx="21" cy="21" r="15.915" fill="none" stroke="currentColor" stroke-opacity="0.1" stroke-width="8" />
      <circle v-for="segment in segments" :key="segment.name" cx="21" cy="21" r="15.915" fill="none"
        :stroke="segment.color" stroke-width="8" pathLength="100"
        :stroke-dasharray="`${segment.percent} ${100 - segment.percent}`" :stroke-dashoffset="-segment.offset">
        <title>{{ segment.name }}：{{ segment.count }} 次（{{ segment.percent.toFixed(1) }}%）</title>
      </circle>
    </svg>

    <ul class="grid w-full grid-cols-2 gap-x-4 gap-y-2 text-sm sm:grid-cols-1 lg:grid-cols-2">
      <li v-for="segment in segments" :key="segment.name" class="flex min-w-0 items-center gap-2">
        <span class="h-2.5 w-2.5 shrink-0 rounded-full" :style="{ backgroundColor: segment.color }"></span>
        <span class="truncate text-base-content/70">{{ segment.name }}</span>
        <span class="ml-auto shrink-0 font-mono text-xs text-base-content/45">{{ segment.count }}</span>
      </li>
    </ul>
  </div>
  <div v-else class="flex h-44 items-center justify-center text-sm text-base-content/45">暂无标签数据</div>
</template>
