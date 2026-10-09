<template>
  <section class="mx-auto max-w-2xl space-y-5 rounded-lg border border-gray-200 bg-white p-5 dark:border-dark-600 dark:bg-dark-800 sm:p-6" data-test="gateway-checkout">
    <header class="flex flex-wrap items-start justify-between gap-3 border-b border-gray-100 pb-4 dark:border-dark-600">
      <div class="min-w-0">
        <h2 class="text-base font-semibold text-gray-900 dark:text-white">USDT</h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ networkLabel }}</p>
      </div>
      <span class="font-mono text-sm tabular-nums text-gray-600 dark:text-gray-300">{{ countdown }}</span>
    </header>

    <div role="status" aria-live="polite" class="flex items-start gap-3 rounded-md p-3" :class="review ? 'bg-amber-50 text-amber-800 dark:bg-amber-900/20 dark:text-amber-200' : 'bg-gray-50 text-gray-700 dark:bg-dark-900 dark:text-gray-300'">
      <Icon :name="review ? 'exclamationTriangle' : accepted ? 'clock' : 'infoCircle'" size="sm" class="mt-0.5 shrink-0" />
      <div class="min-w-0">
        <p class="font-medium" data-test="gateway-status">{{ statusLabel }}</p>
        <p v-if="review" class="mt-1 text-sm">{{ t('payment.gateway.reviewHint') }}</p>
        <p v-else-if="rejected" class="mt-1 text-sm">{{ t('payment.gateway.rejectedHint') }}</p>
        <p v-else-if="creating" class="mt-1 text-sm">{{ t('payment.gateway.creatingHint') }}</p>
        <p v-else-if="closed" class="mt-1 text-sm">{{ t('payment.gateway.closedHint') }}</p>
        <p v-else-if="confirming" class="mt-1 text-sm">{{ requiredConfirmations > 0 ? t('payment.gateway.confirmations', { current: confirmations, required: requiredConfirmations }) : t('payment.gateway.confirmationsUnknown') }}</p>
      </div>
    </div>

    <template v-if="ready && !closed && !review && !rejected && !accepted && !confirming">
      <div>
        <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('payment.gateway.amount') }}</p>
        <p class="mt-1 break-all font-mono text-2xl font-semibold text-gray-900 dark:text-white" data-test="gateway-amount">{{ cryptoAmount }} <span class="text-base">USDT</span></p>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ orderAmount }}</p>
        <button type="button" class="btn btn-secondary mt-3 inline-flex items-center gap-2" data-test="gateway-copy-amount" @click="copyAmount"><Icon :name="amountCopied ? 'check' : 'copy'" size="sm" />{{ t('payment.crypto.copyAmount') }}</button>
      </div>
      <p class="text-sm text-amber-700 dark:text-amber-300">{{ t('payment.gateway.exactAmount') }}</p>
      <div class="flex flex-col gap-4 border-t border-gray-100 pt-5 dark:border-dark-600 sm:flex-row">
        <div class="w-40 shrink-0 bg-white p-2"><canvas ref="qrCanvas" class="block h-36 w-36" :aria-label="t('payment.gateway.address')" role="img"></canvas></div>
        <div class="min-w-0 flex-1">
          <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('payment.gateway.address') }}</p>
          <p class="mt-2 break-all font-mono text-sm text-gray-900 dark:text-white" data-test="gateway-address">{{ receiveAddress }}</p>
          <button type="button" class="btn btn-secondary mt-3 inline-flex items-center gap-2" data-test="gateway-copy-address" @click="copyAddress"><Icon :name="addressCopied ? 'check' : 'copy'" size="sm" />{{ t('payment.crypto.copyAddress') }}</button>
        </div>
      </div>
      <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('payment.gateway.networkWarning', { network: networkLabel }) }}</p>
    </template>

    <div v-else-if="confirming && cryptoAmount" class="text-sm text-gray-700 dark:text-gray-300">
      <p>{{ t('payment.gateway.amount') }}</p>
      <p class="mt-1 break-all font-mono text-lg font-semibold" data-test="gateway-amount">{{ cryptoAmount }} USDT</p>
    </div>

    <div class="space-y-2 border-t border-gray-100 pt-4 text-sm dark:border-dark-600">
      <p class="break-all font-mono text-gray-500 dark:text-gray-400">{{ orderNo }}</p>
      <CryptoTxReceipt v-if="txHash" :tx-hash="txHash" :amount="cryptoAmount" currency="USDT" :network="network" />
    </div>
    <footer class="flex flex-wrap gap-3">
      <button type="button" class="btn btn-secondary inline-flex items-center gap-2" :disabled="checking" @click="emit('check')"><Icon name="refresh" size="sm" />{{ t('payment.gateway.retry') }}</button>
      <router-link v-if="showSupport || review" to="/support" class="btn btn-secondary">{{ t('common.contactSupport') }}</router-link>
      <button v-if="!accepted && !review && !rejected && !closed && !confirming" type="button" class="btn btn-secondary" :disabled="cancelling" @click="confirmingCancel = true">{{ t('payment.qr.cancelOrder') }}</button>
      <button v-if="closed || rejected" type="button" class="btn btn-secondary" @click="emit('done')">{{ t('payment.result.backToRecharge') }}</button>
    </footer>
    <ConfirmDialog :show="confirmingCancel" :title="t('payment.crypto.cancelConfirmTitle')" :message="t('payment.crypto.cancelConfirmMessage')" :confirm-text="t('payment.crypto.cancelConfirmOk')" :cancel-text="t('payment.crypto.cancelConfirmBack')" @confirm="confirmingCancel = false; emit('cancel')" @cancel="confirmingCancel = false" />
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import QRCode from 'qrcode'
import Icon from '@/components/icons/Icon.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import { useClipboard } from '@/composables/useClipboard'
import CryptoTxReceipt from './CryptoTxReceipt.vue'
import { cryptoNetworkLabel } from './cryptoGateway'
import { formatCountdown } from './usdtBsc'

const props = withDefaults(defineProps<{
  network: string; cryptoAmount: string; receiveAddress: string; orderNo: string; orderAmount: string;
  remainingSeconds: number; gatewayStatus?: string; createStage?: string; orderStatus?: string;
  confirmations?: number; requiredConfirmations?: number; txHash?: string; checking?: boolean;
  cancelling?: boolean; showSupport?: boolean;
}>(), { confirmations: 0, requiredConfirmations: 0 })
const emit = defineEmits<{ cancel: []; check: []; done: [] }>()
const { t } = useI18n()
const { copied: amountCopied, copyToClipboard: copyAmountToClipboard } = useClipboard()
const { copied: addressCopied, copyToClipboard: copyAddressToClipboard } = useClipboard()
const qrCanvas = ref<HTMLCanvasElement | null>(null)
const confirmingCancel = ref(false)
const networkLabel = computed(() => cryptoNetworkLabel(props.network))
const countdown = computed(() => formatCountdown(props.remainingSeconds))
const review = computed(() => props.gatewayStatus === 'REVIEW')
const rejected = computed(() => props.createStage === 'rejected')
const accepted = computed(() => ['PAID', 'RECHARGING'].includes(props.orderStatus || '') || props.gatewayStatus === 'PAID')
const closed = computed(() => !accepted.value && ['EXPIRED', 'CANCELLED'].includes(props.gatewayStatus || props.orderStatus || ''))
const confirming = computed(() => props.gatewayStatus === 'CONFIRMING')
const ready = computed(() => {
  const validAddress = props.network === 'bsc_mainnet'
    ? /^0x[0-9a-fA-F]{40}$/.test(props.receiveAddress)
    : props.network === 'tron_mainnet' && /^T[1-9A-HJ-NP-Za-km-z]{33}$/.test(props.receiveAddress)
  const decimals = props.network === 'tron_mainnet' ? 6 : 18
  return validAddress && /^(?:0|[1-9][0-9]*)(?:\.[0-9]+)?$/.test(props.cryptoAmount)
    && /[1-9]/.test(props.cryptoAmount) && (props.cryptoAmount.split('.')[1]?.length || 0) <= decimals
})
const creating = computed(() => !ready.value && !review.value && !rejected.value && !closed.value && !accepted.value)
const statusLabel = computed(() => {
  if (review.value) return t('payment.gateway.review')
  if (rejected.value) return t('payment.gateway.rejected')
  if (accepted.value) return t('payment.gateway.accepted')
  if (closed.value) return t(props.gatewayStatus === 'CANCELLED' || props.orderStatus === 'CANCELLED' ? 'payment.gateway.cancelled' : 'payment.gateway.expired')
  if (creating.value) return t('payment.gateway.creating')
  return t(confirming.value ? 'payment.gateway.confirming' : 'payment.gateway.waiting')
})

async function renderQR() {
  await nextTick()
    if (qrCanvas.value && ready.value && !review.value && !rejected.value && !closed.value && !accepted.value && !confirming.value) {
    await QRCode.toCanvas(qrCanvas.value, props.receiveAddress, { width: 144, margin: 0, errorCorrectionLevel: 'M' })
  }
}
function copyAmount() { void copyAmountToClipboard(props.cryptoAmount, t('payment.crypto.amountCopied')) }
function copyAddress() { void copyAddressToClipboard(props.receiveAddress, t('payment.crypto.addressCopied')) }
watch([() => props.receiveAddress, ready, review, rejected, closed, accepted, confirming], renderQR)
onMounted(renderQR)
</script>
