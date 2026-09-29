<template>
  <div class="card cp-sticky-summary p-5">
    <h3 class="mb-4 text-sm font-semibold text-gray-800 dark:text-gray-200">
      {{ t('payment.summary.title') }}
    </h3>

    <!-- 未选金额：给出下一步提示，而不是一行行零 -->
    <div v-if="!hasAmount" class="cp-fade-in py-6 text-center">
      <Icon name="creditCard" size="xl" class="mx-auto mb-3 text-gray-300 dark:text-dark-600" />
      <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('payment.summary.emptyHint') }}</p>
    </div>

    <template v-else>
      <div class="space-y-2.5 text-sm">
        <div class="flex items-center justify-between">
          <span class="text-gray-500 dark:text-gray-400">{{ t('payment.amountLabel') }}</span>
          <span class="font-medium tabular-nums text-gray-900 dark:text-white">
            {{ formatAmount(displayPayAmount) }}
          </span>
        </div>
        <div v-if="feeRate > 0" class="flex items-center justify-between">
          <span class="text-gray-500 dark:text-gray-400">{{ t('payment.fee') }} ({{ feeRate }}%)</span>
          <span class="font-medium tabular-nums text-gray-900 dark:text-white">
            {{ formatAmount(displayFeeAmount) }}
          </span>
        </div>
        <div
          class="flex items-center justify-between border-t border-gray-200 pt-2.5 dark:border-dark-600"
        >
          <span class="font-medium text-gray-700 dark:text-gray-300">{{ t('payment.actualPay') }}</span>
          <span class="text-xl font-bold tabular-nums text-primary-600 dark:text-primary-400">
            {{ formatAmount(displayTotalAmount) }}
          </span>
        </div>
        <div v-if="bonusEnabled" class="flex items-center justify-between pt-0.5">
          <span class="text-gray-500 dark:text-gray-400">{{ t('payment.creditedBalance') }}</span>
          <span class="font-medium tabular-nums text-emerald-600 dark:text-emerald-400">
            ${{ displayCreditedAmount.toFixed(2) }}
          </span>
        </div>
      </div>

      <!-- 赠送说明：命中门槛时高亮一次，让用户注意到多拿的部分 -->
      <!-- key 用上升沿计数：每次跨过门槛重挂载，动画才会重放 -->
      <p
        v-if="bonusEnabled"
        :key="bonusPulse"
        class="cp-bonus-note mt-4 rounded-xl border border-amber-200 bg-amber-50 px-3 py-2 text-xs leading-relaxed text-amber-800 dark:border-amber-900/60 dark:bg-amber-900/20 dark:text-amber-200"
        :class="{ 'cp-bonus-note-active': bonusActive }"
      >
        <span class="font-semibold">{{ t('payment.rechargeBonusBanner', { percent: bonusPercent }) }}</span>
        <span class="mt-0.5 block">
          {{ t('payment.rechargeBonusPreview', {
            pay: formatAmount(amount),
            credit: `$${creditedAmount.toFixed(2)}`,
          }) }}
        </span>
      </p>

      <p
        v-if="creditMultiplier !== 1"
        class="mt-3 text-xs text-gray-500 dark:text-gray-400"
      >
        {{ t('payment.rechargeRatePreview', { currency, usd: creditMultiplier.toFixed(2) }) }}
      </p>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, toRef } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useCountUp } from '@/composables/useCountUp'

const props = withDefaults(defineProps<{
  amount: number
  feeRate: number
  totalAmount: number
  feeAmount: number
  creditedAmount: number
  creditMultiplier: number
  bonusEnabled: boolean
  bonusPercent: string
  bonusActive?: boolean
  /** 上升沿计数：变化时重挂载赠送行，让高亮动画重放 */
  bonusPulse?: number
  currency?: string
  formatAmount: (value: number) => string
}>(), {
  bonusActive: false,
  bonusPulse: 0,
  currency: 'CNY',
})

const { t } = useI18n()

const hasAmount = computed(() => props.amount > 0)

// 金额变化时平滑滚动，避免数字瞬间跳变看不清差异
const { display: displayPayAmount } = useCountUp(toRef(props, 'amount'))
const { display: displayFeeAmount } = useCountUp(toRef(props, 'feeAmount'))
const { display: displayTotalAmount } = useCountUp(toRef(props, 'totalAmount'))
const { display: displayCreditedAmount } = useCountUp(toRef(props, 'creditedAmount'))
</script>

<style scoped>
/* 只有够宽的屏幕才把摘要固定住，窄屏仍旧跟着页面滚动 */
@media (min-width: 1024px) {
  .cp-sticky-summary {
    position: sticky;
    top: 1.5rem;
  }
}

/* 命中赠送门槛时闪烁一次，提醒用户多拿的部分 */
.cp-bonus-note-active {
  animation: cp-bonus-pulse 900ms ease-out 1;
}

@keyframes cp-bonus-pulse {
  0% {
    box-shadow: 0 0 0 0 rgba(245, 158, 11, 0.45);
  }
  70% {
    box-shadow: 0 0 0 6px rgba(245, 158, 11, 0);
  }
  100% {
    box-shadow: 0 0 0 0 rgba(245, 158, 11, 0);
  }
}

@media (prefers-reduced-motion: reduce) {
  .cp-bonus-note-active {
    animation: none;
  }
}
</style>
