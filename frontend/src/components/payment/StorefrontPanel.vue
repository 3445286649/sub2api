<template>
  <div class="space-y-4">
    <!-- 兑换栏：放在店铺上方，复制卡密后往上就能粘贴，不用离开页面 -->
    <div class="card p-5">
      <ol class="sf-steps" :aria-label="t('payment.storefront.stepsLabel')">
        <li v-for="(step, index) in steps" :key="step" class="sf-step">
          <span class="sf-step-index bg-primary-100 text-primary-700 dark:bg-primary-900/40 dark:text-primary-300">{{ index + 1 }}</span>
          <span>{{ step }}</span>
        </li>
      </ol>
      <form class="mt-4 flex flex-col gap-3 sm:flex-row" @submit.prevent="handleRedeem">
        <div class="relative min-w-0 flex-1">
          <div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3.5">
            <Icon name="gift" size="sm" class="text-gray-400 dark:text-dark-500" />
          </div>
          <input
            v-model="code"
            type="text"
            autocomplete="off"
            spellcheck="false"
            :placeholder="t('payment.storefront.codePlaceholder')"
            :disabled="redeeming"
            class="input py-2.5 pl-10"
            data-testid="storefront-code-input"
          />
        </div>
        <button
          type="submit"
          class="btn btn-primary shrink-0 px-6 py-2.5"
          :disabled="!code.trim() || redeeming"
          data-testid="storefront-redeem-button"
        >
          <span v-if="redeeming" class="flex items-center justify-center gap-2">
            <span class="h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent"></span>
            {{ t('redeem.redeeming') }}
          </span>
          <span v-else>{{ t('payment.storefront.redeem') }}</span>
        </button>
      </form>
      <p
        v-if="resultMessage"
        class="cp-fade-in mt-3 flex items-center gap-1.5 text-sm"
        :class="resultOk ? 'text-emerald-600 dark:text-emerald-400' : 'text-red-600 dark:text-red-400'"
        data-testid="storefront-redeem-result"
      >
        <Icon :name="resultOk ? 'checkCircle' : 'exclamationCircle'" size="sm" class="shrink-0" />
        <span>{{ resultMessage }}</span>
      </p>
    </div>

    <!-- 店铺区：渠道胶囊 + 操作 + iframe -->
    <div class="sf-workspace">
      <div class="sf-toolbar">
        <div v-if="channels.length > 1" class="flex min-w-0 flex-wrap gap-2" role="tablist" :aria-label="t('payment.storefront.channels')">
          <button
            v-for="channel in channels"
            :key="channel.id"
            type="button"
            role="tab"
            class="sf-chip"
            :class="{ 'sf-chip-active': channel.id === activeChannel?.id }"
            :aria-selected="channel.id === activeChannel?.id"
            @click="selectChannel(channel.id)"
          >
            <span class="sf-chip-dot" aria-hidden="true"></span>
            <span class="truncate">{{ channel.name }}</span>
          </button>
        </div>
        <p v-else class="truncate text-sm font-semibold text-gray-700 dark:text-gray-200">{{ activeChannel?.name }}</p>

        <div class="flex shrink-0 items-center gap-2">
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            :title="t('payment.storefront.refresh')"
            :aria-label="t('payment.storefront.refresh')"
            @click="refreshActiveChannel"
          >
            <Icon name="refresh" size="sm" :class="loadingActive ? 'animate-spin' : ''" />
          </button>
          <button type="button" class="btn btn-secondary btn-sm" @click="openActiveExternal">
            <Icon name="externalLink" size="sm" class="sm:mr-1.5" />
            <span class="hidden sm:inline">{{ t('payment.storefront.openExternal') }}</span>
          </button>
        </div>
      </div>

      <div class="sf-frame-shell">
        <!-- 多个渠道的 iframe 都保持挂载，切换渠道不重新加载 -->
        <div
          v-for="channel in channels"
          :key="`${channel.id}:${channel.url}:${frameVersions[channel.id] || 0}`"
          v-show="channel.id === activeChannel?.id"
          class="sf-frame-container"
        >
          <div v-if="!loadedFrames[channel.id] && !failedFrames[channel.id]" class="sf-frame-overlay">
            <Icon name="refresh" size="lg" class="animate-spin text-primary-500" />
            <span>{{ t('payment.storefront.loading') }}</span>
          </div>
          <div v-if="failedFrames[channel.id]" class="sf-frame-overlay sf-frame-error">
            <Icon name="exclamationCircle" size="lg" class="text-amber-500" />
            <strong>{{ t('payment.storefront.loadFailed') }}</strong>
            <span>{{ t('payment.storefront.loadFailedDescription') }}</span>
            <button type="button" class="btn btn-secondary btn-sm" @click="openExternal(channel)">
              <Icon name="externalLink" size="sm" class="mr-1.5" />
              {{ t('payment.storefront.openExternal') }}
            </button>
          </div>
          <iframe
            :src="channel.url"
            :title="t('payment.storefront.frameLabel', { name: channel.name })"
            class="sf-frame"
            allow="payment; clipboard-read; clipboard-write"
            sandbox="allow-forms allow-scripts allow-same-origin allow-popups allow-popups-to-escape-sandbox allow-modals allow-downloads"
            referrerpolicy="no-referrer"
            @load="handleFrameLoad(channel.id)"
            @error="handleFrameError(channel.id)"
          ></iframe>
        </div>
      </div>

      <footer class="sf-footer">
        <Icon name="shield" size="sm" class="shrink-0 text-gray-400 dark:text-dark-500" />
        <span>{{ t('payment.storefront.purchaseHint') }}</span>
      </footer>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { redeemAPI } from '@/api'
import { useAppStore } from '@/stores'
import { useAuthStore } from '@/stores/auth'
import { useSubscriptionStore } from '@/stores/subscriptions'
import type { RechargeStorefrontChannel } from '@/types'

const props = defineProps<{
  channels: RechargeStorefrontChannel[]
  channelId?: string
}>()

const emit = defineEmits<{
  (e: 'update:channelId', value: string): void
}>()

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const subscriptionStore = useSubscriptionStore()

const steps = computed(() => [
  t('payment.storefront.stepBuy'),
  t('payment.storefront.stepCopy'),
  t('payment.storefront.stepRedeem'),
])

// ==================== 渠道与 iframe ====================

const loadedFrames = reactive<Record<string, boolean>>({})
const failedFrames = reactive<Record<string, boolean>>({})
const frameVersions = reactive<Record<string, number>>({})

const activeChannel = computed(() => props.channels.find((channel) => channel.id === props.channelId) ?? props.channels[0])
const loadingActive = computed(() => Boolean(activeChannel.value && !loadedFrames[activeChannel.value.id] && !failedFrames[activeChannel.value.id]))

function selectChannel(channelId: string) {
  if (channelId === activeChannel.value?.id) return
  emit('update:channelId', channelId)
}

function openExternal(channel: RechargeStorefrontChannel) {
  window.open(channel.url, '_blank', 'noopener,noreferrer')
}

function openActiveExternal() {
  if (activeChannel.value) openExternal(activeChannel.value)
}

function refreshActiveChannel() {
  const channel = activeChannel.value
  if (!channel) return
  loadedFrames[channel.id] = false
  failedFrames[channel.id] = false
  frameVersions[channel.id] = (frameVersions[channel.id] || 0) + 1
}

function handleFrameLoad(channelId: string) {
  loadedFrames[channelId] = true
  failedFrames[channelId] = false
}

function handleFrameError(channelId: string) {
  loadedFrames[channelId] = false
  failedFrames[channelId] = true
}

// ==================== 卡密兑换 ====================

const code = ref('')
const redeeming = ref(false)
const resultMessage = ref('')
const resultOk = ref(false)

function describeRedeemResult(result: Awaited<ReturnType<typeof redeemAPI.redeem>>): string {
  if (result.type === 'balance') return t('payment.storefront.redeemedBalance', { amount: result.value.toFixed(2) })
  if (result.type === 'concurrency') return t('payment.storefront.redeemedConcurrency', { value: result.value })
  if (result.type === 'subscription' && result.group_name) return t('payment.storefront.redeemedSubscription', { name: result.group_name })
  return result.message || t('redeem.codeRedeemSuccess')
}

async function handleRedeem() {
  const value = code.value.trim()
  if (!value || redeeming.value) return

  redeeming.value = true
  resultMessage.value = ''
  try {
    const result = await redeemAPI.redeem(value)
    resultOk.value = true
    resultMessage.value = describeRedeemResult(result)
    code.value = ''
    // 刷新用户信息后，充值页顶部的余额会滚动到新值
    try {
      await authStore.refreshUser()
    } catch {
      appStore.showWarning(t('redeem.userRefreshFailed'))
    }
    if (result.type === 'subscription' || result.type === 'subscription_quota_reset') {
      subscriptionStore.fetchActiveSubscriptions(true).catch(() => {})
    }
    appStore.showSuccess(t('redeem.codeRedeemSuccess'))
  } catch (error: unknown) {
    const detail = (error as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    resultOk.value = false
    resultMessage.value = detail || t('redeem.failedToRedeem')
  } finally {
    redeeming.value = false
  }
}
</script>

<style scoped>
/* ============ 三步提示 ============ */
.sf-steps {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem 1rem;
  font-size: 0.8125rem;
  color: rgb(100 116 139);
}

.dark .sf-steps {
  color: rgb(148 163 184);
}

.sf-step {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
}

/* 步骤之间用短横线连接；窄屏会换行，连接线只在一行放得下时显示 */
@media (min-width: 640px) {
  .sf-steps {
    column-gap: 0;
  }

  .sf-step + .sf-step::before {
    content: '';
    display: inline-block;
    width: 1.5rem;
    height: 1px;
    margin: 0 0.6rem;
    background: rgb(203 213 225);
  }

  .dark .sf-step + .sf-step::before {
    background: rgb(71 85 105);
  }
}

.sf-step-index {
  display: inline-flex;
  width: 1.25rem;
  height: 1.25rem;
  align-items: center;
  justify-content: center;
  border-radius: 999px;
  font-size: 0.6875rem;
  font-weight: 700;
}

/* ============ 店铺区 ============ */
.sf-workspace {
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border: 1px solid rgb(226 232 240 / 0.9);
  border-radius: 1rem;
  background: rgb(255 255 255 / 0.9);
  box-shadow: 0 1px 3px rgb(15 23 42 / 0.06);
}

.dark .sf-workspace {
  border-color: rgb(51 65 85 / 0.8);
  background: rgb(15 23 42 / 0.78);
}

.sf-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  border-bottom: 1px solid rgb(226 232 240 / 0.9);
  padding: 0.65rem 0.9rem;
}

.dark .sf-toolbar {
  border-color: rgb(51 65 85 / 0.8);
}

.sf-chip {
  display: inline-flex;
  max-width: 12rem;
  align-items: center;
  gap: 0.45rem;
  border: 1px solid rgb(226 232 240);
  border-radius: 999px;
  padding: 0.35rem 0.85rem;
  color: rgb(100 116 139);
  font-size: 0.8125rem;
  font-weight: 600;
  transition: background-color 160ms ease, color 160ms ease, border-color 160ms ease;
}

.sf-chip:hover {
  border-color: rgb(203 213 225);
  color: rgb(30 41 59);
}

.dark .sf-chip {
  border-color: rgb(51 65 85);
  color: rgb(148 163 184);
}

.dark .sf-chip:hover {
  border-color: rgb(71 85 105);
  color: rgb(226 232 240);
}

.sf-chip-active,
.sf-chip-active:hover {
  border-color: rgb(125 211 252);
  background: rgb(224 242 254);
  color: rgb(3 105 161);
}

.dark .sf-chip-active,
.dark .sf-chip-active:hover {
  border-color: rgb(14 116 144 / 0.6);
  background: rgb(14 116 144 / 0.22);
  color: rgb(125 211 252);
}

.sf-chip-dot {
  width: 0.4rem;
  height: 0.4rem;
  flex-shrink: 0;
  border-radius: 999px;
  background: currentColor;
  opacity: 0.55;
}

.sf-frame-shell {
  position: relative;
  height: 70vh;
  min-height: 30rem;
  background: rgb(248 250 252);
}

.dark .sf-frame-shell {
  background: rgb(2 6 23);
}

.sf-frame-container {
  position: absolute;
  inset: 0;
}

.sf-frame {
  display: block;
  height: 100%;
  width: 100%;
  border: 0;
  background: white;
}

.sf-frame-overlay {
  position: absolute;
  inset: 0;
  z-index: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0.75rem;
  padding: 2rem;
  text-align: center;
  color: rgb(100 116 139);
  background: rgb(248 250 252 / 0.94);
}

.dark .sf-frame-overlay {
  color: rgb(148 163 184);
  background: rgb(2 6 23 / 0.94);
}

.sf-frame-error strong {
  color: rgb(51 65 85);
}

.dark .sf-frame-error strong {
  color: rgb(226 232 240);
}

.sf-footer {
  display: flex;
  align-items: flex-start;
  gap: 0.5rem;
  border-top: 1px solid rgb(226 232 240 / 0.9);
  padding: 0.7rem 0.9rem;
  color: rgb(100 116 139);
  font-size: 0.75rem;
  line-height: 1.45;
}

.dark .sf-footer {
  border-color: rgb(51 65 85 / 0.8);
  color: rgb(148 163 184);
}

@media (max-width: 640px) {
  .sf-frame-shell {
    height: 65vh;
    min-height: 26rem;
  }
}
</style>
