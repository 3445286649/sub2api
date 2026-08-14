<template>
  <article class="flex min-h-[280px] flex-col overflow-hidden rounded-lg border border-emerald-200 bg-white transition-shadow hover:shadow-lg dark:border-emerald-900/60 dark:bg-dark-800">
    <div class="h-1.5 bg-emerald-500" />
    <div class="flex flex-1 flex-col p-4">
      <div class="mb-3 flex items-start justify-between gap-3">
        <div class="min-w-0 flex-1">
          <h3 class="h-12 break-words text-base font-bold leading-6 text-gray-900 line-clamp-2 dark:text-white">{{ product.name }}</h3>
          <p v-if="product.description" class="mt-1 text-xs leading-relaxed text-gray-500 line-clamp-2 dark:text-dark-400">{{ product.description }}</p>
        </div>
        <div class="shrink-0 text-right">
          <div class="flex items-baseline justify-end gap-1">
            <span class="text-2xl font-extrabold text-emerald-600 dark:text-emerald-400">{{ product.points_price }}</span>
            <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('points.unit') }}</span>
          </div>
          <span v-if="product.original_points_price" class="text-xs text-gray-400 line-through dark:text-dark-500">
            {{ product.original_points_price }} {{ t('points.unit') }}
          </span>
        </div>
      </div>

      <div class="mb-3 rounded-lg bg-gray-50 px-3 py-2 text-xs dark:bg-dark-700/50">
        <div class="flex items-center justify-between">
          <span class="text-gray-500 dark:text-dark-400">{{ t('points.balanceReward') }}</span>
          <span class="font-semibold text-emerald-700 dark:text-emerald-300">${{ product.balance_amount.toFixed(2) }}</span>
        </div>
        <div v-if="product.stock_total != null" class="mt-1 flex items-center justify-between">
          <span class="text-gray-500 dark:text-dark-400">{{ t('points.remainingStock') }}</span>
          <span class="font-medium text-gray-700 dark:text-gray-300">{{ Math.max(0, product.stock_total - product.stock_redeemed) }}</span>
        </div>
        <div v-if="product.per_user_limit" class="mt-1 flex items-center justify-between">
          <span class="text-gray-500 dark:text-dark-400">{{ t('points.perUserLimit') }}</span>
          <span class="font-medium text-gray-700 dark:text-gray-300">{{ product.per_user_limit }}</span>
        </div>
      </div>

      <ul v-if="features.length" class="mb-4 space-y-1">
        <li v-for="feature in features" :key="feature" class="flex items-start gap-1.5 text-xs text-gray-600 dark:text-gray-300">
          <Icon name="check" size="sm" class="mt-0.5 shrink-0 text-emerald-500" />
          <span>{{ feature }}</span>
        </li>
      </ul>

      <div class="flex-1" />
      <button type="button" class="btn btn-primary w-full" :disabled="disabled || loading" @click="emit('redeem', product)">
        <span v-if="loading">{{ t('common.processing') }}</span>
        <span v-else>{{ disabled ? t('points.insufficient') : t('points.redeemNow') }}</span>
      </button>
    </div>
  </article>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { PointsProduct } from '@/types/points'

const props = defineProps<{ product: PointsProduct; availablePoints: number; loading?: boolean }>()
const emit = defineEmits<{ redeem: [product: PointsProduct] }>()
const { t } = useI18n()
const features = computed(() => props.product.features.split('\n').map(item => item.trim()).filter(Boolean))
const disabled = computed(() => props.availablePoints < props.product.points_price)
</script>
