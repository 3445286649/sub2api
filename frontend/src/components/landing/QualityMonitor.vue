<template>
  <section v-if="visible" id="quality" class="qm-root">
    <header class="qm-head">
      <p class="qm-eyebrow">
        <span class="qm-live" :class="{ 'is-stale': !snapshot || snapshot.stale }"></span>
        {{ snapshot && !snapshot.stale ? t('home.quality.live') : t('home.nav.quality') }}
      </p>
      <h2 class="qm-title">{{ t('home.quality.title') }}</h2>
      <p class="qm-sub">{{ t('home.quality.subtitle') }}</p>
    </header>

    <!-- 实时数据 -->
    <div v-if="snapshot" v-reveal class="qm-board">
      <div class="qm-board-head">
        <div class="qm-board-title">
          <span class="qm-model">{{ snapshot.model }}</span>
          <span v-if="snapshot.group_name" class="qm-group">{{ snapshot.group_name }}</span>
        </div>
        <span class="qm-interval">{{ intervalLabel }}</span>
      </div>

      <div v-for="test in snapshot.tests" :key="test.key" class="qm-row">
        <div class="qm-row-label">
          <span class="qm-row-name">{{ testName(test.key) }}</span>
          <span class="qm-row-rate">
            {{ formatRate(test.pass_rate) }}
            <small>{{ t('home.quality.passRate') }}</small>
          </span>
        </div>
        <div class="qm-bar" role="img" :aria-label="barLabel(test)">
          <i
            v-for="(point, index) in test.history"
            :key="`${test.key}-${index}`"
            :class="`is-${point.s}`"
            :title="pointTitle(point)"
          ></i>
        </div>
      </div>

      <div class="qm-legend">
        <span><i class="is-passed"></i>{{ t('home.quality.legend.passed') }}</span>
        <span><i class="is-failed"></i>{{ t('home.quality.legend.failed') }}</span>
        <span><i class="is-error"></i>{{ t('home.quality.legend.error') }}</span>
      </div>

      <div class="qm-board-foot">
        <span class="qm-updated">
          {{ snapshot.stale ? t('home.quality.staleAt', { time: relativeUpdated }) : t('home.quality.updatedAt', { time: relativeUpdated }) }}
        </span>
        <a :href="snapshot.detail_url" target="_blank" rel="noopener noreferrer" class="qm-link">
          {{ t('home.quality.viewAll') }}
          <Icon name="externalLink" size="xs" />
        </a>
      </div>
    </div>

    <!-- 暂无数据：只给静态说明，不展示任何数字 -->
    <div v-else-if="state === 'unavailable'" v-reveal class="qm-board qm-board-empty">
      <p>{{ t('home.quality.unavailable') }}</p>
      <a v-if="detailUrl" :href="detailUrl" target="_blank" rel="noopener noreferrer" class="qm-link">
        {{ t('home.quality.viewAll') }}
        <Icon name="externalLink" size="xs" />
      </a>
    </div>

    <div v-else class="qm-board qm-board-loading" aria-hidden="true">
      <span v-for="n in 2" :key="n" class="qm-skeleton"></span>
    </div>

    <!-- 三种检测方式 -->
    <div class="qm-methods">
      <article v-for="(method, index) in methods" :key="method.key" v-reveal="index" class="qm-method">
        <span class="qm-method-icon"><Icon :name="method.icon" size="md" /></span>
        <h3>{{ method.title }}</h3>
        <p>{{ method.desc }}</p>
        <a
          v-if="method.key === 'self' && detailUrl"
          :href="detailUrl"
          target="_blank"
          rel="noopener noreferrer"
          class="qm-method-link"
        >
          {{ t('home.quality.methods.self.action') }}
          <Icon name="arrowRight" size="xs" />
        </a>
      </article>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { vReveal } from '@/directives/reveal'
import {
  getQualityMonitor,
  type QualityMonitorPoint,
  type QualityMonitorSnapshot,
  type QualityMonitorTest,
} from '@/api/qualityMonitor'

type IconName = InstanceType<typeof Icon>['$props']['name']

/** 加载完成后告知父级是否启用，父级据此决定 Hero 标签、对比行与顶栏锚点是否出现 */
const emit = defineEmits<{ (event: 'loaded', enabled: boolean): void }>()

const { t, locale } = useI18n()

/**
 * loading：尚未请求或请求中；ready：有数据；unavailable：已配置但暂无数据；
 * disabled：后台未配置（整章不渲染）。
 */
const state = ref<'loading' | 'ready' | 'unavailable' | 'disabled'>('loading')
const snapshot = ref<QualityMonitorSnapshot | null>(null)
const fallbackDetailUrl = ref('')
const now = ref(Date.now())

const visible = computed(() => state.value !== 'disabled')
const detailUrl = computed(() => snapshot.value?.detail_url || fallbackDetailUrl.value)

const methods = computed<Array<{ key: string; icon: IconName; title: string; desc: string }>>(() => [
  {
    key: 'logic',
    icon: 'lightbulb',
    title: t('home.quality.methods.logic.title'),
    desc: t('home.quality.methods.logic.desc'),
  },
  {
    key: 'creative',
    icon: 'sparkles',
    title: t('home.quality.methods.creative.title'),
    desc: t('home.quality.methods.creative.desc'),
  },
  {
    key: 'self',
    icon: 'beaker',
    title: t('home.quality.methods.self.title'),
    desc: t('home.quality.methods.self.desc'),
  },
])

const intervalLabel = computed(() => {
  const minutes = Math.max(1, Math.round((snapshot.value?.interval_seconds ?? 1200) / 60))
  return t('home.quality.interval', { minutes })
})

const relativeUpdated = computed(() => {
  const updated = snapshot.value?.updated_at ? Date.parse(snapshot.value.updated_at) : NaN
  if (!Number.isFinite(updated)) return '—'
  const minutes = Math.max(0, Math.floor((now.value - updated) / 60000))
  if (minutes < 1) return t('home.quality.justNow')
  if (minutes < 60) return t('home.quality.minutesAgo', { n: minutes })
  return t('home.quality.hoursAgo', { n: Math.floor(minutes / 60) })
})

function testName(key: string): string {
  return key === 'creative' ? t('home.quality.tests.creative') : t('home.quality.tests.logic')
}

function formatRate(rate: number | null): string {
  if (rate === null || !Number.isFinite(rate)) return '—'
  // 不四舍五入到 100%：99.6% 就显示 99.6%，避免「看起来全过」的误导
  const pct = Math.floor(rate * 1000) / 10
  return `${pct % 1 === 0 ? pct.toFixed(0) : pct.toFixed(1)}%`
}

function statusLabel(status: string): string {
  if (status === 'passed') return t('home.quality.legend.passed')
  if (status === 'failed') return t('home.quality.legend.failed')
  return t('home.quality.legend.error')
}

function pointTitle(point: QualityMonitorPoint): string {
  const time = new Date(point.t).toLocaleString(locale.value, {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
  return `${time} · ${statusLabel(point.s)}`
}

function barLabel(test: QualityMonitorTest): string {
  return t('home.quality.barLabel', {
    name: testName(test.key),
    passed: test.passed,
    failed: test.failed,
    errored: test.errored,
  })
}

async function load(): Promise<void> {
  try {
    const res = await getQualityMonitor()
    if (!res.enabled) {
      state.value = 'disabled'
    } else if (res.available && res.data) {
      snapshot.value = res.data
      state.value = 'ready'
    } else {
      fallbackDetailUrl.value = res.detail_url ?? ''
      state.value = 'unavailable'
    }
  } catch {
    // 接口本身失败时同样整章隐藏：不确定是否配置过，宁可不显示
    state.value = 'disabled'
  }
  emit('loaded', visible.value)
}

// 挂载即请求（不阻塞渲染；后端有 5 分钟缓存）。父级需要尽早知道是否启用。
let tick: number | undefined

onMounted(() => {
  tick = window.setInterval(() => {
    now.value = Date.now()
  }, 60_000)
  void load()
})

onUnmounted(() => {
  if (tick) window.clearInterval(tick)
})
</script>

<style scoped>
.qm-root {
  scroll-snap-align: start;
}

.qm-head {
  margin-bottom: 30px;
  text-align: center;
}

.qm-eyebrow {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  margin: 0 0 10px;
  color: var(--accent-ok);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.14em;
  text-transform: uppercase;
}

.qm-live {
  position: relative;
  width: 7px;
  height: 7px;
  border-radius: 999px;
  background: var(--accent-ok);
}

.qm-live::after {
  content: '';
  position: absolute;
  inset: 0;
  border-radius: inherit;
  background: inherit;
  animation: qm-ping 2.2s cubic-bezier(0, 0, 0.2, 1) infinite;
}

.qm-live.is-stale {
  background: var(--muted);
}

.qm-live.is-stale::after {
  animation: none;
}

.qm-title {
  margin: 0;
  color: var(--text);
  font-size: clamp(24px, 3vw, 34px);
  font-weight: 750;
  letter-spacing: -0.025em;
}

.qm-sub {
  max-width: 560px;
  margin: 10px auto 0;
  color: var(--muted);
  font-size: 14.5px;
  line-height: 1.7;
}

/* ---------- 监测面板 ---------- */
.qm-board {
  max-width: 880px;
  margin: 0 auto;
  padding: 22px 24px;
  border: 1px solid var(--border);
  border-radius: 18px;
  background: var(--surface);
  box-shadow: var(--shadow-float);
}

.qm-board-head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 8px 16px;
  margin-bottom: 18px;
}

.qm-board-title {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}

.qm-model {
  color: var(--text);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 14px;
  font-weight: 700;
}

.qm-group {
  padding: 2px 9px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--accent-ok) 13%, transparent);
  color: var(--accent-ok);
  font-size: 12px;
  font-weight: 650;
}

.qm-interval {
  color: var(--muted);
  font-size: 12px;
}

.qm-row + .qm-row {
  margin-top: 16px;
}

.qm-row-label {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 8px;
}

.qm-row-name {
  color: var(--text);
  font-size: 13.5px;
  font-weight: 650;
}

.qm-row-rate {
  color: var(--accent-ok);
  font-size: 15px;
  font-weight: 750;
  font-variant-numeric: tabular-nums;
}

.qm-row-rate small {
  margin-left: 4px;
  color: var(--muted);
  font-size: 11.5px;
  font-weight: 500;
}

/* 状态页式色块条：一格一次检测，最新的在右边 */
.qm-bar {
  display: flex;
  gap: 3px;
  height: 30px;
}

.qm-bar i {
  flex: 1 1 0;
  min-width: 3px;
  border-radius: 3px;
  transition: transform 0.15s ease, opacity 0.15s ease;
}

.qm-bar i:hover {
  transform: scaleY(1.12);
}

.qm-bar i.is-passed,
.qm-legend i.is-passed {
  background: var(--accent-ok);
}

.qm-bar i.is-failed,
.qm-legend i.is-failed {
  background: var(--accent-danger);
}

.qm-bar i.is-error,
.qm-legend i.is-error {
  background: color-mix(in srgb, var(--muted) 40%, transparent);
}

.qm-legend {
  display: flex;
  flex-wrap: wrap;
  gap: 6px 16px;
  margin-top: 14px;
  color: var(--muted);
  font-size: 12px;
}

.qm-legend span {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.qm-legend i {
  width: 9px;
  height: 9px;
  border-radius: 2px;
}

.qm-board-foot {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 8px 16px;
  margin-top: 16px;
  padding-top: 14px;
  border-top: 1px solid var(--border);
}

.qm-updated {
  color: var(--muted);
  font-size: 12.5px;
}

.qm-link,
.qm-method-link {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  color: var(--brand);
  font-size: 13px;
  font-weight: 650;
}

.qm-link:hover,
.qm-method-link:hover {
  text-decoration: underline;
  text-underline-offset: 3px;
}

.qm-board-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
}

.qm-board-empty p {
  margin: 0;
  color: var(--muted);
  font-size: 13.5px;
  text-align: center;
}

.qm-board-loading {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.qm-skeleton {
  height: 52px;
  border-radius: 10px;
  background: var(--surface-3);
  animation: qm-shimmer 1.4s ease-in-out infinite;
}

/* ---------- 三种检测方式 ---------- */
.qm-methods {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 14px;
  max-width: 880px;
  margin: 18px auto 0;
}

.qm-method {
  padding: 18px;
  border: 1px solid var(--border);
  border-radius: 14px;
  background: color-mix(in srgb, var(--surface) 70%, transparent);
}

.qm-method-icon {
  display: inline-grid;
  width: 36px;
  height: 36px;
  margin-bottom: 12px;
  place-items: center;
  border-radius: 10px;
  background: color-mix(in srgb, var(--accent-ok) 13%, transparent);
  color: var(--accent-ok);
}

.qm-method h3 {
  margin: 0 0 6px;
  color: var(--text);
  font-size: 14.5px;
  font-weight: 700;
}

.qm-method p {
  margin: 0;
  color: var(--muted);
  font-size: 12.5px;
  line-height: 1.65;
}

.qm-method-link {
  margin-top: 10px;
}

@keyframes qm-ping {
  0% { opacity: 0.8; transform: scale(1); }
  75%, 100% { opacity: 0; transform: scale(3.2); }
}

@keyframes qm-shimmer {
  50% { opacity: 0.5; }
}

@media (max-width: 767px) {
  .qm-board {
    padding: 18px 16px;
  }

  .qm-bar {
    gap: 2px;
    height: 26px;
  }

  .qm-methods {
    grid-template-columns: 1fr;
  }
}

@media (prefers-reduced-motion: reduce) {
  .qm-live::after,
  .qm-skeleton {
    animation: none;
  }
}
</style>
