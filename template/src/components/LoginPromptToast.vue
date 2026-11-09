<script setup lang="ts">
import { useRouter } from 'vue-router'
import { redirectToLogin } from '@/utils/authRedirect'
import { useLoginPromptStore } from '@/stores/loginPrompt'

const router = useRouter()
const prompt = useLoginPromptStore()

function goLogin() {
  const target = prompt.redirect.startsWith('/') && !prompt.redirect.startsWith('//')
    ? prompt.redirect
    : '/'
  prompt.hide()
  redirectToLogin(router, target)
}
</script>

<template>
  <div class="toast toast-end z-50">
    <div v-if="prompt.visible" role="status" aria-live="polite" class="alert alert-info shadow-lg">
      <span>登录后才能进行点赞、评论或发布内容</span>
      <div class="flex gap-2">
        <button type="button" class="btn btn-ghost btn-xs" @click="prompt.hide()">稍后</button>
        <button type="button" class="btn btn-primary btn-xs" @click="goLogin">去登录</button>
      </div>
    </div>
  </div>
</template>