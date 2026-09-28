<template>
  <section ref="rootEl" class="af-root" :class="{ 'is-started': started }">
    <header class="af-head">
      <h2 class="af-title">{{ t('home.architecture.title') }}</h2>
      <p class="af-sub">{{ t('home.architecture.subtitle') }}</p>
    </header>

    <div class="af-flow">
      <!-- 1. 客户端 -->
      <div class="af-node af-node-client">
        <div class="af-node-head">
          <span class="af-badge">{{ t('home.architecture.client.title') }}</span>
        </div>
        <p class="af-node-desc">{{ t('home.architecture.client.desc') }}</p>
      </div>

      <span class="af-arrow" aria-hidden="true"></span>

      <!-- 2. 网关四层 -->
      <div class="af-node af-node-gateway">
        <div class="af-node-head">
          <span class="af-badge af-badge-brand">{{ t('home.architecture.gateway.title') }}</span>
        </div>
        <ul class="af-steps">
          <li
            v-for="(step, index) in gatewaySteps"
            :key="step.key"
            class="af-step"
            :style="{ '--step-delay': `${index * 0.35}s` }"
          >
            <span class="af-step-label">{{ step.label }}</span>
            <span class="af-step-desc">{{ step.desc }}</span>
          </li>
        </ul>
      </div>

      <span class="af-arrow" aria-hidden="true"></span>

      <!-- 3. 账号池 -->
      <div class="af-node af-node-pool">
        <div class="af-node-head">
          <span class="af-badge">{{ t('home.architecture.pool.title') }}</span>
          <span class="af-pool-count">128</span>
        </div>
        <div class="af-pool-dots" aria-hidden="true">
          <i
            v-for="dot in poolDots"
            :key="dot"
            class="af-pool-dot"
            :class="dot % 11 === 0 ? 'is-standby' : 'is-healthy'"
            :style="{ '--dot-delay': `${dot * 0.045}s` }"
          ></i>
        </div>
        <p class="af-node-desc">{{ t('home.architecture.pool.desc') }}</p>
      </div>

      <span class="af-arrow" aria-hidden="true"></span>

      <!-- 4. 上游 -->
      <div class="af-node af-node-upstream">
        <div class="af-node-head">
          <span class="af-badge">{{ t('home.architecture.upstream.title') }}</span>
        </div>
        <p class="af-node-desc">{{ t('home.architecture.upstream.desc') }}</p>
      </div>
    </div>

    <!-- 回执：整条链路跑完才浮出 -->
    <div class="af-receipt">
      <span class="af-receipt-dot"></span>
      {{ t('home.architecture.receipt') }}
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

const rootEl = ref<HTMLElement | null>(null)
const started = ref(false)

let observer: IntersectionObserver | null = null
let motionQuery: MediaQueryList | null = null

const gatewaySteps = computed(() => [
  {
    key: 'auth',
    label: t('home.architecture.gateway.auth'),
    desc: t('home.architecture.gateway.authDesc'),
  },
  {
    key: 'limit',
    label: t('home.architecture.gateway.limit'),
    desc: t('home.architecture.gateway.limitDesc'),
  },
  {
    key: 'route',
    label: t('home.architecture.gateway.route'),
    desc: t('home.architecture.gateway.routeDesc'),
  },
  {
    key: 'bill',
    label: t('home.architecture.gateway.bill'),
    desc: t('home.architecture.gateway.billDesc'),
  },
])

/** 账号池示意点阵。11 的倍数标为备用，视觉上能看出「有冗余」。 */
const poolDots = Array.from({ length: 44 }, (_, i) => i)

onMounted(() => {
  if (typeof window !== 'undefined' && typeof window.matchMedia === 'function') {
    motionQuery = window.matchMedia('(prefers-reduced-motion: reduce)')
  }

  // 减少动画时直接呈现终态，不做滚动触发
  if (motionQuery?.matches) {
    started.value = true
    return
  }

  if (typeof IntersectionObserver === 'undefined' || !rootEl.value) {
    started.value = true
    return
  }

  observer = new IntersectionObserver(
    (entries) => {
      if (entries.some((entry) => entry.isIntersecting)) {
        started.value = true
        // 只跑一次：光带重复跑会显得吵闹
        observer?.disconnect()
        observer = null
      }
    },
    { threshold: 0.35 },
  )

  observer.observe(rootEl.value)
})

onUnmounted(() => {
  observer?.disconnect()
  observer = null
})
</script>

<style scoped>
.af-root {
  position: relative;
}

.af-head {
  margin-bottom: 26px;
  text-align: center;
}

.af-title {
  margin: 0;
  color: var(--text);
  font-size: 24px;
  font-weight: 750;
  letter-spacing: -0.02em;
}

.af-sub {
  margin: 8px 0 0;
  color: var(--muted);
  font-size: 14px;
}

/* ---------- 布局 ---------- */
.af-flow {
  display: flex;
  align-items: stretch;
  gap: 12px;
}

.af-node {
  flex: 1 1 0;
  min-width: 0;
  padding: 16px 15px;
  border: 1px solid var(--border);
  border-radius: 14px;
  background: var(--surface);
  box-shadow: var(--shadow-card);
  opacity: 0;
  transform: translateY(10px);
  transition: opacity 0.45s ease, transform 0.45s ease, border-color 0.3s ease;
}

/* 节点依次点亮，与「光带跑过去」的节奏对齐 */
.af-node-client    { transition-delay: 0s; }
.af-node-gateway   { transition-delay: 0.3s; }
.af-node-pool      { transition-delay: 0.6s; }
.af-node-upstream  { transition-delay: 0.9s; }

.is-started .af-node {
  opacity: 1;
  transform: translateY(0);
}

.is-started .af-node {
  border-color: color-mix(in srgb, var(--brand) 26%, var(--border));
}

/* ---------- 节点内部 ---------- */
.af-node-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 10px;
}

.af-badge {
  color: var(--text);
  font-size: 13px;
  font-weight: 700;
}

.af-badge-brand {
  color: var(--brand);
}

.af-pool-count {
  color: var(--accent-ok);
  font-size: 12px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}

.af-node-desc {
  margin: 0;
  color: var(--muted);
  font-size: 11.5px;
  line-height: 1.6;
  word-break: break-word;
}

/* ---------- 网关四层 ---------- */
.af-steps {
  display: flex;
  flex-direction: column;
  gap: 7px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.af-step {
  display: flex;
  align-items: baseline;
  gap: 7px;
  padding: 6px 9px;
  border-radius: 8px;
  background: var(--surface-3);
  opacity: 0;
  transform: translateX(-5px);
  transition: opacity 0.4s ease, transform 0.4s ease;
}

/* 四层在网关节点亮起之后再逐层点亮 */
.is-started .af-step {
  opacity: 1;
  transform: translateX(0);
  transition-delay: calc(0.45s + var(--step-delay));
}

.af-step-label {
  flex: 0 0 auto;
  color: var(--brand);
  font-size: 11.5px;
  font-weight: 700;
}

.af-step-desc {
  flex: 1 1 auto;
  min-width: 0;
  color: var(--muted);
  font-size: 10.5px;
  word-break: break-word;
}

/* ---------- 账号池点阵 ---------- */
.af-pool-dots {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-bottom: 10px;
}

.af-pool-dot {
  width: 7px;
  height: 7px;
  border-radius: 999px;
  /* 初始为极淡的状态，点亮后到满色 */
  background: color-mix(in srgb, var(--muted) 26%, transparent);
  transition: background 0.5s ease, box-shadow 0.5s ease;
}

.is-started .af-pool-dot {
  transition-delay: calc(0.7s + var(--dot-delay));
}

.is-started .af-pool-dot.is-healthy {
  background: var(--accent-ok);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--accent-ok) 12%, transparent);
}

/* 备用账号用品牌色区分，示意「有余量」而不是「坏了」 */
.is-started .af-pool-dot.is-standby {
  background: var(--brand-2);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--brand-2) 12%, transparent);
}

/* ---------- 连接箭头 ---------- */
.af-arrow {
  position: relative;
  flex: 0 0 26px;
  align-self: center;
  height: 0;
  border-top: 1px dashed color-mix(in srgb, var(--brand) 34%, var(--border));
}

/* 光点沿路径滑过，作为「请求正在穿过」的视觉线索 */
.af-arrow::after {
  content: '';
  position: absolute;
  top: -3px;
  left: 0;
  width: 6px;
  height: 6px;
  border-radius: 999px;
  background: var(--brand);
  opacity: 0;
}

.is-started .af-arrow::after {
  animation: af-travel 0.55s ease-in-out forwards;
}

.af-node-client + .af-arrow::after { animation-delay: 0.25s; }
.af-node-gateway + .af-arrow::after { animation-delay: 0.55s; }
.af-node-pool + .af-arrow::after { animation-delay: 0.85s; }

/* ---------- 回执 ---------- */
.af-receipt {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  margin-top: 22px;
  color: var(--accent-ok);
  font-size: 13px;
  font-weight: 650;
  opacity: 0;
  transform: scale(0.94);
  transition: opacity 0.4s ease, transform 0.4s ease;
}

.is-started .af-receipt {
  opacity: 1;
  transform: scale(1);
  /* 等整条链路跑完再浮出 */
  transition-delay: 1.5s;
}

.af-receipt-dot {
  width: 7px;
  height: 7px;
  border-radius: 999px;
  background: var(--accent-ok);
  box-shadow: 0 0 0 4px color-mix(in srgb, var(--accent-ok) 14%, transparent);
}

@keyframes af-travel {
  0% { opacity: 1; transform: translateX(0); }
  100% { opacity: 0; transform: translateX(20px); }
}

/* ---------- 响应式：窄屏改竖排，箭头转 90° ---------- */
@media (max-width: 900px) {
  .af-flow {
    flex-direction: column;
    gap: 0;
  }

  .af-node {
    flex: 0 0 auto;
  }

  .af-arrow {
    flex: 0 0 20px;
    align-self: center;
    width: 0;
    height: 20px;
    border-top: none;
    border-left: 1px dashed color-mix(in srgb, var(--brand) 34%, var(--border));
  }

  .af-arrow::after {
    top: 0;
    left: -3px;
  }

  .is-started .af-arrow::after {
    animation: af-travel-v 0.55s ease-in-out forwards;
  }
}

@keyframes af-travel-v {
  0% { opacity: 1; transform: translateY(0); }
  100% { opacity: 0; transform: translateY(14px); }
}

/* 兜底：即使全局 reduce-motion 规则被局部覆盖，这里也保证内容是可见终态 */
@media (prefers-reduced-motion: reduce) {
  .af-node,
  .af-step,
  .af-pool-dot,
  .af-receipt {
    opacity: 1 !important;
    transform: none !important;
  }

  .af-arrow::after {
    display: none;
  }
}
</style>
