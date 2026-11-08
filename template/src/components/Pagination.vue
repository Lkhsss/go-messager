<script setup lang="ts">
import { computed } from 'vue'

interface PageButton {
  label: string
  value: number
  ellipsis: boolean
}

const props = withDefaults(defineProps<{
  page: number
  totalPages: number
  disabled?: boolean
  siblingCount?: number
}>(), {
  disabled: false,
  siblingCount: 1,
})

const emit = defineEmits<{
  change: [page: number]
}>()

// 页数多时只显示首尾、当前页和相邻页。
const buttons = computed<PageButton[]>(() => {
  const total = props.totalPages
  if (total <= 0) return []

  const sibling = Math.max(1, props.siblingCount)
  const maxSlots = sibling * 2 + 5
  const items: PageButton[] = []

  if (total <= maxSlots) {
    for (let i = 1; i <= total; i++) {
      items.push({ label: String(i), value: i, ellipsis: false })
    }
    return items
  }

  const left = Math.max(2, props.page - sibling)
  const right = Math.min(total - 1, props.page + sibling)

  items.push({ label: '1', value: 1, ellipsis: false })
  if (left > 2) items.push({ label: '...', value: 0, ellipsis: true })
  for (let i = left; i <= right; i++) {
    items.push({ label: String(i), value: i, ellipsis: false })
  }
  if (right < total - 1) items.push({ label: '...', value: 0, ellipsis: true })
  items.push({ label: String(total), value: total, ellipsis: false })

  return items
})

function goTo(target: number) {
  if (props.disabled || target < 1 || target > props.totalPages || target === props.page) return
  emit('change', target)
}
</script>

<template>
  <div v-if="totalPages > 1" class="join">
    <button type="button" class="join-item btn" :disabled="disabled || page <= 1" aria-label="上一页"
      @click="goTo(page - 1)">«</button>

    <template v-for="(button, index) in buttons" :key="`${button.label}-${index}`">
      <button v-if="button.ellipsis" type="button" class="join-item btn btn-disabled" disabled>
        {{ button.label }}
      </button>
      <button v-else type="button" class="join-item btn" :class="{ 'btn-active': button.value === page }"
        :aria-current="button.value === page ? 'page' : undefined" :disabled="disabled"
        @click="goTo(button.value)">
        {{ button.label }}
      </button>
    </template>

    <button type="button" class="join-item btn" :disabled="disabled || page >= totalPages" aria-label="下一页"
      @click="goTo(page + 1)">»</button>
  </div>
</template>
