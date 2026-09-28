<template>
  <div
    v-if="mode === 'checkbox' && documents.length > 0"
    class="px-0.5"
  >
    <div class="flex items-start gap-2">
      <input
        id="login-agreement-consent"
        type="checkbox"
        :checked="accepted"
        class="cp-consent-checkbox mt-[2px] h-4 w-4 flex-shrink-0 rounded"
        @change="handleCheckboxChange"
      />
      <div class="min-w-0 flex-1">
        <p class="cp-consent-text text-[13px] leading-5">
          <label for="login-agreement-consent" class="cursor-pointer">
            {{ t('legal.loginAgreementPrompt.checkboxPrefix') }}
          </label>
          <template v-for="(doc, index) in documents" :key="doc.id || doc.title">
            <RouterLink
              :to="documentRoute(doc)"
              target="_blank"
              rel="noopener noreferrer"
              class="cp-consent-link font-medium underline-offset-4"
            >
              {{ doc.title }}
            </RouterLink>
            <span v-if="index < documents.length - 1">{{ t('legal.loginAgreementPrompt.documentSeparator') }}</span>
          </template>
        </p>
      </div>
    </div>
  </div>

  <div
    v-else-if="!accepted && documents.length > 0"
    class="cp-consent-notice rounded-lg p-3 text-sm"
  >
    <div class="flex items-start gap-3">
      <Icon name="shield" size="sm" class="cp-consent-notice-icon mt-0.5 flex-shrink-0" />
      <div class="min-w-0 flex-1">
        <p class="cp-consent-notice-title font-medium">
          {{ t('legal.loginAgreementPrompt.noticeTitle') }}
        </p>
        <p class="cp-consent-notice-desc mt-1">
          {{ t('legal.loginAgreementPrompt.noticeDescription') }}
        </p>
      </div>
      <button
        type="button"
        class="cp-consent-notice-btn flex-shrink-0 rounded-md px-3 py-1.5 text-xs font-medium"
        @click="emit('open')"
      >
        {{ t('legal.loginAgreementPrompt.viewTerms') }}
      </button>
    </div>
  </div>

  <Teleport to="body">
    <Transition name="agreement-fade">
      <div
        v-if="dialogVisible"
        class="cp-agreement-overlay fixed inset-0 z-[140] flex items-center justify-center overflow-y-auto p-4"
      >
        <div class="cp-agreement-panel w-full max-w-[600px] overflow-hidden rounded-2xl">
          <div class="cp-agreement-head px-6 py-6">
            <div class="flex items-start gap-4">
              <span class="cp-agreement-badge flex h-12 w-12 flex-shrink-0 items-center justify-center rounded-xl">
                <Icon name="shield" size="md" />
              </span>
              <div class="min-w-0 flex-1">
                <div class="flex flex-wrap items-center gap-2">
                  <h2 class="cp-agreement-title text-xl font-bold tracking-normal">
                    {{ t('legal.loginAgreementPrompt.dialogTitle') }}
                  </h2>
                  <span
                    v-if="updatedAt"
                    class="cp-agreement-chip rounded-full px-2.5 py-1 text-xs font-medium"
                  >
                    {{ updatedAt }}
                  </span>
                </div>
                <p class="cp-agreement-desc mt-2 text-sm leading-6">
                  {{
                    t('legal.loginAgreementPrompt.dialogDescription', {
                      date: updatedAt || t('legal.loginAgreementPrompt.recently'),
                    })
                  }}
                </p>
              </div>
            </div>
          </div>

          <div class="cp-agreement-body max-h-[58vh] overflow-y-auto px-6 py-5">
            <div class="mb-3 flex items-center justify-between gap-3">
              <p class="cp-agreement-body-title text-sm font-semibold">
                {{ t('legal.loginAgreementPrompt.relatedDocuments') }}
              </p>
            </div>
            <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
              <RouterLink
                v-for="(doc, index) in documents"
                :key="doc.id || doc.title"
                :to="documentRoute(doc)"
                target="_blank"
                rel="noopener noreferrer"
                class="cp-agreement-doc group flex min-h-[72px] w-full items-center gap-3 rounded-xl px-4 py-3 text-left"
              >
                <span class="cp-agreement-doc-icon flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-lg">
                  <Icon :name="documentIcon(index, doc.title)" size="sm" />
                </span>
                <span class="min-w-0 flex-1">
                  <span class="cp-agreement-doc-title block truncate text-sm font-semibold">
                    {{ doc.title }}
                  </span>
                </span>
                <span class="cp-agreement-doc-arrow flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-full">
                  <Icon name="externalLink" size="sm" />
                </span>
              </RouterLink>
            </div>
          </div>

          <div class="cp-agreement-foot px-6 py-4">
            <div class="grid grid-cols-2 gap-3">
              <button
                type="button"
                class="cp-agreement-reject rounded-xl px-4 py-3 text-sm font-semibold"
                @click="emit('reject')"
              >
                {{ t('legal.loginAgreementPrompt.reject') }}
              </button>
              <button
                type="button"
                class="cp-agreement-accept rounded-xl px-4 py-3 text-sm font-semibold"
                @click="emit('accept')"
              >
                {{ t('legal.loginAgreementPrompt.accept') }}
              </button>
            </div>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { LoginAgreementDocument } from '@/types'

const { t } = useI18n()

const props = withDefaults(defineProps<{
  accepted: boolean
  documents: LoginAgreementDocument[]
  mode: 'modal' | 'checkbox' | string
  updatedAt?: string
  visible: boolean
}>(), {
  updatedAt: ''
})

const emit = defineEmits<{
  accept: []
  reject: []
  open: []
}>()

const dialogVisible = computed(() => props.visible && documents.value.length > 0)
const documents = computed(() => props.documents.filter((doc) => doc.title.trim()))
const updatedAt = computed(() => props.updatedAt || '')
const accepted = computed(() => props.accepted)
const mode = computed(() => props.mode === 'checkbox' ? 'checkbox' : 'modal')

function documentRoute(doc: LoginAgreementDocument) {
  return {
    name: 'LegalDocument',
    params: {
      documentId: doc.id || doc.title,
    },
  }
}

function handleCheckboxChange(event: Event): void {
  const checked = (event.target as HTMLInputElement).checked
  if (checked) {
    emit('accept')
  } else {
    emit('reject')
  }
}

function documentIcon(index: number, title: string): 'document' | 'shield' | 'globe' | 'cog' {
  const normalizedTitle = title.toLowerCase()
  if (
    normalizedTitle.includes('policy') ||
    normalizedTitle.includes('privacy') ||
    title.includes('政策') ||
    title.includes('隐私')
  ) {
    return 'shield'
  }
  if (
    normalizedTitle.includes('country') ||
    normalizedTitle.includes('region') ||
    title.includes('国家') ||
    title.includes('地区')
  ) {
    return 'globe'
  }
  if (index === 3) {
    return 'cog'
  }
  return 'document'
}
</script>

<style scoped>
.agreement-fade-enter-active,
.agreement-fade-leave-active {
  transition: opacity 0.18s ease;
}

.agreement-fade-enter-from,
.agreement-fade-leave-to {
  opacity: 0;
}

.agreement-fade-enter-active > div,
.agreement-fade-leave-active > div {
  transition: transform 0.18s ease, opacity 0.18s ease;
}

.agreement-fade-enter-from > div,
.agreement-fade-leave-to > div {
  opacity: 0;
  transform: translateY(8px) scale(0.98);
}

/* ---------- 协议确认：配色统一走全局语义 token ---------- */
.cp-consent-checkbox {
  border-color: var(--border-strong);
  color: var(--brand);
  accent-color: var(--brand);
}

.cp-consent-text {
  color: var(--muted);
}

.cp-consent-text label {
  color: var(--text);
}

.cp-consent-link {
  color: var(--brand);
}

.cp-consent-link:hover {
  text-decoration: underline;
}

.cp-consent-notice {
  border: 1px solid color-mix(in srgb, var(--brand) 26%, var(--border));
  background: color-mix(in srgb, var(--brand) 8%, transparent);
  color: var(--text);
}

.cp-consent-notice-icon,
.cp-consent-notice-title {
  color: var(--brand);
}

.cp-consent-notice-desc {
  color: var(--muted);
}

.cp-consent-notice-btn {
  background: var(--brand);
  color: #fff;
}

:global(.dark) .cp-consent-notice-btn {
  color: #042f2e;
}

.cp-consent-notice-btn:hover {
  filter: brightness(1.06);
}

.cp-agreement-overlay {
  background: color-mix(in srgb, #020617 62%, transparent);
  backdrop-filter: blur(4px);
}

.cp-agreement-panel {
  border: 1px solid var(--border);
  background: var(--surface);
  box-shadow: var(--shadow-float);
  color: var(--text);
}

.cp-agreement-head {
  border-bottom: 1px solid var(--border);
}

.cp-agreement-badge {
  background: color-mix(in srgb, var(--brand) 13%, transparent);
  color: var(--brand);
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--brand) 20%, transparent);
}

.cp-agreement-title {
  color: var(--text);
}

.cp-agreement-chip {
  background: var(--surface-3);
  color: var(--muted);
}

.cp-agreement-desc {
  color: var(--muted);
}

.cp-agreement-body-title {
  color: var(--text);
}

.cp-agreement-doc {
  border: 1px solid var(--border);
  background: var(--surface-3);
  transition: border-color 0.2s ease, background 0.2s ease, transform 0.2s ease;
}

.cp-agreement-doc:hover {
  border-color: color-mix(in srgb, var(--brand) 38%, var(--border));
  background: color-mix(in srgb, var(--brand) 7%, var(--surface));
  transform: translateY(-1px);
}

.cp-agreement-doc-icon {
  background: var(--surface);
  color: var(--muted);
  box-shadow: inset 0 0 0 1px var(--border);
}

.cp-agreement-doc:hover .cp-agreement-doc-icon {
  color: var(--brand);
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--brand) 26%, transparent);
}

.cp-agreement-doc-title {
  color: var(--text);
}

.cp-agreement-doc-arrow {
  color: var(--muted);
}

.cp-agreement-doc:hover .cp-agreement-doc-arrow {
  color: var(--brand);
}

.cp-agreement-foot {
  border-top: 1px solid var(--border);
  background: color-mix(in srgb, var(--surface-3) 70%, transparent);
}

.cp-agreement-reject {
  border: 1px solid var(--border);
  background: transparent;
  color: var(--text);
}

.cp-agreement-reject:hover {
  border-color: color-mix(in srgb, var(--brand) 38%, var(--border));
  background: color-mix(in srgb, var(--brand) 8%, transparent);
}

.cp-agreement-accept {
  border: 0;
  background: linear-gradient(135deg, var(--brand-soft), var(--brand) 55%);
  color: #fff;
  box-shadow: 0 10px 26px -14px color-mix(in srgb, var(--brand) 80%, transparent);
}

:global(.dark) .cp-agreement-accept {
  color: #042f2e;
}

.cp-agreement-accept:hover {
  filter: brightness(1.04);
}

@media (prefers-reduced-motion: reduce) {
  .cp-agreement-doc:hover {
    transform: none;
  }
}
</style>
