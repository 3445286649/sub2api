<template>
  <AppLayout full-height hide-header>
    <section class="relative h-full min-h-0 overflow-hidden bg-gray-50 dark:bg-dark-950" :aria-label="t('infiniteCanvas.title')">
      <div v-if="!connected" class="flex h-full items-center justify-center overflow-auto p-6">
        <div class="max-w-md space-y-4 text-center" role="status">
          <Icon name="grid" size="xl" class="mx-auto text-gray-400" />
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('infiniteCanvas.title') }}</h2>
          <p v-if="!enabled">{{ t('infiniteCanvas.disabled') }}</p>
          <p v-else-if="error" role="alert" class="text-sm text-red-600 dark:text-red-400">{{ t(`infiniteCanvas.${error}`) }}</p>
          <p v-else class="text-sm text-gray-500 dark:text-gray-400">{{ t(embedUrl || loading ? 'infiniteCanvas.connecting' : 'infiniteCanvas.description') }}</p>
          <button v-if="enabled && !embedUrl" type="button" class="btn btn-primary" :disabled="loading || connecting" @click="openSettings">
            {{ t('infiniteCanvas.connectionSettings') }}
          </button>
        </div>
      </div>
      <iframe
        v-if="embedUrl"
        ref="frame"
        :key="sessionId"
        :src="embedUrl"
        :title="t('infiniteCanvas.title')"
        class="absolute inset-0 h-full w-full border-0"
        :class="{ invisible: !connected }"
        :aria-hidden="!connected"
        :tabindex="connected ? 0 : -1"
        referrerpolicy="no-referrer"
        sandbox="allow-scripts allow-same-origin allow-downloads"
        allow="clipboard-write; fullscreen"
      />
      <button
        type="button"
        class="btn btn-secondary absolute bottom-4 left-4 z-10 shadow-sm lg:hidden"
        :aria-label="t('common.toggleMenu')"
        @click="app.toggleMobileSidebar()"
      >
        <Icon name="menu" size="sm" />
      </button>
      <button
        v-if="enabled && embedUrl"
        type="button"
        data-testid="canvas-settings"
        class="btn btn-secondary absolute bottom-4 right-4 z-10 gap-2 shadow-sm"
        @click="openSettings"
      >
        <Icon name="cog" size="sm" />
        {{ t('infiniteCanvas.connectionSettings') }}
      </button>
    </section>
    <BaseDialog :show="settingsOpen && enabled" :title="t('infiniteCanvas.connectionSettings')" width="normal" @close="closeSettings">
      <div class="space-y-4">
        <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('infiniteCanvas.description') }}</p>
        <fieldset v-for="role in CANVAS_CAPABILITIES" :key="role" class="min-w-0 space-y-2 rounded-xl border border-gray-200 p-3 dark:border-dark-700">
          <legend class="px-1 text-sm font-semibold">{{ t(`infiniteCanvas.${role}`) }}</legend>
          <div v-for="(id, index) in draft[role]" :key="id" class="grid min-w-0 grid-cols-[auto_minmax(0,1fr)_auto_auto] items-center gap-2 text-sm">
            <button type="button" :aria-label="t('infiniteCanvas.setDefault')" :disabled="connecting" :class="index === 0 ? 'text-primary-600' : 'text-gray-400'" @click="makeDefault(role, id)">{{ index === 0 ? '●' : '○' }}</button>
            <span class="min-w-0 truncate" :title="keyLabel(id)">{{ keyLabel(id) }}</span>
            <span v-if="index === 0" class="shrink-0 text-xs text-primary-600">{{ t('infiniteCanvas.default') }}</span>
            <button type="button" class="shrink-0 whitespace-nowrap text-gray-500 hover:text-red-600" :disabled="connecting" @click="draft[role] = draft[role].filter(value => value !== id)">{{ t('infiniteCanvas.remove') }}</button>
          </div>
          <label :for="`canvas-${role}-key`" class="sr-only">{{ t(`infiniteCanvas.${role}`) }}</label>
          <select :id="`canvas-${role}-key`" class="input w-full" value="" :disabled="loading || connecting" @change="addKey(role, $event)">
            <option value="">{{ t('infiniteCanvas.addConnection') }}</option>
            <option v-for="key in availableKeys(role)" :key="key.id" :value="String(key.id)">{{ keyLabel(key.id) }}</option>
          </select>
        </fieldset>
        <details class="text-sm">
          <summary class="cursor-pointer text-primary-600">{{ t('infiniteCanvas.pasteKey') }}</summary>
          <div class="mt-2 flex flex-wrap gap-2">
            <select v-model="pasteRole" class="input" :aria-label="t('infiniteCanvas.keyPurpose')"><option v-for="role in CANVAS_CAPABILITIES" :key="role" :value="role">{{ t(`infiniteCanvas.${role}`) }}</option></select>
            <input v-model="pastedKey" type="password" autocomplete="off" class="input min-w-0 flex-1" :aria-label="t('infiniteCanvas.pasteKey')" />
            <button type="button" class="btn btn-secondary" :disabled="loading || connecting || !pastedKey.trim()" @click="addPastedKey">{{ t('infiniteCanvas.add') }}</button>
          </div>
          <p class="mt-2 text-xs text-gray-500">{{ t('infiniteCanvas.ownKeysOnly') }}</p>
        </details>
        <p v-if="loading" role="status" class="text-sm text-gray-500">{{ t('common.loading') }}</p>
        <p v-else-if="!keys.length" role="status" class="text-sm text-amber-700 dark:text-amber-400">{{ t('infiniteCanvas.noKeys') }}</p>
        <p v-if="error" role="alert" class="text-sm text-red-600 dark:text-red-400">{{ t(`infiniteCanvas.${error}`) }}</p>
        <p class="text-xs leading-relaxed text-gray-500 dark:text-gray-400">{{ t('infiniteCanvas.billing') }}</p>
        <p v-if="embedUrl" class="text-xs text-amber-700 dark:text-amber-400">{{ t('infiniteCanvas.switchHint') }}</p>
        <div class="flex flex-wrap items-center gap-4 text-sm">
          <button type="button" class="text-primary-600 hover:underline" :disabled="loading || connecting" @click="loadKeys">{{ t('infiniteCanvas.refresh') }}</button>
          <RouterLink to="/keys" class="text-primary-600 hover:underline">{{ t('infiniteCanvas.manageKeys') }}</RouterLink>
          <RouterLink to="/usage" class="text-primary-600 hover:underline">{{ t('infiniteCanvas.usage') }}</RouterLink>
        </div>
      </div>
      <template #footer>
        <button type="button" class="btn btn-secondary" @click="closeSettings">{{ t('common.cancel') }}</button>
        <button type="button" class="btn btn-primary" :disabled="loading || connecting || !hasSelection(draft)" @click="enterCanvas">{{ t(connected ? 'infiniteCanvas.apply' : 'infiniteCanvas.enter') }}</button>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { keysAPI } from '@/api/keys'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { resolveFeatureFlag, FeatureFlags } from '@/utils/featureFlags'
import { CANVAS_CAPABILITIES, emptyCanvasSelection, parseCanvasSelection, type CanvasSelection, type CanvasCapability, CANVAS_SETTINGS_REFRESH_MS, CANVAS_CONNECT_TIMEOUT_MS, canvasMessage, canvasURL, isCanvasKeyAvailable, isCanvasMessage } from '@/utils/infiniteCanvas'
import type { ApiKey } from '@/types'

const { t, locale } = useI18n()
const app = useAppStore()
const auth = useAuthStore()
const enabled = computed(() => auth.isAuthenticated && resolveFeatureFlag(app.cachedPublicSettings, FeatureFlags.infiniteCanvas))
const keys = ref<ApiKey[]>([])
const draft = ref<CanvasSelection>(emptyCanvasSelection())
const active = ref<CanvasSelection>(emptyCanvasSelection())
const pastedKey = ref('')
const pasteRole = ref<CanvasCapability>('image')
let legacyImageKeyId: number | undefined
let revision = 0
let pending: { revision: number; selection: CanvasSelection } | null = null
const settingsOpen = ref(false)
const loading = ref(false)
const connecting = ref(false)
const error = ref('')
const embedUrl = ref('')
const sessionId = ref('')
const connected = ref(false)
const frame = ref<HTMLIFrameElement | null>(null)
let disposed = false
let generation = 0
let selectionRequest = 0
let request: AbortController | null = null
let timer: ReturnType<typeof setInterval> | undefined
let connectTimer: ReturnType<typeof setTimeout> | undefined
let checking: Promise<boolean> | null = null
let revalidating = false
let initializing = false
let initSent = false

function hasSelection(value: CanvasSelection) { return CANVAS_CAPABILITIES.some(role => value[role].length > 0) }
function keyLabel(id: number) {
  const key = keys.value.find(item => item.id === id)
  return key ? `${key.name} · ${key.group?.name || ''} · #${id}` : `#${id}`
}
function availableKeys(role: CanvasCapability) {
  return keys.value.filter(key => isCanvasKeyAvailable(key, auth.user?.id || 0, role) && !draft.value[role].includes(key.id))
}
function addKey(role: CanvasCapability, event: Event) {
  const select = event.target as HTMLSelectElement
  const id = Number(select.value)
  if (availableKeys(role).some(key => key.id === id)) draft.value[role].push(id)
  select.value = ''
}
function makeDefault(role: CanvasCapability, id: number) { draft.value[role] = [id, ...draft.value[role].filter(value => value !== id)] }
function addPastedKey() {
  const key = keys.value.find(item => item.key === pastedKey.value.trim() && isCanvasKeyAvailable(item, auth.user?.id || 0, pasteRole.value))
  pastedKey.value = ''
  if (!key) { error.value = 'invalidPastedKey'; return }
  if (!draft.value[pasteRole.value].includes(key.id)) draft.value[pasteRole.value].push(key.id)
  error.value = ''
}
function rememberSelection(selection: CanvasSelection | null, userId = auth.user?.id) {
  if (!userId) return
  try {
    const name = `sub2api.canvas.connections.${userId}`
    if (selection === null) {
      localStorage.removeItem(name)
      localStorage.removeItem(`sub2api.canvas.key.${userId}`)
    } else localStorage.setItem(name, JSON.stringify({ selection, legacyImageKeyId }))
  } catch { /* Runtime connections still work without storage. */ }
}
function recalledSelection(): CanvasSelection {
  try {
    const saved = localStorage.getItem(`sub2api.canvas.connections.${auth.user?.id}`)
    if (saved) {
      const parsed = JSON.parse(saved)
      legacyImageKeyId = Number.isSafeInteger(parsed.legacyImageKeyId) && parsed.legacyImageKeyId > 0 ? parsed.legacyImageKeyId : undefined
      return parseCanvasSelection(parsed.selection) || emptyCanvasSelection()
    }
    const id = Number(localStorage.getItem(`sub2api.canvas.key.${auth.user?.id}`))
    if (Number.isSafeInteger(id) && id > 0) { legacyImageKeyId = id; return { image: [id], text: [], video: [] } }
  } catch { /* Ignore invalid preferences. */ }
  return emptyCanvasSelection()
}
async function validatedConfig(selection: CanvasSelection) {
  const userId = auth.user!.id
  const ids = [...new Set(CANVAS_CAPABILITIES.flatMap(role => selection[role]))]
  const found = await Promise.all(ids.map(id => keysAPI.getById(id)))
  if (found.some((key, i) => key.id !== ids[i])) throw new Error('Key mismatch')
  const connections = CANVAS_CAPABILITIES.flatMap(capability => selection[capability].map(id => {
    const key = found.find(item => item.id === id)!
    if (!isCanvasKeyAvailable(key, userId, capability)) throw new Error('Key unavailable')
    return { id: `${capability}-${id}`, apiKeyId: id, apiKey: key.key, capability,
      platform: key.group!.platform, name: `${key.name} · ${key.group!.name} · #${id}` }
  }))
  return { userScope: String(userId), baseUrl: window.location.origin, connections, legacyImageKeyId,
    locale: locale.value, theme: document.documentElement.classList.contains('dark') ? 'dark' : 'light' }
}

function disconnect() {
  generation++
  selectionRequest++
  request?.abort()
  request = null
  clearTimeout(connectTimer)
  frame.value?.contentWindow?.postMessage(canvasMessage('DISPOSE', sessionId.value), window.location.origin)
  embedUrl.value = ''
  sessionId.value = ''
  active.value = emptyCanvasSelection()
  pending = null
  pastedKey.value = ''
  connected.value = false
  initializing = false
  initSent = false
  connecting.value = false
  loading.value = false
}
function invalidateAccess() {
  disconnect()
  settingsOpen.value = false
  keys.value = []
  draft.value = emptyCanvasSelection()
  error.value = 'accessFailed'
}

async function checkAccess(): Promise<boolean> {
  if (checking) return checking
  const operation = (async () => {
    try {
      const userId = auth.user?.id
      const token = auth.token
      if (!userId || !token || localStorage.getItem('auth_token') !== token) throw new Error('Session changed')
      await app.fetchPublicSettings(true)
      if (disposed || userId !== auth.user?.id || token !== auth.token || localStorage.getItem('auth_token') !== token || !enabled.value) throw new Error('Access unavailable')
      return true
    } catch {
      invalidateAccess()
      return false
    }
  })()
  checking = operation
  void operation.then(() => { if (checking === operation) checking = null })
  return operation
}

async function loadKeys() {
  error.value = ''
  const current = generation
  if (!await checkAccess() || current !== generation || disposed) return
  const userId = auth.user!.id
  request?.abort()
  const controller = new AbortController()
  request = controller
  loading.value = true
  try {
    const found: ApiKey[] = []
    let page = 1
    while (true) {
      const result = await keysAPI.list(page, 100, { status: 'active' }, { signal: controller.signal })
      if (disposed || controller.signal.aborted || current !== generation || userId !== auth.user?.id) return
      found.push(...result.items.filter(key => CANVAS_CAPABILITIES.some(role => isCanvasKeyAvailable(key, userId, role))))
      if (page * (result.page_size || 100) >= result.total || result.items.length === 0) break
      page++
    }
    keys.value = found

  } catch {
    if (current === generation && !disposed && !controller.signal.aborted) error.value = 'loadFailed'
  } finally {
    if (request === controller) loading.value = false
  }
}

function openSettings() {
  if (pending || (embedUrl.value && !connected.value)) return
  selectionRequest++
  draft.value = structuredClone(JSON.parse(JSON.stringify(active.value)))
  settingsOpen.value = true
  void loadKeys()
}
function closeSettings() {
  if (pending) return
  selectionRequest++
  connecting.value = false
  pastedKey.value = ''
  settingsOpen.value = false
}
async function connectSelection(selection: CanvasSelection): Promise<boolean> {
  const attempt = ++selectionRequest
  const current = generation
  const userId = auth.user?.id
  const token = auth.token
  connecting.value = true
  error.value = ''
  try {
    if (!userId || !hasSelection(selection) || !await checkAccess()) return false
    const config = await validatedConfig(selection)
    if (disposed || attempt !== selectionRequest || current !== generation || userId !== auth.user?.id || token !== auth.token || !enabled.value) return false
    if (connected.value && frame.value?.contentWindow) {
      pending = { revision: ++revision, selection }
      frame.value.contentWindow.postMessage({ ...canvasMessage('UPDATE', sessionId.value), revision, config }, window.location.origin)
    } else {
      disconnect()
      active.value = selection
      revision = 1
      sessionId.value = crypto.randomUUID()
      embedUrl.value = canvasURL(window.location.origin)
      settingsOpen.value = false
    }
    clearTimeout(connectTimer)
    connectTimer = setTimeout(() => { disconnect(); error.value = 'notReady' }, CANVAS_CONNECT_TIMEOUT_MS)
    return true
  } catch {
    if (attempt === selectionRequest && current === generation && !disposed) error.value = 'setupFailed'
    return false
  } finally {
    if (attempt === selectionRequest && !pending) connecting.value = false
  }
}
async function enterCanvas() { await connectSelection(JSON.parse(JSON.stringify(draft.value))) }
async function restore() {
  const current = generation
  if (!await checkAccess() || disposed || current !== generation) return
  const selection = recalledSelection()
  if (hasSelection(selection) && await connectSelection(selection)) return
  if (!disposed && current === generation && enabled.value) {
    if (hasSelection(selection)) rememberSelection(null)
    settingsOpen.value = true
    await loadKeys()
  }
}

async function receive(event: MessageEvent) {
  const target = frame.value?.contentWindow
  const origin = window.location.origin
  if (!embedUrl.value || !target || !sessionId.value) return
  if (isCanvasMessage(event, target, origin, 'ACK', sessionId.value)) {
    if (!enabled.value || event.data.revision !== revision) return
    if (pending && pending.revision === revision) {
      active.value = pending.selection
      pending = null
      settingsOpen.value = false
      pastedKey.value = ''
    } else if (!initSent) return
    connected.value = true
    connecting.value = false
    rememberSelection(active.value)
    clearTimeout(connectTimer)
    return
  }
  if (isCanvasMessage(event, target, origin, 'ERROR', sessionId.value)) {
    disconnect()
    error.value = 'setupFailed'
    return
  }
  if (!isCanvasMessage(event, target, origin, 'READY') || initializing || initSent) return
  initializing = true
  const current = generation
  const session = sessionId.value
  const userId = auth.user?.id
  try {
    if (!userId || !await checkAccess()) return
    const config = await validatedConfig(active.value)
    if (disposed || current !== generation || session !== sessionId.value || userId !== auth.user?.id || !enabled.value) return
    target.postMessage({ ...canvasMessage('INIT', session), revision, config }, origin)
    initSent = true
  } catch {
    if (current === generation && !disposed) {
      rememberSelection(null, userId)
      disconnect()
      error.value = 'setupFailed'
    }
  }
}

function onStorage(event: StorageEvent) {
  if (event.key === null || event.key === 'auth_token' || event.key === 'auth_user') {
    if (event.key === 'auth_token' && event.newValue === auth.token) return
    if (event.key === 'auth_user' && event.newValue && localStorage.getItem('auth_token') === auth.token) {
      try {
        if (JSON.parse(event.newValue)?.id === auth.user?.id) return
      } catch { /* Invalid stored identity must invalidate the workspace. */ }
    }
    invalidateAccess()
  }
}
async function onFocus() {
  if (revalidating) return
  revalidating = true
  const current = generation
  const userId = auth.user?.id
  try {
    if (!await checkAccess() || !hasSelection(active.value) || !userId || current !== generation || pending) return
    await validatedConfig(active.value)
    if (current !== generation || disposed) return
  } catch {
    if (current === generation && !disposed) {
      rememberSelection(null, userId)
      disconnect()
      error.value = 'setupFailed'
    }
  } finally { revalidating = false }
}
watch([() => auth.user?.id, () => auth.token, () => enabled.value], invalidateAccess)
onMounted(() => {
  window.addEventListener('message', receive)
  window.addEventListener('storage', onStorage)
  window.addEventListener('focus', onFocus)
  timer = setInterval(() => { void onFocus() }, CANVAS_SETTINGS_REFRESH_MS)
  void restore()
})
onUnmounted(() => {
  disposed = true
  disconnect()
  keys.value = []
  clearInterval(timer)
  window.removeEventListener('message', receive)
  window.removeEventListener('storage', onStorage)
  window.removeEventListener('focus', onFocus)
})
</script>
