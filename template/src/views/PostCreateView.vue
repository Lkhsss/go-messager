<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { createPost } from '@/api/post'
import TagPicker from '@/components/TagPicker.vue'
import type { TagItem } from '@/api/tag'
import { isApiError } from '@/api/http'
import { renderMarkdown } from '@/utils/markdown'

// 与后端 maxTagsPerPost 保持一致
const MAX_TAGS = 5

const router = useRouter()

const selectedIds = ref<number[]>([])
const submitting = ref(false)
const errorMsg = ref('')

const form = reactive({
  title: '',
  subtitle: '',
  body: '',
})

// 编辑 / 预览切换；提交的仍然是 Markdown 原文
const showPreview = ref(false)
const previewHtml = computed(() => renderMarkdown(form.body))

function toggleTag(tag: TagItem) {
  if (selectedIds.value.includes(tag.ID)) {
    selectedIds.value = selectedIds.value.filter((id) => id !== tag.ID)
    errorMsg.value = ''
    return
  }
  if (selectedIds.value.length >= MAX_TAGS) {
    errorMsg.value = `最多选择 ${MAX_TAGS} 个标签`
    return
  }
  selectedIds.value = [...selectedIds.value, tag.ID]
  errorMsg.value = ''
}

function validate(): string {
  if (!form.title.trim()) return '请输入标题'
  if (!form.body.trim()) return '请输入正文'
  return ''
}

async function onSubmit() {
  if (submitting.value) return

  errorMsg.value = validate()
  if (errorMsg.value) return

  submitting.value = true
  try {
    await createPost({
      title: form.title.trim(),
      subtitle: form.subtitle.trim(),
      body: form.body,
      tag_ids: selectedIds.value,
    })
    await router.replace('/')
  } catch (err) {
    errorMsg.value = isApiError(err) ? err.message : '发布失败，请稍后再试'
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <main class="mx-auto w-full max-w-3xl p-4 sm:p-6 lg:p-8">
    <div class="mb-6 flex items-center justify-between">
      <h1 class="text-2xl font-semibold lg:text-3xl">写文章</h1>
      <router-link to="/" class="btn btn-ghost btn-sm">返回</router-link>
    </div>

    <form class="card bg-base-100 shadow-sm" novalidate @submit.prevent="onSubmit">
      <div class="card-body gap-4">
        <label class="floating-label">
          <span>标题</span>
          <input v-model="form.title" type="text" name="title" placeholder="请输入标题" maxlength="120"
            class="input input-bordered w-full" :disabled="submitting" />
        </label>

        <label class="floating-label">
          <span>副标题（可选）</span>
          <input v-model="form.subtitle" type="text" name="subtitle" placeholder="一句话摘要" maxlength="200"
            class="input input-bordered w-full" :disabled="submitting" />
        </label>

        <div class="space-y-2">
          <div class="flex items-center justify-between">
            <span class="text-sm font-medium">正文</span>
            <div class="join">
              <button type="button" class="btn btn-xs join-item" :class="{ 'btn-active': !showPreview }"
                @click="showPreview = false">编辑</button>
              <button type="button" class="btn btn-xs join-item" :class="{ 'btn-active': showPreview }"
                @click="showPreview = true">预览</button>
            </div>
          </div>

          <textarea v-if="!showPreview" v-model="form.body" name="body" rows="12" placeholder="支持 Markdown…"
            class="textarea textarea-bordered w-full" :disabled="submitting"></textarea>

          <div v-else class="markdown-body min-h-48 rounded-box border border-base-300 bg-base-100 p-3 text-sm">
            <div v-if="previewHtml" v-html="previewHtml"></div>
            <p v-else class="text-base-content/50">暂无内容</p>
          </div>

          <p class="text-xs text-base-content/50">
            支持 Markdown：标题、列表、引用、表格、链接、图片；代码块会渲染为终端窗口样式。
          </p>
        </div>

        <div class="space-y-2">
          <div class="flex items-center justify-between">
            <span class="text-sm font-medium">标签</span>
            <span class="text-xs text-base-content/60">最多 {{ MAX_TAGS }} 个</span>
          </div>

          <TagPicker mode="select" :selected-ids="selectedIds" :max-select="MAX_TAGS" @toggle="toggleTag" />
        </div>

        <div v-if="errorMsg" role="alert" class="alert alert-error alert-soft py-2 text-sm">
          <span>{{ errorMsg }}</span>
        </div>

        <div class="card-actions justify-end">
          <button type="submit" class="btn btn-primary" :disabled="submitting">
            <span v-if="submitting" class="loading loading-spinner loading-sm"></span>
            发布
          </button>
        </div>
      </div>
    </form>
  </main>
</template>
