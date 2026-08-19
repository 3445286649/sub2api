<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.healthOverview')"
    width="extra-wide"
    @close="emit('close')"
  >
    <div class="health-overview-shell">
      <div class="mb-3 flex flex-wrap items-center justify-between gap-3">
        <p class="text-xs text-gray-500 dark:text-gray-400">
          {{ generatedAtLabel }}
        </p>
        <button
          type="button"
          class="btn btn-secondary inline-flex h-8 items-center gap-1.5 px-3 text-xs"
          :disabled="loading"
          data-testid="health-refresh"
          @click="emit('refresh')"
        >
          <Icon name="refresh" size="xs" :class="loading ? 'animate-spin' : ''" />
          {{ t('admin.accounts.healthOverviewPanel.refresh') }}
        </button>
      </div>

      <div
        v-if="loading && !overview"
        class="flex min-h-64 items-center justify-center text-sm text-gray-500 dark:text-gray-400"
      >
        {{ t('common.loading') }}
      </div>
      <div
        v-else-if="error && !overview"
        class="rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-300"
      >
        {{ error }}
      </div>
      <template v-else-if="overview">
        <section class="health-summary-grid overflow-hidden rounded-lg border border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-800/70">
          <div class="health-summary-cell health-summary-primary">
            <span class="health-summary-label">{{ t('admin.accounts.healthOverviewPanel.overallStatus') }}</span>
            <div :class="['health-summary-value', overallStatus.className]">
              {{ overallStatus.label }}
              <span class="health-summary-hint">
                {{ t('admin.accounts.healthOverviewPanel.riskCount', { count: aggregatedRisks.length }) }}
              </span>
            </div>
          </div>
          <div class="health-summary-cell">
            <span class="health-summary-label">{{ t('admin.accounts.healthOverviewPanel.upstreams') }}</span>
            <div class="health-summary-value">
              {{ totals.upstreams }}
              <span class="health-summary-hint">
                {{ t('admin.accounts.healthOverviewPanel.abnormalCount', { count: totals.abnormalUpstreams }) }}
              </span>
            </div>
          </div>
          <div class="health-summary-cell">
            <span class="health-summary-label">{{ t('admin.accounts.healthOverviewPanel.accounts') }}</span>
            <div class="health-summary-value">
              <span data-testid="health-total-accounts">{{ totals.accounts }}</span>
              <span class="health-summary-hint">{{ t('admin.accounts.healthOverviewPanel.total') }}</span>
            </div>
          </div>
          <div class="health-summary-cell">
            <span class="health-summary-label">{{ t('admin.accounts.healthOverviewPanel.schedulable') }}</span>
            <div class="health-summary-value text-emerald-600 dark:text-emerald-300">
              <span data-testid="health-schedulable-accounts">{{ totals.schedulable }}</span>
              <span class="health-summary-hint">{{ totals.schedulableRate }}</span>
            </div>
          </div>
          <div class="health-summary-cell">
            <span class="health-summary-label">{{ t('admin.accounts.healthOverviewPanel.isolatedDegraded') }}</span>
            <div class="health-summary-value text-red-600 dark:text-red-300">
              {{ totals.isolated + totals.degraded }}
              <span class="health-summary-hint">{{ totals.isolated }} / {{ totals.degraded }}</span>
            </div>
          </div>
          <div class="health-summary-cell">
            <span class="health-summary-label">{{ t('admin.accounts.healthOverviewPanel.affectedGroups') }}</span>
            <div class="health-summary-value text-amber-600 dark:text-amber-300">
              <span data-testid="health-affected-groups">{{ totals.affectedGroups }}</span>
              <span class="health-summary-hint">{{ t('admin.accounts.healthOverviewPanel.needsCoverage') }}</span>
            </div>
          </div>
        </section>

        <section
          v-if="aggregatedRisks.length"
          class="health-risk-panel mt-3 overflow-hidden rounded-lg border border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-800/70"
        >
          <div class="border-b border-gray-200 bg-gray-50 p-4 dark:border-gray-700 dark:bg-gray-900/35 md:border-b-0 md:border-r">
            <div class="flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-gray-100">
              <span class="h-2 w-2 rounded-full bg-red-500 ring-4 ring-red-500/10"></span>
              {{ t('admin.accounts.healthOverviewPanel.keyRisks') }}
            </div>
            <p class="mt-2 text-xs leading-5 text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.healthOverviewPanel.risksMergedHint') }}
            </p>
          </div>
          <div class="px-3">
            <div
              v-for="risk in aggregatedRisks.slice(0, 6)"
              :key="risk.key"
              class="health-risk-row"
              data-testid="health-risk-row"
            >
              <span :class="['health-risk-level', riskLevelClass(risk.level)]">
                {{ riskLevelLabel(risk.level) }} · {{ risk.count }}
              </span>
              <div class="min-w-0 text-sm text-gray-800 dark:text-gray-200">
                <span>{{ risk.label }}</span>
                <span v-if="risk.context" class="ml-2 text-xs text-gray-500 dark:text-gray-400">
                  {{ risk.context }}
                </span>
              </div>
              <button
                type="button"
                class="whitespace-nowrap rounded px-2 py-1 text-xs font-medium text-primary-600 hover:bg-primary-50 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/30 dark:text-primary-300 dark:hover:bg-primary-900/20"
                @click="focusRisk(risk)"
              >
                {{ t('admin.accounts.healthOverviewPanel.locate') }}
              </button>
            </div>
          </div>
        </section>

        <section class="mt-3 flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
          <div class="flex flex-wrap items-center gap-2">
            <div class="health-segmented">
              <button
                type="button"
                :class="['health-segment', { active: onlyAbnormal }]"
                data-testid="health-show-abnormal"
                @click="onlyAbnormal = true"
              >
                {{ t('admin.accounts.healthOverviewPanel.showAbnormal', { count: totals.abnormalUpstreams }) }}
              </button>
              <button
                type="button"
                :class="['health-segment', { active: !onlyAbnormal }]"
                data-testid="health-show-all"
                @click="onlyAbnormal = false"
              >
                {{ t('admin.accounts.healthOverviewPanel.showAll', { count: totals.upstreams }) }}
              </button>
            </div>
            <div class="health-segmented max-w-full overflow-x-auto">
              <button
                v-for="option in statusOptions"
                :key="option.value"
                type="button"
                :class="['health-segment whitespace-nowrap', { active: statusFilter === option.value }]"
                :data-testid="`health-status-${option.value}`"
                @click="statusFilter = option.value"
              >
                {{ option.label }}
              </button>
            </div>
          </div>
          <div class="flex flex-wrap items-center justify-end gap-2">
            <span class="text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.healthOverviewPanel.anomalyFirst') }}
            </span>
            <label class="relative block w-full sm:w-64">
              <Icon name="search" size="xs" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
              <input
                v-model.trim="searchQuery"
                type="search"
                class="input h-9 pl-9 text-sm"
                :placeholder="t('admin.accounts.healthOverviewPanel.searchPlaceholder')"
              />
            </label>
          </div>
        </section>

        <div v-if="error" class="mt-3 rounded-md border border-amber-200 bg-amber-50 p-2 text-xs text-amber-700 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-300">
          {{ error }}
        </div>

        <section class="mt-2 max-h-[58vh] space-y-2 overflow-y-auto pr-1">
          <article
            v-for="url in filteredURLs"
            :key="url.base_url"
            :class="['health-upstream', upstreamAccentClass(url)]"
            data-testid="health-upstream"
          >
            <header class="health-upstream-header">
              <button
                type="button"
                class="inline-flex h-8 w-8 shrink-0 items-center justify-center rounded text-gray-400 hover:bg-gray-100 hover:text-gray-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/30 dark:hover:bg-gray-700 dark:hover:text-gray-200"
                :aria-label="isCollapsed(url.base_url) ? t('admin.accounts.healthOverviewPanel.expand') : t('admin.accounts.healthOverviewPanel.collapse')"
                @click="toggleCollapsed(url.base_url)"
              >
                <Icon :name="isCollapsed(url.base_url) ? 'chevronRight' : 'chevronDown'" size="xs" />
              </button>
              <div class="min-w-0">
                <div class="flex min-w-0 flex-wrap items-center gap-2">
                  <a
                    v-if="isValidUpstreamURL(url.base_url)"
                    :href="url.base_url"
                    target="_blank"
                    rel="noopener noreferrer"
                    class="max-w-full truncate font-mono text-sm font-semibold text-gray-900 hover:text-primary-600 hover:underline dark:text-gray-100 dark:hover:text-primary-300"
                    :title="url.base_url"
                  >
                    {{ url.base_url }}
                  </a>
                  <span v-else class="truncate font-mono text-sm font-semibold text-gray-900 dark:text-gray-100">
                    {{ url.base_url }}
                  </span>
                  <span :class="['health-balance-pill', upstreamBalanceClass(url.balance)]" :title="upstreamBalanceTooltip(url.balance)">
                    {{ upstreamBalanceLabel(url.balance) }}
                  </span>
                  <button
                    type="button"
                    class="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-md text-gray-400 hover:bg-gray-100 hover:text-gray-700 disabled:cursor-wait disabled:opacity-50 dark:hover:bg-gray-700 dark:hover:text-gray-200"
                    :disabled="isRefreshingBalance(url.base_url)"
                    :aria-label="t('admin.accounts.upstreamBalanceRefresh')"
                    data-testid="health-balance-refresh"
                    @click="emit('refresh-balance', url.base_url)"
                  >
                    <Icon name="refresh" size="xs" :class="isRefreshingBalance(url.base_url) ? 'animate-spin' : ''" />
                  </button>
                </div>
                <p v-if="!isCollapsed(url.base_url)" class="mt-1 truncate text-xs text-gray-500 dark:text-gray-400" :title="urlRiskSummary(url)">
                  {{ urlRiskSummary(url) }}
                </p>
              </div>
              <div class="flex flex-wrap items-center justify-end gap-1.5">
                <span class="health-count-pill bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-300">
                  {{ t('admin.accounts.healthAccountCount', { count: countAccounts(url) }) }}
                </span>
                <span class="health-count-pill bg-emerald-50 text-emerald-700 dark:bg-emerald-900/35 dark:text-emerald-300">
                  {{ t('admin.accounts.healthStatus.healthy') }} {{ countStatus(url, 'healthy') }}
                </span>
                <span class="health-count-pill bg-amber-50 text-amber-700 dark:bg-amber-900/35 dark:text-amber-300">
                  {{ t('admin.accounts.healthStatus.degraded') }} {{ countStatus(url, 'degraded') }}
                </span>
                <span class="health-count-pill bg-red-50 text-red-700 dark:bg-red-900/35 dark:text-red-300">
                  {{ t('admin.accounts.healthStatus.isolated') }} {{ countStatus(url, 'isolated') }}
                </span>
                <span class="health-count-pill bg-blue-50 text-blue-700 dark:bg-blue-900/35 dark:text-blue-300">
                  {{ t('admin.accounts.healthStatus.recovering') }} {{ countStatus(url, 'recovering') }}
                </span>
              </div>
            </header>

            <div v-if="!isCollapsed(url.base_url)" class="overflow-x-auto">
              <table class="health-overview-table w-full table-fixed text-sm">
                <colgroup>
                  <col style="width: 190px" />
                  <col style="width: 120px" />
                  <col style="width: 106px" />
                  <col style="width: 116px" />
                  <col />
                  <col style="width: 100px" />
                  <col style="width: 136px" />
                  <col style="width: 76px" />
                </colgroup>
                <thead class="bg-gray-50 text-xs font-semibold text-gray-500 dark:bg-gray-900/45 dark:text-gray-400">
                  <tr>
                    <th class="px-3 py-2 text-left">{{ t('admin.accounts.overviewColumns.account') }}</th>
                    <th class="px-3 py-2 text-left">{{ t('admin.accounts.overviewColumns.key') }}</th>
                    <th class="px-3 py-2 text-left">{{ t('admin.accounts.overviewColumns.health') }}</th>
                    <th class="px-3 py-2 text-left">{{ t('admin.accounts.overviewColumns.autoStatus') }}</th>
                    <th class="px-3 py-2 text-left">{{ t('admin.accounts.overviewColumns.groups') }}</th>
                    <th class="px-3 py-2 text-right">{{ t('admin.accounts.overviewColumns.avgLatency') }}</th>
                    <th class="px-3 py-2 text-right">{{ t('admin.accounts.overviewColumns.nextProbe') }}</th>
                    <th class="px-3 py-2 text-right">{{ t('admin.accounts.overviewColumns.costRate') }}</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-100 dark:divide-gray-700/80">
                  <tr v-for="item in url.accounts" :key="item.account_id" class="hover:bg-gray-50/70 dark:hover:bg-gray-700/25">
                    <td class="overflow-hidden px-3 py-2.5">
                      <div class="truncate text-sm font-semibold text-gray-900 dark:text-gray-100" :title="`#${item.account_id} ${item.account_name}`">
                        #{{ item.account_id }} {{ item.account_name }}
                      </div>
                      <div class="mt-0.5 truncate font-mono text-[10px] text-gray-400 dark:text-gray-500">
                        {{ item.platform || '-' }} · {{ item.type || '-' }}
                      </div>
                    </td>
                    <td class="overflow-hidden px-3 py-2 font-mono text-xs text-gray-500 dark:text-gray-400">
                      <span class="block truncate" :title="item.key_fingerprint || '-'">{{ item.key_fingerprint || '-' }}</span>
                    </td>
                    <td class="overflow-hidden px-3 py-2">
                      <span :class="['health-state-badge', healthBadgeClass(item.status)]">
                        {{ item.score }} / {{ healthStatusLabel(item.status) }}
                      </span>
                    </td>
                    <td class="overflow-hidden px-3 py-2">
                      <span :class="['health-state-badge', autoStatusClass(item)]">
                        {{ autoStatusLabel(item) }}
                      </span>
                    </td>
                    <td class="overflow-hidden px-3 py-2 text-gray-600 dark:text-gray-300">
                      <span class="block truncate" :title="item.group_names?.join(', ') || '-'">
                        {{ item.group_names?.join(', ') || '-' }}
                      </span>
                    </td>
                    <td class="whitespace-nowrap px-3 py-2 text-right font-mono text-xs text-gray-500 dark:text-gray-400">
                      {{ formatLatency(item.scheduler_latency_ewma_ms) }}
                    </td>
                    <td class="whitespace-nowrap px-3 py-2 text-right font-mono text-xs text-gray-500 dark:text-gray-400" :title="item.next_probe_at ? formatDateTime(item.next_probe_at) : '-'">
                      {{ item.next_probe_at ? formatDateTime(item.next_probe_at) : '-' }}
                    </td>
                    <td class="whitespace-nowrap px-3 py-2 text-right font-mono text-xs text-gray-700 dark:text-gray-300">
                      {{ formatRate(item.rate_multiplier) }}x
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </article>

          <div v-if="filteredURLs.length === 0" class="rounded-lg border border-dashed border-gray-300 px-4 py-12 text-center text-sm text-gray-500 dark:border-gray-700 dark:text-gray-400">
            {{ t('admin.accounts.healthOverviewPanel.noMatches') }}
          </div>
        </section>
      </template>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { formatDateTime, formatRelativeTime } from '@/utils/format'
import type {
  AccountHealthOverview,
  AccountHealthRisk,
  AccountHealthSummary,
  AccountHealthURLOverview,
  AccountUpstreamBalanceSnapshot
} from '@/types'

type HealthStatusFilter = 'all' | AccountHealthSummary['status']
type RiskLevel = 'critical' | 'warning' | 'info' | string

interface AggregatedRisk {
  key: string
  type: string
  level: RiskLevel
  count: number
  label: string
  context: string
  source: AccountHealthRisk
}

interface Props {
  show: boolean
  overview: AccountHealthOverview | null
  loading?: boolean
  error?: string
  refreshingBalanceUrls?: Set<string>
}

const props = withDefaults(defineProps<Props>(), {
  loading: false,
  error: '',
  refreshingBalanceUrls: () => new Set<string>()
})

const emit = defineEmits<{
  close: []
  refresh: []
  'refresh-balance': [baseURL: string]
}>()

const { t } = useI18n()
const onlyAbnormal = ref(true)
const statusFilter = ref<HealthStatusFilter>('all')
const searchQuery = ref('')
const collapsedURLs = ref<Set<string>>(new Set())
const nowMs = ref(Date.now())
let clockTimer: ReturnType<typeof setInterval> | null = null

const allAccounts = computed(() => props.overview?.urls.flatMap(url => url.accounts) ?? [])

function isTempUnschedulable(account: AccountHealthSummary): boolean {
  if (!account.temp_unschedulable_until) return false
  const until = Date.parse(account.temp_unschedulable_until)
  return Number.isFinite(until) && until > nowMs.value
}

function isEffectivelySchedulable(account: AccountHealthSummary): boolean {
  return account.schedulable && account.status !== 'isolated' && !isTempUnschedulable(account)
}

function isURLAbnormal(url: AccountHealthURLOverview): boolean {
  return Boolean(
    url.risks?.length ||
    url.insufficient_group_ids?.length ||
    url.accounts.some(account => account.status !== 'healthy' || !isEffectivelySchedulable(account))
  )
}

const affectedGroupKeys = computed(() => {
  const keys = new Set<string>()
  for (const risk of props.overview?.risks ?? []) {
    if (risk.type !== 'group_no_available_accounts' && risk.type !== 'group_single_available_account') continue
    if (risk.group_id != null) keys.add(`id:${risk.group_id}`)
    else if (risk.group_name) keys.add(`name:${risk.group_name}`)
  }
  for (const url of props.overview?.urls ?? []) {
    const groupIDs = url.insufficient_group_ids ?? []
    if (groupIDs.length > 0) {
      for (const groupID of groupIDs) keys.add(`id:${groupID}`)
    } else {
      for (const groupName of url.insufficient_group_names ?? []) keys.add(`name:${groupName}`)
    }
  }
  return keys
})

const totals = computed(() => {
  const accounts = allAccounts.value
  const schedulable = accounts.filter(isEffectivelySchedulable).length
  return {
    upstreams: props.overview?.urls.length ?? 0,
    abnormalUpstreams: props.overview?.urls.filter(isURLAbnormal).length ?? 0,
    accounts: accounts.length,
    schedulable,
    schedulableRate: accounts.length > 0 ? `${((schedulable / accounts.length) * 100).toFixed(1)}%` : '0%',
    isolated: accounts.filter(account => account.status === 'isolated').length,
    degraded: accounts.filter(account => account.status === 'degraded').length,
    affectedGroups: affectedGroupKeys.value.size
  }
})

function riskEntityKey(risk: AccountHealthRisk): string {
  if (risk.group_id != null) return `group:${risk.group_id}`
  if (risk.account_id != null) return `account:${risk.account_id}`
  if (risk.base_url) return `url:${risk.base_url}`
  return `message:${risk.message || risk.type}`
}

function riskRank(level: RiskLevel): number {
  if (level === 'critical') return 3
  if (level === 'warning') return 2
  return 1
}

function aggregateRiskLabel(type: string, count: number): string {
  switch (type) {
    case 'url_all_isolated':
      return t('admin.accounts.healthOverviewPanel.riskAggregated.urlAllUnavailable', { count })
    case 'group_no_available_accounts':
      return t('admin.accounts.healthOverviewPanel.riskAggregated.groupNoneAvailable', { count })
    case 'group_single_available_account':
      return t('admin.accounts.healthOverviewPanel.riskAggregated.groupSingleAvailable', { count })
    case 'consecutive_failures':
      return t('admin.accounts.healthOverviewPanel.riskAggregated.consecutiveFailures', { count })
    case 'healthy_probe_disabled':
      return t('admin.accounts.healthOverviewPanel.riskAggregated.probeDisabled', { count })
    default:
      return type
  }
}

function aggregateRiskContext(type: string, risks: AccountHealthRisk[]): string {
  if (type === 'url_all_isolated') {
    const impacted = risks.reduce((total, risk) => total + (risk.count ?? 0), 0)
    return t('admin.accounts.healthOverviewPanel.affectedAccounts', { count: impacted })
  }
  const labels = risks.map(risk => {
    if (risk.group_name) return risk.group_name
    if (risk.group_id != null) return `#${risk.group_id}`
    if (risk.account_id != null) return `#${risk.account_id}${risk.count ? ` (${risk.count})` : ''}`
    return risk.base_url || ''
  }).filter(Boolean)
  const visible = labels.slice(0, 3).join(', ')
  return labels.length > 3 ? `${visible} +${labels.length - 3}` : visible
}

const aggregatedRisks = computed<AggregatedRisk[]>(() => {
  const grouped = new Map<string, Map<string, AccountHealthRisk>>()
  for (const risk of props.overview?.risks ?? []) {
    const entities = grouped.get(risk.type) ?? new Map<string, AccountHealthRisk>()
    const entityKey = riskEntityKey(risk)
    const current = entities.get(entityKey)
    if (!current || riskRank(risk.level) > riskRank(current.level)) entities.set(entityKey, risk)
    grouped.set(risk.type, entities)
  }

  return [...grouped.entries()]
    .map(([type, entities]) => {
      const risks = [...entities.values()]
      const source = risks.reduce((highest, risk) =>
        riskRank(risk.level) > riskRank(highest.level) ? risk : highest
      )
      return {
        key: type,
        type,
        level: source.level,
        count: risks.length,
        label: aggregateRiskLabel(type, risks.length),
        context: aggregateRiskContext(type, risks),
        source
      }
    })
    .sort((left, right) => riskRank(right.level) - riskRank(left.level) || right.count - left.count || left.type.localeCompare(right.type))
})

const overallStatus = computed(() => {
  if (aggregatedRisks.value.some(risk => risk.level === 'critical')) {
    return { label: t('admin.accounts.healthOverviewPanel.statusCritical'), className: 'text-red-600 dark:text-red-300' }
  }
  if (aggregatedRisks.value.length > 0) {
    return { label: t('admin.accounts.healthOverviewPanel.statusAttention'), className: 'text-amber-600 dark:text-amber-300' }
  }
  return { label: t('admin.accounts.healthOverviewPanel.statusGood'), className: 'text-emerald-600 dark:text-emerald-300' }
})

const statusOptions = computed<Array<{ value: HealthStatusFilter; label: string }>>(() => [
  { value: 'all', label: t('admin.accounts.healthOverviewPanel.allStates') },
  { value: 'isolated', label: t('admin.accounts.healthStatus.isolated') },
  { value: 'degraded', label: t('admin.accounts.healthStatus.degraded') },
  { value: 'recovering', label: t('admin.accounts.healthStatus.recovering') }
])

function accountMatchesSearch(account: AccountHealthSummary, query: string): boolean {
  return [
    String(account.account_id),
    `#${account.account_id}`,
    account.account_name,
    account.key_fingerprint,
    ...(account.group_names ?? [])
  ].some(value => String(value || '').toLowerCase().includes(query))
}

function upstreamPriority(url: AccountHealthURLOverview): number {
  if (url.risks?.some(risk => risk.level === 'critical')) return 4
  if (url.accounts.some(account => account.status === 'isolated')) return 3
  if (url.risks?.length || url.accounts.some(account => account.status === 'degraded' || account.status === 'recovering')) return 2
  return 1
}

const filteredURLs = computed<AccountHealthURLOverview[]>(() => {
  const query = searchQuery.value.toLowerCase()
  return (props.overview?.urls ?? [])
    .filter(url => !onlyAbnormal.value || isURLAbnormal(url))
    .map(url => {
      const urlMatches = !query || url.base_url.toLowerCase().includes(query)
      const accounts = url.accounts.filter(account => {
        if (statusFilter.value !== 'all' && account.status !== statusFilter.value) return false
        return urlMatches || accountMatchesSearch(account, query)
      })
      return { ...url, accounts }
    })
    .filter(url => {
      if (statusFilter.value !== 'all' && url.accounts.length === 0) return false
      if (query && !url.base_url.toLowerCase().includes(query) && url.accounts.length === 0) return false
      return true
    })
    .sort((left, right) => upstreamPriority(right) - upstreamPriority(left) || left.base_url.localeCompare(right.base_url))
})

const generatedAtLabel = computed(() => {
  void nowMs.value
  if (!props.overview?.generated_at) return t('admin.accounts.healthOverviewPanel.notUpdated')
  return t('admin.accounts.healthOverviewPanel.generatedAt', {
    time: formatRelativeTime(props.overview.generated_at)
  })
})

function resetCollapsedState(): void {
  const next = new Set<string>()
  for (const url of props.overview?.urls ?? []) {
    if (!isURLAbnormal(url)) next.add(url.base_url)
  }
  collapsedURLs.value = next
}

function startClock(): void {
  if (clockTimer) return
  nowMs.value = Date.now()
  clockTimer = setInterval(() => {
    nowMs.value = Date.now()
  }, 1000)
}

function stopClock(): void {
  if (!clockTimer) return
  clearInterval(clockTimer)
  clockTimer = null
}

watch(() => props.overview?.generated_at, resetCollapsedState, { immediate: true })
watch(() => props.show, show => show ? startClock() : stopClock(), { immediate: true })
onUnmounted(stopClock)

function isCollapsed(baseURL: string): boolean {
  return collapsedURLs.value.has(baseURL)
}

function toggleCollapsed(baseURL: string): void {
  const next = new Set(collapsedURLs.value)
  if (next.has(baseURL)) next.delete(baseURL)
  else next.add(baseURL)
  collapsedURLs.value = next
}

function focusRisk(risk: AggregatedRisk): void {
  onlyAbnormal.value = true
  statusFilter.value = 'all'
  const source = risk.source
  if (source.account_id != null) searchQuery.value = `#${source.account_id}`
  else if (source.group_name) searchQuery.value = source.group_name
  else if (source.group_id != null) searchQuery.value = `#${source.group_id}`
  else searchQuery.value = source.base_url || ''
  if (source.base_url) {
    const next = new Set(collapsedURLs.value)
    next.delete(source.base_url)
    collapsedURLs.value = next
  }
}

function countStatus(url: AccountHealthURLOverview, status: AccountHealthSummary['status']): number {
  const original = props.overview?.urls.find(item => item.base_url === url.base_url) ?? url
  return original.accounts.filter(account => account.status === status).length
}

function countAccounts(url: AccountHealthURLOverview): number {
  return props.overview?.urls.find(item => item.base_url === url.base_url)?.accounts.length ?? url.accounts.length
}

function riskLevelClass(level: RiskLevel): string {
  if (level === 'critical') return 'bg-red-100 text-red-700 dark:bg-red-900/35 dark:text-red-300'
  if (level === 'warning') return 'bg-amber-100 text-amber-700 dark:bg-amber-900/35 dark:text-amber-300'
  return 'bg-blue-100 text-blue-700 dark:bg-blue-900/35 dark:text-blue-300'
}

function riskLevelLabel(level: RiskLevel): string {
  if (level === 'critical') return t('admin.accounts.healthOverviewPanel.levelCritical')
  if (level === 'warning') return t('admin.accounts.healthOverviewPanel.levelWarning')
  return t('admin.accounts.healthOverviewPanel.levelInfo')
}

function healthRiskLabel(risk: AccountHealthRisk): string {
  const group = risk.group_name || (risk.group_id ? `#${risk.group_id}` : '')
  const account = risk.account_id ? `#${risk.account_id}` : ''
  switch (risk.type) {
    case 'group_no_available_accounts':
      return t('admin.accounts.riskGroupNoAvailable', { group })
    case 'group_single_available_account':
      return t('admin.accounts.riskGroupSingleAvailable', { group })
    case 'url_all_isolated':
      return t('admin.accounts.riskUrlAllIsolated', { count: risk.count ?? 0 })
    case 'consecutive_failures':
      return t('admin.accounts.riskConsecutiveFailures', { account, count: risk.count ?? 0 })
    case 'healthy_probe_disabled':
      return t('admin.accounts.riskHealthyProbeDisabled', { account })
    default:
      return risk.message || risk.type
  }
}

function urlRiskSummary(url: AccountHealthURLOverview): string {
  const primary = [...(url.risks ?? [])].sort((left, right) => riskRank(right.level) - riskRank(left.level))[0]
  if (!primary) return t('admin.accounts.healthOverviewPanel.upstreamHealthy')
  const accountWithError = url.accounts.find(account => account.last_error_message)
  const error = String(accountWithError?.last_error_message || '').trim()
  return error ? `${healthRiskLabel(primary)} · ${truncate(error, 120)}` : healthRiskLabel(primary)
}

function truncate(value: string, maxLength: number): string {
  return value.length > maxLength ? `${value.slice(0, maxLength)}...` : value
}

function upstreamAccentClass(url: AccountHealthURLOverview): string {
  if (url.risks?.some(risk => risk.level === 'critical')) return 'border-l-red-500'
  if (isURLAbnormal(url)) return 'border-l-amber-500'
  return 'border-l-emerald-600/50'
}

function isValidUpstreamURL(baseURL: string): boolean {
  return /^https?:\/\//i.test(String(baseURL || '').trim())
}

function upstreamBalanceValue(snapshot?: AccountUpstreamBalanceSnapshot | null): number | null {
  if (!snapshot) return null
  if (typeof snapshot.remaining === 'number') return snapshot.remaining
  if (typeof snapshot.balance === 'number') return snapshot.balance
  return null
}

function formatBalanceAmount(value: number | null, unit?: string): string {
  if (value === null || Number.isNaN(value)) return '-'
  const digits = Math.abs(value) >= 100 ? 2 : 4
  const amount = value.toLocaleString(undefined, { minimumFractionDigits: 0, maximumFractionDigits: digits })
  const normalizedUnit = unit || 'USD'
  if (normalizedUnit.toLowerCase() === 'upstream') {
    return t('admin.accounts.upstreamBalanceUnitUpstream', { value: amount })
  }
  return normalizedUnit.toUpperCase() === 'USD' ? `$${amount}` : `${amount} ${normalizedUnit}`
}

function upstreamBalanceLabel(snapshot?: AccountUpstreamBalanceSnapshot | null): string {
  if (!snapshot || (!snapshot.checked_at && snapshot.status !== 'checking')) return t('admin.accounts.upstreamBalanceEmpty')
  if (snapshot.status === 'checking') return t('admin.accounts.upstreamBalanceChecking')
  if (snapshot.status === 'ok') {
    return t('admin.accounts.upstreamBalanceValue', {
      value: formatBalanceAmount(upstreamBalanceValue(snapshot), snapshot.unit)
    })
  }
  if (snapshot.status === 'auth_error') return t('admin.accounts.upstreamBalanceAuthError')
  if (snapshot.status === 'error') return t('admin.accounts.upstreamBalanceError')
  return t('admin.accounts.upstreamBalanceUnsupported')
}

function upstreamBalanceClass(snapshot?: AccountUpstreamBalanceSnapshot | null): string {
  if (snapshot?.status === 'ok') return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/35 dark:text-emerald-300'
  if (snapshot?.status === 'checking') return 'bg-blue-50 text-blue-700 dark:bg-blue-900/35 dark:text-blue-300'
  if (snapshot?.status === 'auth_error' || snapshot?.status === 'error') return 'bg-red-50 text-red-700 dark:bg-red-900/35 dark:text-red-300'
  return 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-300'
}

function upstreamBalanceTooltip(snapshot?: AccountUpstreamBalanceSnapshot | null): string {
  if (!snapshot) return t('admin.accounts.upstreamBalanceEmpty')
  return [
    upstreamBalanceLabel(snapshot),
    snapshot.representative_account_id ? t('admin.accounts.upstreamBalanceRepresentative', { id: snapshot.representative_account_id }) : '',
    snapshot.checked_at ? t('admin.accounts.upstreamBalanceCheckedAt', { time: formatDateTime(snapshot.checked_at) }) : '',
    snapshot.source_endpoint ? t('admin.accounts.upstreamBalanceEndpoint', { endpoint: snapshot.source_endpoint }) : '',
    snapshot.unit?.toLowerCase() === 'upstream' ? t('admin.accounts.upstreamBalanceUnitHint') : '',
    snapshot.http_status ? t('admin.accounts.upstreamBalanceHTTPStatus', { status: snapshot.http_status }) : '',
    snapshot.error_message ? truncate(snapshot.error_message, 240) : ''
  ].filter(Boolean).join('\n')
}

function isRefreshingBalance(baseURL: string): boolean {
  return props.refreshingBalanceUrls.has(baseURL)
}

function healthStatusLabel(status: AccountHealthSummary['status']): string {
  return t(`admin.accounts.healthStatus.${status}`)
}

function healthBadgeClass(status: AccountHealthSummary['status']): string {
  if (status === 'isolated') return 'bg-red-100 text-red-700 dark:bg-red-900/35 dark:text-red-300'
  if (status === 'recovering') return 'bg-blue-100 text-blue-700 dark:bg-blue-900/35 dark:text-blue-300'
  if (status === 'degraded') return 'bg-amber-100 text-amber-700 dark:bg-amber-900/35 dark:text-amber-300'
  return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/35 dark:text-emerald-300'
}

function autoStatusLabel(account: AccountHealthSummary): string {
  if (!account.schedulable) return t('admin.accounts.autoStatus.manualOff')
  if (isTempUnschedulable(account)) return t('admin.accounts.autoStatus.tempUnschedulable')
  if (account.status === 'isolated') return t('admin.accounts.autoStatus.isolated')
  if (account.status === 'recovering') return t('admin.accounts.autoStatus.probing')
  if (account.status === 'degraded') return t('admin.accounts.autoStatus.degraded')
  return t('admin.accounts.autoStatus.enabled')
}

function autoStatusClass(account: AccountHealthSummary): string {
  if (!account.schedulable) return 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-300'
  if (isTempUnschedulable(account) || account.status === 'isolated') return 'bg-red-100 text-red-700 dark:bg-red-900/35 dark:text-red-300'
  if (account.status === 'recovering') return 'bg-blue-100 text-blue-700 dark:bg-blue-900/35 dark:text-blue-300'
  if (account.status === 'degraded') return 'bg-amber-100 text-amber-700 dark:bg-amber-900/35 dark:text-amber-300'
  return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/35 dark:text-emerald-300'
}

function formatLatency(value?: number | null): string {
  return typeof value === 'number' && Number.isFinite(value) ? `${Math.round(value)} ms` : '-'
}

function formatRate(value: number): string {
  if (!Number.isFinite(value)) return '1'
  return String(Number(value.toFixed(4)))
}
</script>

<style scoped>
.health-overview-shell {
  letter-spacing: 0;
}

.health-summary-grid {
  display: grid;
  grid-template-columns: minmax(170px, 1.2fr) repeat(5, minmax(120px, 1fr));
}

.health-summary-cell {
  min-height: 76px;
  padding: 14px 16px;
  border-right: 1px solid rgb(229 231 235);
}

.dark .health-summary-cell {
  border-color: rgb(55 65 81);
}

.health-summary-cell:last-child {
  border-right: 0;
}

.health-summary-label {
  display: block;
  color: rgb(107 114 128);
  font-size: 12px;
}

.dark .health-summary-label,
.dark .health-summary-hint {
  color: rgb(156 163 175);
}

.health-summary-value {
  display: flex;
  align-items: baseline;
  gap: 7px;
  margin-top: 5px;
  color: rgb(17 24 39);
  font-size: 22px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}

.dark .health-summary-value:not(.text-emerald-300):not(.text-red-300):not(.text-amber-300) {
  color: rgb(243 244 246);
}

.health-summary-hint {
  color: rgb(156 163 175);
  font-size: 11px;
  font-weight: 500;
}

.health-risk-panel {
  display: grid;
  grid-template-columns: 210px minmax(0, 1fr);
}

.health-risk-row {
  display: grid;
  grid-template-columns: 84px minmax(0, 1fr) auto;
  align-items: center;
  gap: 12px;
  min-height: 44px;
  border-bottom: 1px solid rgb(229 231 235);
}

.dark .health-risk-row {
  border-color: rgb(55 65 81);
}

.health-risk-row:last-child {
  border-bottom: 0;
}

.health-risk-level {
  width: max-content;
  border-radius: 4px;
  padding: 3px 7px;
  font-size: 11px;
  font-weight: 700;
}

.health-segmented {
  display: inline-flex;
  gap: 2px;
  border: 1px solid rgb(229 231 235);
  border-radius: 6px;
  padding: 3px;
  background: rgb(249 250 251);
}

.dark .health-segmented {
  border-color: rgb(55 65 81);
  background: rgb(17 24 39 / 0.55);
}

.health-segment {
  min-height: 30px;
  border-radius: 4px;
  padding: 0 10px;
  color: rgb(107 114 128);
  font-size: 12px;
  font-weight: 500;
}

.health-segment.active {
  color: rgb(17 24 39);
  background: rgb(229 231 235);
}

.dark .health-segment {
  color: rgb(156 163 175);
}

.dark .health-segment.active {
  color: rgb(243 244 246);
  background: rgb(55 65 81);
}

.health-upstream {
  overflow: hidden;
  border: 1px solid rgb(229 231 235);
  border-left-width: 3px;
  border-radius: 7px;
  background: white;
}

.dark .health-upstream {
  border-top-color: rgb(55 65 81);
  border-right-color: rgb(55 65 81);
  border-bottom-color: rgb(55 65 81);
  background: rgb(31 41 55 / 0.72);
}

.health-upstream-header {
  display: grid;
  grid-template-columns: 32px minmax(360px, 1fr) auto;
  align-items: center;
  gap: 10px;
  min-height: 58px;
  padding: 9px 12px;
  background: rgb(249 250 251);
}

.dark .health-upstream-header {
  background: rgb(17 24 39 / 0.48);
}

.health-count-pill,
.health-state-badge,
.health-balance-pill {
  display: inline-flex;
  align-items: center;
  white-space: nowrap;
  border-radius: 4px;
  font-size: 11px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.health-count-pill {
  min-height: 21px;
  padding: 2px 7px;
}

.health-state-badge {
  max-width: 100%;
  padding: 3px 7px;
}

.health-balance-pill {
  max-width: 180px;
  min-height: 23px;
  overflow: hidden;
  padding: 2px 7px;
  text-overflow: ellipsis;
}

.health-overview-table {
  min-width: 1000px;
}

@media (max-width: 1024px) {
  .health-summary-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .health-summary-cell:nth-child(3) {
    border-right: 0;
  }

  .health-summary-cell:nth-child(-n + 3) {
    border-bottom: 1px solid rgb(229 231 235);
  }

  .dark .health-summary-cell:nth-child(-n + 3) {
    border-bottom-color: rgb(55 65 81);
  }

  .health-upstream-header {
    grid-template-columns: 32px minmax(260px, 1fr);
  }

  .health-upstream-header > :last-child {
    grid-column: 2;
    justify-content: flex-start;
  }
}

@media (max-width: 640px) {
  .health-summary-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .health-summary-cell,
  .health-summary-cell:nth-child(3) {
    border-right: 1px solid rgb(229 231 235);
    border-bottom: 1px solid rgb(229 231 235);
  }

  .dark .health-summary-cell,
  .dark .health-summary-cell:nth-child(3) {
    border-right-color: rgb(55 65 81);
    border-bottom-color: rgb(55 65 81);
  }

  .health-summary-cell:nth-child(even) {
    border-right: 0;
  }

  .health-summary-cell:nth-last-child(-n + 2) {
    border-bottom: 0;
  }

  .health-summary-value {
    align-items: flex-start;
    flex-direction: column;
    gap: 2px;
    font-size: 19px;
  }

  .health-risk-panel {
    grid-template-columns: minmax(0, 1fr);
  }

  .health-risk-row {
    grid-template-columns: 80px minmax(0, 1fr);
    padding: 6px 0;
  }

  .health-risk-row > :last-child {
    grid-column: 2;
    justify-self: start;
  }

  .health-upstream-header {
    grid-template-columns: 28px minmax(0, 1fr);
  }
}
</style>
