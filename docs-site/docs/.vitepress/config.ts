import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'subapi 使用教程',
  description: 'subapi 帮助中心：从创建 API Key 到接入 CCSwitch、Codex、Claude Code 和 VS Code。',
  lang: 'zh-CN',
  cleanUrls: true,
  lastUpdated: true,
  themeConfig: {
    nav: [
      { text: '一页速通', link: '/quick-start' },
      { text: 'API Key', link: '/api-key' },
      { text: '排障', link: '/troubleshooting' }
    ],
    sidebar: [
      {
        text: '开始使用',
        items: [
          { text: '新用户 3 分钟速通', link: '/quick-start' },
          { text: 'API Key 创建与安全', link: '/api-key' },
          { text: '计费、余额与使用记录', link: '/billing' }
        ]
      },
      {
        text: '客户端接入',
        items: [
          { text: 'CCSwitch 接入', link: '/ccswitch' },
          { text: 'Codex 接入', link: '/codex' },
          { text: 'Claude Code 接入', link: '/claude-code' },
          { text: 'VS Code 插件接入', link: '/vscode' }
        ]
      },
      {
        text: '排查',
        items: [
          { text: '常见报错排查', link: '/troubleshooting' }
        ]
      }
    ],
    search: {
      provider: 'local'
    },
    outline: {
      label: '本页目录',
      level: [2, 3]
    },
    docFooter: {
      prev: '上一篇',
      next: '下一篇'
    },
    lastUpdated: {
      text: '最后更新',
      formatOptions: {
        dateStyle: 'medium',
        timeStyle: 'short'
      }
    }
  }
})
