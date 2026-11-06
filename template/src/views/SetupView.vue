<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useSystemStore } from '@/stores/system'
import { isApiError } from '@/api/http'

const router = useRouter()
const auth = useAuthStore()
const system = useSystemStore()

const loading = ref(false)
const errorMsg = ref('')

const form = reactive({
  name: '',
  password: '',
  confirm: '',
})

function validate(): string {
  if (!form.name.trim()) return '请输入管理员用户名'
  if (form.password.length < 6) return '密码至少 6 位'
  if (form.password !== form.confirm) return '两次输入的密码不一致'
  return ''
}

async function onSubmit() {
  if (loading.value) return
  errorMsg.value = validate()
  if (errorMsg.value) return

  loading.value = true
  try {
    await auth.signup({ name: form.name.trim(), password: form.password })
    system.markInitialized()
    await router.replace('/')
  } catch (err) {
    // 其它标签页/别人已经先初始化：刷新状态，回登录页
    if (isApiError(err) && err.code === 'USER_EXIST') {
      system.markInitialized()
      await router.replace({ name: 'login' })
      return
    }
    errorMsg.value = isApiError(err) ? err.message : '请求失败，请稍后再试'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="flex min-h-screen items-center justify-center bg-base-200 px-4">
    <div class="card w-full max-w-sm bg-base-100 shadow-xl">
      <div class="card-body gap-4">
        <div class="text-center">
          <h1 class="text-2xl font-semibold">初始化系统</h1>
          <p class="mt-1 text-sm text-base-content/60">创建第一个账号</p>
        </div>

        <div role="alert" class="alert alert-info alert-soft py-2 text-sm">
          <span>数据库里还没有用户，第一个创建的账号将成为管理员（admin）。</span>
        </div>

        <form class="flex flex-col gap-3" novalidate @submit.prevent="onSubmit">
          <label class="floating-label">
            <span>管理员用户名</span>
            <input v-model="form.name" type="text" name="name" placeholder="请输入用户名" autocomplete="username"
              class="input input-bordered w-full" :disabled="loading" />
          </label>

          <label class="floating-label">
            <span>密码</span>
            <input v-model="form.password" type="password" name="password" placeholder="请输入密码"
              autocomplete="new-password" class="input input-bordered w-full" :disabled="loading" />
          </label>

          <label class="floating-label">
            <span>确认密码</span>
            <input v-model="form.confirm" type="password" name="confirm" placeholder="请再次输入密码"
              autocomplete="new-password" class="input input-bordered w-full" :disabled="loading" />
          </label>

          <div v-if="errorMsg" role="alert" class="alert alert-error alert-soft py-2 text-sm">
            <span>{{ errorMsg }}</span>
          </div>

          <button type="submit" class="btn btn-primary w-full" :disabled="loading">
            <span v-if="loading" class="loading loading-spinner loading-sm"></span>
            创建管理员并进入
          </button>
        </form>
      </div>
    </div>
  </div>
</template>
