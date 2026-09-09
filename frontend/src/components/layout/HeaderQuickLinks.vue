<template>
  <div v-if="buttons.length" ref="root" class="relative min-w-0 flex-1" :style="{ maxWidth: naturalWidth ? `${naturalWidth}px` : undefined }" data-testid="header-quick-links" @keydown.esc.stop.prevent="closeAndFocus">
    <div class="flex items-center justify-end gap-2">
      <button v-for="(button, index) in buttons.slice(0, visibleCount)" :key="index" type="button" class="quick-link" :title="button.name" @click="select(button)">
        <Icon :name="icon(button.action)" size="sm" /><span class="truncate">{{ button.name }}</span>
      </button>
      <div v-if="overflow.length" class="relative shrink-0">
        <button ref="moreButton" type="button" class="quick-link" :aria-label="t('headerButtons.more')" :aria-expanded="open" aria-haspopup="true" @click="open = !open">
          <span>{{ t('headerButtons.more') }}</span><Icon name="chevronDown" size="sm" />
        </button>
        <div v-if="open" class="absolute right-0 top-full z-50 mt-2 max-h-[calc(100dvh-80px)] w-56 max-w-[calc(100vw-24px)] overflow-y-auto rounded-xl border border-gray-200 bg-white p-1 shadow-lg dark:border-dark-700 dark:bg-dark-800" :aria-label="t('headerButtons.more')">
          <button v-for="(button, index) in overflow" :key="index" type="button" class="dropdown-item w-full" :title="button.name" @click="select(button)">
            <Icon :name="icon(button.action)" size="sm" /><span class="truncate">{{ button.name }}</span>
          </button>
        </div>
      </div>
    </div>
    <!-- Measure intrinsic widths independently of which links currently fit. -->
    <div class="pointer-events-none invisible absolute left-0 top-0 h-0 w-0 overflow-hidden" aria-hidden="true" inert>
    <div ref="measure" class="flex w-max gap-2">
      <button v-for="(button, index) in buttons" :key="index" type="button" tabindex="-1" class="quick-link"><Icon :name="icon(button.action)" size="sm" /><span class="truncate">{{ button.name }}</span></button>
      <button type="button" tabindex="-1" class="quick-link"><span>{{ t('headerButtons.more') }}</span><Icon name="chevronDown" size="sm" /></button>
    </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { HeaderButton } from '@/types'

const props = defineProps<{ buttons: HeaderButton[] }>()
const emit = defineEmits<{ select: [HeaderButton] }>()
const { t } = useI18n()
const root = ref<HTMLElement | null>(null)
const measure = ref<HTMLElement | null>(null)
const moreButton = ref<HTMLButtonElement | null>(null)
const visibleCount = ref(0)
const naturalWidth = ref(0)
const open = ref(false)
const overflow = computed(() => props.buttons.slice(visibleCount.value))
let observer: ResizeObserver | undefined
let frame = 0

function icon(action: HeaderButton['action']) {
  return ({ external: 'externalLink', recharge: 'creditCard', rebate: 'gift', help: 'questionCircle', radar: 'sparkles', support: 'chat', pixmo: 'sparkles' } as const)[action]
}
function layout() {
  if (!root.value || !measure.value) return
  const widths = Array.from(measure.value.children, child => child.getBoundingClientRect().width)
  const moreWidth = widths.pop() ?? 0
  const available = root.value.getBoundingClientRect().width
  const total = widths.reduce((sum, width) => sum + width, 0) + Math.max(0, widths.length - 1) * 8
  naturalWidth.value = Math.max(total, moreWidth)
  let count = widths.length
  if (total > available) {
    count = 0
    let used = moreWidth
    for (const width of widths) {
      if (used + 8 + width > available) break
      used += 8 + width
      count++
    }
  }
  visibleCount.value = count
  if (count === props.buttons.length) open.value = false
}
function scheduleLayout() {
  cancelAnimationFrame(frame)
  frame = requestAnimationFrame(layout)
}
function select(button: HeaderButton) {
  open.value = false
  emit('select', button)
}
function closeAndFocus() {
  if (!open.value) return
  open.value = false
  moreButton.value?.focus()
}
function outside(event: MouseEvent) {
  if (!root.value?.contains(event.target as Node)) open.value = false
}
async function observe() {
  await nextTick()
  observer?.disconnect()
  if (root.value) observer?.observe(root.value)
  if (measure.value) observer?.observe(measure.value)
  scheduleLayout()
}
watch(() => props.buttons, observe, { deep: true })
onMounted(() => {
  observer = new ResizeObserver(scheduleLayout)
  void observe()
  document.addEventListener('click', outside)
})
onBeforeUnmount(() => {
  observer?.disconnect()
  cancelAnimationFrame(frame)
  document.removeEventListener('click', outside)
})
</script>

<style scoped>
.quick-link {
  @apply inline-flex max-w-36 shrink-0 items-center gap-2 whitespace-nowrap rounded-lg bg-primary-50 px-3 py-1.5 text-sm font-semibold text-primary-700 transition-colors hover:bg-primary-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 dark:bg-primary-900/20 dark:text-primary-300;
}
</style>
