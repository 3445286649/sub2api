# Claude Code 接入

这一页讲的是让 Claude Code 通过 subapi 使用可用模型。推荐路径和 Codex 一样：先用 CCSwitch 导入并启用 loucer_api，测试连通后再打开 Claude Code。

## 最短路径

1. 在 subapi 创建 API Key，并绑定可用分组。
2. 用 Chrome 打开 API Key 页面，点击 **导入到 CCS**。
3. 在 CCSwitch 中点击 loucer_api 的 **启用**，直到显示 **使用中**。
4. 在 CCSwitch 中点击 **刷新用量**，确认 Key 和地址可用。
5. 关闭旧的 Claude Code 终端或 VS Code 窗口。
6. 重新打开 Claude Code，发送一句测试消息。
7. 回到 subapi 的 **使用记录**，确认出现新记录。

## 先确认前置条件

1. 账户有余额或有效订阅。
2. 已经创建 API Key，并且 Key 绑定了可用分组。
3. CCSwitch 中 loucer_api 已经显示 **使用中**。

如果还没创建 Key，先看 [API Key 创建与安全](/api-key)。

![API 密钥页面](/images/subapi/04-api-keys-page.png)

## 1. 导入到 CCSwitch

用 Google Chrome 打开：

~~~text
https://subapi.loucer.cn/keys
~~~

找到要使用的 Key，点击 **导入到 CCS**。

![Chrome 导入 CCSwitch](/images/subapi/10-chrome-import-to-ccswitch.png)

如果 Chrome 弹出是否打开 CCSwitch，选择允许。

## 2. 点击启用

回到 CCSwitch，确认 loucer_api 出现在列表中，地址显示为：

~~~text
https://subapi.loucer.cn
~~~

然后点击 **启用**，直到它显示 **使用中**。

![CCSwitch 确认 loucer_api 正在使用](/images/subapi/ccswitch-active-config.png)

::: warning 重要
Claude Code 打开前，最好先确认 CCSwitch 已经启用 loucer_api。如果先打开 Claude Code，再切换 CCSwitch，旧进程可能不会立刻读取新配置。
:::

## 3. 测试 CCSwitch 连通

在 CCSwitch 里点击 **刷新用量**。

能看到余额或用量，说明当前 Key 和地址基本可用。

如果失败，先排查：

- loucer_api 是否显示 **使用中**。
- API Key 是否完整。
- Key 是否有可用分组。
- 余额或订阅是否有效。

## 4. 打开 Claude Code

确认 CCSwitch 正常后，再打开 Claude Code。

推荐顺序：

1. 关闭已经打开的 Claude Code 终端或 VS Code 窗口。
2. 确认 CCSwitch 中 loucer_api 是 **使用中**。
3. 重新打开终端或 VS Code。
4. 启动 Claude Code。
5. 输入一句最小测试：

~~~text
用一句话回复：连接测试成功。
~~~

如果能正常对话，并且站内 **使用记录** 出现调用记录，说明 Claude Code 已经走 subapi。

## 5. 手动配置方式

不同版本的 Claude Code 和插件支持的供应商字段可能不同。原则是：

- 如果支持 OpenAI Compatible / OpenAI 兼容：使用 subapi 的 OpenAI 兼容地址。
- 如果只支持 Anthropic 官方字段：不要硬填 OpenAI 地址，优先使用站内 **使用密钥** 弹窗或你当前客户端支持的兼容配置。

OpenAI 兼容模式常用配置：

~~~bash
export OPENAI_API_KEY="sk-你的密钥"
export OPENAI_BASE_URL="https://subapi.loucer.cn/v1"
~~~

Windows PowerShell 建议在系统环境变量里设置：

~~~powershell
setx OPENAI_API_KEY "sk-你的密钥"
setx OPENAI_BASE_URL "https://subapi.loucer.cn/v1"
~~~

设置后重新打开终端或 VS Code，再打开 Claude Code。

::: tip 不确定填哪个地址
能自己填写完整 OpenAI Base URL 的地方，优先填 `https://subapi.loucer.cn/v1`。

如果插件会自动追加 `/v1`，再改成 `https://subapi.loucer.cn`。
:::

## 6. VS Code 里的 Claude Code 插件

如果你是在 VS Code 里使用 Claude Code 插件，优先找这些配置项：

| 字段 | 填写 |
| --- | --- |
| Provider | OpenAI Compatible / Custom OpenAI / OpenAI 兼容 |
| API Key | 站内创建的 sk-... |
| Base URL | https://subapi.loucer.cn/v1 |
| Model | 选择 Key 绑定分组支持的模型 |

下图是 VS Code Claude Code 插件的配置示意。不同插件字段名称可能略有差异，核心就是 Provider、API Key、Base URL、Model 这四项。

![VS Code Claude Code 插件配置示意](/images/subapi/vscode-claude-code-settings.png)

如果插件会自动追加 /v1，Base URL 填：

~~~text
https://subapi.loucer.cn
~~~

保存后建议重新加载 VS Code 窗口，再发起一次测试对话。

## 7. 怎么判断是否成功

以站内 **使用记录** 为准：

- 有记录：请求命中了 subapi。
- 没记录：Claude Code 可能仍在走官方账号、其它供应商或旧配置。

## 常见现象

| 现象 | 优先检查 |
| --- | --- |
| Claude Code 能回复，但站内没有记录 | 当前客户端可能仍在走官方 Claude 登录态或旧 Provider |
| CCSwitch 刷新用量失败 | loucer_api 是否启用、Key 是否绑定分组、余额是否可用 |
| 报 401 / unauthorized | Key 错误、Key 被禁用或复制时多了空格 |
| 报 model not found | 当前模型不在 Key 可用分组内 |

如果报 401、余额不足、model not found，看 [常见报错排查](/troubleshooting)。
