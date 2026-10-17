<template>
  <div class="space-y-4">
    <!-- ═══ Terminal States: show result, user clicks to return ═══ -->

    <FMPaymentReview v-if="paymentReviewStatus" :status="paymentReviewStatus" :order-no="displayOrderNumber" />

    <!-- Success -->
    <template v-else-if="outcome === 'success'">
      <div class="card p-6" :class="{ 'mx-auto w-full max-w-2xl': isGateway }">
        <div class="flex flex-col items-center space-y-4 py-4">
          <div class="cp-success-badge flex h-16 w-16 items-center justify-center rounded-full bg-green-100 dark:bg-green-900/30">
            <svg class="h-8 w-8 text-green-500" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
              <path class="cp-success-check" d="M4.5 12.75l6 6 9-13.5" />
            </svg>
          </div>
          <p class="text-lg font-bold text-gray-900 dark:text-white" :class="{ 'break-words text-center': isGateway }">{{ props.orderType === 'subscription' ? t('payment.result.subscriptionSuccess') : t('payment.result.success') }}</p>
          <div v-if="paidOrder" class="w-full rounded-xl bg-gray-50 p-4 dark:bg-dark-800">
            <div class="space-y-2 text-sm">
              <div class="flex justify-between" :class="{ 'items-start gap-4': isGateway }">
                <span class="text-gray-500 dark:text-gray-400" :class="{ 'shrink-0': isGateway }">{{ t('payment.orders.orderId') }}</span>
                <span class="font-medium text-gray-900 dark:text-white" :class="{ 'min-w-0 break-all text-right': isGateway }">#{{ paidOrder.id }}</span>
              </div>
              <div v-if="paidOrder.out_trade_no" class="flex justify-between" :class="{ 'items-start gap-4': isGateway }">
                <span class="text-gray-500 dark:text-gray-400" :class="{ 'shrink-0': isGateway }">{{ t('payment.orders.orderNo') }}</span>
                <span class="font-medium text-gray-900 dark:text-white" :class="{ 'min-w-0 break-all text-right': isGateway }">{{ paidOrder.out_trade_no }}</span>
              </div>
              <div v-if="!(isGateway && paidOrder.order_type === 'subscription')" class="flex justify-between" :class="{ 'items-start gap-4': isGateway }">
                <span class="text-gray-500 dark:text-gray-400" :class="{ 'shrink-0': isGateway }">{{ t('payment.orders.amount') }}</span>
                <span class="font-medium text-gray-900 dark:text-white" :class="{ 'min-w-0 break-all text-right': isGateway }">
                  {{ paidOrder.order_type === 'balance' ? '$' + paidOrder.amount.toFixed(2) : formatGatewayAmount(paidOrder.amount, paidOrder.currency) }}
                  <span v-if="(paidOrder.bonus_amount ?? 0) > 0" class="ml-1 text-xs font-normal text-amber-600 dark:text-amber-400">
                    ({{ t('payment.orders.bonusIncluded', { amount: '$' + (paidOrder.bonus_amount ?? 0).toFixed(2) }) }})
                  </span>
                </span>
              </div>
              <div class="flex justify-between" :class="{ 'items-start gap-4': isGateway }">
                <span class="text-gray-500 dark:text-gray-400" :class="{ 'shrink-0': isGateway }">{{ t('payment.orders.payAmount') }}</span>
                <span class="font-medium text-gray-900 dark:text-white" :class="{ 'min-w-0 break-all text-right': isGateway }">{{ formatGatewayAmount(paidOrder.pay_amount, paidOrder.currency) }}</span>
              </div>
              <CryptoTxReceipt
                v-if="isUsdtBsc || isGateway"
                :tx-hash="paidOrder.crypto_tx_hash"
                :amount="paidOrder.crypto_amount || cryptoAmountText"
                :currency="paidOrder.crypto_currency || cryptoCurrencyText"
                :network="paidOrder.crypto_network || props.cryptoNetwork"
              />
            </div>
          </div>
          <button class="btn btn-primary" @click="handleDone">{{ t('common.confirm') }}</button>
        </div>
      </div>
    </template>

    <CryptoGatewayCheckout
      v-else-if="isGateway"
      :network="gatewayOrder?.crypto_network || props.cryptoNetwork || ''"
      :crypto-amount="gatewayOrder?.crypto_amount || props.cryptoAmount || ''"
      :crypto-rate="gatewayOrder?.crypto_rate || props.cryptoRate"
      :receive-address="gatewayOrder?.receive_address || props.receiveAddress || ''"
      :order-no="gatewayOrder?.out_trade_no || props.outTradeNo || ''"
      :order-amount="displayPaymentAmount"
      :remaining-seconds="remainingSeconds"
      :total-seconds="orderLifetimeSeconds"
      :gateway-status="gatewayOrder?.gateway_status || props.gatewayStatus"
      :create-stage="gatewayOrder?.create_stage || props.createStage"
      :order-status="gatewayOrder?.status"
      :confirmations="gatewayOrder?.confirmations ?? props.confirmations"
      :required-confirmations="gatewayOrder?.required_confirmations ?? props.requiredConfirmations"
      :tx-hash="gatewayOrder?.crypto_tx_hash"
      :checking="gatewayChecking"
      :query-failed="gatewayQueryFailed"
      :cancelling="cancelling"
      :show-support="supportEnabled"
      @check="checkGatewayOrder"
      @cancel="handleCancel"
      @done="handleDone"
    />

    <!-- USDT-BSC 过期 / 取消：宽限期内仍可能入账，继续自动核对一段时间 -->
    <template v-else-if="usdtWatchStarted && (outcome === 'expired' || outcome === 'cancelled')">
      <div class="mx-auto w-full max-w-2xl">
        <div class="card px-4 pb-6 pt-8 text-center sm:px-6">
          <div
            :class="[
              'mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full',
              outcome === 'expired' ? 'bg-amber-50 text-amber-500 dark:bg-amber-500/10' : 'border border-gray-200 bg-gray-50 text-gray-400 dark:border-dark-600 dark:bg-dark-800',
            ]"
          >
            <Icon :name="outcome === 'expired' ? 'clock' : 'x'" size="xl" />
          </div>
          <p class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ outcome === 'expired' ? t('payment.qr.expired') : t('payment.qr.cancelled') }}
          </p>
          <p class="mt-1.5 text-sm text-gray-500 dark:text-gray-400">{{ t('payment.crypto.closedDesc') }}</p>
          <div data-test="usdt-watch" class="my-5 flex items-center gap-3 rounded-xl border border-gray-200 bg-gray-50 px-3.5 py-3 text-left dark:border-dark-600 dark:bg-dark-900/60" role="status" aria-live="polite">
            <span class="relative flex h-2.5 w-2.5 shrink-0" aria-hidden="true">
              <span v-if="watchActive" class="absolute inline-flex h-full w-full animate-ping rounded-full bg-primary-400 opacity-75 motion-reduce:animate-none"></span>
              <span :class="['relative inline-flex h-2.5 w-2.5 rounded-full', watchActive ? 'bg-primary-500' : 'bg-gray-300 dark:bg-dark-500']"></span>
            </span>
            <div class="min-w-0 flex-1">
              <p class="text-sm font-semibold text-gray-900 dark:text-white">
                {{ watchActive ? t('payment.crypto.watching') : t('payment.crypto.watchEnded') }}
              </p>
              <p class="text-xs text-gray-500 dark:text-gray-400">
                {{ watchActive ? t('payment.crypto.watchingHint') : t('payment.crypto.watchEndedHint') }}
              </p>
            </div>
            <span v-if="watchActive" class="font-mono text-sm font-semibold tabular-nums text-gray-600 dark:text-gray-300">{{ watchCountdown }}</span>
          </div>
          <div class="flex gap-2 rounded-xl border border-primary-500/30 bg-primary-50 px-3 py-2.5 text-left text-[13px] leading-relaxed text-primary-700 dark:bg-primary-500/10 dark:text-primary-300">
            <Icon name="infoCircle" size="sm" class="mt-0.5 shrink-0" />
            <p>{{ outcome === 'expired' ? t('payment.crypto.expiredNote') : t('payment.crypto.cancelledNote') }}</p>
          </div>
          <div class="mt-5 flex flex-wrap justify-center gap-2.5">
            <router-link v-if="supportEnabled" to="/support" class="btn btn-secondary">{{ t('common.contactSupport') }}</router-link>
            <button class="btn btn-primary" @click="handleDone">{{ t('payment.crypto.reorder') }}</button>
          </div>
        </div>
      </div>
    </template>

    <!-- Cancelled -->
    <template v-else-if="outcome === 'cancelled'">
      <div class="card p-6">
        <div class="flex flex-col items-center space-y-4 py-4">
          <div class="flex h-16 w-16 items-center justify-center rounded-full bg-gray-100 dark:bg-dark-700">
            <svg class="h-8 w-8 text-gray-400 dark:text-gray-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </div>
          <p class="text-lg font-bold text-gray-900 dark:text-white">{{ t('payment.qr.cancelled') }}</p>
          <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('payment.qr.cancelledDesc') }}</p>
          <button class="btn btn-primary" @click="handleDone">{{ t('common.confirm') }}</button>
        </div>
      </div>
    </template>

    <!-- Expired / Failed -->
    <template v-else-if="outcome === 'expired'">
      <div class="card p-6">
        <div class="flex flex-col items-center space-y-4 py-4">
          <div class="flex h-16 w-16 items-center justify-center rounded-full bg-orange-100 dark:bg-orange-900/30">
            <svg class="h-8 w-8 text-orange-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
          </div>
          <p class="text-lg font-bold text-gray-900 dark:text-white">{{ t('payment.qr.expired') }}</p>
          <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('payment.qr.expiredDesc') }}</p>
          <button class="btn btn-primary" @click="handleDone">{{ t('common.confirm') }}</button>
        </div>
      </div>
    </template>

    <!-- ═══ Active States: QR or Popup waiting ═══ -->

    <!-- Mobile Alipay app handoff. The QR fallback stays hidden until launch timeout. -->
    <template v-else-if="isMobileAlipayDeepLink">
      <template v-if="!deepLinkFallbackVisible">
        <div class="card p-6">
          <div class="flex flex-col items-center space-y-4 py-4 text-center">
            <div
              v-if="deepLinkState === 'launching'"
              class="h-10 w-10 animate-spin rounded-full border-4 border-[#00AEEF] border-t-transparent"
            ></div>
            <div
              v-else
              class="flex h-12 w-12 items-center justify-center rounded-full bg-blue-50 dark:bg-blue-950/30"
            >
              <Icon name="checkCircle" size="lg" class="text-[#00AEEF]" />
            </div>
            <p class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ deepLinkState === 'backgrounded' ? t('payment.qr.alipayContinueInApp') : t('payment.qr.alipayOpening') }}
            </p>
            <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('payment.qr.alipayWaitingHint') }}</p>
            <button
              v-if="deepLinkState === 'backgrounded'"
              data-test="reopen-alipay"
              class="btn btn-alipay inline-flex items-center gap-2 text-sm"
              @click="reopenAlipay"
            >
              <Icon name="externalLink" size="sm" />
              {{ t('payment.qr.reopenAlipay') }}
            </button>
          </div>
        </div>
        <div class="card p-4 text-center">
          <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('payment.qr.expiresIn') }}</p>
          <p class="mt-1 text-2xl font-bold tabular-nums text-gray-900 dark:text-white">{{ countdownDisplay }}</p>
          <p class="mt-1 text-xs text-gray-400 dark:text-gray-500">{{ t('payment.qr.waitingPayment') }}</p>
        </div>
      </template>
      <template v-else>
        <div data-test="alipay-qr-fallback" class="card p-6">
          <div class="flex flex-col items-center space-y-4">
            <div class="text-center">
              <p class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('payment.qr.alipayFallbackTitle') }}</p>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('payment.qr.alipayFallbackHint') }}</p>
            </div>
            <div class="w-full space-y-2 border-y border-gray-100 py-3 text-sm dark:border-dark-600">
              <div class="flex items-start justify-between gap-4">
                <span class="text-gray-500 dark:text-gray-400">{{ t('payment.orders.payAmount') }}</span>
                <span class="font-semibold text-gray-900 dark:text-white">{{ displayPaymentAmount }}</span>
              </div>
              <div class="flex items-start justify-between gap-4">
                <span class="text-gray-500 dark:text-gray-400">{{ t('payment.orders.orderNo') }}</span>
                <span class="max-w-[70%] break-all text-right font-mono text-xs text-gray-900 dark:text-white">
                  {{ displayOrderNumber }}
                </span>
              </div>
              <div class="flex items-start justify-between gap-4">
                <span class="text-gray-500 dark:text-gray-400">{{ t('payment.qr.expiresIn') }}</span>
                <span class="font-semibold tabular-nums text-gray-900 dark:text-white">{{ countdownDisplay }}</span>
              </div>
            </div>
            <div :class="['relative rounded-lg border-2 p-4', qrBorderClass]">
              <canvas ref="qrCanvas" class="mx-auto"></canvas>
              <div class="pointer-events-none absolute inset-0 flex items-center justify-center">
                <span :class="['rounded-full p-2 shadow ring-2 ring-white', qrLogoBgClass]">
                  <img :src="qrLogoIcon" alt="" class="h-5 w-5 brightness-0 invert" />
                </span>
              </div>
            </div>
            <p class="text-center text-sm leading-6 text-gray-600 dark:text-gray-300">
              {{ t('payment.qr.alipaySaveAndScanHint') }}
            </p>
            <div class="grid w-full gap-2 sm:grid-cols-2">
              <button
                data-test="reopen-alipay"
                class="btn btn-alipay inline-flex items-center justify-center gap-2"
                @click="reopenAlipay"
              >
                <Icon name="externalLink" size="sm" />
                {{ t('payment.qr.reopenAlipay') }}
              </button>
              <button
                data-test="save-alipay-qr"
                class="btn btn-secondary inline-flex items-center justify-center gap-2"
                @click="saveQRCode"
              >
                <Icon name="download" size="sm" />
                {{ t('payment.qr.saveQRCode') }}
              </button>
            </div>
            <button class="btn btn-secondary w-full" @click="handleDone">
              {{ t('payment.result.backToRecharge') }}
            </button>
          </div>
        </div>
      </template>
    </template>

    <!-- USDT-BSC checkout -->
    <template v-else-if="showQRCode && isUsdtBsc">
      <UsdtBscCheckout
        :crypto-amount="cryptoAmountText"
        :crypto-currency="cryptoCurrencyText"
        :crypto-rate="props.cryptoRate"
        :order-amount="displayPaymentAmount"
        :receive-address="(props.receiveAddress || qrUrl).trim()"
        :order-no="displayOrderNumber"
        :remaining-seconds="remainingSeconds"
        :total-seconds="orderLifetimeSeconds"
        :cancelling="cancelling"
        :show-support="supportEnabled"
        @cancel="handleCancel"
      />
    </template>

    <!-- Shared checkout for QR and popup payments. -->
    <div v-else class="card cp-payment-checkout mx-auto w-full max-w-[460px] overflow-hidden" data-test="payment-checkout">
      <PaymentCheckoutHeader :title="showQRCode ? scanTitle : t('payment.qr.payInNewWindowHint')" />
      <div class="flex flex-col items-center gap-4 px-4 py-5 sm:px-6">
        <div v-if="props.payAmount != null || props.amount != null" class="text-center">
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.orders.payAmount') }}</p>
          <p class="mt-1 text-3xl font-bold tabular-nums text-gray-900 dark:text-white" data-test="checkout-pay-amount">{{ displayPaymentAmount }}</p>
          <p v-if="props.orderType === 'balance' && props.amount != null" class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            {{ t('payment.orders.amount') }} · {{ formatGatewayAmount(props.amount, 'USD') }}
          </p>
        </div>
        <template v-if="showQRCode">
          <div :class="['cp-checkout-qr relative max-w-full rounded-2xl border-2 bg-white p-3', qrBorderClass]">
            <canvas ref="qrCanvas" class="mx-auto max-w-full rounded-lg"></canvas>
            <div v-if="showQrLogo" class="pointer-events-none absolute inset-0 flex items-center justify-center">
              <span :class="['rounded-full p-2 shadow ring-2 ring-white', qrLogoBgClass]">
                <img :src="qrLogoIcon" alt="" class="h-5 w-5 brightness-0 invert" />
              </span>
            </div>
          </div>
          <p v-if="scanHint" class="text-center text-sm text-gray-500 dark:text-gray-400">{{ scanHint }}</p>
        </template>
        <div v-else class="flex flex-col items-center gap-3 py-3 text-center">
          <div class="h-10 w-10 animate-spin rounded-full border-4 border-primary-500 border-t-transparent motion-reduce:animate-none"></div>
          <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('payment.qr.payInNewWindowHint') }}</p>
        </div>
        <div class="w-full text-center">
          <p class="text-xs font-bold text-gray-400 dark:text-gray-500">{{ t('payment.qr.expiresIn') }}</p>
          <p class="mt-1 text-3xl font-extrabold tabular-nums text-primary-600 dark:text-primary-400">{{ countdownDisplay }}</p>
          <p class="mt-1 flex items-center justify-center gap-1.5 text-xs text-gray-500 dark:text-gray-400" role="status">
            <span class="cp-checkout-pulse h-1.5 w-1.5 rounded-full bg-primary-500" aria-hidden="true"></span>
            {{ t('payment.qr.waitingPayment') }}
          </p>
        </div>
      </div>
      <div class="flex flex-wrap justify-end gap-2 border-t border-gray-100 px-4 py-3 dark:border-dark-700 sm:px-6">
        <button v-if="payUrl" class="btn btn-primary text-sm" @click="reopenPopup">{{ t('payment.qr.openPayWindow') }}</button>
        <button class="btn btn-secondary text-sm" :disabled="cancelling" @click="handleCancel">
          {{ cancelling ? t('common.processing') : t('payment.qr.cancelOrder') }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { usePaymentStore } from '@/stores/payment'
import { useAppStore } from '@/stores'
import { paymentAPI } from '@/api/payment'
import { extractI18nErrorMessage } from '@/utils/apiError'
import {
  getPaymentPopupFeatures,
  isBuiltInAlipayMethod,
  isBuiltInWxpayMethod,
} from '@/components/payment/providerConfig'
import { formatPaymentAmount, normalizePaymentCurrency } from '@/components/payment/currency'
import { FeatureFlags, resolveFeatureFlag } from '@/utils/featureFlags'
import type { PaymentOrder } from '@/types/payment'
import Icon from '@/components/icons/Icon.vue'
import CryptoTxReceipt from './CryptoTxReceipt.vue'
import UsdtBscCheckout from './UsdtBscCheckout.vue'
import CryptoGatewayCheckout from './CryptoGatewayCheckout.vue'
import FMPaymentReview from './FMPaymentReview.vue'
import PaymentCheckoutHeader from './PaymentCheckoutHeader.vue'
import { formatCountdown } from './usdtBsc'
import QRCode from 'qrcode'
import alipayIcon from '@/assets/icons/alipay.svg'
import wxpayIcon from '@/assets/icons/wxpay.svg'
import {
  createAlipayDeepLinkLauncher,
  type AlipayDeepLinkLauncher,
  type AlipayDeepLinkState,
} from './alipayDeepLink'

const props = defineProps<{
  orderId: number
  amount?: number
  payAmount?: number
  qrCode: string
  expiresAt: string
  paymentType: string
  payUrl?: string
  orderType?: string
  currency?: string
  cryptoAmount?: string
  cryptoCurrency?: string
  cryptoNetwork?: string
  cryptoRate?: string
  cryptoRateSource?: string
  receiveAddress?: string
  outTradeNo?: string
  mobileAlipayDeepLink?: boolean
  createStage?: string
  gatewayStatus?: string
  confirmations?: number
  requiredConfirmations?: number
}>()

type PaymentOutcome = 'success' | 'cancelled' | 'expired'

const emit = defineEmits<{ done: []; success: []; settled: [outcome: PaymentOutcome] }>()

const i18n = useI18n()
const { t } = i18n
const paymentStore = usePaymentStore()
const appStore = useAppStore()

const qrCanvas = ref<HTMLCanvasElement | null>(null)
const qrUrl = ref('')
const remainingSeconds = ref(0)
// 订单总时长，等第一次轮询拿到 created_at 再算；0 表示未知
const orderLifetimeSeconds = ref(0)
const cancelling = ref(false)
const paidOrder = ref<PaymentOrder | null>(null)
const gatewayOrder = ref<PaymentOrder | null>(null)
const gatewayChecking = ref(false)
const paymentReviewStatus = ref('')
const gatewayQueryFailed = ref(false)
const isGateway = computed(() => props.paymentType === 'usdt_gateway')
const deepLinkState = ref<AlipayDeepLinkState>('idle')
const deepLinkFallbackVisible = ref(false)
const paymentCurrency = computed(() => normalizePaymentCurrency(props.currency))
const localeCode = computed(() => {
  const raw = i18n.locale as unknown
  if (typeof raw === 'string') return raw
  if (raw && typeof raw === 'object' && 'value' in raw) {
    return String((raw as { value?: string }).value || '')
  }
  return undefined
})

// Terminal outcome: null = still active, 'success' | 'cancelled' | 'expired'
const outcome = ref<PaymentOutcome | null>(null)

let pollTimer: ReturnType<typeof setInterval> | null = null
let countdownTimer: ReturnType<typeof setInterval> | null = null
let verifyAttempts = 0
let lastVerifyAt = 0
let alipayLauncher: AlipayDeepLinkLauncher | null = null
let orderFailed = false

const VERIFY_RETRY_INTERVAL_MS = 15000
const VERIFY_RETRY_MAX_ATTEMPTS = 6
// 用户从复制金额到钱包转出常常要几分钟，固定次数的核单会在转账前就用完，
// 所以 USDT 在页面可见时按固定间隔一直核单。
const USDT_VERIFY_INTERVAL_MS = 20000
// 后端对过期、取消的 USDT 订单还会再扫 5 分钟宽限期，前端多看一次扫描周期（60 秒）
const USDT_WATCH_SECONDS = 6 * 60
const USDT_WATCH_POLL_SECONDS = 15
const USDT_WATCH_VERIFY_INTERVAL_MS = 30000

const isAlipay = computed(() => isBuiltInAlipayMethod(props.paymentType))
const isWxpay = computed(() => isBuiltInWxpayMethod(props.paymentType))
const isUsdtBsc = computed(() => {
  const normalized = props.paymentType.trim().toLowerCase()
  return normalized === 'usdt_bsc' || (normalized.includes('usdt') && normalized.includes('bsc'))
})
const showQrLogo = computed(() => isAlipay.value || isWxpay.value)
const qrLogoIcon = computed(() => isAlipay.value ? alipayIcon : wxpayIcon)
const isMobileAlipayDeepLink = computed(() => props.mobileAlipayDeepLink === true && isAlipay.value && !!qrUrl.value)
const showQRCode = computed(() => !!qrUrl.value && (!isMobileAlipayDeepLink.value || deepLinkFallbackVisible.value))
const cryptoAmountText = computed(() => (props.cryptoAmount || '').trim())
const cryptoCurrencyText = computed(() => (props.cryptoCurrency || 'USDT').trim())
const supportEnabled = computed(() => resolveFeatureFlag(appStore.cachedPublicSettings, FeatureFlags.supportTickets))

const qrBorderClass = computed(() => {
  if (isAlipay.value) return 'border-[#00AEEF] bg-blue-50 dark:border-[#00AEEF]/70 dark:bg-blue-950/20'
  if (isWxpay.value) return 'border-[#2BB741] bg-green-50 dark:border-[#2BB741]/70 dark:bg-green-950/20'
  return 'border-gray-200 bg-white dark:border-dark-600 dark:bg-dark-800'
})

const qrLogoBgClass = computed(() => {
  if (isAlipay.value) return 'bg-[#00AEEF]'
  if (isWxpay.value) return 'bg-[#2BB741]'
  return 'bg-gray-400'
})

const scanTitle = computed(() => {
  if (isAlipay.value) return t('payment.qr.scanAlipay')
  if (isWxpay.value) return t('payment.qr.scanWxpay')
  return t('payment.qr.scanToPay')
})

const scanHint = computed(() => {
  if (isAlipay.value) return t('payment.qr.scanAlipayHint')
  if (isWxpay.value) return t('payment.qr.scanWxpayHint')
  return ''
})

const countdownDisplay = computed(() => {
  const m = Math.floor(remainingSeconds.value / 60)
  const s = remainingSeconds.value % 60
  return m.toString().padStart(2, '0') + ':' + s.toString().padStart(2, '0')
})

const displayPaymentAmount = computed(() => formatGatewayAmount(props.payAmount || props.amount || 0))
const displayOrderNumber = computed(() => props.outTradeNo || `#${props.orderId}`)

function formatGatewayAmount(value: number, currency?: string | null): string {
  return formatPaymentAmount(value, currency || paymentCurrency.value, localeCode.value)
}

function isSuccessStatus(status: string | null | undefined): boolean {
  return status === 'COMPLETED' || status === 'PAID' || status === 'RECHARGING'
}

function reopenPopup() {
  if (props.payUrl) {
    const win = window.open(props.payUrl, 'paymentPopup', getPaymentPopupFeatures())
    if (!win || win.closed) {
      window.location.href = props.payUrl
    }
  }
}

function setOutcome(next: PaymentOutcome) {
  if (outcome.value === next) return
  outcome.value = next
  emit('settled', next)
  if (next !== 'success' && isUsdtBsc.value && !orderFailed) startUsdtWatch(next)
}

function orderLifetimeOf(order: PaymentOrder): number {
  const lifetime = (Date.parse(order.expires_at) - Date.parse(order.created_at)) / 1000
  return Number.isFinite(lifetime) && lifetime > 0 ? Math.floor(lifetime) : 0
}

// ── USDT 过期 / 取消后的自动核对 ──
// 宽限期内到账的订单后端照样入账，前端继续轮询，到账就切到成功页。
const usdtWatchStarted = ref(false)
const usdtWatchRemaining = ref(0)
let usdtWatchTimer: ReturnType<typeof setInterval> | null = null
let usdtWatchTicks = 0
let usdtWatchInFlight = false
let lastWatchVerifyAt = 0

const watchActive = computed(() => usdtWatchRemaining.value > 0)
const watchCountdown = computed(() => formatCountdown(usdtWatchRemaining.value))

function startUsdtWatch(reason: Exclude<PaymentOutcome, 'success'>) {
  stopUsdtWatch()
  usdtWatchStarted.value = true
  // 过期订单的宽限期从原过期时间算起；取消的订单从现在算
  const expiresAt = Date.parse(props.expiresAt)
  usdtWatchRemaining.value = reason === 'expired' && Number.isFinite(expiresAt)
    ? Math.max(0, Math.min(USDT_WATCH_SECONDS, Math.floor((expiresAt - Date.now()) / 1000) + USDT_WATCH_SECONDS))
    : USDT_WATCH_SECONDS
  if (usdtWatchRemaining.value <= 0) return
  usdtWatchTicks = 0
  usdtWatchTimer = setInterval(() => {
    usdtWatchRemaining.value = Math.max(0, usdtWatchRemaining.value - 1)
    usdtWatchTicks++
    if (usdtWatchRemaining.value <= 0) {
      stopUsdtWatch()
      // 最后再看一次，覆盖结束前刚入账的情况
      void pollUsdtWatch(true)
    } else if (usdtWatchTicks % USDT_WATCH_POLL_SECONDS === 0) {
      void pollUsdtWatch()
    }
  }, 1000)
  void pollUsdtWatch()
}

function stopUsdtWatch() {
  if (usdtWatchTimer) { clearInterval(usdtWatchTimer); usdtWatchTimer = null }
}

// 函数而不是直接比较：await 之后 outcome 可能已被主轮询改掉，避免 TS 沿用 await 前的类型收窄
const isPaidOutcome = () => outcome.value === 'success'

async function pollUsdtWatch(final = false) {
  if (!props.orderId || usdtWatchInFlight || isPaidOutcome()) return
  usdtWatchInFlight = true
  try {
    let order = await paymentStore.pollOrderStatus(props.orderId)
    if (!order || (!usdtWatchTimer && !final)) return
    const status = String(order.status || '').trim().toUpperCase()
    const outTradeNo = String(order.out_trade_no || '').trim()
    // 后端只核对 PENDING / EXPIRED；已取消的订单只能等后端定时扫描
    if (outTradeNo && (status === 'PENDING' || status === 'EXPIRED')
      && Date.now() - lastWatchVerifyAt >= USDT_WATCH_VERIFY_INTERVAL_MS) {
      lastWatchVerifyAt = Date.now()
      try {
        order = (await paymentAPI.verifyOrder(outTradeNo)).data ?? order
      } catch {
        // 核单失败不影响继续轮询
      }
    }
    if (isPaidOutcome() || (!usdtWatchTimer && !final)) return
    if (isSuccessStatus(order.status)) {
      stopUsdtWatch()
      usdtWatchRemaining.value = 0
      paidOrder.value = order
      setOutcome('success')
      emit('success')
    }
  } finally {
    usdtWatchInFlight = false
  }
}

async function renderQR() {
  await nextTick()
  if (!showQRCode.value || !qrCanvas.value || !qrUrl.value) return
  await QRCode.toCanvas(qrCanvas.value, qrUrl.value, {
    width: 220, margin: 2,
    errorCorrectionLevel: 'M',
  })
}

function updateDeepLinkState(state: AlipayDeepLinkState) {
  deepLinkState.value = state
  if (state === 'fallback') {
    deepLinkFallbackVisible.value = true
    renderQR()
  } else if (state === 'backgrounded') {
    deepLinkFallbackVisible.value = false
  }
}

function reopenAlipay() {
  alipayLauncher?.launch()
}

function saveQRCode() {
  const canvas = qrCanvas.value
  if (!canvas) return
  const link = document.createElement('a')
  link.href = canvas.toDataURL('image/png')
  link.download = `alipay-${props.outTradeNo || props.orderId}.png`
  document.body.appendChild(link)
  link.click()
  link.remove()
}

async function tryRecoverPendingOrder(order: PaymentOrder): Promise<PaymentOrder> {
  if (order.payment_review_status) return order
  if (!isWxpay.value && !isAlipay.value && !isUsdtBsc.value) return order
  const outTradeNo = String(order.out_trade_no || '').trim()
  if (!outTradeNo) return order
  const normalizedStatus = String(order.status || '').trim().toUpperCase()
  if (normalizedStatus !== 'PENDING') return order
  const now = Date.now()
  if (isUsdtBsc.value) {
    if (document.hidden || now - lastVerifyAt < USDT_VERIFY_INTERVAL_MS) return order
  } else if (verifyAttempts >= VERIFY_RETRY_MAX_ATTEMPTS || now - lastVerifyAt < VERIFY_RETRY_INTERVAL_MS) {
    return order
  }

  lastVerifyAt = now
  verifyAttempts += 1
  try {
    const result = await paymentAPI.verifyOrder(outTradeNo)
    return result.data ?? order
  } catch {
    return order
  }
}

let gatewayDisposed = false
async function checkGatewayOrder() {
  if (!props.orderId || gatewayChecking.value || gatewayDisposed || outcome.value === 'success') return
  gatewayChecking.value = true
  try {
    let order = await paymentStore.pollOrderStatus(props.orderId)
    if (gatewayDisposed) return
    if (!order) {
      gatewayQueryFailed.value = true
      return
    }
    let queryFailed = false
    const now = Date.now()
    if (order.out_trade_no && !order.paid_at && now - lastVerifyAt >= USDT_VERIFY_INTERVAL_MS) {
      lastVerifyAt = now
      try {
        order = (await paymentAPI.verifyOrder(order.out_trade_no)).data || order
      } catch {
        queryFailed = true
      }
    }
    if (gatewayDisposed) return
    gatewayOrder.value = order
    gatewayQueryFailed.value = queryFailed
    orderLifetimeSeconds.value = orderLifetimeOf(order)
    const expiresAt = Date.parse(order.expires_at)
    if (Number.isFinite(expiresAt)) remainingSeconds.value = Math.max(0, Math.floor((expiresAt - Date.now()) / 1000))
    if (order.status === 'COMPLETED') {
      paidOrder.value = order
      cleanup()
      setOutcome('success')
      emit('success')
    }
  } catch {
    if (!gatewayDisposed) gatewayQueryFailed.value = true
  } finally {
    gatewayChecking.value = false
  }
}

let pollInFlight = false
async function pollStatus() {
  if (!props.orderId || outcome.value) return
  // 防重入：接口（含 verifyOrder 二次确认）响应慢于 3 秒轮询间隔时避免并发重叠请求。
  if (pollInFlight) return
  pollInFlight = true
  try {
    let order = await paymentStore.pollOrderStatus(props.orderId)
    if (!order) return
    paymentReviewStatus.value = order.payment_review_status || ''
    // 已进入终态则不再处理迟到的响应。
    if (outcome.value) return
    if (!orderLifetimeSeconds.value) orderLifetimeSeconds.value = orderLifetimeOf(order)
    order = await tryRecoverPendingOrder(order)
    paymentReviewStatus.value = order.payment_review_status || ''
    if (paymentReviewStatus.value) return
    if (outcome.value) return
    if (isSuccessStatus(order.status)) {
      cleanup()
      paidOrder.value = order
      setOutcome('success')
      emit('success')
    } else if (order.status === 'CANCELLED') {
      cleanup()
      setOutcome('cancelled')
    } else if (order.status === 'EXPIRED' || order.status === 'FAILED') {
      cleanup()
      orderFailed = order.status === 'FAILED'
      setOutcome('expired')
    }
  } finally {
    pollInFlight = false
  }
}

function startCountdown(seconds: number) {
  remainingSeconds.value = Math.max(0, seconds)
  const deadline = Date.now() + remainingSeconds.value * 1000
  if (remainingSeconds.value <= 0) { setOutcome('expired'); return }
  countdownTimer = setInterval(() => {
    remainingSeconds.value = Math.max(0, Math.floor((deadline - Date.now()) / 1000))
    if (remainingSeconds.value <= 0 && !paymentReviewStatus.value) { setOutcome('expired'); cleanup() }
  }, 1000)
}

async function handleCancel() {
  if (!props.orderId || cancelling.value) return
  cancelling.value = true
  try {
    await paymentAPI.cancelOrder(props.orderId)
    if (isGateway.value) {
      await checkGatewayOrder()
      return
    }
    cleanup()
    setOutcome('cancelled')
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  } finally {
    cancelling.value = false
  }
}

function handleDone() { cleanup(); stopUsdtWatch(); emit('done') }

function cleanup() {
  if (pollTimer) { clearInterval(pollTimer); pollTimer = null }
  if (countdownTimer) { clearInterval(countdownTimer); countdownTimer = null }
  alipayLauncher?.dispose()
  alipayLauncher = null
}

// Initialize on mount
qrUrl.value = props.qrCode
verifyAttempts = 0
lastVerifyAt = 0
let seconds = 30 * 60
if (props.expiresAt) {
  seconds = Math.floor((new Date(props.expiresAt).getTime() - Date.now()) / 1000)
}
if (isGateway.value) {
  remainingSeconds.value = Math.max(0, seconds)
  countdownTimer = setInterval(() => { remainingSeconds.value = Math.max(0, remainingSeconds.value - 1) }, 1000)
  pollTimer = setInterval(checkGatewayOrder, 5000)
  void checkGatewayOrder()
} else {
  startCountdown(seconds)
  pollTimer = setInterval(pollStatus, 3000)
}
renderQR()

watch([() => qrUrl.value, showQRCode], () => renderQR())
onMounted(() => {
  if (!isMobileAlipayDeepLink.value) return
  alipayLauncher = createAlipayDeepLinkLauncher({
    qrCode: qrUrl.value,
    document,
    lifecycleTarget: window,
    userAgent: window.navigator.userAgent,
    assignLocation: (url) => window.location.assign(url),
    onStateChange: updateDeepLinkState,
  })
  alipayLauncher.launch()
})
onUnmounted(() => { gatewayDisposed = true; cleanup(); stopUsdtWatch() })
</script>

<style scoped>
.cp-payment-checkout { padding: 0; }
.cp-checkout-qr {
  box-shadow: 0 0 24px rgb(13 148 136 / 12%);
}
.cp-checkout-pulse { animation: cp-checkout-pulse 2s ease infinite; }
@keyframes cp-checkout-pulse {
  50% { opacity: 0.35; transform: scale(0.6); }
}
@media (prefers-reduced-motion: reduce) {
  .cp-checkout-pulse { animation: none; }
}

/* ============ 支付成功：圆圈弹入 + 对勾描边画出 ============ */
/* 成功页是用户刚付完钱看到的一屏，动效只做一次、不循环，总时长约 700ms */
.cp-success-badge {
  animation: cp-badge-in 420ms cubic-bezier(0.34, 1.56, 0.64, 1) both;
}

@keyframes cp-badge-in {
  0% {
    opacity: 0;
    transform: scale(0.4);
  }
  60% {
    opacity: 1;
    transform: scale(1.06);
  }
  100% {
    opacity: 1;
    transform: scale(1);
  }
}

/* 用 dasharray 把描边「画」出来：路径长度约 30，取 32 留出余量 */
.cp-success-check {
  stroke-dasharray: 32;
  stroke-dashoffset: 32;
  animation: cp-check-draw 400ms ease-out 260ms both;
}

@keyframes cp-check-draw {
  to {
    stroke-dashoffset: 0;
  }
}

/* 系统开启「减少动态效果」时不做动画，直接呈现最终状态 */
@media (prefers-reduced-motion: reduce) {
  .cp-success-badge,
  .cp-success-check {
    animation: none;
  }

  .cp-success-check {
    stroke-dashoffset: 0;
  }
}
</style>
