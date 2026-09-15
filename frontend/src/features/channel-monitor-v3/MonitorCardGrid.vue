<template>
  <div class="min-w-0">
    <nav class="mb-5 flex gap-2 overflow-x-auto border-b border-gray-200 pb-3 dark:border-dark-700" :aria-label="t('monitorCommon.displayCategory')">
      <button type="button" class="category" :aria-pressed="selected === null" @click="selected = null">{{ t('monitorCommon.allCategories') }} ({{ rows.length }})</button>
      <button v-for="section in sections" :key="section.platform" type="button" class="category" :aria-pressed="selected === section.platform" @click="selected = section.platform">{{ providerLabel(section.platform) }} ({{ section.rows.length }})</button>
    </nav>
    <EmptyState v-if="!rows.length" :title="t('channelMonitorV2.empty.title')" :description="t('channelMonitorV2.empty.description')" />
    <section v-for="section in visibleSections" :key="section.platform" class="mb-7 last:mb-0" :aria-label="providerLabel(section.platform)">
      <h2 class="mb-3 flex items-center gap-2 text-base font-semibold text-gray-900 dark:text-gray-100">
        <ProviderIcon :provider="section.rows[0].platform" :size="20" />
        {{ providerLabel(section.platform) }} <span class="rounded-full border border-gray-200 bg-gray-100 px-2.5 py-0.5 text-xs font-medium text-gray-600 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-300">{{ section.rows.length }}</span>
      </h2>
      <div class="grid grid-cols-1 gap-5 md:grid-cols-2 xl:grid-cols-3 min-[1800px]:grid-cols-4">
        <ChannelMonitorV3Card v-for="row in section.rows" :key="row.monitor_id ?? JSON.stringify([row.platform, row.group_id, row.model])" :row="{ ...row, buckets: alignedBuckets(row) }" :user-rate-multiplier="rates?.[row.group_id || 0]" :timeline-length="timelineLength" />
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { MonitorCoverage } from '@/api/channelMonitorV2'
import type { V3CardRow } from './types'
import type { CardTimelineBucket } from './cardHealth'
import EmptyState from '@/components/common/EmptyState.vue'
import ProviderIcon from '@/components/user/monitor/ProviderIcon.vue'
import { useChannelMonitorFormat } from '@/composables/useChannelMonitorFormat'
import { MONITOR_CATEGORY_NAMES, monitorDisplayCategory } from '@/utils/monitorDisplayCategory'
import ChannelMonitorV3Card from './ChannelMonitorV3Card.vue'

const props = defineProps<{ rows: V3CardRow[]; coverage: MonitorCoverage; rates?: Record<number, number>; completedWindow?: boolean }>()
const { t } = useI18n()
const { providerLabel } = useChannelMonitorFormat()
const selected = ref<string | null>(null)
const sections = computed(() => {
  const groups = new Map<string, V3CardRow[]>()
  for (const row of props.rows) {
    const category = monitorDisplayCategory({ provider: row.platform as Parameters<typeof monitorDisplayCategory>[0]['provider'], display_category: row.display_category })
    groups.set(category, [...(groups.get(category) || []), row])
  }
  const order = Object.values(MONITOR_CATEGORY_NAMES)
  const rank = (platform: string) => order.includes(platform) ? order.indexOf(platform) : order.length
  return [...groups].sort(([a], [b]) => rank(a) - rank(b) || a.localeCompare(b)).map(([platform, rows]) => ({
    platform, rows: rows.sort((a, b) => (a.display_order ?? 0) - (b.display_order ?? 0) || (a.monitor_id ?? a.group_id ?? 0) - (b.monitor_id ?? b.group_id ?? 0)),
  }))
})
const visibleSections = computed(() => sections.value.filter(section => selected.value === null || section.platform === selected.value))
watch(sections, value => {
  if (selected.value !== null && !value.some(section => section.platform === selected.value)) selected.value = null
})

const timelineLength = computed(() => {
  const step = Math.max(1, props.coverage.bucket_seconds) * 1000
  const start = Math.floor(Date.parse(props.coverage.requested_start) / step) * step
  const requestedEnd = Date.parse(props.coverage.requested_end || props.coverage.data_through)
  const end = props.completedWindow ? requestedEnd : Math.min(requestedEnd, Date.parse(props.coverage.data_through))
  if (!Number.isFinite(start) || !Number.isFinite(end)) return 0
  // V3's requested end is already the last complete interval. Passive coverage
  // may lag behind independently; confirmed probes must still remain visible.
  return Math.min(400, Math.max(0, Math.floor((end - start) / step)))
})
function alignedBuckets(row: V3CardRow): CardTimelineBucket[] {
  const step = Math.max(1, props.coverage.bucket_seconds) * 1000
  const start = Math.floor(Date.parse(props.coverage.requested_start) / step) * step
  const buckets = new Map(row.buckets.map(bucket => [Date.parse(bucket.bucket_start), bucket]))
  const probes = new Map((row.probe_buckets || []).map(bucket => [Date.parse(bucket.bucket_start), bucket]))
  return Array.from({ length: timelineLength.value }, (_, index) => {
    const stamp = start + index * step
    const coveredStart = Date.parse(props.coverage.coverage_start)
    const coveredEnd = Date.parse(props.coverage.data_through)
    const outsideCoverage = !Number.isFinite(coveredStart) || !Number.isFinite(coveredEnd) || stamp < coveredStart || stamp + step > coveredEnd
    const existing = buckets.get(stamp)
    const probe = !row.unlinked && (!existing || existing.metrics.traffic_state !== 'traffic') ? probes.get(stamp) : undefined
    const incomplete = !probe && (outsideCoverage || row.metrics.traffic_state == null)
    if (existing) return { ...existing, incomplete, probe, probeModel: row.probe_model, bucket_end: new Date(stamp + step).toISOString() }
    return {
      probe, probeModel: row.probe_model,
      bucket_start: new Date(stamp).toISOString(),
      incomplete,
      bucket_end: new Date(stamp + step).toISOString(),
      metrics: { ...row.metrics, traffic_state: incomplete ? undefined : 'no_traffic', error_rate: 0, cache_rate: 0, ttft: { ...row.metrics.ttft, p50_ms: null } },
      health: { ...row.health, overall: 'unknown', error_rate: 'unknown', cache: 'unknown', ttft: 'unknown' },
    }
  })
}
</script>

<style scoped>
.category { @apply shrink-0 rounded-md px-3 py-2 text-sm text-gray-600 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-800; }
.category[aria-pressed='true'] { @apply bg-primary-600 text-white hover:bg-primary-600 dark:bg-primary-600; }
</style>
