<template>
  <AppLayout>
    <div class="space-y-6">
      <div v-if="loading" class="flex justify-center py-12">
        <div
          class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"
        ></div>
      </div>

      <template v-else-if="detail">
        <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <div class="card p-5">
            <p class="flex items-center gap-1.5 text-sm text-gray-500 dark:text-dark-400">
              <Icon name="dollar" size="sm" class="text-primary-500" />
              {{ t('affiliate.stats.rebateRate') }}
            </p>
            <p class="mt-2 text-2xl font-semibold text-primary-600 dark:text-primary-400">
              {{ formattedRebateRate }}<span class="ml-0.5 text-base font-medium">%</span>
            </p>
            <p class="mt-1 text-xs text-gray-400 dark:text-dark-500">
              {{ t('affiliate.stats.rebateRateHint') }}
            </p>
          </div>
          <div class="card p-5">
            <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('affiliate.stats.invitedUsers') }}</p>
            <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">
              {{ formatCount(detail.aff_count) }}
            </p>
          </div>
          <div class="card p-5">
            <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('affiliate.stats.availableQuota') }}</p>
            <p class="mt-2 text-2xl font-semibold text-emerald-600 dark:text-emerald-400">
              {{ formatCurrency(detail.aff_quota) }}
            </p>
          </div>
          <div class="card p-5">
            <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('affiliate.stats.totalQuota') }}</p>
            <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">
              {{ formatCurrency(detail.aff_history_quota) }}
            </p>
            <p v-if="detail.aff_frozen_quota > 0" class="mt-1 text-xs text-amber-600 dark:text-amber-400">
              {{ t('affiliate.stats.frozenQuota') }}: {{ formatCurrency(detail.aff_frozen_quota) }}
            </p>
          </div>
        </div>

        <div class="card p-6">
          <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('affiliate.title') }}</h3>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('affiliate.description') }}</p>

          <div class="mt-5 grid gap-4 md:grid-cols-2">
            <div class="space-y-2">
              <p class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('affiliate.yourCode') }}</p>
              <div class="flex flex-col items-stretch gap-2 rounded-xl border border-gray-200 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-900 sm:flex-row sm:items-center">
                <code class="min-w-0 break-all text-sm font-semibold text-gray-900 dark:text-white sm:flex-1 sm:truncate">{{ detail.aff_code }}</code>
                <button class="btn btn-secondary btn-sm w-full sm:w-auto sm:shrink-0" @click="copyCode">
                  <Icon name="copy" size="sm" />
                  <span>{{ t('affiliate.copyCode') }}</span>
                </button>
              </div>
            </div>

            <div class="space-y-2">
              <p class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('affiliate.inviteLink') }}</p>
              <div class="flex flex-col items-stretch gap-2 rounded-xl border border-gray-200 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-900 sm:flex-row sm:items-center">
                <code class="min-w-0 break-all text-sm text-gray-700 dark:text-gray-300 sm:flex-1 sm:truncate">{{ inviteLink }}</code>
                <button class="btn btn-secondary btn-sm w-full sm:w-auto sm:shrink-0" @click="copyInviteLink">
                  <Icon name="copy" size="sm" />
                  <span>{{ t('affiliate.copyLink') }}</span>
                </button>
              </div>
            </div>
          </div>

          <div class="mt-5 rounded-xl border border-primary-200 bg-primary-50 p-4 dark:border-primary-900/40 dark:bg-primary-900/20">
            <p class="text-sm font-medium text-primary-800 dark:text-primary-200">{{ t('affiliate.tips.title') }}</p>
            <ul class="mt-2 space-y-1 text-sm text-primary-700 dark:text-primary-300">
              <li>1. {{ t('affiliate.tips.line1') }}</li>
              <li>2. {{ t('affiliate.tips.line2', { rate: `${formattedRebateRate}%` }) }}</li>
              <li>3. {{ t('affiliate.tips.line3') }}</li>
              <li v-if="detail.aff_frozen_quota > 0">4. {{ t('affiliate.tips.line4') }}</li>
            </ul>
          </div>
        </div>

        <div class="card p-6">
          <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('affiliate.transfer.title') }}</h3>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('affiliate.transfer.description') }}</p>
            </div>
            <button
              class="btn btn-primary"
              :disabled="transferring || detail.aff_quota <= 0"
              @click="transferQuota"
            >
              <Icon v-if="transferring" name="refresh" size="sm" class="animate-spin" />
              <Icon v-else name="dollar" size="sm" />
              <span>{{ transferring ? t('affiliate.transfer.transferring') : t('affiliate.transfer.button') }}</span>
            </button>
          </div>
          <p v-if="detail.aff_quota <= 0" class="mt-3 text-sm text-amber-600 dark:text-amber-400">
            {{ t('affiliate.transfer.empty') }}
          </p>
        </div>

        <div class="card p-6">
          <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('affiliate.invitees.title') }}</h3>
          <div v-if="detail.invitees.length === 0" class="mt-4 rounded-xl border border-dashed border-gray-300 p-6 text-center text-sm text-gray-500 dark:border-dark-700 dark:text-dark-400">
            {{ t('affiliate.invitees.empty') }}
          </div>
          <template v-else>
            <div class="mt-4 divide-y divide-gray-100 dark:divide-dark-800 md:hidden">
              <section v-for="item in detail.invitees" :key="item.user_id" class="py-4 first:pt-0 last:pb-0">
                <div class="flex min-w-0 items-start justify-between gap-3">
                  <div class="min-w-0">
                    <p class="truncate text-sm font-medium text-gray-900 dark:text-white">{{ item.username || '-' }}</p>
                    <p class="mt-0.5 truncate text-xs text-gray-500 dark:text-dark-400">{{ item.email || '-' }}</p>
                  </div>
                  <span class="shrink-0 rounded-full px-2.5 py-1 text-xs font-medium" :class="pointsStatusClass(item)">
                    {{ pointsStatusLabel(item) }}
                  </span>
                </div>

                <div class="mt-3 grid grid-cols-2 gap-x-4 gap-y-3 text-sm">
                  <div class="col-span-2">
                    <div class="flex items-center justify-between gap-3 text-xs">
                      <span class="text-gray-500 dark:text-dark-400">{{ t('affiliate.invitees.columns.progress') }}</span>
                      <span class="font-medium text-gray-800 dark:text-gray-200">{{ pointsProgress(item) }}</span>
                    </div>
                    <div class="mt-1.5 h-1.5 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700">
                      <div class="h-full rounded-full bg-primary-500 transition-[width]" :style="{ width: `${pointsProgressPercent(item)}%` }"></div>
                    </div>
                  </div>
                  <div>
                    <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('affiliate.invitees.columns.reward') }}</p>
                    <p class="mt-1 font-medium text-gray-900 dark:text-white">{{ rewardPointsLabel(item) }}</p>
                  </div>
                  <div>
                    <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('affiliate.invitees.columns.timeline') }}</p>
                    <p class="mt-1 text-gray-700 dark:text-gray-300">{{ pointsTimeline(item) }}</p>
                  </div>
                  <p class="col-span-2 text-xs leading-5 text-gray-500 dark:text-dark-400">{{ pointsStatusReason(item) }}</p>
                  <div>
                    <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('affiliate.invitees.columns.rebate') }}</p>
                    <p class="mt-1 font-medium text-emerald-600 dark:text-emerald-400">{{ formatCurrency(item.total_rebate) }}</p>
                  </div>
                  <div>
                    <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('affiliate.invitees.columns.joinedAt') }}</p>
                    <p class="mt-1 text-gray-700 dark:text-gray-300">{{ formatDateTime(item.created_at) || '-' }}</p>
                  </div>
                </div>
              </section>
            </div>

            <div class="mt-4 hidden overflow-x-auto md:block">
            <table class="w-full min-w-[1040px] text-left text-sm">
              <thead>
                <tr class="border-b border-gray-200 text-gray-500 dark:border-dark-700 dark:text-dark-400">
                  <th class="px-3 py-2 font-medium">{{ t('affiliate.invitees.columns.user') }}</th>
                  <th class="px-3 py-2 font-medium">{{ t('affiliate.invitees.columns.status') }}</th>
                  <th class="w-44 px-3 py-2 font-medium">{{ t('affiliate.invitees.columns.progress') }}</th>
                  <th class="px-3 py-2 font-medium">{{ t('affiliate.invitees.columns.reward') }}</th>
                  <th class="px-3 py-2 font-medium">{{ t('affiliate.invitees.columns.timeline') }}</th>
                  <th class="px-3 py-2 font-medium text-right">{{ t('affiliate.invitees.columns.rebate') }}</th>
                  <th class="px-3 py-2 font-medium">{{ t('affiliate.invitees.columns.joinedAt') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="item in detail.invitees"
                  :key="item.user_id"
                  class="border-b border-gray-100 last:border-b-0 dark:border-dark-800"
                >
                  <td class="px-3 py-3">
                    <p class="font-medium text-gray-900 dark:text-white">{{ item.username || '-' }}</p>
                    <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">{{ item.email || '-' }}</p>
                  </td>
                  <td class="px-3 py-3 align-top">
                    <span class="inline-flex rounded-full px-2.5 py-1 text-xs font-medium" :class="pointsStatusClass(item)">
                      {{ pointsStatusLabel(item) }}
                    </span>
                    <p class="mt-1.5 max-w-52 text-xs leading-5 text-gray-500 dark:text-dark-400">{{ pointsStatusReason(item) }}</p>
                  </td>
                  <td class="px-3 py-3 align-top">
                    <div class="flex items-center justify-between gap-3 text-xs">
                      <span class="font-medium text-gray-800 dark:text-gray-200">{{ pointsProgress(item) }}</span>
                      <span class="text-gray-400">{{ pointsProgressPercent(item) }}%</span>
                    </div>
                    <div class="mt-2 h-1.5 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700">
                      <div class="h-full rounded-full bg-primary-500 transition-[width]" :style="{ width: `${pointsProgressPercent(item)}%` }"></div>
                    </div>
                  </td>
                  <td class="px-3 py-3 font-medium text-gray-900 dark:text-white">{{ rewardPointsLabel(item) }}</td>
                  <td class="px-3 py-3 text-xs leading-5 text-gray-700 dark:text-gray-300">{{ pointsTimeline(item) }}</td>
                  <td class="px-3 py-3 text-right font-medium text-emerald-600 dark:text-emerald-400">{{ formatCurrency(item.total_rebate) }}</td>
                  <td class="px-3 py-3 text-gray-700 dark:text-gray-300">{{ formatDateTime(item.created_at) || '-' }}</td>
                </tr>
              </tbody>
            </table>
          </div>
          </template>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import userAPI from '@/api/user'
import type { AffiliateInvitee, UserAffiliateDetail } from '@/types'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { useClipboard } from '@/composables/useClipboard'
import { formatCurrency, formatDateTime } from '@/utils/format'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const { copyToClipboard } = useClipboard()

const loading = ref(true)
const transferring = ref(false)
const detail = ref<UserAffiliateDetail | null>(null)

const inviteLink = computed(() => {
  if (!detail.value) return ''
  if (typeof window === 'undefined') return `/register?aff=${encodeURIComponent(detail.value.aff_code)}`
  return `${window.location.origin}/register?aff=${encodeURIComponent(detail.value.aff_code)}`
})

// Rebate rate is a percentage in the range [0, 100]; backend already clamps it.
// We trim trailing zeros (e.g. 20.00 → "20", 12.50 → "12.5") for a cleaner UI.
const formattedRebateRate = computed(() => {
  const v = detail.value?.effective_rebate_rate_percent ?? 0
  const rounded = Math.round(v * 100) / 100
  return Number.isInteger(rounded) ? String(rounded) : rounded.toString()
})

function formatCount(value: number): string {
  return value.toLocaleString()
}

type InviteePointsStatus = NonNullable<AffiliateInvitee['points_status']>

const pointsStatusStyles: Record<InviteePointsStatus, string> = {
  not_recharged: 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-gray-300',
  progressing: 'bg-blue-50 text-blue-700 dark:bg-blue-950/50 dark:text-blue-300',
  pending: 'bg-amber-50 text-amber-700 dark:bg-amber-950/50 dark:text-amber-300',
  available: 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950/50 dark:text-emerald-300',
  revoked: 'bg-rose-50 text-rose-700 dark:bg-rose-950/50 dark:text-rose-300',
  expired: 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-400',
}

const pointsStatusLabelKeys: Record<InviteePointsStatus, string> = {
  not_recharged: 'affiliate.invitees.status.notRecharged',
  progressing: 'affiliate.invitees.status.progressing',
  pending: 'affiliate.invitees.status.pending',
  available: 'affiliate.invitees.status.available',
  revoked: 'affiliate.invitees.status.revoked',
  expired: 'affiliate.invitees.status.expired',
}

function normalizedPointsStatus(item: AffiliateInvitee): InviteePointsStatus | null {
  return item.points_status || null
}

function pointsStatusClass(item: AffiliateInvitee): string {
  const status = normalizedPointsStatus(item)
  return status ? pointsStatusStyles[status] : pointsStatusStyles.not_recharged
}

function pointsStatusLabel(item: AffiliateInvitee): string {
  const status = normalizedPointsStatus(item)
  return status ? t(pointsStatusLabelKeys[status]) : t('affiliate.invitees.status.syncing')
}

function formatPointsAmount(value: number | undefined): string {
  const normalized = Number.isFinite(value) ? Number(value) : 0
  const rounded = Math.round(normalized * 100) / 100
  return Number.isInteger(rounded) ? String(rounded) : rounded.toFixed(2).replace(/0+$/, '').replace(/\.$/, '')
}

function pointsProgress(item: AffiliateInvitee): string {
  if (!normalizedPointsStatus(item)) return '-'
  return `${formatPointsAmount(item.qualifying_amount)} / ${formatPointsAmount(item.threshold_amount)}`
}

function pointsProgressPercent(item: AffiliateInvitee): number {
  if (!normalizedPointsStatus(item)) return 0
  const threshold = Number(item.threshold_amount) || 0
  if (threshold <= 0) return 0
  return Math.min(100, Math.max(0, Math.round(((Number(item.qualifying_amount) || 0) / threshold) * 100)))
}

function rewardPointsLabel(item: AffiliateInvitee): string {
  if (!normalizedPointsStatus(item)) return '-'
  return t('affiliate.invitees.rewardPoints', { points: item.reward_points || 0 })
}

function pointsStatusReason(item: AffiliateInvitee): string {
  const status = normalizedPointsStatus(item)
  if (!status) return t('affiliate.invitees.reason.syncing')
  switch (status) {
    case 'progressing':
      return t('affiliate.invitees.reason.remaining', {
        amount: formatPointsAmount(Math.max(0, (Number(item.threshold_amount) || 0) - (Number(item.qualifying_amount) || 0))),
      })
    case 'pending':
      return t('affiliate.invitees.reason.pending')
    case 'available':
      return t('affiliate.invitees.reason.available')
    case 'revoked':
      return t('affiliate.invitees.reason.refundBelowThreshold')
    case 'expired':
      return t('affiliate.invitees.reason.qualificationWindowExpired', { days: item.qualification_window_days || 30 })
    default:
      return t('affiliate.invitees.reason.notRecharged')
  }
}

function pointsTimeline(item: AffiliateInvitee): string {
  const status = normalizedPointsStatus(item)
  if (!status) return '-'
  if (status === 'pending' && item.release_at) {
    return t('affiliate.invitees.timeline.releaseAt', { time: formatDateTime(item.release_at) })
  }
  if (status === 'available' && item.released_at) {
    return t('affiliate.invitees.timeline.releasedAt', { time: formatDateTime(item.released_at) })
  }
  if (status === 'revoked' && item.revoked_at) {
    return t('affiliate.invitees.timeline.revokedAt', { time: formatDateTime(item.revoked_at) })
  }
  if (item.qualification_deadline) {
    return t(status === 'expired' ? 'affiliate.invitees.timeline.expiredAt' : 'affiliate.invitees.timeline.deadline', {
      time: formatDateTime(item.qualification_deadline),
    })
  }
  return '-'
}

async function loadAffiliateDetail(silent = false): Promise<void> {
  if (!silent) {
    loading.value = true
  }
  try {
    detail.value = await userAPI.getAffiliateDetail()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('affiliate.loadFailed')))
  } finally {
    if (!silent) {
      loading.value = false
    }
  }
}

async function copyCode(): Promise<void> {
  if (!detail.value?.aff_code) return
  await copyToClipboard(detail.value.aff_code, t('affiliate.codeCopied'))
}

async function copyInviteLink(): Promise<void> {
  if (!inviteLink.value) return
  await copyToClipboard(inviteLink.value, t('affiliate.linkCopied'))
}

async function transferQuota(): Promise<void> {
  if (!detail.value || detail.value.aff_quota <= 0 || transferring.value) return
  transferring.value = true
  try {
    const resp = await userAPI.transferAffiliateQuota()
    appStore.showSuccess(t('affiliate.transfer.success', { amount: formatCurrency(resp.transferred_quota) }))
    await Promise.all([
      loadAffiliateDetail(true),
      authStore.refreshUser().catch(() => undefined),
    ])
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('affiliate.transferFailed')))
  } finally {
    transferring.value = false
  }
}

onMounted(() => {
  void loadAffiliateDetail()
})
</script>
