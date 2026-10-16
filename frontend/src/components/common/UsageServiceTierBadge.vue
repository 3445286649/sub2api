<script setup lang="ts">
import { computed, ref, useId } from 'vue'
import { useI18n } from 'vue-i18n'
import { onClickOutside, useEventListener } from '@vueuse/core'
import { normalizeUsageServiceTier } from '@/utils/usageServiceTier'

const props = defineProps<{ tier?: string | null; free?: boolean }>()
const { t } = useI18n()
const tier = computed(() => normalizeUsageServiceTier(props.tier))
const visible = computed(() => tier.value === 'priority' || tier.value === 'ultrafast')
const label = computed(() => t(props.free
  ? (tier.value === 'ultrafast' ? 'usage.freeUltrafastEnabled' : 'usage.freeFastEnabled')
  : (tier.value === 'ultrafast' ? 'usage.billedUltrafast' : 'usage.billedFast')))
const id = useId()
const maskId = `usage-tier-mask-${id}`
const tooltipId = `usage-tier-tooltip-${id}`
const trigger = ref<HTMLElement | null>(null)
const open = ref(false)
const position = ref({ left: '0px', top: '0px' })
const bolt = 'M10.6 2.1 2.6 12.9c-.4.5 0 1.1.6 1.1h5.2l-1.1 7.6c-.1.6.7.9 1 .4l7.8-11c.4-.5 0-1.1-.6-1.1h-5.1l1.3-7.4c.1-.6-.7-.9-1.1-.4Z'

function show() {
  if (!trigger.value) return
  const rect = trigger.value.getBoundingClientRect()
  position.value = {
    left: `${Math.max(Math.min(144, window.innerWidth / 2), Math.min(window.innerWidth - 144, rect.left + rect.width / 2))}px`,
    top: `${rect.top >= 40 ? rect.top - 34 : rect.bottom + 6}px`,
  }
  open.value = true
}
function close() { open.value = false }
onClickOutside(trigger, close)
useEventListener(window, 'scroll', close, true)
useEventListener(window, 'resize', close)
</script>

<template>
  <button
    v-if="visible"
    ref="trigger"
    type="button"
    data-testid="usage-tier-badge"
    :data-tier="tier"
    :aria-label="label"
    :aria-describedby="open ? tooltipId : undefined"
    class="inline-flex h-6 w-5 shrink-0 cursor-help items-center justify-center rounded text-amber-700 outline-none hover:bg-amber-500/10 focus-visible:ring-2 focus-visible:ring-amber-500 dark:text-[#f5c35b]"
    @mouseenter="show"
    @mouseleave="close"
    @focus="show"
    @blur="close"
    @click.stop="show"
    @keydown.esc.stop="close"
  >
    <svg v-if="tier === 'ultrafast'" class="h-[18px] w-4" viewBox="0 0 24 28" fill="currentColor" aria-hidden="true">
      <defs>
        <mask :id="maskId" maskUnits="userSpaceOnUse" x="0" y="0" width="24" height="28">
          <rect width="24" height="28" fill="white" />
          <path :d="bolt" transform="translate(6 3)" fill="black" stroke="black" stroke-width="2" stroke-linejoin="round" />
        </mask>
      </defs>
      <path :d="bolt" opacity=".68" :mask="`url(#${maskId})`" />
      <path :d="bolt" transform="translate(6 3)" />
    </svg>
    <svg v-else class="h-4 w-3" viewBox="0 0 18 24" fill="currentColor" aria-hidden="true">
      <path :d="bolt" />
    </svg>
  </button>
  <Teleport to="body">
    <div v-if="visible && open" :id="tooltipId" role="tooltip" :style="position"
      class="pointer-events-none fixed z-[99999] w-max max-w-[calc(100vw-16px)] -translate-x-1/2 rounded border border-gray-700 bg-gray-900 px-2.5 py-1.5 text-center text-xs text-white shadow-lg">
      {{ label }}
    </div>
  </Teleport>
</template>
