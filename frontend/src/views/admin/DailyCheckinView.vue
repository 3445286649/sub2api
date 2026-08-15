<template>
  <AppLayout>
    <div class="space-y-5">
      <div>
        <h1 class="text-xl font-semibold text-gray-900 dark:text-white">{{ t('dailyCheckin.adminTitle') }}</h1>
        <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('dailyCheckin.adminDescription') }}</p>
      </div>

      <div v-if="loading" class="flex justify-center py-20"><div class="h-8 w-8 animate-spin rounded-full border-4 border-primary-500 border-t-transparent" /></div>
      <template v-else>
        <section class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
          <div v-for="item in statItems" :key="item.label" class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
            <p class="text-xs text-gray-500 dark:text-dark-400">{{ item.label }}</p>
            <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ item.value }}</p>
          </div>
        </section>

        <form class="rounded-lg border border-gray-200 bg-white p-5 dark:border-dark-700 dark:bg-dark-800" @submit.prevent="saveConfig">
          <div class="flex items-center justify-between border-b border-gray-100 pb-4 dark:border-dark-700">
            <div>
              <p class="font-medium text-gray-900 dark:text-white">{{ t('dailyCheckin.featureEnabled') }}</p>
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">v{{ config.rule_version }}</p>
            </div>
            <ToggleSwitch :label="t('dailyCheckin.featureEnabled')" :checked="config.enabled" @toggle="config.enabled = !config.enabled" />
          </div>
          <div class="mt-4 grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
            <label class="space-y-1"><span class="input-label">{{ t('dailyCheckin.baseReward') }}</span><input v-model.number="config.base_reward" class="input" type="number" min="0" max="1000000" step="0.01" required /></label>
            <label class="space-y-1"><span class="input-label">{{ t('dailyCheckin.milestone7') }}</span><input v-model.number="config.milestone_7" class="input" type="number" min="0" max="1000000" step="0.01" required /></label>
            <label class="space-y-1"><span class="input-label">{{ t('dailyCheckin.milestone15') }}</span><input v-model.number="config.milestone_15" class="input" type="number" min="0" max="1000000" step="0.01" required /></label>
            <label class="space-y-1"><span class="input-label">{{ t('dailyCheckin.milestone30') }}</span><input v-model.number="config.milestone_30" class="input" type="number" min="0" max="1000000" step="0.01" required /></label>
            <label class="space-y-1"><span class="input-label">{{ t('dailyCheckin.cycleDays') }}</span><input v-model.number="config.cycle_days" class="input bg-gray-50 dark:bg-dark-900" type="number" readonly /></label>
            <label class="space-y-1"><span class="input-label">{{ t('dailyCheckin.minAccountAge') }}</span><input v-model.number="config.min_account_age_hours" class="input" type="number" min="0" max="87600" step="1" required /></label>
            <label class="space-y-1"><span class="input-label">{{ t('dailyCheckin.dailyBudget') }}</span><input v-model.number="config.daily_budget" class="input" type="number" min="0" max="1000000" step="0.01" required /></label>
            <div class="flex items-end justify-between rounded-lg border border-gray-200 px-3 py-2 dark:border-dark-700">
              <span class="input-label">{{ t('dailyCheckin.requireVerified') }}</span>
              <ToggleSwitch :label="t('dailyCheckin.requireVerified')" :checked="config.require_verified" @toggle="config.require_verified = !config.require_verified" />
            </div>
          </div>
          <div class="mt-5 flex justify-end"><button class="btn btn-primary" type="submit" :disabled="saving">{{ t('dailyCheckin.saveConfig') }}</button></div>
        </form>

        <section class="overflow-hidden rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800">
          <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-100 px-4 py-3 dark:border-dark-700">
            <h2 class="font-medium text-gray-900 dark:text-white">{{ t('dailyCheckin.records') }}</h2>
            <input v-model="dateFilter" class="input w-44" type="date" @change="loadRecords" />
          </div>
          <div class="overflow-x-auto">
            <table class="min-w-full divide-y divide-gray-200 text-sm dark:divide-dark-700">
              <thead class="bg-gray-50 text-left text-xs text-gray-500 dark:bg-dark-900/50 dark:text-dark-400"><tr>
                <th class="px-4 py-3">{{ t('points.user') }}</th><th class="px-4 py-3">{{ t('dailyCheckin.date') }}</th><th class="px-4 py-3">{{ t('dailyCheckin.cycleDayColumn') }}</th><th class="px-4 py-3">{{ t('dailyCheckin.baseRewardColumn') }}</th><th class="px-4 py-3">{{ t('dailyCheckin.milestoneRewardColumn') }}</th><th class="px-4 py-3">{{ t('dailyCheckin.totalRewardColumn') }}</th><th class="px-4 py-3">{{ t('dailyCheckin.balanceAfter') }}</th>
              </tr></thead>
              <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                <tr v-for="record in records" :key="record.id">
                  <td class="px-4 py-3"><p class="font-medium text-gray-900 dark:text-white">{{ record.user_email }}</p><p class="text-xs text-gray-400">ID {{ record.user_id }}</p></td>
                  <td class="px-4 py-3">{{ record.business_date.slice(0, 10) }}</td><td class="px-4 py-3">{{ record.cycle_day }}</td><td class="px-4 py-3">{{ money(record.base_reward) }}</td><td class="px-4 py-3">{{ money(record.milestone_reward) }}</td><td class="px-4 py-3 font-semibold text-emerald-600">{{ money(record.total_reward) }}</td><td class="px-4 py-3">{{ money(record.balance_after) }}</td>
                </tr>
                <tr v-if="records.length === 0"><td colspan="7" class="px-4 py-12 text-center text-gray-500">{{ t('dailyCheckin.noRecords') }}</td></tr>
              </tbody>
            </table>
          </div>
        </section>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import ToggleSwitch from '@/components/payment/ToggleSwitch.vue'
import { adminDailyCheckinAPI } from '@/api/admin/dailyCheckin'
import { useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { DailyCheckinConfig, DailyCheckinRecord, DailyCheckinStats } from '@/types/dailyCheckin'

const { t } = useI18n()
const appStore = useAppStore()
const loading = ref(false)
const saving = ref(false)
const dateFilter = ref('')
const records = ref<DailyCheckinRecord[]>([])
const stats = reactive<DailyCheckinStats>({ today_claims: 0, today_reward: 0, month_reward: 0, completed_cycles: 0 })
const config = reactive<DailyCheckinConfig>({ enabled: false, base_reward: 0.13, cycle_days: 30, milestone_7: 2, milestone_15: 5, milestone_30: 8, min_account_age_hours: 0, require_verified: false, daily_budget: 0, rule_version: 1 })
const statItems = computed(() => [
  { label: t('dailyCheckin.todayClaims'), value: String(stats.today_claims) },
  { label: t('dailyCheckin.todayIssued'), value: money(stats.today_reward) },
  { label: t('dailyCheckin.monthIssued'), value: money(stats.month_reward) },
  { label: t('dailyCheckin.completedCyclesMonth'), value: String(stats.completed_cycles) }
])
function money(value: number): string { return `$${Number(value || 0).toFixed(2)}` }
async function loadRecords(): Promise<void> { const response = await adminDailyCheckinAPI.getRecords({ page: 1, page_size: 100, date: dateFilter.value || undefined }); records.value = response.data.items || [] }
async function load(): Promise<void> {
  loading.value = true
  try { const [configRes, statsRes] = await Promise.all([adminDailyCheckinAPI.getConfig(), adminDailyCheckinAPI.getStats(), loadRecords()]); Object.assign(config, configRes.data); Object.assign(stats, statsRes.data) }
  catch (error) { appStore.showError(extractApiErrorMessage(error, t('common.error'))) }
  finally { loading.value = false }
}
async function saveConfig(): Promise<void> {
  saving.value = true
  try { const response = await adminDailyCheckinAPI.updateConfig({ ...config }); Object.assign(config, response.data); appStore.showSuccess(t('dailyCheckin.configSaved')) }
  catch (error) { appStore.showError(extractApiErrorMessage(error, t('common.error'))) }
  finally { saving.value = false }
}
onMounted(load)
</script>
