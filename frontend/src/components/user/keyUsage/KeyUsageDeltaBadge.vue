<template>
  <span class="inline-flex items-center gap-1 whitespace-nowrap text-xs" data-testid="ku-delta">
    <span v-if="label" class="text-ku-text-2">{{ label }}</span>
    <span class="tabular-nums" :class="toneClass">{{ text }}</span>
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{ current: number; previous?: number | null; label?: string }>()
const pct = computed(() => {
  if (props.previous === null || props.previous === undefined || props.previous === 0) return null
  return ((props.current - props.previous) / props.previous) * 100
})
const text = computed(() => {
  const v = pct.value
  if (v === null) return '—'
  if (v === 0) return '0%'
  const rounded = Math.round(Math.abs(v))
  const body = rounded === 0 ? '<1%' : `${rounded}%`
  return v > 0 ? `▲ +${body}` : `▼ -${body}`
})
const toneClass = computed(() => pct.value === null || pct.value === 0 ? 'text-ku-text-3' : pct.value > 0 ? 'text-ku-up' : 'text-ku-down')
</script>
