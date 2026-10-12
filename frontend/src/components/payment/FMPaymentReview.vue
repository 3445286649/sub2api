<template>
  <div class="card mx-auto w-full max-w-md p-6 text-center" role="status" aria-live="polite" data-test="fm-payment-review">
    <div v-if="status !== 'review'" class="mx-auto mb-4 h-10 w-10 animate-spin rounded-full border-4 border-primary-500 border-t-transparent motion-reduce:animate-none" aria-hidden="true"></div>
    <h3 class="text-lg font-semibold text-gray-900 dark:text-white">
      {{ t(status === 'review' ? 'payment.fmReviewTitle' : status === 'crediting' ? 'payment.fmCreditingTitle' : 'payment.fmVerifyingTitle') }}
    </h3>
    <p class="mt-3 text-sm leading-relaxed text-gray-600 dark:text-gray-300">
      {{ t(status === 'review' ? 'payment.fmReviewRequired' : status === 'crediting' ? 'payment.fmCrediting' : 'payment.fmVerifying') }}
    </p>
    <p class="mt-4 break-all text-sm text-gray-500 dark:text-gray-400">{{ t('payment.orders.orderNo') }}：{{ orderNo }}</p>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
defineProps<{ status: string; orderNo: string }>()
const { t } = useI18n()
</script>
