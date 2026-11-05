<script setup lang="ts">
import { ref, watch } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { isApiError } from '@/api/http'
import UserAvatar from '@/components/UserAvatar.vue'
import { EMOJI_REACTIONS, PRIMARY_REACTIONS, reactionCount } from '@/api/reaction'

const auth = useAuthStore()

// 设置页展示「我获得的反应」用
const allReactions = [...PRIMARY_REACTIONS, ...EMOJI_REACTIONS]

const email = ref(auth.user?.email ?? '')
const saving = ref(false)
const errorMsg = ref('')
const successMsg = ref('')

// 登录态从别处刷新时同步输入框
watch(
  () => auth.user?.email,
  (next) => {
    email.value = next ?? ''
  },
)

async function saveEmail() {
  if (saving.value) return

  saving.value = true
  errorMsg.value = ''
  successMsg.value = ''
  try {
    await auth.updateProfile(email.value.trim())
    successMsg.value = email.value.trim()
      ? '邮箱已保存，头像已更新'
      : '已清除邮箱，将显示用户名首字母头像'
  } catch (err) {
    errorMsg.value = isApiError(err) ? err.message : '保存失败，请稍后再试'
  } finally {
    saving.value = false
  }
}

async function clearEmail() {
  email.value = ''
  await saveEmail()
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
        <h2 class="card-title text-lg">头像（Gravatar）</h2>

        <div class="flex items-center gap-4">
          <UserAvatar :name="auth.user?.name ?? '?'" :hash="auth.user?.gravatar_hash" :size="72" />
          <p class="text-sm text-base-content/60">
            头像直接来自 Gravatar，网站不保存图片。填写邮箱后自动获取；留空则显示用户名首字母。
          </p>
        </div>

        <form class="flex flex-col gap-2 sm:flex-row" novalidate @submit.prevent="saveEmail">
          <label class="floating-label flex-1">
            <span>Gravatar 邮箱</span>
            <input v-model="email" type="email" name="email" placeholder="you@example.com" autocomplete="email"
              class="input input-bordered w-full" :disabled="saving" />
          </label>
          <button type="submit" class="btn btn-primary sm:self-end" :disabled="saving">
            <span v-if="saving" class="loading loading-spinner loading-sm"></span>
            保存
          </button>
          <button type="button" class="btn btn-ghost sm:self-end" :disabled="saving" @click="clearEmail">
            清除
          </button>
        </form>

        <p class="text-xs text-base-content/50">
          邮箱仅用于生成头像，不会公开展示；文章列表只使用邮箱的 MD5 哈希向 gravatar.com 请求图片。
        </p>

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

