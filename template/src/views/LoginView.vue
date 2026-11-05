<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { isApiError } from '@/api/http'

type Mode = 'login' | 'register'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

const mode = ref<Mode>('login')
const loading = ref(false)
const errorMsg = ref('')

const form = reactive({
  name: '',
  password: '',
  confirm: '',
  email: '',
})

const isRegister = computed(() => mode.value === 'register')
const title = computed(() => (isRegister.value ? '注册账号' : '登录'))
const submitText = computed(() => (isRegister.value ? '注册' : '登录'))

function switchMode(next: Mode) {
  if (mode.value === next) return
  mode.value = next
  form.password = ''
  form.confirm = ''
  errorMsg.value = ''
}

function validate(): string {
  const name = form.name.trim()
  if (!name) return '请输入用户名'
  if (!form.password) return '请输入密码'
  if (isRegister.value) {
    if (form.password.length < 6) return '密码至少 6 位'
    if (form.password !== form.confirm) return '两次输入的密码不一致'
    const email = form.email.trim()
    if (email && !email.includes('@')) return '邮箱格式不正确'
  }
  return ''
}

async function onSubmit() {
  if (loading.value) return
  errorMsg.value = validate()
  if (errorMsg.value) return

  loading.value = true
  try {
    const credentials = { name: form.name.trim(), password: form.password }
    if (isRegister.value) {
      const email = form.email.trim()
      await auth.signup({ ...credentials, ...(email ? { email } : {}) })
    } else {
      await auth.login(credentials)
    }

    // 只接受站内路径，避免开放重定向
    const redirect = route.query.redirect
    const target = typeof redirect === 'string' && redirect.startsWith('/') && !redirect.startsWith('//')
      ? redirect
      : '/'
    await router.replace(target)
  } catch (err) {
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
          <h1 class="text-2xl font-semibold">Go Messager</h1>
          <p class="mt-1 text-sm text-base-content/60">{{ title }}</p>
        </div>

        <div role="tablist" class="tabs tabs-box w-full">
          <button type="button" role="tab" class="tab flex-1" :class="{ 'tab-active': !isRegister }"
            @click="switchMode('login')">登录</button>
          <button type="button" role="tab" class="tab flex-1" :class="{ 'tab-active': isRegister }"
            @click="switchMode('register')">注册</button>
        </div>

        <form class="flex flex-col gap-3" novalidate @submit.prevent="onSubmit">
          <label class="floating-label">
            <span>用户名</span>
            <input v-model="form.name" type="text" name="name" placeholder="请输入用户名" autocomplete="username"
              class="input input-bordered w-full" :disabled="loading" />
          </label>

          <label class="floating-label">
            <span>密码</span>
            <input v-model="form.password" type="password" name="password" placeholder="请输入密码"
              :autocomplete="isRegister ? 'new-password' : 'current-password'" class="input input-bordered w-full"
              :disabled="loading" />
          </label>

          <label v-if="isRegister" class="floating-label">
            <span>邮箱（可选，用于 Gravatar 头像）</span>
            <input v-model="form.email" type="email" name="email" placeholder="you@example.com" autocomplete="email"
              class="input input-bordered w-full" :disabled="loading" />
          </label>

          <label v-if="isRegister" class="floating-label">
            <span>确认密码</span>
            <input v-model="form.confirm" type="password" name="confirm" placeholder="请再次输入密码"
              autocomplete="new-password" class="input input-bordered w-full" :disabled="loading" />
          </label>

          <div v-if="errorMsg" role="alert" class="alert alert-error alert-soft py-2 text-sm">
            <span>{{ errorMsg }}</span>
          </div>

          <button type="submit" class="btn btn-primary w-full" :disabled="loading">
            <span v-if="loading" class="loading loading-spinner loading-sm"></span>
            {{ submitText }}
          </button>
        </form>

        <p class="text-center text-xs text-base-content/50">
          {{ isRegister ? '已有账号？' : '还没有账号？' }}
          <button type="button" class="link link-primary"
            @click="switchMode(isRegister ? 'login' : 'register')">{{ isRegister ? '去登录' : '去注册' }}</button>
        </p>
      </div>
    </div>
  </div>
</template>
