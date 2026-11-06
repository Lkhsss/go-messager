<script setup lang="ts">
import { computed } from 'vue'
import type { HeatmapPoint } from '@/api/stats'

const props = defineProps<{ data: HeatmapPoint[] }>()

function parseDate(value: string): Date {
  return new Date(`${value}T00:00:00`)
}

function formatDate(date: Date): string {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

const countByDate = computed(() => new Map(props.data.map((item) => [item.date, item.count])))
const maxCount = computed(() => Math.max(1, ...props.data.map((item) => item.count)))

const cells = computed(() => {
  if (props.data.length === 0) return []

  const sorted = [...props.data].sort((a, b) => a.date.localeCompare(b.date))
  const first = parseDate(sorted[0]!.date)
  const last = parseDate(sorted[sorted.length - 1]!.date)
  const cursor = new Date(first)
  cursor.setDate(cursor.getDate() - cursor.getDay())

  const result: { date: string; count: number; level: string }[] = []
  while (cursor <= last) {
    const key = formatDate(cursor)
    const count = countByDate.value.get(key) ?? 0
    result.push({ date: key, count, level: levelClass(count) })
    cursor.setDate(cursor.getDate() + 1)
  }
  return result
})

function levelClass(count: number): string {
  if (count <= 0) return 'bg-base-200'
  const ratio = count / maxCount.value
  if (ratio <= 0.25) return 'bg-success/25'
  if (ratio <= 0.5) return 'bg-success/45'
  if (ratio <= 0.75) return 'bg-success/65'
  return 'bg-success'
}

const total = computed(() => props.data.reduce((sum, item) => sum + item.count, 0))
</script>

<template>
  <div>
    <div v-if="cells.length" class="overflow-x-auto pb-2">
      <div class="inline-grid gap-1" style="grid-template-rows: repeat(7, 0.75rem); grid-auto-flow: column; grid-auto-columns: 0.75rem">
        <span v-for="cell in cells" :key="cell.date" class="h-3 w-3 rounded-[2px]" :class="cell.level"
          :title="`${cell.date}：${cell.count} 次互动`"></span>
      </div>
    </div>
    <div v-else class="flex h-24 items-center justify-center text-sm text-base-content/45">暂无互动数据</div>

    <div class="mt-2 flex items-center justify-between gap-3 text-xs text-base-content/45">
      <span>过去一年共 {{ total }} 次互动</span>
      <span class="flex items-center gap-1">
        少
        <i class="h-3 w-3 rounded-[2px] bg-base-200"></i>
        <i class="h-3 w-3 rounded-[2px] bg-success/25"></i>
        <i class="h-3 w-3 rounded-[2px] bg-success/45"></i>
        <i class="h-3 w-3 rounded-[2px] bg-success/65"></i>
        <i class="h-3 w-3 rounded-[2px] bg-success"></i>
        多
      </span>
    </div>
  </div>
</template>

