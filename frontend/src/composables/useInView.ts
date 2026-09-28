import { onMounted, onUnmounted, ref, type Ref } from 'vue'

/**
 * 元素是否在视口内。
 *
 * 用于让装饰性动画在滚出视口时暂停，避免后台空转。
 * 环境不支持 IntersectionObserver 时恒为 true——宁可多跑，也不能让动画永远不启动。
 */
export function useInView(
  target: Ref<HTMLElement | null>,
  options: IntersectionObserverInit = { threshold: 0 },
): Ref<boolean> {
  const inView = ref(true)
  let observer: IntersectionObserver | null = null

  onMounted(() => {
    if (typeof IntersectionObserver === 'undefined' || !target.value) return

    observer = new IntersectionObserver((entries) => {
      const entry = entries[entries.length - 1]
      if (entry) inView.value = entry.isIntersecting
    }, options)
    observer.observe(target.value)
  })

  onUnmounted(() => {
    observer?.disconnect()
    observer = null
  })

  return inView
}
