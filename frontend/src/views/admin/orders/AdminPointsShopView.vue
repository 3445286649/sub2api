<template>
  <AppLayout>
    <div class="space-y-4">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div class="inline-flex rounded-lg bg-gray-100 p-1 dark:bg-dark-800">
          <button v-for="tab in tabs" :key="tab.key" type="button" class="rounded-md px-4 py-2 text-sm font-medium"
            :class="activeTab === tab.key ? 'bg-white text-gray-900 shadow-sm dark:bg-dark-700 dark:text-white' : 'text-gray-500 dark:text-dark-400'"
            @click="activeTab = tab.key">{{ tab.label }}</button>
        </div>
        <button v-if="activeTab === 'products'" type="button" class="btn btn-primary" @click="openProduct()">
          <Icon name="plus" size="sm" />
          {{ t('points.createProduct') }}
        </button>
      </div>

      <div v-if="loading" class="flex justify-center py-20"><div class="h-8 w-8 animate-spin rounded-full border-4 border-primary-500 border-t-transparent" /></div>

      <div v-else-if="activeTab === 'products'" class="overflow-hidden rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800">
        <div class="overflow-x-auto">
          <table class="min-w-full divide-y divide-gray-200 text-sm dark:divide-dark-700">
            <thead class="bg-gray-50 text-left text-xs text-gray-500 dark:bg-dark-900/50 dark:text-dark-400"><tr>
              <th class="px-4 py-3">{{ t('points.productName') }}</th><th class="px-4 py-3">{{ t('points.pointsPrice') }}</th>
              <th class="px-4 py-3">{{ t('points.balanceReward') }}</th><th class="px-4 py-3">{{ t('points.stockTotal') }}</th>
              <th class="px-4 py-3">{{ t('points.forSale') }}</th><th class="px-4 py-3 text-right">{{ t('common.actions') }}</th>
            </tr></thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="product in products" :key="product.id">
                <td class="px-4 py-3"><p class="font-medium text-gray-900 dark:text-white">{{ product.name }}</p><p class="max-w-md truncate text-xs text-gray-500">{{ product.description }}</p></td>
                <td class="px-4 py-3 font-semibold text-emerald-600">{{ product.points_price }}</td>
                <td class="px-4 py-3">${{ product.balance_amount.toFixed(2) }}</td>
                <td class="px-4 py-3">{{ product.stock_total == null ? '∞' : `${product.stock_redeemed}/${product.stock_total}` }}</td>
                <td class="px-4 py-3"><span :class="product.for_sale ? 'badge badge-success' : 'badge badge-gray'">{{ product.for_sale ? t('common.enabled') : t('common.disabled') }}</span></td>
                <td class="px-4 py-3"><div class="flex justify-end gap-2"><button class="btn btn-secondary btn-sm" @click="openProduct(product)">{{ t('common.edit') }}</button><button class="btn btn-danger btn-sm" @click="deletingProduct = product">{{ t('common.delete') }}</button></div></td>
              </tr>
              <tr v-if="products.length === 0"><td colspan="6" class="px-4 py-12 text-center text-gray-500">{{ t('points.noProducts') }}</td></tr>
            </tbody>
          </table>
        </div>
      </div>

      <div v-else-if="activeTab === 'orders'" class="overflow-hidden rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800">
        <div class="overflow-x-auto"><table class="min-w-full divide-y divide-gray-200 text-sm dark:divide-dark-700">
          <thead class="bg-gray-50 text-left text-xs text-gray-500 dark:bg-dark-900/50 dark:text-dark-400"><tr><th class="px-4 py-3">{{ t('points.orderNo') }}</th><th class="px-4 py-3">{{ t('points.user') }}</th><th class="px-4 py-3">{{ t('points.productName') }}</th><th class="px-4 py-3">{{ t('points.pointsPrice') }}</th><th class="px-4 py-3">{{ t('points.balanceReward') }}</th><th class="px-4 py-3">{{ t('points.time') }}</th></tr></thead>
          <tbody class="divide-y divide-gray-100 dark:divide-dark-700"><tr v-for="order in orders" :key="order.id"><td class="px-4 py-3 font-mono text-xs">{{ order.order_no }}</td><td class="px-4 py-3">{{ order.user_email }}</td><td class="px-4 py-3">{{ order.product_name }}</td><td class="px-4 py-3">{{ order.points_price }}</td><td class="px-4 py-3">${{ order.balance_amount.toFixed(2) }}</td><td class="px-4 py-3 text-xs">{{ formatTime(order.created_at) }}</td></tr><tr v-if="orders.length === 0"><td colspan="6" class="px-4 py-12 text-center text-gray-500">{{ t('common.noData') }}</td></tr></tbody>
        </table></div>
      </div>

      <form v-else class="max-w-3xl space-y-5 rounded-lg border border-gray-200 bg-white p-6 dark:border-dark-700 dark:bg-dark-800" @submit.prevent="saveRules">
        <div class="flex items-center justify-between"><div><p class="font-medium text-gray-900 dark:text-white">{{ t('points.enabled') }}</p></div><ToggleSwitch :label="t('points.enabled')" :checked="config.enabled" @toggle="config.enabled = !config.enabled" /></div>
        <div class="grid gap-4 sm:grid-cols-2">
          <label class="space-y-1"><span class="input-label">{{ t('points.threshold') }}</span><input v-model.number="config.invite_threshold_amount" class="input" type="number" min="0.01" step="0.01" required /></label>
          <label class="space-y-1"><span class="input-label">{{ t('points.rewardPoints') }}</span><input v-model.number="config.invite_reward_points" class="input" type="number" min="1" step="1" required /></label>
          <label class="space-y-1"><span class="input-label">{{ t('points.windowDays') }}</span><input v-model.number="config.qualification_window_days" class="input" type="number" min="1" max="3650" required /></label>
          <label class="space-y-1"><span class="input-label">{{ t('points.freezeHours') }}</span><input v-model.number="config.freeze_hours" class="input" type="number" min="0" max="8760" required /></label>
        </div>
        <div class="flex justify-end"><button class="btn btn-primary" type="submit" :disabled="saving">{{ t('points.saveRules') }}</button></div>
      </form>
    </div>

    <BaseDialog :show="showProductDialog" :title="editingProduct ? t('points.editProduct') : t('points.createProduct')" width="wide" @close="showProductDialog = false">
      <form id="points-product-form" class="space-y-4" @submit.prevent="saveProduct">
        <div class="grid gap-4 sm:grid-cols-2">
          <label class="space-y-1"><span class="input-label">{{ t('points.productName') }} *</span><input v-model="productForm.name" class="input" maxlength="100" required /></label>
          <label class="space-y-1"><span class="input-label">{{ t('points.pointsPrice') }} *</span><input v-model.number="productForm.points_price" class="input" type="number" min="1" required /></label>
          <label class="space-y-1 sm:col-span-2"><span class="input-label">{{ t('points.description') }}</span><textarea v-model="productForm.description" class="input" rows="2" /></label>
          <label class="space-y-1"><span class="input-label">{{ t('points.originalPointsPrice') }}</span><input v-model.number="productForm.original_points_price" class="input" type="number" min="1" /></label>
          <label class="space-y-1"><span class="input-label">{{ t('points.balanceReward') }} *</span><input v-model.number="productForm.balance_amount" class="input" type="number" min="0.01" step="0.01" required /></label>
          <label class="space-y-1"><span class="input-label">{{ t('points.stockTotal') }}</span><input v-model.number="productForm.stock_total" class="input" type="number" min="0" :placeholder="t('points.unlimitedStock')" /></label>
          <label class="space-y-1"><span class="input-label">{{ t('points.perUserLimit') }}</span><input v-model.number="productForm.per_user_limit" class="input" type="number" min="1" /></label>
          <label class="space-y-1"><span class="input-label">{{ t('points.sortOrder') }}</span><input v-model.number="productForm.sort_order" class="input" type="number" /></label>
          <div class="flex items-end justify-between pb-2"><span class="input-label">{{ t('points.forSale') }}</span><ToggleSwitch :label="t('points.forSale')" :checked="productForm.for_sale" @toggle="productForm.for_sale = !productForm.for_sale" /></div>
          <label class="space-y-1 sm:col-span-2"><span class="input-label">{{ t('points.features') }}</span><textarea v-model="productForm.features" class="input" rows="3" /></label>
        </div>
      </form>
      <template #footer><div class="flex justify-end gap-3"><button class="btn btn-secondary" @click="showProductDialog = false">{{ t('common.cancel') }}</button><button form="points-product-form" type="submit" class="btn btn-primary" :disabled="saving">{{ t('common.save') }}</button></div></template>
    </BaseDialog>
    <ConfirmDialog :show="deletingProduct != null" :title="t('common.delete')" :message="t('points.deleteConfirm')" danger @confirm="deleteProduct" @cancel="deletingProduct = null" />
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import ToggleSwitch from '@/components/payment/ToggleSwitch.vue'
import Icon from '@/components/icons/Icon.vue'
import { adminPointsAPI } from '@/api/admin/points'
import { useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { PointsConfig, PointsProduct, PointsProductInput, PointsShopOrder } from '@/types/points'

const { t } = useI18n()
const appStore = useAppStore()
const tabs = [{ key: 'products', label: t('points.products') }, { key: 'orders', label: t('points.orders') }, { key: 'rules', label: t('points.rules') }] as const
const activeTab = ref<'products' | 'orders' | 'rules'>('products')
const loading = ref(false)
const saving = ref(false)
const products = ref<PointsProduct[]>([])
const orders = ref<PointsShopOrder[]>([])
const config = reactive<PointsConfig>({ enabled: true, invite_threshold_amount: 50, invite_reward_points: 1, qualification_window_days: 30, freeze_hours: 168 })
const showProductDialog = ref(false)
const editingProduct = ref<PointsProduct | null>(null)
const deletingProduct = ref<PointsProduct | null>(null)
const productForm = reactive<PointsProductInput>(emptyProduct())

function emptyProduct(): PointsProductInput { return { product_type: 'balance', name: '', description: '', points_price: 1, original_points_price: null, balance_amount: 1, stock_total: null, per_user_limit: null, features: '', sort_order: 0, for_sale: true } }
function formatTime(value: string) { return new Date(value).toLocaleString() }

async function load() {
  loading.value = true
  try {
    const [productRes, orderRes, configRes] = await Promise.all([adminPointsAPI.getProducts(), adminPointsAPI.getOrders({ page_size: 100 }), adminPointsAPI.getConfig()])
    products.value = productRes.data || []
    orders.value = orderRes.data.items || []
    Object.assign(config, configRes.data)
  } catch (err: unknown) { appStore.showError(extractApiErrorMessage(err, t('common.error'))) }
  finally { loading.value = false }
}

function openProduct(product?: PointsProduct) {
  editingProduct.value = product ?? null
  Object.assign(productForm, product ? { ...product } : emptyProduct())
  showProductDialog.value = true
}

async function saveProduct() {
  saving.value = true
  try {
    const payload = {
      ...productForm,
      original_points_price: optionalNumber(productForm.original_points_price),
      stock_total: optionalNumber(productForm.stock_total),
      per_user_limit: optionalNumber(productForm.per_user_limit),
    }
    if (editingProduct.value) await adminPointsAPI.updateProduct(editingProduct.value.id, payload)
    else await adminPointsAPI.createProduct(payload)
    showProductDialog.value = false
    await load()
    appStore.showSuccess(t('common.saved'))
  } catch (err: unknown) { appStore.showError(extractApiErrorMessage(err, t('common.error'))) }
  finally { saving.value = false }
}

function optionalNumber(value: number | null | undefined): number | null {
  if (value == null || value === ('' as unknown as number)) return null
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : null
}

async function deleteProduct() {
  if (!deletingProduct.value) return
  try { await adminPointsAPI.deleteProduct(deletingProduct.value.id); deletingProduct.value = null; await load() }
  catch (err: unknown) { appStore.showError(extractApiErrorMessage(err, t('common.error'))) }
}

async function saveRules() {
  saving.value = true
  try { const result = await adminPointsAPI.updateConfig({ ...config }); Object.assign(config, result.data); appStore.showSuccess(t('common.saved')) }
  catch (err: unknown) { appStore.showError(extractApiErrorMessage(err, t('common.error'))) }
  finally { saving.value = false }
}

watch(activeTab, tab => { if (tab === 'orders') load().catch(() => {}) })
onMounted(load)
</script>
