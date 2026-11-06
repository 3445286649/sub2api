<template>
  <div class="space-y-6" data-testid="invoice-panel">
    <!-- 头图：可开票金额 + 申请/记录切换 -->
    <div class="inv-hero card relative overflow-hidden px-5 py-5 sm:px-6">
      <div class="inv-hero-glow" aria-hidden="true" />
      <div class="relative flex flex-wrap items-end justify-between gap-4">
        <div class="min-w-0">
          <p class="flex items-center gap-1.5 text-xs font-medium text-gray-500 dark:text-gray-400">
            <Icon name="document" size="xs" />{{ t('invoice.hero.invoiceable') }}
          </p>
          <p class="mt-1 text-3xl font-bold tabular-nums tracking-tight text-gray-900 dark:text-white">
            <span v-if="loadingOrders && !ordersLoaded" class="inline-block h-8 w-36 animate-pulse rounded-lg bg-gray-200 align-middle dark:bg-dark-700" />
            <template v-else>{{ formatCny(summary.total_invoiceable_amount) }}</template>
          </p>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('invoice.hero.orderCount', { count: summary.invoiceable_count }) }}</p>
        </div>
        <div class="flex rounded-lg bg-gray-100 p-0.5 dark:bg-dark-900/60" role="tablist">
          <button
            v-for="v in (['apply', 'records'] as const)"
            :key="v"
            type="button"
            role="tab"
            :aria-selected="view === v"
            class="rounded-md px-3.5 py-1.5 text-sm font-medium transition-all"
            :class="view === v ? 'bg-white text-gray-900 shadow-sm dark:bg-dark-700 dark:text-white' : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200'"
            :data-testid="`invoice-view-${v}`"
            @click="switchView(v)"
          >{{ t(`invoice.views.${v}`) }}</button>
        </div>
      </div>
      <p class="relative mt-4 flex items-start gap-1.5 border-t border-gray-100 pt-3 text-xs leading-relaxed text-gray-500 dark:border-dark-700/60 dark:text-gray-400">
        <Icon name="infoCircle" size="xs" class="mt-0.5 shrink-0" />{{ t('invoice.hero.rules') }}
      </p>
    </div>

    <!-- 申请开票 -->
    <template v-if="view === 'apply'">
      <!-- 提交成功 -->
      <div v-if="submitted" class="card px-6 py-12 text-center" data-testid="invoice-success">
        <span class="inv-success-ring mx-auto flex h-14 w-14 items-center justify-center rounded-full bg-emerald-500 text-white">
          <Icon name="check" size="lg" />
        </span>
        <h3 class="mt-5 text-lg font-semibold text-gray-900 dark:text-white">{{ t('invoice.success.title') }}</h3>
        <p class="mx-auto mt-2 max-w-sm text-sm text-gray-500 dark:text-gray-400">
          {{ t('invoice.success.description', { id: submitted.id, email: submitted.receiver_email }) }}
        </p>
        <p class="mt-4 text-2xl font-bold tabular-nums text-gray-900 dark:text-white">{{ formatCny(submitted.total_amount) }}</p>
        <div class="mt-6 flex flex-wrap justify-center gap-2">
          <button type="button" class="btn btn-secondary" @click="resetFlow">{{ t('invoice.success.applyMore') }}</button>
          <button type="button" class="btn btn-primary" @click="openRecord(submitted.id, true)">{{ t('invoice.success.viewRecords') }}</button>
        </div>
      </div>

      <template v-else>
        <div class="card px-5 py-3.5">
          <InvoiceSteps :current="step" @go="step = $event" />
        </div>

        <div class="inv-cols">
          <div class="min-w-0 space-y-6">
            <!-- 第 1 步：选择订单 -->
            <div v-if="step === 0" class="card p-5">
              <div class="mb-4 flex flex-wrap items-center justify-between gap-2">
                <h3 class="text-sm font-semibold text-gray-800 dark:text-gray-200">{{ t('invoice.orders.heading') }}</h3>
                <label v-if="openOrders.length > 0" class="flex cursor-pointer select-none items-center gap-2 text-xs text-gray-600 dark:text-gray-300">
                  <input type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500 dark:border-dark-500 dark:bg-dark-700"
                    :checked="allSelected" :indeterminate.prop="selectedIds.size > 0 && !allSelected" @change="toggleAll" />
                  {{ t('invoice.orders.selectAll') }}
                </label>
              </div>

              <div v-if="loadingOrders && !ordersLoaded" class="space-y-2.5">
                <div v-for="i in 4" :key="i" class="h-[66px] animate-pulse rounded-xl bg-gray-100 dark:bg-dark-700/60" />
              </div>
              <div v-else-if="orders.length === 0" class="py-12 text-center">
                <span class="mx-auto mb-3 flex h-12 w-12 items-center justify-center rounded-2xl bg-gray-100 dark:bg-dark-700">
                  <Icon name="document" size="lg" class="text-gray-400 dark:text-dark-400" />
                </span>
                <p class="text-sm font-medium text-gray-700 dark:text-gray-200">{{ t('invoice.orders.empty') }}</p>
                <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('invoice.orders.emptyHint') }}</p>
                <button type="button" class="btn btn-primary btn-sm mt-4" @click="emit('go-recharge')">{{ t('invoice.orders.goRecharge') }}</button>
              </div>
              <div v-else class="space-y-2.5">
                <InvoiceOrderItem v-for="o in orders" :key="o.id" :order="o" :selected="selectedIds.has(o.id)" @toggle="toggleOrder" />
              </div>
            </div>

            <!-- 第 2 步：抬头 -->
            <div v-else-if="step === 1" class="card p-5">
              <h3 class="mb-4 text-sm font-semibold text-gray-800 dark:text-gray-200">{{ t('invoice.title.heading') }}</h3>
              <InvoiceTitleForm v-model="form" :errors="fieldErrors" :show-all-errors="showAllErrors" :categories="categories" />
            </div>

            <!-- 第 3 步：确认 -->
            <div v-else class="card space-y-5 p-5">
              <h3 class="text-sm font-semibold text-gray-800 dark:text-gray-200">{{ t('invoice.confirm.heading') }}</h3>
              <section>
                <p class="mb-2 text-xs font-medium uppercase tracking-wide text-gray-400 dark:text-dark-400">{{ t('invoice.confirm.titleSection') }}</p>
                <dl class="space-y-2 rounded-xl bg-gray-50 px-4 py-3.5 text-sm dark:bg-dark-900/40">
                  <div v-for="row in confirmRows" :key="row.label" class="flex justify-between gap-4">
                    <dt class="shrink-0 text-gray-500 dark:text-gray-400">{{ row.label }}</dt>
                    <dd class="min-w-0 break-all text-right font-medium text-gray-900 dark:text-gray-100" :class="{ 'font-mono text-[13px]': row.mono }">{{ row.value }}</dd>
                  </div>
                </dl>
              </section>
              <section>
                <p class="mb-2 text-xs font-medium uppercase tracking-wide text-gray-400 dark:text-dark-400">
                  {{ t('invoice.confirm.ordersSection') }} · {{ selectedOrders.length }}
                </p>
                <ul class="divide-y divide-gray-100 rounded-xl border border-gray-100 dark:divide-dark-700 dark:border-dark-700">
                  <li v-for="o in selectedOrders" :key="o.id" class="flex items-center justify-between gap-3 px-4 py-2.5 text-sm">
                    <span class="truncate font-mono text-[13px] text-gray-700 dark:text-gray-200">{{ orderNumberOf(o) }}</span>
                    <span class="shrink-0 font-medium tabular-nums text-gray-900 dark:text-white">{{ formatCny(o.invoiceable_amount) }}</span>
                  </li>
                </ul>
              </section>
              <label class="flex cursor-pointer select-none items-start gap-2.5 rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-200">
                <input v-model="agreed" type="checkbox" class="mt-0.5 h-4 w-4 shrink-0 rounded border-amber-300 text-primary-600 focus:ring-primary-500" data-testid="invoice-agree" />
                {{ t('invoice.confirm.agree') }}
              </label>
            </div>
          </div>

          <!-- 右侧摘要 + 操作 -->
          <div class="space-y-3 lg:sticky lg:top-6">
            <InvoiceSummaryCard :count="selectedOrders.length" :amount="selectedAmount" :bonus="selectedBonus" :title-text="step > 0 ? titleText : ''" />
            <button
              type="button"
              class="btn btn-primary w-full py-3 text-base"
              :disabled="!canProceed || submitting"
              data-testid="invoice-next"
              @click="proceed"
            >
              <span v-if="submitting" class="mr-2 h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent" />
              {{ primaryLabel }}
            </button>
            <button v-if="step > 0" type="button" class="btn btn-ghost w-full" :disabled="submitting" @click="step -= 1">
              <Icon name="arrowLeft" size="sm" class="mr-1.5" />{{ t('invoice.summary.back') }}
            </button>
          </div>
        </div>
      </template>
    </template>

    <!-- 开票记录 -->
    <InvoiceRecordList
      v-else
      v-model:status="recordStatus"
      :items="records"
      :loading="loadingRecords"
      :page="recordPage"
      :pages="recordPages"
      @open="openRecord"
      @page="loadRecords"
    />

    <InvoiceDetailDialog :show="detailOpen" :loading="loadingDetail" :request="detail" @close="detailOpen = false" @reapply="reapply" />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { invoiceAPI } from '@/api/invoice'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { extractI18nErrorMessage } from '@/utils/apiError'
import type { InvoiceableOrder, InvoiceableSummary, InvoiceCategoryOption, InvoiceRequest, InvoiceRequestStatus, InvoiceTitleForm as TitleForm } from '@/types/invoice'
import InvoiceDetailDialog from './InvoiceDetailDialog.vue'
import InvoiceOrderItem from './InvoiceOrderItem.vue'
import InvoiceRecordList from './InvoiceRecordList.vue'
import InvoiceSteps from './InvoiceSteps.vue'
import InvoiceSummaryCard from './InvoiceSummaryCard.vue'
import InvoiceTitleForm from './InvoiceTitleForm.vue'
import { emptyInvoiceTitleForm, formatCny, maskMiddle, orderNumberOf, sumAmounts, validateInvoiceTitle } from './invoiceFormat'

const emit = defineEmits<{ (e: 'go-recharge'): void }>()
const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()

const view = ref<'apply' | 'records'>('apply')
const step = ref(0)

// ---- 可开票订单 ----
const orders = ref<InvoiceableOrder[]>([])
const summary = reactive<InvoiceableSummary>({ total_invoiceable_amount: 0, invoiceable_count: 0 })
const loadingOrders = ref(false)
const ordersLoaded = ref(false)
const selectedIds = ref(new Set<number>())

const openOrders = computed(() => orders.value.filter((o) => !o.already_invoiced))
const selectedOrders = computed(() => orders.value.filter((o) => selectedIds.value.has(o.id)))
const selectedAmount = computed(() => sumAmounts(selectedOrders.value.map((o) => o.invoiceable_amount)))
const selectedBonus = computed(() => sumAmounts(selectedOrders.value.map((o) => o.bonus_amount)))
const allSelected = computed(() => openOrders.value.length > 0 && openOrders.value.every((o) => selectedIds.value.has(o.id)))

function toggleOrder(id: number) {
  const next = new Set(selectedIds.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  selectedIds.value = next
}

function toggleAll() {
  selectedIds.value = allSelected.value ? new Set() : new Set(openOrders.value.map((o) => o.id))
}

async function loadOrders() {
  loadingOrders.value = true
  try {
    const res = await invoiceAPI.getInvoiceableOrders()
    orders.value = res.items
    Object.assign(summary, res.summary)
    // 刷新后去掉已失效的勾选
    const valid = new Set(res.items.filter((o) => !o.already_invoiced).map((o) => o.id))
    selectedIds.value = new Set([...selectedIds.value].filter((id) => valid.has(id)))
    ordersLoaded.value = true
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'invoice.errors', t('invoice.errors.loadFailed')))
  } finally {
    loadingOrders.value = false
  }
}

// ---- 抬头 ----
const form = ref<TitleForm>(emptyInvoiceTitleForm(authStore.user?.email ?? ''))

// ---- 开票类目 ----
// 管理员没配置任何启用类目时为空数组，此时不显示类目选择，提交也不带类目
const categories = ref<InvoiceCategoryOption[]>([])

async function loadCategories() {
  try {
    categories.value = await invoiceAPI.getCategories()
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'invoice.errors', t('invoice.errors.loadFailed')))
    return
  }
  // 已选的类目被管理员停用后要重新选；只剩一个类目时直接选中
  const current = form.value.category_id
  if (current !== null && !categories.value.some((c) => c.id === current)) form.value.category_id = null
  if (form.value.category_id === null && categories.value.length === 1) form.value.category_id = categories.value[0].id
}

const showAllErrors = ref(false)
const fieldErrors = computed(() => validateInvoiceTitle(form.value, categories.value.length > 0))
const titleValid = computed(() => Object.keys(fieldErrors.value).length === 0)
const titleText = computed(() =>
  (form.value.invoice_type === 'enterprise' ? form.value.enterprise_name : form.value.individual_name).trim(),
)

const confirmRows = computed(() => {
  const f = form.value
  const rows: { label: string; value: string; mono?: boolean }[] = [
    { label: t('invoice.confirm.type'), value: t(`invoice.title.${f.invoice_type}`) },
  ]
  const chosen = categories.value.find((c) => c.id === f.category_id)
  if (chosen) rows.push({ label: t('invoice.category.label'), value: chosen.name })
  if (f.invoice_type === 'enterprise') {
    rows.push({ label: t('invoice.title.enterpriseName'), value: f.enterprise_name.trim() })
    rows.push({ label: t('invoice.title.taxNumber'), value: f.enterprise_tax_number.trim().toUpperCase(), mono: true })
    if (f.enterprise_address.trim()) rows.push({ label: t('invoice.title.address'), value: f.enterprise_address.trim() })
    if (f.enterprise_phone.trim()) rows.push({ label: t('invoice.title.phone'), value: f.enterprise_phone.trim() })
    if (f.enterprise_bank.trim()) rows.push({ label: t('invoice.title.bank'), value: f.enterprise_bank.trim() })
    if (f.enterprise_bank_account.trim()) rows.push({ label: t('invoice.title.bankAccount'), value: f.enterprise_bank_account.replace(/\s/g, ''), mono: true })
  } else {
    rows.push({ label: t('invoice.title.individualName'), value: f.individual_name.trim() })
    if (f.individual_id_number.trim()) rows.push({ label: t('invoice.title.idNumber'), value: maskMiddle(f.individual_id_number.trim().toUpperCase()), mono: true })
  }
  rows.push({ label: t('invoice.title.receiverEmail'), value: f.receiver_email.trim() })
  return rows
})

// ---- 步骤推进 ----
const agreed = ref(false)
const submitting = ref(false)
const submitted = ref<InvoiceRequest | null>(null)

const canProceed = computed(() => {
  if (selectedOrders.value.length === 0) return false
  if (step.value === 2) return agreed.value
  return true
})

const primaryLabel = computed(() => {
  if (submitting.value) return t('invoice.summary.submitting')
  if (step.value === 0) return t('invoice.summary.toTitle')
  if (step.value === 1) return t('invoice.summary.toConfirm')
  return t('invoice.summary.submit')
})

function proceed() {
  if (step.value === 0) {
    step.value = 1
  } else if (step.value === 1) {
    if (!titleValid.value) {
      showAllErrors.value = true
      return
    }
    agreed.value = false
    step.value = 2
  } else {
    void submit()
  }
}

async function submit() {
  if (selectedOrders.value.length === 0) {
    appStore.showError(t('invoice.errors.noOrders'))
    return
  }
  const f = form.value
  const enterprise = f.invoice_type === 'enterprise'
  submitting.value = true
  try {
    submitted.value = await invoiceAPI.createRequest({
      invoice_type: f.invoice_type,
      order_ids: selectedOrders.value.map((o) => o.id),
      receiver_email: f.receiver_email.trim(),
      category_id: f.category_id ?? undefined,
      ...(enterprise
        ? {
            enterprise_name: f.enterprise_name.trim(),
            enterprise_tax_number: f.enterprise_tax_number.trim().toUpperCase(),
            enterprise_address: f.enterprise_address.trim(),
            enterprise_phone: f.enterprise_phone.trim(),
            enterprise_bank: f.enterprise_bank.trim(),
            enterprise_bank_account: f.enterprise_bank_account.replace(/\s/g, ''),
          }
        : {
            individual_name: f.individual_name.trim(),
            individual_id_number: f.individual_id_number.trim().toUpperCase(),
          }),
    })
    selectedIds.value = new Set()
    void loadOrders()
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'invoice.errors', t('invoice.errors.submitFailed')))
    void loadOrders()
    void loadCategories()
  } finally {
    submitting.value = false
  }
}

function resetFlow() {
  submitted.value = null
  step.value = 0
  agreed.value = false
  showAllErrors.value = false
}

// ---- 开票记录 ----
const records = ref<InvoiceRequest[]>([])
const recordStatus = ref<InvoiceRequestStatus | ''>('')
const recordPage = ref(1)
const recordPages = ref(1)
const loadingRecords = ref(false)

async function loadRecords(page = 1) {
  loadingRecords.value = true
  try {
    const res = await invoiceAPI.listRequests({ page, page_size: 10, status: recordStatus.value || undefined })
    records.value = res.items
    recordPage.value = res.page
    recordPages.value = res.pages
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'invoice.errors', t('invoice.errors.loadFailed')))
  } finally {
    loadingRecords.value = false
  }
}

watch(recordStatus, () => loadRecords(1))

function switchView(v: 'apply' | 'records') {
  view.value = v
  if (v === 'records') void loadRecords(1)
  else if (submitted.value) resetFlow()
}

// ---- 详情 ----
const detailOpen = ref(false)
const loadingDetail = ref(false)
const detail = ref<InvoiceRequest | null>(null)

async function openRecord(id: number, fromSuccess = false) {
  if (fromSuccess) {
    resetFlow()
    view.value = 'records'
    void loadRecords(1)
  }
  detail.value = null
  detailOpen.value = true
  loadingDetail.value = true
  try {
    detail.value = await invoiceAPI.getRequest(id)
  } catch (err) {
    detailOpen.value = false
    appStore.showError(extractI18nErrorMessage(err, t, 'invoice.errors', t('invoice.errors.loadFailed')))
  } finally {
    loadingDetail.value = false
  }
}

/** 驳回后重新申请：带上原抬头，订单需要重新勾选 */
function reapply(req: InvoiceRequest) {
  detailOpen.value = false
  const next = emptyInvoiceTitleForm(req.receiver_email)
  next.invoice_type = req.invoice_type
  // 带上原类目；该类目已停用或被删除时清空，让用户重新选
  const prior = req.category_id ?? null
  next.category_id = prior !== null && categories.value.some((c) => c.id === prior) ? prior : null
  if (next.category_id === null && categories.value.length === 1) next.category_id = categories.value[0].id
  if (req.invoice_type === 'enterprise') {
    next.enterprise_name = req.title
    next.enterprise_tax_number = req.enterprise_tax_number ?? ''
    next.enterprise_address = req.enterprise_address ?? ''
    next.enterprise_phone = req.enterprise_phone ?? ''
    next.enterprise_bank = req.enterprise_bank ?? ''
    next.enterprise_bank_account = req.enterprise_bank_account ?? ''
  } else {
    next.individual_name = req.title
  }
  form.value = next
  resetFlow()
  view.value = 'apply'
}

onMounted(() => {
  void loadOrders()
  void loadCategories()
})
</script>

<style scoped>
.inv-cols {
  display: grid;
  gap: 1.5rem;
  align-items: start;
}

@media (min-width: 1024px) {
  .inv-cols {
    grid-template-columns: minmax(0, 1fr) 20rem;
  }
}

.inv-hero-glow {
  position: absolute;
  inset: -40% -10% auto auto;
  width: 22rem;
  height: 22rem;
  border-radius: 9999px;
  background: radial-gradient(circle, rgba(20, 184, 166, 0.16), transparent 65%);
  pointer-events: none;
}

.inv-success-ring {
  box-shadow: 0 0 0 8px rgba(16, 185, 129, 0.12);
  animation: inv-pop 0.35s ease-out;
}

@keyframes inv-pop {
  from {
    transform: scale(0.6);
    opacity: 0;
  }
  to {
    transform: scale(1);
    opacity: 1;
  }
}

@media (prefers-reduced-motion: reduce) {
  .inv-success-ring {
    animation: none;
  }
}
</style>
