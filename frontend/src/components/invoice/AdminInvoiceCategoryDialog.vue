<template>
  <BaseDialog :show="show" :title="t('invoice.categories.title')" width="wide" mobile-sheet @close="emit('close')">
    <div class="space-y-5" data-testid="invoice-category-dialog">
      <p class="text-sm text-gray-600 dark:text-gray-300">{{ t('invoice.categories.description') }}</p>

      <!-- 新增 / 编辑 -->
      <section class="rounded-xl border border-gray-200 p-4 dark:border-dark-600">
        <h3 class="mb-3 text-sm font-semibold text-gray-800 dark:text-gray-200">
          {{ editingId === null ? t('invoice.categories.addTitle') : t('invoice.categories.editTitle') }}
        </h3>
        <div class="grid gap-3 sm:grid-cols-2">
          <div>
            <label class="input-label" for="inv-cat-name">{{ t('invoice.categories.name') }}<span class="text-red-500"> *</span></label>
            <input
              id="inv-cat-name"
              v-model="draft.name"
              class="input"
              :class="{ 'input-error': showError && !draft.name.trim() }"
              maxlength="100"
              :placeholder="t('invoice.categories.namePlaceholder')"
              data-testid="invoice-category-name"
              @keydown.enter.prevent="save"
            />
            <p v-if="showError && !draft.name.trim()" class="input-error-text">{{ t('invoice.errors.INVOICE_CATEGORY_NAME_REQUIRED') }}</p>
          </div>
          <div>
            <label class="input-label" for="inv-cat-desc">{{ t('invoice.categories.descriptionLabel') }}</label>
            <input
              id="inv-cat-desc"
              v-model="draft.description"
              class="input"
              maxlength="500"
              :placeholder="t('invoice.categories.descriptionPlaceholder')"
              data-testid="invoice-category-description"
              @keydown.enter.prevent="save"
            />
          </div>
        </div>
        <div class="mt-3 flex justify-end gap-2">
          <button v-if="editingId !== null" type="button" class="btn btn-secondary" :disabled="saving" @click="resetDraft">{{ t('invoice.categories.cancelEdit') }}</button>
          <button type="button" class="btn btn-primary" :disabled="saving" data-testid="invoice-category-save" @click="save">
            <span v-if="saving" class="mr-2 h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent" />
            {{ editingId === null ? t('invoice.categories.add') : t('invoice.categories.update') }}
          </button>
        </div>
      </section>

      <!-- 已配置的类目 -->
      <section>
        <h3 class="mb-3 text-sm font-semibold text-gray-800 dark:text-gray-200">
          {{ t('invoice.categories.listTitle') }}<span class="ml-1.5 font-normal tabular-nums text-gray-400">{{ items.length }}</span>
        </h3>

        <div v-if="loading && items.length === 0" class="space-y-2">
          <div v-for="i in 3" :key="i" class="h-12 animate-pulse rounded-lg bg-gray-100 dark:bg-dark-700" />
        </div>
        <p v-else-if="items.length === 0" class="rounded-xl border border-dashed border-gray-200 py-8 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400" data-testid="invoice-category-empty">
          {{ t('invoice.categories.empty') }}
        </p>
        <ul v-else class="divide-y divide-gray-100 rounded-xl border border-gray-100 dark:divide-dark-700 dark:border-dark-700" :class="{ 'opacity-60': loading }">
          <li v-for="(item, index) in items" :key="item.id" class="flex flex-wrap items-center gap-3 px-3.5 py-2.5" :data-testid="`invoice-category-row-${item.id}`">
            <div class="min-w-0 flex-1">
              <p class="flex flex-wrap items-center gap-2 text-sm font-medium text-gray-900 dark:text-white">
                <span class="break-all" :class="{ 'text-gray-400 line-through dark:text-dark-400': !item.enabled }">{{ item.name }}</span>
                <span v-if="!item.enabled" class="rounded-full bg-gray-100 px-2 py-0.5 text-[11px] font-normal text-gray-500 dark:bg-dark-700 dark:text-gray-400">{{ t('invoice.categories.disabled') }}</span>
              </p>
              <p v-if="item.description" class="mt-0.5 break-all text-xs text-gray-500 dark:text-gray-400">{{ item.description }}</p>
              <p v-if="item.request_count > 0" class="mt-0.5 text-xs text-gray-400 dark:text-dark-400">{{ t('invoice.categories.used', { count: item.request_count }) }}</p>
            </div>
            <div class="flex shrink-0 items-center gap-1">
              <button type="button" class="btn btn-secondary btn-sm" :disabled="index === 0 || busy" :title="t('invoice.categories.moveUp')" :data-testid="`invoice-category-up-${item.id}`" @click="move(index, -1)">
                <Icon name="chevronDown" size="sm" class="rotate-180" />
              </button>
              <button type="button" class="btn btn-secondary btn-sm" :disabled="index === items.length - 1 || busy" :title="t('invoice.categories.moveDown')" :data-testid="`invoice-category-down-${item.id}`" @click="move(index, 1)">
                <Icon name="chevronDown" size="sm" />
              </button>
              <button type="button" class="btn btn-secondary btn-sm" :disabled="busy" :data-testid="`invoice-category-toggle-${item.id}`" @click="toggle(item)">
                {{ item.enabled ? t('invoice.categories.disable') : t('invoice.categories.enable') }}
              </button>
              <button type="button" class="btn btn-secondary btn-sm" :disabled="busy" :data-testid="`invoice-category-edit-${item.id}`" @click="startEdit(item)">
                <Icon name="edit" size="sm" class="mr-1" />{{ t('invoice.categories.edit') }}
              </button>
              <button
                type="button"
                class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-red-600 hover:bg-red-50 disabled:cursor-not-allowed disabled:opacity-40 disabled:hover:bg-transparent dark:text-red-400 dark:hover:bg-red-900/20"
                :disabled="item.request_count > 0 || busy"
                :title="item.request_count > 0 ? t('invoice.categories.deleteBlocked') : ''"
                :data-testid="`invoice-category-delete-${item.id}`"
                @click="pendingDelete = item"
              >
                <Icon name="trash" size="xs" />{{ t('invoice.categories.delete') }}
              </button>
            </div>
          </li>
        </ul>
      </section>
    </div>

    <template #footer>
      <div class="flex w-full justify-end">
        <button type="button" class="btn btn-secondary" @click="emit('close')">{{ t('common.close') }}</button>
      </div>
    </template>
  </BaseDialog>

  <ConfirmDialog
    :show="pendingDelete !== null"
    :title="t('invoice.categories.deleteTitle')"
    :message="t('invoice.categories.deleteMessage', { name: pendingDelete?.name ?? '' })"
    :confirm-text="t('invoice.categories.delete')"
    danger
    @confirm="confirmDelete"
    @cancel="pendingDelete = null"
  />
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { adminInvoiceAPI } from '@/api/admin/invoice'
import { useAppStore } from '@/stores/app'
import { extractI18nErrorMessage } from '@/utils/apiError'
import type { AdminInvoiceCategory } from '@/types/invoice'

const props = defineProps<{ show: boolean }>()
// changed：类目增删改、启停、排序后通知父页面刷新筛选项
const emit = defineEmits<{ (e: 'close'): void; (e: 'changed'): void }>()
const { t } = useI18n()
const appStore = useAppStore()

const items = ref<AdminInvoiceCategory[]>([])
const loading = ref(false)
const saving = ref(false)
const busyId = ref<number | null>(null)
const busy = computed(() => saving.value || busyId.value !== null)

async function load() {
  loading.value = true
  try {
    items.value = await adminInvoiceAPI.listCategories()
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'invoice.errors', t('invoice.categories.loadFailed')))
  } finally {
    loading.value = false
  }
}

// ---- 新增 / 编辑 ----
const editingId = ref<number | null>(null)
const draft = reactive({ name: '', description: '' })
const showError = ref(false)

function resetDraft() {
  editingId.value = null
  draft.name = ''
  draft.description = ''
  showError.value = false
}

function startEdit(category: AdminInvoiceCategory) {
  editingId.value = category.id
  draft.name = category.name
  draft.description = category.description
  showError.value = false
}

async function save() {
  if (saving.value) return
  const name = draft.name.trim()
  if (!name) {
    showError.value = true
    return
  }
  saving.value = true
  try {
    const payload = { name, description: draft.description.trim() }
    if (editingId.value === null) await adminInvoiceAPI.createCategory(payload)
    else await adminInvoiceAPI.updateCategory(editingId.value, payload)
    appStore.showSuccess(t('invoice.categories.saved'))
    resetDraft()
    await load()
    emit('changed')
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'invoice.errors', t('invoice.categories.saveFailed')))
  } finally {
    saving.value = false
  }
}

// ---- 行内操作：失败后重新拉一次列表，避免界面和服务端不一致 ----
async function run(id: number, action: () => Promise<void>, fallbackKey: string) {
  busyId.value = id
  try {
    await action()
    emit('changed')
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'invoice.errors', t(fallbackKey)))
    await load()
  } finally {
    busyId.value = null
  }
}

function toggle(category: AdminInvoiceCategory) {
  return run(category.id, async () => {
    await adminInvoiceAPI.updateCategory(category.id, { enabled: !category.enabled })
    await load()
  }, 'invoice.categories.toggleFailed')
}

function move(index: number, delta: -1 | 1) {
  const target = index + delta
  if (target < 0 || target >= items.value.length) return
  const ids = items.value.map((c) => c.id)
  ;[ids[index], ids[target]] = [ids[target], ids[index]]
  return run(items.value[index].id, async () => {
    items.value = await adminInvoiceAPI.reorderCategories(ids)
  }, 'invoice.categories.reorderFailed')
}

// ---- 删除：只有没被申请引用的类目能删 ----
const pendingDelete = ref<AdminInvoiceCategory | null>(null)

async function confirmDelete() {
  const target = pendingDelete.value
  pendingDelete.value = null
  if (!target) return
  await run(target.id, async () => {
    await adminInvoiceAPI.deleteCategory(target.id)
    if (editingId.value === target.id) resetDraft()
    appStore.showSuccess(t('invoice.categories.deleted'))
    await load()
  }, 'invoice.categories.deleteFailed')
}

watch(
  () => props.show,
  (open) => {
    if (!open) return
    resetDraft()
    pendingDelete.value = null
    void load()
  },
  { immediate: true },
)
</script>
