<template>
  <section class="border-t border-gray-100 pt-4 dark:border-dark-700">
    <div class="mb-4 flex items-center justify-between gap-3">
      <h3 class="text-sm font-medium text-gray-900 dark:text-white">{{ t('headerButtons.title') }}</h3>
      <button type="button" class="btn btn-secondary inline-flex items-center gap-2" :disabled="modelValue.filter(item => item.action === 'external').length >= 6" @click="add">
        <Icon name="plus" size="sm" />{{ t('common.add') }}
      </button>
    </div>
    <div v-for="(button, index) in modelValue" :key="index" class="grid grid-cols-1 gap-3 border-b border-gray-100 py-3 dark:border-dark-700 lg:grid-cols-[minmax(0,1fr)_minmax(0,2fr)_auto]">
      <label class="min-w-0 text-xs text-gray-500">
        {{ t('headerButtons.name') }}
        <input :value="button.name" class="input mt-1 w-full" maxlength="32" :aria-label="t('headerButtons.name')" @input="update(index, { name: ($event.target as HTMLInputElement).value })" />
      </label>
      <label class="min-w-0 text-xs text-gray-500">
        {{ t('headerButtons.url') }}
        <input :value="button.url" class="input mt-1 w-full" type="url" maxlength="2048" :placeholder="button.action === 'external' ? 'https://' : t('headerButtons.defaultAction')" :aria-label="t('headerButtons.url')" @input="update(index, { url: ($event.target as HTMLInputElement).value })" />
      </label>
      <div class="flex items-center gap-2 lg:pt-5">
        <Toggle :model-value="button.enabled" :aria-label="t('headerButtons.enabled')" @update:model-value="update(index, { enabled: $event })" />
        <button type="button" class="icon-button" :title="t('headerButtons.up')" :aria-label="t('headerButtons.up')" :disabled="index === 0" @click="move(index, -1)"><Icon name="chevronUp" size="sm" /></button>
        <button type="button" class="icon-button" :title="t('headerButtons.down')" :aria-label="t('headerButtons.down')" :disabled="index === modelValue.length - 1" @click="move(index, 1)"><Icon name="chevronDown" size="sm" /></button>
        <button v-if="button.action === 'external'" type="button" class="icon-button text-red-500" :title="t('common.delete')" :aria-label="t('common.delete')" @click="emit('update:modelValue', modelValue.filter((_, i) => i !== index))"><Icon name="trash" size="sm" /></button>
      </div>
    </div>
    <p v-if="!modelValue.length" class="py-4 text-sm text-gray-500">{{ t('headerButtons.empty') }}</p>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import Toggle from '@/components/common/Toggle.vue'
import type { HeaderButton } from '@/types'

const props = defineProps<{ modelValue: HeaderButton[] }>()
const emit = defineEmits<{ 'update:modelValue': [HeaderButton[]] }>()
const { t } = useI18n()
function update(index: number, patch: Partial<HeaderButton>) {
  emit('update:modelValue', props.modelValue.map((button, i) => i === index ? { ...button, ...patch } : button))
}
function add() {
  if (props.modelValue.filter(button => button.action === 'external').length >= 6) return
  emit('update:modelValue', [...props.modelValue, { action: 'external', name: '', url: '', enabled: true }])
}
function move(index: number, direction: number) {
  const buttons = [...props.modelValue]
  const target = index + direction
  if (target < 0 || target >= buttons.length) return
  ;[buttons[index], buttons[target]] = [buttons[target], buttons[index]]
  emit('update:modelValue', buttons)
}
</script>
