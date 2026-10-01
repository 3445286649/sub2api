<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { getQualityMonitor } from '@/api/qualityMonitor'
import type { QualityMonitorGroup, QualityStatus } from '@/api/qualityMonitor'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

const emit = defineEmits<{
  /** 监测章节是否可展示；HomeView 用此决定导航锚点与对比表是否显示质量行 */
  loaded: [enabled: boolean]
}>()

const enabled = ref(false)
const available = ref(false)
const stale = ref(false)
const groups = ref<QualityMonitorGroup[]>([])
const detailURL = ref('')
const error = ref(false)

const MAX_HISTORY = 60

onMounted(async () => {
  try {
    const resp = await getQualityMonitor()
    enabled.value = resp.enabled
    emit('loaded', resp.enabled)
    if (!resp.enabled) return

    available.value = resp.available
    if (resp.available && resp.data) {
      stale.value = resp.data.stale
      groups.value = resp.data.groups
      detailURL.value = resp.data.detail_url
    } else if (resp.detail_url) {
      detailURL.value = resp.detail_url
    }
  } catch {
    error.value = true
    emit('loaded', false)
  }
})

function statusLabel(s: QualityStatus): string {
  const map: Record<QualityStatus, string> = {
    passed: t('home.quality.status.passed'),
    failed: t('home.quality.status.failed'),
    error: t('home.quality.status.error'),
  }
  return map[s] || s
}

/** 通过率百分比文本，如「98.2%」；无有效结果时返回「—」 */
function passRateText(rate: number | null): string {
  return rate != null ? `${rate.toFixed(1)}%` : '—'
}

/** 分组的总检测次数 */
function totalCount(g: QualityMonitorGroup): number {
  return g.passed + g.failed + g.errored
}

/** 补齐空槽到 60 格；前端不关心上游的 records 真实长度，只显示最多 60 个 */
const paddedHistory = computed(() => {
  return groups.value.map((g) => {
    const slots = [...g.history]
    while (slots.length < MAX_HISTORY) {
      slots.unshift({ s: 'error', t: '' }) // 空槽用 error 渲染为灰色
    }
    return { ...g, history: slots }
  })
})
</script>

<template>
  <section v-if="enabled" id="quality" class="cp-chapter">
    <header class="cp-chapter-head">
      <h2 class="cp-chapter-title">{{ t('home.quality.title') }}</h2>
      <p class="cp-chapter-lead">{{ t('home.quality.lead') }}</p>
    </header>

    <div v-if="error" class="qm-error">
      {{ t('home.quality.error') }}
    </div>

    <div v-else-if="!available" class="qm-empty">
      <p>{{ t('home.quality.empty') }}</p>
      <a v-if="detailURL" :href="detailURL" target="_blank" rel="noopener" class="qm-link">
        {{ t('home.quality.visit') }} →
      </a>
    </div>

    <div v-else class="qm-grid">
      <article v-for="group in paddedHistory" :key="group.id" v-reveal class="qm-card">
        <header class="qm-card-head">
          <h3 class="qm-card-title">{{ group.name }}</h3>
          <div class="qm-card-meta">
            <span class="qm-model">{{ group.model }}</span>
            <span v-if="group.reasoning_effort" class="qm-effort">
              {{ t('home.quality.reasoning') }}: {{ group.reasoning_effort }}
            </span>
          </div>
        </header>

        <div class="qm-stats">
          <div class="qm-stat">
            <span class="qm-stat-label">{{ t('home.quality.passRate') }}</span>
            <span class="qm-stat-value qm-rate">{{ passRateText(group.pass_rate) }}</span>
          </div>
          <div class="qm-stat">
            <span class="qm-stat-label">{{ t('home.quality.total') }}</span>
            <span class="qm-stat-value">{{ totalCount(group) }}</span>
          </div>
          <div class="qm-stat qm-stat-passed">
            <span class="qm-stat-label">{{ t('home.quality.passed') }}</span>
            <span class="qm-stat-value">{{ group.passed }}</span>
          </div>
          <div class="qm-stat qm-stat-failed">
            <span class="qm-stat-label">{{ t('home.quality.failed') }}</span>
            <span class="qm-stat-value">{{ group.failed }}</span>
          </div>
          <div class="qm-stat qm-stat-error">
            <span class="qm-stat-label">{{ t('home.quality.errored') }}</span>
            <span class="qm-stat-value">{{ group.errored }}</span>
          </div>
        </div>

        <div class="qm-timeline">
          <div
            v-for="(point, idx) in group.history"
            :key="idx"
            :class="['qm-dot', `qm-dot-${point.s}`, { 'qm-dot-empty': !point.t }]"
            :title="point.t ? `${statusLabel(point.s)} · ${new Date(point.t).toLocaleString()}` : ''"
          ></div>
        </div>

        <footer v-if="group.latest_status" class="qm-card-footer">
          <span>{{ t('home.quality.latest') }}:</span>
          <span :class="['qm-status', `qm-status-${group.latest_status}`]">
            {{ statusLabel(group.latest_status) }}
          </span>
          <span v-if="group.next_run_at" class="qm-next">
            {{ t('home.quality.next') }}:
            {{ new Date(group.next_run_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) }}
          </span>
        </footer>
      </article>

      <footer v-if="stale" class="qm-stale">
        {{ t('home.quality.stale') }}
      </footer>

      <a v-if="detailURL" :href="detailURL" target="_blank" rel="noopener" class="qm-detail-link">
        {{ t('home.quality.detail') }} →
      </a>
    </div>
  </section>
</template>

<style scoped>
.qm-error,
.qm-empty {
  padding: clamp(2rem, 4vw, 3rem);
  text-align: center;
  color: var(--text-secondary);
  background: var(--surface-2);
  border-radius: 16px;
}

.qm-link,
.qm-detail-link {
  display: inline-block;
  margin-top: 1rem;
  color: var(--accent-brand);
  text-decoration: none;
  font-weight: 500;
}

.qm-link:hover,
.qm-detail-link:hover {
  text-decoration: underline;
}

.qm-grid {
  display: grid;
  gap: clamp(1.5rem, 3vw, 2rem);
  grid-template-columns: repeat(auto-fit, minmax(min(100%, 28rem), 1fr));
}

.qm-card {
  padding: clamp(1.5rem, 3vw, 2rem);
  background: var(--surface-2);
  border-radius: 16px;
  display: grid;
  gap: 1.5rem;
}

.qm-card-head {
  display: grid;
  gap: 0.5rem;
}

.qm-card-title {
  font-size: clamp(1.125rem, 2vw, 1.25rem);
  font-weight: 600;
  color: var(--text-primary);
  margin: 0;
}

.qm-card-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
  font-size: 0.875rem;
  color: var(--text-secondary);
}

.qm-model {
  font-family: var(--font-mono);
  padding: 0.25rem 0.625rem;
  background: var(--surface-3);
  border-radius: 6px;
}

.qm-effort {
  opacity: 0.8;
}

.qm-stats {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(5rem, 1fr));
  gap: 1rem;
}

.qm-stat {
  display: grid;
  gap: 0.25rem;
}

.qm-stat-label {
  font-size: 0.8125rem;
  color: var(--text-secondary);
  text-transform: uppercase;
  letter-spacing: 0.03em;
}

.qm-stat-value {
  font-size: clamp(1.25rem, 2.5vw, 1.5rem);
  font-weight: 600;
  color: var(--text-primary);
  font-variant-numeric: tabular-nums;
}

.qm-rate {
  color: var(--accent-brand);
}

.qm-stat-passed .qm-stat-value {
  color: var(--accent-ok);
}

.qm-stat-failed .qm-stat-value {
  color: var(--accent-danger);
}

.qm-stat-error .qm-stat-value {
  color: var(--text-tertiary);
}

.qm-timeline {
  display: grid;
  grid-template-columns: repeat(60, 1fr);
  gap: 2px;
  padding: 0.75rem;
  background: var(--surface-3);
  border-radius: 8px;
}

.qm-dot {
  aspect-ratio: 1;
  border-radius: 2px;
  transition: transform 0.15s ease;
}

.qm-dot:not(.qm-dot-empty):hover {
  transform: scale(1.4);
  z-index: 1;
}

.qm-dot-passed {
  background: var(--accent-ok);
}

.qm-dot-failed {
  background: var(--accent-danger);
}

.qm-dot-error,
.qm-dot-empty {
  background: color-mix(in srgb, var(--text-tertiary) 20%, transparent);
}

.qm-card-footer {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.875rem;
  color: var(--text-secondary);
  padding-top: 0.5rem;
  border-top: 1px solid var(--surface-3);
}

.qm-status {
  font-weight: 500;
  padding: 0.25rem 0.625rem;
  border-radius: 6px;
}

.qm-status-passed {
  background: color-mix(in srgb, var(--accent-ok) 14%, transparent);
  color: var(--accent-ok);
}

.qm-status-failed {
  background: color-mix(in srgb, var(--accent-danger) 14%, transparent);
  color: var(--accent-danger);
}

.qm-status-error {
  background: var(--surface-3);
  color: var(--text-tertiary);
}

.qm-next {
  margin-left: auto;
  font-variant-numeric: tabular-nums;
}

.qm-stale {
  grid-column: 1 / -1;
  padding: 1rem;
  text-align: center;
  font-size: 0.875rem;
  color: var(--accent-warn);
  background: color-mix(in srgb, var(--accent-warn) 10%, transparent);
  border-radius: 8px;
}

.qm-detail-link {
  grid-column: 1 / -1;
  text-align: center;
  padding: 1rem;
  font-weight: 500;
}
</style>

