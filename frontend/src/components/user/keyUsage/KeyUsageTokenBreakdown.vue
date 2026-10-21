<template>
  <HelpTooltip v-if="isDesktop && rows.length" width-class="w-56" class="!ml-0" data-testid="ku-token">
    <template #trigger>
      <span tabindex="0" class="cursor-help tabular-nums underline decoration-dotted decoration-ku-text-3 underline-offset-4 focus:outline-none focus-visible:ring-2 focus-visible:ring-ku-primary/40" @focus="openFromFocus" @blur="closeFromBlur" @keydown.esc="closeFromBlur">{{ text }}</span>
    </template>
    <dl class="space-y-1" data-testid="ku-token-rows"><div v-for="row in rows" :key="row.key" class="flex justify-between gap-3"><dt class="text-gray-300">{{ row.label }}</dt><dd class="tabular-nums">{{ row.value }}<span v-if="row.share" class="ml-1.5 text-gray-400">{{ row.share }}</span></dd></div></dl>
  </HelpTooltip>
  <span v-else-if="isDesktop || !rows.length" class="tabular-nums">{{ text }}</span>
  <span v-else class="contents">
    <button type="button" class="tabular-nums underline decoration-dotted decoration-ku-text-3 underline-offset-4" :aria-expanded="open" data-testid="ku-token-toggle" @click.stop="open = !open">{{ text }}</button>
    <dl v-if="open" class="mt-2 grid w-full basis-full grid-cols-[auto_1fr] gap-x-3 gap-y-1 rounded-lg bg-ku-bg p-2 text-xs text-ku-text-2" data-testid="ku-token-rows" @click.stop><template v-for="row in rows" :key="row.key"><dt>{{ row.label }}</dt><dd class="text-right tabular-nums text-ku-text">{{ row.value }}<span v-if="row.share" class="ml-1.5 text-ku-text-3">{{ row.share }}</span></dd></template></dl>
  </span>
</template>

<script setup lang="ts">
import { computed, getCurrentInstance, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import { useKeyUsageDesktop } from '@/composables/useKeyUsageDesktop'
import type { APIKeyUsageTotals } from '@/api/usage'

type TokenFields = Partial<Pick<APIKeyUsageTotals, 'input_tokens' | 'output_tokens' | 'cache_read_tokens' | 'cache_creation_tokens'>> & { total_tokens: number }
const props = defineProps<{ totals: TokenFields; text: string }>()
const { t } = useI18n()
const isDesktop = useKeyUsageDesktop()
const open = ref(false)
const instance = getCurrentInstance()
const rows = computed(() => {
  const total = props.totals.total_tokens
  const fields = [
    { key: 'input_tokens', label: t('admin.usage.inputTokens') },
    { key: 'output_tokens', label: t('admin.usage.outputTokens') },
    { key: 'cache_read_tokens', label: t('usage.keySummary.cacheRead') },
    { key: 'cache_creation_tokens', label: t('usage.keySummary.cacheWrite') },
  ] as const
  // Missing/null fields are omitted rather than shown as zero.
  return fields.flatMap(f => {
    const v = props.totals[f.key]
    if (v === undefined || v === null) return []
    return [{ key: f.key, label: f.label, value: v.toLocaleString(), share: total > 0 ? `${((v / total) * 100).toFixed(1)}%` : '' }]
  })
})
// HelpTooltip opens on hover; mirror it for keyboard focus.
function tooltipEl() { return instance?.subTree.el as HTMLElement | undefined }
function openFromFocus() { tooltipEl()?.dispatchEvent(new MouseEvent('mouseenter')) }
function closeFromBlur() { tooltipEl()?.dispatchEvent(new MouseEvent('mouseleave')) }
</script>
