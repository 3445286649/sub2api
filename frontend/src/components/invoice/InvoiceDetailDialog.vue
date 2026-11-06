<template>
  <BaseDialog :show="show" :title="request ? t('invoice.detail.title', { id: request.id }) : ''" width="normal" mobile-sheet @close="emit('close')">
    <div v-if="loading && !request" class="space-y-3 py-2">
      <div v-for="i in 4" :key="i" class="h-10 animate-pulse rounded-lg bg-gray-100 dark:bg-dark-700" />
    </div>

    <div v-else-if="request" class="space-y-5">
      <div class="flex flex-wrap items-center justify-between gap-3 rounded-xl bg-gray-50 px-4 py-3.5 dark:bg-dark-900/40">
        <div>
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('invoice.summary.amount') }}</p>
          <p class="text-2xl font-bold tabular-nums text-gray-900 dark:text-white">{{ formatCny(request.total_amount) }}</p>
        </div>
        <InvoiceStatusBadge :status="request.status" />
      </div>

      <div
        v-if="request.status === 'rejected' && request.reject_reason"
        class="flex gap-2.5 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-500/30 dark:bg-red-500/10 dark:text-red-300"
      >
        <Icon name="exclamationCircle" size="sm" class="mt-0.5 shrink-0" />
        <div>
          <p class="font-medium">{{ t('invoice.detail.rejectReason') }}</p>
          <p class="mt-0.5 text-red-600/90 dark:text-red-300/90">{{ request.reject_reason }}</p>
        </div>
      </div>

      <div class="grid gap-5 sm:grid-cols-[1fr_11rem]">
        <dl class="space-y-2.5 text-sm">
          <div v-for="row in rows" :key="row.label" class="flex justify-between gap-4">
            <dt class="shrink-0 text-gray-500 dark:text-gray-400">{{ row.label }}</dt>
            <dd class="min-w-0 break-all text-right text-gray-900 dark:text-gray-100" :class="{ 'font-mono text-[13px]': row.mono }">{{ row.value }}</dd>
          </div>
        </dl>
        <InvoiceTimeline :request="request" class="border-t border-gray-100 pt-4 dark:border-dark-700 sm:border-l sm:border-t-0 sm:pl-5 sm:pt-0" />
      </div>

      <div v-if="request.orders?.length">
        <p class="mb-2 text-xs font-medium uppercase tracking-wide text-gray-400 dark:text-dark-400">
          {{ t('invoice.confirm.ordersSection') }} · {{ request.orders.length }}
        </p>
        <ul class="divide-y divide-gray-100 rounded-xl border border-gray-100 dark:divide-dark-700 dark:border-dark-700">
          <li v-for="o in request.orders" :key="o.id" class="flex items-center justify-between gap-3 px-3.5 py-2.5 text-sm">
            <div class="min-w-0">
              <p class="truncate font-mono text-[13px] text-gray-800 dark:text-gray-100">{{ orderNumberOf(o) }}</p>
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ formatDateTimeToMinute(o.paid_at) }}</p>
            </div>
            <span class="shrink-0 font-medium tabular-nums text-gray-900 dark:text-white">{{ formatCny(o.invoiceable_amount) }}</span>
          </li>
        </ul>
      </div>
    </div>

    <template #footer>
      <div class="flex w-full flex-wrap justify-end gap-2">
        <button type="button" class="btn btn-secondary" @click="emit('close')">{{ t('common.close') }}</button>
        <button v-if="request?.status === 'rejected'" type="button" class="btn btn-primary" @click="emit('reapply', request)">
          {{ t('invoice.detail.reapply') }}
        </button>
        <button
          v-if="request?.status === 'completed' && request.has_file"
          type="button"
          class="btn btn-primary"
          :disabled="downloading"
          data-testid="invoice-download"
          @click="download"
        >
          <Icon name="download" size="sm" class="mr-1.5" />{{ downloading ? t('invoice.records.downloading') : t('invoice.records.download') }}
        </button>
        <a
          v-else-if="request?.status === 'completed' && request.invoice_file_url"
          :href="request.invoice_file_url"
          target="_blank"
          rel="noopener noreferrer"
          class="btn btn-primary"
        >
          <Icon name="download" size="sm" class="mr-1.5" />{{ t('invoice.records.download') }}
        </a>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { formatDateTimeToMinute } from '@/utils/format'
import type { InvoiceRequest } from '@/types/invoice'
import InvoiceStatusBadge from './InvoiceStatusBadge.vue'
import InvoiceTimeline from './InvoiceTimeline.vue'
import { invoiceAPI } from '@/api/invoice'
import { useAppStore } from '@/stores/app'
import { saveBlob } from './invoiceFile'
import { formatCny, orderNumberOf } from './invoiceFormat'

const props = defineProps<{ show: boolean; loading: boolean; request: InvoiceRequest | null }>()
const emit = defineEmits<{ (e: 'close'): void; (e: 'reapply', request: InvoiceRequest): void }>()
const { t } = useI18n()
const appStore = useAppStore()
const downloading = ref(false)

// 登录态在请求头里，不能直接用 <a href>，先取 Blob 再保存。失败时错误体也是 Blob，只能按状态码区分。
async function download() {
  const r = props.request
  if (!r || downloading.value) return
  downloading.value = true
  try {
    const blob = await invoiceAPI.downloadFile(r.id)
    saveBlob(blob, r.invoice_file_name || `invoice-${r.id}.pdf`)
  } catch (err) {
    const status = (err as { status?: number } | null)?.status
    appStore.showError(status === 404 ? t('invoice.records.fileNotFound') : t('invoice.records.downloadFailed'))
  } finally {
    downloading.value = false
  }
}

const rows = computed(() => {
  const r = props.request
  if (!r) return []
  const enterprise = r.invoice_type === 'enterprise'
  const list: { label: string; value: string; mono?: boolean }[] = [
    { label: t('invoice.confirm.type'), value: t(`invoice.title.${r.invoice_type}`) },
    { label: enterprise ? t('invoice.title.enterpriseName') : t('invoice.title.individualName'), value: r.title },
  ]
  // 历史申请没有类目，不显示这一行
  if (r.category_name) list.push({ label: t('invoice.category.label'), value: r.category_name })
  if (enterprise && r.enterprise_tax_number) list.push({ label: t('invoice.title.taxNumber'), value: r.enterprise_tax_number, mono: true })
  if (!enterprise && r.individual_id_number) list.push({ label: t('invoice.title.idNumber'), value: r.individual_id_number, mono: true })
  list.push({ label: t('invoice.detail.receiverEmail'), value: r.receiver_email })
  if (r.invoice_number) list.push({ label: t('invoice.detail.invoiceNumber'), value: r.invoice_number, mono: true })
  return list
})
</script>
