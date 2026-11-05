<template>
  <BaseDialog :show="show" :title="request ? t('invoice.detail.title', { id: request.id }) : ''" width="wide" mobile-sheet @close="emit('close')">
    <div v-if="loading && !request" class="space-y-3 py-2">
      <div v-for="i in 5" :key="i" class="h-10 animate-pulse rounded-lg bg-gray-100 dark:bg-dark-700" />
    </div>

    <div v-else-if="request" class="space-y-5" data-testid="admin-invoice-dialog">
      <!-- 概览 -->
      <div class="flex flex-wrap items-center justify-between gap-3 rounded-xl bg-gray-50 px-4 py-3.5 dark:bg-dark-900/40">
        <div class="min-w-0">
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('invoice.summary.amount') }}</p>
          <p class="text-2xl font-bold tabular-nums text-gray-900 dark:text-white">{{ formatCny(request.total_amount) }}</p>
          <p class="mt-0.5 truncate text-xs text-gray-500 dark:text-gray-400">
            {{ request.user_email }} · #{{ request.user_id }} · {{ t('invoice.records.appliedAt', { time: formatDateTimeToMinute(request.created_at) }) }}
          </p>
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
          <p class="mt-0.5">{{ request.reject_reason }}</p>
        </div>
      </div>

      <div class="grid gap-5 md:grid-cols-[1fr_12rem]">
        <!-- 抬头：开票时要逐项抄到税务平台，所以每行都能复制 -->
        <section>
          <p class="mb-2 text-xs font-medium uppercase tracking-wide text-gray-400 dark:text-dark-400">{{ t('invoice.confirm.titleSection') }}</p>
          <dl class="divide-y divide-gray-100 rounded-xl border border-gray-100 text-sm dark:divide-dark-700 dark:border-dark-700">
            <div v-for="row in titleRows" :key="row.label" class="group flex items-center justify-between gap-3 px-3.5 py-2">
              <dt class="shrink-0 text-gray-500 dark:text-gray-400">{{ row.label }}</dt>
              <dd class="flex min-w-0 items-center gap-1.5">
                <span class="break-all text-right text-gray-900 dark:text-gray-100" :class="{ 'font-mono text-[13px]': row.mono }">{{ row.value }}</span>
                <button
                  v-if="row.copy"
                  type="button"
                  class="shrink-0 rounded p-1 text-gray-400 opacity-60 transition hover:bg-gray-100 hover:text-primary-600 group-hover:opacity-100 dark:hover:bg-dark-700"
                  :title="t('invoice.admin.copy')"
                  @click="copyToClipboard(row.value, t('invoice.admin.copied', { field: row.label }))"
                >
                  <Icon name="copy" size="xs" />
                </button>
              </dd>
            </div>
          </dl>
        </section>
        <section>
          <p class="mb-2 text-xs font-medium uppercase tracking-wide text-gray-400 dark:text-dark-400">{{ t('invoice.admin.progress') }}</p>
          <InvoiceTimeline :request="request" />
        </section>
      </div>

      <!-- 订单明细：实付 / 赠送分开列，只有实付计入开票 -->
      <section v-if="request.orders?.length">
        <p class="mb-2 text-xs font-medium uppercase tracking-wide text-gray-400 dark:text-dark-400">
          {{ t('invoice.confirm.ordersSection') }} · {{ request.orders.length }}
        </p>
        <div class="overflow-x-auto rounded-xl border border-gray-100 dark:border-dark-700">
          <table class="w-full min-w-[520px] text-sm">
            <thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-900/40 dark:text-gray-400">
              <tr>
                <th class="px-3.5 py-2 text-left font-medium">{{ t('invoice.admin.orderCode') }}</th>
                <th class="px-3.5 py-2 text-left font-medium">{{ t('invoice.admin.paidAt') }}</th>
                <th class="px-3.5 py-2 text-left font-medium">{{ t('invoice.admin.method') }}</th>
                <th class="px-3.5 py-2 text-right font-medium">{{ t('invoice.orders.paid') }}</th>
                <th class="px-3.5 py-2 text-right font-medium">{{ t('invoice.admin.bonus') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="o in request.orders" :key="o.id">
                <td class="px-3.5 py-2 font-mono text-[13px] text-gray-800 dark:text-gray-100">{{ orderNumberOf(o) }}</td>
                <td class="whitespace-nowrap px-3.5 py-2 text-gray-500 dark:text-gray-400">{{ formatDateTimeToMinute(o.paid_at) }}</td>
                <td class="px-3.5 py-2 text-gray-600 dark:text-gray-300">{{ te(`payment.methods.${o.payment_type}`) ? t(`payment.methods.${o.payment_type}`) : o.payment_type }}</td>
                <td class="px-3.5 py-2 text-right font-medium tabular-nums text-emerald-600 dark:text-emerald-400">{{ formatCny(o.invoiceable_amount) }}</td>
                <td class="px-3.5 py-2 text-right tabular-nums text-gray-400 dark:text-dark-400">{{ o.bonus_amount > 0 ? `$${o.bonus_amount.toFixed(2)}` : '—' }}</td>
              </tr>
            </tbody>
            <tfoot class="border-t border-gray-200 text-sm dark:border-dark-600">
              <tr>
                <td colspan="3" class="px-3.5 py-2.5 text-gray-500 dark:text-gray-400">{{ t('invoice.admin.totalHint') }}</td>
                <td class="px-3.5 py-2.5 text-right font-bold tabular-nums text-gray-900 dark:text-white">{{ formatCny(request.total_amount) }}</td>
                <td class="px-3.5 py-2.5 text-right tabular-nums text-gray-400 line-through dark:text-dark-400">${{ bonusTotal.toFixed(2) }}</td>
              </tr>
            </tfoot>
          </table>
        </div>
      </section>

      <!-- 已开具：发票信息 -->
      <section v-if="request.status === 'completed'" class="rounded-xl border border-emerald-200 bg-emerald-50/60 px-4 py-3.5 text-sm dark:border-emerald-500/30 dark:bg-emerald-500/10">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div class="min-w-0">
            <p class="text-xs text-emerald-700/80 dark:text-emerald-300/80">{{ t('invoice.detail.invoiceNumber') }}</p>
            <p class="font-mono text-[13px] font-medium text-emerald-900 dark:text-emerald-100">{{ request.invoice_number }}</p>
            <p class="mt-1 text-xs text-emerald-700/80 dark:text-emerald-300/80">
              {{ request.email_sent_at
                ? t('invoice.admin.emailSentAt', { time: formatDateTimeToMinute(request.email_sent_at), email: request.receiver_email })
                : t('invoice.admin.emailNotSent') }}
            </p>
          </div>
          <div class="flex flex-wrap gap-2">
            <button v-if="request.has_file" type="button" class="btn btn-secondary btn-sm" :disabled="downloading" data-testid="admin-invoice-download" @click="emit('download-file', request.id)">
              <Icon name="download" size="sm" class="mr-1" />{{ t('invoice.admin.file.download') }}
            </button>
            <a v-if="request.invoice_file_url" :href="request.invoice_file_url" target="_blank" rel="noopener noreferrer" class="btn btn-secondary btn-sm">
              <Icon name="externalLink" size="sm" class="mr-1" />{{ t('invoice.admin.viewFile') }}
            </a>
          </div>
        </div>
      </section>

      <!-- 操作区 -->
      <section v-if="actionable" class="rounded-xl border border-gray-200 p-4 dark:border-dark-600">
        <div class="mb-4 flex rounded-lg bg-gray-100 p-0.5 dark:bg-dark-900/60" role="tablist">
          <button
            v-for="m in modes"
            :key="m"
            type="button"
            role="tab"
            :aria-selected="mode === m"
            class="flex-1 rounded-md px-3 py-1.5 text-sm font-medium transition-all"
            :class="mode === m ? 'bg-white text-gray-900 shadow-sm dark:bg-dark-700 dark:text-white' : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200'"
            :data-testid="`admin-invoice-mode-${m}`"
            @click="mode = m"
          >{{ t(`invoice.admin.mode.${m}`) }}</button>
        </div>

        <div v-if="mode === 'complete'" class="grid gap-4 sm:grid-cols-2">
          <div>
            <label class="input-label" for="adm-inv-number">{{ t('invoice.detail.invoiceNumber') }}<span class="text-red-500"> *</span></label>
            <input id="adm-inv-number" v-model.trim="draft.invoice_number" class="input font-mono" :class="{ 'input-error': showErrors && !draft.invoice_number }"
              :placeholder="t('invoice.admin.invoiceNumberPlaceholder')" maxlength="32" spellcheck="false" />
          </div>
          <div>
            <label class="input-label">{{ t('invoice.admin.file.label') }}<span class="text-red-500"> *</span></label>
            <input ref="fileInput" type="file" accept=".pdf,application/pdf" class="hidden" data-testid="admin-invoice-file-input" @change="onFileChosen" />
            <div
              v-if="request.has_file"
              class="flex items-center justify-between gap-2 rounded-lg border border-emerald-200 bg-emerald-50/60 px-3 py-2 text-sm dark:border-emerald-500/30 dark:bg-emerald-500/10"
              data-testid="admin-invoice-file-card"
            >
              <div class="min-w-0">
                <p class="truncate font-medium text-emerald-900 dark:text-emerald-100">{{ request.invoice_file_name }}</p>
                <p class="text-xs text-emerald-700/80 dark:text-emerald-300/80">{{ formatFileSize(request.invoice_file_size) }}</p>
              </div>
              <div class="flex shrink-0 gap-1.5">
                <button type="button" class="btn btn-secondary btn-sm" :disabled="downloading || uploading" @click="emit('download-file', request.id)">
                  <Icon name="download" size="sm" />
                </button>
                <button type="button" class="btn btn-secondary btn-sm" :disabled="uploading" data-testid="admin-invoice-replace-file" @click="pickFile">
                  {{ uploading ? t('invoice.admin.file.uploading') : t('invoice.admin.file.replace') }}
                </button>
              </div>
            </div>
            <button v-else type="button" class="btn btn-secondary w-full" :disabled="uploading" data-testid="admin-invoice-choose-file" @click="pickFile">
              <Icon name="upload" size="sm" class="mr-1.5" />{{ uploading ? t('invoice.admin.file.uploading') : t('invoice.admin.file.choose') }}
            </button>
            <p v-if="fileError" class="input-error-text" data-testid="admin-invoice-file-error">{{ fileError }}</p>
            <p v-else-if="showErrors && !fileOk" class="input-error-text">{{ t('invoice.admin.file.required') }}</p>
            <p v-else class="input-hint">{{ t('invoice.admin.file.hint') }}</p>
            <button type="button" class="mt-1 text-xs text-primary-600 hover:underline dark:text-primary-400" data-testid="admin-invoice-toggle-link" @click="linkMode = !linkMode">
              {{ linkMode ? t('invoice.admin.file.useUpload') : t('invoice.admin.file.useLink') }}
            </button>
            <template v-if="linkMode">
              <input v-model.trim="draft.invoice_file_url" class="input mt-2" :class="{ 'input-error': showErrors && !fileOk }" placeholder="https://" spellcheck="false" />
              <p v-if="showErrors && draft.invoice_file_url && !fileUrlValid" class="input-error-text">{{ t('invoice.admin.fileUrlInvalid') }}</p>
            </template>
          </div>
          <label class="flex cursor-pointer select-none items-center gap-2 text-sm text-gray-700 dark:text-gray-300 sm:col-span-2">
            <input v-model="draft.send_email" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500 dark:border-dark-500 dark:bg-dark-700" />
            {{ t('invoice.admin.sendEmailNow', { email: request.receiver_email }) }}
          </label>
        </div>

        <div v-else-if="mode === 'reject'">
          <label class="input-label" for="adm-inv-reject">{{ t('invoice.detail.rejectReason') }}<span class="text-red-500"> *</span></label>
          <textarea id="adm-inv-reject" v-model="draft.reject_reason" rows="3" class="input" :class="{ 'input-error': showErrors && !draft.reject_reason.trim() }"
            :placeholder="t('invoice.admin.rejectPlaceholder')" />
          <div class="mt-2 flex flex-wrap gap-1.5">
            <button v-for="preset in rejectPresets" :key="preset" type="button"
              class="rounded-full bg-gray-100 px-2.5 py-1 text-xs text-gray-600 hover:bg-gray-200 dark:bg-dark-700 dark:text-gray-300 dark:hover:bg-dark-600"
              @click="draft.reject_reason = preset">{{ preset }}</button>
          </div>
          <p class="input-hint mt-2">{{ t('invoice.admin.rejectHint') }}</p>
        </div>

        <p v-else class="text-sm text-gray-600 dark:text-gray-300">{{ t('invoice.admin.processingHint') }}</p>

        <div class="mt-4">
          <label class="input-label" for="adm-inv-notes">{{ t('invoice.detail.adminNotes') }}</label>
          <input id="adm-inv-notes" v-model="draft.admin_notes" class="input" :placeholder="t('invoice.admin.notesPlaceholder')" />
        </div>
      </section>
    </div>

    <template #footer>
      <div class="flex w-full flex-wrap justify-end gap-2">
        <button type="button" class="btn btn-secondary" :disabled="submitting" @click="emit('close')">{{ t('common.close') }}</button>
        <button v-if="request?.status === 'completed'" type="button" class="btn btn-secondary" :disabled="submitting" @click="emit('send-email', request.id)">
          <Icon name="mail" size="sm" class="mr-1.5" />{{ request.email_sent_at ? t('invoice.admin.resendEmail') : t('invoice.admin.sendEmail') }}
        </button>
        <button
          v-if="actionable"
          type="button"
          class="btn"
          :class="mode === 'reject' ? 'btn-danger' : 'btn-primary'"
          :disabled="submitting"
          data-testid="admin-invoice-submit"
          @click="submit"
        >
          <span v-if="submitting" class="mr-2 h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent" />
          {{ t(`invoice.admin.submit.${mode}`) }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { useClipboard } from '@/composables/useClipboard'
import { formatDateTimeToMinute } from '@/utils/format'
import type { AdminInvoiceRequest, AdminUpdateInvoiceStatus } from '@/types/invoice'
import InvoiceStatusBadge from './InvoiceStatusBadge.vue'
import InvoiceTimeline from './InvoiceTimeline.vue'
import { checkInvoiceFile, formatFileSize } from './invoiceFile'
import { formatCny, orderNumberOf, sumAmounts } from './invoiceFormat'

type Mode = 'processing' | 'complete' | 'reject'

const props = defineProps<{
  show: boolean
  loading: boolean
  submitting: boolean
  request: AdminInvoiceRequest | null
  uploading?: boolean
  downloading?: boolean
}>()
const emit = defineEmits<{
  (e: 'close'): void
  (e: 'update', id: number, payload: AdminUpdateInvoiceStatus): void
  (e: 'send-email', id: number): void
  (e: 'upload-file', id: number, file: File): void
  (e: 'download-file', id: number): void
}>()
const { t, te } = useI18n()
const { copyToClipboard } = useClipboard()

const actionable = computed(() => props.request?.status === 'pending' || props.request?.status === 'processing')
const modes = computed<Mode[]>(() => (props.request?.status === 'pending' ? ['processing', 'complete', 'reject'] : ['complete', 'reject']))
const mode = ref<Mode>('processing')
const showErrors = ref(false)
const fileError = ref('')
// 默认走上传；只有申请上已经有外部链接、且没有上传文件时，才展开链接输入框
const linkMode = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)
const draft = reactive({ invoice_number: '', invoice_file_url: '', send_email: true, reject_reason: '', admin_notes: '' })

// 切换到新的申请时重置操作区
watch(
  () => props.request?.id,
  () => {
    const r = props.request
    mode.value = r?.status === 'pending' ? 'processing' : 'complete'
    showErrors.value = false
    fileError.value = ''
    linkMode.value = !!r?.invoice_file_url && !r?.has_file
    Object.assign(draft, {
      invoice_number: r?.invoice_number ?? '',
      invoice_file_url: r?.invoice_file_url ?? '',
      send_email: true,
      reject_reason: '',
      admin_notes: r?.admin_notes ?? '',
    })
  },
  { immediate: true },
)

// 只有展开链接输入框时，填的链接才算数；收起后即使里面还有字也不会提交
const effectiveUrl = computed(() => (linkMode.value ? draft.invoice_file_url : ''))
const fileUrlValid = computed(() => /^https?:\/\/\S+$/i.test(effectiveUrl.value))
// 发票文件二选一：已上传 PDF，或填了有效的外部链接；已上传时多填的链接也要有效
const fileOk = computed(() => (props.request?.has_file ? !effectiveUrl.value || fileUrlValid.value : fileUrlValid.value))
const bonusTotal = computed(() => sumAmounts((props.request?.orders ?? []).map((o) => o.bonus_amount)))

const rejectPresets = computed(() => [
  t('invoice.admin.rejectPreset.titleMismatch'),
  t('invoice.admin.rejectPreset.refunded'),
  t('invoice.admin.rejectPreset.duplicate'),
])

const titleRows = computed(() => {
  const r = props.request
  if (!r) return []
  const enterprise = r.invoice_type === 'enterprise'
  const rows: { label: string; value: string; mono?: boolean; copy?: boolean }[] = [
    { label: t('invoice.confirm.type'), value: t(`invoice.title.${r.invoice_type}`) },
    { label: enterprise ? t('invoice.title.enterpriseName') : t('invoice.title.individualName'), value: r.title, copy: true },
  ]
  const optional: [string, string | null | undefined, boolean?][] = enterprise
    ? [
        [t('invoice.title.taxNumber'), r.enterprise_tax_number, true],
        [t('invoice.title.address'), r.enterprise_address],
        [t('invoice.title.phone'), r.enterprise_phone],
        [t('invoice.title.bank'), r.enterprise_bank],
        [t('invoice.title.bankAccount'), r.enterprise_bank_account, true],
      ]
    : [[t('invoice.title.idNumber'), r.individual_id_number, true]]
  for (const [label, value, mono] of optional) {
    if (value) rows.push({ label, value, mono, copy: true })
  }
  rows.push({ label: t('invoice.detail.receiverEmail'), value: r.receiver_email, copy: true })
  if (r.admin_notes && !actionable.value) rows.push({ label: t('invoice.detail.adminNotes'), value: r.admin_notes })
  return rows
})

function pickFile() {
  fileError.value = ''
  fileInput.value?.click()
}

function onFileChosen(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  // 清空后选同一个文件也会再次触发 change
  input.value = ''
  if (!file || !props.request) return
  const check = checkInvoiceFile(file)
  if (check !== 'ok') {
    fileError.value = t(`invoice.admin.file.${check}`)
    return
  }
  fileError.value = ''
  emit('upload-file', props.request.id, file)
}

function submit() {
  const r = props.request
  if (!r) return
  const notes = draft.admin_notes.trim()
  if (mode.value === 'processing') {
    emit('update', r.id, { status: 'processing', admin_notes: notes })
    return
  }
  if (mode.value === 'complete') {
    if (!draft.invoice_number || !fileOk.value) {
      showErrors.value = true
      return
    }
    emit('update', r.id, {
      status: 'completed',
      admin_notes: notes,
      invoice_number: draft.invoice_number,
      invoice_file_url: effectiveUrl.value || undefined,
      send_email: draft.send_email,
    })
    return
  }
  if (!draft.reject_reason.trim()) {
    showErrors.value = true
    return
  }
  emit('update', r.id, { status: 'rejected', admin_notes: notes, reject_reason: draft.reject_reason.trim() })
}
</script>
