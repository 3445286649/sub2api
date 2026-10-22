<script setup lang="ts">
/**
 * 软件下载卡片：左侧名称和说明，右侧下载按钮。
 * 外部链接新窗口打开；站内文件（/downloads/...）直接下载。
 */
import { computed } from 'vue'

const props = defineProps<{
  name: string
  href: string
  /** 一句话提示，例如「选择 .msi 安装包」 */
  hint?: string
  /** 卡片左侧的字母图标，默认取名称首字母 */
  badge?: string
  /** 按钮文字，默认「下载」 */
  label?: string
}>()

const external = computed(() => /^https?:\/\//.test(props.href))
const badgeText = computed(() => props.badge ?? props.name.slice(0, 1).toUpperCase())
</script>

<template>
  <div class="download-card">
    <span class="download-card__badge" aria-hidden="true">{{ badgeText }}</span>
    <div class="download-card__body">
      <strong class="download-card__name">{{ name }}</strong>
      <span v-if="hint" class="download-card__hint">{{ hint }}</span>
    </div>
    <a
      class="download-card__button"
      :href="href"
      :target="external ? '_blank' : undefined"
      :rel="external ? 'noopener noreferrer' : undefined"
      :download="external ? undefined : ''"
      :aria-label="`${label ?? '下载'} ${name}`"
    >
      {{ label ?? '下载' }}
    </a>
  </div>
</template>
