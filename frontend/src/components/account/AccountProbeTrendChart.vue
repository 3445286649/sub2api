<template>
  <div class="w-full overflow-hidden rounded-md border border-gray-200 bg-gray-50/40 dark:border-dark-700 dark:bg-dark-900/30">
    <svg viewBox="0 0 720 220" class="block h-[220px] w-full" role="img" :aria-label="ariaLabel">
      <g class="stroke-gray-200 dark:stroke-dark-700" stroke-width="1">
        <line v-for="y in gridY" :key="y" x1="50" :y1="y" x2="704" :y2="y" />
      </g>
      <g class="fill-gray-400 text-[10px] dark:fill-gray-500">
        <text v-for="label in yLabels" :key="label.y" x="44" :y="label.y + 3" text-anchor="end">{{ label.text }}</text>
        <text v-for="label in xLabels" :key="label.x" :x="label.x" y="210" text-anchor="middle">{{ label.text }}</text>
      </g>
      <polyline
        v-for="(segment, index) in lineSegments"
        :key="index"
        :points="segment"
        fill="none"
        class="stroke-emerald-500 dark:stroke-emerald-400"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
      <circle v-for="point in successPoints" :key="point.key" :cx="point.x" :cy="point.y" r="2.5" class="fill-emerald-500 dark:fill-emerald-400" />
      <g v-for="point in failurePoints" :key="point.key" class="stroke-red-500 dark:stroke-red-400" stroke-width="2" stroke-linecap="round">
        <line :x1="point.x - 4" :y1="point.y - 4" :x2="point.x + 4" :y2="point.y + 4" />
        <line :x1="point.x + 4" :y1="point.y - 4" :x2="point.x - 4" :y2="point.y + 4" />
      </g>
      <text v-if="trend.points.length === 0" x="377" y="112" text-anchor="middle" class="fill-gray-400 text-xs dark:fill-gray-500">{{ emptyText }}</text>
    </svg>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { AccountProbeTrend } from '@/types'

const props = defineProps<{ trend: AccountProbeTrend; ariaLabel: string; emptyText: string }>()
const left = 50
const right = 704
const top = 16
const bottom = 190
const gridY = [top, top + (bottom - top) / 2, bottom]
const fromMs = computed(() => Date.parse(props.trend.from))
const toMs = computed(() => Date.parse(props.trend.to))
const latencies = computed(() => props.trend.points.flatMap(point => point.latency_ms == null ? [] : [point.latency_ms]))
const minLatency = computed(() => Math.min(...latencies.value, 0))
const maxLatency = computed(() => Math.max(...latencies.value, 1))

function xFor(timestamp: string): number {
  return left + ((Date.parse(timestamp) - fromMs.value) / Math.max(toMs.value - fromMs.value, 1)) * (right - left)
}
function yFor(latency: number): number {
  return bottom - ((latency - minLatency.value) / Math.max(maxLatency.value - minLatency.value, 1)) * (bottom - top)
}
function formatLatency(value: number): string {
  return value >= 1000 ? `${(value / 1000).toFixed(value >= 10000 ? 0 : 1)}s` : `${Math.round(value)}ms`
}
function formatTime(value: number): string {
  const date = new Date(value)
  return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}

const yLabels = computed(() => gridY.map((y, index) => ({
  y,
  text: formatLatency(maxLatency.value - (index / 2) * (maxLatency.value - minLatency.value)),
})))
const xLabels = computed(() => [0, 0.25, 0.5, 0.75, 1].map(ratio => ({
  x: left + ratio * (right - left),
  text: formatTime(fromMs.value + ratio * (toMs.value - fromMs.value)),
})))
const successPoints = computed(() => props.trend.points.flatMap((point, index) => point.latency_ms == null ? [] : [{ key: `${point.timestamp}-${index}`, x: xFor(point.timestamp), y: yFor(point.latency_ms) }]))
const lineSegments = computed(() => {
  const segments: string[] = []
  let current: string[] = []
  for (const point of props.trend.points) {
    if (point.latency_ms == null) {
      if (current.length > 1) segments.push(current.join(' '))
      current = []
    } else {
      current.push(`${xFor(point.timestamp)},${yFor(point.latency_ms)}`)
    }
  }
  if (current.length > 1) segments.push(current.join(' '))
  return segments
})
const failurePoints = computed(() => props.trend.points.flatMap((point, index) => point.failure_count <= 0 ? [] : [{ key: `${point.timestamp}-${index}`, x: xFor(point.timestamp), y: point.latency_ms == null ? (top + bottom) / 2 : yFor(point.latency_ms) }]))
</script>
