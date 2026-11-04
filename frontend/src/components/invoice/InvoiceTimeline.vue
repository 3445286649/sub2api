<template>
  <ol class="relative space-y-4 pl-6">
    <span class="absolute bottom-2 left-[7px] top-2 w-px bg-gray-200 dark:bg-dark-700" aria-hidden="true" />
    <li v-for="node in nodes" :key="node.key" class="relative">
      <span
        class="absolute -left-6 top-0.5 flex h-[15px] w-[15px] items-center justify-center rounded-full ring-4 ring-white dark:ring-dark-800"
        :class="node.done ? node.dotClass : 'bg-gray-200 dark:bg-dark-600'"
        aria-hidden="true"
      >
        <span v-if="node.current" class="h-1.5 w-1.5 rounded-full bg-white" />
      </span>
      <p class="text-sm" :class="node.done ? 'font-medium text-gray-900 dark:text-white' : 'text-gray-400 dark:text-dark-400'">
        {{ t(`invoice.timeline.${node.key}`) }}
      </p>
      <p v-if="node.time" class="mt-0.5 text-xs tabular-nums text-gray-500 dark:text-gray-400">{{ formatDateTimeToMinute(node.time) }}</p>
    </li>
  </ol>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { formatDateTimeToMinute } from '@/utils/format'
import type { InvoiceRequest } from '@/types/invoice'

const props = defineProps<{ request: InvoiceRequest }>()
const { t } = useI18n()

interface TimelineNode {
  key: 'submitted' | 'processing' | 'issued' | 'emailed' | 'rejected'
  time?: string | null
  done: boolean
  current: boolean
  dotClass: string
}

const nodes = computed<TimelineNode[]>(() => {
  const r = props.request
  const ok = 'bg-primary-500'
  if (r.status === 'rejected') {
    return [
      { key: 'submitted', time: r.created_at, done: true, current: false, dotClass: ok },
      { key: 'rejected', time: r.rejected_at, done: true, current: true, dotClass: 'bg-red-500' },
    ]
  }
  const reached = { pending: 0, processing: 1, completed: 2, rejected: 0 }[r.status] + (r.email_sent_at ? 1 : 0)
  const list: Omit<TimelineNode, 'done' | 'current' | 'dotClass'>[] = [
    { key: 'submitted', time: r.created_at },
    { key: 'processing', time: r.processing_at },
    { key: 'issued', time: r.completed_at },
    { key: 'emailed', time: r.email_sent_at },
  ]
  return list.map((node, i) => ({
    ...node,
    done: i <= reached,
    current: i === reached,
    dotClass: r.status === 'completed' ? 'bg-emerald-500' : ok,
  }))
})
</script>
