<template>
  <label
    class="inv-order group"
    :class="{
      'inv-order-selected': selected,
      'inv-order-disabled': order.already_invoiced,
    }"
    :data-testid="`invoice-order-${order.id}`"
  >
    <input
      type="checkbox"
      class="sr-only"
      :checked="selected"
      :disabled="order.already_invoiced"
      @change="emit('toggle', order.id)"
    />
    <span class="inv-check" aria-hidden="true">
      <Icon v-if="selected" name="check" size="xs" class="text-white" />
    </span>

    <div class="min-w-0 flex-1">
      <div class="flex flex-wrap items-center gap-x-2 gap-y-1">
        <span class="truncate font-mono text-[13px] font-semibold text-gray-800 dark:text-gray-100">{{ orderNumberOf(order) }}</span>
        <span class="rounded-md bg-gray-100 px-1.5 py-0.5 text-[11px] text-gray-500 dark:bg-dark-700 dark:text-gray-400">
          {{ orderTypeLabel }}
        </span>
        <span
          v-if="order.already_invoiced"
          class="rounded-md bg-gray-100 px-1.5 py-0.5 text-[11px] text-gray-500 dark:bg-dark-700 dark:text-gray-400"
        >
          {{ order.invoice_request_id ? t('invoice.orders.claimed', { id: order.invoice_request_id }) : t('invoice.orders.excluded') }}
        </span>
      </div>
      <div class="mt-1 flex flex-wrap items-center gap-x-3 gap-y-0.5 text-xs text-gray-500 dark:text-gray-400">
        <span>{{ formatDateTimeToMinute(order.paid_at) }}</span>
        <span>{{ methodLabel }}</span>
        <span v-if="order.bonus_amount > 0" class="text-gray-400 dark:text-dark-400">
          {{ t('invoice.orders.bonus', { amount: `$${order.bonus_amount.toFixed(2)}` }) }}
        </span>
      </div>
    </div>

    <div class="shrink-0 text-right">
      <p class="text-[11px] text-gray-400 dark:text-dark-400">{{ t('invoice.orders.paid') }}</p>
      <p
        class="text-base font-bold tabular-nums"
        :class="order.already_invoiced ? 'text-gray-400 dark:text-dark-400' : 'text-gray-900 dark:text-white'"
      >
        {{ formatCny(order.invoiceable_amount) }}
      </p>
    </div>
  </label>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { formatDateTimeToMinute } from '@/utils/format'
import type { InvoiceableOrder } from '@/types/invoice'
import { formatCny, orderNumberOf } from './invoiceFormat'

const props = defineProps<{ order: InvoiceableOrder; selected: boolean }>()
const emit = defineEmits<{ (e: 'toggle', id: number): void }>()
const { t, te } = useI18n()

const orderTypeLabel = computed(() => {
  const key = `invoice.orders.type.${props.order.order_type}`
  return te(key) ? t(key) : props.order.order_type
})
const methodLabel = computed(() => {
  const key = `payment.methods.${props.order.payment_type}`
  return te(key) ? t(key) : props.order.payment_type
})
</script>

<style scoped>
.inv-order {
  @apply flex cursor-pointer items-center gap-3 rounded-xl border border-gray-200 bg-white/60 px-4 py-3 transition-all duration-200;
  @apply hover:border-primary-300 hover:bg-white dark:border-dark-700 dark:bg-dark-800/40 dark:hover:border-primary-700/70 dark:hover:bg-dark-800/70;
}
.inv-order-selected {
  @apply border-primary-500 bg-primary-50/60 shadow-sm ring-1 ring-primary-500/30;
  @apply dark:border-primary-500/70 dark:bg-primary-500/10;
}
.inv-order-disabled {
  @apply cursor-not-allowed opacity-60 hover:border-gray-200 hover:bg-white/60 dark:hover:border-dark-700 dark:hover:bg-dark-800/40;
}
.inv-check {
  @apply flex h-5 w-5 shrink-0 items-center justify-center rounded-md border-2 border-gray-300 transition-colors dark:border-dark-500;
}
.inv-order-selected .inv-check {
  @apply border-primary-500 bg-primary-500;
}
.inv-order:focus-within {
  @apply outline-none ring-2 ring-primary-500/40;
}
</style>
