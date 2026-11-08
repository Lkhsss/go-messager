<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { createTag, deleteTag, fetchTags, updateTag, type TagItem } from '@/api/tag'
import { isApiError } from '@/api/http'
import { tagBadgeStyle } from '@/utils/tagColor'
import AdminUsersPanel from '@/components/AdminUsersPanel.vue'

const activeTab = ref<'tags' | 'users'>('tags')

const tags = ref<TagItem[]>([])
const name = ref('')
const loading = ref(false)
const errorMsg = ref('')
const successMsg = ref('')

// 颜色可选：不勾选「自定义颜色」就用默认高级灰
const useCustomColor = ref(false)
const color = ref('#3b82f6')
const hexColorPattern = /^#?([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$/

// 编辑标签
const editDialog = ref<HTMLDialogElement | null>(null)
const editing = ref<TagItem | null>(null)
const editName = ref('')
const editUseCustomColor = ref(false)
const editColor = ref('#3b82f6')
const editSaving = ref(false)
const editError = ref('')

// 删除标签
const deleteDialog = ref<HTMLDialogElement | null>(null)
const deletingTag = ref<TagItem | null>(null)
const deleting = ref(false)
const deleteError = ref('')

function resetColor() {
  useCustomColor.value = false
  color.value = '#3b82f6'
}

function sortTags(list: TagItem[]): TagItem[] {
  return [...list].sort((a, b) => a.name.localeCompare(b.name))
}

function isValidColor(raw: string): boolean {
  return !raw || hexColorPattern.test(raw)
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
  if ([...trimmed].length > 30) {
    errorMsg.value = '标签名最多 30 个字符'
    return
  }

  const trimmedColor = useCustomColor.value ? color.value.trim() : ''
  if (!isValidColor(trimmedColor)) {
    errorMsg.value = '颜色需为 #RGB 或 #RRGGBB 格式的十六进制'
    return
  }

  loading.value = true
  errorMsg.value = ''
  successMsg.value = ''
  try {
    const tag = await createTag(trimmed, trimmedColor)
    tags.value = sortTags([...tags.value, tag])
    name.value = ''
    successMsg.value = `标签「${tag.name}」已创建`
  } catch (err) {
    errorMsg.value = isApiError(err) ? err.message : '创建失败，请稍后再试'
  } finally {
    loading.value = false
  }
}

function openEdit(tag: TagItem) {
  editing.value = tag
  editName.value = tag.name
  editUseCustomColor.value = Boolean(tag.color)
  editColor.value = tag.color || '#3b82f6'
  editError.value = ''
  errorMsg.value = ''
  successMsg.value = ''
  if (!editDialog.value?.open) {
    editDialog.value?.showModal()
  }
}

function closeEdit() {
  if (editSaving.value) return
  editDialog.value?.close()
}

function onEditClosed() {
  editing.value = null
  editError.value = ''
}

function onEditCancel(event: Event) {
  if (editSaving.value) {
    event.preventDefault()
  }
}

async function saveEdit() {
  if (editSaving.value) return

  const target = editing.value
  if (!target) return

  const trimmedName = editName.value.trim()
  if (!trimmedName) {
    editError.value = '请输入标签名'
    return
  }
  if ([...trimmedName].length > 30) {
    editError.value = '标签名最多 30 个字符'
    return
  }

  const trimmedColor = editUseCustomColor.value ? editColor.value.trim() : ''
  if (!isValidColor(trimmedColor)) {
    editError.value = '颜色需为 #RGB 或 #RRGGBB 格式的十六进制'
    return
  }

  editSaving.value = true
  editError.value = ''
  errorMsg.value = ''
  successMsg.value = ''
  try {
    const updated = await updateTag(target.ID, trimmedName, trimmedColor)
    tags.value = sortTags(tags.value.map((tag) => (tag.ID === updated.ID ? updated : tag)))
    successMsg.value = `标签「${updated.name}」已更新`
    editSaving.value = false
    editDialog.value?.close()
  } catch (err) {
    editError.value = isApiError(err) ? err.message : '更新失败，请稍后再试'
  } finally {
    editSaving.value = false
  }
}

function openDelete(tag: TagItem) {
  deletingTag.value = tag
  deleteError.value = ''
  errorMsg.value = ''
  successMsg.value = ''
  if (!deleteDialog.value?.open) {
    deleteDialog.value?.showModal()
  }
}

function closeDelete() {
  if (deleting.value) return
  deleteDialog.value?.close()
}

function onDeleteClosed() {
  deletingTag.value = null
  deleteError.value = ''
}

function onDeleteCancel(event: Event) {
  if (deleting.value) {
    event.preventDefault()
  }
}

async function confirmDelete() {
  if (deleting.value) return

  const target = deletingTag.value
  if (!target) return

  deleting.value = true
  deleteError.value = ''
  errorMsg.value = ''
  successMsg.value = ''
  try {
    await deleteTag(target.ID)
    tags.value = tags.value.filter((tag) => tag.ID !== target.ID)
    successMsg.value = `标签「${target.name}」已删除`
    deleting.value = false
    deleteDialog.value?.close()
  } catch (err) {
    deleteError.value = isApiError(err) ? err.message : '删除失败，请稍后再试'
  } finally {
    deleting.value = false
  }
}

onMounted(() => {
  void loadTags()
})
</script>

<template>
  <main class="mx-auto w-full p-4 sm:p-6 lg:p-8"
    :class="activeTab === 'users' ? 'max-w-5xl' : 'max-w-3xl'">
    <div class="mb-4">
      <h1 class="text-2xl font-semibold lg:text-3xl">管理</h1>
    </div>

    <div role="tablist" class="tabs tabs-box mb-5">
      <button type="button" role="tab" class="tab" :class="{ 'tab-active': activeTab === 'tags' }"
        :aria-selected="activeTab === 'tags'" @click="activeTab = 'tags'">标签管理</button>
      <button type="button" role="tab" class="tab" :class="{ 'tab-active': activeTab === 'users' }"
        :aria-selected="activeTab === 'users'" @click="activeTab = 'users'">用户管理</button>
    </div>

    <section v-show="activeTab === 'tags'" class="card bg-base-100 shadow-sm">
      <div class="card-body gap-4">
        <div>
          <h2 class="card-title text-lg">标签管理</h2>
          <p class="mt-1 text-sm text-base-content/60">标签只有管理员能创建、编辑和删除，发帖时每人最多选择 5 个。</p>
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

          <!-- 不勾选使用默认色 -->
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
          <div v-else class="space-y-2">
            <div v-for="tag in tags" :key="tag.ID"
              class="flex flex-wrap items-center gap-2 rounded-box border border-base-200 px-3 py-2">
              <span class="badge" :style="tagBadgeStyle(tag)">#{{ tag.name }}</span>
              <span class="text-xs text-base-content/50">{{ tag.usage_count }} 篇文章</span>
              <div class="ml-auto flex items-center gap-1">
                <button type="button" class="btn btn-ghost btn-xs" :disabled="deleting || editSaving"
                  @click="openEdit(tag)">
                  编辑
                </button>
                <button type="button" class="btn btn-ghost btn-xs text-error" :disabled="deleting || editSaving"
                  @click="openDelete(tag)">
                  删除
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>

    <AdminUsersPanel v-if="activeTab === 'users'" />

    <!-- 编辑标签弹窗 -->
    <dialog ref="editDialog" class="modal" @cancel="onEditCancel" @close="onEditClosed">
      <div class="modal-box max-w-lg">
        <h3 class="text-lg font-semibold">编辑标签</h3>

        <form class="mt-4 space-y-4" novalidate @submit.prevent="saveEdit">
          <label class="floating-label">
            <span>标签名</span>
            <input v-model="editName" type="text" placeholder="例如：Go" maxlength="30" class="input input-bordered w-full"
              :disabled="editSaving" />
          </label>

          <div class="flex flex-wrap items-center gap-3 text-sm">
            <label class="flex cursor-pointer items-center gap-2">
              <input v-model="editUseCustomColor" type="checkbox" class="toggle toggle-sm" :disabled="editSaving" />
              自定义颜色
            </label>

            <template v-if="editUseCustomColor">
              <input v-model="editColor" type="color" class="h-9 w-14 cursor-pointer rounded border border-base-300 bg-base-100 p-1"
                aria-label="选择标签颜色" :disabled="editSaving" />
              <input v-model="editColor" type="text" class="input input-bordered input-sm w-28" placeholder="#6b7280"
                maxlength="7" aria-label="十六进制颜色" :disabled="editSaving" />
            </template>

            <span class="inline-flex items-center gap-1 text-xs text-base-content/60">
              预览：
              <span class="badge" :style="tagBadgeStyle({ color: editUseCustomColor ? editColor : '' })">
                #{{ editName.trim() || '标签' }}
              </span>
            </span>
          </div>

          <div v-if="editError" role="alert" class="alert alert-error alert-soft py-2 text-sm">
            <span>{{ editError }}</span>
          </div>

          <div class="modal-action">
            <button type="button" class="btn btn-ghost" :disabled="editSaving" @click="closeEdit">取消</button>
            <button type="submit" class="btn btn-primary" :disabled="editSaving">
              <span v-if="editSaving" class="loading loading-spinner loading-sm"></span>
              保存
            </button>
          </div>
        </form>
      </div>
      <form method="dialog" class="modal-backdrop" :class="{ 'pointer-events-none': editSaving }"><button>close</button></form>
    </dialog>

    <!-- 删除标签确认弹窗 -->
    <dialog ref="deleteDialog" class="modal" @cancel="onDeleteCancel" @close="onDeleteClosed">
      <div class="modal-box">
        <h3 class="text-lg font-semibold">删除标签</h3>

        <div class="mt-3 space-y-3 text-sm">
          <p>
            确定删除标签
            <span v-if="deletingTag" class="badge align-middle" :style="tagBadgeStyle(deletingTag)">
              #{{ deletingTag.name }}
            </span>
            吗？
          </p>
          <p v-if="deletingTag && deletingTag.usage_count > 0" class="text-warning">
            该标签被 {{ deletingTag.usage_count }} 篇文章使用，删除后会从这些文章中移除，文章本身不会删除。
          </p>
          <p v-else class="text-base-content/60">删除后不可恢复。</p>

          <div v-if="deleteError" role="alert" class="alert alert-error alert-soft py-2 text-sm">
            <span>{{ deleteError }}</span>
          </div>
        </div>

        <div class="modal-action">
          <button type="button" class="btn btn-ghost" :disabled="deleting" @click="closeDelete">取消</button>
          <button type="button" class="btn btn-error" :disabled="deleting" @click="confirmDelete">
            <span v-if="deleting" class="loading loading-spinner loading-sm"></span>
            删除
          </button>
        </div>
      </div>
      <form method="dialog" class="modal-backdrop" :class="{ 'pointer-events-none': deleting }"><button>close</button></form>
    </dialog>
  </main>
</template>

