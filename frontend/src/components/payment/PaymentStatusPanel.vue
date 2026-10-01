<template>
  <div class="space-y-4">
    <!-- ═══ Terminal States: show result, user clicks to return ═══ -->

    <!-- Success -->
    <template v-if="outcome === 'success'">
      <div class="card p-6">
        <div class="flex flex-col items-center space-y-4 py-4">
          <div class="cp-success-badge flex h-16 w-16 items-center justify-center rounded-full bg-green-100 dark:bg-green-900/30">
            <svg class="h-8 w-8 text-green-500" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
              <path class="cp-success-check" d="M4.5 12.75l6 6 9-13.5" />
            </svg>
          </div>
          <p class="text-lg font-bold text-gray-900 dark:text-white">{{ props.orderType === 'subscription' ? t('payment.result.subscriptionSuccess') : t('payment.result.success') }}</p>
          <div v-if="paidOrder" class="w-full rounded-xl bg-gray-50 p-4 dark:bg-dark-800">
            <div class="space-y-2 text-sm">
              <div class="flex justify-between">
                <span class="text-gray-500 dark:text-gray-400">{{ t('payment.orders.orderId') }}</span>
                <span class="font-medium text-gray-900 dark:text-white">#{{ paidOrder.id }}</span>
              </div>
              <div v-if="paidOrder.out_trade_no" class="flex justify-between">
                <span class="text-gray-500 dark:text-gray-400">{{ t('payment.orders.orderNo') }}</span>
                <span class="font-medium text-gray-900 dark:text-white">{{ paidOrder.out_trade_no }}</span>
              </div>
              <div class="flex justify-between">
                <span class="text-gray-500 dark:text-gray-400">{{ t('payment.orders.amount') }}</span>
                <span class="font-medium text-gray-900 dark:text-white">{{ paidOrder.order_type === 'balance' ? '$' + paidOrder.amount.toFixed(2) : formatGatewayAmount(paidOrder.amount, paidOrder.currency) }}</span>
              </div>
              <div class="flex justify-between">
                <span class="text-gray-500 dark:text-gray-400">{{ t('payment.orders.payAmount') }}</span>
                <span class="font-medium text-gray-900 dark:text-white">{{ formatGatewayAmount(paidOrder.pay_amount, paidOrder.currency) }}</span>
              </div>
            </div>
          </div>
          <button class="btn btn-primary" @click="handleDone">{{ t('common.confirm') }}</button>
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

    <!-- QR Code Mode -->
    <template v-else-if="showQRCode">
      <div class="card p-6">
        <div class="flex flex-col items-center space-y-4">
          <p class="text-lg font-semibold text-gray-900 dark:text-white">{{ scanTitle }}</p>
          <div :class="['relative rounded-lg border-2 p-4', qrBorderClass, isUsdtBsc ? 'usdt-qr-container' : '']">
            <canvas ref="qrCanvas" class="mx-auto" :class="isUsdtBsc ? 'usdt-qr-canvas' : ''"></canvas>
            <!-- Brand logo overlay -->
            <div v-if="showQrLogo" class="pointer-events-none absolute inset-0 flex items-center justify-center">
              <span :class="['rounded-full p-2 shadow ring-2 ring-white', qrLogoBgClass]">
                <img :src="qrLogoIcon" alt="" class="h-5 w-5 brightness-0 invert" />
              </span>
            </div>
          </div>
          <!-- USDT Payment Details -->
          <div v-if="isUsdtBsc" class="usdt-panel w-full space-y-4">
            <!-- Amount Section with CNY prominent -->
            <div class="usdt-amount-card">
              <div class="flex items-start justify-between gap-4">
                <div class="flex-1">
                  <!-- CNY amount prominent -->
                  <p class="usdt-cny-amount">¥ {{ props.amount?.toFixed(2) || '0.00' }}</p>
                  <!-- USDT amount secondary -->
                  <p v-if="cryptoPayAmountDisplay" class="usdt-amount-value">{{ cryptoPayAmountDisplay }}</p>
                  <!-- Rate hint tertiary -->
                  <p v-if="cryptoRateDisplay" class="usdt-rate-hint">{{ cryptoRateDisplay }}</p>
                </div>
                <button
                  v-if="cryptoPayAmountDisplay"
                  type="button"
                  class="usdt-copy-btn-large"
                  @click="copyPayAmount"
                >
                  <Icon name="copy" size="sm" />
                  <span>{{ amountCopied ? t('payment.crypto.copied') : t('payment.crypto.copyAmount') }}</span>
                </button>
              </div>
            </div>

            <!-- Network Badge -->
            <div class="usdt-network-badge">
              <span class="usdt-network-label">{{ t('payment.crypto.usdtBscNetwork') }}</span>
            </div>

            <!-- Address Section with full address on desktop -->
            <div class="usdt-address-card">
              <div class="mb-2 flex items-center justify-between gap-3">
                <span class="usdt-address-label">{{ t('payment.crypto.walletAddress') }}</span>
                <button
                  type="button"
                  class="usdt-copy-btn-large"
                  @click="copyWalletAddress"
                >
                  <Icon name="copy" size="sm" />
                  <span>{{ addressCopied ? t('payment.crypto.copied') : t('payment.crypto.copyAddress') }}</span>
                </button>
              </div>
              <!-- Desktop: full address, Mobile: truncated -->
              <p class="usdt-address-value usdt-address-desktop">{{ walletAddress }}</p>
              <p class="usdt-address-value usdt-address-mobile">{{ truncatedAddress }}</p>
            </div>

            <!-- Warning - simplified -->
            <div class="usdt-warning">
              <svg class="usdt-warning-icon" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
              </svg>
              <span>{{ t('payment.crypto.usdtBscWarningShort') }}</span>
            </div>
          </div>
          <button v-if="payUrl" class="btn btn-secondary text-sm" @click="reopenPopup">
            {{ t('payment.qr.openPayWindow') }}
          </button>
        </div>
      </div>
      <div class="card p-4 text-center">
        <p class="usdt-countdown-label">{{ t('payment.qr.expiresIn') }}</p>
        <p class="usdt-countdown-value">{{ countdownDisplay }}</p>
        <p class="usdt-countdown-hint">{{ t('payment.qr.waitingPayment') }}</p>
      </div>
      <button class="btn btn-secondary w-full" :disabled="cancelling" @click="handleCancel">
        {{ cancelling ? t('common.processing') : t('payment.qr.cancelOrder') }}
      </button>
    </template>

    <!-- Waiting for Popup/Redirect Mode -->
    <template v-else>
      <div class="card p-6">
        <div class="flex flex-col items-center space-y-4 py-4">
          <div class="h-10 w-10 animate-spin rounded-full border-4 border-primary-500 border-t-transparent"></div>
          <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('payment.qr.payInNewWindowHint') }}</p>
          <button v-if="payUrl" class="btn btn-secondary text-sm" @click="reopenPopup">
            {{ t('payment.qr.openPayWindow') }}
          </button>
        </div>
      </div>
      <div class="card p-4 text-center">
        <p class="mt-1 text-2xl font-bold tabular-nums text-gray-900 dark:text-white">{{ countdownDisplay }}</p>
        <p class="mt-1 text-xs text-gray-400 dark:text-gray-500">{{ t('payment.qr.waitingPayment') }}</p>
      </div>
      <button class="btn btn-secondary w-full" :disabled="cancelling" @click="handleCancel">
        {{ cancelling ? t('common.processing') : t('payment.qr.cancelOrder') }}
      </button>
    </template>
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
import { useClipboard } from '@/composables/useClipboard'
import type { PaymentOrder } from '@/types/payment'
import Icon from '@/components/icons/Icon.vue'
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
}>()

type PaymentOutcome = 'success' | 'cancelled' | 'expired'

const emit = defineEmits<{ done: []; success: []; settled: [outcome: PaymentOutcome] }>()

const i18n = useI18n()
const { t } = i18n
const paymentStore = usePaymentStore()
const appStore = useAppStore()
const { copied: addressCopied, copyToClipboard: copyAddressToClipboard } = useClipboard()
const { copied: amountCopied, copyToClipboard: copyAmountToClipboard } = useClipboard()

const qrCanvas = ref<HTMLCanvasElement | null>(null)
const qrUrl = ref('')
const remainingSeconds = ref(0)
const cancelling = ref(false)
const paidOrder = ref<PaymentOrder | null>(null)
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

const VERIFY_RETRY_INTERVAL_MS = 15000
const VERIFY_RETRY_MAX_ATTEMPTS = 6

const isAlipay = computed(() => isBuiltInAlipayMethod(props.paymentType))
const isWxpay = computed(() => isBuiltInWxpayMethod(props.paymentType))
const isUsdtBsc = computed(() => {
  const normalized = props.paymentType.trim().toLowerCase()
  return normalized === 'usdt_bsc' || (normalized.includes('usdt') && normalized.includes('bsc'))
})
const showQrLogo = computed(() => isAlipay.value || isWxpay.value)
const qrLogoIcon = computed(() => isAlipay.value ? alipayIcon : wxpayIcon)
const walletAddress = computed(() => (props.receiveAddress || qrUrl.value).trim())
const isMobileAlipayDeepLink = computed(() => props.mobileAlipayDeepLink === true && isAlipay.value && !!qrUrl.value)
const showQRCode = computed(() => !!qrUrl.value && (!isMobileAlipayDeepLink.value || deepLinkFallbackVisible.value))
const cryptoPayAmountDisplay = computed(() => {
  const amount = (props.cryptoAmount || '').trim()
  if (!amount) return ''
  const currency = (props.cryptoCurrency || 'USDT').trim()
  return `${amount} ${currency}`
})
const cryptoRateDisplay = computed(() => {
  const rate = (props.cryptoRate || '').trim()
  if (!rate) return ''
  const numeric = Number(rate)
  const formatted = Number.isFinite(numeric) && numeric > 0 ? numeric.toFixed(4).replace(/0+$/, '').replace(/\.$/, '') : rate
  return t('payment.crypto.exchangeRateValue', { rate: formatted })
})

const truncatedAddress = computed(() => {
  const addr = walletAddress.value
  if (addr.length <= 16) return addr
  // 桌面端全显示，移动端截断
  return addr.substring(0, 8) + '...' + addr.substring(addr.length - 6)
})

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
  if (isUsdtBsc.value) return t('payment.crypto.scanUsdtBsc')
  if (isAlipay.value) return t('payment.qr.scanAlipay')
  if (isWxpay.value) return t('payment.qr.scanWxpay')
  return t('payment.qr.scanToPay')
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

async function copyWalletAddress() {
  await copyAddressToClipboard(walletAddress.value, t('payment.crypto.addressCopied'))
}

async function copyPayAmount() {
  const amount = (props.cryptoAmount || '').trim()
  if (!amount) return
  await copyAmountToClipboard(amount, t('payment.crypto.amountCopied'))
}

function setOutcome(next: PaymentOutcome) {
  if (outcome.value === next) return
  outcome.value = next
  emit('settled', next)
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
  if (!isWxpay.value && !isAlipay.value && !isUsdtBsc.value) return order
  const outTradeNo = String(order.out_trade_no || '').trim()
  if (!outTradeNo) return order
  const normalizedStatus = String(order.status || '').trim().toUpperCase()
  if (normalizedStatus !== 'PENDING') return order
  const now = Date.now()
  if (verifyAttempts >= VERIFY_RETRY_MAX_ATTEMPTS || now - lastVerifyAt < VERIFY_RETRY_INTERVAL_MS) {
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

let pollInFlight = false
async function pollStatus() {
  if (!props.orderId || outcome.value) return
  // 防重入：接口（含 verifyOrder 二次确认）响应慢于 3 秒轮询间隔时避免并发重叠请求。
  if (pollInFlight) return
  pollInFlight = true
  try {
    let order = await paymentStore.pollOrderStatus(props.orderId)
    if (!order) return
    // 已进入终态则不再处理迟到的响应。
    if (outcome.value) return
    order = await tryRecoverPendingOrder(order)
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
      setOutcome('expired')
    }
  } finally {
    pollInFlight = false
  }
}

function startCountdown(seconds: number) {
  remainingSeconds.value = Math.max(0, seconds)
  if (remainingSeconds.value <= 0) { setOutcome('expired'); return }
  countdownTimer = setInterval(() => {
    remainingSeconds.value--
    if (remainingSeconds.value <= 0) { setOutcome('expired'); cleanup() }
  }, 1000)
}

async function handleCancel() {
  if (!props.orderId || cancelling.value) return
  cancelling.value = true
  try {
    await paymentAPI.cancelOrder(props.orderId)
    cleanup()
    setOutcome('cancelled')
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  } finally {
    cancelling.value = false
  }
}

function handleDone() { cleanup(); emit('done') }

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
startCountdown(seconds)
pollTimer = setInterval(pollStatus, 3000)
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
onUnmounted(() => cleanup())
</script>

<style scoped>
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

/* ============ USDT 支付面板优化 ============ */
.usdt-panel {
  --usdt-surface-1: #0A0D12;
  --usdt-surface-2: #0F131C;
  --usdt-surface-3: #161D2B;
  --usdt-accent: #6EE7B7;
  --usdt-accent-dim: #34D399;
  --usdt-text-primary: #F9FAFB;
  --usdt-text-secondary: #9CA3AF;
  --usdt-warning: #F59E0B;
}

:root:not(.dark) .usdt-panel {
  --usdt-surface-1: #F9FAFB;
  --usdt-surface-2: #FFFFFF;
  --usdt-surface-3: #F3F4F6;
  --usdt-accent: #059669;
  --usdt-accent-dim: #10B981;
  --usdt-text-primary: #111827;
  --usdt-text-secondary: #6B7280;
  --usdt-warning: #D97706;
}

/* QR Code enlarged for USDT */
.usdt-qr-container {
  border-color: color-mix(in srgb, var(--usdt-accent) 30%, transparent);
  background: var(--usdt-surface-2);
}

.usdt-qr-canvas {
  width: 240px !important;
  height: 240px !important;
}

/* Amount Card: CNY prominent */
.usdt-amount-card {
  background: var(--usdt-surface-2);
  border-radius: 16px;
  padding: clamp(1.25rem, 3vw, 1.75rem);
  border: 1px solid color-mix(in srgb, var(--usdt-accent) 20%, transparent);
}

.usdt-cny-amount {
  font-size: clamp(2rem, 5vw, 2.75rem);
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  color: var(--usdt-text-primary);
  line-height: 1.1;
  margin-bottom: 0.5rem;
}

.usdt-amount-value {
  font-size: clamp(1.25rem, 3vw, 1.5rem);
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  color: var(--usdt-accent);
  line-height: 1.2;
  font-family: ui-monospace, 'Cascadia Code', 'Source Code Pro', Menlo, Consolas, 'DejaVu Sans Mono', monospace;
}

.usdt-rate-hint {
  margin-top: 0.25rem;
  font-size: 0.875rem;
  color: var(--usdt-text-secondary);
  font-variant-numeric: tabular-nums;
}

.usdt-network-badge {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0.625rem 1rem;
  background: color-mix(in srgb, var(--usdt-accent) 12%, transparent);
  border-radius: 999px;
  border: 1px solid color-mix(in srgb, var(--usdt-accent) 30%, transparent);
}

.usdt-network-label {
  font-size: 0.8125rem;
  font-weight: 600;
  color: var(--usdt-accent-dim);
  letter-spacing: 0.02em;
}

.usdt-address-card {
  background: var(--usdt-surface-2);
  border-radius: 12px;
  padding: clamp(1rem, 2.5vw, 1.25rem);
  border: 1px solid var(--usdt-surface-3);
}

.usdt-address-label {
  font-size: 0.8125rem;
  font-weight: 500;
  color: var(--usdt-text-secondary);
  text-transform: uppercase;
  letter-spacing: 0.03em;
}

.usdt-address-value {
  font-family: ui-monospace, 'Cascadia Code', 'Source Code Pro', Menlo, Consolas, 'DejaVu Sans Mono', monospace;
  font-size: 0.875rem;
  font-weight: 500;
  color: var(--usdt-text-primary);
  word-break: break-all;
  line-height: 1.6;
}

/* Desktop: show full address, Mobile: show truncated */
.usdt-address-desktop {
  display: none;
}

.usdt-address-mobile {
  display: block;
}

@media (min-width: 640px) {
  .usdt-address-desktop {
    display: block;
    font-size: 0.9375rem;
  }

  .usdt-address-mobile {
    display: none;
  }
}

.usdt-copy-btn-large {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.625rem 1.125rem;
  font-size: 0.875rem;
  font-weight: 600;
  color: var(--usdt-accent-dim);
  background: color-mix(in srgb, var(--usdt-accent) 10%, transparent);
  border-radius: 999px;
  border: 1px solid color-mix(in srgb, var(--usdt-accent) 25%, transparent);
  cursor: pointer;
  transition: all 0.15s ease;
  white-space: nowrap;
}

.usdt-copy-btn-large:hover {
  background: color-mix(in srgb, var(--usdt-accent) 18%, transparent);
  transform: translateY(-1px);
  box-shadow: 0 4px 12px color-mix(in srgb, var(--usdt-accent) 20%, transparent);
}

.usdt-copy-btn-large:active {
  transform: translateY(0);
}

.usdt-warning {
  display: flex;
  align-items: flex-start;
  gap: 0.625rem;
  padding: 0.875rem 1rem;
  background: color-mix(in srgb, var(--usdt-warning) 8%, transparent);
  border-radius: 12px;
  border: 1px solid color-mix(in srgb, var(--usdt-warning) 20%, transparent);
  font-size: 0.8125rem;
  line-height: 1.5;
  color: var(--usdt-warning);
}

.usdt-warning-icon {
  flex-shrink: 0;
  width: 1.125rem;
  height: 1.125rem;
  margin-top: 0.0625rem;
}

.usdt-countdown-label {
  font-size: 0.875rem;
  color: var(--usdt-text-secondary);
  font-weight: 500;
  text-transform: uppercase;
  letter-spacing: 0.03em;
}

.usdt-countdown-value {
  margin-top: 0.5rem;
  font-size: clamp(2.5rem, 5vw, 3.5rem);
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  color: var(--usdt-accent-dim);
  line-height: 1;
}

.usdt-countdown-hint {
  margin-top: 0.5rem;
  font-size: 0.75rem;
  color: var(--usdt-text-secondary);
  opacity: 0.7;
}
</style>
