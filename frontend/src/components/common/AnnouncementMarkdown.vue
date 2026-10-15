<template>
  <div ref="body" class="markdown-body prose prose-sm max-w-none dark:prose-invert" v-html="renderedContent" @click="openImage" @keydown="handleImageKeydown"></div>
  <Teleport to="body">
    <div v-if="previewSrc" class="fixed inset-0 z-[150] flex items-center justify-center bg-black/85 p-4" role="dialog" aria-modal="true" :aria-label="t('announcements.imagePreview')" @click.self="closePreview" @keydown.esc.stop="closePreview">
      <button ref="closeButton" type="button" class="absolute right-4 top-4 flex h-10 w-10 items-center justify-center rounded bg-white text-gray-900" :aria-label="t('common.close')" @click="closePreview">
        <Icon name="x" size="md" />
      </button>
      <img :src="previewSrc" :alt="previewAlt" class="max-h-[90vh] max-w-[95vw] object-contain" />
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUpdated, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import Icon from '@/components/icons/Icon.vue'
import '@/styles/announcement-markdown.css'

const props = defineProps<{ content: string }>()
const { t } = useI18n()
const body = ref<HTMLElement | null>(null)
const closeButton = ref<HTMLButtonElement | null>(null)
const previewSrc = ref('')
const previewAlt = ref('')
const triggerImage = ref<HTMLImageElement | null>(null)
const renderedContent = computed(() => {
  const html = DOMPurify.sanitize(marked.parse(props.content || '', { async: false, breaks: true, gfm: true }) as string)
  const template = document.createElement('template')
  template.innerHTML = html
  let gallery: HTMLDivElement | null = null

  for (const node of Array.from(template.content.childNodes)) {
    if (node.nodeType === Node.TEXT_NODE && !node.textContent?.trim()) continue
    if (!(node instanceof HTMLParagraphElement)) {
      gallery = null
      continue
    }

    const children = Array.from(node.childNodes)
    const images = children.filter((child): child is HTMLImageElement => child instanceof HTMLImageElement)
    const imageOnly = images.length > 0 && children.every(child =>
      child instanceof HTMLImageElement || child instanceof HTMLBRElement ||
      (child.nodeType === Node.TEXT_NODE && !child.textContent?.trim())
    )
    if (!imageOnly) {
      gallery = null
      continue
    }

    if (!gallery) {
      gallery = document.createElement('div')
      gallery.className = 'announcement-image-grid'
      node.before(gallery)
    }
    images.forEach(image => gallery?.append(image))
    node.remove()
  }

  return template.innerHTML
})

function prepareImages() {
  body.value?.querySelectorAll('img').forEach(image => {
    image.tabIndex = 0
    image.setAttribute('role', 'button')
    image.setAttribute('aria-label', t('announcements.imagePreview'))
  })
}

onMounted(prepareImages)
onUpdated(prepareImages)

async function openImage(event: Event) {
  const image = event.target
  if (!(image instanceof HTMLImageElement) || !body.value?.contains(image)) return
  triggerImage.value = image
  previewSrc.value = image.currentSrc || image.src
  previewAlt.value = image.alt
  await nextTick()
  closeButton.value?.focus()
}

function handleImageKeydown(event: KeyboardEvent) {
  if (!(event.target instanceof HTMLImageElement) || !['Enter', ' '].includes(event.key)) return
  event.preventDefault()
  void openImage(event)
}

function closePreview() {
  previewSrc.value = ''
  triggerImage.value?.focus()
  triggerImage.value = null
}
</script>
