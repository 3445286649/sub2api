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

    <div v-if="snapshot" v-reveal class="qm-board" :class="{ 'is-stale': snapshot.stale }">
      <article v-for="(group, groupIndex) in snapshot.groups" :key="group.id" class="qm-row" :style="{ '--qm-row': groupIndex }">
        <div class="qm-row-head">
          <div class="qm-row-id">
            <h3 class="qm-row-name">{{ group.name || group.id }}</h3>
            <div class="qm-row-tags">
              <span class="qm-model">{{ group.model }}</span>
              <span v-if="group.reasoning_effort" class="qm-tag">{{ t('home.quality.reasoning') }} {{ group.reasoning_effort }}</span>
              <span v-if="group.latest_status === 'failed'" class="qm-tag is-failed">{{ t('home.quality.latestFailed') }}</span>
            </div>
          </div>
          <p class="qm-rate" :class="rateTone(group.pass_rate)">
            <strong>{{ formatRate(group.pass_rate) }}</strong>
            <small>{{ t('home.quality.passRate') }}</small>
          </p>
        </div>
        <TransitionGroup
          tag="div"
          name="qm-cell"
          class="qm-bar"
          :style="{ '--qm-slots': HISTORY_SLOTS }"
          role="img"
          :aria-label="barLabel(group)"
        >
          <i
            v-for="(slot, index) in historySlots(group)"
            :key="slot.key"
            :style="{ '--qm-slot': index }"
            :class="[slot.point ? `is-${slot.point.s}` : 'is-empty', { 'is-latest': index === HISTORY_SLOTS - 1 && slot.point }]"
            :title="slot.point ? pointTitle(slot.point) : t('home.quality.legend.empty')"
            aria-hidden="true"
          ></i>
        </TransitionGroup>
        <div class="qm-row-foot">
          <span v-if="group.history.length" class="qm-counts">
            <span><i class="is-passed"></i>{{ t('home.quality.count.passed', { n: group.passed }) }}</span>
            <span><i class="is-failed"></i>{{ t('home.quality.count.failed', { n: group.failed }) }}</span>
            <span><i class="is-error"></i>{{ t('home.quality.count.error', { n: group.errored }) }}</span>
          </span>
          <span v-else>{{ t('home.quality.empty') }}</span>
          <span v-if="group.interval_seconds > 0 || group.next_run_at" class="qm-schedule">
            <template v-if="group.interval_seconds > 0">{{ intervalLabel(group) }}</template>
            <template v-if="group.interval_seconds > 0 && group.next_run_at"> · </template>
            <template v-if="group.next_run_at">{{ t('home.quality.next') }} {{ formatTime(group.next_run_at) }}</template>
          </span>
        </div>
      </article>

      <div class="qm-board-foot">
        <span class="qm-updated">
          {{ snapshot.stale ? t('home.quality.staleAt', { time: relativeUpdated }) : t('home.quality.updatedAt', { time: relativeUpdated }) }}
          <span class="qm-hint">· {{ t('home.quality.barHint', { n: HISTORY_SLOTS }) }}</span>
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
      <span v-for="n in 3" :key="n" class="qm-skeleton"></span>
    </div>

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
  type QualityMonitorGroup,
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

// v0.2.10 / v0.2.11 的历史窗口为 48 次；空槽不代表检测结果。
const HISTORY_SLOTS = 48

function historySlots(group: QualityMonitorGroup) {
  const occurrences = new Map<string, number>()
  const points = group.history.map((point) => {
    const occurrence = occurrences.get(point.t) ?? 0
    occurrences.set(point.t, occurrence + 1)
    return { key: `point-${point.t}-${occurrence}`, point }
  }).slice(-HISTORY_SLOTS)
  return [
    ...Array.from({ length: HISTORY_SLOTS - points.length }, (_, index) => ({
      key: `empty-${index}`, point: null,
    })),
    ...points,
  ]
}

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
    key: 'self',
    icon: 'beaker',
    title: t('home.quality.methods.self.title'),
    desc: t('home.quality.methods.self.desc'),
  },
])

function intervalLabel(group: QualityMonitorGroup): string {
  const minutes = Math.max(1, Math.round(group.interval_seconds / 60))
  return t('home.quality.interval', { minutes })
}

const relativeUpdated = computed(() => {
  const updated = snapshot.value?.updated_at ? Date.parse(snapshot.value.updated_at) : NaN
  if (!Number.isFinite(updated)) return '—'
  const minutes = Math.max(0, Math.floor((now.value - updated) / 60000))
  if (minutes < 1) return t('home.quality.justNow')
  if (minutes < 60) return t('home.quality.minutesAgo', { n: minutes })
  return t('home.quality.hoursAgo', { n: Math.floor(minutes / 60) })
})

function formatRate(rate: number | null): string {
  if (rate === null || !Number.isFinite(rate)) return '—'
  // 不四舍五入到 100%：99.6% 就显示 99.6%，避免「看起来全过」的误导
  const pct = Math.floor(rate * 1000) / 10
  return `${pct % 1 === 0 ? pct.toFixed(0) : pct.toFixed(1)}%`
}

// 通过率低于 95% 就不再用绿色，避免看起来像「一切正常」
function rateTone(rate: number | null): string {
  if (rate === null || !Number.isFinite(rate)) return 'is-none'
  if (rate >= 0.95) return 'is-ok'
  if (rate >= 0.8) return 'is-warn'
  return 'is-danger'
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

function barLabel(group: QualityMonitorGroup): string {
  return t('home.quality.barLabel', {
    name: group.name || group.id,
    passed: group.passed,
    failed: group.failed,
    errored: group.errored,
  })
}

function formatTime(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '—' : date.toLocaleTimeString(locale.value, { hour: '2-digit', minute: '2-digit' })
}

let loading = false
let disposed = false

async function load(): Promise<void> {
  if (loading || disposed) return
  loading = true
  try {
    const res = await getQualityMonitor()
    if (disposed) return
    if (!res.enabled) {
      snapshot.value = null
      state.value = 'disabled'
    } else if (res.available && res.data?.groups.length) {
      snapshot.value = res.data
      state.value = 'ready'
    } else {
      fallbackDetailUrl.value = res.detail_url ?? res.data?.detail_url ?? ''
      if (snapshot.value) snapshot.value = { ...snapshot.value, stale: true }
      state.value = snapshot.value ? 'ready' : 'unavailable'
    }
  } catch {
    if (disposed) return
    // 首次失败时隐藏；刷新失败保留已展示的数据并标记过期。
    if (snapshot.value) snapshot.value = { ...snapshot.value, stale: true }
    state.value = snapshot.value ? 'ready' : 'disabled'
  } finally {
    loading = false
  }
  emit('loaded', visible.value)
}

// 挂载即请求（不阻塞渲染；后端有 5 分钟缓存）。父级需要尽早知道是否启用。
let tick: number | undefined
let refresh: number | undefined

onMounted(() => {
  tick = window.setInterval(() => {
    now.value = Date.now()
  }, 60_000)
  refresh = window.setInterval(() => { void load() }, 300_000)
  void load()
})

onUnmounted(() => {
  disposed = true
  if (tick) window.clearInterval(tick)
  if (refresh) window.clearInterval(refresh)
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
  padding: 6px 24px 18px;
  border: 1px solid var(--border);
  border-radius: 18px;
  background: var(--surface);
  box-shadow: var(--shadow-float);
}

/* 一个分组一行，三个分组时也能一屏看完 */
.qm-row {
  padding: 18px 0 16px;
}

.qm-row + .qm-row {
  border-top: 1px solid var(--border);
}

.qm-row-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 12px;
}

.qm-row-id {
  min-width: 0;
}

.qm-row-name {
  margin: 0 0 6px;
  color: var(--text);
  font-size: 15px;
  font-weight: 700;
  letter-spacing: -0.01em;
}

.qm-row-tags {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
}

.qm-model {
  color: var(--text);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12.5px;
  font-weight: 650;
}

.qm-tag {
  padding: 1px 8px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--accent-ok) 13%, transparent);
  color: var(--accent-ok);
  font-size: 11.5px;
  font-weight: 650;
}

.qm-tag.is-failed {
  background: color-mix(in srgb, var(--accent-danger) 13%, transparent);
  color: var(--accent-danger);
}

.qm-rate {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  flex-shrink: 0;
  margin: 0;
  line-height: 1;
}

.qm-rate strong {
  font-size: 26px;
  font-weight: 750;
  letter-spacing: -0.02em;
  font-variant-numeric: tabular-nums;
}

.qm-rate small {
  margin-top: 5px;
  color: var(--muted);
  font-size: 11.5px;
  font-weight: 500;
}

.qm-rate.is-ok strong { color: var(--accent-ok); }
.qm-rate.is-warn strong { color: var(--accent-warn); }
.qm-rate.is-danger strong { color: var(--accent-danger); }
.qm-rate.is-none strong { color: var(--muted); }

/* 状态页式色块条：一格一次检测，最新的在右边 */
.qm-bar {
  --qm-gap: 3px;
  --qm-step: calc((100% + var(--qm-gap)) / var(--qm-slots));
  position: relative;
  height: 28px;
  overflow: hidden;
  padding: 4px 0;
  margin: -4px 0;
  box-sizing: content-box;
}

/* 面板滚入视口时色块条从左往右扫出，各行错开；刷新数据不会重播 */
.qm-board.is-revealed .qm-bar {
  animation: qm-wipe 0.9s cubic-bezier(0.22, 1, 0.36, 1) backwards;
  animation-delay: calc(0.18s + var(--qm-row, 0) * 0.12s);
}

.qm-bar i {
  position: absolute;
  top: 4px;
  bottom: 4px;
  left: calc(var(--qm-slot) * var(--qm-step));
  width: calc(var(--qm-step) - var(--qm-gap));
  border-radius: 3px;
  transition: left 0.45s ease, transform 0.2s ease, opacity 0.45s ease;
}

.qm-bar i.qm-cell-enter-from {
  opacity: 0;
  transform: translateX(calc(100% + var(--qm-gap)));
}

.qm-bar i.qm-cell-leave-to {
  opacity: 0;
}

.qm-bar i.is-empty {
  background: transparent;
  box-shadow: inset 0 0 0 1px var(--border);
  transition: none;
}

.qm-bar i:not(.is-empty):hover {
  transform: scaleY(1.14);
}

/* 最新一格轻微呼吸，表示监测仍在进行；数据过期时停下 */
.qm-bar i.is-latest {
  animation: qm-breathe 2.4s ease-in-out infinite;
}

.qm-board.is-stale .qm-bar i.is-latest {
  animation: none;
}

.qm-bar i.is-passed,
.qm-counts i.is-passed {
  background: var(--accent-ok);
}

.qm-bar i.is-failed,
.qm-counts i.is-failed {
  background: var(--accent-danger);
}

.qm-bar i.is-error,
.qm-counts i.is-error {
  background: color-mix(in srgb, var(--muted) 40%, transparent);
}

.qm-row-foot {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 6px 16px;
  margin-top: 10px;
  color: var(--muted);
  font-size: 12px;
}

.qm-counts {
  display: inline-flex;
  flex-wrap: wrap;
  gap: 4px 14px;
}

.qm-counts span {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-variant-numeric: tabular-nums;
}

.qm-counts i {
  width: 8px;
  height: 8px;
  border-radius: 2px;
}

.qm-schedule {
  font-variant-numeric: tabular-nums;
}

.qm-board-foot {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 8px 16px;
  padding-top: 14px;
  border-top: 1px solid var(--border);
}

.qm-updated {
  color: var(--muted);
  font-size: 12.5px;
}

.qm-hint {
  opacity: 0.8;
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

/* ---------- 检测方式 ---------- */
.qm-methods {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
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

@keyframes qm-wipe {
  from { clip-path: inset(0 100% 0 0); }
  to { clip-path: inset(0 0 0 0); }
}

@keyframes qm-breathe {
  50% { opacity: 0.55; }
}

@media (max-width: 767px) {
  .qm-board {
    padding: 2px 16px 16px;
  }

  .qm-row-name {
    font-size: 14px;
  }

  .qm-rate strong {
    font-size: 22px;
  }

  .qm-bar {
    --qm-gap: 2px;
    height: 24px;
  }

  .qm-hint {
    display: none;
  }

  .qm-methods {
    grid-template-columns: 1fr;
  }
}

@media (prefers-reduced-motion: reduce) {
  .qm-bar i {
    transition: none;
  }

  .qm-board.is-revealed .qm-bar,
  .qm-bar i.is-latest,
  .qm-live::after,
  .qm-skeleton {
    animation: none;
  }
}
</style>
