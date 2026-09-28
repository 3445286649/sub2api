# 快速开始

目标：用最短的路径完成 **充值 → 创建 Key → 接入客户端 → 发出一次能验证的请求**。第一次使用，照着做大约 3 分钟。

![仪表盘](/images/subapi/01-dashboard-overview.png)

## 开始前

- 打开 CodePlan：[codeplan.codes](https://codeplan.codes) 或 [subapi.loucer.cn](https://subapi.loucer.cn)。两个地址是同一个站点，账户通用。
- 注册并登录账户。

## 第一步：充值或兑换

账户里要有余额或有效订阅，请求才能发出去。

- **有兑换码**：左侧 **兑换**，粘贴兑换码。
- **站内充值**：左侧 **充值/订阅**，选择金额和支付方式。

详细说明见 [计费、余额与使用记录](/billing)。

## 第二步：创建 API Key

1. 左侧 **API 密钥** → **创建密钥**。
2. 写一个名称，**选择分组**（必选）。
3. 保存。

分组不知道怎么选，见 [模型与分组怎么选](/models-and-groups)。

## 第三步：接入客户端

**推荐用 CCSwitch 一键导入**，Codex、Claude Code、VS Code 插件都能直接用：

1. 安装 [CCSwitch](/ccswitch#_1-下载并安装)。
2. 用 Chrome 打开 [API 密钥](https://codeplan.codes/keys)，点击 Key 右侧的 **导入到 CCS**。
3. 在 CCSwitch 里点击 **启用**，直到 **CodePlan** 显示 **使用中**。
4. 重新打开你的客户端。

不想装 CCSwitch？看对应客户端的手动配置：[Codex](/codex) · [Claude Code](/claude-code) · [VS Code](/vscode) · [Gemini CLI](/gemini-cli) · [OpenCode](/opencode)。

## 第四步：验证

在客户端里发一句：

```text
用一句话回复：连接测试成功。
```

然后打开站内 [使用记录](https://codeplan.codes/usage)。

::: tip 成功的标志
**使用记录里出现了这次调用**，才说明请求真的走了 CodePlan。客户端能回复但记录里没有，见 [能回复但没有使用记录](/troubleshooting#能回复但没有使用记录)。
:::

## 遇到问题

- 报错：[常见报错排查](/troubleshooting)
- 找人：[售后与联系](/support)
