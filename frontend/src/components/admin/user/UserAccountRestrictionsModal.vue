<template>
  <BaseDialog :show="show" :title="t('admin.users.accountRestrictions.title')" width="wide" @close="close">
    <div v-if="user" class="space-y-4">
      <p class="break-all text-sm font-medium text-gray-900 dark:text-white">{{ user.email }}</p>
      <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('admin.users.accountRestrictions.hint') }}</p>
      <p v-if="initialLoading" role="status" class="py-6 text-center">{{ t('common.loading') }}</p>
      <div v-else-if="loadError" role="alert" class="space-y-2 text-sm text-red-600 dark:text-red-400">
        <p>{{ loadError }}</p>
        <button type="button" class="btn btn-secondary" @click="initialize">{{ t('common.refresh') }}</button>
      </div>
      <template v-else>
        <div>
          <label for="restriction-group" class="input-label">{{ t('admin.users.accountRestrictions.group') }}</label>
          <Select id="restriction-group" v-model="groupId" :aria-label="t('admin.users.accountRestrictions.group')" :options="groupOptions" :disabled="saving || dirty" :searchable="true" />
          <p v-if="dirty" class="input-hint">{{ t('admin.users.accountRestrictions.unsaved') }}</p>
        </div>
        <template v-if="groupId">
          <div class="flex flex-wrap items-center justify-between gap-2">
            <p class="text-sm text-gray-600 dark:text-gray-300">{{ t('admin.users.accountRestrictions.selected', { count: selected.length }) }}</p>
            <button type="button" class="btn btn-secondary" :disabled="saving || selected.length === 0" @click="selected = []">{{ t('admin.users.accountRestrictions.clear') }}</button>
          </div>
          <form class="flex gap-2" @submit.prevent="searchAccounts">
            <input v-model="search" class="input min-w-0 flex-1" :aria-label="t('admin.users.accountRestrictions.search')" :placeholder="t('admin.users.accountRestrictions.search')" :disabled="saving" />
            <button type="submit" class="btn btn-secondary" :disabled="saving || accountsLoading">{{ t('common.search') }}</button>
          </form>
          <div class="max-h-80 overflow-y-auto rounded-lg border border-gray-200 dark:border-dark-600" :aria-busy="accountsLoading">
            <p v-if="accountsLoading" role="status" class="p-6 text-center text-sm">{{ t('common.loading') }}</p>
            <div v-else-if="accountsError" role="alert" class="p-4 text-sm text-red-600 dark:text-red-400">
              <p>{{ accountsError }}</p>
              <button type="button" class="btn btn-secondary mt-2" @click="loadAccounts">{{ t('common.refresh') }}</button>
            </div>
            <p v-else-if="accounts.length === 0" class="p-6 text-center text-sm text-gray-500">{{ t('admin.users.accountRestrictions.empty') }}</p>
            <label v-for="account in accounts" v-else :key="account.id" class="flex cursor-pointer items-center gap-3 border-b border-gray-100 p-3 last:border-0 hover:bg-gray-50 dark:border-dark-700 dark:hover:bg-dark-700">
              <input v-model="selected" type="checkbox" :value="account.id" :disabled="saving || (selected.length >= 1000 && !selected.includes(account.id))" class="h-4 w-4 rounded border-gray-300 text-primary-600" />
              <span class="min-w-0 flex-1">
                <span class="block break-all text-sm text-gray-900 dark:text-white">{{ account.name }}</span>
                <span class="text-xs text-gray-500">#{{ account.id }} · {{ account.platform }} · {{ account.type }}</span>
              </span>
              <span v-if="selected.includes(account.id)" class="shrink-0 text-xs text-red-600 dark:text-red-400">{{ t('admin.users.accountRestrictions.blocked') }}</span>
            </label>
          </div>
          <div class="flex items-center justify-between gap-2 text-sm">
            <button type="button" class="btn btn-secondary" :disabled="page <= 1 || accountsLoading || saving" @click="changePage(-1)">{{ t('admin.users.accountRestrictions.previousPage') }}</button>
            <span>{{ page }} / {{ Math.max(1, Math.ceil(total / pageSize)) }}</span>
            <button type="button" class="btn btn-secondary" :disabled="page * pageSize >= total || accountsLoading || saving" @click="changePage(1)">{{ t('admin.users.accountRestrictions.nextPage') }}</button>
          </div>
        </template>
      </template>
    </div>
    <template #footer>
      <div class="flex flex-wrap justify-end gap-3">
        <button type="button" class="btn btn-secondary" :disabled="saving" @click="close">{{ t('common.cancel') }}</button>
        <button type="button" class="btn btn-secondary" :disabled="saving || !dirty" @click="resetSelection">{{ t('admin.users.accountRestrictions.undo') }}</button>
        <button type="button" class="btn btn-primary" :disabled="saving || initialLoading || !!loadError || !groupId || !dirty" @click="save">{{ saving ? t('common.saving') : t('common.save') }}</button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import type { AccountListItem, AdminGroup, AdminUser } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'

const props = defineProps<{ show: boolean; user: AdminUser | null }>()
const emit = defineEmits<{ close: []; success: [] }>()
const { t } = useI18n()
const appStore = useAppStore()
const groups = ref<AdminGroup[]>([])
const restrictions = ref<Record<number, number[]>>({})
const groupId = ref<number | null>(null)
const selected = ref<number[]>([])
const accounts = ref<AccountListItem[]>([])
const search = ref('')
const appliedSearch = ref('')
const page = ref(1)
const pageSize = 50
const total = ref(0)
const initialLoading = ref(false)
const accountsLoading = ref(false)
const saving = ref(false)
const loadError = ref('')
const accountsError = ref('')
let initialController: AbortController | undefined
let accountController: AbortController | undefined

const groupOptions = computed(() => groups.value.map(group => ({ value: group.id, label: `${group.name} · ${group.platform}` })))
const dirty = computed(() => {
  const original = groupId.value ? restrictions.value[groupId.value] || [] : []
  return selected.value.length !== original.length || selected.value.some(id => !original.includes(id))
})

function close() { if (!saving.value) emit('close') }
function resetSelection() { selected.value = [...(restrictions.value[groupId.value!] || [])] }

async function initialize() {
  initialController?.abort()
  accountController?.abort()
  const controller = new AbortController()
  initialController = controller
  const userId = props.user?.id
  if (!props.show || !userId) return
  initialLoading.value = true
  loadError.value = ''
  groupId.value = null
  selected.value = []
  groups.value = []
  restrictions.value = {}
  try {
    const allGroups: AdminGroup[] = []
    const rules = await adminAPI.users.getAccountRestrictions(userId, controller.signal)
    for (let groupPage = 1; ; groupPage++) {
      const result = await adminAPI.groups.list(groupPage, 100, undefined, { signal: controller.signal })
      allGroups.push(...result.items)
      if (allGroups.length >= result.total || result.items.length === 0) break
    }
    if (controller.signal.aborted) return
    restrictions.value = rules.restrictions || {}
    groups.value = allGroups
    groupId.value = allGroups.find(group => restrictions.value[group.id]?.length)?.id ?? allGroups[0]?.id ?? null
  } catch {
    if (!controller.signal.aborted) loadError.value = t('admin.users.accountRestrictions.loadError')
  } finally {
    if (!controller.signal.aborted) initialLoading.value = false
  }
}

async function loadAccounts() {
  accountController?.abort()
  if (!props.show || !groupId.value) return
  const controller = new AbortController()
  accountController = controller
  accountsLoading.value = true
  accountsError.value = ''
  accounts.value = []
  try {
    const result = await adminAPI.accounts.list(page.value, pageSize, {
      group: String(groupId.value), search: appliedSearch.value, lite: 'true'
    }, { signal: controller.signal })
    if (controller.signal.aborted) return
    accounts.value = result.items
    total.value = result.total
  } catch {
    if (!controller.signal.aborted) accountsError.value = t('admin.users.accountRestrictions.loadError')
  } finally {
    if (!controller.signal.aborted) accountsLoading.value = false
  }
}

function searchAccounts() { appliedSearch.value = search.value.trim(); page.value = 1; void loadAccounts() }
function changePage(delta: number) { page.value += delta; void loadAccounts() }

async function save() {
  if (!props.user || !groupId.value || !dirty.value || saving.value) return
  saving.value = true
  const savedUser = props.user.id
  const savedGroup = groupId.value
  try {
    const result = await adminAPI.users.updateAccountRestrictions(savedUser, savedGroup, [...selected.value])
    if (!props.show || props.user?.id !== savedUser) return
    restrictions.value[savedGroup] = result.account_ids
    selected.value = [...result.account_ids]
    appStore.showSuccess(t('admin.users.accountRestrictions.saved'))
    emit('success')
  } catch {
    appStore.showError(t('admin.users.accountRestrictions.saveError'))
  } finally { saving.value = false }
}

watch(groupId, () => {
  resetSelection()
  search.value = ''
  appliedSearch.value = ''
  page.value = 1
  void loadAccounts()
})
watch(() => [props.show, props.user?.id] as const, ([show]) => {
  if (show) void initialize()
  else { initialController?.abort(); accountController?.abort() }
}, { immediate: true })
onBeforeUnmount(() => { initialController?.abort(); accountController?.abort() })
</script>
