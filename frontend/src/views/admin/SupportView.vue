<template>
  <AppLayout>
    <div v-if="!supportEnabled" class="card p-8 text-center">
      <div class="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-xl bg-gray-100 text-gray-500 dark:bg-dark-800 dark:text-dark-300">
        <Icon name="chat" size="lg" />
      </div>
      <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.support.disabledTitle') }}</h2>
      <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">{{ t('admin.support.disabledDescription') }}</p>
    </div>

    <div v-else class="grid h-[calc(100vh-9rem)] min-h-[640px] gap-4 xl:grid-cols-[420px_minmax(0,1fr)]">
      <section class="card flex min-h-0 flex-col overflow-hidden">
        <div class="space-y-3 border-b border-gray-100 p-4 dark:border-dark-700">
          <div class="flex items-center justify-between gap-3">
            <div>
              <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.support.title') }}</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('admin.support.description') }}</p>
            </div>
            <button class="btn btn-secondary shrink-0" :disabled="loadingTickets" @click="loadTickets">
              <Icon name="refresh" size="sm" :class="loadingTickets ? 'animate-spin' : ''" />
            </button>
          </div>

          <input
            v-model="filters.search"
            class="input"
            :placeholder="t('admin.support.searchPlaceholder')"
            @keyup.enter="loadTickets"
          />
          <div class="grid grid-cols-2 gap-2">
            <select v-model="filters.status" class="input h-10" @change="loadTickets">
              <option value="">{{ t('support.filters.allStatus') }}</option>
              <option value="pending_admin">{{ t('support.status.pending_admin') }}</option>
              <option value="pending_user">{{ t('support.status.pending_user') }}</option>
              <option value="closed">{{ t('support.status.closed') }}</option>
            </select>
            <select v-model="filters.category" class="input h-10" @change="loadTickets">
              <option value="">{{ t('support.filters.allCategory') }}</option>
              <option v-for="item in categories" :key="item.value" :value="item.value">{{ item.label }}</option>
            </select>
            <select v-model="filters.priority" class="input h-10" @change="loadTickets">
              <option value="">{{ t('support.filters.allPriority') }}</option>
              <option v-for="item in priorities" :key="item.value" :value="item.value">{{ item.label }}</option>
            </select>
            <label class="flex h-10 items-center gap-2 rounded-lg border border-gray-200 px-3 text-sm text-gray-700 dark:border-dark-600 dark:text-dark-200">
              <input v-model="filters.unread_only" type="checkbox" class="rounded" @change="loadTickets" />
              {{ t('admin.support.unreadOnly') }}
            </label>
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
                  <span v-if="ticket.admin_unread" class="h-2 w-2 rounded-full bg-red-500"></span>
                  <span class="truncate text-sm font-semibold text-gray-900 dark:text-white">{{ ticket.title }}</span>
                </div>
                <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                  #{{ ticket.id }} · {{ ticket.user?.email || `UID ${ticket.user_id}` }}
                </div>
                <div class="mt-1 flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-dark-400">
                  <span>{{ categoryLabel(ticket.category) }}</span>
                  <span>{{ priorityLabel(ticket.priority) }}</span>
                  <span>{{ formatDate(ticket.last_message_at || ticket.created_at) }}</span>
                </div>
              </div>
              <span class="badge shrink-0" :class="statusBadgeClass(ticket.status)">
                {{ statusLabel(ticket.status) }}
              </span>
            </div>
          </button>
          <div v-if="!loadingTickets && tickets.length === 0" class="p-8 text-center text-sm text-gray-500 dark:text-dark-400">
            {{ t('admin.support.empty') }}
          </div>
          <div v-if="loadingTickets" class="p-8 text-center text-sm text-gray-500 dark:text-dark-400">
            {{ t('common.loading') }}
          </div>
        </div>
      </section>

      <section class="card flex min-h-0 flex-col overflow-hidden">
        <template v-if="selectedTicket">
          <div class="border-b border-gray-100 p-4 dark:border-dark-700">
            <div class="flex flex-wrap items-start justify-between gap-4">
              <div class="min-w-0">
                <div class="flex flex-wrap items-center gap-2">
                  <h2 class="truncate text-lg font-semibold text-gray-900 dark:text-white">{{ selectedTicket.title }}</h2>
                  <span class="badge" :class="statusBadgeClass(selectedTicket.status)">
                    {{ statusLabel(selectedTicket.status) }}
                  </span>
                </div>
                <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
                  #{{ selectedTicket.id }} · {{ selectedTicket.user?.email || `UID ${selectedTicket.user_id}` }}
                </p>
              </div>
              <div class="flex flex-wrap gap-2">
                <select v-model="editForm.status" class="input h-10 w-36" @change="updateTicket">
                  <option value="open">{{ t('support.status.open') }}</option>
                  <option value="pending_admin">{{ t('support.status.pending_admin') }}</option>
                  <option value="pending_user">{{ t('support.status.pending_user') }}</option>
                  <option value="closed">{{ t('support.status.closed') }}</option>
                </select>
                <select v-model="editForm.priority" class="input h-10 w-32" @change="updateTicket">
                  <option v-for="item in priorities" :key="item.value" :value="item.value">{{ item.label }}</option>
                </select>
                <select v-model="editForm.category" class="input h-10 w-36" @change="updateTicket">
                  <option v-for="item in categories" :key="item.value" :value="item.value">{{ item.label }}</option>
                </select>
                <button
                  v-if="selectedTicket.status !== 'closed'"
                  class="btn btn-secondary h-10"
                  :disabled="acting"
                  @click="closeTicket"
                >
                  {{ t('support.close') }}
                </button>
                <button
                  v-else
                  class="btn btn-primary h-10"
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
              :class="message.sender_role === 'admin' ? 'justify-end' : 'justify-start'"
            >
              <div
                class="max-w-[78%] rounded-2xl px-4 py-3 shadow-sm"
                :class="message.sender_role === 'admin'
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
              :placeholder="selectedTicket.status === 'closed' ? t('support.closedHint') : t('admin.support.replyPlaceholder')"
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
            <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.support.noSelection') }}</h3>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('admin.support.noSelectionHint') }}</p>
          </div>
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { adminAPI } from '@/api/admin'
import type { SupportTicket, SupportTicketMessage } from '@/types'
import { useAppStore } from '@/stores/app'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()
const appStore = useAppStore()

const tickets = ref<SupportTicket[]>([])
const selectedTicket = ref<SupportTicket | null>(null)
const messages = ref<SupportTicketMessage[]>([])
const loadingTickets = ref(false)
const loadingMessages = ref(false)
const sending = ref(false)
const acting = ref(false)
const draft = ref('')
const messagePane = ref<HTMLElement | null>(null)
let pollTimer: number | null = null

const supportEnabled = computed(() => appStore.cachedPublicSettings?.support_tickets_enabled !== false)

const filters = reactive({
  status: 'pending_admin',
  category: '',
  priority: '',
  search: '',
  unread_only: false
})

const editForm = reactive({
  status: '',
  category: '',
  priority: ''
})

const categories = computed(() => [
  { value: 'general', label: t('support.category.general') },
  { value: 'recharge', label: t('support.category.recharge') },
  { value: 'subscription', label: t('support.category.subscription') },
  { value: 'api_issue', label: t('support.category.api_issue') },
  { value: 'account', label: t('support.category.account') }
])

const priorities = computed(() => [
  { value: 'low', label: t('support.priority.low') },
  { value: 'normal', label: t('support.priority.normal') },
  { value: 'high', label: t('support.priority.high') },
  { value: 'urgent', label: t('support.priority.urgent') }
])

async function loadTickets() {
  if (!supportEnabled.value) return
  loadingTickets.value = true
  try {
    const data = await adminAPI.support.list(1, 80, {
      status: filters.status || undefined,
      category: filters.category || undefined,
      priority: filters.priority || undefined,
      search: filters.search || undefined,
      unread_only: filters.unread_only || undefined
    })
    tickets.value = data.items
    if (selectedTicket.value) {
      const fresh = tickets.value.find((item) => item.id === selectedTicket.value?.id)
      if (fresh) {
        selectedTicket.value = fresh
        syncEditForm(fresh)
      }
    }
  } finally {
    loadingTickets.value = false
  }
}

async function selectTicket(ticket: SupportTicket) {
  if (!supportEnabled.value) return
  selectedTicket.value = ticket
  syncEditForm(ticket)
  await loadMessages()
  await adminAPI.support.markRead(ticket.id)
  await loadTickets()
}

async function loadMessages() {
  if (!supportEnabled.value || !selectedTicket.value) return
  loadingMessages.value = true
  try {
    messages.value = await adminAPI.support.listMessages(selectedTicket.value.id, { limit: 100 })
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
    await adminAPI.support.markRead(selectedTicket.value.id)
  }
}

async function sendMessage() {
  if (!supportEnabled.value || !selectedTicket.value || !draft.value.trim()) return
  sending.value = true
  try {
    await adminAPI.support.sendMessage(selectedTicket.value.id, draft.value)
    draft.value = ''
    await poll()
  } finally {
    sending.value = false
  }
}

async function updateTicket() {
  if (!supportEnabled.value || !selectedTicket.value) return
  acting.value = true
  try {
    selectedTicket.value = await adminAPI.support.update(selectedTicket.value.id, {
      status: editForm.status,
      category: editForm.category,
      priority: editForm.priority
    })
    syncEditForm(selectedTicket.value)
    await loadTickets()
  } finally {
    acting.value = false
  }
}

async function closeTicket() {
  if (!supportEnabled.value || !selectedTicket.value) return
  acting.value = true
  try {
    selectedTicket.value = await adminAPI.support.close(selectedTicket.value.id)
    syncEditForm(selectedTicket.value)
    await loadTickets()
  } finally {
    acting.value = false
  }
}

async function reopenTicket() {
  if (!supportEnabled.value || !selectedTicket.value) return
  acting.value = true
  try {
    const result = await adminAPI.support.reopen(selectedTicket.value.id)
    selectedTicket.value = result.ticket
    syncEditForm(result.ticket)
    await poll()
  } finally {
    acting.value = false
  }
}

function syncEditForm(ticket: SupportTicket) {
  editForm.status = ticket.status
  editForm.category = ticket.category
  editForm.priority = ticket.priority
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

function priorityLabel(value: string) {
  return t(`support.priority.${value}`)
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
