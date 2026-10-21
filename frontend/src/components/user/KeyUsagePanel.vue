<template>
  <div class="space-y-4 md:space-y-5" data-testid="key-usage-panel">
    <!-- Desktop / tablet filter bar -->
    <form class="hidden rounded-xl border border-ku-border bg-ku-card p-4 shadow-card md:block" data-testid="ku-filter-desktop" @submit.prevent="apply">
      <div class="flex flex-wrap items-center gap-3">
        <div class="inline-flex shrink-0 overflow-hidden rounded-lg border border-ku-input-border" role="group" :aria-label="t('admin.dashboard.timeRange')">
          <button v-for="option in desktopPeriods" :key="option" type="button" :data-period="option" :aria-pressed="period === option" class="h-9 px-3 text-sm transition-colors" :class="period === option ? 'bg-ku-primary text-white' : 'text-ku-text hover:bg-ku-hover'" @click="period = option">{{ t(`usage.keySummary.periodShort.${option}`) }}</button>
        </div>
        <div class="w-44 shrink-0">
          <label class="sr-only" for="key-usage-group">{{ t('admin.usage.group') }}</label>
          <Select id="key-usage-group" :aria-label="t('admin.usage.group')" v-model="groupId" :options="groupOptions" searchable />
        </div>
        <div class="relative min-w-0" :class="searchExpanded ? 'flex-1' : 'xl:flex-1'">
          <label class="sr-only" for="key-usage-search">{{ t('usage.keySummary.search') }}</label>
          <button v-if="!searchExpanded" type="button" class="flex h-9 w-9 items-center justify-center rounded-lg border border-ku-input-border text-ku-text-2 hover:bg-ku-hover xl:hidden" :aria-label="t('usage.keySummary.expandSearch')" data-testid="ku-search-expand" @click="expandSearch"><Icon name="search" size="sm" /></button>
          <div :class="searchExpanded ? 'block' : 'hidden xl:block'">
            <Icon name="search" size="sm" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-ku-text-3" />
            <input id="key-usage-search" ref="searchInput" v-model="search" maxlength="100" class="input h-9 pl-9" :placeholder="t('usage.keySummary.search')" @change="apply" @blur="collapseSearch" />
          </div>
        </div>
        <div class="w-36 shrink-0">
          <label class="sr-only" for="key-usage-sort">{{ t('usage.keySummary.sort') }}</label>
          <Select id="key-usage-sort" :aria-label="t('usage.keySummary.sort')" v-model="sortBy" :options="sortOptions" />
        </div>
        <div class="ml-auto flex shrink-0 items-center gap-3">
          <label class="flex items-center gap-2 text-sm text-ku-text"><Toggle v-model="onlyUsed" :aria-label="t('usage.keySummary.onlyUsed')" data-testid="ku-only-used" />{{ t('usage.keySummary.onlyUsed') }}</label>
          <button type="submit" class="flex h-8 w-8 items-center justify-center rounded-lg border border-ku-input-border text-ku-text-2 hover:bg-ku-hover disabled:opacity-60" :disabled="loading" :aria-label="t('usage.keySummary.refresh')" data-testid="ku-refresh"><Icon name="refresh" size="sm" :class="loading && 'animate-spin'" /></button>
        </div>
      </div>
      <div v-if="period === 'custom'" class="mt-3 flex flex-wrap items-end gap-3">
        <div><label class="input-label" for="key-usage-start">{{ t('dates.startDate') }}</label><input id="key-usage-start" v-model="customStart" class="input" type="date" :max="customEnd" /></div>
        <div><label class="input-label" for="key-usage-end">{{ t('dates.endDate') }}</label><input id="key-usage-end" v-model="customEnd" class="input" type="date" :min="customStart" /></div>
        <button type="submit" class="btn btn-primary">{{ t('dates.apply') }}</button>
      </div>
      <div class="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-ku-text-2">
        <span class="inline-flex items-center tabular-nums" data-testid="ku-range">{{ rangeLabel }}<HelpTooltip :content="t('usage.keySummary.hint')" width-class="w-72" /></span>
        <button v-if="keyId" type="button" class="text-ku-link hover:underline" @click="keyId = undefined">{{ t('usage.keySummary.showAll') }}</button>
      </div>
    </form>

    <!-- Mobile filter row -->
    <div class="space-y-2 md:hidden" data-testid="ku-filter-mobile">
      <div class="flex items-center gap-2">
        <div class="scrollbar-hide flex min-w-0 flex-1 gap-2 overflow-x-auto" role="group" :aria-label="t('admin.dashboard.timeRange')">
          <button v-for="option in desktopPeriods" :key="option" type="button" :data-period="option" :aria-pressed="period === option" class="relative h-8 shrink-0 rounded-2xl border px-3 text-sm before:absolute before:-inset-y-1.5 before:inset-x-0" :class="period === option ? 'border-ku-primary bg-ku-primary text-white' : 'border-ku-border bg-ku-card text-ku-text'" @click="period = option">{{ t(`usage.keySummary.periodShort.${option}`) }}</button>
        </div>
        <button type="button" class="relative flex h-8 shrink-0 items-center gap-1 rounded-lg border border-ku-border bg-ku-card px-3 text-sm text-ku-text before:absolute before:-inset-y-1.5 before:inset-x-0" data-testid="ku-filter-open" :aria-label="filterCount ? t('usage.keySummary.filterCount', { n: filterCount }) : t('usage.keySummary.filter')" @click="openFilterPanel"><Icon name="filter" size="sm" />{{ t('usage.keySummary.filter') }}<span v-if="filterCount" class="ml-0.5 inline-flex h-4 min-w-4 items-center justify-center rounded-full bg-ku-primary px-1 text-[10px] font-medium text-white" data-testid="ku-filter-badge">{{ filterCount }}</span></button>
      </div>
      <form v-if="period === 'custom'" class="flex flex-wrap items-end gap-2" @submit.prevent="apply">
        <div class="min-w-0 flex-1"><label class="input-label" for="key-usage-start-m">{{ t('dates.startDate') }}</label><input id="key-usage-start-m" v-model="customStart" class="input w-full" type="date" :max="customEnd" /></div>
        <div class="min-w-0 flex-1"><label class="input-label" for="key-usage-end-m">{{ t('dates.endDate') }}</label><input id="key-usage-end-m" v-model="customEnd" class="input w-full" type="date" :min="customStart" /></div>
        <button type="submit" class="btn btn-primary">{{ t('dates.apply') }}</button>
      </form>
      <div class="flex items-center justify-between gap-2 text-xs text-ku-text-2">
        <span class="inline-flex min-w-0 items-center tabular-nums">{{ rangeLabel }}<HelpTooltip :content="t('usage.keySummary.hint')" trigger="click" width-class="w-64" /></span>
        <button v-if="keyId" type="button" class="shrink-0 text-ku-link" @click="keyId = undefined">{{ t('usage.keySummary.showAll') }}</button>
        <button type="button" class="relative flex h-8 w-8 shrink-0 items-center justify-center rounded-lg text-ku-text-2 before:absolute before:-inset-1.5" :disabled="loading" :aria-label="t('usage.keySummary.refresh')" @click="apply"><Icon name="refresh" size="sm" :class="loading && 'animate-spin'" /></button>
      </div>
    </div>

    <div v-if="error" role="alert" class="flex items-center justify-between gap-3 rounded-xl border border-ku-border bg-ku-card p-5 text-red-600 dark:text-red-400">
      <span>{{ error }}</span><button class="btn btn-secondary" @click="load">{{ t('common.tryAgain') }}</button>
    </div>
    <div v-else-if="loading && !result" class="flex justify-center rounded-xl border border-ku-border bg-ku-card p-12" role="status"><LoadingSpinner /></div>
    <template v-else-if="result">
      <div class="grid grid-cols-2 gap-3 lg:grid-cols-4" :class="loading && 'opacity-60'">
        <KeyUsageStatCard v-for="stat in stats" :key="stat.key" :label="stat.label" :value="stat.value" :value-title="stat.title" :caption="stat.caption" :compare="stat.compare" :compare-label="compareLabel" :data-stat="stat.key">
          <template v-if="stat.trend" #trend><KeyUsageSparkline :values="stat.trend.values" :labels="stat.trend.labels" /></template>
        </KeyUsageStatCard>
      </div>
      <p v-if="result.summary.subscription_cost > 0" class="text-xs text-ku-text-3">{{ t('usage.keySummary.subscriptionHint') }} <span :title="keyUsageMoneyTitle(result.summary.subscription_cost)">{{ formatKeyUsageMoney(result.summary.subscription_cost) }}</span></p>
      <div v-if="result.items.length === 0" class="rounded-xl border border-ku-border bg-ku-card p-12 text-center text-sm text-ku-text-2">{{ t('usage.keySummary.empty') }}</div>
      <template v-else>
        <div class="hidden overflow-hidden rounded-xl border border-ku-border bg-ku-card shadow-card md:block" :class="loading && 'opacity-60'">
          <div class="overflow-x-auto">
            <table class="w-full text-sm">
              <thead class="bg-ku-bg text-left text-xs font-normal text-ku-text-2"><tr>
                <th class="px-5 py-3 font-normal">{{ t('usage.keySummary.name') }}</th><th class="px-4 py-3 font-normal">{{ t('usage.keySummary.group') }}</th>
                <th class="px-4 py-3 text-right font-normal">{{ t('usage.keySummary.requests') }}</th><th class="px-4 py-3 text-right font-normal">{{ t('usage.keySummary.token') }}</th>
                <th class="px-4 py-3 text-right font-normal">{{ t('usage.keySummary.cost') }}</th><th class="px-5 py-3 text-right font-normal">{{ t('usage.keySummary.details') }}</th>
              </tr></thead>
              <tbody class="divide-y divide-ku-border">
                <tr v-for="item in result.items" :key="item.api_key_id" class="text-ku-text transition-colors hover:bg-ku-hover" data-testid="ku-row">
                  <td class="max-w-48 px-4 py-3 lg:px-5 xl:max-w-64"><div class="flex min-w-0 items-center gap-2"><span class="truncate font-medium" :title="item.name">{{ item.name }}</span><KeyUsageDeletedTag v-if="item.deleted">{{ t('usage.errors.keyDeleted') }}</KeyUsageDeletedTag></div></td>
                  <td class="px-4 py-3"><KeyUsageGroupPill v-if="item.group_name" class="max-w-[160px]" :name="item.group_name" :platform="item.group_platform" :title="t('usage.keySummary.currentGroupTitle', { name: item.group_name })" /><span v-else class="text-ku-text-3">—</span></td>
                  <td class="px-4 py-3 text-right tabular-nums">{{ item.requests.toLocaleString() }}</td>
                  <td class="px-4 py-3 text-right"><KeyUsageTokenBreakdown :totals="item" :text="formatCompactNumber(item.total_tokens)" /></td>
                  <td class="w-32 px-4 py-3 text-right xl:w-40"><div class="font-medium tabular-nums" :title="keyUsageMoneyTitle(item.actual_cost)">{{ formatKeyUsageMoney(item.actual_cost) }}</div><div class="ml-auto mt-1.5 h-1 w-full max-w-[8rem] overflow-hidden rounded-full bg-ku-border" aria-hidden="true"><div class="h-full rounded-full bg-ku-primary" data-testid="ku-bar" :style="{ width: keyUsageShareWidth(item.actual_cost, result.summary.actual_cost) }" /></div></td>
                  <td class="px-5 py-3 text-right"><button type="button" class="inline-flex items-center gap-0.5 whitespace-nowrap text-ku-link hover:underline" data-testid="ku-details" @click="showDetails(item)">{{ t('usage.keySummary.details') }}<Icon name="chevronRight" size="xs" aria-hidden="true" /></button></td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
        <div class="flex flex-col gap-2 md:hidden" :class="loading && 'opacity-60'">
          <div v-for="item in result.items" :key="item.api_key_id" role="button" tabindex="0" class="flex min-h-[88px] w-full cursor-pointer items-center gap-2 rounded-xl border border-ku-border bg-ku-card p-3 text-left focus:outline-none focus-visible:ring-2 focus-visible:ring-ku-primary/40" data-testid="ku-card" @click="showDetails(item)" @keydown.enter.prevent="showDetails(item)" @keydown.space.prevent="showDetails(item)">
            <div class="min-w-0 flex-1 space-y-1.5">
              <div class="flex items-center justify-between gap-2"><span class="min-w-0 truncate text-[15px] font-semibold text-ku-text" :title="item.name">{{ item.name }}</span><span class="shrink-0 text-[15px] font-semibold tabular-nums text-ku-text" :title="keyUsageMoneyTitle(item.actual_cost)">{{ formatKeyUsageMoney(item.actual_cost) }}</span></div>
              <div class="flex min-w-0 items-center gap-2"><KeyUsageGroupPill v-if="item.group_name" :name="item.group_name" :platform="item.group_platform" :title="t('usage.keySummary.currentGroupTitle', { name: item.group_name })" /><KeyUsageDeletedTag v-if="item.deleted">{{ t('usage.errors.keyDeleted') }}</KeyUsageDeletedTag></div>
              <div class="flex flex-wrap items-center justify-between gap-x-2 text-xs text-ku-text-2"><span class="tabular-nums">{{ t('usage.keySummary.requestsUnit', { n: item.requests.toLocaleString() }) }} · <KeyUsageTokenBreakdown :totals="item" :text="t('usage.keySummary.tokenUnit', { n: formatCompactNumber(item.total_tokens) })" /></span><span class="shrink-0 tabular-nums" data-testid="ku-share">{{ formatKeyUsageShare(item.actual_cost, result.summary.actual_cost) }}</span></div>
              <div class="h-[3px] overflow-hidden rounded-full bg-ku-border" aria-hidden="true"><div class="h-full rounded-full bg-ku-primary" :style="{ width: keyUsageShareWidth(item.actual_cost, result.summary.actual_cost) }" /></div>
            </div>
            <Icon name="chevronRight" size="sm" class="shrink-0 text-ku-text-3" aria-hidden="true" />
          </div>
        </div>
      </template>
      <Pagination v-if="result.total > 0" :page="page" :page-size="pageSize" :total="result.total" @update:page="changePage" @update:page-size="changePageSize" />
    </template>

    <!-- Mobile filter sheet: edits a draft and only takes effect on Apply -->
    <BaseDialog :show="filterOpen" :title="t('usage.keySummary.filter')" width="normal" mobile-sheet close-on-click-outside @close="filterOpen = false">
      <div class="space-y-4" data-testid="ku-filter-sheet">
        <div><p class="input-label">{{ t('usage.keySummary.moreTime') }}</p><div class="flex flex-wrap gap-2"><button v-for="option in sheetPeriods" :key="option" type="button" :data-period="option" :aria-pressed="draft.period === option" class="h-8 rounded-2xl border px-3 text-sm" :class="draft.period === option ? 'border-ku-primary bg-ku-primary text-white' : 'border-ku-border text-ku-text'" @click="draft.period = draft.period === option ? defaultPeriod : option">{{ t(`usage.keySummary.periodShort.${option}`) }}</button></div></div>
        <div><label class="input-label" for="key-usage-group-m">{{ t('admin.usage.group') }}</label><Select id="key-usage-group-m" :aria-label="t('admin.usage.group')" v-model="draft.groupId" :options="groupOptions" searchable /></div>
        <div><label class="input-label" for="key-usage-search-m">{{ t('usage.keySummary.search') }}</label><input id="key-usage-search-m" v-model="draft.search" maxlength="100" class="input" :placeholder="t('usage.keySummary.search')" /></div>
        <div><label class="input-label" for="key-usage-sort-m">{{ t('usage.keySummary.sort') }}</label><Select id="key-usage-sort-m" :aria-label="t('usage.keySummary.sort')" v-model="draft.sortBy" :options="sortOptions" /></div>
        <label class="flex items-center justify-between gap-3 text-sm text-ku-text">{{ t('usage.keySummary.onlyUsed') }}<Toggle v-model="draft.onlyUsed" :aria-label="t('usage.keySummary.onlyUsed')" /></label>
      </div>
      <template #footer>
        <div class="grid w-full grid-cols-2 gap-3 pb-[env(safe-area-inset-bottom)]"><button type="button" class="btn btn-secondary h-11" data-testid="ku-filter-reset" @click="resetDraft">{{ t('usage.keySummary.reset') }}</button><button type="button" class="btn btn-primary h-11" data-testid="ku-filter-apply" @click="applyDraft">{{ t('usage.keySummary.apply') }}</button></div>
      </template>
    </BaseDialog>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { usageAPI, type APIKeySummaryParams, type APIKeyUsageSummary, type APIKeyUsageSummaryItem, type APIKeyUsageDetailSelection, type APIKeyUsageBucket } from '@/api/usage'
import type { Group } from '@/types'
import Select from '@/components/common/Select.vue'
import Toggle from '@/components/common/Toggle.vue'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Pagination from '@/components/common/Pagination.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'
import KeyUsageStatCard from './keyUsage/KeyUsageStatCard.vue'
import KeyUsageSparkline from './keyUsage/KeyUsageSparkline.vue'
import KeyUsageGroupPill from './keyUsage/KeyUsageGroupPill.vue'
import KeyUsageDeletedTag from './keyUsage/KeyUsageDeletedTag.vue'
import KeyUsageTokenBreakdown from './keyUsage/KeyUsageTokenBreakdown.vue'
import { formatCompactNumber, formatDateLocalInput } from '@/utils/format'
import { formatKeyUsageMoney, formatKeyUsageShare, keyUsageMoneyTitle, keyUsageShareWidth } from '@/utils/keyUsageMoney'
import { formatKeyUsageRange, keyUsageCompareLabel, keyUsageGranularity, keyUsageRange, keyUsageTimezone, previousKeyUsageRange, type KeyUsagePeriod } from '@/utils/keyUsageRange'

type SortBy = NonNullable<APIKeySummaryParams['sort_by']>
const props = defineProps<{ groups: Group[]; initialKeyId?: number }>()
const emit = defineEmits<{ details: [params: APIKeyUsageDetailSelection] }>()
const { t } = useI18n()
const defaultPeriod: KeyUsagePeriod = '7d'
const defaultSort: SortBy = 'actual_cost'
const desktopPeriods = ['24h', '7d', '30d', 'custom'] as const
const sheetPeriods = ['14d', 'thisMonth', 'lastMonth'] as const
const period = ref<KeyUsagePeriod>(defaultPeriod)
const customStart = ref(formatDateLocalInput(new Date(Date.now() - 6 * 86400000)))
const customEnd = ref(formatDateLocalInput(new Date()))
const groupId = ref<number | null>(null)
const keyId = ref(props.initialKeyId)
const search = ref('')
const onlyUsed = ref(false)
const sortBy = ref<SortBy>(defaultSort)
const page = ref(1)
const pageSize = ref(20)
const range = ref(keyUsageRange(defaultPeriod))
const appliedPeriod = ref<KeyUsagePeriod>(defaultPeriod)
const result = ref<APIKeyUsageSummary | null>(null)
const error = ref('')
const loading = ref(false)
const searchExpanded = ref(false)
const searchInput = ref<HTMLInputElement | null>(null)
const filterOpen = ref(false)
const draft = reactive({ period: defaultPeriod as KeyUsagePeriod, groupId: null as number | null, search: '', sortBy: defaultSort, onlyUsed: false })
let controller: AbortController | undefined

const groupOptions = computed(() => [{ value: null, label: t('admin.usage.allGroups') }, ...props.groups.map(g => ({ value: g.id, label: g.name }))])
const sortOptions = computed(() => [
  { value: 'actual_cost', label: t('usage.keySummary.cost') },
  { value: 'total_tokens', label: t('usage.keySummary.tokens') },
  { value: 'requests', label: t('usage.keySummary.requests') },
])
const rangeLabel = computed(() => formatKeyUsageRange(range.value))
const compareLabel = computed(() => t(`usage.keySummary.${keyUsageCompareLabel(appliedPeriod.value)}`))
// Only conditions that live in the mobile sheet count; the top time chips are visible already.
const filterCount = computed(() => [
  (sheetPeriods as readonly string[]).includes(period.value), groupId.value !== null, search.value.trim() !== '', sortBy.value !== defaultSort, onlyUsed.value,
].filter(Boolean).length)

const pad = (n: number) => String(n).padStart(2, '0')
function bucketLabel(b: APIKeyUsageBucket, granularity: 'hour' | 'day') {
  const d = new Date(b.bucket)
  const day = `${pad(d.getMonth() + 1)}/${pad(d.getDate())}`
  return granularity === 'hour' ? `${day} ${pad(d.getHours())}:${pad(d.getMinutes())}` : day
}
function trendOf(pick: (b: APIKeyUsageBucket) => number, format: (v: number) => string) {
  const trend = result.value?.trend
  if (!trend?.length) return undefined
  const granularity = keyUsageGranularity(range.value)
  return { values: trend.map(pick), labels: trend.map(b => `${bucketLabel(b, granularity)} · ${format(pick(b))}`) }
}
const stats = computed(() => {
  const r = result.value
  if (!r) return []
  const prev = r.previous
  return [
    { key: 'keys', label: t('usage.keySummary.keyCount'), value: r.total.toLocaleString(), caption: onlyUsed.value ? t('usage.keySummary.activeKeys') : undefined },
    { key: 'requests', label: t('usage.keySummary.requests'), value: r.summary.requests.toLocaleString(), compare: { current: r.summary.requests, previous: prev?.requests ?? null }, trend: trendOf(b => b.requests, v => v.toLocaleString()) },
    { key: 'tokens', label: t('usage.keySummary.tokens'), value: formatCompactNumber(r.summary.total_tokens), compare: { current: r.summary.total_tokens, previous: prev?.total_tokens ?? null }, trend: trendOf(b => b.total_tokens, formatCompactNumber) },
    { key: 'cost', label: t('usage.keySummary.cost'), value: formatKeyUsageMoney(r.summary.actual_cost), title: keyUsageMoneyTitle(r.summary.actual_cost), compare: { current: r.summary.actual_cost, previous: prev?.actual_cost ?? null }, trend: trendOf(b => b.actual_cost, formatKeyUsageMoney) },
  ]
})

async function load() {
  controller?.abort()
  const request = new AbortController()
  controller = request
  loading.value = true
  error.value = ''
  try {
    const data = await usageAPI.getAPIKeySummary({
      ...range.value, ...previousKeyUsageRange(appliedPeriod.value, range.value),
      granularity: keyUsageGranularity(range.value), timezone: keyUsageTimezone(),
      api_key_id: keyId.value, group_id: groupId.value ?? undefined, search: search.value.trim(), only_used: onlyUsed.value,
      sort_by: sortBy.value, page: page.value, page_size: pageSize.value,
    }, request.signal)
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
  appliedPeriod.value = period.value
  page.value = 1
  void load()
}
function changePage(value: number) { page.value = value; void load() }
function changePageSize(value: number) { pageSize.value = value; page.value = 1; void load() }
function showDetails(item: APIKeyUsageSummaryItem) {
  emit('details', {
    api_key_id: item.api_key_id, key_name: item.name, key_deleted: item.deleted,
    ...range.value, period: appliedPeriod.value, group_id: groupId.value ?? undefined,
    group_name: props.groups.find(group => group.id === groupId.value)?.name,
  })
}
async function expandSearch() {
  searchExpanded.value = true
  await nextTick()
  searchInput.value?.focus()
}
function collapseSearch() { if (!search.value) searchExpanded.value = false }
function openFilterPanel() {
  Object.assign(draft, { period: period.value, groupId: groupId.value, search: search.value, sortBy: sortBy.value, onlyUsed: onlyUsed.value })
  filterOpen.value = true
}
function resetDraft() { Object.assign(draft, { period: defaultPeriod, groupId: null, search: '', sortBy: defaultSort, onlyUsed: false }) }
function applyDraft() {
  // Watched filters reload through the watcher; a search-only change needs an explicit apply.
  const watchedChanged = draft.period !== period.value || draft.groupId !== groupId.value || draft.sortBy !== sortBy.value || draft.onlyUsed !== onlyUsed.value
  Object.assign(draft, { search: draft.search.trim() })
  period.value = draft.period
  groupId.value = draft.groupId
  sortBy.value = draft.sortBy
  onlyUsed.value = draft.onlyUsed
  const searchChanged = draft.search !== search.value
  search.value = draft.search
  filterOpen.value = false
  if (!watchedChanged && searchChanged) apply()
}
watch([period, groupId, keyId, onlyUsed, sortBy], apply)
watch(() => props.initialKeyId, value => { keyId.value = value })
onBeforeUnmount(() => controller?.abort())
void load()
</script>
