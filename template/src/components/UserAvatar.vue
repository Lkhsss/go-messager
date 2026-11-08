<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { avatarUrl } from '@/utils/avatar'

const props = withDefaults(
  defineProps<{
    name: string
    // 头像版本号（来自 /api/me 或文章作者），为空时只显示用户名首字母
    version?: string
    // 本地预览（data URL），优先级高于 version
    src?: string
    size?: number
  }>(),
  {
    version: '',
    src: '',
    size: 40,
  },
)

// 已经加载成功的 URL 缓存：列表翻页/多个相同作者头像时不再闪一下占位
const loadedUrls = new Set<string>()

const loaded = ref(false)
let loadToken = 0

const url = computed(() => props.src || avatarUrl(props.name, props.version))
const initial = computed(() => (props.name.trim().slice(0, 1) || '?').toUpperCase())
const boxStyle = computed(() => ({ width: `${props.size}px`, height: `${props.size}px` }))
const textStyle = computed(() => ({ fontSize: `${Math.round(props.size * 0.42)}px` }))

function onImageError() {
  loaded.value = false
  // 从缓存移除，下次仍会重新尝试加载
  if (url.value) {
    loadedUrls.delete(url.value)
  }
}

// 先用字母占位，等头像真正加载完成再显示图片；
// 加载失败（网络/图片异常）就保持字母头像，避免出现空白或破图。
watch(
  url,
  (next) => {
    loaded.value = false
    loadToken++

    if (!next) {
      return
    }
    if (loadedUrls.has(next)) {
      loaded.value = true
      return
    }

    const token = loadToken
    const img = new Image()
    img.referrerPolicy = 'no-referrer'
    img.onload = () => {
      if (token !== loadToken) return
      loadedUrls.add(next)
      loaded.value = true
    }
    img.onerror = () => {
      if (token !== loadToken) return
      loaded.value = false
    }
    img.src = next
  },
  { immediate: true },
)
</script>

<template>
  <div class="relative shrink-0 overflow-hidden rounded-full" :style="boxStyle">
    <!-- 字母占位，图片加载后覆盖 -->
    <div class="absolute inset-0 flex items-center justify-center bg-primary text-primary-content">
      <span class="font-semibold" :style="textStyle">{{ initial }}</span>
    </div>

    <Transition name="avatar-fade">
      <img v-if="url && loaded" :src="url" :alt="`${name} 的头像`" referrerpolicy="no-referrer"
        class="absolute inset-0 h-full w-full object-cover" @error="onImageError" />
    </Transition>
  </div>
</template>

<style scoped>
.avatar-fade-enter-active {
  transition: opacity 0.2s ease;
}

.avatar-fade-enter-from {
  opacity: 0;
}
</style>

