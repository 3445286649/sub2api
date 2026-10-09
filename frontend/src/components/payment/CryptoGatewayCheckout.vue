<template>
  <section class="mx-auto w-full max-w-2xl" data-test="gateway-checkout">
    <div class="card divide-y divide-gray-100 overflow-hidden dark:divide-dark-700/60">
      <header class="px-4 py-5 sm:px-6">
        <div class="flex flex-wrap items-center gap-3">
          <img :src="usdtIcon" alt="" class="h-10 w-10 shrink-0" />
          <div class="min-w-0 flex-1">
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('payment.crypto.title') }}</h2>
            <p class="mt-0.5 truncate text-xs text-gray-500 dark:text-gray-400" :title="orderNo">{{ t('payment.orders.orderNo') }} {{ orderNo }}</p>
          </div>
          <span class="hidden shrink-0 items-center gap-1.5 rounded-full px-2 py-0.5 text-[11px] font-semibold sm:inline-flex" :class="networkBadgeClass">
            <span class="h-1.5 w-1.5 rounded-full bg-current"></span>{{ networkBadge }}
          </span>
          <div v-if="showCountdown" class="shrink-0 text-right" role="timer" aria-live="off">
            <p class="font-mono text-xl font-semibold leading-tight tabular-nums" :class="urgent ? 'text-amber-500' : 'text-gray-900 dark:text-white'">{{ countdown }}</p>
            <p class="text-[11px] text-gray-500 dark:text-gray-400">{{ t('payment.crypto.expiresSuffix') }}</p>
          </div>
        </div>
        <p class="mt-2 text-xs text-gray-500 dark:text-gray-400 sm:hidden">{{ networkLabel }}</p>
        <template v-if="showCountdown">
          <div class="mt-4 h-1 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700">
            <div class="h-full rounded-full transition-[width] duration-1000 ease-linear motion-reduce:transition-none" :class="urgent ? 'bg-amber-500' : 'bg-primary-500'" :style="{ width: progressPercent + '%' }"></div>
          </div>
          <p v-if="urgent && payable" data-test="gateway-urgent" class="mt-2.5 flex items-start gap-1.5 text-xs text-amber-700 dark:text-amber-300">
            <Icon name="clock" size="sm" class="shrink-0" />{{ t('payment.gateway.urgentHint') }}
          </p>
        </template>
      </header>

      <template v-if="payable">
        <div class="px-4 py-5 sm:px-6">
          <div class="mb-3 flex items-center gap-2 text-sm font-semibold text-gray-600 dark:text-gray-300">
            <span :class="stepNumClass(0)"><Icon v-if="amountDone" name="check" size="xs" :stroke-width="3" /><template v-else>1</template></span>
            {{ t('payment.crypto.stepAmount') }}
            <span v-if="rateText" class="ml-auto inline-flex items-center gap-1 text-xs font-medium text-gray-500 dark:text-gray-400"><Icon name="lock" size="xs" />{{ t('payment.crypto.rateLocked') }}</span>
          </div>
          <div class="flex min-w-0 flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
            <p data-test="gateway-amount" class="min-w-0 break-all font-mono text-[32px] font-bold leading-tight tabular-nums text-gray-900 dark:text-white sm:text-[40px]">
              <span>{{ amountParts.head }}</span><span v-if="amountParts.tail" data-test="gateway-amount-tail" class="text-amber-500">{{ amountParts.tail }}</span><span class="ml-1.5 inline-block text-[15px] font-semibold text-gray-500 dark:text-gray-400">USDT</span>
            </p>
            <button type="button" data-test="gateway-copy-amount" class="btn btn-primary w-full shrink-0 py-3 sm:w-auto sm:py-2.5" @click="copyAmount">
              <Icon :name="amountCopied ? 'check' : 'copy'" size="sm" />{{ amountCopied ? t('payment.crypto.copied') : t('payment.crypto.copyAmount') }}
            </button>
          </div>
          <p class="mt-2.5 flex flex-wrap items-center gap-x-2.5 gap-y-1 text-sm text-gray-500 dark:text-gray-400">
            <span>{{ t('payment.crypto.orderAmount') }} <b class="font-semibold text-gray-700 dark:text-gray-200">{{ orderAmount }}</b></span>
            <template v-if="rateText"><span class="h-[3px] w-[3px] rounded-full bg-gray-300 dark:bg-dark-500"></span><span>{{ rateText }}</span></template>
          </p>
          <div class="mt-3.5 flex gap-2 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2.5 text-[13px] leading-relaxed text-amber-800 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-200">
            <Icon name="exclamationTriangle" size="sm" class="mt-0.5 shrink-0" />
            <p><b>{{ t('payment.gateway.exactAmount') }}</b> {{ t('payment.crypto.exactAmountDesc') }}</p>
          </div>
        </div>

        <div class="px-4 py-5 sm:px-6">
          <div class="mb-3 flex items-center gap-2 text-sm font-semibold text-gray-600 dark:text-gray-300">
            <span :class="stepNumClass(1)"><Icon v-if="addressDone" name="check" size="xs" :stroke-width="3" /><template v-else>2</template></span>
            {{ t('payment.crypto.stepAddress') }}
            <span class="ml-auto rounded-full border px-2 py-0.5 text-[11px] font-semibold" :class="networkBadgeClass">{{ t('payment.gateway.networkOnly', { network: networkStandard }) }}</span>
          </div>
          <div class="grid grid-cols-1 items-center gap-3 sm:grid-cols-[auto_minmax(0,1fr)] sm:gap-5">
            <div class="order-2 flex flex-col items-center gap-3 sm:order-none">
              <div id="gateway-qr" class="relative rounded-lg border border-gray-200 bg-white p-2.5 dark:border-dark-600" :class="qrOpen ? 'block' : 'hidden sm:block'">
                <canvas ref="qrCanvas" class="block h-[148px] w-[148px]" :aria-label="t('payment.crypto.walletAddress')" role="img"></canvas>
                <div class="pointer-events-none absolute inset-0 flex items-center justify-center"><span class="rounded-full bg-white p-1 shadow-[0_0_0_3px_#fff]"><img :src="usdtIcon" alt="" class="h-6 w-6" /></span></div>
              </div>
              <button type="button" data-test="gateway-toggle-qr" class="btn btn-ghost px-3 py-2 text-sm sm:hidden" aria-controls="gateway-qr" :aria-expanded="qrOpen" @click="qrOpen = !qrOpen">
                <Icon name="grid" size="sm" />{{ qrOpen ? t('payment.crypto.hideQr') : t('payment.crypto.showQr') }}
              </button>
            </div>
            <div class="min-w-0">
              <p data-test="gateway-address" class="break-all rounded-lg border border-gray-200 bg-gray-50 px-3.5 py-3 font-mono text-sm leading-[1.7] text-gray-600 dark:border-dark-600 dark:bg-dark-900/60 dark:text-gray-300 sm:text-[15px]">
                <span v-for="(group, index) in addressGroups" :key="index" class="inline-block" :class="[index < addressGroups.length - 1 ? 'mr-[0.45em]' : '', index === 0 || index === addressGroups.length - 1 ? 'font-bold text-gray-900 dark:text-white' : '']">{{ group }}</span>
              </p>
              <p class="mb-3 mt-2 text-xs text-gray-500 dark:text-gray-400">{{ t('payment.crypto.addressHint') }}</p>
              <button type="button" data-test="gateway-copy-address" class="btn w-full border border-primary-200 bg-primary-50 py-3 text-primary-700 hover:border-primary-400 dark:border-primary-500/30 dark:bg-primary-500/10 dark:text-primary-300 sm:w-auto sm:py-2.5" @click="copyAddress">
                <Icon :name="addressCopied ? 'check' : 'copy'" size="sm" />{{ addressCopied ? t('payment.crypto.copied') : t('payment.crypto.copyAddress') }}
              </button>
            </div>
          </div>
        </div>

        <div class="px-4 py-5 sm:px-6">
          <div class="mb-3 flex items-center gap-2 text-sm font-semibold text-gray-600 dark:text-gray-300"><span :class="stepNumClass(2)">3</span>{{ t('payment.crypto.stepWaiting') }}</div>
          <div class="flex items-center gap-3" role="status" aria-live="polite">
            <span class="relative flex h-2.5 w-2.5 shrink-0" aria-hidden="true"><span class="absolute inline-flex h-full w-full animate-ping rounded-full bg-primary-400 opacity-75 motion-reduce:animate-none"></span><span class="relative inline-flex h-2.5 w-2.5 rounded-full bg-primary-500"></span></span>
            <div><p data-test="gateway-status" class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('payment.gateway.waiting') }}</p><p class="text-[13px] text-gray-500 dark:text-gray-400">{{ t('payment.gateway.listeningHint') }}</p></div>
          </div>
        </div>

        <div class="px-4 py-5 sm:px-6">
          <ul class="grid gap-2 text-[13px] text-gray-600 dark:text-gray-300">
            <li class="flex gap-2"><Icon name="checkCircle" size="sm" class="mt-0.5 shrink-0 text-primary-500" /><span><b class="font-semibold text-gray-900 dark:text-white">{{ t('payment.gateway.networkOnly', { network: networkLabel }) }}：</b>{{ t('payment.crypto.ruleNetworkDesc') }}</span></li>
            <li class="flex gap-2"><Icon name="checkCircle" size="sm" class="mt-0.5 shrink-0 text-primary-500" /><span><b class="font-semibold text-gray-900 dark:text-white">{{ t(network === 'tron_mainnet' ? 'payment.gateway.tronFeeTitle' : 'payment.crypto.ruleGasTitle') }}</b>{{ t(network === 'tron_mainnet' ? 'payment.gateway.tronFeeDesc' : 'payment.crypto.ruleGasDesc') }}</span></li>
            <li class="flex gap-2"><Icon name="checkCircle" size="sm" class="mt-0.5 shrink-0 text-primary-500" /><span><b class="font-semibold text-gray-900 dark:text-white">{{ t('payment.crypto.ruleOnceTitle') }}</b>{{ t('payment.crypto.ruleOnceDesc') }}</span></li>
          </ul>
          <details class="group mt-3 rounded-lg border border-dashed border-gray-300 dark:border-dark-600">
            <summary class="flex cursor-pointer list-none items-center gap-1.5 px-3 py-2.5 text-[13px] text-gray-500 dark:text-gray-400 [&::-webkit-details-marker]:hidden"><Icon name="chevronRight" size="xs" class="transition-transform group-open:rotate-90" />{{ t('payment.crypto.exchangeTitle') }}</summary>
            <div class="space-y-1.5 pb-3 pl-8 pr-3 text-[13px] leading-relaxed text-gray-600 dark:text-gray-300"><p>{{ t('payment.crypto.exchangeWithdrawalWarning') }}</p><p>{{ t('payment.gateway.exchangeDecimalsWarning') }}</p></div>
          </details>
        </div>
      </template>

      <div v-else class="px-4 py-8 text-center sm:px-6" role="status" aria-live="polite">
        <div class="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full" :class="stateIconClass">
          <span v-if="mode === 'creating' || mode === 'accepted' || mode === 'checkingExpiry'" class="h-8 w-8 animate-spin rounded-full border-[3px] border-current border-t-transparent motion-reduce:animate-none" aria-hidden="true"></span>
          <Icon v-else :name="mode === 'review' ? 'exclamationTriangle' : mode === 'rejected' || mode === 'cancelled' ? 'xCircle' : mode === 'confirming' ? 'shield' : 'clock'" size="xl" />
        </div>
        <h3 data-test="gateway-status" class="text-lg font-semibold text-gray-900 dark:text-white">{{ statusLabel }}</h3>
        <p class="mx-auto mt-2 max-w-lg text-sm leading-6 text-gray-500 dark:text-gray-400">{{ stateHint }}</p>
        <template v-if="mode === 'confirming'">
          <p class="mt-5 font-mono text-xl font-semibold tabular-nums text-primary-600 dark:text-primary-400" data-test="gateway-confirmations">{{ requiredConfirmations > 0 ? t('payment.gateway.confirmations', { current: confirmations, required: requiredConfirmations }) : t('payment.gateway.confirmationsUnknown') }}</p>
          <div v-if="requiredConfirmations > 0" class="mx-auto mt-3 h-1.5 max-w-xs overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700"><div class="h-full rounded-full bg-primary-500" :style="{ width: confirmationPercent + '%' }"></div></div>
          <p v-if="cryptoAmount" class="mt-4 break-all font-mono text-sm text-gray-600 dark:text-gray-300" data-test="gateway-amount">{{ cryptoAmount }} USDT</p>
        </template>
        <p v-if="mode === 'expired' || mode === 'cancelled' || mode === 'checkingExpiry'" class="mt-5 flex items-start justify-center gap-2 text-[13px] text-primary-700 dark:text-primary-300"><Icon name="refresh" size="sm" class="mt-0.5 shrink-0" />{{ t('payment.gateway.closedWatchHint') }}</p>
      </div>

      <div v-if="txHash" class="space-y-2 px-4 py-4 text-sm sm:px-6"><CryptoTxReceipt :tx-hash="txHash" :amount="cryptoAmount" currency="USDT" :network="network" /></div>
    </div>

    <p v-if="queryFailed" role="alert" data-test="gateway-query-failed" class="mt-3 flex items-start gap-2 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2.5 text-[13px] text-amber-800 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-200"><Icon name="exclamationTriangle" size="sm" class="mt-0.5 shrink-0" />{{ t('payment.gateway.queryFailed') }}</p>
    <footer class="mt-3.5 flex flex-wrap items-center gap-2.5 px-1">
      <router-link v-if="showSupport" to="/support" class="mr-auto text-sm text-gray-500 hover:text-gray-900 dark:text-gray-400 dark:hover:text-white">{{ t('payment.crypto.needHelp') }}</router-link>
      <span v-else class="mr-auto"></span>
      <button type="button" data-test="gateway-check" class="btn btn-secondary inline-flex items-center gap-2" :disabled="checking" @click="emit('check')"><Icon name="refresh" size="sm" :class="checking ? 'animate-spin motion-reduce:animate-none' : ''" />{{ t('payment.gateway.retry') }}</button>
      <button v-if="payable || mode === 'creating'" type="button" data-test="gateway-cancel" class="btn btn-secondary" :disabled="cancelling" @click="confirmingCancel = true">{{ cancelling ? t('common.processing') : t('payment.qr.cancelOrder') }}</button>
      <button v-if="mode === 'expired' || mode === 'cancelled' || mode === 'rejected'" type="button" class="btn btn-primary" @click="emit('done')">{{ t('payment.result.backToRecharge') }}</button>
    </footer>
    <ConfirmDialog :show="confirmingCancel" :title="t('payment.crypto.cancelConfirmTitle')" :message="t('payment.gateway.cancelConfirmMessage')" :confirm-text="t('payment.crypto.cancelConfirmOk')" :cancel-text="t('payment.crypto.cancelConfirmBack')" danger @confirm="onConfirmCancel" @cancel="confirmingCancel = false" />
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import QRCode from 'qrcode'
import Icon from '@/components/icons/Icon.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import { useClipboard } from '@/composables/useClipboard'
import usdtIcon from '@/assets/icons/usdt.svg'
import CryptoTxReceipt from './CryptoTxReceipt.vue'
import { cryptoNetworkLabel } from './cryptoGateway'
import { formatCountdown, formatUsdtRate, groupWalletAddress, splitUsdtAmount } from './usdtBsc'

const props = withDefaults(defineProps<{
  network: string; cryptoAmount: string; receiveAddress: string; orderNo: string; orderAmount: string;
  remainingSeconds: number; totalSeconds?: number; cryptoRate?: string; gatewayStatus?: string; createStage?: string; orderStatus?: string;
  confirmations?: number; requiredConfirmations?: number; txHash?: string; checking?: boolean;
  cancelling?: boolean; showSupport?: boolean; queryFailed?: boolean;
}>(), { confirmations: 0, requiredConfirmations: 0, totalSeconds: 0 })
const emit = defineEmits<{ cancel: []; check: []; done: [] }>()
const { t } = useI18n()
const { copied: amountCopied, copyToClipboard: copyAmountToClipboard } = useClipboard()
const { copied: addressCopied, copyToClipboard: copyAddressToClipboard } = useClipboard()
const qrCanvas = ref<HTMLCanvasElement | null>(null)
const qrOpen = ref(false)
const amountDone = ref(false)
const addressDone = ref(false)
const confirmingCancel = ref(false)
const networkLabel = computed(() => cryptoNetworkLabel(props.network))
const networkStandard = computed(() => props.network === 'tron_mainnet' ? 'TRC20' : props.network === 'bsc_mainnet' ? 'BEP20' : '')
const networkBadge = computed(() => props.network === 'tron_mainnet' ? 'TRON · TRC20' : props.network === 'bsc_mainnet' ? 'BSC · BEP20' : t('payment.gateway.network'))
const networkBadgeClass = computed(() => props.network === 'tron_mainnet' ? 'border-red-200 bg-red-50 text-red-700 dark:border-red-500/30 dark:bg-red-500/10 dark:text-red-300' : 'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-300')
const countdown = computed(() => formatCountdown(props.remainingSeconds))
const urgent = computed(() => props.remainingSeconds <= 300)
const amountParts = computed(() => splitUsdtAmount(props.cryptoAmount))
const addressGroups = computed(() => groupWalletAddress(props.receiveAddress))
const rateText = computed(() => props.cryptoRate ? t('payment.crypto.exchangeRateValue', { rate: formatUsdtRate(props.cryptoRate) }) : '')
const progressPercent = computed(() => props.totalSeconds > 0 ? Math.min(100, Math.max(0, props.remainingSeconds / props.totalSeconds * 100)) : 100)
const confirmationPercent = computed(() => Math.min(100, Math.max(0, props.confirmations / props.requiredConfirmations * 100)))
const ready = computed(() => {
  const validAddress = props.network === 'bsc_mainnet'
    ? /^0x[0-9a-fA-F]{40}$/.test(props.receiveAddress)
    : props.network === 'tron_mainnet' && /^T[1-9A-HJ-NP-Za-km-z]{33}$/.test(props.receiveAddress)
  const decimals = props.network === 'tron_mainnet' ? 6 : 18
  return validAddress && /^(?:0|[1-9][0-9]*)(?:\.[0-9]+)?$/.test(props.cryptoAmount)
    && /[1-9]/.test(props.cryptoAmount) && (props.cryptoAmount.split('.')[1]?.length || 0) <= decimals
})
const mode = computed(() => {
  if (props.gatewayStatus === 'REVIEW') return 'review'
  if (['PAID', 'RECHARGING', 'COMPLETED'].includes(props.orderStatus || '') || props.gatewayStatus === 'PAID') return 'accepted'
  if (props.createStage === 'rejected' || props.orderStatus === 'FAILED') return 'rejected'
  if (props.gatewayStatus === 'CONFIRMING') return 'confirming'
  if (props.gatewayStatus === 'CANCELLED' || props.orderStatus === 'CANCELLED') return 'cancelled'
  if (props.gatewayStatus === 'EXPIRED' || props.orderStatus === 'EXPIRED') return 'expired'
  if (props.remainingSeconds <= 0) return 'checkingExpiry'
  if (!ready.value || ['creating', 'unknown'].includes(props.createStage || '')) return 'creating'
  return 'waiting'
})
const payable = computed(() => mode.value === 'waiting')
const showCountdown = computed(() => payable.value || mode.value === 'creating')
const statusLabel = computed(() => t('payment.gateway.' + mode.value))
const stateHint = computed(() => t('payment.gateway.' + (mode.value === 'expired' || mode.value === 'cancelled' ? 'closed' : mode.value) + 'Hint'))
const stateIconClass = computed(() => mode.value === 'review' || mode.value === 'expired'
  ? 'bg-amber-50 text-amber-500 dark:bg-amber-500/10'
  : mode.value === 'rejected' ? 'bg-red-50 text-red-500 dark:bg-red-500/10'
    : mode.value === 'cancelled' ? 'bg-gray-100 text-gray-400 dark:bg-dark-700'
      : 'bg-primary-50 text-primary-600 dark:bg-primary-500/10 dark:text-primary-400')

function stepNumClass(index: number) {
  const done = [amountDone.value, addressDone.value, false]
  const base = 'flex h-5 w-5 shrink-0 items-center justify-center rounded-full border-[1.5px] text-[11px] font-bold'
  if (done[index]) return [base, 'border-primary-500 bg-primary-500 text-white']
  return [base, done.indexOf(false) === index ? 'border-primary-500 text-primary-700 ring-4 ring-primary-500/25 dark:text-primary-300' : 'border-gray-300 text-gray-500 dark:border-dark-500 dark:text-gray-400']
}
async function copyAmount() {
  if (payable.value && await copyAmountToClipboard(props.cryptoAmount, t('payment.crypto.amountCopied'))) amountDone.value = true
}
async function copyAddress() {
  if (payable.value && await copyAddressToClipboard(props.receiveAddress, t('payment.crypto.addressCopied'))) addressDone.value = true
}
function onConfirmCancel() {
  confirmingCancel.value = false
  if (payable.value || mode.value === 'creating') emit('cancel')
}
async function renderQR() {
  await nextTick()
  if (qrCanvas.value && payable.value) await QRCode.toCanvas(qrCanvas.value, props.receiveAddress, { width: 148, margin: 1, errorCorrectionLevel: 'H' })
}
watch([() => props.receiveAddress, payable], renderQR)
watch([() => props.cryptoAmount, () => props.receiveAddress], () => { amountDone.value = false; addressDone.value = false })
watch(mode, () => { confirmingCancel.value = false })
onMounted(renderQR)
</script>
