<template>
  <svg
    viewBox="0 0 120 28"
    class="block h-7 w-[120px] overflow-visible"
    role="img"
    :aria-label="ariaLabel"
  >
    <line x1="1" y1="27" x2="119" y2="27" class="stroke-gray-200 dark:stroke-dark-600" stroke-width="1" />
    <polyline
      v-for="(segment, index) in lineSegments"
      :key="index"
      :points="segment"
      fill="none"
      class="stroke-emerald-500 dark:stroke-emerald-400"
      stroke-width="1.5"
      stroke-linecap="round"
      stroke-linejoin="round"
    />
    <circle
      v-for="point in successPoints"
      :key="`success-${point.key}`"
      :cx="point.x"
      :cy="point.y"
      r="1.4"
      class="fill-emerald-500 dark:fill-emerald-400"
    />
    <g
      v-for="point in failurePoints"
      :key="`failure-${point.key}`"
      class="stroke-red-500 dark:stroke-red-400"
      stroke-width="1.5"
      stroke-linecap="round"
    >
      <line :x1="point.x - 2.5" y1="11.5" :x2="point.x + 2.5" y2="16.5" />
      <line :x1="point.x + 2.5" y1="11.5" :x2="point.x - 2.5" y2="16.5" />
    </g>
  </svg>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { AccountProbeTrend } from '@/types'

const props = defineProps<{
  trend: AccountProbeTrend
  ariaLabel: string
}>()

const fromMs = computed(() => Date.parse(props.trend.from))
const toMs = computed(() => Date.parse(props.trend.to))
const latencies = computed(() => props.trend.points.flatMap(point => point.latency_ms == null ? [] : [point.latency_ms]))
const minLatency = computed(() => Math.min(...latencies.value, 0))
const maxLatency = computed(() => Math.max(...latencies.value, 1))

function xFor(timestamp: string): number {
  const span = Math.max(toMs.value - fromMs.value, 1)
  return 2 + ((Date.parse(timestamp) - fromMs.value) / span) * 116
}

function yFor(latency: number): number {
  const span = Math.max(maxLatency.value - minLatency.value, 1)
  return 25 - ((latency - minLatency.value) / span) * 22
}

const successPoints = computed(() => props.trend.points.flatMap((point, index) => {
  if (point.latency_ms == null) return []
  return [{ key: `${point.timestamp}-${index}`, x: xFor(point.timestamp), y: yFor(point.latency_ms) }]
}))

const lineSegments = computed(() => {
  const segments: string[] = []
  let current: string[] = []
  for (const point of props.trend.points) {
    if (point.latency_ms == null) {
      if (current.length > 1) segments.push(current.join(' '))
      current = []
      continue
    }
    current.push(`${xFor(point.timestamp)},${yFor(point.latency_ms)}`)
  }
  if (current.length > 1) segments.push(current.join(' '))
  return segments
})

const failurePoints = computed(() => props.trend.points.flatMap((point, index) => {
  if (point.failure_count <= 0) return []
  return [{ key: `${point.timestamp}-${index}`, x: xFor(point.timestamp) }]
}))
</script>
