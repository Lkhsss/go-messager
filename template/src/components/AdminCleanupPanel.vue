<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  cleanupAll,
  cleanupCategory,
  fetchCleanupStats,
  vacuumDatabase,
  type CleanupCategory,
  type CleanupStats,
} from '@/api/adminCleanup'
import { isApiError } from '@/api/http'
import { formatBytes } from '@/utils/format'
import CleanupChart from '@/components/charts/CleanupChart.vue'

type PendingAction =
  | { type: 'category'; category: CleanupCategory }
  | { type: 'all' }
  | { type: 'vacuum' }

const stats = ref<CleanupStats | null>(null)
const loading = ref(false)
const saving = ref(false)
const errorMsg = ref('')
const successMsg = ref('')

const dialog = ref<HTMLDialogElement | null>(null)
const pending = ref<PendingAction | null>(null)
const actionError = ref('')

const categories = computed(() => stats.value?.categories ?? [])
const totalCount = computed(() => stats.value?.total_count ?? 0)
const totalBytes = computed(() => stats.value?.total_bytes ?? 0)

const dialogTitle = computed(() => {
  if (!pending.value) return ''
  if (pending.value.type === 'all') return '清理全部'
  if (pending.value.type === 'vacuum') return '压缩数据库'
  return `清理${pending.value.category.label}`
})

const dialogDescription = computed(() => {
  if (!pending.value) return ''
  if (pending.value.type === 'all') {
    return `将永久删除全部 ${totalCount.value} 项可清理数据，估算释放 ${formatBytes(totalBytes.value)}，且不可恢复。`
  }
  if (pending.value.type === 'vacuum') {
    return '将重建数据库文件并回收空闲页。执行期间会短暂占用较多磁盘和 CPU，建议在低峰时段操作。'
  }
  const { category } = pending.value
  return `将永久删除 ${category.count} 项${category.label}，估算释放 ${formatBytes(category.bytes)}，且不可恢复。`
})

async function load() {
  loading.value = true
  errorMsg.value = ''
  try {
    stats.value = await fetchCleanupStats()
  } catch (err) {
    errorMsg.value = isApiError(err) ? err.message : '获取清理统计失败，请稍后再试'
  } finally {
    loading.value = false
  }
}

function openAction(action: PendingAction) {
  if (saving.value) return
  pending.value = action
  actionError.value = ''
  successMsg.value = ''
  if (!dialog.value?.open) {
    dialog.value?.showModal()
  }
}

function openCategory(category: CleanupCategory) {
  if (category.count <= 0) return
  openAction({ type: 'category', category })
}

function closeDialog() {
  if (saving.value) return
  dialog.value?.close()
}

function onDialogClosed() {
  pending.value = null
  actionError.value = ''
}

function onDialogCancel(event: Event) {
  if (saving.value) {
    event.preventDefault()
  }
}

async function confirmAction() {
  const action = pending.value
  if (!action || saving.value) return

  saving.value = true
  actionError.value = ''
  successMsg.value = ''
  try {
    if (action.type === 'category') {
      const result = await cleanupCategory(action.category.key)
      successMsg.value = `已清理${action.category.label} ${result.deleted} 项，估算释放 ${formatBytes(result.freed_bytes)}`
    } else if (action.type === 'all') {
      const results = await cleanupAll()
      const deleted = results.reduce((sum, item) => sum + item.deleted, 0)
      const freed = results.reduce((sum, item) => sum + item.freed_bytes, 0)
      successMsg.value = `已清理 ${deleted} 项，估算释放 ${formatBytes(freed)}`
    } else {
      const result = await vacuumDatabase()
      successMsg.value = `数据库已压缩，回收 ${formatBytes(result.freed_bytes)}，当前大小 ${formatBytes(result.after_bytes)}`
    }

    await load()
    dialog.value?.close()
  } catch (err) {
    actionError.value = isApiError(err) ? err.message : '操作失败，请稍后再试'
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  void load()
})
</script>
<template>
  <div class="space-y-4">
    <div class="card bg-base-100 shadow-sm">
      <div class="card-body gap-4">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div>
            <h2 class="card-title text-lg">清理</h2>
            <p class="mt-1 text-sm text-base-content/60">
              清理软删除的文章、孤立评论与残留数据；删除后不可恢复，建议操作前备份数据库。
            </p>
          </div>
          <button type="button" class="btn btn-sm btn-ghost" :disabled="loading || saving" @click="load">
            刷新统计
          </button>
        </div>

        <div v-if="errorMsg" role="alert" class="alert alert-error alert-soft py-2 text-sm">
          <span>{{ errorMsg }}</span>
        </div>
        <div v-if="successMsg" role="status" class="alert alert-success alert-soft py-2 text-sm">
          <span>{{ successMsg }}</span>
        </div>

        <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
          <div class="rounded-box border border-base-200 p-3">
            <p class="text-xs text-base-content/55">数据库大小</p>
            <p class="mt-1 text-lg font-semibold">{{ formatBytes(stats?.database.file_size_bytes ?? 0) }}</p>
            <p class="mt-1 text-xs text-base-content/45">空闲页 {{ formatBytes(stats?.database.free_bytes ?? 0) }}</p>
          </div>
          <div class="rounded-box border border-base-200 p-3">
            <p class="text-xs text-base-content/55">可清理条目</p>
            <p class="mt-1 text-lg font-semibold">{{ totalCount }} 项</p>
            <p class="mt-1 text-xs text-base-content/45">共 {{ categories.length }} 个分类</p>
          </div>
          <div class="rounded-box border border-base-200 p-3">
            <p class="text-xs text-base-content/55">估算可释放</p>
            <p class="mt-1 text-lg font-semibold">{{ formatBytes(totalBytes) }}</p>
            <p class="mt-1 text-xs text-base-content/45">压缩数据库可回收空闲页</p>
          </div>
        </div>

        <div class="rounded-box border border-base-200 p-3">
          <CleanupChart :categories="categories" />
        </div>

        <div class="space-y-2">
          <div v-for="category in categories" :key="category.key"
            class="flex flex-wrap items-center gap-2 rounded-box border border-base-200 px-3 py-2">
            <div class="min-w-0 flex-1">
              <div class="flex flex-wrap items-center gap-2">
                <span class="text-sm font-medium">{{ category.label }}</span>
                <span class="badge badge-ghost badge-sm">{{ category.count }} 项</span>
                <span class="text-xs text-base-content/50">约 {{ formatBytes(category.bytes) }}</span>
              </div>
              <p class="mt-1 text-xs text-base-content/55">{{ category.description }}</p>
            </div>
            <button type="button" class="btn btn-sm btn-warning"
              :disabled="loading || saving || category.count === 0" @click="openCategory(category)">
              清理
            </button>
          </div>
          <p v-if="!loading && categories.length === 0" class="text-sm text-base-content/60">暂无分类数据</p>
        </div>

        <div class="flex flex-wrap justify-end gap-2">
          <button type="button" class="btn btn-sm" :disabled="loading || saving" @click="openAction({ type: 'vacuum' })">
            压缩数据库
          </button>
          <button type="button" class="btn btn-sm btn-error" :disabled="loading || saving || totalCount === 0"
            @click="openAction({ type: 'all' })">
            一键清理全部
          </button>
        </div>
      </div>
    </div>

    <dialog ref="dialog" class="modal" @cancel="onDialogCancel" @close="onDialogClosed">
      <div class="modal-box">
        <h3 class="text-lg font-semibold">{{ dialogTitle }}</h3>

        <div class="mt-3 space-y-3 text-sm">
          <p class="text-warning">{{ dialogDescription }}</p>

          <div v-if="actionError" role="alert" class="alert alert-error alert-soft py-2 text-sm">
            <span>{{ actionError }}</span>
          </div>
        </div>

        <div class="modal-action">
          <button type="button" class="btn btn-ghost" :disabled="saving" @click="closeDialog">取消</button>
          <button type="button" class="btn" :class="pending?.type === 'vacuum' ? 'btn-warning' : 'btn-error'"
            :disabled="saving" @click="confirmAction">
            <span v-if="saving" class="loading loading-spinner loading-sm"></span>
            确定
          </button>
        </div>
      </div>
      <form method="dialog" class="modal-backdrop" :class="{ 'pointer-events-none': saving }">
        <button>close</button>
      </form>
    </dialog>
  </div>
</template>
