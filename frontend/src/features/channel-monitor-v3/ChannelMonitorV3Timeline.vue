<template>
  <div class="mt-4 border-t border-slate-200/80 pt-3 dark:border-dark-700/60">
    <div class="mb-2 text-[10px] font-semibold tracking-widest text-gray-400">
      {{ length ? t('channelMonitorV2.v3.intervals', { n: length }) : t('channelMonitorV2.v3.noTraffic') }}
    </div>
    <div class="v3-timeline-bars" role="list" :aria-label="t('channelMonitorV2.timeRange')">
      <div v-for="(bar, index) in displayBars" :key="bar.key" class="v3-bar-slot"
        :style="entranceStyle(bar.key, index)" role="listitem" :aria-label="bar.label">
        <span class="v3-bar-hitbox" aria-hidden="true">
          <span class="v3-soft-glass-bar" :class="[bar.colorClass, { 'is-probe': bar.probe }]" :style="{ height: `${bar.height}%` }" />
        </span>
      </div>
    </div>
    <div v-if="length" class="mt-1 flex justify-between text-[9px] uppercase tracking-widest text-gray-400">
      <span>{{ t('monitorCommon.past') }}</span><span>{{ t('monitorCommon.now') }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { cardState, cardHealthStyles, type CardTimelineBucket } from './cardHealth'

const props = withDefaults(defineProps<{ buckets?: CardTimelineBucket[]; length?: number }>(), {
  buckets: () => [], length: 18,
})
const { t, locale } = useI18n()
const entranceDelays = new Map<string, number>()
let mounted = false
onMounted(() => { mounted = true })
function entranceStyle(key: string, index: number) {
  if (!entranceDelays.has(key)) entranceDelays.set(key, mounted ? 0 : index * (300 / Math.max(1, props.length - 1)))
  return { '--entrance-delay': `${entranceDelays.get(key)}ms` }
}
const displayBars = computed(() => [...props.buckets]
  .sort((a, b) => Date.parse(a.bucket_start) - Date.parse(b.bucket_start))
  .slice(-Math.max(1, props.length))
  .filter(() => props.length > 0)
  .map(bucket => {
    const state = cardState(bucket)
    const probe = !!bucket.probe && !bucket.incomplete && bucket.metrics.traffic_state !== 'traffic'
    const empty = state === 'noTraffic' || state === 'unknown'
    const time = new Date(bucket.bucket_start).toLocaleString(locale.value)
    return {
      key: bucket.bucket_start, probe, height: empty ? 100 : cardHealthStyles[state].height,
      colorClass: `${cardHealthStyles[state].bar}${empty ? ' is-empty' : ''}`,
      label: `${time} · ${t(`channelMonitorV2.v3.${state}`)}${probe ? ' · ' + t('channelMonitorV2.v3.probeSource') : ''}`,
    }
  }))
watch(() => displayBars.value.map(bar => bar.key), keys => {
  const visible = new Set(keys)
  for (const key of entranceDelays.keys()) if (!visible.has(key)) entranceDelays.delete(key)
})
</script>

<style scoped>
.v3-timeline-bars { display: flex; height: 20px; width: 100%; gap: 2px; }
.v3-bar-slot {
  position: relative; display: flex; align-items: flex-end; min-width: 0; height: 100%; flex: 1 1 0%;
  border-radius: 3px; animation: v3-slot-enter 200ms ease-out both; animation-delay: var(--entrance-delay, 0ms);
}
.v3-bar-slot::before {
  content: ''; position: absolute; bottom: 0; width: 100%; height: 20px; border-radius: 3px; pointer-events: none;
  background: rgba(148, 163, 184, 0.10); box-shadow: inset 0 0 0 1px rgba(148, 163, 184, 0.12);
}
.v3-bar-hitbox { display: flex; align-items: flex-end; width: 100%; height: 100%; min-width: 0; }
.v3-soft-glass-bar { display: block; width: 100%; min-height: 3px; border-radius: 3px; transition: height 180ms ease, background-color 180ms ease; }
.v3-soft-glass-bar.is-empty { opacity: 0.3; }
.v3-soft-glass-bar.is-probe { outline: 1px solid rgba(255,255,255,0.7); outline-offset: -1px; }
@keyframes v3-slot-enter { from { transform: translateY(2px); opacity: 0; } to { transform: translateY(0); opacity: 1; } }
@media (prefers-reduced-motion: reduce) { .v3-bar-slot { animation: none; } .v3-soft-glass-bar { transition: none; } }
</style>
