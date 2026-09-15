<template>
  <article class="group relative z-0 reference-glass-card flex min-h-[286px] flex-col overflow-visible rounded-[24px] p-5 text-left hover:z-20">
    <header class="flex min-h-[64px] items-start gap-3">
      <span class="grid h-9 w-9 shrink-0 place-items-center rounded-xl ring-1 ring-black/5 dark:ring-white/10" :class="providerGradient(row.platform)">
        <ProviderIcon :provider="row.platform" :size="20" />
      </span>
      <div class="min-w-0 flex-1">
        <div class="line-clamp-2 break-words text-base font-semibold leading-5 text-gray-900 dark:text-gray-100" :title="groupLabel">{{ groupLabel }}</div>
        <div class="mt-1 flex min-w-0 flex-wrap items-center gap-1.5">
          <span class="rounded-md px-1.5 py-0.5 text-[10px] font-medium" :class="providerBadgeClass(row.platform)">{{ providerLabel(row.platform) }}</span>
          <span v-if="userRateMultiplier != null" class="rounded-md bg-primary-50 px-1.5 py-0.5 font-mono text-[10px] font-medium text-primary-700 dark:bg-dark-700 dark:text-gray-300">{{ t('channelMonitorV2.referenceCard.userRate') }} {{ formattedUserRate }}</span>
        </div>
      </div>
    </header>

    <div class="mb-7 mt-3 grid grid-cols-3 gap-2">
      <div class="rounded-2xl border border-slate-200/80 bg-slate-50/85 p-3 dark:border-dark-700/50 dark:bg-dark-900/40">
        <div class="text-[10px] font-semibold uppercase tracking-wider text-gray-400">{{ t('channelMonitorV2.referenceCard.cacheRate') }}</div>
        <div class="mt-1.5 font-mono text-lg font-bold tabular-nums text-gray-900 dark:text-gray-100">{{ cacheRate }}</div>
      </div>
      <div class="rounded-2xl border border-slate-200/80 bg-slate-50/85 p-3 dark:border-dark-700/50 dark:bg-dark-900/40">
        <div class="text-[10px] font-semibold uppercase tracking-wider text-gray-400">{{ t('channelMonitorV2.referenceCard.successRate') }}</div>
        <div class="mt-1.5 font-mono text-lg font-bold tabular-nums" :class="availabilityClass">{{ successRate }}</div>
      </div>
      <div class="rounded-2xl border border-slate-200/80 bg-slate-50/85 p-3 dark:border-dark-700/50 dark:bg-dark-900/40">
        <div class="text-[10px] font-semibold uppercase tracking-wider text-gray-400">{{ t('channelMonitorV2.referenceCard.ttft') }}</div>
        <div class="mt-1.5 font-mono text-lg font-bold tabular-nums text-gray-900 dark:text-gray-100" :title="t('channelMonitorV2.v3.ttftHint')">{{ ttft }}</div>
      </div>
    </div>

    <ChannelMonitorV3Timeline
      class="mt-auto"
      :buckets="row.buckets"
      :length="timelineLength"
    />
  </article>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { V3CardRow } from './types'
import { formatMonitorMs, formatMonitorPercent } from '@/features/channel-monitor-v2/monitorFormat'
import { providerGradient, useChannelMonitorFormat } from '@/composables/useChannelMonitorFormat'
import { cardState, metricClasses } from './cardHealth'
import ProviderIcon from '@/components/user/monitor/ProviderIcon.vue'
import ChannelMonitorV3Timeline from './ChannelMonitorV3Timeline.vue'

const props = defineProps<{
  row: V3CardRow
  timelineLength: number
  userRateMultiplier?: number | null
}>()
const { t } = useI18n()
const { providerLabel, providerBadgeClass } = useChannelMonitorFormat()

const groupLabel = computed(() => [props.row.group_name || (props.row.group_id ? `#${props.row.group_id}` : providerLabel(props.row.platform)), props.row.model].filter(Boolean).join(' / '))
const formattedUserRate = computed(() => {
  const value = props.userRateMultiplier
  return typeof value === 'number' && Number.isFinite(value) ? `${value.toFixed(2)}x` : '-'
})
const latestMetrics = computed(() => props.row.metrics)
const latestHealth = computed(() => props.row.health)
// Keep the existing V2 selected-range summary and sample validity.
const cacheRate = computed(() => latestMetrics.value.traffic_state === 'traffic' && latestHealth.value.cache && latestHealth.value.cache !== 'unknown' ? formatMonitorPercent(latestMetrics.value.cache_rate) : '-')
const availabilityPercent = computed(() => (1 - latestMetrics.value.error_rate) * 100)
const successRate = computed(() => latestMetrics.value.traffic_state === 'traffic' ? formatMonitorPercent(availabilityPercent.value / 100) : '-')
const availabilityState = computed(() => cardState({ bucket_start: '', metrics: latestMetrics.value, health: latestHealth.value }))
const availabilityClass = computed(() => metricClasses[availabilityState.value])
const ttft = computed(() => latestMetrics.value.traffic_state === 'traffic' ? formatMonitorMs(latestMetrics.value.ttft.p50_ms) : '-')
</script>


<style scoped>
.reference-glass-card {
  @apply bg-white dark:bg-dark-800 border border-gray-200 dark:border-dark-700 transition-shadow duration-200;
  box-shadow: 0 12px 36px rgba(30, 64, 175, 0.08), 0 2px 8px rgba(15, 23, 42, 0.04);
}
.reference-glass-card:hover {
  box-shadow: 0 18px 46px rgba(30, 64, 175, 0.12), 0 4px 12px rgba(15, 23, 42, 0.05);
}
</style>
