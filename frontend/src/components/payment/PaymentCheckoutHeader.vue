<template>
      <div class="cp-pay-header">
        <div class="cp-pay-header__icon">
          <img v-if="appStore.siteLogo" :src="appStore.siteLogo" alt="site logo" class="cp-pay-header__logo-img" />
          <!-- pixel fries fallback -->
          <svg v-else viewBox="0 0 80 80" fill="none" class="cp-pay-header__logo-svg">
            <rect width="80" height="80" rx="14" fill="#F5B800"/>
            <rect x="22" y="16" width="8" height="28" rx="2" fill="#FFE566"/>
            <rect x="22" y="12" width="8" height="8" rx="1" fill="#FFD700"/>
            <rect x="32" y="12" width="8" height="32" rx="2" fill="#FFE566"/>
            <rect x="32" y="8"  width="8" height="8"  rx="1" fill="#FFD700"/>
            <rect x="42" y="14" width="8" height="30" rx="2" fill="#FFE566"/>
            <rect x="42" y="10" width="8" height="8"  rx="1" fill="#FFD700"/>
            <rect x="52" y="16" width="8" height="28" rx="2" fill="#FFE566"/>
            <rect x="52" y="12" width="8" height="8"  rx="1" fill="#FFD700"/>
            <rect x="18" y="42" width="44" height="26" rx="4" fill="#CC2200"/>
            <path d="M18 44 Q40 38 62 44 L62 48 Q40 42 18 48 Z" fill="#E03010"/>
            <rect x="31" y="52" width="5" height="10" rx="1" fill="#F5B800"/>
            <rect x="44" y="52" width="5" height="10" rx="1" fill="#F5B800"/>
            <rect x="36" y="56" width="8" height="6"  rx="1" fill="#F5B800"/>
          </svg>
        </div>
        <div class="cp-pay-header__text">
          <div class="cp-pay-header__title">
            <span class="cp-pay-header__brand">{{ appStore.siteName || 'codeplan' }}</span>
            <span class="cp-pay-header__lock">
              <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
                <rect x="3" y="11" width="18" height="11" rx="2"/>
                <path d="M7 11V7a5 5 0 0110 0v4"/>
              </svg>
              {{ t('payment.qr.securePay') }}
            </span>
          </div>
          <h3 :id="titleId" class="cp-pay-header__sub">{{ title }}</h3>
        </div>
        <slot name="actions" />
      </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'

defineProps<{ title: string; titleId?: string }>()
const { t } = useI18n()
const appStore = useAppStore()
</script>

<style scoped>
.cp-pay-header {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 14px 18px 12px;
  background: var(--color-surface-3, theme('colors.gray.50'));
  border-bottom: 1px solid var(--color-border, theme('colors.gray.200'));
}
.dark .cp-pay-header {
  background: theme('colors.dark.900');
  border-bottom-color: theme('colors.dark.700');
}

.cp-pay-header__icon {
  width: 34px;
  height: 34px;
  border-radius: 10px;
  overflow: hidden;
  flex-shrink: 0;
}
.cp-pay-header__logo-img {
  width: 100%;
  height: 100%;
  object-fit: contain;
  border-radius: 8px;
}
.cp-pay-header__logo-svg {
  width: 100%;
  height: 100%;
}

.cp-pay-header__text {
  flex: 1;
  min-width: 0;
}
.cp-pay-header__title {
  display: flex;
  align-items: center;
  gap: 7px;
  flex-wrap: wrap;
  font-size: 13px;
  font-weight: 700;
  color: theme('colors.gray.900');
}
.dark .cp-pay-header__title {
  color: theme('colors.gray.100');
}
.cp-pay-header__brand {
  background: linear-gradient(90deg, theme('colors.primary.600'), theme('colors.violet.500'));
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
  font-weight: 800;
  overflow-wrap: anywhere;
}
.dark .cp-pay-header__brand {
  background: linear-gradient(90deg, theme('colors.primary.400'), theme('colors.violet.400'));
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
}
.cp-pay-header__lock {
  display: flex;
  align-items: center;
  gap: 3px;
  font-size: 10px;
  font-weight: 600;
  color: theme('colors.emerald.600');
}
.dark .cp-pay-header__lock {
  color: theme('colors.emerald.400');
}
.cp-pay-header__sub {
  font-size: 11px;
  color: theme('colors.gray.500');
  margin-top: 1px;
}
.dark .cp-pay-header__sub {
  color: theme('colors.gray.400');
}

</style>
