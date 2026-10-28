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
          <input :id="`${idPrefix}-${tool}-price`" class="input" type="text" inputmode="decimal" autocomplete="off" maxlength="32" required :value="drafts[tool].price_per_call" @input="updateNumber(tool, 'price_per_call', $event)" @blur="normalizeDraft(tool, 'price_per_call')" />
        </div>
        <div v-if="value[tool]?.mode === 'independent'">
          <label class="input-label" :for="`${idPrefix}-${tool}-rate`">{{ t('admin.groups.deepSeekTools.toolMultiplier') }}</label>
          <input :id="`${idPrefix}-${tool}-rate`" class="input" type="text" inputmode="decimal" autocomplete="off" maxlength="32" required :value="drafts[tool].rate_multiplier" @input="updateNumber(tool, 'rate_multiplier', $event)" @blur="normalizeDraft(tool, 'rate_multiplier')" />
        </div>
      </div>
      <p class="input-hint mt-1">{{ t('admin.groups.deepSeekTools.finalPrice', { price: finalPrice(tool) }) }}</p>
    </div>
    <p class="input-hint">{{ t('admin.groups.deepSeekTools.pricingHint') }}</p>
  </div>
</template>

<script setup lang="ts">
import { reactive, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { DeepSeekToolPricing } from '@/types'

const { t } = useI18n()
const tools = ['search', 'fetch'] as const
type Tool = typeof tools[number]
type NumericField = 'price_per_call' | 'rate_multiplier'
const props = defineProps<{
  value: DeepSeekToolPricing
  groupRate: number | string | null
  idPrefix: string
}>()
const emit = defineEmits<{ (event: 'update:value', value: DeepSeekToolPricing): void }>()
const displayValue = (value: number | null | undefined) => value == null ? '' : String(value)
const parseDecimal = (raw: string): number | null => {
  if (!/^(?:\d+(?:\.\d*)?|\.\d+)$/.test(raw)) return null
  const value = Number(raw)
  return Number.isFinite(value) ? value : null
}
const drafts = reactive(Object.fromEntries(tools.map(tool => [tool, {
  price_per_call: displayValue(props.value[tool]?.price_per_call),
  rate_multiplier: displayValue(props.value[tool]?.rate_multiplier === undefined ? 1 : props.value[tool].rate_multiplier),
}])) as Record<Tool, Record<NumericField, string>>)

watch(() => props.value, value => {
  for (const tool of tools) {
    for (const field of ['price_per_call', 'rate_multiplier'] as const) {
      const current = field === 'rate_multiplier' && value[tool]?.[field] === undefined ? 1 : value[tool]?.[field]
      if (parseDecimal(drafts[tool][field]) !== current) {
        drafts[tool][field] = displayValue(current)
      }
    }
  }
}, { deep: true })

const update = (tool: 'search' | 'fetch', field: string, value: string | number | null) => {
  emit('update:value', {
    ...props.value,
    [tool]: { ...props.value[tool], [field]: value },
  } as DeepSeekToolPricing)
}
const updateNumber = (tool: Tool, field: NumericField, event: Event) => {
  const input = event.target as HTMLInputElement
  drafts[tool][field] = input.value
  update(tool, field, parseDecimal(input.value))
}
const normalizeDraft = (tool: Tool, field: NumericField) => {
  const value = parseDecimal(drafts[tool][field])
  if (value !== null) drafts[tool][field] = String(value)
}
const finalPrice = (tool: 'search' | 'fetch') => {
  const entry = props.value[tool]
  if (entry.price_per_call === null || !Number.isFinite(entry.price_per_call)) return '-'
  const rate = entry.mode === 'fixed' ? 1 : entry.mode === 'independent' ? entry.rate_multiplier ?? 1 : Number(props.groupRate ?? 1)
  const amount = entry.price_per_call * rate
  return Number.isFinite(amount) ? `$${amount.toLocaleString('en-US', { maximumFractionDigits: 12 })}` : '-'
}
</script>
