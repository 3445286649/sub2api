<template>
  <span
    class="grid h-3.5 w-[120px] grid-cols-10 gap-1"
    role="img"
    :aria-label="ariaLabel"
  >
    <span
      v-for="slot in slots"
      :key="slot.key"
      data-probe-slot
      :data-state="slot.state"
      :data-timestamp="slot.point?.timestamp"
      :data-latest="slot.latest ? 'true' : undefined"
      :title="slot.title"
      aria-hidden="true"
      class="h-3.5 min-w-0 rounded-[2px] transition-opacity duration-150 hover:opacity-75"
      :class="[
        slot.state === 'success' && 'bg-emerald-500 dark:bg-emerald-400',
        slot.state === 'failure' && 'bg-red-500 dark:bg-red-400',
        slot.state === 'empty' && 'bg-gray-200 dark:bg-dark-600',
        slot.latest && 'ring-1 ring-gray-500 ring-offset-1 ring-offset-white dark:ring-gray-300 dark:ring-offset-dark-800',
      ]"
    />
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { AccountProbePoint, AccountProbeTrend } from '@/types'

const slotCount = 10

const props = defineProps<{
  trend: AccountProbeTrend
  ariaLabel: string
  successLabel: string
  failureLabel: string
  emptyLabel: string
}>()

type ProbeSlot = {
  key: string
  state: 'success' | 'failure' | 'empty'
  point?: AccountProbePoint
  latest: boolean
  title: string
}

function pointTitle(point: AccountProbePoint, state: ProbeSlot['state']): string {
  const parsed = new Date(point.timestamp)
  const time = Number.isNaN(parsed.getTime()) ? point.timestamp : parsed.toLocaleString()
  if (state === 'failure') {
    const reason = point.error_message || point.error_category
    return [props.failureLabel, time, reason].filter(Boolean).join(' · ')
  }
  const latency = point.latency_ms == null ? '' : `${point.latency_ms} ms`
  return [props.successLabel, time, latency].filter(Boolean).join(' · ')
}

const slots = computed<ProbeSlot[]>(() => {
  const points = props.trend.points.slice(-slotCount)
  const emptySlots: ProbeSlot[] = Array.from({ length: slotCount - points.length }, (_, index) => ({
    key: `empty-${index}`,
    state: 'empty',
    latest: false,
    title: props.emptyLabel,
  }))
  const pointSlots = points.map<ProbeSlot>((point, index) => {
    const state: ProbeSlot['state'] = point.failure_count > 0 ? 'failure' : 'success'
    return {
      key: `${point.timestamp}-${index}`,
      state,
      point,
      latest: index === points.length - 1,
      title: pointTitle(point, state),
    }
  })
  return [...emptySlots, ...pointSlots]
})
</script>
