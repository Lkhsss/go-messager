<script setup lang="ts">
import { computed } from "vue";
import { renderMarkdown } from "@/utils/markdown";
import type { TagItem } from "@/api/tag";

const props = defineProps<{
  source: string;
  // 已审核标签，用于正文药丸取色；不传时全部用默认色（例如评论）。
  tags?: Pick<TagItem, "name" | "color">[];
}>();

const html = computed(() => renderMarkdown(props.source ?? "", props.tags));
</script>

<template>
  <!-- 唯一引用 markdown 工具的组件，异步加载。 -->
  <div class="markdown-body" v-html="html"></div>
</template>
