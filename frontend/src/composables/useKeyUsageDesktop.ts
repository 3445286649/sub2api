import { onBeforeUnmount, onMounted, ref } from 'vue'

// Tracks the md breakpoint (768px) used by the key usage screens.
export function useKeyUsageDesktop() {
  const query = '(min-width: 768px)'
  const canMatch = typeof window !== 'undefined' && typeof window.matchMedia === 'function'
  const isDesktop = ref(canMatch ? window.matchMedia(query).matches : true)
  let mql: MediaQueryList | undefined
  const update = (e: MediaQueryListEvent) => { isDesktop.value = e.matches }
  onMounted(() => {
    if (!canMatch) return
    mql = window.matchMedia(query)
    isDesktop.value = mql.matches
    mql.addEventListener?.('change', update)
  })
  onBeforeUnmount(() => mql?.removeEventListener?.('change', update))
  return isDesktop
}
