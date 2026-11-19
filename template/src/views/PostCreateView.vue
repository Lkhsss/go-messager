<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { createPost, fetchPost, updatePost } from "@/api/post";
import TagPicker from "@/components/TagPicker.vue";
import { fetchTags, type TagItem } from "@/api/tag";
import { isApiError } from "@/api/http";
import { extractBodyTags, renderMarkdown } from "@/utils/markdown";
import { tagBadgeStyle } from "@/utils/tagColor";
import { clearPostDraft, readPostDraft, savePostDraft } from "@/utils/postDraft";
import { useAuthStore } from "@/stores/auth";
import { useToastStore } from "@/stores/toast";

// 正文自动标签解析防抖
const AUTO_TAG_DELAY_MS = 250;
// 与后端 maxPostBodyRunes 保持一致
const MAX_BODY = 100000;
// 停止输入后多久静默落盘草稿
const DRAFT_SAVE_DELAY_MS = 800;

const route = useRoute();
const router = useRouter();
const auth = useAuthStore();
const toast = useToastStore();

const postId = computed(() => {
  const raw = Array.isArray(route.params.id) ? route.params.id[0] : route.params.id;
  const value = Number(raw);
  return Number.isInteger(value) && value > 0 ? value : null;
});
const isEdit = computed(() => route.name === "post-edit");

const selectedIds = ref<number[]>([]);
const knownTags = ref<TagItem[]>([]);
// 已审核标签，用于自动标签取色与审核状态判断。
const allReviewedTags = ref<TagItem[]>([]);
// 正文实时解析出的标签名。
const autoTagNames = ref<string[]>([]);
const bodyTextarea = ref<HTMLTextAreaElement | null>(null);
let autoTagTimer: number | undefined;
const submitting = ref(false);
const loadingPost = ref(false);
const loadError = ref("");
const errorMsg = ref("");

const form = reactive({
  title: "",
  subtitle: "",
  body: "",
});

// 编辑 / 预览切换；提交的仍然是 Markdown 原文
const showPreview = ref(false);
const previewHtml = computed(() => renderMarkdown(form.body, allReviewedTags.value));
const bodyLength = computed(() => Array.from(form.body).length);

// 已审核标签映射，供正文自动标签取色。
const reviewedTagByName = computed(() => {
  const map = new Map<string, TagItem>();
  for (const tag of allReviewedTags.value) map.set(tag.name, tag);
  return map;
});

// 手选标签名集合，自动识别结果不重复展示。
const selectedTagNames = computed(() => new Set(draftTags().map((tag) => tag.name)));

// 正文自动识别的标签；tag 为空表示待审核或新标签。
const autoTagItems = computed(() =>
  autoTagNames.value
    .filter((name) => !selectedTagNames.value.has(name))
    .map((name) => ({ name, tag: reviewedTagByName.value.get(name) })),
);

function refreshAutoTags() {
  autoTagNames.value = extractBodyTags(form.body);
}

// 点击自动标签，定位到正文中首次出现的位置。
function locateAutoTag(name: string) {
  const textarea = bodyTextarea.value;
  if (!textarea) return;
  const needle = `#${name}`;
  const index = form.body.indexOf(needle);
  if (index < 0) return;

  showPreview.value = false;
  void nextTick(() => {
    textarea.focus();
    textarea.setSelectionRange(index, index + needle.length);
  });
}

async function loadReviewedTags() {
  try {
    allReviewedTags.value = await fetchTags({ sort: "name" });
  } catch {
    allReviewedTags.value = [];
  }
}

// 草稿状态：draftExists 控制「清除草稿」按钮，draftRestored 控制恢复提示。
const draftExists = ref(false);
const draftRestored = ref(false);
const clearDraftDialog = ref<HTMLDialogElement | null>(null);

let draftTimer: number | undefined;
// 发布成功后不再写草稿，避免离开页面时把已发布内容又存回来。
let draftPublished = false;

function draftUserName(): string {
  return auth.user?.name ?? "";
}

// 标签用快照保存：标签之后被删，恢复草稿时仍能显示名字和颜色。
function draftTags(): TagItem[] {
  return selectedIds.value
    .map((id) => knownTags.value.find((tag) => tag.ID === id))
    .filter((tag): tag is TagItem => Boolean(tag));
}

function draftHasContent(): boolean {
  return (
    form.title.trim() !== "" ||
    form.subtitle.trim() !== "" ||
    form.body.trim() !== "" ||
    selectedIds.value.length > 0
  );
}

// 静默保存；内容清空时顺手删掉旧草稿。
function saveDraftNow() {
  if (isEdit.value || draftPublished) return;

  const name = draftUserName();
  if (!name) return;

  if (!draftHasContent()) {
    clearPostDraft(name);
    draftExists.value = false;
    return;
  }

  const saved = savePostDraft(name, {
    title: form.title,
    subtitle: form.subtitle,
    body: form.body,
    tags: draftTags(),
  });
  if (saved) {
    draftExists.value = true;
  }
}

function scheduleDraftSave() {
  if (isEdit.value || draftPublished) return;
  if (draftTimer !== undefined) {
    window.clearTimeout(draftTimer);
  }
  draftTimer = window.setTimeout(() => {
    draftTimer = undefined;
    saveDraftNow();
  }, DRAFT_SAVE_DELAY_MS);
}

// 关闭页面、切到后台或站内跳转前立即落盘，避免丢最后一段输入。
function flushDraft() {
  if (draftTimer !== undefined) {
    window.clearTimeout(draftTimer);
    draftTimer = undefined;
  }
  saveDraftNow();
}

function onVisibilityChange() {
  if (document.visibilityState === "hidden") {
    flushDraft();
  }
}

function restoreDraft() {
  if (isEdit.value) return;

  const draft = readPostDraft(draftUserName());
  if (!draft) return;

  form.title = draft.title;
  form.subtitle = draft.subtitle;
  form.body = draft.body;
  knownTags.value = draft.tags;
  selectedIds.value = draft.tags.map((tag) => tag.ID);
  draftExists.value = true;
  draftRestored.value = true;
}

function dropDraft() {
  if (draftTimer !== undefined) {
    window.clearTimeout(draftTimer);
    draftTimer = undefined;
  }

  const name = draftUserName();
  if (name) {
    clearPostDraft(name);
  }
  draftExists.value = false;
  draftRestored.value = false;
}

function openClearDraft() {
  if (!clearDraftDialog.value?.open) {
    clearDraftDialog.value?.showModal();
  }
}

function closeClearDraft() {
  clearDraftDialog.value?.close();
}

function confirmClearDraft() {
  dropDraft();
  form.title = "";
  form.subtitle = "";
  form.body = "";
  selectedIds.value = [];
  knownTags.value = [];
  errorMsg.value = "";
  showPreview.value = false;
  closeClearDraft();
}

function toggleTag(tag: TagItem) {
  if (selectedIds.value.includes(tag.ID)) {
    selectedIds.value = selectedIds.value.filter((id) => id !== tag.ID);
    errorMsg.value = "";
    return;
  }
  // 记住标签快照，草稿保存和恢复时都能取到完整信息。
  if (!knownTags.value.some((item) => item.ID === tag.ID)) {
    knownTags.value = [...knownTags.value, tag];
  }
  selectedIds.value = [...selectedIds.value, tag.ID];
  errorMsg.value = "";
}

function validate(): string {
  if (!form.title.trim()) return "请输入标题";
  if (!form.body.trim()) return "请输入正文";
  if (bodyLength.value > MAX_BODY) return `正文最多 ${MAX_BODY} 个字符`;
  return "";
}

async function loadPost() {
  if (!postId.value) {
    loadError.value = "文章 ID 无效";
    return;
  }

  loadingPost.value = true;
  loadError.value = "";
  errorMsg.value = "";
  try {
    const post = await fetchPost(postId.value);
    if (!auth.user || post.user !== auth.user.name) {
      loadError.value = "只能编辑自己的文章";
      return;
    }

    form.title = post.title;
    form.subtitle = post.subtitle;
    form.body = post.body ?? "";
    knownTags.value = post.tags ?? [];
    selectedIds.value = (post.tags ?? []).map((tag) => tag.ID);
  } catch (err) {
    loadError.value = isApiError(err) ? err.message : "加载文章失败，请稍后再试";
  } finally {
    loadingPost.value = false;
  }
}

onMounted(() => {
  void loadReviewedTags();
  if (isEdit.value) {
    void loadPost();
  } else {
    // 上次没发布就离开：自动恢复草稿。
    restoreDraft();
  }

  // 移动端切后台不一定触发 beforeunload，用 pagehide/visibilitychange 兜底。
  window.addEventListener("pagehide", flushDraft);
  window.addEventListener("beforeunload", flushDraft);
  document.addEventListener("visibilitychange", onVisibilityChange);
});

onBeforeUnmount(() => {
  if (autoTagTimer !== undefined) window.clearTimeout(autoTagTimer);
  window.removeEventListener("pagehide", flushDraft);
  window.removeEventListener("beforeunload", flushDraft);
  document.removeEventListener("visibilitychange", onVisibilityChange);
  flushDraft();
});

// 输入或标签变化后静默保存。
watch([() => form.title, () => form.subtitle, () => form.body, selectedIds], scheduleDraftSave);

// 正文变化后防抖解析自动标签。
watch(
  () => form.body,
  () => {
    if (autoTagTimer !== undefined) window.clearTimeout(autoTagTimer);
    autoTagTimer = window.setTimeout(() => {
      autoTagTimer = undefined;
      refreshAutoTags();
    }, AUTO_TAG_DELAY_MS);
  },
  { immediate: true },
);

async function onSubmit() {
  if (submitting.value) return;

  errorMsg.value = validate();
  if (errorMsg.value) return;

  const id = isEdit.value ? postId.value : null;
  if (isEdit.value && !id) {
    errorMsg.value = "文章 ID 无效";
    return;
  }

  submitting.value = true;
  try {
    const input = {
      title: form.title.trim(),
      subtitle: form.subtitle.trim(),
      body: form.body,
      tag_ids: selectedIds.value,
    };

    if (id) {
      await updatePost(id, input);
      await router.replace("/my-posts");
    } else {
      const created = await createPost(input);
      // 发布成功：草稿作废，并且不再随卸载写回。
      draftPublished = true;
      dropDraft();
      if (created.xp) {
        auth.applyExperience(created.xp.exp, created.xp.level);
        toast.notifyXp(created.xp);
      }
      await router.replace("/");
    }
  } catch (err) {
    errorMsg.value = isApiError(err)
      ? err.message
      : id
        ? "保存失败，请稍后再试"
        : "发布失败，请稍后再试";
  } finally {
    submitting.value = false;
  }
}
</script>

<template>
  <main class="mx-auto w-full max-w-3xl p-4 sm:p-6 lg:p-8">
    <div class="mb-6 flex items-center justify-between">
      <h1 class="text-2xl font-semibold lg:text-3xl">{{ isEdit ? "编辑文章" : "写文章" }}</h1>
      <router-link :to="isEdit ? '/my-posts' : '/'" class="btn btn-ghost btn-sm">返回</router-link>
    </div>

    <div v-if="loadingPost" class="flex justify-center py-16">
      <span class="loading loading-spinner loading-lg"></span>
    </div>

    <div v-else-if="loadError" role="alert" class="alert alert-error alert-soft">
      <span>{{ loadError }}</span>
      <router-link to="/my-posts" class="btn btn-sm">返回我的文章</router-link>
    </div>

    <form v-else class="card bg-base-100 shadow-sm" novalidate @submit.prevent="onSubmit">
      <div class="card-body gap-4">
        <!-- 上次未发布的草稿已自动恢复 -->
        <div v-if="draftRestored" role="status" class="alert alert-info alert-soft py-2 text-sm">
          <span>已恢复上次未发布的草稿，发布后会自动清除。</span>
        </div>

        <label class="floating-label">
          <span>标题</span>
          <input
            v-model="form.title"
            type="text"
            name="title"
            placeholder="请输入标题"
            maxlength="120"
            class="input input-bordered w-full"
            :disabled="submitting"
          />
        </label>

        <label class="floating-label">
          <span>副标题（可选）</span>
          <input
            v-model="form.subtitle"
            type="text"
            name="subtitle"
            placeholder="一句话摘要"
            maxlength="200"
            class="input input-bordered w-full"
            :disabled="submitting"
          />
        </label>

        <div class="space-y-2">
          <div class="flex items-center justify-between">
            <span class="text-sm font-medium">正文</span>
            <div class="join">
              <button
                type="button"
                class="btn btn-xs join-item"
                :class="{ 'btn-active': !showPreview }"
                @click="showPreview = false"
              >
                编辑
              </button>
              <button
                type="button"
                class="btn btn-xs join-item"
                :class="{ 'btn-active': showPreview }"
                @click="showPreview = true"
              >
                预览
              </button>
            </div>
          </div>

          <textarea
            v-if="!showPreview"
            ref="bodyTextarea"
            v-model="form.body"
            name="body"
            rows="12"
            placeholder="支持 Markdown…"
            class="textarea textarea-bordered w-full"
            :disabled="submitting"
          ></textarea>

          <div
            v-else
            class="markdown-body min-h-48 rounded-box border border-base-300 bg-base-100 p-3 text-sm"
          >
            <div v-if="previewHtml" v-html="previewHtml"></div>
            <p v-else class="text-base-content/50">暂无内容</p>
          </div>

          <p class="text-right text-xs text-base-content/45">
            {{ bodyLength }} / {{ MAX_BODY }} 字
          </p>

          <p class="text-xs text-base-content/50">
            支持 Markdown：标题、列表、引用、表格、链接、图片；代码块会渲染为终端窗口样式。
          </p>
        </div>

        <div class="space-y-2">
          <div class="flex items-center justify-between">
            <span class="text-sm font-medium">标签</span>
            <span class="text-xs text-base-content/60">可多选，点击可取消</span>
          </div>

          <TagPicker
            mode="select"
            :selected-ids="selectedIds"
            :known-tags="knownTags"
            @toggle="toggleTag"
          />

          <div
            v-if="autoTagItems.length > 0"
            class="space-y-1.5 rounded-box border border-base-200 bg-base-200/30 p-2.5"
          >
            <p class="text-xs text-base-content/60">正文自动识别（{{ autoTagItems.length }}）</p>
            <div class="flex flex-wrap gap-1.5">
              <button
                v-for="item in autoTagItems"
                :key="item.name"
                type="button"
                class="badge badge-sm cursor-pointer"
                :style="tagBadgeStyle(item.tag ?? { color: '' })"
                :title="item.tag ? `被 ${item.tag.usage_count} 篇文章使用` : '标签待审核'"
                @click="locateAutoTag(item.name)"
              >
                #{{ item.name }}
              </button>
            </div>
            <p class="text-xs text-base-content/45">
              正文里的 #标签 会自动加入本文，管理员审核后才会出现在首页筛选。
            </p>
          </div>
        </div>

        <div v-if="errorMsg" role="alert" class="alert alert-error alert-soft py-2 text-sm">
          <span>{{ errorMsg }}</span>
        </div>

        <div class="card-actions items-center justify-end">
          <button
            v-if="!isEdit && draftExists"
            type="button"
            class="btn btn-ghost btn-sm text-error"
            :disabled="submitting"
            @click="openClearDraft"
          >
            清除草稿
          </button>
          <button type="submit" class="btn btn-primary" :disabled="submitting">
            <span v-if="submitting" class="loading loading-spinner loading-sm"></span>
            {{ isEdit ? "保存修改" : "发布" }}
          </button>
        </div>
      </div>
    </form>

    <!-- 清除草稿确认弹窗 -->
    <dialog ref="clearDraftDialog" class="modal">
      <div class="modal-box">
        <h3 class="text-lg font-semibold">清除草稿</h3>
        <p class="mt-3 text-sm text-base-content/70">
          当前标题、正文和标签选择都会被清空，且无法恢复。
        </p>
        <div class="modal-action">
          <button type="button" class="btn btn-ghost" @click="closeClearDraft">取消</button>
          <button type="button" class="btn btn-error" @click="confirmClearDraft">清除</button>
        </div>
      </div>
      <form method="dialog" class="modal-backdrop"><button>close</button></form>
    </dialog>
  </main>
</template>
