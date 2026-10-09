<template>
  <section class="card" data-testid="telegram-payment-settings" :aria-busy="loading || operating">
    <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
      <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
        {{ t('admin.settings.telegram.title') }}
      </h2>
    </div>

    <div class="space-y-5 p-6">
      <div v-if="loading" class="flex items-center gap-2 text-sm text-gray-500" role="status">
        <Icon name="refresh" size="sm" class="animate-spin" />
        {{ t('common.loading') }}
      </div>
      <div v-else-if="loadFailed" class="flex flex-wrap items-center justify-between gap-3" role="alert">
        <p class="text-sm text-red-600 dark:text-red-400">{{ t('admin.settings.telegram.loadFailed') }}</p>
        <button type="button" class="btn btn-secondary btn-sm" data-testid="telegram-retry" @click="loadSettings">
          <Icon name="refresh" size="xs" class="mr-1.5" />
          {{ t('admin.settings.telegram.retry') }}
        </button>
      </div>
      <template v-else-if="settings">
        <div class="flex items-center justify-between gap-4">
          <label id="telegram-enabled-label" class="text-sm font-medium text-gray-900 dark:text-white">
            {{ t('admin.settings.telegram.enabled') }}
          </label>
          <Toggle
            v-model="form.enabled"
            :disabled="operating"
            aria-labelledby="telegram-enabled-label"
            data-testid="telegram-enabled"
          />
        </div>

        <div class="flex items-center justify-between gap-4 border-t border-gray-100 pt-4 dark:border-dark-700">
          <div class="min-w-0">
            <label id="telegram-verify-easypay-label" class="text-sm font-medium text-gray-900 dark:text-white">
              {{ t('admin.settings.telegram.verifyEasyPay') }}
            </label>
            <p id="telegram-verify-easypay-hint" class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.settings.telegram.verifyEasyPayHint') }}
            </p>
          </div>
          <Toggle
            v-model="form.verify_easypay_orders"
            :disabled="operating"
            aria-labelledby="telegram-verify-easypay-label"
            aria-describedby="telegram-verify-easypay-hint"
            data-testid="telegram-verify-easypay"
          />
        </div>

        <div class="grid grid-cols-1 gap-5 md:grid-cols-2">
          <div class="min-w-0">
            <div class="mb-2 flex flex-wrap items-center justify-between gap-2">
              <label for="telegram-bot-token" class="text-sm font-medium text-gray-700 dark:text-gray-300">
                {{ t('admin.settings.telegram.botToken') }}
              </label>
              <span class="text-xs text-gray-500 dark:text-gray-400" data-testid="telegram-token-status">
                {{ settings.bot_token_configured ? t('admin.settings.telegram.configured') : t('admin.settings.telegram.notConfigured') }}
              </span>
            </div>
            <input
              id="telegram-bot-token"
              v-model="form.bot_token"
              type="password"
              class="input"
              autocomplete="new-password"
              spellcheck="false"
              :disabled="operating"
              :placeholder="t('admin.settings.telegram.botTokenPlaceholder')"
              aria-describedby="telegram-token-hint"
              data-1p-ignore
              data-lpignore="true"
              data-bwignore="true"
              @keydown.enter.prevent="saveSettings"
            />
            <p id="telegram-token-hint" class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
              {{ settings.bot_token_configured ? t('admin.settings.telegram.keepTokenHint') : t('admin.settings.telegram.newTokenHint') }}
            </p>
          </div>
          <div class="min-w-0">
            <label for="telegram-chat-id" class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.settings.telegram.chatId') }}
            </label>
            <input
              id="telegram-chat-id"
              v-model="form.chat_id"
              type="text"
              class="input"
              autocomplete="off"
              spellcheck="false"
              :disabled="operating"
              placeholder="-1001234567890"
              @keydown.enter.prevent="saveSettings"
            />
          </div>
        </div>

        <div class="flex flex-wrap items-center gap-3 border-t border-gray-100 pt-4 dark:border-dark-700">
          <button
            type="button"
            class="btn btn-primary btn-sm"
            :disabled="operating"
            data-testid="telegram-save"
            @click="saveSettings"
          >
            <Icon :name="saving ? 'refresh' : 'check'" size="xs" class="mr-1.5" :class="{ 'animate-spin': saving }" />
            {{ saving ? t('common.saving') : t('common.save') }}
          </button>
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            :disabled="operating"
            data-testid="telegram-test"
            @click="sendTestMessage"
          >
            <Icon :name="testing ? 'refresh' : 'chat'" size="xs" class="mr-1.5" :class="{ 'animate-spin': testing }" />
            {{ t('admin.settings.telegram.sendTest') }}
          </button>
          <button
            v-if="settings.bot_token_configured"
            type="button"
            class="btn btn-secondary btn-sm text-red-600 dark:text-red-400"
            :disabled="operating"
            data-testid="telegram-clear-token"
            @click="showClearConfirm = true"
          >
            <Icon name="trash" size="xs" class="mr-1.5" />
            {{ t('admin.settings.telegram.clearToken') }}
          </button>
        </div>
      </template>
    </div>

    <ConfirmDialog
      :show="showClearConfirm"
      :title="t('admin.settings.telegram.clearToken')"
      :message="t('admin.settings.telegram.clearTokenConfirm')"
      :confirm-text="t('admin.settings.telegram.clearToken')"
      danger
      @confirm="clearToken"
      @cancel="showClearConfirm = false"
    />
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  getTelegramSettings,
  testTelegramSettings,
  updateTelegramSettings,
  type TelegramSettings,
  type UpdateTelegramSettingsRequest
} from '@/api/admin/telegram'
import { useAppStore } from '@/stores'
import Toggle from '@/components/common/Toggle.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
const appStore = useAppStore()
const settings = ref<TelegramSettings | null>(null)
const form = reactive({ enabled: false, verify_easypay_orders: false, bot_token: '', chat_id: '' })
const loading = ref(true)
const loadFailed = ref(false)
const saving = ref(false)
const testing = ref(false)
const showClearConfirm = ref(false)
const operating = computed(() => saving.value || testing.value)

function applySettings(next: TelegramSettings) {
  settings.value = next
  form.enabled = next.enabled
  form.verify_easypay_orders = next.verify_easypay_orders
  form.chat_id = next.chat_id
  form.bot_token = ''
}

async function loadSettings() {
  if (operating.value) return
  loading.value = true
  try {
    applySettings(await getTelegramSettings())
    loadFailed.value = false
  } catch {
    loadFailed.value = true
  } finally {
    loading.value = false
  }
}

function buildRequest(): UpdateTelegramSettingsRequest {
  const request: UpdateTelegramSettingsRequest = {
    enabled: form.enabled,
    verify_easypay_orders: form.verify_easypay_orders,
    chat_id: form.chat_id.trim()
  }
  const token = form.bot_token.trim()
  if (token) request.bot_token = token
  return request
}

function validateRequest(request: UpdateTelegramSettingsRequest, requireDelivery = false): boolean {
  if (request.enabled || requireDelivery) {
    if (!request.bot_token && !settings.value?.bot_token_configured) {
      appStore.showError(t('admin.settings.telegram.tokenRequired'))
      return false
    }
    if (!request.chat_id) {
      appStore.showError(t('admin.settings.telegram.chatIdRequired'))
      return false
    }
  }
  return true
}

async function saveSettings() {
  if (loading.value || loadFailed.value || !settings.value || operating.value) return
  const request = buildRequest()
  if (!validateRequest(request)) return
  saving.value = true
  try {
    applySettings(await updateTelegramSettings(request))
    appStore.showSuccess(t('admin.settings.telegram.saved'))
  } catch {
    appStore.showError(t('admin.settings.telegram.saveFailed'))
  } finally {
    saving.value = false
  }
}

async function sendTestMessage() {
  if (loading.value || loadFailed.value || !settings.value || operating.value) return
  const request = buildRequest()
  if (!validateRequest(request, true)) return
  testing.value = true
  try {
    await testTelegramSettings(request)
    appStore.showSuccess(t('admin.settings.telegram.testSent'))
  } catch {
    appStore.showError(t('admin.settings.telegram.testFailed'))
  } finally {
    testing.value = false
  }
}

async function clearToken() {
  showClearConfirm.value = false
  if (!settings.value || loading.value || loadFailed.value || operating.value) return
  saving.value = true
  try {
    applySettings(await updateTelegramSettings({
      enabled: false,
      verify_easypay_orders: settings.value.verify_easypay_orders,
      chat_id: settings.value.chat_id,
      clear_bot_token: true
    }))
    appStore.showSuccess(t('admin.settings.telegram.tokenCleared'))
  } catch {
    appStore.showError(t('admin.settings.telegram.saveFailed'))
  } finally {
    saving.value = false
  }
}

onMounted(loadSettings)
</script>
