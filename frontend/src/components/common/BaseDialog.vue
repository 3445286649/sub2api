<template>
  <Teleport to="body">
    <Transition name="modal">
      <div
        v-if="show"
        :class="['modal-overlay', sheetActive && 'modal-overlay--sheet']"
        :style="zIndexStyle"
        :aria-labelledby="dialogId"
        role="dialog"
        aria-modal="true"
        @click.self="handleClose"
      >
        <!-- Modal panel -->
        <div
          ref="dialogRef"
          :class="['modal-content', widthClasses, sheetActive && 'modal-content--sheet']"
          :style="sheetStyle"
          data-testid="base-dialog-panel"
          @click.stop
          @pointerdown="onSheetPointerDown"
          @pointermove="onSheetPointerMove"
          @pointerup="onSheetPointerUp"
          @pointercancel="onSheetPointerUp"
        >
          <div v-if="sheetActive" class="flex h-6 shrink-0 items-start justify-center pt-2" data-sheet-handle aria-hidden="true">
            <div class="h-1 w-9 rounded-full bg-ku-border"></div>
          </div>
          <!-- Header — override entirely with #header slot if provided -->
          <slot name="header" :title-id="dialogId">
            <div class="modal-header">
              <h3 :id="dialogId" class="modal-title">
                {{ title }}
              </h3>
              <button
                v-if="showCloseButton"
                @click="emit('close')"
                class="-mr-2 rounded-xl p-2 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/30 focus-visible:ring-offset-2 dark:text-dark-500 dark:hover:bg-dark-700 dark:hover:text-dark-300 dark:focus-visible:ring-offset-dark-900"
                aria-label="Close modal"
              >
                <Icon name="x" size="md" />
              </button>
            </div>
          </slot>

          <!-- Body -->
          <div ref="modalBodyRef" :class="['modal-body', sheetActive && 'overscroll-contain pb-[calc(1rem+env(safe-area-inset-bottom))]']">
            <slot></slot>
          </div>

          <!-- Footer -->
          <div v-if="$slots.footer" class="modal-footer">
            <slot name="footer"></slot>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script lang="ts">
let dialogIdCounter = 0
const openDialogs = new Set<string>()
</script>

<script setup lang="ts">
import { computed, watch, onMounted, onUnmounted, ref, nextTick } from 'vue'
import Icon from '@/components/icons/Icon.vue'

// 生成唯一ID以避免多个对话框时ID冲突
const dialogId = `modal-title-${++dialogIdCounter}`

// 焦点管理
const dialogRef = ref<HTMLElement | null>(null)
const modalBodyRef = ref<HTMLElement | null>(null)
let previousActiveElement: HTMLElement | null = null

type DialogWidth = 'narrow' | 'normal' | 'wide' | 'extra-wide' | 'full' | 'detail'

interface Props {
  show: boolean
  title: string
  width?: DialogWidth
  closeOnEscape?: boolean
  closeOnClickOutside?: boolean
  showCloseButton?: boolean
  zIndex?: number
  /** Below md, render as a bottom sheet (92dvh) with a drag handle and swipe-to-close. */
  mobileSheet?: boolean
}

interface Emits {
  (e: 'close'): void
}

const props = withDefaults(defineProps<Props>(), {
  width: 'normal',
  closeOnEscape: true,
  closeOnClickOutside: false,
  showCloseButton: true,
  zIndex: 50,
  mobileSheet: false
})

const emit = defineEmits<Emits>()

// Custom z-index style (overrides the default z-50 from CSS)
const zIndexStyle = computed(() => {
  return props.zIndex !== 50 ? { zIndex: props.zIndex } : undefined
})

const widthClasses = computed(() => {
  // Width guidance: narrow=confirm/short prompts, normal=standard forms,
  // wide=multi-section forms or rich content, extra-wide=analytics/tables,
  // full=full-screen or very dense layouts.
  const widths: Record<DialogWidth, string> = {
    narrow: 'max-w-md',
    normal: 'max-w-lg',
    wide: 'w-full sm:max-w-2xl md:max-w-3xl lg:max-w-4xl',
    'extra-wide': 'w-full sm:max-w-3xl md:max-w-4xl lg:max-w-5xl xl:max-w-6xl',
    full: 'w-full sm:max-w-4xl md:max-w-5xl lg:max-w-6xl xl:max-w-7xl',
    detail: 'w-full md:max-w-[880px]'
  }
  return widths[props.width]
})

// Bottom sheet (mobile only)
const mobileQuery = '(max-width: 767.98px)'
const isMobile = ref(typeof window !== 'undefined' && typeof window.matchMedia === 'function' && window.matchMedia(mobileQuery).matches)
let mobileMql: MediaQueryList | undefined
const onMobileChange = (e: MediaQueryListEvent) => { isMobile.value = e.matches }
const sheetActive = computed(() => props.mobileSheet && isMobile.value)
const dragOffset = ref(0)
const dragging = ref(false)
let dragStartY = 0
let dragStartTime = 0
let dragPointerId: number | null = null
let dragCandidate = false
const reducedMotion = () => typeof window !== 'undefined' && typeof window.matchMedia === 'function' && window.matchMedia('(prefers-reduced-motion: reduce)').matches
const sheetStyle = computed(() => {
  if (!sheetActive.value || dragOffset.value === 0) return undefined
  return { transform: `translateY(${dragOffset.value}px)`, transition: dragging.value || reducedMotion() ? 'none' : undefined }
})

// The handle/header can always drag; the body only when it is scrolled to the top.
function canStartDrag(target: EventTarget | null) {
  if (!(target instanceof Element)) return false
  if (target.closest('input, textarea, select, [contenteditable="true"]')) return false
  if (target.closest('[data-sheet-handle], .modal-header')) return true
  const body = modalBodyRef.value
  return !!body && body.contains(target) && body.scrollTop <= 0
}
function onSheetPointerDown(e: PointerEvent) {
  if (!sheetActive.value || !canStartDrag(e.target)) return
  dragCandidate = true
  dragPointerId = e.pointerId
  dragStartY = e.clientY
  dragStartTime = performance.now()
}
function onSheetPointerMove(e: PointerEvent) {
  if (!dragCandidate || e.pointerId !== dragPointerId) return
  const dy = e.clientY - dragStartY
  if (!dragging.value) {
    // Upward movement means the user wants to scroll the content instead.
    if (dy < 0) { dragCandidate = false; return }
    if (dy < 6) return
    dragging.value = true
  }
  dragOffset.value = Math.max(0, dy)
}
function onSheetPointerUp(e: PointerEvent) {
  if (!dragCandidate || e.pointerId !== dragPointerId) return
  dragCandidate = false
  dragPointerId = null
  if (!dragging.value) return
  dragging.value = false
  const distance = Math.max(0, e.clientY - dragStartY)
  const elapsed = Math.max(1, performance.now() - dragStartTime)
  const height = dialogRef.value?.getBoundingClientRect().height || 0
  const threshold = Math.min(120, height * 0.25 || 120)
  if (distance > threshold || distance / elapsed > 0.5) emit('close')
  dragOffset.value = 0
}

const handleClose = () => {
  if (props.closeOnClickOutside) {
    emit('close')
  }
}

const handleEscape = (event: KeyboardEvent) => {
  if (props.show && props.closeOnEscape && event.key === 'Escape' && [...openDialogs].pop() === dialogId) {
    emit('close')
  }
}

const updateScrollLock = (isOpen: boolean) => {
  if (isOpen) openDialogs.add(dialogId)
  else openDialogs.delete(dialogId)
  document.body.classList.toggle('modal-open', openDialogs.size > 0)
}

// Prevent body scroll when modal is open and manage focus
watch(
  () => props.show,
  async (isOpen) => {
    if (isOpen) {
      // 保存当前焦点元素
      previousActiveElement = document.activeElement as HTMLElement
      // 使用CSS类而不是直接操作style,更易于管理多个对话框
      updateScrollLock(true)

      // 等待DOM更新后设置焦点到对话框
      await nextTick()
      if (modalBodyRef.value) {
        modalBodyRef.value.scrollTop = 0
      }
      if (dialogRef.value) {
        const firstFocusable = dialogRef.value.querySelector<HTMLElement>(
          'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
        )
        firstFocusable?.focus()
      }
    } else {
      updateScrollLock(false)
      // 恢复之前的焦点
      if (previousActiveElement && typeof previousActiveElement.focus === 'function') {
        previousActiveElement.focus()
      }
      previousActiveElement = null
    }
  },
  { immediate: true }
)

onMounted(() => {
  document.addEventListener('keydown', handleEscape)
  if (typeof window !== 'undefined' && typeof window.matchMedia === 'function') {
    mobileMql = window.matchMedia(mobileQuery)
    isMobile.value = mobileMql.matches
    mobileMql.addEventListener?.('change', onMobileChange)
  }
})

onUnmounted(() => {
  document.removeEventListener('keydown', handleEscape)
  mobileMql?.removeEventListener?.('change', onMobileChange)
  // 确保组件卸载时移除滚动锁定
  updateScrollLock(false)
})
</script>
