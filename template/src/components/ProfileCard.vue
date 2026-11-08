<script setup lang="ts">
import { computed } from 'vue'
import type { Role } from '@/api/authStorage'
import type { ReactionSummary } from '@/api/reaction'
import { PRIMARY_REACTIONS, reactionCount } from '@/api/reaction'
import UserAvatar from '@/components/UserAvatar.vue'

const props = withDefaults(
  defineProps<{
    name: string
    role?: Role
    avatarVersion?: string
    previewSrc?: string
    reactionTotals?: ReactionSummary
  }>(),
  {
    role: 'user',
    avatarVersion: '',
    previewSrc: '',
  },
)

const ROLE_LABELS: Record<Role, string> = {
  admin: '管理员',
  user: '成员',
  none: '已停用',
}

const roleLabel = computed(() => ROLE_LABELS[props.role])
const isAdmin = computed(() => props.role === 'admin')
const cardLabel = computed(() => `${props.name} 的个人名片，角色：${roleLabel.value}`)
</script>

<template>
  <div class="hover-3d w-full max-w-md" role="group" :aria-label="cardLabel">
    <!-- 8 个空 div 是 hover-3d 感应层。 -->
    <div class="profile-card relative flex aspect-[1.58] min-h-52 w-full flex-col overflow-hidden rounded-[1.75rem] text-slate-50">
      <div class="profile-glow" aria-hidden="true"></div>
      <div class="profile-grid" aria-hidden="true"></div>
      <div class="profile-grain" aria-hidden="true"></div>

      <div class="relative z-10 flex h-full flex-col justify-between gap-4 p-5 sm:p-6">
        <header class="flex items-start justify-between gap-3">
          <div class="flex items-center gap-3">
            <!-- 支付波纹 -->
            <svg class="h-6 w-6 text-white/50" viewBox="0 0 24 24" fill="none" aria-hidden="true">
              <path d="M7.5 8.5a7 7 0 0 1 0 7M11 6a11 11 0 0 1 0 12M14.5 3.5a15 15 0 0 1 0 17" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" />
            </svg>
          </div>

          <div class="text-right">
            <p class="profile-wordmark">GO MESSAGER</p>
            <p class="mt-0.5 text-[0.55rem] uppercase tracking-[0.3em] text-white/35">Member Card</p>
          </div>
        </header>

        <div class="flex min-w-0 items-center gap-4">
          <UserAvatar :name="name" :version="avatarVersion" :src="previewSrc" :size="56"
            class="ring-2 ring-white/25 ring-offset-2 ring-offset-[#0b1224]" />
          <div class="min-w-0">
            <p class="text-[0.55rem] uppercase tracking-[0.28em] text-white/40">Card Holder</p>
            <p class="profile-name truncate text-xl font-semibold sm:text-2xl">{{ name }}</p>
            <span class="mt-1 inline-flex items-center gap-1.5 rounded-full border px-2 py-0.5 text-[0.65rem]"
              :class="isAdmin ? 'border-amber-200/40 bg-amber-200/15 text-amber-100' : 'border-white/15 bg-white/10 text-white/75'">
              <span class="h-1 w-1 rounded-full" :class="isAdmin ? 'bg-amber-200' : 'bg-white/50'"></span>
              {{ roleLabel }}
            </span>
          </div>
        </div>

        <footer class="flex items-end justify-between gap-3">
          <div class="flex items-end gap-4 sm:gap-5">
            <div v-for="option in PRIMARY_REACTIONS" :key="option.type" class="text-center">
              <p class="text-sm leading-none">{{ option.emoji }}</p>
              <p class="profile-stat font-mono text-lg leading-tight">{{ reactionCount(reactionTotals, option.type) }}</p>
            </div>
          </div>

          <div class="flex items-center gap-2">
            <span class="hidden text-[0.55rem] uppercase tracking-[0.3em] text-white/30 sm:inline">Member</span>
            <span class="profile-holo" aria-hidden="true"></span>
          </div>
        </footer>
      </div>
    </div>

    <div v-for="n in 8" :key="n" aria-hidden="true"></div>
  </div>
</template>

<style scoped>
/* 显式使用 3 个可收缩列：避免卡面内容把 hover-3d 网格撑得比容器还宽（移动端溢出） */
.hover-3d {
  display: grid;
  width: 100%;
  max-width: min(100%, 28rem);
  min-width: 0;
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.profile-card {
  min-width: 0;
  max-width: 100%;
  background:
    radial-gradient(120% 120% at 100% 0%, rgba(56, 189, 248, 0.26), transparent 55%),
    radial-gradient(90% 100% at 0% 100%, rgba(129, 140, 248, 0.22), transparent 60%),
    linear-gradient(135deg, #0b1224 0%, #111a33 45%, #080d19 100%);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.2),
    inset 0 0 0 1px rgba(255, 255, 255, 0.08),
    0 24px 60px -24px rgba(2, 6, 23, 0.75);
}

/* daisyUI 的悬停高光用的是 :first-child::before，这里用普通子元素做背景层 */
.profile-glow {
  position: absolute;
  inset: 0;
  pointer-events: none;
  background:
    radial-gradient(40% 60% at 85% 20%, rgba(125, 211, 252, 0.2), transparent 70%),
    radial-gradient(30% 50% at 15% 85%, rgba(196, 181, 253, 0.18), transparent 75%);
}

.profile-grid {
  position: absolute;
  inset: 0;
  pointer-events: none;
  opacity: 0.16;
  background-image:
    linear-gradient(rgba(255, 255, 255, 0.08) 1px, transparent 1px),
    linear-gradient(90deg, rgba(255, 255, 255, 0.08) 1px, transparent 1px);
  background-size: 2.5rem 2.5rem;
  -webkit-mask-image: linear-gradient(to bottom right, black, transparent 70%);
  mask-image: linear-gradient(to bottom right, black, transparent 70%);
}

.profile-grain {
  position: absolute;
  inset: 0;
  pointer-events: none;
  opacity: 0.35;
  mix-blend-mode: soft-light;
  background-image: radial-gradient(rgba(255, 255, 255, 0.45) 0.5px, transparent 0.6px);
  background-size: 3px 3px;
}

.profile-holo {
  display: inline-block;
  width: 2.35rem;
  height: 2.35rem;
  border-radius: 9999px;
  opacity: 0.75;
  background:
    radial-gradient(circle at 30% 25%, rgba(255, 255, 255, 0.85), transparent 35%),
    conic-gradient(from 210deg, #67e8f9, #a78bfa, #f0abfc, #fde68a, #67e8f9);
  filter: blur(0.3px) saturate(1.2);
  box-shadow:
    inset 0 0 0 1px rgba(255, 255, 255, 0.5),
    0 0 18px rgba(167, 139, 250, 0.45);
}

.profile-wordmark {
  font-size: 0.62rem;
  font-weight: 700;
  letter-spacing: 0.32em;
  color: rgba(255, 255, 255, 0.85);
}

.profile-name {
  text-shadow:
    0 1px 0 rgba(255, 255, 255, 0.12),
    0 6px 18px rgba(2, 6, 23, 0.45);
}

.profile-stat {
  color: #f8fafc;
  text-shadow: 0 1px 0 rgba(255, 255, 255, 0.2);
}

/* 触屏/低动态偏好：保留静态名片，关闭倾斜和放大 */
@media (prefers-reduced-motion: reduce) {
  .hover-3d > :first-child {
    transform: none !important;
    scale: none !important;
    transition: none !important;
  }
}
</style>
