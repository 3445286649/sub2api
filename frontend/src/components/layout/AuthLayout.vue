<template>
  <div class="cp-auth-shell relative flex min-h-screen items-center justify-center overflow-hidden p-4">
    <!-- Background -->
    <div class="cp-auth-grid absolute inset-0"></div>
    <div class="cp-auth-glow absolute inset-0"></div>

    <!-- Decorative Elements -->
    <div class="pointer-events-none absolute inset-0 overflow-hidden">
      <!-- Gradient Orbs -->
      <div class="absolute -right-40 -top-40 h-80 w-80 rounded-full bg-primary-400/20 blur-3xl"></div>
      <div
        class="absolute -bottom-40 -left-40 h-80 w-80 rounded-full bg-primary-500/15 blur-3xl"
      ></div>
      <div
        class="absolute left-1/2 top-1/2 h-96 w-96 -translate-x-1/2 -translate-y-1/2 rounded-full bg-primary-300/10 blur-3xl"
      ></div>

      <!-- Grid Pattern -->
      <div
        class="absolute inset-0 bg-[linear-gradient(rgba(20,184,166,0.03)_1px,transparent_1px),linear-gradient(90deg,rgba(20,184,166,0.03)_1px,transparent_1px)] bg-[size:64px_64px]"
      ></div>
    </div>

    <div class="cp-auth-tools absolute right-5 top-4 z-20 flex items-center gap-2">
      <LocaleSwitcher />
      <button
        type="button"
        class="cp-auth-theme"
        :title="isDark ? '切换到浅色模式' : '切换到深色模式'"
        @click="toggleTheme"
      >
        <Icon v-if="isDark" name="sun" size="sm" />
        <Icon v-else name="moon" size="sm" />
      </button>
    </div>

    <!-- Content Container -->
    <div class="relative z-10 w-full max-w-md">
      <!-- Logo/Brand -->
      <div class="mb-8 text-center">
        <!-- Custom Logo or Default Logo -->
        <div
          class="mb-4 inline-flex h-16 w-16 items-center justify-center overflow-hidden rounded-2xl shadow-lg shadow-primary-500/30"
        >
          <img :src="siteLogo || '/logo.svg'" alt="Logo" class="h-full w-full object-contain" />
        </div>
        <h1 class="cp-auth-brand text-gradient mb-2 text-3xl font-bold">
          {{ siteName }}
        </h1>
        <p class="text-sm text-gray-500 dark:text-dark-400">
          {{ siteSubtitle }}
        </p>
      </div>

      <!-- Card Container -->
      <div class="cp-auth-card card-glass rounded-2xl p-8 shadow-glass">
        <slot />
      </div>

      <!-- Footer Links -->
      <div class="mt-6 text-center text-sm">
        <slot name="footer" />
      </div>

      <!-- Copyright -->
      <div class="mt-8 text-center text-xs text-gray-400 dark:text-dark-500">
        &copy; {{ currentYear }} {{ siteName }}. All rights reserved.
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useAppStore } from '@/stores'
import { sanitizeUrl } from '@/utils/url'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'

const appStore = useAppStore()

const siteName = computed(() => appStore.siteName || 'Sub2API')
const siteLogo = computed(() => sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || 'Subscription to API Conversion Platform')
const currentYear = computed(() => new Date().getFullYear())
const isDark = ref(document.documentElement.classList.contains('dark'))

function toggleTheme(): void {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

onMounted(() => {
  appStore.fetchPublicSettings()
})
</script>

<style scoped>
.text-gradient {
  @apply bg-gradient-to-r from-primary-600 to-primary-500 bg-clip-text text-transparent;
}

.cp-auth-shell {
  --cp-bg: #f5f4ff;
  --cp-surface: #ffffff;
  --cp-subtle: #eeecff;
  --cp-text: #0e0c1a;
  --cp-muted: #5e5878;
  --cp-border: rgba(31, 23, 70, 0.12);
  background: var(--cp-bg);
  color: var(--cp-text);
}

:global(.dark) .cp-auth-shell {
  --cp-bg: #0c0b14;
  --cp-surface: #13111f;
  --cp-subtle: #1a1829;
  --cp-text: #f0eeff;
  --cp-muted: #9e99b8;
  --cp-border: rgba(255, 255, 255, 0.1);
}

.cp-auth-grid {
  opacity: 0.75;
  background-image: linear-gradient(rgba(124, 58, 237, 0.035) 1px, transparent 1px), linear-gradient(90deg, rgba(124, 58, 237, 0.035) 1px, transparent 1px);
  background-size: 56px 56px;
  mask-image: radial-gradient(ellipse 92% 70% at 50% 50%, #000 8%, transparent 72%);
}

.cp-auth-glow {
  pointer-events: none;
  background: radial-gradient(42% 38% at 18% 14%, rgba(124, 58, 237, 0.13), transparent 62%), radial-gradient(38% 34% at 86% 10%, rgba(14, 116, 144, 0.1), transparent 62%);
  filter: blur(28px);
  animation: cp-auth-drift 24s ease-in-out infinite alternate;
}

@keyframes cp-auth-drift {
  to { transform: translate3d(2%, -2%, 0) scale(1.05); }
}

.cp-auth-tools :deep(.relative > button) {
  color: var(--cp-muted);
}

.cp-auth-theme {
  display: grid;
  width: 36px;
  height: 36px;
  place-items: center;
  border: 1px solid var(--cp-border);
  border-radius: 10px;
  color: var(--cp-muted);
  background: transparent;
  transition: border-color 0.22s ease, background 0.22s ease, transform 0.22s ease;
}

.cp-auth-theme:hover {
  border-color: rgba(124, 58, 237, 0.42);
  background: rgba(124, 58, 237, 0.1);
  transform: translateY(-1px);
}

.cp-auth-card {
  border: 1px solid var(--cp-border);
  background: color-mix(in srgb, var(--cp-surface) 90%, transparent);
  box-shadow: 0 40px 88px -32px rgba(0, 0, 0, 0.45), 0 0 0 1px rgba(0, 0, 0, 0.14), inset 0 1px 0 rgba(255, 255, 255, 0.06);
  backdrop-filter: blur(16px);
  animation: cp-auth-rise 0.72s cubic-bezier(0.22, 1, 0.36, 1) 0.15s both;
}

@keyframes cp-auth-rise {
  from { opacity: 0; transform: translateY(22px); }
  to { opacity: 1; transform: translateY(0); }
}

.cp-auth-brand {
  background: linear-gradient(100deg, #a78bfa 6%, #7c3aed 52%, #0e7490 100%);
  background-clip: text;
  -webkit-background-clip: text;
  color: transparent;
  background-size: 220% 100%;
  animation: cp-auth-gradient 11s ease-in-out infinite alternate;
}

@keyframes cp-auth-gradient {
  to { background-position: 100% 50%; }
}

@media (max-width: 520px) {
  .cp-auth-shell { align-items: flex-start; padding: 76px 16px 46px; overflow-y: auto; }
  .cp-auth-tools { right: 12px; }
  .cp-auth-card { padding: 24px 20px; }
}

@media (prefers-reduced-motion: reduce) {
  .cp-auth-glow, .cp-auth-card, .cp-auth-brand { animation: none; }
}
</style>
