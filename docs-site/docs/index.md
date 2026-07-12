# subapi 使用教程

把这页当作帮助中心入口。第一次使用，直接按「一页速通」走；配置某个客户端时，进入对应专题页。

## 一页速通

1. 确认账户有余额或有效订阅。
2. 进入 **API 密钥** 页面创建 Key，并绑定可用分组。
3. 客户端填写 `https://subapi.loucer.cn/v1` 和你的 API Key。
4. 如果使用 CCSwitch，用 Google Chrome 打开站内 Key 页面并点击 **导入到 CCS**。
5. 发起一次测试请求，到站内 **使用记录** 验证是否命中 subapi。

![仪表盘入口](/images/subapi/01-dashboard-overview.png)

## 子页面导航

- [🚀 新用户 3 分钟速通](/quick-start)
- [🔑 API Key 创建与安全](/api-key)
- [🔄 CCSwitch 接入](/ccswitch)
- [💻 Codex 接入](/codex)
- [🎨 Codex 生图 Skill 接入](/codex-imagegen2)
- [🧠 Claude Code 接入](/claude-code)
- [🧩 VS Code 插件接入](/vscode)
- [🛠️ 常见报错排查](/troubleshooting)
- [💳 计费、余额与使用记录](/billing)
- [🧭 模型与分组怎么选](/models-and-groups)
- [💎 Gemini CLI 接入](/gemini-cli)
- [🧰 OpenCode 接入](/opencode)
- [🗺️ 站内功能与入口说明](/site-features)

## 常见入口

- 站点首页：`https://subapi.loucer.cn`
- API 密钥：`https://subapi.loucer.cn/keys`
- CCSwitch 官网：`https://ccswitch.io`
- CCSwitch GitHub：`https://github.com/farion1231/cc-switch/releases`

## 售后群

遇到使用或订单问题时，可以扫码加入售后群。建议优先加入一群；一群无法加入时再加入二群。

<div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(260px, 1fr)); gap: 24px; align-items: start;">
  <div>
    <h3>一群</h3>
    <p><strong>群名：</strong>loucer中转站一群<br><strong>群号：</strong><code>593768879</code></p>
    <img src="/images/subapi/after-sales-group-1.jpg" alt="loucer中转站一群二维码" style="width: 100%; max-width: 420px; border-radius: 8px;" />
  </div>
  <div>
    <h3>二群</h3>
    <p><strong>群名：</strong>loucer中转站二群<br><strong>群号：</strong><code>701461317</code></p>
    <img src="/images/subapi/after-sales-group-2.jpg" alt="loucer中转站二群二维码" style="width: 100%; max-width: 420px; border-radius: 8px;" />
  </div>
</div>

## 最后更新时间

2026-07-12：新增售后群一群、二群二维码。

2026-07-09：新增 Codex 生图 Skill 接入教程，并补充 subapi 中转站配置和 2K 生图示例。

2026-07-08：整理为 VitePress 静态帮助中心结构，适合部署到独立文档域名。
