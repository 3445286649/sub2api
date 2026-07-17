<template>
  <div class="overflow-hidden rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900">
    <div class="flex flex-col gap-1 border-b border-gray-100 px-4 py-3 dark:border-dark-800 sm:flex-row sm:items-center sm:justify-between">
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
        {{ t('usageRebate.trend.title') }}
      </h3>
      <span class="text-xs text-gray-500 dark:text-dark-400">
        {{ t('usageRebate.trend.period', { start: startDate, end: endDate }) }}
      </span>
    </div>

    <div v-if="loading" class="flex h-64 items-center justify-center" aria-live="polite">
      <div class="h-7 w-7 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
    </div>
    <div v-else-if="unavailable" class="flex h-48 items-center justify-center px-4 text-sm text-gray-500 dark:text-dark-400">
      {{ t('usageRebate.trend.loadFailed') }}
    </div>
    <div v-else-if="hasData && chartData" class="h-64 p-4">
      <Line :data="chartData" :options="chartOptions" />
    </div>
    <div v-else class="flex h-48 items-center justify-center px-4 text-sm text-gray-500 dark:text-dark-400">
      {{ t('usageRebate.trend.noData') }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  CategoryScale,
  Chart as ChartJS,
  Filler,
  Legend,
  LineElement,
  LinearScale,
  PointElement,
  Tooltip,
  type ChartData,
  type ChartOptions,
  type TooltipItem,
} from 'chart.js'
import { Line } from 'vue-chartjs'

import type { UsageRebateReward } from '@/api/usageRebate'
import type { TrendDataPoint } from '@/types'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend, Filler)

interface RebateTrendPoint {
  date: string
  spend: number
  reward: number
  rewardRecord?: UsageRebateReward
}

const props = withDefaults(defineProps<{
  trendData: TrendDataPoint[]
  rewards: UsageRebateReward[]
  startDate: string
  endDate: string
  loading?: boolean
  unavailable?: boolean
}>(), {
  loading: false,
  unavailable: false,
})

const { t } = useI18n()

function numeric(value: string | number): number {
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : 0
}

function parseISODate(value: string): Date | null {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value)
  if (!match) return null
  return new Date(Date.UTC(Number(match[1]), Number(match[2]) - 1, Number(match[3])))
}

function formatISODate(value: Date): string {
  return value.toISOString().slice(0, 10)
}

function buildDateRange(startDate: string, endDate: string): string[] {
  const start = parseISODate(startDate)
  const end = parseISODate(endDate)
  if (!start || !end || start > end) return []

  const dates: string[] = []
  for (const cursor = new Date(start); cursor <= end; cursor.setUTCDate(cursor.getUTCDate() + 1)) {
    dates.push(formatISODate(cursor))
  }
  return dates
}

function formatMoney(value: string | number): string {
  return `$${numeric(value).toFixed(8).replace(/0+$/, '').replace(/\.$/, '')}`
}

function formatPercent(value: string | number): string {
  return `${numeric(value).toFixed(1).replace(/\.0$/, '')}%`
}

const points = computed<RebateTrendPoint[]>(() => {
  const spendByDate = new Map<string, number>()
  for (const item of props.trendData) {
    const date = item.date.slice(0, 10)
    spendByDate.set(date, (spendByDate.get(date) ?? 0) + numeric(item.actual_cost))
  }

  const rewardByDate = new Map(props.rewards.map((reward) => [reward.business_date, reward]))

  return buildDateRange(props.startDate, props.endDate).map((date) => {
    const rewardRecord = rewardByDate.get(date)
    return {
      date,
      spend: spendByDate.get(date) ?? 0,
      reward: rewardRecord ? numeric(rewardRecord.reward_amount) : 0,
      rewardRecord,
    }
  })
})

const hasData = computed(() => props.trendData.length > 0 || props.rewards.length > 0)

const isDarkMode = computed(() => document.documentElement.classList.contains('dark'))
const colors = computed(() => ({
  text: isDarkMode.value ? '#d1d5db' : '#4b5563',
  grid: isDarkMode.value ? '#374151' : '#e5e7eb',
  spend: '#2563eb',
  reward: '#10b981',
}))

const chartData = computed<ChartData<'line', number[], string> | null>(() => {
  if (points.value.length === 0) return null

  return {
    labels: points.value.map((point) => point.date),
    datasets: [
      {
        label: t('usageRebate.trend.spend'),
        data: points.value.map((point) => point.spend),
        yAxisID: 'ySpend',
        borderColor: colors.value.spend,
        backgroundColor: `${colors.value.spend}18`,
        pointBackgroundColor: colors.value.spend,
        pointRadius: 2,
        pointHoverRadius: 5,
        borderWidth: 2,
        fill: true,
        tension: 0.3,
      },
      {
        label: t('usageRebate.trend.reward'),
        data: points.value.map((point) => point.reward),
        yAxisID: 'yReward',
        borderColor: colors.value.reward,
        backgroundColor: colors.value.reward,
        pointBackgroundColor: colors.value.reward,
        pointRadius: 2,
        pointHoverRadius: 5,
        borderWidth: 2,
        fill: false,
        tension: 0.3,
      },
    ],
  }
})

const chartOptions = computed<ChartOptions<'line'>>(() => ({
  responsive: true,
  maintainAspectRatio: false,
  interaction: {
    intersect: false,
    mode: 'index',
  },
  plugins: {
    legend: {
      position: 'top',
      align: 'end',
      labels: {
        color: colors.value.text,
        usePointStyle: true,
        pointStyle: 'circle',
        boxWidth: 8,
        boxHeight: 8,
      },
    },
    tooltip: {
      callbacks: {
        title: (items: TooltipItem<'line'>[]) => points.value[items[0]?.dataIndex]?.date ?? '',
        label: (context: TooltipItem<'line'>) => {
          const label = context.dataset.yAxisID === 'yReward'
            ? t('usageRebate.trend.reward')
            : t('usageRebate.trend.spend')
          return `${label}: ${formatMoney(Number(context.raw))}`
        },
        afterBody: (items: TooltipItem<'line'>[]) => {
          const reward = points.value[items[0]?.dataIndex]?.rewardRecord
          if (!reward) return [t('usageRebate.trend.notRanked')]
          return [
            `${t('usageRebate.rank')}: #${reward.rank}`,
            `${t('usageRebate.rate')}: ${formatPercent(reward.rebate_percent)}`,
            `${t('usageRebate.status')}: ${t(`usageRebate.statuses.${reward.status}`)}`,
          ]
        },
      },
    },
  },
  scales: {
    x: {
      grid: { display: false },
      ticks: {
        color: colors.value.text,
        autoSkip: true,
        maxTicksLimit: 8,
        maxRotation: 0,
        callback: (_value, index) => points.value[index]?.date.slice(5) ?? '',
      },
    },
    ySpend: {
      beginAtZero: true,
      position: 'left',
      grid: { color: colors.value.grid },
      ticks: {
        color: colors.value.spend,
        callback: (value) => formatMoney(Number(value)),
      },
    },
    yReward: {
      beginAtZero: true,
      position: 'right',
      grid: { drawOnChartArea: false },
      ticks: {
        color: colors.value.reward,
        callback: (value) => formatMoney(Number(value)),
      },
    },
  },
}))
</script>
