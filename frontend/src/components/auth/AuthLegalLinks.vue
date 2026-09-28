<template>
  <!--
    登录 / 注册页底部的常驻条款入口，与「协议闸门」是否开启无关：
    闸门关闭时用户仍应能找到协议原文。
  -->
  <p v-if="visible" class="cp-legal-links">
    <span>{{ t('legal.legalLinks.prefix') }}</span>
    <template v-for="(doc, index) in documents" :key="doc.id || doc.title">
      <RouterLink
        :to="`/legal/${doc.id}`"
        target="_blank"
        rel="noopener noreferrer"
        class="cp-legal-link"
      >
        {{ doc.title }}
      </RouterLink>
      <span v-if="index < documents.length - 1" class="cp-legal-sep">
        {{ t('legal.legalLinks.separator') }}
      </span>
    </template>
  </p>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'
import { selectLegalDocuments } from '@/utils/legalDocuments'

/**
 * 勾选模式下，条款链接已经内联显示在勾选框那一行，
 * 页脚再列一遍会同一页出现两组同样的链接，所以由父级关掉。
 */
const props = withDefaults(defineProps<{ enabled?: boolean }>(), { enabled: true })

const { t } = useI18n()
const appStore = useAppStore()

/**
 * 只列出有正文的文档：后端在未配置时会返回一组正文为空的默认协议，
 * 直接展示会让用户点进空白页。
 */
const documents = computed(() =>
  selectLegalDocuments(appStore.cachedPublicSettings?.login_agreement_documents)
)

const visible = computed(() => props.enabled && documents.value.length > 0)
</script>

<style scoped>
.cp-legal-links {
  margin: 0;
  color: var(--muted);
  font-size: 12px;
  line-height: 1.7;
  text-align: center;
}

.cp-legal-link {
  color: var(--muted);
  text-decoration: underline;
  text-underline-offset: 3px;
  text-decoration-color: color-mix(in srgb, var(--muted) 45%, transparent);
  transition: color 0.2s ease, text-decoration-color 0.2s ease;
}

.cp-legal-link:hover {
  color: var(--brand);
  text-decoration-color: currentColor;
}

.cp-legal-sep {
  margin: 0 6px;
  color: color-mix(in srgb, var(--muted) 65%, transparent);
}
</style>
