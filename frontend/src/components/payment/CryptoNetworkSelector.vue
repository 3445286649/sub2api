<template>
  <fieldset class="mt-4" data-test="crypto-network-selector">
    <legend class="mb-2 text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('payment.gateway.network') }}</legend>
    <div class="flex flex-wrap gap-3">
      <label v-for="network in options" :key="network.value" class="flex min-h-10 items-center gap-2 rounded-md border px-3 text-sm" :class="modelValue === network.value ? 'border-primary-500 text-primary-700 dark:text-primary-300' : 'border-gray-200 text-gray-700 dark:border-dark-600 dark:text-gray-300'">
        <input type="radio" :name="name" :value="network.value" :checked="modelValue === network.value" @change="emit('update:modelValue', network.value)" />
        {{ network.label }}
      </label>
      <p v-if="!options.length" class="text-sm text-gray-500 dark:text-gray-400">{{ t('payment.gateway.noNetworks') }}</p>
    </div>
  </fieldset>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { CryptoNetwork } from '@/types/payment'
import { CRYPTO_NETWORKS, supportedCryptoNetworks } from './cryptoGateway'

const props = defineProps<{ modelValue: string; networks: string[]; name: string }>()
const emit = defineEmits<{ 'update:modelValue': [network: CryptoNetwork] }>()
const { t } = useI18n()
const options = computed(() => {
  const available = supportedCryptoNetworks(props.networks)
  return CRYPTO_NETWORKS.filter(network => available.includes(network.value))
})
</script>
