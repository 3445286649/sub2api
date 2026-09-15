<template>
  <AppLayout>
    <div class="space-y-5 pb-10" data-testid="channel-status-v3">
      <header class="border-b border-gray-200 pb-4 dark:border-dark-700">
        <div class="flex items-center justify-between gap-3">
          <div>
            <h1 class="text-xl font-semibold text-gray-900 dark:text-white">{{ t('nav.channelStatus') }}</h1>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ snapshot ? t('channelMonitorV2.updatedTo', { time: updatedTime }) : t('common.loading') }}</p>
          </div>
          <button type="button" class="btn btn-secondary btn-icon" :title="t('common.refresh')" :aria-label="t('common.refresh')" :disabled="refreshing" @click="reload(false)"><Icon name="refresh" size="sm" :class="{ 'animate-spin': refreshing }" /></button>
        </div>
        <div class="mt-4 flex flex-wrap items-center gap-2" role="group" :aria-label="t('channelMonitorV2.timeRange')">
          <button v-for="option in ranges" :key="option" type="button" class="tab !px-3 !py-1.5" :class="{ 'tab-active': range === option }" :aria-pressed="range === option" @click="range = option">{{ t(`channelMonitorV2.ranges.${option}`) }}</button>
          <span v-if="snapshot && !snapshot.coverage.coverage_complete" class="text-xs text-gray-500">{{ t('channelMonitorV2.partialCoverage') }}</span>
        </div>
      </header>
      <p v-if="failed" role="alert" class="text-sm text-red-600 dark:text-red-400">{{ t(matrix ? 'channelMonitorV2.v3.refreshFailed' : 'channelMonitorV2.v3.loadFailed') }}</p>
      <div v-if="!matrix && refreshing" class="grid grid-cols-1 gap-5 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4" aria-busy="true">
        <div v-for="n in 6" :key="n" class="h-72 animate-pulse rounded-lg bg-gray-100 dark:bg-dark-800" />
      </div>
      <MonitorCardGrid v-else-if="matrix" :rows="cardRows" :coverage="matrix.coverage" :rates="rates" />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import MonitorCardGrid from '@/features/channel-monitor-v3/MonitorCardGrid.vue'
import { createMonitorPageCache } from '@/features/channel-monitor-v2/monitorPageCache'
import * as api from '@/api/channelMonitorV2'
import { apiClient } from '@/api/client'
import type { V3CardConfig, V3CardRow } from '@/features/channel-monitor-v3/types'
import userGroupsAPI from '@/api/groups'
import type { MonitorRange, MonitorSnapshot, MonitorMatrixResponse } from '@/api/channelMonitorV2'
import { useAuthStore } from '@/stores/auth'

const { t, locale } = useI18n()
const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const ranges: MonitorRange[] = ['90m', '24h', '7d', '30d']
const range = ref<MonitorRange>(ranges.includes(route.query.range as MonitorRange) ? route.query.range as MonitorRange : '90m')
const snapshot = ref<MonitorSnapshot | null>(null)
const matrix = ref<MonitorMatrixResponse | null>(null)
const rates = ref<Record<number, number>>({})
const refreshing = ref(false)
const failed = ref(false)
const cardConfigs = ref<V3CardConfig[]>([])
const cardRows = computed<V3CardRow[]>(() => cardConfigs.value.map(card => {
  const matches = matrix.value?.items.filter(row => row.group_id === card.v3_group_id) || []
  const stats = matches.length === 1 ? matches[0] : undefined
  const metrics = stats?.metrics || { ...snapshot.value!.metrics, traffic_state: undefined, error_rate: 0, cache_rate: 0, ttft: { ...snapshot.value!.metrics.ttft, p50_ms: null } }
  return { platform: card.provider, group_id: card.v3_group_id || undefined, group_name: card.name, monitor_id: card.id,
    display_category: card.display_category, display_order: card.display_order, unlinked: !card.v3_group_id,
    probe_model: card.probe_model, probe_buckets: card.v3_fusion_enabled ? card.probe_buckets : [],
    metrics, health: stats?.health || snapshot.value!.health, buckets: stats?.buckets || [] }
}))
const cache = createMonitorPageCache()
let controller: AbortController | null = null
let refreshTimer: ReturnType<typeof setTimeout> | undefined
let identity = 0
const updatedTime = computed(() => snapshot.value ? new Date(snapshot.value.coverage.data_through).toLocaleString(locale.value) : '')

function schedule() {
  clearTimeout(refreshTimer)
  refreshTimer = setTimeout(() => { if (!document.hidden) void reload(false); else schedule() }, 60000)
}

async function reload(useCache = true) {
  controller?.abort()
  clearTimeout(refreshTimer)
  const request = new AbortController()
  controller = request
  const filter = { range: range.value, platforms: [], groupIds: [], models: [] }
  const key = cache.key(auth.user?.id, false, filter, 'platform_group')
  refreshing.value = true
  failed.value = false
  try {
    const cached = useCache ? cache.get(key) : undefined
    const next = cached || await Promise.all([
      api.getSnapshot(filter, false, request.signal),
      api.getMatrix(filter, 'platform_group', false, request.signal),
    ]).then(([snapshot, matrix]) => ({ snapshot, matrix }))
    const { data: configs } = await apiClient.get<V3CardConfig[]>('/channel-monitors/v3-cards', { signal: request.signal, params: { range: filter.range } })
    if (!next || request.signal.aborted || controller !== request) return
    snapshot.value = next.snapshot
    matrix.value = next.matrix
    cardConfigs.value = configs
    if (!cached) cache.set(key, next)
    schedule()
  } catch {
    if (request.signal.aborted || controller !== request) return
    failed.value = true
    schedule()
  } finally {
    if (controller === request) refreshing.value = false
  }
}

async function loadRates() {
  const current = identity
  try {
    const [groups, custom] = await Promise.all([userGroupsAPI.getAvailable(), userGroupsAPI.getUserGroupRates()])
    if (current === identity) rates.value = Object.fromEntries(groups.map(g => [g.id, custom[g.id] ?? g.rate_multiplier]))
  } catch { /* Optional prices do not block monitoring. */ }
}
watch(range, () => {
  snapshot.value = null
  matrix.value = null
  void router.replace({ query: { ...route.query, range: range.value } })
  void reload()
})
watch([() => auth.user?.id, () => auth.isAdmin], () => {
  identity++
  controller?.abort()
  cache.clear()
  snapshot.value = null
  matrix.value = null
  rates.value = {}
  cardConfigs.value = []
  void loadRates()
  void reload(false)
})
onMounted(() => {
  void loadRates()
  void reload()
})
onBeforeUnmount(() => {
  identity++
  controller?.abort()
  clearTimeout(refreshTimer)
  cache.clear()
})
</script>
