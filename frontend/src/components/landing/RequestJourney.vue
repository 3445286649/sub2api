<template>
  <div
    class="rj-root"
    :class="{ 'rj-is-compact': compact }"
    aria-hidden="true"
  >
    <!-- 状态条：默认展示，仅在 compact（登录页左栏 / 紧凑首页）下隐藏 -->
    <div v-if="!compact" class="rj-rail">
      <div v-for="stat in railStats" :key="stat.key" class="rj-rail-item">
        <span class="rj-dot"></span>
        <span class="rj-rail-label">{{ stat.label }}</span>
        <span class="rj-rail-value">{{ stat.value }}</span>
      </div>
    </div>

    <div class="rj-window">
      <div class="rj-header">
        <span class="rj-lights">
          <i class="rj-light rj-light-close"></i>
          <i class="rj-light rj-light-min"></i>
          <i class="rj-light rj-light-max"></i>
        </span>
        <span class="rj-title">{{ t('home.journey.title') }}</span>
        <span class="rj-scenes">
          <i
            v-for="(scene, index) in scenes"
            :key="scene.id"
            class="rj-scene-dot"
            :class="{ 'is-active': index === sceneIndex }"
          ></i>
        </span>
      </div>

      <div class="rj-body">
        <!-- 命令打字机 -->
        <div class="rj-line rj-cmd">
          <span class="rj-prompt">$</span>
          <span class="rj-cmd-text">{{ typedCommand }}</span>
          <span v-if="!staticMode" class="rj-caret"></span>
        </div>

        <!-- 逐行揭示的处理链路 -->
        <div
          v-for="(line, index) in revealedLines"
          :key="`${sceneIndex}-${line.key}`"
          class="rj-line rj-result"
          :class="[
            `rj-tone-${line.tone}`,
            { 'is-error': index === errorIndex },
          ]"
        >
          <span class="rj-marker">{{ line.tone === 'summary' ? '✓' : '→' }}</span>
          <span v-if="line.label" class="rj-label">{{ line.label }}</span>
          <span class="rj-text">{{ line.text }}</span>
          <span v-if="line.timing" class="rj-timing">{{ line.timing }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

interface Props {
  /** 精简模式：只跑「正常调度」场景，隐藏状态条。用于登录页左栏与紧凑首页。 */
  compact?: boolean
  /** 是否自动播放。关闭后停在终态。 */
  autoplay?: boolean
  /** 是否循环场景。关闭后播完最后一个场景就停住。 */
  loopScenes?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  compact: false,
  autoplay: true,
  loopScenes: true,
})

const { t } = useI18n()

/** 打字机的命令内容。刻意保持短，避免慢速网络下动画过长。 */
const COMMAND = 'curl -X POST /v1/messages -H "x-session-id: a3f9"'

const TYPE_MS = 34
const LINE_GAP_MS = 700
const HOLD_MS = 1800

type LineTone = 'default' | 'error' | 'ok' | 'summary'

interface JourneyLine {
  key: string
  label: string
  text: string
  timing?: string
  tone: LineTone
}

interface Scene {
  id: 'normal' | 'sticky' | 'failover'
  lines: JourneyLine[]
}

/**
 * 场景文案全部用静态 t() 键书写。
 * 不要改成动态拼接键——i18n 检查脚本只静态扫描字面量，
 * 动态键既不会被校验，缺键时也只会在运行时暴露原始键名。
 */
const scenes = computed<Scene[]>(() => {
  const normal: Scene = {
    id: 'normal',
    lines: [
      {
        key: 'auth',
        tone: 'default',
        label: t('home.journey.labels.auth'),
        text: t('home.journey.scenes.normal.auth'),
        timing: '12ms',
      },
      {
        key: 'quota',
        tone: 'default',
        label: t('home.journey.labels.quota'),
        text: t('home.journey.scenes.normal.quota'),
        timing: '8ms',
      },
      {
        key: 'route',
        tone: 'default',
        label: t('home.journey.labels.route'),
        text: t('home.journey.scenes.normal.route'),
      },
      {
        key: 'upstream',
        tone: 'ok',
        label: t('home.journey.labels.upstream'),
        text: t('home.journey.scenes.normal.upstream'),
      },
      {
        key: 'summary',
        tone: 'summary',
        label: '',
        text: [
          t('home.journey.scenes.normal.total'),
          t('home.journey.scenes.normal.cost'),
          t('home.journey.scenes.normal.bound'),
        ].join(' · '),
      },
    ],
  }

  const sticky: Scene = {
    id: 'sticky',
    lines: [
      {
        key: 'session',
        tone: 'default',
        label: t('home.journey.labels.session'),
        text: t('home.journey.scenes.sticky.session'),
      },
      {
        key: 'upstream',
        tone: 'ok',
        label: t('home.journey.labels.upstream'),
        text: t('home.journey.scenes.sticky.upstream'),
      },
      {
        key: 'summary',
        tone: 'summary',
        label: '',
        text: [
          `${t('home.journey.scenes.sticky.cacheLabel')} ${t('home.journey.scenes.sticky.cacheValue')}`,
          `${t('home.journey.scenes.sticky.savedLabel')} ${t('home.journey.scenes.sticky.savedValue')}`,
        ].join(' · '),
      },
    ],
  }

  const failover: Scene = {
    id: 'failover',
    lines: [
      {
        key: 'route',
        tone: 'default',
        label: t('home.journey.labels.route'),
        text: t('home.journey.scenes.failover.route'),
      },
      {
        key: 'failed',
        tone: 'error',
        label: t('home.journey.labels.upstream'),
        text: t('home.journey.scenes.failover.upstreamFail'),
      },
      {
        key: 'retry',
        tone: 'default',
        label: t('home.journey.labels.retry'),
        text: t('home.journey.scenes.failover.retry'),
      },
      {
        key: 'ok',
        tone: 'ok',
        label: t('home.journey.labels.upstream'),
        text: t('home.journey.scenes.failover.upstreamOk'),
      },
      {
        key: 'summary',
        tone: 'summary',
        label: '',
        text: [
          t('home.journey.scenes.failover.transparent'),
          t('home.journey.scenes.failover.switches'),
          t('home.journey.scenes.failover.overhead'),
        ].join(' · '),
      },
    ],
  }

  // 精简模式只保留正常调度场景，避免小尺寸容器里反复跳动
  return props.compact ? [normal] : [normal, sticky, failover]
})

const railStats = computed(() => [
  { key: 'pool', label: t('home.journey.rail.pool'), value: '128' },
  { key: 'success', label: t('home.journey.rail.success'), value: '99.97%' },
  { key: 'latency', label: t('home.journey.rail.latency'), value: '1.2s' },
])

const sceneIndex = ref(0)
const typedCount = ref(0)
const visibleCount = ref(0)
const errorIndex = ref(-1)
const staticMode = ref(false)

const typedCommand = computed(() => COMMAND.slice(0, typedCount.value))

const activeLines = computed<JourneyLine[]>(
  () => scenes.value[sceneIndex.value]?.lines ?? [],
)

const revealedLines = computed<JourneyLine[]>(() =>
  activeLines.value.slice(0, visibleCount.value),
)

/** 所有待触发的定时器。组件卸载时必须全部清掉，否则会持续持有响应式引用。 */
let timers: number[] = []
let motionQuery: MediaQueryList | null = null

function clearTimers(): void {
  timers.forEach((id) => window.clearTimeout(id))
  timers = []
}

function schedule(fn: () => void, delay: number): void {
  timers.push(window.setTimeout(fn, delay))
}

/** 渲染某个场景的静态终态（用于 reduce-motion 与 autoplay=false）。 */
function renderStatic(index: number): void {
  clearTimers()
  sceneIndex.value = index
  typedCount.value = COMMAND.length
  visibleCount.value = scenes.value[index]?.lines.length ?? 0
  errorIndex.value = -1
}

function runScene(index: number): void {
  clearTimers()

  sceneIndex.value = index
  typedCount.value = 0
  visibleCount.value = 0
  errorIndex.value = -1

  const scene = scenes.value[index]
  if (!scene) return

  // 1) 命令逐字打出
  for (let i = 1; i <= COMMAND.length; i += 1) {
    schedule(() => {
      typedCount.value = i
    }, i * TYPE_MS)
  }

  const afterCommand = COMMAND.length * TYPE_MS + 260

  // 2) 处理链路逐行揭示；失败行短暂闪红
  scene.lines.forEach((line, lineIndex) => {
    schedule(() => {
      visibleCount.value = lineIndex + 1
      if (line.tone === 'error') {
        errorIndex.value = lineIndex
        schedule(() => {
          errorIndex.value = -1
        }, 900)
      }
    }, afterCommand + lineIndex * LINE_GAP_MS)
  })

  // 3) 停留后切下一场景
  const sceneEnd = afterCommand + scene.lines.length * LINE_GAP_MS + HOLD_MS
  const hasNext = index < scenes.value.length - 1
  if (props.loopScenes || hasNext) {
    schedule(() => {
      runScene((index + 1) % scenes.value.length)
    }, sceneEnd)
  }
}

function applyMotionPreference(): void {
  const reduced = motionQuery?.matches ?? false
  staticMode.value = reduced || !props.autoplay

  if (staticMode.value) {
    // 不做动画时直接给出完整终态，比截断成两行更有信息量
    renderStatic(0)
  }
}

onMounted(() => {
  if (typeof window !== 'undefined' && typeof window.matchMedia === 'function') {
    motionQuery = window.matchMedia('(prefers-reduced-motion: reduce)')
  }

  applyMotionPreference()

  if (!staticMode.value) {
    runScene(0)
  }

  // 用户在会话中切换系统「减少动画」时同步停下
  motionQuery?.addEventListener?.('change', applyMotionPreference)
})

onUnmounted(() => {
  clearTimers()
  motionQuery?.removeEventListener?.('change', applyMotionPreference)
})
</script>

<style scoped>
/* 使用全局语义 token（src/style.css），深浅色自动跟随，无需各自覆盖 */
.rj-root {
  display: flex;
  align-items: stretch;
  gap: 14px;
  contain: layout paint;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
}

/* ---------- 左侧状态条 ---------- */
.rj-rail {
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 12px;
  flex: 0 0 auto;
}

.rj-rail-item {
  display: flex;
  align-items: center;
  gap: 7px;
  font-size: 11px;
  white-space: nowrap;
}

.rj-dot {
  width: 6px;
  height: 6px;
  border-radius: 999px;
  background: var(--accent-ok);
  box-shadow: 0 0 0 4px color-mix(in srgb, var(--accent-ok) 14%, transparent);
  animation: rj-breathe 2.4s ease-in-out infinite;
}

.rj-rail-label {
  color: var(--muted);
}

.rj-rail-value {
  color: var(--text);
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}

/* ---------- 终端窗口 ---------- */
.rj-window {
  flex: 1 1 auto;
  min-width: 0;
  overflow: hidden;
  border: 1px solid color-mix(in srgb, var(--brand-2) 24%, var(--border));
  border-radius: 16px;
  background: var(--surface);
  box-shadow: var(--shadow-float);
}

:global(.dark) .rj-window {
  background: linear-gradient(160deg, #141b26 0%, #0d1319 100%);
}

.rj-header {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 11px 15px;
  border-bottom: 1px solid var(--border);
  background: color-mix(in srgb, var(--surface-3) 60%, transparent);
}

.rj-lights {
  display: flex;
  gap: 7px;
  flex: 0 0 auto;
}

.rj-light {
  width: 11px;
  height: 11px;
  border-radius: 999px;
  display: block;
}

.rj-light-close { background: #ef4444; }
.rj-light-min { background: #eab308; }
.rj-light-max { background: #22c55e; }

.rj-title {
  flex: 1 1 auto;
  text-align: center;
  font-size: 11px;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--muted);
}

.rj-scenes {
  display: flex;
  gap: 5px;
  flex: 0 0 auto;
}

.rj-scene-dot {
  width: 6px;
  height: 6px;
  border-radius: 999px;
  background: var(--border-strong);
  transition: background 0.3s ease, transform 0.3s ease;
}

.rj-scene-dot.is-active {
  background: var(--brand);
  transform: scale(1.25);
}

/* ---------- 终端正文 ---------- */
.rj-body {
  display: flex;
  flex-direction: column;
  gap: 9px;
  padding: 20px 22px;
  min-height: 232px;
  font-size: 13px;
  line-height: 1.5;
}

.rj-line {
  display: flex;
  align-items: baseline;
  gap: 8px;
  min-width: 0;
  flex-wrap: wrap;
}

.rj-prompt {
  flex: 0 0 auto;
  color: var(--accent-ok);
  font-weight: 700;
}

.rj-cmd-text {
  color: var(--text);
  word-break: break-all;
}

/* 打字机光标：静态模式下隐藏，避免误以为是「还在加载」 */
.rj-caret {
  display: inline-block;
  width: 7px;
  height: 14px;
  background: var(--brand);
  animation: rj-blink 1s step-end infinite;
}

.rj-result {
  animation: rj-line-in 0.28s cubic-bezier(0.22, 1, 0.36, 1) both;
}

.rj-marker {
  flex: 0 0 auto;
  color: var(--brand);
  font-weight: 700;
}

.rj-label {
  flex: 0 0 auto;
  min-width: 34px;
  color: var(--brand-2);
  font-weight: 700;
}

.rj-text {
  flex: 1 1 auto;
  min-width: 0;
  color: var(--muted);
  word-break: break-word;
}

.rj-timing {
  flex: 0 0 auto;
  color: var(--muted);
  opacity: 0.72;
  font-variant-numeric: tabular-nums;
}

.rj-tone-ok .rj-text {
  color: var(--accent-ok);
  font-weight: 650;
}

.rj-tone-summary .rj-marker {
  color: var(--accent-ok);
}

.rj-tone-summary .rj-text {
  color: var(--text);
  font-weight: 650;
}

/* 失败行闪红：全站最有说服力的一帧——上游 429，客户端仍拿到 200 */
.rj-tone-error .rj-marker,
.rj-tone-error .rj-text {
  color: var(--accent-danger);
  font-weight: 650;
}

.rj-result.is-error {
  border-radius: 6px;
  background: color-mix(in srgb, var(--accent-danger) 12%, transparent);
  animation: rj-flash 0.9s ease-out 1;
}

/* ---------- 精简模式 ---------- */
.rj-is-compact .rj-window {
  border-radius: 14px;
}

.rj-is-compact .rj-body {
  min-height: 0;
  padding: 16px 18px;
  font-size: 12px;
}

/* ---------- 动画 ---------- */
@keyframes rj-blink {
  0%, 50% { opacity: 1; }
  51%, 100% { opacity: 0; }
}

@keyframes rj-line-in {
  from { opacity: 0; transform: translateY(5px); }
  to { opacity: 1; transform: translateY(0); }
}

@keyframes rj-breathe {
  50% { opacity: 0.4; transform: scale(0.8); }
}

@keyframes rj-flash {
  0% { background: color-mix(in srgb, var(--accent-danger) 30%, transparent); }
  100% { background: color-mix(in srgb, var(--accent-danger) 12%, transparent); }
}

/* 窄屏：状态条移到窗口上方，避免横向挤压 */
@media (max-width: 640px) {
  .rj-root {
    flex-direction: column;
    gap: 10px;
  }

  .rj-rail {
    flex-direction: row;
    flex-wrap: wrap;
    justify-content: flex-start;
    gap: 10px 14px;
  }

  .rj-body {
    min-height: 0;
    padding: 15px 16px;
    font-size: 12px;
  }

  .rj-label {
    min-width: 0;
  }
}
</style>
