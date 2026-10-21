<template>
  <div class="relative hidden h-7 w-20 shrink-0 md:block" aria-hidden="true" data-testid="ku-sparkline" @pointermove="onMove" @pointerleave="hover = -1">
    <svg :viewBox="`0 0 ${W} ${H}`" preserveAspectRatio="none" class="h-full w-full overflow-visible">
      <defs><linearGradient :id="gradId" x1="0" y1="0" x2="0" y2="1"><stop offset="0%" stop-color="var(--ku-primary)" stop-opacity="0.3" /><stop offset="100%" stop-color="var(--ku-primary)" stop-opacity="0" /></linearGradient></defs>
      <path v-if="points.length > 1" :d="areaPath" :fill="`url(#${gradId})`" />
      <path v-if="points.length > 1" :d="linePath" fill="none" stroke="var(--ku-primary)" stroke-width="1.5" vector-effect="non-scaling-stroke" stroke-linejoin="round" />
      <template v-if="hover >= 0 && points[hover]">
        <line :x1="points[hover][0]" :x2="points[hover][0]" y1="0" :y2="H" stroke="var(--ku-text-3)" stroke-width="1" vector-effect="non-scaling-stroke" />
      </template>
    </svg>
    <span v-if="hover >= 0 && points[hover]" class="pointer-events-none absolute h-1.5 w-1.5 -translate-x-1/2 -translate-y-1/2 rounded-full bg-ku-primary" :style="{ left: `${(points[hover][0] / W) * 100}%`, top: `${(points[hover][1] / H) * 100}%` }" />
    <div v-if="hover >= 0 && labels[hover]" class="pointer-events-none absolute bottom-full z-20 mb-1.5 -translate-x-1/2 whitespace-nowrap rounded-md bg-gray-900 px-2 py-1 text-[11px] text-white shadow-lg dark:bg-gray-800" :style="{ left: `${(points[hover][0] / W) * 100}%` }" data-testid="ku-sparkline-tip">{{ labels[hover] }}</div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'

let sparkSeq = 0
const props = defineProps<{ values: number[]; labels: string[] }>()
const W = 80
const H = 28
const gradId = `ku-spark-${++sparkSeq}`
const hover = ref(-1)
const points = computed(() => {
  const v = props.values
  if (v.length === 0) return [] as [number, number][]
  const max = Math.max(...v)
  const min = Math.min(...v)
  const span = max - min || 1
  return v.map((y, i) => [v.length === 1 ? W / 2 : (i / (v.length - 1)) * W, H - 2 - ((y - min) / span) * (H - 4)] as [number, number])
})
const linePath = computed(() => points.value.map((p, i) => `${i ? 'L' : 'M'}${p[0]},${p[1]}`).join(''))
const areaPath = computed(() => `${linePath.value}L${W},${H}L0,${H}Z`)
// Snap the pointer to the nearest bucket; touch input has no hover interaction.
function onMove(e: PointerEvent) {
  if (e.pointerType === 'touch' || points.value.length === 0) return
  const rect = (e.currentTarget as HTMLElement).getBoundingClientRect()
  const ratio = rect.width ? (e.clientX - rect.left) / rect.width : 0
  hover.value = Math.max(0, Math.min(points.value.length - 1, Math.round(ratio * (points.value.length - 1))))
}
defineExpose({ hover })
</script>
