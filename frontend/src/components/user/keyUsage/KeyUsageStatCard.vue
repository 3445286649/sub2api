<template>
  <div class="min-w-0 rounded-xl border border-ku-border bg-ku-card p-3 shadow-card md:p-4" :class="compact ? 'md:p-4' : ''" data-testid="ku-stat">
    <p class="truncate text-xs text-ku-text-2">{{ label }}</p>
    <div class="mt-1 flex min-w-0 items-end justify-between gap-2">
      <div class="min-w-0">
        <p class="truncate font-semibold tabular-nums text-ku-text" :class="compact ? 'text-[17px] md:text-2xl' : 'text-[22px] md:text-[28px] md:leading-9'" :title="valueTitle">{{ value }}</p>
        <p v-if="caption" class="mt-0.5 text-xs text-ku-text-3">{{ caption }}</p>
        <KeyUsageDeltaBadge v-if="compare" class="mt-1" :current="compare.current" :previous="compare.previous" :label="compareLabel" />
      </div>
      <slot name="trend" />
    </div>
  </div>
</template>

<script setup lang="ts">
import KeyUsageDeltaBadge from './KeyUsageDeltaBadge.vue'

defineProps<{
  label: string
  value: string
  valueTitle?: string
  caption?: string
  compare?: { current: number; previous?: number | null }
  compareLabel?: string
  compact?: boolean
}>()
</script>
