<template>
  <section class="border-t border-gray-200 pt-4 dark:border-dark-400">
    <h4 class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('admin.groups.imageInput.title') }}</h4>
    <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.groups.imageInput.hint') }}</p>
    <div class="mt-3 flex flex-wrap gap-2">
      <input
        v-model="newModel"
        :list="candidateListId"
        :aria-label="t('admin.groups.imageInput.model')"
        :placeholder="t('admin.groups.imageInput.model')"
        class="input min-w-0 flex-1"
        @keydown.enter.prevent="addModel"
      />
      <datalist :id="candidateListId">
        <option v-for="model in candidates" :key="model" :value="model" />
      </datalist>
      <button type="button" class="btn btn-secondary shrink-0" @click="addModel">
        {{ t('admin.groups.imageInput.add') }}
      </button>
    </div>
    <p v-if="errorKey" class="mt-1 text-xs text-red-600" role="alert">{{ t(errorKey) }}</p>
    <div v-if="models.length" class="mt-3 divide-y divide-gray-200 rounded-md border border-gray-200 dark:divide-dark-400 dark:border-dark-400">
      <div v-for="model in models" :key="model" class="flex flex-wrap items-center justify-between gap-2 px-3 py-2">
        <span class="min-w-0 break-all text-sm text-gray-700 dark:text-gray-300">{{ model }}</span>
        <select
          class="input w-36 shrink-0"
          :aria-label="`${model}: ${t('admin.groups.imageInput.title')}`"
          :value="modelValue[model] ? 'enabled' : 'disabled'"
          @change="updateModel(model, ($event.target as HTMLSelectElement).value)"
        >
          <option value="auto">{{ t('admin.groups.imageInput.auto') }}</option>
          <option value="enabled">{{ t('admin.groups.imageInput.enabled') }}</option>
          <option value="disabled">{{ t('admin.groups.imageInput.disabled') }}</option>
        </select>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'

const props = defineProps<{
  modelValue: Record<string, boolean>
  candidates: string[]
  candidateListId: string
}>()
const emit = defineEmits<{ 'update:modelValue': [value: Record<string, boolean>] }>()
const { t } = useI18n()
const newModel = ref('')
const errorKey = ref('')
const models = computed(() => Object.keys(props.modelValue).sort())

function addModel() {
  const model = newModel.value.trim()
  if (!model || model.length > 100 || /[*?\x00]/.test(model) ||
      ['__proto__', 'prototype', 'constructor'].includes(model)) {
    errorKey.value = 'admin.groups.imageInput.invalidModel'
    return
  }
  if (models.value.some((existing) => existing.toLowerCase() === model.toLowerCase()) || models.value.length >= 256) {
    errorKey.value = 'admin.groups.imageInput.duplicateOrLimit'
    return
  }
  emit('update:modelValue', { ...props.modelValue, [model]: true })
  newModel.value = ''
  errorKey.value = ''
}

function updateModel(model: string, value: string) {
  const next = { ...props.modelValue }
  if (value === 'auto') delete next[model]
  else next[model] = value === 'enabled'
  emit('update:modelValue', next)
}
</script>
