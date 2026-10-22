import { defineConfig } from 'vitepress'

/**
 * 站点地址统一在这里维护，正文通过 {{ $site... }} 之外的方式引用时也以这里为准。
 *
 * - SITE_URL：用户访问入口，也是客户端里要填的地址。必须与主站后台「API 端点地址」一致，
 *   因为站内「使用密钥」弹窗和「导入到 CCS」生成的配置用的就是它。
 * - ALT_SITE_URL：备用地址，只在文档里作为「主地址打不开时可替换」提一句。
 * 两个域名指向同一个后端，账户、余额、Key 完全通用。
 */
const SITE_URL = 'https://codeplan.codes'
const ALT_SITE_URL = 'https://subapi.loucer.cn'

export default defineConfig({
  title: 'CodePlan 使用教程',
  description: 'CodePlan 帮助中心：创建 API Key，接入 CCSwitch、Codex、Claude Code、VS Code 与 Gemini CLI。',
  lang: 'zh-CN',
  cleanUrls: true,
  lastUpdated: true,

  // 运维说明不对用户发布，只保留在 README
  srcExclude: ['deploy.md'],

  head: [
    ['link', { rel: 'icon', type: 'image/png', href: '/favicon.png' }],
    ['link', { rel: 'apple-touch-icon', href: '/apple-touch-icon.png' }],
    ['meta', { name: 'theme-color', content: '#0d9488' }],
  ],

  themeConfig: {
    logo: { src: '/logo.png', alt: 'CodePlan' },
    siteTitle: 'CodePlan 文档',

    nav: [
      { text: '快速开始', link: '/quick-start' },
      { text: '客户端接入', link: '/ccswitch' },
      { text: '常见问题', link: '/troubleshooting' },
      { text: '返回 CodePlan', link: SITE_URL },
    ],

    sidebar: [
      {
        text: '开始使用',
        items: [
          { text: '快速开始', link: '/quick-start' },
          { text: 'API Key 与接入地址', link: '/api-key' },
          { text: '模型与分组怎么选', link: '/models-and-groups' },
          { text: '计费、余额与使用记录', link: '/billing' },
        ],
      },
      {
        text: '客户端接入',
        items: [
          { text: 'CCSwitch（推荐）', link: '/ccswitch' },
          { text: 'Codex', link: '/codex' },
          { text: 'Claude Code', link: '/claude-code' },
          { text: 'VS Code 插件', link: '/vscode' },
          { text: 'Gemini CLI', link: '/gemini-cli' },
          { text: 'OpenCode', link: '/opencode' },
        ],
      },
      {
        text: '进阶工具',
        items: [
          { text: 'Codex 生图 Skill', link: '/codex-imagegen2' },
          { text: '余额悬浮窗', link: '/balance-float' },
        ],
      },
      {
        text: '帮助',
        items: [
          { text: '常见报错排查', link: '/troubleshooting' },
          { text: '找到配置文件', link: '/config-files' },
          { text: '站内功能与入口', link: '/site-features' },
          { text: '售后与联系', link: '/support' },
          { text: '更新记录', link: '/changelog' },
        ],
      },
    ],

    socialLinks: [],

    search: {
      provider: 'local',
      options: {
        translations: {
          button: { buttonText: '搜索文档', buttonAriaLabel: '搜索文档' },
          modal: {
            noResultsText: '没有找到相关内容',
            resetButtonTitle: '清除搜索',
            footer: { selectText: '选择', navigateText: '切换', closeText: '关闭' },
          },
        },
      },
    },

    outline: { label: '本页目录', level: [2, 3] },
    docFooter: { prev: '上一篇', next: '下一篇' },
    lastUpdated: {
      text: '最后更新',
      formatOptions: { dateStyle: 'medium', timeStyle: 'short' },
    },

    returnToTopLabel: '回到顶部',
    sidebarMenuLabel: '目录',
    darkModeSwitchLabel: '外观',
    lightModeSwitchTitle: '切换到浅色模式',
    darkModeSwitchTitle: '切换到深色模式',

    footer: {
      message: `访问地址：<a href="${SITE_URL}">${SITE_URL.replace('https://', '')}</a> · 备用 <a href="${ALT_SITE_URL}">${ALT_SITE_URL.replace('https://', '')}</a>（同一站点，账户通用）`,
      copyright: '© CodePlan',
    },
  },
})
