<template>
  <div ref="root" class="mt-4 border-t border-slate-200/80 pt-3 dark:border-dark-700/60">
    <div class="mb-2 flex justify-between text-[10px] font-semibold uppercase tracking-widest text-gray-400">
      <span>{{ length ? t('channelMonitorV2.v3.intervals', { n: length }) : t('channelMonitorV2.v3.noTraffic') }}</span>
    </div>

    <div class="v3-timeline-bars" @mouseleave="clearHoveredBar">
      <div
        v-for="(bar, index) in displayBars"
        :key="bar.key"
        class="v3-bar-slot"
        :style="entranceStyle(bar.key, index)"
        @mouseenter="setHoveredBar(index, $event)"
      >
        <button
          type="button"
          class="v3-bar-hitbox"
          :class="{
            'is-active': hoveredBarIndex === index,
          }"
          :aria-label="bar.title || '-'"
          :aria-pressed="pinnedIndex === index"
          @click="togglePinned(index, $event)"
          @focus="setHoveredBar(index, $event)"
          @blur="clearHoveredBar"
        >
          <span class="v3-bar-visual" :style="barMotionStyle(index)" aria-hidden="true">
            <span
              class="v3-soft-glass-bar"
              :class="[bar.colorClass, { 'is-probe': bar.probe }]"
              :style="{ backgroundColor: bar.color, height: `${bar.height}%` }"
            />
          </span>
        </button>

        <Teleport to="body">
          <Transition name="v3-timeline-tooltip">
            <div
              v-if="hoveredBarIndex === index && bar.title"
              class="v3-timeline-tooltip"
              role="tooltip"
              :style="tooltipStyle"
            >
              <div class="mb-2 flex items-center justify-between gap-3 border-b border-white/20 pb-2">
                <span>{{ bar.time }}</span><span>{{ bar.status }}</span>
              </div>
              <dl class="grid grid-cols-[1fr_auto] gap-x-5 gap-y-1.5">
                <template v-for="metric in bar.metrics" :key="metric.label">
                  <dt class="text-gray-300">{{ metric.label }}</dt><dd class="text-right tabular-nums">{{ metric.value }}</dd>
                </template>
              </dl>
              <p v-if="bar.note" class="mt-2 text-gray-300">{{ bar.note }}</p>
            </div>
          </Transition>
        </Teleport>
      </div>
    </div>

    <div v-if="length" class="mt-1 flex justify-between text-[9px] uppercase tracking-widest text-gray-400">
      <span>{{ t('monitorCommon.past') }}</span>
      <span>{{ t('monitorCommon.now') }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onMounted, onBeforeUnmount, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { formatMonitorMs, formatMonitorPercent } from '@/features/channel-monitor-v2/monitorFormat'

import { cardState, errorColor, cardHealthStyles, type CardTimelineBucket } from './cardHealth'

const props = withDefaults(defineProps<{
  buckets?: CardTimelineBucket[]
  length?: number
}>(), {
  buckets: () => [],
  length: 18,
})

const { t, locale } = useI18n()
const hoveredBarIndex = ref<number | null>(null)
const pinnedIndex = ref<number | null>(null)
const root = ref<HTMLElement | null>(null)
const entranceDelays = new Map<string, number>()
let mounted = false
function entranceStyle(key: string, index: number) {
  if (!entranceDelays.has(key)) entranceDelays.set(key, mounted ? 0 : index * (300 / Math.max(1, props.length - 1)))
  return { '--entrance-delay': `${entranceDelays.get(key)}ms` }
}
const tooltipPosition = ref({ left: 0, top: 0, x: '-50%', y: '-100%' })

function setHoveredBar(index: number, event?: Event) {
  if (pinnedIndex.value !== null && pinnedIndex.value !== index) return
  hoveredBarIndex.value = index
  const target = event?.currentTarget
  if (!(target instanceof HTMLElement) || typeof window === 'undefined') return

  const rect = target.getBoundingClientRect()
  const viewportGutter = 16
  const maxTooltipWidth = Math.min(280, window.innerWidth - viewportGutter * 2)
  const center = rect.left + rect.width / 2
  const below = rect.top < 180
  const top = below ? rect.bottom + 8 : rect.top - 8
  const y = below ? '0%' : '-100%'
  if (center + maxTooltipWidth / 2 > window.innerWidth - viewportGutter) {
    tooltipPosition.value = { left: window.innerWidth - viewportGutter, top, x: '-100%', y }
  } else if (center - maxTooltipWidth / 2 < viewportGutter) {
    tooltipPosition.value = { left: viewportGutter, top, x: '0%', y }
  } else {
    tooltipPosition.value = { left: center, top, x: '-50%', y }
  }
}

function clearHoveredBar() {
  if (pinnedIndex.value !== null) return
  hoveredBarIndex.value = null
}

function closeTooltip() {
  pinnedIndex.value = null
  hoveredBarIndex.value = null
}
function togglePinned(index: number, event: Event) {
  if (pinnedIndex.value === index) { closeTooltip(); return }
  pinnedIndex.value = null
  setHoveredBar(index, event)
  pinnedIndex.value = index
}
function outside(event: PointerEvent) {
  if (event.target instanceof Node && !root.value?.contains(event.target)) closeTooltip()
}
function escape(event: KeyboardEvent) { if (event.key === 'Escape') closeTooltip() }
onMounted(() => {
  mounted = true
  document.addEventListener('pointerdown', outside)
  document.addEventListener('keydown', escape)
  window.addEventListener('scroll', closeTooltip, true)
  window.addEventListener('resize', closeTooltip)
})
onBeforeUnmount(() => {
  document.removeEventListener('pointerdown', outside)
  document.removeEventListener('keydown', escape)
  window.removeEventListener('scroll', closeTooltip, true)
  window.removeEventListener('resize', closeTooltip)
})

function barMotionStyle(index: number) {
  const active = hoveredBarIndex.value === index
  return {
    // Keep every slot's horizontal footprint fixed so edge bars cannot expand
    // the page or move the pointer between neighboring hit targets.
    '--bar-scale-x': '1',
    '--bar-scale-y': active ? '1.05' : '1',
    '--bar-opacity': '1',
    '--bar-lift': active ? '-1px' : '0px',
  }
}

interface TimelineBar {
  probe?: boolean
  height: number
  color?: string
  note?: string
  key: string
  colorClass: string
  title: string
  time: string
  status: string
  metrics: Array<{ label: string; value: string }>
}

function formatBucketTime(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return new Intl.DateTimeFormat(locale.value || undefined, {
    month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit',
  }).format(date)
}

const displayBars = computed<TimelineBar[]>(() => {
  if (props.length <= 0) return []
  const real = [...props.buckets]
    .sort((a, b) => Date.parse(a.bucket_start) - Date.parse(b.bucket_start))
    .slice(-props.length)
  const bars: TimelineBar[] = Array.from({ length: Math.max(0, props.length - real.length) }, (_, index) => ({
    key: `empty-${index}`, colorClass: `${cardHealthStyles.unknown.bar} is-empty`, height: 100,
    title: t('channelMonitorV2.noRecord'), time: '-', status: t('channelMonitorV2.noRecord'), metrics: [],
  }))
  for (const bucket of real) {
    const state = cardState(bucket)
    const isProbe = !!bucket.probe && !bucket.incomplete && bucket.metrics.traffic_state !== 'traffic' && bucket.probe.success_count + bucket.probe.failure_count > 0
    const style = cardHealthStyles[state]
    const time = `${formatBucketTime(bucket.bucket_start)} - ${formatBucketTime(bucket.bucket_end || bucket.bucket_start)}`
    const status = t(`channelMonitorV2.v3.${state}`)
    const hasMetrics = !isProbe && state !== 'noTraffic' && state !== 'unknown'
    const values = {
      availability: !hasMetrics ? '-' : formatMonitorPercent(1 - bucket.metrics.error_rate, locale.value),
      error: !hasMetrics ? '-' : formatMonitorPercent(bucket.metrics.error_rate, locale.value),
      cache: !hasMetrics || !bucket.health.cache || bucket.health.cache === 'unknown' ? '-' : formatMonitorPercent(bucket.metrics.cache_rate, locale.value),
      ttft: !hasMetrics ? '-' : formatMonitorMs(bucket.metrics.ttft.p50_ms),
    }
    bars.push({
      key: bucket.bucket_start, probe: isProbe, colorClass: `${style.bar}${state === 'unknown' ? ' is-pending' : state === 'noTraffic' ? ' is-empty' : ''}`, height: state === 'noTraffic' || state === 'unknown' ? 100 : style.height, time, status,
      color: hasMetrics ? errorColor(bucket.metrics.error_rate) : undefined,
      note: state === 'noTraffic' ? t('channelMonitorV2.v3.noRequests') : hasMetrics && bucket.health.error_rate === 'unknown' ? t('channelMonitorV2.v3.smallSample') : '',
      title: isProbe ? `${time} · ${t('channelMonitorV2.v3.probeSource')} · ${status}` : `${status} · ${t('channelMonitorV2.referenceCard.timelineTooltip', { time, ...values })}`,
      metrics: isProbe ? [
        { label: t('channelMonitorV2.v3.source'), value: t('channelMonitorV2.v3.probeSource') },
        { label: t('channelMonitorV2.v3.probeModel'), value: bucket.probeModel || '-' },
        { label: t('channelMonitorV2.v3.probeSuccess'), value: String(bucket.probe!.success_count) },
        { label: t('channelMonitorV2.v3.probeFailure'), value: String(bucket.probe!.failure_count) },
        { label: t('channelMonitorV2.v3.probeRate'), value: formatMonitorPercent(bucket.probe!.success_count / (bucket.probe!.success_count + bucket.probe!.failure_count), locale.value) },
        { label: t('channelMonitorV2.v3.lastProbe'), value: formatBucketTime(bucket.probe!.last_checked_at) },
      ] : [
        { label: t('channelMonitorV2.referenceCard.successRate'), value: values.availability },
        { label: t('channelMonitorV2.referenceCard.cacheRate'), value: values.cache },
        { label: t('channelMonitorV2.metrics.errorRate'), value: values.error },
        { label: t('channelMonitorV2.referenceCard.ttft'), value: values.ttft },
      ],
    })
  }
  return bars
})
watch(() => displayBars.value.map(bar => bar.title).join('|'), closeTooltip)
watch(() => displayBars.value.map(bar => bar.key), keys => {
  const visible = new Set(keys)
  for (const key of entranceDelays.keys()) if (!visible.has(key)) entranceDelays.delete(key)
})

const tooltipStyle = computed(() => ({
  '--tooltip-left': `${tooltipPosition.value.left}px`,
  '--tooltip-top': `${tooltipPosition.value.top}px`,
  '--tooltip-x': tooltipPosition.value.x,
  '--tooltip-y': tooltipPosition.value.y,
}))
</script>

<style scoped>
.v3-soft-glass-bar {
	display: block;
	width: 100%;
	height: 100%;
	min-height: 3px;
	border-radius: 3px;
  transform-origin: bottom;
}

.v3-timeline-bars {
	display: flex;
	position: relative;
	height: 20px;
	width: 100%;
	gap: 4px;
	isolation: isolate;
}

.v3-soft-glass-bar.is-pending {
  background-image: repeating-linear-gradient(135deg, transparent 0 2px, rgba(100, 116, 139, 0.7) 2px 4px);
}

.v3-bar-slot {
	position: relative;
	display: flex;
	align-items: flex-end;
	min-width: 0;
	height: 100%;
	flex: 1 1 0%;
  border-radius: 3px;
  animation: v3-slot-enter 200ms ease-out both;
  animation-delay: var(--entrance-delay, 0ms);
}

.v3-bar-slot::before {
  content: '';
  position: absolute;
  bottom: 0;
  width: 100%;
  height: 20px;
  border-radius: 3px;
  pointer-events: none;
  background: rgba(148, 163, 184, 0.10);
  box-shadow: inset 0 0 0 1px rgba(148, 163, 184, 0.12);
}

.v3-soft-glass-bar.is-empty { opacity: 0.3; }
.v3-soft-glass-bar.is-pending { opacity: 0.45; }
.v3-soft-glass-bar.is-probe { outline: 1px solid rgba(255,255,255,0.7); outline-offset: -2px; }

.v3-bar-hitbox {
	position: relative;
	display: flex;
	align-items: flex-end;
	width: 100%;
	height: 100%;
	min-width: 0;
	padding: 0;
	border: 0;
	background: transparent;
	cursor: crosshair;
	appearance: none;
}

.v3-bar-visual {
	display: flex;
	align-items: flex-end;
	width: 100%;
	height: 100%;
	transform: translateY(var(--bar-lift, 0px)) scaleX(var(--bar-scale-x, 1)) scaleY(var(--bar-scale-y, 1));
	transform-origin: center bottom;
	opacity: var(--bar-opacity, 1);
	pointer-events: none;
	will-change: transform, opacity;
	transition: transform 240ms cubic-bezier(0.22, 1, 0.36, 1), opacity 220ms ease;
}

.v3-bar-hitbox:focus-visible {
	outline: 2px solid rgb(59 130 246 / 0.6);
	outline-offset: 2px;
	border-radius: 4px;
}

.v3-bar-hitbox.is-active {
	z-index: 3;
}

.v3-bar-hitbox.is-active .v3-soft-glass-bar {
	filter: saturate(1.12) brightness(1.05);
	box-shadow: 0 4px 10px rgb(15 118 110 / 0.24);
}

.v3-bar-hitbox.is-pressed {
	z-index: 2;
}

.v3-timeline-tooltip {
	position: fixed;
	left: var(--tooltip-left, 50%);
	top: var(--tooltip-top, 0px);
	z-index: 50;
	width: 260px;
	max-width: min(280px, calc(100vw - 32px));
	transform: translateX(var(--tooltip-x, -50%)) translateY(var(--tooltip-y, -100%));
	border: 1px solid rgb(255 255 255 / 0.84);
	border-radius: 9px;
	background: rgb(15 23 42);
	padding: 12px;
	color: rgb(248 250 252);
	font-size: 12px;
	font-weight: 600;
	line-height: 1.35;
	letter-spacing: 0;
	white-space: normal;
	box-shadow: 0 10px 24px rgb(15 23 42 / 0.2);
	pointer-events: none;
}



.v3-timeline-tooltip-enter-active,
.v3-timeline-tooltip-leave-active {
	transition: opacity 100ms ease, transform 120ms cubic-bezier(0.22, 1, 0.36, 1);
}

.v3-timeline-tooltip-enter-from,
.v3-timeline-tooltip-leave-to {
	opacity: 0;
	transform: translateX(var(--tooltip-x, -50%)) translateY(var(--tooltip-y, -100%)) scale(0.96);
}

@keyframes v3-slot-enter {
  from {
    transform: translateY(2px);
    opacity: 0;
  }
  to {
    transform: translateY(0);
    opacity: 1;
  }
}

@media (prefers-reduced-motion: reduce) {
  .v3-bar-slot {
    animation: none;
  }

  .v3-bar-hitbox,
  .v3-bar-visual,
  .v3-timeline-tooltip-enter-active,
  .v3-timeline-tooltip-leave-active {
    transition: none;
  }
}

@media (max-width: 639px) {
  .v3-timeline-bars { height: 36px; }
  .v3-bar-visual { height: 20px; }
}
</style>
