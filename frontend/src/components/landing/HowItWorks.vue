<template>
  <section
    id="how"
    ref="rootEl"
    class="hw-root"
    :class="pinned ? 'is-pinned' : 'is-timeline'"
    :style="pinned ? { height: `${steps.length * STEP_VH + 100}vh` } : undefined"
  >
    <div class="hw-sticky">
      <header class="hw-head">
        <p class="hw-eyebrow">{{ t('home.nav.how') }}</p>
        <h2 class="hw-title">{{ t('home.how.title') }}</h2>
        <p class="hw-sub">{{ t('home.how.subtitle') }}</p>
      </header>

      <!-- ========== 桌面：钉屏演示 ========== -->
      <div v-if="pinned" class="hw-body">
        <ol class="hw-steps">
          <li v-for="(step, index) in steps" :key="step.key">
            <button
              type="button"
              class="hw-step"
              :class="{ 'is-active': index === current, 'is-done': index < current }"
              :aria-current="index === current ? 'step' : undefined"
              @click="scrollToStep(index)"
            >
              <span class="hw-step-num">{{ index + 1 }}</span>
              <span class="hw-step-text">
                <span class="hw-step-title">{{ step.title }}</span>
                <span class="hw-step-desc"><span>{{ step.desc }}</span></span>
              </span>
            </button>
          </li>
        </ol>

        <div class="hw-stage" aria-hidden="true">
          <div class="hw-flow">
            <div class="hw-node is-lit">
              <span class="hw-node-title">{{ t('home.architecture.client.title') }}</span>
              <span class="hw-node-desc">{{ t('home.architecture.client.desc') }}</span>
            </div>

            <span class="hw-link" :class="linkState(0, [0, 1])"><i></i></span>

            <div class="hw-node hw-node-gateway is-lit">
              <span class="hw-node-title is-brand">{{ t('home.architecture.gateway.title') }}</span>
              <span class="hw-chips">
                <span
                  v-for="chip in gatewayChips"
                  :key="chip.key"
                  class="hw-chip"
                  :class="{ 'is-on': current >= chip.from, 'is-active': chip.active.includes(current) }"
                >
                  {{ chip.label }}
                </span>
              </span>
            </div>

            <span class="hw-link" :class="linkState(2, [2])"><i></i></span>

            <div class="hw-node" :class="{ 'is-lit': current >= 2 }">
              <span class="hw-node-title">
                {{ t('home.architecture.pool.title') }}
                <em class="hw-pool-count">128</em>
              </span>
              <span class="hw-pool">
                <i
                  v-for="dot in POOL_DOTS"
                  :key="dot"
                  :class="poolDotClass(dot)"
                ></i>
              </span>
            </div>

            <span class="hw-link" :class="linkState(3, [3, 4])"><i></i></span>

            <div class="hw-node" :class="{ 'is-lit': current >= 3 }">
              <span class="hw-node-title">{{ t('home.architecture.upstream.title') }}</span>
              <span class="hw-node-desc">{{ t('home.architecture.upstream.desc') }}</span>
            </div>
          </div>

          <Transition name="hw-detail" mode="out-in">
            <div :key="current" class="hw-detail" :class="`is-${steps[current].key}`">
              <span class="hw-detail-icon"><Icon :name="steps[current].icon" size="md" /></span>
              <span class="hw-detail-text">
                <span class="hw-detail-label">{{ steps[current].title }}</span>
                <code class="hw-detail-log">{{ steps[current].detail }}</code>
              </span>
            </div>
          </Transition>

          <div class="hw-progress">
            <i :style="{ transform: `scaleX(${progress})` }"></i>
          </div>
        </div>
      </div>

      <!-- ========== 窄屏 / 矮屏 / 减少动画：竖向时间线 ========== -->
      <ol v-else class="hw-timeline">
        <li
          v-for="(step, index) in steps"
          :key="step.key"
          v-reveal="index"
          class="hw-tl-item"
          :class="`is-${step.key}`"
        >
          <span class="hw-tl-num">{{ index + 1 }}</span>
          <div class="hw-tl-card">
            <div class="hw-tl-head">
              <Icon :name="step.icon" size="sm" />
              <h3>{{ step.title }}</h3>
            </div>
            <p>{{ step.desc }}</p>
            <code class="hw-detail-log">{{ step.detail }}</code>
          </div>
        </li>
      </ol>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useMediaQuery } from '@/composables/useMediaQuery'
import { progressToStep, useScrollProgress } from '@/composables/useScrollProgress'
import { vReveal } from '@/directives/reveal'

type IconName = InstanceType<typeof Icon>['$props']['name']

const { t } = useI18n()

/** 每一步占用的滚动距离（vh）。太短会一闪而过，太长显得拖沓。 */
const STEP_VH = 65

/** 账号池点阵下标：#047 为调度选中，#012 为故障账号，#089 为备用 */
const POOL_DOTS = Array.from({ length: 24 }, (_, i) => i)
const DOT_SELECTED = 5
const DOT_FAILED = 2
const DOT_BACKUP = 17

const rootEl = ref<HTMLElement | null>(null)

// 钉屏只在「宽且高」的桌面上开：矮屏笔记本上钉住的画面会被顶栏挤掉一截
const pinned = useMediaQuery(
  '(min-width: 1024px) and (min-height: 720px) and (prefers-reduced-motion: no-preference)',
)
const progress = useScrollProgress(rootEl, pinned)

const steps = computed<Array<{ key: string; icon: IconName; title: string; desc: string; detail: string }>>(() => [
  {
    key: 'auth',
    icon: 'shield',
    title: t('home.how.steps.auth.title'),
    desc: t('home.how.steps.auth.desc'),
    detail: t('home.how.steps.auth.detail'),
  },
  {
    key: 'limit',
    icon: 'bolt',
    title: t('home.how.steps.limit.title'),
    desc: t('home.how.steps.limit.desc'),
    detail: t('home.how.steps.limit.detail'),
  },
  {
    key: 'route',
    icon: 'link',
    title: t('home.how.steps.route.title'),
    desc: t('home.how.steps.route.desc'),
    detail: t('home.how.steps.route.detail'),
  },
  {
    key: 'failover',
    icon: 'refresh',
    title: t('home.how.steps.failover.title'),
    desc: t('home.how.steps.failover.desc'),
    detail: t('home.how.steps.failover.detail'),
  },
  {
    key: 'billing',
    icon: 'calculator',
    title: t('home.how.steps.billing.title'),
    desc: t('home.how.steps.billing.desc'),
    detail: t('home.how.steps.billing.detail'),
  },
])

const current = computed(() => progressToStep(progress.value, steps.value.length))

/** 网关内四道处理：from = 从第几步起常亮，active = 哪些步骤下高亮 */
const gatewayChips = computed(() => [
  { key: 'auth', label: t('home.architecture.gateway.auth'), from: 0, active: [0] },
  { key: 'limit', label: t('home.architecture.gateway.limit'), from: 1, active: [1] },
  { key: 'route', label: t('home.architecture.gateway.route'), from: 2, active: [2, 3] },
  { key: 'bill', label: t('home.architecture.gateway.bill'), from: 4, active: [4] },
])

/** 连线状态：从 onFrom 步起常亮，flowSteps 内跑光点 */
function linkState(onFrom: number, flowSteps: number[]): Record<string, boolean> {
  return {
    'is-on': current.value >= onFrom,
    'is-flowing': flowSteps.includes(current.value),
  }
}

function poolDotClass(dot: number): string {
  const step = current.value
  if (step < 2) return ''
  if (step === 2) return dot === DOT_SELECTED ? 'is-selected' : 'is-ok'
  // 故障转移：#012 闪红，#089 接手；计费阶段保持接手后的状态
  if (dot === DOT_FAILED) return step === 3 ? 'is-failed' : 'is-idle'
  if (dot === DOT_BACKUP) return 'is-selected'
  return 'is-ok'
}

function scrollToStep(index: number): void {
  const el = rootEl.value
  if (!el) return
  const top = el.getBoundingClientRect().top + window.scrollY
  const scrollable = el.offsetHeight - window.innerHeight
  window.scrollTo({
    top: top + (scrollable * (index + 0.5)) / steps.value.length,
    behavior: 'smooth',
  })
}
</script>

<style scoped>
.hw-root {
  position: relative;
  /* 允许吸附在章节起点，内部不设吸附点 */
  scroll-snap-align: start;
}

.hw-head {
  margin-bottom: 30px;
  text-align: center;
}

.hw-eyebrow {
  margin: 0 0 10px;
  color: var(--brand);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.14em;
  text-transform: uppercase;
}

.hw-title {
  margin: 0;
  color: var(--text);
  font-size: clamp(24px, 3vw, 34px);
  font-weight: 750;
  letter-spacing: -0.025em;
}

.hw-sub {
  margin: 10px 0 0;
  color: var(--muted);
  font-size: 14.5px;
}

/* ================= 钉屏模式 ================= */
.is-pinned .hw-sticky {
  position: sticky;
  top: 64px;
  display: flex;
  flex-direction: column;
  justify-content: center;
  height: calc(100vh - 64px);
  padding-bottom: 24px;
}

.hw-body {
  display: grid;
  grid-template-columns: minmax(0, 0.9fr) minmax(0, 1.35fr);
  gap: 40px;
  align-items: center;
}

/* ---------- 左：步骤清单 ---------- */
.hw-steps {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.hw-step {
  display: flex;
  gap: 14px;
  width: 100%;
  padding: 12px 14px;
  border: 1px solid transparent;
  border-radius: 14px;
  background: transparent;
  color: inherit;
  text-align: left;
  cursor: pointer;
  opacity: 0.5;
  transition: opacity 0.35s ease, background 0.35s ease, border-color 0.35s ease;
}

.hw-step:hover {
  opacity: 0.8;
}

.hw-step.is-active {
  opacity: 1;
  border-color: color-mix(in srgb, var(--brand) 30%, var(--border));
  background: var(--surface);
  box-shadow: var(--shadow-card);
}

.hw-step:focus-visible {
  outline: 2px solid var(--brand);
  outline-offset: 2px;
}

.hw-step-num {
  display: grid;
  flex: 0 0 28px;
  height: 28px;
  place-items: center;
  border: 1px solid var(--border-strong);
  border-radius: 999px;
  color: var(--muted);
  font-size: 12px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  transition: background 0.35s ease, color 0.35s ease, border-color 0.35s ease;
}

.hw-step.is-done .hw-step-num {
  border-color: color-mix(in srgb, var(--accent-ok) 50%, transparent);
  color: var(--accent-ok);
}

.hw-step.is-active .hw-step-num {
  border-color: var(--brand);
  background: var(--brand);
  color: #042f2e;
}

.hw-step-text {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 3px;
}

.hw-step-title {
  color: var(--text);
  font-size: 15px;
  font-weight: 700;
}

/* 非当前步只露标题，当前步展开说明：清单高度稳定，但信息集中在焦点上 */
.hw-step-desc {
  display: grid;
  grid-template-rows: 0fr;
  color: var(--muted);
  font-size: 13px;
  line-height: 1.65;
  overflow: hidden;
  transition: grid-template-rows 0.35s ease;
}

.hw-step-desc > span {
  min-height: 0;
  overflow: hidden;
}

.hw-step.is-active .hw-step-desc {
  grid-template-rows: 1fr;
}

/* ---------- 右：舞台 ---------- */
.hw-stage {
  padding: 26px;
  border: 1px solid var(--border);
  border-radius: 20px;
  background: color-mix(in srgb, var(--surface) 88%, transparent);
  box-shadow: var(--shadow-float);
}

.hw-flow {
  display: flex;
  align-items: stretch;
  gap: 0;
}

.hw-node {
  display: flex;
  flex: 1 1 0;
  min-width: 0;
  flex-direction: column;
  gap: 8px;
  padding: 14px 12px;
  border: 1px solid var(--border);
  border-radius: 14px;
  background: var(--surface-2);
  opacity: 0.45;
  transition: opacity 0.4s ease, border-color 0.4s ease, background 0.4s ease;
}

.hw-node.is-lit {
  opacity: 1;
  border-color: color-mix(in srgb, var(--brand) 28%, var(--border));
  background: var(--surface);
}

.hw-node-gateway {
  flex-grow: 1.25;
}

.hw-node-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
  color: var(--text);
  font-size: 13px;
  font-weight: 700;
}

.hw-node-title.is-brand {
  color: var(--brand);
}

.hw-node-desc {
  color: var(--muted);
  font-size: 11px;
  line-height: 1.55;
  word-break: break-word;
}

.hw-pool-count {
  color: var(--accent-ok);
  font-size: 11.5px;
  font-style: normal;
  font-variant-numeric: tabular-nums;
}

/* 网关四道处理 */
.hw-chips {
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.hw-chip {
  padding: 5px 8px;
  border: 1px solid transparent;
  border-radius: 8px;
  background: var(--surface-3);
  color: var(--muted);
  font-size: 11.5px;
  font-weight: 650;
  transition: color 0.3s ease, background 0.3s ease, border-color 0.3s ease;
}

.hw-chip.is-on {
  color: var(--brand);
}

.hw-chip.is-active {
  border-color: color-mix(in srgb, var(--brand) 45%, transparent);
  background: color-mix(in srgb, var(--brand) 14%, transparent);
}

/* 账号池点阵 */
.hw-pool {
  display: grid;
  grid-template-columns: repeat(6, 1fr);
  gap: 5px;
}

.hw-pool i {
  aspect-ratio: 1;
  border-radius: 999px;
  background: color-mix(in srgb, var(--muted) 24%, transparent);
  transition: background 0.35s ease, box-shadow 0.35s ease, transform 0.35s ease;
}

.hw-pool i.is-ok {
  background: color-mix(in srgb, var(--accent-ok) 70%, transparent);
}

.hw-pool i.is-idle {
  background: color-mix(in srgb, var(--muted) 30%, transparent);
}

.hw-pool i.is-selected {
  background: var(--brand);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--brand) 28%, transparent);
  transform: scale(1.2);
}

.hw-pool i.is-failed {
  background: var(--accent-danger);
  animation: hw-fail 0.9s ease-in-out 2;
}

/* 连线 + 光点 */
.hw-link {
  position: relative;
  flex: 0 0 22px;
  align-self: center;
  height: 0;
  border-top: 1px dashed var(--border-strong);
  transition: border-color 0.35s ease;
}

.hw-link.is-on {
  border-top-color: color-mix(in srgb, var(--brand) 55%, transparent);
}

.hw-link i {
  position: absolute;
  top: -3px;
  left: 0;
  width: 6px;
  height: 6px;
  border-radius: 999px;
  background: var(--brand);
  opacity: 0;
}

.hw-link.is-flowing i {
  animation: hw-travel 1.2s ease-in-out infinite;
}

/* 当前步骤的日志行 */
.hw-detail {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-top: 22px;
  padding: 14px 16px;
  border: 1px solid var(--border);
  border-radius: 14px;
  background: var(--surface-2);
}

.hw-detail-icon {
  display: grid;
  flex: 0 0 38px;
  height: 38px;
  place-items: center;
  border-radius: 11px;
  background: color-mix(in srgb, var(--brand) 13%, transparent);
  color: var(--brand);
}

.hw-detail.is-failover {
  animation: hw-flash 0.9s ease-out 1;
}

.hw-detail.is-billing .hw-detail-icon {
  background: color-mix(in srgb, var(--accent-ok) 14%, transparent);
  color: var(--accent-ok);
}

.hw-detail-text {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 3px;
}

.hw-detail-label {
  color: var(--text);
  font-size: 13.5px;
  font-weight: 700;
}

.hw-detail-log {
  color: var(--accent-ok);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
  word-break: break-word;
}

.hw-detail-enter-active,
.hw-detail-leave-active {
  transition: opacity 0.22s ease, transform 0.22s ease;
}

.hw-detail-enter-from {
  opacity: 0;
  transform: translateY(6px);
}

.hw-detail-leave-to {
  opacity: 0;
  transform: translateY(-6px);
}

.hw-progress {
  height: 3px;
  margin-top: 18px;
  overflow: hidden;
  border-radius: 999px;
  background: var(--surface-3);
}

.hw-progress i {
  display: block;
  height: 100%;
  background: linear-gradient(90deg, var(--brand-soft), var(--brand), var(--brand-2));
  transform-origin: left;
}

/* ================= 时间线模式 ================= */
.hw-timeline {
  position: relative;
  display: flex;
  max-width: 640px;
  flex-direction: column;
  gap: 14px;
  margin: 0 auto;
  padding: 0;
  list-style: none;
}

/* 贯穿的竖线 */
.hw-timeline::before {
  content: '';
  position: absolute;
  top: 14px;
  bottom: 14px;
  left: 13px;
  border-left: 1px dashed color-mix(in srgb, var(--brand) 40%, var(--border));
}

.hw-tl-item {
  position: relative;
  display: flex;
  gap: 14px;
}

.hw-tl-num {
  position: relative;
  z-index: 1;
  display: grid;
  flex: 0 0 28px;
  height: 28px;
  place-items: center;
  border-radius: 999px;
  background: var(--brand);
  color: #042f2e;
  font-size: 12px;
  font-weight: 700;
}

.hw-tl-card {
  flex: 1 1 auto;
  min-width: 0;
  padding: 16px;
  border: 1px solid var(--border);
  border-radius: 14px;
  background: var(--surface);
  box-shadow: var(--shadow-card);
}

.hw-tl-head {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--brand);
}

.hw-tl-head h3 {
  margin: 0;
  color: var(--text);
  font-size: 15px;
  font-weight: 700;
}

.hw-tl-card p {
  margin: 8px 0 10px;
  color: var(--muted);
  font-size: 13px;
  line-height: 1.65;
}

.hw-tl-item.is-failover .hw-tl-num {
  background: var(--accent-warn);
}

/* ================= 动画 ================= */
@keyframes hw-travel {
  0% { opacity: 0; transform: translateX(0); }
  20% { opacity: 1; }
  80% { opacity: 1; }
  100% { opacity: 0; transform: translateX(16px); }
}

@keyframes hw-fail {
  50% { box-shadow: 0 0 0 5px color-mix(in srgb, var(--accent-danger) 30%, transparent); }
}

@keyframes hw-flash {
  0% { background: color-mix(in srgb, var(--accent-danger) 16%, var(--surface-2)); }
  100% { background: var(--surface-2); }
}
</style>
