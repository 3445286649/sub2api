<template>
  <div>
    <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
      {{ t('payment.paymentMethod') }}
    </label>
    <div
      data-testid="payment-method-grid"
      class="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4"
    >
      <button
        v-for="method in sortedMethods"
        :key="method.type"
        type="button"
        :title="methodLabel(method)"
        :disabled="!method.available"
        :data-selected="selected === method.type ? 'true' : 'false'"
        :class="[
          'cp-method group relative flex h-[64px] min-w-0 flex-col items-center justify-center rounded-xl border px-3 transition-all duration-200',
          !method.available
            ? 'cursor-not-allowed border-gray-200 bg-gray-50 opacity-50 dark:border-dark-700 dark:bg-dark-800/50'
            : selected === method.type
              ? methodSelectedClass(method.type)
              : 'border-gray-200 bg-white text-gray-700 hover:border-gray-300 hover:bg-gray-50 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-200 dark:hover:border-dark-500 dark:hover:bg-dark-700/60',
        ]"
        @click="method.available && emit('select', method.type)"
      >
        <!-- 选中标记 -->
        <span
          v-if="method.available"
          class="absolute right-1.5 top-1.5 flex h-4 w-4 items-center justify-center rounded-full transition-all duration-200"
          :class="selected === method.type ? 'scale-100 opacity-100' : 'scale-50 opacity-0'"
          :style="{ backgroundColor: methodAccent(method.type) }"
          aria-hidden="true"
        >
          <svg class="h-2.5 w-2.5 text-white" viewBox="0 0 20 20" fill="currentColor">
            <path fill-rule="evenodd" d="M16.7 5.3a1 1 0 010 1.4l-7.5 7.5a1 1 0 01-1.4 0L3.3 9.7a1 1 0 011.4-1.4l3.8 3.8 6.8-6.8a1 1 0 011.4 0z" clip-rule="evenodd" />
          </svg>
        </span>
        <span class="flex w-full min-w-0 items-center justify-center gap-2">
          <img :src="methodIcon(method.type)" :alt="methodLabel(method)" class="h-7 w-7 shrink-0 object-contain" />
          <span class="flex min-w-0 flex-col items-start leading-none">
            <span data-testid="payment-method-label" class="block w-full truncate text-base font-semibold">
              {{ methodLabel(method) }}
            </span>
            <span
              class="text-[10px] tracking-wide text-gray-500 dark:text-dark-400"
            >
              {{ methodSubLabel(method) }}
            </span>
          </span>
        </span>
      </button>
    </div>
    <!-- USDT 选中时说明链和汇率来源，减少选错链的风险 -->
    <p
      v-if="selected === 'usdt_bsc'"
      class="cp-fade-in mt-2 text-xs text-gray-500 dark:text-dark-400"
    >
      {{ t('payment.usdtHint') }}
    </p>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { METHOD_ORDER } from './providerConfig'
import alipayIcon from '@/assets/icons/alipay.svg'
import wxpayIcon from '@/assets/icons/wxpay.svg'
import stripeIcon from '@/assets/icons/stripe.svg'
import airwallexIcon from '@/assets/icons/airwallex.svg'
import usdtIcon from '@/assets/icons/usdt.svg'

export interface PaymentMethodOption {
  type: string
  display_name?: string
  fee_rate: number
  available: boolean
}

const props = defineProps<{
  methods: PaymentMethodOption[]
  selected: string
}>()

const emit = defineEmits<{
  select: [type: string]
}>()

const { t } = useI18n()

const METHOD_ICONS: Record<string, string> = {
  alipay: alipayIcon,
  wxpay: wxpayIcon,
  stripe: stripeIcon,
  airwallex: airwallexIcon,
  usdt_bsc: usdtIcon,
}

const sortedMethods = computed(() => {
  const order: readonly string[] = METHOD_ORDER
  return [...props.methods].sort((a, b) => {
    const ai = order.indexOf(a.type)
    const bi = order.indexOf(b.type)
    return (ai === -1 ? 999 : ai) - (bi === -1 ? 999 : bi)
  })
})

function methodIcon(type: string): string {
  if (type.includes('alipay')) return METHOD_ICONS.alipay
  if (type.includes('wxpay')) return METHOD_ICONS.wxpay
  if (type === 'airwallex') return METHOD_ICONS.airwallex
  return METHOD_ICONS[type] || alipayIcon
}

function methodLabel(method: PaymentMethodOption): string {
  return method.display_name?.trim() || t(`payment.methods.${method.type}`, method.type)
}

// 副标题优先显示费率；没有费率时用 USDT 的链说明占位，其余留空
function methodSubLabel(method: PaymentMethodOption): string {
  if (method.fee_rate > 0) {
    return `${t('payment.fee')} ${method.fee_rate}%`
  }
  if (method.type === 'usdt_bsc') return 'BSC'
  return ''
}

// 勾选标记跟随各渠道品牌色
function methodAccent(type: string): string {
  if (type.includes('alipay')) return '#02A9F1'
  if (type.includes('wxpay')) return '#09BB07'
  if (type === 'stripe') return '#676BE5'
  if (type === 'airwallex') return '#FF6B3D'
  if (type === 'usdt_bsc') return '#10b981'
  return '#0d9488'
}

function methodSelectedClass(type: string): string {
  if (type === 'alipay') return 'border-[#02A9F1] bg-blue-50 text-gray-900 shadow-sm dark:bg-blue-950 dark:text-gray-100'
  if (type === 'wxpay') return 'border-[#09BB07] bg-green-50 text-gray-900 shadow-sm dark:bg-green-950 dark:text-gray-100'
  if (type === 'stripe') return 'border-[#676BE5] bg-indigo-50 text-gray-900 shadow-sm dark:bg-indigo-950 dark:text-gray-100'
  if (type === 'airwallex') return 'border-[#FF6B3D] bg-orange-50 text-gray-900 shadow-sm dark:border-[#FF8E3C] dark:bg-orange-950 dark:text-gray-100'
  if (type === 'usdt_bsc') return 'border-emerald-500 bg-emerald-50 text-gray-900 shadow-sm dark:border-emerald-500 dark:bg-emerald-950 dark:text-gray-100'
  return 'border-primary-500 bg-primary-50 text-gray-900 shadow-sm dark:bg-primary-950 dark:text-gray-100'
}
</script>
