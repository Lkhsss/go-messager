<script setup lang="ts">
import { computed, defineAsyncComponent, ref } from "vue";
import { useRoute } from "vue-router";
import type { CommentItem } from "@/api/post";
import { createComment } from "@/api/post";
import { toggleReaction, type ReactionType } from "@/api/reaction";
import { isApiError } from "@/api/http";
import { useLoginPromptStore } from "@/stores/loginPrompt";
import UserAvatar from "@/components/UserAvatar.vue";
import ReactionBar from "@/components/ReactionBar.vue";
import { useAuthStore } from "@/stores/auth";
import { useToastStore } from "@/stores/toast";

// 评论正文同样按需加载 Markdown 渲染链路
const MarkdownContent = defineAsyncComponent(() => import("@/components/MarkdownContent.vue"));

defineOptions({ name: "CommentNode" });

const props = withDefaults(
  defineProps<{
    comment: CommentItem;
    postId: number;
    depth?: number;
  }>(),
  { depth: 0 },
);

const emit = defineEmits<{
  (e: "created"): void;
  (e: "delete", comment: CommentItem): void;
}>();

const auth = useAuthStore();
const loginPrompt = useLoginPromptStore();
const toast = useToastStore();
const route = useRoute();
const canDelete = computed(() => auth.isAdmin || auth.user?.name === props.comment.user);

const replying = ref(false);
const replyBody = ref("");
const submitting = ref(false);
const reacting = ref(false);
const errorMsg = ref("");

function formatDate(value: string): string {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString("zh-CN");
}

function toggleReply() {
  if (!auth.isLoggedIn) {
    loginPrompt.show(route.fullPath);
    return;
  }
  replying.value = !replying.value;
}

async function submitReply() {
  if (submitting.value) return;

  if (!auth.isLoggedIn) {
    loginPrompt.show(route.fullPath);
    return;
  }

  const body = replyBody.value.trim();
  if (!body) {
    errorMsg.value = "请输入回复内容";
    return;
  }

  submitting.value = true;
  errorMsg.value = "";
  try {
    const created = await createComment(props.postId, body, props.comment.ID);
    if (created.xp) {
      auth.applyExperience(created.xp.exp, created.xp.level);
      toast.notifyXp(created.xp);
    }
    replyBody.value = "";
    replying.value = false;
    emit("created");
  } catch (err) {
    errorMsg.value = isApiError(err) ? err.message : "回复失败，请稍后再试";
  } finally {
    submitting.value = false;
  }
}

async function onReact(type: ReactionType) {
  if (reacting.value) return;
  if (!auth.isLoggedIn) {
    loginPrompt.show(route.fullPath);
    return;
  }

  reacting.value = true;
  errorMsg.value = "";
  try {
    const result = await toggleReaction(props.comment.ID, type);
    props.comment.reactions = result.reactions;
    props.comment.my_reactions = result.my_reactions;
    if (result.xp) {
      auth.applyExperience(result.xp.exp, result.xp.level);
      toast.notifyXp(result.xp);
    }
  } catch (err) {
    errorMsg.value = isApiError(err) ? err.message : "操作失败，请稍后再试";
  } finally {
    reacting.value = false;
  }
}
</script>

<template>
  <article
    class="rounded-box border border-base-200 p-3 sm:p-4"
    :class="{ 'ml-3 border-l-4 border-l-base-300 sm:ml-6': depth > 0 }"
  >
    <header class="flex items-center gap-2">
      <UserAvatar :name="comment.user" :version="comment.user_avatar_version" :size="32" />
      <div class="flex min-w-0 items-center gap-2">
        <p class="truncate text-sm font-medium">{{ comment.user }}</p>
        <span
          v-if="comment.user_level"
          class="badge badge-sm shrink-0 rounded-full border-primary/30 bg-primary/10 font-mono text-primary"
          :title="`等级 ${comment.user_level}`"
          >Lv.{{ comment.user_level }}</span
        >
        <p class="text-[0.65rem] text-base-content/40">{{ formatDate(comment.CreatedAt) }}</p>
      </div>
    </header>

    <MarkdownContent :source="comment.body ?? ''" class="mt-2 text-sm text-base-content/80" />

    <div class="mt-2 flex flex-wrap items-center gap-2">
      <ReactionBar
        :reactions="comment.reactions"
        :my-reactions="comment.my_reactions"
        :busy="reacting"
        @react="onReact"
      />
      <button type="button" class="btn btn-ghost btn-xs" @click="toggleReply">回复</button>
      <button
        v-if="canDelete"
        type="button"
        class="btn btn-ghost btn-xs text-error"
        @click="$emit('delete', comment)"
      >
        删除
      </button>
    </div>

    <div v-if="replying" class="mt-2 space-y-2">
      <textarea
        v-model="replyBody"
        rows="3"
        class="textarea textarea-bordered w-full text-sm"
        placeholder="写下回复…"
        :disabled="submitting"
      ></textarea>
      <div class="flex justify-end gap-2">
        <button
          type="button"
          class="btn btn-ghost btn-xs"
          :disabled="submitting"
          @click="
            replying = false;
            replyBody = '';
            errorMsg = '';
          "
        >
          取消
        </button>
        <button
          type="button"
          class="btn btn-primary btn-xs"
          :disabled="submitting"
          @click="submitReply"
        >
          <span v-if="submitting" class="loading loading-spinner loading-xs"></span>
          回复
        </button>
      </div>
    </div>

    <div v-if="errorMsg" class="mt-1 text-xs text-error">{{ errorMsg }}</div>

    <div v-if="comment.children.length" class="mt-3 space-y-3">
      <CommentNode
        v-for="child in comment.children"
        :key="child.ID"
        :comment="child"
        :post-id="postId"
        :depth="depth + 1"
        @created="$emit('created')"
        @delete="$emit('delete', $event)"
      />
    </div>
  </article>
</template>
