<template>
  <div class="cp-auth-shell" @pointermove="handlePointerMove">
    <!-- 背景：网格 + 两个跟随鼠标轻微视差的光晕（原为三个，收敛以降低视觉噪音） -->
    <div class="cp-auth-grid" aria-hidden="true"></div>
    <div class="cp-auth-glow cp-auth-glow-a" :style="glowStyleA" aria-hidden="true"></div>
    <div class="cp-auth-glow cp-auth-glow-b" :style="glowStyleB" aria-hidden="true"></div>

    <div class="cp-auth-tools">
      <LocaleSwitcher />
      <button
        type="button"
        class="cp-auth-theme"
        :class="{ 'is-dark': isDark }"
        :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
        :aria-label="isDark ? t('home.switchToLight') : t('home.switchToDark')"
        @click="toggleTheme"
      >
        <Icon v-if="isDark" name="sun" size="sm" />
        <Icon v-else name="moon" size="sm" />
      </button>
    </div>

    <div class="cp-auth-layout">
      <!-- 左栏：品牌故事区（≥1024px 显示）。登录是转化点，不能只有一张表单。 -->
      <aside class="cp-auth-aside" aria-hidden="true">
        <div class="cp-auth-aside-brand">
          <img :src="siteLogo || '/logo.svg'" alt="" class="cp-auth-aside-logo" />
          <span class="cp-auth-aside-name">{{ siteName }}</span>
        </div>

        <h2 class="cp-auth-aside-title">{{ t('home.heroSubtitle') }}</h2>

        <!-- 三句卖点轮播 -->
        <div class="cp-auth-points">
          <Transition name="cp-point" mode="out-in">
            <p :key="pointIndex" class="cp-auth-point">
              <span class="cp-auth-point-dot"></span>
              {{ asidePoints[pointIndex] }}
            </p>
          </Transition>
          <div class="cp-auth-point-dots">
            <i
              v-for="(_, index) in asidePoints"
              :key="index"
              :class="{ 'is-active': index === pointIndex }"
            ></i>
          </div>
        </div>

        <RequestJourney compact class="cp-auth-journey" />

        <p class="cp-auth-aside-tools">{{ t('auth.trustTools') }}</p>
      </aside>

      <!-- 右栏：表单 -->
      <div class="cp-auth-main">
        <div class="cp-auth-brand-block">
          <div class="cp-auth-logo">
            <img :src="siteLogo || '/logo.svg'" alt="Logo" class="h-full w-full object-contain" />
          </div>
          <h1 class="cp-auth-brand">{{ siteName }}</h1>
          <p class="cp-auth-subtitle">{{ siteSubtitle }}</p>
        </div>

        <div class="cp-auth-card">
          <slot />
        </div>

        <p class="cp-auth-trust">
          <Icon name="lock" size="xs" />
          {{ t('auth.trustNote') }}
        </p>

        <div class="cp-auth-footer">
          <slot name="footer" />
        </div>

        <div class="cp-auth-copyright">
          &copy; {{ currentYear }} {{ siteName }}. All rights reserved.
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'
import { sanitizeUrl } from '@/utils/url'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import RequestJourney from '@/components/landing/RequestJourney.vue'
import { useTheme } from '@/composables/useTheme'

const { t } = useI18n()
const appStore = useAppStore()

const siteName = computed(() => appStore.siteName || 'Sub2API')
const siteLogo = computed(() => sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || 'Subscription to API Conversion Platform')
const currentYear = computed(() => new Date().getFullYear())
const { isDark, toggleTheme } = useTheme()

// ---------- 左栏卖点轮播 ----------
const asidePoints = computed(() => [
  t('auth.asidePoints.one'),
  t('auth.asidePoints.two'),
  t('auth.asidePoints.three'),
])

const pointIndex = ref(0)
let pointTimer: number | undefined

// ---------- 光晕鼠标视差 ----------
// 位移上限 8px：足够给出反馈，又不会让背景「晃」起来
const PARALLAX_PX = 8
const offset = ref({ x: 0, y: 0 })
let frame = 0
let pendingEvent: PointerEvent | null = null
let reducedMotion = false

const glowStyleA = computed(() => ({
  transform: `translate3d(${offset.value.x}px, ${offset.value.y}px, 0)`,
}))

const glowStyleB = computed(() => ({
  transform: `translate3d(${-offset.value.x}px, ${-offset.value.y}px, 0)`,
}))

function handlePointerMove(event: PointerEvent): void {
  if (reducedMotion || event.pointerType !== 'mouse') return
  pendingEvent = event
  if (frame) return
  // rAF 节流：pointermove 触发频率远高于刷新率
  frame = requestAnimationFrame(() => {
    frame = 0
    if (!pendingEvent) return
    const { innerWidth, innerHeight } = window
    offset.value = {
      x: ((pendingEvent.clientX / innerWidth) - 0.5) * 2 * PARALLAX_PX,
      y: ((pendingEvent.clientY / innerHeight) - 0.5) * 2 * PARALLAX_PX,
    }
    pendingEvent = null
  })
}

onMounted(() => {
  appStore.fetchPublicSettings()

  if (typeof window !== 'undefined' && typeof window.matchMedia === 'function') {
    reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches
  }

  if (!reducedMotion) {
    pointTimer = window.setInterval(() => {
      pointIndex.value = (pointIndex.value + 1) % asidePoints.value.length
    }, 4000)
  }
})

onUnmounted(() => {
  if (pointTimer) window.clearInterval(pointTimer)
  if (frame) cancelAnimationFrame(frame)
})
</script>

<style scoped>
/* 配色全部走全局语义 token（src/style.css），深浅色自动跟随 */
.cp-auth-shell {
  position: relative;
  display: flex;
  min-height: 100vh;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  padding: 48px 20px;
  background: var(--surface-2);
  color: var(--text);
}

/* ---------- 背景 ---------- */
.cp-auth-grid {
  position: absolute;
  inset: 0;
  pointer-events: none;
  /* 原 0.75，降低以减少噪音 */
  opacity: 0.5;
  background-image:
    linear-gradient(color-mix(in srgb, var(--brand) 5%, transparent) 1px, transparent 1px),
    linear-gradient(90deg, color-mix(in srgb, var(--brand) 5%, transparent) 1px, transparent 1px);
  background-size: 56px 56px;
  mask-image: radial-gradient(ellipse 92% 70% at 50% 50%, #000 8%, transparent 72%);
}

.cp-auth-glow {
  position: absolute;
  width: 520px;
  height: 520px;
  border-radius: 999px;
  pointer-events: none;
  filter: blur(60px);
  transition: transform 0.6s cubic-bezier(0.22, 1, 0.36, 1);
  will-change: transform;
}

.cp-auth-glow-a {
  top: -180px;
  left: -140px;
  background: var(--glow-brand);
}

.cp-auth-glow-b {
  right: -160px;
  bottom: -200px;
  background: var(--glow-brand-2);
}

/* ---------- 右上角工具 ---------- */
.cp-auth-tools {
  position: absolute;
  top: 16px;
  right: 20px;
  z-index: 20;
  display: flex;
  align-items: center;
  gap: 8px;
}

.cp-auth-tools :deep(.relative > button) {
  color: var(--muted);
}

.cp-auth-theme {
  display: grid;
  width: 36px;
  height: 36px;
  place-items: center;
  border: 1px solid var(--border);
  border-radius: 10px;
  color: var(--muted);
  background: transparent;
  transition: border-color 0.22s ease, background 0.22s ease, transform 0.3s ease;
}

.cp-auth-theme:hover {
  border-color: color-mix(in srgb, var(--brand) 42%, var(--border));
  background: color-mix(in srgb, var(--brand) 10%, transparent);
}

/* 切换时图标旋转一次 */
.cp-auth-theme.is-dark {
  transform: rotate(180deg);
}

/* ---------- 布局 ---------- */
.cp-auth-layout {
  position: relative;
  z-index: 10;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 64px;
  width: 100%;
  max-width: 1080px;
}

/* ---------- 左栏 ---------- */
.cp-auth-aside {
  display: none;
  flex: 1 1 0;
  min-width: 0;
  max-width: 480px;
}

.cp-auth-aside-brand {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 26px;
}

.cp-auth-aside-logo {
  width: 36px;
  height: 36px;
  border-radius: 10px;
  object-fit: contain;
}

.cp-auth-aside-name {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 15px;
  font-weight: 700;
}

.cp-auth-aside-title {
  margin: 0 0 22px;
  font-size: 34px;
  font-weight: 750;
  letter-spacing: -0.035em;
  line-height: 1.15;
  background: linear-gradient(100deg, var(--brand-soft), var(--brand), var(--brand-2));
  background-clip: text;
  -webkit-background-clip: text;
  color: transparent;
}

.cp-auth-points {
  min-height: 56px;
  margin-bottom: 28px;
}

.cp-auth-point {
  display: flex;
  align-items: center;
  gap: 10px;
  margin: 0;
  color: var(--text);
  font-size: 15.5px;
  font-weight: 600;
}

.cp-auth-point-dot {
  width: 7px;
  height: 7px;
  flex-shrink: 0;
  border-radius: 999px;
  background: var(--accent-ok);
  box-shadow: 0 0 0 4px color-mix(in srgb, var(--accent-ok) 15%, transparent);
}

.cp-auth-point-dots {
  display: flex;
  gap: 6px;
  margin-top: 14px;
  padding-left: 17px;
}

.cp-auth-point-dots i {
  width: 18px;
  height: 3px;
  border-radius: 999px;
  background: var(--border-strong);
  transition: background 0.3s ease, width 0.3s ease;
}

.cp-auth-point-dots i.is-active {
  width: 28px;
  background: var(--brand);
}

.cp-point-enter-active,
.cp-point-leave-active {
  transition: opacity 0.35s ease, transform 0.35s ease;
}

.cp-point-enter-from {
  opacity: 0;
  transform: translateY(8px);
}

.cp-point-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}

.cp-auth-journey {
  margin-bottom: 20px;
}

.cp-auth-aside-tools {
  margin: 0;
  color: var(--muted);
  font-size: 12.5px;
}

/* ---------- 右栏 ---------- */
.cp-auth-main {
  width: 100%;
  max-width: 440px;
}

.cp-auth-brand-block {
  margin-bottom: 28px;
  text-align: center;
}

.cp-auth-logo {
  display: inline-flex;
  width: 64px;
  height: 64px;
  margin-bottom: 16px;
  overflow: hidden;
  border-radius: 18px;
  box-shadow: 0 14px 32px -14px color-mix(in srgb, var(--brand) 60%, transparent);
}

.cp-auth-brand {
  margin: 0 0 8px;
  font-size: 30px;
  font-weight: 750;
  letter-spacing: -0.03em;
  background: linear-gradient(100deg, var(--brand-soft), var(--brand), var(--brand-2));
  background-clip: text;
  -webkit-background-clip: text;
  color: transparent;
  background-size: 220% 100%;
  animation: cp-auth-gradient 11s ease-in-out infinite alternate;
}

.cp-auth-subtitle {
  margin: 0;
  color: var(--muted);
  font-size: 14px;
}

.cp-auth-card {
  padding: 32px;
  border: 1px solid var(--border);
  border-radius: 20px;
  background: color-mix(in srgb, var(--surface) 92%, transparent);
  box-shadow: var(--shadow-float), inset 0 1px 0 color-mix(in srgb, #fff 6%, transparent);
  backdrop-filter: blur(16px);
  animation: cp-auth-rise 0.72s cubic-bezier(0.22, 1, 0.36, 1) 0.1s both;
}

.cp-auth-trust {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  margin: 16px 0 0;
  color: var(--muted);
  font-size: 12px;
  text-align: center;
}

.cp-auth-trust svg {
  flex-shrink: 0;
  color: var(--accent-ok);
}

.cp-auth-footer {
  margin-top: 20px;
  font-size: 14px;
  text-align: center;
}

.cp-auth-copyright {
  margin-top: 28px;
  color: var(--muted);
  opacity: 0.75;
  font-size: 12px;
  text-align: center;
}

/* ---------- 动画 ---------- */
@keyframes cp-auth-rise {
  from { opacity: 0; transform: translateY(22px); }
  to { opacity: 1; transform: translateY(0); }
}

@keyframes cp-auth-gradient {
  to { background-position: 100% 50%; }
}

/* ---------- 响应式 ---------- */
@media (min-width: 1024px) {
  .cp-auth-aside {
    display: block;
  }

  .cp-auth-layout {
    justify-content: space-between;
  }
}

@media (max-width: 520px) {
  .cp-auth-shell {
    align-items: flex-start;
    /* 原 76px，手机上要滚才能看到表单 */
    padding: 64px 16px 40px;
    overflow-y: auto;
  }

  .cp-auth-tools {
    right: 12px;
  }

  .cp-auth-brand-block {
    margin-bottom: 20px;
  }

  .cp-auth-logo {
    width: 52px;
    height: 52px;
  }

  .cp-auth-brand {
    font-size: 25px;
  }

  .cp-auth-card {
    padding: 24px 20px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .cp-auth-glow,
  .cp-auth-card,
  .cp-auth-brand,
  .cp-auth-theme {
    animation: none;
    transition: none;
  }
}
</style>
