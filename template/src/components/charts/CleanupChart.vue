<script setup lang="ts">
import { computed } from 'vue'
import type { CleanupCategory } from '@/api/adminCleanup'
import { formatBytes } from '@/utils/format'

const props = defineProps<{ categories: CleanupCategory[] }>()

const colors = ['#3b82f6', '#8b5cf6', '#ec4899', '#f59e0b', '#10b981', '#06b6d4']

function colorFor(index: number): string {
  return colors[index % colors.length] ?? '#3b82f6'
}

// 按分类顺序固定颜色，两个饼图同一分类颜色一致。
const colorByKey = computed(() => {
  const map = new Map<string, string>()
  props.categories.forEach((category, index) => {
    map.set(category.key, colorFor(index))
  })
  return map
})

const countSlices = computed(() =>
  buildSlices(
    props.categories,
    (category) => category.count,
    (category) => `${category.count} 项`,
  ),
)

const bytesSlices = computed(() =>
  buildSlices(
    props.categories,
    (category) => category.bytes,
    (category) => formatBytes(category.bytes),
  ),
)

const countTotal = computed(() => props.categories.reduce((sum, category) => sum + category.count, 0))
const bytesTotal = computed(() => props.categories.reduce((sum, category) => sum + category.bytes, 0))

function buildSlices(
  categories: CleanupCategory[],
  valueOf: (category: CleanupCategory) => number,
  detailOf: (category: CleanupCategory) => string,
) {
  const visible = categories.filter((category) => valueOf(category) > 0)
  const total = visible.reduce((sum, category) => sum + valueOf(category), 0)

  let offset = 0
  return visible.map((category) => {
    const percent = total > 0 ? (valueOf(category) / total) * 100 : 0
    const slice = {
      key: category.key,
      label: category.label,
      color: colorByKey.value.get(category.key) ?? '#3b82f6',
      detail: detailOf(category),
      percent,
      offset,
    }
    offset += percent
    return slice
  })
}

function formatPercent(percent: number): string {
  return percent >= 10 ? percent.toFixed(0) : percent.toFixed(1)
}
</script>
<template>
  <div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
    <!-- 条目数分布 -->
    <section class="space-y-3">
      <div class="flex items-baseline justify-between gap-2">
        <h4 class="text-sm font-medium">条目数分布</h4>
        <span class="text-xs text-base-content/45">共 {{ countTotal }} 项</span>
      </div>

      <div v-if="countSlices.length" class="flex flex-col items-center gap-4 sm:flex-row">
        <div class="relative h-40 w-40 shrink-0">
          <svg viewBox="0 0 42 42" class="h-full w-full -rotate-90" role="img" aria-label="可清理条目数饼图">
            <circle cx="21" cy="21" r="15.915" fill="none" stroke="currentColor" stroke-opacity="0.1"
              stroke-width="8" />
            <circle v-for="slice in countSlices" :key="slice.key" cx="21" cy="21" r="15.915" fill="none"
              :stroke="slice.color" stroke-width="8" pathLength="100"
              :stroke-dasharray="`${slice.percent} ${100 - slice.percent}`" :stroke-dashoffset="-slice.offset">
              <title>{{ slice.label }}：{{ slice.detail }}（{{ formatPercent(slice.percent) }}%）</title>
            </circle>
          </svg>
          <div class="pointer-events-none absolute inset-0 flex flex-col items-center justify-center">
            <span class="text-lg font-semibold">{{ countTotal }}</span>
            <span class="text-xs text-base-content/50">项</span>
          </div>
        </div>

        <ul class="w-full space-y-1.5 text-sm">
          <li v-for="slice in countSlices" :key="slice.key" class="flex min-w-0 items-center gap-2">
            <span class="h-2.5 w-2.5 shrink-0 rounded-full" :style="{ backgroundColor: slice.color }"></span>
            <span class="truncate text-base-content/70">{{ slice.label }}</span>
            <span class="ml-auto shrink-0 font-mono text-xs text-base-content/45">
              {{ slice.detail }}（{{ formatPercent(slice.percent) }}%）
            </span>
          </li>
        </ul>
      </div>
      <div v-else class="flex h-40 items-center justify-center text-sm text-base-content/45">暂无可清理数据</div>
    </section>

    <!-- 空间占用分布 -->
    <section class="space-y-3">
      <div class="flex items-baseline justify-between gap-2">
        <h4 class="text-sm font-medium">空间占用分布（估算）</h4>
        <span class="text-xs text-base-content/45">约 {{ formatBytes(bytesTotal) }}</span>
      </div>

      <div v-if="bytesSlices.length" class="flex flex-col items-center gap-4 sm:flex-row">
        <div class="relative h-40 w-40 shrink-0">
          <svg viewBox="0 0 42 42" class="h-full w-full -rotate-90" role="img" aria-label="可清理数据空间占用饼图">
            <circle cx="21" cy="21" r="15.915" fill="none" stroke="currentColor" stroke-opacity="0.1"
              stroke-width="8" />
            <circle v-for="slice in bytesSlices" :key="slice.key" cx="21" cy="21" r="15.915" fill="none"
              :stroke="slice.color" stroke-width="8" pathLength="100"
              :stroke-dasharray="`${slice.percent} ${100 - slice.percent}`" :stroke-dashoffset="-slice.offset">
              <title>{{ slice.label }}：{{ slice.detail }}（{{ formatPercent(slice.percent) }}%）</title>
            </circle>
          </svg>
          <div class="pointer-events-none absolute inset-0 flex flex-col items-center justify-center">
            <span class="text-sm font-semibold">{{ formatBytes(bytesTotal) }}</span>
            <span class="text-xs text-base-content/50">估算</span>
          </div>
        </div>

        <ul class="w-full space-y-1.5 text-sm">
          <li v-for="slice in bytesSlices" :key="slice.key" class="flex min-w-0 items-center gap-2">
            <span class="h-2.5 w-2.5 shrink-0 rounded-full" :style="{ backgroundColor: slice.color }"></span>
            <span class="truncate text-base-content/70">{{ slice.label }}</span>
            <span class="ml-auto shrink-0 font-mono text-xs text-base-content/45">
              {{ slice.detail }}（{{ formatPercent(slice.percent) }}%）
            </span>
          </li>
        </ul>
      </div>
      <div v-else class="flex h-40 items-center justify-center text-sm text-base-content/45">暂无可清理数据</div>
    </section>
  </div>
</template>
