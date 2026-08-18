<template>
  <AppLayout>
    <section class="backup-recharge-page" aria-labelledby="backup-recharge-title">
      <header class="backup-recharge-header">
        <div class="min-w-0">
          <div class="flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.14em] text-primary-600 dark:text-primary-300">
            <Icon name="creditCard" size="sm" />
            <span>{{ t('nav.backupRecharge') }}</span>
          </div>
          <h1 id="backup-recharge-title" class="mt-2 text-2xl font-semibold tracking-tight text-gray-950 dark:text-white">
            {{ t('backupRecharge.title') }}
          </h1>
          <p class="mt-1 max-w-2xl text-sm text-gray-500 dark:text-dark-400">
            {{ t('backupRecharge.description') }}
          </p>
        </div>

        <div v-if="activeChannel" class="flex shrink-0 items-center gap-2">
          <button
            type="button"
            class="icon-button"
            :title="t('backupRecharge.refresh')"
            :aria-label="t('backupRecharge.refresh')"
            @click="refreshActiveChannel"
          >
            <Icon name="refresh" size="sm" :class="loadingActive ? 'animate-spin' : ''" />
          </button>
          <button type="button" class="btn btn-secondary btn-sm" @click="openActiveExternal">
            <Icon name="externalLink" size="sm" class="mr-1.5" />
            <span class="hidden sm:inline">{{ t('backupRecharge.openExternal') }}</span>
          </button>
          <button type="button" class="btn btn-primary btn-sm" @click="goRedeem">
            <Icon name="gift" size="sm" class="mr-1.5" />
            <span>{{ t('backupRecharge.goRedeem') }}</span>
          </button>
        </div>
      </header>

      <div v-if="!settingsReady" class="backup-recharge-state">
        <Icon name="refresh" size="lg" class="animate-spin text-primary-500" />
        <p>{{ t('backupRecharge.loading') }}</p>
      </div>

      <div v-else-if="channels.length === 0" class="backup-recharge-state">
        <Icon name="creditCard" size="lg" class="text-gray-400 dark:text-dark-500" />
        <h2>{{ t('backupRecharge.unavailable') }}</h2>
        <p>{{ t('backupRecharge.unavailableDescription') }}</p>
      </div>

      <div v-else class="backup-recharge-workspace">
        <nav class="backup-recharge-tabs" :aria-label="t('backupRecharge.title')">
          <button
            v-for="channel in channels"
            :key="channel.id"
            type="button"
            class="backup-recharge-tab"
            :class="{ 'backup-recharge-tab-active': channel.id === activeChannel?.id }"
            @click="selectChannel(channel.id)"
          >
            <span class="backup-recharge-tab-dot" aria-hidden="true"></span>
            <span class="truncate">{{ channel.name }}</span>
          </button>
        </nav>

        <div class="backup-recharge-frame-shell">
          <div
            v-for="channel in channels"
            :key="`${channel.id}:${channel.url}:${frameVersions[channel.id] || 0}`"
            v-show="channel.id === activeChannel?.id"
            class="backup-recharge-frame-container"
          >
            <div v-if="!loadedFrames[channel.id] && !failedFrames[channel.id]" class="backup-recharge-frame-overlay">
              <Icon name="refresh" size="lg" class="animate-spin text-primary-500" />
              <span>{{ t('backupRecharge.loading') }}</span>
            </div>
            <div v-if="failedFrames[channel.id]" class="backup-recharge-frame-overlay backup-recharge-frame-error">
              <Icon name="exclamationCircle" size="lg" class="text-amber-500" />
              <strong>{{ t('backupRecharge.loadFailed') }}</strong>
              <span>{{ t('backupRecharge.loadFailedDescription') }}</span>
              <button type="button" class="btn btn-secondary btn-sm" @click="openExternal(channel)">
                <Icon name="externalLink" size="sm" class="mr-1.5" />
                {{ t('backupRecharge.openExternal') }}
              </button>
            </div>
            <iframe
              :src="channel.url"
              :title="t('backupRecharge.frameLabel', { name: channel.name })"
              class="backup-recharge-frame"
              allow="payment; clipboard-read; clipboard-write"
              sandbox="allow-forms allow-scripts allow-same-origin allow-popups allow-popups-to-escape-sandbox allow-modals allow-downloads"
              referrerpolicy="no-referrer"
              @load="handleFrameLoad(channel.id)"
              @error="handleFrameError(channel.id)"
            ></iframe>
          </div>
        </div>

        <footer class="backup-recharge-footer">
          <Icon name="shield" size="sm" class="shrink-0 text-gray-400 dark:text-dark-500" />
          <span>{{ t('backupRecharge.purchaseHint') }}</span>
        </footer>
      </div>
    </section>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores'
import type { RechargeStorefrontChannel } from '@/types'
import { resolveRechargeStorefrontChannels } from '@/utils/rechargeStorefront'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const appStore = useAppStore()

const loadedFrames = reactive<Record<string, boolean>>({})
const failedFrames = reactive<Record<string, boolean>>({})
const frameVersions = reactive<Record<string, number>>({})

const settingsReady = computed(() => appStore.publicSettingsLoaded)
const channels = computed<RechargeStorefrontChannel[]>(() => resolveRechargeStorefrontChannels(appStore.cachedPublicSettings))
const activeChannel = computed(() => channels.value.find((channel) => channel.id === String(route.params.channelId)) ?? channels.value[0])
const loadingActive = computed(() => Boolean(activeChannel.value && !loadedFrames[activeChannel.value.id] && !failedFrames[activeChannel.value.id]))

function selectChannel(channelId: string) {
  if (channelId === String(route.params.channelId)) return
  void router.push({ name: 'BackupRecharge', params: { channelId } })
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

function goRedeem() {
  void router.push('/redeem')
}

watch(
  [settingsReady, channels, () => route.params.channelId],
  ([ready, available]) => {
    if (!ready || available.length === 0) return
    const currentId = String(route.params.channelId || '')
    if (!available.some((channel) => channel.id === currentId)) {
      void router.replace({ name: 'BackupRecharge', params: { channelId: available[0].id } })
    }
  },
  { immediate: true },
)

onMounted(() => {
  if (!appStore.publicSettingsLoaded) void appStore.fetchPublicSettings()
})
</script>

<style scoped>
.backup-recharge-page {
  display: flex;
  min-height: calc(100vh - 9rem);
  flex-direction: column;
  gap: 1rem;
}

.backup-recharge-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.backup-recharge-workspace {
  display: flex;
  min-height: 0;
  flex: 1;
  flex-direction: column;
  overflow: hidden;
  border: 1px solid rgb(226 232 240 / 0.9);
  border-radius: 0.5rem;
  background: rgb(255 255 255 / 0.82);
  box-shadow: 0 18px 50px rgb(15 23 42 / 0.08);
}

.dark .backup-recharge-workspace {
  border-color: rgb(51 65 85 / 0.8);
  background: rgb(15 23 42 / 0.78);
  box-shadow: 0 18px 50px rgb(0 0 0 / 0.2);
}

.backup-recharge-tabs {
  display: flex;
  flex-shrink: 0;
  gap: 0.5rem;
  overflow-x: auto;
  border-bottom: 1px solid rgb(226 232 240 / 0.9);
  padding: 0.65rem;
}

.dark .backup-recharge-tabs {
  border-color: rgb(51 65 85 / 0.8);
}

.backup-recharge-tab {
  display: inline-flex;
  min-width: 8.5rem;
  align-items: center;
  gap: 0.5rem;
  border-radius: 0.375rem;
  padding: 0.6rem 0.8rem;
  color: rgb(100 116 139);
  font-size: 0.875rem;
  font-weight: 600;
  transition: background-color 160ms ease, color 160ms ease;
}

.backup-recharge-tab:hover {
  background: rgb(241 245 249);
  color: rgb(30 41 59);
}

.dark .backup-recharge-tab:hover {
  background: rgb(30 41 59);
  color: rgb(226 232 240);
}

.backup-recharge-tab-active {
  background: rgb(224 242 254);
  color: rgb(3 105 161);
}

.dark .backup-recharge-tab-active {
  background: rgb(14 116 144 / 0.22);
  color: rgb(125 211 252);
}

.backup-recharge-tab-dot {
  width: 0.45rem;
  height: 0.45rem;
  flex-shrink: 0;
  border-radius: 999px;
  background: currentColor;
  opacity: 0.55;
}

.backup-recharge-frame-shell {
  position: relative;
  min-height: 30rem;
  flex: 1;
  overflow: hidden;
  background: rgb(248 250 252);
}

.dark .backup-recharge-frame-shell {
  background: rgb(2 6 23);
}

.backup-recharge-frame-container {
  position: absolute;
  inset: 0;
}

.backup-recharge-frame {
  display: block;
  height: 100%;
  width: 100%;
  border: 0;
  background: white;
}

.backup-recharge-frame-overlay {
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

.dark .backup-recharge-frame-overlay {
  color: rgb(148 163 184);
  background: rgb(2 6 23 / 0.94);
}

.backup-recharge-frame-error strong {
  color: rgb(51 65 85);
}

.dark .backup-recharge-frame-error strong {
  color: rgb(226 232 240);
}

.backup-recharge-state {
  display: flex;
  min-height: 30rem;
  flex: 1;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0.65rem;
  border: 1px dashed rgb(203 213 225);
  border-radius: 0.5rem;
  padding: 2rem;
  text-align: center;
  color: rgb(100 116 139);
}

.backup-recharge-state h2 {
  color: rgb(51 65 85);
  font-size: 1.05rem;
  font-weight: 700;
}

.dark .backup-recharge-state {
  border-color: rgb(71 85 105);
  color: rgb(148 163 184);
}

.dark .backup-recharge-state h2 {
  color: rgb(226 232 240);
}

.backup-recharge-footer {
  display: flex;
  align-items: flex-start;
  gap: 0.5rem;
  border-top: 1px solid rgb(226 232 240 / 0.9);
  padding: 0.7rem 0.9rem;
  color: rgb(100 116 139);
  font-size: 0.75rem;
  line-height: 1.45;
}

.dark .backup-recharge-footer {
  border-color: rgb(51 65 85 / 0.8);
  color: rgb(148 163 184);
}

@media (min-width: 641px) {
  .backup-recharge-header {
    justify-content: flex-end;
  }

  .backup-recharge-header > div:first-child {
    display: none;
  }
}

@media (max-width: 640px) {
  .backup-recharge-header {
    flex-direction: column;
  }

  .backup-recharge-header > div:last-child {
    width: 100%;
  }

  .backup-recharge-header > div:last-child .btn {
    flex: 1;
    justify-content: center;
  }

  .backup-recharge-frame-shell {
    min-height: 26rem;
  }
}
</style>
