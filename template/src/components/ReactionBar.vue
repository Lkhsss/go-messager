<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import {
  EMOJI_REACTIONS,
  PRIMARY_REACTIONS,
  reactionCount,
  type ReactionSummary,
  type ReactionType,
} from '@/api/reaction'

const props = withDefaults(
  defineProps<{
    reactions?: ReactionSummary
    myReactions?: ReactionType[]
    // 请求进行中时禁用按钮，避免连点造成来回切换
    busy?: boolean
  }>(),
  {
    myReactions: () => [],
    busy: false,
  },
)

const emit = defineEmits<{ (e: 'react', type: ReactionType): void }>()

const showPicker = ref(false)
const pickerRoot = ref<HTMLElement | null>(null)

function isActive(type: ReactionType): boolean {
  return props.myReactions.includes(type)
}

function count(type: ReactionType): number {
  return reactionCount(props.reactions, type)
}

// 点击数大于 1 的额外表情直接展示在问号后面，方便新用户直接点击
const visibleEmojiReactions = computed(() =>
  EMOJI_REACTIONS.filter((option) => count(option.type) > 1),
)
function onReact(type: ReactionType) {
  if (props.busy) return
  emit('react', type)
}

function onDocumentClick(event: MouseEvent) {
  if (!pickerRoot.value) return
  const target = event.target
  if (target instanceof Node && !pickerRoot.value.contains(target)) {
    showPicker.value = false
  }
}

onMounted(() => document.addEventListener('click', onDocumentClick))
onBeforeUnmount(() => document.removeEventListener('click', onDocumentClick))
</script>

<template>
  <div class="flex flex-wrap items-center gap-2">
    <button v-for="option in PRIMARY_REACTIONS" :key="option.type" type="button" class="reaction-chip"
      :class="{ 'reaction-chip--active': isActive(option.type) }" :disabled="busy"
      :aria-pressed="isActive(option.type)"
      :title="isActive(option.type) ? `${option.label}（再次点击取消）` : option.label" @click="onReact(option.type)">
      <span class="text-base leading-none">{{ option.emoji }}</span>
      <span v-if="count(option.type) > 0" class="text-xs">{{ count(option.type) }}</span>
    </button>

    <button v-for="option in visibleEmojiReactions" :key="`visible-${option.type}`" type="button" class="reaction-chip"
      :class="{ 'reaction-chip--active': isActive(option.type) }" :disabled="busy"
      :aria-pressed="isActive(option.type)"
      :title="isActive(option.type) ? `${option.label}（再次点击取消）` : option.label" @click="onReact(option.type)">
      <span class="text-base leading-none">{{ option.emoji }}</span>
      <span class="text-xs">{{ count(option.type) }}</span>
    </button>

    <div ref="pickerRoot" class="relative">
      <button type="button" class="reaction-chip" :disabled="busy" title="更多表情" aria-haspopup="true"
        :aria-expanded="showPicker" @click.stop="showPicker = !showPicker">
        <span class="text-base leading-none">😊</span>
        <span class="text-xs">＋</span>
      </button>

      <div v-if="showPicker"
        class="absolute bottom-full left-0 z-20 mb-2 flex gap-1 rounded-box border border-base-300 bg-base-100 p-2 shadow-lg">
        <button v-for="option in EMOJI_REACTIONS" :key="option.type" type="button" class="reaction-chip"
          :class="{ 'reaction-chip--active': isActive(option.type) }" :disabled="busy"
          :aria-pressed="isActive(option.type)"
          :title="isActive(option.type) ? `${option.label}（再次点击取消）` : option.label"
          @click.stop="onReact(option.type)">
          <span class="text-base leading-none">{{ option.emoji }}</span>
          <span v-if="count(option.type) > 0" class="text-xs">{{ count(option.type) }}</span>
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.reaction-chip {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  padding: 0.125rem 0.5rem;
  border-radius: 9999px;
  border: 1px solid var(--color-base-300, #d1d5db);
  background: transparent;
  color: inherit;
  font-size: 0.75rem;
  opacity: 0.85;
  transition: background-color 0.15s, border-color 0.15s, opacity 0.15s;
}

.reaction-chip:hover {
  opacity: 1;
  background: color-mix(in srgb, currentColor 6%, transparent);
}

.reaction-chip--active {
  opacity: 1;
  font-weight: 600;
  border-color: var(--color-primary, #3b82f6);
  background: color-mix(in srgb, var(--color-primary, #3b82f6) 14%, transparent);
  color: var(--color-primary, #3b82f6);
}

.reaction-chip:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}
</style>

