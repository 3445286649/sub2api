<template>
  <div class="space-y-5">
    <div v-if="categoryOptions.length > 0" data-testid="invoice-category">
      <label class="input-label" for="inv-category">{{ t('invoice.category.label') }}<span class="text-red-500"> *</span></label>
      <div
        v-if="categoryOptions.length === 1"
        id="inv-category"
        class="rounded-xl border border-gray-200 bg-gray-50 px-4 py-2.5 text-sm text-gray-900 dark:border-dark-600 dark:bg-dark-900/40 dark:text-gray-100"
        data-testid="invoice-category-fixed"
      >{{ categoryOptions[0].label }}</div>
      <Select
        v-else
        id="inv-category"
        :model-value="form.category_id"
        :options="categoryOptions"
        :placeholder="t('invoice.category.placeholder')"
        :error="Boolean(errorOf('category_id'))"
        data-testid="invoice-category-select"
        @update:model-value="onCategoryChange"
      />
      <p v-if="errorOf('category_id')" class="input-error-text">{{ errorOf('category_id') }}</p>
      <p v-else-if="describedCategory?.description" class="input-hint">{{ describedCategory.description }}</p>
    </div>

    <div class="grid grid-cols-2 gap-3" role="radiogroup" :aria-label="t('invoice.confirm.type')">
      <button
        v-for="option in typeOptions"
        :key="option.value"
        type="button"
        role="radio"
        :aria-checked="form.invoice_type === option.value"
        class="inv-type"
        :class="{ 'inv-type-active': form.invoice_type === option.value }"
        :data-testid="`invoice-type-${option.value}`"
        @click="form.invoice_type = option.value"
      >
        <span class="inv-type-icon">
          <Icon :name="option.icon" size="md" />
        </span>
        <span class="min-w-0 text-left">
          <span class="block text-sm font-semibold text-gray-900 dark:text-white">{{ option.label }}</span>
          <span class="mt-0.5 block truncate text-xs text-gray-500 dark:text-gray-400">{{ option.hint }}</span>
        </span>
      </button>
    </div>

    <Transition name="inv-fade" mode="out-in">
      <div v-if="form.invoice_type === 'enterprise'" key="enterprise" class="space-y-4">
        <div class="grid gap-4 sm:grid-cols-2">
          <div class="sm:col-span-2">
            <label class="input-label" for="inv-enterprise-name">{{ t('invoice.title.enterpriseName') }}<span class="text-red-500"> *</span></label>
            <input id="inv-enterprise-name" v-model="form.enterprise_name" class="input" :class="fieldClass('enterprise_name')"
              :placeholder="t('invoice.title.enterpriseNamePlaceholder')" autocomplete="organization" @blur="touch('enterprise_name')" />
            <p v-if="errorOf('enterprise_name')" class="input-error-text">{{ errorOf('enterprise_name') }}</p>
          </div>
          <div class="sm:col-span-2">
            <label class="input-label" for="inv-tax">{{ t('invoice.title.taxNumber') }}<span class="text-red-500"> *</span></label>
            <input id="inv-tax" v-model="form.enterprise_tax_number" class="input font-mono uppercase" :class="fieldClass('enterprise_tax_number')"
              :placeholder="t('invoice.title.taxNumberPlaceholder')" maxlength="20" spellcheck="false" @blur="touch('enterprise_tax_number')" />
            <p v-if="errorOf('enterprise_tax_number')" class="input-error-text">{{ errorOf('enterprise_tax_number') }}</p>
          </div>
        </div>

        <button
          type="button"
          class="flex w-full items-center justify-between rounded-xl border border-dashed border-gray-200 px-4 py-2.5 text-sm text-gray-600 transition-colors hover:border-primary-300 hover:text-primary-700 dark:border-dark-600 dark:text-gray-300 dark:hover:border-primary-700 dark:hover:text-primary-300"
          :aria-expanded="showMore"
          @click="showMore = !showMore"
        >
          <span>{{ t('invoice.title.moreFields') }}</span>
          <Icon name="chevronDown" size="sm" class="transition-transform duration-200" :class="{ 'rotate-180': showMore }" />
        </button>
        <div v-show="showMore" class="grid gap-4 sm:grid-cols-2">
          <div class="sm:col-span-2">
            <label class="input-label" for="inv-address">{{ t('invoice.title.address') }}</label>
            <input id="inv-address" v-model="form.enterprise_address" class="input" :placeholder="t('invoice.title.optional')" />
          </div>
          <div>
            <label class="input-label" for="inv-phone">{{ t('invoice.title.phone') }}</label>
            <input id="inv-phone" v-model="form.enterprise_phone" class="input" :placeholder="t('invoice.title.optional')" />
          </div>
          <div>
            <label class="input-label" for="inv-bank">{{ t('invoice.title.bank') }}</label>
            <input id="inv-bank" v-model="form.enterprise_bank" class="input" :placeholder="t('invoice.title.optional')" />
          </div>
          <div class="sm:col-span-2">
            <label class="input-label" for="inv-bank-account">{{ t('invoice.title.bankAccount') }}</label>
            <input id="inv-bank-account" v-model="form.enterprise_bank_account" class="input font-mono" :class="fieldClass('enterprise_bank_account')"
              :placeholder="t('invoice.title.optional')" inputmode="numeric" @blur="touch('enterprise_bank_account')" />
            <p v-if="errorOf('enterprise_bank_account')" class="input-error-text">{{ errorOf('enterprise_bank_account') }}</p>
          </div>
        </div>
      </div>

      <div v-else key="individual" class="grid gap-4 sm:grid-cols-2">
        <div>
          <label class="input-label" for="inv-name">{{ t('invoice.title.individualName') }}<span class="text-red-500"> *</span></label>
          <input id="inv-name" v-model="form.individual_name" class="input" :class="fieldClass('individual_name')"
            :placeholder="t('invoice.title.individualNamePlaceholder')" autocomplete="name" @blur="touch('individual_name')" />
          <p v-if="errorOf('individual_name')" class="input-error-text">{{ errorOf('individual_name') }}</p>
        </div>
        <div>
          <label class="input-label" for="inv-id">{{ t('invoice.title.idNumber') }}</label>
          <input id="inv-id" v-model="form.individual_id_number" class="input font-mono uppercase" :class="fieldClass('individual_id_number')"
            :placeholder="t('invoice.title.idNumberPlaceholder')" maxlength="18" spellcheck="false" @blur="touch('individual_id_number')" />
          <p v-if="errorOf('individual_id_number')" class="input-error-text">{{ errorOf('individual_id_number') }}</p>
        </div>
      </div>
    </Transition>

    <div class="border-t border-gray-100 pt-5 dark:border-dark-700/60">
      <label class="input-label" for="inv-email">{{ t('invoice.title.receiverEmail') }}<span class="text-red-500"> *</span></label>
      <div class="relative">
        <Icon name="mail" size="sm" class="pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 text-gray-400 dark:text-dark-500" />
        <input id="inv-email" v-model="form.receiver_email" type="email" class="input pl-10" :class="fieldClass('receiver_email')"
          autocomplete="email" @blur="touch('receiver_email')" />
      </div>
      <p v-if="errorOf('receiver_email')" class="input-error-text">{{ errorOf('receiver_email') }}</p>
      <p v-else class="input-hint">{{ t('invoice.title.receiverEmailHint') }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import Select from '@/components/common/Select.vue'
import type { InvoiceCategoryOption, InvoiceTitleForm, InvoiceType } from '@/types/invoice'
import type { InvoiceFieldErrors } from './invoiceFormat'

const form = defineModel<InvoiceTitleForm>({ required: true })
const props = defineProps<{ errors: InvoiceFieldErrors; showAllErrors: boolean; categories?: InvoiceCategoryOption[] }>()
const { t } = useI18n()

const showMore = ref(Boolean(form.value.enterprise_address || form.value.enterprise_bank || form.value.enterprise_phone))
const touched = ref(new Set<keyof InvoiceTitleForm>())

const typeOptions = computed<{ value: InvoiceType; icon: 'database' | 'user'; label: string; hint: string }[]>(() => [
  { value: 'enterprise', icon: 'database', label: t('invoice.title.enterprise'), hint: t('invoice.title.enterpriseHint') },
  { value: 'individual', icon: 'user', label: t('invoice.title.individual'), hint: t('invoice.title.individualHint') },
])

const categoryOptions = computed(() =>
  (props.categories ?? []).map((c) => ({ value: c.id, label: c.name, description: c.description })),
)

// 只有一个类目时它就是唯一选择，说明直接显示；多个类目时显示已选那个的说明
const describedCategory = computed(() => {
  const list = props.categories ?? []
  return list.length === 1 ? list[0] : list.find((c) => c.id === form.value.category_id)
})

function onCategoryChange(value: string | number | boolean | null) {
  form.value.category_id = typeof value === 'number' ? value : null
  touch('category_id')
}

function touch(field: keyof InvoiceTitleForm) {
  touched.value.add(field)
}

function errorOf(field: keyof InvoiceTitleForm): string {
  const key = props.errors[field]
  if (!key || (!props.showAllErrors && !touched.value.has(field))) return ''
  return t(`invoice.validation.${key}`)
}

function fieldClass(field: keyof InvoiceTitleForm) {
  return errorOf(field) ? 'input-error' : ''
}
</script>

<style scoped>
.inv-type {
  @apply flex items-center gap-3 rounded-xl border border-gray-200 bg-white/60 px-4 py-3.5 transition-all duration-200;
  @apply hover:border-primary-300 dark:border-dark-700 dark:bg-dark-800/40 dark:hover:border-primary-700/70;
}
.inv-type-active {
  @apply border-primary-500 bg-primary-50/70 ring-1 ring-primary-500/30 dark:border-primary-500/70 dark:bg-primary-500/10;
}
.inv-type-icon {
  @apply flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-gray-100 text-gray-500 transition-colors dark:bg-dark-700 dark:text-gray-400;
}
.inv-type-active .inv-type-icon {
  @apply bg-primary-500 text-white dark:bg-primary-500;
}
.inv-fade-enter-active,
.inv-fade-leave-active {
  transition: opacity 0.15s ease, transform 0.15s ease;
}
.inv-fade-enter-from,
.inv-fade-leave-to {
  opacity: 0;
  transform: translateY(4px);
}
</style>
