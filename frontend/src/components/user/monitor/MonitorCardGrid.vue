<template>
  <div>
    <div
      v-if="loading && items.length === 0"
      class="grid gap-5 grid-cols-1 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4"
    >
      <div
        v-for="i in 6"
        :key="i"
        class="p-5 rounded-2xl min-h-[280px] bg-white/70 dark:bg-dark-800/60 border border-gray-200/80 dark:border-dark-700/70 animate-pulse"
      >
        <div class="flex items-start gap-3">
          <div class="w-9 h-9 rounded-xl bg-gray-200 dark:bg-dark-700"></div>
          <div class="flex-1 space-y-2">
            <div class="h-4 w-2/3 rounded bg-gray-200 dark:bg-dark-700"></div>
            <div class="h-3 w-1/2 rounded bg-gray-200 dark:bg-dark-700"></div>
          </div>
          <div class="h-6 w-16 rounded-full bg-gray-200 dark:bg-dark-700"></div>
        </div>
        <div class="mt-5 grid grid-cols-2 gap-2">
          <div class="h-16 rounded-xl bg-gray-100 dark:bg-dark-900/40"></div>
          <div class="h-16 rounded-xl bg-gray-100 dark:bg-dark-900/40"></div>
        </div>
        <div class="mt-6 h-5 w-full rounded bg-gray-100 dark:bg-dark-900/40"></div>
      </div>
    </div>

    <EmptyState
      v-else-if="items.length === 0"
      :title="t('channelStatus.empty.title')"
      :description="t('channelStatus.empty.description')"
    />

    <div v-else class="min-w-0">
      <div class="mb-6 flex max-w-full gap-2 overflow-x-auto border-b border-gray-200 pb-3 dark:border-dark-700" :aria-label="t('monitorCommon.displayCategory')">
        <button type="button" class="shrink-0 rounded-md px-3 py-2 text-sm" :class="categoryButtonClass(selectedCategory === null)" :aria-pressed="selectedCategory === null" @click="selectedCategory = null">
          {{ t('monitorCommon.allCategories') }} <span class="ml-1 tabular-nums opacity-70">({{ items.length }})</span>
        </button>
        <button v-for="group in categories" :key="group.name" type="button" class="shrink-0 rounded-md px-3 py-2 text-sm" :class="categoryButtonClass(selectedCategory === group.name)" :aria-pressed="selectedCategory === group.name" @click="selectedCategory = group.name">
          {{ group.name }} <span class="ml-1 tabular-nums opacity-70">({{ group.items.length }})</span>
        </button>
      </div>
      <section v-for="group in visibleCategories" :key="group.name" class="mb-8 last:mb-0" :aria-label="group.name">
        <h2 class="mb-4 flex flex-wrap items-center gap-2 text-base font-semibold text-gray-900 dark:text-gray-100">
          <span class="[overflow-wrap:anywhere]">{{ group.name }}</span>
          <span class="text-xs font-normal text-gray-500 dark:text-gray-400">{{ t('monitorCommon.categoryCount', { count: group.items.length }) }}</span>
        </h2>
        <div class="grid grid-cols-1 gap-5 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
      <MonitorCard
        v-for="(item, idx) in group.items"
        :key="item.id"
        :item="item"
        :window="window"
        :availability-value="resolveAvailability(item)"
        :countdown-seconds="countdownSeconds"
        class="cp-card-reveal"
        :style="{ '--reveal-delay': `${Math.min(idx * 40, 360)}ms` }"
        @click="emit('cardClick', item)"
      />
        </div>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { groupMonitorCategories } from '@/utils/monitorDisplayCategory'
import { useI18n } from 'vue-i18n'
import type { UserMonitorView, UserMonitorDetail } from '@/api/channelMonitor'
import EmptyState from '@/components/common/EmptyState.vue'
import MonitorCard from './MonitorCard.vue'

const props = defineProps<{
  items: UserMonitorView[]
  window: '7d' | '15d' | '30d'
  countdownSeconds: number
  loading: boolean
  detailCache: Record<number, UserMonitorDetail>
}>()

const emit = defineEmits<{
  (e: 'cardClick', item: UserMonitorView): void
}>()

const { t } = useI18n()
const selectedCategory = ref<string | null>(null)
const categories = computed(() => groupMonitorCategories(props.items))
const visibleCategories = computed(() => selectedCategory.value === null
  ? categories.value : categories.value.filter(group => group.name === selectedCategory.value))
watch(categories, groups => {
  if (selectedCategory.value !== null && !groups.some(group => group.name === selectedCategory.value)) {
    selectedCategory.value = null
  }
})

function categoryButtonClass(active: boolean) {
  return active ? 'bg-primary-600 text-white' : 'text-gray-600 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-800'
}

function resolveAvailability(item: UserMonitorView): number | null {
  if (props.window === '7d') {
    return item.availability_7d ?? null
  }
  const detail = props.detailCache[item.id]
  if (!detail) return null
  const primary = detail.models.find(m => m.model === item.primary_model)
  if (!primary) return null
  return props.window === '15d' ? primary.availability_15d ?? null : primary.availability_30d ?? null
}
</script>
