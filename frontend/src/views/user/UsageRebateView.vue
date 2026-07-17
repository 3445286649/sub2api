<template>
  <AppLayout>
    <div class="space-y-6">
      <div v-if="loading" class="flex justify-center py-16" aria-live="polite">
        <div class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
      </div>

      <template v-else-if="overview">
        <section class="border-b border-gray-200 pb-6 dark:border-dark-700">
          <div class="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
            <div>
              <div class="flex items-center gap-2">
                <span
                  :class="overview.enabled
                    ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/25 dark:text-emerald-300'
                    : 'bg-gray-100 text-gray-600 dark:bg-dark-800 dark:text-gray-300'"
                  class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium"
                >
                  <span :class="overview.enabled ? 'bg-emerald-500' : 'bg-gray-400'" class="h-1.5 w-1.5 rounded-full"></span>
                  {{ overview.enabled ? t('usageRebate.enabled') : t('usageRebate.disabled') }}
                </span>
              </div>
              <p class="mt-3 text-sm text-gray-500 dark:text-dark-400">
                {{ overview.business_date }} · {{ overview.timezone }}
              </p>
            </div>
            <div class="grid grid-cols-2 gap-x-8 gap-y-2 sm:text-right">
              <div>
                <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('usageRebate.settlement') }}</p>
                <p class="mt-1 text-base font-semibold tabular-nums text-gray-900 dark:text-white">{{ overview.settlement_time }}</p>
              </div>
              <div>
                <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('usageRebate.participants') }}</p>
                <p class="mt-1 text-base font-semibold tabular-nums text-gray-900 dark:text-white">{{ overview.leaderboard.length }}/20</p>
              </div>
            </div>
          </div>
        </section>

        <section>
          <div class="mb-3 flex items-center justify-between gap-4">
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('usageRebate.leaderboard') }}</h2>
            <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('usageRebate.balanceOnly') }}</span>
          </div>
          <div class="overflow-hidden rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900">
            <div class="overflow-x-auto">
              <table class="w-full min-w-[760px] text-left text-sm">
                <thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-800 dark:text-dark-400">
                  <tr>
                    <th class="w-20 px-4 py-3 font-medium">{{ t('usageRebate.rank') }}</th>
                    <th class="px-4 py-3 font-medium">{{ t('usageRebate.user') }}</th>
                    <th class="px-4 py-3 text-right font-medium">{{ t('usageRebate.requests') }}</th>
                    <th class="px-4 py-3 text-right font-medium">{{ t('usageRebate.tokens') }}</th>
                    <th class="px-4 py-3 text-right font-medium">{{ t('usageRebate.spend') }}</th>
                    <th class="px-4 py-3 text-right font-medium">{{ t('usageRebate.rate') }}</th>
                    <th class="px-4 py-3 text-right font-medium">{{ t('usageRebate.estimated') }}</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-100 dark:divide-dark-800">
                  <tr v-if="overview.leaderboard.length === 0">
                    <td colspan="7" class="px-4 py-12 text-center text-gray-500 dark:text-dark-400">
                      {{ overview.enabled ? t('usageRebate.empty') : t('usageRebate.disabledEmpty') }}
                    </td>
                  </tr>
                  <tr
                    v-for="row in overview.leaderboard"
                    :key="row.rank"
                    :class="row.is_me ? 'bg-primary-50/60 dark:bg-primary-900/10' : ''"
                    class="transition-colors hover:bg-gray-50 dark:hover:bg-dark-800/60"
                  >
                    <td class="px-4 py-3">
                      <span
                        :class="rankClass(row.rank)"
                        class="inline-flex h-7 min-w-7 items-center justify-center rounded-full px-2 text-xs font-semibold tabular-nums"
                      >{{ row.rank }}</span>
                    </td>
                    <td class="px-4 py-3 font-medium text-gray-900 dark:text-white">{{ row.username }}</td>
                    <td class="px-4 py-3 text-right tabular-nums text-gray-600 dark:text-gray-300">{{ formatInteger(row.requests) }}</td>
                    <td class="px-4 py-3 text-right tabular-nums text-gray-600 dark:text-gray-300">{{ formatCompact(row.tokens) }}</td>
                    <td class="px-4 py-3 text-right font-medium tabular-nums text-gray-900 dark:text-white">{{ formatUSD(row.spend_amount) }}</td>
                    <td class="px-4 py-3 text-right font-medium tabular-nums text-primary-600 dark:text-primary-400">{{ formatPercent(row.rebate_percent) }}</td>
                    <td class="px-4 py-3 text-right font-semibold tabular-nums text-emerald-600 dark:text-emerald-400">{{ formatUSD(row.estimated_reward) }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </section>

        <section>
          <h2 class="mb-3 text-base font-semibold text-gray-900 dark:text-white">{{ t('usageRebate.myRewards') }}</h2>
          <UsageRebateTrendChart
            class="mb-4"
            :trend-data="trendData"
            :rewards="overview.my_rewards"
            :start-date="trendStartDate"
            :end-date="trendEndDate"
            :loading="trendLoading"
            :unavailable="trendUnavailable"
          />
          <div class="overflow-hidden rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900">
            <div class="overflow-x-auto">
              <table class="w-full min-w-[620px] text-left text-sm">
                <thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-800 dark:text-dark-400">
                  <tr>
                    <th class="px-4 py-3 font-medium">{{ t('usageRebate.date') }}</th>
                    <th class="px-4 py-3 text-right font-medium">{{ t('usageRebate.rank') }}</th>
                    <th class="px-4 py-3 text-right font-medium">{{ t('usageRebate.spend') }}</th>
                    <th class="px-4 py-3 text-right font-medium">{{ t('usageRebate.rate') }}</th>
                    <th class="px-4 py-3 text-right font-medium">{{ t('usageRebate.reward') }}</th>
                    <th class="px-4 py-3 text-right font-medium">{{ t('usageRebate.status') }}</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-100 dark:divide-dark-800">
                  <tr v-if="overview.my_rewards.length === 0">
                    <td colspan="6" class="px-4 py-10 text-center text-gray-500 dark:text-dark-400">{{ t('usageRebate.noRewards') }}</td>
                  </tr>
                  <tr v-for="reward in overview.my_rewards" :key="reward.business_date">
                    <td class="px-4 py-3 text-gray-700 dark:text-gray-300">{{ reward.business_date }}</td>
                    <td class="px-4 py-3 text-right tabular-nums text-gray-700 dark:text-gray-300">#{{ reward.rank }}</td>
                    <td class="px-4 py-3 text-right tabular-nums text-gray-700 dark:text-gray-300">{{ formatUSD(reward.spend_amount) }}</td>
                    <td class="px-4 py-3 text-right tabular-nums text-gray-700 dark:text-gray-300">{{ formatPercent(reward.rebate_percent) }}</td>
                    <td class="px-4 py-3 text-right font-semibold tabular-nums text-emerald-600 dark:text-emerald-400">{{ formatUSD(reward.reward_amount) }}</td>
                    <td class="px-4 py-3 text-right">
                      <span :class="statusClass(reward.status)" class="inline-flex rounded-full px-2 py-0.5 text-xs font-medium">
                        {{ t(`usageRebate.statuses.${reward.status}`) }}
                      </span>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </section>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import UsageRebateTrendChart from '@/components/user/UsageRebateTrendChart.vue'
import usageRebateAPI, { type UsageRebateOverview, type UsageRebateReward } from '@/api/usageRebate'
import usageAPI from '@/api/usage'
import type { TrendDataPoint } from '@/types'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()
const loading = ref(true)
const overview = ref<UsageRebateOverview | null>(null)
const trendLoading = ref(false)
const trendUnavailable = ref(false)
const trendData = ref<TrendDataPoint[]>([])
const trendStartDate = ref('')
const trendEndDate = ref('')

function numeric(value: string | number): number {
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : 0
}

function formatUSD(value: string | number): string {
  return `$${numeric(value).toFixed(8).replace(/0+$/, '').replace(/\.$/, '')}`
}

function formatPercent(value: string | number): string {
  return `${numeric(value).toFixed(1).replace(/\.0$/, '')}%`
}

function formatInteger(value: number): string {
  return Math.max(0, Number(value) || 0).toLocaleString()
}

function formatCompact(value: number): string {
  return new Intl.NumberFormat(undefined, { notation: 'compact', maximumFractionDigits: 1 }).format(Math.max(0, Number(value) || 0))
}

function rankClass(rank: number): string {
  if (rank === 1) return 'bg-amber-100 text-amber-800 dark:bg-amber-900/30 dark:text-amber-300'
  if (rank === 2) return 'bg-gray-200 text-gray-700 dark:bg-gray-700 dark:text-gray-200'
  if (rank === 3) return 'bg-orange-100 text-orange-800 dark:bg-orange-900/30 dark:text-orange-300'
  return 'bg-gray-100 text-gray-600 dark:bg-dark-800 dark:text-gray-300'
}

function statusClass(status: UsageRebateReward['status']): string {
  if (status === 'credited') return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/25 dark:text-emerald-300'
  if (status === 'unknown') return 'bg-amber-50 text-amber-700 dark:bg-amber-900/25 dark:text-amber-300'
  if (status === 'failed') return 'bg-red-50 text-red-700 dark:bg-red-900/25 dark:text-red-300'
  return 'bg-blue-50 text-blue-700 dark:bg-blue-900/25 dark:text-blue-300'
}

function shiftISODate(value: string, days: number): string {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value)
  if (!match) return value
  const date = new Date(Date.UTC(Number(match[1]), Number(match[2]) - 1, Number(match[3])))
  date.setUTCDate(date.getUTCDate() + days)
  return date.toISOString().slice(0, 10)
}

async function loadTrend(endDate: string): Promise<void> {
  trendEndDate.value = endDate
  trendStartDate.value = shiftISODate(endDate, -29)
  trendLoading.value = true
  trendUnavailable.value = false
  try {
    const response = await usageAPI.getDashboardTrend({
      start_date: trendStartDate.value,
      end_date: trendEndDate.value,
      granularity: 'day',
      billing_type: 0,
      timezone: 'Asia/Shanghai',
    })
    trendData.value = Array.isArray(response.trend) ? response.trend : []
  } catch {
    trendData.value = []
    trendUnavailable.value = true
  } finally {
    trendLoading.value = false
  }
}

async function loadOverview(): Promise<void> {
  loading.value = true
  let loadedOverview: UsageRebateOverview | null = null
  try {
    loadedOverview = await usageRebateAPI.getOverview()
    overview.value = loadedOverview
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('usageRebate.loadFailed')))
  } finally {
    loading.value = false
  }

  if (loadedOverview) {
    await loadTrend(loadedOverview.business_date)
  }
}

onMounted(() => void loadOverview())
</script>
