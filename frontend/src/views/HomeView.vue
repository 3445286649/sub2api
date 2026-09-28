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

  <!-- Default Home Page：5 章叙事——是什么 → 跟我有关 → 凭什么 → 比谁强 → 怎么用 -->
  <div
    v-else
    data-testid="default-home"
    class="cp-home-default relative flex min-h-screen flex-col overflow-x-clip"
  >
    <!-- 顶栏：固定；滚动后毛玻璃 + 底边线；顶部一条阅读进度 -->
    <header class="cp-topbar" :class="{ 'is-scrolled': scrolled }">
      <span class="cp-topbar-progress" :style="{ transform: `scaleX(${pageProgress})` }"></span>
      <nav class="cp-topbar-inner">
        <a href="#top" class="cp-topbar-brand" @click.prevent="scrollToId('top')">
          <img :src="siteLogo || '/logo.svg'" alt="Logo" class="cp-topbar-logo" />
          <span class="cp-brand-name">{{ siteName }}</span>
        </a>

        <!-- 章节锚点（桌面） -->
        <div class="cp-topbar-anchors">
          <a
            v-for="anchor in anchors"
            :key="anchor.id"
            :href="`#${anchor.id}`"
            :class="{ 'is-active': activeSection === anchor.id }"
            @click.prevent="scrollToId(anchor.id)"
          >
            {{ anchor.label }}
          </a>
        </div>

        <div class="cp-topbar-actions">
          <LocaleSwitcher />

          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="cp-icon-btn"
            :title="t('home.viewDocs')"
          >
            <Icon name="book" size="md" />
          </a>

          <router-link
            v-if="showModelPlazaEntry"
            to="/model-plaza"
            class="cp-icon-btn"
            :title="t('nav.modelPlaza')"
          >
            <Icon name="grid" size="md" />
            <span class="hidden sm:inline">{{ t('nav.modelPlaza') }}</span>
          </router-link>

          <button
            type="button"
            class="cp-icon-btn"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
            :aria-label="isDark ? t('home.switchToLight') : t('home.switchToDark')"
            @click="toggleTheme"
          >
            <Icon v-if="isDark" name="sun" size="md" />
            <Icon v-else name="moon" size="md" />
          </button>

          <router-link v-if="isAuthenticated" :to="dashboardPath" class="cp-topbar-cta">
            <span class="cp-topbar-avatar">{{ userInitial }}</span>
            {{ t('home.dashboard') }}
          </router-link>
          <router-link v-else to="/login" class="cp-topbar-cta">
            {{ t('home.login') }}
          </router-link>
        </div>
      </nav>
    </header>

    <main class="cp-main">
      <!-- ========== ① Hero：这是什么 ========== -->
      <section id="top" class="cp-chapter cp-hero">
        <div class="cp-hero-grid">
          <div class="cp-hero-left">
            <div class="cp-eyebrow"><span class="cp-pulse"></span>{{ siteSubtitle }}</div>
            <h1 class="cp-hero-title">
              {{ siteName }}<br /><span>{{ t('home.heroSubtitle') }}</span>
            </h1>
            <p class="cp-hero-sub">{{ t('home.heroDescription') }}</p>

            <div class="cp-hero-cta">
              <router-link :to="primaryTo" class="cp-btn cp-btn-primary">
                {{ isAuthenticated ? t('home.goToDashboard') : t('home.getStarted') }}
                <Icon name="arrowRight" size="sm" :stroke-width="2" />
              </router-link>
              <!-- 配了文档地址就外链；没配则滚到「开始使用」，按钮永远有用 -->
              <a
                v-if="docUrl"
                :href="docUrl"
                target="_blank"
                rel="noopener noreferrer"
                class="cp-btn cp-btn-ghost"
              >
                {{ t('home.viewDocs') }}
                <Icon name="externalLink" size="sm" />
              </a>
              <a v-else href="#start" class="cp-btn cp-btn-ghost" @click.prevent="scrollToId('start')">
                {{ t('home.start.guide') }}
                <Icon name="arrowDown" size="sm" />
              </a>
            </div>

            <div class="cp-proof">
              <span><Icon name="check" size="xs" />{{ t('home.proof.sdk') }}</span>
              <span><Icon name="check" size="xs" />{{ t('home.proof.setup') }}</span>
              <span><Icon name="check" size="xs" />{{ t('home.proof.billing') }}</span>
              <a
                v-if="qualityEnabled"
                href="#quality"
                class="cp-proof-quality"
                @click.prevent="scrollToId('quality')"
              >
                <span class="cp-proof-dot"></span>{{ t('home.proof.quality') }}
              </a>
            </div>
          </div>

          <div class="cp-hero-right">
            <RequestJourney class="w-full max-w-[520px]" />
          </div>
        </div>

        <a
          href="#pain"
          class="cp-scroll-hint"
          :class="{ 'is-hidden': scrolled }"
          @click.prevent="scrollToId('pain')"
        >
          <span>{{ t('home.scrollHint') }}</span>
          <Icon name="chevronDown" size="sm" />
        </a>
      </section>

      <!-- ========== ② 痛点：跟我有关吗 ========== -->
      <section id="pain" class="cp-chapter">
        <header class="cp-chapter-head">
          <h2 class="cp-chapter-title">{{ t('home.painPoints.title') }}</h2>
        </header>
        <div class="cp-pain-grid">
          <article
            v-for="(item, index) in painPoints"
            :key="item.key"
            v-reveal="index"
            class="cp-card cp-pain-card"
          >
            <span class="cp-pain-icon"><Icon :name="item.icon" size="md" /></span>
            <h3 class="cp-card-title">{{ item.title }}</h3>
            <p class="cp-card-desc">{{ item.desc }}</p>
          </article>
        </div>
      </section>

      <!-- ========== ③ 工作原理：凭什么（桌面钉屏演示） ========== -->
      <HowItWorks class="cp-chapter" />

      <!-- ========== ③½ 满血保障：质量没打折（后台未配置监测地址时整章不渲染） ========== -->
      <QualityMonitor class="cp-chapter" @loaded="qualityEnabled = $event" />

      <!-- ========== ④ 为什么选我们：对比 + 支持的模型 ========== -->
      <section id="compare" class="cp-chapter">
        <header class="cp-chapter-head">
          <h2 class="cp-chapter-title">{{ t('home.comparison.title') }}</h2>
        </header>
        <div v-reveal class="cp-compare-wrap">
          <table class="cp-compare">
            <thead>
              <tr>
                <th scope="col">{{ t('home.comparison.headers.feature') }}</th>
                <th scope="col">{{ t('home.comparison.headers.official') }}</th>
                <th scope="col" class="cp-compare-us">{{ t('home.comparison.headers.us') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="row in comparisonRows" :key="row.key">
                <th scope="row">{{ row.feature }}</th>
                <td class="cp-compare-official">{{ row.official }}</td>
                <td class="cp-compare-us">
                  <Icon name="check" size="xs" class="cp-compare-check" />{{ row.us }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <div id="models" class="cp-models">
          <div class="cp-models-head">
            <h3 class="cp-models-title">{{ t('home.providers.title') }}</h3>
            <router-link v-if="showModelPlazaEntry" to="/model-plaza" class="cp-models-link">
              {{ t('home.providers.viewAll') }}
              <Icon name="arrowRight" size="xs" />
            </router-link>
          </div>

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
        </div>
      </section>

      <!-- ========== ⑤ 开始使用：怎么用起来 ========== -->
      <QuickStart
        class="cp-chapter cp-chapter-last"
        :api-base-url="apiBaseUrl"
        :doc-url="docUrl"
        :primary-to="primaryTo"
        :primary-label="isAuthenticated ? t('home.goToDashboard') : t('home.cta.button')"
      />
    </main>

    <!-- 页脚：每一项都来自站点设置，没配就不显示 -->
    <footer class="cp-footer">
      <div class="cp-footer-inner">
        <div class="cp-footer-brand">
          <div class="cp-footer-logo-row">
            <img :src="siteLogo || '/logo.svg'" alt="Logo" class="cp-topbar-logo" />
            <span class="cp-brand-name">{{ siteName }}</span>
          </div>
          <p>{{ t('home.heroSubtitle') }}</p>
        </div>

        <div class="cp-footer-col">
          <h4>{{ t('home.footer.product') }}</h4>
          <router-link v-if="showModelPlazaEntry" to="/model-plaza">{{ t('nav.modelPlaza') }}</router-link>
          <router-link to="/key-usage">{{ t('home.footer.keyUsage') }}</router-link>
          <router-link :to="primaryTo">
            {{ isAuthenticated ? t('home.dashboard') : t('home.login') }}
          </router-link>
        </div>

        <div class="cp-footer-col">
          <h4>{{ t('home.footer.resources') }}</h4>
          <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer">{{ t('home.docs') }}</a>
          <a href="#start" @click.prevent="scrollToId('start')">{{ t('home.start.guide') }}</a>
          <a :href="githubUrl" target="_blank" rel="noopener noreferrer">GitHub</a>
        </div>

        <div v-if="legalDocuments.length" class="cp-footer-col">
          <h4>{{ t('home.footer.legal') }}</h4>
          <router-link v-for="doc in legalDocuments" :key="doc.id" :to="`/legal/${doc.id}`">
            {{ doc.title }}
          </router-link>
        </div>
      </div>

      <div class="cp-footer-bottom">
        <span>&copy; {{ currentYear }} {{ siteName }}. {{ t('home.footer.allRightsReserved') }}</span>
        <span v-if="contactInfo" class="cp-footer-contact">{{ t('home.footer.contact') }}: {{ contactInfo }}</span>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useAppStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import RequestJourney from '@/components/landing/RequestJourney.vue'
import HowItWorks from '@/components/landing/HowItWorks.vue'
import QuickStart from '@/components/landing/QuickStart.vue'
import QualityMonitor from '@/components/landing/QualityMonitor.vue'
import ModelIcon from '@/components/common/ModelIcon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import { sanitizeUrl } from '@/utils/url'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'
import { useTheme } from '@/composables/useTheme'
import { vReveal } from '@/directives/reveal'

const { t } = useI18n()
const { isDark, toggleTheme } = useTheme()

const authStore = useAuthStore()
const appStore = useAppStore()

// Site settings - directly from appStore (already initialized from injected config)
const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'Sub2API')
const siteLogo = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || 'AI API Gateway Platform')
const docUrl = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl || ''))
const apiBaseUrl = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.api_base_url || ''))
const contactInfo = computed(() => (appStore.cachedPublicSettings?.contact_info || '').trim())
/** 登录协议里配置的文档（用户协议 / 隐私政策等），由 /legal/:id 公开展示 */
const legalDocuments = computed(() =>
  (appStore.cachedPublicSettings?.login_agreement_documents ?? []).filter((doc) => doc.id && doc.title),
)
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
const primaryTo = computed(() => (isAuthenticated.value ? dashboardPath.value : '/login'))
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
  // 只有真的接入了公开监测才能这么说
  ...(qualityEnabled.value
    ? [
        {
          key: 'quality',
          feature: t('home.comparison.items.quality.feature'),
          official: t('home.comparison.items.quality.official'),
          us: t('home.comparison.items.quality.us'),
        },
      ]
    : []),
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

// ==================== 滚动体验 ====================

/** 顶栏锚点。id 与各章节 section 的 id 对应 */
const anchors = computed(() => [
  { id: 'how', label: t('home.nav.how') },
  ...(qualityEnabled.value ? [{ id: 'quality', label: t('home.nav.quality') }] : []),
  { id: 'compare', label: t('home.nav.compare') },
  { id: 'models', label: t('home.nav.models') },
  { id: 'start', label: t('home.nav.start') },
])

/** 满血监测是否已配置；由 QualityMonitor 加载完成后回传 */
const qualityEnabled = ref(false)
const scrolled = ref(false)
const pageProgress = ref(0)
const activeSection = ref('')
let scrollFrame = 0
let sectionObserver: IntersectionObserver | null = null

function updateScrollState(): void {
  scrollFrame = 0
  const doc = document.documentElement
  const max = doc.scrollHeight - window.innerHeight
  scrolled.value = window.scrollY > 12
  pageProgress.value = max > 0 ? Math.min(window.scrollY / max, 1) : 0
}

function handleScroll(): void {
  if (scrollFrame) return
  scrollFrame = requestAnimationFrame(updateScrollState)
}

function scrollToId(id: string): void {
  const el = document.getElementById(id)
  if (!el) return
  // scroll-padding-top 已为固定顶栏留出空间（style.css 的 html.cp-home-scroll）
  el.scrollIntoView({ block: 'start' })
}

/** 用一条贴近视口上部的观察线判断「当前所在章节」，驱动锚点高亮 */
function observeSections(): void {
  if (typeof IntersectionObserver === 'undefined') return
  sectionObserver = new IntersectionObserver(
    (entries) => {
      entries.forEach((entry) => {
        if (entry.isIntersecting) activeSection.value = entry.target.id
      })
    },
    { rootMargin: '-35% 0px -60% 0px' },
  )
  ;['top', 'pain', 'how', 'quality', 'compare', 'start'].forEach((id) => {
    const el = document.getElementById(id)
    if (el) sectionObserver?.observe(el)
  })
}

// 满血章节异步出现，挂载时的观察器还看不到它，出现后补挂
watch(qualityEnabled, async (enabled) => {
  if (!enabled) return
  await nextTick()
  const el = document.getElementById('quality')
  if (el) sectionObserver?.observe(el)
})

onMounted(() => {
  // Check auth state
  authStore.checkAuth()

  // Ensure public settings are loaded (will use cache if already loaded from injected config)
  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings()
  }

  // 平滑滚动 / 锚点偏移 / 桌面轻吸附只作用于首页，离开时移除
  document.documentElement.classList.add('cp-home-scroll')
  window.addEventListener('scroll', handleScroll, { passive: true })
  updateScrollState()
  observeSections()
})

onUnmounted(() => {
  document.documentElement.classList.remove('cp-home-scroll')
  window.removeEventListener('scroll', handleScroll)
  if (scrollFrame) cancelAnimationFrame(scrollFrame)
  sectionObserver?.disconnect()
  sectionObserver = null
})
</script>

<style scoped>
/* ============================================================
 * 首页皮肤
 *
 * 配色统一走全局语义 token（src/style.css 的 :root / .dark），
 * 选择器一律使用语义类名，不用布局工具类当选择器。
 * 入场动画由 v-reveal 指令统一处理（样式在 style.css 的 .cp-reveal）。
 * ============================================================ */

.cp-home-default {
  background:
    radial-gradient(42% 38% at 18% 6%, var(--glow-brand), transparent 62%),
    radial-gradient(38% 34% at 86% 4%, var(--glow-brand-2), transparent 62%),
    var(--surface-2);
  color: var(--text);
  font-family: Inter, 'PingFang SC', 'Microsoft YaHei', system-ui, sans-serif;
  isolation: isolate;
}

/* 背景网格：只铺在首屏区域，靠 mask 淡出 */
.cp-home-default::before {
  content: '';
  position: absolute;
  inset: 0 0 auto;
  z-index: -1;
  height: 110vh;
  pointer-events: none;
  opacity: 0.8;
  background-image:
    linear-gradient(color-mix(in srgb, var(--brand) 4%, transparent) 1px, transparent 1px),
    linear-gradient(90deg, color-mix(in srgb, var(--brand) 4%, transparent) 1px, transparent 1px);
  background-size: 56px 56px;
  mask-image: radial-gradient(ellipse 92% 70% at 50% 40%, #000 8%, transparent 72%);
}

.cp-brand-name {
  color: var(--text);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 15px;
  font-weight: 700;
  letter-spacing: 0.02em;
  white-space: nowrap;
}

/* ============================================================
 * 顶栏
 * ============================================================ */
.cp-topbar {
  position: fixed;
  inset: 0 0 auto;
  z-index: 40;
  border-bottom: 1px solid transparent;
  transition: background 0.3s ease, border-color 0.3s ease, backdrop-filter 0.3s ease;
}

.cp-topbar.is-scrolled {
  border-bottom-color: var(--border);
  background: color-mix(in srgb, var(--surface-2) 78%, transparent);
  backdrop-filter: blur(18px) saturate(150%);
}

.cp-topbar-progress {
  position: absolute;
  inset: auto 0 -1px;
  height: 2px;
  background: linear-gradient(90deg, var(--brand-soft), var(--brand), var(--brand-2));
  transform-origin: left;
  pointer-events: none;
}

.cp-topbar-inner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  width: min(100%, 1180px);
  height: 64px;
  margin: 0 auto;
  padding: 0 28px;
}

.cp-topbar-brand {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 10px;
}

.cp-topbar-logo {
  width: 34px;
  height: 34px;
  flex-shrink: 0;
  border-radius: 10px;
  object-fit: contain;
}

.cp-topbar-anchors {
  display: none;
  gap: 2px;
}

.cp-topbar-anchors a {
  position: relative;
  padding: 7px 12px;
  border-radius: 9px;
  color: var(--muted);
  font-size: 13.5px;
  font-weight: 550;
  white-space: nowrap;
  transition: color 0.2s ease, background 0.2s ease;
}

.cp-topbar-anchors a:hover {
  color: var(--text);
  background: color-mix(in srgb, var(--brand) 8%, transparent);
}

.cp-topbar-anchors a.is-active {
  color: var(--brand);
}

.cp-topbar-anchors a.is-active::after {
  content: '';
  position: absolute;
  inset: auto 12px 2px;
  height: 2px;
  border-radius: 2px;
  background: var(--brand);
}

.cp-topbar-actions {
  display: flex;
  flex-shrink: 0;
  align-items: center;
  gap: 6px;
}

.cp-icon-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px;
  border-radius: 10px;
  color: var(--muted);
  font-size: 13.5px;
  transition: color 0.2s ease, background 0.2s ease;
}

.cp-icon-btn:hover {
  color: var(--text);
  background: color-mix(in srgb, var(--brand) 9%, transparent);
}

.cp-topbar-cta {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  margin-left: 4px;
  padding: 7px 15px;
  border-radius: 999px;
  background: var(--brand);
  color: #042f2e;
  font-size: 13px;
  font-weight: 700;
  white-space: nowrap;
  transition: transform 0.2s ease, box-shadow 0.25s ease;
}

.cp-topbar-cta:hover {
  transform: translateY(-1px);
  box-shadow: 0 8px 20px -10px color-mix(in srgb, var(--brand) 70%, transparent);
}

.cp-topbar-avatar {
  display: grid;
  width: 20px;
  height: 20px;
  place-items: center;
  border-radius: 999px;
  background: color-mix(in srgb, #fff 45%, transparent);
  font-size: 10.5px;
}

/* ============================================================
 * 章节
 * ============================================================ */
.cp-main {
  width: min(100%, 1180px);
  margin: 0 auto;
  padding: 0 28px;
}

.cp-chapter {
  padding: 96px 0;
  scroll-snap-align: start;
}

.cp-chapter-last {
  padding-bottom: 120px;
}

.cp-chapter-head {
  margin-bottom: 32px;
  text-align: center;
}

.cp-chapter-title {
  margin: 0;
  color: var(--text);
  font-size: clamp(24px, 3vw, 34px);
  font-weight: 750;
  letter-spacing: -0.025em;
}

/* ---------- ① Hero ---------- */
.cp-hero {
  position: relative;
  display: flex;
  min-height: 100svh;
  flex-direction: column;
  justify-content: center;
  padding-top: 96px;
}

.cp-hero-grid {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 44px;
}

.cp-hero-left {
  flex: 1;
  min-width: 0;
  text-align: center;
}

.cp-eyebrow {
  display: inline-flex;
  align-items: center;
  gap: 9px;
  margin-bottom: 18px;
  padding: 5px 13px 5px 9px;
  border: 1px solid var(--border);
  border-radius: 999px;
  background: color-mix(in srgb, var(--brand) 7%, transparent);
  color: var(--muted);
  font-size: 12px;
  font-weight: 550;
}

.cp-pulse {
  position: relative;
  width: 7px;
  height: 7px;
  border-radius: 999px;
  background: var(--brand);
}

.cp-pulse::after {
  content: '';
  position: absolute;
  inset: 0;
  border-radius: inherit;
  background: var(--brand);
  animation: cp-ping 2.2s cubic-bezier(0, 0, 0.2, 1) infinite;
}

.cp-hero-title {
  margin: 0 0 18px;
  color: var(--text);
  font-size: clamp(34px, 5.2vw, 60px);
  font-weight: 780;
  letter-spacing: -0.045em;
  line-height: 1.08;
}

.cp-hero-title span {
  background: linear-gradient(100deg, var(--brand-soft), var(--brand) 45%, var(--brand-2));
  background-size: 220% 100%;
  background-clip: text;
  -webkit-background-clip: text;
  color: transparent;
  animation: cp-grad 10s ease-in-out infinite alternate;
}

.cp-hero-sub {
  max-width: 540px;
  margin: 0 auto;
  color: var(--muted);
  font-size: 16px;
  line-height: 1.75;
}

.cp-hero-cta {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 12px;
  margin-top: 32px;
}

.cp-btn {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-height: 46px;
  padding: 12px 24px;
  border-radius: 12px;
  font-size: 15px;
  font-weight: 650;
  white-space: nowrap;
  transition: transform 0.22s ease, box-shadow 0.28s ease, border-color 0.22s ease;
}

.cp-btn:hover {
  transform: translateY(-2px);
}

.cp-btn:focus-visible {
  outline: 2px solid var(--brand);
  outline-offset: 2px;
}

.cp-btn-primary {
  position: relative;
  overflow: hidden;
  background: linear-gradient(135deg, var(--brand-soft), var(--brand));
  color: #042f2e;
  box-shadow: 0 12px 28px -12px color-mix(in srgb, var(--brand) 60%, transparent);
}

/* 悬停高光从左扫到右 */
.cp-btn-primary::after {
  content: '';
  position: absolute;
  inset: 0 auto 0 -130%;
  width: 60%;
  background: linear-gradient(100deg, transparent, rgb(255 255 255 / 0.45), transparent);
  transform: skewX(-18deg);
}

.cp-btn-primary:hover::after {
  animation: cp-sheen 0.9s ease;
}

.cp-btn-ghost {
  border: 1px solid var(--border-strong);
  background: color-mix(in srgb, var(--surface) 70%, transparent);
  color: var(--text);
  backdrop-filter: blur(10px);
}

.cp-btn-ghost:hover {
  border-color: color-mix(in srgb, var(--brand) 45%, var(--border));
}

.cp-proof {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 10px 20px;
  margin-top: 28px;
  padding-top: 20px;
  border-top: 1px solid var(--border);
  color: var(--muted);
  font-size: 12.5px;
}

.cp-proof span {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.cp-proof svg {
  color: var(--accent-ok);
}

.cp-proof-quality {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  color: var(--accent-ok);
  font-weight: 650;
}

.cp-proof-quality:hover {
  text-decoration: underline;
  text-underline-offset: 3px;
}

.cp-proof-dot {
  position: relative;
  width: 7px;
  height: 7px;
  border-radius: 999px;
  background: var(--accent-ok);
}

.cp-proof-dot::after {
  content: '';
  position: absolute;
  inset: 0;
  border-radius: inherit;
  background: inherit;
  animation: cp-ping 2.2s cubic-bezier(0, 0, 0.2, 1) infinite;
}

.cp-hero-right {
  display: flex;
  flex: 1;
  justify-content: center;
  width: 100%;
  min-width: 0;
}

.cp-scroll-hint {
  position: absolute;
  bottom: 28px;
  left: 50%;
  display: none;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  color: var(--muted);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 10px;
  letter-spacing: 0.2em;
  text-transform: uppercase;
  transform: translateX(-50%);
  transition: opacity 0.4s ease;
}

.cp-scroll-hint svg {
  color: var(--brand);
  animation: cp-bob 2s ease-in-out infinite;
}

.cp-scroll-hint.is-hidden {
  opacity: 0;
  pointer-events: none;
}

/* ---------- 通用卡片 ---------- */
.cp-card {
  border: 1px solid var(--border);
  border-radius: 16px;
  background: var(--surface);
  box-shadow: var(--shadow-card);
  transition: border-color 0.28s ease, transform 0.28s ease, box-shadow 0.28s ease;
}

.cp-card:hover {
  border-color: color-mix(in srgb, var(--brand) 38%, var(--border));
  transform: translateY(-3px);
  box-shadow: var(--shadow-float);
}

.cp-card-title {
  margin: 0 0 8px;
  color: var(--text);
  font-size: 15.5px;
  font-weight: 700;
}

.cp-card-desc {
  margin: 0;
  color: var(--muted);
  font-size: 13px;
  line-height: 1.7;
}

/* ---------- ② 痛点 ---------- */
.cp-pain-grid {
  display: grid;
  gap: 14px;
  grid-template-columns: repeat(4, minmax(0, 1fr));
}

.cp-pain-card {
  padding: 22px;
}

.cp-pain-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  margin-bottom: 16px;
  border-radius: 12px;
  background: color-mix(in srgb, var(--accent-warn) 14%, transparent);
  color: var(--accent-warn);
}

/* ---------- ④ 对比 + 模型 ---------- */
.cp-compare-wrap {
  overflow-x: auto;
  border: 1px solid var(--border);
  border-radius: 16px;
  background: var(--surface);
  box-shadow: var(--shadow-card);
}

.cp-compare {
  width: 100%;
  min-width: 520px;
  border-collapse: collapse;
  font-size: 14px;
}

.cp-compare th,
.cp-compare td {
  padding: 15px 20px;
  text-align: left;
  border-bottom: 1px solid var(--border);
}

.cp-compare thead th {
  color: var(--muted);
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  white-space: nowrap;
}

.cp-compare tbody th {
  color: var(--text);
  font-weight: 650;
  white-space: nowrap;
}

.cp-compare tbody tr:last-child th,
.cp-compare tbody tr:last-child td {
  border-bottom: none;
}

.cp-compare-official {
  color: var(--muted);
}

/* 本平台列高亮：整列淡色底 + 品牌色文字 */
.cp-compare-us {
  color: var(--brand);
  font-weight: 650;
  background: color-mix(in srgb, var(--brand) 7%, transparent);
}

.cp-compare thead .cp-compare-us {
  background: color-mix(in srgb, var(--brand) 12%, transparent);
}

.cp-compare-check {
  display: inline-block;
  margin-right: 6px;
  vertical-align: -1px;
}

.cp-models {
  margin-top: 56px;
}

.cp-models-head {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 18px;
}

.cp-models-title {
  margin: 0;
  color: var(--text);
  font-size: 17px;
  font-weight: 700;
}

.cp-models-link {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  color: var(--brand);
  font-size: 13.5px;
  font-weight: 650;
}

.cp-models-link:hover {
  text-decoration: underline;
  text-underline-offset: 3px;
}

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
  flex: 0 0 auto;
  align-items: center;
  gap: 10px;
  padding: 12px 18px;
  border: 1px solid var(--border);
  border-radius: 13px;
  background: var(--surface);
  transition: border-color 0.25s ease;
}

.cp-provider:hover {
  border-color: color-mix(in srgb, var(--brand) 34%, var(--border));
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
  /* 单色化：统一跟随文本色，消除各家品牌色的冲突 */
  color: var(--muted);
  transition: color 0.25s ease;
}

.cp-provider:hover .cp-provider-logo {
  color: var(--brand);
}

/* ModelIcon 在 path 上写死了 fill，深色下 GPT 会看不见；首页内统一改为跟随 currentColor */
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
  color: var(--muted);
  font-size: 12px;
  font-weight: 700;
}

.cp-provider-name {
  color: var(--text);
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
  color: var(--muted);
}

/* ============================================================
 * 页脚
 * ============================================================ */
.cp-footer {
  border-top: 1px solid var(--border);
  background: color-mix(in srgb, var(--surface) 55%, transparent);
}

.cp-footer-inner {
  display: grid;
  grid-template-columns: minmax(0, 1.6fr) repeat(3, minmax(0, 1fr));
  gap: 32px;
  width: min(100%, 1180px);
  margin: 0 auto;
  padding: 48px 28px 32px;
}

.cp-footer-logo-row {
  display: flex;
  align-items: center;
  gap: 10px;
}

.cp-footer-brand p {
  max-width: 280px;
  margin: 12px 0 0;
  color: var(--muted);
  font-size: 13px;
  line-height: 1.7;
}

.cp-footer-col {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.cp-footer-col h4 {
  margin: 0 0 4px;
  color: var(--text);
  font-size: 13px;
  font-weight: 700;
}

.cp-footer-col a {
  width: fit-content;
  color: var(--muted);
  font-size: 13px;
  transition: color 0.2s ease;
}

.cp-footer-col a:hover {
  color: var(--brand);
}

.cp-footer-bottom {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  gap: 8px 20px;
  width: min(100%, 1180px);
  margin: 0 auto;
  padding: 18px 28px 26px;
  border-top: 1px solid var(--border);
  color: var(--muted);
  font-size: 12.5px;
}

.cp-footer-contact {
  overflow-wrap: anywhere;
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


/* ============================================================
 * 动画
 * ============================================================ */
@keyframes cp-pulse {
  50% { opacity: 0.35; transform: scale(0.75); }
}

@keyframes cp-ping {
  0% { opacity: 0.8; transform: scale(1); }
  75%, 100% { opacity: 0; transform: scale(3.2); }
}

@keyframes cp-grad {
  to { background-position: 100% 50%; }
}

@keyframes cp-sheen {
  to { left: 150%; }
}

@keyframes cp-bob {
  0%, 100% { opacity: 0.5; transform: translateY(0); }
  50% { opacity: 1; transform: translateY(4px); }
}

@keyframes cp-marquee {
  from { transform: translateX(0); }
  /* 内层复制了一份，位移 50% 正好接上 */
  to { transform: translateX(-50%); }
}

/* ============================================================
 * 响应式
 * ============================================================ */
@media (min-width: 1024px) {
  .cp-topbar-anchors {
    display: flex;
  }

  .cp-hero-grid {
    flex-direction: row;
    justify-content: space-between;
    gap: 56px;
  }

  .cp-hero-left {
    text-align: left;
  }

  .cp-hero-sub {
    margin-left: 0;
  }

  .cp-hero-cta,
  .cp-proof {
    justify-content: flex-start;
  }

  .cp-hero-right {
    justify-content: flex-end;
    width: auto;
  }
}

/* 滚动提示只在够高的桌面屏上出现，矮屏会压到 Hero 内容 */
@media (min-width: 1024px) and (min-height: 760px) {
  .cp-scroll-hint {
    display: flex;
  }
}

@media (max-width: 1023px) {
  .cp-pain-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .cp-footer-inner {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .cp-footer-brand {
    grid-column: 1 / -1;
  }
}

@media (max-width: 767px) {
  .cp-topbar-inner {
    height: 58px;
    padding: 0 14px;
    gap: 8px;
  }

  .cp-topbar-logo {
    width: 30px;
    height: 30px;
  }

  .cp-brand-name {
    font-size: 13px;
  }

  .cp-topbar-actions {
    gap: 2px;
  }

  .cp-topbar-cta {
    padding: 6px 12px;
  }

  .cp-main {
    padding: 0 16px;
  }

  .cp-chapter {
    padding: 64px 0;
  }

  .cp-chapter-last {
    padding-bottom: 72px;
  }

  .cp-hero {
    min-height: auto;
    padding-top: 100px;
  }

  .cp-hero-title {
    font-size: clamp(30px, 9vw, 42px);
  }

  .cp-hero-sub {
    font-size: 14.5px;
  }

  .cp-btn {
    padding: 11px 18px;
    font-size: 14px;
  }

  .cp-proof {
    gap: 8px 14px;
    font-size: 11.5px;
  }

  .cp-pain-grid {
    grid-template-columns: 1fr;
  }

  .cp-compare th,
  .cp-compare td {
    padding: 11px 13px;
  }

  .cp-models {
    margin-top: 44px;
  }

  .cp-footer-inner {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 24px;
    padding: 36px 16px 24px;
  }

  .cp-footer-bottom {
    padding: 16px 16px 22px;
  }
}

/* 兜底：即使全局 reduce-motion 规则被局部覆盖，也不留动画 */
@media (prefers-reduced-motion: reduce) {
  .cp-pulse::after,
  .cp-proof-dot::after,
  .cp-hero-title span,
  .cp-marquee-track,
  .cp-scroll-hint svg {
    animation: none !important;
  }

  .cp-btn:hover,
  .cp-card:hover,
  .cp-topbar-cta:hover {
    transform: none;
  }
}
</style>
