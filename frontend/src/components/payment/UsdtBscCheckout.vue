<template>
  <div class="mx-auto w-full max-w-2xl">
    <div class="card divide-y divide-gray-100 overflow-hidden dark:divide-dark-700/60">
      <!-- 头部：币种、网络、倒计时 -->
      <div class="px-4 py-5 sm:px-6">
        <div class="flex items-center gap-3 pb-4">
          <img :src="usdtIcon" alt="" class="h-10 w-10 shrink-0" />
          <div class="min-w-0 flex-1">
            <p class="text-base font-semibold text-gray-900 dark:text-white">{{ t('payment.crypto.title') }}</p>
            <p class="truncate text-xs text-gray-500 dark:text-gray-400">{{ t('payment.orders.orderNo') }} {{ orderNo }}</p>
          </div>
          <span class="hidden shrink-0 items-center gap-1.5 rounded-full bg-amber-400/15 px-2 py-0.5 text-[11px] font-semibold text-amber-700 dark:text-amber-300 sm:inline-flex">
            <span class="h-1.5 w-1.5 rounded-full bg-[#F0B90B]"></span>BSC · BEP20
          </span>
          <div class="shrink-0 text-right" role="timer" aria-live="off">
            <p :class="['font-mono text-xl font-semibold leading-tight tabular-nums', urgent ? 'text-amber-500' : 'text-gray-900 dark:text-white']">
              {{ countdown }}
            </p>
            <p class="text-[11px] text-gray-500 dark:text-gray-400">{{ t('payment.crypto.expiresSuffix') }}</p>
          </div>
        </div>
        <div class="h-1 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700">
          <div
            :class="['h-full rounded-full bg-gradient-to-r transition-[width] duration-1000 ease-linear', urgent ? 'from-amber-400 to-amber-500' : 'from-primary-500 to-primary-600']"
            :style="{ width: `${progressPercent}%` }"
          ></div>
        </div>
        <p v-if="urgent" data-test="usdt-urgent" class="mt-2.5 flex items-center gap-1.5 text-xs text-amber-700 dark:text-amber-300">
          <Icon name="clock" size="sm" class="shrink-0" />
          {{ t('payment.crypto.urgentHint') }}
        </p>
      </div>

      <!-- 1. 转账金额 -->
      <div class="px-4 py-5 sm:px-6">
        <div class="mb-3 flex items-center gap-2 text-sm font-semibold text-gray-600 dark:text-gray-300">
          <span :class="stepNumClass(0)">
            <Icon v-if="stepDone[0]" name="check" size="xs" :stroke-width="3" />
            <template v-else>1</template>
          </span>
          {{ t('payment.crypto.stepAmount') }}
          <span class="ml-auto inline-flex items-center gap-1 text-xs font-medium text-gray-500 dark:text-gray-400">
            <Icon name="lock" size="xs" />{{ t('payment.crypto.rateLocked') }}
          </span>
        </div>
        <div class="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
          <p data-test="usdt-amount" class="whitespace-nowrap font-mono font-bold leading-none tracking-tight tabular-nums text-gray-900 dark:text-white">
            <span class="text-[32px] sm:text-[40px]">{{ amountParts.head }}</span><span
              v-if="amountParts.tail"
              data-test="usdt-amount-tail"
              class="text-[32px] text-amber-500 sm:text-[40px]"
            >{{ amountParts.tail }}</span><span class="ml-1.5 text-[15px] font-semibold tracking-normal text-gray-500 dark:text-gray-400">{{ currencyLabel }}</span>
          </p>
          <button type="button" data-test="copy-amount" class="btn btn-primary w-full py-3 sm:w-auto sm:py-2.5" @click="copyAmount">
            <Icon :name="amountCopied ? 'check' : 'copy'" size="sm" />
            {{ amountCopied ? t('payment.crypto.copied') : t('payment.crypto.copyAmount') }}
          </button>
        </div>
        <p class="mt-2.5 flex flex-wrap items-center gap-x-2.5 gap-y-1 text-sm text-gray-500 dark:text-gray-400">
          <span>{{ t('payment.crypto.orderAmount') }} <b class="font-semibold text-gray-700 dark:text-gray-200">{{ orderAmount }}</b></span>
          <template v-if="rateText">
            <span class="h-[3px] w-[3px] rounded-full bg-gray-300 dark:bg-dark-500"></span>
            <span>{{ rateText }}</span>
          </template>
        </p>
        <div class="mt-3.5 flex gap-2 rounded-xl border border-amber-200 bg-amber-50 px-3 py-2.5 text-[13px] leading-relaxed text-amber-800 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-200">
          <Icon name="exclamationTriangle" size="sm" class="mt-0.5 shrink-0" />
          <p><b class="font-semibold">{{ t('payment.crypto.exactAmountTitle') }}</b>{{ t('payment.crypto.exactAmountDesc') }}</p>
        </div>
      </div>

      <!-- 2. 收款地址 -->
      <div class="px-4 py-5 sm:px-6">
        <div class="mb-3 flex items-center gap-2 text-sm font-semibold text-gray-600 dark:text-gray-300">
          <span :class="stepNumClass(1)">
            <Icon v-if="stepDone[1]" name="check" size="xs" :stroke-width="3" />
            <template v-else>2</template>
          </span>
          {{ t('payment.crypto.stepAddress') }}
          <span class="ml-auto rounded-full border border-amber-200 bg-amber-50 px-2 py-0.5 text-[11px] font-semibold text-amber-700 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-300">
            {{ t('payment.crypto.bep20Only') }}
          </span>
        </div>
        <div class="grid grid-cols-1 items-center gap-3 sm:grid-cols-[auto_1fr] sm:gap-5">
          <div class="order-2 flex flex-col items-center gap-3 sm:order-none">
            <div
              id="usdt-qr"
              :class="['relative rounded-2xl border border-gray-200 bg-white p-2.5 dark:border-dark-600', qrOpen ? 'block' : 'hidden sm:block']"
            >
              <canvas ref="qrCanvas" class="block" :aria-label="t('payment.crypto.walletAddress')" role="img"></canvas>
              <div class="pointer-events-none absolute inset-0 flex items-center justify-center">
                <span class="rounded-full bg-white p-1 shadow-[0_0_0_3px_#fff]">
                  <img :src="usdtIcon" alt="" class="h-6 w-6" />
                </span>
              </div>
            </div>
            <button
              type="button"
              data-test="toggle-qr"
              class="btn btn-ghost px-3 py-2 text-sm sm:hidden"
              aria-controls="usdt-qr"
              :aria-expanded="qrOpen"
              @click="qrOpen = !qrOpen"
            >
              <Icon name="grid" size="sm" />
              {{ qrOpen ? t('payment.crypto.hideQr') : t('payment.crypto.showQr') }}
            </button>
          </div>
          <div class="min-w-0">
            <p
              data-test="usdt-address"
              class="break-all rounded-xl border border-gray-200 bg-gray-50 px-3.5 py-3 font-mono text-sm leading-[1.7] text-gray-600 dark:border-dark-600 dark:bg-dark-900/60 dark:text-gray-300 sm:text-[15px]"
            >
              <span
                v-for="(group, index) in addressGroups"
                :key="index"
                :class="['inline-block', index < addressGroups.length - 1 ? 'mr-[0.45em]' : '', index === 0 || index === addressGroups.length - 1 ? 'font-bold text-gray-900 dark:text-white' : '']"
              >{{ group }}</span>
            </p>
            <p class="mb-3 mt-2 text-xs text-gray-500 dark:text-gray-400">{{ t('payment.crypto.addressHint') }}</p>
            <button
              type="button"
              class="wallet-address-copy btn w-full border border-primary-200 bg-primary-50 py-3 text-primary-700 hover:border-primary-400 dark:border-primary-500/30 dark:bg-primary-500/10 dark:text-primary-300 sm:w-auto sm:py-2.5"
              @click="copyAddress"
            >
              <Icon :name="addressCopied ? 'check' : 'copy'" size="sm" />
              {{ addressCopied ? t('payment.crypto.copied') : t('payment.crypto.copyAddress') }}
            </button>
          </div>
        </div>
      </div>

      <!-- 3. 等待到账 -->
      <div class="px-4 py-5 sm:px-6">
        <div class="mb-3 flex items-center gap-2 text-sm font-semibold text-gray-600 dark:text-gray-300">
          <span :class="stepNumClass(2)">3</span>
          {{ t('payment.crypto.stepWaiting') }}
        </div>
        <div class="flex items-center gap-3" role="status">
          <span class="relative flex h-2.5 w-2.5 shrink-0" aria-hidden="true">
            <span class="absolute inline-flex h-full w-full animate-ping rounded-full bg-primary-400 opacity-75 motion-reduce:animate-none"></span>
            <span class="relative inline-flex h-2.5 w-2.5 rounded-full bg-primary-500"></span>
          </span>
          <div>
            <p class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('payment.crypto.listening') }}</p>
            <p class="text-[13px] text-gray-500 dark:text-gray-400">{{ t('payment.crypto.listeningHint') }}</p>
          </div>
        </div>
      </div>

      <!-- 注意事项 -->
      <div class="px-4 py-5 sm:px-6">
        <ul class="grid gap-2">
          <li v-for="rule in rules" :key="rule.title" class="flex gap-2 text-[13px] text-gray-600 dark:text-gray-300">
            <Icon name="checkCircle" size="sm" class="mt-0.5 shrink-0 text-primary-500" />
            <span><b class="font-semibold text-gray-900 dark:text-white">{{ rule.title }}</b>{{ rule.desc }}</span>
          </li>
        </ul>
        <details class="group mt-3 rounded-xl border border-dashed border-gray-300 dark:border-dark-600">
          <summary class="flex cursor-pointer list-none items-center gap-1.5 px-3 py-2.5 text-[13px] text-gray-500 dark:text-gray-400 [&::-webkit-details-marker]:hidden">
            <Icon name="chevronRight" size="xs" class="transition-transform group-open:rotate-90" />
            {{ t('payment.crypto.exchangeTitle') }}
          </summary>
          <div class="space-y-1.5 pb-3 pl-8 pr-3 text-[13px] leading-relaxed text-gray-600 dark:text-gray-300">
            <p>{{ t('payment.crypto.exchangeWithdrawalWarning') }}</p>
            <p>{{ t('payment.crypto.exchangeDecimalsWarning') }}</p>
          </div>
        </details>
      </div>
    </div>

    <div class="mt-3.5 flex items-center justify-between gap-3 px-1">
      <router-link v-if="showSupport" to="/support" class="text-sm text-gray-500 hover:text-gray-900 dark:text-gray-400 dark:hover:text-white">
        {{ t('payment.crypto.needHelp') }}
      </router-link>
      <span v-else></span>
      <button type="button" data-test="usdt-cancel" class="btn btn-secondary" :disabled="cancelling" @click="confirmingCancel = true">
        {{ cancelling ? t('common.processing') : t('payment.qr.cancelOrder') }}
      </button>
    </div>

    <ConfirmDialog
      :show="confirmingCancel"
      :title="t('payment.crypto.cancelConfirmTitle')"
      :message="t('payment.crypto.cancelConfirmMessage')"
      :confirm-text="t('payment.crypto.cancelConfirmOk')"
      :cancel-text="t('payment.crypto.cancelConfirmBack')"
      danger
      @confirm="onConfirmCancel"
      @cancel="confirmingCancel = false"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import QRCode from 'qrcode'
import Icon from '@/components/icons/Icon.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import { useClipboard } from '@/composables/useClipboard'
import usdtIcon from '@/assets/icons/usdt.svg'
import { formatCountdown, formatUsdtRate, groupWalletAddress, splitUsdtAmount } from './usdtBsc'

const props = defineProps<{
  cryptoAmount: string
  cryptoCurrency?: string
  cryptoRate?: string
  orderAmount: string
  receiveAddress: string
  orderNo: string
  remainingSeconds: number
  totalSeconds: number
  cancelling?: boolean
  showSupport?: boolean
}>()

const emit = defineEmits<{ cancel: [] }>()

const { t } = useI18n()
const { copied: amountCopied, copyToClipboard: copyAmountToClipboard } = useClipboard()
const { copied: addressCopied, copyToClipboard: copyAddressToClipboard } = useClipboard()

const qrCanvas = ref<HTMLCanvasElement | null>(null)
const qrOpen = ref(false)
const amountDone = ref(false)
const addressDone = ref(false)
const confirmingCancel = ref(false)

const URGENT_SECONDS = 5 * 60

const amountParts = computed(() => splitUsdtAmount(props.cryptoAmount))
const currencyLabel = computed(() => (props.cryptoCurrency || 'USDT').trim())
const addressGroups = computed(() => groupWalletAddress(props.receiveAddress))
const countdown = computed(() => formatCountdown(props.remainingSeconds))
const urgent = computed(() => props.remainingSeconds <= URGENT_SECONDS)
// 总时长要等第一次轮询拿到 created_at 才知道，之前进度条保持满格
const progressPercent = computed(() => {
  if (props.totalSeconds <= 0) return 100
  return Math.min(100, Math.max(0, (props.remainingSeconds / props.totalSeconds) * 100))
})
const rateText = computed(() => {
  const rate = formatUsdtRate(props.cryptoRate)
  return rate ? t('payment.crypto.exchangeRateValue', { rate }) : ''
})
const stepDone = computed(() => [amountDone.value, addressDone.value, false])
const currentStep = computed(() => stepDone.value.indexOf(false))
const rules = computed(() => [
  { title: t('payment.crypto.ruleNetworkTitle'), desc: t('payment.crypto.ruleNetworkDesc') },
  { title: t('payment.crypto.ruleGasTitle'), desc: t('payment.crypto.ruleGasDesc') },
  { title: t('payment.crypto.ruleOnceTitle'), desc: t('payment.crypto.ruleOnceDesc') },
])

function stepNumClass(index: number): string[] {
  const base = 'flex h-5 w-5 shrink-0 items-center justify-center rounded-full border-[1.5px] text-[11px] font-bold transition-all duration-200'
  if (stepDone.value[index]) return [base, 'border-primary-500 bg-primary-500 text-white']
  if (currentStep.value === index) {
    return [base, 'border-primary-500 text-primary-700 ring-4 ring-primary-500/25 dark:text-primary-300']
  }
  return [base, 'border-gray-300 text-gray-500 dark:border-dark-500 dark:text-gray-400']
}

async function copyAmount() {
  const amount = props.cryptoAmount.trim()
  if (!amount) return
  if (await copyAmountToClipboard(amount, t('payment.crypto.amountCopied'))) amountDone.value = true
}

async function copyAddress() {
  const address = props.receiveAddress.trim()
  if (!address) return
  if (await copyAddressToClipboard(address, t('payment.crypto.addressCopied'))) addressDone.value = true
}

function onConfirmCancel() {
  confirmingCancel.value = false
  emit('cancel')
}

async function renderQR() {
  const address = props.receiveAddress.trim()
  if (!qrCanvas.value || !address) return
  // 暗色模式下也保持白底黑码，保证钱包能扫出来
  await QRCode.toCanvas(qrCanvas.value, address, {
    width: 148,
    margin: 1,
    errorCorrectionLevel: 'M',
  })
}

onMounted(renderQR)
watch(() => props.receiveAddress, renderQR)
</script>
