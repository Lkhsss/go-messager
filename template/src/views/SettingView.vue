<script setup lang="ts">
import { computed, ref } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { isApiError } from '@/api/http'
import UserAvatar from '@/components/UserAvatar.vue'
import { EMOJI_REACTIONS, PRIMARY_REACTIONS, reactionCount } from '@/api/reaction'
import { fileToAvatarDataUrl } from '@/utils/avatar'

const auth = useAuthStore()

// 设置页展示「我获得的反应」用
const allReactions = [...PRIMARY_REACTIONS, ...EMOJI_REACTIONS]

// 本地预览（尚未保存的 data URL）
const pendingAvatar = ref('')
const fileInput = ref<HTMLInputElement | null>(null)
const saving = ref(false)
const errorMsg = ref('')
const successMsg = ref('')

const previewSrc = computed(() => pendingAvatar.value || undefined)

function pickFile() {
  fileInput.value?.click()
}

async function onFileChange(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = '' // 允许重复选择同一张图
  if (!file) return

  errorMsg.value = ''
  successMsg.value = ''
  try {
    pendingAvatar.value = await fileToAvatarDataUrl(file)
  } catch (err) {
    errorMsg.value = err instanceof Error ? err.message : '图片处理失败'
  }
}

async function saveAvatar() {
  if (saving.value || !pendingAvatar.value) return

  saving.value = true
  errorMsg.value = ''
  successMsg.value = ''
  try {
    await auth.updateAvatar(pendingAvatar.value)
    pendingAvatar.value = ''
    successMsg.value = '头像已保存'
  } catch (err) {
    errorMsg.value = isApiError(err) ? err.message : '头像保存失败，请稍后再试'
  } finally {
    saving.value = false
  }
}

async function removeAvatar() {
  if (saving.value) return

  saving.value = true
  errorMsg.value = ''
  successMsg.value = ''
  try {
    await auth.removeAvatar()
    pendingAvatar.value = ''
    successMsg.value = '头像已移除，将显示用户名首字母'
  } catch (err) {
    errorMsg.value = isApiError(err) ? err.message : '头像移除失败，请稍后再试'
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <main class="mx-auto w-full max-w-3xl p-4 sm:p-6 lg:p-8">
    <h1 class="text-2xl font-semibold lg:text-3xl">设置</h1>

    <section class="card mt-6 bg-base-100 shadow-sm">
      <div class="card-body gap-4">
        <h2 class="card-title text-lg">账号</h2>

        <div class="space-y-2 text-sm">
          <p>
            用户名：<span class="font-medium">{{ auth.user?.name ?? '-' }}</span>
          </p>
          <p class="flex items-center gap-2">
            角色：
            <span class="badge badge-sm" :class="auth.isAdmin ? 'badge-primary' : 'badge-ghost'">
              {{ auth.isAdmin ? '管理员' : '普通用户' }}
            </span>
          </p>
        </div>

        <div v-if="auth.user" class="space-y-2 text-sm">
          <p class="font-medium">我获得的反应</p>
          <div class="flex flex-wrap items-center gap-2">
            <span v-for="option in allReactions" :key="option.type" class="badge badge-sm" :title="option.label">
              {{ option.emoji }} {{ reactionCount(auth.user?.reaction_totals, option.type) }}
            </span>
          </div>
        </div>

        <p v-if="auth.isAdmin" class="text-sm text-base-content/60">
          标签管理已移到侧边栏的「管理」页面。
        </p>
      </div>
    </section>

    <section class="card mt-6 bg-base-100 shadow-sm">
      <div class="card-body gap-4">
        <h2 class="card-title text-lg">头像</h2>

        <div class="flex flex-wrap items-center gap-4">
          <UserAvatar :name="auth.user?.name ?? '?'" :version="auth.user?.avatar_version" :src="previewSrc" :size="72" />
          <div class="space-y-1 text-sm text-base-content/60">
            <p>上传的图片会压缩为 256×256，以 base64 形式保存在数据库中。</p>
            <p class="text-xs text-base-content/50">支持 JPEG/PNG，原图不超过 5MB；不设置则显示用户名首字母。</p>
          </div>
        </div>

        <input ref="fileInput" type="file" accept="image/jpeg,image/png" class="hidden" @change="onFileChange" />

        <div class="flex flex-wrap gap-2">
          <button type="button" class="btn btn-primary btn-sm" :disabled="saving" @click="pickFile">
            选择图片
          </button>
          <button type="button" class="btn btn-sm" :disabled="saving || !pendingAvatar" @click="saveAvatar">
            <span v-if="saving" class="loading loading-spinner loading-sm"></span>
            保存头像
          </button>
          <button type="button" class="btn btn-ghost btn-sm"
            :disabled="saving || (!auth.user?.avatar_version && !pendingAvatar)" @click="removeAvatar">
            移除头像
          </button>
        </div>

        <div v-if="errorMsg" role="alert" class="alert alert-error alert-soft py-2 text-sm">
          <span>{{ errorMsg }}</span>
        </div>
        <div v-if="successMsg" role="status" class="alert alert-success alert-soft py-2 text-sm">
          <span>{{ successMsg }}</span>
        </div>
      </div>
    </section>
  </main>
</template>

