<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.acquisition.title') }}</h2>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('admin.acquisition.description') }}</p>
        </div>
        <button class="btn btn-primary" @click="startCreate">
          <Icon name="plus" size="sm" />
          <span>{{ t('admin.acquisition.actions.create') }}</span>
        </button>
      </div>

      <div v-if="loading" class="flex justify-center py-12">
        <div class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
      </div>

      <div v-else class="grid gap-6 xl:grid-cols-[0.95fr_1.05fr]">
        <div class="card p-6">
          <div class="mb-4 flex items-center justify-between">
            <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.acquisition.campaigns') }}</h3>
            <button class="btn btn-secondary btn-sm" @click="loadCampaigns">
              <Icon name="refresh" size="sm" />
              <span>{{ t('common.refresh') }}</span>
            </button>
          </div>
          <div v-if="campaigns.length === 0" class="rounded-lg border border-dashed border-gray-300 p-6 text-center text-sm text-gray-500 dark:border-dark-700">
            {{ t('admin.acquisition.empty') }}
          </div>
          <div v-else class="space-y-3">
            <button
              v-for="item in campaigns"
              :key="item.id"
              type="button"
              class="w-full rounded-lg border p-4 text-left transition hover:border-primary-300 hover:bg-primary-50/40 dark:hover:bg-primary-900/10"
              :class="selectedCampaign?.id === item.id ? 'border-primary-400 bg-primary-50 dark:border-primary-600 dark:bg-primary-900/20' : 'border-gray-200 dark:border-dark-700'"
              @click="selectCampaign(item.id)"
            >
              <div class="flex items-start justify-between gap-3">
                <div class="min-w-0">
                  <div class="truncate font-medium text-gray-900 dark:text-white">{{ item.name }}</div>
                  <div class="mt-1 text-xs text-gray-500">{{ formatDateTime(item.starts_at) }} - {{ formatDateTime(item.ends_at) }}</div>
                </div>
                <span class="rounded-full px-2 py-0.5 text-xs font-medium" :class="statusClass(item.status)">
                  {{ statusLabel(item.status) }}
                </span>
              </div>
              <div class="mt-3 flex flex-wrap gap-2 text-xs text-gray-500">
                <span>{{ t('admin.acquisition.pool') }} {{ formatCurrency(item.leaderboard_pool_usd) }}</span>
                <span>{{ t('admin.acquisition.prizes') }} {{ item.lottery_prize_configs.length }}</span>
              </div>
            </button>
          </div>
        </div>

        <div class="space-y-6">
          <form class="card p-6" @submit.prevent="saveCampaign">
            <div class="mb-5 flex items-center justify-between">
              <h3 class="text-base font-semibold text-gray-900 dark:text-white">
                {{ editingId ? t('admin.acquisition.form.editTitle') : t('admin.acquisition.form.createTitle') }}
              </h3>
              <button v-if="editingId" type="button" class="btn btn-secondary btn-sm" @click="startCreate">
                {{ t('admin.acquisition.actions.newDraft') }}
              </button>
            </div>

            <div class="grid gap-4 md:grid-cols-2">
              <div class="md:col-span-2">
                <label class="input-label">{{ t('admin.acquisition.form.name') }}</label>
                <input v-model="form.name" class="input" type="text" />
              </div>
              <div>
                <label class="input-label">{{ t('admin.acquisition.form.status') }}</label>
                <select v-model="form.status" class="input">
                  <option value="draft">{{ t('admin.acquisition.status.draft') }}</option>
                  <option value="active">{{ t('admin.acquisition.status.active') }}</option>
                </select>
              </div>
              <div>
                <label class="input-label">{{ t('admin.acquisition.form.pool') }}</label>
                <input v-model.number="form.leaderboard_pool_usd" class="input" type="number" min="0" step="0.01" />
              </div>
              <div>
                <label class="input-label">{{ t('admin.acquisition.form.startsAt') }}</label>
                <input v-model="form.starts_at" class="input" type="datetime-local" />
              </div>
              <div>
                <label class="input-label">{{ t('admin.acquisition.form.endsAt') }}</label>
                <input v-model="form.ends_at" class="input" type="datetime-local" />
              </div>
            </div>

            <div class="mt-5 grid gap-4 md:grid-cols-2">
              <div class="flex items-center justify-between rounded-lg border border-gray-200 p-3 dark:border-dark-700">
                <div>
                  <p class="text-sm font-medium text-gray-900 dark:text-white">{{ t('admin.acquisition.form.leaderboardEnabled') }}</p>
                  <p class="text-xs text-gray-500">{{ t('admin.acquisition.form.leaderboardHint') }}</p>
                </div>
                <Toggle v-model="form.leaderboard_enabled" />
              </div>
              <div class="flex items-center justify-between rounded-lg border border-gray-200 p-3 dark:border-dark-700">
                <div>
                  <p class="text-sm font-medium text-gray-900 dark:text-white">{{ t('admin.acquisition.form.lotteryEnabled') }}</p>
                  <p class="text-xs text-gray-500">{{ t('admin.acquisition.form.lotteryHint') }}</p>
                </div>
                <Toggle v-model="form.lottery_enabled" />
              </div>
            </div>

            <div class="mt-5">
              <label class="input-label">{{ t('admin.acquisition.form.shares') }}</label>
              <div class="grid grid-cols-5 gap-2">
                <input
                  v-for="(_, idx) in form.leaderboard_shares"
                  :key="idx"
                  v-model.number="form.leaderboard_shares[idx]"
                  class="input"
                  type="number"
                  min="0"
                  step="0.01"
                  :title="`#${idx + 1}`"
                />
              </div>
            </div>

            <div class="mt-5 space-y-3">
              <div class="flex items-center justify-between">
                <label class="input-label mb-0">{{ t('admin.acquisition.form.prizes') }}</label>
                <button type="button" class="btn btn-secondary btn-sm" @click="addPrize">
                  <Icon name="plus" size="sm" />
                  <span>{{ t('admin.acquisition.actions.addPrize') }}</span>
                </button>
              </div>
              <div v-for="(prize, idx) in form.lottery_prize_configs" :key="idx" class="grid gap-2 rounded-lg border border-gray-200 p-3 dark:border-dark-700 md:grid-cols-[1fr_110px_90px_110px_auto]">
                <input v-model="prize.name" class="input" :placeholder="t('admin.acquisition.form.prizeName')" />
                <input v-model.number="prize.amount_usd" class="input" type="number" min="0" step="0.01" :placeholder="t('admin.acquisition.form.prizeAmount')" />
                <input v-model.number="prize.count" class="input" type="number" min="1" step="1" :placeholder="t('admin.acquisition.form.prizeCount')" />
                <input v-model.number="prize.per_user_cap" class="input" type="number" min="0" step="1" :placeholder="t('admin.acquisition.form.prizeCap')" />
                <button type="button" class="btn btn-secondary btn-sm text-red-600" @click="removePrize(idx)">
                  <Icon name="trash" size="sm" />
                </button>
              </div>
            </div>

            <div class="mt-5">
              <label class="input-label">{{ t('admin.acquisition.form.seed') }}</label>
              <input v-model="form.lottery_seed" class="input font-mono" type="text" />
            </div>

            <div class="mt-6 flex justify-end gap-2">
              <button type="button" class="btn btn-secondary" :disabled="saving" @click="resetFormFromSelected">
                {{ t('common.cancel') }}
              </button>
              <button type="submit" class="btn btn-primary" :disabled="saving">
                <Icon v-if="saving" name="refresh" size="sm" class="animate-spin" />
                <span>{{ saving ? t('common.saving') : t('common.save') }}</span>
              </button>
            </div>
          </form>

          <div v-if="detail" class="card p-6">
            <div class="mb-4 flex items-center justify-between">
              <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.acquisition.detail.title') }}</h3>
              <button class="btn btn-secondary btn-sm" :disabled="settling" @click="settleSelected">
                <Icon v-if="settling" name="refresh" size="sm" class="animate-spin" />
                <Icon v-else name="badge" size="sm" />
                <span>{{ t('admin.acquisition.actions.settle') }}</span>
              </button>
            </div>

            <div class="grid gap-3 sm:grid-cols-3">
              <div class="rounded-lg bg-gray-50 p-3 dark:bg-dark-800">
                <p class="text-xs text-gray-500">{{ t('admin.acquisition.detail.participants') }}</p>
                <p class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">{{ detail.leaderboard.length }}</p>
              </div>
              <div class="rounded-lg bg-gray-50 p-3 dark:bg-dark-800">
                <p class="text-xs text-gray-500">{{ t('admin.acquisition.detail.rewards') }}</p>
                <p class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">{{ detail.rewards.length }}</p>
              </div>
              <div class="rounded-lg bg-gray-50 p-3 dark:bg-dark-800">
                <p class="text-xs text-gray-500">{{ t('admin.acquisition.detail.paid') }}</p>
                <p class="mt-1 text-xl font-semibold text-emerald-600 dark:text-emerald-400">{{ paidRewardCount }}</p>
              </div>
            </div>

            <div class="mt-4 overflow-x-auto">
              <table class="w-full min-w-[620px] text-left text-sm">
                <thead>
                  <tr class="border-b border-gray-200 text-gray-500 dark:border-dark-700 dark:text-dark-400">
                    <th class="px-3 py-2 font-medium">{{ t('admin.acquisition.detail.rewardType') }}</th>
                    <th class="px-3 py-2 font-medium">{{ t('admin.acquisition.detail.user') }}</th>
                    <th class="px-3 py-2 font-medium text-right">{{ t('admin.acquisition.detail.amount') }}</th>
                    <th class="px-3 py-2 font-medium">{{ t('admin.acquisition.detail.status') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-if="detail.rewards.length === 0">
                    <td colspan="4" class="px-3 py-6 text-center text-gray-500">{{ t('admin.acquisition.detail.emptyRewards') }}</td>
                  </tr>
                  <tr v-for="reward in detail.rewards" :key="reward.id" class="border-b border-gray-100 last:border-b-0 dark:border-dark-800">
                    <td class="px-3 py-3">{{ rewardLabel(reward) }}</td>
                    <td class="px-3 py-3">#{{ reward.user_id }}</td>
                    <td class="px-3 py-3 text-right font-medium text-emerald-600">{{ formatCurrency(reward.amount) }}</td>
                    <td class="px-3 py-3">{{ reward.status }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import Toggle from '@/components/common/Toggle.vue'
import { adminAPI } from '@/api/admin'
import type { AcquisitionCampaignInput } from '@/api/admin/acquisition'
import type { AcquisitionCampaign, AcquisitionReward, AcquisitionUserSummary } from '@/api/acquisition'
import { useAppStore } from '@/stores/app'
import { formatCurrency, formatDateTime } from '@/utils/format'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(true)
const saving = ref(false)
const settling = ref(false)
const campaigns = ref<AcquisitionCampaign[]>([])
const selectedCampaign = ref<AcquisitionCampaign | null>(null)
const detail = ref<AcquisitionUserSummary | null>(null)
const editingId = ref<number | null>(null)

const form = reactive<AcquisitionCampaignInput>({
  name: '',
  status: 'draft',
  starts_at: '',
  ends_at: '',
  leaderboard_enabled: true,
  lottery_enabled: true,
  leaderboard_pool_usd: 0,
  leaderboard_shares: [40, 25, 15, 12, 8],
  lottery_prize_configs: [],
  lottery_seed: '',
})

const paidRewardCount = computed(() => detail.value?.rewards.filter((item) => item.status === 'paid').length ?? 0)

function toLocalInput(value?: string | null): string {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`
}

function fromLocalInput(value: string): string {
  return value ? new Date(value).toISOString() : ''
}

function defaultWindow() {
  const start = new Date()
  start.setMinutes(0, 0, 0)
  const end = new Date(start)
  end.setDate(end.getDate() + 7)
  return { start: toLocalInput(start.toISOString()), end: toLocalInput(end.toISOString()) }
}

function resetForm(campaign?: AcquisitionCampaign | null) {
  if (campaign) {
    editingId.value = campaign.id
    form.name = campaign.name
    form.status = campaign.status === 'active' ? 'active' : 'draft'
    form.starts_at = toLocalInput(campaign.starts_at)
    form.ends_at = toLocalInput(campaign.ends_at)
    form.leaderboard_enabled = campaign.leaderboard_enabled
    form.lottery_enabled = campaign.lottery_enabled
    form.leaderboard_pool_usd = campaign.leaderboard_pool_usd
    form.leaderboard_shares = [...(campaign.leaderboard_shares?.length ? campaign.leaderboard_shares : [40, 25, 15, 12, 8])].slice(0, 5)
    while (form.leaderboard_shares.length < 5) form.leaderboard_shares.push(0)
    form.lottery_prize_configs = campaign.lottery_prize_configs.map((item) => ({ ...item }))
    form.lottery_seed = campaign.lottery_seed || ''
    return
  }
  const win = defaultWindow()
  editingId.value = null
  form.name = t('admin.acquisition.form.defaultName')
  form.status = 'draft'
  form.starts_at = win.start
  form.ends_at = win.end
  form.leaderboard_enabled = true
  form.lottery_enabled = true
  form.leaderboard_pool_usd = 100
  form.leaderboard_shares = [40, 25, 15, 12, 8]
  form.lottery_prize_configs = [
    { name: t('admin.acquisition.form.defaultPrize'), amount_usd: 3, count: 10, per_user_cap: 0 },
  ]
  form.lottery_seed = ''
}

function buildPayload(): AcquisitionCampaignInput {
  return {
    ...form,
    starts_at: fromLocalInput(form.starts_at),
    ends_at: fromLocalInput(form.ends_at),
    leaderboard_shares: form.leaderboard_shares.slice(0, 5).map((value) => Number(value) || 0),
    lottery_prize_configs: form.lottery_prize_configs
      .map((item) => ({
        name: item.name.trim() || t('admin.acquisition.form.defaultPrize'),
        amount_usd: Number(item.amount_usd) || 0,
        count: Number(item.count) || 0,
        per_user_cap: Number(item.per_user_cap) || 0,
      }))
      .filter((item) => item.amount_usd > 0 && item.count > 0),
  }
}

async function loadCampaigns() {
  loading.value = true
  try {
    const resp = await adminAPI.acquisition.listCampaigns()
    campaigns.value = resp.items
    if (!selectedCampaign.value && campaigns.value.length > 0) {
      await selectCampaign(campaigns.value[0].id)
    }
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.acquisition.loadFailed')))
  } finally {
    loading.value = false
  }
}

async function selectCampaign(id: number) {
  const campaign = campaigns.value.find((item) => item.id === id) || null
  selectedCampaign.value = campaign
  resetForm(campaign)
  if (!campaign) return
  try {
    detail.value = await adminAPI.acquisition.getCampaign(id)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.acquisition.detailLoadFailed')))
  }
}

function startCreate() {
  selectedCampaign.value = null
  detail.value = null
  resetForm(null)
}

function resetFormFromSelected() {
  resetForm(selectedCampaign.value)
}

function addPrize() {
  form.lottery_prize_configs.push({ name: '', amount_usd: 1, count: 1, per_user_cap: 0 })
}

function removePrize(index: number) {
  form.lottery_prize_configs.splice(index, 1)
}

async function saveCampaign() {
  saving.value = true
  try {
    const payload = buildPayload()
    const saved = editingId.value
      ? await adminAPI.acquisition.updateCampaign(editingId.value, payload)
      : await adminAPI.acquisition.createCampaign(payload)
    appStore.showSuccess(t('admin.acquisition.saveSuccess'))
    await loadCampaigns()
    await selectCampaign(saved.id)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.acquisition.saveFailed')))
  } finally {
    saving.value = false
  }
}

async function settleSelected() {
  if (!selectedCampaign.value) return
  settling.value = true
  try {
    await adminAPI.acquisition.settleCampaign(selectedCampaign.value.id)
    appStore.showSuccess(t('admin.acquisition.settleSuccess'))
    await loadCampaigns()
    await selectCampaign(selectedCampaign.value.id)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.acquisition.settleFailed')))
  } finally {
    settling.value = false
  }
}

function statusLabel(status: string): string {
  return t(`admin.acquisition.status.${status}`, status)
}

function statusClass(status: string): string {
  if (status === 'active') return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
  if (status === 'settled') return 'bg-gray-100 text-gray-700 dark:bg-dark-800 dark:text-gray-300'
  if (status === 'settling') return 'bg-amber-50 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
  return 'bg-blue-50 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300'
}

function rewardLabel(reward: AcquisitionReward): string {
  if (reward.reward_type === 'leaderboard') return t('admin.acquisition.detail.leaderboardReward', { rank: reward.rank || '-' })
  return t('admin.acquisition.detail.lotteryReward', { prize: reward.prize_name || '-' })
}

onMounted(() => {
  resetForm(null)
  void loadCampaigns()
})
</script>
