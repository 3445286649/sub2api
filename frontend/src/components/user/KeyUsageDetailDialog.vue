<template>
  <BaseDialog :show="!closed" :title="title" width="detail" mobile-sheet close-on-click-outside @close="close">
    <template #header="{ titleId }">
      <div class="modal-header gap-3">
        <div class="flex min-w-0 items-center gap-2">
          <h3 :id="titleId" class="min-w-0 truncate text-lg font-semibold text-ku-text" :title="title">{{ title }}</h3>
          <KeyUsageDeletedTag v-if="detail?.deleted || selection.key_deleted" data-testid="key-detail-deleted">{{ t('usage.errors.keyDeleted') }}</KeyUsageDeletedTag>
        </div>
        <button type="button" class="shrink-0 rounded-lg p-2 text-ku-text-2 hover:bg-ku-hover focus-visible:outline focus-visible:outline-2 focus-visible:outline-ku-primary" :aria-label="t('common.close')" @click="close"><Icon name="x" size="md" /></button>
      </div>
    </template>

    <div class="space-y-4 md:space-y-5" data-testid="key-usage-detail-dialog">
        <div class="sticky -top-3 z-10 -mx-4 bg-ku-sheet px-4 py-2 sm:-mx-6 sm:px-6 md:static md:mx-0 md:bg-transparent md:p-0">
          <div class="flex flex-col gap-2 md:flex-row md:flex-wrap md:items-center">
            <div class="scrollbar-hide flex gap-2 overflow-x-auto md:inline-flex md:gap-0 md:overflow-hidden md:rounded-lg md:border md:border-ku-input-border" role="group" :aria-label="t('admin.dashboard.timeRange')">
              <button v-for="option in periodOptions" :key="option" type="button" :data-period="option" :aria-pressed="period === option" class="h-8 shrink-0 rounded-2xl border px-3 text-sm transition-colors md:rounded-none md:border-0" :class="period === option ? 'border-ku-primary bg-ku-primary text-white' : 'border-ku-border text-ku-text hover:bg-ku-hover'" @click="selectPeriod(option)">{{ t(`usage.keySummary.periodShort.${option}`) }}</button>
            </div>
            <p class="hidden text-xs tabular-nums text-ku-text-2 md:ml-auto md:block" data-testid="key-detail-range">{{ rangeLabel }}</p>
          </div>
        </div>
      <div class="space-y-2">
        <p class="text-xs tabular-nums text-ku-text-2 md:hidden">{{ rangeLabel }}</p>
        <form v-if="period === 'custom'" class="flex flex-wrap items-end gap-3" @submit.prevent="applyCustom">
          <div class="min-w-0 flex-1 sm:flex-none"><label class="input-label" for="key-detail-start">{{ t('dates.startDate') }}</label><input id="key-detail-start" v-model="customStart" class="input w-full" type="date" :max="customEnd" /></div>
          <div class="min-w-0 flex-1 sm:flex-none"><label class="input-label" for="key-detail-end">{{ t('dates.endDate') }}</label><input id="key-detail-end" v-model="customEnd" class="input w-full" type="date" :min="customStart" /></div>
          <button type="submit" class="btn btn-primary" data-testid="key-detail-apply">{{ t('dates.apply') }}</button>
        </form>
        <p v-if="rangeError" role="alert" class="text-sm text-red-600 dark:text-red-400">{{ rangeError }}</p>
        <div v-if="groupId !== undefined" class="flex flex-wrap items-center gap-2 text-xs">
          <span class="text-ku-text-2">{{ t('usage.keyDetail.filteredGroup') }}</span>
          <KeyUsageGroupPill :name="selection.group_name || t('usage.keyDetail.groupFallback', { id: groupId })" :platform="filteredGroupPlatform" />
          <button type="button" data-testid="key-detail-all-groups" class="text-ku-link hover:underline" @click="clearGroup">{{ t('usage.keyDetail.allUsedGroups') }}</button>
        </div>
      </div>

      <div v-if="loading" role="status" class="flex justify-center py-12"><LoadingSpinner /></div>
      <div v-else-if="loadError" role="alert" class="flex flex-wrap items-center justify-between gap-3 rounded-xl bg-red-50 p-4 text-sm text-red-600 dark:bg-red-950/20 dark:text-red-400">
        <span>{{ t('usage.failedToLoad') }}</span><button type="button" class="btn btn-secondary" data-testid="key-detail-retry" @click="load">{{ t('common.tryAgain') }}</button>
      </div>
      <template v-else-if="detail">
        <div class="grid grid-cols-3 gap-2 md:gap-3">
          <KeyUsageStatCard v-for="stat in stats" :key="stat.key" compact class="!bg-ku-bg !shadow-none" :label="stat.label" :value="stat.value" :value-title="stat.title" :compare="stat.compare" :compare-label="compareLabel" />
        </div>
        <p v-if="detail.summary.subscription_cost > 0" class="text-xs leading-5 text-ku-text-3">{{ t('usage.keySummary.subscriptionHint') }} <span :title="keyUsageMoneyTitle(detail.summary.subscription_cost)">{{ formatKeyUsageMoney(detail.summary.subscription_cost) }}</span></p>
        <KeyUsageSpendArea v-if="detail.series?.length" :series="detail.series" :granularity="granularity" :title="granularity === 'hour' ? t('usage.keyDetail.hourlySpend') : t('usage.keyDetail.dailySpend')" />
        <div v-if="detail.groups.length === 0" class="rounded-xl border border-dashed border-ku-border px-4 py-12 text-center text-sm text-ku-text-2">{{ t('usage.keyDetail.empty') }}</div>
        <article v-for="group in detail.groups" :key="groupKey(group)" class="overflow-hidden border-y border-ku-border md:rounded-xl md:border" data-testid="key-detail-group">
          <button type="button" class="flex min-h-11 w-full items-center gap-2 bg-ku-bg px-4 py-2.5 text-left hover:bg-ku-hover" :aria-expanded="isExpanded(group)" :aria-label="`${groupName(group)} ${t('usage.keyDetail.toggleGroup')}`" data-testid="key-detail-group-toggle" @click="toggleGroup(group)">
            <Icon name="chevronRight" size="sm" class="shrink-0 text-ku-text-3 transition-transform" :class="isExpanded(group) && 'rotate-90'" aria-hidden="true" />
            <div class="flex min-w-0 flex-1 flex-col gap-1 md:flex-row md:items-center md:justify-between md:gap-3">
              <div class="flex min-w-0 items-center gap-2"><KeyUsageGroupPill :name="groupName(group)" :platform="group.group_platform" /><KeyUsageDeletedTag v-if="group.group_deleted">{{ t('usage.keyDetail.deletedGroup') }}</KeyUsageDeletedTag></div>
              <span class="shrink-0 text-xs tabular-nums text-ku-text-2 md:text-sm" data-testid="key-detail-group-summary">{{ t('usage.keySummary.requestsUnit', { n: group.requests.toLocaleString() }) }}<span class="hidden md:inline"> · {{ t('usage.keySummary.tokenUnit', { n: formatCompactNumber(group.total_tokens) }) }}</span> · <span :title="keyUsageMoneyTitle(group.actual_cost)">{{ formatKeyUsageMoney(group.actual_cost) }}</span></span>
            </div>
          </button>
          <div v-if="isExpanded(group)">
            <div class="hidden grid-cols-[minmax(0,1fr)_6rem_8rem_7rem_9rem] gap-4 border-y border-ku-border px-4 py-2 text-xs text-ku-text-2 md:grid"><span>{{ t('usage.model') }}</span><span class="text-right">{{ t('usage.keySummary.requests') }}</span><span class="text-right">{{ t('usage.keySummary.token') }}</span><span class="text-right">{{ t('usage.keySummary.cost') }}</span><span class="text-right">{{ t('usage.keyDetail.share') }}</span></div>
            <div class="divide-y divide-ku-border">
              <div v-for="model in group.models" :key="model.model" class="px-4 py-3 text-sm text-ku-text md:grid md:grid-cols-[minmax(0,1fr)_6rem_8rem_7rem_9rem] md:items-center md:gap-4" data-testid="key-detail-model">
                <div class="flex items-center justify-between gap-2 md:contents">
                  <p class="min-w-0 truncate font-medium" :title="model.model">{{ model.model || t('usage.keyDetail.unrecordedModel') }}</p>
                  <span class="hidden text-right tabular-nums md:block">{{ model.requests.toLocaleString() }}</span>
                  <span class="hidden text-right md:block"><KeyUsageTokenBreakdown :totals="model" :text="formatCompactNumber(model.total_tokens)" /></span>
                  <span class="shrink-0 text-right font-medium tabular-nums" :title="keyUsageMoneyTitle(model.actual_cost)">{{ formatKeyUsageMoney(model.actual_cost) }}</span>
                  <span class="hidden items-center justify-end gap-2 md:flex"><span class="h-1.5 w-[60px] overflow-hidden rounded-full bg-ku-border" aria-hidden="true"><span class="block h-full rounded-full bg-ku-primary" :style="{ width: keyUsageShareWidth(model.actual_cost, group.actual_cost) }" /></span><span class="w-9 text-right text-xs tabular-nums text-ku-text-2" data-testid="key-detail-share">{{ formatKeyUsageShare(model.actual_cost, group.actual_cost) }}</span></span>
                </div>
                <div class="mt-1 flex flex-wrap items-center gap-x-2 text-xs text-ku-text-2 md:hidden">
                  <span class="tabular-nums">{{ t('usage.keySummary.requestsUnit', { n: model.requests.toLocaleString() }) }} · <KeyUsageTokenBreakdown :totals="model" :text="t('usage.keySummary.tokenUnit', { n: formatCompactNumber(model.total_tokens) })" /></span>
                  <span class="h-[3px] min-w-8 flex-1 overflow-hidden rounded-full bg-ku-border" aria-hidden="true"><span class="block h-full rounded-full bg-ku-primary" :style="{ width: keyUsageShareWidth(model.actual_cost, group.actual_cost) }" /></span>
                  <span class="shrink-0 tabular-nums">{{ formatKeyUsageShare(model.actual_cost, group.actual_cost) }}</span>
                </div>
              </div>
            </div>
          </div>
        </article>
      </template>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { usageAPI, type APIKeyUsageBreakdown, type APIKeyUsageBreakdownGroup, type APIKeyUsageDetailSelection } from '@/api/usage'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import KeyUsageStatCard from './keyUsage/KeyUsageStatCard.vue'
import KeyUsageGroupPill from './keyUsage/KeyUsageGroupPill.vue'
import KeyUsageDeletedTag from './keyUsage/KeyUsageDeletedTag.vue'
import KeyUsageTokenBreakdown from './keyUsage/KeyUsageTokenBreakdown.vue'
import KeyUsageSpendArea from './keyUsage/KeyUsageSpendArea.vue'
import { formatCompactNumber } from '@/utils/format'
import { formatKeyUsageMoney, formatKeyUsageShare, keyUsageMoneyTitle, keyUsageShareWidth } from '@/utils/keyUsageMoney'
import { formatKeyUsageRange, keyUsageCompareLabel, keyUsageDisplayDates, keyUsageGranularity, keyUsageRange, keyUsageTimezone, previousKeyUsageRange, type KeyUsagePeriod } from '@/utils/keyUsageRange'

const props = defineProps<{ selection: APIKeyUsageDetailSelection }>()
const emit = defineEmits<{ close: [] }>()
const { t } = useI18n()
const periodOptions: KeyUsagePeriod[] = ['24h', '7d', '14d', '30d', 'thisMonth', 'lastMonth', 'custom']
const period = ref<KeyUsagePeriod>(props.selection.period)
const appliedPeriod = ref<KeyUsagePeriod>(props.selection.period)
const range = ref({ start_time: props.selection.start_time, end_time: props.selection.end_time })
const groupId = ref<number>()
const customStart = ref('')
const customEnd = ref('')
const rangeError = ref('')
const loadError = ref(false)
const loading = ref(false)
const detail = ref<APIKeyUsageBreakdown | null>(null)
const expanded = ref(new Set<string>())
let controller: AbortController | undefined
const closed = ref(false)

const title = computed(() => `${t('usage.keyDetail.title')} · ${detail.value?.name || props.selection.key_name}`)
const rangeLabel = computed(() => formatKeyUsageRange(range.value))
const granularity = computed(() => keyUsageGranularity(range.value))
const compareLabel = computed(() => t(`usage.keySummary.${keyUsageCompareLabel(appliedPeriod.value)}`))
const filteredGroupPlatform = computed(() => detail.value?.groups.find(g => g.group_id === groupId.value)?.group_platform)
const stats = computed(() => {
  const d = detail.value
  if (!d) return []
  const prev = d.previous_summary
  return [
    { key: 'requests', label: t('usage.keySummary.requests'), value: d.summary.requests.toLocaleString(), compare: { current: d.summary.requests, previous: prev?.requests ?? null } },
    { key: 'tokens', label: t('usage.keySummary.tokens'), value: formatCompactNumber(d.summary.total_tokens), compare: { current: d.summary.total_tokens, previous: prev?.total_tokens ?? null } },
    { key: 'cost', label: t('usage.keySummary.cost'), value: formatKeyUsageMoney(d.summary.actual_cost), title: keyUsageMoneyTitle(d.summary.actual_cost), compare: { current: d.summary.actual_cost, previous: prev?.actual_cost ?? null } },
  ]
})
const groupKey = (group: APIKeyUsageBreakdownGroup) => String(group.group_id ?? 'unrecorded')
function groupName(group: APIKeyUsageBreakdownGroup) {
  if (group.group_id === null) return t('usage.keyDetail.unrecordedGroup')
  return group.group_name || t('usage.keyDetail.groupFallback', { id: group.group_id })
}
const isExpanded = (group: APIKeyUsageBreakdownGroup) => expanded.value.has(groupKey(group))
function toggleGroup(group: APIKeyUsageBreakdownGroup) {
  const next = new Set(expanded.value)
  const key = groupKey(group)
  if (next.has(key)) next.delete(key)
  else next.add(key)
  expanded.value = next
}
// Default: a single group is open; otherwise only the top-spending non-deleted group.
function defaultExpanded(groups: APIKeyUsageBreakdownGroup[]) {
  if (groups.length === 1) return new Set([groupKey(groups[0])])
  const top = groups.filter(g => !g.group_deleted).reduce<APIKeyUsageBreakdownGroup | null>((best, g) => !best || g.actual_cost > best.actual_cost ? g : best, null)
  return new Set(top ? [groupKey(top)] : [])
}
async function load() {
  controller?.abort()
  if (closed.value) return
  const request = new AbortController()
  controller = request
  loading.value = true
  loadError.value = false
  detail.value = null
  try {
    const data = await usageAPI.getAPIKeyBreakdown(props.selection.api_key_id, {
      ...range.value, group_id: groupId.value, ...previousKeyUsageRange(appliedPeriod.value, range.value),
      granularity: granularity.value, timezone: keyUsageTimezone(),
    }, request.signal)
    if (!request.signal.aborted) { detail.value = data; expanded.value = defaultExpanded(data.groups) }
  } catch {
    if (!request.signal.aborted) loadError.value = true
  } finally {
    if (!request.signal.aborted) loading.value = false
  }
}
function selectPeriod(value: KeyUsagePeriod) {
  period.value = value
  rangeError.value = ''
  if (value === 'custom') return
  range.value = keyUsageRange(value)
  appliedPeriod.value = value
  const dates = keyUsageDisplayDates(range.value)
  customStart.value = dates.startDate
  customEnd.value = dates.endDate
  void load()
}
function applyCustom() {
  try { range.value = keyUsageRange('custom', new Date(), customStart.value, customEnd.value) }
  catch { rangeError.value = t('usage.keySummary.invalidRange'); return }
  rangeError.value = ''
  appliedPeriod.value = 'custom'
  void load()
}
function clearGroup() { groupId.value = undefined; void load() }
async function close() {
  if (closed.value) return
  closed.value = true
  controller?.abort()
  // Let BaseDialog release its scroll lock and restore focus before unmounting.
  await nextTick()
  emit('close')
}
watch(() => props.selection, selection => {
  closed.value = false
  period.value = selection.period
  appliedPeriod.value = selection.period
  range.value = { start_time: selection.start_time, end_time: selection.end_time }
  groupId.value = selection.group_id
  const dates = keyUsageDisplayDates(range.value)
  customStart.value = dates.startDate
  customEnd.value = dates.endDate
  rangeError.value = ''
  void load()
}, { immediate: true })
onBeforeUnmount(() => { closed.value = true; controller?.abort() })
</script>
