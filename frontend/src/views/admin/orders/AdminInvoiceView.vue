<template>
  <AppLayout>
    <div class="space-y-4" data-testid="admin-invoice-view">
      <!-- 统计 -->
      <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
        <button
          v-for="card in statCards"
          :key="card.key"
          type="button"
          class="card p-4 text-left transition-shadow hover:shadow-md"
          :class="{ 'ring-2 ring-primary-500/50': card.status && filters.status === card.status }"
          @click="card.status && setStatus(filters.status === card.status ? '' : card.status)"
        >
          <div class="flex items-center gap-3">
            <div class="rounded-lg p-2" :class="card.iconWrap">
              <Icon :name="card.icon" size="md" :stroke-width="2" />
            </div>
            <div class="min-w-0">
              <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ card.label }}</p>
              <p class="text-xl font-bold tabular-nums text-gray-900 dark:text-white">
                <span v-if="!statsLoaded" class="inline-block h-6 w-16 animate-pulse rounded bg-gray-200 align-middle dark:bg-dark-700" />
                <template v-else>{{ card.value }}</template>
              </p>
              <p class="truncate text-xs text-gray-500 dark:text-gray-400">{{ card.sub }}</p>
            </div>
          </div>
        </button>
      </div>

      <!-- 筛选 -->
      <div class="card p-4">
        <div class="flex flex-wrap items-center gap-3">
          <div class="flex-1 sm:max-w-72">
            <input v-model="keyword" type="text" class="input" :placeholder="t('invoice.admin.searchPlaceholder')" @input="debounceLoad" />
          </div>
          <Select v-model="filters.status" :options="statusOptions" class="w-36" @change="reload" />
          <Select v-model="filters.invoice_type" :options="typeOptions" class="w-36" @change="reload" />
          <div class="flex flex-1 justify-end gap-2">
            <button type="button" class="btn btn-secondary" data-testid="admin-invoice-exclusions" @click="exclusionsOpen = true">
              <Icon name="ban" size="sm" class="mr-1.5" />{{ t('invoice.exclusions.open') }}
            </button>
            <button class="btn btn-secondary" :disabled="loading" :title="t('common.refresh')" @click="refreshAll">
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
          </div>
        </div>
      </div>

      <!-- 列表 -->
      <DataTable :columns="columns" :data="items" :loading="loading" row-key="id">
        <template #cell-id="{ value }">
          <span class="font-mono text-sm">#{{ value }}</span>
        </template>
        <template #cell-title="{ row }">
          <div class="flex min-w-0 items-center gap-2.5">
            <span class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-gray-400">
              <Icon :name="row.invoice_type === 'enterprise' ? 'database' : 'user'" size="sm" />
            </span>
            <div class="min-w-0">
              <p class="max-w-[16rem] truncate text-sm font-medium text-gray-900 dark:text-white">{{ row.title }}</p>
              <p class="max-w-[16rem] truncate font-mono text-xs text-gray-400 dark:text-dark-400">
                {{ row.invoice_type === 'enterprise' ? row.enterprise_tax_number : t('invoice.title.individual') }}
              </p>
            </div>
          </div>
        </template>
        <template #cell-user_email="{ value, row }">
          <div class="text-sm">
            <p class="text-gray-900 dark:text-white">{{ value }}</p>
            <p class="text-xs text-gray-400">#{{ row.user_id }}</p>
          </div>
        </template>
        <template #cell-total_amount="{ value, row }">
          <div class="text-sm">
            <p class="font-semibold tabular-nums text-gray-900 dark:text-white">{{ formatCny(value) }}</p>
            <p class="text-xs text-gray-400">{{ t('invoice.records.orderCount', { count: row.order_count }) }}</p>
          </div>
        </template>
        <template #cell-status="{ value, row }">
          <div class="flex flex-col items-start gap-1">
            <InvoiceStatusBadge :status="value" />
            <span v-if="value === 'completed' && !row.email_sent_at" class="text-[11px] text-amber-600 dark:text-amber-400">{{ t('invoice.admin.emailNotSent') }}</span>
          </div>
        </template>
        <template #cell-created_at="{ value }">
          <span class="whitespace-nowrap text-xs text-gray-500 dark:text-gray-400">{{ formatDateTimeToMinute(value) }}</span>
        </template>
        <template #cell-actions="{ row }">
          <button
            type="button"
            class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium"
            :class="row.status === 'pending' || row.status === 'processing'
              ? 'text-primary-600 hover:bg-primary-50 dark:text-primary-400 dark:hover:bg-primary-900/20'
              : 'text-gray-600 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-dark-600'"
            :data-testid="`admin-invoice-open-${row.id}`"
            @click="openDetail(row.id)"
          >
            <Icon :name="row.status === 'pending' || row.status === 'processing' ? 'edit' : 'eye'" size="sm" />
            {{ row.status === 'pending' || row.status === 'processing' ? t('invoice.admin.process') : t('common.view') }}
          </button>
        </template>
      </DataTable>
      <Pagination
        v-if="pagination.total > 0"
        :page="pagination.page"
        :total="pagination.total"
        :page-size="pagination.page_size"
        @update:page="onPage"
        @update:page-size="onPageSize"
      />
    </div>

    <AdminInvoiceProcessDialog
      :show="detailOpen"
      :loading="detailLoading"
      :submitting="submitting"
      :request="detail"
      :uploading="uploading"
      :downloading="downloading"
      @close="detailOpen = false"
      @update="handleUpdate"
      @send-email="handleSendEmail"
      @upload-file="handleUpload"
      @download-file="handleDownload"
    />
    <AdminInvoiceExclusionDialog :show="exclusionsOpen" @close="exclusionsOpen = false" />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import AdminInvoiceExclusionDialog from '@/components/invoice/AdminInvoiceExclusionDialog.vue'
import AdminInvoiceProcessDialog from '@/components/invoice/AdminInvoiceProcessDialog.vue'
import InvoiceStatusBadge from '@/components/invoice/InvoiceStatusBadge.vue'
import { saveBlob } from '@/components/invoice/invoiceFile'
import { formatCny } from '@/components/invoice/invoiceFormat'
import { adminInvoiceAPI, INVOICE_PENDING_EVENT } from '@/api/admin/invoice'
import { useAppStore } from '@/stores/app'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { formatDateTimeToMinute } from '@/utils/format'
import type { Column } from '@/components/common/types'
import type {
  AdminInvoiceRequest,
  AdminUpdateInvoiceStatus,
  InvoiceRequestStatus,
  InvoiceStatusCounts,
  InvoiceType,
} from '@/types/invoice'

const { t } = useI18n()
const route = useRoute()
const appStore = useAppStore()

// ---- 统计 ----
const stats = reactive<InvoiceStatusCounts>({ pending: 0, processing: 0, completed: 0, rejected: 0, open_amount: 0, completed_amount: 0, today_new: 0 })
const statsLoaded = ref(false)

async function loadStats() {
  try {
    Object.assign(stats, await adminInvoiceAPI.getStats())
    statsLoaded.value = true
    window.dispatchEvent(new CustomEvent(INVOICE_PENDING_EVENT, { detail: stats.pending }))
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'invoice.errors', t('invoice.errors.loadFailed')))
  }
}

const statCards = computed(() => [
  {
    key: 'pending', status: 'pending' as const, icon: 'clock' as const, label: t('invoice.status.pending'),
    value: stats.pending, sub: t('invoice.admin.todayNew', { count: stats.today_new }),
    iconWrap: 'bg-amber-100 text-amber-600 dark:bg-amber-900/30 dark:text-amber-400',
  },
  {
    key: 'processing', status: 'processing' as const, icon: 'sync' as const, label: t('invoice.status.processing'),
    value: stats.processing, sub: t('invoice.admin.openAmount', { amount: formatCny(stats.open_amount) }),
    iconWrap: 'bg-sky-100 text-sky-600 dark:bg-sky-900/30 dark:text-sky-400',
  },
  {
    key: 'completed', status: 'completed' as const, icon: 'checkCircle' as const, label: t('invoice.status.completed'),
    value: stats.completed, sub: t('invoice.admin.completedAmount', { amount: formatCny(stats.completed_amount) }),
    iconWrap: 'bg-emerald-100 text-emerald-600 dark:bg-emerald-900/30 dark:text-emerald-400',
  },
  {
    key: 'rejected', status: 'rejected' as const, icon: 'xCircle' as const, label: t('invoice.status.rejected'),
    value: stats.rejected, sub: t('invoice.admin.rejectedSub'),
    iconWrap: 'bg-red-100 text-red-600 dark:bg-red-900/30 dark:text-red-400',
  },
])

// ---- 列表 ----
const items = ref<AdminInvoiceRequest[]>([])
const loading = ref(false)
const keyword = ref('')
const filters = reactive<{ status: InvoiceRequestStatus | ''; invoice_type: InvoiceType | '' }>({
  status: (['pending', 'processing', 'completed', 'rejected'] as const).find((s) => s === route.query.status) ?? '',
  invoice_type: '',
})
const pagination = reactive({ page: 1, page_size: 20, total: 0 })

const columns = computed<Column[]>(() => [
  { key: 'id', label: t('invoice.admin.col.id') },
  { key: 'title', label: t('invoice.admin.col.title') },
  { key: 'user_email', label: t('invoice.admin.col.user') },
  { key: 'total_amount', label: t('invoice.admin.col.amount') },
  { key: 'status', label: t('invoice.admin.col.status') },
  { key: 'created_at', label: t('invoice.admin.col.createdAt') },
  { key: 'actions', label: t('invoice.admin.col.actions') },
])

const statusOptions = computed(() => [
  { value: '', label: t('invoice.admin.allStatuses') },
  ...(['pending', 'processing', 'completed', 'rejected'] as const).map((s) => ({ value: s, label: t(`invoice.status.${s}`) })),
])
const typeOptions = computed(() => [
  { value: '', label: t('invoice.admin.allTypes') },
  { value: 'enterprise', label: t('invoice.title.enterprise') },
  { value: 'individual', label: t('invoice.title.individual') },
])

async function loadList() {
  loading.value = true
  try {
    const res = await adminInvoiceAPI.listRequests({
      page: pagination.page,
      page_size: pagination.page_size,
      status: filters.status || undefined,
      invoice_type: filters.invoice_type || undefined,
      keyword: keyword.value.trim() || undefined,
    })
    items.value = res.items
    pagination.total = res.total
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'invoice.errors', t('invoice.errors.loadFailed')))
  } finally {
    loading.value = false
  }
}

function reload() {
  pagination.page = 1
  void loadList()
}
function setStatus(status: InvoiceRequestStatus | '') {
  filters.status = status
  reload()
}
function refreshAll() {
  void loadStats()
  void loadList()
}
function onPage(page: number) {
  pagination.page = page
  void loadList()
}
function onPageSize(size: number) {
  pagination.page_size = size
  reload()
}

let debounceTimer: ReturnType<typeof setTimeout> | null = null
function debounceLoad() {
  if (debounceTimer) clearTimeout(debounceTimer)
  debounceTimer = setTimeout(reload, 300)
}

const exclusionsOpen = ref(false)

// ---- 详情 / 处理 ----
const detailOpen = ref(false)
const detailLoading = ref(false)
const submitting = ref(false)
const detail = ref<AdminInvoiceRequest | null>(null)

async function openDetail(id: number) {
  detail.value = null
  detailOpen.value = true
  detailLoading.value = true
  try {
    detail.value = await adminInvoiceAPI.getRequest(id)
  } catch (err) {
    detailOpen.value = false
    appStore.showError(extractI18nErrorMessage(err, t, 'invoice.errors', t('invoice.errors.loadFailed')))
  } finally {
    detailLoading.value = false
  }
}

async function handleUpdate(id: number, payload: AdminUpdateInvoiceStatus) {
  submitting.value = true
  try {
    detail.value = await adminInvoiceAPI.updateStatus(id, payload)
    appStore.showSuccess(t(`invoice.admin.updated.${payload.status}`))
    // 发票已开出但邮件没发出去时状态不回滚，这里提醒管理员手动重发
    if (payload.send_email && !detail.value.email_sent_at) appStore.showWarning(t('invoice.admin.emailFailedAfterComplete'))
    refreshAll()
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'invoice.errors', t('invoice.admin.updateFailed')))
  } finally {
    submitting.value = false
  }
}

async function handleSendEmail(id: number) {
  submitting.value = true
  try {
    detail.value = await adminInvoiceAPI.sendEmail(id)
    appStore.showSuccess(t('invoice.admin.emailSent', { email: detail.value.receiver_email }))
    void loadList()
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'invoice.errors', t('invoice.admin.updateFailed')))
  } finally {
    submitting.value = false
  }
}

const uploading = ref(false)
const downloading = ref(false)

async function handleUpload(id: number, file: File) {
  uploading.value = true
  try {
    const updated = await adminInvoiceAPI.uploadFile(id, file)
    // 上传期间管理员可能已经切到别的申请，只刷新仍在查看的那一条
    if (detail.value?.id === id) detail.value = updated
    appStore.showSuccess(t('invoice.admin.file.uploaded'))
    void loadList()
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'invoice.errors', t('invoice.admin.file.uploadFailed')))
  } finally {
    uploading.value = false
  }
}

async function handleDownload(id: number) {
  downloading.value = true
  try {
    const blob = await adminInvoiceAPI.downloadFile(id)
    const name = detail.value?.id === id ? detail.value.invoice_file_name : null
    saveBlob(blob, name || `invoice-${id}.pdf`)
  } catch {
    appStore.showError(t('invoice.admin.file.downloadFailed'))
  } finally {
    downloading.value = false
  }
}

onMounted(refreshAll)
</script>
