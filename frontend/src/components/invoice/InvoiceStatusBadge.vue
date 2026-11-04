<template>
  <span
    class="inline-flex shrink-0 items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium ring-1 ring-inset"
    :class="meta.badgeClass"
  >
    <Icon :name="meta.icon" size="xs" :class="status === 'processing' ? 'animate-spin [animation-duration:2.5s]' : ''" />
    {{ t(`invoice.status.${status}`) }}
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { InvoiceRequestStatus } from '@/types/invoice'
import { INVOICE_STATUS_META } from './invoiceFormat'

const props = defineProps<{ status: InvoiceRequestStatus }>()
const { t } = useI18n()
const meta = computed(() => INVOICE_STATUS_META[props.status])
</script>
