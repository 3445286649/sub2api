<template>
  <section class="min-w-0" data-testid="ku-spend-area">
    <h4 class="mb-2 text-sm font-semibold text-ku-text">{{ title }}</h4>
    <div class="h-[100px] md:h-[120px]">
      <Line :key="themeKey" :data="chartData" :options="chartOptions" />
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { Chart as ChartJS, CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Filler, type ScriptableContext } from 'chart.js'
import { Line } from 'vue-chartjs'
import { THEME_CHANGE_EVENT, useTheme } from '@/composables/useTheme'
import { formatKeyUsageMoney } from '@/utils/keyUsageMoney'
import type { APIKeyUsageBucket } from '@/api/usage'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Filler)

const props = defineProps<{ series: APIKeyUsageBucket[]; granularity: 'hour' | 'day'; title: string }>()
const { isDark } = useTheme()
const palette = ref(readPalette())
// Chart.js cannot read CSS variables; re-read them on theme change and remount.
const themeKey = computed(() => `${isDark.value}-${palette.value.primary}`)
function readPalette() {
  const css = typeof window === 'undefined' ? null : getComputedStyle(document.documentElement)
  const v = (name: string, fallback: string) => css?.getPropertyValue(name).trim() || fallback
  return { primary: v('--ku-primary', '#14b8a6'), grid: v('--ku-border', '#eef0f3'), text: v('--ku-text-3', '#9ca3af') }
}
const refresh = () => { palette.value = readPalette() }
// Canvas color stops need plain rgba(); expand #rrggbb tokens here.
function withAlpha(hex: string, alpha: number) {
  const m = /^#([0-9a-f]{6})$/i.exec(hex.trim())
  if (!m) return alpha > 0 ? hex : 'transparent'
  const n = Number.parseInt(m[1], 16)
  return `rgba(${n >> 16}, ${(n >> 8) & 255}, ${n & 255}, ${alpha})`
}
onMounted(() => window.addEventListener(THEME_CHANGE_EVENT, refresh))
onBeforeUnmount(() => window.removeEventListener(THEME_CHANGE_EVENT, refresh))

const pad = (n: number) => String(n).padStart(2, '0')
function bucketLabel(iso: string) {
  const d = new Date(iso)
  return props.granularity === 'hour' ? `${pad(d.getHours())}:${pad(d.getMinutes())}` : `${pad(d.getMonth() + 1)}/${pad(d.getDate())}`
}
const chartData = computed(() => ({
  labels: props.series.map(b => bucketLabel(b.bucket)),
  datasets: [{
    data: props.series.map(b => b.actual_cost),
    borderColor: palette.value.primary,
    borderWidth: 2,
    pointRadius: 0,
    pointHoverRadius: 3,
    tension: 0.3,
    fill: true,
    backgroundColor: (ctx: ScriptableContext<'line'>) => {
      const area = ctx.chart.chartArea
      if (!area) return 'transparent'
      const g = ctx.chart.ctx.createLinearGradient(0, area.top, 0, area.bottom)
      g.addColorStop(0, withAlpha(palette.value.primary, 0.3))
      g.addColorStop(1, withAlpha(palette.value.primary, 0))
      return g
    },
  }],
}))
const chartOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  animation: false as const,
  interaction: { intersect: false, mode: 'index' as const },
  plugins: { legend: { display: false }, tooltip: { callbacks: { label: (c: { parsed: { y: number | null } }) => formatKeyUsageMoney(c.parsed.y ?? 0) } } },
  scales: {
    x: { grid: { display: false }, ticks: { color: palette.value.text, maxTicksLimit: 4, maxRotation: 0, font: { size: 11 } } },
    y: { beginAtZero: true, grid: { color: palette.value.grid }, border: { display: false }, ticks: { color: palette.value.text, maxTicksLimit: 3, font: { size: 11 }, callback: (v: string | number) => formatKeyUsageMoney(Number(v)) } },
  },
}))
</script>
