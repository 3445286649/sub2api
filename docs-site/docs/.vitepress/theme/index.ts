/**
 * CodePlan 文档主题：沿用 VitePress 默认主题，只覆盖配色与少量组件样式。
 *
 * 配色与主站 frontend/src/style.css 的语义 token 保持一致（teal 主色 + 紫色辅色），
 * 让用户从主站点「查看文档」过来时是同一个产品的观感。
 * 主站首页的钉屏演示、滚动吸附等动效刻意不搬过来：文档是边看边操作的工具书。
 */
import DefaultTheme from 'vitepress/theme'
import type { Theme } from 'vitepress'
import './custom.css'
import OsTabs from './components/OsTabs.vue'
import StepFlow from './components/StepFlow.vue'
import DownloadCard from './components/DownloadCard.vue'

export default {
  extends: DefaultTheme,
  enhanceApp({ app }) {
    // 正文 Markdown 里直接用：<OsTabs>、<StepFlow>、<DownloadCard>
    app.component('OsTabs', OsTabs)
    app.component('StepFlow', StepFlow)
    app.component('DownloadCard', DownloadCard)
  },
} satisfies Theme
