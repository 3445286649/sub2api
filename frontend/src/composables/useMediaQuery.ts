import { onMounted, onUnmounted, ref, type Ref } from 'vue'

/**
 * 响应式的 matchMedia。查询结果变化（窗口缩放、系统切换「减少动画」）时同步更新。
 * 不支持 matchMedia 的环境（jsdom 等）返回 false。
 */
export function useMediaQuery(query: string): Ref<boolean> {
  const matches = ref(false)
  let mql: MediaQueryList | null = null

  const update = () => {
    matches.value = mql?.matches ?? false
  }

  onMounted(() => {
    if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return
    mql = window.matchMedia(query)
    update()
    mql.addEventListener?.('change', update)
  })

  onUnmounted(() => {
    mql?.removeEventListener?.('change', update)
    mql = null
  })

  return matches
}
