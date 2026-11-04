<template>
  <BaseDialog :show="show" :title="t('invoice.exclusions.title')" width="wide" mobile-sheet @close="emit('close')">
    <div class="space-y-5" data-testid="invoice-exclusion-dialog">
      <p class="text-sm text-gray-600 dark:text-gray-300">{{ t('invoice.exclusions.description') }}</p>

      <!-- 录入：手输一个或粘贴一批 -->
      <section>
        <label class="input-label" for="inv-exclusion-input">{{ t('invoice.exclusions.inputLabel') }}</label>
        <textarea
          id="inv-exclusion-input"
          v-model="input"
          rows="5"
          class="input font-mono text-[13px]"
          spellcheck="false"
          :placeholder="t('invoice.exclusions.inputPlaceholder')"
          data-testid="invoice-exclusion-input"
        />
        <div class="mt-2 flex flex-wrap items-center justify-between gap-3">
          <p class="input-hint !mt-0">
            {{ parsed.length > 0 ? t('invoice.exclusions.parsedCount', { count: parsed.length }) : t('invoice.exclusions.inputHint') }}
          </p>
          <button
            type="button"
            class="btn btn-primary"
            :disabled="parsed.length === 0 || submitting"
            data-testid="invoice-exclusion-submit"
            @click="submit"
          >
            <span v-if="submitting" class="mr-2 h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent" />
            {{ parsed.length > 0 ? t('invoice.exclusions.submit', { count: parsed.length }) : t('invoice.exclusions.submitIdle') }}
          </button>
        </div>
      </section>

      <!-- 本次结果：每个号的去向都列出来 -->
      <section v-if="result" class="space-y-2 rounded-xl border border-gray-200 p-4 text-sm dark:border-dark-600" data-testid="invoice-exclusion-result">
        <p class="flex items-center gap-1.5 font-medium" :class="result.added.length > 0 ? 'text-emerald-600 dark:text-emerald-400' : 'text-gray-700 dark:text-gray-200'">
          <Icon :name="result.added.length > 0 ? 'checkCircle' : 'infoCircle'" size="sm" />
          {{ result.added.length > 0 ? t('invoice.exclusions.resultAdded', { count: result.added.length }) : t('invoice.exclusions.resultNothingAdded') }}
        </p>
        <div v-for="group in resultGroups" :key="group.key" :data-testid="`invoice-exclusion-${group.key}`">
          <p class="text-xs font-medium" :class="group.tone">{{ group.label }}</p>
          <p class="mt-0.5 break-all font-mono text-xs text-gray-600 dark:text-gray-300">{{ group.numbers.join('、') }}</p>
        </div>
        <p v-if="hasRetryable" class="text-xs text-gray-500 dark:text-gray-400">{{ t('invoice.exclusions.resultRetryHint') }}</p>
      </section>

      <!-- 已排除列表 -->
      <section>
        <div class="mb-3 flex flex-wrap items-center justify-between gap-3">
          <h3 class="text-sm font-semibold text-gray-800 dark:text-gray-200">
            {{ t('invoice.exclusions.listTitle') }}<span class="ml-1.5 font-normal tabular-nums text-gray-400">{{ total }}</span>
          </h3>
          <input v-model="keyword" type="text" class="input w-full sm:w-64" :placeholder="t('invoice.exclusions.searchPlaceholder')" @input="debounceSearch" />
        </div>

        <div v-if="loading && items.length === 0" class="space-y-2">
          <div v-for="i in 3" :key="i" class="h-10 animate-pulse rounded-lg bg-gray-100 dark:bg-dark-700" />
        </div>
        <p v-else-if="items.length === 0" class="rounded-xl border border-dashed border-gray-200 py-8 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400">
          {{ t('invoice.exclusions.empty') }}
        </p>
        <div v-else class="overflow-x-auto rounded-xl border border-gray-100 dark:border-dark-700" :class="{ 'opacity-60': loading }">
          <table class="w-full min-w-[560px] text-sm">
            <thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-900/40 dark:text-gray-400">
              <tr>
                <th class="px-3.5 py-2 text-left font-medium">{{ t('invoice.exclusions.colOrder') }}</th>
                <th class="px-3.5 py-2 text-left font-medium">{{ t('invoice.exclusions.colUser') }}</th>
                <th class="px-3.5 py-2 text-right font-medium">{{ t('invoice.exclusions.colAmount') }}</th>
                <th class="px-3.5 py-2 text-left font-medium">{{ t('invoice.exclusions.colCreatedAt') }}</th>
                <th class="px-3.5 py-2" />
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="item in items" :key="item.id" :data-testid="`invoice-exclusion-row-${item.id}`">
                <td class="px-3.5 py-2 font-mono text-[13px] text-gray-800 dark:text-gray-100">{{ item.out_trade_no || `#${item.order_id}` }}</td>
                <td class="px-3.5 py-2 text-gray-600 dark:text-gray-300">{{ item.user_email }}</td>
                <td class="px-3.5 py-2 text-right tabular-nums text-gray-900 dark:text-white">{{ formatCny(item.pay_amount) }}</td>
                <td class="whitespace-nowrap px-3.5 py-2 text-xs text-gray-500 dark:text-gray-400">{{ formatDateTimeToMinute(item.created_at) }}</td>
                <td class="px-3.5 py-2 text-right">
                  <button
                    type="button"
                    class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-red-600 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-900/20"
                    :data-testid="`invoice-exclusion-remove-${item.id}`"
                    @click="pendingRemoval = item"
                  >
                    <Icon name="trash" size="xs" />{{ t('invoice.exclusions.remove') }}
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <div v-if="pages > 1" class="mt-3 flex items-center justify-center gap-3 text-sm">
          <button type="button" class="btn btn-secondary btn-sm" :disabled="page <= 1 || loading" @click="load(page - 1)">
            <Icon name="chevronLeft" size="sm" />
          </button>
          <span class="tabular-nums text-gray-500 dark:text-gray-400">{{ page }} / {{ pages }}</span>
          <button type="button" class="btn btn-secondary btn-sm" :disabled="page >= pages || loading" @click="load(page + 1)">
            <Icon name="chevronRight" size="sm" />
          </button>
        </div>
      </section>
    </div>

    <template #footer>
      <div class="flex w-full justify-end">
        <button type="button" class="btn btn-secondary" @click="emit('close')">{{ t('common.close') }}</button>
      </div>
    </template>
  </BaseDialog>

  <ConfirmDialog
    :show="pendingRemoval !== null"
    :title="t('invoice.exclusions.removeTitle')"
    :message="t('invoice.exclusions.removeMessage', { order: pendingRemoval?.out_trade_no || `#${pendingRemoval?.order_id}` })"
    :confirm-text="t('invoice.exclusions.remove')"
    danger
    @confirm="confirmRemoval"
    @cancel="pendingRemoval = null"
  />
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { adminInvoiceAPI } from '@/api/admin/invoice'
import { useAppStore } from '@/stores/app'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { formatDateTimeToMinute } from '@/utils/format'
import type { AddInvoiceExclusionsResult, InvoiceExclusion } from '@/types/invoice'
import { formatCny, parseOrderNumbers } from './invoiceFormat'

const props = defineProps<{ show: boolean }>()
const emit = defineEmits<{ (e: 'close'): void }>()
const { t } = useI18n()
const appStore = useAppStore()

// ---- 录入 ----
const input = ref('')
const submitting = ref(false)
const result = ref<AddInvoiceExclusionsResult | null>(null)
const parsed = computed(() => parseOrderNumbers(input.value))

const hasRetryable = computed(() => {
  const r = result.value
  return Boolean(r && (r.not_found.length > 0 || r.ambiguous.length > 0))
})

const resultGroups = computed(() => {
  const r = result.value
  if (!r) return []
  const warn = 'text-amber-600 dark:text-amber-400'
  const muted = 'text-gray-500 dark:text-gray-400'
  return [
    { key: 'not-found', numbers: r.not_found, tone: warn, label: t('invoice.exclusions.notFound', { count: r.not_found.length }) },
    { key: 'ambiguous', numbers: r.ambiguous, tone: warn, label: t('invoice.exclusions.ambiguous', { count: r.ambiguous.length }) },
    { key: 'unpaid', numbers: r.unpaid, tone: muted, label: t('invoice.exclusions.unpaid', { count: r.unpaid.length }) },
    { key: 'already', numbers: r.already_excluded, tone: muted, label: t('invoice.exclusions.alreadyExcluded', { count: r.already_excluded.length }) },
  ].filter((group) => group.numbers.length > 0)
})

async function submit() {
  if (parsed.value.length === 0) return
  submitting.value = true
  try {
    const res = await adminInvoiceAPI.addExclusions(parsed.value)
    result.value = res
    // 只留下需要修改的号，成功的和无需处理的都清掉，避免重复提交
    input.value = [...res.not_found, ...res.ambiguous].join('\n')
    if (res.added.length > 0) {
      keyword.value = ''
      await load(1)
    }
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'invoice.errors', t('invoice.exclusions.submitFailed')))
  } finally {
    submitting.value = false
  }
}

// ---- 已排除列表 ----
const PAGE_SIZE = 10
const items = ref<InvoiceExclusion[]>([])
const loading = ref(false)
const keyword = ref('')
const page = ref(1)
const pages = ref(1)
const total = ref(0)

async function load(target = 1) {
  loading.value = true
  try {
    const res = await adminInvoiceAPI.listExclusions({ page: target, page_size: PAGE_SIZE, keyword: keyword.value.trim() || undefined })
    items.value = res.items
    page.value = res.page
    pages.value = res.pages
    total.value = res.total
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'invoice.errors', t('invoice.exclusions.loadFailed')))
  } finally {
    loading.value = false
  }
}

let searchTimer: ReturnType<typeof setTimeout> | null = null
function debounceSearch() {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(() => void load(1), 300)
}
onBeforeUnmount(() => {
  if (searchTimer) clearTimeout(searchTimer)
})

// ---- 移除 ----
const pendingRemoval = ref<InvoiceExclusion | null>(null)

async function confirmRemoval() {
  const target = pendingRemoval.value
  pendingRemoval.value = null
  if (!target) return
  try {
    await adminInvoiceAPI.removeExclusion(target.id)
    appStore.showSuccess(t('invoice.exclusions.removed'))
    // 删掉当前页最后一条时退回上一页
    await load(items.value.length === 1 && page.value > 1 ? page.value - 1 : page.value)
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'invoice.errors', t('invoice.exclusions.removeFailed')))
  }
}

watch(
  () => props.show,
  (open) => {
    if (!open) return
    input.value = ''
    result.value = null
    keyword.value = ''
    void load(1)
  },
  { immediate: true },
)
</script>
