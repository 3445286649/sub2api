<script setup lang="ts">
/**
 * 按操作系统切换的内容块：<OsTabs> 里放 #windows / #macos / #linux 具名插槽。
 *
 * - 首次访问按 navigator 猜系统，用户手动切换后记住选择，跨页面保持一致。
 * - 同一页面里的多个 OsTabs 会同步切换（用户看 Windows 就一路都是 Windows）。
 * - SSR 阶段统一渲染 Windows，挂载后再纠正，避免水合不一致。
 */
import { computed, onBeforeUnmount, onMounted, ref, useSlots } from 'vue'

type Os = 'windows' | 'macos' | 'linux'

const LABELS: Record<Os, string> = { windows: 'Windows', macos: 'macOS', linux: 'Linux' }
const STORAGE_KEY = 'codeplan-docs-os'
const SYNC_EVENT = 'codeplan-docs-os-change'

const slots = useSlots()
const available = computed(() => (Object.keys(LABELS) as Os[]).filter((os) => slots[os]))
const active = ref<Os>('windows')

function detectOs(): Os {
  const ua = navigator.userAgent
  if (/Mac|iPhone|iPad/i.test(ua)) return 'macos'
  if (/Linux|X11/i.test(ua) && !/Android/i.test(ua)) return 'linux'
  return 'windows'
}

function readSaved(): Os | null {
  try {
    const v = localStorage.getItem(STORAGE_KEY)
    return v === 'windows' || v === 'macos' || v === 'linux' ? v : null
  } catch {
    return null
  }
}

/** 选中系统不在本组插槽里时（比如只有 Win/mac 两栏却猜成 Linux），退回第一栏 */
function apply(os: Os) {
  active.value = available.value.includes(os) ? os : available.value[0]
}

function select(os: Os) {
  apply(os)
  try {
    localStorage.setItem(STORAGE_KEY, os)
  } catch {
    // 隐私模式下写不进去不影响切换
  }
  window.dispatchEvent(new CustomEvent<Os>(SYNC_EVENT, { detail: os }))
}

function onSync(e: Event) {
  apply((e as CustomEvent<Os>).detail)
}

function onKeydown(e: KeyboardEvent, index: number) {
  const list = available.value
  let next = -1
  if (e.key === 'ArrowRight') next = (index + 1) % list.length
  else if (e.key === 'ArrowLeft') next = (index - 1 + list.length) % list.length
  if (next < 0) return
  e.preventDefault()
  select(list[next])
  ;((e.currentTarget as HTMLElement).parentElement?.children[next] as HTMLElement | undefined)?.focus()
}

onMounted(() => {
  apply(readSaved() ?? detectOs())
  window.addEventListener(SYNC_EVENT, onSync)
})

onBeforeUnmount(() => window.removeEventListener(SYNC_EVENT, onSync))
</script>

<template>
  <div class="os-tabs">
    <div class="os-tabs__bar" role="tablist" aria-label="选择你的电脑系统">
      <button
        v-for="(os, i) in available"
        :key="os"
        type="button"
        role="tab"
        class="os-tabs__tab"
        :class="{ 'is-active': active === os }"
        :aria-selected="active === os"
        :tabindex="active === os ? 0 : -1"
        @click="select(os)"
        @keydown="onKeydown($event, i)"
      >
        {{ LABELS[os] }}
      </button>
    </div>
    <div v-for="os in available" v-show="active === os" :key="os" class="os-tabs__panel" role="tabpanel">
      <slot :name="os" />
    </div>
  </div>
</template>
