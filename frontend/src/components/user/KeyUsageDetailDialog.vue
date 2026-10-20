<template>
  <BaseDialog :show="!closed" :title="title" width="extra-wide" @close="close">
    <template #header="{ titleId }">
      <div class="modal-header gap-3">
        <h3 :id="titleId" class="modal-title min-w-0 break-words">{{ title }}</h3>
        <button type="button" class="shrink-0 rounded-lg p-2 text-gray-500 hover:bg-gray-100 focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary-500 dark:hover:bg-dark-700" :aria-label="t('common.close')" @click="close"><Icon name="x" size="md" /></button>
      </div>
    </template>

    <div class="space-y-5" data-testid="key-usage-detail-dialog">
      <div class="space-y-3">
        <span v-if="detail?.deleted || selection.key_deleted" class="text-xs text-gray-500">{{ t('usage.errors.keyDeleted') }}</span>
        <div class="flex flex-wrap gap-2" role="group" :aria-label="t('admin.dashboard.timeRange')">
          <button v-for="option in periodOptions" :key="option.value" type="button" :data-period="option.value" :aria-pressed="period === option.value" class="btn px-3 py-2 text-xs sm:text-sm" :class="period === option.value ? 'btn-primary' : 'btn-secondary'" @click="selectPeriod(option.value)">{{ option.label }}</button>
        </div>
        <form v-if="period === 'custom'" class="flex flex-wrap items-end gap-3" @submit.prevent="applyCustom">
          <div class="min-w-0 flex-1 sm:flex-none"><label class="input-label" for="key-detail-start">{{ t('dates.startDate') }}</label><input id="key-detail-start" v-model="customStart" class="input w-full" type="date" :max="customEnd" /></div>
          <div class="min-w-0 flex-1 sm:flex-none"><label class="input-label" for="key-detail-end">{{ t('dates.endDate') }}</label><input id="key-detail-end" v-model="customEnd" class="input w-full" type="date" :min="customStart" /></div>
          <button type="submit" class="btn btn-primary" data-testid="key-detail-apply">{{ t('dates.apply') }}</button>
        </form>
        <p v-if="rangeError" role="alert" class="text-sm text-red-600 dark:text-red-400">{{ rangeError }}</p>
        <p class="text-xs leading-5 text-gray-500 dark:text-gray-400">{{ t('usage.keyDetail.range') }}: {{ rangeLabel }}</p>
        <div v-if="groupId !== undefined" class="flex flex-wrap items-center gap-2 text-xs">
          <span class="rounded-md bg-gray-100 px-2 py-1 text-gray-700 dark:bg-dark-700 dark:text-gray-200">{{ t('usage.keyDetail.filteredGroup') }}: {{ selection.group_name || t('usage.keyDetail.groupFallback', { id: groupId }) }}</span>
          <button type="button" data-testid="key-detail-all-groups" class="text-primary-600 hover:underline dark:text-primary-400" @click="clearGroup">{{ t('usage.keyDetail.allUsedGroups') }}</button>
        </div>
      </div>

      <div v-if="loading" role="status" class="flex justify-center py-12"><LoadingSpinner /></div>
      <div v-else-if="loadError" role="alert" class="flex flex-wrap items-center justify-between gap-3 rounded-xl bg-red-50 p-4 text-sm text-red-600 dark:bg-red-950/20 dark:text-red-400">
        <span>{{ t('usage.failedToLoad') }}</span><button type="button" class="btn btn-secondary" data-testid="key-detail-retry" @click="load">{{ t('common.tryAgain') }}</button>
      </div>
      <template v-else-if="detail">
        <div class="grid grid-cols-2 gap-2 sm:grid-cols-3 sm:gap-3">
          <div v-for="stat in stats" :key="stat.label" class="min-w-0 rounded-xl border border-gray-200 bg-gray-50 p-3 last:col-span-2 dark:border-dark-700 dark:bg-dark-900/50 sm:p-4 sm:last:col-span-1">
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ stat.label }}</p>
            <p class="mt-2 break-all text-lg font-semibold tabular-nums text-gray-900 dark:text-white sm:text-xl">{{ stat.value }}</p>
          </div>
        </div>
        <p v-if="detail.summary.subscription_cost > 0" class="text-xs leading-5 text-gray-500 dark:text-gray-400">{{ t('usage.keySummary.subscriptionHint') }} ${{ money(detail.summary.subscription_cost) }}</p>
        <div v-if="detail.groups.length === 0" class="rounded-xl border border-dashed border-gray-200 px-4 py-12 text-center text-sm text-gray-500 dark:border-dark-700 dark:text-gray-400">{{ t('usage.keyDetail.empty') }}</div>
        <article v-for="group in detail.groups" :key="group.group_id ?? 'unrecorded'" class="overflow-hidden rounded-xl border border-gray-200 dark:border-dark-700" data-testid="key-detail-group">
          <div class="space-y-3 border-b border-gray-200 bg-gray-50 px-4 py-3 dark:border-dark-700 dark:bg-dark-900/50">
            <div class="flex flex-wrap items-center gap-2"><h4 class="min-w-0 break-words font-medium text-gray-900 dark:text-white">{{ groupName(group) }}</h4><span v-if="group.group_deleted" class="rounded bg-gray-200 px-1.5 py-0.5 text-xs text-gray-600 dark:bg-dark-700 dark:text-gray-300">{{ t('usage.keyDetail.deletedGroup') }}</span></div>
            <dl class="grid grid-cols-3 gap-3 text-xs text-gray-500 dark:text-gray-400"><div><dt>{{ t('usage.keySummary.requests') }}</dt><dd class="mt-1 break-all font-medium tabular-nums text-gray-800 dark:text-gray-200">{{ group.requests.toLocaleString() }}</dd></div><div><dt>{{ t('usage.keySummary.tokens') }}</dt><dd class="mt-1 break-all font-medium tabular-nums text-gray-800 dark:text-gray-200">{{ formatCompactNumber(group.total_tokens) }}</dd></div><div class="text-right"><dt>{{ t('usage.keySummary.cost') }}</dt><dd class="mt-1 break-all font-medium tabular-nums text-gray-800 dark:text-gray-200">${{ money(group.actual_cost) }}</dd></div></dl>
          </div>
          <div class="hidden grid-cols-[minmax(0,1fr)_7rem_10rem_8rem] gap-4 border-b border-gray-100 px-4 py-2 text-xs text-gray-500 dark:border-dark-700 dark:text-gray-400 md:grid"><span>{{ t('usage.model') }}</span><span class="text-right">{{ t('usage.keySummary.requests') }}</span><span class="text-right">{{ t('usage.keySummary.tokens') }}</span><span class="text-right">{{ t('usage.keySummary.cost') }}</span></div>
          <div class="divide-y divide-gray-100 dark:divide-dark-700">
            <div v-for="model in group.models" :key="model.model" class="grid grid-cols-3 gap-3 px-4 py-4 text-sm md:grid-cols-[minmax(0,1fr)_7rem_10rem_8rem] md:gap-4" data-testid="key-detail-model">
              <p class="col-span-3 min-w-0 break-words font-medium text-gray-800 dark:text-gray-200 md:col-span-1">{{ model.model || t('usage.keyDetail.unrecordedModel') }}</p>
              <div class="min-w-0 md:text-right"><span class="mb-1 block text-xs text-gray-500 md:hidden">{{ t('usage.keySummary.requests') }}</span><span class="break-all tabular-nums text-gray-800 dark:text-gray-200">{{ model.requests.toLocaleString() }}</span></div>
              <div class="min-w-0 md:text-right"><span class="mb-1 block text-xs text-gray-500 md:hidden">{{ t('usage.keySummary.tokens') }}</span><details class="text-gray-800 dark:text-gray-200"><summary class="cursor-pointer break-all tabular-nums" :aria-label="`${model.model} ${t('usage.tokenDetails')}`">{{ formatCompactNumber(model.total_tokens) }}</summary><p class="mt-2 break-words text-xs leading-6 text-gray-500 dark:text-gray-400">{{ tokenDetails(model) }}</p></details></div>
              <div class="min-w-0 text-right"><span class="mb-1 block text-xs text-gray-500 md:hidden">{{ t('usage.keySummary.cost') }}</span><span class="break-all font-medium tabular-nums text-gray-800 dark:text-gray-200">${{ money(model.actual_cost) }}</span></div>
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
import { usageAPI, type APIKeyUsageBreakdown, type APIKeyUsageBreakdownGroup, type APIKeyUsageDetailSelection, type APIKeyUsageTotals } from '@/api/usage'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { formatCompactNumber } from '@/utils/format'
import { keyUsageDisplayDates, keyUsageRange, type KeyUsagePeriod } from '@/utils/keyUsageRange'

const props = defineProps<{ selection: APIKeyUsageDetailSelection }>()
const emit = defineEmits<{ close: [] }>()
const { t } = useI18n()
const period = ref<KeyUsagePeriod>(props.selection.period)
const range = ref({ start_time: props.selection.start_time, end_time: props.selection.end_time })
const groupId = ref<number>()
const customStart = ref('')
const customEnd = ref('')
const rangeError = ref('')
const loadError = ref(false)
const loading = ref(false)
const detail = ref<APIKeyUsageBreakdown | null>(null)
let controller: AbortController | undefined
const closed = ref(false)

const title = computed(() => `${t('usage.keyDetail.title')} · ${detail.value?.name || props.selection.key_name}`)
const rangeLabel = computed(() => `${new Date(range.value.start_time).toLocaleString()} — ${new Date(new Date(range.value.end_time).getTime() - 1).toLocaleString()}`)
const periodOptions = computed(() => [
  { value: '24h' as const, label: t('dates.last24Hours') }, { value: '7d' as const, label: t('dates.last7Days') },
  { value: '14d' as const, label: t('dates.last14Days') }, { value: '30d' as const, label: t('dates.last30Days') },
  { value: 'thisMonth' as const, label: t('dates.thisMonth') }, { value: 'lastMonth' as const, label: t('dates.lastMonth') },
  { value: 'custom' as const, label: t('usage.keySummary.custom') },
])
const money = (value: number) => value.toFixed(4)
const stats = computed(() => detail.value ? [
  { label: t('usage.keySummary.requests'), value: detail.value.summary.requests.toLocaleString() },
  { label: t('usage.keySummary.tokens'), value: formatCompactNumber(detail.value.summary.total_tokens) },
  { label: t('usage.keySummary.cost'), value: `$${money(detail.value.summary.actual_cost)}` },
] : [])
function groupName(group: APIKeyUsageBreakdownGroup) {
  if (group.group_id === null) return t('usage.keyDetail.unrecordedGroup')
  return group.group_name || t('usage.keyDetail.groupFallback', { id: group.group_id })
}
function tokenDetails(item: APIKeyUsageTotals) {
  return `${t('admin.usage.inputTokens')}: ${item.input_tokens.toLocaleString()} / ${t('admin.usage.outputTokens')}: ${item.output_tokens.toLocaleString()} / ${t('usage.keySummary.cacheRead')}: ${item.cache_read_tokens.toLocaleString()} / ${t('usage.keySummary.cacheWrite')}: ${item.cache_creation_tokens.toLocaleString()}`
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
    const data = await usageAPI.getAPIKeyBreakdown(props.selection.api_key_id, { ...range.value, group_id: groupId.value }, request.signal)
    if (!request.signal.aborted) detail.value = data
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
  const dates = keyUsageDisplayDates(range.value)
  customStart.value = dates.startDate
  customEnd.value = dates.endDate
  void load()
}
function applyCustom() {
  try { range.value = keyUsageRange('custom', new Date(), customStart.value, customEnd.value) }
  catch { rangeError.value = t('usage.keySummary.invalidRange'); return }
  rangeError.value = ''
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
