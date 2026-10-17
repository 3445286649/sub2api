<template>
  <label :class="layout === 'inline' ? 'flex cursor-pointer items-center gap-1.5 whitespace-nowrap sm:flex-col sm:gap-0.5' : 'flex cursor-pointer flex-col items-center gap-0.5'">
    <span class="text-xs text-gray-500 dark:text-gray-400 whitespace-nowrap">{{ label }}</span>
    <button
      type="button"
      role="switch"
      :aria-checked="checked"
      @click="emit('toggle')"
      :class="[
        'relative inline-flex h-5 w-9 shrink-0 rounded-full border-2 border-transparent transition-colors duration-200',
        checked ? 'bg-primary-500' : 'bg-gray-300 dark:bg-dark-600',
      ]"
    >
      <span :class="[
        'pointer-events-none inline-block h-4 w-4 rounded-full bg-white shadow-sm transition-transform duration-200',
        checked ? 'translate-x-4' : 'translate-x-0',
      ]" />
    </button>
  </label>
</template>

<script setup lang="ts">
withDefaults(defineProps<{ label: string; checked: boolean; layout?: 'stacked' | 'inline' }>(), { layout: 'stacked' })
const emit = defineEmits<{ toggle: [] }>()
</script>
