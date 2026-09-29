import { ref, watch, onScopeDispose, type Ref } from 'vue'

/**
 * 把数字变化做成平滑滚动，而不是瞬间跳变。
 *
 * 用在支付摘要里的金额上：改金额或改支付方式时，实付、到账这些数字滑动到新值，
 * 比直接替换更容易看清变化了多少。
 *
 * 系统开启「减少动态效果」时直接跳到目标值，不做动画。
 */
export function useCountUp(source: Ref<number>, duration = 420) {
  const display = ref(source.value)
  let frame = 0
  let startTime = 0
  let from = source.value
  let to = source.value

  function prefersReducedMotion(): boolean {
    if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return false
    return window.matchMedia('(prefers-reduced-motion: reduce)').matches
  }

  // 缓出：起步快、收尾稳，适合金额这种数字变化
  function easeOutCubic(t: number): number {
    return 1 - Math.pow(1 - t, 3)
  }

  function stop() {
    if (frame) {
      cancelAnimationFrame(frame)
      frame = 0
    }
  }

  function tick(now: number) {
    const elapsed = now - startTime
    const progress = duration > 0 ? Math.min(elapsed / duration, 1) : 1
    display.value = from + (to - from) * easeOutCubic(progress)
    if (progress < 1) {
      frame = requestAnimationFrame(tick)
    } else {
      display.value = to
      frame = 0
    }
  }

  watch(source, (value) => {
    const target = Number.isFinite(value) ? value : 0
    if (target === to) return
    stop()
    to = target

    if (prefersReducedMotion() || typeof requestAnimationFrame !== 'function') {
      from = target
      display.value = target
      return
    }

    // 从当前显示值起步，避免上一次动画没结束就再次改金额时出现回跳
    from = display.value
    startTime = performance.now()
    frame = requestAnimationFrame(tick)
  })

  onScopeDispose(stop)

  return { display }
}
