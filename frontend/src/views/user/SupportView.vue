<template>
  <AppLayout>
    <div v-if="!supportEnabled" class="card p-8 text-center">
      <div class="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-xl bg-gray-100 text-gray-500 dark:bg-dark-800 dark:text-dark-300">
        <Icon name="chat" size="lg" />
      </div>
      <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('support.disabledTitle') }}</h2>
      <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">{{ t('support.disabledDescription') }}</p>
    </div>

    <div v-else class="grid h-[calc(100vh-9rem)] min-h-[620px] gap-4 lg:grid-cols-[360px_minmax(0,1fr)]">
      <section class="card flex min-h-0 flex-col overflow-hidden">
        <div class="border-b border-gray-100 p-4 dark:border-dark-700">
          <div class="flex items-center justify-between gap-3">
            <div>
              <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('support.title') }}</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('support.description') }}</p>
            </div>
            <button class="btn btn-primary shrink-0" @click="openCreate">
              <Icon name="plus" size="sm" class="mr-1" />
              {{ t('support.newTicket') }}
            </button>
          </div>
          <div class="mt-4 flex gap-2">
            <select v-model="statusFilter" class="input h-10" @change="loadTickets">
              <option value="">{{ t('support.filters.all') }}</option>
              <option value="pending_admin">{{ t('support.status.pending_admin') }}</option>
              <option value="pending_user">{{ t('support.status.pending_user') }}</option>
              <option value="closed">{{ t('support.status.closed') }}</option>
            </select>
            <button class="btn btn-secondary h-10" :disabled="loadingTickets" @click="loadTickets">
              <Icon name="refresh" size="sm" :class="loadingTickets ? 'animate-spin' : ''" />
            </button>
          </div>
        </div>

        <div class="min-h-0 flex-1 overflow-y-auto">
          <button
            v-for="ticket in tickets"
            :key="ticket.id"
            type="button"
            class="w-full border-b border-gray-100 px-4 py-3 text-left transition hover:bg-gray-50 dark:border-dark-700 dark:hover:bg-dark-700/50"
            :class="selectedTicket?.id === ticket.id ? 'bg-primary-50/80 dark:bg-primary-900/20' : ''"
            @click="selectTicket(ticket)"
          >
            <div class="flex items-start justify-between gap-3">
              <div class="min-w-0">
                <div class="flex items-center gap-2">
                  <span v-if="ticket.user_unread" class="h-2 w-2 rounded-full bg-primary-500"></span>
                  <span class="truncate text-sm font-semibold text-gray-900 dark:text-white">{{ ticket.title }}</span>
                </div>
                <div class="mt-1 flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-dark-400">
                  <span>#{{ ticket.id }}</span>
                  <span>{{ categoryLabel(ticket.category) }}</span>
                  <span>{{ formatDate(ticket.last_message_at || ticket.created_at) }}</span>
                </div>
              </div>
              <span class="badge shrink-0" :class="statusBadgeClass(ticket.status)">
                {{ statusLabel(ticket.status) }}
              </span>
            </div>
          </button>

          <div v-if="!loadingTickets && tickets.length === 0" class="p-8 text-center text-sm text-gray-500 dark:text-dark-400">
            {{ t('support.empty') }}
          </div>
          <div v-if="loadingTickets" class="p-8 text-center text-sm text-gray-500 dark:text-dark-400">
            {{ t('common.loading') }}
          </div>
        </div>
      </section>

      <section class="card flex min-h-0 flex-col overflow-hidden">
        <template v-if="selectedTicket">
          <div class="border-b border-gray-100 p-4 dark:border-dark-700">
            <div class="flex flex-wrap items-start justify-between gap-3">
              <div class="min-w-0">
                <div class="flex flex-wrap items-center gap-2">
                  <h2 class="truncate text-lg font-semibold text-gray-900 dark:text-white">{{ selectedTicket.title }}</h2>
                  <span class="badge" :class="statusBadgeClass(selectedTicket.status)">
                    {{ statusLabel(selectedTicket.status) }}
                  </span>
                </div>
                <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
                  #{{ selectedTicket.id }} · {{ categoryLabel(selectedTicket.category) }}
                </p>
              </div>
              <div class="flex gap-2">
                <button
                  v-if="selectedTicket.status !== 'closed'"
                  class="btn btn-secondary"
                  :disabled="acting"
                  @click="closeTicket"
                >
                  {{ t('support.close') }}
                </button>
                <button
                  v-else
                  class="btn btn-primary"
                  :disabled="acting"
                  @click="reopenTicket"
                >
                  {{ t('support.reopen') }}
                </button>
              </div>
            </div>
          </div>

          <div ref="messagePane" class="min-h-0 flex-1 space-y-3 overflow-y-auto bg-gray-50/80 p-4 dark:bg-dark-900/40">
            <div
              v-for="message in messages"
              :key="message.id"
              class="flex"
              :class="message.sender_role === 'user' ? 'justify-end' : 'justify-start'"
            >
              <div
                class="max-w-[78%] rounded-2xl px-4 py-3 shadow-sm"
                :class="message.sender_role === 'user'
                  ? 'rounded-br-md bg-primary-600 text-white'
                  : 'rounded-bl-md border border-gray-200 bg-white text-gray-900 dark:border-dark-700 dark:bg-dark-800 dark:text-white'"
              >
                <div class="mb-1 text-xs opacity-75">
                  {{ senderLabel(message.sender_role) }} · {{ formatDate(message.created_at) }}
                </div>
                <p class="whitespace-pre-wrap break-words text-sm leading-6">{{ message.content }}</p>
              </div>
            </div>
            <div v-if="loadingMessages" class="text-center text-sm text-gray-500 dark:text-dark-400">
              {{ t('common.loading') }}
            </div>
          </div>

          <form class="border-t border-gray-100 p-4 dark:border-dark-700" @submit.prevent="sendMessage">
            <textarea
              v-model="draft"
              class="input min-h-[88px] resize-none"
              :disabled="selectedTicket.status === 'closed' || sending"
              maxlength="2000"
              :placeholder="selectedTicket.status === 'closed' ? t('support.closedHint') : t('support.replyPlaceholder')"
            ></textarea>
            <div class="mt-3 flex items-center justify-between gap-3">
              <span class="text-xs text-gray-500 dark:text-dark-400">{{ draft.length }}/2000</span>
              <button class="btn btn-primary" :disabled="!draft.trim() || selectedTicket.status === 'closed' || sending">
                {{ sending ? t('support.sending') : t('support.send') }}
              </button>
            </div>
          </form>
        </template>

        <div v-else class="flex min-h-0 flex-1 items-center justify-center p-8 text-center">
          <div>
            <div class="mx-auto mb-3 flex h-12 w-12 items-center justify-center rounded-xl bg-primary-50 text-primary-600 dark:bg-primary-900/20 dark:text-primary-300">
              <Icon name="chat" size="lg" />
            </div>
            <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('support.noSelection') }}</h3>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('support.noSelectionHint') }}</p>
          </div>
        </div>
      </section>
    </div>

    <BaseDialog :show="showCreate" :title="t('support.newTicket')" @close="showCreate = false">
      <form class="space-y-4" @submit.prevent="createTicket">
        <div>
          <label class="input-label">{{ t('support.form.title') }}</label>
          <input v-model="createForm.title" class="input" maxlength="120" required />
        </div>
        <div>
          <label class="input-label">{{ t('support.form.category') }}</label>
          <select v-model="createForm.category" class="input">
            <option v-for="item in categories" :key="item.value" :value="item.value">{{ item.label }}</option>
          </select>
        </div>
        <div>
          <label class="input-label">{{ t('support.form.content') }}</label>
          <textarea v-model="createForm.content" class="input min-h-[140px]" maxlength="2000" required></textarea>
        </div>
        <div v-if="errorMessage" class="rounded-lg bg-red-50 p-3 text-sm text-red-600 dark:bg-red-900/20 dark:text-red-300">
          {{ errorMessage }}
        </div>
        <div class="flex justify-end gap-2">
          <button type="button" class="btn btn-secondary" @click="showCreate = false">{{ t('common.cancel') }}</button>
          <button class="btn btn-primary" :disabled="creating">{{ creating ? t('common.saving') : t('support.create') }}</button>
        </div>
      </form>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { supportAPI } from '@/api'
import type { SupportTicket, SupportTicketMessage } from '@/types'
import { useAppStore } from '@/stores/app'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()
const appStore = useAppStore()

const tickets = ref<SupportTicket[]>([])
const selectedTicket = ref<SupportTicket | null>(null)
const messages = ref<SupportTicketMessage[]>([])
const statusFilter = ref('')
const loadingTickets = ref(false)
const loadingMessages = ref(false)
const sending = ref(false)
const acting = ref(false)
const draft = ref('')
const showCreate = ref(false)
const creating = ref(false)
const errorMessage = ref('')
const messagePane = ref<HTMLElement | null>(null)
let pollTimer: number | null = null

const supportEnabled = computed(() => appStore.cachedPublicSettings?.support_tickets_enabled !== false)

const createForm = reactive({
  title: '',
  category: 'general',
  content: ''
})

const categories = computed(() => [
  { value: 'general', label: t('support.category.general') },
  { value: 'recharge', label: t('support.category.recharge') },
  { value: 'subscription', label: t('support.category.subscription') },
  { value: 'api_issue', label: t('support.category.api_issue') },
  { value: 'account', label: t('support.category.account') }
])

async function loadTickets() {
  if (!supportEnabled.value) return
  loadingTickets.value = true
  try {
    const data = await supportAPI.list(1, 50, { status: statusFilter.value || undefined })
    tickets.value = data.items
    if (selectedTicket.value) {
      selectedTicket.value = tickets.value.find((item) => item.id === selectedTicket.value?.id) || selectedTicket.value
    }
  } finally {
    loadingTickets.value = false
  }
}

async function selectTicket(ticket: SupportTicket) {
  if (!supportEnabled.value) return
  selectedTicket.value = ticket
  await loadMessages()
  await supportAPI.markRead(ticket.id)
  await loadTickets()
}

async function loadMessages() {
  if (!supportEnabled.value || !selectedTicket.value) return
  loadingMessages.value = true
  try {
    messages.value = await supportAPI.listMessages(selectedTicket.value.id, { limit: 100 })
    await nextTick()
    scrollToBottom()
  } finally {
    loadingMessages.value = false
  }
}

async function poll() {
  if (!supportEnabled.value) return
  await loadTickets()
  if (selectedTicket.value) {
    await loadMessages()
    await supportAPI.markRead(selectedTicket.value.id)
  }
}

async function sendMessage() {
  if (!supportEnabled.value || !selectedTicket.value || !draft.value.trim()) return
  sending.value = true
  try {
    await supportAPI.sendMessage(selectedTicket.value.id, draft.value)
    draft.value = ''
    await poll()
  } finally {
    sending.value = false
  }
}

function openCreate() {
  if (!supportEnabled.value) return
  errorMessage.value = ''
  createForm.title = ''
  createForm.category = 'general'
  createForm.content = ''
  showCreate.value = true
}

async function createTicket() {
  if (!supportEnabled.value) return
  creating.value = true
  errorMessage.value = ''
  try {
    const ticket = await supportAPI.create({ ...createForm })
    showCreate.value = false
    await loadTickets()
    await selectTicket(ticket)
  } catch (error: any) {
    errorMessage.value = error?.message || t('support.failed')
  } finally {
    creating.value = false
  }
}

async function closeTicket() {
  if (!supportEnabled.value || !selectedTicket.value) return
  acting.value = true
  try {
    selectedTicket.value = await supportAPI.close(selectedTicket.value.id)
    await loadTickets()
  } finally {
    acting.value = false
  }
}

async function reopenTicket() {
  if (!supportEnabled.value || !selectedTicket.value) return
  acting.value = true
  try {
    const result = await supportAPI.reopen(selectedTicket.value.id)
    selectedTicket.value = result.ticket
    await poll()
  } finally {
    acting.value = false
  }
}

function scrollToBottom() {
  if (messagePane.value) {
    messagePane.value.scrollTop = messagePane.value.scrollHeight
  }
}

function formatDate(value?: string | null) {
  return value ? formatDateTime(value) : '-'
}

function statusLabel(value: string) {
  return t(`support.status.${value}`)
}

function categoryLabel(value: string) {
  return t(`support.category.${value}`)
}

function senderLabel(value: string) {
  return value === 'admin' ? t('support.sender.admin') : t('support.sender.user')
}

function statusBadgeClass(value: string) {
  if (value === 'closed') return 'badge-gray'
  if (value === 'pending_user') return 'badge-warning'
  if (value === 'pending_admin') return 'badge-primary'
  return 'badge-success'
}

function stopPolling() {
  if (pollTimer) {
    window.clearInterval(pollTimer)
    pollTimer = null
  }
}

function clearSupportState() {
  tickets.value = []
  selectedTicket.value = null
  messages.value = []
  draft.value = ''
  showCreate.value = false
}

async function startPolling() {
  if (!supportEnabled.value || pollTimer) return
  await loadTickets()
  pollTimer = window.setInterval(() => {
    void poll()
  }, 8000)
}

onMounted(() => {
  void startPolling()
})

onUnmounted(() => {
  stopPolling()
})

watch(supportEnabled, (enabled) => {
  if (enabled) {
    void startPolling()
  } else {
    stopPolling()
    clearSupportState()
  }
})
</script>
