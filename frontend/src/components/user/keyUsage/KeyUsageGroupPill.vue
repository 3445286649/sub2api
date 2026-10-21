<template>
  <span class="inline-flex h-6 min-w-0 max-w-full items-center truncate whitespace-nowrap rounded-full px-2.5 text-xs font-medium" :style="pillStyle" :title="title || name" :data-tone="tone">{{ name }}</span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { keyUsageGroupTone } from '@/utils/keyUsageGroupColor'

const props = defineProps<{ name: string; platform?: string | null; title?: string }>()
const tone = computed(() => keyUsageGroupTone(props.platform))
// Text uses the group color; background mixes it at --ku-pill-alpha (15% light / 20% dark).
const pillStyle = computed(() => ({
  color: `var(--ku-group-${tone.value})`,
  background: `color-mix(in srgb, var(--ku-group-${tone.value}) var(--ku-pill-alpha), transparent)`,
}))
</script>
