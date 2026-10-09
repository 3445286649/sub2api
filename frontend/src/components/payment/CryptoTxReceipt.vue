<template>
  <div v-if="amountText" class="flex justify-between gap-3">
    <span class="text-gray-500 dark:text-gray-400">{{ t('payment.crypto.paidOnChain') }}</span>
    <span data-test="crypto-paid-amount" class="font-mono font-medium text-gray-900 dark:text-white">{{ amountText }}</span>
  </div>
  <div v-if="hasHash" class="flex items-center justify-between gap-3">
    <span class="shrink-0 text-gray-500 dark:text-gray-400">{{ t('payment.crypto.txHash') }}</span>
    <span class="flex min-w-0 items-center gap-1">
      <a
        data-test="crypto-tx-link"
        :href="explorerUrl"
        target="_blank"
        rel="noopener noreferrer"
        :title="t('payment.crypto.viewOnExplorer')"
        class="inline-flex min-w-0 items-center gap-1 font-mono font-medium text-primary-600 hover:underline dark:text-primary-400"
      >
        <span class="truncate">{{ shortHash }}</span>
        <Icon name="externalLink" size="xs" class="shrink-0" />
      </a>
      <button
        type="button"
        data-test="crypto-tx-copy"
        class="shrink-0 rounded-md p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-700 dark:hover:bg-dark-700 dark:hover:text-gray-200"
        :aria-label="t('payment.crypto.copyTxHash')"
        :title="t('payment.crypto.copyTxHash')"
        @click="copyHash"
      >
        <Icon :name="copied ? 'check' : 'copy'" size="xs" />
      </button>
    </span>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useClipboard } from '@/composables/useClipboard'
import { shortenTxHash } from './usdtBsc'
import { cryptoTransactionUrl } from './cryptoGateway'

const props = defineProps<{
  txHash?: string
  amount?: string
  currency?: string
  network?: string
}>()

const { t } = useI18n()
const { copied, copyToClipboard } = useClipboard()

const hash = computed(() => (props.txHash || '').trim())
const hasHash = computed(() => !!explorerUrl.value)
const shortHash = computed(() => shortenTxHash(hash.value))
const explorerUrl = computed(() => cryptoTransactionUrl(hash.value, props.network || 'bsc_mainnet'))
const amountText = computed(() => {
  const amount = (props.amount || '').trim()
  return amount ? `${amount} ${(props.currency || 'USDT').trim()}` : ''
})

function copyHash() {
  void copyToClipboard(hash.value, t('payment.crypto.txHashCopied'))
}
</script>
