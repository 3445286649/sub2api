<template>
  <!-- Custom Home Content: Full Page Mode -->
  <div v-if="hasHomeContent" class="min-h-screen">
    <!-- iframe mode -->
    <iframe
      v-if="isHomeContentUrl"
      :src="homeContent.trim()"
      class="h-screen w-full border-0"
      allowfullscreen
    ></iframe>
    <!-- HTML mode - SECURITY: homeContent is admin-only setting, XSS risk is acceptable -->
    <div v-else v-html="homeContent"></div>
  </div>

  <!-- Compact Home Page：保持极简定位，只统一配色并让页面能自证用途 -->
  <div
    v-else-if="compactHomeEnabled"
    data-testid="compact-home"
    class="cp-compact flex min-h-screen flex-col"
  >
    <header class="cp-compact-header px-4 py-4 sm:px-6">
      <nav class="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-3 sm:gap-4">
        <div class="flex min-w-0 flex-1 items-center gap-3">
          <img
            :src="siteLogo || '/logo.svg'"
            alt="Logo"
            class="h-9 w-9 shrink-0 rounded-lg object-contain"
          />
          <span class="min-w-0 truncate text-base font-semibold">{{ siteName }}</span>
        </div>
        <div class="flex max-w-full shrink-0 flex-wrap items-center justify-end gap-2">
          <LocaleSwitcher />
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="cp-compact-icon-btn"
            :title="t('home.viewDocs')"
          >
            <Icon name="book" size="md" />
          </a>
          <router-link
            v-if="showModelPlazaEntry"
            to="/model-plaza"
            class="cp-compact-icon-btn cp-compact-plaza"
            :title="t('nav.modelPlaza')"
          >
            <Icon name="grid" size="md" />
            <span class="hidden sm:inline">{{ t('nav.modelPlaza') }}</span>
          </router-link>
          <button
            class="cp-compact-icon-btn"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
            @click="toggleTheme"
          >
            <Icon v-if="isDark" name="sun" size="md" />
            <Icon v-else name="moon" size="md" />
          </button>
          <router-link
            :to="isAuthenticated ? dashboardPath : '/login'"
            class="cp-compact-login"
          >
            {{ isAuthenticated ? t('home.dashboard') : t('home.login') }}
          </router-link>
        </div>
      </nav>
    </header>

    <main class="flex min-w-0 flex-1 items-center justify-center px-4 py-16 sm:px-6">
      <div class="cp-compact-body">
        <div class="cp-compact-intro">
          <img
            :src="siteLogo || '/logo.svg'"
            alt="Logo"
            class="mx-auto mb-6 h-20 w-20 rounded-2xl object-contain"
          />
          <h1 class="cp-compact-title [overflow-wrap:anywhere]">{{ siteName }}</h1>
          <p class="cp-compact-sub whitespace-pre-wrap [overflow-wrap:anywhere]">{{ siteSubtitle }}</p>
          <router-link
            :to="isAuthenticated ? dashboardPath : '/login'"
            class="cp-compact-cta"
          >
            {{ isAuthenticated ? t('home.goToDashboard') : t('home.login') }}
            <Icon name="arrowRight" size="sm" />
          </router-link>
          <p class="cp-compact-trust">{{ t('home.compact.trust') }}</p>
        </div>

        <!-- 让极简页也能一眼看出「这是干什么的」 -->
        <RequestJourney compact class="cp-compact-journey" />
      </div>
    </main>

    <footer class="cp-compact-footer px-4 py-5 sm:px-6">
      <span>&copy; {{ currentYear }} {{ siteName }}</span>
      <span class="cp-compact-footer-links">
        <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer">
          {{ t('home.docs') }}
        </a>
        <a :href="githubUrl" target="_blank" rel="noopener noreferrer">GitHub</a>
      </span>
    </footer>
  </div>

  <!-- Default Home Page -->
  <div
    v-else
    data-testid="default-home"
    class="cp-home-default relative flex min-h-screen flex-col overflow-hidden"
  >
    <!-- Header -->
    <header class="relative z-20 px-6 py-4">
      <nav class="mx-auto flex max-w-6xl items-center justify-between">
        <!-- Logo -->
        <div class="flex items-center gap-3">
          <div class="h-10 w-10 overflow-hidden rounded-xl shadow-md">
            <img :src="siteLogo || '/logo.svg'" alt="Logo" class="h-full w-full object-contain" />
          </div>
          <span class="cp-brand-name">{{ siteName }}</span>
        </div>

        <!-- Nav Actions -->
        <div class="flex items-center gap-3">
          <!-- Language Switcher -->
          <LocaleSwitcher />

          <!-- Doc Link -->
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="rounded-lg p-2 text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white"
            :title="t('home.viewDocs')"
          >
            <Icon name="book" size="md" />
          </a>

          <!-- Model Plaza Link -->
          <router-link
            v-if="showModelPlazaEntry"
            to="/model-plaza"
            class="inline-flex items-center gap-1.5 rounded-lg p-2 text-sm text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white"
            :title="t('nav.modelPlaza')"
          >
            <Icon name="grid" size="md" />
            <span class="hidden sm:inline">{{ t('nav.modelPlaza') }}</span>
          </router-link>

          <!-- Theme Toggle -->
          <button
            @click="toggleTheme"
            class="rounded-lg p-2 text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
          >
            <Icon v-if="isDark" name="sun" size="md" />
            <Icon v-else name="moon" size="md" />
          </button>

          <!-- Login / Dashboard Button -->
          <router-link
            v-if="isAuthenticated"
            :to="dashboardPath"
            class="inline-flex items-center gap-1.5 rounded-full bg-gray-900 py-1 pl-1 pr-2.5 transition-colors hover:bg-gray-800 dark:bg-gray-800 dark:hover:bg-gray-700"
          >
            <span
              class="flex h-5 w-5 items-center justify-center rounded-full bg-gradient-to-br from-primary-400 to-primary-600 text-[10px] font-semibold text-white"
            >
              {{ userInitial }}
            </span>
            <span class="text-xs font-medium text-white">{{ t('home.dashboard') }}</span>
            <svg
              class="h-3 w-3 text-gray-400"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              stroke-width="2"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                d="M4.5 19.5l15-15m0 0H8.25m11.25 0v11.25"
              />
            </svg>
          </router-link>
          <router-link
            v-else
            to="/login"
            class="inline-flex items-center rounded-full bg-gray-900 px-3 py-1 text-xs font-medium text-white transition-colors hover:bg-gray-800 dark:bg-gray-800 dark:hover:bg-gray-700"
          >
            {{ t('home.login') }}
          </router-link>
        </div>
      </nav>
    </header>

    <!-- Main Content -->
    <main class="relative z-10 flex-1 px-6 py-16">
      <div class="mx-auto max-w-6xl">
        <!-- ========== 1. Hero ========== -->
        <section class="cp-section cp-hero">
          <div class="cp-hero-grid">
            <!-- Left: Text Content -->
            <div class="cp-hero-left">
              <div class="cp-eyebrow"><span class="cp-pulse"></span>{{ siteSubtitle }}</div>
              <h1 class="cp-hero-title">
                {{ siteName }}<br /><span>{{ t('home.heroSubtitle') }}</span>
              </h1>
              <p class="cp-hero-sub">
                {{ t('home.heroDescription') }}
              </p>

              <!-- CTA Button -->
              <div class="cp-hero-cta">
                <router-link
                  :to="isAuthenticated ? dashboardPath : '/login'"
                  class="cp-primary-btn btn btn-primary"
                >
                  {{ isAuthenticated ? t('home.goToDashboard') : t('home.getStarted') }}
                  <Icon name="arrowRight" size="md" class="ml-2" :stroke-width="2" />
                </router-link>
                <a
                  v-if="docUrl"
                  :href="docUrl"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="cp-secondary-btn"
                >
                  {{ t('home.viewDocs') }} <Icon name="arrowRight" size="sm" />
                </a>
              </div>

              <div class="cp-proof">
                <span><Icon name="check" size="xs" />{{ t('home.proof.sdk') }}</span>
                <span><Icon name="check" size="xs" />{{ t('home.proof.setup') }}</span>
                <span><Icon name="check" size="xs" />{{ t('home.proof.billing') }}</span>
              </div>
            </div>

            <!-- Right: Request Journey Animation -->
            <div class="cp-hero-right">
              <RequestJourney class="w-full max-w-[520px]" />
            </div>
          </div>
        </section>

        <!-- ========== 2. Pain Points ========== -->
        <section class="cp-section cp-pain">
          <header class="cp-section-head">
            <h2 class="cp-section-title">{{ t('home.painPoints.title') }}</h2>
          </header>
          <div class="cp-pain-grid">
            <article
              v-for="(item, index) in painPoints"
              :key="item.key"
              class="cp-card cp-pain-card"
              :style="{ '--cp-stagger': `${index * 0.08}s` }"
            >
              <span class="cp-pain-icon"><Icon :name="item.icon" size="md" /></span>
              <h3 class="cp-card-title">{{ item.title }}</h3>
              <p class="cp-card-desc">{{ item.desc }}</p>
            </article>
          </div>
        </section>

        <!-- ========== 3. Architecture Flow ========== -->
        <section class="cp-section">
          <ArchitectureFlow />
        </section>

        <!-- ========== 4. Capabilities ========== -->
        <section class="cp-section cp-capabilities">
          <header class="cp-section-head">
            <h2 class="cp-section-title">{{ t('home.solutions.title') }}</h2>
            <p class="cp-section-sub">{{ t('home.solutions.subtitle') }}</p>
          </header>
          <div class="cp-feature-grid">
            <article
              v-for="(feature, index) in capabilities"
              :key="feature.key"
              class="cp-card cp-feature-card"
              :style="{ '--cp-stagger': `${index * 0.06}s` }"
            >
              <span class="cp-feature-icon"><Icon :name="feature.icon" size="lg" /></span>
              <h3 class="cp-card-title">{{ feature.title }}</h3>
              <p class="cp-card-desc">{{ feature.desc }}</p>
            </article>
          </div>
        </section>

        <!-- ========== 5. Comparison ========== -->
        <section class="cp-section cp-comparison">
          <header class="cp-section-head">
            <h2 class="cp-section-title">{{ t('home.comparison.title') }}</h2>
          </header>
          <div class="cp-compare-wrap">
            <table class="cp-compare">
              <thead>
                <tr>
                  <th scope="col">{{ t('home.comparison.headers.feature') }}</th>
                  <th scope="col">{{ t('home.comparison.headers.official') }}</th>
                  <th scope="col" class="cp-compare-us">
                    {{ t('home.comparison.headers.us') }}
                  </th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="row in comparisonRows" :key="row.key">
                  <th scope="row">{{ row.feature }}</th>
                  <td class="cp-compare-official">{{ row.official }}</td>
                  <td class="cp-compare-us">{{ row.us }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>

        <!-- ========== 6. Providers ========== -->
        <section class="cp-section cp-providers">
          <header class="cp-section-head">
            <h2 class="cp-section-title">{{ t('home.providers.title') }}</h2>
            <p class="cp-section-sub">{{ t('home.providers.description') }}</p>
          </header>

          <!-- 走马灯：内层复制一份实现无缝循环，hover 暂停 -->
          <div class="cp-marquee">
            <div class="cp-marquee-track">
              <template v-for="pass in 2" :key="pass">
                <div
                  v-for="provider in providers"
                  :key="`${pass}-${provider.key}`"
                  class="cp-provider"
                  :class="{ 'is-soon': !provider.supported }"
                  :aria-hidden="pass === 2 ? 'true' : undefined"
                >
                  <span class="cp-provider-logo">
                    <ModelIcon v-if="isModelIcon(provider.icon)" :model="modelIconHint(provider.icon)" size="20px" />
                    <PlatformIcon
                      v-else-if="isPlatformIcon(provider.icon)"
                      :platform="provider.icon"
                      size="md"
                    />
                    <span v-else class="cp-provider-fallback">{{ provider.initial }}</span>
                  </span>
                  <span class="cp-provider-name">{{ provider.name }}</span>
                  <span
                    class="cp-provider-tag"
                    :class="provider.supported ? 'is-supported' : 'is-soon'"
                  >
                    {{ provider.supported ? t('home.providers.supported') : t('home.providers.soon') }}
                  </span>
                </div>
              </template>
            </div>
          </div>
        </section>

        <!-- ========== 7. Final CTA ========== -->
        <section class="cp-section cp-cta">
          <h2 class="cp-cta-title">{{ t('home.cta.title') }}</h2>
          <p class="cp-cta-desc">{{ t('home.cta.description') }}</p>
          <router-link
            :to="isAuthenticated ? dashboardPath : '/login'"
            class="cp-primary-btn btn btn-primary cp-cta-btn"
          >
            {{ isAuthenticated ? t('home.goToDashboard') : t('home.cta.button') }}
            <Icon name="arrowRight" size="md" class="ml-2" :stroke-width="2" />
          </router-link>
        </section>
      </div>
    </main>


    <!-- Footer -->
    <footer class="relative z-10 border-t border-gray-200/50 px-6 py-8 dark:border-dark-800/50">
      <div
        class="mx-auto flex max-w-6xl flex-col items-center justify-center gap-4 text-center sm:flex-row sm:text-left"
      >
        <p class="text-sm text-gray-500 dark:text-dark-400">
          &copy; {{ currentYear }} {{ siteName }}. {{ t('home.footer.allRightsReserved') }}
        </p>
        <div class="flex items-center gap-4">
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="text-sm text-gray-500 transition-colors hover:text-gray-700 dark:text-dark-400 dark:hover:text-white"
          >
            {{ t('home.docs') }}
          </a>
          <a
            :href="githubUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="text-sm text-gray-500 transition-colors hover:text-gray-700 dark:text-dark-400 dark:hover:text-white"
          >
            GitHub
          </a>
        </div>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useAppStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import RequestJourney from '@/components/landing/RequestJourney.vue'
import ArchitectureFlow from '@/components/landing/ArchitectureFlow.vue'
import ModelIcon from '@/components/common/ModelIcon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import { sanitizeUrl } from '@/utils/url'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'
import { useTheme } from '@/composables/useTheme'

const { t } = useI18n()
const { isDark, toggleTheme } = useTheme()

const authStore = useAuthStore()
const appStore = useAppStore()

// Site settings - directly from appStore (already initialized from injected config)
const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'Sub2API')
const siteLogo = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || 'AI API Gateway Platform')
const docUrl = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl || ''))
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')
const hasHomeContent = computed(() => homeContent.value.trim().length > 0)
const compactHomeEnabled = computed(() => appStore.cachedPublicSettings?.compact_home_enabled === true)
const modelPlazaEnabled = computed(() => isFeatureFlagEnabled(FeatureFlags.modelPlaza))

// Check if homeContent is a URL (for iframe display)
const isHomeContentUrl = computed(() => {
  const content = homeContent.value.trim()
  return content.startsWith('http://') || content.startsWith('https://')
})

// GitHub URL
const githubUrl = 'https://github.com/Wei-Shaw/sub2api'

// Auth state
const isAuthenticated = computed(() => authStore.isAuthenticated)
const modelPlazaRequiresAuth = computed(
  () => appStore.cachedPublicSettings?.model_plaza_require_auth === true,
)
const showModelPlazaEntry = computed(
  () => modelPlazaEnabled.value && (isAuthenticated.value || !modelPlazaRequiresAuth.value),
)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => isAdmin.value ? '/admin/dashboard' : '/dashboard')
const userInitial = computed(() => {
  const user = authStore.user
  if (!user || !user.email) return ''
  return user.email.charAt(0).toUpperCase()
})

// Current year for footer
const currentYear = computed(() => new Date().getFullYear())

// ==================== Landing 区块数据 ====================
// 全部用静态 t() 字面量：i18n 检查脚本只静态扫描字面量键，
// 动态拼接的键不会被校验，缺键时也只会在运行时暴露原始键名。

/** Icon 组件的 name 收窄类型。沿用 UsageHelpModal 的既有写法。 */
type IconName = InstanceType<typeof Icon>['$props']['name']

/** 2. 用户痛点 */
const painPoints = computed<Array<{ key: string; icon: IconName; title: string; desc: string }>>(() => [
  {
    key: 'expensive',
    icon: 'dollar',
    title: t('home.painPoints.items.expensive.title'),
    desc: t('home.painPoints.items.expensive.desc'),
  },
  {
    key: 'complex',
    icon: 'users',
    title: t('home.painPoints.items.complex.title'),
    desc: t('home.painPoints.items.complex.desc'),
  },
  {
    key: 'unstable',
    icon: 'bolt',
    title: t('home.painPoints.items.unstable.title'),
    desc: t('home.painPoints.items.unstable.desc'),
  },
  {
    key: 'noControl',
    icon: 'chart',
    title: t('home.painPoints.items.noControl.title'),
    desc: t('home.painPoints.items.noControl.desc'),
  },
])

/** 4. 能力网格：从「管理员视角」改为「用户收益视角」 */
const capabilities = computed<Array<{ key: string; icon: IconName; title: string; desc: string }>>(() => [
  {
    key: 'auth',
    icon: 'shield',
    title: t('home.features.auth'),
    desc: t('home.features.authDesc'),
  },
  {
    key: 'route',
    icon: 'server',
    title: t('home.features.route'),
    desc: t('home.features.routeDesc'),
  },
  {
    key: 'sticky',
    icon: 'link',
    title: t('home.features.sticky'),
    desc: t('home.features.stickyDesc'),
  },
  {
    key: 'rate',
    icon: 'bolt',
    title: t('home.features.rate'),
    desc: t('home.features.rateDesc'),
  },
  {
    key: 'cost',
    icon: 'calculator',
    title: t('home.features.cost'),
    desc: t('home.features.costDesc'),
  },
  {
    key: 'compat',
    icon: 'terminal',
    title: t('home.features.compat'),
    desc: t('home.features.compatDesc'),
  },
])

/** 5. 对比表 */
const comparisonRows = computed(() => [
  {
    key: 'pricing',
    feature: t('home.comparison.items.pricing.feature'),
    official: t('home.comparison.items.pricing.official'),
    us: t('home.comparison.items.pricing.us'),
  },
  {
    key: 'models',
    feature: t('home.comparison.items.models.feature'),
    official: t('home.comparison.items.models.official'),
    us: t('home.comparison.items.models.us'),
  },
  {
    key: 'management',
    feature: t('home.comparison.items.management.feature'),
    official: t('home.comparison.items.management.official'),
    us: t('home.comparison.items.management.us'),
  },
  {
    key: 'stability',
    feature: t('home.comparison.items.stability.feature'),
    official: t('home.comparison.items.stability.official'),
    us: t('home.comparison.items.stability.us'),
  },
  {
    key: 'control',
    feature: t('home.comparison.items.control.feature'),
    official: t('home.comparison.items.control.official'),
    us: t('home.comparison.items.control.us'),
  },
])

/**
 * 6. 供应商。
 * 图标用 ModelIcon / PlatformIcon 内嵌的 lobehub SVG path
 * ——@lobehub/icons 本体是 React 组件库，Vue 项目无法直接引用。
 * Antigravity 在 lobehub 无图标，由 PlatformIcon 提供。
 */
/** ModelIcon 支持的 key（lobehub 提取的 SVG） */
const MODEL_ICON_KEYS = new Set(['claude', 'openai', 'gemini', 'qwen', 'deepseek'])

function isModelIcon(key: string): boolean {
  return MODEL_ICON_KEYS.has(key)
}

/**
 * ModelIcon 的入参是「模型名」而非图标键——它靠 startsWith('gpt') /
 * includes('claude') 之类的规则识别厂商。直接传 'openai' 匹配不到任何规则，
 * 会落到它内部的首字母回退。这里给每个图标键一个能被识别的代表性模型名。
 */
const MODEL_ICON_HINT: Record<string, string> = {
  claude: 'claude',
  openai: 'gpt',
  gemini: 'gemini',
  qwen: 'qwen',
  deepseek: 'deepseek',
}

function modelIconHint(key: string): string {
  return MODEL_ICON_HINT[key] ?? key
}

/**
 * PlatformIcon 只承载 antigravity —— lobehub 没有它的图标，
 * 由 PlatformIcon 内嵌的云朵 mark 提供。用类型谓词让模板拿到正确的
 * GroupPlatform 类型，而不是宽泛的 string。
 */
function isPlatformIcon(key: string): key is 'antigravity' {
  return key === 'antigravity'
}

const providers = computed<
  Array<{
    key: string
    name: string
    icon: 'claude' | 'openai' | 'gemini' | 'antigravity' | 'qwen' | 'deepseek' | ''
    initial: string
    supported: boolean
  }>
>(() => [
  {
    key: 'claude',
    name: t('home.providers.claude'),
    icon: 'claude',
    initial: 'C',
    supported: true,
  },
  {
    key: 'gpt',
    name: 'GPT',
    icon: 'openai',
    initial: 'G',
    supported: true,
  },
  {
    key: 'gemini',
    name: t('home.providers.gemini'),
    icon: 'gemini',
    initial: 'G',
    supported: true,
  },
  {
    key: 'antigravity',
    name: t('home.providers.antigravity'),
    icon: 'antigravity',
    initial: 'A',
    supported: true,
  },
  {
    key: 'qwen',
    name: 'Qwen',
    icon: 'qwen',
    initial: 'Q',
    supported: true,
  },
  {
    key: 'deepseek',
    name: 'DeepSeek',
    icon: 'deepseek',
    initial: 'D',
    supported: true,
  },
  {
    key: 'more',
    name: t('home.providers.more'),
    icon: '',
    initial: '+',
    supported: false,
  },
])

onMounted(() => {
  // Check auth state
  authStore.checkAuth()

  // Ensure public settings are loaded (will use cache if already loaded from injected config)
  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings()
  }
})
</script>

<style scoped>
/* ============================================================
 * 首页皮肤
 *
 * 配色统一走全局语义 token（src/style.css 的 :root / .dark），
 * 不再在本文件里写 :global(.dark) 覆盖——那是「割裂」的根源。
 *
 * 选择器一律使用语义类名（.cp-section / .cp-card / ...），
 * 不再用 .mb-12.grid 这类布局工具类当选择器：模板动一个 div 皮肤就失效。
 * ============================================================ */

.cp-home-default {
  --cp-bg: var(--surface-2);
  --cp-surface: var(--surface);
  --cp-text: var(--text);
  --cp-muted: var(--muted);
  --cp-border: var(--border);
  --cp-accent: var(--brand);
  --cp-accent-soft: var(--brand-soft);
  --cp-cyan: var(--brand);
  background:
    radial-gradient(42% 38% at 18% 14%, var(--glow-brand), transparent 62%),
    radial-gradient(38% 34% at 86% 10%, var(--glow-brand-2), transparent 62%),
    var(--cp-bg);
  color: var(--cp-text);
  font-family: Inter, 'PingFang SC', 'Microsoft YaHei', system-ui, sans-serif;
  isolation: isolate;
}

/* 背景网格：靠 mask 做中心聚焦，边缘自然淡出 */
.cp-home-default::before {
  content: '';
  position: absolute;
  inset: 0;
  z-index: -1;
  pointer-events: none;
  opacity: 0.8;
  background-image:
    linear-gradient(color-mix(in srgb, var(--brand) 4%, transparent) 1px, transparent 1px),
    linear-gradient(90deg, color-mix(in srgb, var(--brand) 4%, transparent) 1px, transparent 1px);
  background-size: 56px 56px;
  mask-image: radial-gradient(ellipse 92% 70% at 50% 50%, #000 8%, transparent 72%);
}

/* ---------- 顶栏 ---------- */
.cp-home-default header {
  width: min(100%, 1180px);
  margin: 0 auto;
  padding: 18px 28px;
}

.cp-home-default header nav {
  max-width: none;
}

.cp-brand-name {
  color: var(--cp-text);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 15px;
  font-weight: 700;
  letter-spacing: 0.02em;
}

/* ---------- 主体 ---------- */
.cp-home-default main {
  width: min(100%, 1180px);
  margin: 0 auto;
  padding-top: clamp(40px, 7vh, 80px);
}

.cp-home-default main > div {
  max-width: none;
}

.cp-section {
  margin-bottom: 72px;
}

.cp-section-head {
  margin-bottom: 28px;
  text-align: center;
}

.cp-section-title {
  margin: 0;
  color: var(--cp-text);
  font-size: 26px;
  font-weight: 750;
  letter-spacing: -0.02em;
}

.cp-section-sub {
  margin: 8px 0 0;
  color: var(--cp-muted);
  font-size: 14px;
}

/* ---------- 1. Hero ---------- */
.cp-hero-grid {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 44px;
}

.cp-hero-left {
  flex: 1;
  text-align: center;
}

.cp-eyebrow {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 17px;
  color: var(--cp-cyan);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.cp-pulse {
  width: 7px;
  height: 7px;
  border-radius: 999px;
  background: var(--brand);
  box-shadow: 0 0 0 5px color-mix(in srgb, var(--brand) 16%, transparent);
  animation: cp-pulse 1.8s ease-in-out infinite;
}

.cp-hero-title {
  margin: 0 0 16px;
  color: var(--cp-text);
  font-size: clamp(32px, 6vw, 56px);
  font-weight: 750;
  letter-spacing: -0.045em;
  line-height: 1.08;
}

.cp-hero-title span {
  background: linear-gradient(100deg, var(--cp-accent-soft), var(--cp-accent), var(--brand-2));
  background-clip: text;
  -webkit-background-clip: text;
  color: transparent;
}

.cp-hero-sub {
  max-width: 560px;
  margin: 0 auto;
  color: var(--cp-muted);
  font-size: 16px;
  line-height: 1.75;
}

.cp-hero-cta {
  margin-top: 32px;
}

.cp-hero-right {
  flex: 1;
  display: flex;
  justify-content: center;
  width: 100%;
}

.cp-primary-btn,
.cp-secondary-btn {
  min-height: 44px;
  border-radius: 11px;
  transition: transform 0.22s ease, box-shadow 0.28s ease, border-color 0.22s ease;
}

.cp-primary-btn {
  padding: 12px 30px;
  font-size: 15px;
  box-shadow: 0 12px 28px -12px color-mix(in srgb, var(--brand) 55%, transparent);
}

.cp-primary-btn:hover {
  transform: translateY(-2px);
}

.cp-secondary-btn {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  margin-left: 10px;
  padding: 11px 18px;
  border: 1px solid var(--cp-border);
  border-radius: 11px;
  color: var(--cp-text);
  background: var(--cp-surface);
  font-size: 14px;
  font-weight: 650;
  backdrop-filter: blur(12px);
}

.cp-secondary-btn:hover {
  border-color: color-mix(in srgb, var(--brand) 42%, var(--cp-border));
  transform: translateY(-2px);
}

.cp-proof {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 16px;
  margin-top: 24px;
  color: var(--cp-muted);
  font-size: 12px;
}

.cp-proof span {
  display: inline-flex;
  align-items: center;
  gap: 5px;
}

.cp-proof svg {
  color: var(--accent-ok);
}

/* ---------- 通用卡片 ---------- */
.cp-card {
  border: 1px solid var(--cp-border);
  border-radius: 16px;
  background: var(--cp-surface);
  box-shadow: var(--shadow-card);
  transition: border-color 0.28s ease, transform 0.28s ease, box-shadow 0.28s ease;
  /* 交错入场：由父级传入 --cp-stagger */
  animation: cp-rise 0.5s cubic-bezier(0.22, 1, 0.36, 1) both;
  animation-delay: var(--cp-stagger, 0s);
}

.cp-card:hover {
  border-color: color-mix(in srgb, var(--brand) 38%, var(--cp-border));
  transform: translateY(-3px);
  box-shadow: var(--shadow-float);
}

.cp-card-title {
  margin: 0 0 8px;
  color: var(--cp-text);
  font-size: 15.5px;
  font-weight: 700;
}

.cp-card-desc {
  margin: 0;
  color: var(--cp-muted);
  font-size: 13px;
  line-height: 1.7;
}

/* ---------- 2. 痛点 ---------- */
/* 4 张卡：宽屏 4 列、中屏 2 列，不用 auto-fit 以免排成 3+1 */
.cp-pain-grid {
  display: grid;
  gap: 14px;
  grid-template-columns: repeat(4, minmax(0, 1fr));
}

@media (max-width: 1023px) {
  .cp-pain-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

.cp-pain-card {
  padding: 20px;
}

.cp-pain-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 38px;
  height: 38px;
  margin-bottom: 14px;
  border-radius: 11px;
  background: color-mix(in srgb, var(--accent-warn) 14%, transparent);
  color: var(--accent-warn);
}

/* ---------- 4. 能力网格 ---------- */
/* 6 张卡：桌面固定 3 列（两行排满），避免 auto-fit 在宽屏排成 4+2 留出空洞 */
.cp-feature-grid {
  display: grid;
  gap: 14px;
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

@media (max-width: 1023px) {
  .cp-feature-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

.cp-feature-card {
  padding: 22px;
}

.cp-feature-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 44px;
  height: 44px;
  margin-bottom: 16px;
  border-radius: 13px;
  background: color-mix(in srgb, var(--brand) 13%, transparent);
  color: var(--brand);
  transition: transform 0.28s ease;
}

.cp-feature-card:hover .cp-feature-icon {
  transform: scale(1.08);
}

/* ---------- 5. 对比表 ---------- */
.cp-compare-wrap {
  overflow-x: auto;
  border: 1px solid var(--cp-border);
  border-radius: 16px;
  background: var(--cp-surface);
  box-shadow: var(--shadow-card);
}

.cp-compare {
  width: 100%;
  min-width: 520px;
  border-collapse: collapse;
  font-size: 13.5px;
}

.cp-compare th,
.cp-compare td {
  padding: 14px 18px;
  text-align: left;
  border-bottom: 1px solid var(--cp-border);
}

.cp-compare thead th {
  color: var(--cp-muted);
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  white-space: nowrap;
}

.cp-compare tbody th {
  color: var(--cp-text);
  font-weight: 650;
  white-space: nowrap;
}

.cp-compare tbody tr:last-child th,
.cp-compare tbody tr:last-child td {
  border-bottom: none;
}

.cp-compare-official {
  color: var(--cp-muted);
}

/* 本平台列高亮：整列淡色底 + 品牌色文字 */
.cp-compare-us {
  color: var(--brand);
  font-weight: 650;
  background: color-mix(in srgb, var(--brand) 7%, transparent);
}

/* 表头那格要略强一点，避免和表体糊在一起 */
.cp-compare thead .cp-compare-us {
  color: var(--brand);
  background: color-mix(in srgb, var(--brand) 11%, transparent);
}

/* ---------- 6. 供应商走马灯 ---------- */
.cp-marquee {
  position: relative;
  overflow: hidden;
  /* 两端渐隐，让循环看不出接缝 */
  mask-image: linear-gradient(90deg, transparent, #000 8%, #000 92%, transparent);
}

.cp-marquee-track {
  display: flex;
  gap: 14px;
  width: max-content;
  animation: cp-marquee 32s linear infinite;
}

.cp-marquee:hover .cp-marquee-track {
  animation-play-state: paused;
}

.cp-provider {
  display: flex;
  align-items: center;
  gap: 10px;
  flex: 0 0 auto;
  padding: 12px 18px;
  border: 1px solid var(--cp-border);
  border-radius: 13px;
  background: var(--cp-surface);
  transition: border-color 0.25s ease;
}

.cp-provider:hover {
  border-color: color-mix(in srgb, var(--brand) 34%, var(--cp-border));
}

.cp-provider.is-soon {
  opacity: 0.55;
}

.cp-provider-logo {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  /* 单色化：统一跟随文本色，彻底消除原先四种渐变色的冲突 */
  color: var(--cp-muted);
  transition: color 0.25s ease;
}

.cp-provider:hover .cp-provider-logo {
  color: var(--brand);
}

/*
 * ModelIcon 在每个 <path> 上写死了品牌色 fill（GPT 为纯黑 #000，深色模式下不可见）。
 * 首页要求单色统一，这里在作用域内覆盖为 currentColor，不改共享组件。
 */
.cp-provider-logo :deep(svg path) {
  fill: currentColor;
}

.cp-provider-fallback {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  border-radius: 7px;
  background: var(--surface-3);
  color: var(--cp-muted);
  font-size: 12px;
  font-weight: 700;
}

.cp-provider-name {
  color: var(--cp-text);
  font-size: 14px;
  font-weight: 600;
  white-space: nowrap;
}

.cp-provider-tag {
  padding: 2px 7px;
  border-radius: 6px;
  font-size: 10.5px;
  font-weight: 700;
  white-space: nowrap;
}

.cp-provider-tag.is-supported {
  background: color-mix(in srgb, var(--accent-ok) 14%, transparent);
  color: var(--accent-ok);
}

.cp-provider-tag.is-soon {
  background: var(--surface-3);
  color: var(--cp-muted);
}

/* ---------- 7. 收尾 CTA ---------- */
.cp-cta {
  padding: 46px 28px;
  border: 1px solid var(--cp-border);
  border-radius: 22px;
  background: var(--cp-surface);
  box-shadow: var(--shadow-card);
  text-align: center;
  /* 渐变描边缓慢流动 */
  background-image: linear-gradient(
    130deg,
    color-mix(in srgb, var(--brand) 7%, var(--cp-surface)),
    var(--cp-surface) 45%,
    color-mix(in srgb, var(--brand-2) 7%, var(--cp-surface))
  );
}

.cp-cta-title {
  margin: 0 0 10px;
  color: var(--cp-text);
  font-size: 24px;
  font-weight: 750;
  letter-spacing: -0.02em;
}

.cp-cta-desc {
  margin: 0 0 26px;
  color: var(--cp-muted);
  font-size: 14px;
}

.cp-cta-btn {
  padding: 12px 30px;
  font-size: 15px;
}

/* ---------- 页脚 ---------- */
.cp-home-default footer {
  border-color: var(--cp-border);
}

/* ============================================================
 * 紧凑首页（compact_home_enabled）
 * 与全量首页共用同一套 token，只做极简布局，不引入额外区块。
 * ============================================================ */
.cp-compact {
  background:
    radial-gradient(50% 40% at 50% 0%, var(--glow-brand), transparent 70%),
    var(--surface-2);
  color: var(--text);
}

.cp-compact-header {
  border-bottom: 1px solid var(--border);
}

.cp-compact-icon-btn {
  display: inline-flex;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  gap: 6px;
  min-width: 40px;
  height: 40px;
  padding: 0 10px;
  border-radius: 10px;
  color: var(--muted);
  font-size: 14px;
  font-weight: 500;
  transition: background 0.2s ease, color 0.2s ease;
}

.cp-compact-icon-btn:hover {
  background: var(--surface-3);
  color: var(--text);
}

.cp-compact-login {
  display: inline-flex;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  min-height: 40px;
  padding: 8px 16px;
  border-radius: 10px;
  background: var(--brand);
  color: #fff;
  font-size: 14px;
  font-weight: 600;
  transition: filter 0.2s ease;
}

:global(.dark) .cp-compact-login {
  color: #042f2e;
}

.cp-compact-login:hover {
  filter: brightness(1.08);
}

.cp-compact-body {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 40px;
  width: 100%;
  max-width: 1040px;
  min-width: 0;
}

.cp-compact-intro {
  max-width: 560px;
  min-width: 0;
  text-align: center;
}

.cp-compact-title {
  margin: 0;
  font-size: clamp(28px, 5vw, 38px);
  font-weight: 750;
  letter-spacing: -0.03em;
}

.cp-compact-sub {
  margin: 16px 0 0;
  color: var(--muted);
  font-size: 15px;
  line-height: 1.7;
}

.cp-compact-cta {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-height: 44px;
  margin-top: 30px;
  padding: 11px 24px;
  border-radius: 11px;
  background: var(--brand);
  color: #fff;
  font-size: 15px;
  font-weight: 650;
  box-shadow: 0 12px 28px -12px color-mix(in srgb, var(--brand) 55%, transparent);
  transition: transform 0.22s ease;
}

:global(.dark) .cp-compact-cta {
  color: #042f2e;
}

.cp-compact-cta:hover {
  transform: translateY(-2px);
}

.cp-compact-trust {
  margin: 16px 0 0;
  color: var(--muted);
  font-size: 12px;
}

.cp-compact-journey {
  width: 100%;
  max-width: 460px;
}

.cp-compact-footer {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: center;
  gap: 8px 20px;
  min-width: 0;
  border-top: 1px solid var(--border);
  color: var(--muted);
  font-size: 13px;
  overflow-wrap: anywhere;
}

.cp-compact-footer-links {
  display: inline-flex;
  gap: 16px;
}

.cp-compact-footer-links a:hover {
  color: var(--text);
}

@media (min-width: 1024px) {
  .cp-compact-body {
    flex-direction: row;
    justify-content: space-between;
    gap: 56px;
  }

  .cp-compact-intro {
    text-align: left;
  }

  .cp-compact-intro img {
    margin-left: 0;
  }
}

/* ---------- 动画 ---------- */
@keyframes cp-pulse {
  50% { opacity: 0.35; transform: scale(0.75); }
}

@keyframes cp-rise {
  from { opacity: 0; transform: translateY(14px); }
  to { opacity: 1; transform: translateY(0); }
}

@keyframes cp-marquee {
  from { transform: translateX(0); }
  /* 内层复制了一份，位移 50% 正好接上 */
  to { transform: translateX(-50%); }
}

/* ---------- 响应式 ---------- */
@media (min-width: 1024px) {
  .cp-hero-grid {
    flex-direction: row;
    align-items: center;
    justify-content: space-between;
    gap: 56px;
  }

  .cp-hero-left {
    text-align: left;
  }

  .cp-hero-sub {
    margin-left: 0;
  }

  .cp-proof {
    justify-content: flex-start;
  }

  .cp-hero-right {
    justify-content: flex-end;
    width: auto;
  }
}

@media (max-width: 767px) {
  .cp-home-default header {
    padding: 12px 16px;
  }

  .cp-brand-name {
    font-size: 13px;
  }

  .cp-home-default main {
    padding: 40px 16px 16px;
  }

  .cp-section {
    margin-bottom: 52px;
  }

  .cp-section-title {
    font-size: 21px;
  }

  .cp-hero-title {
    font-size: clamp(30px, 9vw, 42px);
  }

  .cp-hero-sub {
    font-size: 14px;
  }

  .cp-secondary-btn {
    margin-top: 10px;
    margin-left: 0;
  }

  .cp-proof {
    gap: 8px 14px;
    font-size: 11px;
  }

  .cp-pain-grid,
  .cp-feature-grid {
    grid-template-columns: 1fr;
  }

  .cp-cta {
    padding: 32px 20px;
  }

  .cp-compare th,
  .cp-compare td {
    padding: 11px 13px;
  }
}

/* 兜底：即使全局 reduce-motion 规则被局部覆盖，也保证卡片可见 */
@media (prefers-reduced-motion: reduce) {
  .cp-pulse,
  .cp-marquee-track,
  .cp-card {
    animation: none !important;
  }

  .cp-card {
    opacity: 1;
    transform: none;
  }

  .cp-primary-btn:hover,
  .cp-secondary-btn:hover,
  .cp-card:hover {
    transform: none;
  }
}
</style>
