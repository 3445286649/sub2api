<template>
  <teleport to="body">
    <transition name="usage-help-fade">
      <div
        v-if="open"
        class="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/65 px-3 py-4 backdrop-blur-sm sm:px-5"
        role="dialog"
        aria-modal="true"
        aria-labelledby="usage-help-title"
        @click.self="close"
      >
        <div class="flex h-[min(88vh,54rem)] w-full max-w-6xl flex-col overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-2xl dark:border-dark-700 dark:bg-dark-900">
          <div class="flex items-start justify-between gap-4 border-b border-gray-100 px-5 py-4 dark:border-dark-700 sm:px-6">
            <div class="min-w-0">
              <p class="text-xs font-semibold text-primary-600 dark:text-primary-400">
                SubAPI Guide
              </p>
              <h2 id="usage-help-title" class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">
                使用帮助
              </h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                从充值到账、创建 API Key，到导入 CC Switch 并验证余额的完整流程。
              </p>
            </div>
            <button
              type="button"
              class="rounded-lg p-2 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:hover:bg-dark-800 dark:hover:text-gray-200"
              aria-label="关闭"
              @click="close"
            >
              <Icon name="x" size="sm" />
            </button>
          </div>

          <div class="flex min-h-0 flex-1 flex-col lg:flex-row">
            <aside class="border-b border-gray-100 bg-gray-50/80 px-4 py-3 dark:border-dark-700 dark:bg-dark-950/30 lg:w-64 lg:border-b-0 lg:border-r lg:px-4 lg:py-5">
              <div class="flex gap-2 overflow-x-auto pb-1 lg:flex-col lg:overflow-visible lg:pb-0">
                <button
                  v-for="(step, index) in steps"
                  :key="step.id"
                  type="button"
                  class="flex shrink-0 items-center gap-2 rounded-xl px-3 py-2 text-left text-sm transition-colors lg:w-full"
                  :class="activeStep === step.id
                    ? 'bg-white text-primary-700 shadow-sm ring-1 ring-primary-100 dark:bg-dark-800 dark:text-primary-300 dark:ring-primary-500/20'
                    : 'text-gray-600 hover:bg-white hover:text-gray-900 dark:text-gray-400 dark:hover:bg-dark-800 dark:hover:text-white'"
                  @click="scrollToStep(step.id)"
                >
                  <span class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-gray-100 text-xs font-semibold text-gray-500 dark:bg-dark-700 dark:text-gray-300">
                    {{ index + 1 }}
                  </span>
                  <span class="max-w-[9rem] truncate lg:max-w-none">{{ step.title }}</span>
                </button>
              </div>
            </aside>

            <main ref="contentRef" class="min-h-0 flex-1 overflow-y-auto bg-white px-4 py-5 dark:bg-dark-900 sm:px-6">
              <div class="mx-auto max-w-4xl space-y-4">
                <div class="rounded-xl border border-primary-100 bg-primary-50 px-4 py-3 dark:border-primary-500/20 dark:bg-primary-500/10">
                  <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                    <div>
                      <div class="text-sm font-semibold text-primary-800 dark:text-primary-200">
                        API Base URL
                      </div>
                      <div class="mt-1 break-all font-mono text-sm text-primary-700 dark:text-primary-300">
                        {{ links.baseUrl }}
                      </div>
                    </div>
                    <div class="flex shrink-0 gap-2">
                      <button type="button" class="usage-help-action" @click="copyText(links.baseUrl, 'API Base URL')">
                        <Icon name="copy" size="sm" />
                        复制
                      </button>
                      <button type="button" class="usage-help-action" @click="openLink(links.baseUrl)">
                        <Icon name="externalLink" size="sm" />
                        打开
                      </button>
                    </div>
                  </div>
                </div>

                <section
                  v-for="step in steps"
                  :id="step.id"
                  :key="step.id"
                  class="scroll-mt-4 rounded-2xl border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-900 sm:p-5"
                >
                  <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                    <div>
                      <div class="flex items-center gap-2 text-xs font-semibold text-gray-400 dark:text-gray-500">
                        <Icon :name="step.icon" size="sm" />
                        {{ step.badge }}
                      </div>
                      <h3 class="mt-2 text-base font-semibold text-gray-900 dark:text-white">
                        {{ step.title }}
                      </h3>
                      <p class="mt-2 text-sm leading-6 text-gray-600 dark:text-gray-300">
                        {{ step.goal }}
                      </p>
                    </div>
                    <div v-if="step.actions?.length || step.image" class="flex shrink-0 flex-wrap gap-2">
                      <button
                        v-if="step.image"
                        type="button"
                        class="usage-help-action"
                        @click="openPreview(step.image)"
                      >
                        <Icon name="eye" size="sm" />
                        查看示意图
                      </button>
                      <button
                        v-for="action in step.actions"
                        :key="action.label"
                        type="button"
                        class="usage-help-action"
                        @click="handleAction(action)"
                      >
                        <Icon :name="action.type === 'copy' ? 'copy' : 'externalLink'" size="sm" />
                        {{ action.label }}
                      </button>
                    </div>
                  </div>

                  <div class="mt-4 space-y-4">
                    <HelpBlock title="操作步骤" :items="step.steps" icon="checkCircle" />
                    <HelpBlock title="检查点" :items="step.checks" icon="eye" tone="success" />
                    <HelpBlock title="异常处理" :items="step.fallbacks" icon="exclamationTriangle" tone="warning" />
                  </div>
                </section>
              </div>
            </main>
          </div>
        </div>

        <div
          v-if="previewImage"
          class="fixed inset-0 z-[60] flex items-center justify-center bg-slate-950/80 px-4 py-6 backdrop-blur-sm"
          role="dialog"
          aria-modal="true"
          :aria-label="previewImage.alt"
          @click.self="closePreview"
        >
          <div class="flex max-h-[88vh] w-full max-w-5xl flex-col overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-2xl dark:border-dark-700 dark:bg-dark-900">
            <div class="flex items-center justify-between gap-4 border-b border-gray-100 px-5 py-4 dark:border-dark-700">
              <div class="min-w-0">
                <h3 class="truncate text-base font-semibold text-gray-900 dark:text-white">
                  {{ previewImage.alt }}
                </h3>
                <p class="mt-1 truncate text-sm text-gray-500 dark:text-gray-400">
                  {{ previewImage.caption }}
                </p>
              </div>
              <button
                type="button"
                class="rounded-lg p-2 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:hover:bg-dark-800 dark:hover:text-gray-200"
                aria-label="关闭示意图"
                @click="closePreview"
              >
                <Icon name="x" size="sm" />
              </button>
            </div>
            <div class="flex min-h-0 flex-1 items-center justify-center overflow-auto bg-gray-50 p-3 dark:bg-dark-950 sm:p-5">
              <img
                :src="previewImage.src"
                :alt="previewImage.alt"
                class="h-auto max-h-[72vh] max-w-full rounded-xl border border-gray-200 bg-white object-contain dark:border-dark-700 dark:bg-dark-900"
              >
            </div>
          </div>
        </div>
      </div>
    </transition>
  </teleport>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onBeforeUnmount, ref, watch } from 'vue'
import { useAppStore } from '@/stores'
import Icon from '@/components/icons/Icon.vue'

type IconName = InstanceType<typeof Icon>['$props']['name']

interface StepAction {
  label: string
  type: 'copy' | 'open'
  value: string
  toastLabel: string
}

interface HelpStep {
  id: string
  title: string
  badge: string
  icon: IconName
  goal: string
  steps: string[]
  checks: string[]
  fallbacks: string[]
  actions?: StepAction[]
  image?: {
    src: string
    alt: string
    caption: string
  }
}

type HelpImage = NonNullable<HelpStep['image']>

const HelpBlock = defineComponent({
  name: 'HelpBlock',
  props: {
    title: { type: String, required: true },
    items: { type: Array as () => string[], required: true },
    icon: { type: String as () => IconName, required: true },
    tone: { type: String as () => 'default' | 'success' | 'warning', default: 'default' },
  },
  setup(props) {
    const toneClass = computed(() => ({
      default: 'text-gray-500 dark:text-gray-400',
      success: 'text-emerald-600 dark:text-emerald-400',
      warning: 'text-amber-600 dark:text-amber-400',
    }[props.tone]))

    return () => h('div', { class: 'rounded-xl bg-gray-50 p-3 dark:bg-dark-950/35' }, [
      h('div', { class: 'mb-2 flex items-center gap-2 text-sm font-semibold text-gray-800 dark:text-gray-100' }, [
        h(Icon, { name: props.icon, size: 'sm', class: toneClass.value }),
        h('span', props.title),
      ]),
      h('ul', { class: 'space-y-1.5 text-sm leading-6 text-gray-600 dark:text-gray-300' }, props.items.map((item) =>
        h('li', { class: 'flex gap-2' }, [
          h('span', { class: 'mt-2 h-1.5 w-1.5 shrink-0 rounded-full bg-gray-300 dark:bg-dark-600' }),
          h('span', item),
        ]),
      )),
    ])
  },
})

const props = defineProps<{
  open: boolean
}>()

const emit = defineEmits<{
  (event: 'update:open', value: boolean): void
}>()

const appStore = useAppStore()
const activeStep = ref('usage-help-fast-path')
const contentRef = ref<HTMLElement | null>(null)
const previewImage = ref<HelpImage | null>(null)

const links = {
  baseUrl: 'https://subapi.loucer.cn',
  ccswitch: 'https://ccswitch.io',
  windowsBackup: 'https://wwarq.lanzn.com/ie5Si3ss9ckb',
  macBackup: 'https://wwarq.lanzn.com/iGvms3ss9fgf',
  linuxGithub: 'https://github.com/farion1231/cc-switch/releases',
  bilibili: 'https://www.bilibili.com/video/BV1rfrVByEAe/?spm_id_from=333.337.search-card.all.click&vd_source=cec2b355ec3ccfb626f48f7306fd6e7b',
}

const steps: HelpStep[] = [
  {
    id: 'usage-help-fast-path',
    title: '新用户最短路径',
    badge: 'Overview',
    icon: 'lightbulb',
    goal: '只记住一条主线：先让账户有余额或订阅，再创建 API Key，最后导入 CC Switch 使用。',
    steps: [
      '登录后先看仪表盘，确认余额、订阅和左侧菜单都正常显示。',
      '通过充值商城购买兑换码，或直接在站内充值订阅。',
      '进入 API Keys 创建密钥，复制客户端配置，再用 Google Chrome 导入 CC Switch。',
    ],
    checks: [
      '余额大于 0 或订阅处于有效期内。',
      'API Key 列表能看到新建密钥。',
      'CC Switch 中状态显示为“使用中”。',
    ],
    fallbacks: [
      '余额或订阅没到账时先看订单状态，再联系售后群处理。',
      '导入没有反应时，优先确认是否使用 Google Chrome 打开网站。',
    ],
    image: {
      src: '/usage-help/01-dashboard-overview.png',
      alt: '仪表盘概览',
      caption: '登录后先从仪表盘确认账户状态。',
    },
  },
  {
    id: 'usage-help-redeem',
    title: '充值商城购买兑换码 + 站内兑换',
    badge: 'Redeem',
    icon: 'gift',
    goal: '适合先在充值商城购买卡密，再回到站内把兑换码兑换到账户。',
    steps: [
      '点击右上角“充值商城”，按页面提示购买对应面额的兑换码。',
      '回到站内兑换页面，粘贴兑换码并提交。',
      '兑换成功后刷新仪表盘，确认余额或权益已经更新。',
    ],
    checks: [
      '兑换码没有多余空格或换行。',
      '兑换页提示成功，账户余额或订阅同步变化。',
      '订单页面能查到对应记录。',
    ],
    fallbacks: [
      '兑换码提示无效时，先核对是否复制完整。',
      '已付款但未收到兑换码时，带订单号联系售后群。',
    ],
    image: {
      src: '/usage-help/02-redeem-code-page.png',
      alt: '站内兑换码页面',
      caption: '购买卡密后在站内完成兑换。',
    },
  },
  {
    id: 'usage-help-recharge',
    title: '站内直接充值/订单状态',
    badge: 'Recharge',
    icon: 'creditCard',
    goal: '如果站内充值入口可用，可以直接创建订单并通过订单状态确认是否到账。',
    steps: [
      '进入充值/订阅页面，选择需要的套餐或充值金额。',
      '完成支付后回到站内订单页面查看状态。',
      '订单为“已完成”后，再检查余额或订阅是否更新。',
    ],
    checks: [
      '订单状态不是“待支付”或“失败”。',
      '余额、订阅有效期和订单金额一致。',
      '创建 API Key 前，账户已有可用额度。',
    ],
    fallbacks: [
      '支付完成但订单未更新时，等待片刻刷新订单页面。',
      '长时间不到账时，截图订单号和支付凭证给售后处理。',
    ],
    image: {
      src: '/usage-help/03-recharge-subscription-page.png',
      alt: '充值订阅页面',
      caption: '站内充值后用订单页确认状态。',
    },
  },
  {
    id: 'usage-help-api-key',
    title: '创建 API Key 和分组提醒',
    badge: 'API Key',
    icon: 'key',
    goal: 'API Key 是导入客户端的凭证。创建时注意选择正确分组，有些分组没账号会导致探测或请求失败。',
    steps: [
      '进入 API Keys 页面，点击创建密钥。',
      '填写名称，按实际需要选择分组和额度限制。',
      '创建完成后打开“使用密钥”弹窗，复制客户端配置或点击导入按钮。',
    ],
    checks: [
      '密钥只在创建后完整展示一次，后续页面会打码。',
      '分组内必须有可用账号，否则客户端显示正常但请求可能失败。',
      '需要给下游使用时，复制的是客户端配置，不是后台管理信息。',
    ],
    fallbacks: [
      '忘记保存完整密钥时，删除旧 Key 后重新创建。',
      '下游反馈余额不显示时，先用 PowerShell 或浏览器确认接口能否正常返回。',
    ],
    image: {
      src: '/usage-help/08-use-api-key-modal.png',
      alt: '使用 API Key 弹窗',
      caption: '截图使用打码版本，不展示完整密钥。',
    },
  },
  {
    id: 'usage-help-download',
    title: '下载并安装 CC Switch',
    badge: 'Download',
    icon: 'download',
    goal: '先安装 CC Switch，再从网站导入配置。Windows、macOS、Linux 可按自己的系统选择下载。',
    steps: [
      '优先打开 CC Switch 官网下载最新版本。',
      '官网访问异常时，可使用 Windows 或 macOS 备用下载链接。',
      'Linux 用户或需要历史版本时，到 GitHub Releases 页面下载。',
    ],
    checks: [
      '安装完成后，本机能打开 CC Switch。',
      '浏览器导入前，CC Switch 已经启动或允许被唤起。',
      'Windows 安装包如被拦截，确认来源后再手动允许。',
    ],
    fallbacks: [
      '官网下载慢时改用备用下载链接。',
      '安装后无法启动时，先查看系统安全软件是否拦截。',
    ],
    actions: [
      { label: '打开官网', type: 'open', value: links.ccswitch, toastLabel: 'CC Switch 官网' },
      { label: 'Windows 备用', type: 'open', value: links.windowsBackup, toastLabel: 'Windows 备用下载' },
      { label: 'macOS 备用', type: 'open', value: links.macBackup, toastLabel: 'macOS 备用下载' },
      { label: 'GitHub', type: 'open', value: links.linuxGithub, toastLabel: 'GitHub Releases' },
    ],
    image: {
      src: '/usage-help/12-ccswitch-download-links.png',
      alt: 'CC Switch 下载链接',
      caption: '按系统选择下载入口。',
    },
  },
  {
    id: 'usage-help-chrome-import',
    title: 'Google Chrome 导入 CCSwitch',
    badge: 'Import',
    icon: 'globe',
    goal: '导入动作依赖浏览器唤起本地应用，建议用 Google Chrome 打开站内页面再点击导入。',
    steps: [
      '用 Google Chrome 登录网站，不建议在微信内置浏览器或不兼容浏览器里操作。',
      '进入 API Key 的“使用密钥”弹窗，点击导入到 CC Switch。',
      '浏览器弹出确认时允许打开 CC Switch。',
    ],
    checks: [
      '浏览器地址栏是正常网站地址，不是聊天软件内置网页。',
      '点击导入后，CC Switch 能被唤起并出现配置。',
      '导入的 Base URL 为 https://subapi.loucer.cn。',
    ],
    fallbacks: [
      '点击导入无反应时，换 Google Chrome 重新登录后再试。',
      '仍无法唤起时，手动复制 Base URL 和 API Key 到 CC Switch。',
    ],
    actions: [
      { label: '复制 Base URL', type: 'copy', value: links.baseUrl, toastLabel: 'API Base URL' },
    ],
    image: {
      src: '/usage-help/10-chrome-import-to-ccswitch.png',
      alt: 'Chrome 导入 CC Switch',
      caption: '在 Google Chrome 中完成导入动作最稳定。',
    },
  },
  {
    id: 'usage-help-enable',
    title: 'CCSwitch 启用到“使用中”',
    badge: 'Enable',
    icon: 'checkCircle',
    goal: '导入成功不等于已经启用，需要在 CC Switch 中把配置切换到“使用中”。',
    steps: [
      '打开 CC Switch，确认列表里已经出现刚导入的站点配置。',
      '选中该配置并点击启用。',
      '状态变为“使用中”后，再进行余额探活或请求测试。',
    ],
    checks: [
      '列表里只有一个当前要用的配置处于“使用中”。',
      '站点名称、Base URL 和密钥对应同一个账号。',
      '余额探活或模型请求返回正常。',
    ],
    fallbacks: [
      '列表里配置太多时，先禁用旧配置，避免下游误用。',
      '启用后仍请求失败，回站内确认 Key 没被禁用、余额/订阅仍有效。',
    ],
    image: {
      src: '/usage-help/11-ccswitch-imported-list.png',
      alt: 'CC Switch 已导入列表',
      caption: '导入后还需要启用到“使用中”。',
    },
  },
  {
    id: 'usage-help-faq',
    title: '常见问题',
    badge: 'FAQ',
    icon: 'questionCircle',
    goal: '这里收拢最常见的卡点：余额不到账、导入失败、下游看不到余额、分组没有账号。',
    steps: [
      '余额不到账：先看订单状态，订单已完成仍异常再联系售后。',
      '导入失败：确认使用 Google Chrome，并且本机已经安装 CC Switch。',
      '下游看不到余额：先用网站/API Key 自测，确认 Key、Base URL、余额和分组都正常。',
    ],
    checks: [
      'Base URL 固定使用 https://subapi.loucer.cn。',
      'API Key 不能多复制空格，也不要把后台截图发给下游。',
      '分组里必须放账号，否则探活或请求可能失败。',
    ],
    fallbacks: [
      '需要视频版流程时，打开 Bilibili 教程跟着做。',
      '遇到订单、兑换码、密钥泄露等问题，及时联系售后群。',
    ],
    actions: [
      { label: '视频教程', type: 'open', value: links.bilibili, toastLabel: 'Bilibili 教程' },
      { label: '复制 Base URL', type: 'copy', value: links.baseUrl, toastLabel: 'API Base URL' },
    ],
    image: {
      src: '/usage-help/13-ccswitch-video-guide.png',
      alt: 'Bilibili 视频教程',
      caption: '视频教程可作为备用参考。',
    },
  },
]

watch(() => props.open, (isOpen) => {
  if (typeof document === 'undefined') return
  document.body.style.overflow = isOpen ? 'hidden' : ''
  if (isOpen) {
    activeStep.value = steps[0].id
  }
}, { immediate: true })

onBeforeUnmount(() => {
  if (typeof document !== 'undefined') {
    document.body.style.overflow = ''
  }
})

function close() {
  closePreview()
  emit('update:open', false)
}

function scrollToStep(id: string) {
  activeStep.value = id
  const target = document.getElementById(id)
  target?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

function openLink(url: string) {
  window.open(url, '_blank', 'noopener,noreferrer')
}

function openPreview(image: HelpImage) {
  previewImage.value = image
}

function closePreview() {
  previewImage.value = null
}

async function copyText(text: string, label: string) {
  try {
    await navigator.clipboard.writeText(text)
    appStore.showSuccess(`${label} 已复制`, 1800)
  } catch (error) {
    console.error('Failed to copy usage help text:', error)
    appStore.showError('复制失败，请手动复制', 2200)
  }
}

function handleAction(action: StepAction) {
  if (action.type === 'copy') {
    void copyText(action.value, action.toastLabel)
    return
  }
  openLink(action.value)
}
</script>

<style scoped>
.usage-help-action {
  @apply inline-flex items-center gap-1.5 rounded-lg border border-gray-200 bg-white px-3 py-1.5 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50 hover:text-gray-900 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-300 dark:hover:bg-dark-700 dark:hover:text-white;
}

.usage-help-fade-enter-active,
.usage-help-fade-leave-active {
  transition: opacity 0.16s ease;
}

.usage-help-fade-enter-from,
.usage-help-fade-leave-to {
  opacity: 0;
}
</style>
