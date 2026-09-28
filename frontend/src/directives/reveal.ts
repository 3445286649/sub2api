import type { Directive } from 'vue'

/**
 * v-reveal：元素滚入视口时淡入上浮，只播一次。
 *
 * 用法：`v-reveal` 或 `v-reveal="index"`（index 用于同组元素错峰，每级 70ms）。
 *
 * 样式在 src/style.css 的 `.cp-reveal`。入场用 animation 的 backwards 填充而非
 * transition：播完后回落到元素自身样式，不会吞掉卡片原有的 hover 位移。
 *
 * 不支持 IntersectionObserver 或系统开启「减少动画」时直接显示，
 * 保证内容永远不会因为观察器失效而一直透明。
 */

let observer: IntersectionObserver | null = null

function reveal(el: Element): void {
  el.classList.add('is-revealed')
}

function getObserver(): IntersectionObserver | null {
  if (typeof IntersectionObserver === 'undefined') return null
  if (!observer) {
    observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (!entry.isIntersecting) return
          reveal(entry.target)
          observer?.unobserve(entry.target)
        })
      },
      // 底部收进 8%：元素露出一点点就播，太早播用户看不到
      { threshold: 0.12, rootMargin: '0px 0px -8% 0px' },
    )
  }
  return observer
}

function prefersReducedMotion(): boolean {
  return (
    typeof window !== 'undefined' &&
    typeof window.matchMedia === 'function' &&
    window.matchMedia('(prefers-reduced-motion: reduce)').matches
  )
}

export const vReveal: Directive<HTMLElement, number | undefined> = {
  mounted(el, binding) {
    el.classList.add('cp-reveal')
    if (typeof binding.value === 'number') {
      el.style.setProperty('--reveal-i', String(binding.value))
    }

    const io = prefersReducedMotion() ? null : getObserver()
    if (!io) {
      reveal(el)
      return
    }
    io.observe(el)
  },
  unmounted(el) {
    observer?.unobserve(el)
  },
}
