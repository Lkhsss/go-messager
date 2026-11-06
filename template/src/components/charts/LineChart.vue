<script setup lang="ts">
import { computed } from 'vue'
import type { TimePoint } from '@/api/stats'

const props = withDefaults(defineProps<{ data: TimePoint[]; height?: number }>(), {
  height: 260,
})

const W = 800
const H = 280
const padLeft = 44
const padRight = 16
const padTop = 18
const padBottom = 32

const maxCount = computed(() => Math.max(1, ...props.data.map((point) => point.count)))
const plotWidth = W - padLeft - padRight
const plotHeight = H - padTop - padBottom

function x(index: number): number {
  if (props.data.length <= 1) return padLeft + plotWidth / 2
  return padLeft + (index / (props.data.length - 1)) * plotWidth
}

function y(count: number): number {
  return padTop + plotHeight - (count / maxCount.value) * plotHeight
}

const points = computed(() => props.data.map((point, index) => ({
  ...point,
  x: x(index),
  y: y(point.count),
})))

const polyline = computed(() => points.value.map((point) => `${point.x},${point.y}`).join(' '))

const labels = computed(() => {
  if (props.data.length === 0) return []
  const step = Math.max(1, Math.ceil(props.data.length / 8))
  return props.data
    .map((point, index) => ({ ...point, index }))
    .filter((point) => point.index % step === 0 || point.index === props.data.length - 1)
})

const gridLines = computed(() => [0, 0.25, 0.5, 0.75, 1].map((ratio) => ({
  y: padTop + plotHeight * ratio,
  value: Math.round(maxCount.value * (1 - ratio)),
})))
</script>

<template>
  <div class="w-full" :style="{ height: `${height}px` }">
    <svg v-if="data.length" :viewBox="`0 0 ${W} ${H}`" class="h-full w-full" role="img" aria-label="文章发送折线统计图">
      <g v-for="line in gridLines" :key="line.y">
        <line :x1="padLeft" :x2="W - padRight" :y1="line.y" :y2="line.y" stroke="currentColor"
          stroke-opacity="0.12" stroke-width="1" />
        <text :x="padLeft - 8" :y="line.y + 4" text-anchor="end" fill="currentColor" opacity="0.45"
          font-size="11">{{ line.value }}</text>
      </g>

      <polyline :points="polyline" fill="none" stroke="var(--color-primary, #3b82f6)" stroke-width="2.5"
        stroke-linejoin="round" stroke-linecap="round" />

      <g v-for="point in points" :key="`${point.label}-${point.x}`">
        <circle :cx="point.x" :cy="point.y" r="3.5" fill="var(--color-base-100, #fff)"
          stroke="var(--color-primary, #3b82f6)" stroke-width="2">
          <title>{{ point.label }}：{{ point.count }} 篇</title>
        </circle>
      </g>

      <g v-for="label in labels" :key="`label-${label.index}`">
        <text :x="x(label.index)" :y="H - 8" text-anchor="middle" fill="currentColor" opacity="0.55"
          font-size="11">{{ label.label }}</text>
      </g>
    </svg>
    <div v-else class="flex h-full items-center justify-center text-sm text-base-content/45">暂无数据</div>
  </div>
</template>
