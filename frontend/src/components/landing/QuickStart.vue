<template>
  <section id="start" class="qs-root">
    <header class="qs-head">
      <p class="qs-eyebrow">{{ t('home.nav.start') }}</p>
      <h2 class="qs-title">{{ t('home.start.title') }}</h2>
      <p class="qs-sub">{{ t('home.start.subtitle') }}</p>
    </header>

    <!-- 三步上手 -->
    <ol class="qs-steps">
      <li v-for="(step, index) in steps" :key="step.key" v-reveal="index" class="qs-step">
        <span class="qs-step-num">{{ index + 1 }}</span>
        <span class="qs-step-text">
          <span class="qs-step-title">{{ step.title }}</span>
          <span class="qs-step-desc">{{ step.desc }}</span>
        </span>
      </li>
    </ol>

    <!-- 可复制的接入配置：地址取自站点设置，用户复制即可用 -->
    <div v-reveal class="qs-code">
      <div class="qs-tabs" role="tablist">
        <button
          v-for="snippet in snippets"
          :key="snippet.key"
          type="button"
          role="tab"
          class="qs-tab"
          :class="{ 'is-active': snippet.key === activeKey }"
          :aria-selected="snippet.key === activeKey"
          @click="activeKey = snippet.key"
        >
          {{ snippet.label }}
        </button>
        <button type="button" class="qs-copy" :class="{ 'is-copied': copied }" @click="copyActive">
          <Icon :name="copied ? 'check' : 'copy'" size="xs" />
          {{ copied ? t('home.start.copied') : t('home.start.copy') }}
        </button>
      </div>
      <div class="qs-file">{{ activeSnippet.file }}</div>
      <pre class="qs-pre"><code>{{ activeSnippet.code }}</code></pre>
    </div>

    <div class="qs-actions">
      <router-link :to="primaryTo" class="qs-btn qs-btn-primary">
        {{ primaryLabel }}
        <Icon name="arrowRight" size="sm" :stroke-width="2" />
      </router-link>
      <a
        v-if="docUrl"
        :href="docUrl"
        target="_blank"
        rel="noopener noreferrer"
        class="qs-btn qs-btn-ghost"
      >
        {{ t('home.viewDocs') }}
        <Icon name="externalLink" size="sm" />
      </a>
      <router-link v-else to="/key-usage" class="qs-btn qs-btn-ghost">
        {{ t('home.start.usage') }}
      </router-link>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useClipboard } from '@/composables/useClipboard'
import { vReveal } from '@/directives/reveal'

const props = defineProps<{
  /** 站点设置里的 API 地址，未配置时用当前域名 */
  apiBaseUrl: string
  docUrl: string
  primaryTo: string
  primaryLabel: string
}>()

const { t } = useI18n()
const { copied, copyToClipboard } = useClipboard()

const steps = computed(() => [
  { key: 'register', title: t('home.start.steps.register.title'), desc: t('home.start.steps.register.desc') },
  { key: 'key', title: t('home.start.steps.key.title'), desc: t('home.start.steps.key.desc') },
  { key: 'baseUrl', title: t('home.start.steps.baseUrl.title'), desc: t('home.start.steps.baseUrl.desc') },
])

/**
 * 与 UseKeyModal 的生成规则保持一致：
 * Anthropic 系客户端自己拼 /v1/messages，给根地址；OpenAI 系给 /v1。
 */
const baseRoot = computed(() => {
  const raw = props.apiBaseUrl || (typeof window !== 'undefined' ? window.location.origin : '')
  return raw.replace(/\/v1\/?$/, '').replace(/\/+$/, '')
})
const apiBase = computed(() => `${baseRoot.value}/v1`)

const snippets = computed(() => {
  const key = t('home.start.keyPlaceholder')
  return [
    {
      key: 'claude',
      label: 'Claude Code',
      file: 'Terminal',
      code: `export ANTHROPIC_BASE_URL="${baseRoot.value}"
export ANTHROPIC_AUTH_TOKEN="${key}"
claude`,
    },
    {
      key: 'codex',
      label: 'Codex',
      file: '~/.codex/config.toml',
      code: `model_provider = "OpenAI"

[model_providers.OpenAI]
name = "OpenAI"
base_url = "${apiBase.value}"
wire_api = "responses"
requires_openai_auth = true

# ~/.codex/auth.json
# { "OPENAI_API_KEY": "${key}" }`,
    },
    {
      key: 'sdk',
      label: 'OpenAI SDK',
      file: 'main.py',
      code: `from openai import OpenAI

client = OpenAI(
    base_url="${apiBase.value}",
    api_key="${key}",
)

resp = client.chat.completions.create(
    model="claude-sonnet-4-5",
    messages=[{"role": "user", "content": "Hello"}],
)`,
    },
    {
      key: 'curl',
      label: 'cURL',
      file: 'Terminal',
      code: `curl ${apiBase.value}/messages \\
  -H "x-api-key: ${key}" \\
  -H "anthropic-version: 2023-06-01" \\
  -H "content-type: application/json" \\
  -d '{"model":"claude-sonnet-4-5","max_tokens":256,
       "messages":[{"role":"user","content":"Hello"}]}'`,
    },
  ]
})

const activeKey = ref('claude')
const activeSnippet = computed(
  () => snippets.value.find((snippet) => snippet.key === activeKey.value) ?? snippets.value[0],
)

function copyActive(): void {
  void copyToClipboard(activeSnippet.value.code)
}
</script>

<style scoped>
.qs-root {
  width: 100%;
  max-width: 860px;
  margin: 0 auto;
}

.qs-head {
  margin-bottom: 28px;
  text-align: center;
}

.qs-eyebrow {
  margin: 0 0 10px;
  color: var(--brand);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.14em;
  text-transform: uppercase;
}

.qs-title {
  margin: 0;
  color: var(--text);
  font-size: clamp(24px, 3vw, 34px);
  font-weight: 750;
  letter-spacing: -0.025em;
}

.qs-sub {
  margin: 10px 0 0;
  color: var(--muted);
  font-size: 14.5px;
}

/* ---------- 三步 ---------- */
.qs-steps {
  position: relative;
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 14px;
  margin: 0 0 20px;
  padding: 0;
  list-style: none;
}

.qs-step {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px 16px;
  border: 1px solid var(--border);
  border-radius: 14px;
  background: var(--surface);
}

.qs-step-num {
  display: grid;
  flex: 0 0 30px;
  height: 30px;
  place-items: center;
  border-radius: 999px;
  background: color-mix(in srgb, var(--brand) 14%, transparent);
  color: var(--brand);
  font-size: 13px;
  font-weight: 750;
}

.qs-step-text {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 2px;
}

.qs-step-title {
  color: var(--text);
  font-size: 14px;
  font-weight: 700;
}

.qs-step-desc {
  color: var(--muted);
  font-size: 12px;
}

/* ---------- 代码块 ---------- */
.qs-code {
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: 16px;
  background: var(--surface);
  box-shadow: var(--shadow-float);
}

.qs-tabs {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 8px;
  overflow-x: auto;
  border-bottom: 1px solid var(--border);
  background: var(--surface-2);
}

.qs-tab {
  flex: 0 0 auto;
  padding: 6px 12px;
  border: 1px solid transparent;
  border-radius: 9px;
  background: transparent;
  color: var(--muted);
  font-size: 12.5px;
  font-weight: 650;
  cursor: pointer;
  transition: color 0.2s ease, background 0.2s ease, border-color 0.2s ease;
}

.qs-tab:hover {
  color: var(--text);
}

.qs-tab.is-active {
  border-color: var(--border);
  background: var(--surface);
  color: var(--text);
}

.qs-tab:focus-visible,
.qs-copy:focus-visible {
  outline: 2px solid var(--brand);
  outline-offset: 1px;
}

.qs-copy {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 5px;
  margin-left: auto;
  padding: 6px 11px;
  border: 1px solid var(--border);
  border-radius: 9px;
  background: var(--surface);
  color: var(--muted);
  font-size: 12px;
  font-weight: 650;
  cursor: pointer;
  transition: color 0.2s ease, border-color 0.2s ease;
}

.qs-copy:hover {
  border-color: color-mix(in srgb, var(--brand) 40%, var(--border));
  color: var(--brand);
}

.qs-copy.is-copied {
  border-color: color-mix(in srgb, var(--accent-ok) 45%, transparent);
  color: var(--accent-ok);
}

.qs-file {
  padding: 10px 18px 0;
  color: var(--muted);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 11px;
}

.qs-pre {
  margin: 0;
  padding: 10px 18px 18px;
  overflow-x: auto;
  color: var(--text);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12.5px;
  line-height: 1.75;
  white-space: pre;
}

/* ---------- 按钮 ---------- */
.qs-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 12px;
  margin-top: 26px;
}

.qs-btn {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-height: 44px;
  padding: 11px 24px;
  border-radius: 12px;
  font-size: 14.5px;
  font-weight: 650;
  transition: transform 0.22s ease, border-color 0.22s ease, box-shadow 0.28s ease;
}

.qs-btn:hover {
  transform: translateY(-2px);
}

.qs-btn-primary {
  background: linear-gradient(135deg, var(--brand-soft), var(--brand));
  color: #042f2e;
  box-shadow: 0 12px 28px -12px color-mix(in srgb, var(--brand) 60%, transparent);
}

.qs-btn-ghost {
  border: 1px solid var(--border-strong);
  background: var(--surface);
  color: var(--text);
}

.qs-btn-ghost:hover {
  border-color: color-mix(in srgb, var(--brand) 45%, var(--border));
}

@media (max-width: 767px) {
  .qs-steps {
    grid-template-columns: 1fr;
    gap: 8px;
  }

  .qs-step {
    padding: 11px 14px;
  }

  .qs-pre {
    font-size: 11.5px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .qs-btn:hover {
    transform: none;
  }
}
</style>
