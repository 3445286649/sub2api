<template>
  <div class="card inv-sticky p-5">
    <h3 class="mb-4 text-sm font-semibold text-gray-800 dark:text-gray-200">{{ t('invoice.summary.title') }}</h3>

    <div v-if="count === 0" class="py-6 text-center">
      <Icon name="document" size="xl" class="mx-auto mb-3 text-gray-300 dark:text-dark-600" />
      <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('invoice.summary.emptyHint') }}</p>
    </div>

    <div v-else class="space-y-2.5 text-sm">
      <div class="flex items-center justify-between">
        <span class="text-gray-500 dark:text-gray-400">{{ t('invoice.summary.selected') }}</span>
        <span class="font-medium tabular-nums text-gray-900 dark:text-white">{{ t('invoice.summary.selectedValue', { count }) }}</span>
      </div>
      <div class="flex items-center justify-between">
        <span class="text-gray-500 dark:text-gray-400">{{ t('invoice.summary.paidTotal') }}</span>
        <span class="font-medium tabular-nums text-gray-900 dark:text-white">{{ formatCny(amount) }}</span>
      </div>
      <div v-if="bonus > 0" class="flex items-center justify-between">
        <span class="text-gray-500 dark:text-gray-400">{{ t('invoice.summary.bonusExcluded') }}</span>
        <span class="tabular-nums text-gray-400 line-through dark:text-dark-400">${{ bonus.toFixed(2) }}</span>
      </div>
      <div class="flex items-end justify-between border-t border-gray-200 pt-3 dark:border-dark-600">
        <span class="font-medium text-gray-700 dark:text-gray-300">{{ t('invoice.summary.amount') }}</span>
        <span class="text-2xl font-bold tabular-nums text-primary-600 dark:text-primary-400">{{ formatCny(animatedAmount) }}</span>
      </div>
      <div v-if="titleText" class="mt-1 rounded-xl bg-gray-50 px-3 py-2.5 text-xs dark:bg-dark-900/40">
        <p class="text-gray-400 dark:text-dark-400">{{ t('invoice.confirm.titleSection') }}</p>
        <p class="mt-0.5 truncate font-medium text-gray-800 dark:text-gray-100">{{ titleText }}</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { toRef } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useCountUp } from '@/composables/useCountUp'
import { formatCny } from './invoiceFormat'

const props = defineProps<{ count: number; amount: number; bonus: number; titleText?: string }>()
const { t } = useI18n()

const { display: animatedAmount } = useCountUp(toRef(props, 'amount'))
</script>

<style scoped>
@media (min-width: 1024px) {
  .inv-sticky {
    position: sticky;
    top: 1.5rem;
  }
}
</style>
