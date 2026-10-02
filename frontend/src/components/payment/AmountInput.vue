<template>
  <div class="space-y-5">
    <!-- Quick Amount Buttons -->
    <div>
      <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
        {{ t('payment.quickAmounts') }}
      </label>
      <div class="grid grid-cols-3 gap-x-2.5 gap-y-4">
        <button
          v-for="amt in filteredAmounts"
          :key="amt"
          type="button"
          :data-selected="modelValue === amt ? 'true' : 'false'"
          :class="[
            // 统一最小高度，避免有赠送行的档位比其它档位高出半截、把九宫格撑得参差
            'cp-amt group relative flex min-h-[68px] flex-col items-center justify-center rounded-xl border px-3 py-3 text-center transition-all duration-200',
            modelValue === amt
              ? 'border-primary-500 bg-primary-50 shadow-sm dark:border-primary-400 dark:bg-primary-900/30'
              : 'border-gray-200 bg-white hover:border-primary-300 hover:bg-gray-50 dark:border-dark-600 dark:bg-dark-800 dark:hover:border-primary-700 dark:hover:bg-dark-700/60',
          ]"
          @click="selectAmount(amt)"
        >
          <!-- 选中标记 -->
          <span
            class="cp-amt-check absolute right-1.5 top-1.5 flex h-4 w-4 items-center justify-center rounded-full bg-primary-500 text-white transition-all duration-200"
            :class="modelValue === amt ? 'scale-100 opacity-100' : 'scale-50 opacity-0'"
            aria-hidden="true"
          >
            <svg class="h-2.5 w-2.5" viewBox="0 0 20 20" fill="currentColor">
              <path fill-rule="evenodd" d="M16.7 5.3a1 1 0 010 1.4l-7.5 7.5a1 1 0 01-1.4 0L3.3 9.7a1 1 0 011.4-1.4l3.8 3.8 6.8-6.8a1 1 0 011.4 0z" clip-rule="evenodd" />
            </svg>
          </span>
          <!-- 赠送角标：只在达到门槛的档位上出现。靠左，避免和右上角的选中标记重叠 -->
          <span
            v-if="bonusBadgeText(amt)"
            class="absolute -top-2 left-2 rounded-full bg-gradient-to-r from-amber-400 to-orange-500 px-1.5 py-px text-[10px] font-bold leading-4 text-white shadow-sm"
          >
            {{ bonusBadgeText(amt) }}
          </span>
          <span
            :class="[
              'block text-base font-semibold tabular-nums leading-5 transition-colors',
              modelValue === amt ? 'text-primary-700 dark:text-primary-300' : 'text-gray-800 dark:text-gray-100',
            ]"
          >
            {{ formattedAmount(amt) }}
          </span>
          <!-- 到账行只在该档位真有赠送时出现；没有赠送的档位数字和面值一样，显示出来只是噪音 -->
          <span
            v-if="hasBonus(amt)"
            class="mt-1 block text-xs font-normal tabular-nums text-emerald-600 dark:text-emerald-400"
          >
            {{ props.bonusTiers.length && bonusMode === 'discount' ? t('payment.rechargeBonus.payShort', { amount: formatAmountText(tierQuote(amt).payBase) }) : t('payment.quickAmountCredit', { amount: formatCreditedAmount(creditedAmountFor(amt)) }) }}
          </span>
        </button>
      </div>
    </div>

    <!-- Custom Amount Input -->
    <div>
      <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
        {{ t('payment.customAmount') }}
      </label>
      <div class="relative">
        <span class="absolute left-3.5 top-1/2 -translate-y-1/2 text-sm font-medium text-gray-400 dark:text-dark-400">
          {{ symbol }}
        </span>
        <input
          type="text"
          inputmode="decimal"
          :value="customText"
          :placeholder="placeholderText"
          :class="[
            'input w-full py-3 pl-9 pr-4 text-base tabular-nums',
            outOfRange ? 'input-error' : '',
          ]"
          @input="handleInput"
        />
      </div>
      <!-- 超范围时立即提示，不用等到点按钮 -->
      <p v-if="rangeHint" class="mt-1.5 text-xs text-amber-600 dark:text-amber-400">
        {{ rangeHint }}
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { RechargeBonusTier } from '@/types/payment'
import { quoteRechargeBonus, type RechargeBonusMode } from '@/utils/rechargeBonus'
import { currencySymbol } from './currency'

const props = withDefaults(defineProps<{
  amounts?: number[]
  modelValue: number | null
  min?: number
  max?: number
  bonusTiers?: RechargeBonusTier[]
  bonusMode?: RechargeBonusMode
  multiplier?: number
  bonusEnabled?: boolean
  creditMultiplier?: number
  bonusThreshold?: number
  creditAmountFor?: (amount: number) => number
  formatCreditAmount?: (value: number) => string
  /** 实际支付币种，决定卡片和输入框上的货币符号 */
  currency?: string
  /** 赠送比例（百分比），用来说明角标；不传时由 multiplier 反推 */
  bonusPercent?: number
  /** 按支付币种格式化金额，未提供时退化为纯数字 */
  formatAmount?: (value: number) => string
}>(), {
  amounts: () => [10, 20, 50, 100, 200, 500, 1000, 2000, 5000],
  min: 0,
  max: 0,
  bonusTiers: () => [],
  bonusMode: 'bonus',
  multiplier: 1,
  bonusEnabled: false,
  creditMultiplier: 1,
  bonusThreshold: 0,
  currency: 'CNY',
  bonusPercent: 0,
  creditAmountFor: undefined,
  formatCreditAmount: undefined,
  formatAmount: undefined,
})

const emit = defineEmits<{
  'update:modelValue': [value: number | null]
}>()

const { t } = useI18n()

const customText = ref('')

const symbol = computed(() => currencySymbol(props.currency))

// 0 = no limit
const filteredAmounts = computed(() =>
  props.amounts.filter((a) => (props.min <= 0 || a >= props.min) && (props.max <= 0 || a <= props.max))
)

const placeholderText = computed(() => {
  if (props.min > 0 && props.max > 0) return `${props.min} - ${props.max}`
  if (props.min > 0) return `≥ ${props.min}`
  if (props.max > 0) return `≤ ${props.max}`
  return t('payment.enterAmount')
})

const AMOUNT_PATTERN = /^\d*(\.\d{0,2})?$/

// 百分比：优先用后端给的 bonusPercent，否则从倍率反推
const bonusPercentValue = computed(() => {
  if (props.bonusPercent > 0) return props.bonusPercent
  const derived = Math.round((props.creditMultiplier - 1) * 10000) / 100
  return derived > 0 ? derived : 0
})

// 赠送角标只标在真正达到门槛的档位上
function tierQuote(amt: number) {
  const digits = new Intl.NumberFormat('en', { style: 'currency', currency: props.currency }).resolvedOptions().maximumFractionDigits
  return quoteRechargeBonus(props.bonusTiers, amt, { multiplier: props.multiplier, mode: props.bonusMode, currencyDigits: digits })
}
function bonusBadgeText(amt: number): string {
  if (props.bonusTiers.length) {
    const q = tierQuote(amt)
    return q.percent > 0 ? (props.bonusMode === 'discount' ? `-${q.percent}%` : `+${q.percent}%`) : ''
  }
  if (!props.bonusEnabled || bonusPercentValue.value <= 0) return ''
  if (props.bonusThreshold > 0 && amt < props.bonusThreshold) return ''
  const percent = bonusPercentValue.value
  return `+${Number.isInteger(percent) ? percent : percent.toFixed(2).replace(/\.?0+$/, '')}%`
}

// 该档位是否真有赠送：到账金额必须高于面值，否则那行数字和面值重复，只是噪音
function hasBonus(amt: number): boolean {
  if (props.bonusTiers.length) return tierQuote(amt).percent > 0
  if (!props.bonusEnabled) return false
  return creditedAmountFor(amt) > amt
}

const outOfRange = computed(() => {
  const value = props.modelValue
  if (value === null || value <= 0) return false
  if (props.min > 0 && value < props.min) return true
  if (props.max > 0 && value > props.max) return true
  return false
})

const rangeHint = computed(() => {
  const value = props.modelValue
  if (value === null || value <= 0 || !outOfRange.value) return ''
  if (props.min > 0 && value < props.min) {
    return t('payment.amountBelowMin', { min: formatAmountText(props.min) })
  }
  return t('payment.amountAboveMax', { max: formatAmountText(props.max) })
})

function formatAmountText(value: number): string {
  return props.formatAmount?.(value) ?? `${symbol.value}${value}`
}

function formattedAmount(amt: number): string {
  return props.formatAmount?.(amt) ?? `${symbol.value}${amt}`
}

function creditedAmountFor(amt: number) {
  if (props.bonusTiers.length) return tierQuote(amt).credited
  if (props.creditAmountFor) {
    return props.creditAmountFor(amt)
  }
  return Math.round((amt * props.creditMultiplier) * 100) / 100
}

function formatCreditedAmount(value: number) {
  return props.formatCreditAmount?.(value) ?? value.toFixed(2)
}

function selectAmount(amt: number) {
  customText.value = String(amt)
  emit('update:modelValue', amt)
}

function handleInput(e: Event) {
  const input = e.target as HTMLInputElement
  const val = input.value
  if (!AMOUNT_PATTERN.test(val)) {
    input.value = customText.value
    return
  }
  customText.value = val
  if (val === '') {
    emit('update:modelValue', null)
    return
  }
  const num = parseFloat(val)
  if (!isNaN(num) && num > 0) {
    emit('update:modelValue', num)
  } else {
    emit('update:modelValue', null)
  }
}

watch(() => props.modelValue, (v) => {
  if (v !== null && String(v) !== customText.value) {
    customText.value = String(v)
  }
}, { immediate: true })
</script>
