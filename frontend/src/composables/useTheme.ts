import { ref, type Ref } from 'vue'

/**
 * 全局主题（深色/浅色）单一事实源。
 *
 * 改造前，main.ts / HomeView / KeyUsageView / AppSidebar / AuthLayout 各自实现了
 * 一份 initTheme + toggleTheme，读写同一组 localStorage 与 <html>.dark 类。
 * 只要其中一处行为漂移（例如读了 prefers-color-scheme 而另一处没读），
 * 用户在首页 → 登录 → 控制台之间就会看到深浅色跳变。
 *
 * 这里用模块级单例：所有调用方共享同一个 isDark ref 与同一套读写逻辑。
 * 切换时额外派发 THEME_CHANGE_EVENT，供 Chart.js 等非响应式消费方订阅。
 */

export const THEME_STORAGE_KEY = 'theme'
export const THEME_CHANGE_EVENT = 'themechange'

export type ThemeName = 'light' | 'dark'

/** 模块级单例：多个组件调用 useTheme() 拿到的是同一个 ref。 */
const isDark: Ref<boolean> = ref(false)

let initialized = false

function prefersDark(): boolean {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return false
  return window.matchMedia('(prefers-color-scheme: dark)').matches
}

function readStoredTheme(): ThemeName | null {
  try {
    const saved = localStorage.getItem(THEME_STORAGE_KEY)
    return saved === 'dark' || saved === 'light' ? saved : null
  } catch {
    // 隐私模式或禁用存储时不影响渲染，退回系统偏好
    return null
  }
}

function writeStoredTheme(theme: ThemeName): void {
  try {
    localStorage.setItem(THEME_STORAGE_KEY, theme)
  } catch {
    // 存储不可用时仅本次会话生效
  }
}

function applyToDocument(dark: boolean): void {
  if (typeof document === 'undefined') return
  document.documentElement.classList.toggle('dark', dark)
}

function emitThemeChange(dark: boolean): void {
  if (typeof window === 'undefined' || typeof window.dispatchEvent !== 'function') return
  window.dispatchEvent(
    new CustomEvent<{ theme: ThemeName; isDark: boolean }>(THEME_CHANGE_EVENT, {
      detail: { theme: dark ? 'dark' : 'light', isDark: dark },
    }),
  )
}

/**
 * 与 index.html 的内联脚本保持一致：显式选择优先，否则跟随系统偏好。
 * 幂等，可安全地在多个组件中调用。
 */
export function initTheme(): void {
  const stored = readStoredTheme()
  const dark = stored ? stored === 'dark' : prefersDark()

  isDark.value = dark
  applyToDocument(dark)
  initialized = true
}

/** 读取当前主题；若尚未初始化则先初始化（避免组件顶层读取到默认值）。 */
export function resolveTheme(): boolean {
  if (!initialized) initTheme()
  return isDark.value
}

export function setTheme(theme: ThemeName): void {
  const dark = theme === 'dark'
  isDark.value = dark
  applyToDocument(dark)
  writeStoredTheme(theme)
  emitThemeChange(dark)
}

export function toggleTheme(): void {
  setTheme(isDark.value ? 'light' : 'dark')
}

export interface UseThemeReturn {
  isDark: Ref<boolean>
  toggleTheme: () => void
  setTheme: (theme: ThemeName) => void
  initTheme: () => void
  resolveTheme: () => boolean
}

export function useTheme(): UseThemeReturn {
  return { isDark, toggleTheme, setTheme, initTheme, resolveTheme }
}
