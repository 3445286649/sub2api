<template>
  <div class="space-y-5" data-testid="key-usage-panel">
    <form class="card space-y-4 p-4 sm:p-6" @submit.prevent="apply">
      <div class="grid grid-cols-2 items-end gap-3 lg:flex lg:flex-wrap">
        <div class="min-w-0 lg:w-40">
          <label class="input-label" for="key-usage-period">{{ t('admin.dashboard.timeRange') }}</label>
          <Select id="key-usage-period" :aria-label="t('admin.dashboard.timeRange')" v-model="period" :options="periodOptions" />
        </div>
        <div class="min-w-0 lg:w-48">
          <label class="input-label" for="key-usage-group">{{ t('admin.usage.group') }}</label>
          <Select id="key-usage-group" :aria-label="t('admin.usage.group')" v-model="groupId" :options="groupOptions" searchable />
        </div>
        <div class="min-w-0 lg:w-48">
          <label class="input-label" for="key-usage-search">{{ t('usage.keySummary.search') }}</label>
          <input id="key-usage-search" v-model="search" maxlength="100" class="input" :placeholder="t('usage.keySummary.search')" @change="apply" />
        </div>
        <div class="min-w-0 lg:w-44">
          <label class="input-label" for="key-usage-sort">{{ t('usage.keySummary.sort') }}</label>
          <Select id="key-usage-sort" :aria-label="t('usage.keySummary.sort')" v-model="sortBy" :options="sortOptions" />
        </div>
        <button type="submit" class="btn btn-secondary lg:self-end" :disabled="loading">{{ t('common.refresh') }}</button>
      </div>
      <div v-if="period === 'custom'" class="flex flex-wrap items-end gap-3">
        <div><label class="input-label" for="key-usage-start">{{ t('dates.startDate') }}</label><input id="key-usage-start" v-model="customStart" class="input" type="date" :max="customEnd" /></div>
        <div><label class="input-label" for="key-usage-end">{{ t('dates.endDate') }}</label><input id="key-usage-end" v-model="customEnd" class="input" type="date" :min="customStart" /></div>
        <button type="submit" class="btn btn-primary">{{ t('dates.apply') }}</button>
      </div>
      <div class="flex flex-wrap items-center gap-4 text-sm">
        <label class="flex items-center gap-2 text-gray-600 dark:text-gray-300"><input v-model="onlyUsed" type="checkbox" class="rounded" />{{ t('usage.keySummary.onlyUsed') }}</label>
        <button v-if="keyId" type="button" class="text-primary-600 dark:text-primary-400" @click="keyId = undefined">{{ t('usage.keySummary.showAll') }}</button>
        <span class="text-xs text-gray-500">{{ rangeLabel }}</span>
      </div>
      <p class="text-xs leading-5 text-gray-500 dark:text-gray-400">{{ t('usage.keySummary.hint') }}</p>
    </form>

    <div v-if="error" role="alert" class="card flex items-center justify-between gap-3 p-5 text-red-600 dark:text-red-400">
      <span>{{ error }}</span><button class="btn btn-secondary" @click="load">{{ t('common.tryAgain') }}</button>
    </div>
    <div v-else-if="loading" class="card flex justify-center p-12" role="status"><LoadingSpinner /></div>
    <template v-else-if="result">
      <div class="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <div v-for="stat in stats" :key="stat.label" class="card min-w-0 p-4">
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ stat.label }}</p>
          <p class="mt-2 break-words text-xl font-semibold tabular-nums text-gray-900 dark:text-white">{{ stat.value }}</p>
        </div>
      </div>
      <p v-if="result.summary.subscription_cost > 0" class="text-xs text-gray-500">{{ t('usage.keySummary.subscriptionHint') }} ${{ money(result.summary.subscription_cost) }}</p>
      <div class="card overflow-hidden">
        <div v-if="result.items.length === 0" class="p-12 text-center text-sm text-gray-500">{{ t('usage.keySummary.empty') }}</div>
        <template v-else>
          <div class="hidden overflow-x-auto md:block">
            <table class="w-full text-sm">
              <thead class="bg-gray-50 text-left text-xs text-gray-500 dark:bg-dark-800"><tr>
                <th class="px-5 py-3">{{ t('usage.keySummary.name') }}</th><th class="px-4 py-3">{{ t('usage.keySummary.currentGroup') }}</th>
                <th class="px-4 py-3 text-right">{{ t('usage.keySummary.requests') }}</th><th class="px-4 py-3 text-right">{{ t('usage.keySummary.tokens') }}</th>
                <th class="px-4 py-3 text-right">{{ t('usage.keySummary.cost') }}</th><th class="px-5 py-3">{{ t('common.actions') }}</th>
              </tr></thead>
              <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                <tr v-for="item in result.items" :key="item.api_key_id" class="text-gray-800 dark:text-gray-200">
                  <td class="max-w-64 px-5 py-4"><div class="break-words font-medium">{{ item.name }}</div><span v-if="item.deleted" class="text-xs text-gray-500">{{ t('usage.errors.keyDeleted') }}</span></td>
                  <td class="max-w-48 break-words px-4 py-4">{{ item.group_name || '—' }}</td>
                  <td class="px-4 py-4 text-right tabular-nums">{{ item.requests.toLocaleString() }}</td>
                  <td class="px-4 py-4 text-right tabular-nums"><details><summary class="cursor-pointer">{{ formatCompactNumber(item.total_tokens) }}</summary><div class="mt-2 whitespace-nowrap text-xs leading-6 text-gray-500">{{ tokenDetails(item) }}</div></details></td>
                  <td class="px-4 py-4 text-right font-medium tabular-nums">${{ money(item.actual_cost) }}</td>
                  <td class="px-5 py-4"><button class="whitespace-nowrap text-primary-600 dark:text-primary-400" @click="showDetails(item)">{{ t('usage.keySummary.details') }}</button></td>
                </tr>
              </tbody>
            </table>
          </div>
          <div class="divide-y divide-gray-100 dark:divide-dark-700 md:hidden">
            <article v-for="item in result.items" :key="item.api_key_id" class="space-y-3 p-4">
              <div class="flex items-start justify-between gap-3"><div class="min-w-0"><h3 class="break-words font-medium text-gray-900 dark:text-white">{{ item.name }}</h3><p class="mt-1 break-words text-xs text-gray-500">{{ t('usage.keySummary.currentGroup') }}: {{ item.group_name || '—' }} <span v-if="item.deleted">· {{ t('usage.errors.keyDeleted') }}</span></p></div><button class="shrink-0 text-sm text-primary-600 dark:text-primary-400" @click="showDetails(item)">{{ t('usage.keySummary.details') }}</button></div>
              <div class="grid grid-cols-3 gap-2 text-xs text-gray-500"><div>{{ t('usage.keySummary.requests') }}<p class="mt-1 break-all font-medium tabular-nums text-gray-900 dark:text-white">{{ item.requests.toLocaleString() }}</p></div><div>{{ t('usage.keySummary.tokens') }}<p class="mt-1 break-all font-medium tabular-nums text-gray-900 dark:text-white">{{ formatCompactNumber(item.total_tokens) }}</p></div><div class="text-right">{{ t('usage.keySummary.cost') }}<p class="mt-1 break-all font-medium tabular-nums text-gray-900 dark:text-white">${{ money(item.actual_cost) }}</p></div></div>
              <details class="text-xs text-gray-500"><summary class="cursor-pointer">{{ t('usage.tokenDetails') }}</summary><p class="mt-2 break-words leading-6">{{ tokenDetails(item) }}</p></details>
            </article>
          </div>
        </template>
        <Pagination v-if="result.total > 0" :page="page" :page-size="pageSize" :total="result.total" @update:page="changePage" @update:page-size="changePageSize" />
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { usageAPI, type APIKeySummaryParams, type APIKeyUsageSummary, type APIKeyUsageSummaryItem } from '@/api/usage'
import type { Group } from '@/types'
import Select from '@/components/common/Select.vue'
import Pagination from '@/components/common/Pagination.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { formatCompactNumber, formatDateLocalInput } from '@/utils/format'
import { keyUsageRange, type KeyUsagePeriod } from '@/utils/keyUsageRange'

const props = defineProps<{ groups: Group[]; initialKeyId?: number }>()
const emit = defineEmits<{ details: [params: { api_key_id: number; key_name: string; start_time: string; end_time: string; group_id?: number }] }>()
const { t } = useI18n()
const period = ref<KeyUsagePeriod>('7d')
const customStart = ref(formatDateLocalInput(new Date(Date.now() - 6 * 86400000)))
const customEnd = ref(formatDateLocalInput(new Date()))
const groupId = ref<number | null>(null)
const keyId = ref(props.initialKeyId)
const search = ref('')
const onlyUsed = ref(false)
const sortBy = ref<NonNullable<APIKeySummaryParams['sort_by']>>('actual_cost')
const page = ref(1)
const pageSize = ref(20)
const range = ref(keyUsageRange('7d'))
const result = ref<APIKeyUsageSummary | null>(null)
const error = ref('')
const loading = ref(false)
let controller: AbortController | undefined
const periodOptions = computed(() => [
  { value: '24h', label: t('dates.last24Hours') }, { value: '7d', label: t('dates.last7Days') },
  { value: '14d', label: t('dates.last14Days') }, { value: '30d', label: t('dates.last30Days') },
  { value: 'thisMonth', label: t('dates.thisMonth') }, { value: 'lastMonth', label: t('dates.lastMonth') },
  { value: 'custom', label: t('usage.keySummary.custom') },
])
const groupOptions = computed(() => [{ value: null, label: t('admin.usage.allGroups') }, ...props.groups.map(g => ({ value: g.id, label: g.name }))])
const sortOptions = computed(() => [
  { value: 'actual_cost', label: t('usage.keySummary.cost') },
  { value: 'total_tokens', label: t('usage.keySummary.tokens') },
  { value: 'requests', label: t('usage.keySummary.requests') },
])
const money = (v: number) => v.toFixed(4)
const rangeLabel = computed(() => `${new Date(range.value.start_time).toLocaleString()} — ${new Date(new Date(range.value.end_time).getTime() - 1).toLocaleString()}`)
const stats = computed(() => result.value ? [
  { label: t('usage.keySummary.keyCount'), value: result.value.total.toLocaleString() },
  { label: t('usage.keySummary.requests'), value: result.value.summary.requests.toLocaleString() },
  { label: t('usage.keySummary.tokens'), value: formatCompactNumber(result.value.summary.total_tokens) },
  { label: t('usage.keySummary.cost'), value: `$${money(result.value.summary.actual_cost)}` },
] : [])
function tokenDetails(item: APIKeyUsageSummaryItem) {
  return `${t('admin.usage.inputTokens')}: ${item.input_tokens.toLocaleString()} / ${t('admin.usage.outputTokens')}: ${item.output_tokens.toLocaleString()} / ${t('usage.keySummary.cacheRead')}: ${item.cache_read_tokens.toLocaleString()} / ${t('usage.keySummary.cacheWrite')}: ${item.cache_creation_tokens.toLocaleString()}`
}
async function load() {
  controller?.abort()
  const request = new AbortController()
  controller = request
  loading.value = true
  error.value = ''
  try {
    const data = await usageAPI.getAPIKeySummary({ ...range.value, api_key_id: keyId.value, group_id: groupId.value ?? undefined, search: search.value.trim(), only_used: onlyUsed.value, sort_by: sortBy.value, page: page.value, page_size: pageSize.value }, request.signal)
    if (!request.signal.aborted) result.value = data
  } catch {
    if (!request.signal.aborted) { result.value = null; error.value = t('usage.failedToLoad') }
  } finally {
    if (!request.signal.aborted) loading.value = false
  }
}
function apply() {
  try { range.value = keyUsageRange(period.value, new Date(), customStart.value, customEnd.value) }
  catch { controller?.abort(); loading.value = false; error.value = t('usage.keySummary.invalidRange'); return }
  page.value = 1
  void load()
}
function changePage(value: number) { page.value = value; void load() }
function changePageSize(value: number) { pageSize.value = value; page.value = 1; void load() }
function showDetails(item: APIKeyUsageSummaryItem) {
  emit('details', { api_key_id: item.api_key_id, key_name: item.name, ...range.value, group_id: groupId.value ?? undefined })
}
watch([period, groupId, keyId, onlyUsed, sortBy], apply)
watch(() => props.initialKeyId, value => { keyId.value = value })
onBeforeUnmount(() => controller?.abort())
void load()
</script>
