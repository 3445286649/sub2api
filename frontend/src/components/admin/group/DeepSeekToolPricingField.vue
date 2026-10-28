<template>
  <div class="space-y-3">
    <div v-for="tool in tools" :key="tool" class="border-t border-gray-200 pt-3 dark:border-dark-400">
      <div class="mb-2 text-sm font-medium text-gray-700 dark:text-gray-200">{{ t(`admin.groups.deepSeekTools.${tool}`) }}</div>
      <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
        <div>
          <label class="input-label" :for="`${idPrefix}-${tool}-mode`">{{ t('admin.groups.deepSeekTools.rateMode') }}</label>
          <select :id="`${idPrefix}-${tool}-mode`" class="input" :value="value[tool]?.mode ?? 'group'" @change="update(tool, 'mode', ($event.target as HTMLSelectElement).value)">
            <option value="group">{{ t('admin.groups.deepSeekTools.rateGroup') }}</option>
            <option value="fixed">{{ t('admin.groups.deepSeekTools.rateFixed') }}</option>
            <option value="independent">{{ t('admin.groups.deepSeekTools.rateIndependent') }}</option>
          </select>
        </div>
        <div>
          <label class="input-label" :for="`${idPrefix}-${tool}-price`">{{ t('admin.groups.deepSeekTools.pricePerCall') }}</label>
          <input :id="`${idPrefix}-${tool}-price`" class="input" type="number" min="0" max="1000000" step="0.00000001" required :value="value[tool]?.price_per_call ?? ''" @input="updateNumber(tool, 'price_per_call', $event)" />
        </div>
        <div v-if="value[tool]?.mode === 'independent'">
          <label class="input-label" :for="`${idPrefix}-${tool}-rate`">{{ t('admin.groups.deepSeekTools.toolMultiplier') }}</label>
          <input :id="`${idPrefix}-${tool}-rate`" class="input" type="number" min="0.0001" max="1000000" step="0.0001" required :value="value[tool]?.rate_multiplier ?? 1" @input="updateNumber(tool, 'rate_multiplier', $event)" />
        </div>
      </div>
      <p class="input-hint mt-1">{{ t('admin.groups.deepSeekTools.finalPrice', { price: finalPrice(tool) }) }}</p>
    </div>
    <p class="input-hint">{{ t('admin.groups.deepSeekTools.pricingHint') }}</p>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { DeepSeekToolPricing } from '@/types'

const { t } = useI18n()
const tools = ['search', 'fetch'] as const
const props = defineProps<{
  value: DeepSeekToolPricing
  groupRate: number | string | null
  idPrefix: string
}>()
const emit = defineEmits<{ (event: 'update:value', value: DeepSeekToolPricing): void }>()

const update = (tool: 'search' | 'fetch', field: string, value: string | number | null) => {
  emit('update:value', {
    ...props.value,
    [tool]: { ...props.value[tool], [field]: value },
  } as DeepSeekToolPricing)
}
const updateNumber = (tool: 'search' | 'fetch', field: string, event: Event) => {
  const input = event.target as HTMLInputElement
  update(tool, field, input.value === '' ? null : Number(input.value))
}
const finalPrice = (tool: 'search' | 'fetch') => {
  const entry = props.value[tool]
  if (entry.price_per_call === null || !Number.isFinite(entry.price_per_call)) return '-'
  const rate = entry.mode === 'fixed' ? 1 : entry.mode === 'independent' ? entry.rate_multiplier ?? 1 : Number(props.groupRate ?? 1)
  const amount = entry.price_per_call * rate
  return Number.isFinite(amount) ? `$${amount.toLocaleString('en-US', { maximumFractionDigits: 12 })}` : '-'
}
</script>
