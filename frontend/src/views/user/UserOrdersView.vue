<template>
  <AppLayout>
    <div class="space-y-4">
      <!-- Filters -->
      <div class="card p-4">
        <div class="flex flex-col gap-3 sm:flex-row sm:items-center">
          <div class="inline-flex w-full rounded-lg bg-gray-100 p-1 dark:bg-dark-800 sm:w-auto" role="tablist" :aria-label="t('points.userOrders.tabsLabel')">
            <button
              v-for="tab in orderTabs"
              :key="tab.key"
              type="button"
              role="tab"
              class="min-w-0 flex-1 rounded-md px-3 py-1.5 text-sm font-medium transition-colors sm:flex-none"
              :class="activeOrderTab === tab.key ? 'bg-white text-gray-900 shadow-sm dark:bg-dark-700 dark:text-white' : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200'"
              :aria-selected="activeOrderTab === tab.key"
              @click="switchOrderTab(tab.key)"
            >
              {{ tab.label }}
            </button>
          </div>
          <Select v-if="activeOrderTab === 'payment'" v-model="currentFilter" :options="statusFilters" class="w-full sm:w-36" @change="fetchOrders" />
          <div class="flex flex-1 items-center justify-end gap-2">
            <button @click="fetchOrders" :disabled="loading" class="btn btn-secondary" :title="t('common.refresh')">
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
            <button class="btn btn-primary" @click="router.push('/purchase')">{{ t('payment.result.backToRecharge') }}</button>
          </div>
        </div>
      </div>

      <!-- Table -->
      <OrderTable v-if="activeOrderTab === 'payment'" :orders="paymentOrders" :loading="loading">
        <template #actions="{ row }">
          <div class="flex items-center gap-2">
            <button v-if="row.status === 'PENDING'" @click="handleCancel(row.id)" class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-yellow-600 hover:bg-yellow-50 dark:text-yellow-400 dark:hover:bg-yellow-900/20">
              <Icon name="x" size="sm" />
              <span>{{ t('payment.orders.cancel') }}</span>
            </button>
            <button v-if="canRequestRefund(row)" @click="openRefundDialog(row)" class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-purple-600 hover:bg-purple-50 dark:text-purple-400 dark:hover:bg-purple-900/20">
              <Icon name="dollar" size="sm" />
              <span>{{ t('payment.orders.requestRefund') }}</span>
            </button>
          </div>
        </template>
      </OrderTable>
      <DataTable v-else :columns="pointsOrderColumns" :data="pointsOrders" :loading="loading">
        <template #cell-order_no="{ value }">
          <span class="font-mono text-xs text-gray-700 dark:text-gray-300">{{ value }}</span>
        </template>
        <template #cell-product_name="{ value }">
          <span class="font-medium text-gray-900 dark:text-white">{{ value }}</span>
        </template>
        <template #cell-points_price="{ value }">
          <span class="font-medium text-rose-600 dark:text-rose-400">-{{ value }} {{ t('points.unit') }}</span>
        </template>
        <template #cell-balance_amount="{ value }">
          <span class="font-medium text-emerald-600 dark:text-emerald-400">+${{ Number(value).toFixed(2) }}</span>
        </template>
        <template #cell-balance_after="{ value }">
          <span class="text-gray-700 dark:text-gray-300">${{ Number(value).toFixed(2) }}</span>
        </template>
        <template #cell-status="{ value }">
          <span class="inline-flex rounded-full bg-emerald-50 px-2.5 py-1 text-xs font-medium text-emerald-700 dark:bg-emerald-950/50 dark:text-emerald-300">
            {{ value === 'completed' ? t('points.userOrders.completed') : value }}
          </span>
        </template>
        <template #cell-completed_at="{ value }">
          <span class="text-xs text-gray-500 dark:text-gray-400">{{ formatOrderTime(value) }}</span>
        </template>
      </DataTable>

      <!-- Pagination -->
      <Pagination
        v-if="pagination.total > 0"
        :page="pagination.page"
        :total="pagination.total"
        :page-size="pagination.page_size"
        @update:page="handlePageChange"
        @update:pageSize="handlePageSizeChange"
      />
    </div>

    <!-- Cancel Confirm Dialog -->
    <BaseDialog :show="!!cancelTargetId" :title="t('payment.orders.cancel')" width="narrow" @close="cancelTargetId = null">
      <p class="text-sm text-gray-600 dark:text-gray-300">{{ t('payment.confirmCancel') }}</p>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button class="btn btn-secondary" @click="cancelTargetId = null">{{ t('common.cancel') }}</button>
          <button class="btn btn-danger" :disabled="actionLoading" @click="confirmCancel">{{ actionLoading ? t('common.processing') : t('payment.orders.cancel') }}</button>
        </div>
      </template>
    </BaseDialog>

    <!-- Refund Dialog -->
    <BaseDialog :show="!!refundTarget" :title="t('payment.orders.requestRefund')" @close="refundTarget = null">
      <div v-if="refundTarget" class="space-y-4">
        <div class="rounded-xl bg-gray-50 p-4 dark:bg-dark-800">
          <div class="flex justify-between text-sm">
            <span class="text-gray-500 dark:text-gray-400">{{ t('payment.orders.orderId') }}</span>
            <span class="font-mono text-gray-900 dark:text-white">#{{ refundTarget.id }}</span>
          </div>
          <div class="mt-2 flex justify-between text-sm">
            <span class="text-gray-500 dark:text-gray-400">{{ t('payment.orders.amount') }}</span>
            <span class="text-gray-900 dark:text-white">${{ refundTarget.amount.toFixed(2) }}</span>
          </div>
        </div>
        <div>
          <label class="input-label">{{ t('payment.refundReason') }}</label>
          <textarea v-model="refundReason" rows="3" class="input mt-1 w-full" :placeholder="t('payment.refundReasonPlaceholder')" />
        </div>
      </div>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button class="btn btn-secondary" @click="refundTarget = null">{{ t('common.cancel') }}</button>
          <button class="btn btn-primary" :disabled="actionLoading || !refundReason.trim()" @click="confirmRefund">{{ actionLoading ? t('common.processing') : t('payment.orders.requestRefund') }}</button>
        </div>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useAppStore } from '@/stores'
import { paymentAPI } from '@/api/payment'
import { pointsAPI } from '@/api/points'
import { extractI18nErrorMessage } from '@/utils/apiError'
import type { PaymentOrder } from '@/types/payment'
import type { PointsShopOrder } from '@/types/points'
import type { Column } from '@/components/common/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import OrderTable from '@/components/payment/OrderTable.vue'
import DataTable from '@/components/common/DataTable.vue'

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()

const loading = ref(false)
const actionLoading = ref(false)
const activeOrderTab = ref<'payment' | 'points'>('payment')
const paymentOrders = ref<PaymentOrder[]>([])
const pointsOrders = ref<PointsShopOrder[]>([])
const refundEligibleProviders = ref<Set<string>>(new Set())
const currentFilter = ref('')
const cancelTargetId = ref<number | null>(null)
const refundTarget = ref<PaymentOrder | null>(null)
const refundReason = ref('')
const pagination = reactive({ page: 1, page_size: 20, total: 0 })
let fetchRequestId = 0

const orderTabs = computed(() => [
  { key: 'payment' as const, label: t('points.userOrders.paymentTab') },
  { key: 'points' as const, label: t('points.userOrders.redemptionTab') },
])

const pointsOrderColumns = computed((): Column[] => [
  { key: 'order_no', label: t('points.orderNo') },
  { key: 'product_name', label: t('points.userOrders.product') },
  { key: 'points_price', label: t('points.userOrders.pointsSpent') },
  { key: 'balance_amount', label: t('points.userOrders.balanceCredited') },
  { key: 'balance_after', label: t('points.userOrders.balanceAfter') },
  { key: 'status', label: t('points.userOrders.status') },
  { key: 'completed_at', label: t('points.userOrders.completedAt') },
])

const statusFilters = computed(() => [
  { value: '', label: t('common.all') },
  { value: 'PENDING', label: t('payment.status.pending') },
  { value: 'COMPLETED', label: t('payment.status.completed') },
  { value: 'FAILED', label: t('payment.status.failed') },
  { value: 'REFUNDED', label: t('payment.status.refunded') },
])

async function fetchOrders() {
  const requestId = ++fetchRequestId
  const requestedTab = activeOrderTab.value
  loading.value = true
  try {
    if (requestedTab === 'points') {
      const res = await pointsAPI.getOrders({ page: pagination.page, page_size: pagination.page_size })
      if (requestId !== fetchRequestId) return
      pointsOrders.value = res.data.items || []
      pagination.total = res.data.total || 0
    } else {
      const res = await paymentAPI.getMyOrders({
        page: pagination.page,
        page_size: pagination.page_size,
        status: currentFilter.value || undefined,
      })
      if (requestId !== fetchRequestId) return
      paymentOrders.value = res.data.items || []
      pagination.total = res.data.total || 0
    }
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  } finally {
    if (requestId === fetchRequestId) loading.value = false
  }
}

function switchOrderTab(tab: 'payment' | 'points') {
  if (activeOrderTab.value === tab) return
  activeOrderTab.value = tab
  pagination.page = 1
  pagination.total = 0
  void fetchOrders()
}

function formatOrderTime(value: string): string {
  return new Date(value).toLocaleString()
}

function handlePageChange(page: number) { pagination.page = page; fetchOrders() }
function handlePageSizeChange(size: number) { pagination.page_size = size; pagination.page = 1; fetchOrders() }

function handleCancel(orderId: number) { cancelTargetId.value = orderId }

async function confirmCancel() {
  if (!cancelTargetId.value) return
  actionLoading.value = true
  try {
    await paymentAPI.cancelOrder(cancelTargetId.value)
    appStore.showSuccess(t('common.success'))
    cancelTargetId.value = null
    await fetchOrders()
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  } finally {
    actionLoading.value = false
  }
}

function openRefundDialog(order: PaymentOrder) { refundTarget.value = order; refundReason.value = '' }

async function confirmRefund() {
  if (!refundTarget.value || !refundReason.value.trim()) return
  actionLoading.value = true
  try {
    await paymentAPI.requestRefund(refundTarget.value.id, { reason: refundReason.value.trim() })
    appStore.showSuccess(t('common.success'))
    refundTarget.value = null
    refundReason.value = ''
    await fetchOrders()
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  } finally {
    actionLoading.value = false
  }
}

function canRequestRefund(order: PaymentOrder): boolean {
  if (order.status !== 'COMPLETED') return false
  if (!order.provider_instance_id) return false
  return refundEligibleProviders.value.has(order.provider_instance_id)
}

async function loadRefundEligibility() {
  try {
    const res = await paymentAPI.getRefundEligibleProviders()
    refundEligibleProviders.value = new Set(res.data.provider_instance_ids || [])
  } catch { /* ignore — default to hiding refund button */ }
}

onMounted(() => { fetchOrders(); loadRefundEligibility() })
</script>
