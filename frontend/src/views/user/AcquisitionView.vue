<template>
  <AppLayout>
    <div class="space-y-6">
      <div v-if="loading" class="flex justify-center py-12">
        <div class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
      </div>

      <template v-else>
        <div v-if="!summary?.campaign" class="card p-8 text-center">
          <Icon name="gift" size="xl" class="mx-auto text-gray-400" />
          <h3 class="mt-3 text-base font-semibold text-gray-900 dark:text-white">{{ t('acquisition.emptyTitle') }}</h3>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('acquisition.emptyDescription') }}</p>
        </div>

        <template v-else>
          <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
            <div class="card p-5">
              <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('acquisition.stats.validInvites') }}</p>
              <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ summary.valid_invites }}</p>
            </div>
            <div class="card p-5">
              <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('acquisition.stats.rank') }}</p>
              <p class="mt-2 text-2xl font-semibold text-primary-600 dark:text-primary-400">
                {{ summary.rank > 0 ? `#${summary.rank}` : '-' }}
              </p>
            </div>
            <div class="card p-5">
              <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('acquisition.stats.tickets') }}</p>
              <p class="mt-2 text-2xl font-semibold text-amber-600 dark:text-amber-400">{{ summary.ticket_count }}</p>
            </div>
            <div class="card p-5">
              <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('acquisition.stats.pool') }}</p>
              <p class="mt-2 text-2xl font-semibold text-emerald-600 dark:text-emerald-400">
                {{ formatCurrency(summary.campaign.leaderboard_pool_usd) }}
              </p>
            </div>
          </div>

          <div class="card p-6">
            <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
              <div>
                <div class="flex flex-wrap items-center gap-2">
                  <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ summary.campaign.name }}</h3>
                  <span class="rounded-full bg-primary-50 px-2 py-0.5 text-xs font-medium text-primary-700 dark:bg-primary-900/30 dark:text-primary-300">
                    {{ campaignStatusLabel(summary.campaign.status) }}
                  </span>
                </div>
                <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">
                  {{ formatDateTime(summary.campaign.starts_at) }} - {{ formatDateTime(summary.campaign.ends_at) }}
                </p>
              </div>
              <div class="flex flex-wrap gap-2 text-xs">
                <span class="rounded-full bg-gray-100 px-2.5 py-1 text-gray-700 dark:bg-dark-800 dark:text-gray-300">
                  {{ summary.campaign.leaderboard_enabled ? t('acquisition.flags.leaderboardOn') : t('acquisition.flags.leaderboardOff') }}
                </span>
                <span class="rounded-full bg-gray-100 px-2.5 py-1 text-gray-700 dark:bg-dark-800 dark:text-gray-300">
                  {{ summary.campaign.lottery_enabled ? t('acquisition.flags.lotteryOn') : t('acquisition.flags.lotteryOff') }}
                </span>
              </div>
            </div>

            <div class="mt-5 grid gap-4 md:grid-cols-2">
              <div class="space-y-2">
                <p class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('acquisition.inviteCode') }}</p>
                <div class="flex items-center gap-2 rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-900">
                  <code class="min-w-0 flex-1 truncate text-sm font-semibold text-gray-900 dark:text-white">{{ summary.aff_code || '-' }}</code>
                  <button class="btn btn-secondary btn-sm" :disabled="!summary.aff_code" @click="copyCode">
                    <Icon name="copy" size="sm" />
                    <span>{{ t('acquisition.copyCode') }}</span>
                  </button>
                </div>
              </div>
              <div class="space-y-2">
                <p class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('acquisition.inviteLink') }}</p>
                <div class="flex items-center gap-2 rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-900">
                  <code class="min-w-0 flex-1 truncate text-sm text-gray-700 dark:text-gray-300">{{ inviteLink }}</code>
                  <button class="btn btn-secondary btn-sm" :disabled="!inviteLink" @click="copyInviteLink">
                    <Icon name="copy" size="sm" />
                    <span>{{ t('acquisition.copyLink') }}</span>
                  </button>
                </div>
              </div>
            </div>
          </div>

          <div class="grid gap-6 xl:grid-cols-[1.2fr_0.8fr]">
            <div class="card p-6">
              <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('acquisition.leaderboard.title') }}</h3>
              <div class="mt-4 overflow-x-auto">
                <table class="w-full min-w-[560px] text-left text-sm">
                  <thead>
                    <tr class="border-b border-gray-200 text-gray-500 dark:border-dark-700 dark:text-dark-400">
                      <th class="px-3 py-2 font-medium">{{ t('acquisition.leaderboard.rank') }}</th>
                      <th class="px-3 py-2 font-medium">{{ t('acquisition.leaderboard.user') }}</th>
                      <th class="px-3 py-2 font-medium text-right">{{ t('acquisition.leaderboard.invites') }}</th>
                      <th class="px-3 py-2 font-medium text-right">{{ t('acquisition.leaderboard.reward') }}</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-if="summary.leaderboard.length === 0">
                      <td colspan="4" class="px-3 py-6 text-center text-gray-500">{{ t('acquisition.leaderboard.empty') }}</td>
                    </tr>
                    <tr v-for="row in summary.leaderboard" :key="row.user_id" class="border-b border-gray-100 last:border-b-0 dark:border-dark-800">
                      <td class="px-3 py-3 font-semibold text-gray-900 dark:text-white">#{{ row.rank }}</td>
                      <td class="px-3 py-3">
                        <div class="font-medium text-gray-900 dark:text-white">{{ row.username || row.email || `#${row.user_id}` }}</div>
                        <div class="text-xs text-gray-500">{{ row.email }}</div>
                      </td>
                      <td class="px-3 py-3 text-right text-gray-700 dark:text-gray-300">{{ row.invite_count }}</td>
                      <td class="px-3 py-3 text-right font-medium text-emerald-600 dark:text-emerald-400">{{ formatCurrency(row.reward_amount) }}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>

            <div class="card p-6">
              <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('acquisition.rewards.title') }}</h3>
              <div v-if="summary.rewards.length === 0" class="mt-4 rounded-lg border border-dashed border-gray-300 p-6 text-center text-sm text-gray-500 dark:border-dark-700 dark:text-dark-400">
                {{ t('acquisition.rewards.empty') }}
              </div>
              <div v-else class="mt-4 space-y-3">
                <div v-for="reward in summary.rewards" :key="reward.id" class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
                  <div class="flex items-center justify-between gap-3">
                    <div>
                      <p class="text-sm font-medium text-gray-900 dark:text-white">{{ rewardTitle(reward) }}</p>
                      <p class="mt-0.5 text-xs text-gray-500">{{ reward.status }} · {{ formatDateTime(reward.created_at) }}</p>
                    </div>
                    <p class="font-semibold text-emerald-600 dark:text-emerald-400">{{ formatCurrency(reward.amount) }}</p>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </template>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import acquisitionAPI, { type AcquisitionReward, type AcquisitionUserSummary } from '@/api/acquisition'
import { useAppStore } from '@/stores/app'
import { useClipboard } from '@/composables/useClipboard'
import { formatCurrency, formatDateTime } from '@/utils/format'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()
const { copyToClipboard } = useClipboard()

const loading = ref(true)
const summary = ref<AcquisitionUserSummary | null>(null)

const inviteLink = computed(() => {
  const code = summary.value?.aff_code
  if (!code) return ''
  if (summary.value?.invite_link) return summary.value.invite_link
  if (typeof window === 'undefined') return `/register?aff=${encodeURIComponent(code)}`
  return `${window.location.origin}/register?aff=${encodeURIComponent(code)}`
})

async function loadSummary(): Promise<void> {
  loading.value = true
  try {
    summary.value = await acquisitionAPI.getCurrent()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('acquisition.loadFailed')))
  } finally {
    loading.value = false
  }
}

async function copyCode(): Promise<void> {
  if (!summary.value?.aff_code) return
  await copyToClipboard(summary.value.aff_code, t('acquisition.codeCopied'))
}

async function copyInviteLink(): Promise<void> {
  if (!inviteLink.value) return
  await copyToClipboard(inviteLink.value, t('acquisition.linkCopied'))
}

function campaignStatusLabel(status: string): string {
  return t(`acquisition.status.${status}`, status)
}

function rewardTitle(reward: AcquisitionReward): string {
  if (reward.reward_type === 'leaderboard') {
    return t('acquisition.rewards.leaderboard', { rank: reward.rank || '-' })
  }
  return t('acquisition.rewards.lottery', { prize: reward.prize_name || '-' })
}

onMounted(() => {
  void loadSummary()
})
</script>
