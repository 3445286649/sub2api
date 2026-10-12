<template>
  <section class="card p-5" :aria-busy="loading || saving">
    <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.settings.payment.fmRoutingTitle') }}</h2>
    <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.settings.payment.fmRoutingHint') }}</p>
    <p v-if="error" class="mt-3 text-sm text-red-600" role="alert">{{ error }}</p>
    <button v-if="!loaded && !loading" type="button" class="btn btn-secondary mt-3" @click="load">{{ t('common.refresh') }}</button>
    <form v-if="loaded" class="mt-4 space-y-4" @submit.prevent="save">
      <div class="grid gap-4 sm:grid-cols-2">
        <div v-for="method in methods" :key="method">
          <label :for="`fm-source-${method}`" class="input-label">{{ t(`payment.methods.${method}`) }}</label>
          <select :id="`fm-source-${method}`" v-model="sources[method]" class="input" :disabled="saving">
            <option v-for="option in options(method)" :key="option.value" :value="option.value">{{ locale === 'zh' ? option.labelZh : option.labelEn }}</option>
          </select>
          <label class="mt-2 flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
            <input v-model="enabled[method]" type="checkbox" :disabled="saving" />{{ t('common.enabled') }}
          </label>
        </div>
      </div>
      <button type="submit" class="btn btn-primary" :disabled="saving">{{ t(saving ? 'common.saving' : 'common.save') }}</button>
      <p v-if="saved" role="status" class="text-sm text-green-600">{{ t('common.saved') }}</p>
    </form>
  </section>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminPaymentAPI } from '@/api/admin/payment'
import { getPaymentVisibleMethodSourceOptions, normalizePaymentVisibleMethodSource, type PaymentVisibleMethod } from '@/api/admin/settings'
import type { ProviderInstance } from '@/types/payment'

const props = defineProps<{ providers: ProviderInstance[] }>()
const { t, locale } = useI18n()
const methods: PaymentVisibleMethod[] = ['alipay', 'wxpay']
const sources = reactive({ alipay: '', wxpay: '' })
const enabled = reactive({ alipay: true, wxpay: true })
const loading = ref(false)
const loaded = ref(false)
const saving = ref(false)
const saved = ref(false)
const error = ref('')

function options(method: PaymentVisibleMethod) {
  return getPaymentVisibleMethodSourceOptions(method).filter(option => {
    if (!option.value || sources[method] === option.value) return true
    const providerKey = option.value.startsWith('zhifufm') ? 'zhifufm' : option.value.startsWith('easypay') ? 'easypay' : method
    return props.providers.some(p => p.enabled && p.provider_key === providerKey && p.supported_types.includes(method))
  })
}
async function load() {
  loading.value = true
  error.value = ''
  try {
    const { data } = await adminPaymentAPI.getConfig()
    sources.alipay = normalizePaymentVisibleMethodSource('alipay', data.payment_visible_method_alipay_source)
    sources.wxpay = normalizePaymentVisibleMethodSource('wxpay', data.payment_visible_method_wxpay_source)
    enabled.alipay = data.payment_visible_method_alipay_enabled !== false
    enabled.wxpay = data.payment_visible_method_wxpay_enabled !== false
    loaded.value = true
  } catch { error.value = t('admin.settings.payment.fmRoutingFailed') }
  finally { loading.value = false }
}
async function save() {
  saving.value = true
  saved.value = false
  error.value = ''
  try {
    await adminPaymentAPI.updateConfig({
      payment_visible_method_alipay_source: sources.alipay,
      payment_visible_method_wxpay_source: sources.wxpay,
      payment_visible_method_alipay_enabled: enabled.alipay,
      payment_visible_method_wxpay_enabled: enabled.wxpay,
    })
    saved.value = true
  } catch { error.value = t('admin.settings.payment.fmRoutingFailed') }
  finally { saving.value = false }
}
onMounted(load)
</script>
