<template>
  <div
    data-provider-card
    :class="[
      'group relative rounded-lg border transition-all',
      enabled ? 'border-gray-200 dark:border-dark-600' : 'border-gray-200 bg-gray-50 opacity-50 dark:border-dark-700 dark:bg-dark-800/50',
    ]"
    :title="!enabled ? t('admin.settings.payment.typeDisabled') + ' — ' + t('admin.settings.payment.enableTypesFirst') : undefined"
  >
    <div :class="[
      'flex min-w-0 flex-col gap-2 px-3 py-3 sm:flex-row sm:items-center sm:justify-between sm:px-4 sm:py-2.5',
      !enabled && 'pointer-events-none',
    ]">
      <div class="flex min-w-0 flex-col gap-1.5 sm:flex-row sm:items-center sm:gap-3">
        <div class="flex min-w-0 items-center justify-between gap-2 sm:contents">
          <div class="flex min-w-0 items-center gap-2 sm:gap-3">
            <div :class="[
              'shrink-0 rounded-md p-1 sm:p-1.5',
              provider.enabled && enabled ? 'bg-green-100 dark:bg-green-900/30' : 'bg-gray-100 dark:bg-dark-700',
            ]">
              <Icon
                name="server"
                size="sm"
                :class="provider.enabled && enabled ? 'text-green-600 dark:text-green-400' : 'text-gray-400'"
              />
            </div>
            <span class="min-w-0 truncate text-sm font-medium text-gray-900 dark:text-white">{{ provider.name }}</span>
          </div>
          <div class="flex shrink-0 items-center gap-1 sm:hidden">
            <button v-if="provider.provider_key === 'bepusdt'" type="button" :disabled="checking" @click="emit('check')" :title="t('admin.settings.payment.gatewayCheck')" :aria-label="t('admin.settings.payment.gatewayCheck')" class="rounded-md p-1.5 text-gray-500 hover:bg-blue-50 hover:text-blue-600 disabled:opacity-50 dark:hover:bg-blue-900/20 dark:hover:text-blue-400"><Icon name="refresh" size="sm" :class="checking ? 'animate-spin' : ''" /></button>
            <button type="button" @click="emit('edit')" :title="t('common.edit')" :aria-label="t('common.edit')" class="rounded-md p-1.5 text-gray-500 hover:bg-blue-50 hover:text-blue-600 dark:hover:bg-blue-900/20 dark:hover:text-blue-400"><Icon name="edit" size="sm" /></button>
            <button type="button" @click="emit('delete')" :title="t('common.delete')" :aria-label="t('common.delete')" class="rounded-md p-1.5 text-gray-500 hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400"><Icon name="trash" size="sm" /></button>
          </div>
        </div>
        <div class="flex min-w-0 items-center gap-1 text-xs text-gray-400 dark:text-gray-500 sm:contents">
          <span class="hidden sm:inline">{{ keyLabel }}</span>
          <span class="whitespace-nowrap sm:hidden">{{ keyLabel }}<template v-if="provider.payment_mode"> · {{ modeLabel }}</template></span>
          <span v-if="provider.payment_mode" class="hidden sm:inline">· {{ modeLabel }}</span>
        </div>
        <span v-if="enabled && availableTypes.length" class="hidden text-xs text-gray-300 dark:text-gray-600 sm:inline">|</span>
        <div v-if="enabled" class="flex min-w-0 flex-wrap items-center gap-1">
          <button
            v-for="pt in availableTypes"
            :key="pt.value"
            type="button"
            @click="emit('toggleType', pt.value)"
            :class="[
              'whitespace-nowrap rounded px-2 py-0.5 text-xs font-medium transition-all',
              isSelected(pt.value)
                ? 'bg-primary-500 text-white'
                : 'bg-gray-100 text-gray-400 dark:bg-dark-700 dark:text-gray-500',
            ]"
          >{{ pt.label }}</button>
        </div>
      </div>

      <!-- Right: toggles + actions -->
      <div class="flex flex-wrap items-center gap-x-3 gap-y-2 sm:flex-nowrap sm:gap-4">
        <ToggleSwitch layout="inline" :label="t('common.enabled')" :checked="provider.enabled" @toggle="emit('toggleField', 'enabled')" />
        <ToggleSwitch v-if="!['bepusdt', 'zhifufm', 'jianpay'].includes(provider.provider_key)" layout="inline" :label="t('admin.settings.payment.refundEnabled')" :checked="provider.refund_enabled" @toggle="emit('toggleField', 'refund_enabled')" />
        <ToggleSwitch v-if="!['bepusdt', 'zhifufm', 'jianpay'].includes(provider.provider_key) && provider.refund_enabled" layout="inline" :label="t('admin.settings.payment.allowUserRefund')" :checked="provider.allow_user_refund" @toggle="emit('toggleField', 'allow_user_refund')" />
        <div class="hidden items-center gap-2 border-l border-gray-200 pl-3 dark:border-dark-600 sm:flex">
          <button v-if="provider.provider_key === 'bepusdt'" type="button" :disabled="checking" @click="emit('check')" class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-blue-50 hover:text-blue-600 disabled:opacity-50 dark:hover:bg-blue-900/20 dark:hover:text-blue-400">
            <Icon name="refresh" size="sm" :class="checking ? 'animate-spin' : ''" />
            <span class="text-xs">{{ t('admin.settings.payment.gatewayCheck') }}</span>
          </button>
          <button type="button" @click="emit('edit')" class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-blue-50 hover:text-blue-600 dark:hover:bg-blue-900/20 dark:hover:text-blue-400">
            <Icon name="edit" size="sm" />
            <span class="text-xs">{{ t('common.edit') }}</span>
          </button>
          <button type="button" @click="emit('delete')" class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400">
            <Icon name="trash" size="sm" />
            <span class="text-xs">{{ t('common.delete') }}</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import ToggleSwitch from './ToggleSwitch.vue'
import type { ProviderInstance } from '@/types/payment'
import type { TypeOption } from './providerConfig'
import { PAYMENT_MODE_QRCODE, PAYMENT_MODE_POPUP, PAYMENT_MODE_REDIRECT } from './providerConfig'

const PROVIDER_KEY_LABELS: Record<string, string> = {
  easypay: 'admin.settings.payment.providerEasypay',
  zhifufm: 'admin.settings.payment.providerZhifufm',
  jianpay: 'admin.settings.payment.providerJianpay',
  alipay: 'admin.settings.payment.providerAlipay',
  wxpay: 'admin.settings.payment.providerWxpay',
  stripe: 'admin.settings.payment.providerStripe',
  airwallex: 'admin.settings.payment.providerAirwallex',
  usdt_bsc: 'admin.settings.payment.providerUsdtBsc',
  bepusdt: 'admin.settings.payment.providerBepusdt',
}

const props = defineProps<{
  provider: ProviderInstance
  enabled: boolean
  availableTypes: TypeOption[]
  checking?: boolean
}>()

const emit = defineEmits<{
  toggleField: [field: 'enabled' | 'refund_enabled' | 'allow_user_refund']
  toggleType: [type: string]
  edit: []
  delete: []
  check: []
}>()

const { t } = useI18n()

const keyLabel = computed(() => t(PROVIDER_KEY_LABELS[props.provider.provider_key] || props.provider.provider_key))

const modeLabel = computed(() => {
  if (props.provider.payment_mode === PAYMENT_MODE_QRCODE) return t('admin.settings.payment.modeQRCode')
  if (props.provider.payment_mode === PAYMENT_MODE_POPUP) return t('admin.settings.payment.modePopup')
  if (props.provider.payment_mode === PAYMENT_MODE_REDIRECT) return t('admin.settings.payment.modeRedirect')
  return ''
})

function isSelected(type: string): boolean {
  return Array.isArray(props.provider.supported_types) && props.provider.supported_types.includes(type)
}
</script>
