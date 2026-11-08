<script setup lang="ts">
import { computed } from 'vue'
import { renderMarkdown } from '@/utils/markdown'

const props = defineProps<{
  source: string
}>()

const html = computed(() => renderMarkdown(props.source ?? ''))
</script>

<template>
  <!-- 唯一引用 markdown 工具的组件，异步加载。 -->
  <div class="markdown-body" v-html="html"></div>
</template>
