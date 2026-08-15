<template>
  <BaseDialog :show="store.open" :title="t('dailyCheckin.title')" width="normal" @close="store.close">
    <div v-if="store.loading && !status" class="flex min-h-72 items-center justify-center">
      <Icon name="refresh" size="lg" class="animate-spin text-primary-500" />
    </div>
    <div v-else-if="store.statusError || !status" class="py-14 text-center">
      <Icon name="infoCircle" size="xl" class="mx-auto text-amber-500" />
      <p class="mt-3 text-sm text-gray-500 dark:text-dark-400">{{ t('dailyCheckin.loadFailed') }}</p>
      <button type="button" class="btn btn-secondary mt-4" @click="store.show">{{ t('dailyCheckin.retry') }}</button>
    </div>
    <div v-else-if="!status.config.enabled" class="py-14 text-center">
      <Icon name="calendar" size="xl" class="mx-auto text-gray-300 dark:text-dark-600" />
      <p class="mt-3 text-sm text-gray-500 dark:text-dark-400">{{ t('dailyCheckin.disabled') }}</p>
    </div>
    <div v-else class="space-y-4">
      <section class="grid grid-cols-[auto_minmax(0,1fr)] items-center gap-3 rounded-lg border border-emerald-200 bg-emerald-50/70 p-3 dark:border-emerald-900 dark:bg-emerald-950/20 sm:grid-cols-[auto_minmax(0,1fr)_auto]">
        <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-emerald-600 text-white dark:bg-emerald-500 dark:text-emerald-950">
          <Icon name="gift" size="md" />
        </div>
        <div class="min-w-0">
          <div class="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
            <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('dailyCheckin.todayReward') }}</span>
            <span class="text-xl font-semibold text-gray-900 dark:text-white">{{ money(todayReward) }}</span>
            <span v-if="todayMilestone > 0" class="text-xs font-medium text-emerald-700 dark:text-emerald-300">
              {{ t('dailyCheckin.includesMilestone', { amount: money(todayMilestone) }) }}
            </span>
          </div>
          <p class="mt-0.5 truncate text-xs text-gray-500 dark:text-dark-400" :title="nextMilestoneText">{{ nextMilestoneText }}</p>
        </div>
        <button
          type="button"
          class="col-span-2 flex h-10 items-center justify-center gap-2 rounded-lg bg-primary-600 px-4 text-sm font-semibold text-white transition-colors hover:bg-primary-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/40 focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:bg-gray-300 dark:focus-visible:ring-offset-dark-800 dark:disabled:bg-dark-700 sm:col-span-1 sm:min-w-28"
          :disabled="status.today_claimed || !status.eligible || store.claiming"
          @click="claim"
        >
          <Icon :name="status.today_claimed ? 'checkCircle' : 'calendar'" size="sm" :class="{ 'animate-pulse': store.claiming }" />
          {{ status.today_claimed ? t('dailyCheckin.claimed') : store.claiming ? t('dailyCheckin.claiming') : t('dailyCheckin.claimNow') }}
        </button>
      </section>

      <p v-if="!status.today_claimed && !status.eligible" class="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-700 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-300">
        {{ eligibilityMessage }}
      </p>

      <section class="grid grid-cols-3 divide-x divide-gray-200 rounded-lg border border-gray-200 py-2.5 dark:divide-dark-700 dark:border-dark-700">
        <div v-for="item in summaryItems" :key="item.label" class="min-w-0 px-2 text-center sm:px-3">
          <p class="truncate text-xs text-gray-500 dark:text-dark-400">{{ item.label }}</p>
          <p class="mt-0.5 truncate text-base font-semibold text-gray-900 dark:text-white">{{ item.value }}</p>
        </div>
      </section>

      <section class="rounded-lg border border-gray-200 px-3 py-3 dark:border-dark-700">
        <div class="flex items-center justify-between gap-3">
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('dailyCheckin.milestones') }}</h3>
          <span class="text-xs tabular-nums text-gray-500 dark:text-dark-400">{{ cycleCount }} / {{ cycleDays }}</span>
        </div>
        <div class="relative mt-3 h-1.5 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700">
          <div class="h-full rounded-full bg-emerald-500 transition-[width] duration-300" :style="{ width: cycleProgressWidth }" />
        </div>
        <div class="mt-2 grid grid-cols-3 gap-2">
          <div v-for="milestone in milestones" :key="milestone.day" class="flex min-w-0 items-center justify-center gap-1 text-xs" :class="milestone.reached ? 'font-medium text-emerald-700 dark:text-emerald-300' : 'text-gray-500 dark:text-dark-400'">
            <Icon v-if="milestone.reached" name="check" size="xs" class="shrink-0" />
            <span class="truncate">{{ t('dailyCheckin.milestoneCompact', { day: milestone.day, amount: money(milestone.reward) }) }}</span>
          </div>
        </div>
        <p class="mt-2 text-center text-xs text-gray-500 dark:text-dark-400">
          {{ t('dailyCheckin.progressMeta', { consecutive: cycle?.consecutive_days ?? 0, cycles: status.completed_cycles ?? 0 }) }}
        </p>
      </section>

      <div class="flex border-b border-gray-200 dark:border-dark-700" role="tablist">
        <button v-for="tab in tabs" :key="tab.key" type="button" role="tab" :aria-selected="activeTab === tab.key" class="border-b-2 px-4 py-2 text-sm font-medium" :class="activeTab === tab.key ? 'border-primary-500 text-primary-600 dark:text-primary-400' : 'border-transparent text-gray-500 hover:text-gray-800 dark:text-dark-400 dark:hover:text-white'" @click="activeTab = tab.key">
          {{ tab.label }}
        </button>
      </div>

      <section v-if="activeTab === 'calendar'" role="tabpanel">
        <div class="mb-2 flex items-center justify-between gap-2">
          <div class="flex items-center gap-1">
            <button type="button" class="flex h-8 w-8 items-center justify-center rounded-md text-gray-500 hover:bg-gray-100 hover:text-gray-800 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/40 dark:text-dark-400 dark:hover:bg-dark-700 dark:hover:text-white" :aria-label="t('dailyCheckin.previousMonth')" :title="t('dailyCheckin.previousMonth')" @click="changeMonth(-1)">
              <Icon name="chevronLeft" size="sm" />
            </button>
            <h3 class="min-w-28 text-center text-sm font-semibold text-gray-900 dark:text-white">{{ monthTitle }}</h3>
            <button type="button" class="flex h-8 w-8 items-center justify-center rounded-md text-gray-500 hover:bg-gray-100 hover:text-gray-800 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/40 disabled:cursor-not-allowed disabled:opacity-30 dark:text-dark-400 dark:hover:bg-dark-700 dark:hover:text-white" :aria-label="t('dailyCheckin.nextMonth')" :title="t('dailyCheckin.nextMonth')" :disabled="isCurrentMonth" @click="changeMonth(1)">
              <Icon name="chevronRight" size="sm" />
            </button>
          </div>
          <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('dailyCheckin.shanghaiTime') }}</span>
        </div>
        <div class="grid grid-cols-7 gap-1 text-center text-xs">
          <div v-for="weekday in weekdays" :key="weekday" class="py-1 text-gray-400">{{ weekday }}</div>
          <div v-for="(day, index) in calendarDays" :key="index" class="flex h-8 items-center justify-center rounded-md sm:h-9" :class="calendarClass(day)">
            <span v-if="day">{{ day }}</span>
          </div>
        </div>
      </section>

      <section v-else class="max-h-64 overflow-y-auto" role="tabpanel">
        <div v-if="store.history.length === 0" class="py-10 text-center text-sm text-gray-500 dark:text-dark-400">{{ t('dailyCheckin.noHistory') }}</div>
        <div v-for="record in store.history" :key="record.id" class="flex items-center justify-between border-b border-gray-100 py-2.5 last:border-0 dark:border-dark-700">
          <div>
            <p class="text-sm font-medium text-gray-900 dark:text-white">{{ businessDate(record.business_date) }}</p>
            <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">{{ t('dailyCheckin.cycleDay', { day: record.cycle_day }) }}</p>
          </div>
          <div class="text-right">
            <p class="text-sm font-semibold text-emerald-600 dark:text-emerald-400">+{{ money(record.total_reward) }}</p>
            <p v-if="record.milestone_reward > 0" class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">{{ t('dailyCheckin.milestoneReward') }}</p>
          </div>
        </div>
      </section>

      <section class="border-t border-gray-100 pt-3 dark:border-dark-700">
        <h3 class="flex items-center gap-1.5 text-sm font-semibold text-gray-900 dark:text-white">
          <Icon name="infoCircle" size="sm" class="text-gray-400" />
          {{ t('dailyCheckin.rulesTitle') }}
        </h3>
        <ul class="mt-2 list-disc space-y-1 pl-5 text-xs leading-5 text-gray-500 dark:text-dark-400">
          <li v-for="rule in rules" :key="rule">{{ rule }}</li>
        </ul>
      </section>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore, useAuthStore, useDailyCheckinStore } from '@/stores'

const { t, locale } = useI18n()
const store = useDailyCheckinStore()
const authStore = useAuthStore()
const appStore = useAppStore()
const activeTab = ref<'calendar' | 'history'>('calendar')
const displayedMonth = ref('')
const status = computed(() => store.status)
const cycle = computed(() => status.value?.current_cycle)
const cycleCount = computed(() => cycle.value?.checkin_count ?? 0)
const cycleDays = computed(() => cycle.value?.cycle_days ?? status.value?.config.cycle_days ?? 30)
const nextDay = computed(() => Math.min(cycleCount.value + 1, cycleDays.value))
const todayRecord = computed(() => store.history.find((item) => businessDate(item.business_date) === status.value?.today))
const rewardDay = computed(() => status.value?.today_claimed ? cycleCount.value : nextDay.value)
const todayMilestone = computed(() => Number(todayRecord.value?.milestone_reward ?? milestoneReward(rewardDay.value)))
const todayReward = computed(() => Number(todayRecord.value?.total_reward ?? (Number(cycle.value?.base_reward ?? status.value?.config.base_reward ?? 0) + todayMilestone.value)))
const milestones = computed(() => [
  { day: 7, reward: Number(cycle.value?.milestone_7_reward ?? status.value?.config.milestone_7 ?? 0), reached: cycleCount.value >= 7 },
  { day: 15, reward: Number(cycle.value?.milestone_15_reward ?? status.value?.config.milestone_15 ?? 0), reached: cycleCount.value >= 15 },
  { day: 30, reward: Number(cycle.value?.milestone_30_reward ?? status.value?.config.milestone_30 ?? 0), reached: cycleCount.value >= 30 }
])
const cycleProgressWidth = computed(() => `${Math.min(100, Math.max(0, cycleCount.value / cycleDays.value * 100))}%`)
const summaryItems = computed(() => [
  { label: t('dailyCheckin.cycleProgress'), value: `${cycleCount.value}/${cycleDays.value}` },
  { label: t('dailyCheckin.cycleEarned'), value: money(cycle.value?.total_reward ?? 0) },
  { label: t('dailyCheckin.historyEarned'), value: money(status.value?.history_reward ?? 0) }
])
const nextMilestoneText = computed(() => status.value?.next_milestone
  ? t('dailyCheckin.nextMilestone', { days: status.value.days_to_next_milestone, milestone: status.value.next_milestone })
  : t('dailyCheckin.cycleCompleteSoon'))
const eligibilityMessage = computed(() => {
  const reason = status.value?.ineligible_reason
  if (reason === 'account_too_new') return t('dailyCheckin.accountTooNew')
  if (reason === 'verification_required') return t('dailyCheckin.verificationRequired')
  if (reason === 'inactive') return t('dailyCheckin.accountInactive')
  return t('dailyCheckin.notEligible')
})
const tabs = computed(() => [{ key: 'calendar' as const, label: t('dailyCheckin.calendar') }, { key: 'history' as const, label: t('dailyCheckin.history') }])
const rules = computed(() => [
  t('dailyCheckin.ruleDaily'),
  t('dailyCheckin.ruleBaseReward'),
  t('dailyCheckin.ruleMilestones'),
  t('dailyCheckin.ruleMissedDay'),
  t('dailyCheckin.ruleCycle'),
  t('dailyCheckin.ruleBalance')
])
const shanghaiToday = computed(() => (status.value?.today || new Date().toLocaleDateString('en-CA', { timeZone: 'Asia/Shanghai' })).slice(0, 10))
const calendarContext = computed(() => {
  const [year, month] = (displayedMonth.value || shanghaiToday.value.slice(0, 7)).split('-').map(Number)
  return { year, month: month - 1 }
})
const monthTitle = computed(() => new Intl.DateTimeFormat(locale.value, { year: 'numeric', month: 'long', timeZone: 'Asia/Shanghai' }).format(new Date(Date.UTC(calendarContext.value.year, calendarContext.value.month, 1))))
const isCurrentMonth = computed(() => (displayedMonth.value || shanghaiToday.value.slice(0, 7)) >= shanghaiToday.value.slice(0, 7))
const weekdays = computed(() => locale.value === 'zh' ? ['一', '二', '三', '四', '五', '六', '日'] : ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'])
const calendarDays = computed(() => {
  const { year, month } = calendarContext.value
  const first = new Date(Date.UTC(year, month, 1))
  const offset = (first.getUTCDay() + 6) % 7
  const count = new Date(Date.UTC(year, month + 1, 0)).getUTCDate()
  return [...Array(offset).fill(null), ...Array.from({ length: count }, (_, i) => i + 1)]
})
const claimedDates = computed(() => new Set(store.history.map((item) => businessDate(item.business_date))))

watch(shanghaiToday, (today) => {
  if (!displayedMonth.value) displayedMonth.value = today.slice(0, 7)
}, { immediate: true })

function milestoneReward(day: number): number {
  return milestones.value.find((item) => item.day === day)?.reward ?? 0
}
function money(value: number): string { return `$${Number(value || 0).toFixed(2)}` }
function businessDate(value: string): string { return value.slice(0, 10) }
function changeMonth(offset: number): void {
  const { year, month } = calendarContext.value
  const next = new Date(Date.UTC(year, month + offset, 1))
  displayedMonth.value = `${next.getUTCFullYear()}-${String(next.getUTCMonth() + 1).padStart(2, '0')}`
}
function calendarClass(day: number | null): string {
  if (!day) return ''
  const { year, month } = calendarContext.value
  const date = `${year}-${String(month + 1).padStart(2, '0')}-${String(day).padStart(2, '0')}`
  if (claimedDates.value.has(date)) return 'bg-emerald-100 font-semibold text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
  if (date === status.value?.today) return 'border border-primary-400 font-medium text-primary-600 dark:text-primary-400'
  return 'text-gray-600 dark:text-dark-300'
}
async function claim(): Promise<void> {
  try {
    const record = await store.claim()
    await authStore.refreshUser()
    appStore.showSuccess(t('dailyCheckin.claimSuccess', { amount: money(record.total_reward) }))
  } catch (error) {
    const message = error instanceof Error ? error.message : t('dailyCheckin.claimFailed')
    appStore.showError(message)
  }
}
</script>
