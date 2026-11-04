<template>
  <ol class="flex items-center gap-2" :aria-label="t('invoice.views.apply')">
    <li v-for="(step, index) in steps" :key="step" class="flex min-w-0 flex-1 items-center gap-2">
      <button
        type="button"
        class="flex min-w-0 items-center gap-2 rounded-lg py-1 text-left disabled:cursor-default"
        :disabled="index >= current"
        :aria-current="index === current ? 'step' : undefined"
        @click="emit('go', index)"
      >
        <span
          class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full text-xs font-semibold transition-colors"
          :class="index < current
            ? 'bg-primary-500 text-white'
            : index === current
              ? 'bg-primary-500/15 text-primary-700 ring-2 ring-primary-500 dark:text-primary-300'
              : 'bg-gray-100 text-gray-400 dark:bg-dark-700 dark:text-dark-400'"
        >
          <Icon v-if="index < current" name="check" size="xs" />
          <template v-else>{{ index + 1 }}</template>
        </span>
        <span
          class="truncate text-sm"
          :class="index <= current ? 'font-medium text-gray-900 dark:text-white' : 'text-gray-400 dark:text-dark-400'"
        >{{ t(`invoice.steps.${step}`) }}</span>
      </button>
      <span
        v-if="index < steps.length - 1"
        class="h-px min-w-[1rem] flex-1 transition-colors"
        :class="index < current ? 'bg-primary-400 dark:bg-primary-600' : 'bg-gray-200 dark:bg-dark-700'"
        aria-hidden="true"
      />
    </li>
  </ol>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'

defineProps<{ current: number }>()
const emit = defineEmits<{ (e: 'go', index: number): void }>()
const { t } = useI18n()
const steps = ['orders', 'title', 'confirm'] as const
</script>
