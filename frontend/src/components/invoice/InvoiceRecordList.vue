<template>
  <div class="card p-5">
    <div class="-mx-1 mb-4 flex gap-1.5 overflow-x-auto px-1 pb-1">
      <button
        v-for="f in filters"
        :key="f.value || 'all'"
        type="button"
        class="shrink-0 rounded-full px-3 py-1 text-xs font-medium transition-colors"
        :class="status === f.value
          ? 'bg-primary-500 text-white'
          : 'bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-dark-700 dark:text-gray-300 dark:hover:bg-dark-600'"
        @click="setStatus(f.value)"
      >{{ f.label }}</button>
    </div>

    <div v-if="loading && items.length === 0" class="space-y-3">
      <div v-for="i in 3" :key="i" class="h-[72px] animate-pulse rounded-xl bg-gray-100 dark:bg-dark-700/60" />
    </div>

    <div v-else-if="items.length === 0" class="py-12 text-center">
      <span class="mx-auto mb-3 flex h-12 w-12 items-center justify-center rounded-2xl bg-gray-100 dark:bg-dark-700">
        <Icon name="inbox" size="lg" class="text-gray-400 dark:text-dark-400" />
      </span>
      <p class="text-sm font-medium text-gray-700 dark:text-gray-200">{{ t('invoice.records.empty') }}</p>
      <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('invoice.records.emptyHint') }}</p>
    </div>

    <ul v-else class="space-y-2.5" :class="{ 'opacity-60 transition-opacity': loading }">
      <li v-for="r in items" :key="r.id">
        <button
          type="button"
          class="group flex w-full items-center gap-3 rounded-xl border border-gray-200 bg-white/60 px-4 py-3 text-left transition-all hover:border-primary-300 hover:bg-white dark:border-dark-700 dark:bg-dark-800/40 dark:hover:border-primary-700/70 dark:hover:bg-dark-800/70"
          @click="emit('open', r.id)"
        >
          <span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg" :class="iconWrapClass(r.status)">
            <Icon :name="r.invoice_type === 'enterprise' ? 'database' : 'user'" size="md" />
          </span>
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2">
              <span class="truncate text-sm font-semibold text-gray-900 dark:text-white">{{ r.title }}</span>
              <InvoiceStatusBadge :status="r.status" />
            </div>
            <p class="mt-1 truncate text-xs text-gray-500 dark:text-gray-400">
              #{{ r.id }} · <template v-if="r.category_name">{{ r.category_name }} · </template>{{ t('invoice.records.orderCount', { count: r.order_count }) }} ·
              {{ t('invoice.records.appliedAt', { time: formatDateTimeToMinute(r.created_at) }) }}
            </p>
          </div>
          <div class="shrink-0 text-right">
            <p class="text-base font-bold tabular-nums text-gray-900 dark:text-white">{{ formatCny(r.total_amount) }}</p>
            <p class="mt-0.5 inline-flex items-center text-xs text-gray-400 group-hover:text-primary-600 dark:text-dark-400 dark:group-hover:text-primary-400">
              {{ t('invoice.records.detail') }}<Icon name="chevronRight" size="xs" class="ml-0.5" />
            </p>
          </div>
        </button>
      </li>
    </ul>

    <div v-if="pages > 1" class="mt-4 flex items-center justify-center gap-3 text-sm">
      <button type="button" class="btn btn-secondary btn-sm" :disabled="page <= 1 || loading" @click="emit('page', page - 1)">
        <Icon name="chevronLeft" size="sm" />
      </button>
      <span class="tabular-nums text-gray-500 dark:text-gray-400">{{ page }} / {{ pages }}</span>
      <button type="button" class="btn btn-secondary btn-sm" :disabled="page >= pages || loading" @click="emit('page', page + 1)">
        <Icon name="chevronRight" size="sm" />
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { formatDateTimeToMinute } from '@/utils/format'
import type { InvoiceRequest, InvoiceRequestStatus } from '@/types/invoice'
import InvoiceStatusBadge from './InvoiceStatusBadge.vue'
import { formatCny } from './invoiceFormat'

defineProps<{ items: InvoiceRequest[]; loading: boolean; page: number; pages: number }>()
const status = defineModel<InvoiceRequestStatus | ''>('status', { default: '' })
const emit = defineEmits<{ (e: 'open', id: number): void; (e: 'page', page: number): void }>()
const { t } = useI18n()

const filters = computed<{ value: InvoiceRequestStatus | ''; label: string }[]>(() => [
  { value: '', label: t('invoice.records.filterAll') },
  ...(['pending', 'processing', 'completed', 'rejected'] as const).map((s) => ({ value: s, label: t(`invoice.status.${s}`) })),
])

function setStatus(value: InvoiceRequestStatus | '') {
  status.value = value
}

function iconWrapClass(s: InvoiceRequestStatus) {
  return {
    pending: 'bg-amber-50 text-amber-600 dark:bg-amber-500/10 dark:text-amber-300',
    processing: 'bg-sky-50 text-sky-600 dark:bg-sky-500/10 dark:text-sky-300',
    completed: 'bg-emerald-50 text-emerald-600 dark:bg-emerald-500/10 dark:text-emerald-300',
    rejected: 'bg-red-50 text-red-500 dark:bg-red-500/10 dark:text-red-300',
  }[s]
}
</script>
