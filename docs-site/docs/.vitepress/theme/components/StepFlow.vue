<script setup lang="ts">
/**
 * 页首的步骤概览：一眼看到总共几步，点击跳到对应小节。
 * steps 写成「标题|锚点」，锚点可省略，例如 :steps="['安装工具|#install', '创建密钥']"。
 */
import { computed } from 'vue'

const props = defineProps<{ steps: string[] }>()

const items = computed(() =>
  props.steps.map((raw) => {
    const [title, href] = raw.split('|')
    return { title: title.trim(), href: href?.trim() }
  }),
)
</script>

<template>
  <ol class="step-flow">
    <li v-for="(item, i) in items" :key="item.title" class="step-flow__item">
      <component :is="item.href ? 'a' : 'span'" :href="item.href" class="step-flow__link">
        <span class="step-flow__index" aria-hidden="true">{{ i + 1 }}</span>
        <span class="step-flow__title">{{ item.title }}</span>
      </component>
    </li>
  </ol>
</template>
