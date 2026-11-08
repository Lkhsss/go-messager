<script setup lang="ts">
import { computed } from 'vue'
import { renderMarkdown } from '@/utils/markdown'

const props = defineProps<{
  source: string
}>()

const html = computed(() => renderMarkdown(props.source ?? ''))
</script>

<template>
  <!-- 唯一引用 utils/markdown 的组件：配合 defineAsyncComponent 让列表页不加载 markdown-it / highlight.js -->
  <div class="markdown-body" v-html="html"></div>
</template>
