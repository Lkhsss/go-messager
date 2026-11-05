<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { gravatarUrl } from '@/utils/gravatar'

const props = withDefaults(
  defineProps<{
    name: string
    // md5(email)；为空时显示用户名首字母
    hash?: string
    size?: number
  }>(),
  {
    hash: '',
    size: 40,
  },
)

// Gravatar 加载失败（网络不可达 / 图片异常）时回退到字母头像
const failed = ref(false)

watch(
  () => props.hash,
  () => {
    failed.value = false
  },
)

const url = computed(() => gravatarUrl(props.hash, props.size))
const initial = computed(() => (props.name.trim().slice(0, 1) || '?').toUpperCase())
const boxStyle = computed(() => ({ width: `${props.size}px`, height: `${props.size}px` }))
const textStyle = computed(() => ({ fontSize: `${Math.round(props.size * 0.42)}px` }))
</script>

<template>
  <div class="shrink-0 overflow-hidden rounded-full" :style="boxStyle">
    <img v-if="url && !failed" :src="url" :alt="`${name} 的头像`" referrerpolicy="no-referrer"
      class="h-full w-full object-cover" @error="failed = true" />
    <div v-else class="flex h-full w-full items-center justify-center bg-primary text-primary-content">
      <span class="font-semibold" :style="textStyle">{{ initial }}</span>
    </div>
  </div>
</template>
