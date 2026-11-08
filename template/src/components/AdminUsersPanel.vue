<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import Pagination from '@/components/Pagination.vue'
import UserAvatar from '@/components/UserAvatar.vue'
import UserStatsCards from '@/components/UserStatsCards.vue'
import {
  changeAdminUserPassword,
  deleteAdminUser,
  fetchAdminUserDetail,
  fetchAdminUsers,
  setAdminUserDisabled,
  type AdminUserDetail,
  type AdminUserItem,
  type AdminUserRole,
} from '@/api/adminUser'
import { isApiError, type PaginationMeta } from '@/api/http'

const PER_PAGE = 10

const users = ref<AdminUserItem[]>([])
const meta = ref<PaginationMeta | null>(null)
const page = ref(1)
const keyword = ref('')
const loading = ref(false)
const errorMsg = ref('')
const successMsg = ref('')

const detailDialog = ref<HTMLDialogElement | null>(null)
const selectedUser = ref<AdminUserItem | null>(null)
const detail = ref<AdminUserDetail | null>(null)
const detailLoading = ref(false)
const detailError = ref('')

const passwordDialog = ref<HTMLDialogElement | null>(null)
const passwordTarget = ref<AdminUserItem | null>(null)
const newPassword = ref('')
const passwordSaving = ref(false)
const passwordError = ref('')

type ActionMode = 'disable' | 'enable' | 'delete'

const actionDialog = ref<HTMLDialogElement | null>(null)
const actionTarget = ref<AdminUserItem | null>(null)
const actionMode = ref<ActionMode>('disable')
const actionSaving = ref(false)
const actionError = ref('')

const actionTitle = computed(() => {
  if (actionMode.value === 'delete') return '删除用户'
  return actionMode.value === 'disable' ? '封禁用户' : '解封用户'
})

function formatDate(value?: string | null): string {
  if (!value) return '从未'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString('zh-CN')
}

function roleLabel(role: AdminUserRole): string {
  if (role === 'admin') return '管理员'
  if (role === 'none') return '已封禁'
  return '用户'
}

function roleBadgeClass(role: AdminUserRole): string {
  if (role === 'admin') return 'badge-primary'
  if (role === 'none') return 'badge-error'
  return 'badge-ghost'
}

async function loadUsers(next = page.value, query = keyword.value) {
  if (loading.value) return

  loading.value = true
  errorMsg.value = ''
  try {
    const result = await fetchAdminUsers({ page: next, perPage: PER_PAGE, q: query })
    users.value = result.users
    meta.value = result.meta
    page.value = result.meta.page > 0 ? result.meta.page : next
    keyword.value = query.trim()
  } catch (err) {
    errorMsg.value = isApiError(err) ? err.message : '获取用户列表失败，请稍后再试'
  } finally {
    loading.value = false
  }
}

function submitSearch() {
  void loadUsers(1)
}

function goTo(next: number) {
  const totalPages = meta.value?.total_pages ?? 0
  if (loading.value || next < 1 || next > totalPages || next === page.value) return
  void loadUsers(next)
}

function openDetail(user: AdminUserItem) {
  selectedUser.value = user
  detail.value = null
  detailError.value = ''
  if (!detailDialog.value?.open) {
    detailDialog.value?.showModal()
  }
  void loadDetail(user.id)
}

async function loadDetail(id: number) {
  detailLoading.value = true
  detailError.value = ''
  try {
    detail.value = await fetchAdminUserDetail(id)
  } catch (err) {
    detailError.value = isApiError(err) ? err.message : '获取用户详情失败，请稍后再试'
  } finally {
    detailLoading.value = false
  }
}

function closeDetail() {
  detailDialog.value?.close()
}

function onDetailClosed() {
  selectedUser.value = null
  detail.value = null
  detailError.value = ''
}

function openPassword(user: AdminUserItem) {
  passwordTarget.value = user
  newPassword.value = ''
  passwordError.value = ''
  if (!passwordDialog.value?.open) {
    passwordDialog.value?.showModal()
  }
}

function closePassword() {
  if (passwordSaving.value) return
  passwordDialog.value?.close()
}

function onPasswordClosed() {
  passwordTarget.value = null
  newPassword.value = ''
  passwordError.value = ''
}

function onPasswordCancel(event: Event) {
  if (passwordSaving.value) {
    event.preventDefault()
  }
}

async function savePassword() {
  if (passwordSaving.value) return

  const target = passwordTarget.value
  if (!target) return
  if (!newPassword.value) {
    passwordError.value = '密码不能为空'
    return
  }
  if (new TextEncoder().encode(newPassword.value).length > 72) {
    passwordError.value = '密码最多 72 字节'
    return
  }

  passwordSaving.value = true
  passwordError.value = ''
  successMsg.value = ''
  try {
    await changeAdminUserPassword(target.id, newPassword.value)
    successMsg.value = `已修改「${target.name}」的密码`
    passwordSaving.value = false
    passwordDialog.value?.close()
  } catch (err) {
    passwordError.value = isApiError(err) ? err.message : '修改密码失败，请稍后再试'
  } finally {
    passwordSaving.value = false
  }
}

function openAction(user: AdminUserItem, mode: ActionMode) {
  actionTarget.value = user
  actionMode.value = mode
  actionError.value = ''
  if (!actionDialog.value?.open) {
    actionDialog.value?.showModal()
  }
}

function closeAction() {
  if (actionSaving.value) return
  actionDialog.value?.close()
}

function onActionClosed() {
  actionTarget.value = null
  actionError.value = ''
}

function onActionCancel(event: Event) {
  if (actionSaving.value) {
    event.preventDefault()
  }
}

async function confirmAction() {
  if (actionSaving.value) return

  const target = actionTarget.value
  if (!target) return

  actionSaving.value = true
  actionError.value = ''
  successMsg.value = ''
  try {
    if (actionMode.value === 'delete') {
      await deleteAdminUser(target.id)
      successMsg.value = `用户「${target.name}」已删除`
    } else {
      const disabled = actionMode.value === 'disable'
      await setAdminUserDisabled(target.id, disabled)
      successMsg.value = disabled
        ? `用户「${target.name}」已封禁`
        : `用户「${target.name}」已解封`
    }

    actionSaving.value = false
    actionDialog.value?.close()

    // 删除当前页最后一条时回到上一页，避免停在空页。
    const nextPage = actionMode.value === 'delete' && users.value.length === 1 && page.value > 1
      ? page.value - 1
      : page.value
    await loadUsers(nextPage)
  } catch (err) {
    actionError.value = isApiError(err) ? err.message : '操作失败，请稍后再试'
  } finally {
    actionSaving.value = false
  }
}

onMounted(() => {
  void loadUsers()
})
</script>

<template>
  <section class="card bg-base-100 shadow-sm">
    <div class="card-body gap-4">
      <div class="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h2 class="card-title text-lg">用户管理</h2>
          <p class="mt-1 text-sm text-base-content/60">共 {{ meta?.total ?? 0 }} 位用户</p>
        </div>
        <form class="join" @submit.prevent="submitSearch">
          <input v-model="keyword" type="search" placeholder="搜索用户名" aria-label="搜索用户名"
            class="input input-bordered input-sm join-item w-40 sm:w-52" :disabled="loading" />
          <button type="submit" class="btn btn-sm join-item" :disabled="loading">搜索</button>
        </form>
      </div>

      <div v-if="errorMsg" role="alert" class="alert alert-error alert-soft py-2 text-sm">
        <span>{{ errorMsg }}</span>
        <button type="button" class="btn btn-sm" @click="loadUsers()">重试</button>
      </div>
      <div v-if="successMsg" role="status" class="alert alert-success alert-soft py-2 text-sm">
        <span>{{ successMsg }}</span>
      </div>

      <div v-if="loading && users.length === 0" class="flex justify-center py-10">
        <span class="loading loading-spinner loading-lg"></span>
      </div>
      <div v-else-if="users.length === 0" class="py-10 text-center text-sm text-base-content/50">
        没有找到用户
      </div>

      <div v-else class="space-y-2" :class="{ 'opacity-60': loading }">
        <article v-for="user in users" :key="user.id"
          class="rounded-box border border-base-200 bg-base-100 p-3 transition hover:border-base-300">
          <div class="flex flex-wrap items-center gap-3">
            <UserAvatar :name="user.name" :version="user.avatar_version" :size="40" />

            <div class="min-w-0 flex-1">
              <div class="flex flex-wrap items-center gap-2">
                <span class="truncate font-medium">{{ user.name }}</span>
                <span class="badge badge-sm" :class="roleBadgeClass(user.role)">{{ roleLabel(user.role) }}</span>
              </div>
              <p class="mt-1 text-xs text-base-content/45">
                上次活跃 {{ formatDate(user.last_active_at) }}
              </p>
            </div>

            <div class="grid grid-cols-3 gap-3 text-center">
              <div>
                <p class="text-[0.65rem] text-base-content/45">文章</p>
                <p class="font-mono text-sm font-semibold">{{ user.post_count }}</p>
              </div>
              <div>
                <p class="text-[0.65rem] text-base-content/45">评论</p>
                <p class="font-mono text-sm font-semibold">{{ user.comment_count }}</p>
              </div>
              <div>
                <p class="text-[0.65rem] text-base-content/45">获赞</p>
                <p class="font-mono text-sm font-semibold">{{ user.like }}</p>
              </div>
            </div>

            <div class="flex flex-wrap items-center justify-end gap-1">
              <button type="button" class="btn btn-ghost btn-xs" :disabled="loading" @click="openDetail(user)">详情</button>
              <button v-if="user.role !== 'admin'" type="button" class="btn btn-ghost btn-xs"
                :disabled="loading || actionSaving || passwordSaving"
                @click="openAction(user, user.role === 'none' ? 'enable' : 'disable')">
                {{ user.role === 'none' ? '解封' : '封禁' }}
              </button>
              <button type="button" class="btn btn-ghost btn-xs" :disabled="loading || passwordSaving"
                @click="openPassword(user)">修改密码</button>
              <button v-if="user.role !== 'admin'" type="button" class="btn btn-ghost btn-xs text-error"
                :disabled="loading || actionSaving" @click="openAction(user, 'delete')">删除</button>
            </div>
          </div>
        </article>
      </div>

      <div v-if="(meta?.total_pages ?? 0) > 1" class="flex justify-end">
        <Pagination :page="page" :total-pages="meta?.total_pages ?? 0" :disabled="loading" @change="goTo" />
      </div>
    </div>
  </section>

  <!-- 用户详情统计 -->
  <dialog ref="detailDialog" class="modal" @close="onDetailClosed">
    <div class="modal-box max-w-4xl">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div class="min-w-0">
          <h3 class="truncate text-lg font-semibold">
            {{ detail?.user.name ?? selectedUser?.name }}
          </h3>
          <p class="mt-1 text-xs text-base-content/45">
            注册 {{ formatDate(detail?.user.created_at ?? selectedUser?.created_at) }}
            · 上次活跃 {{ formatDate(detail?.user.last_active_at ?? selectedUser?.last_active_at) }}
            · 登录 {{ detail?.user.login_count ?? selectedUser?.login_count ?? 0 }} 次
          </p>
        </div>
        <span class="badge" :class="roleBadgeClass(detail?.user.role ?? selectedUser?.role ?? 'user')">
          {{ roleLabel(detail?.user.role ?? selectedUser?.role ?? 'user') }}
        </span>
      </div>

      <div v-if="detailLoading" class="flex justify-center py-10">
        <span class="loading loading-spinner loading-lg"></span>
      </div>
      <div v-else-if="detailError" role="alert" class="alert alert-error alert-soft mt-4">
        <span>{{ detailError }}</span>
        <button v-if="selectedUser" type="button" class="btn btn-sm" @click="loadDetail(selectedUser.id)">重试</button>
      </div>
      <div v-else-if="detail" class="mt-4">
        <UserStatsCards :stats="detail.stats" />
      </div>

      <div class="modal-action">
        <button type="button" class="btn btn-ghost" @click="closeDetail">关闭</button>
      </div>
    </div>
    <form method="dialog" class="modal-backdrop"><button>close</button></form>
  </dialog>

  <!-- 修改密码 -->
  <dialog ref="passwordDialog" class="modal" @cancel="onPasswordCancel" @close="onPasswordClosed">
    <form class="modal-box" novalidate @submit.prevent="savePassword">
      <h3 class="text-lg font-semibold">修改密码</h3>
      <p class="mt-1 text-sm text-base-content/60">用户：{{ passwordTarget?.name }}</p>

      <label class="floating-label mt-4">
        <span>新密码</span>
        <input v-model="newPassword" type="password" placeholder="输入新密码" autocomplete="new-password"
          class="input input-bordered w-full" :disabled="passwordSaving" />
      </label>

      <div v-if="passwordError" role="alert" class="alert alert-error alert-soft mt-3 py-2 text-sm">
        <span>{{ passwordError }}</span>
      </div>

      <div class="modal-action">
        <button type="button" class="btn btn-ghost" :disabled="passwordSaving" @click="closePassword">取消</button>
        <button type="submit" class="btn btn-primary" :disabled="passwordSaving">
          <span v-if="passwordSaving" class="loading loading-spinner loading-sm"></span>
          保存
        </button>
      </div>
    </form>
    <form method="dialog" class="modal-backdrop" :class="{ 'pointer-events-none': passwordSaving }">
      <button>close</button>
    </form>
  </dialog>

  <!-- 封禁/解封/删除确认 -->
  <dialog ref="actionDialog" class="modal" @cancel="onActionCancel" @close="onActionClosed">
    <div class="modal-box">
      <h3 class="text-lg font-semibold">{{ actionTitle }}</h3>

      <div class="mt-3 space-y-3 text-sm">
        <p>
          确定对用户
          <span class="font-medium">{{ actionTarget?.name }}</span>
          执行此操作吗？
        </p>
        <p v-if="actionMode === 'delete'" class="text-warning">
          只删除账号，文章和评论会保留。删除后账号不能登录，且不可恢复。
        </p>
        <p v-else-if="actionMode === 'disable'" class="text-warning">
          封禁后该用户会立即无法登录，已登录的令牌也会失效。
        </p>
        <p v-else class="text-base-content/60">解封后该用户可以重新登录。</p>

        <div v-if="actionError" role="alert" class="alert alert-error alert-soft py-2 text-sm">
          <span>{{ actionError }}</span>
        </div>
      </div>

      <div class="modal-action">
        <button type="button" class="btn btn-ghost" :disabled="actionSaving" @click="closeAction">取消</button>
        <button type="button" class="btn" :class="actionMode === 'delete' ? 'btn-error' : actionMode === 'disable' ? 'btn-warning' : 'btn-primary'"
          :disabled="actionSaving" @click="confirmAction">
          <span v-if="actionSaving" class="loading loading-spinner loading-sm"></span>
          确定
        </button>
      </div>
    </div>
    <form method="dialog" class="modal-backdrop" :class="{ 'pointer-events-none': actionSaving }">
      <button>close</button>
    </form>
  </dialog>

</template>

