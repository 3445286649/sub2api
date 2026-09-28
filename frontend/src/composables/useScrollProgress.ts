import { onUnmounted, ref, watch, type Ref } from 'vue'

/** 把 0–1 的滚动进度映射为步骤下标。纯函数，便于单测。 */
export function progressToStep(progress: number, count: number): number {
  if (count <= 0 || !Number.isFinite(progress)) return 0
  const clamped = Math.min(Math.max(progress, 0), 1)
  return Math.min(count - 1, Math.floor(clamped * count))
}

/**
 * 计算「钉屏」区块的滚动进度：区块顶部到达视口顶部时为 0，
 * 区块底部到达视口底部时为 1。
 */
export function sectionProgress(rectTop: number, sectionHeight: number, viewportHeight: number): number {
  const scrollable = sectionHeight - viewportHeight
  if (scrollable <= 0) return 0
  return Math.min(Math.max(-rectTop / scrollable, 0), 1)
}

/**
 * 跟踪某个高区块在视口中的滚动进度。仅在 enabled 为 true 时监听，
 * 用 rAF 合并 scroll 事件，避免每次滚动都触发响应式更新。
 */
export function useScrollProgress(target: Ref<HTMLElement | null>, enabled: Ref<boolean>): Ref<number> {
  const progress = ref(0)
  let frame = 0

  const measure = () => {
    frame = 0
    const el = target.value
    if (!el) return
    progress.value = sectionProgress(el.getBoundingClientRect().top, el.offsetHeight, window.innerHeight)
  }

  const onScroll = () => {
    if (frame) return
    frame = requestAnimationFrame(measure)
  }

  const start = () => {
    window.addEventListener('scroll', onScroll, { passive: true })
    window.addEventListener('resize', onScroll, { passive: true })
    onScroll()
  }

  const stop = () => {
    window.removeEventListener('scroll', onScroll)
    window.removeEventListener('resize', onScroll)
    if (frame) cancelAnimationFrame(frame)
    frame = 0
  }

  watch(
    [enabled, target],
    ([isEnabled, el]) => {
      stop()
      if (isEnabled && el && typeof window !== 'undefined') start()
    },
    { immediate: true, flush: 'post' },
  )

  onUnmounted(stop)

  return progress
}
