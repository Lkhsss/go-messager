<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { createTag, fetchTags, type TagItem } from '@/api/tag'
import { isApiError } from '@/api/http'
import { tagBadgeStyle } from '@/utils/tagColor'

const tags = ref<TagItem[]>([])
const name = ref('')
const loading = ref(false)
const errorMsg = ref('')
const successMsg = ref('')

// 颜色可选：不勾选「自定义颜色」就用默认高级灰
const useCustomColor = ref(false)
const color = ref('#3b82f6')
const hexColorPattern = /^#?([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$/

function resetColor() {
  useCustomColor.value = false
  color.value = '#3b82f6'
}

async function loadTags() {
  try {
    tags.value = await fetchTags()
  } catch {
    // 列表加载失败不阻塞创建
  }
}

async function onSubmit() {
  if (loading.value) return

  const trimmed = name.value.trim()
  if (!trimmed) {
    errorMsg.value = '请输入标签名'
    return
  }

  const trimmedColor = useCustomColor.value ? color.value.trim() : ''
  if (trimmedColor && !hexColorPattern.test(trimmedColor)) {
    errorMsg.value = '颜色需为 #RGB 或 #RRGGBB 格式的十六进制'
    return
  }

  loading.value = true
  errorMsg.value = ''
  successMsg.value = ''
  try {
    const tag = await createTag(trimmed, trimmedColor)
    tags.value = [...tags.value, tag].sort((a, b) => a.name.localeCompare(b.name))
    name.value = ''
    successMsg.value = `标签「${tag.name}」已创建`
  } catch (err) {
    errorMsg.value = isApiError(err) ? err.message : '创建失败，请稍后再试'
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  void loadTags()
})
</script>

<template>
  <main class="mx-auto w-full max-w-3xl p-4 sm:p-6 lg:p-8">
    <div class="mb-6">
      <h1 class="text-2xl font-semibold lg:text-3xl">管理</h1>
      <p class="mt-1 text-sm text-base-content/60">仅管理员可见的管理功能</p>
    </div>

    <section class="card bg-base-100 shadow-sm">
      <div class="card-body gap-4">
        <div>
          <h2 class="card-title text-lg">标签管理</h2>
          <p class="mt-1 text-sm text-base-content/60">标签只有管理员能创建，发帖时每人最多选择 5 个。</p>
        </div>

        <form class="space-y-3" novalidate @submit.prevent="onSubmit">
          <div class="flex flex-col gap-2 sm:flex-row">
            <label class="floating-label flex-1">
              <span>标签名</span>
              <input v-model="name" type="text" name="tag" placeholder="例如：Go" maxlength="30"
                class="input input-bordered w-full" :disabled="loading" />
            </label>
            <button type="submit" class="btn btn-primary sm:self-end" :disabled="loading">
              <span v-if="loading" class="loading loading-spinner loading-sm"></span>
              创建标签
            </button>
          </div>

          <!-- 颜色可选：不勾选则使用默认高级灰 -->
          <div class="flex flex-wrap items-center gap-3 text-sm">
            <label class="flex cursor-pointer items-center gap-2">
              <input v-model="useCustomColor" type="checkbox" class="toggle toggle-sm" />
              自定义颜色
            </label>

            <template v-if="useCustomColor">
              <input v-model="color" type="color" class="h-9 w-14 cursor-pointer rounded border border-base-300 bg-base-100 p-1"
                aria-label="选择标签颜色" />
              <input v-model="color" type="text" class="input input-bordered input-sm w-28" placeholder="#6b7280"
                maxlength="7" aria-label="十六进制颜色" />
              <button type="button" class="btn btn-ghost btn-xs" @click="resetColor">使用默认灰</button>
            </template>

            <span class="inline-flex items-center gap-1 text-xs text-base-content/60">
              预览：
              <span class="badge" :style="tagBadgeStyle({ color: useCustomColor ? color : '' })">
                #{{ name.trim() || '标签' }}
              </span>
            </span>
          </div>
        </form>

        <div v-if="errorMsg" role="alert" class="alert alert-error alert-soft py-2 text-sm">
          <span>{{ errorMsg }}</span>
        </div>
        <div v-if="successMsg" role="status" class="alert alert-success alert-soft py-2 text-sm">
          <span>{{ successMsg }}</span>
        </div>

        <div>
          <h3 class="mb-2 text-sm font-medium">已有标签（{{ tags.length }}）</h3>
          <div v-if="tags.length === 0" class="text-sm text-base-content/60">还没有标签</div>
          <div v-else class="flex flex-wrap gap-2">
            <span v-for="tag in tags" :key="tag.ID" class="badge" :style="tagBadgeStyle(tag)"
              :title="`被 ${tag.usage_count} 篇文章使用`">
              #{{ tag.name }}
              <span class="ml-1 opacity-60">{{ tag.usage_count }}</span>
            </span>
          </div>
        </div>
      </div>
    </section>
  </main>
</template>
